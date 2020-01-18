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

package function

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	_LZ_HASH_SEED     = 0x7FEB352D
	_LZ_HASH_LOG      = 18
	_LZ_HASH_SHIFT    = uint(32 - _LZ_HASH_LOG)
	_LZ_MAX_DISTANCE1 = (1 << 17) - 1
	_LZ_MAX_DISTANCE2 = (1 << 24) - 1
	_LZ_MIN_MATCH     = 4
	_LZ_MIN_LENGTH    = 16
)

// LZCodec Simple byte oriented LZ77 implementation.
// It is a modified LZ4 with a bigger window, a bigger hash map, 3+n*8 bit
// literal lengths and 17 or 24 bit match lengths.
type LZCodec struct {
	hashes []int32
}

// NewLZCodec creates a new instance of LZCodec
func NewLZCodec() (*LZCodec, error) {
	this := &LZCodec{}
	this.hashes = make([]int32, 0)
	return this, nil
}

// NewLZCodecWithCtx creates a new instance of LZCodec using a
// configuration map as parameter.
func NewLZCodecWithCtx(ctx *map[string]interface{}) (*LZCodec, error) {
	this := &LZCodec{}
	this.hashes = make([]int32, 0)
	return this, nil
}

func emitLength(buf []byte, length int) int {
	idx := 0

	for length >= 0xFF {
		buf[idx] = 0xFF
		idx++
		length -= 0xFF
	}

	buf[idx] = byte(length)
	return idx + 1
}

func emitLiterals(src, dst []byte) {
	length := len(src)

	for i := 0; i < length; i += 8 {
		copy(dst[i:], src[i:i+8])
	}
}

func emitLastLiterals(src, dst []byte) int {
	dstIdx := 1
	litLen := len(src)

	if litLen >= 7 {
		dst[0] = byte(7 << 5)
		dstIdx += emitLength(dst[1:], litLen-7)
	} else {
		dst[0] = byte(litLen << 5)
	}

	copy(dst[dstIdx:], src[0:litLen])
	return dstIdx + litLen
}

func lzhash(p []byte) uint32 {
	return (binary.LittleEndian.Uint32(p) * _LZ_HASH_SEED) >> _LZ_HASH_SHIFT
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	// If too small, skip
	if count < _LZ_MIN_LENGTH {
		return 0, 0, fmt.Errorf("Block too small, skip")
	}

	srcEnd := count - 8

	if len(this.hashes) == 0 {
		this.hashes = make([]int32, 1<<_LZ_HASH_LOG)
	}

	maxDist := _LZ_MAX_DISTANCE2
	dst[0] = 1

	if srcEnd < 4*_LZ_MAX_DISTANCE1 {
		maxDist = _LZ_MAX_DISTANCE1
		dst[0] = 0
	}

	srcIdx := 0
	dstIdx := 1
	anchor := 0

	for srcIdx < srcEnd {
		var minRef int

		if srcIdx < maxDist {
			minRef = 0
		} else {
			minRef = srcIdx - maxDist
		}

		h := lzhash(src[srcIdx:])
		ref := int(this.hashes[h])
		bestLen := 0

		// Find a match
		if ref > minRef && binary.LittleEndian.Uint32(src[srcIdx:]) == binary.LittleEndian.Uint32(src[ref:]) {
			maxMatch := srcEnd - srcIdx
			bestLen = 4

			for bestLen+4 < maxMatch && binary.LittleEndian.Uint32(src[srcIdx+bestLen:]) == binary.LittleEndian.Uint32(src[ref+bestLen:]) {
				bestLen += 4
			}

			for bestLen < maxMatch && src[ref+bestLen] == src[srcIdx+bestLen] {
				bestLen++
			}
		}

		// No good match ?
		if bestLen < _LZ_MIN_MATCH {
			this.hashes[h] = int32(srcIdx)
			srcIdx++
			continue
		}

		// Emit token
		// Token: 3 bits litLen + 1 bit flag + 4 bits mLen
		// flag = if maxDist = (1<<17)-1, then highest bit of distance
		//        else 1 if dist needs 3 bytes (> 0xFFFF) and 0 otherwise
		mLen := bestLen - _LZ_MIN_MATCH
		dist := srcIdx - ref
		var token int

		if dist > 0xFFFF {
			token = 0x10
		} else {
			token = 0
		}

		if mLen < 15 {
			token += mLen
		} else {
			token += 0x0F
		}

		// Literals to process ?
		if anchor == srcIdx {
			dst[dstIdx] = byte(token)
			dstIdx++
		} else {
			// Process match
			litLen := srcIdx - anchor

			// Emit literal length
			if litLen >= 7 {
				dst[dstIdx] = byte((7 << 5) | token)
				dstIdx++
				dstIdx += emitLength(dst[dstIdx:], litLen-7)
			} else {
				dst[dstIdx] = byte((litLen << 5) | token)
				dstIdx++
			}

			// Emit literals
			emitLiterals(src[anchor:anchor+litLen], dst[dstIdx:])
			dstIdx += litLen
		}

		// Emit match length
		if mLen >= 0x0F {
			dstIdx += emitLength(dst[dstIdx:], mLen-0x0F)
		}

		// Emit distance
		if maxDist == _LZ_MAX_DISTANCE2 && dist > 0xFFFF {
			dst[dstIdx] = byte(dist >> 16)
			dstIdx++
		}

		dst[dstIdx] = byte(dist >> 8)
		dstIdx++
		dst[dstIdx] = byte(dist)
		dstIdx++

		// Fill _hashes and update positions
		anchor = srcIdx + bestLen
		this.hashes[h] = int32(srcIdx)
		srcIdx++

		for srcIdx < anchor {
			this.hashes[lzhash(src[srcIdx:])] = int32(srcIdx)
			srcIdx++
		}
	}

	// Emit last literals
	dstIdx += emitLastLiterals(src[anchor:srcEnd+8], dst[dstIdx:])
	return uint(srcEnd + 8), uint(dstIdx), error(nil)
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)
	srcEnd := count - 8
	dstEnd := len(dst) - 8
	dstIdx := 0
	maxDist := _LZ_MAX_DISTANCE2

	if src[0] == 0 {
		maxDist = _LZ_MAX_DISTANCE1
	}

	srcIdx := 1

	for {
		token := int(src[srcIdx])
		srcIdx++

		if token >= 32 {
			// Get literal length
			litLen := token >> 5

			if litLen == 7 {
				for srcIdx < srcEnd && src[srcIdx] == 0xFF {
					srcIdx++
					litLen += 0xFF
				}

				if srcIdx >= srcEnd+8 {
					return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: Invalid literals length decoded: %d", litLen)
				}

				litLen += int(src[srcIdx])
				srcIdx++
			}

			// Copy literals and exit ?
			if dstIdx+litLen > dstEnd || srcIdx+litLen > srcEnd {
				copy(dst[dstIdx:], src[srcIdx:srcIdx+litLen])
				srcIdx += litLen
				dstIdx += litLen
				break
			}

			// Emit literals
			emitLiterals(src[srcIdx:srcIdx+litLen], dst[dstIdx:])
			srcIdx += litLen
			dstIdx += litLen
		}

		// Get match length
		mLen := token & 0x0F

		if mLen == 15 {
			for srcIdx < srcEnd && src[srcIdx] == 0xFF {
				srcIdx++
				mLen += 0xFF
			}

			if srcIdx < srcEnd {
				mLen += int(src[srcIdx])
				srcIdx++
			}
		}

		mLen += _LZ_MIN_MATCH
		mEnd := dstIdx + mLen

		// Sanity check
		if mEnd > dstEnd+8 {
			return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: Invalid match length decoded: %d", mLen)
		}

		// Get distance
		dist := (int(src[srcIdx]) << 8) | int(src[srcIdx+1])
		srcIdx += 2

		if (token & 0x10) != 0 {
			if maxDist == _LZ_MAX_DISTANCE1 {
				dist += 65536
			} else {
				dist = (dist << 8) | int(src[srcIdx])
				srcIdx++
			}
		}

		// Sanity check
		if dstIdx < dist || dist > maxDist {
			return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: Invalid distance decoded: %d", dist)
		}

		// Copy match
		if dist > 8 {
			ref := dstIdx - dist

			for {
				// No overlap
				copy(dst[dstIdx:], dst[ref:ref+8])
				ref += 8
				dstIdx += 8

				if dstIdx >= mEnd {
					break
				}
			}
		} else {
			ref := dstIdx - dist

			for i := 0; i < mLen; i++ {
				dst[dstIdx+i] = dst[ref+i]
			}
		}

		dstIdx = mEnd
	}

	return uint(srcIdx), uint(dstIdx), nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this LZCodec) MaxEncodedLen(srcLen int) int {
	if srcLen <= 1024 {
		return srcLen + 16
	}

	return srcLen + srcLen/64
}
