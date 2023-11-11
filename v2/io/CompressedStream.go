/*
Copyright 2011-2024 Frederic Langlet
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
	"sync"
	"sync/atomic"
	"time"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/bitstream"
	"github.com/flanglet/kanzi-go/v2/entropy"
	internal "github.com/flanglet/kanzi-go/v2/internal"
	"github.com/flanglet/kanzi-go/v2/transform"
	"github.com/flanglet/kanzi-go/v2/util"
	"github.com/flanglet/kanzi-go/v2/util/hash"
)

// Write to/read from bitstream using a 2 step process:
// Encoding:
// - step 1: a ByteFunction is used to reduce the size of the input data (bytes input & output)
// - step 2: an EntropyEncoder is used to entropy code the results of step 1 (bytes input, bits output)
// Decoding is the exact reverse process.

const (
	_BITSTREAM_TYPE             = 0x4B414E5A // "KANZ"
	_BITSTREAM_FORMAT_VERSION   = 4
	_STREAM_DEFAULT_BUFFER_SIZE = 256 * 1024
	_EXTRA_BUFFER_SIZE          = 512
	_COPY_BLOCK_MASK            = 0x80
	_TRANSFORMS_MASK            = 0x10
	_MIN_BITSTREAM_BLOCK_SIZE   = 1024
	_MAX_BITSTREAM_BLOCK_SIZE   = 1024 * 1024 * 1024
	_SMALL_BLOCK_SIZE           = 15
	_MAX_CONCURRENCY            = 64
	_CANCEL_TASKS_ID            = -1
	_UNKNOWN_NB_BLOCKS          = 65536
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

// CompressedOutputStream a Writer that writes compressed data
// to an OutputBitStream.
type CompressedOutputStream struct {
	blockSize     int
	hasher        *hash.XXHash32
	buffers       []blockBuffer
	entropyType   uint32
	transformType uint64
	obs           kanzi.OutputBitStream
	initialized   int32
	closed        int32
	blockID       int32
	jobs          int
	nbInputBlocks int
	available     int
	listeners     []kanzi.Listener
	ctx           map[string]interface{}
}

type encodingTask struct {
	iBuffer            *blockBuffer
	oBuffer            *blockBuffer
	hasher             *hash.XXHash32
	blockLength        uint
	blockTransformType uint64
	blockEntropyType   uint32
	currentBlockID     int32
	processedBlockID   *int32
	wg                 *sync.WaitGroup
	listeners          []kanzi.Listener
	obs                kanzi.OutputBitStream
	ctx                map[string]interface{}
}

type encodingTaskResult struct {
	err *IOError
}

// NewCompressedOutputStream creates a new instance of CompressedOutputStream
func NewCompressedOutputStream(os io.WriteCloser, codec, transform string, blockSize, jobs uint, checksum bool) (*CompressedOutputStream, error) {
	ctx := make(map[string]interface{})
	ctx["codec"] = codec
	ctx["transform"] = transform
	ctx["blockSize"] = blockSize
	ctx["jobs"] = jobs
	ctx["checksum"] = checksum
	return NewCompressedOutputStreamWithCtx(os, ctx)
}

// NewCompressedOutputStreamWithCtx creates a new instance of CompressedOutputStream using a
// map of parameters and a writer
func NewCompressedOutputStreamWithCtx(os io.WriteCloser, ctx map[string]interface{}) (*CompressedOutputStream, error) {
	var err error
	var obs kanzi.OutputBitStream

	if obs, err = bitstream.NewDefaultOutputBitStream(os, _STREAM_DEFAULT_BUFFER_SIZE); err != nil {
		errMsg := fmt.Sprintf("Cannot create output bit stream: %v", err)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_CREATE_BITSTREAM}
	}

	return createCompressedOutputStreamWithCtx(obs, ctx)
}

// NewCompressedOutputStreamWithCtx2 creates a new instance of CompressedOutputStream using a
// map of parameters and a custom output bitstream
func NewCompressedOutputStreamWithCtx2(obs kanzi.OutputBitStream, ctx map[string]interface{}) (*CompressedOutputStream, error) {
	return createCompressedOutputStreamWithCtx(obs, ctx)
}

func createCompressedOutputStreamWithCtx(obs kanzi.OutputBitStream, ctx map[string]interface{}) (*CompressedOutputStream, error) {
	if obs == nil {
		return nil, &IOError{msg: "Invalid null output bitstream parameter", code: kanzi.ERR_CREATE_STREAM}
	}

	if ctx == nil {
		return nil, &IOError{msg: "Invalid null context parameter", code: kanzi.ERR_CREATE_STREAM}
	}

	entropyCodec := ctx["codec"].(string)
	t := ctx["transform"].(string)
	tasks := ctx["jobs"].(uint)

	if tasks == 0 || tasks > _MAX_CONCURRENCY {
		errMsg := fmt.Sprintf("The number of jobs must be in [1..%d], got %d", _MAX_CONCURRENCY, tasks)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_CREATE_STREAM}
	}

	bSize := ctx["blockSize"].(uint)

	if bSize > _MAX_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at most %d MB", _MAX_BITSTREAM_BLOCK_SIZE>>20)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_CREATE_STREAM}
	}

	if bSize < _MIN_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at least %d", _MIN_BITSTREAM_BLOCK_SIZE)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_CREATE_STREAM}
	}

	if int(bSize)&-16 != int(bSize) {
		return nil, &IOError{msg: "The block size must be a multiple of 16", code: kanzi.ERR_CREATE_STREAM}
	}

	ctx["bsVersion"] = uint(_BITSTREAM_FORMAT_VERSION)
	this := &CompressedOutputStream{}
	this.obs = obs

	// Check entropy type validity (panic on error)
	var eType uint32
	var err error

	if eType, err = entropy.GetType(entropyCodec); err != nil {
		return nil, &IOError{msg: err.Error(), code: kanzi.ERR_CREATE_STREAM}
	}

	this.entropyType = eType

	// Check transform type validity
	this.transformType, err = transform.GetType(t)

	if err != nil {
		return nil, &IOError{msg: err.Error(), code: kanzi.ERR_CREATE_STREAM}
	}

	this.blockSize = int(bSize)
	this.available = 0
	nbBlocks := _UNKNOWN_NB_BLOCKS

	// If input size has been provided, calculate the number of blocks
	// in the input data else use 0. A value of 63 means '63 or more blocks'.
	// This value is written to the bitstream header to let the decoder make
	// better decisions about memory usage and job allocation in concurrent
	// decompression scenario.
	if val, containsKey := ctx["fileSize"]; containsKey {
		fileSize := val.(int64)
		nbBlocks = int((fileSize + int64(bSize-1)) / int64(bSize))
	}

	if nbBlocks >= _MAX_CONCURRENCY {
		this.nbInputBlocks = _MAX_CONCURRENCY - 1
	} else if nbBlocks == 0 {
		this.nbInputBlocks = 1
	} else {
		this.nbInputBlocks = nbBlocks
	}

	if checksum := ctx["checksum"].(bool); checksum == true {
		var err error
		this.hasher, err = hash.NewXXHash32(_BITSTREAM_TYPE)

		if err != nil {
			return nil, err
		}
	}

	this.jobs = int(tasks)
	this.buffers = make([]blockBuffer, 2*this.jobs)

	// Allocate first buffer and add padding for incompressible blocks
	bufSize := this.blockSize + this.blockSize>>6

	if bufSize < 65536 {
		bufSize = 65536
	}

	this.buffers[0] = blockBuffer{Buf: make([]byte, bufSize)}
	this.buffers[this.jobs] = blockBuffer{Buf: make([]byte, 0)}

	for i := 1; i < this.jobs; i++ {
		this.buffers[i] = blockBuffer{Buf: make([]byte, 0)}
		this.buffers[i+this.jobs] = blockBuffer{Buf: make([]byte, 0)}
	}

	this.blockID = 0
	this.listeners = make([]kanzi.Listener, 0)
	this.ctx = ctx
	return this, nil
}

// AddListener adds an event listener to this output stream.
// Returns true if the listener has been added.
func (this *CompressedOutputStream) AddListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

// RemoveListener removes an event listener from this output stream.
// Returns true if the listener has been removed.
func (this *CompressedOutputStream) RemoveListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	for i, e := range this.listeners {
		if e == bl {
			this.listeners = append(this.listeners[:i-1], this.listeners[i+1:]...)
			return true
		}
	}

	return false
}

func (this *CompressedOutputStream) writeHeader() *IOError {
	cksum := uint32(0)

	if this.hasher != nil {
		cksum = 1
	}

	if this.obs.WriteBits(_BITSTREAM_TYPE, 32) != 32 {
		return &IOError{msg: "Cannot write bitstream type to header", code: kanzi.ERR_WRITE_FILE}
	}

	if this.obs.WriteBits(_BITSTREAM_FORMAT_VERSION, 4) != 4 {
		return &IOError{msg: "Cannot write bitstream version to header", code: kanzi.ERR_WRITE_FILE}
	}

	if this.obs.WriteBits(uint64(cksum), 1) != 1 {
		return &IOError{msg: "Cannot write checksum to header", code: kanzi.ERR_WRITE_FILE}
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

	if this.obs.WriteBits(uint64(this.nbInputBlocks&(_MAX_CONCURRENCY-1)), 6) != 6 {
		return &IOError{msg: "Cannot write number of blocks to header", code: kanzi.ERR_WRITE_FILE}
	}

	HASH := uint32(0x1E35A7BD)
	cksum = HASH * _BITSTREAM_FORMAT_VERSION
	cksum ^= (HASH * uint32(this.entropyType))
	cksum ^= (HASH * uint32(this.transformType>>32))
	cksum ^= (HASH * uint32(this.transformType))
	cksum ^= (HASH * uint32(this.blockSize))
	cksum ^= (HASH * uint32(this.nbInputBlocks))
	cksum = (cksum >> 23) ^ (cksum >> 3)

	if this.obs.WriteBits(uint64(cksum), 4) != 4 {
		return &IOError{msg: "Cannot write checksum to header", code: kanzi.ERR_WRITE_FILE}
	}

	return nil
}

// Write writes len(block) bytes from block to the underlying data stream.
// It returns the number of bytes written from block (0 <= n <= len(block)) and
// any error encountered that caused the write to stop early.
func (this *CompressedOutputStream) Write(block []byte) (int, error) {
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
						bufSize := this.blockSize + this.blockSize>>6

						if bufSize < 65536 {
							bufSize = 65536
						}

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

// Close writes the buffered data to the output stream then writes
// a final empty block and releases resources.
// Close makes the bitstream unavailable for further writes. Idempotent.
func (this *CompressedOutputStream) Close() error {
	if atomic.SwapInt32(&this.closed, 1) == 1 {
		return nil
	}

	if err := this.processBlock(); err != nil {
		return err
	}

	// Write end block of size 0
	this.obs.WriteBits(0, 5) // write length-3 (5 bits max)
	this.obs.WriteBits(0, 3)

	if _, err := this.obs.Close(); err != nil {
		return err
	}

	// Release resources
	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: make([]byte, 0)}
	}

	return nil
}

func (this *CompressedOutputStream) processBlock() error {
	if atomic.SwapInt32(&this.initialized, 1) == 0 {
		if err := this.writeHeader(); err != nil {
			return err
		}
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

	// Assign optimal number of tasks and jobs per task
	if nbTasks > 1 {
		// Limit the number of jobs if there are fewer blocks that this.jobs
		// It allows more jobs per task and reduces memory usage.
		if nbTasks > this.nbInputBlocks {
			nbTasks = this.nbInputBlocks
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

		copyCtx := make(map[string]interface{})

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
			hasher:             this.hasher,
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
func (this *CompressedOutputStream) GetWritten() uint64 {
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
	checksum := uint32(0)

	defer func() {
		if r := recover(); r != nil {
			res.err = &IOError{msg: r.(error).Error(), code: kanzi.ERR_PROCESS_BLOCK}
		}

		// Unblock other tasks
		if res.err != nil {
			atomic.StoreInt32(this.processedBlockID, _CANCEL_TASKS_ID)
		} else if atomic.LoadInt32(this.processedBlockID) == this.currentBlockID-1 {
			atomic.StoreInt32(this.processedBlockID, this.currentBlockID)
		}

		this.wg.Done()
	}()

	// Compute block checksum
	if this.hasher != nil {
		checksum = this.hasher.Hash(data[0:this.blockLength])
	}

	if len(this.listeners) > 0 {
		// Notify before transform
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, int(this.currentBlockID),
			int64(this.blockLength), checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	if this.blockLength <= _SMALL_BLOCK_SIZE {
		this.blockTransformType = transform.NONE_TYPE
		this.blockEntropyType = entropy.NONE_TYPE
		mode |= byte(_COPY_BLOCK_MASK)
	} else {
		if skipOpt, prst := this.ctx["skipBlocks"]; prst == true {
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

	if this.blockLength >= 4 {
		magic := internal.GetMagicType(data)

		if internal.IsDataCompressed(magic) == true {
			this.ctx["dataType"] = internal.DT_BIN
		} else if internal.IsDataMultimedia(magic) == true {
			this.ctx["dataType"] = internal.DT_MULTIMEDIA
		} else if internal.IsDataExecutable(magic) == true {
			this.ctx["dataType"] = internal.DT_EXE
		}
	}

	if len(this.iBuffer.Buf) < requiredSize {
		extraBuf := make([]byte, requiredSize-len(this.iBuffer.Buf))
		data = append(data, extraBuf...)
		this.iBuffer.Buf = data
	}

	if len(this.oBuffer.Buf) < requiredSize {
		extraBuf := make([]byte, requiredSize-len(this.oBuffer.Buf))
		buffer = append(buffer, extraBuf...)
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
			int64(postTransformLength), checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	bufSize := postTransformLength

	if bufSize < this.blockLength+(this.blockLength>>3) {
		bufSize = this.blockLength + (this.blockLength >> 3)
	}

	if bufSize < 512*1024 {
		bufSize = 512 * 1024
	}

	if len(data) < int(bufSize) {
		// Rare case where the transform expanded the input or the entropy
		// coder may expand the size
		data = make([]byte, bufSize)
	}

	// Create a bitstream local to the task
	bufStream := util.NewBufferStream(data[0:0:cap(data)])
	obs, _ := bitstream.NewDefaultOutputBitStream(bufStream, 16384)

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
	if this.hasher != nil {
		obs.WriteBits(uint64(checksum), 32)
	}

	if len(this.listeners) > 0 {
		// Notify before entropy
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, int(this.currentBlockID),
			int64(postTransformLength), checksum, this.hasher != nil, time.Now())
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

	// Lock free synchronization
	for n := 0; ; n++ {
		taskID := atomic.LoadInt32(this.processedBlockID)

		if taskID == _CANCEL_TASKS_ID {
			return
		}

		if taskID == this.currentBlockID-1 {
			break
		}

		runtime.Gosched()
	}

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, int(this.currentBlockID),
			int64((written+7)>>3), checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
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
		//nolint
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
	checksum       uint32
	completionTime time.Time
}

// CompressedInputStream a Reader that reads compressed data
// from an InputBitStream.
type CompressedInputStream struct {
	blockSize       int
	hasher          *hash.XXHash32
	buffers         []blockBuffer
	entropyType     uint32
	transformType   uint64
	ibs             kanzi.InputBitStream
	initialized     int32
	closed          int32
	blockID         int32
	jobs            int
	bufferThreshold int
	available       int // decoded not consumed bytes
	consumed        int // decoded consumed bytes
	nbInputBlocks   int
	listeners       []kanzi.Listener
	ctx             map[string]interface{}
}

type decodingTask struct {
	iBuffer            *blockBuffer
	oBuffer            *blockBuffer
	hasher             *hash.XXHash32
	blockLength        uint
	blockTransformType uint64
	blockEntropyType   uint32
	currentBlockID     int32
	processedBlockID   *int32
	wg                 *sync.WaitGroup
	listeners          []kanzi.Listener
	ibs                kanzi.InputBitStream
	ctx                map[string]interface{}
}

// NewCompressedInputStream creates a new instance of CompressedInputStream
func NewCompressedInputStream(is io.ReadCloser, jobs uint) (*CompressedInputStream, error) {
	ctx := make(map[string]interface{})
	ctx["jobs"] = jobs
	return NewCompressedInputStreamWithCtx(is, ctx)
}

// NewCompressedInputStreamWithCtx creates a new instance of CompressedInputStream
// using a map of parameters
func NewCompressedInputStreamWithCtx(is io.ReadCloser, ctx map[string]interface{}) (*CompressedInputStream, error) {
	var err error
	var ibs kanzi.InputBitStream

	if ibs, err = bitstream.NewDefaultInputBitStream(is, _STREAM_DEFAULT_BUFFER_SIZE); err != nil {
		errMsg := fmt.Sprintf("Cannot create input bit stream: %v", err)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_CREATE_BITSTREAM}
	}

	return createCompressedInputStreamWithCtx(ibs, ctx)
}

// NewCompressedInputStreamWithCtx2 creates a new instance of CompressedInputStream
// using a map of parameters and a custom input bitstream
func NewCompressedInputStreamWithCtx2(ibs kanzi.InputBitStream, ctx map[string]interface{}) (*CompressedInputStream, error) {
	return createCompressedInputStreamWithCtx(ibs, ctx)
}

func createCompressedInputStreamWithCtx(ibs kanzi.InputBitStream, ctx map[string]interface{}) (*CompressedInputStream, error) {
	if ibs == nil {
		return nil, &IOError{msg: "Invalid null input bitstream parameter", code: kanzi.ERR_CREATE_STREAM}
	}

	if ctx == nil {
		return nil, &IOError{msg: "Invalid null context parameter", code: kanzi.ERR_CREATE_STREAM}
	}

	tasks := ctx["jobs"].(uint)

	if tasks == 0 || tasks > _MAX_CONCURRENCY {
		errMsg := fmt.Sprintf("The number of jobs must be in [1..%d], got %d", _MAX_CONCURRENCY, tasks)
		return nil, &IOError{msg: errMsg, code: kanzi.ERR_CREATE_STREAM}
	}

	this := &CompressedInputStream{}
	this.ibs = ibs
	this.jobs = int(tasks)
	this.blockID = 0
	this.consumed = 0
	this.available = 0
	this.bufferThreshold = 0
	this.buffers = make([]blockBuffer, 2*this.jobs)

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: make([]byte, 0)}
	}

	this.listeners = make([]kanzi.Listener, 0)
	this.ctx = ctx
	this.blockSize = 0
	this.entropyType = entropy.NONE_TYPE
	this.transformType = transform.NONE_TYPE
	return this, nil
}

// AddListener adds an event listener to this input stream.
// Returns true if the listener has been added.
func (this *CompressedInputStream) AddListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

// RemoveListener removes an event listener from this input stream.
// Returns true if the listener has been removed.
func (this *CompressedInputStream) RemoveListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	for i, e := range this.listeners {
		if e == bl {
			this.listeners = append(this.listeners[0:i-1], this.listeners[i+1:]...)
			return true
		}
	}

	return false
}

func (this *CompressedInputStream) readHeader() error {
	defer func() {
		if r := recover(); r != nil {
			panic(&IOError{msg: "Cannot read bitstream header: " + r.(error).Error(), code: kanzi.ERR_READ_FILE})
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
	var err error

	// Read block checksum
	if this.ibs.ReadBit() == 1 {
		this.hasher, err = hash.NewXXHash32(_BITSTREAM_TYPE)

		if err != nil {
			return err
		}
	}

	// Read entropy codec
	this.entropyType = uint32(this.ibs.ReadBits(5))
	var eType string

	if eType, err = entropy.GetName(this.entropyType); err != nil {
		errMsg := fmt.Sprintf("Invalid bitstream, invalid entropy type: %d", this.entropyType)
		return &IOError{msg: errMsg, code: kanzi.ERR_INVALID_CODEC}
	}

	this.ctx["codec"] = eType
	this.ctx["extra"] = this.entropyType == entropy.TPAQX_TYPE

	// Read transforms: 8*6 bits
	this.transformType = this.ibs.ReadBits(48)
	var tType string

	if tType, err = transform.GetName(this.transformType); err != nil {
		errMsg := fmt.Sprintf("Invalid bitstream, invalid transform type: %d", this.transformType)
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

	// Read number of blocks in input. 0 means 'unknown' and 63 means 63 or more.
	this.nbInputBlocks = int(this.ibs.ReadBits(6))

	if this.nbInputBlocks == 0 {
		this.nbInputBlocks = _UNKNOWN_NB_BLOCKS
	}

	// Read checksum
	cksum1 := uint32(this.ibs.ReadBits(4))

	if bsVersion >= 3 {
		// Verify checksum from bitstream version 3
		HASH := uint32(0x1E35A7BD)
		var cksum2 uint32
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
	}

	if len(this.listeners) > 0 {
		msg := ""
		msg += fmt.Sprintf("Checksum set to %v\n", this.hasher != nil)
		msg += fmt.Sprintf("Block size set to %d bytes\n", this.blockSize)
		w1, _ := entropy.GetName(this.entropyType)

		if w1 == "NONE" {
			w1 = "no"
		}

		msg += fmt.Sprintf("Using %v entropy codec (stage 1)\n", w1)
		w2, _ := transform.GetName(this.transformType)

		if w2 == "NONE" {
			w2 = "no"
		}

		msg += fmt.Sprintf("Using %v transform (stage 2)\n", w2)
		evt := kanzi.NewEventFromString(kanzi.EVT_AFTER_HEADER_DECODING, 0, msg, time.Now())
		notifyListeners(this.listeners, evt)
	}

	return nil
}

// Close reads the buffered data from the input stream and releases resources.
// Close makes the bitstream unavailable for further reads. Idempotent
func (this *CompressedInputStream) Close() error {
	if atomic.SwapInt32(&this.closed, 1) == 1 {
		return nil
	}

	if _, err := this.ibs.Close(); err != nil {
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
// It returns the number of bytes read (0 <= n <= len(block)) and any error encountered.
func (this *CompressedInputStream) Read(block []byte) (int, error) {
	if atomic.LoadInt32(&this.closed) == 1 {
		return 0, &IOError{msg: "Stream closed", code: kanzi.ERR_READ_FILE}
	}

	if atomic.SwapInt32(&this.initialized, 1) == 0 {
		if err := this.readHeader(); err != nil {
			return 0, err
		}
	}

	off := 0
	remaining := len(block)

	for remaining > 0 {
		avail := this.available
		bufOff := this.consumed % this.blockSize

		if avail > this.bufferThreshold-bufOff {
			avail = this.bufferThreshold - bufOff
		}

		lenChunk := remaining

		// lenChunk = min(remaining, min(this.available, this.bufferThreshold-bufOff))
		if lenChunk > avail {
			lenChunk = avail
		}

		if lenChunk > 0 {
			// Process a chunk of in-buffer data. No access to bitstream required
			bufID := this.consumed / this.blockSize
			copy(block[off:], this.buffers[bufID].Buf[bufOff:bufOff+lenChunk])
			off += lenChunk
			remaining -= lenChunk
			this.available -= lenChunk
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

func (this *CompressedInputStream) processBlock() (int, error) {
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
	decoded := 0

	for {
		nbTasks := this.jobs
		var jobsPerTask []uint

		// Assign optimal number of tasks and jobs per task
		if nbTasks > 1 {
			// Limit the number of jobs if there are fewer blocks that this.jobs
			// It allows more jobs per task and reduces memory usage.
			if nbTasks > this.nbInputBlocks {
				nbTasks = this.nbInputBlocks
			}

			jobsPerTask, _ = internal.ComputeJobsPerTask(make([]uint, nbTasks), uint(this.jobs), uint(nbTasks))
		} else {
			jobsPerTask = []uint{uint(this.jobs)}
		}

		results := make([]decodingTaskResult, nbTasks)
		wg := sync.WaitGroup{}
		firstID := this.blockID
		bufSize := this.blockSize + _EXTRA_BUFFER_SIZE

		if bufSize < this.blockSize+(this.blockSize>>4) {
			bufSize = this.blockSize + (this.blockSize >> 4)
		}

		// Invoke as many go routines as required
		for taskID := 0; taskID < nbTasks; taskID++ {
			if len(this.buffers[taskID].Buf) < int(bufSize) {
				this.buffers[taskID].Buf = make([]byte, bufSize)
			}

			copyCtx := make(map[string]interface{})

			for k, v := range this.ctx {
				copyCtx[k] = v
			}

			copyCtx["jobs"] = jobsPerTask[taskID]
			results[taskID] = decodingTaskResult{}
			wg.Add(1)

			task := decodingTask{
				iBuffer:            &this.buffers[taskID],
				oBuffer:            &this.buffers[this.jobs+taskID],
				hasher:             this.hasher,
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
		skipped := 0

		// Process results
		for _, r := range results {
			if r.decoded > this.blockSize {
				return decoded, &IOError{msg: "Invalid data", code: kanzi.ERR_PROCESS_BLOCK}
			}

			decoded += r.decoded

			if r.err != nil {
				return decoded, r.err
			}

			if r.skipped == true {
				skipped++
			}
		}

		n := 0

		for _, r := range results {
			copy(this.buffers[n].Buf, r.data[0:r.decoded])
			n++

			if len(listeners) > 0 {
				// Notify after transform ... in block order !
				evt := kanzi.NewEvent(kanzi.EVT_AFTER_TRANSFORM, int(r.blockID),
					int64(r.decoded), r.checksum, this.hasher != nil, r.completionTime)
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
func (this *CompressedInputStream) GetRead() uint64 {
	return (this.ibs.Read() + 7) >> 3
}

// Decode mode + transformed entropy coded data
// mode | 0b10000000 => copy block
// mode | 0b0yy00000 => size(size(block))-1
// mode | 0b000y0000 => 1 if more than 4 transforms
//
// case 4 transforms or less
// mode	| 0b0000yyyy => transform sequence skip flags (1 means skip)
//
// case more than 4 transforms
// mode | 0b00000000
//
// then 0byyyyyyyy => transform sequence skip flags (1 means skip)
func (this *decodingTask) decode(res *decodingTaskResult) {
	data := this.iBuffer.Buf
	buffer := this.oBuffer.Buf
	decoded := 0
	checksum1 := uint32(0)
	skipped := false

	defer func() {
		res.data = this.iBuffer.Buf
		res.decoded = decoded
		res.blockID = int(this.currentBlockID)
		res.completionTime = time.Now()
		res.checksum = checksum1
		res.skipped = skipped

		if r := recover(); r != nil {
			res.err = &IOError{msg: r.(error).Error(), code: kanzi.ERR_PROCESS_BLOCK}
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
	for {
		taskID := atomic.LoadInt32(this.processedBlockID)

		if taskID == _CANCEL_TASKS_ID {
			return
		}

		if taskID == this.currentBlockID-1 {
			break
		}

		runtime.Gosched()
	}

	// Read shared bitstream sequentially
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
		extraBuf := make([]byte, maxL-len(data))
		buffer = append(data, extraBuf...)
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
		from := v.(int)
		if int(this.currentBlockID) < from {
			skipped = true
			return
		}
	}

	if v, hasKey := this.ctx["to"]; hasKey {
		to := v.(int)

		if int(this.currentBlockID) >= to {
			skipped = true
			return
		}
	}

	// All the code below is concurrent
	// Create a bitstream local to the task
	bufStream := util.NewBufferStream(data[0:r])
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

	if preTransformLength == 0 {
		res.err = &IOError{msg: "Invalid block size", code: kanzi.ERR_BLOCK_SIZE}
		return
	}

	if preTransformLength > _MAX_BITSTREAM_BLOCK_SIZE {
		// Error => cancel concurrent decoding tasks
		errMsg := fmt.Sprintf("Invalid compressed block length: %d", preTransformLength)
		res.err = &IOError{msg: errMsg, code: kanzi.ERR_BLOCK_SIZE}
		return
	}

	// Extract checksum from bit stream (if any)
	if this.hasher != nil {
		checksum1 = uint32(ibs.ReadBits(32))
	}

	if len(this.listeners) > 0 {
		// Notify before entropy (block size in bitstream is unknown)
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, int(this.currentBlockID),
			int64(-1), checksum1, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	bufferSize := this.blockLength

	if bufferSize < preTransformLength+_EXTRA_BUFFER_SIZE {
		bufferSize = preTransformLength + _EXTRA_BUFFER_SIZE
	}

	if len(buffer) < int(bufferSize) {
		extraBuf := make([]byte, int(bufferSize)-len(buffer))
		buffer = append(buffer, extraBuf...)
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

	defer ed.Dispose()

	// Block entropy decode
	if _, err = ed.Read(buffer[0:preTransformLength]); err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = &IOError{msg: err.Error(), code: kanzi.ERR_PROCESS_BLOCK}
		return
	}

	ibs.Close()

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, int(this.currentBlockID),
			int64(ibs.Read())/8, checksum1, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	if len(this.listeners) > 0 {
		// Notify before transform
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, int(this.currentBlockID),
			int64(preTransformLength), checksum1, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
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
	if this.hasher != nil {
		checksum2 := this.hasher.Hash(data[0:decoded])

		if checksum2 != checksum1 {
			errMsg := fmt.Sprintf("Corrupted bitstream: expected checksum %x, found %x", checksum1, checksum2)
			res.err = &IOError{msg: errMsg, code: kanzi.ERR_CRC_CHECK}
			return
		}
	}
}
