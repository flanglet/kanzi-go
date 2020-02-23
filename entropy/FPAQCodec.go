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

package entropy

import (
	"encoding/binary"
	"errors"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	_FPAQ_TOP       = uint64(0x00FFFFFFFFFFFFFF)
	_FPAQ_MASK_0_56 = uint64(0x00FFFFFFFFFFFFFF)
	_FPAQ_MASK_0_24 = uint64(0x0000000000FFFFFF)
	_FPAQ_MASK_0_32 = uint64(0x00000000FFFFFFFF)
	_FPAQ_PSCALE    = 1 << 16
)

// FPAQEncoder entropy encoder derived from fpaq0r by Matt Mahoney & Alexander Ratushnyak.
// See http://mattmahoney.net/dc/#fpaq0.
// Simple (and fast) adaptive order 0 entropy coder
type FPAQEncoder struct {
	low       uint64
	high      uint64
	bitstream kanzi.OutputBitStream
	disposed  bool
	buffer    []byte
	index     int
	probs     [256]int // probability of bit=1
	ctxIdx    byte     // previous bits
}

// NewFPAQEncoder creates an instance of FPAQEncoder
func NewFPAQEncoder(bs kanzi.OutputBitStream) (*FPAQEncoder, error) {
	if bs == nil {
		return nil, errors.New("FPAQ codec: Invalid null bitstream parameter")
	}

	this := new(FPAQEncoder)
	this.low = 0
	this.high = _BINARY_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	this.ctxIdx = 1

	for i := range this.probs {
		this.probs[i] = _FPAQ_PSCALE >> 1
	}

	return this, nil
}

// EncodeByte encodes the given value into the bitstream bit by bit
func (this *FPAQEncoder) EncodeByte(val byte) {
	bits := int(val) + 256
	this.encodeBit(val&0x80, 1)
	this.encodeBit(val&0x40, bits>>7)
	this.encodeBit(val&0x20, bits>>6)
	this.encodeBit(val&0x10, bits>>5)
	this.encodeBit(val&0x08, bits>>4)
	this.encodeBit(val&0x04, bits>>3)
	this.encodeBit(val&0x02, bits>>2)
	this.encodeBit(val&0x01, bits>>1)
}

// encodeBit encodes one bit
func (this *FPAQEncoder) encodeBit(bit byte, pIdx int) {
	// Calculate interval split
	// Written in a way to maximize accuracy of multiplication/division
	split := (((this.high - this.low) >> 4) * uint64(this.probs[pIdx]>>4)) >> 8

	// Update probabilities
	if bit == 0 {
		this.low += (split + 1)
		this.probs[pIdx] -= (this.probs[pIdx] >> 6)
	} else {
		this.high = this.low + split
		this.probs[pIdx] -= (((this.probs[pIdx] - _FPAQ_PSCALE) >> 6) + 1)
	}

	// Write unchanged first 32 bits to bitstream
	for (this.low^this.high)>>24 == 0 {
		this.flush()
	}
}

// Write encodes the data provided into the bitstream. Return the number of byte
// written to the bitstream. Splits big blocks into chunks and encode the chunks
// byte by byte sequentially into the bitstream.
func (this *FPAQEncoder) Write(block []byte) (int, error) {
	count := len(block)

	if count > 1<<30 {
		return -1, errors.New("FPAQ codec: Invalid block size parameter (max is 1<<30)")
	}

	startChunk := 0
	end := count
	length := count
	err := error(nil)

	if count >= 1<<26 {
		// If the block is big (>=64MB), split the encoding to avoid allocating
		// too much memory.
		if count < 1<<29 {
			length = count >> 3
		} else {
			length = count >> 4
		}
	} else if count < 64 {
		length = 64
	}

	// Split block into chunks, read bit array from bitstream and decode chunk
	for startChunk < end {
		chunkSize := length

		if startChunk+length >= end {
			chunkSize = end - startChunk
		}

		if len(this.buffer) < (chunkSize + (chunkSize >> 3)) {
			this.buffer = make([]byte, chunkSize+(chunkSize>>3))
		}

		this.index = 0
		buf := block[startChunk : startChunk+chunkSize]

		for i := range buf {
			this.EncodeByte(buf[i])
		}

		WriteVarInt(this.bitstream, uint32(this.index))
		this.bitstream.WriteArray(this.buffer, uint(8*this.index))
		startChunk += chunkSize

		if startChunk < end {
			this.bitstream.WriteBits(this.low|_MASK_0_24, 56)
		}
	}

	return count, err
}

func (this *FPAQEncoder) flush() {
	binary.BigEndian.PutUint32(this.buffer[this.index:], uint32(this.high>>24))
	this.index += 4
	this.low <<= 32
	this.high = (this.high << 32) | _MASK_0_32
}

// BitStream returns the underlying bitstream
func (this *FPAQEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// Dispose must be called before getting rid of the entropy encoder
// This idempotent implmentation writes the last buffered bits into the
// bitstream.
func (this *FPAQEncoder) Dispose() {
	if this.disposed == true {
		return
	}

	this.disposed = true
	this.bitstream.WriteBits(this.low|_MASK_0_24, 56)
}

// FPAQDecoder entropy decoder derived from fpaq0r by Matt Mahoney & Alexander Ratushnyak.
// See http://mattmahoney.net/dc/#fpaq0.
// Simple (and fast) adaptive order 0 entropy coder
type FPAQDecoder struct {
	low         uint64
	high        uint64
	current     uint64
	initialized bool
	bitstream   kanzi.InputBitStream
	buffer      []byte
	index       int
	probs       [256]int // probability of bit=1
	ctx         byte     // previous bits
}

// NewFPAQDecoder creates an instance of FPAQDecoder
func NewFPAQDecoder(bs kanzi.InputBitStream) (*FPAQDecoder, error) {
	if bs == nil {
		return nil, errors.New("FPAQ codec: Invalid null bitstream parameter")
	}

	// Defer stream reading. We are creating the object, we should not do any I/O
	this := new(FPAQDecoder)
	this.low = 0
	this.high = _BINARY_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	this.ctx = 1

	for i := range this.probs {
		this.probs[i] = _FPAQ_PSCALE >> 1
	}

	return this, nil
}

// DecodeByte decodes the given value from the bitstream bit by bit
func (this *FPAQDecoder) DecodeByte() byte {
	this.ctx = 1
	this.decodeBit(this.probs[this.ctx] >> 4)
	this.decodeBit(this.probs[this.ctx] >> 4)
	this.decodeBit(this.probs[this.ctx] >> 4)
	this.decodeBit(this.probs[this.ctx] >> 4)
	this.decodeBit(this.probs[this.ctx] >> 4)
	this.decodeBit(this.probs[this.ctx] >> 4)
	this.decodeBit(this.probs[this.ctx] >> 4)
	this.decodeBit(this.probs[this.ctx] >> 4)
	return byte(this.ctx)
}

// Initialized returns true if Initialize() has been called at least once
func (this *FPAQDecoder) Initialized() bool {
	return this.initialized
}

// Initialize initializes the decoder by prefetching the first bits
// and saving them into a buffer. This code is idempotent.
func (this *FPAQDecoder) Initialize() {
	if this.initialized == true {
		return
	}

	this.current = this.bitstream.ReadBits(56)
	this.initialized = true
}

// decodeBit decodes one bit
func (this *FPAQDecoder) decodeBit(pred int) byte {
	// Calculate interval split
	// Written in a way to maximize accuracy of multiplication/division
	split := ((((this.high - this.low) >> 4) * uint64(pred)) >> 8) + this.low
	var bit byte

	// Update probabilities
	if split >= this.current {
		bit = 1
		this.high = split
		this.probs[this.ctx] -= (((this.probs[this.ctx] - _FPAQ_PSCALE) >> 6) + 1)
		this.ctx += (this.ctx + 1)
	} else {
		bit = 0
		this.low = -^split
		this.probs[this.ctx] -= (this.probs[this.ctx] >> 6)
		this.ctx += this.ctx
	}

	// Read 32 bits from bitstream
	for (this.low^this.high)>>24 == 0 {
		this.read()
	}

	return bit
}

func (this *FPAQDecoder) read() {
	this.low = (this.low << 32) & _MASK_0_56
	this.high = ((this.high << 32) | _MASK_0_32) & _MASK_0_56
	val := uint64(binary.BigEndian.Uint32(this.buffer[this.index:]))
	this.current = ((this.current << 32) | val) & _MASK_0_56
	this.index += 4
}

// Read decodes data from the bitstream and return it in the provided buffer.
// Return the number of bytes read from the bitstream.
// Splits big blocks into chunks and decode the chunks byte by byte sequentially from the bitstream.
func (this *FPAQDecoder) Read(block []byte) (int, error) {
	count := len(block)

	if count > 1<<30 {
		return -1, errors.New("FPAQ codec: Invalid block size parameter (max is 1<<30)")
	}

	startChunk := 0
	end := count
	length := count
	err := error(nil)

	if count >= 1<<26 {
		// If the block is big (>=64MB), split the decoding to avoid allocating
		// too much memory.
		if count < 1<<29 {
			length = count >> 3
		} else {
			length = count >> 4
		}
	} else if count < 64 {
		length = 64
	}

	// Split block into chunks, read bit array from bitstream and decode chunk
	for startChunk < end {
		chunkSize := length

		if startChunk+length >= end {
			chunkSize = end - startChunk
		}

		if len(this.buffer) < (chunkSize*9)>>3 {
			this.buffer = make([]byte, (chunkSize*9)>>3)
		}

		szBytes := ReadVarInt(this.bitstream)
		this.current = this.bitstream.ReadBits(56)
		this.initialized = true

		if szBytes != 0 {
			this.bitstream.ReadArray(this.buffer, uint(8*szBytes))
		}

		this.index = 0
		buf := block[startChunk : startChunk+chunkSize]

		for i := range buf {
			buf[i] = this.DecodeByte()
		}

		startChunk += chunkSize
	}

	return count, err
}

// BitStream returns the underlying bitstream
func (this *FPAQDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Dispose must be called before getting rid of the entropy decoder
// This implementation does nothing.
func (this *FPAQDecoder) Dispose() {
}
