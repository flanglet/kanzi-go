/*
Copyright 2011-2026 Frederic Langlet
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

package transform

import (
	"encoding/binary"
	"errors"
	"math/bits"
)

const (
	_LZF_HASH_LOG          = 18
	_LZF_MIN_MATCH         = 6
	_LZF_MIN_DISTANCE      = 8
	_LZF_MAX_DISTANCE      = (1 << 26) - 2
	_LZF_MAX_MATCH         = 65793
	_LZF_TAIL              = 18
	_LZF_MIN_BLOCK_LENGTH  = _LZF_MIN_MATCH + _LZF_TAIL
	_LZF_READ_LENGTH_GUARD = 4
)

// LZFCodec is a fast raw LZ77 codec intended for use without an entropy coder.
type LZFCodec struct {
	hashes []int32
}

// NewLZFCodec creates a new LZFCodec.
func NewLZFCodec() (*LZFCodec, error) {
	return &LZFCodec{}, nil
}

// NewLZFCodecWithCtx creates a new LZFCodec using a configuration map.
func NewLZFCodecWithCtx(_ *map[string]any) (*LZFCodec, error) {
	return NewLZFCodec()
}

// MaxEncodedLen returns the maximum required output buffer size.
func (this *LZFCodec) MaxEncodedLen(srcLen int) int {
	if srcLen <= 1024 {
		return srcLen + 32
	}

	return srcLen + srcLen/64 + 32
}

func hashLZF(value uint64) int {
	return int(((value << 16) * uint64(0x1E35A7BD)) >> (64 - _LZF_HASH_LOG))
}

func findMatchLZF(src []byte, pos, ref, limit int) int {
	length := _LZF_MIN_MATCH

	for length+8 <= limit {
		diff := binary.LittleEndian.Uint64(src[pos+length:]) ^ binary.LittleEndian.Uint64(src[ref+length:])

		if diff != 0 {
			return length + (bits.TrailingZeros64(diff) >> 3)
		}

		length += 8
	}

	for length < limit && src[pos+length] == src[ref+length] {
		length++
	}

	return length
}

func writeLengthLZF(dst []byte, length int) int {
	if length < 254 {
		dst[0] = byte(length)
		return 1
	}

	if length < 65790 {
		length -= 254
		dst[0] = 254
		dst[1] = byte(length >> 8)
		dst[2] = byte(length)
		return 3
	}

	length -= 255
	dst[0] = 255
	dst[1] = byte(length >> 16)
	dst[2] = byte(length >> 8)
	dst[3] = byte(length)
	return 4
}

func readLengthLZF(src []byte, pos *int, end int) (int, bool) {
	if *pos >= end {
		return 0, false
	}

	length := int(src[*pos])
	(*pos)++

	if length < 254 {
		return length, true
	}

	if length == 254 {
		length += (int(src[*pos]) << 8) | int(src[*pos+1])
		*pos += 2
		return length, true
	}

	length += (int(src[*pos]) << 16) | (int(src[*pos+1]) << 8) | int(src[*pos+2])
	*pos += 3
	return length, true
}

// Forward applies the LZF transform to src and writes the result to dst.
func (this *LZFCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	count := len(src)

	if count < _LZF_MIN_BLOCK_LENGTH {
		return 0, 0, errors.New("LZF forward transform skip: block too small")
	}

	if len(dst) < this.MaxEncodedLen(count) {
		return 0, 0, errors.New("LZF forward transform skip: output buffer is too small")
	}

	if len(this.hashes) == 0 {
		this.hashes = make([]int32, 1<<_LZF_HASH_LOG)
	}

	for i := range this.hashes {
		this.hashes[i] = -1
	}

	srcEnd := count - _LZF_TAIL
	maxDistance := _LZF_MAX_DISTANCE

	if srcEnd < 4*262144 {
		maxDistance = 262151
	}

	pos := 0
	anchor := 0
	encoded := 5
	repeat := 0
	misses := 0

	for pos < srcEnd-_LZF_MIN_MATCH {
		ref := -1
		startPos := pos
		currentWord := binary.LittleEndian.Uint64(src[pos:])
		idx := hashLZF(currentWord)
		candidate := int(this.hashes[idx])
		pos1 := startPos + 1
		nextWord := currentWord >> 8
		refAtPos1 := pos1 - repeat
		matchAtPos1 := repeat >= _LZF_MIN_DISTANCE && repeat <= pos1 &&
			((nextWord^binary.LittleEndian.Uint64(src[refAtPos1:]))&uint64(0x0000FFFFFFFFFFFF)) == 0

		if matchAtPos1 {
			pos = pos1
			ref = refAtPos1
		} else {
			candidateDistance := pos - candidate
			prefixMatch := candidate >= 0 && candidateDistance <= maxDistance &&
				((currentWord^binary.LittleEndian.Uint64(src[candidate:]))&uint64(0x0000FFFFFFFFFFFF)) == 0

			if !prefixMatch {
				this.hashes[idx] = int32(pos)
			} else if candidateDistance >= _LZF_MIN_DISTANCE {
				this.hashes[idx] = int32(pos)
				ref = candidate
			}

			if ref < 0 {
				pos += 1 + (misses >> 5)
				misses++
				continue
			}
		}

		remaining := srcEnd - startPos
		maxMatch := min(remaining-1, _LZF_MAX_MATCH)
		length := findMatchLZF(src, pos, ref, maxMatch)

		if length < remaining {
			h1 := hashLZF(nextWord)
			candidate1 := int(this.hashes[h1])
			candidateDistance1 := pos1 - candidate1
			prefixMatch1 := candidate1 >= 0 && candidateDistance1 <= maxDistance &&
				((nextWord^binary.LittleEndian.Uint64(src[candidate1:]))&uint64(0x0000FFFFFFFFFFFF)) == 0

			if candidateDistance1 >= _LZF_MIN_DISTANCE {
				this.hashes[h1] = int32(pos1)

				if prefixMatch1 && binary.LittleEndian.Uint32(src[pos1+length-3:]) == binary.LittleEndian.Uint32(src[candidate1+length-3:]) {
					length1 := findMatchLZF(src, pos1, candidate1, maxMatch)

					if length1 >= length {
						pos = pos1
						ref = candidate1
						length = length1
					}
				}
			} else if !prefixMatch1 {
				this.hashes[h1] = int32(pos1)
			}
		}

		for pos > anchor && ref > 0 && src[pos-1] == src[ref-1] && length < _LZF_MAX_MATCH {
			pos--
			ref--
			length++
		}

		literals := pos - anchor

		if literals > 0 {
			if literals < 48 {
				dst[encoded] = byte(literals + 15)
				encoded++
			} else {
				if literals >= 1<<24 {
					return uint(count), uint(encoded), errors.New("LZF forward transform skip: too many literals")
				}

				dst[encoded] = 0x3F
				encoded++
				encoded += writeLengthLZF(dst[encoded:], literals-48)
			}

			copy(dst[encoded:], src[anchor:anchor+literals])
			encoded += literals
		}

		lenExtra := length - _LZF_MIN_MATCH
		lenNibble := min(lenExtra, 15)
		distance := pos - ref

		if distance == repeat {
			dst[encoded] = byte(lenNibble)
			encoded++
		} else {
			value := uint32(distance - _LZF_MIN_DISTANCE)
			width := 1

			if value >= 262144 {
				width = 3
			} else if value >= 1024 {
				width = 2
			}

			dst[encoded] = byte((width << 6) | (int(value&3) << 4) | lenNibble)
			encoded++
			binary.LittleEndian.PutUint32(dst[encoded:], value>>2)
			encoded += width
		}

		if lenNibble == 15 {
			encoded += writeLengthLZF(dst[encoded:], lenExtra-15)
		}

		anchor = pos + length
		this.hashes[hashLZF(binary.LittleEndian.Uint64(src[pos+2:]))] = int32(pos + 2)
		this.hashes[hashLZF(binary.LittleEndian.Uint64(src[pos+4:]))] = int32(pos + 4)
		this.hashes[hashLZF(binary.LittleEndian.Uint64(src[anchor-4:]))] = int32(anchor - 4)
		this.hashes[hashLZF(binary.LittleEndian.Uint64(src[anchor-3:]))] = int32(anchor - 3)
		this.hashes[hashLZF(binary.LittleEndian.Uint64(src[anchor-1:]))] = int32(anchor - 1)
		pos = anchor
		repeat = distance
		misses = 0
	}

	literals := count - anchor

	if literals < 48 {
		dst[encoded] = byte(literals + 15)
		encoded++
	} else {
		if literals >= 1<<24 {
			return uint(count), uint(encoded), errors.New("LZF forward transform skip: too many literals")
		}

		dst[encoded] = 0x3F
		encoded++
		encoded += writeLengthLZF(dst[encoded:], literals-48)
	}

	copy(dst[encoded:], src[anchor:])
	encoded += literals

	if encoded > count-count/100 {
		return uint(count), uint(encoded), errors.New("LZF forward transform skip: no compression")
	}

	flags := (_LZF_MIN_MATCH - 2) << 1
	if maxDistance == _LZF_MAX_DISTANCE {
		flags |= 1
	}

	binary.LittleEndian.PutUint32(dst, uint32(count))
	dst[4] = byte(flags)
	return uint(count), uint(encoded), nil
}

// Inverse applies the inverse LZF transform to src and writes the result to dst.
func (this *LZFCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	count := len(src)
	if count < 6 || cap(src) < _LZF_READ_LENGTH_GUARD || count > cap(src)-_LZF_READ_LENGTH_GUARD {
		return 0, 0, errors.New("LZF inverse transform failed: invalid input or missing guard bytes")
	}

	if len(dst) < 15 || &src[0] == &dst[0] {
		return 0, 0, errors.New("LZF inverse transform failed: invalid output buffer")
	}

	src = src[:count+_LZF_READ_LENGTH_GUARD]
	size := uint64(binary.LittleEndian.Uint32(src))

	if size < uint64(_LZF_MIN_BLOCK_LENGTH) || size > uint64(len(dst)-15) {
		return 0, 0, errors.New("LZF inverse transform failed: invalid output size")
	}

	flags := int(src[4])
	if flags&0xE0 != 0 {
		return 0, 0, errors.New("LZF inverse transform failed: invalid flags")
	}

	minMatch := ((flags >> 1) & 15) + 2
	maxDistance := 262151

	if flags&1 != 0 {
		maxDistance = _LZF_MAX_DISTANCE
	}

	ip := 5
	op := 0
	repeat := 0

	for ip < count {
		token := int(src[ip])
		ip++

		if token < 16 || token >= 64 {
			// Match token: 0000MMMM repeats the previous distance, while
			// FFDDMMMM carries one to three explicit distance bytes.
			distance := repeat
			width := token >> 6

			if width != 0 {
				if width > count-ip {
					return uint(ip), uint(op), errors.New("LZF inverse transform failed: truncated distance")
				}

				mask := uint32(1)<<(8*width) - 1
				value := binary.LittleEndian.Uint32(src[ip:]) & mask
				distance = int((value<<2)|uint32((token>>4)&3)) + _LZF_MIN_DISTANCE
				ip += width
			}

			length := token & 15
			if distance < _LZF_MIN_DISTANCE || distance > maxDistance || distance > op {
				return uint(ip), uint(op), errors.New("LZF inverse transform failed: invalid distance")
			}

			if length == 15 {
				extra, ok := readLengthLZF(src, &ip, count)
				if !ok {
					return uint(ip), uint(op), errors.New("LZF inverse transform failed: truncated match length")
				}

				length += extra
			}

			length += minMatch
			if length > int(size)-op-_LZF_TAIL {
				return uint(ip), uint(op), errors.New("LZF inverse transform failed: invalid match length")
			}

			for i := 0; i < length; i++ {
				dst[op+i] = dst[op-distance+i]
			}

			op += length
			repeat = distance
		} else {
			// Literal tokens 16..62 encode lengths 1..47; 0x3F extends
			// from 48 bytes.
			literals := token - 15

			if token == 0x3F {
				extra, ok := readLengthLZF(src, &ip, count)
				if !ok {
					return uint(ip), uint(op), errors.New("LZF inverse transform failed: truncated literal length")
				}

				literals = 48 + extra
			}

			if literals == count-ip {
				if literals < _LZF_TAIL || literals != int(size)-op {
					return uint(ip), uint(op), errors.New("LZF inverse transform failed: invalid final literals")
				}

				copy(dst[op:op+literals], src[ip:ip+literals])
				op += literals
				return uint(count), uint(op), nil
			}

			if literals > int(size)-op-_LZF_TAIL || literals > count-ip-_LZF_TAIL {
				return uint(ip), uint(op), errors.New("LZF inverse transform failed: invalid literal length")
			}

			copy(dst[op:op+literals], src[ip:ip+literals])
			ip += literals
			op += literals
		}
	}

	return uint(ip), uint(op), errors.New("LZF inverse transform failed: missing final literal run")
}
