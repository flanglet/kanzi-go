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

package function

// Snappy is a fast compression codec aiming for very high speed and
// reasonable compression ratios.
import (
	"encoding/binary"
	"errors"
	"fmt"
	kanzi "github.com/flanglet/kanzi-go"
)

const (
	MAX_OFFSET       = 32768
	MAX_TABLE_SIZE   = 16384
	TAG_LITERAL      = 0x00
	TAG_COPY1        = 0x01
	TAG_COPY2        = 0x02
	TAG_DEC_LEN1     = 0xF0
	TAG_DEC_LEN2     = 0xF4
	TAG_DEC_LEN3     = 0xF8
	TAG_DEC_LEN4     = 0xFC
	TAG_ENC_LEN1     = byte(TAG_DEC_LEN1 | TAG_LITERAL)
	TAG_ENC_LEN2     = byte(TAG_DEC_LEN2 | TAG_LITERAL)
	TAG_ENC_LEN3     = byte(TAG_DEC_LEN3 | TAG_LITERAL)
	TAG_ENG_LEN4     = byte(TAG_DEC_LEN4 | TAG_LITERAL)
	B0               = byte(TAG_DEC_LEN4 | TAG_COPY2)
	SNAPPY_HASH_SEED = 0x1E35A7BD
)

type SnappyCodec struct {
	buffer []int32
}

func NewSnappyCodec() (*SnappyCodec, error) {
	this := new(SnappyCodec)
	this.buffer = make([]int32, MAX_TABLE_SIZE)
	return this, nil
}

// snappyEmitLiteral writes a literal chunk and returns the number of bytes written.
func snappyEmitLiteral(src, dst []byte) int {
	dstIdx := 0
	length := len(src)
	n := len(src) - 1

	if n < 60 {
		dst[0] = byte((n << 2) | TAG_LITERAL)
		dstIdx = 1

		if length <= 16 {
			i0 := 0

			if length >= 8 {
				dst[1] = src[0]
				dst[2] = src[1]
				dst[3] = src[2]
				dst[4] = src[3]
				dst[5] = src[4]
				dst[6] = src[5]
				dst[7] = src[6]
				dst[8] = src[7]
				i0 = 8
			}

			for i := i0; i < length; i++ {
				dst[i+1] = src[i]
			}

			return length + 1
		}
	} else if n < 0x0100 {
		dst[0] = TAG_ENC_LEN1
		dst[1] = byte(n)
		dstIdx = 2
	} else if n < 0x010000 {
		dst[0] = TAG_ENC_LEN2
		dst[1] = byte(n)
		dst[2] = byte(n >> 8)
		dstIdx = 3
	} else if n < 0x01000000 {
		dst[0] = TAG_ENC_LEN3
		dst[1] = byte(n)
		dst[2] = byte(n >> 8)
		dst[3] = byte(n >> 16)
		dstIdx = 4
	} else {
		dst[0] = TAG_ENG_LEN4
		dst[1] = byte(n)
		dst[2] = byte(n >> 8)
		dst[3] = byte(n >> 16)
		dst[4] = byte(n >> 24)
		dstIdx = 5
	}

	copy(dst[dstIdx:], src[0:length])
	return length + dstIdx
}

// snappyEmitCopy writes a copy chunk and returns the number of bytes written.
func snappyEmitCopy(dst []byte, offset int) int {
	idx := 0
	b1 := byte(offset)
	b2 := byte(offset >> 8)
	length := len(dst)

	for length >= 64 {
		dst[idx] = B0
		dst[idx+1] = b1
		dst[idx+2] = b2
		idx += 3
		length -= 64
	}

	if (offset < 2048) && (length < 12) && (length >= 4) {
		dst[idx] = byte(((b2 & 0x07) << 5) | (byte(length-4) << 2) | TAG_COPY1)
		dst[idx+1] = b1
		idx += 2
	} else {
		dst[idx] = byte(((length - 1) << 2) | TAG_COPY2)
		dst[idx+1] = b1
		dst[idx+2] = b2
		idx += 3
	}

	return idx
}

func (this *SnappyCodec) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if len(src) == 0 {
		return uint(0), uint(0), nil
	}

	if dst == nil || len(dst) == 0 {
		return uint(0), uint(0), errors.New("Invalid null or empty destination buffer")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	// The block starts with the varint-encoded length of the decompressed bytes.
	dstIdx := this.putUvarint(dst, uint64(count))

	// Return early if src is short
	if count <= 4 {
		if count > 0 {
			dstIdx += snappyEmitLiteral(src[0:count], dst[dstIdx:])
		}

		return uint(count), uint(dstIdx), nil
	}

	// The hash table size ranges from 1<<8 to 1<<14 inclusive.
	shift := uint(24)
	tableSize := 256
	table := this.buffer // aliasing
	max := count

	if max > MAX_TABLE_SIZE {
		max = MAX_TABLE_SIZE
	}

	for tableSize < max {
		shift--
		tableSize <<= 1
	}

	lit := 0 // The start position of any pending literal bytes
	ends := count - 3

	// The encoded block must start with a literal, as there are no previous
	// bytes to copy, so we start looking for hash matches at index 1
	srcIdx := 1

	for srcIdx < ends {
		// Update the hash table
		h := (binary.LittleEndian.Uint32(src[srcIdx:srcIdx+4]) * SNAPPY_HASH_SEED) >> shift
		t := int(table[h]) // The last position with the same hash as srcIdx
		table[h] = int32(srcIdx)

		// If t is invalid or src[srcIdx:srcIdx+4] differs from src[t:t+4], accumulate a literal byte
		if (t == 0) || (srcIdx-t >= MAX_OFFSET) || (kanzi.DifferentInts(src[srcIdx:srcIdx+4], src[t:t+4])) {
			srcIdx++
			continue
		}

		// We have a match. First, emit any pending literal bytes
		if lit != srcIdx {
			dstIdx += snappyEmitLiteral(src[lit:srcIdx], dst[dstIdx:])
		}

		// Extend the match to be as long as possible
		s0 := srcIdx
		srcIdx += 4
		t += 4

		for (srcIdx < count) && (src[srcIdx] == src[t]) {
			srcIdx++
			t++
		}

		// Emit the copied bytes
		dstIdx += snappyEmitCopy(dst[dstIdx:dstIdx+srcIdx-s0], srcIdx-t)
		lit = srcIdx
	}

	// Emit any final pending literal bytes and return
	if lit != count {
		dstIdx += snappyEmitLiteral(src[lit:count], dst[dstIdx:])
	}

	return uint(count), uint(dstIdx), nil
}

func (this *SnappyCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if len(src) == 0 {
		return uint(0), uint(0), nil
	}

	if dst == nil || len(dst) == 0 {
		return uint(0), uint(0), errors.New("Invalid null or empty destination buffer")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	// Get decoded length
	dLen, idx, err := this.getDecodedLength(src)
	src = src[idx:]

	if err != nil {
		return 0, 0, fmt.Errorf("Decoding error: %v", err)
	}

	if len(dst) < int(dLen) {
		return 0, 0, errors.New("Decoding error: output buffer too small")
	}

	ends := uint(len(src))
	endd := uint(len(dst))
	s := uint(0)
	d := uint(0)
	var offset uint
	var length uint

	for s < ends {
		switch src[s] & 0x03 {
		case TAG_LITERAL:
			{
				x := uint(src[s] & 0xFC)

				if x < TAG_DEC_LEN1 {
					s++
					x >>= 2
				} else if x == TAG_DEC_LEN1 {
					s += 2
					x = uint(src[s-1])
				} else if x == TAG_DEC_LEN2 {
					s += 3
					x = uint(src[s-2]) | (uint(src[s-1]) << 8)
				} else if x == TAG_DEC_LEN3 {
					s += 4
					x = uint(src[s-3]) | (uint(src[s-2]) << 8) |
						(uint(src[s-1]) << 16)
				} else if x == TAG_DEC_LEN4 {
					s += 5
					x = uint(src[s-4]) | (uint(src[s-3]) << 8) |
						(uint(src[s-2]) << 16) | (uint(src[s-1]) << 24)
				}

				length = x + 1

				if (length <= 0) || (length > endd-d) || (length > ends-s) {
					break
				}

				if length < 16 {
					for i := uint(0); i < length; i++ {
						dst[d+i] = src[s+i]
					}
				} else {
					copy(dst[d:], src[s:s+length])
				}

				d += length
				s += length
				continue
			}

		case TAG_COPY1:
			{
				s += 2
				length = 4 + ((uint(src[s-2]) >> 2) & 0x07)
				offset = (uint(src[s-2]&0xE0) << 3) | uint(src[s-1])
				break
			}

		case TAG_COPY2:
			{
				s += 3
				length = 1 + (uint(src[s-3]) >> 2)
				offset = uint(src[s-2]) | (uint(src[s-1]) << 8)
				break
			}

		default:
		}

		end := d + length

		if (offset > d) || (end > endd) {
			break
		}

		for d < end {
			dst[d] = dst[d-offset]
			d++
		}
	}

	if d != dLen {
		err = fmt.Errorf("Decoding error: decoded %v byte(s), expected %v", d, dLen)
		return s, d, err
	}

	return ends, d, nil
}

func (this SnappyCodec) putUvarint(buf []byte, val uint64) int {
	idx := 0

	for val >= 0x80 {
		buf[idx] = byte(val | 0x80)
		idx++
		val >>= 7
	}

	buf[idx] = byte(val)
	return idx + 1
}

// Uvarint decodes an uint64 from the input array and returns that value.
func (this SnappyCodec) getUvarint(buf []byte) (uint64, int, error) {
	res := uint64(0)
	s := uint(0)

	for i := range buf {
		b := uint64(buf[i])

		if s >= 63 {
			if ((s == 63) && (b > 1)) || (s > 63) {
				return 0, 0, errors.New("Overflow: value is larger than 64 bits")
			}
		}

		if (b & 0x80) == 0 {
			return (res | (b << s)), i + 1, nil
		}

		res |= ((b & 0x7F) << s)
		s += 7
	}

	return 0, 0, errors.New("Input buffer too small")
}

// getDecodedLength returns the length of the decoded block
func (this SnappyCodec) getDecodedLength(buf []byte) (uint, int, error) {
	v, idx, err := this.getUvarint(buf)

	if err != nil {
		return 0, idx, err
	}

	if v > 0x7FFFFFFF {
		return 0, idx, errors.New("Overflow: invalid length")
	}

	return uint(v), idx, nil
}

// MaxEncodedLen returns the maximum length of a snappy block, given its
// uncompressed length.
//
// Compressed data can be defined as:
//    compressed := item* literal*
//    item       := literal* copy
//
// The trailing literal sequence has a space blowup of at most 62/60
// since a literal of length 60 needs one tag byte + one extra byte
// for length information.
//
// Item blowup is trickier to measure. Suppose the "copy" op copies
// 4 bytes of data. Because of a special check in the encoding code,
// we produce a 4-byte copy only if the offset is < 65536. Therefore
// the copy op takes 3 bytes to encode, and this type of item leads
// to at most the 62/60 blowup for representing literals.
//
// Suppose the "copy" op copies 5 bytes of data. If the offset is big
// enough, it will take 5 bytes to encode the copy op. Therefore the
// worst case here is a one-byte literal followed by a five-byte copy.
// That is, 6 bytes of input turn into 7 bytes of "compressed" data.
//
// This last factor dominates the blowup, so the final estimate is:
func (this SnappyCodec) MaxEncodedLen(srcLen int) int {
	return 32 + srcLen + srcLen/6
}
