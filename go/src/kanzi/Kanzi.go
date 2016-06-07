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

package kanzi

import (
	"bytes"
)

// An integer function is an operation that takes an array of integers as input and
// and turns it into another array of integers. The size of the returned array
// is not known in advance (by the caller).
// Return index in src, index in dst and error
type IntTransform interface {
	Forward(src, dst []int) (uint, uint, error)

	Inverse(src, dst []int) (uint, uint, error)
}

// A byte function is an operation that takes an array of bytes as input and
// turns it into another array of bytes. The size of the returned array is not
// known in advance (by the caller).
// Return index in src, index in dst and error
type ByteTransform interface {
	Forward(src, dst []byte) (uint, uint, error)

	Inverse(src, dst []byte) (uint, uint, error)
}

// An integer function is an operation that transforms the input int array and writes
// the result in the output int array. The result may have a different size.
// The function may fail if input and output array are the same array.
// The index of input and output arrays are updated appropriately.
// Return index in src, index in dst and error
type IntFunction interface {
	Forward(src, dst []int) (uint, uint, error)

	Inverse(src, dst []int) (uint, uint, error)

	// Return the max size required for the encoding output buffer
	// If the max size of the output buffer is not known, return -1
	MaxEncodedLen(srcLen int) int
}

// A byte function is an operation that transforms the input byte array and writes
// the result in the output byte array. The result may have a different size.
// The function may fail if input and output array are the same array.
// Return index in src, index in dst and error
type ByteFunction interface {
	Forward(src, dst []byte) (uint, uint, error)

	Inverse(src, dst []byte) (uint, uint, error)

	// Return the max size required for the encoding output buffer
	// If the max size of the output buffer is not known, return -1
	MaxEncodedLen(srcLen int) int
}

type InputBitStream interface {
	ReadBit() int // panic if error

	ReadBits(length uint) uint64 // panic if error

	Close() (bool, error)

	Read() uint64 // number of bits read so far

	HasMoreToRead() (bool, error)
}

type OutputBitStream interface {
	WriteBit(bit int) // panic if error

	WriteBits(bits uint64, length uint) uint // panic if error

	Close() (bool, error)

	Written() uint64 // number of bits written so far
}

type EntropyEncoder interface {
	// Encode the array provided into the bitstream. Return the number of byte
	// written to the bitstream
	Encode(block []byte) (int, error)

	// Return the underlying bitstream
	BitStream() OutputBitStream

	// Must be called before getting rid of the entropy encoder
	Dispose()
}

type EntropyDecoder interface {
	// Decode the next chunk of data from the bitstream and return in the
	// provided buffer.
	Decode(block []byte) (int, error)

	// Return the underlying bitstream
	BitStream() InputBitStream

	// Must be called before getting rid of the entropy decoder
	// Trying to encode after a call to dispose gives undefined behavior
	Dispose()
}

type Sizeable interface {
	Size() uint

	SetSize(sz uint) bool
}

func SameIntSlices(slice1, slice2 []int, deepCheck bool) bool {
	if slice2 == nil {
		return slice1 == nil
	}

	if slice1 == nil {
		return false
	}

	if &slice1 == &slice2 {
		return true
	}

	if len(slice1) != len(slice2) {
		return false
	}

	if slice2[0] != slice1[0] {
		return false
	}

	slice2[0] = ^slice2[0]

	if slice2[0] != ^slice1[0] {
		slice2[0] = ^slice2[0]
		return false
	}

	slice2[0] = ^slice2[0]

	if deepCheck == true {
		for i := range slice1 {
			if slice2[i] != slice1[i] {
				return false
			}
		}

		return true
	}

	return false
}

func SameByteSlices(slice1, slice2 []byte, deepCheck bool) bool {
	if slice2 == nil {
		return slice1 == nil
	}

	if slice1 == nil {
		return false
	}

	if &slice1 == &slice2 {
		return true
	}

	if len(slice1) != len(slice2) {
		return false
	}

	if deepCheck == true {
		return bytes.Equal(slice1, slice2)
	} else {
		return false
	}
}
