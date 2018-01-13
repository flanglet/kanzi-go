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
		this.bitstream.WriteBits(binary.BigEndian.Uint64(block[i:]), 64)
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
		binary.BigEndian.PutUint64(block[i:], this.bitstream.ReadBits(64))
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
