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
	BINARY_ENTROPY_TOP = uint64(0x00FFFFFFFFFFFFFF)
	MASK_24_56         = uint64(0x00FFFFFFFF000000)
	MASK_0_56          = uint64(0x00FFFFFFFFFFFFFF)
	MASK_0_24          = uint64(0x0000000000FFFFFF)
	MASK_0_32          = uint64(0x00000000FFFFFFFF)
)

type BinaryEntropyEncoder struct {
	predictor kanzi.Predictor
	low       uint64
	high      uint64
	bitstream kanzi.OutputBitStream
	disposed  bool
	buffer    []byte
	index     int
}

func NewBinaryEntropyEncoder(bs kanzi.OutputBitStream, predictor kanzi.Predictor) (*BinaryEntropyEncoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if predictor == nil {
		return nil, errors.New("Invalid null predictor parameter")
	}

	this := new(BinaryEntropyEncoder)
	this.predictor = predictor
	this.low = 0
	this.high = BINARY_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	return this, nil
}

func (this *BinaryEntropyEncoder) EncodeByte(val byte) {
	this.EncodeBit((val >> 7) & 1)
	this.EncodeBit((val >> 6) & 1)
	this.EncodeBit((val >> 5) & 1)
	this.EncodeBit((val >> 4) & 1)
	this.EncodeBit((val >> 3) & 1)
	this.EncodeBit((val >> 2) & 1)
	this.EncodeBit((val >> 1) & 1)
	this.EncodeBit(val & 1)
}

func (this *BinaryEntropyEncoder) EncodeBit(bit byte) {
	// Calculate interval split
	// Written in a way to maximize accuracy of multiplication/division
	split := (((this.high - this.low) >> 4) * uint64(this.predictor.Get())) >> 8

	// Update fields with new interval bounds
	b := -uint64(bit)
	this.high -= (b & (this.high - this.low - split))
	this.low += (^b & -^split)

	// Update predictor
	this.predictor.Update(bit)

	// Write unchanged first 32 bits to bitstream
	for (this.low^this.high)&MASK_24_56 == 0 {
		this.flush()
	}
}

func (this *BinaryEntropyEncoder) Encode(block []byte) (int, error) {
	count := len(block)

	if count > 1<<30 {
		return -1, errors.New("Invalid block size parameter (max is 1<<30)")
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

		if len(this.buffer) < (chunkSize*9)>>3 {
			this.buffer = make([]byte, (chunkSize*9)>>3)
		}

		this.index = 0
		buf := block[startChunk : startChunk+chunkSize]

		for i := range buf {
			this.EncodeByte(buf[i])
		}

		WriteVarInt(this.bitstream, this.index)
		this.bitstream.WriteArray(this.buffer, uint(8*this.index))
		startChunk += chunkSize

		if startChunk < end {
			this.bitstream.WriteBits(this.low|MASK_0_24, 56)
		}
	}

	return count, err
}

func (this *BinaryEntropyEncoder) flush() {
	binary.BigEndian.PutUint32(this.buffer[this.index:], uint32(this.high>>24))
	this.index += 4
	this.low <<= 32
	this.high = (this.high << 32) | MASK_0_32
}

func (this *BinaryEntropyEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

func (this *BinaryEntropyEncoder) Dispose() {
	if this.disposed == true {
		return
	}

	this.disposed = true
	this.bitstream.WriteBits(this.low|MASK_0_24, 56)
}

type BinaryEntropyDecoder struct {
	predictor   kanzi.Predictor
	low         uint64
	high        uint64
	current     uint64
	initialized bool
	bitstream   kanzi.InputBitStream
	buffer      []byte
	index       int
}

func NewBinaryEntropyDecoder(bs kanzi.InputBitStream, predictor kanzi.Predictor) (*BinaryEntropyDecoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if predictor == nil {
		return nil, errors.New("Invalid null predictor parameter")
	}

	// Defer stream reading. We are creating the object, we should not do any I/O
	this := new(BinaryEntropyDecoder)
	this.predictor = predictor
	this.low = 0
	this.high = BINARY_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	return this, nil
}

func (this *BinaryEntropyDecoder) DecodeByte() byte {
	return (this.DecodeBit() << 7) |
		(this.DecodeBit() << 6) |
		(this.DecodeBit() << 5) |
		(this.DecodeBit() << 4) |
		(this.DecodeBit() << 3) |
		(this.DecodeBit() << 2) |
		(this.DecodeBit() << 1) |
		this.DecodeBit()
}

func (this *BinaryEntropyDecoder) Initialized() bool {
	return this.initialized
}

func (this *BinaryEntropyDecoder) Initialize() {
	if this.initialized == true {
		return
	}

	this.current = this.bitstream.ReadBits(56)
	this.initialized = true
}

func (this *BinaryEntropyDecoder) DecodeBit() byte {
	// Calculate interval split
	// Written in a way to maximize accuracy of multiplication/division
	split := ((((this.high - this.low) >> 4) * uint64(this.predictor.Get())) >> 8) + this.low
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
	for (this.low^this.high)&MASK_24_56 == 0 {
		this.read()
	}

	return bit
}

func (this *BinaryEntropyDecoder) read() {
	this.low = (this.low << 32) & MASK_0_56
	this.high = ((this.high << 32) | MASK_0_32) & MASK_0_56
	val := uint64(binary.BigEndian.Uint32(this.buffer[this.index:])) & 0xFFFFFFFF
	this.current = ((this.current << 32) | val) & MASK_0_56
	this.index += 4
}

func (this *BinaryEntropyDecoder) Decode(block []byte) (int, error) {
	count := len(block)

	if count > 1<<30 {
		return -1, errors.New("Invalid block size parameter (max is 1<<30)")
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
		this.bitstream.ReadArray(this.buffer, uint(8*szBytes))
		this.index = 0
		buf := block[startChunk : startChunk+chunkSize]

		for i := range buf {
			buf[i] = this.DecodeByte()
		}

		startChunk += chunkSize
	}

	return count, err
}

func (this *BinaryEntropyDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *BinaryEntropyDecoder) Dispose() {
}
