/*
Copyright 2011-2017 Frederic Langlet
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
	"kanzi"
	"unsafe"
)

// XXHash64 is an extremely fast hash algorithm. It was written by Yann Collet.
// Port to Go from the original source code: https://github.com/Cyan4973/xxHash

const (
	XXHASH_PRIME64_1 = uint64(0x9E3779B185EBCA87)
	XXHASH_PRIME64_2 = uint64(0xC2B2AE3D27D4EB4F)
	XXHASH_PRIME64_3 = uint64(0x165667B19E3779F9)
	XXHASH_PRIME64_4 = uint64(0x85EBCA77C2b2AE63)
	XXHASH_PRIME64_5 = uint64(0x27D4EB2F165667C5)
)

type endianXXHash64 interface {
	Uint32(p uintptr) uint32
	Uint64(p uintptr) uint64
	loop(p, limit uintptr, v1, v2, v3, v4 uint64) (uintptr, uint64, uint64, uint64, uint64)
}

type littleEndianXXHash64 struct {
	kanzi.LittleEndian // uses unsafe package
}

func (littleEndianXXHash64) loop(p, limit uintptr, v1, v2, v3, v4 uint64) (uintptr, uint64, uint64, uint64, uint64) {
	for p <= limit {
		v1 = xxHash64Round(v1, *(*uint64)(unsafe.Pointer(p)))
		v2 = xxHash64Round(v2, *(*uint64)(unsafe.Pointer(p + 8)))
		v3 = xxHash64Round(v3, *(*uint64)(unsafe.Pointer(p + 16)))
		v4 = xxHash64Round(v4, *(*uint64)(unsafe.Pointer(p + 24)))
		p += 32
	}

	return p, v1, v2, v3, v4
}

type bigEndianXXHash64 struct {
	kanzi.BigEndian // uses unsafe package
}

func (bigEndianXXHash64) loop(p, limit uintptr, v1, v2, v3, v4 uint64) (uintptr, uint64, uint64, uint64, uint64) {
	for p <= limit {
		var v uint64
		v = *(*uint64)(unsafe.Pointer(p))
		v = ((v << 56) & 0xFF00000000000000) | ((v << 40) & 0x00FF000000000000) |
			((v << 24) & 0x0000FF0000000000) | ((v << 8) & 0x000000FF00000000) |
			((v >> 8) & 0x00000000FF000000) | ((v >> 24) & 0x0000000000FF0000) |
			((v >> 40) & 0x000000000000FF00) | ((v >> 56) & 0x00000000000000FF)
		v1 = xxHash64Round(v1, v)
		v = *(*uint64)(unsafe.Pointer(p + 8))
		v = ((v << 56) & 0xFF00000000000000) | ((v << 40) & 0x00FF000000000000) |
			((v << 24) & 0x0000FF0000000000) | ((v << 8) & 0x000000FF00000000) |
			((v >> 8) & 0x00000000FF000000) | ((v >> 24) & 0x0000000000FF0000) |
			((v >> 40) & 0x000000000000FF00) | ((v >> 56) & 0x00000000000000FF)
		v = ((v << 24) & 0xFF000000) | ((v << 8) & 0x00FF0000) | ((v >> 8) & 0x0000FF00) | ((v >> 24) & 0x000000FF)
		v2 = xxHash64Round(v2, v)
		v = *(*uint64)(unsafe.Pointer(p + 16))
		v = ((v << 56) & 0xFF00000000000000) | ((v << 40) & 0x00FF000000000000) |
			((v << 24) & 0x0000FF0000000000) | ((v << 8) & 0x000000FF00000000) |
			((v >> 8) & 0x00000000FF000000) | ((v >> 24) & 0x0000000000FF0000) |
			((v >> 40) & 0x000000000000FF00) | ((v >> 56) & 0x00000000000000FF)
		v = ((v << 24) & 0xFF000000) | ((v << 8) & 0x00FF0000) | ((v >> 8) & 0x0000FF00) | ((v >> 24) & 0x000000FF)
		v3 = xxHash64Round(v3, v)
		v = *(*uint64)(unsafe.Pointer(p + 24))
		v = ((v << 56) & 0xFF00000000000000) | ((v << 40) & 0x00FF000000000000) |
			((v << 24) & 0x0000FF0000000000) | ((v << 8) & 0x000000FF00000000) |
			((v >> 8) & 0x00000000FF000000) | ((v >> 24) & 0x0000000000FF0000) |
			((v >> 40) & 0x000000000000FF00) | ((v >> 56) & 0x00000000000000FF)
		v = ((v << 24) & 0xFF000000) | ((v << 8) & 0x00FF0000) | ((v >> 8) & 0x0000FF00) | ((v >> 24) & 0x000000FF)
		v4 = xxHash64Round(v4, v)
		p += 32
	}

	return p, v1, v2, v3, v4
}

type XXHash64 struct {
	seed       uint64
	endianHash endianXXHash64
}

func NewXXHash64(seed uint64) (*XXHash64, error) {
	this := new(XXHash64)
	this.seed = seed

	if kanzi.IsBigEndian() {
		this.endianHash = &bigEndianXXHash64{}
	} else {
		this.endianHash = &littleEndianXXHash64{}
	}

	return this, nil
}

func (this *XXHash64) SetSeed(seed uint64) {
	this.seed = seed
}

func (this *XXHash64) Hash(data []byte) uint64 {
	length := len(data)
	p := uintptr(unsafe.Pointer(&data[0]))
	end := p + uintptr(length)
	var h64 uint64

	if length >= 32 {
		v1 := this.seed + XXHASH_PRIME64_1 + XXHASH_PRIME64_2
		v2 := this.seed + XXHASH_PRIME64_2
		v3 := this.seed
		v4 := this.seed - XXHASH_PRIME64_1

		p, v1, v2, v3, v4 = this.endianHash.loop(p, end-32, v1, v2, v3, v4)

		h64 = ((v1 << 1) | (v1 >> 31))
		h64 += ((v2 << 7) | (v2 >> 25))
		h64 += ((v3 << 12) | (v3 >> 20))
		h64 += ((v4 << 18) | (v4 >> 14))

		h64 = xxHash64MergeRound(h64, v1)
		h64 = xxHash64MergeRound(h64, v2)
		h64 = xxHash64MergeRound(h64, v3)
		h64 = xxHash64MergeRound(h64, v4)
	} else {
		h64 = this.seed + XXHASH_PRIME64_5
	}

	h64 += uint64(length)

	for p+8 <= end {
		h64 ^= xxHash64Round(0, this.endianHash.Uint64(p))
		h64 = ((h64<<27)|(h64>>37))*XXHASH_PRIME64_1 + XXHASH_PRIME64_4
		p += 8
	}

	for p+4 <= end {
		h64 ^= (uint64(this.endianHash.Uint32(p)) * XXHASH_PRIME64_1)
		h64 = ((h64<<23)|(h64>>41))*XXHASH_PRIME64_2 + XXHASH_PRIME64_3
		p += 4
	}

	for p < end {
		h64 += (uint64(*(*byte)(unsafe.Pointer(p))) * XXHASH_PRIME64_5)
		h64 = ((h64 << 11) | (h64 >> 53)) * XXHASH_PRIME64_1
		p++
	}

	h64 ^= (h64 >> 33)
	h64 *= XXHASH_PRIME64_2
	h64 ^= (h64 >> 29)
	h64 *= XXHASH_PRIME64_3
	return h64 ^ (h64 >> 32)
}

func xxHash64Round(acc, val uint64) uint64 {
	acc += (val * XXHASH_PRIME64_2)
	return ((acc << 13) | (acc >> 19)) * XXHASH_PRIME64_1
}

func xxHash64MergeRound(acc, val uint64) uint64 {
	acc ^= xxHash64Round(0, val)
	return acc*XXHASH_PRIME64_1 + XXHASH_PRIME64_4
}
