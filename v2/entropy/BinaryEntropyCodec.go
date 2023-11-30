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

package entropy

import (
	"encoding/binary"
	"errors"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

const (
	_BINARY_ENTROPY_TOP       = uint64(0x00FFFFFFFFFFFFFF)
	_MASK_0_56                = uint64(0x00FFFFFFFFFFFFFF)
	_MASK_0_24                = uint64(0x0000000000FFFFFF)
	_MASK_0_32                = uint64(0x00000000FFFFFFFF)
	_BINARY_ENTROPY_MAX_BLOCK = 1 << 30
	_BINARY_ENTROPY_MAX_CHUNK = 1 << 26
)

// BinaryEntropyEncoder entropy encoder based on arithmetic coding and
// using an external probability predictor.
type BinaryEntropyEncoder struct {
	predictor kanzi.Predictor
	low       uint64
	high      uint64
	bitstream kanzi.OutputBitStream
	disposed  bool
	buffer    []byte
	index     int
}

// NewBinaryEntropyEncoder creates an instance of BinaryEntropyEncoder using the
// given predictor to predict the probability of the next bit to be one. It outputs
// to the given OutputBitstream
func NewBinaryEntropyEncoder(bs kanzi.OutputBitStream, predictor kanzi.Predictor) (*BinaryEntropyEncoder, error) {
	if bs == nil {
		return nil, errors.New("Binary entropy codec: Invalid null bitstream parameter")
	}

	if predictor == nil {
		return nil, errors.New("Binary entropy codec: Invalid null predictor parameter")
	}

	this := &BinaryEntropyEncoder{}
	this.predictor = predictor
	this.low = 0
	this.high = _BINARY_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	return this, nil
}

// EncodeByte encodes the given value into the bitstream bit by bit
func (this *BinaryEntropyEncoder) EncodeByte(val byte) {
	this.EncodeBit((val>>7)&1, this.predictor.Get())
	this.EncodeBit((val>>6)&1, this.predictor.Get())
	this.EncodeBit((val>>5)&1, this.predictor.Get())
	this.EncodeBit((val>>4)&1, this.predictor.Get())
	this.EncodeBit((val>>3)&1, this.predictor.Get())
	this.EncodeBit((val>>2)&1, this.predictor.Get())
	this.EncodeBit((val>>1)&1, this.predictor.Get())
	this.EncodeBit(val&1, this.predictor.Get())
}

// EncodeBit encodes one bit into the bitstream using arithmetic coding
// and the probability predictor provided at creation time.
func (this *BinaryEntropyEncoder) EncodeBit(bit byte, pred int) {
	// Calculate interval split
	// Written in a way to maximize accuracy of multiplication/division
	split := (((this.high - this.low) >> 4) * uint64(pred)) >> 8

	// Update fields with new interval bounds
	if bit == 0 {
		this.low += (split + 1)
	} else {
		this.high = this.low + split
	}

	this.predictor.Update(bit)

	// Write unchanged first 32 bits to bitstream
	for (this.low ^ this.high) < (1 << 24) {
		this.flush()
	}
}

// Write encodes the data provided into the bitstream. Return the number of byte
// written to the bitstream. Splits big blocks into chunks and encode the chunks
// byte by byte sequentially into the bitstream.
func (this *BinaryEntropyEncoder) Write(block []byte) (int, error) {
	count := len(block)

	if count > _BINARY_ENTROPY_MAX_BLOCK {
		return -1, errors.New("Binary entropy codec: Invalid block size parameter (max is 1<<30)")
	}

	startChunk := 0
	end := count
	length := count
	err := error(nil)

	if count >= _BINARY_ENTROPY_MAX_CHUNK {
		// If the block is big (>=64MB), split the encoding to avoid allocating
		// too much memory.
		if count < 8*_BINARY_ENTROPY_MAX_CHUNK {
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

func (this *BinaryEntropyEncoder) flush() {
	binary.BigEndian.PutUint32(this.buffer[this.index:], uint32(this.high>>24))
	this.index += 4
	this.low <<= 32
	this.high = (this.high << 32) | _MASK_0_32
}

// BitStream returns the underlying bitstream
func (this *BinaryEntropyEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// Dispose must be called before getting rid of the entropy encoder
// This idempotent implementation writes the last buffered bits into the
// bitstream.
func (this *BinaryEntropyEncoder) Dispose() {
	if this.disposed == true {
		return
	}

	this.disposed = true
	this.bitstream.WriteBits(this.low|_MASK_0_24, 56)
}

// BinaryEntropyDecoder entropy decoder based on arithmetic coding and
// using an external probability predictor.
type BinaryEntropyDecoder struct {
	predictor kanzi.Predictor
	low       uint64
	high      uint64
	current   uint64
	bitstream kanzi.InputBitStream
	buffer    []byte
	index     int
}

// NewBinaryEntropyDecoder creates an instance of BinaryEntropyDecoder using the
// given predictor to predict the probability of the next bit to be one. It outputs
// to the given OutputBitstream
func NewBinaryEntropyDecoder(bs kanzi.InputBitStream, predictor kanzi.Predictor) (*BinaryEntropyDecoder, error) {
	if bs == nil {
		return nil, errors.New("Binary entropy codec: Invalid null bitstream parameter")
	}

	if predictor == nil {
		return nil, errors.New("Binary entropy codec: Invalid null predictor parameter")
	}

	// Defer stream reading. We are creating the object, we should not do any I/O
	this := &BinaryEntropyDecoder{}
	this.predictor = predictor
	this.low = 0
	this.high = _BINARY_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	return this, nil
}

// DecodeByte decodes the given value from the bitstream bit by bit
func (this *BinaryEntropyDecoder) DecodeByte() byte {
	return (this.DecodeBit(this.predictor.Get()) << 7) |
		(this.DecodeBit(this.predictor.Get()) << 6) |
		(this.DecodeBit(this.predictor.Get()) << 5) |
		(this.DecodeBit(this.predictor.Get()) << 4) |
		(this.DecodeBit(this.predictor.Get()) << 3) |
		(this.DecodeBit(this.predictor.Get()) << 2) |
		(this.DecodeBit(this.predictor.Get()) << 1) |
		this.DecodeBit(this.predictor.Get())
}

// DecodeBit decodes one bit from the bitstream using arithmetic coding
// and the probability predictor provided at creation time.
func (this *BinaryEntropyDecoder) DecodeBit(pred int) byte {
	// Calculate interval split
	// Written in a way to maximize accuracy of multiplication/division
	split := ((((this.high - this.low) >> 4) * uint64(pred)) >> 8) + this.low
	var bit byte

	// Update predictor
	if split >= this.current {
		bit = 1
		this.high = split
		this.predictor.Update(1)
	} else {
		bit = 0
		this.low = -^split
		this.predictor.Update(0)
	}

	// Read 32 bits from bitstream
	for (this.low ^ this.high) < (1 << 24) {
		this.read()
	}

	return bit
}

func (this *BinaryEntropyDecoder) read() {
	this.low = (this.low << 32) & _MASK_0_56
	this.high = ((this.high << 32) | _MASK_0_32) & _MASK_0_56
	val := uint64(binary.BigEndian.Uint32(this.buffer[this.index:]))
	this.current = ((this.current << 32) | val) & _MASK_0_56
	this.index += 4
}

// Read decodes data from the bitstream and return it in the provided buffer.
// Return the number of bytes read from the bitstream.
// Splits big blocks into chunks and decode the chunks byte by byte sequentially from the bitstream.
func (this *BinaryEntropyDecoder) Read(block []byte) (int, error) {
	count := len(block)

	if count > _BINARY_ENTROPY_MAX_BLOCK {
		return -1, errors.New("Binary entropy codec: Invalid block size parameter (max is 1<<30)")
	}

	startChunk := 0
	end := count
	length := count
	err := error(nil)

	if count >= _BINARY_ENTROPY_MAX_CHUNK {
		// If the block is big (>=64MB), split the decoding to avoid allocating
		// too much memory.
		if count < 8*_BINARY_ENTROPY_MAX_CHUNK {
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

		if len(this.buffer) < chunkSize+(chunkSize>>3) {
			this.buffer = make([]byte, chunkSize+(chunkSize>>3))
		}

		szBytes := ReadVarInt(this.bitstream)
		this.current = this.bitstream.ReadBits(56)

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
func (this *BinaryEntropyDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Dispose must be called before getting rid of the entropy decoder
// This implementation does nothing.
func (this *BinaryEntropyDecoder) Dispose() {
}
