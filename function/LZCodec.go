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

// Simple byte oriented LZ77 codec implementation.
// It is just LZ4 modified to use a bigger hash map.

const (
	_LZ_HASH_SEED    = 0x7FEB352D
	_HASH_LOG_SMALL  = 12
	_HASH_LOG_BIG    = 16
	_MAX_DISTANCE    = (1 << 16) - 1
	_SKIP_STRENGTH   = 6
	_LAST_LITERALS   = 5
	_MIN_MATCH       = 4
	_MF_LIMIT        = 12
	_ML_BITS         = 4
	_ML_MASK         = (1 << _ML_BITS) - 1
	_RUN_BITS        = 8 - _ML_BITS
	_RUN_MASK        = (1 << _RUN_BITS) - 1
	_COPY_LENGTH     = 8
	_MIN_LENGTH      = 14
	_MAX_LENGTH      = (32 * 1024 * 1024) - 4 - _MIN_MATCH
	_SEARCH_MATCH_NB = 1 << 6
)

// LZCodec Lempel Ziv (LZ77) codec based on LZ4
type LZCodec struct {
	buffer []int32
}

// NewLZCodec creates a new instance of LZCodec
func NewLZCodec() (*LZCodec, error) {
	this := &LZCodec{}
	this.buffer = make([]int32, 0)
	return this, nil
}

// NewLZCodecWithCtx creates a new instance of LZCodec  using a
// configuration map as parameter.
func NewLZCodecWithCtx(ctx *map[string]interface{}) (*LZCodec, error) {
	this := &LZCodec{}
	this.buffer = make([]int32, 0)
	return this, nil
}

func emitLength(buf []byte, length int) int {
	idx := 0

	for length >= 0x1FE {
		buf[idx] = 0xFF
		buf[idx+1] = 0xFF
		idx += 2
		length -= 0x1FE
	}

	if length >= 0xFF {
		buf[idx] = 0xFF
		idx++
		length -= 0xFF
	}

	buf[idx] = byte(length)
	return idx + 1
}

func emitLastLiterals(src, dst []byte) int {
	dstIdx := 1
	runLength := len(src)

	if runLength >= _RUN_MASK {
		dst[0] = byte(_RUN_MASK << _ML_BITS)
		dstIdx += emitLength(dst[1:], runLength-_RUN_MASK)
	} else {
		dst[0] = byte(runLength << _ML_BITS)
	}

	copy(dst[dstIdx:], src[0:runLength])
	return dstIdx + runLength
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

	var hashLog uint

	if count < _MAX_DISTANCE {
		hashLog = _HASH_LOG_SMALL
	} else {
		hashLog = _HASH_LOG_BIG
	}

	hashShift := 32 - hashLog
	srcEnd := count
	matchLimit := srcEnd - _LAST_LITERALS
	mfLimit := srcEnd - _MF_LIMIT
	srcIdx := 0
	dstIdx := 0
	anchor := 0

	if count > _MIN_LENGTH {
		if len(this.buffer) < 1<<hashLog {
			this.buffer = make([]int32, 1<<hashLog)
		} else {
			for i := range this.buffer {
				this.buffer[i] = 0
			}
		}

		// First byte
		table := this.buffer
		h32 := (binary.LittleEndian.Uint32(src[srcIdx:]) * _LZ_HASH_SEED) >> hashShift
		table[h32] = int32(srcIdx)
		srcIdx++
		h32 = (binary.LittleEndian.Uint32(src[srcIdx:]) * _LZ_HASH_SEED) >> hashShift

		for {
			fwdIdx := srcIdx
			step := 1
			searchMatchNb := _SEARCH_MATCH_NB
			var match int

			// Find a match
			for {
				srcIdx = fwdIdx
				fwdIdx += step

				if fwdIdx > mfLimit {
					// Emit last literals
					dstIdx += emitLastLiterals(src[anchor:srcEnd], dst[dstIdx:])
					return uint(srcEnd), uint(dstIdx), error(nil)
				}

				step = searchMatchNb >> _SKIP_STRENGTH
				searchMatchNb++
				match = int(table[h32])
				table[h32] = int32(srcIdx)
				h32 = (binary.LittleEndian.Uint32(src[fwdIdx:]) * _LZ_HASH_SEED) >> hashShift

				if binary.LittleEndian.Uint32(src[srcIdx:]) == binary.LittleEndian.Uint32(src[match:]) && match > srcIdx-_MAX_DISTANCE {
					break
				}
			}

			// Catch up
			for match > 0 && srcIdx > anchor && src[match-1] == src[srcIdx-1] {
				match--
				srcIdx--
			}

			// Emit literal length
			litLength := srcIdx - anchor
			token := dstIdx
			dstIdx++

			if litLength >= _RUN_MASK {
				dst[token] = byte(_RUN_MASK << _ML_BITS)
				dstIdx += emitLength(dst[dstIdx:], litLength-_RUN_MASK)
			} else {
				dst[token] = byte(litLength << _ML_BITS)
			}

			// Copy literals
			copy(dst[dstIdx:], src[anchor:anchor+litLength])
			dstIdx += litLength

			// Next match
			for {
				// Emit offset
				dst[dstIdx] = byte(srcIdx - match)
				dst[dstIdx+1] = byte((srcIdx - match) >> 8)
				dstIdx += 2

				// Emit match length
				srcIdx += _MIN_MATCH
				match += _MIN_MATCH
				anchor = srcIdx

				for srcIdx < matchLimit && src[srcIdx] == src[match] {
					srcIdx++
					match++
				}

				matchLength := srcIdx - anchor

				// Emit match length
				if matchLength >= _ML_MASK {
					dst[token] += byte(_ML_MASK)
					dstIdx += emitLength(dst[dstIdx:], matchLength-_ML_MASK)
				} else {
					dst[token] += byte(matchLength)
				}

				anchor = srcIdx

				if srcIdx > mfLimit {
					dstIdx += emitLastLiterals(src[anchor:srcEnd], dst[dstIdx:])
					return uint(srcEnd), uint(dstIdx), error(nil)
				}

				// Fill table
				h32 = (binary.LittleEndian.Uint32(src[srcIdx-2:]) * _LZ_HASH_SEED) >> hashShift
				table[h32] = int32(srcIdx - 2)

				// Test next position
				h32 = (binary.LittleEndian.Uint32(src[srcIdx:]) * _LZ_HASH_SEED) >> hashShift
				match = int(table[h32])
				table[h32] = int32(srcIdx)

				if binary.LittleEndian.Uint32(src[srcIdx:]) != binary.LittleEndian.Uint32(src[match:]) || match <= srcIdx-_MAX_DISTANCE {
					break
				}

				token = dstIdx
				dstIdx++
				dst[token] = 0
			}

			// Prepare next loop
			srcIdx++
			h32 = (binary.LittleEndian.Uint32(src[srcIdx:]) * _LZ_HASH_SEED) >> hashShift
		}
	}

	// Emit last literals
	dstIdx += emitLastLiterals(src[anchor:srcEnd], dst[dstIdx:])
	return uint(srcEnd), uint(dstIdx), error(nil)
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
	srcEnd := count - _COPY_LENGTH
	dstEnd := len(dst) - _COPY_LENGTH
	srcIdx := 0
	dstIdx := 0

	for {
		// Get literal length
		token := int(src[srcIdx])
		srcIdx++
		length := token >> _ML_BITS

		if length == _RUN_MASK {
			for src[srcIdx] == byte(0xFF) && srcIdx < count {
				srcIdx++
				length += 0xFF
			}

			length += int(src[srcIdx])
			srcIdx++

			if length > _MAX_LENGTH {
				return 0, 0, fmt.Errorf("Invalid length decoded: %d", length)
			}
		}

		// Copy literals
		if dstIdx+length > dstEnd || srcIdx+length > srcEnd {
			copy(dst[dstIdx:], src[srcIdx:srcIdx+length])
			srcIdx += length
			dstIdx += length
			break
		}

		for i := 0; i < length; i++ {
			dst[dstIdx+i] = src[srcIdx+i]
		}

		srcIdx += length
		dstIdx += length

		if dstIdx > dstEnd || srcIdx > srcEnd {
			break
		}

		// Get offset
		delta := int(src[srcIdx]) | (int(src[srcIdx+1]) << 8)
		srcIdx += 2
		match := dstIdx - delta

		if match < 0 {
			break
		}

		length = token & _ML_MASK

		// Get match length
		if length == _ML_MASK {
			for src[srcIdx] == 0xFF && srcIdx < count {
				srcIdx++
				length += 0xFF
			}

			if srcIdx < count {
				length += int(src[srcIdx])
				srcIdx++
			}

			if length > _MAX_LENGTH || srcIdx == count {
				return 0, 0, fmt.Errorf("Invalid length decoded: %d", length)
			}
		}

		length += _MIN_MATCH
		cpy := dstIdx + length

		if cpy > dstEnd {
			// Do not use copy on (potentially) overlapping slices
			for i := 0; i < length; i++ {
				dst[dstIdx+i] = dst[match+i]
			}
		} else {
			if dstIdx >= match+8 {
				for {
					binary.LittleEndian.PutUint64(dst[dstIdx:], binary.LittleEndian.Uint64(dst[match:]))
					match += 8
					dstIdx += 8

					if dstIdx >= cpy {
						break
					}
				}
			} else {
				// Unroll loop
				for {
					s := dst[match : match+8]
					d := dst[dstIdx : dstIdx+8]
					d[0] = s[0]
					d[1] = s[1]
					d[2] = s[2]
					d[3] = s[3]
					d[4] = s[4]
					d[5] = s[5]
					d[6] = s[6]
					d[7] = s[7]
					match += 8
					dstIdx += 8

					if dstIdx >= cpy {
						break
					}
				}
			}
		}

		// Correction
		dstIdx = cpy
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
