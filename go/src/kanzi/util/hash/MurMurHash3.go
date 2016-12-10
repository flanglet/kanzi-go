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

package hash

import (
	"kanzi"
	"unsafe"
)

// MurmurHash3 was written by Austin Appleby, and is placed in the public
// domain. The author hereby disclaims copyright to this source code.
// Original source code: https://github.com/aappleby/smhasher

const (
	MURMUR_HASH3_C1 = uint32(0xcc9e2d51)
	MURMUR_HASH3_C2 = uint32(0x1b873593)
	MURMUR_HASH3_C3 = uint32(0xe6546b64)
	MURMUR_HASH3_C4 = uint32(0x85ebca6b)
	MURMUR_HASH3_C5 = uint32(0xc2b2ae35)
)

type endianMurMurHash3 interface {
	loop(p, limit uintptr, k1 uint32) (uintptr, uint32)
}

type littleEndianMurMurHash3 struct {
}

func (littleEndianMurMurHash3) loop(p, limit uintptr, h1 uint32) (uintptr, uint32) {
	for p < limit {
		k1 := *(*uint32)(unsafe.Pointer(p))
		k1 *= MURMUR_HASH3_C1
		k1 = (k1 << 15) | (k1 >> 17)
		k1 *= MURMUR_HASH3_C2
		h1 ^= k1
		h1 = (h1 << 13) | (h1 >> 19)
		h1 = (h1 * 5) + MURMUR_HASH3_C3
		p += 4
	}

	return p, h1
}

type bigEndianMurMurHash3 struct {
}

func (bigEndianMurMurHash3) loop(p, limit uintptr, h1 uint32) (uintptr, uint32) {
	for p < limit {
		k1 := *(*uint32)(unsafe.Pointer(p))
		k1 = ((k1 << 24) & 0xFF000000) | ((k1 << 8) & 0x00FF0000) | ((k1 >> 8) & 0x0000FF00) | ((k1 >> 24) & 0x000000FF)
		k1 *= MURMUR_HASH3_C1
		k1 = (k1 << 15) | (k1 >> 17)
		k1 *= MURMUR_HASH3_C2
		h1 ^= k1
		h1 = (h1 << 13) | (h1 >> 19)
		h1 = (h1 * 5) + MURMUR_HASH3_C3
		p += 4
	}

	return p, h1
}

type MurMurHash3 struct {
	seed       uint32
	endianHash endianMurMurHash3 // uses unsafe package
}

func NewMurMurHash3(seed uint32) (*MurMurHash3, error) {
	this := new(MurMurHash3)
	this.seed = seed

	if kanzi.IsBigEndian() {
		this.endianHash = &bigEndianMurMurHash3{}
	} else {
		this.endianHash = &littleEndianMurMurHash3{}
	}

	return this, nil
}

func (this *MurMurHash3) SetSeed(seed uint32) {
	this.seed = seed
}

func (this *MurMurHash3) Hash(data []byte) uint32 {
	length := len(data)
	p := uintptr(unsafe.Pointer(&data[0]))
	end := p + uintptr(length)
	h1 := this.seed // aliasing

	if length >= 4 {
		// Body
		p, h1 = this.endianHash.loop(p, end-4, h1)
	}

	// Tail
	k1 := uint32(0)

	switch length & 3 {
	case 3:
		k1 ^= ((uint32(*(*byte)(unsafe.Pointer(p + 2)))) << 16)
		fallthrough
	case 2:
		k1 ^= ((uint32(*(*byte)(unsafe.Pointer(p + 1)))) << 8)
		fallthrough
	case 1:
		k1 ^= uint32(*(*byte)(unsafe.Pointer(p)))
		k1 *= MURMUR_HASH3_C1
		k1 = (k1 << 15) | (k1 >> 17)
		k1 *= MURMUR_HASH3_C2
		h1 ^= k1
	}

	// Finalization
	h1 ^= uint32(length)
	h1 ^= (h1 >> 16)
	h1 *= MURMUR_HASH3_C4
	h1 ^= (h1 >> 13)
	h1 *= MURMUR_HASH3_C5
	return h1 ^ (h1 >> 16)
}
