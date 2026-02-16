/*
Copyright 2011-2026 Frederic Langlet
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

// XXHash64 is an extremely fast hash algorithm. It was written by Yann Collet.
// Port to Go from the original source code: https://github.com/Cyan4973/xxHash

const (
	_XXHASH_PRIME64_1 = uint64(0x9E3779B185EBCA87)
	_XXHASH_PRIME64_2 = uint64(0xC2B2AE3D27D4EB4F)
	_XXHASH_PRIME64_3 = uint64(0x165667B19E3779F9)
	_XXHASH_PRIME64_4 = uint64(0x85EBCA77C2b2AE63)
	_XXHASH_PRIME64_5 = uint64(0x27D4EB2F165667C5)
)

// XXHash64 hash seed
type XXHash64 struct {
	seed uint64
}

// NewXXHash64 creates a new insytance of XXHash64
func NewXXHash64(seed uint64) (*XXHash64, error) {
	this := new(XXHash64)
	this.seed = seed
	return this, nil
}

// SetSeed sets the hash seed
func (this *XXHash64) SetSeed(seed uint64) {
	this.seed = seed
}

// Hash hashes the provided data
func (this *XXHash64) Hash(data []byte) uint64 {
	end := len(data)
	var h64 uint64
	n := 0

	if end >= 32 {
		end32 := end - 32
		v1 := this.seed + _XXHASH_PRIME64_1 + _XXHASH_PRIME64_2
		v2 := this.seed + _XXHASH_PRIME64_2
		v3 := this.seed
		v4 := this.seed - _XXHASH_PRIME64_1

		for n <= end32 {
			buf := data[n : n+32]
			v1 = xxHash64Round(v1, binary.LittleEndian.Uint64(buf[0:8]))
			v2 = xxHash64Round(v2, binary.LittleEndian.Uint64(buf[8:16]))
			v3 = xxHash64Round(v3, binary.LittleEndian.Uint64(buf[16:24]))
			v4 = xxHash64Round(v4, binary.LittleEndian.Uint64(buf[24:32]))
			n += 32
		}

		h64 = ((v1 << 1) | (v1 >> 31)) + ((v2 << 7) | (v2 >> 25)) +
			((v3 << 12) | (v3 >> 20)) + ((v4 << 18) | (v4 >> 14))

		h64 = xxHash64MergeRound(h64, v1)
		h64 = xxHash64MergeRound(h64, v2)
		h64 = xxHash64MergeRound(h64, v3)
		h64 = xxHash64MergeRound(h64, v4)
	} else {
		h64 = this.seed + _XXHASH_PRIME64_5
	}

	h64 += uint64(end)

	for n+8 <= end {
		h64 ^= xxHash64Round(0, binary.LittleEndian.Uint64(data[n:n+8]))
		h64 = ((h64<<27)|(h64>>37))*_XXHASH_PRIME64_1 + _XXHASH_PRIME64_4
		n += 8
	}

	for n+4 <= end {
		h64 ^= (uint64(binary.LittleEndian.Uint32(data[n:n+4])) * _XXHASH_PRIME64_1)
		h64 = ((h64<<23)|(h64>>41))*_XXHASH_PRIME64_2 + _XXHASH_PRIME64_3
		n += 4
	}

	for n < end {
		h64 += (uint64(data[n]) * _XXHASH_PRIME64_5)
		h64 = ((h64 << 11) | (h64 >> 53)) * _XXHASH_PRIME64_1
		n++
	}

	h64 ^= (h64 >> 33)
	h64 *= _XXHASH_PRIME64_2
	h64 ^= (h64 >> 29)
	h64 *= _XXHASH_PRIME64_3
	return h64 ^ (h64 >> 32)
}

func xxHash64Round(acc, val uint64) uint64 {
	acc += (val * _XXHASH_PRIME64_2)
	return ((acc << 31) | (acc >> 33)) * _XXHASH_PRIME64_1
}

func xxHash64MergeRound(acc, val uint64) uint64 {
	acc ^= xxHash64Round(0, val)
	return acc*_XXHASH_PRIME64_1 + _XXHASH_PRIME64_4
}
