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
	"errors"
	"fmt"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

// RiceGolombEncoder a Rice Golomb Entropy Encoder
type RiceGolombEncoder struct {
	signed    bool
	logBase   uint
	base      uint64
	bitstream kanzi.OutputBitStream
}

// NewRiceGolombEncoder creates a new instance of RiceGolombEncoder
// If sgn is true, values will be encoded as signed (int8) in the bitstream.
// Using a sign improves compression ratio for distributions centered on 0 (E.G. Gaussian)
// Example: -1 is better compressed as -1 (1 followed by '-') than as 255
func NewRiceGolombEncoder(bs kanzi.OutputBitStream, sgn bool, logBase uint) (*RiceGolombEncoder, error) {
	if bs == nil {
		return nil, errors.New("RiceGolomb codec: Invalid null bitstream parameter")
	}

	if logBase < 1 || logBase > 12 {
		return nil, fmt.Errorf("RiceGolomb codec: Invalid logBase '%v' value (must be in [1..12])", logBase)
	}

	this := &RiceGolombEncoder{}
	this.signed = sgn
	this.bitstream = bs
	this.logBase = logBase
	this.base = uint64(1 << logBase)
	return this, nil
}

// Signed returns true if this encoder is sign aware
func (this *RiceGolombEncoder) Signed() bool {
	return this.signed
}

// Dispose this implementation does nothing
func (this *RiceGolombEncoder) Dispose() {
}

// EncodeByte encodes the given value into the bitstream
func (this *RiceGolombEncoder) EncodeByte(val byte) {
	if val == 0 {
		this.bitstream.WriteBits(this.base, this.logBase+1)
		return
	}

	var emit uint64

	if this.signed == true && val&0x80 != 0 {
		emit = uint64(-val)
	} else {
		emit = uint64(val)
	}

	// quotient is unary encoded, remainder is binary encoded
	n := uint(emit>>this.logBase) + this.logBase + 1
	emit = this.base | (emit & (this.base - 1))

	if this.signed == true {
		// Add 0 for positive and 1 for negative sign (considering
		// msb as byte 'sign')
		n++
		emit = (emit << 1) | uint64((val>>7)&1)
	}

	this.bitstream.WriteBits(emit, n)
}

// BitStream returns the underlying bitstream
func (this *RiceGolombEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// Write encodes the data provided into the bitstream. Return the number of byte
// written to the bitstream
func (this *RiceGolombEncoder) Write(block []byte) (int, error) {
	for i := range block {
		this.EncodeByte(block[i])
	}

	return len(block), nil
}

// RiceGolombDecoder Exponential Golomb Entropy Decoder
type RiceGolombDecoder struct {
	signed    bool
	logBase   uint
	bitstream kanzi.InputBitStream
}

// NewRiceGolombDecoder creates a new instance of RiceGolombDecoder
// If sgn is true, values from the bitstream will be decoded as signed (int8)
func NewRiceGolombDecoder(bs kanzi.InputBitStream, sgn bool, logBase uint) (*RiceGolombDecoder, error) {
	if bs == nil {
		return nil, errors.New("RiceGolomb codec: Invalid null bitstream parameter")
	}

	if logBase < 1 || logBase > 12 {
		return nil, errors.New("RiceGolomb codec: Invalid logBase value (must be in [1..12])")
	}

	this := &RiceGolombDecoder{}
	this.signed = sgn
	this.bitstream = bs
	this.logBase = logBase
	return this, nil
}

// Signed returns true if this decoder is sign aware
func (this *RiceGolombDecoder) Signed() bool {
	return this.signed
}

// Dispose this implementation does nothing
func (this *RiceGolombDecoder) Dispose() {
}

// DecodeByte decodes one byte from the bitstream
// If the decoder is sign aware, the returned value is an int8 cast to a byte
func (this *RiceGolombDecoder) DecodeByte() byte {
	q := 0

	// quotient is unary encoded
	for this.bitstream.ReadBit() == 0 {
		q++
	}

	// remainder is binary encoded
	res := byte((q << this.logBase) | int(this.bitstream.ReadBits(this.logBase)))

	if this.signed == true && res != 0 {
		// If res != 0, Get the 'sign', encoded as 1 for negative values
		if this.bitstream.ReadBit() == 1 {
			return -res
		}
	}

	return res
}

// BitStream returns the underlying bitstream
func (this *RiceGolombDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Read decodes data from the bitstream and return it in the provided buffer.
// Return the number of bytes read from the bitstream
func (this *RiceGolombDecoder) Read(block []byte) (int, error) {
	for i := range block {
		block[i] = this.DecodeByte()
	}

	return len(block), nil
}
