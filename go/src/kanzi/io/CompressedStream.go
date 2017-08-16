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
	"bytes"
	"fmt"
	"io"
	"kanzi"
	"kanzi/bitstream"
	"kanzi/entropy"
	"kanzi/function"
	"kanzi/util/hash"
	"sync/atomic"
)

// Write to/read from stream using a 2 step process:
// Encoding:
// - step 1: a ByteFunction is used to reduce the size of the input data (bytes input & output)
// - step 2: an EntropyEncoder is used to entropy code the results of step 1 (bytes input, bits output)
// Decoding is the exact reverse process.

const (
	BITSTREAM_TYPE             = 0x4B414E5A // "KANZ"
	BITSTREAM_FORMAT_VERSION   = 4
	STREAM_DEFAULT_BUFFER_SIZE = 1024 * 1024
	EXTRA_BUFFER_SIZE          = 256
	COPY_LENGTH_MASK           = 0x0F
	SMALL_BLOCK_MASK           = 0x80
	SKIP_FUNCTION_MASK         = 0x40
	MIN_BITSTREAM_BLOCK_SIZE   = 1024
	MAX_BITSTREAM_BLOCK_SIZE   = 1024 * 1024 * 1024
	SMALL_BLOCK_SIZE           = 15

	ERR_MISSING_PARAM       = -1
	ERR_BLOCK_SIZE          = -2
	ERR_INVALID_CODEC       = -3
	ERR_CREATE_COMPRESSOR   = -4
	ERR_CREATE_DECOMPRESSOR = -5
	ERR_OUTPUT_IS_DIR       = -6
	ERR_OVERWRITE_FILE      = -7
	ERR_CREATE_FILE         = -8
	ERR_CREATE_BITSTREAM    = -9
	ERR_OPEN_FILE           = -10
	ERR_READ_FILE           = -11
	ERR_WRITE_FILE          = -12
	ERR_PROCESS_BLOCK       = -13
	ERR_CREATE_CODEC        = -14
	ERR_INVALID_FILE        = -15
	ERR_STREAM_VERSION      = -16
	ERR_CREATE_STREAM       = -17
	ERR_INVALID_PARAM       = -18
	ERR_UNKNOWN             = -127
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
	hasher        *hash.XXHash32
	data          []byte
	buffers       []blockBuffer
	entropyType   uint16
	transformType uint16
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
	iBuffer         *blockBuffer
	oBuffer         *blockBuffer
	hasher          *hash.XXHash32
	blockLength     uint
	typeOfTransform uint16
	typeOfEntropy   uint16
	currentBlockId  int
	input           chan error
	output          chan error
	listeners       []kanzi.Listener
	obs             kanzi.OutputBitStream
	ctx             map[string]interface{}
}

func NewCompressedOutputStream(os io.WriteCloser, ctx map[string]interface{}) (*CompressedOutputStream, error) {
	if os == nil {
		return nil, NewIOError("Invalid null writer parameter", ERR_CREATE_STREAM)
	}

	if ctx == nil {
		return nil, NewIOError("Invalid null context parameter", ERR_CREATE_STREAM)
	}

	entropyCodec := ctx["codec"].(string)
	transform := ctx["transform"].(string)
	bSize := ctx["blockSize"].(uint)

	if bSize > MAX_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at most %d MB", MAX_BITSTREAM_BLOCK_SIZE>>20)
		return nil, NewIOError(errMsg, ERR_CREATE_STREAM)
	}

	if bSize < MIN_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at least %d", MIN_BITSTREAM_BLOCK_SIZE)
		return nil, NewIOError(errMsg, ERR_CREATE_STREAM)
	}

	if int(bSize)&-16 != int(bSize) {
		return nil, NewIOError("The block size must be a multiple of 16", ERR_CREATE_STREAM)
	}

	tasks := ctx["jobs"].(uint)

	if tasks < 0 || tasks > 16 { // 0 indicates no user choice
		return nil, NewIOError("The number of jobs must be in [1..16]", ERR_CREATE_STREAM)
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
	this.entropyType = entropy.GetEntropyCodecType(entropyCodec)

	// Check transform type validity (panic on error)
	this.transformType = GetByteFunctionType(transform)

	this.blockSize = bSize
	checksum := ctx["checksum"].(bool)

	if checksum == true {
		this.hasher, err = hash.NewXXHash32(BITSTREAM_TYPE)

		if err != nil {
			return nil, err
		}
	}

	if tasks == 0 {
		this.jobs = 1
	} else {
		this.jobs = int(tasks)
	}

	this.data = make([]byte, this.jobs*int(this.blockSize))
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
		return NewIOError("Cannot write bitstream type to header", ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(BITSTREAM_FORMAT_VERSION, 7) != 7 {
		return NewIOError("Cannot write bitstream version to header", ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(cksum), 1) != 1 {
		return NewIOError("Cannot write checksum to header", ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.entropyType), 5) != 5 {
		return NewIOError("Cannot write entropy type to header", ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.transformType), 16) != 16 {
		return NewIOError("Cannot write transform types to header", ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(uint64(this.blockSize>>4), 26) != 26 {
		return NewIOError("Cannot write block size to header", ERR_WRITE_FILE)
	}

	if this.obs.WriteBits(0, 9) != 9 {
		return NewIOError("Cannot write reserved bits to header", ERR_WRITE_FILE)
	}

	return nil
}

func (this *CompressedOutputStream) Write(array []byte) (int, error) {
	if atomic.LoadInt32(&this.closed) == 1 {
		return 0, NewIOError("Stream closed", ERR_WRITE_FILE)
	}

	startChunk := 0
	remaining := len(array)
	bSize := int(this.jobs) * int(this.blockSize)

	for remaining > 0 {
		if this.curIdx >= bSize {
			// Buffer full, time to encode
			if err := this.processBlock(); err != nil {
				return len(array) - remaining, err
			}
		}

		lenChunk := len(array) - startChunk

		if lenChunk+this.curIdx >= bSize {
			// Limit to number of available bytes in buffer
			lenChunk = bSize - this.curIdx
		}

		// Process a chunk of in-buffer data. No access to bitstream required
		copy(this.data[this.curIdx:], array[startChunk:startChunk+lenChunk])
		this.curIdx += lenChunk
		startChunk += lenChunk
		remaining -= lenChunk
	}

	return len(array) - remaining, nil
}

func (this *CompressedOutputStream) Close() error {
	if atomic.SwapInt32(&this.closed, 1) == 1 {
		return nil
	}

	if this.curIdx > 0 {
		if err := this.processBlock(); err != nil {
			return err
		}

		this.curIdx = 0
	}

	// Write end block of size 0
	this.obs.WriteBits(SMALL_BLOCK_MASK, 8)

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

func (this *CompressedOutputStream) processBlock() error {
	if this.curIdx == 0 {
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
			iBuffer:         &this.buffers[2*jobId],
			oBuffer:         &this.buffers[2*jobId+1],
			hasher:          this.hasher,
			blockLength:     sz,
			typeOfTransform: this.transformType,
			typeOfEntropy:   this.entropyType,
			currentBlockId:  this.blockId + jobId + 1,
			input:           this.channels[jobId],
			output:          this.channels[jobId+1],
			obs:             this.obs,
			listeners:       listeners,
			ctx:             copyCtx}

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
// mode: 0b1000xxxx => small block (written as is) + 4 LSB for block size (0-15)
//       0x00xxxx00 => transform sequence skip flags (1 means skip)
//       0x000000xx => size(size(block))-1
func (this *EncodingTask) encode() {
	data := this.iBuffer.Buf
	buffer := this.oBuffer.Buf
	mode := byte(0)
	dataSize := uint(0)
	postTransformLength := this.blockLength
	checksum := uint32(0)

	// Compute block checksum
	if this.hasher != nil {
		checksum = this.hasher.Hash(data[0:this.blockLength])
	}

	if len(this.listeners) > 0 {
		// Notify before transform
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, this.currentBlockId,
			int64(this.blockLength), checksum, this.hasher != nil)
		notifyListeners(this.listeners, evt)
	}

	if this.blockLength <= SMALL_BLOCK_SIZE {
		// Just copy
		if !kanzi.SameByteSlices(buffer, data, false) {
			if len(buffer) < int(this.blockLength) {
				buffer = make([]byte, this.blockLength)
				this.oBuffer.Buf = buffer
			}

			copy(buffer, data[0:this.blockLength])
		}

		mode = byte(SMALL_BLOCK_SIZE | (this.blockLength & COPY_LENGTH_MASK))
	} else {
		this.ctx["size"] = this.blockLength
		t, err := NewByteFunction(this.ctx, this.typeOfTransform)

		if err != nil {
			<-this.input
			this.output <- NewIOError(err.Error(), ERR_CREATE_CODEC)
			return
		}

		requiredSize := t.MaxEncodedLen(int(this.blockLength))

		if len(buffer) < requiredSize {
			buffer = make([]byte, requiredSize)
			this.oBuffer.Buf = buffer
		}

		// Forward transform (ignore error, encode skipFlags)
		_, postTransformLength, _ = t.Forward(data[0:this.blockLength], buffer)
		mode |= ((t.SkipFlags() & function.TRANSFORM_SKIP_MASK) << 2)

		for i := uint64(0xFF); i < uint64(postTransformLength); i <<= 8 {
			dataSize++
		}

		if dataSize > 3 {
			<-this.input
			this.output <- NewIOError("Invalid block data length", ERR_WRITE_FILE)
			return
		}

		// Record size of 'block size' - 1 in bytes
		mode |= byte(dataSize & 0x03)
		dataSize++
	}

	if len(this.listeners) > 0 {
		// Notify after transform
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_TRANSFORM, this.currentBlockId,
			int64(postTransformLength), checksum, this.hasher != nil)
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
	this.obs.WriteBits(uint64(mode), 8)

	if dataSize > 0 {
		this.obs.WriteBits(uint64(postTransformLength), 8*dataSize)
	}

	// Write checksum
	if this.hasher != nil {
		this.obs.WriteBits(uint64(checksum), 32)
	}

	if len(this.listeners) > 0 {
		// Notify before entropy
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, this.currentBlockId,
			int64(postTransformLength), checksum, this.hasher != nil)
		notifyListeners(this.listeners, evt)
	}

	// Each block is encoded separately
	// Rebuild the entropy encoder to reset block statistics
	ee, err := entropy.NewEntropyEncoder(this.obs, this.ctx, this.typeOfEntropy)

	if err != nil {
		this.output <- NewIOError(err.Error(), ERR_CREATE_CODEC)
		return
	}

	// Entropy encode block
	_, err = ee.Encode(buffer[0:postTransformLength])

	if err != nil {
		this.output <- NewIOError(err.Error(), ERR_PROCESS_BLOCK)
		return
	}

	// Dispose before displaying statistics. Dispose may write to the bitstream
	ee.Dispose()

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, this.currentBlockId,
			int64(this.obs.Written()-written)/8, checksum, this.hasher != nil)
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
	err      *IOError
	data     []byte
	decoded  int
	blockId  int
	text     string
	checksum uint32
}

type semaphore chan bool

type CompressedInputStream struct {
	blockSize     uint
	hasher        *hash.XXHash32
	data          []byte
	buffers       []blockBuffer
	entropyType   uint16
	transformType uint16
	ibs           kanzi.InputBitStream
	initialized   int32
	closed        int32
	blockId       int
	maxIdx        int
	curIdx        int
	jobs          int
	syncChan      []semaphore
	resChan       chan Message
	listeners     []kanzi.Listener
	readLastBlock bool
	ctx           map[string]interface{}
}

type DecodingTask struct {
	iBuffer         *blockBuffer
	oBuffer         *blockBuffer
	hasher          *hash.XXHash32
	blockLength     uint
	typeOfTransform uint16
	typeOfEntropy   uint16
	currentBlockId  int
	input           chan bool
	output          chan bool
	result          chan Message
	listeners       []kanzi.Listener
	ibs             kanzi.InputBitStream
	ctx             map[string]interface{}
}

func NewCompressedInputStream(is io.ReadCloser, ctx map[string]interface{}) (*CompressedInputStream, error) {
	if is == nil {
		return nil, NewIOError("Invalid null reader parameter", ERR_CREATE_STREAM)
	}

	if ctx == nil {
		return nil, NewIOError("Invalid null context parameter", ERR_CREATE_STREAM)
	}

	tasks := ctx["jobs"].(uint)

	if tasks < 0 || tasks > 16 { // 0 indicates no user choice
		return nil, NewIOError("The number of jobs must be in [1..16]", ERR_CREATE_STREAM)
	}

	this := new(CompressedInputStream)

	if tasks == 0 {
		this.jobs = 1
	} else {
		this.jobs = int(tasks)
	}

	this.blockId = 0
	this.data = EMPTY_BYTE_SLICE
	this.buffers = make([]blockBuffer, 2*this.jobs)

	for i := range this.buffers {
		this.buffers[i] = blockBuffer{Buf: EMPTY_BYTE_SLICE}
	}

	// Channel of semaphores
	this.syncChan = make([]semaphore, this.jobs)

	for i := range this.syncChan {
		if i > 0 {
			// First channel is nil
			this.syncChan[i] = make(semaphore)
		}
	}

	this.resChan = make(chan Message)
	var err error

	if this.ibs, err = bitstream.NewDefaultInputBitStream(is, STREAM_DEFAULT_BUFFER_SIZE); err != nil {
		errMsg := fmt.Sprintf("Cannot create input bit stream: %v", err)
		return nil, NewIOError(errMsg, ERR_CREATE_BITSTREAM)
	}

	this.listeners = make([]kanzi.Listener, 0)
	this.ctx = ctx
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
			panic(NewIOError("Cannot read bitstream header: "+r.(error).Error(), ERR_READ_FILE))
		}
	}()

	// Read stream type
	fileType := this.ibs.ReadBits(32)

	// Sanity check
	if fileType != BITSTREAM_TYPE {
		errMsg := fmt.Sprintf("Invalid stream type")
		return NewIOError(errMsg, ERR_INVALID_FILE)
	}

	version := this.ibs.ReadBits(7)

	// Sanity check
	if version != BITSTREAM_FORMAT_VERSION {
		errMsg := fmt.Sprintf("Invalid bitstream, cannot read this version of the stream: %d", version)
		return NewIOError(errMsg, ERR_STREAM_VERSION)
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
	this.entropyType = uint16(this.ibs.ReadBits(5))

	// Read transform
	this.transformType = uint16(this.ibs.ReadBits(16))

	// Read block size
	this.blockSize = uint(this.ibs.ReadBits(26)) << 4

	if this.blockSize < MIN_BITSTREAM_BLOCK_SIZE || this.blockSize > MAX_BITSTREAM_BLOCK_SIZE {
		errMsg := fmt.Sprintf("Invalid bitstream, incorrect block size: %d", this.blockSize)
		return NewIOError(errMsg, ERR_BLOCK_SIZE)
	}

	// Read reserved bits
	this.ibs.ReadBits(9)

	if len(this.listeners) > 0 {
		msg := ""
		msg += fmt.Sprintf("Checksum set to %v\n", this.hasher != nil)
		msg += fmt.Sprintf("Block size set to %d bytes\n", this.blockSize)
		w1 := GetByteFunctionName(this.transformType)

		if w1 == "NONE" {
			w1 = "no"
		}

		msg += fmt.Sprintf("Using %v transform (stage 1)\n", w1)
		w2 := entropy.GetEntropyCodecName(this.entropyType)

		if w2 == "NONE" {
			w2 = "no"
		}

		msg += fmt.Sprintf("Using %v entropy codec (stage 2)", w2)
		evt := kanzi.NewEventFromString(kanzi.EVT_AFTER_HEADER_DECODING, 0, msg)
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

	for _, c := range this.syncChan {
		if c != nil {
			close(c)
		}
	}

	close(this.resChan)
	return nil
}

// Implement kanzi.InputStream interface
func (this *CompressedInputStream) Read(array []byte) (int, error) {
	if atomic.LoadInt32(&this.closed) == 1 {
		return 0, NewIOError("Stream closed", ERR_READ_FILE)
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

	if len(this.data) < this.jobs*blkSize {
		this.data = make([]byte, this.jobs*blkSize)
	}

	// Add a padding area to manage any block with header (of size <= EXTRA_BUFFER_SIZE)
	blkSize += EXTRA_BUFFER_SIZE

	// Protect against future concurrent modification of the list of block listeners
	listeners := make([]kanzi.Listener, len(this.listeners))
	copy(listeners, this.listeners)

	// Invoke as many go routines as required
	for jobId := 0; jobId < this.jobs; jobId++ {
		curChan := this.syncChan[jobId]
		nextChan := this.syncChan[(jobId+1)%this.jobs]

		if len(this.buffers[2*jobId].Buf) < blkSize {
			this.buffers[2*jobId].Buf = make([]byte, blkSize)
		}

		copyCtx := make(map[string]interface{})

		for k, v := range this.ctx {
			copyCtx[k] = v
		}

		task := DecodingTask{
			iBuffer:         &this.buffers[2*jobId],
			oBuffer:         &this.buffers[2*jobId+1],
			hasher:          this.hasher,
			blockLength:     uint(blkSize),
			typeOfTransform: this.transformType,
			typeOfEntropy:   this.entropyType,
			currentBlockId:  this.blockId + jobId + 1,
			input:           curChan,
			output:          nextChan,
			result:          this.resChan,
			listeners:       listeners,
			ibs:             this.ibs,
			ctx:             copyCtx}

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
	results := make([]Message, this.jobs)

	// Wait for completion of all concurrent tasks
	for _ = range results {
		// Listen for results on the shared channel
		res := <-this.resChan

		// Order the results based on block ID
		results[res.blockId-this.blockId-1] = res
	}

	// Process results
	for _, res := range results {
		if res.err != nil {
			if err == nil {
				// Keep first error encountered
				err = res.err
			}
		} else {
			copy(this.data[offset:], res.data[0:res.decoded])
			offset += res.decoded
			decoded += res.decoded

			if len(listeners) > 0 {
				// Notify after transform ... in block order
				evt := kanzi.NewEvent(kanzi.EVT_AFTER_TRANSFORM, res.blockId,
					int64(res.decoded), res.checksum, this.hasher != nil)
				notifyListeners(listeners, evt)
			}

			if res.decoded == 0 {
				this.readLastBlock = true
			}
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
		chan2 <- msg
	}
}

// Decode mode + transformed entropy coded data
// mode: 0b1000xxxx => small block (written as is) + 4 LSB for block size (0-15)
//       0x00xxxx00 => transform sequence skip flags (1 means skip)
//       0x000000xx => size(size(block))-1
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
			res.err = NewIOError(r.(error).Error(), ERR_READ_FILE)
			notify(this.output, this.result, false, res)
		}
	}()

	// Extract block header directly from bitstream
	read := this.ibs.Read()
	mode := byte(this.ibs.ReadBits(8))
	var preTransformLength uint
	checksum1 := uint32(0)

	if (mode & SMALL_BLOCK_MASK) != 0 {
		preTransformLength = uint(mode & COPY_LENGTH_MASK)
	} else {
		dataSize := uint(1 + (mode & 0x03))
		length := dataSize << 3
		mask := uint64(1<<length) - 1
		preTransformLength = uint(this.ibs.ReadBits(length) & mask)
	}

	if preTransformLength == 0 {
		// Last block is empty, return success and cancel pending tasks
		res.decoded = 0
		notify(this.output, this.result, false, res)
		return
	}

	if preTransformLength > MAX_BITSTREAM_BLOCK_SIZE {
		// Error => cancel concurrent decoding tasks
		errMsg := fmt.Sprintf("Invalid compressed block length: %d", preTransformLength)
		res.err = NewIOError(errMsg, ERR_BLOCK_SIZE)
		notify(this.output, this.result, false, res)
		return
	}

	// Extract checksum from bit stream (if any)
	if this.hasher != nil {
		checksum1 = uint32(this.ibs.ReadBits(32))
	}

	if len(this.listeners) > 0 {
		// Notify before entropy (block size in bitstream is unknown)
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_ENTROPY, this.currentBlockId,
			int64(-1), checksum1, this.hasher != nil)
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

	// Each block is decoded separately
	// Rebuild the entropy decoder to reset block statistics
	ed, err := entropy.NewEntropyDecoder(this.ibs, this.ctx, this.typeOfEntropy)

	if err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = NewIOError(err.Error(), ERR_INVALID_CODEC)
		notify(this.output, this.result, false, res)
		return
	}

	defer ed.Dispose()

	// Block entropy decode
	if _, err = ed.Decode(buffer[0:preTransformLength]); err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = NewIOError(err.Error(), ERR_PROCESS_BLOCK)
		notify(this.output, this.result, false, res)
		return
	}

	if len(this.listeners) > 0 {
		// Notify after entropy
		evt := kanzi.NewEvent(kanzi.EVT_AFTER_ENTROPY, this.currentBlockId,
			int64(this.ibs.Read()-read)/8, checksum1, this.hasher != nil)
		notifyListeners(this.listeners, evt)
	}

	// After completion of the entropy decoding, unfreeze the task processing
	// the next block (if any)
	notify(this.output, nil, true, res)

	if len(this.listeners) > 0 {
		// Notify before transform
		evt := kanzi.NewEvent(kanzi.EVT_BEFORE_TRANSFORM, this.currentBlockId,
			int64(preTransformLength), checksum1, this.hasher != nil)
		notifyListeners(this.listeners, evt)
	}

	read = this.ibs.Read() - read

	if mode&SMALL_BLOCK_MASK != 0 {
		if !bytes.Equal(buffer, data) {
			copy(data, buffer[0:preTransformLength])
		}

		res.decoded = int(preTransformLength)
	} else {
		this.ctx["size"] = preTransformLength
		transform, err := NewByteFunction(this.ctx, this.typeOfTransform)

		if err != nil {
			// Error => return
			res.err = NewIOError(err.Error(), ERR_INVALID_CODEC)
			notify(nil, this.result, false, res)
			return
		}

		transform.SetSkipFlags(((mode >> 2) & function.TRANSFORM_SKIP_MASK))
		var oIdx uint

		// Inverse transform
		if _, oIdx, err = transform.Inverse(buffer[0:preTransformLength], data); err != nil {
			// Error => return
			res.err = NewIOError(err.Error(), ERR_PROCESS_BLOCK)
			notify(nil, this.result, false, res)
			return
		}

		res.decoded = int(oIdx)

		// Verify checksum
		if this.hasher != nil {
			checksum2 := this.hasher.Hash(data[0:res.decoded])

			if checksum2 != checksum1 {
				errMsg := fmt.Sprintf("Corrupted bitstream: expected checksum %x, found %x", checksum1, checksum2)
				res.err = NewIOError(errMsg, ERR_PROCESS_BLOCK)
				notify(nil, this.result, false, res)
				return
			}
		}

	}

	notify(nil, this.result, false, res)
}
