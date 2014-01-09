/*
Copyright 2011-2013 Frederic Langlet
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
	"container/list"
	"errors"
	"fmt"
	"io"
	"kanzi"
	"kanzi/bitstream"
	"kanzi/entropy"
	"kanzi/function"
	"kanzi/util"
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
	COPY_LENGTH_MASK           = 0x0F
	SMALL_BLOCK_MASK           = 0x80
	SKIP_FUNCTION_MASK         = 0x40
	MIN_BLOCK_SIZE             = 1024
	MAX_BLOCK_SIZE             = (16 * 1024 * 1024) - 4
	SMALL_BLOCK_SIZE           = 15

	ERR_MISSING_FILENAME    = -1
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
	this := new(IOError)
	this.msg = msg
	this.code = code
	return this
}

// Implement error interface
func (this IOError) Error() string {
	return fmt.Sprintf("%v: %v", this.msg, this.code)
}

func (this IOError) Message() string {
	return this.msg
}

func (this IOError) ErrorCode() int {
	return this.code
}

type CompressedOutputStream struct {
	blockSize     uint
	hasher        *util.XXHash
	data          []byte
	buffers       [][]byte
	entropyType   byte
	transformType byte
	obs           kanzi.OutputBitStream
	debugWriter   io.Writer
	initialized   bool
	closed        bool
	blockId       int
	curIdx        int
	jobs          int
	channels      []chan error
	listeners     *list.List
}

func NewCompressedOutputStream(entropyCodec string, functionType string, os kanzi.OutputStream, blockSize uint,
	checksum bool, debugWriter io.Writer, jobs uint) (*CompressedOutputStream, error) {
	if os == nil {
		return nil, errors.New("Invalid null output stream parameter")
	}

	if blockSize > MAX_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at most %d", MAX_BLOCK_SIZE)
		return nil, errors.New(errMsg)
	}

	if blockSize < MIN_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at least %d", MIN_BLOCK_SIZE)
		return nil, errors.New(errMsg)
	}

	if jobs < 1 || jobs > 16 {
		return nil, errors.New("The number of jobs must be in [1..16]")
	}

	this := new(CompressedOutputStream)
	var err error

	bufferSize := blockSize

	if bufferSize < 65536 {
		bufferSize = 65536
	}

	if this.obs, err = bitstream.NewDefaultOutputBitStream(os, bufferSize); err != nil {
		return nil, err
	}

	eType := entropyCodec[0]

	// Check entropy type validity
	checkedEntropyType, err := entropy.GetEntropyCodecName(eType)

	if err != nil {
		return nil, NewIOError(err.Error(), ERR_CREATE_CODEC)
	}

	if checkedEntropyType != entropyCodec {
		errMsg := "Unsupported entropy type: " + entropyCodec
		return nil, NewIOError(errMsg, ERR_CREATE_CODEC)
	}

	this.entropyType = eType
	fType := functionType[0]

	// Check transform type validity
	checkedFunctionType, err := function.GetByteFunctionName(fType)

	if err != nil {
		return nil, err
	}

	if checkedFunctionType != functionType {
		errMsg := "Unsupported transform type: " + functionType
		return nil, NewIOError(errMsg, ERR_CREATE_CODEC)
	}

	this.transformType = fType
	this.blockSize = blockSize

	if checksum == true {
		this.hasher, err = util.NewXXHash(BITSTREAM_TYPE)

		if err != nil {
			return nil, err
		}
	}

	this.data = make([]byte, jobs*blockSize)
	this.buffers = make([][]byte, jobs)

	for i := range this.buffers {
		this.buffers[i] = EMPTY_BYTE_SLICE
	}

	this.debugWriter = debugWriter
	this.jobs = int(jobs)
	this.blockId = 0
	this.channels = make([]chan error, this.jobs+1)

	for i := range this.channels {
		this.channels[i] = make(chan error)
	}

	this.listeners = list.New()
	return this, nil
}

func (this *CompressedOutputStream) AddListener(bl BlockListener) bool {
	if bl == nil {
		return false
	}

	this.listeners.PushFront(bl)
	return true
}

func (this *CompressedOutputStream) RemoveListener(bl BlockListener) bool {
	if bl == nil {
		return false
	}

	for e := this.listeners.Front(); e != nil; e = e.Next() {
		if e.Value == bl {
			this.listeners.Remove(e)
			return true
		}
	}

	return false
}

func (this *CompressedOutputStream) WriteHeader() *IOError {
	if this.initialized == true {
		return nil
	}

	var err error

	if _, err = this.obs.WriteBits(BITSTREAM_TYPE, 32); err != nil {
		return NewIOError("Cannot write bitstream type to header", ERR_WRITE_FILE)
	}

	if _, err = this.obs.WriteBits(BITSTREAM_FORMAT_VERSION, 7); err != nil {
		return NewIOError("Cannot write bitstream version to header", ERR_WRITE_FILE)
	}

	cksum := 0

	if this.hasher != nil {
		cksum = 1
	}

	if err = this.obs.WriteBit(cksum); err != nil {
		return NewIOError("Cannot write checksum to header", ERR_WRITE_FILE)
	}

	if _, err = this.obs.WriteBits(uint64(this.entropyType&0x7F), 7); err != nil {
		return NewIOError("Cannot write entropy type to header", ERR_WRITE_FILE)
	}

	if _, err = this.obs.WriteBits(uint64(this.transformType&0x7F), 7); err != nil {
		return NewIOError("Cannot write transform type to header", ERR_WRITE_FILE)
	}

	if _, err = this.obs.WriteBits(uint64(this.blockSize), 26); err != nil {
		return NewIOError("Cannot write block size to header", ERR_WRITE_FILE)
	}

	return nil
}

// Implement the kanzi.OutputStream interface
func (this *CompressedOutputStream) Write(array []byte) (int, error) {
	if this.closed == true {
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

// Implement the kanzi.OutputStream interface
func (this *CompressedOutputStream) Sync() error {
	// Let the embedded bitstream take care of the flushing
	return nil
}

// Implement the kanzi.OutputStream interface
func (this *CompressedOutputStream) Close() error {
	if this.closed == true {
		return nil
	}

	if this.curIdx > 0 {
		if err := this.processBlock(); err != nil {
			return err
		}

		this.curIdx = 0
	}

	// Write end block of size 0
	if _, err := this.obs.WriteBits(SMALL_BLOCK_MASK, 8); err != nil {
		return err
	}

	if _, err := this.obs.Close(); err != nil {
		return err
	}

	this.closed = true

	// Release resources
	this.data = EMPTY_BYTE_SLICE

	for i := range this.buffers {
		this.buffers[i] = EMPTY_BYTE_SLICE
	}

	for _, c := range this.channels {
		close(c)
	}

	this.listeners.Init()
	return nil
}

func (this *CompressedOutputStream) processBlock() error {
	if this.curIdx == 0 {
		return nil
	}

	if this.initialized == false {
		if err := this.WriteHeader(); err != nil {
			return err
		}

		this.initialized = true
	}

	offset := uint(0)
	blockNumber := this.blockId

	// Protect against future concurrent modification of the list of block listeners
	listeners_ := make([]BlockListener, this.listeners.Len())

	if this.listeners.Len() > 0 {
		n := 0

		for e := this.listeners.Front(); e != nil; e = e.Next() {
			listeners_[n] = e.Value.(BlockListener)
			n++
		}
	}

	// Invoke as many go routines as required
	for jobId := 0; jobId < this.jobs; jobId++ {
		blockNumber++
		sz := uint(this.curIdx)

		if sz >= this.blockSize {
			sz = this.blockSize
		}

		// Invoke the tasks concurrently
		// Tasks are chained through channels. Upon completion of transform
		// (concurrently) the tasks wait for a signal to start entropy encoding
		go this.encode(this.data[offset:offset+sz], this.buffers[jobId], sz,
			this.transformType, this.entropyType, blockNumber,
			this.channels[jobId], this.channels[jobId+1], listeners_)

		offset += sz
		this.curIdx -= int(sz)

		if this.curIdx == 0 {
			break
		}
	}

	// Allow start of entropy coding for first block
	this.channels[0] <- error(nil)

	// Wait for completion of last task
	err := <-this.channels[blockNumber-this.blockId]

	this.blockId += this.jobs
	return err
}

// Return the number of bytes written so far
func (this *CompressedOutputStream) GetWritten() uint64 {
	return (this.obs.Written() + 7) >> 3
}

func (this *CompressedOutputStream) encode(data, buf []byte, blockLength uint,
	typeOfTransform byte, typeOfEntropy byte, currentBlockId int,
	input, output chan error, listeners_ []BlockListener) {
	transform, err := function.NewByteFunction(blockLength, typeOfTransform)

	if err != nil {
		output <- NewIOError(err.Error(), ERR_CREATE_CODEC)
	}

	buffer := buf
	requiredSize := transform.MaxEncodedLen(int(blockLength))

	if requiredSize == -1 {
		// Max size unknown => guess
		requiredSize = int(blockLength) * 5 >> 2
	}

	if typeOfTransform == 'N' {
		buffer = data // share buffers if no transform
	} else if len(buf) < requiredSize {
		buffer = make([]byte, requiredSize)
	}

	mode := byte(0)
	dataSize := uint(0)
	compressedLength := blockLength
	checksum := uint32(0)
	iIdx := uint(0)
	oIdx := uint(0)

	// Compute block checksum
	if this.hasher != nil {
		checksum = this.hasher.Hash(data[0:blockLength])
	}

	if len(listeners_) > 0 {
		// Notify before transform
		evt, err := NewBlockEvent(EVT_BEFORE_TRANSFORM, currentBlockId,
			int(blockLength), checksum, this.hasher != nil)

		if err == nil {
			for _, bl := range listeners_ {
				bl.ProcessEvent(evt)
			}
		}
	}

	if blockLength <= SMALL_BLOCK_SIZE {
		// Just copy
		if !kanzi.SameByteSlices(buffer, data, false) {
			copy(buffer, data[0:blockLength])
		}

		iIdx += blockLength
		oIdx += blockLength
		mode = byte(SMALL_BLOCK_SIZE | (blockLength & COPY_LENGTH_MASK))
	} else {

		// Forward transform
		iIdx, oIdx, err = transform.Forward(data, buffer)

		if err != nil || oIdx >= blockLength {
			// Transform failed or did not compress, skip and copy
			if !kanzi.SameByteSlices(buffer, data, false) {
				copy(buffer, data)
			}

			iIdx = blockLength
			oIdx = blockLength
			mode |= SKIP_FUNCTION_MASK
		}

		compressedLength = oIdx
		dataSize++

		for i := uint(0xFF); i < compressedLength; i <<= 8 {
			dataSize++
		}

		// Record size of 'block size' in bytes
		mode |= byte(dataSize & 0x03)
	}

	if len(listeners_) > 0 {
		// Notify after transform
		evt, err := NewBlockEvent(EVT_AFTER_TRANSFORM, currentBlockId,
			int(compressedLength), checksum, this.hasher != nil)

		if err == nil {
			for _, bl := range listeners_ {
				bl.ProcessEvent(evt)
			}
		}
	}

	// Wait for the concurrent task processing the previous block to complete
	// entropy encoding. Entropy encoding must happen sequentially (and
	// in the correct block order) in the bitstream.
	err2 := <-input

	if err2 != nil {
		output <- err2
	}

	// Each block is encoded separately
	// Rebuild the entropy encoder to reset block statistics
	ee, err := entropy.NewEntropyEncoder(this.obs, typeOfEntropy)

	if err != nil {
		output <- NewIOError(err.Error(), ERR_CREATE_CODEC)
	}

	// Write block 'header' (mode + compressed length)
	written := this.obs.Written()
	this.obs.WriteBits(uint64(mode), 8)

	if dataSize > 0 {
		if _, err = this.obs.WriteBits(uint64(compressedLength), 8*dataSize); err != nil {
			output <- NewIOError(err.Error(), ERR_WRITE_FILE)
		}
	}

	// Write checksum
	if this.hasher != nil {
		if _, err = this.obs.WriteBits(uint64(checksum), 32); err != nil {
			output <- NewIOError(err.Error(), ERR_WRITE_FILE)
		}
	}

	if len(listeners_) > 0 {
		// Notify before entropy
		evt, err := NewBlockEvent(EVT_BEFORE_ENTROPY, currentBlockId,
			int(compressedLength), checksum, this.hasher != nil)

		if err == nil {
			for _, bl := range listeners_ {
				bl.ProcessEvent(evt)
			}
		}
	}

	// Entropy encode block
	_, err = ee.Encode(buffer[0:compressedLength])

	if err != nil {
		output <- NewIOError(err.Error(), ERR_PROCESS_BLOCK)
	}

	// Dispose before displaying statistics. Dispose may write to the bitstream
	ee.Dispose()

	if len(listeners_) > 0 {
		// Notify after entropy
		evt, err := NewBlockEvent(EVT_AFTER_ENTROPY, currentBlockId,
			int(this.obs.Written()-written)/8, checksum, this.hasher != nil)

		if err == nil {
			for _, bl := range listeners_ {
				bl.ProcessEvent(evt)
			}
		}
	}

	// Notify of completion of the task
	output <- error(nil)
}

type Message struct {
	err      *IOError
	decoded  int
	blockId  int
	text     string
	checksum uint32
}

type semaphore chan bool

type CompressedInputStream struct {
	blockSize     uint
	hasher        *util.XXHash
	data          []byte
	buffers       [][]byte
	entropyType   byte
	transformType byte
	ibs           kanzi.InputBitStream
	debugWriter   io.Writer
	initialized   bool
	closed        bool
	blockId       int
	maxIdx        int
	curIdx        int
	jobs          int
	syncChan      []semaphore
	resChan       chan Message
	listeners     *list.List
}

func NewCompressedInputStream(is kanzi.InputStream,
	debugWriter io.Writer, jobs uint) (*CompressedInputStream, error) {
	if is == nil {
		return nil, errors.New("Invalid null input stream parameter")
	}

	if jobs < 1 || jobs > 16 {
		return nil, errors.New("The number of jobs must be in [1..16]")
	}

	this := new(CompressedInputStream)
	this.debugWriter = debugWriter
	this.jobs = int(jobs)
	this.blockId = 0
	this.data = EMPTY_BYTE_SLICE
	this.buffers = make([][]byte, jobs)

	for i := range this.buffers {
		this.buffers[i] = EMPTY_BYTE_SLICE
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

	this.listeners = list.New()
	return this, nil
}

func (this *CompressedInputStream) AddListener(bl BlockListener) bool {
	if bl == nil {
		return false
	}

	this.listeners.PushFront(bl)
	return true
}

func (this *CompressedInputStream) RemoveListener(bl BlockListener) bool {
	if bl == nil {
		return false
	}

	for e := this.listeners.Front(); e != nil; e = e.Next() {
		if e.Value == bl {
			this.listeners.Remove(e)
			return true
		}
	}

	return false
}

func (this *CompressedInputStream) ReadHeader() error {
	if this.initialized == true {
		return nil
	}

	fileType := uint64(0)
	var err error

	// Read stream type
	if fileType, err = this.ibs.ReadBits(32); err != nil {
		errMsg := fmt.Sprintf("Error reading stream type in header from input file: %v", err)
		return NewIOError(errMsg, ERR_READ_FILE)
	}

	// Sanity check
	if fileType != BITSTREAM_TYPE {
		errMsg := fmt.Sprintf("Invalid stream type: expected %#x, got %#x", BITSTREAM_TYPE, fileType)
		return NewIOError(errMsg, ERR_INVALID_FILE)
	}

	header := uint64(0)

	if header, err = this.ibs.ReadBits(48); err != nil {
		errMsg := fmt.Sprintf("Invalid bitstream, cannot read header: %v", err)
		return NewIOError(errMsg, ERR_READ_FILE)
	}

	version := int(header>>41) & 0x7F

	// Sanity check
	if version != BITSTREAM_FORMAT_VERSION {
		errMsg := fmt.Sprintf("Invalid bitstream, cannot read this version of the stream: %d", version)
		return NewIOError(errMsg, ERR_STREAM_VERSION)
	}

	// Read block checksum
	checksum := (header >> 40) & 1

	if checksum == 1 {
		if this.hasher, err = util.NewXXHash(BITSTREAM_TYPE); err != nil {
			return err
		}
	}

	// Read entropy codec
	this.entropyType = byte(header>>33) & 0x7F

	// Read transform
	this.transformType = byte(header>>26) & 0x7F

	// Read block size
	this.blockSize = uint(header & 0x03FFFFFF)

	if this.blockSize > MAX_BLOCK_SIZE {
		errMsg := fmt.Sprintf("Invalid bitstream, incorrect block size: %d", this.blockSize)
		return NewIOError(errMsg, ERR_BLOCK_SIZE)
	}

	if this.debugWriter != nil {
		fmt.Fprintf(this.debugWriter, "Checksum set to %v\n", (this.hasher != nil))
		fmt.Fprintf(this.debugWriter, "Block size set to %d bytes\n", this.blockSize)
		w1, err := function.GetByteFunctionName(this.transformType)

		if err != nil {
			errMsg := fmt.Sprintf("Invalid bitstream, unknown transform type: %d", this.transformType)
			return NewIOError(errMsg, ERR_INVALID_CODEC)
		}

		if w1 == "NONE" {
			w1 = "no"
		}

		fmt.Fprintf(this.debugWriter, "Using %v transform (stage 1)\n", w1)
		w2, err := entropy.GetEntropyCodecName(this.entropyType)

		if w2 == "NONE" {
			w2 = "no"
		}

		if err != nil {
			errMsg := fmt.Sprintf("Invalid bitstream, unknown entropy codec type: %d", this.entropyType)
			return NewIOError(errMsg, ERR_INVALID_CODEC)
		}

		fmt.Fprintf(this.debugWriter, "Using %v entropy codec (stage 2)\n", w2)
	}

	return nil
}

// Implement kanzi.InputStream interface
func (this *CompressedInputStream) Close() error {
	if this.closed == true {
		return nil
	}

	if _, err := this.ibs.Close(); err != nil {
		return err
	}

	this.closed = true

	// Release resources
	this.maxIdx = 0
	this.data = EMPTY_BYTE_SLICE

	for i := range this.buffers {
		this.buffers[i] = EMPTY_BYTE_SLICE
	}

	for _, c := range this.syncChan {
		if c != nil {
			close(c)
		}
	}

	close(this.resChan)
	this.listeners.Init()
	return nil
}

// Implement kanzi.InputStream interface
func (this *CompressedInputStream) Read(array []byte) (int, error) {
	if this.closed == true {
		return 0, NewIOError("Stream closed", ERR_READ_FILE)
	}

	startChunk := 0
	remaining := len(array)

	for remaining > 0 {
		if this.curIdx >= this.maxIdx {
			// Initially this.maxIdx = 0
			// If this.maxIdx < len(array), the end of stream has been
			// reached in the previous call
			if this.maxIdx > 0 && this.maxIdx < len(array) {
				if len(array) == remaining {
					// EOF and we did not read any bytes in this call
					return -1, nil
				} else {
					return len(array) - remaining, nil
				}
			}

			var err error

			// Buffer empty, time to decode
			if this.maxIdx, err = this.processBlock(); err != nil {
				return len(array) - remaining, err
			}

			if this.maxIdx == 0 {
				if len(array) == remaining {
					// EOF and we did not read any bytes in this call
					return -1, nil
				} else {
					return len(array) - remaining, nil
				}
			}
		}

		lenChunk := len(array) - startChunk

		if lenChunk+this.curIdx >= this.maxIdx {
			// Limit to number of available bytes in buffer
			lenChunk = this.maxIdx - this.curIdx
		}

		// Process a chunk of in-buffer data. No access to bitstream required
		copy(array[startChunk:], this.data[this.curIdx:this.curIdx+lenChunk])
		remaining -= lenChunk
		this.curIdx += lenChunk
		startChunk += lenChunk
	}

	return len(array) - remaining, nil
}

func (this *CompressedInputStream) processBlock() (int, error) {
	if this.initialized == false {
		if err := this.ReadHeader(); err != nil {
			return 0, err
		}

		this.initialized = true
	}

	// Initially this.maxIdx = 0
	// If this.maxIdx < len(this.data), the end of stream has been
	// reached in the previous call
	if this.maxIdx > 0 && this.maxIdx < len(this.data) {
		return 0, nil
	}

	if len(this.data) < int(this.blockSize)*this.jobs {
		this.data = make([]byte, this.jobs*int(this.blockSize))
	}

	blockNumber := this.blockId
	offset := uint(0)

	// Protect against future concurrent modification of the list of block listeners
	listeners_ := make([]BlockListener, this.listeners.Len())

	if this.listeners.Len() > 0 {
		n := 0

		for e := this.listeners.Front(); e != nil; e = e.Next() {
			listeners_[n] = e.Value.(BlockListener)
			n++
		}
	}

	// Invoke as many go routines as required
	for jobId := 0; jobId < this.jobs; jobId++ {
		blockNumber++
		curChan := this.syncChan[jobId]
		nextChan := this.syncChan[(jobId+1)%this.jobs]

		// Invoke the tasks concurrently
		// Tasks are daisy chained through channels. All tasks wait for a signal
		// on the input channel to start entropy decoding and then issue a message
		// to the next task on the output channel upon entropy decoding completion.
		// The transform step runs concurrently. The result is returned on the shared
		// channel. The output channel is nil for the last task and the input channel
		// is nil for the first task.
		go this.decode(this.data[offset:offset+this.blockSize], this.buffers[jobId],
			this.transformType, this.entropyType, blockNumber,
			curChan, nextChan, this.resChan, listeners_)

		offset += this.blockSize
	}

	err := error(nil)
	decoded := 0
	results := make([]Message, this.jobs)

	// Wait for completion of all concurrent tasks
	for i := 0; i < this.jobs; i++ {
		// Listen for results on the shared channel
		msg := <-this.resChan

		// Order the results based on block ID
		results[msg.blockId-this.blockId-1] = msg
	}

	// Process results
	for _, res := range results {
		if res.err != nil {
			if err != nil {
				// Keep first error encountered
				err = res.err
			}
		} else {
			// Add the number of decoded bytes for the current block
			decoded += res.decoded
		}

		if len(listeners_) > 0 {
			// Notify listeners after transform
			evt, err := NewBlockEvent(EVT_AFTER_TRANSFORM, res.blockId,
				res.decoded, res.checksum, this.hasher != nil)

			if err == nil {
				for _, bl := range listeners_ {
					bl.ProcessEvent(evt)
				}
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

func (this *CompressedInputStream) decode(data, buf []byte,
	typeOfTransform byte, typeOfEntropy byte, currentBlockId int,
	input, output chan bool, result chan Message,
	listeners_ []BlockListener) {
	buffer := buf
	res := Message{blockId: currentBlockId}

	// Wait for task processing the previous block to complete
	if input != nil {
		run := <-input

		// If one of the previous tasks failed, skip
		if run == false {
			notify(output, result, false, res)
			return
		}
	}

	// Extract header directly from bitstream
	read := this.ibs.Read()
	r, err := this.ibs.ReadBits(8)

	if err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = NewIOError(err.Error(), ERR_READ_FILE)
		notify(output, result, false, res)
		return
	}

	mode := byte(r)
	var compressedLength uint
	checksum1 := uint32(0)

	if (mode & SMALL_BLOCK_MASK) != 0 {
		compressedLength = uint(mode & COPY_LENGTH_MASK)
	} else {
		dataSize := uint(mode & 0x03)
		length := dataSize << 3
		mask := uint64(1<<length) - 1

		if r, err = this.ibs.ReadBits(length); err != nil {
			// Error => cancel concurrent decoding tasks
			res.err = NewIOError(err.Error(), ERR_READ_FILE)
			notify(output, result, false, res)
			return
		}

		compressedLength = uint(r & mask)
	}

	if compressedLength == 0 {
		// Last block is empty, return success and cancel pending tasks
		res.decoded = 0
		notify(output, result, false, res)
		return
	}

	if compressedLength > MAX_BLOCK_SIZE {
		// Error => cancel concurrent decoding tasks
		errMsg := fmt.Sprintf("Invalid compressed block length: %d", compressedLength)
		res.err = NewIOError(errMsg, ERR_BLOCK_SIZE)
		notify(output, result, false, res)
		return
	}

	// Extract checksum from bit stream (if any)
	if this.hasher != nil {
		if r, err = this.ibs.ReadBits(32); err != nil {
			// Error => cancel concurrent decoding tasks
			res.err = NewIOError(err.Error(), ERR_READ_FILE)
			notify(output, result, false, res)
			return
		}

		checksum1 = uint32(r)
	}

	if len(listeners_) > 0 {
		// Notify before entropy (block size in bitstream is unknown)
		evt, err := NewBlockEvent(EVT_BEFORE_ENTROPY, currentBlockId,
			-1, checksum1, this.hasher != nil)

		if err == nil {
			for _, bl := range listeners_ {
				bl.ProcessEvent(evt)
			}
		}
	}

	res.checksum = checksum1

	if this.transformType == 'N' {
		buffer = data // share buffers if no transform
	} else if len(buffer) < int(this.blockSize) {
		buffer = make([]byte, this.blockSize)
	}

	// Each block is decoded separately
	// Rebuild the entropy decoder to reset block statistics
	ed, err := entropy.NewEntropyDecoder(this.ibs, typeOfEntropy)

	if err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = NewIOError(err.Error(), ERR_INVALID_CODEC)
		notify(output, result, false, res)
		return
	}

	defer ed.Dispose()

	// Block entropy decode
	_, err = ed.Decode(buffer[0:compressedLength])

	if err != nil {
		// Error => cancel concurrent decoding tasks
		res.err = NewIOError(err.Error(), ERR_PROCESS_BLOCK)
		notify(output, result, false, res)
		return
	}

	if len(listeners_) > 0 {
		// Notify after entropy
		evt, err := NewBlockEvent(EVT_AFTER_ENTROPY, currentBlockId,
			int((this.ibs.Read()-read)/8), checksum1, this.hasher != nil)

		if err == nil {
			for _, bl := range listeners_ {
				bl.ProcessEvent(evt)
			}
		}
	}

	// After completion of the entropy decoding, unfreeze the task processing
	// the next block (if any)
	notify(output, nil, true, res)

	if len(listeners_) > 0 {
		// Notify before transform
		evt, err := NewBlockEvent(EVT_BEFORE_TRANSFORM, currentBlockId,
			int(compressedLength), checksum1, this.hasher != nil)

		if err == nil {
			for _, bl := range listeners_ {
				bl.ProcessEvent(evt)
			}
		}
	}

	read = this.ibs.Read() - read

	if ((mode & SMALL_BLOCK_MASK) != 0) || ((mode & SKIP_FUNCTION_MASK) != 0) {
		if !bytes.Equal(buffer, data) {
			copy(data, buffer[0:compressedLength])
		}

		res.decoded = int(compressedLength)
	} else {
		// Each block is decoded separately
		// Rebuild the entropy decoder to reset block statistics
		transform, err := function.NewByteFunction(compressedLength, typeOfTransform)

		if err != nil {
			// Error => return
			res.err = NewIOError(err.Error(), ERR_INVALID_CODEC)
			notify(nil, result, false, res)
			return
		}

		var oIdx uint

		// Inverse transform
		if _, oIdx, err = transform.Inverse(buffer, data); err != nil {
			// Error => return
			res.err = NewIOError(err.Error(), ERR_PROCESS_BLOCK)
			notify(nil, result, false, res)
			return
		}

		res.decoded = int(oIdx)

		// Verify checksum
		if this.hasher != nil {
			checksum2 := this.hasher.Hash(data[0:res.decoded])

			if checksum2 != checksum1 {
				errMsg := fmt.Sprintf("Corrupted bitstream: expected checksum %x, found %x", checksum1, checksum2)
				res.err = NewIOError(errMsg, ERR_PROCESS_BLOCK)
				notify(nil, result, false, res)
				return
			}
		}

	}

	notify(nil, result, false, res)
}
