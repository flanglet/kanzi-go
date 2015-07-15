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
	"kanzi"
)

// Null entropy encoder and decoder
// Pass through codec that writes the input bytes directly to the bitstream
type NullEntropyEncoder struct {
	bitstream kanzi.OutputBitStream
}

func NewNullEntropyEncoder(bs kanzi.OutputBitStream) (*NullEntropyEncoder, error) {
	this := new(NullEntropyEncoder)
	this.bitstream = bs
	return this, nil
}

func (this *NullEntropyEncoder) Encode(block []byte) (int, error) {
	len8 := len(block) & -8

	for i := 0; i < len8; i += 8 {
		val := uint64(block[i]) << 56
		val |= (uint64(block[i+1]) << 48)
		val |= (uint64(block[i+2]) << 40)
		val |= (uint64(block[i+3]) << 32)
		val |= (uint64(block[i+4]) << 24)
		val |= (uint64(block[i+5]) << 16)
		val |= (uint64(block[i+6]) << 8)
		val |= uint64(block[i+7])
		this.bitstream.WriteBits(val, 64)
	}

	for i := len8; i < len(block); i++ {
		this.bitstream.WriteBits(uint64(block[i]), 8)
	}

	return len(block), nil
}

func (this *NullEntropyEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

func (this *NullEntropyEncoder) Dispose() {
}

type NullEntropyDecoder struct {
	bitstream kanzi.InputBitStream
}

func NewNullEntropyDecoder(bs kanzi.InputBitStream) (*NullEntropyDecoder, error) {
	this := new(NullEntropyDecoder)
	this.bitstream = bs
	return this, nil
}

func (this *NullEntropyDecoder) Decode(block []byte) (int, error) {
	len8 := len(block) & -8

	for i := 0; i < len8; i += 8 {
		val := this.bitstream.ReadBits(64)
		block[i] = byte(val >> 56)
		block[i+1] = byte(val >> 48)
		block[i+2] = byte(val >> 40)
		block[i+3] = byte(val >> 32)
		block[i+4] = byte(val >> 24)
		block[i+5] = byte(val >> 16)
		block[i+6] = byte(val >> 8)
		block[i+7] = byte(val)
	}

	for i := len8; i < len(block); i++ {
		block[i] = byte(this.bitstream.ReadBits(8))
	}

	return len(block), nil
}

func (this *NullEntropyDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *NullEntropyDecoder) Dispose() {
}
