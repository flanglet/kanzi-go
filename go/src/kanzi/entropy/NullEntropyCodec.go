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
// Pass through that writes the data directly to the bitstream
type NullEntropyEncoder struct {
	bitstream kanzi.OutputBitStream
}

func NewNullEntropyEncoder(bs kanzi.OutputBitStream) (*NullEntropyEncoder, error) {
	this := new(NullEntropyEncoder)
	this.bitstream = bs
	return this, nil
}

func (this *NullEntropyEncoder) Encode(block []byte) (int, error) {
	err := error(nil)
	len8 := len(block) & -8

	for i := 0; i < len8; i += 8 {
		if err = this.encodeLong(block, i); err != nil {
			return i, err
		}
	}

	for i := len8; i < len(block); i++ {
		if err = this.EncodeByte(block[i]); err != nil {
			return i, err
		}
	}

	return len(block), nil
}

func (this *NullEntropyEncoder) EncodeByte(val byte) error {
	_, err := this.bitstream.WriteBits(uint64(val), 8)
	return err
}

func (this *NullEntropyEncoder) encodeLong(block []byte, offset int) error {
	val := uint64(block[offset]) << 56
	val |= (uint64(block[offset+1]) << 48)
	val |= (uint64(block[offset+2]) << 40)
	val |= (uint64(block[offset+3]) << 32)
	val |= (uint64(block[offset+4]) << 24)
	val |= (uint64(block[offset+5]) << 16)
	val |= (uint64(block[offset+6]) << 8)
	val |= uint64(block[offset+7])
	_, err := this.bitstream.WriteBits(val, 64)
	return err
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
	err := error(nil)
	len8 := len(block) & -8

	for i := 0; i < len8; i += 8 {
		if err = this.decodeLong(block, i); err != nil {
			return i, err
		}

	}

	for i := len8; i < len(block); i++ {
		if block[i], err = this.DecodeByte(); err != nil {
			return i, err
		}
	}

	return len(block), nil
}

func (this *NullEntropyDecoder) DecodeByte() (byte, error) {
	r, err := this.bitstream.ReadBits(8)
	return byte(r), err
}

func (this *NullEntropyDecoder) decodeLong(block []byte, offset int) error {
	val, err := this.bitstream.ReadBits(64)
	
	if err != nil {
	    return err
	}
	
	block[offset] = byte(val >> 56)
	block[offset+1] = byte(val >> 48)
	block[offset+2] = byte(val >> 40)
	block[offset+3] = byte(val >> 32)
	block[offset+4] = byte(val >> 24)
	block[offset+5] = byte(val >> 16)
	block[offset+6] = byte(val >> 8)
	block[offset+7] = byte(val)
    return nil
}

func (this *NullEntropyDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *NullEntropyDecoder) Dispose() {
}
