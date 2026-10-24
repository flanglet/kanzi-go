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
	_LZPX_HASH_LOG         = 17
	_LZPX_HASH_SHIFT       = 15
	_LZPX_CHUNK_SIZE       = 65536
	_LZPX_MIN_CHUNK_SIZE   = 33 // bitmap plus at least one end-of-block token
	_LZPX_MIN_ABSENT_CODES = 16
)

// LZPXCodec implements the Lempel-Ziv Predict Extra transform. The block
// header stores minMatch-2 once; each subchunk has its own literal alphabet.
type LZPXCodec struct {
	hashes []int32
}

// NewLZPXCodec creates an LZPX transform.
func NewLZPXCodec() (*LZPXCodec, error) {
	return &LZPXCodec{}, nil
}

// MaxEncodedLen returns the input size. Forward only succeeds when the
// transform saves at least one percent, so a larger output is rejected.
func (this *LZPXCodec) MaxEncodedLen(srcLen int) int {
	return srcLen
}

// Forward encodes the input using one minimum-match value per transform block
// and a literal alphabet for each internal chunk.
func (this *LZPXCodec) Forward(src, dst []byte) (uint, uint, error) {
	count := len(src)

	if count == 0 {
		return 0, 0, nil
	}

	if count < 4 || len(dst) < count {
		return 0, 0, errors.New("LZPX forward transform skip: invalid or insufficient buffer")
	}

	if len(this.hashes) == 0 {
		this.hashes = make([]int32, 1<<_LZPX_HASH_LOG)
	} else {
		clear(this.hashes)
	}

	const minMatch = 2
	dstIdx := 0
	dst[dstIdx] = byte(minMatch - 2)
	dstIdx++
	srcIdx := 0

	for srcIdx < count {
		size := count - srcIdx

		if size > _LZPX_CHUNK_SIZE {
			size = _LZPX_CHUNK_SIZE
		}

		var freqs [256]int
		alphabet := 0

		for i := 0; i < size; i++ {
			v := int(src[srcIdx+i])

			if freqs[v] == 0 {
				alphabet++
			}

			freqs[v]++
		}

		if alphabet > 256-_LZPX_MIN_ABSENT_CODES {
			clear(freqs[:])
			alphabet = 0
			size = 0

			for alphabet < 256-_LZPX_MIN_ABSENT_CODES {
				v := int(src[srcIdx+size])
				size++

				if freqs[v] == 0 {
					alphabet++
				}

				freqs[v]++
			}
		}

		if count-dstIdx < _LZPX_MIN_CHUNK_SIZE {
			return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
		}

		bitmapIdx := dstIdx
		clear(dst[dstIdx : dstIdx+32])
		var codes [256]byte
		nbCodes := 0

		for i := 0; i < 256; i++ {
			if freqs[i] == 0 {
				codes[nbCodes] = byte(i)
				nbCodes++
			} else {
				dst[bitmapIdx+(i>>3)] |= byte(1 << uint(i&7))
			}
		}

		dstIdx += 32
		end := srcIdx + size
		maxMatch := minMatch + nbCodes - 2

		if srcIdx == 0 {
			for srcIdx < 4 {
				if dstIdx >= count {
					return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
				}

				dst[dstIdx] = src[srcIdx]
				dstIdx++
				srcIdx++
			}
		}

		for srcIdx < end {
			h := lzpxHash(binary.LittleEndian.Uint32(src[srcIdx-4:]))
			ref := int(this.hashes[h])
			length := 0

			if ref != 0 {
				limit := maxMatch

				if end-srcIdx < limit {
					limit = end - srcIdx
				}

				length = lzpxFindMatch(src, srcIdx, ref, limit)
			}

			if dstIdx >= count {
				return 0, 0, errors.New("LZPX forward transform skip: output buffer too small")
			}

			if length < minMatch {
				dst[dstIdx] = src[srcIdx]
				dstIdx++
				this.hashes[lzpxHash(binary.LittleEndian.Uint32(src[srcIdx-4:]))] = int32(srcIdx)
				srcIdx++
				continue
			}

			dst[dstIdx] = codes[length-minMatch+1]
			dstIdx++
			next := srcIdx + length

			for srcIdx < next {
				this.hashes[lzpxHash(binary.LittleEndian.Uint32(src[srcIdx-4:]))] = int32(srcIdx)
				srcIdx++
			}
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

	minMatch := int(src[0]) + 2
	srcIdx := 1
	dstIdx := 0

	for srcIdx < count {
		if count-srcIdx < _LZPX_MIN_CHUNK_SIZE {
			return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: truncated chunk")
		}

		var lengths [256]int
		nbCodes := 0

		for i := 0; i < 256; i++ {
			if (src[srcIdx+(i>>3)] & byte(1<<uint(i&7))) != 0 {
				lengths[i] = -1
			} else {
				nbCodes++
				if nbCodes == 1 {
					lengths[i] = 0
				} else {
					lengths[i] = minMatch + nbCodes - 2
				}
			}
		}

		srcIdx += 32

		if nbCodes < 2 {
			return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: invalid alphabet")
		}

		blockStart := dstIdx
		endBlock := false

		if dstIdx == 0 {
			if count-srcIdx < 4 {
				return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: truncated initial literals")
			}

			for i := 0; i < 4; i++ {
				if lengths[src[srcIdx+i]] != -1 {
					return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: initial symbols must be literals")
				}
			}
		}

		for srcIdx < count {
			val := src[srcIdx]
			srcIdx++
			length := lengths[val]

			if length == 0 {
				endBlock = true
				break
			}

			if length < 0 {
				if dstIdx >= len(dst) || dstIdx-blockStart >= _LZPX_CHUNK_SIZE {
					return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: output overflow")
				}

				if dstIdx >= 4 {
					this.hashes[lzpxHash(binary.LittleEndian.Uint32(dst[dstIdx-4:]))] = int32(dstIdx)
				}

				dst[dstIdx] = val
				dstIdx++
				continue
			}

			ref := int(this.hashes[lzpxHash(binary.LittleEndian.Uint32(dst[dstIdx-4:]))])

			if ref == 0 || ref >= dstIdx {
				return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: invalid reference")
			}

			if length > len(dst)-dstIdx || length > _LZPX_CHUNK_SIZE-(dstIdx-blockStart) {
				return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: match out of range")
			}

			next := dstIdx + length

			for dstIdx < next {
				if dstIdx >= 4 {
					this.hashes[lzpxHash(binary.LittleEndian.Uint32(dst[dstIdx-4:]))] = int32(dstIdx)
				}

				dst[dstIdx] = dst[ref]
				dstIdx++
				ref++
			}
		}

		if !endBlock || dstIdx == blockStart {
			return uint(srcIdx), uint(dstIdx), errors.New("LZPX inverse transform failed: missing end marker")
		}
	}

	return uint(count), uint(dstIdx), nil
}

func lzpxHash(ctx uint32) uint32 {
	return (0x7FEB352D * ctx) >> _LZPX_HASH_SHIFT
}

func lzpxFindMatch(src []byte, srcIdx, ref, maxMatch int) int {
	n := 0

	for n+8 <= maxMatch {
		diff := binary.LittleEndian.Uint64(src[srcIdx+n:]) ^ binary.LittleEndian.Uint64(src[ref+n:])

		if diff != 0 {
			n += bits.TrailingZeros64(diff) >> 3
			break
		}

		n += 8
	}

	return n
}
