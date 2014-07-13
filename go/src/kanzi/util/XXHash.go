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

package util

import "unsafe"

// XXHash is an extremely fast hash algorithm. It was written by Yann Collet.
// Port to Go from the original source code: https://code.google.com/p/xxhash/

const (
	PRIME1 = uint32(2654435761)
	PRIME2 = uint32(2246822519)
	PRIME3 = uint32(3266489917)
	PRIME4 = uint32(668265263)
	PRIME5 = uint32(374761393)
)

type XXHash struct {
	seed uint32
}

func NewXXHash(seed uint32) (*XXHash, error) {
	this := new(XXHash)
	this.seed = seed
	return this, nil
}

func (this *XXHash) SetSeed(seed uint32) {
	this.seed = seed
}

func (this *XXHash) Hash(data []byte) uint32 {
	length := uint32(len(data))
	p := uintptr(unsafe.Pointer(&data[0]))
	end := p + uintptr(length)
	var h32 uint32

	if length >= 16 {
		limit := end - 16
		v1 := this.seed + PRIME1 + PRIME2
		v2 := this.seed + PRIME2
		v3 := this.seed
		v4 := this.seed - PRIME1

		for p <= limit {
			v1 += ((*(*uint32)(unsafe.Pointer(p))) * PRIME2)
			v1 = ((v1 << 13) | (v1 >> 19)) * PRIME1
			v2 += ((*(*uint32)(unsafe.Pointer(p + 4))) * PRIME2)
			v2 = ((v2 << 13) | (v2 >> 19)) * PRIME1
			v3 += ((*(*uint32)(unsafe.Pointer(p + 8))) * PRIME2)
			v3 = ((v3 << 13) | (v3 >> 19)) * PRIME1
			v4 += ((*(*uint32)(unsafe.Pointer(p + 12))) * PRIME2)
			v4 = ((v4 << 13) | (v4 >> 19)) * PRIME1
			p += 16
		}

		h32 = ((v1 << 1) | (v1 >> 31))
		h32 += ((v2 << 7) | (v2 >> 25))
		h32 += ((v3 << 12) | (v3 >> 20))
		h32 += ((v4 << 18) | (v4 >> 14))
	} else {
		h32 = this.seed + PRIME5
	}

	h32 += length

	for p <= end-4 {
		h32 += ((*(*uint32)(unsafe.Pointer(p))) * PRIME3)
		h32 = ((h32 << 17) | (h32 >> 15)) * PRIME4
		p += 4
	}

	for p < end {
		h32 += ((*(*uint32)(unsafe.Pointer(p))) * PRIME5)
		h32 = ((h32 << 11) | (h32 >> 21)) * PRIME1
		p++
	}

	h32 ^= (h32 >> 15)
	h32 *= PRIME2
	h32 ^= (h32 >> 13)
	h32 *= PRIME3
	return h32 ^ (h32 >> 16)
}
