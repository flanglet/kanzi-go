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
	"io"
	"sync/atomic"
	"time"

	kanzi "github.com/flanglet/kanzi-go"
	"github.com/flanglet/kanzi-go/bitstream"
	"github.com/flanglet/kanzi-go/entropy"
	"github.com/flanglet/kanzi-go/function"
	"github.com/flanglet/kanzi-go/util/hash"
)

// Write to/read from stream using a 2 step process:
// Encoding:
// - step 1: a ByteFunction is used to reduce the size of the input data (bytes input & output)
// - step 2: an EntropyEncoder is used to entropy code the results of step 1 (bytes input, bits output)
// Decoding is the exact reverse process.

const (
	_BITSTREAM_TYPE             = 0x4B414E5A // "KANZ"
	_BITSTREAM_FORMAT_VERSION   = 8
	_STREAM_DEFAULT_BUFFER_SIZE = 256 * 1024
	_EXTRA_BUFFER_SIZE          = 256
	_COPY_BLOCK_MASK            = 0x80
	_TRANSFORMS_MASK            = 0x10
	_MIN_BITSTREAM_BLOCK_SIZE   = 1024
	_MAX_BITSTREAM_BLOCK_SIZE   = 1024 * 1024 * 1024
	_SMALL_BLOCK_SIZE           = 15
	_MAX_CONCURRENCY            = 64
)

var (
	_EMPTY_BYTE_SLICE = make([]byte, 0)
)

// IOError an extended error containing a message and a code value
type IOError struct {
	msg  string
	code int
}

// NewIOError creates a new instance of IOError
func NewIOError(msg string, code int) *IOError {
	return &IOError{msg: msg, code: code}
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
	// Enclose a buffer in a struct to share it between stream and tasks
	// and reduce memory allocation.
	// The tasks can re-allocate the buffer as needed.
	Buf []byte
}

// CompressedOutputStream a Writer that writes compressed data
// to an OutputBitStream.
type CompressedOutputStream struct {
	blockSize     uint
	nbInputBlocks uint8
	hasher        *hash.XXHash32
	data          []byte
	buffers       []blockBuffer
	entropyType   uint32
	transformType uint64
	obs           kanzi.OutputBitStream
	initialized   int32
	closed        int32
	blockID       int
	curIdx        int
	jobs          int
	channels      []chan error
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
	currentBlockID     int
	input              chan error
	output             chan error
	listeners          []kanzi.Listener
	obs                kanzi.OutputBitStream
	ctx                map[string]interface{}
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
// map of parameters
func NewCompressedOutputStreamWithCtx(os io.WriteCloser, ctx map[string]interface{}) (*CompressedOutputStream, error) {
	if os == nil {
		return nil, NewIOError("Invalid null writer parameter", kanzi.ERR_CREATE_STREAM)
	}

	if ctx == nil {
		return nil, NewIOError("Invalid null context parameter", kanzi.ERR_CREATE_STREAM)
	}

	entropyCodec := ctx["codec"].(string)
	transform := ctx["transform"].(string)
	tasks := ctx["jobs"].(uint)

	if tasks == 0 || tasks > _MAX_CONCURRENCY {
		errMsg := fmt.Sprintf("The number of jobs must be in [1..%v]", _MAX_CONCURRENCY)
		return nil, NewIOError(errMsg, kanzi.ERR_CREATE_STREAM)
	}

	bSize := ctx["blockSize"].(uint)

	if bSize > _MAX_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at most %d MB", _MAX_BITSTREAM_BLOCK_SIZE>>20)
		return nil, NewIOError(errMsg, kanzi.ERR_CREATE_STREAM)
	}

	if bSize < _MIN_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at least %d", _MIN_BITSTREAM_BLOCK_SIZE)
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

	if this.obs, err = bitstream.NewDefaultOutputBitStream(os, _STREAM_DEFAULT_BUFFER_SIZE); err != nil {
		return nil, err
	}

	// Check entropy type validity (panic on error)
	this.entropyType = entropy.GetType(entropyCodec)

	// Check transform type validity (panic on error)
	this.transformType = function.GetType(transform)

	this.blockSize = bSize
	nbBlocks := uint8(0)

	// If input size has been provided, calculate the number of blocks
	// in the input data else use 0. A value of 63 means '63 or more blocks'.
	// This value is written to the bitstream header to let the decoder make
	// better decisions about memory usage and job allocation in concurrent
	// decompression scenario.
	if val, containsKey := ctx["fileSize"]; containsKey {
		fileSize := val.(int64)
		nbBlocks = uint8((fileSize + int64(bSize-1)) / int64(bSize))
	}

	if nbBlocks > 63 {
		this.nbInputBlocks = 63
	} else {
		this.nbInputBlocks = nbBlocks
	}

	checksum := ctx["checksum"].(bool)

	if checksum == true {
		this.hasher, err = hash.NewXXHash32(_BITSTREAM_TYPE)

		if err != nil {
			return nil, err
		}
	}

	this.jobs = int(tasks)
	this.data = make([]byte, int(this.blockSize)) // initially 1 blockSize
	this.buffers = make([]blockBuffer, 2*this.jobs)

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: _EMPTY_BYTE_SLICE}
	}

	this.blockID = 0
	this.channels = make([]chan error, this.jobs+1)

	for i := range this.channels {
		this.channels[i] = make(chan error)
	}

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
	cksum := 0

	if this.hasher != nil {
		cksum = 1
	}

	if this.obs.WriteBits(_BITSTREAM_TYPE, 32) != 32 {
		return NewIOError("Cannot write bitstream type to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(_BITSTREAM_FORMAT_VERSION, 5) != 5 {
		return NewIOError("Cannot write bitstream version to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(cksum), 1) != 1 {
		return NewIOError("Cannot write checksum to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.entropyType), 5) != 5 {
		return NewIOError("Cannot write entropy type to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.transformType), 48) != 48 {
		return NewIOError("Cannot write transform types to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.blockSize>>4), 28) != 28 {
		return NewIOError("Cannot write block size to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.nbInputBlocks), 6) != 6 {
		return NewIOError("Cannot write number of blocks to header", kanzi.ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(0, 3) != 3 {
		return NewIOError("Cannot write reserved bits to header", kanzi.ERR_WRITE_FILE)
	}

	return nil
}

// Write writes len(block) bytes from block to the underlying data stream.
// It returns the number of bytes written from block (0 <= n <= len(block)) and
// any error encountered that caused the write to stop early.
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

// Close writes the buffered data to the output stream then writes
// a final empty block and releases resources.
// Close makes the bitstream unavailable for further writes. Idempotent.
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
	this.obs.WriteBits(_COPY_BLOCK_MASK, 8)
	this.obs.WriteBits(0, 8)

	if _, err := this.obs.Close(); err != nil {
		return err
	}

	// Release resources
	this.data = _EMPTY_BYTE_SLICE

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: _EMPTY_BYTE_SLICE}
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
	for jobID := 0; jobID < this.jobs; jobID++ {
		if this.curIdx == 0 {
			break
		}

		nbJobs = jobID + 1
		sz := uint(this.curIdx)

		if sz >= this.blockSize {
			sz = this.blockSize
		}

		if len(this.buffers[2*jobID].Buf) < int(sz) {
			this.buffers[2*jobID].Buf = make([]byte, sz)
		}

		copy(this.buffers[2*jobID].Buf, this.data[offset:offset+sz])
		copyCtx := make(map[string]interface{})

		for k, v := range this.ctx {
			copyCtx[k] = v
		}

		task := encodingTask{
			iBuffer:            &this.buffers[2*jobID],
			oBuffer:            &this.buffers[2*jobID+1],
			hasher:             this.hasher,
			blockLength:        sz,
			blockTransformType: this.transformType,
			blockEntropyType:   this.entropyType,
			currentBlockID:     this.blockID + jobID + 1,
			input:              this.channels[jobID],
			output:             this.channels[jobID+1],
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

	this.blockID += this.jobs
	return err
}

// GetWritten returns the number of bytes written so far
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
func (this *encodingTask) encode() {
	data := this.iBuffer.Buf
	buffer := this.oBuffer.Buf
	mode := byte(0)
	var postTransformLength uint
	checksum := uint32(0)

	// Compute block checksum
	if this.hasher != nil {
		checksum = this.hasher.Hash(data[0:this.blockLength])
	}

	if len(this.listeners) > 0 {
		// Notify before transform
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, this.currentBlockID,
			int64(this.blockLength), checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	inputReceived := false

	defer func() {
		if r := recover(); r != nil {
			if inputReceived == false {
				<-this.input
			}

			this.output <- NewIOError(r.(error).Error(), kanzi.ERR_PROCESS_BLOCK)
		}
	}()

	if this.blockLength <= _SMALL_BLOCK_SIZE {
		if this.blockLength == 0 {
			this.blockTransformType = function.NONE_TYPE
			this.blockEntropyType = entropy.NONE_TYPE
			mode |= byte(_COPY_BLOCK_MASK)
		}
	} else {

		if skip, prst := this.ctx["skipBlocks"]; prst == true {
			if skip.(bool) == true {
				histo := [256]int{}
				entropy1024 := entropy.ComputeFirstOrderEntropy1024(data[0:this.blockLength], histo[:])
				//this.ctx["histo0"] = histo

				if entropy1024 >= entropy.INCOMPRESSIBLE_THRESHOLD {
					this.blockTransformType = function.NONE_TYPE
					this.blockEntropyType = entropy.NONE_TYPE
					mode |= _COPY_BLOCK_MASK
				}
			}
		}
	}

	this.ctx["size"] = this.blockLength
	t, err := function.NewByteFunction(&this.ctx, this.blockTransformType)

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
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_TRANSFORM, this.currentBlockID,
			int64(postTransformLength), checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	// Wait for the concurrent task processing the previous block to complete
	// entropy encoding. Entropy encoding must happen sequentially (and
	// in the correct block order) in the bitstream.
	err2 := <-this.input
	inputReceived = true

	if err2 != nil {
		this.output <- err2
		return
	}

	// Write block 'header' (mode + compressed length)
	written := this.obs.Written()

	if ((mode & _COPY_BLOCK_MASK) != 0) || (t.Len() <= 4) {
		mode |= byte(t.SkipFlags() >> 4)
		this.obs.WriteBits(uint64(mode), 8)
	} else {
		mode |= _TRANSFORMS_MASK
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
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, this.currentBlockID,
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
	_, err = ee.Write(buffer[0:postTransformLength])

	if err != nil {
		this.output <- NewIOError(err.Error(), kanzi.ERR_PROCESS_BLOCK)
		return
	}

	// Dispose before displaying statistics. Dispose may write to the bitstream
	ee.Dispose()

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, this.currentBlockID,
			int64(this.obs.Written()-written)/8, checksum, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	// Notify of completion of the task
	this.output <- error(nil)
}

func notifyListeners(listeners []kanzi.Listener, evt *kanzi.Event) {
	defer func() {
		//lint:ignore SA9003 ignore panics in listeners
		if r := recover(); r != nil {
			// Ignore panics in block listeners
		}
	}()

	for _, bl := range listeners {
		bl.ProcessEvent(evt)
	}
}

type message struct {
	err            *IOError
	data           []byte
	decoded        int
	blockID        int
	checksum       uint32
	completionTime time.Time
}

type semaphore chan bool

// CompressedInputStream a Reader that reads compressed data
// from an InputBitStream.
type CompressedInputStream struct {
	blockSize     uint
	nbInputBlocks uint8
	hasher        *hash.XXHash32
	data          []byte
	buffers       []blockBuffer
	entropyType   uint32
	transformType uint64
	ibs           kanzi.InputBitStream
	initialized   int32
	closed        int32
	blockID       int
	maxIdx        int
	curIdx        int
	jobs          int
	resChan       chan message
	listeners     []kanzi.Listener
	readLastBlock bool
	ctx           map[string]interface{}
}

type decodingTask struct {
	iBuffer            *blockBuffer
	oBuffer            *blockBuffer
	hasher             *hash.XXHash32
	blockLength        uint
	blockTransformType uint64
	blockEntropyType   uint32
	currentBlockID     int
	input              chan bool
	output             chan bool
	result             chan message
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
	if is == nil {
		return nil, NewIOError("Invalid null reader parameter", kanzi.ERR_CREATE_STREAM)
	}

	if ctx == nil {
		return nil, NewIOError("Invalid null context parameter", kanzi.ERR_CREATE_STREAM)
	}

	tasks := ctx["jobs"].(uint)

	if tasks == 0 || tasks > _MAX_CONCURRENCY {
		errMsg := fmt.Sprintf("The number of jobs must be in [1..%v]", _MAX_CONCURRENCY)
		return nil, NewIOError(errMsg, kanzi.ERR_CREATE_STREAM)
	}

	this := new(CompressedInputStream)

	this.jobs = int(tasks)
	this.blockID = 0
	this.data = _EMPTY_BYTE_SLICE
	this.buffers = make([]blockBuffer, 2*this.jobs)

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: _EMPTY_BYTE_SLICE}
	}

	this.resChan = make(chan message)
	var err error

	if this.ibs, err = bitstream.NewDefaultInputBitStream(is, _STREAM_DEFAULT_BUFFER_SIZE); err != nil {
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
			panic(NewIOError("Cannot read bitstream header: "+r.(error).Error(), kanzi.ERR_READ_FILE))
		}
	}()

	// Read stream type
	fileType := this.ibs.ReadBits(32)

	// Sanity check
	if fileType != _BITSTREAM_TYPE {
		return NewIOError("Invalid stream type", kanzi.ERR_INVALID_FILE)
	}

	version := this.ibs.ReadBits(5)

	// Sanity check
	if version != _BITSTREAM_FORMAT_VERSION {
		errMsg := fmt.Sprintf("Invalid bitstream, cannot read this version of the stream: %d", version)
		return NewIOError(errMsg, kanzi.ERR_STREAM_VERSION)
	}

	// Read block checksum
	if this.ibs.ReadBit() == 1 {
		var err error
		this.hasher, err = hash.NewXXHash32(_BITSTREAM_TYPE)

		if err != nil {
			return err
		}
	}

	// Read entropy codec
	this.entropyType = uint32(this.ibs.ReadBits(5))
	this.ctx["codec"] = entropy.GetName(this.entropyType)

	// Read transforms: 8*6 bits
	this.transformType = this.ibs.ReadBits(48)
	this.ctx["transform"] = function.GetName(this.transformType)

	// Read block size
	this.blockSize = uint(this.ibs.ReadBits(28)) << 4
	this.ctx["blockSize"] = this.blockSize

	if this.blockSize < _MIN_BITSTREAM_BLOCK_SIZE || this.blockSize > _MAX_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("Invalid bitstream, incorrect block size: %d", this.blockSize)
		return NewIOError(errMsg, kanzi.ERR_BLOCK_SIZE)
	}

	if uint64(this.blockSize)*uint64(this.jobs) >= uint64(1<<31) {
		this.jobs = int(uint(1<<31) / this.blockSize)
	}

	// Read number of blocks in input. 0 means 'unknown' and 63 means 63 or more.
	this.nbInputBlocks = uint8(this.ibs.ReadBits(6))

	// Read reserved bits
	this.ibs.ReadBits(3)

	if len(this.listeners) > 0 {
		msg := ""
		msg += fmt.Sprintf("Checksum set to %v\n", this.hasher != nil)
		msg += fmt.Sprintf("Block size set to %d bytes\n", this.blockSize)
		w1 := entropy.GetName(this.entropyType)

		if w1 == "NONE" {
			w1 = "no"
		}

		msg += fmt.Sprintf("Using %v entropy codec (stage 1)\n", w1)
		w2 := function.GetName(this.transformType)

		if w2 == "NONE" {
			w2 = "no"
		}

		msg += fmt.Sprintf("Using %v transform (stage 2)\n", w2)
		evt := kanzi.NewEventFromString(kanzi.EVT_AFTER_HEADER_DECODING, 0, msg, time.Now())
		notifyListeners(this.listeners, evt)
	}

	return nil
}

// Close reads the buffered data intto the input stream and releases resources.
// Close makes the bitstream unavailable for further reads. Idempotent
func (this *CompressedInputStream) Close() error {
	if atomic.SwapInt32(&this.closed, 1) == 1 {
		return nil
	}

	if _, err := this.ibs.Close(); err != nil {
		return err
	}

	// Release resources
	this.maxIdx = 0
	this.data = _EMPTY_BYTE_SLICE

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: _EMPTY_BYTE_SLICE}
	}

	close(this.resChan)
	return nil
}

// Read reads up to len(block) bytes into block.
// It returns the number of bytes read (0 <= n <= len(block)) and any error encountered.
func (this *CompressedInputStream) Read(block []byte) (int, error) {
	if atomic.LoadInt32(&this.closed) == 1 {
		return 0, NewIOError("Stream closed", kanzi.ERR_READ_FILE)
	}

	startChunk := 0
	remaining := len(block)

	for remaining > 0 {
		lenChunk := len(block) - startChunk

		if lenChunk+this.curIdx >= this.maxIdx {
			// Limit to number of available bytes in buffer
			lenChunk = this.maxIdx - this.curIdx
		}

		if lenChunk > 0 {
			// Process a chunk of in-buffer data. No access to bitstream required
			copy(block[startChunk:], this.data[this.curIdx:this.curIdx+lenChunk])
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
				return len(block) - remaining, err
			}

			if this.maxIdx == 0 {
				// Reached end of stream
				if len(block) == remaining {
					// EOF and we did not read any bytes in this call
					return 0, nil
				}

				break
			}
		}
	}

	return len(block) - remaining, nil
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

	// Add a padding area to manage any block with header or temporarily expanded
	if _EXTRA_BUFFER_SIZE >= (blkSize >> 4) {
		blkSize += _EXTRA_BUFFER_SIZE
	} else {
		blkSize += (blkSize >> 4)
	}

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
	for jobID := range syncChan {
		// Lazy instantiation of input buffers this.buffers[2*jobID]
		// Output buffers this.buffers[2*jobID+1] are lazily instantiated
		// by the decoding tasks.
		if len(this.buffers[2*jobID].Buf) < blkSize {
			this.buffers[2*jobID].Buf = make([]byte, blkSize)
		}

		copyCtx := make(map[string]interface{})

		for k, v := range this.ctx {
			copyCtx[k] = v
		}

		copyCtx["jobs"] = jobsPerTask[jobID]

		task := decodingTask{
			iBuffer:            &this.buffers[2*jobID],
			oBuffer:            &this.buffers[2*jobID+1],
			hasher:             this.hasher,
			blockLength:        uint(blkSize),
			blockTransformType: this.transformType,
			blockEntropyType:   this.entropyType,
			currentBlockID:     this.blockID + jobID + 1,
			input:              syncChan[jobID],
			output:             syncChan[(jobID+1)%int(nbJobs)],
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
	results := make([]message, nbJobs)

	// Wait for completion of all concurrent tasks
	for range results {
		// Listen for results on the shared channel
		res := <-this.resChan

		// Order the results based on block ID
		results[res.blockID-this.blockID-1] = res
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
			evt := kanzi.NewEvent(kanzi.EVT_AFTER_TRANSFORM, res.blockID,
				int64(res.decoded), res.checksum, this.hasher != nil, res.completionTime)
			notifyListeners(listeners, evt)
		}

		if res.decoded == 0 {
			this.readLastBlock = true
			break
		}
	}

	this.blockID += this.jobs
	this.curIdx = 0
	return decoded, err
}

// GetRead returns the number of bytes read so far
func (this *CompressedInputStream) GetRead() uint64 {
	return (this.ibs.Read() + 7) >> 3
}

// Used by block decoding tasks to synchronize and return result
func notify(chan1 chan bool, chan2 chan message, run bool, msg message) {
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
func (this *decodingTask) decode() {
	data := this.iBuffer.Buf
	buffer := this.oBuffer.Buf
	res := message{blockID: this.currentBlockID, data: data}

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

	if mode&_COPY_BLOCK_MASK != 0 {
		this.blockTransformType = function.NONE_TYPE
		this.blockEntropyType = entropy.NONE_TYPE
	} else {
		if mode&_TRANSFORMS_MASK != 0 {
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

	if preTransformLength > _MAX_BITSTREAM_BLOCK_SIZE {
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
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, this.currentBlockID,
			int64(-1), checksum1, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	res.checksum = checksum1
	bufferSize := this.blockLength

	if bufferSize < preTransformLength+_EXTRA_BUFFER_SIZE {
		bufferSize = preTransformLength + _EXTRA_BUFFER_SIZE
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
	if _, err = ed.Read(buffer[0:preTransformLength]); err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = NewIOError(err.Error(), kanzi.ERR_PROCESS_BLOCK)
		notify(this.output, this.result, false, res)
		return
	}

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, this.currentBlockID,
			int64(this.ibs.Read()-read)/8, checksum1, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	// After completion of the entropy decoding, unfreeze the task processing
	// the next block (if any)
	notify(this.output, nil, true, res)

	if len(this.listeners) > 0 {
		// Notify before transform
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, this.currentBlockID,
			int64(preTransformLength), checksum1, this.hasher != nil, time.Now())
		notifyListeners(this.listeners, evt)
	}

	this.ctx["size"] = preTransformLength
	transform, err := function.NewByteFunction(&this.ctx, this.blockTransformType)

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
