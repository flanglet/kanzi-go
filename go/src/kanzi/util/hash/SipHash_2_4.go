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

package hash

import (
	"kanzi"
	"unsafe"
)

// Port of SipHash (64 bits) to Go. Implemented with CROUNDS=2, dROUNDS=4.
// SipHash was designed by Jean-Philippe Aumasson and Daniel J. Bernstein.
// See https://131002.net/siphash/

const (
	SIPHASH_PRIME0 = uint64(0x736f6d6570736575)
	SIPHASH_PRIME1 = uint64(0x646f72616e646f6d)
	SIPHASH_PRIME2 = uint64(0x6c7967656e657261)
	SIPHASH_PRIME3 = uint64(0x7465646279746573)
)

type SipHash_2_4 struct {
	v0         uint64
	v1         uint64
	v2         uint64
	v3         uint64
	endianHash kanzi.ByteOrder // uses unsafe package
}

func NewSipHash() (*SipHash_2_4, error) {
	this := new(SipHash_2_4)

	if kanzi.IsBigEndian() {
		this.endianHash = &kanzi.BigEndian{}
	} else {
		this.endianHash = &kanzi.LittleEndian{}
	}

	return this, nil
}

func NewSipHashFromBuf(seed []byte) (*SipHash_2_4, error) {
	this := new(SipHash_2_4)
	this.SetSeedFromBuf(seed)
	return this, nil
}

func NewSipHashFromLong(k0, k1 uint64) (*SipHash_2_4, error) {
	this := new(SipHash_2_4)
	this.SetSeedFromLongs(k0, k1)
	return this, nil
}

func (this *SipHash_2_4) SetSeedFromBuf(seed []byte) {
	if len(seed) != 16 {
		panic("Seed length must be exactly 16")
	}

	p := uintptr(unsafe.Pointer(&seed[0]))
	this.SetSeedFromLongs(this.endianHash.Uint64(p), this.endianHash.Uint64(p+8))
}

func (this *SipHash_2_4) SetSeedFromLongs(k0, k1 uint64) {
	this.v0 = SIPHASH_PRIME0 ^ k0
	this.v1 = SIPHASH_PRIME1 ^ k1
	this.v2 = SIPHASH_PRIME2 ^ k0
	this.v3 = SIPHASH_PRIME3 ^ k1
}

func (this *SipHash_2_4) Hash(data []byte) uint64 {
	length := len(data)
	p := uintptr(unsafe.Pointer(&data[0]))
	end := p + uintptr(length)

	if length >= 8 {
		end8 := end - 8

		for p < end8 {
			m := this.endianHash.Uint64(p)
			this.v3 ^= m
			this.sipRound()
			this.sipRound()
			this.v0 ^= m
			p += 8
		}
	}

	last := uint64(length&0xFF) << 56
	

	for shift := uint(0); p < end; shift+=8 {
		last |= (uint64(*(*byte)(unsafe.Pointer(p))) << shift)
		p++
	}

	this.v3 ^= last
	this.sipRound()
	this.sipRound()
	this.v0 ^= last
	this.v2 ^= 0xFF
	this.sipRound()
	this.sipRound()
	this.sipRound()
	this.sipRound()
	this.v0 = this.v0 ^ this.v1 ^ this.v2 ^ this.v3
	return this.v0
}

func (this *SipHash_2_4) sipRound() {
	this.v0 += this.v1
	this.v1 = (this.v1 << 13) | (this.v1 >> 51)
	this.v1 ^= this.v0
	this.v0 = (this.v0 << 32) | (this.v0 >> 32)
	this.v2 += this.v3
	this.v3 = (this.v3 << 16) | (this.v3 >> 48)
	this.v3 ^= this.v2
	this.v0 += this.v3
	this.v3 = (this.v3 << 21) | (this.v3 >> 43)
	this.v3 ^= this.v0
	this.v2 += this.v1
	this.v1 = (this.v1 << 17) | (this.v1 >> 47)
	this.v1 ^= this.v2
	this.v2 = (this.v2 << 32) | (this.v2 >> 32)
}
