/*
Copyright 2011-2025 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
you may obtain a copy of the License at

                http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package io provides the implementations of a Writer and a Reader
// used to respectively losslessly compress and decompress data.
package io

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/bitstream"
	"github.com/flanglet/kanzi-go/v2/entropy"
	"github.com/flanglet/kanzi-go/v2/hash"
	"github.com/flanglet/kanzi-go/v2/internal"
	"github.com/flanglet/kanzi-go/v2/transform"
)

// Write to/read from bitstream using a 2 step process:
// Encoding:
// - step 1: a ByteFunction is used to reduce the size of the input data (bytes input & output)
// - step 2: an EntropyEncoder is used to entropy code the results of step 1 (bytes input, bits output)
// Decoding is the exact reverse process.

const (
	_BITSTREAM_TYPE             = 0x4B414E5A // "KANZ"
	_BITSTREAM_FORMAT_VERSION   = 6
	_STREAM_DEFAULT_BUFFER_SIZE = 256 * 1024
	_EXTRA_BUFFER_SIZE          = 512
	_COPY_BLOCK_MASK            = 0x80
	_TRANSFORMS_MASK            = 0x10
	_MIN_BITSTREAM_BLOCK_SIZE   = 1024
	_MAX_BITSTREAM_BLOCK_SIZE   = 1024 * 1024 * 1024
	_SMALL_BLOCK_SIZE           = 15
	_MAX_CONCURRENCY            = 64
	_CANCEL_TASKS_ID            = -1
)

// IOError an extended error containing a message and a code value
type IOError struct {
	msg  string
	code int
}

// Error returns the underlying error
func (this IOError) Error() string {
	return fmt.Sprintf("%v (code %v)", this.msg, this.code)
}

// Message returns the message string associated with the error
func (this IOError) Message() string {
	return this.msg
}

// ErrorCode returns the code value associated with the error
func (this IOError) ErrorCode() int {
	return this.code
}

type blockBuffer struct {
	// Enclose a slice in a struct to share it between stream and tasks
	// and reduce memory allocation.
	// The tasks can re-allocate the slice as needed.
	Buf []byte
}

// Writer a Writer that writes compressed data
// to an OutputBitStream.
type Writer struct {
	blockSize     int
	hasher32      *hash.XXHash32
	hasher64      *hash.XXHash64
	buffers       []blockBuffer
	entropyType   uint32
	transformType uint64
	inputSize     int64
	obs           kanzi.OutputBitStream
	initialized   int32
	closed        int32
	blockID       int32
	jobs          int
	nbInputBlocks int
	available     int
	listeners     []kanzi.Listener
	ctx           map[string]any
	headless      bool
}

type encodingTask struct {
	iBuffer            *blockBuffer
	oBuffer            *blockBuffer
	hasher32           *hash.XXHash32
	hasher64           *hash.XXHash64
	blockLength        uint
	blockTransformType uint64
	blockEntropyType   uint32
	currentBlockID     int32
	processedBlockID   *int32
	wg                 *sync.WaitGroup
	listeners          []kanzi.Listener
	obs                kanzi.OutputBitStream
	ctx                map[string]any
}

type encodingTaskResult struct {
	err *IOError
}

// NewWriter creates a new instance of Writer.
// The writer writes compressed data blocks to the provided os.
// Use 0 if the file size is not available
// Use headerless == false to create a bitstream without a header. The decompressor
// must know all the compression parameters to be able to decompress the bitstream.
// The headerless mode is only useful in very specific scenarios.
// checksum must be 0, 32 or 64
func NewWriter(os io.WriteCloser, transform, entropy string, blockSize, jobs uint, checksum uint, fileSize int64, headerless bool) (*Writer, error) {
	ctx := make(map[string]any)
	ctx["entropy"] = entropy
	ctx["transform"] = transform
	ctx["blockSize"] = blockSize
	ctx["jobs"] = jobs
	ctx["checksum"] = checksum
	ctx["fileSize"] = fileSize
	ctx["headerless"] = headerless
	return NewWriterWithCtx(os, ctx)
}

// NewWriterWithCtx creates a new instance of Writer using a
// map of parameters and a writer.
// The writer writes compressed data blocks to the provided os
// using a default output bitstream.
func NewWriterWithCtx(os io.WriteCloser, ctx map[string]any) (*Writer, error) {
	var err error
	var obs kanzi.OutputBitStream

	if obs, err = bitstream.NewDefaultOutputBitStream(os, _STREAM_DEFAULT_BUFFER_SIZE); err != nil {
		errMsg := fmt.Sprintf("Cannot create output bit stream: %v", err)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_CREATE_BITSTREAM}
	}

	return createWriterWithCtx(obs, ctx)
}

// NewWriterWithCtx2 creates a new instance of Writer using a
// map of parameters and a custom output bitstream.
// The writer writes compressed data blocks to the provided output bitstream.
func NewWriterWithCtx2(obs kanzi.OutputBitStream, ctx map[string]any) (*Writer, error) {
	return createWriterWithCtx(obs, ctx)
}

func createWriterWithCtx(obs kanzi.OutputBitStream, ctx map[string]any) (*Writer, error) {
	if obs == nil {
		return nil, &IOError{msg: "Invalid null output bitstream parameter", code: kanzi.ERR_INVALID_PARAM}
	}

	if ctx == nil {
		return nil, &IOError{msg: "Invalid null context parameter", code: kanzi.ERR_INVALID_PARAM}
	}

	entropyCodec := ctx["entropy"].(string)
	t := ctx["transform"].(string)
	tasks := ctx["jobs"].(uint)

	if tasks == 0 || tasks > _MAX_CONCURRENCY {
		errMsg := fmt.Sprintf("The number of jobs must be in [1..%d], got %d", _MAX_CONCURRENCY, tasks)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_INVALID_PARAM}
	}

	bSize := ctx["blockSize"].(uint)

	if bSize > _MAX_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at most %d MB", _MAX_BITSTREAM_BLOCK_SIZE>>20)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_INVALID_PARAM}
	}

	if bSize < _MIN_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at least %d", _MIN_BITSTREAM_BLOCK_SIZE)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_INVALID_PARAM}
	}

	if int(bSize)&-16 != int(bSize) {
		return nil, &IOError{msg: "The block size must be a multiple of 16", code: kanzi.ERR_INVALID_PARAM}
	}

	this := &Writer{}
	this.obs = obs
	this.ctx = ctx

	// Check entropy type validity (panic on error)
	var eType uint32
	var err error

	if eType, err = entropy.GetType(entropyCodec); err != nil {
		return nil, &IOError{msg: err.Error(), code: kanzi.ERR_INVALID_PARAM}
	}

	this.entropyType = eType

	// Check transform type validity
	this.transformType, err = transform.GetType(t)

	if err != nil {
		return nil, &IOError{msg: err.Error(), code: kanzi.ERR_INVALID_PARAM}
	}

	this.blockSize = int(bSize)
	this.available = 0
	nbBlocks := 0

	// If input size has been provided, calculate the number of blocks
	if val, hasKey := ctx["fileSize"]; hasKey {
		this.inputSize = val.(int64)
		nbBlocks = int((this.inputSize + int64(bSize-1)) / int64(bSize))
	}

	this.nbInputBlocks = min(nbBlocks, _MAX_CONCURRENCY-1)

	if checksum := ctx["checksum"].(uint); checksum != 0 {
		var err error

		if checksum == 32 {
			this.hasher32, err = hash.NewXXHash32(_BITSTREAM_TYPE)
		} else if checksum == 64 {
			this.hasher64, err = hash.NewXXHash64(_BITSTREAM_TYPE)
		} else {
			err = &IOError{msg: "The lock checksum size must be 32 or 64 bits", code: kanzi.ERR_INVALID_PARAM}
		}

		if err != nil {
			return nil, err
		}
	}

	if hdl, hasKey := ctx["headerless"]; hasKey == true {
		this.headless = hdl.(bool)
	} else {
		this.headless = false
	}

	ctx["bsVersion"] = uint(_BITSTREAM_FORMAT_VERSION)
	this.jobs = int(tasks)
	this.buffers = make([]blockBuffer, 2*this.jobs)

	// Allocate first buffer and add padding for incompressible blocks
	bufSize := max(this.blockSize+this.blockSize>>3, 256*1024)
	this.buffers[0] = blockBuffer{Buf: make([]byte, bufSize)}
	this.buffers[this.jobs] = blockBuffer{Buf: make([]byte, 0)}

	for i := 1; i < this.jobs; i++ {
		this.buffers[i] = blockBuffer{Buf: make([]byte, 0)}
		this.buffers[i+this.jobs] = blockBuffer{Buf: make([]byte, 0)}
	}

	this.blockID = 0
	this.listeners = make([]kanzi.Listener, 0)
	return this, nil
}

// AddListener adds an event listener to this writer.
// Returns true if the listener has been added.
func (this *Writer) AddListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

// RemoveListener removes an event listener from this writer.
// Returns true if the listener has been removed.
func (this *Writer) RemoveListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	for i, e := range this.listeners {
		if e == bl {
			this.listeners = append(this.listeners[:i], this.listeners[i+1:]...)
			return true
		}
	}

	return false
}

func (this *Writer) writeHeader() *IOError {
	if this.headless == true || atomic.SwapInt32(&this.initialized, 1) != 0 {
		return nil
	}

	ckSize := 0

	if this.hasher32 != nil {
		ckSize = 1
	} else if this.hasher64 != nil {
		ckSize = 2
	}

	if this.obs.WriteBits(_BITSTREAM_TYPE, 32) != 32 {
		return &IOError{msg: "Cannot write bitstream type to header", code: kanzi.ERR_WRITE_FILE}
	}

	if this.obs.WriteBits(_BITSTREAM_FORMAT_VERSION, 4) != 4 {
		return &IOError{msg: "Cannot write bitstream version to header", code: kanzi.ERR_WRITE_FILE}
	}

	if this.obs.WriteBits(uint64(ckSize), 2) != 2 {
		return &IOError{msg: "Cannot write checksum size to header", code: kanzi.ERR_WRITE_FILE}
	}

	if this.obs.WriteBits(uint64(this.entropyType), 5) != 5 {
		return &IOError{msg: "Cannot write entropy type to header", code: kanzi.ERR_WRITE_FILE}
	}

	if this.obs.WriteBits(uint64(this.transformType), 48) != 48 {
		return &IOError{msg: "Cannot write transform types to header", code: kanzi.ERR_WRITE_FILE}
	}

	if this.obs.WriteBits(uint64(this.blockSize>>4), 28) != 28 {
		return &IOError{msg: "Cannot write block size to header", code: kanzi.ERR_WRITE_FILE}
	}

	// this.inputSize not provided or >= 2^48 -> 0, <2^16 -> 1, <2^32 -> 2, <2^48 -> 3
	var szMask uint

	switch {
	case this.inputSize == 0:
		szMask = 0
	case this.inputSize >= (int64(1) << 48):
		szMask = 0
	case this.inputSize >= (int64(1) << 32):
		szMask = 3
	case this.inputSize >= (int64(1) << 16):
		szMask = 2
	default:
		szMask = 1
	}

	if this.obs.WriteBits(uint64(szMask), 2) != 2 {
		return &IOError{msg: "Cannot write size of input to header", code: kanzi.ERR_WRITE_FILE}
	}

	if szMask > 0 {
		if this.obs.WriteBits(uint64(this.inputSize), 16*szMask) != 16*szMask {
			return &IOError{msg: "Cannot write size of input to header", code: kanzi.ERR_WRITE_FILE}
		}
	}

	padding := uint64(0)

	if this.obs.WriteBits(padding, 15) != 15 {
		return &IOError{msg: "Cannot write padding to header", code: kanzi.ERR_WRITE_FILE}
	}

	seed := uint32(0x01030507 * _BITSTREAM_FORMAT_VERSION)
	HASH := uint32(0x1E35A7BD)
	cksum := HASH * seed
	cksum ^= (HASH * uint32(^ckSize))
	cksum ^= (HASH * uint32(^this.entropyType))
	cksum ^= (HASH * uint32((^this.transformType)>>32))
	cksum ^= (HASH * uint32(^this.transformType))
	cksum ^= (HASH * uint32(^this.blockSize))

	if szMask > 0 {
		cksum ^= (HASH * uint32((^this.inputSize)>>32))
		cksum ^= (HASH * uint32(^this.inputSize))
	}

	cksum = (cksum >> 23) ^ (cksum >> 3)

	if this.obs.WriteBits(uint64(cksum), 24) != 24 {
		return &IOError{msg: "Cannot write checksum to header", code: kanzi.ERR_WRITE_FILE}
	}

	return nil
}

// Write writes len(block) bytes from block to the underlying data stream.
// Returns the number of bytes written from block (0 <= n <= len(block)) and
// any error encountered that caused the write to stop early.
func (this *Writer) Write(block []byte) (int, error) {
	if atomic.LoadInt32(&this.closed) == 1 {
		return 0, &IOError{msg: "Stream closed", code: kanzi.ERR_WRITE_FILE}
	}

	off := 0
	remaining := len(block)

	for remaining > 0 {
		lenChunk := remaining
		bufOff := this.available % this.blockSize

		if lenChunk > this.blockSize-bufOff {
			lenChunk = this.blockSize - bufOff
		}

		if lenChunk > 0 {
			// Process a chunk of in-buffer data. No access to bitstream required
			bufID := this.available / this.blockSize
			copy(this.buffers[bufID].Buf[bufOff:], block[off:off+lenChunk])
			bufOff += lenChunk
			off += lenChunk
			remaining -= lenChunk
			this.available += lenChunk

			if bufOff >= this.blockSize {
				if bufID+1 < this.jobs {
					// Current write buffer is full
					if len(this.buffers[bufID+1].Buf) == 0 {
						bufSize := max(this.blockSize+this.blockSize>>3, 256*1024)
						this.buffers[bufID+1].Buf = make([]byte, bufSize)
					}
				} else {
					// If all buffers are full, time to encode
					if err := this.processBlock(); err != nil {
						return len(block) - remaining, err
					}
				}
			}

			if remaining == 0 {
				break
			}
		}
	}

	return len(block) - remaining, nil
}

// Close writes the buffered data to the writer then writes
// a final empty block and releases resources.
// Close makes the bitstream unavailable for further writes. Idempotent.
func (this *Writer) Close() error {
	if atomic.SwapInt32(&this.closed, 1) == 1 {
		return nil
	}

	if err := this.processBlock(); err != nil {
		return err
	}

	// Write end block of size 0
	this.obs.WriteBits(0, 5) // write length-3 (5 bits max)
	this.obs.WriteBits(0, 3)

	if err := this.obs.Close(); err != nil {
		return err
	}

	// Release resources
	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: make([]byte, 0)}
	}

	return nil
}

func (this *Writer) processBlock() error {
	if err := this.writeHeader(); err != nil {
		return err
	}

	if this.available == 0 {
		return nil
	}

	off := 0

	// Protect against future concurrent modification of the list of block listeners
	listeners := make([]kanzi.Listener, len(this.listeners))
	copy(listeners, this.listeners)

	nbTasks := this.jobs
	var jobsPerTask []uint

	// Assign optimal number of tasks and jobs per task (if the number of blocks is known)
	if nbTasks > 1 {
		// Limit the number of jobs if there are fewer blocks that this.jobs
		// It allows more jobs per task and reduces memory usage.
		if this.nbInputBlocks > 0 {
			nbTasks = min(nbTasks, this.nbInputBlocks)
		}

		jobsPerTask, _ = internal.ComputeJobsPerTask(make([]uint, nbTasks), uint(this.jobs), uint(nbTasks))
	} else {
		jobsPerTask = []uint{uint(this.jobs)}
	}

	tasks := 0
	wg := sync.WaitGroup{}
	results := make([]encodingTaskResult, nbTasks)
	firstID := this.blockID

	// Invoke as many go routines as required
	for taskID := 0; taskID < nbTasks; taskID++ {
		dataLength := this.available

		if dataLength > this.blockSize {
			dataLength = this.blockSize
		}

		if dataLength == 0 {
			break
		}

		copyCtx := make(map[string]any)

		for k, v := range this.ctx {
			copyCtx[k] = v
		}

		copyCtx["jobs"] = jobsPerTask[taskID]
		wg.Add(1)
		tasks++
		off += dataLength
		this.available -= dataLength

		task := encodingTask{
			iBuffer:            &this.buffers[taskID],
			oBuffer:            &this.buffers[this.jobs+taskID],
			hasher32:           this.hasher32,
			hasher64:           this.hasher64,
			blockLength:        uint(dataLength),
			blockTransformType: this.transformType,
			blockEntropyType:   this.entropyType,
			currentBlockID:     firstID + int32(taskID) + 1,
			processedBlockID:   &this.blockID,
			wg:                 &wg,
			obs:                this.obs,
			listeners:          listeners,
			ctx:                copyCtx}

		// Invoke the tasks concurrently
		go task.encode(&results[taskID])
	}

	// Wait for completion of all tasks
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return r.err
		}
	}

	return nil
}

// GetWritten returns the number of bytes written so far
func (this *Writer) GetWritten() uint64 {
	return (this.obs.Written() + 7) >> 3
}

// Encode mode + transformed entropy coded data
// mode | 0b10000000 => copy block
// mode | 0b0yy00000 => size(size(block))-1
// mode | 0b000y0000 => 1 if more than 4 transforms
//
// case 4 transforms or less
// mode | 0b0000yyyy => transform sequence skip flags (1 means skip)
//
// case more than 4 transforms
// mode | 0b00000000
//
// then 0byyyyyyyy => transform sequence skip flags (1 means skip)
func (this *encodingTask) encode(res *encodingTaskResult) {
	data := this.iBuffer.Buf
	buffer := this.oBuffer.Buf
	mode := byte(0)
	checksum := uint64(0)

	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				res.err = &IOError{msg: v.Error(), code: kanzi.ERR_PROCESS_BLOCK}
			default:
				res.err = &IOError{msg: fmt.Sprint(v), code: kanzi.ERR_PROCESS_BLOCK}
			}
		}

		// Unblock other tasks
		if res.err != nil {
			atomic.StoreInt32(this.processedBlockID, _CANCEL_TASKS_ID)
		} else {
			atomic.CompareAndSwapInt32(this.processedBlockID, this.currentBlockID-1, this.currentBlockID)
		}

		this.wg.Done()
	}()

	hashType := kanzi.EVT_HASH_NONE

	// Compute block checksum
	if this.hasher32 != nil {
		checksum = uint64(this.hasher32.Hash(data[0:this.blockLength]))
		hashType = kanzi.EVT_HASH_32BITS
	} else if this.hasher64 != nil {
		checksum = this.hasher64.Hash(data[0:this.blockLength])
		hashType = kanzi.EVT_HASH_64BITS
	}

	if len(this.listeners) > 0 {
		// Notify before transform
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, int(this.currentBlockID),
			int64(this.blockLength), checksum, hashType, time.Now())
		notifyListeners(this.listeners, evt)
	}

	if this.blockLength <= _SMALL_BLOCK_SIZE {
		this.blockTransformType = transform.NONE_TYPE
		this.blockEntropyType = entropy.NONE_TYPE
		mode |= byte(_COPY_BLOCK_MASK)
	} else {
		if skipOpt, hasKey := this.ctx["skipBlocks"]; hasKey == true {
			if skipOpt.(bool) == true {
				skip := false

				if this.blockLength >= 8 {
					skip = internal.IsDataCompressed(internal.GetMagicType(data))
				}

				if skip == false {
					histo := [256]int{}
					internal.ComputeHistogram(data[0:this.blockLength], histo[:], true, false)
					entropy1024 := internal.ComputeFirstOrderEntropy1024(int(this.blockLength), histo[:])
					skip = entropy1024 >= entropy.INCOMPRESSIBLE_THRESHOLD
					//this.ctx["histo0"] = histo
				}

				if skip == true {
					this.blockTransformType = transform.NONE_TYPE
					this.blockEntropyType = entropy.NONE_TYPE
					mode |= _COPY_BLOCK_MASK
				}
			}
		}
	}

	this.ctx["size"] = this.blockLength
	t, err := transform.New(&this.ctx, this.blockTransformType)

	if err != nil {
		res.err = &IOError{msg: err.Error(), code: kanzi.ERR_CREATE_CODEC}
		return
	}

	requiredSize := t.MaxEncodedLen(int(this.blockLength))
	magic := internal.GetMagicType(data)

	if internal.IsDataCompressed(magic) == true {
		this.ctx["dataType"] = internal.DT_BIN
	} else if internal.IsDataMultimedia(magic) == true {
		this.ctx["dataType"] = internal.DT_MULTIMEDIA
	} else if internal.IsDataExecutable(magic) == true {
		this.ctx["dataType"] = internal.DT_EXE
	}

	if len(this.iBuffer.Buf) < requiredSize {
		extraBuf := make([]byte, requiredSize-len(this.iBuffer.Buf))
		data = append(data, extraBuf...)
		this.iBuffer.Buf = data
	}

	if len(this.oBuffer.Buf) < requiredSize {
		buffer = make([]byte, requiredSize)
		this.oBuffer.Buf = buffer
	}

	// Forward transform (ignore error, encode skipFlags)
	_, postTransformLength, _ := t.Forward(data[0:this.blockLength], buffer)
	this.ctx["size"] = postTransformLength
	dataSize := uint(1)

	if postTransformLength >= 256 {
		dataSize = uint(internal.Log2NoCheck(uint32(postTransformLength))>>3) + 1

		if dataSize > 4 {
			res.err = &IOError{msg: "Invalid block data length", code: kanzi.ERR_WRITE_FILE}
			return
		}
	}

	// Record size of 'block size' - 1 in bytes
	mode |= byte(((dataSize - 1) & 0x03) << 5)

	if len(this.listeners) > 0 {
		// Notify after transform
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_TRANSFORM, int(this.currentBlockID),
			int64(postTransformLength), checksum, hashType, time.Now())
		notifyListeners(this.listeners, evt)
	}

	bufSize := max(postTransformLength, max(this.blockLength+(this.blockLength>>3), 256*1024))

	if len(data) < int(bufSize) {
		// Rare case where the transform expanded the input or the entropy
		// coder may expand the size
		data = make([]byte, bufSize)
	}

	// Create a bitstream local to the task
	bufStream := internal.NewBufferStream(data[0:0:cap(data)])
	obs, _ := bitstream.NewDefaultOutputBitStream(bufStream, 16384)
	skipFlags := t.SkipFlags()

	// Write block 'header' (mode + compressed length)
	if ((mode & _COPY_BLOCK_MASK) != 0) || (t.Len() <= 4) {
		mode |= byte(t.SkipFlags() >> 4)
		obs.WriteBits(uint64(mode), 8)
	} else {
		mode |= _TRANSFORMS_MASK
		obs.WriteBits(uint64(mode), 8)
		obs.WriteBits(uint64(t.SkipFlags()), 8)
	}

	obs.WriteBits(uint64(postTransformLength), 8*dataSize)

	// Write checksum
	if this.hasher32 != nil {
		obs.WriteBits(checksum, 32)
	} else if this.hasher64 != nil {
		obs.WriteBits(checksum, 64)
	}

	if len(this.listeners) > 0 {
		// Notify before entropy
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, int(this.currentBlockID),
			int64(postTransformLength), checksum, hashType, time.Now())
		notifyListeners(this.listeners, evt)
	}

	// Each block is encoded separately
	// Rebuild the entropy encoder to reset block statistics
	ee, err := entropy.NewEntropyEncoder(obs, this.ctx, this.blockEntropyType)

	if err != nil {
		res.err = &IOError{msg: err.Error(), code: kanzi.ERR_CREATE_CODEC}
		return
	}

	// Entropy encode block
	if _, err = ee.Write(buffer[0:postTransformLength]); err != nil {
		res.err = &IOError{msg: err.Error(), code: kanzi.ERR_PROCESS_BLOCK}
		return
	}

	// Dispose before displaying statistics. Dispose may write to the bitstream
	ee.Dispose()
	obs.Close()
	written := obs.Written()

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, int(this.currentBlockID),
			int64((written+7)>>3), checksum, hashType, time.Now())
		notifyListeners(this.listeners, evt)

		if v, hasKey := this.ctx["verbosity"]; hasKey {
			blockOffset := this.obs.Written()

			if v.(uint) > 4 {
				msg := fmt.Sprintf("{ \"type\":\"%s\", \"id\":%d, \"offset\":%d, \"skipFlags\":%.8b }",
					"BLOCK_INFO", int(this.currentBlockID), blockOffset, skipFlags)
				evt1 := kanzi.NewEventFromString(kanzi.EVT_BLOCK_INFO, int(this.currentBlockID), msg, time.Now())
				notifyListeners(this.listeners, evt1)
			}
		}
	}

	// Lock free synchronization
	for n := 0; ; n++ {
		taskID := atomic.LoadInt32(this.processedBlockID)

		if taskID == _CANCEL_TASKS_ID {
			return
		}

		if taskID == this.currentBlockID-1 {
			break
		}

		if n&0x1F == 0 {
			runtime.Gosched()
		}
	}

	// Emit block size in bits (max size pre-entropy is 1 GB = 1 << 30 bytes)
	lw := uint(3)

	if written >= 8 {
		lw = uint(internal.Log2NoCheck(uint32(written>>3)) + 4)
	}

	this.obs.WriteBits(uint64(lw-3), 5) // write length-3 (5 bits max)
	this.obs.WriteBits(written, lw)
	chkSize := uint(1 << 30)

	if written < 1<<30 {
		chkSize = uint(written)
	}

	// Emit data to shared bitstream
	for n := uint(0); written > 0; {
		this.obs.WriteArray(data[n:], chkSize)
		n += (chkSize + 7) >> 3
		written -= uint64(chkSize)
		chkSize = uint(1 << 30)

		if written < 1<<30 {
			chkSize = uint(written)
		}
	}
}

func notifyListeners(listeners []kanzi.Listener, evt *kanzi.Event) {
	defer func() {
		// nolint:staticcheck
		if r := recover(); r != nil {
			//lint:ignore SA9003
			// Ignore panics in block listeners
		}
	}()

	for _, bl := range listeners {
		bl.ProcessEvent(evt)
	}
}

type decodingTaskResult struct {
	err            *IOError
	data           []byte
	decoded        int
	blockID        int
	skipped        bool
	checksum       uint64
	completionTime time.Time
}

// Reader a Reader that reads compressed data
// from an InputBitStream.
type Reader struct {
	blockSize       int
	hasher32        *hash.XXHash32
	hasher64        *hash.XXHash64
	buffers         []blockBuffer
	entropyType     uint32
	transformType   uint64
	outputSize      int64
	ibs             kanzi.InputBitStream
	initialized     int32
	closed          int32
	blockID         int32
	jobs            int
	bufferThreshold int
	available       int64 // decoded not consumed bytes
	consumed        int   // decoded consumed bytes
	nbInputBlocks   int
	listeners       []kanzi.Listener
	ctx             map[string]any
	parentCtx       *map[string]any
	headless        bool
}

type decodingTask struct {
	iBuffer            *blockBuffer
	oBuffer            *blockBuffer
	hasher32           *hash.XXHash32
	hasher64           *hash.XXHash64
	blockLength        uint
	blockTransformType uint64
	blockEntropyType   uint32
	currentBlockID     int32
	processedBlockID   *int32
	wg                 *sync.WaitGroup
	listeners          []kanzi.Listener
	ibs                kanzi.InputBitStream
	ctx                map[string]any
}

// NewReader creates a new instance of Reader.
// The reader reads compressed data blocks from the provided is.
func NewReader(is io.ReadCloser, jobs uint) (*Reader, error) {
	ctx := make(map[string]any)
	ctx["jobs"] = jobs
	return NewReaderWithCtx(is, ctx)
}

// NewHeaderlessReader creates a new instance of Reader to decompress a headerless bitstream.
// The reader reads compressed data blocks from the provided is.
// Use 0 if the original file size is not known.
// The default value for the bitstream version is _BITSTREAM_FORMAT_VERSION.
// If the provided compression parameters do not match those used during compression,
// the decompression is likely to fail.
// checksum must be 0, 32 or 64
func NewHeaderlessReader(is io.ReadCloser, jobs uint, transform, entropy string, blockSize uint, checksum uint, originalSize int64, bsVersion uint) (*Reader, error) {
	ctx := make(map[string]any)
	ctx["jobs"] = jobs
	ctx["transform"] = transform
	ctx["entropy"] = entropy
	ctx["blockSize"] = blockSize
	ctx["checksum"] = checksum
	ctx["outputSize"] = originalSize
	ctx["bsVersion"] = bsVersion
	return NewReaderWithCtx(is, ctx)
}

// NewReaderWithCtx creates a new instance of Reader using a map of parameters.
// The reader reads compressed data blocks from the provided is
// using a default input bitstream.
func NewReaderWithCtx(is io.ReadCloser, ctx map[string]any) (*Reader, error) {
	var err error
	var ibs kanzi.InputBitStream

	if ibs, err = bitstream.NewDefaultInputBitStream(is, _STREAM_DEFAULT_BUFFER_SIZE); err != nil {
		errMsg := fmt.Sprintf("Cannot create input bit stream: %v", err)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_CREATE_BITSTREAM}
	}

	return createReaderWithCtx(ibs, ctx)
}

// NewReaderWithCtx2 creates a new instance of Reader.
// using a map of parameters and a custom input bitstream.
// The reader reads compressed data blocks from the provided input bitstream.
func NewReaderWithCtx2(ibs kanzi.InputBitStream, ctx map[string]any) (*Reader, error) {
	return createReaderWithCtx(ibs, ctx)
}

func createReaderWithCtx(ibs kanzi.InputBitStream, ctx map[string]any) (*Reader, error) {
	if ibs == nil {
		return nil, &IOError{msg: "Invalid null input bitstream parameter", code: kanzi.ERR_CREATE_DECOMPRESSOR}
	}

	if ctx == nil {
		return nil, &IOError{msg: "Invalid null context parameter", code: kanzi.ERR_CREATE_DECOMPRESSOR}
	}

	tasks := ctx["jobs"].(uint)

	if tasks == 0 || tasks > _MAX_CONCURRENCY {
		errMsg := fmt.Sprintf("The number of jobs must be in [1..%d], got %d", _MAX_CONCURRENCY, tasks)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_CREATE_DECOMPRESSOR}
	}

	this := &Reader{}
	this.ibs = ibs
	this.jobs = int(tasks)
	this.blockID = 0
	this.consumed = 0
	this.available = 0
	this.outputSize = 0
	this.nbInputBlocks = 0
	this.bufferThreshold = 0
	this.buffers = make([]blockBuffer, 2*this.jobs)

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: make([]byte, 0)}
	}

	this.listeners = make([]kanzi.Listener, 0)
	this.ctx = ctx
	this.parentCtx = &ctx
	this.blockSize = 0
	this.entropyType = entropy.NONE_TYPE
	this.transformType = transform.NONE_TYPE
	this.headless = false

	if hdl, hasKey := ctx["headerless"]; hasKey == true {
		this.headless = hdl.(bool)

		// Validate required values
		if err := this.validateHeaderless(); err != nil {
			return nil, err
		}
	}

	return this, nil
}

func (this *Reader) validateHeaderless() error {
	var err error

	if bsv, hasKey := this.ctx["bsVersion"]; hasKey {
		bsVersion := bsv.(uint)

		if bsVersion > _BITSTREAM_FORMAT_VERSION {
			errMsg := fmt.Sprintf("Invalid bitstream version, cannot read this version of the stream: %d", bsVersion)
			return &IOError{msg: errMsg, code: kanzi.ERR_INVALID_PARAM}
		}
	} else {
		this.ctx["bsVersion"] = _BITSTREAM_FORMAT_VERSION
	}

	if e, hasKey := this.ctx["entropy"]; hasKey {
		eName := e.(string)
		this.entropyType, err = entropy.GetType(eName)

		if err != nil {
			return &IOError{msg: err.Error(), code: kanzi.ERR_INVALID_PARAM}
		}
	} else {
		return &IOError{msg: "Missing entropy in headerless mode", code: kanzi.ERR_MISSING_PARAM}
	}

	if t, hasKey := this.ctx["transform"]; hasKey {
		tName := t.(string)
		this.transformType, err = transform.GetType(tName)

		if err != nil {
			return &IOError{msg: err.Error(), code: kanzi.ERR_INVALID_PARAM}
		}
	} else {
		return &IOError{msg: "Missing transform in headerless mode", code: kanzi.ERR_MISSING_PARAM}
	}

	if b, hasKey := this.ctx["blockSize"]; hasKey {
		blk := b.(uint)

		if blk < _MIN_BITSTREAM_BLOCK_SIZE || blk > _MAX_BITSTREAM_BLOCK_SIZE {
			errMsg := fmt.Sprintf("Invalid block size: %d", blk)
			return &IOError{msg: errMsg, code: kanzi.ERR_INVALID_PARAM}
		}

		this.blockSize = int(blk)
		this.bufferThreshold = this.blockSize
	} else {
		return &IOError{msg: "Missing block size in headerless mode", code: kanzi.ERR_MISSING_PARAM}
	}

	if c, hasKey := this.ctx["checksum"]; hasKey {
		if c.(uint) != 0 {
			if c == 32 {
				this.hasher32, err = hash.NewXXHash32(_BITSTREAM_TYPE)
			} else if c == 64 {
				this.hasher64, err = hash.NewXXHash64(_BITSTREAM_TYPE)
			} else {
				err = &IOError{msg: "The lock checksum size must be 32 or 64 bits", code: kanzi.ERR_INVALID_PARAM}
			}

			if err != nil {
				return err
			}
		}
	}

	if s, hasKey := this.ctx["outputSize"]; hasKey {
		this.outputSize = s.(int64)

		if this.outputSize < 0 || this.outputSize >= (1<<48) {
			this.outputSize = 0 // 'not provided'
		}

		nbBlocks := int((this.outputSize + int64(this.blockSize-1)) / int64(this.blockSize))
		this.nbInputBlocks = min(nbBlocks, _MAX_CONCURRENCY-1)
	}

	return nil
}

// AddListener adds an event listener to this reader.
// Returns true if the listener has been added.
func (this *Reader) AddListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

// RemoveListener removes an event listener from this reader
// Returns true if the listener has been removed.
func (this *Reader) RemoveListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	for i, e := range this.listeners {
		if e == bl {
			this.listeners = append(this.listeners[:i], this.listeners[i+1:]...)
			return true
		}
	}

	return false
}

// Use a named return value to update the error in the defer function (after return is executed)
func (this *Reader) readHeader() (err error) {
	if this.headless == true || atomic.SwapInt32(&this.initialized, 1) != 0 {
		return nil
	}

	defer func() {
		if r := recover(); r != nil {

			switch v := r.(type) {
			case error:
				err = &IOError{msg: v.Error(), code: kanzi.ERR_PROCESS_BLOCK}
			default:
				err = &IOError{msg: fmt.Sprint(v), code: kanzi.ERR_PROCESS_BLOCK}
			}
		}
	}()

	// Read stream type
	fileType := this.ibs.ReadBits(32)

	// Sanity check
	if fileType != _BITSTREAM_TYPE {
		return &IOError{msg: "Invalid stream type", code: kanzi.ERR_INVALID_FILE}
	}

	bsVersion := uint(this.ibs.ReadBits(4))

	// Sanity check
	if bsVersion > _BITSTREAM_FORMAT_VERSION {
		errMsg := fmt.Sprintf("Invalid bitstream, cannot read this version of the stream: %d", bsVersion)
		return &IOError{msg: errMsg, code: kanzi.ERR_STREAM_VERSION}
	}

	this.ctx["bsVersion"] = bsVersion
	ckSize := uint64(0)

	// Read block checksum
	if bsVersion >= 6 {
		ckSize = this.ibs.ReadBits(2)

		if ckSize == 1 {
			this.hasher32, err = hash.NewXXHash32(_BITSTREAM_TYPE)
		} else if ckSize == 2 {
			this.hasher64, err = hash.NewXXHash64(_BITSTREAM_TYPE)
		} else if ckSize == 3 {
			errMsg := fmt.Sprintf("Invalid bitstream, incorrect checksum size: %d", ckSize)
			return &IOError{msg: errMsg, code: kanzi.ERR_INVALID_CODEC}
		}
	} else if this.ibs.ReadBit() == 1 {
		this.hasher32, err = hash.NewXXHash32(_BITSTREAM_TYPE)
	}

	if err != nil {
		return err
	}

	// Read entropy codec
	this.entropyType = uint32(this.ibs.ReadBits(5))
	var eType string

	if eType, err = entropy.GetName(this.entropyType); err != nil {
		errMsg := fmt.Sprintf("Invalid bitstream, incorrect entropy type: %d", this.entropyType)
		return &IOError{msg: errMsg, code: kanzi.ERR_INVALID_CODEC}
	}

	this.ctx["entropy"] = eType

	// Read transforms: 8*6 bits
	this.transformType = this.ibs.ReadBits(48)
	var tType string

	if tType, err = transform.GetName(this.transformType); err != nil {
		errMsg := fmt.Sprintf("Invalid bitstream, incorrect transform type: %d", this.transformType)
		return &IOError{msg: errMsg, code: kanzi.ERR_INVALID_CODEC}
	}

	this.ctx["transform"] = tType

	// Read block size
	this.blockSize = int(this.ibs.ReadBits(28)) << 4

	if this.blockSize < _MIN_BITSTREAM_BLOCK_SIZE || this.blockSize > _MAX_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("Invalid bitstream, incorrect block size: %d", this.blockSize)
		return &IOError{msg: errMsg, code: kanzi.ERR_BLOCK_SIZE}
	}

	this.ctx["blockSize"] = uint(this.blockSize)
	this.bufferThreshold = this.blockSize
	szMask := uint(0)

	if bsVersion >= 5 {
		// Read original size
		// 0 -> not provided, <2^16 -> 1, <2^32 -> 2, <2^48 -> 3
		szMask = uint(this.ibs.ReadBits(2))

		if szMask != 0 {
			this.outputSize = int64(this.ibs.ReadBits(16 * szMask))

			if this.parentCtx != nil {
				(*this.parentCtx)["outputSize"] = this.outputSize
			}

			nbBlocks := int((this.outputSize + int64(this.blockSize-1)) / int64(this.blockSize))
			this.nbInputBlocks = min(nbBlocks, _MAX_CONCURRENCY-1)
		}

		crcSize := uint(16)
		seed := uint32(bsVersion)

		if bsVersion >= 6 {
			// Padding
			this.ibs.ReadBits(15)
			crcSize = uint(24)
			seed = uint32(0x01030507 * bsVersion)
		}

		// Read and verify checksum
		cksum1 := uint32(this.ibs.ReadBits(crcSize))
		var cksum2 uint32
		HASH := uint32(0x1E35A7BD)
		cksum2 = HASH * seed

		if bsVersion >= 6 {
			cksum2 ^= (HASH * uint32(^ckSize))
		}

		cksum2 ^= (HASH * uint32(^this.entropyType))
		cksum2 ^= (HASH * uint32((^this.transformType)>>32))
		cksum2 ^= (HASH * uint32(^this.transformType))
		cksum2 ^= (HASH * uint32(^this.blockSize))

		if szMask > 0 {
			cksum2 ^= (HASH * uint32((^this.outputSize)>>32))
			cksum2 ^= (HASH * uint32(^this.outputSize))
		}

		cksum2 = (cksum2 >> 23) ^ (cksum2 >> 3)

		if cksum1 != (cksum2 & ((1 << crcSize) - 1)) {
			return &IOError{msg: "Invalid bitstream: checksum mismatch", code: kanzi.ERR_CRC_CHECK}
		}
	} else if bsVersion >= 3 {
		// Read number of blocks in input. 0 means 'unknown' and 63 means 63 or more.
		this.nbInputBlocks = int(this.ibs.ReadBits(6))

		// Read and verify checksum
		cksum1 := uint32(this.ibs.ReadBits(4))
		var cksum2 uint32
		HASH := uint32(0x1E35A7BD)
		cksum2 = HASH * uint32(bsVersion)
		cksum2 ^= (HASH * uint32(this.entropyType))
		cksum2 ^= (HASH * uint32(this.transformType>>32))
		cksum2 ^= (HASH * uint32(this.transformType))
		cksum2 ^= (HASH * uint32(this.blockSize))
		cksum2 ^= (HASH * uint32(this.nbInputBlocks))
		cksum2 = (cksum2 >> 23) ^ (cksum2 >> 3)

		if cksum1 != (cksum2 & 0x0F) {
			return &IOError{msg: "Invalid bitstream: corrupted header", code: kanzi.ERR_INVALID_FILE}
		}
	} else {
		// Header prior to version 3
		this.nbInputBlocks = int(this.ibs.ReadBits(6))
		this.ibs.ReadBits(4) // reserved
	}

	if len(this.listeners) > 0 {
		var sb strings.Builder
		ckSize := 0

		if this.hasher32 != nil {
			ckSize = 32
		} else if this.hasher64 != nil {
			ckSize = 64
		}

		sb.WriteString(this.ctx["inputName"].(string))
		sb.WriteString(",")
		sb.WriteString(fmt.Sprintf("%d,", bsVersion))
		sb.WriteString(fmt.Sprintf("%d,", ckSize))
		sb.WriteString(fmt.Sprintf("%d,", this.blockSize))
		w1, _ := entropy.GetName(this.entropyType)

		if w1 == "NONE" {
			w1 = ""
		}

		sb.WriteString(w1)
		sb.WriteString(",")
		w2, _ := transform.GetName(this.transformType)

		if w2 == "NONE" {
			w2 = ""
		}

		sb.WriteString(w2)
		sb.WriteString(",")
		fileSize := this.ctx["fileSize"].(int64)
		sb.WriteString(fmt.Sprintf("%d,", fileSize))

		if szMask != 0 {
			sb.WriteString(fmt.Sprintf("%d", this.outputSize))
			sb.WriteString(",")
		}

		evt := kanzi.NewEventFromString(kanzi.EVT_AFTER_HEADER_DECODING, 0, sb.String(), time.Now())
		notifyListeners(this.listeners, evt)
	}

	return nil
}

// Close reads the buffered data from the reader and releases resources.
// Close makes the bitstream unavailable for further reads. Idempotent
func (this *Reader) Close() error {
	if atomic.SwapInt32(&this.closed, 1) == 1 {
		return nil
	}

	if err := this.ibs.Close(); err != nil {
		return err
	}

	this.available = 0

	// Release resources
	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: make([]byte, 0)}
	}

	return nil
}

// Read reads up to len(block) bytes and copies them into block.
// Returns the number of bytes read (0 <= n <= len(block)) and any error encountered.
// io.EOF is returned when the end of stream is reached.
func (this *Reader) Read(block []byte) (int, error) {
	if atomic.LoadInt32(&this.closed) == 1 {
		return 0, &IOError{msg: "Stream closed", code: kanzi.ERR_READ_FILE}
	}

	if err := this.readHeader(); err != nil {
		return 0, err
	}

	off := 0
	remaining := len(block)

	for remaining > 0 {
		bufOff := this.consumed % this.blockSize
		lenChunk := min(remaining, int(min(this.available, int64(this.bufferThreshold-bufOff))))

		if lenChunk > 0 {
			// Process a chunk of in-buffer data. No access to bitstream required
			bufID := this.consumed / this.blockSize
			copy(block[off:], this.buffers[bufID].Buf[bufOff:bufOff+lenChunk])
			off += lenChunk
			remaining -= lenChunk
			this.available -= int64(lenChunk)
			this.consumed += lenChunk

			if this.available > 0 && bufOff+lenChunk >= this.bufferThreshold {
				// Move to next buffer
				continue
			}

			if remaining == 0 {
				break
			}
		}

		// Buffer empty, time to decode
		if this.available == 0 {
			var err error

			if this.available, err = this.processBlock(); err != nil {
				return len(block) - remaining, err
			}

			if this.available == 0 {
				// Reached end of stream
				if len(block) == remaining {
					// EOF and we did not read any bytes in this call
					return 0, io.EOF
				}

				break
			}
		}
	}

	return len(block) - remaining, nil
}

func (this *Reader) processBlock() (int64, error) {
	if atomic.LoadInt32(&this.blockID) == _CANCEL_TASKS_ID {
		return 0, nil
	}

	blkSize := this.blockSize

	// Add a padding area to manage any block temporarily expanded
	if _EXTRA_BUFFER_SIZE >= (blkSize >> 4) {
		blkSize += _EXTRA_BUFFER_SIZE
	} else {
		blkSize += (blkSize >> 4)
	}

	// Protect against future concurrent modification of the list of block listeners
	listeners := make([]kanzi.Listener, len(this.listeners))
	copy(listeners, this.listeners)
	decoded := int64(0)

	nbTasks := this.jobs
	var jobsPerTask []uint

	// Assign optimal number of tasks and jobs per task (if the number of blocks is known)
	if nbTasks > 1 {
		// Limit the number of jobs if there are fewer blocks that this.jobs
		// It allows more jobs per task and reduces memory usage.
		if this.nbInputBlocks > 0 {
			nbTasks = min(nbTasks, this.nbInputBlocks)
		}

		jobsPerTask, _ = internal.ComputeJobsPerTask(make([]uint, nbTasks), uint(this.jobs), uint(nbTasks))
	} else {
		jobsPerTask = []uint{uint(this.jobs)}
	}

	bufSize := this.blockSize + _EXTRA_BUFFER_SIZE

	if bufSize < this.blockSize+(this.blockSize>>4) {
		bufSize = this.blockSize + (this.blockSize >> 4)
	}

	for {
		results := make([]decodingTaskResult, nbTasks)
		wg := sync.WaitGroup{}
		firstID := this.blockID

		// Invoke as many go routines as required
		for taskID := 0; taskID < nbTasks; taskID++ {
			if len(this.buffers[taskID].Buf) < int(bufSize) {
				this.buffers[taskID].Buf = make([]byte, bufSize)
			}

			copyCtx := make(map[string]any)

			for k, v := range this.ctx {
				copyCtx[k] = v
			}

			copyCtx["jobs"] = jobsPerTask[taskID]
			results[taskID] = decodingTaskResult{}
			wg.Add(1)

			task := decodingTask{
				iBuffer:            &this.buffers[taskID],
				oBuffer:            &this.buffers[this.jobs+taskID],
				hasher32:           this.hasher32,
				hasher64:           this.hasher64,
				blockLength:        uint(blkSize),
				blockTransformType: this.transformType,
				blockEntropyType:   this.entropyType,
				currentBlockID:     firstID + int32(taskID) + 1,
				processedBlockID:   &this.blockID,
				wg:                 &wg,
				listeners:          listeners,
				ibs:                this.ibs,
				ctx:                copyCtx}

			// Invoke the tasks concurrently
			go task.decode(&results[taskID])
		}

		// Wait for completion of all tasks
		wg.Wait()

		// Process results
		n, skipped := 0, 0

		for _, r := range results {
			if r.skipped == true {
				skipped++
				continue
			}

			if r.decoded > this.blockSize {
				errMsg := fmt.Sprintf("Block %d incorrectly decompressed", r.blockID)
				return decoded, &IOError{msg: errMsg, code: kanzi.ERR_PROCESS_BLOCK}
			}

			decoded += int64(r.decoded)

			if r.err != nil {
				return decoded, r.err
			}

			copy(this.buffers[n].Buf, r.data[0:r.decoded])
			n++
			hashType := kanzi.EVT_HASH_NONE

			if this.hasher32 != nil {
				hashType = kanzi.EVT_HASH_32BITS
			} else if this.hasher64 != nil {
				hashType = kanzi.EVT_HASH_64BITS
			}

			if len(listeners) > 0 {
				// Notify after transform ... in block order
				evt := kanzi.NewEvent(kanzi.EVT_AFTER_TRANSFORM, int(r.blockID),
					int64(r.decoded), r.checksum, hashType, r.completionTime)
				notifyListeners(listeners, evt)
			}
		}

		// Unless all blocks were skipped, exit the loop (usual case)
		if skipped != nbTasks {
			break
		}
	}

	this.consumed = 0
	return decoded, nil
}

// GetRead returns the number of bytes read so far
func (this *Reader) GetRead() uint64 {
	return (this.ibs.Read() + 7) >> 3
}

// Decode mode + transformed entropy coded data
// mode | 0b10000000 => copy block
// mode | 0b0yy00000 => size(size(block))-1
// mode | 0b000y0000 => 1 if more than 4 transforms
//
// case 4 transforms or less
// mode | 0b0000yyyy => transform sequence skip flags (1 means skip)
//
// case more than 4 transforms
// mode | 0b00000000
//
// then 0byyyyyyyy => transform sequence skip flags (1 means skip)
func (this *decodingTask) decode(res *decodingTaskResult) {
	data := this.iBuffer.Buf
	buffer := this.oBuffer.Buf
	decoded := 0
	checksum1 := uint64(0)
	skipped := false

	defer func() {
		res.data = this.iBuffer.Buf
		res.decoded = decoded
		res.blockID = int(this.currentBlockID)
		res.completionTime = time.Now()
		res.checksum = checksum1
		res.skipped = skipped

		if r := recover(); r != nil {
			err, ok := r.(error)

			if ok {
				res.err = &IOError{msg: err.Error(), code: kanzi.ERR_PROCESS_BLOCK}
			} else {
				res.err = &IOError{msg: "Unknown error", code: kanzi.ERR_PROCESS_BLOCK}
			}
		}

		// Unblock other tasks
		if res.err != nil || (res.decoded == 0 && res.skipped == false) {
			atomic.StoreInt32(this.processedBlockID, _CANCEL_TASKS_ID)
		} else if atomic.LoadInt32(this.processedBlockID) == this.currentBlockID-1 {
			atomic.StoreInt32(this.processedBlockID, this.currentBlockID)
		}

		this.wg.Done()
	}()

	// Lock free synchronization
	for n := 0; ; n++ {
		taskID := atomic.LoadInt32(this.processedBlockID)

		if taskID == _CANCEL_TASKS_ID {
			return
		}

		if taskID == this.currentBlockID-1 {
			break
		}

		if n&0x1F == 0 {
			runtime.Gosched()
		}
	}

	// Read shared bitstream sequentially
	blockOffset := this.ibs.Read()
	lr := uint(this.ibs.ReadBits(5)) + 3
	read := this.ibs.ReadBits(lr)

	if read == 0 {
		return
	}

	if read > uint64(1)<<34 {
		res.err = &IOError{msg: "Invalid block size", code: kanzi.ERR_BLOCK_SIZE}
		return
	}

	r := int((read + 7) >> 3)
	maxL := r

	if int(this.blockLength) > r {
		maxL = int(this.blockLength)
	}

	if len(data) < maxL {
		data = make([]byte, maxL)
		this.iBuffer.Buf = data
	}

	// Read data from shared bitstream
	for n := uint(0); read > 0; {
		chkSize := uint(1 << 30)

		if read < 1<<30 {
			chkSize = uint(read)
		}

		this.ibs.ReadArray(data[n:], chkSize)
		n += ((chkSize + 7) >> 3)
		read -= uint64(chkSize)
	}

	// After completion of the bitstream reading, increment the block id.
	// It unblocks the task processing the next block (if any)
	atomic.StoreInt32(this.processedBlockID, this.currentBlockID)

	// Check if the block must be skipped
	if v, hasKey := this.ctx["from"]; hasKey {
		if int(this.currentBlockID) < v.(int) {
			skipped = true
			return
		}
	}

	if v, hasKey := this.ctx["to"]; hasKey {
		if int(this.currentBlockID) >= v.(int) {
			skipped = true
			return
		}
	}

	// All the code below is concurrent
	// Create a bitstream local to the task
	bufStream := internal.NewBufferStream(data[0:r])
	ibs, _ := bitstream.NewDefaultInputBitStream(bufStream, 16384)

	mode := byte(ibs.ReadBits(8))
	skipFlags := byte(0)

	if mode&_COPY_BLOCK_MASK != 0 {
		this.blockTransformType = transform.NONE_TYPE
		this.blockEntropyType = entropy.NONE_TYPE
	} else {
		if mode&_TRANSFORMS_MASK != 0 {
			skipFlags = byte(ibs.ReadBits(8))
		} else {
			skipFlags = (mode << 4) | 0x0F
		}
	}

	dataSize := 1 + uint((mode>>5)&0x03)
	length := dataSize << 3
	mask := uint64(1<<length) - 1
	preTransformLength := uint(ibs.ReadBits(length) & mask)
	maxTransformLength := min(max(this.blockLength+this.blockLength/2, 2048), _MAX_BITSTREAM_BLOCK_SIZE)

	if preTransformLength == 0 || preTransformLength > maxTransformLength {
		// Error => cancel concurrent decoding tasks
		errMsg := fmt.Sprintf("Invalid compressed block size: %d", preTransformLength)
		res.err = &IOError{msg: errMsg, code: kanzi.ERR_BLOCK_SIZE}
		return
	}

	hashType := kanzi.EVT_HASH_NONE

	// Extract checksum from bit stream (if any)
	if this.hasher32 != nil {
		checksum1 = ibs.ReadBits(32)
		hashType = kanzi.EVT_HASH_32BITS
	} else if this.hasher64 != nil {
		checksum1 = ibs.ReadBits(64)
		hashType = kanzi.EVT_HASH_64BITS
	}

	if len(this.listeners) > 0 {
		if v, hasKey := this.ctx["verbosity"]; hasKey {
			if v.(uint) > 4 {
				msg := fmt.Sprintf("{ \"type\":\"%s\", \"id\":%d, \"offset\":%d, \"skipFlags\":%.8b }",
					"BLOCK_INFO", int(this.currentBlockID), blockOffset, skipFlags)
				evt1 := kanzi.NewEventFromString(kanzi.EVT_BLOCK_INFO, int(this.currentBlockID), msg, time.Now())
				notifyListeners(this.listeners, evt1)
			}
		}

		// Notify before entropy
		evt2 := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, int(this.currentBlockID),
			int64(r), checksum1, hashType, time.Now())
		notifyListeners(this.listeners, evt2)
	}

	bufferSize := max(this.blockLength, preTransformLength+_EXTRA_BUFFER_SIZE)

	if len(buffer) < int(bufferSize) {
		buffer = make([]byte, int(bufferSize))
		this.oBuffer.Buf = buffer
	}

	this.ctx["size"] = preTransformLength

	// Each block is decoded separately
	// Rebuild the entropy decoder to reset block statistics
	ed, err := entropy.NewEntropyDecoder(ibs, this.ctx, this.blockEntropyType)

	if err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = &IOError{msg: err.Error(), code: kanzi.ERR_INVALID_CODEC}
		return
	}

	// Block entropy decode
	if _, err = ed.Read(buffer[0:preTransformLength]); err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = &IOError{msg: err.Error(), code: kanzi.ERR_PROCESS_BLOCK}
		return
	}

	ed.Dispose()
	ibs.Close()

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt1 := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, int(this.currentBlockID),
			int64(preTransformLength), checksum1, hashType, time.Now())
		notifyListeners(this.listeners, evt1)

		// Notify before transform
		evt2 := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, int(this.currentBlockID),
			int64(preTransformLength), checksum1, hashType, time.Now())
		notifyListeners(this.listeners, evt2)
	}

	this.ctx["size"] = preTransformLength
	transform, err := transform.New(&this.ctx, this.blockTransformType)

	if err != nil {
		// Error => return
		res.err = &IOError{msg: err.Error(), code: kanzi.ERR_INVALID_CODEC}
		return
	}

	transform.SetSkipFlags(skipFlags)
	var oIdx uint

	// Inverse transform
	if _, oIdx, err = transform.Inverse(buffer[0:preTransformLength], data); err != nil {
		// Error => return
		res.err = &IOError{msg: err.Error(), code: kanzi.ERR_PROCESS_BLOCK}
		return
	}

	decoded = int(oIdx)

	// Verify checksum
	if this.hasher32 != nil {
		checksum2 := this.hasher32.Hash(data[0:decoded])

		if checksum2 != uint32(checksum1) {
			errMsg := fmt.Sprintf("Corrupted bitstream: expected checksum %x, found %x", checksum1, checksum2)
			res.err = &IOError{msg: errMsg, code: kanzi.ERR_CRC_CHECK}
			return
		}
	} else if this.hasher64 != nil {
		checksum2 := this.hasher64.Hash(data[0:decoded])

		if checksum2 != checksum1 {
			errMsg := fmt.Sprintf("Corrupted bitstream: expected checksum %x, found %x", checksum1, checksum2)
			res.err = &IOError{msg: errMsg, code: kanzi.ERR_CRC_CHECK}
			return
		}
	}
}
