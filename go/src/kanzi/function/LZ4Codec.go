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

package function

import (
	"errors"
	"fmt"
	"kanzi"
	"unsafe"
)

// Go implementation of a LZ4 codec.
// LZ4 is a very fast lossless compression algorithm created by Yann Collet.
// See original code here: https://code.google.com/p/lz4/
// More details on the algorithm are available here:
// http://fastcompression.blogspot.com/2011/05/lz4-explained.html

const (
	HASH_SEED       = 0x9E3779B1
	HASH_LOG        = 12
	HASH_LOG_64K    = 13
	MAX_DISTANCE    = (1 << 16) - 1
	SKIP_STRENGTH   = 6
	LAST_LITERALS   = 5
	MIN_MATCH       = 4
	MF_LIMIT        = 12
	LZ4_64K_LIMIT   = MAX_DISTANCE + MF_LIMIT
	ML_BITS         = 4
	ML_MASK         = (1 << ML_BITS) - 1
	RUN_BITS        = 8 - ML_BITS
	RUN_MASK        = (1 << RUN_BITS) - 1
	COPY_LENGTH     = 8
	MIN_LENGTH      = 14
	MAX_LENGTH      = (32 * 1024 * 1024) - 4 - MIN_MATCH
	ACCELERATION    = 1
	SKIP_TRIGGER    = 6
	SEARCH_MATCH_NB = ACCELERATION << SKIP_TRIGGER
)

var (
	SHIFT1 = getShiftValue(0)
	SHIFT2 = getShiftValue(1)
	SHIFT3 = getShiftValue(2)
	SHIFT4 = getShiftValue(3)
)

func isBigEndian() bool {
	x := uint32(0x01020304)

	if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
		return true
	}

	return false
}

func getShiftValue(index uint) uint {
	index &= 3

	if isBigEndian() {
		return 24 - (index << 3)
	}

	return index << 3
}

type LZ4Codec struct {
	buffer []int
}

func NewLZ4Codec() (*LZ4Codec, error) {
	this := new(LZ4Codec)
	this.buffer = make([]int, 1<<HASH_LOG_64K)
	return this, nil
}

func writeLength(array []byte, length int) int {
	idx := 0

	for length >= 0x1FE {
		array[idx] = 0xFF
		array[idx+1] = 0xFF
		idx += 2
		length -= 0x1FE
	}

	if length >= 0xFF {
		array[idx] = 0xFF
		length -= 0xFF
		idx++
	}

	array[idx] = byte(length)
	return idx + 1
}

func writeLastLiterals(src []byte, dst []byte, runLength int) int {
	dstIdx := 1

	// Emit literal lengths
	if runLength >= RUN_MASK {
		dst[0] = byte(RUN_MASK << ML_BITS)
		dstIdx += writeLength(dst[1:], runLength-RUN_MASK)
	} else {
		dst[0] = byte(runLength << ML_BITS)
	}

	copy(dst[dstIdx:], src[0:runLength])
	return dstIdx + runLength
}

// Generates same byte output as LZ4_compress_generic in LZ4 r131 (7/15)
// for a 32 bit architecture.
func (this *LZ4Codec) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	var hashLog uint

	if count < LZ4_64K_LIMIT {
		hashLog = HASH_LOG_64K
	} else {
		hashLog = HASH_LOG
	}

	hashShift := 32 - hashLog
	srcEnd := count
	matchLimit := srcEnd - LAST_LITERALS
	mfLimit := srcEnd - MF_LIMIT
	srcIdx := 0
	dstIdx := 0
	anchor := 0

	if count > MIN_LENGTH {
		table := this.buffer[0 : 1<<hashLog]

		for i := range table {
			table[i] = 0
		}

		// First byte
		h32 := (readInt(src[srcIdx:]) * HASH_SEED) >> hashShift
		table[h32] = srcIdx
		srcIdx++
		h32 = (readInt(src[srcIdx:]) * HASH_SEED) >> hashShift

		for {
			fwdIdx := srcIdx
			step := 1
			searchMatchNb := SEARCH_MATCH_NB
			var match int

			// Find a match
			for {
				srcIdx = fwdIdx
				fwdIdx += step

				if fwdIdx > mfLimit {
					// Encode last literals
					dstIdx += writeLastLiterals(src[anchor:], dst[dstIdx:], srcEnd-anchor)
					return uint(srcEnd), uint(dstIdx), error(nil)
				}

				step = searchMatchNb >> SKIP_STRENGTH
				searchMatchNb++
				match = table[h32]
				table[h32] = srcIdx
				h32 = (readInt(src[fwdIdx:]) * HASH_SEED) >> hashShift

				if sameInts(src[srcIdx:srcIdx+4], src[match:match+4]) == true && match > srcIdx-MAX_DISTANCE {
					break
				}
			}

			// Catch up
			for match > 0 && srcIdx > anchor && src[match-1] == src[srcIdx-1] {
				match--
				srcIdx--
			}

			// Encode literal length
			litLength := srcIdx - anchor
			token := dstIdx
			dstIdx++

			if litLength >= RUN_MASK {
				dst[token] = byte(RUN_MASK << ML_BITS)
				dstIdx += writeLength(dst[dstIdx:], litLength-RUN_MASK)
			} else {
				dst[token] = byte(litLength << ML_BITS)
			}

			// Copy literals
			copy(dst[dstIdx:], src[anchor:anchor+litLength])
			dstIdx += litLength

			// Next match
			for {
				// Encode offset
				dst[dstIdx] = byte(srcIdx - match)
				dst[dstIdx+1] = byte((srcIdx - match) >> 8)
				dstIdx += 2

				// Encode match length
				srcIdx += MIN_MATCH
				match += MIN_MATCH
				anchor = srcIdx

				for srcIdx < matchLimit && src[srcIdx] == src[match] {
					srcIdx++
					match++
				}

				matchLength := srcIdx - anchor

				// Encode match length
				if matchLength >= ML_MASK {
					dst[token] += byte(ML_MASK)
					dstIdx += writeLength(dst[dstIdx:], matchLength-ML_MASK)
				} else {
					dst[token] += byte(matchLength)
				}

				anchor = srcIdx

				if srcIdx > mfLimit {
					dstIdx += writeLastLiterals(src[anchor:], dst[dstIdx:], srcEnd-anchor)
					return uint(srcEnd), uint(dstIdx), error(nil)
				}

				// Fill table
				h32 = (readInt(src[srcIdx-2:]) * HASH_SEED) >> hashShift
				table[h32] = srcIdx - 2

				// Test next position
				h32 = (readInt(src[srcIdx:]) * HASH_SEED) >> hashShift
				match = table[h32]
				table[h32] = srcIdx

				if sameInts(src[srcIdx:srcIdx+4], src[match:match+4]) == false || match <= srcIdx-MAX_DISTANCE {
					break
				}

				token = dstIdx
				dstIdx++
				dst[token] = 0
			}

			// Prepare next loop
			srcIdx++
			h32 = (readInt(src[srcIdx:]) * HASH_SEED) >> hashShift
		}
	}

	// Encode last literals
	dstIdx += writeLastLiterals(src[anchor:], dst[dstIdx:], srcEnd-anchor)
	return uint(srcEnd), uint(dstIdx), error(nil)
}

func sameInts(src, dst []byte) bool {
	return src[0] == dst[0] &&
		src[1] == dst[1] &&
		src[2] == dst[2] &&
		src[3] == dst[3]
}

func readInt(array []byte) uint32 {
	return (uint32(array[0]) << SHIFT1) |
		(uint32(array[1]) << SHIFT2) |
		(uint32(array[2]) << SHIFT3) |
		(uint32(array[3]) << SHIFT4)
}

// Reads same byte input as LZ4_decompress_generic in LZ4 r131 (7/15)
// for a 32 bit architecture.
func (this *LZ4Codec) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)
	srcEnd := count - COPY_LENGTH
	dstEnd := len(dst) - COPY_LENGTH
	srcIdx := 0
	dstIdx := 0

	for {
		// Get literal length
		token := int(src[srcIdx])
		srcIdx++
		length := token >> ML_BITS

		if length == RUN_MASK {
			for src[srcIdx] == byte(0xFF) && srcIdx < count {
				srcIdx++
				length += 0xFF
			}

			length += int(src[srcIdx])
			srcIdx++

			if length > MAX_LENGTH {
				return 0, 0, fmt.Errorf("Invalid length decoded: %d", length)
			}
		}

		// Copy literals
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

		length = token & ML_MASK

		// Get match length
		if length == ML_MASK {
			for src[srcIdx] == byte(0xFF) && srcIdx < count {
				srcIdx++
				length += 0xFF
			}

			length += int(src[srcIdx])
			srcIdx++

			if length > MAX_LENGTH {
				return 0, 0, fmt.Errorf("Invalid length decoded: %d", length)
			}
		}

		length += MIN_MATCH
		cpy := dstIdx + length

		if cpy > dstEnd {
			// Do not use copy on (potentially) overlapping slices
			for i := 0; i < length; i++ {
				dst[dstIdx+i] = dst[match+i]
			}
		} else {
			// Unroll loop
			for {
				dst[dstIdx] = dst[match]
				dst[dstIdx+1] = dst[match+1]
				dst[dstIdx+2] = dst[match+2]
				dst[dstIdx+3] = dst[match+3]
				dst[dstIdx+4] = dst[match+4]
				dst[dstIdx+5] = dst[match+5]
				dst[dstIdx+6] = dst[match+6]
				dst[dstIdx+7] = dst[match+7]
				match += 8
				dstIdx += 8

				if dstIdx >= cpy {
					break
				}
			}
		}

		// Correction
		dstIdx = cpy
	}

	return uint(count), uint(dstIdx), nil
}

func (this LZ4Codec) MaxEncodedLen(srcLen int) int {
	return srcLen + (srcLen / 255) + 16
}
