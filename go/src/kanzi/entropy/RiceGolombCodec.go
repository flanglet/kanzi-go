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

package entropy

import (
	"errors"
	"fmt"
	"kanzi"
)

type RiceGolombEncoder struct {
	signed    bool
	logBase   uint64
	base      uint64
	bitstream kanzi.OutputBitStream
}

// If sgn is true, the input value is turned into an int8
// Managing sign improves compression ratio for distributions centered on 0 (E.G. Gaussian)
// Example: -1 is better compressed as int8 (1 followed by -) than as byte (-1 & 255 = 255)
func NewRiceGolombEncoder(bs kanzi.OutputBitStream, sgn bool, logBase uint) (*RiceGolombEncoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if logBase <= 0 || logBase >= 8 {
		return nil, fmt.Errorf("Invalid logBase '%v' value (must be in [1..7])", logBase)
	}

	this := new(RiceGolombEncoder)
	this.signed = sgn
	this.bitstream = bs
	this.logBase = uint64(logBase)
	this.base = uint64(1 << logBase)
	return this, nil
}

func (this *RiceGolombEncoder) Signed() bool {
	return this.signed
}

func (this *RiceGolombEncoder) Dispose() {
}

func (this *RiceGolombEncoder) EncodeByte(val byte) {
	if val == 0 {
		this.bitstream.WriteBits(this.base, uint(this.logBase+1))
		return
	}

	var emit uint64

	if this.signed == false || val&0x80 == 0 {
		emit = uint64(val)
	} else {
		emit = uint64(^val) + 1
	}

	// quotient is unary encoded, remainder is binary encoded
	n := uint(1 + (emit >> this.logBase) + this.logBase)
	emit = this.base | (emit & (this.base - 1))

	if this.signed == true {
		// Add 0 for positive and 1 for negative sign (considering
		// msb as byte 'sign')
		n++
		emit = (emit << 1) | uint64((val>>7)&1)
	}

	this.bitstream.WriteBits(emit, n)
}

func (this *RiceGolombEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

func (this *RiceGolombEncoder) Encode(block []byte) (int, error) {
	for i := range block {
		this.EncodeByte(block[i])
	}

	return len(block), nil
}

type RiceGolombDecoder struct {
	signed    bool
	logBase   uint
	bitstream kanzi.InputBitStream
}

// If sgn is true, the extracted value is treated as an int8
func NewRiceGolombDecoder(bs kanzi.InputBitStream, sgn bool, logBase uint) (*RiceGolombDecoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if logBase <= 0 || logBase >= 8 {
		return nil, errors.New("Invalid logBase value (must be in [1..7])")
	}

	this := new(RiceGolombDecoder)
	this.signed = sgn
	this.bitstream = bs
	this.logBase = logBase
	return this, nil
}
func (this *RiceGolombDecoder) Signed() bool {
	return this.signed
}

func (this *RiceGolombDecoder) Dispose() {
}

// If the decoder is signed, the returned value is a byte encoded int8
func (this *RiceGolombDecoder) DecodeByte() byte {
	q := 0

	// quotient is unary encoded
	for this.bitstream.ReadBit() == 0 {
		q++
	}

	// remainder is binary encoded
	res := (q << this.logBase) | int(this.bitstream.ReadBits(this.logBase))

	if res != 0 && this.signed == true {
		// If res != 0, Get the 'sign', encoded as 1 for negative values
		if this.bitstream.ReadBit() == 1 {
			return byte(^res + 1)
		}
	}

	return byte(res)
}

func (this *RiceGolombDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *RiceGolombDecoder) Decode(block []byte) (int, error) {
	for i := range block {
		block[i] = this.DecodeByte()
	}

	return len(block), nil
}