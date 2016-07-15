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

package util

import "unsafe"

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
	v0 uint64
	v1 uint64
	v2 uint64
	v3 uint64
}

func NewSipHash() (*SipHash_2_4, error) {
	this := new(SipHash_2_4)
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

	this.SetSeedFromLongs(bytesToLong(seed[0:8]), bytesToLong(seed[8:16]))
}

func (this *SipHash_2_4) SetSeedFromLongs(k0, k1 uint64) {
	this.v0 = SIPHASH_PRIME0 ^ k0
	this.v1 = SIPHASH_PRIME1 ^ k1
	this.v2 = SIPHASH_PRIME2 ^ k0
	this.v3 = SIPHASH_PRIME3 ^ k1
}

func (this *SipHash_2_4) Hash(data []byte) uint64 {
	length := len(data)
	end8 := length & -8
	var n int

	for n = 0; n < end8; n += 8 {
		m := bytesToLong(data[n : n+8])
		this.v3 ^= m
		this.sipRound()
		this.sipRound()
		this.v0 ^= m
	}

	last := (uint64(length) & 0xFF) << 56

	for i := uint(0); n < length; i += 8 {
		last |= ((uint64(data[n])) << i)
		n++
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

func bytesToLong(buf []byte) uint64 {
	p := uintptr(unsafe.Pointer(&buf[0]))
	return *(*uint64)(unsafe.Pointer(p))
	/*
		return ((uint64(buf[7])) << 56) |
			((uint64(buf[6])) << 48) |
			((uint64(buf[5])) << 40) |
			((uint64(buf[4])) << 32) |
			((uint64(buf[3])) << 24) |
			((uint64(buf[2])) << 16) |
			((uint64(buf[1])) << 8) |
			(uint64(buf[0]))
	*/
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
