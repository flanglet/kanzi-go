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
	HASH_SEED                   = 0x9E3779B1
	HASH_LOG                    = 12
	HASH_LOG_64K                = 13
	MAX_DISTANCE                = (1 << 16) - 1
	SKIP_STRENGTH               = 6
	LAST_LITERALS               = 5
	MIN_MATCH                   = 4
	MF_LIMIT                    = 12
	LZ4_64K_LIMIT               = MAX_DISTANCE + MF_LIMIT
	ML_BITS                     = 4
	ML_MASK                     = (1 << ML_BITS) - 1
	RUN_BITS                    = 8 - ML_BITS
	RUN_MASK                    = (1 << RUN_BITS) - 1
	COPY_LENGTH                 = 8
	MIN_LENGTH                  = 14
	MAX_LENGTH                  = (32 * 1024 * 1024) - 4 - MIN_MATCH
	DEFAULT_FIND_MATCH_ATTEMPTS = (1 << SKIP_STRENGTH) + 3
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
	size   uint
	buffer []int
}

func NewLZ4Codec(sz uint) (*LZ4Codec, error) {
	this := new(LZ4Codec)
	this.size = sz
	this.buffer = make([]int, 1<<HASH_LOG_64K)
	return this, nil
}

func (this *LZ4Codec) Size() uint {
	return this.size
}

func (this *LZ4Codec) SetSize(sz uint) bool {
	this.size = sz
	return true
}

func writeLength(array []byte, length int) int {
	index := 0

	for length >= 0x1FE {
		array[index] = 0xFF
		array[index+1] = 0xFF
		length -= 0x1FE
		index += 2
	}

	if length >= 0xFF {
		array[index] = 0xFF
		length -= 0xFF
		index++
	}

	array[index] = byte(length)
	return index + 1
}

func emitLiterals(src []byte, dst []byte, runLen int, last bool) (int, int, int) {
	var token int
	dstIdx := 0

	// Emit literal lengths
	if runLen >= RUN_MASK {
		token = RUN_MASK << ML_BITS

		if last == true {
			dst[dstIdx] = byte(token)
			dstIdx++
		}

		dstIdx += writeLength(dst[dstIdx:], runLen-RUN_MASK)
	} else {
		token = runLen << ML_BITS

		if last == true {
			dst[dstIdx] = byte(token)
			dstIdx++
		}
	}

	// Emit literals
	for i := 0; i < runLen; i++ {
		dst[dstIdx+i] = src[i]
	}

	return runLen, dstIdx + runLen, token
}

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

	count := int(this.size)

	if this.size == 0 {
		count = len(src)
	}

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	if count < MIN_LENGTH {
		srcIdx, dstIdx, _ := emitLiterals(src, dst, count, true)
		return uint(srcIdx), uint(dstIdx), error(nil)
	}

	var hashLog uint

	if count < LZ4_64K_LIMIT {
		hashLog = HASH_LOG_64K
	} else {
		hashLog = HASH_LOG
	}

	hashShift := 32 - hashLog
	srcEnd := count
	srcLimit := srcEnd - LAST_LITERALS
	mfLimit := srcEnd - MF_LIMIT
	srcIdx := 0
	dstIdx := 0
	anchor := srcIdx
	srcIdx++
	table := this.buffer // aliasing

	for i := (1 << hashLog) - 1; i >= 0; i-- {
		table[i] = 0
	}

	for {
		attempts := DEFAULT_FIND_MATCH_ATTEMPTS
		fwdIdx := srcIdx
		var ref int

		// Find a match
		for {
			srcIdx = fwdIdx
			fwdIdx += (attempts >> SKIP_STRENGTH)

			if fwdIdx > mfLimit {
				_, dstDelta, _ := emitLiterals(src[anchor:], dst[dstIdx:], srcEnd-anchor, true)
				return uint(srcEnd), uint(dstIdx + dstDelta), error(nil)
			}

			attempts++
			h32 := (readInt(src, srcIdx) * HASH_SEED) >> hashShift
			ref = table[h32]
			table[h32] = srcIdx

			if differentInts(src, ref, srcIdx) == false && ref > srcIdx-MAX_DISTANCE {
				break
			}
		}

		// Catch up
		for ref > 0 && srcIdx > anchor && src[ref-1] == src[srcIdx-1] {
			ref--
			srcIdx--
		}

		// Encode literal length
		runLen := srcIdx - anchor
		tokenOff := dstIdx
		dstIdx++
		_, dstDelta, token := emitLiterals(src[anchor:], dst[dstIdx:], runLen, false)
		dstIdx += dstDelta

		for true {
			// Encode offset
			dst[dstIdx] = byte(srcIdx - ref)
			dst[dstIdx+1] = byte((srcIdx - ref) >> 8)
			dstIdx += 2

			// Count matches
			srcIdx += MIN_MATCH
			ref += MIN_MATCH
			anchor = srcIdx

			for srcIdx < srcLimit && src[srcIdx] == src[ref] {
				srcIdx++
				ref++
			}

			matchLen := srcIdx - anchor

			// Encode match length
			if matchLen >= ML_MASK {
				dst[tokenOff] = byte(token | ML_MASK)
				dstIdx += writeLength(dst[dstIdx:], matchLen-ML_MASK)
			} else {
				dst[tokenOff] = byte(token | matchLen)
			}

			// Test end of chunk
			if srcIdx > mfLimit {
				_, dstDelta, _ := emitLiterals(src[srcIdx:], dst[dstIdx:], srcEnd-srcIdx, true)
				return uint(srcEnd), uint(dstIdx + dstDelta), error(nil)
			}

			// Test next position
			h32_1 := (readInt(src, srcIdx-2) * HASH_SEED) >> hashShift
			h32_2 := (readInt(src, srcIdx) * HASH_SEED) >> hashShift
			table[h32_1] = srcIdx - 2
			ref = table[h32_2]
			table[h32_2] = srcIdx

			if differentInts(src, ref, srcIdx) == true || ref <= srcIdx-MAX_DISTANCE {
				break
			}

			tokenOff = dstIdx
			dstIdx++
			token = 0
		}

		// Update
		anchor = srcIdx
		srcIdx++
	}
}

func differentInts(array []byte, srcIdx, dstIdx int) bool {
	return (array[srcIdx] != array[dstIdx]) ||
		(array[srcIdx+1] != array[dstIdx+1]) ||
		(array[srcIdx+2] != array[dstIdx+2]) ||
		(array[srcIdx+3] != array[dstIdx+3])
}

func readInt(array []byte, srcIdx int) uint32 {
	return (uint32(array[srcIdx]) << SHIFT1) |
		(uint32(array[srcIdx+1]) << SHIFT2) |
		(uint32(array[srcIdx+2]) << SHIFT3) |
		(uint32(array[srcIdx+3]) << SHIFT4)
}

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

	count := int(this.size)

	if this.size == 0 {
		count = len(src)
	}

	srcEnd := count - COPY_LENGTH
	dstEnd := len(dst) - COPY_LENGTH
	srcIdx := 0
	dstIdx := 0

	for {
		token := int(src[srcIdx])
		srcIdx++

		// Literals
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
		matchOffset := dstIdx - delta
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
		matchEnd := dstIdx + length

		if matchEnd > dstEnd {
			// Do not use copy on (potentially) overlapping slices
			for i := 0; i < length; i++ {
				dst[dstIdx+i] = dst[matchOffset+i]
			}
		} else {
			// Unroll loop
			for {
				dst[dstIdx] = dst[matchOffset]
				dst[dstIdx+1] = dst[matchOffset+1]
				dst[dstIdx+2] = dst[matchOffset+2]
				dst[dstIdx+3] = dst[matchOffset+3]
				dst[dstIdx+4] = dst[matchOffset+4]
				dst[dstIdx+5] = dst[matchOffset+5]
				dst[dstIdx+6] = dst[matchOffset+6]
				dst[dstIdx+7] = dst[matchOffset+7]
				matchOffset += 8
				dstIdx += 8

				if dstIdx >= matchEnd {
					break
				}
			}
		}

		// Correction
		dstIdx = matchEnd
	}

	return uint(count), uint(dstIdx), nil
}

func (this LZ4Codec) MaxEncodedLen(srcLen int) int {
	return srcLen + (srcLen / 255) + 16
}
