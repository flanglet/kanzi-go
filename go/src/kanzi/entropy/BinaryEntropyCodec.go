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
	"errors"
	"kanzi"
)

const (
	BINARY_ENTROPY_TOP = uint64(0x00FFFFFFFFFFFFFF)
	MASK_24_56         = uint64(0x00FFFFFFFF000000)
	MASK_0_24          = uint64(0x0000000000FFFFFF)
	MASK_0_32          = uint64(0x00000000FFFFFFFF)
)

type Predictor interface {
	// Update the probability model
	Update(bit byte)

	// Return the split value representing the probability of 1 in the [0..4095] range.
	// E.G. 410 represents roughly a probability of 10% for 1
	Get() int
}

type BinaryEntropyEncoder struct {
	predictor Predictor
	low       uint64
	high      uint64
	bitstream kanzi.OutputBitStream
	disposed  bool
}

func NewBinaryEntropyEncoder(bs kanzi.OutputBitStream, predictor Predictor) (*BinaryEntropyEncoder, error) {
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
	this.low += (^b & (split + 1))

	// Update predictor
	this.predictor.Update(bit)

	// Write unchanged first 32 bits to bitstream
	for (this.low^this.high)&MASK_24_56 == 0 {
		this.flush()
	}
}

func (this *BinaryEntropyEncoder) Encode(block []byte) (int, error) {
	for i := range block {
		this.EncodeByte(block[i])
	}

	return len(block), nil
}

func (this *BinaryEntropyEncoder) flush() {
	this.bitstream.WriteBits(this.high>>24, 32)
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
	predictor   Predictor
	low         uint64
	high        uint64
	current     uint64
	initialized bool
	bitstream   kanzi.InputBitStream
}

func NewBinaryEntropyDecoder(bs kanzi.InputBitStream, predictor Predictor) (*BinaryEntropyDecoder, error) {
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
	return this, nil
}

func (this *BinaryEntropyDecoder) DecodeByte() byte {
	return byte((this.DecodeBit() << 7) |
		(this.DecodeBit() << 6) |
		(this.DecodeBit() << 5) |
		(this.DecodeBit() << 4) |
		(this.DecodeBit() << 3) |
		(this.DecodeBit() << 2) |
		(this.DecodeBit() << 1) |
		this.DecodeBit())
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

	if split >= this.current {
	    bit = 1
		this.high = split
	} else {
	    bit = 0
		this.low = split + 1
	}

	// Update predictor
	this.predictor.Update(bit)

	// Read 32 bits from bitstream
	for (this.low^this.high)&MASK_24_56 == 0 {
		this.read()
	}

	return bit
}

func (this *BinaryEntropyDecoder) read() {
	this.low = this.low << 32
	this.high = (this.high << 32) | MASK_0_32
	this.current = (this.current << 32) | this.bitstream.ReadBits(32)
}

func (this *BinaryEntropyDecoder) Decode(block []byte) (int, error) {
	err := error(nil)

	// Deferred initialization: the bitstream may not be ready at build time
	// Initialize 'current' with bytes read from the bitstream
	if this.Initialized() == false {
		this.Initialize()
	}

	for i := range block {
		block[i] = this.DecodeByte()
	}

	return len(block), err
}

func (this *BinaryEntropyDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *BinaryEntropyDecoder) Dispose() {
}
