/*
Copyright 2011-2013 Frederic Langlet
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

// XXHash32 is an extremely fast hash algorithm. It was written by Yann Collet.
// Port to Go from the original source code: https://github.com/Cyan4973/xxHash

const (
	XXHASH_PRIME32_1 = uint32(2654435761)
	XXHASH_PRIME32_2 = uint32(2246822519)
	XXHASH_PRIME32_3 = uint32(3266489917)
	XXHASH_PRIME32_4 = uint32(668265263)
	XXHASH_PRIME32_5 = uint32(374761393)
)

type endianXXHash32 interface {
	Uint32(p uintptr) uint32
	loop(p, limit uintptr, v1, v2, v3, v4 uint32) (uintptr, uint32, uint32, uint32, uint32)
}

type littleEndianXXHash32 struct {
	kanzi.LittleEndian // uses unsafe package
}

func (littleEndianXXHash32) loop(p, limit uintptr, v1, v2, v3, v4 uint32) (uintptr, uint32, uint32, uint32, uint32) {
	for p <= limit {
		v1 = xxHash32Round(v1, *(*uint32)(unsafe.Pointer(p)))
		v2 = xxHash32Round(v2, *(*uint32)(unsafe.Pointer(p + 4)))
		v3 = xxHash32Round(v3, *(*uint32)(unsafe.Pointer(p + 8)))
		v4 = xxHash32Round(v4, *(*uint32)(unsafe.Pointer(p + 12)))
		p += 16
	}

	return p, v1, v2, v3, v4
}

type bigEndianXXHash32 struct {
	kanzi.BigEndian // uses unsafe package
}

func (bigEndianXXHash32) loop(p, limit uintptr, v1, v2, v3, v4 uint32) (uintptr, uint32, uint32, uint32, uint32) {
	for p <= limit {
		var v uint32
		v = *(*uint32)(unsafe.Pointer(p))
		v = ((v << 24) & 0xFF000000) | ((v << 8) & 0x00FF0000) | ((v >> 8) & 0x0000FF00) | ((v >> 24) & 0x000000FF)
		v1 = xxHash32Round(v1, v)
		v = *(*uint32)(unsafe.Pointer(p + 4))
		v = ((v << 24) & 0xFF000000) | ((v << 8) & 0x00FF0000) | ((v >> 8) & 0x0000FF00) | ((v >> 24) & 0x000000FF)
		v2 = xxHash32Round(v2, v)
		v = *(*uint32)(unsafe.Pointer(p + 8))
		v = ((v << 24) & 0xFF000000) | ((v << 8) & 0x00FF0000) | ((v >> 8) & 0x0000FF00) | ((v >> 24) & 0x000000FF)
		v3 = xxHash32Round(v3, v)
		v = *(*uint32)(unsafe.Pointer(p + 12))
		v = ((v << 24) & 0xFF000000) | ((v << 8) & 0x00FF0000) | ((v >> 8) & 0x0000FF00) | ((v >> 24) & 0x000000FF)
		v4 = xxHash32Round(v4, v)
		p += 16
	}

	return p, v1, v2, v3, v4
}

type XXHash32 struct {
	seed       uint32
	endianHash endianXXHash32
}

func NewXXHash32(seed uint32) (*XXHash32, error) {
	this := new(XXHash32)
	this.seed = seed

	if kanzi.IsBigEndian() {
		this.endianHash = &bigEndianXXHash32{}
	} else {
		this.endianHash = &littleEndianXXHash32{}
	}

	return this, nil
}

func (this *XXHash32) SetSeed(seed uint32) {
	this.seed = seed
}

func (this *XXHash32) Hash(data []byte) uint32 {
	length := len(data)
	p := uintptr(unsafe.Pointer(&data[0]))
	end := p + uintptr(length)
	var h32 uint32

	if length >= 16 {
		v1 := this.seed + XXHASH_PRIME32_1 + XXHASH_PRIME32_2
		v2 := this.seed + XXHASH_PRIME32_2
		v3 := this.seed
		v4 := this.seed - XXHASH_PRIME32_1

		p, v1, v2, v3, v4 = this.endianHash.loop(p, end-16, v1, v2, v3, v4)

		h32 = ((v1 << 1) | (v1 >> 31))
		h32 += ((v2 << 7) | (v2 >> 25))
		h32 += ((v3 << 12) | (v3 >> 20))
		h32 += ((v4 << 18) | (v4 >> 14))
	} else {
		h32 = this.seed + XXHASH_PRIME32_5
	}

	h32 += uint32(length)

	for p+4 <= end {
		h32 += (this.endianHash.Uint32(p) * XXHASH_PRIME32_3)
		h32 = ((h32 << 17) | (h32 >> 15)) * XXHASH_PRIME32_4
		p += 4
	}

	for p < end {
		h32 += (uint32(*(*byte)(unsafe.Pointer(p))) * XXHASH_PRIME32_5)
		h32 = ((h32 << 11) | (h32 >> 21)) * XXHASH_PRIME32_1
		p++
	}

	h32 ^= (h32 >> 15)
	h32 *= XXHASH_PRIME32_2
	h32 ^= (h32 >> 13)
	h32 *= XXHASH_PRIME32_3
	return h32 ^ (h32 >> 16)
}

func xxHash32Round(acc, val uint32) uint32 {
	acc += (val * XXHASH_PRIME32_2)
	return ((acc << 13) | (acc >> 19)) * XXHASH_PRIME32_1
}
