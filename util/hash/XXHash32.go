/*
Copyright 2011-2021 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License")
you may not use this file except in compliance with the License.
you may obtain a copy of the License at

                http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hash

import (
	"encoding/binary"
)

// XXHash32 is an extremely fast hash algorithm. It was written by Yann Collet.
// Port to Go from the original source code: https://github.com/Cyan4973/xxHash

const (
	_XXHASH_PRIME32_1 = uint32(2654435761)
	_XXHASH_PRIME32_2 = uint32(2246822519)
	_XXHASH_PRIME32_3 = uint32(3266489917)
	_XXHASH_PRIME32_4 = uint32(668265263)
	_XXHASH_PRIME32_5 = uint32(374761393)
)

// XXHash32 hash seed
type XXHash32 struct {
	seed uint32
}

// NewXXHash32 creates a new insytance of XXHash32
func NewXXHash32(seed uint32) (*XXHash32, error) {
	this := new(XXHash32)
	this.seed = seed
	return this, nil
}

// SetSeed sets the hash seed
func (this *XXHash32) SetSeed(seed uint32) {
	this.seed = seed
}

// Hash hashes the provided data
func (this *XXHash32) Hash(data []byte) uint32 {
	end := len(data)
	var h32 uint32
	n := 0

	if end >= 16 {
		end16 := end - 16
		v1 := this.seed + _XXHASH_PRIME32_1 + _XXHASH_PRIME32_2
		v2 := this.seed + _XXHASH_PRIME32_2
		v3 := this.seed
		v4 := this.seed - _XXHASH_PRIME32_1

		for n <= end16 {
			buf := data[n : n+16]
			v1 = xxHash32Round(v1, binary.LittleEndian.Uint32(buf[0:4]))
			v2 = xxHash32Round(v2, binary.LittleEndian.Uint32(buf[4:8]))
			v3 = xxHash32Round(v3, binary.LittleEndian.Uint32(buf[8:12]))
			v4 = xxHash32Round(v4, binary.LittleEndian.Uint32(buf[12:16]))
			n += 16
		}

		h32 = ((v1 << 1) | (v1 >> 31)) + ((v2 << 7) | (v2 >> 25)) +
			((v3 << 12) | (v3 >> 20)) + ((v4 << 18) | (v4 >> 14))
	} else {
		h32 = this.seed + _XXHASH_PRIME32_5
	}

	h32 += uint32(end)

	for n+4 <= end {
		h32 += (binary.LittleEndian.Uint32(data[n:n+4]) * _XXHASH_PRIME32_3)
		h32 = ((h32 << 17) | (h32 >> 15)) * _XXHASH_PRIME32_4
		n += 4
	}

	for n < end {
		h32 += (uint32(data[n]) * _XXHASH_PRIME32_5)
		h32 = ((h32 << 11) | (h32 >> 21)) * _XXHASH_PRIME32_1
		n++
	}

	h32 ^= (h32 >> 15)
	h32 *= _XXHASH_PRIME32_2
	h32 ^= (h32 >> 13)
	h32 *= _XXHASH_PRIME32_3
	return h32 ^ (h32 >> 16)
}

func xxHash32Round(acc, val uint32) uint32 {
	acc += (val * _XXHASH_PRIME32_2)
	return ((acc << 13) | (acc >> 19)) * _XXHASH_PRIME32_1
}
