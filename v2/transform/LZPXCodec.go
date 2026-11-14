/*
Copyright 2011-2026 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License")
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
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
	"fmt"
	"math/bits"
)

const (
	_LZPX_HASH_LOG         = 18
	_LZPX_HASH_SHIFT       = 14
	_LZPX_CHUNK_SIZE       = 4194304
	_LZPX_MIN_CHUNK_SIZE   = 33 // bitmap plus at least one end-of-block token
	_LZPX_MIN_MATCH        = 2
	_LZPX_MIN_ABSENT_CODES = 16
)

// LZPXCodec implements the Lempel-Ziv Predict Extra transform. The block
// header stores minMatch-2 in its low 6 bits and selects stride 8 with its high bit.
type LZPXCodec struct {
	hashes     []int32
	hashes64   []uint64
	hashStride int
}

// NewLZPXCodec creates an LZPX transform with the default hash stride.
func NewLZPXCodec() (*LZPXCodec, error) {
	return &LZPXCodec{hashStride: 1}, nil
}

// NewLZPXCodecWithCtx creates an LZPX transform using its configuration map.
func NewLZPXCodecWithCtx(ctx *map[string]any) (*LZPXCodec, error) {
	hashStride := 1

	if ctx != nil {
		if val, containsKey := (*ctx)["lzpxType"]; containsKey {
			if lzpxType, ok := val.(int); ok && lzpxType == 2 {
				hashStride = 8
			}
		}
	}

	return &LZPXCodec{hashStride: hashStride}, nil
}

// MaxEncodedLen returns the input size. Forward only succeeds when the
// transform saves at least one percent, so a larger output is rejected.
func (this *LZPXCodec) MaxEncodedLen(srcLen int) int {
	return srcLen
}

// Forward encodes the input using one minimum-match value per transform block
// and a literal alphabet for each internal chunk.
//
// The block header stores minMatch-2 in its low 6 bits; its high bit selects
// stride 8. Each chunk starts with a 256-bit literal bitmap (least significant bit first).
// Absent bytes are split into:
// - codes[0]: end-of-chunk marker
// - codes[1..nbCodes-3]: 1-byte short matches [minMatch .. maxShort]
// - codes[nbCodes-2]: matches [maxShort+1 .. maxShort+256] followed by 1 byte offset (0..255)
// - codes[nbCodes-1]: matches >= maxShort+257 followed by unsigned LEB128 (7 bits/byte, 0x80=more)
// The predictor table spans all chunks.
func (this *LZPXCodec) Forward(src, dst []byte) (uint, uint, error) {
	count := len(src)

	if count == 0 {
		return 0, 0, nil
	}

	if count < 4 || len(dst) < count {
		return 0, 0, errors.New("LZPX forward transform skip: invalid or insufficient buffer")
	}

	if len(this.hashes64) == 0 {
		this.hashes64 = make([]uint64, 1<<_LZPX_HASH_LOG)
	} else {
		clear(this.hashes64)
	}

	hashes := this.hashes64
	stride := this.hashStride
	dstIdx := 0
	header := (_LZPX_MIN_MATCH - 2) & 0x3F
	if stride == 8 {
		header |= 0x80
	}
	dst[dstIdx] = byte(header)
	dstIdx++
	srcIdx := 0

	// Chunks need their own token alphabet: absent byte values encode the
	// end marker, short match lengths, and the extended-length prefix.
	// Limit chunks to _LZPX_CHUNK_SIZE and adapt the alphabet cap for long chunks.
	for srcIdx < count {
		if count-dstIdx < _LZPX_MIN_CHUNK_SIZE {
			return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
		}

		size := count - srcIdx

		if size > _LZPX_CHUNK_SIZE {
			size = _LZPX_CHUNK_SIZE
		}

		var seen [4]uint64
		alphabet := 0

		for i := 0; i < size; i++ {
			val := uint(src[srcIdx+i])
			bit := uint64(1) << (val & 63)
			wIdx := val >> 6

			if seen[wIdx]&bit != 0 {
				continue
			}

			maxAlphabet := 216
			if i < 4096 {
				maxAlphabet = 256 - _LZPX_MIN_ABSENT_CODES
			}

			if alphabet >= maxAlphabet {
				size = i
				break
			}

			seen[wIdx] |= bit
			alphabet++
		}

		var codes [256]byte
		nbCodes := 0

		// Mark present bytes as literals; absent bytes become end and match codes.
		for k := 0; k < 4; k++ {
			binary.LittleEndian.PutUint64(dst[dstIdx+(k<<3):], seen[k])
			absent := ^seen[k]

			for absent != 0 {
				codes[nbCodes] = byte((k << 6) | bits.TrailingZeros64(absent))
				nbCodes++
				absent &= absent - 1
			}
		}

		dstIdx += 32
		end := srcIdx + size

		// Reserve 3 control codes (EOB, 1-byte long offset, LEB128 long offset),
		// leaving nbCodes - 3 absent bytes for 1-byte short lengths [_LZPX_MIN_MATCH .. maxShort].
		maxShort := _LZPX_MIN_MATCH + nbCodes - 4

		if srcIdx == 0 {
			// The first four bytes have no preceding context, so emit them as literals.
			if count-dstIdx < 4 {
				return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
			}

			copy(dst[dstIdx:dstIdx+4], src[srcIdx:srcIdx+4])
			dstIdx += 4
			srcIdx += 4
		}

		// Main loop
		for srcIdx < end {
			length := 0
			h := lzpxHash(binary.LittleEndian.Uint32(src[srcIdx-4:]))
			entry := hashes[h]
			ref := int(entry >> 16)
			var tag uint16

			if srcIdx+2 <= count {
				tag = binary.LittleEndian.Uint16(src[srcIdx:])
			}

			if ref != 0 && end-srcIdx >= _LZPX_MIN_MATCH && uint16(entry) == tag {
				limit := end - srcIdx
				length = lzpxFindMatch(src, srcIdx, ref, limit)

				if stride == 8 && length >= _LZPX_MIN_MATCH && srcIdx+length+1 < end {
					h2 := lzpxHash(binary.LittleEndian.Uint32(src[srcIdx-3:]))
					entry2 := hashes[h2]
					ref2 := int(entry2 >> 16)

					if ref2 != 0 && uint16(entry2) == binary.LittleEndian.Uint16(src[srcIdx+1:]) {
						limit2 := end - srcIdx - 1
						length2 := lzpxFindMatch(src, srcIdx+1, ref2, limit2)

						if length2 > length {
							length = 0
						}
					}
				}
			}

			if dstIdx >= count {
				return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
			}

			hashes[h] = (uint64(srcIdx) << 16) | uint64(tag)

			if length < _LZPX_MIN_MATCH {
				dst[dstIdx] = src[srcIdx]
				dstIdx++
				srcIdx++
				continue
			}

			if length <= maxShort {
				dst[dstIdx] = codes[length-_LZPX_MIN_MATCH+1]
				dstIdx++
			} else {
				rem := length - (maxShort + 1)

				if rem < 256 {
					// Medium match [maxShort+1 .. maxShort+256]: prefix code + 1 byte (0..255).
					dst[dstIdx] = codes[nbCodes-2]
					dstIdx++

					if dstIdx >= count {
						return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
					}

					dst[dstIdx] = byte(rem)
					dstIdx++
				} else {
					// Long match >= maxShort+257: prefix code + LEB128 of (length - maxShort - 257).
					// Write 7 bits per byte (LSB first); set bit 7 (0x80) on all but the last byte.
					dst[dstIdx] = codes[nbCodes-1]
					dstIdx++
					rem -= 256

					for rem >= 128 {
						if dstIdx >= count {
							return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
						}

						dst[dstIdx] = byte((rem & 0x7F) | 0x80)
						dstIdx++
						rem >>= 7
					}

					if dstIdx >= count {
						return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
					}

					dst[dstIdx] = byte(rem)
					dstIdx++
				}
			}

			next := srcIdx + length

			// Sample hash positions at a fixed stride within this match.
			for hashPos := srcIdx + stride; hashPos < next; hashPos += stride {
				var t uint16

				if hashPos+2 <= count {
					t = binary.LittleEndian.Uint16(src[hashPos:])
				}

				hashes[lzpxHash(binary.LittleEndian.Uint32(src[hashPos-4:]))] = (uint64(hashPos) << 16) | uint64(t)
			}

			srcIdx = next
		}

		if dstIdx >= count {
			return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
		}

		dst[dstIdx] = codes[0]
		dstIdx++
	}

	if dstIdx > count-count/100 {
		return 0, 0, fmt.Errorf("LZPX forward transform skip: no compression")
	}

	return uint(count), uint(dstIdx), nil
}

// Inverse decodes an LZPX block whose minimum-match value is stored once in
// the block header and whose subchunk alphabets are stored independently.
func (this *LZPXCodec) Inverse(src, dst []byte) (uint, uint, error) {
	count := len(src)

	if count == 0 {
		return 0, 0, nil
	}

	if count < _LZPX_MIN_CHUNK_SIZE+1 {
		return 0, 0, errors.New("LZPX inverse transform failed: block too small")
	}

	if len(this.hashes) == 0 {
		this.hashes = make([]int32, 1<<_LZPX_HASH_LOG)
	} else {
		clear(this.hashes)
	}

	hashes := this.hashes
	dstEnd := len(dst)
	srcIdx := 0
	dstIdx := 0

	// The transform header carries the hash stride and minimum match length.
	header := int(src[srcIdx])
	srcIdx++
	stride := 1
	if header&0x80 != 0 {
		stride = 8
	}
	this.hashStride = stride
	minMatch := (header & 0x3F) + 2

	// The first four output bytes seed the hash context. Validate them
	// against the first chunk alphabet before entering the chunk loop.
	firstChunk := srcIdx

	if count-firstChunk < 32+4 {
		return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: truncated initial literals")
	}

	firstTokens := firstChunk + 32

	for i := 0; i < 4; i++ {
		val := uint(src[firstTokens+i])

		if (uint(src[firstChunk+(int(val)>>3)]) & (1 << (val & 7))) == 0 {
			return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: initial symbols must be literals")
		}
	}

	// Maintain the 4-byte hash context in a register (little-endian order)
	// to avoid store-to-load forwarding stalls on dst[dstIdx - 4].
	var ctx uint32
	var lengths [256]int

	for srcIdx < count {
		if count-srcIdx < _LZPX_MIN_CHUNK_SIZE {
			return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: truncated chunk")
		}

		for i := range lengths {
			lengths[i] = -1
		}

		nbCodes := 0
		firstCode := 0

		// Extract absent bytes (0 bits in the 256-bit bitmap) in ascending order.
		// Assign consecutive match lengths starting at (minMatch - 1) so that:
		// - 0th code (overwritten to 0 below) = EOB
		// - next codes = short matches [minMatch .. maxShort]
		// - second-to-last code = maxShort + 1 (base for 1-byte offset 0..255)
		// - last code = maxShort + 2 (marker for base maxShort + 257 + LEB128)
		for k := 0; k < 4; k++ {
			absent := ^binary.LittleEndian.Uint64(src[srcIdx+(k<<3):])

			for absent != 0 {
				code := (k << 6) | bits.TrailingZeros64(absent)
				lengths[code] = minMatch + nbCodes - 1

				if nbCodes == 0 {
					firstCode = code
				}

				nbCodes++
				absent &= absent - 1
			}
		}

		// Need EOB, at least 1 short length, and both extended-length codes.
		if nbCodes < 4 {
			return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: invalid alphabet")
		}

		lengths[firstCode] = 0
		maxShort := minMatch + nbCodes - 4
		srcIdx += 32
		blockStart := dstIdx
		endBlock := false

		// Main loop
		for srcIdx < count {
			val := src[srcIdx]
			srcIdx++
			length := lengths[val]

			if length == 0 {
				endBlock = true
				break
			}

			if length < 0 {
				if dstIdx >= dstEnd || dstIdx-blockStart >= _LZPX_CHUNK_SIZE {
					return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: output overflow")
				}

				if dstIdx >= 4 {
					hashes[lzpxHash(ctx)] = int32(dstIdx)
				}

				dst[dstIdx] = val
				dstIdx++
				ctx = (ctx >> 8) | (uint32(val) << 24)
				continue
			}

			// Decode lengths longer than maxShort:
			// - length == maxShort + 1: read 1 unsigned byte offset (adds 0..255).
			// - length == maxShort + 2: base is maxShort + 257, plus 7-bit LEB128 varint
			//   (at most 4 varint bytes, since _LZPX_CHUNK_SIZE < 2^28).
			if length > maxShort {
				if length == maxShort+1 {
					if srcIdx >= count {
						return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: truncated extended match")
					}

					length += int(src[srcIdx])
					srcIdx++

					if length > _LZPX_CHUNK_SIZE {
						return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: match out of range")
					}
				} else {
					length = maxShort + 1 + 256
					shift := uint(0)

					for {
						if srcIdx >= count || shift > 21 {
							return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: invalid extended match")
						}

						b := int(src[srcIdx])
						srcIdx++
						length += int(uint32(b&0x7F) << shift)

						if length > _LZPX_CHUNK_SIZE {
							return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: match out of range")
						}

						shift += 7

						if b < 128 {
							break
						}
					}
				}
			}

			h := lzpxHash(ctx)
			ref := int(hashes[h])

			if ref == 0 || ref >= dstIdx {
				return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: invalid reference")
			}

			if length > dstEnd-dstIdx || length > _LZPX_CHUNK_SIZE-(dstIdx-blockStart) {
				return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: match out of range")
			}

			hashes[h] = int32(dstIdx)
			dist := dstIdx - ref
			next := dstIdx + length

			if dist >= length {
				copy(dst[dstIdx:next], dst[ref:ref+length])
			} else if dist >= 8 && next+7 <= dstEnd {
				n := 0

				for n < length {
					binary.LittleEndian.PutUint64(dst[dstIdx+n:], binary.LittleEndian.Uint64(dst[ref+n:]))
					n += 8
				}
			} else {
				for n := 0; n < length; n++ {
					dst[dstIdx+n] = dst[ref+n]
				}
			}

			// Update sampled hash entries and advance rolling ctx. When dist >= length,
			// read from the source ref rather than dst to avoid store-to-load stalls.
			if stride > 1 {
				for p := dstIdx + stride; p < next; p += stride {
					hashes[lzpxHash(binary.LittleEndian.Uint32(dst[p-4:]))] = int32(p)
				}

				if dist >= length {
					if length >= 4 {
						ctx = binary.LittleEndian.Uint32(dst[ref+length-4:])
					} else if length == 2 {
						ctx = (ctx >> 16) | (uint32(binary.LittleEndian.Uint16(dst[ref:])) << 16)
					} else if length == 3 {
						ctx = (ctx >> 24) | (uint32(binary.LittleEndian.Uint32(dst[ref:])) << 8)
					} else {
						ctx = binary.LittleEndian.Uint32(dst[next-4:])
					}
				} else {
					ctx = binary.LittleEndian.Uint32(dst[next-4:])
				}
			} else {
				// hashes[h] for dstIdx was already updated above; advance n from 0 to
				// feed ctx before overwriting dstIdx + n for n >= 1.
				if dist >= length {
					for n := 0; n < length; n++ {
						hashes[lzpxHash(ctx)] = int32(dstIdx + n)
						ctx = (ctx >> 8) | (uint32(dst[ref+n]) << 24)
					}
				} else {
					for p := dstIdx + 1; p < next; p++ {
						hashes[lzpxHash(binary.LittleEndian.Uint32(dst[p-4:]))] = int32(p)
					}

					ctx = binary.LittleEndian.Uint32(dst[next-4:])
				}
			}

			dstIdx = next
		}

		if !endBlock || dstIdx == blockStart {
			return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: missing end marker")
		}
	}

	return uint(count), uint(dstIdx), nil
}

func lzpxHash(ctx uint32) uint32 {
	return (0x7FEB352D * (ctx ^ (ctx >> 16))) >> _LZPX_HASH_SHIFT
}

func lzpxFindMatch(src []byte, srcIdx, ref, maxMatch int) int {
	n := 0

	for n+8 <= maxMatch {
		diff := binary.LittleEndian.Uint64(src[srcIdx+n:]) ^ binary.LittleEndian.Uint64(src[ref+n:])

		if diff != 0 {
			return n + (bits.TrailingZeros64(diff) >> 3)
		}

		n += 8
	}

	if n+4 <= maxMatch {
		diff := binary.LittleEndian.Uint32(src[srcIdx+n:]) ^ binary.LittleEndian.Uint32(src[ref+n:])

		if diff != 0 {
			return n + (bits.TrailingZeros32(diff) >> 3)
		}

		n += 4
	}

	for n < maxMatch && src[srcIdx+n] == src[ref+n] {
		n++
	}

	return n
}
