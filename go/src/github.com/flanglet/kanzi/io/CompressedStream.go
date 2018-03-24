/*
Copyright 2011-2017 Frederic Langlet
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

package io

import (
	"fmt"
	kanzi "github.com/flanglet/kanzi"
	"github.com/flanglet/kanzi/bitstream"
	"github.com/flanglet/kanzi/entropy"
	"github.com/flanglet/kanzi/function"
	"github.com/flanglet/kanzi/util/hash"
	"io"
	"sync/atomic"
	"time"
)

// Write to/read from stream using a 2 step process:
// Encoding:
// - step 1: a ByteFunction is used to reduce the size of the input data (bytes input & output)
// - step 2: an EntropyEncoder is used to entropy code the results of step 1 (bytes input, bits output)
// Decoding is the exact reverse process.

const (
	BITSTREAM_TYPE             = 0x4B414E5A // "KANZ"
	BITSTREAM_FORMAT_VERSION   = 5
	STREAM_DEFAULT_BUFFER_SIZE = 1024 * 1024
	EXTRA_BUFFER_SIZE          = 256
	COPY_BLOCK_MASK            = 0x80
	TRANSFORMS_MASK            = 0x10
	MIN_BITSTREAM_BLOCK_SIZE   = 1024
	MAX_BITSTREAM_BLOCK_SIZE   = 1024 * 1024 * 1024
	SMALL_BLOCK_SIZE           = 15
	MAX_CONCURRENCY            = 32
)

var (
	EMPTY_BYTE_SLICE = make([]byte, 0)
)

type IOError struct {
	msg  string
	code int
}

func NewIOError(msg string, code int) *IOError {
	return &IOError{msg: msg, code: code}
}

// Implement error interface
func (this IOError) Error() string {
	return fmt.Sprintf("%v (code %v)", this.msg, this.code)
}

func (this IOError) Message() string {
	return this.msg
}

func (this IOError) ErrorCode() int {
	return this.code
}

type blockBuffer struct {
	// Enclose a buffer in a struct to share it between stream and tasks
	// and reduce memory allocation.
	// The tasks can re-allocate the buffer as needed.
	Buf []byte
}

type CompressedOutputStream struct {
	blockSize     uint
	nbInputBlocks uint8
	hasher        *hash.XXHash32
	data          []byte
	buffers       []blockBuffer
	entropyType   uint32
	transformType uint32
	obs           kanzi.OutputBitStream
	initialized   int32
	closed        int32
	blockId       int
	curIdx        int
	jobs          int
	channels      []chan error
	listeners     []kanzi.Listener
	ctx           map[string]interface{}
}

type EncodingTask struct {
	iBuffer            *blockBuffer
	oBuffer            *blockBuffer
	hasher             *hash.XXHash32
	blockLength        uint
	blockTransformType uint32
	blockEntropyType   uint32
	currentBlockId     int
	input              chan error
	output             chan error
	listeners          []kanzi.Listener
	obs                kanzi.OutputBitStream
	ctx                map[string]interface{}
}

func NewCompressedOutputStream(os io.WriteCloser, ctx map[string]interface{}) (*CompressedOutputStream, error) {
	if os == nil {
		return nil, NewIOError("Invalid null writer parameter", kanzi.ERR_CREATE_STREAM)
	}

	if ctx == nil {
		return nil, NewIOError("Invalid null context parameter", kanzi.ERR_CREATE_STREAM)
	}

	entropyCodec := ctx["codec"].(string)
	transform := ctx["transform"].(string)
	tasks := ctx["jobs"].(uint)

	if tasks == 0 || tasks > MAX_CONCURRENCY {
		errMsg := fmt.Sprintf("The number of jobs must be in [1..%v]", MAX_CONCURRENCY)
		return nil, NewIOError(errMsg, kanzi.ERR_CREATE_STREAM)
	}

	bSize := ctx["blockSize"].(uint)

	if bSize > MAX_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at most %d MB", MAX_BITSTREAM_BLOCK_SIZE>>20)
		return nil, NewIOError(errMsg, kanzi.ERR_CREATE_STREAM)
	}

	if bSize < MIN_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at least %d", MIN_BITSTREAM_BLOCK_SIZE)
		return nil, NewIOError(errMsg, kanzi.ERR_CREATE_STREAM)
	}

	if int(bSize)&-16 != int(bSize) {
		return nil, NewIOError("The block size must be a multiple of 16", kanzi.ERR_CREATE_STREAM)
	}

	if uint64(bSize)*uint64(tasks) >= uint64(1<<31) {
		tasks = (1 << 31) / bSize
	}

	this := new(CompressedOutputStream)
	var err error

	bufferSize := bSize

	if bufferSize > 65536 {
		bufferSize = 65536
	}

	if this.obs, err = bitstream.NewDefaultOutputBitStream(os, bufferSize); err != nil {
		return nil, err
	}

	// Check entropy type validity (panic on error)
	this.entropyType = entropy.GetType(entropyCodec)

	// Check transform type validity (panic on error)
	this.transformType = function.GetType(transform)
	nbBlocks := uint8(0)

	this.blockSize = bSize

	// If input size has been provided, calculate the number of blocks
	// in the input data else use 0. A value of 63 means '63 or more blocks'.
	// This value is written to the bitstream header to let the decoder make
	// better decisions about memory usage and job allocation in concurrent
	// decompression scenario.
	if fileSize, ok := ctx["fileSize"].(int64); ok {
		nbBlocks = uint8((uint(fileSize) + (bSize - 1)) / bSize)
	}

	if nbBlocks > 63 {
		this.nbInputBlocks = 63
	} else {
		this.nbInputBlocks = nbBlocks
	}

	checksum := ctx["checksum"].(bool)

	if checksum == true {
		this.hasher, err = hash.NewXXHash32(BITSTREAM_TYPE)

		if err != nil {
			return nil, err
		}
	}

	this.jobs = int(tasks)
	this.data = make([]byte, int(this.blockSize)) // initially 1 blockSize
	this.buffers = make([]blockBuffer, 2*this.jobs)

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: EMPTY_BYTE_SLICE}
	}

	this.blockId = 0
	this.channels = make([]chan error, this.jobs+1)

	for i := range this.channels {
		this.channels[i] = make(chan error)
	}

	this.listeners = make([]kanzi.Listener, 0)
	this.ctx = ctx
	return this, nil
}

func (this *CompressedOutputStream) AddListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

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
	cksum := 0

	if this.hasher != nil {
		cksum = 1
	}

	if this.obs.WriteBits(BITSTREAM_TYPE, 32) != 32 {
		return NewIOError("Cannot write bitstream type to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(BITSTREAM_FORMAT_VERSION, 5) != 5 {
		return NewIOError("Cannot write bitstream version to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(cksum), 1) != 1 {
		return NewIOError("Cannot write checksum to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.entropyType), 5) != 5 {
		return NewIOError("Cannot write entropy type to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.transformType), 32) != 32 {
		return NewIOError("Cannot write transform types to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.blockSize>>4), 26) != 26 {
		return NewIOError("Cannot write block size to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.nbInputBlocks), 6) != 6 {
		return NewIOError("Cannot write number of blocks to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(0, 5) != 5 {
		return NewIOError("Cannot write reserved bits to header", kanzi.ERR_WRITE_FILE)
	}

	return nil
}

func (this *CompressedOutputStream) Write(block []byte) (int, error) {
	if atomic.LoadInt32(&this.closed) == 1 {
		return 0, NewIOError("Stream closed", kanzi.ERR_WRITE_FILE)
	}

	startChunk := 0
	remaining := len(block)

	for remaining > 0 {
		lenChunk := len(block) - startChunk

		if lenChunk+this.curIdx >= len(this.data) {
			// Limit to number of available bytes in buffer
			lenChunk = len(this.data) - this.curIdx
		}

		if lenChunk > 0 {
			// Process a chunk of in-buffer data. No access to bitstream required
			copy(this.data[this.curIdx:], block[startChunk:startChunk+lenChunk])
			this.curIdx += lenChunk
			startChunk += lenChunk
			remaining -= lenChunk

			if remaining == 0 {
				break
			}
		}

		if this.curIdx >= len(this.data) {
			// Buffer full, time to encode
			if err := this.processBlock(false); err != nil {
				return len(block) - remaining, err
			}
		}
	}

	return len(block) - remaining, nil
}

func (this *CompressedOutputStream) Close() error {
	if atomic.SwapInt32(&this.closed, 1) == 1 {
		return nil
	}

	if this.curIdx > 0 {
		if err := this.processBlock(true); err != nil {
			return err
		}

		this.curIdx = 0
	}

	// Write end block of size 0
	this.obs.WriteBits(COPY_BLOCK_MASK, 8)
	this.obs.WriteBits(0, 8)

	if _, err := this.obs.Close(); err != nil {
		return err
	}

	// Release resources
	this.data = EMPTY_BYTE_SLICE

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: EMPTY_BYTE_SLICE}
	}

	for _, c := range this.channels {
		close(c)
	}

	return nil
}

func (this *CompressedOutputStream) processBlock(force bool) error {
	if this.curIdx == 0 {
		return nil
	}

	if !force && len(this.data) < int(this.blockSize)*this.jobs {
		// Grow byte array until max allowed
		buf := make([]byte, len(this.data)+int(this.blockSize))
		copy(buf, this.data)
		this.data = buf
		return nil
	}

	if atomic.SwapInt32(&this.initialized, 1) == 0 {
		if err := this.writeHeader(); err != nil {
			return err
		}
	}

	offset := uint(0)

	// Protect against future concurrent modification of the list of block listeners
	listeners := make([]kanzi.Listener, len(this.listeners))
	copy(listeners, this.listeners)
	nbJobs := 0

	// Invoke as many go routines as required
	for jobId := 0; jobId < this.jobs; jobId++ {
		if this.curIdx == 0 {
			break
		}

		nbJobs = jobId + 1
		sz := uint(this.curIdx)

		if sz >= this.blockSize {
			sz = this.blockSize
		}

		if len(this.buffers[2*jobId].Buf) < int(sz) {
			this.buffers[2*jobId].Buf = make([]byte, sz)
		}

		copy(this.buffers[2*jobId].Buf, this.data[offset:offset+sz])
		copyCtx := make(map[string]interface{})

		for k, v := range this.ctx {
			copyCtx[k] = v
		}

		task := EncodingTask{
			iBuffer:            &this.buffers[2*jobId],
			oBuffer:            &this.buffers[2*jobId+1],
			hasher:             this.hasher,
			blockLength:        sz,
			blockTransformType: this.transformType,
			blockEntropyType:   this.entropyType,
			currentBlockId:     this.blockId + jobId + 1,
			input:              this.channels[jobId],
			output:             this.channels[jobId+1],
			obs:                this.obs,
			listeners:          listeners,
			ctx:                copyCtx}

		// Invoke the tasks concurrently
		// Tasks are chained through channels. Upon completion of transform
		// (concurrently) the tasks wait for a signal to start entropy encoding
		go task.encode()

		offset += sz
		this.curIdx -= int(sz)
	}

	// Allow start of entropy coding for first block
	this.channels[0] <- error(nil)

	// Wait for completion of last task
	err := <-this.channels[nbJobs]

	this.blockId += this.jobs
	return err
}

// Return the number of bytes written so far
func (this *CompressedOutputStream) GetWritten() uint64 {
	return (this.obs.Written() + 7) >> 3
}

// Encode mode + transformed entropy coded data
// mode | 0b10000000 => copy block
//      | 0b0yy00000 => size(size(block))-1
//      | 0b000y0000 => 1 if more than 4 transforms
//  case 4 transforms or less
//      | 0b0000yyyy => transform sequence skip flags (1 means skip)
//  case more than 4 transforms
//      | 0b00000000
//      then 0byyyyyyyy => transform sequence skip flags (1 means skip)
func (this *EncodingTask) encode() {
	data := this.iBuffer.Buf
	buffer := this.oBuffer.Buf
	mode := byte(0)
	postTransformLength := this.blockLength
	checksum := uint32(0)

	// Compute block checksum
	if this.hasher != nil {
		checksum = this.hasher.Hash(data[0:this.blockLength])
	}

	if len(this.listeners) > 0 {
		// Notify before transform
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, this.currentBlockId,
			int64(this.blockLength), checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	if this.blockLength <= SMALL_BLOCK_SIZE {
		if this.blockLength == 0 {
			this.blockTransformType = function.NONE_TYPE
			this.blockEntropyType = entropy.NONE_TYPE
			mode |= byte(COPY_BLOCK_MASK)
		}
	} else {
		histo := make([]int, 256)
		entropy1024 := entropy.ComputeFirstOrderEntropy1024(data[0:this.blockLength], histo)
		//this.ctx["histo0"] = histo

		if entropy1024 >= entropy.INCOMPRESSIBLE_THRESHOLD {
			this.blockTransformType = function.NONE_TYPE
			this.blockEntropyType = entropy.NONE_TYPE
			mode |= COPY_BLOCK_MASK
		}
	}

	this.ctx["size"] = this.blockLength
	t, err := function.NewByteFunction(this.ctx, this.blockTransformType)

	if err != nil {
		<-this.input
		this.output <- NewIOError(err.Error(), kanzi.ERR_CREATE_CODEC)
		return
	}

	requiredSize := t.MaxEncodedLen(int(this.blockLength))

	if len(buffer) < requiredSize {
		buffer = make([]byte, requiredSize)
		this.oBuffer.Buf = buffer
	}

	// Forward transform (ignore error, encode skipFlags)
	_, postTransformLength, _ = t.Forward(data[0:this.blockLength], buffer)
	this.ctx["size"] = postTransformLength
	dataSize := uint(0)

	for i := uint64(0xFF); i < uint64(postTransformLength); i <<= 8 {
		dataSize++
	}

	if dataSize > 3 {
		<-this.input
		this.output <- NewIOError("Invalid block data length", kanzi.ERR_WRITE_FILE)
		return
	}

	// Record size of 'block size' - 1 in bytes
	mode |= byte((dataSize & 0x03) << 5)
	dataSize++

	if len(this.listeners) > 0 {
		// Notify after transform
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_TRANSFORM, this.currentBlockId,
			int64(postTransformLength), checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	// Wait for the concurrent task processing the previous block to complete
	// entropy encoding. Entropy encoding must happen sequentially (and
	// in the correct block order) in the bitstream.
	err2 := <-this.input

	if err2 != nil {
		this.output <- err2
		return
	}

	// Write block 'header' (mode + compressed length)
	written := this.obs.Written()

	if ((mode & COPY_BLOCK_MASK) != 0) || (t.NbFunctions() <= 4) {
		mode |= byte(t.SkipFlags() >> 4)
		this.obs.WriteBits(uint64(mode), 8)
	} else {
		mode |= TRANSFORMS_MASK
		this.obs.WriteBits(uint64(mode), 8)
		this.obs.WriteBits(uint64(t.SkipFlags()), 8)
	}

	this.obs.WriteBits(uint64(postTransformLength), 8*dataSize)

	// Write checksum
	if this.hasher != nil {
		this.obs.WriteBits(uint64(checksum), 32)
	}

	if len(this.listeners) > 0 {
		// Notify before entropy
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, this.currentBlockId,
			int64(postTransformLength), checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	// Each block is encoded separately
	// Rebuild the entropy encoder to reset block statistics
	ee, err := entropy.NewEntropyEncoder(this.obs, this.ctx, this.blockEntropyType)

	if err != nil {
		this.output <- NewIOError(err.Error(), kanzi.ERR_CREATE_CODEC)
		return
	}

	// Entropy encode block
	_, err = ee.Encode(buffer[0:postTransformLength])

	if err != nil {
		this.output <- NewIOError(err.Error(), kanzi.ERR_PROCESS_BLOCK)
		return
	}

	// Dispose before displaying statistics. Dispose may write to the bitstream
	ee.Dispose()

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, this.currentBlockId,
			int64(this.obs.Written()-written)/8, checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	// Notify of completion of the task
	this.output <- error(nil)
}

func notifyListeners(listeners []kanzi.Listener, evt *kanzi.Event) {
	defer func() {
		if r := recover(); r != nil {
			// Ignore exceptions in block listeners
		}
	}()

	for _, bl := range listeners {
		bl.ProcessEvent(evt)
	}
}

type Message struct {
	err            *IOError
	data           []byte
	decoded        int
	blockId        int
	text           string
	checksum       uint32
	completionTime time.Time
}

type semaphore chan bool

type CompressedInputStream struct {
	blockSize     uint
	nbInputBlocks uint8
	hasher        *hash.XXHash32
	data          []byte
	buffers       []blockBuffer
	entropyType   uint32
	transformType uint32
	ibs           kanzi.InputBitStream
	initialized   int32
	closed        int32
	blockId       int
	maxIdx        int
	curIdx        int
	jobs          int
	resChan       chan Message
	listeners     []kanzi.Listener
	readLastBlock bool
	ctx           map[string]interface{}
}

type DecodingTask struct {
	iBuffer            *blockBuffer
	oBuffer            *blockBuffer
	hasher             *hash.XXHash32
	blockLength        uint
	blockTransformType uint32
	blockEntropyType   uint32
	currentBlockId     int
	input              chan bool
	output             chan bool
	result             chan Message
	listeners          []kanzi.Listener
	ibs                kanzi.InputBitStream
	ctx                map[string]interface{}
}

func NewCompressedInputStream(is io.ReadCloser, ctx map[string]interface{}) (*CompressedInputStream, error) {
	if is == nil {
		return nil, NewIOError("Invalid null reader parameter", kanzi.ERR_CREATE_STREAM)
	}

	if ctx == nil {
		return nil, NewIOError("Invalid null context parameter", kanzi.ERR_CREATE_STREAM)
	}

	tasks := ctx["jobs"].(uint)

	if tasks == 0 || tasks > MAX_CONCURRENCY {
		errMsg := fmt.Sprintf("The number of jobs must be in [1..%v]", MAX_CONCURRENCY)
		return nil, NewIOError(errMsg, kanzi.ERR_CREATE_STREAM)
	}

	this := new(CompressedInputStream)

	this.jobs = int(tasks)
	this.blockId = 0
	this.data = EMPTY_BYTE_SLICE
	this.buffers = make([]blockBuffer, 2*this.jobs)

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: EMPTY_BYTE_SLICE}
	}

	this.resChan = make(chan Message)
	var err error

	if this.ibs, err = bitstream.NewDefaultInputBitStream(is, STREAM_DEFAULT_BUFFER_SIZE); err != nil {
		errMsg := fmt.Sprintf("Cannot create input bit stream: %v", err)
		return nil, NewIOError(errMsg, kanzi.ERR_CREATE_BITSTREAM)
	}

	this.listeners = make([]kanzi.Listener, 0)
	this.ctx = ctx
	this.blockSize = 0
	this.entropyType = entropy.NONE_TYPE
	this.transformType = function.NONE_TYPE
	return this, nil
}

func (this *CompressedInputStream) AddListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

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
			panic(NewIOError("Cannot read bitstream header: "+r.(error).Error(), kanzi.ERR_READ_FILE))
		}
	}()

	// Read stream type
	fileType := this.ibs.ReadBits(32)

	// Sanity check
	if fileType != BITSTREAM_TYPE {
		errMsg := fmt.Sprintf("Invalid stream type")
		return NewIOError(errMsg, kanzi.ERR_INVALID_FILE)
	}

	version := this.ibs.ReadBits(5)

	// Sanity check
	if version != BITSTREAM_FORMAT_VERSION {
		errMsg := fmt.Sprintf("Invalid bitstream, cannot read this version of the stream: %d", version)
		return NewIOError(errMsg, kanzi.ERR_STREAM_VERSION)
	}

	// Read block checksum
	if this.ibs.ReadBit() == 1 {
		var err error
		this.hasher, err = hash.NewXXHash32(BITSTREAM_TYPE)

		if err != nil {
			return err
		}
	}

	// Read entropy codec
	this.entropyType = uint32(this.ibs.ReadBits(5))

	// Read transforms: 8*4 bits
	this.transformType = uint32(this.ibs.ReadBits(32))

	// Read block size
	this.blockSize = uint(this.ibs.ReadBits(26)) << 4
	this.ctx["blockSize"] = this.blockSize

	if this.blockSize < MIN_BITSTREAM_BLOCK_SIZE || this.blockSize > MAX_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("Invalid bitstream, incorrect block size: %d", this.blockSize)
		return NewIOError(errMsg, kanzi.ERR_BLOCK_SIZE)
	}

	if uint64(this.blockSize)*uint64(this.jobs) >= uint64(1<<31) {
		this.jobs = int(uint(1<<31) / this.blockSize)
	}

	// Read number of blocks in input. 0 means 'unknown' and 63 means 63 or more.
	this.nbInputBlocks = uint8(this.ibs.ReadBits(6))

	// Read reserved bits
	this.ibs.ReadBits(5)

	if len(this.listeners) > 0 {
		msg := ""
		msg += fmt.Sprintf("Checksum set to %v\n", this.hasher != nil)
		msg += fmt.Sprintf("Block size set to %d bytes\n", this.blockSize)
		w1 := function.GetName(this.transformType)

		if w1 == "NONE" {
			w1 = "no"
		}

		msg += fmt.Sprintf("Using %v transform (stage 1)\n", w1)
		w2 := entropy.GetName(this.entropyType)

		if w2 == "NONE" {
			w2 = "no"
		}

		msg += fmt.Sprintf("Using %v entropy codec (stage 2)", w2)
		evt := kanzi.NewEventFromString(kanzi.EVT_AFTER_HEADER_DECODING, 0, msg, time.Now())
		notifyListeners(this.listeners, evt)
	}

	return nil
}

// Implement kanzi.InputStream interface
func (this *CompressedInputStream) Close() error {
	if atomic.SwapInt32(&this.closed, 1) == 1 {
		return nil
	}

	if _, err := this.ibs.Close(); err != nil {
		return err
	}

	// Release resources
	this.maxIdx = 0
	this.data = EMPTY_BYTE_SLICE

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: EMPTY_BYTE_SLICE}
	}

	close(this.resChan)
	return nil
}

// Implement kanzi.InputStream interface
func (this *CompressedInputStream) Read(array []byte) (int, error) {
	if atomic.LoadInt32(&this.closed) == 1 {
		return 0, NewIOError("Stream closed", kanzi.ERR_READ_FILE)
	}

	startChunk := 0
	remaining := len(array)

	for remaining > 0 {
		lenChunk := len(array) - startChunk

		if lenChunk+this.curIdx >= this.maxIdx {
			// Limit to number of available bytes in buffer
			lenChunk = this.maxIdx - this.curIdx
		}

		if lenChunk > 0 {
			// Process a chunk of in-buffer data. No access to bitstream required
			copy(array[startChunk:], this.data[this.curIdx:this.curIdx+lenChunk])
			this.curIdx += lenChunk
			startChunk += lenChunk
			remaining -= lenChunk

			if remaining == 0 {
				break
			}
		}

		// Buffer empty, time to decode
		if this.curIdx >= this.maxIdx {
			var err error

			if this.maxIdx, err = this.processBlock(); err != nil {
				return len(array) - remaining, err
			}

			if this.maxIdx == 0 {
				// Reached end of stream
				if len(array) == remaining {
					// EOF and we did not read any bytes in this call
					return 0, nil
				}

				break
			}
		}
	}

	return len(array) - remaining, nil
}

func (this *CompressedInputStream) processBlock() (int, error) {
	if atomic.SwapInt32(&this.initialized, 1) == 0 {
		if err := this.readHeader(); err != nil {
			return 0, err
		}
	}

	if this.readLastBlock == true {
		return 0, nil
	}

	blkSize := int(this.blockSize)

	// Add a padding area to manage any block with header (of size <= EXTRA_BUFFER_SIZE)
	blkSize += EXTRA_BUFFER_SIZE

	// Protect against future concurrent modification of the list of block listeners
	listeners := make([]kanzi.Listener, len(this.listeners))
	copy(listeners, this.listeners)

	nbJobs := uint(this.jobs)
	var jobsPerTask []uint

	// Assign optimal number of tasks and jobs per task
	if nbJobs > 1 {
		// If the number of input blocks is available, use it to optimize
		// memory usage
		if this.nbInputBlocks != 0 {
			// Limit the number of jobs if there are fewer blocks that this.jobs
			// It allows more jobs per task and reduces memory usage.
			if nbJobs > uint(this.nbInputBlocks) {
				nbJobs = uint(this.nbInputBlocks)
			}
		}

		jobsPerTask = kanzi.ComputeJobsPerTask(make([]uint, nbJobs), uint(this.jobs), nbJobs)
	} else {
		jobsPerTask = make([]uint, nbJobs)
		jobsPerTask[0] = uint(this.jobs)
	}

	// Channel of semaphores
	syncChan := make([]semaphore, nbJobs)

	for i := range syncChan {
		if i > 0 {
			// First channel is nil
			syncChan[i] = make(semaphore)
		}
	}

	defer func() {
		for _, c := range syncChan {
			if c != nil {
				close(c)
			}
		}
	}()

	// Invoke as many go routines as required
	for jobId := range syncChan {
		// Lazy instantiation of input buffers this.buffers[2*jobId]
		// Output buffers this.buffers[2*jobId+1] are lazily instantiated
		// by the decoding tasks.
		if len(this.buffers[2*jobId].Buf) < blkSize {
			this.buffers[2*jobId].Buf = make([]byte, blkSize)
		}

		copyCtx := make(map[string]interface{})

		for k, v := range this.ctx {
			copyCtx[k] = v
		}

		task := DecodingTask{
			iBuffer:            &this.buffers[2*jobId],
			oBuffer:            &this.buffers[2*jobId+1],
			hasher:             this.hasher,
			blockLength:        uint(blkSize),
			blockTransformType: this.transformType,
			blockEntropyType:   this.entropyType,
			currentBlockId:     this.blockId + jobId + 1,
			input:              syncChan[jobId],
			output:             syncChan[(jobId+1)%int(nbJobs)],
			result:             this.resChan,
			listeners:          listeners,
			ibs:                this.ibs,
			ctx:                copyCtx}

		// Invoke the tasks concurrently
		// Tasks are daisy chained through channels. All tasks wait for a signal
		// on the input channel to start entropy decoding and then issue a message
		// to the next task on the output channel upon entropy decoding completion.
		// The transform step runs concurrently. The result is returned on the shared
		// channel. The output channel is nil for the last task and the input channel
		// is nil for the first task.
		go task.decode()
	}

	var err error
	decoded := 0
	offset := 0
	results := make([]Message, nbJobs)

	// Wait for completion of all concurrent tasks
	for _ = range results {
		// Listen for results on the shared channel
		res := <-this.resChan

		// Order the results based on block ID
		results[res.blockId-this.blockId-1] = res
		decoded += res.decoded

		if res.err != nil {
			return decoded, res.err
		}
	}

	if decoded > int(nbJobs)*int(this.blockSize) {
		return decoded, NewIOError("Invalid data", kanzi.ERR_PROCESS_BLOCK)
	}

	if len(this.data) < decoded {
		this.data = make([]byte, decoded)
	}

	// Process results
	for _, res := range results {
		copy(this.data[offset:], res.data[0:res.decoded])
		offset += res.decoded

		if len(listeners) > 0 {
			// Notify after transform ... in block order !
			evt := kanzi.NewEvent(kanzi.EVT_AFTER_TRANSFORM, res.blockId,
				int64(res.decoded), res.checksum, this.hasher != nil, res.completionTime)
			notifyListeners(listeners, evt)
		}

		if res.decoded == 0 {
			this.readLastBlock = true
			break
		}
	}

	this.blockId += this.jobs
	this.curIdx = 0
	return decoded, err
}

// Return the number of bytes read so far
func (this *CompressedInputStream) GetRead() uint64 {
	return (this.ibs.Read() + 7) >> 3
}

// Used by block decoding tasks to synchronize and return result
func notify(chan1 chan bool, chan2 chan Message, run bool, msg Message) {
	if chan1 != nil {
		chan1 <- run
	}

	if chan2 != nil {
		msg.completionTime = time.Now()
		chan2 <- msg
	}
}

// Decode mode + transformed entropy coded data
// mode | 0b10000000 => copy block
//      | 0b0yy00000 => size(size(block))-1
//      | 0b000y0000 => 1 if more than 4 transforms
//  case 4 transforms or less
//      | 0b0000yyyy => transform sequence skip flags (1 means skip)
//  case more than 4 transforms
//      | 0b00000000
//      then 0byyyyyyyy => transform sequence skip flags (1 means skip)
func (this *DecodingTask) decode() {
	data := this.iBuffer.Buf
	buffer := this.oBuffer.Buf
	res := Message{blockId: this.currentBlockId, data: data}

	// Wait for task processing the previous block to complete
	if this.input != nil {
		run := <-this.input

		// If one of the previous tasks failed, skip
		if run == false {
			notify(this.output, this.result, false, res)
			return
		}
	}

	defer func() {
		if r := recover(); r != nil {
			// Error => cancel concurrent decoding tasks
			res.err = NewIOError(r.(error).Error(), kanzi.ERR_READ_FILE)
			notify(this.output, this.result, false, res)
		}
	}()

	// Extract block header directly from bitstream
	read := this.ibs.Read()
	mode := byte(this.ibs.ReadBits(8))
	skipFlags := byte(0)

	if mode&COPY_BLOCK_MASK != 0 {
		this.blockTransformType = function.NONE_TYPE
		this.blockEntropyType = entropy.NONE_TYPE
	} else {
		if mode&TRANSFORMS_MASK != 0 {
			skipFlags = byte(this.ibs.ReadBits(8))
		} else {
			skipFlags = (mode << 4) | 0x0F
		}
	}

	dataSize := 1 + uint((mode>>5)&0x03)
	length := dataSize << 3
	mask := uint64(1<<length) - 1
	preTransformLength := uint(this.ibs.ReadBits(length) & mask)

	if preTransformLength == 0 {
		// Last block is empty, return success and cancel pending tasks
		res.decoded = 0
		notify(this.output, this.result, false, res)
		return
	}

	if preTransformLength > MAX_BITSTREAM_BLOCK_SIZE {
		// Error => cancel concurrent decoding tasks
		errMsg := fmt.Sprintf("Invalid compressed block length: %d", preTransformLength)
		res.err = NewIOError(errMsg, kanzi.ERR_BLOCK_SIZE)
		notify(this.output, this.result, false, res)
		return
	}

	checksum1 := uint32(0)

	// Extract checksum from bit stream (if any)
	if this.hasher != nil {
		checksum1 = uint32(this.ibs.ReadBits(32))
	}

	if len(this.listeners) > 0 {
		// Notify before entropy (block size in bitstream is unknown)
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, this.currentBlockId,
			int64(-1), checksum1, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	res.checksum = checksum1
	bufferSize := this.blockLength

	if bufferSize < preTransformLength+EXTRA_BUFFER_SIZE {
		bufferSize = preTransformLength + EXTRA_BUFFER_SIZE
	}

	if len(buffer) < int(bufferSize) {
		buffer = make([]byte, bufferSize)
		this.oBuffer.Buf = buffer
	}

	this.ctx["size"] = preTransformLength

	// Each block is decoded separately
	// Rebuild the entropy decoder to reset block statistics
	ed, err := entropy.NewEntropyDecoder(this.ibs, this.ctx, this.blockEntropyType)

	if err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = NewIOError(err.Error(), kanzi.ERR_INVALID_CODEC)
		notify(this.output, this.result, false, res)
		return
	}

	defer ed.Dispose()

	// Block entropy decode
	if _, err = ed.Decode(buffer[0:preTransformLength]); err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = NewIOError(err.Error(), kanzi.ERR_PROCESS_BLOCK)
		notify(this.output, this.result, false, res)
		return
	}

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, this.currentBlockId,
			int64(this.ibs.Read()-read)/8, checksum1, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	// After completion of the entropy decoding, unfreeze the task processing
	// the next block (if any)
	notify(this.output, nil, true, res)

	if len(this.listeners) > 0 {
		// Notify before transform
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, this.currentBlockId,
			int64(preTransformLength), checksum1, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	this.ctx["size"] = preTransformLength
	transform, err := function.NewByteFunction(this.ctx, this.blockTransformType)

	if err != nil {
		// Error => return
		res.err = NewIOError(err.Error(), kanzi.ERR_INVALID_CODEC)
		notify(nil, this.result, false, res)
		return
	}

	transform.SetSkipFlags(skipFlags)
	var oIdx uint

	// Inverse transform
	if _, oIdx, err = transform.Inverse(buffer[0:preTransformLength], data); err != nil {
		// Error => return
		res.err = NewIOError(err.Error(), kanzi.ERR_PROCESS_BLOCK)
		notify(nil, this.result, false, res)
		return
	}

	res.decoded = int(oIdx)

	// Verify checksum
	if this.hasher != nil {
		checksum2 := this.hasher.Hash(data[0:res.decoded])

		if checksum2 != checksum1 {
			errMsg := fmt.Sprintf("Corrupted bitstream: expected checksum %x, found %x", checksum1, checksum2)
			res.err = NewIOError(errMsg, kanzi.ERR_CRC_CHECK)
			notify(nil, this.result, false, res)
			return
		}
	}

	notify(nil, this.result, false, res)
}
