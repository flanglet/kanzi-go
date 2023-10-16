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
	kanzi "github.com/flanglet/kanzi-go/v2"
)

// NullEntropyEncoder pass through entropy encoder (writes the input bytes directly
// to the bitstream)
type NullEntropyEncoder struct {
	bitstream kanzi.OutputBitStream
}

// NewNullEntropyEncoder  creates a new instance of NullEntropyEncoder
func NewNullEntropyEncoder(bs kanzi.OutputBitStream) (*NullEntropyEncoder, error) {
	this := new(NullEntropyEncoder)
	this.bitstream = bs
	return this, nil
}

// Write encodes the data provided into the bitstream. Return the number of byte
// written to the bitstream
func (this *NullEntropyEncoder) Write(block []byte) (int, error) {
	res := 0
	count := len(block)
	idx := 0

	for count > 0 {
		ckSize := count

		if ckSize > 1<<23 {
			ckSize = 1 << 23
		}

		res += int(this.bitstream.WriteArray(block[idx:], uint(8*ckSize)) >> 3)
		idx += ckSize
		count -= ckSize
	}

	return res, nil
}

// BitStream returns the underlying bitstream
func (this *NullEntropyEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// Dispose this implementation does nothing
func (this *NullEntropyEncoder) Dispose() {
}

// NullEntropyDecoder pass through entropy decoder (reads the input bytes directly
// from the bitstream)
type NullEntropyDecoder struct {
	bitstream kanzi.InputBitStream
}

// NewNullEntropyDecoder  creates a new instance of NullEntropyDecoder
func NewNullEntropyDecoder(bs kanzi.InputBitStream) (*NullEntropyDecoder, error) {
	this := new(NullEntropyDecoder)
	this.bitstream = bs
	return this, nil
}

// Read decodes data from the bitstream and return it in the provided buffer.
// Return the number of bytes read from the bitstream
func (this *NullEntropyDecoder) Read(block []byte) (int, error) {
	res := 0
	count := len(block)
	idx := 0

	for count > 0 {
		ckSize := count

		if ckSize > 1<<23 {
			ckSize = 1 << 23
		}

		res += int(this.bitstream.ReadArray(block[idx:], uint(8*ckSize)) >> 3)
		idx += ckSize
		count -= ckSize
	}

	return res, nil
}

// BitStream returns the underlying bitstream
func (this *NullEntropyDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Dispose this implementation does nothing
func (this *NullEntropyDecoder) Dispose() {
}
