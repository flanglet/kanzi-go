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
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"

	"github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_SRT_MAX_HEADER_SIZE = 5 * 256
)

// SRT Sorted Ranks Transform
// Sorted Ranks Transform is typically used after a BWT to reduce the variance
// of the data prior to entropy coding.
type SRT struct {
}

// NewSRT creates a new instance of SRT
func NewSRT() (*SRT, error) {
	this := &SRT{}
	return this, nil
}

// NewSRTWithCtx creates a new instance of SRT using a
// configuration map as parameter.
func NewSRTWithCtx(ctx *map[string]any) (*SRT, error) {
	this := &SRT{}
	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *SRT) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if len(dst)-_SRT_MAX_HEADER_SIZE < len(src) {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), this.MaxEncodedLen(len(src)))
	}

	count := len(src)
	s2r := [256]byte{}
	r2s := [256]byte{}
	freqs := [256]int{}

	// Count frequencies separately from finding the initial move-to-front order.
	internal.ComputeHistogram(src, freqs[:], true, false)
	first := [256]int{}
	n := 0
	for c, freq := range freqs {
		if uint64(freq) >= 1<<35 {
			return 0, 0, errors.New("SRT forward transform failed: frequency exceeds five bytes")
		}

		if freq == 0 {
			continue
		}
		pos := bytes.IndexByte(src, byte(c))
		first[c] = pos
		j := n
		for j > 0 && first[r2s[j-1]] > pos {
			r2s[j] = r2s[j-1]
			j--
		}
		r2s[j] = byte(c)
		n++
	}
	for i, c := range r2s[:n] {
		s2r[c] = byte(i)
	}

	// init arrays
	symbols := [256]byte{}
	nbSymbols := this.preprocess(freqs[:], symbols[:])
	buckets := [256]int{}

	for i, bucketPos := 0, 0; i < nbSymbols; i++ {
		c := symbols[i]
		buckets[c] = bucketPos
		bucketPos += int(freqs[c])
	}

	headerSize := this.encodeHeader(freqs[:], dst)
	dst = dst[headerSize:]

	// encoding
	for i := 0; i < count; {
		c := src[i]
		r := s2r[c]
		p := buckets[c]
		dst[p] = r
		p++

		if r > 0 {
			for {
				t := r2s[r-1]
				r2s[r], s2r[t] = t, r

				if r == 1 {
					break
				}

				r--
			}

			r2s[0] = c
			s2r[c] = 0
		}

		i++
		if i < count && src[i] == c {
			end := srtRunEnd(src, i+1, count, c)
			clear(dst[p : p+end-i])
			p += end - i
			i = end
		}

		buckets[c] = p
	}

	return uint(count), uint(count + headerSize), nil
}

// srtRunEnd returns the end of an identical-byte run.
func srtRunEnd(src []byte, pos, end int, value byte) int {
	word := uint64(value) * 0x0101010101010101
	for end-pos >= 8 {
		diff := binary.LittleEndian.Uint64(src[pos:]) ^ word
		if diff != 0 {
			return pos + bits.TrailingZeros64(diff)/8
		}
		pos += 8
	}
	for pos < end && src[pos] == value {
		pos++
	}
	return pos
}

func (this SRT) preprocess(freqs []int, symbols []byte) int {
	nbSymbols := 0

	for i := range freqs {
		if freqs[i] == 0 {
			continue
		}

		symbols[nbSymbols] = byte(i)
		nbSymbols++
	}

	h := 4

	for h < nbSymbols {
		h = h*3 + 1
	}

	for {
		h /= 3

		for i := h; i < nbSymbols; i++ {
			t := symbols[i]
			var b int

			for b = i - h; (b >= 0) && (freqs[symbols[b]] < freqs[t] || (t < symbols[b] && freqs[t] == freqs[symbols[b]])); b -= h {
				symbols[b+h] = symbols[b]
			}

			symbols[b+h] = t
		}

		if h == 1 {
			break
		}
	}

	return nbSymbols
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *SRT) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if len(dst) == 0 {
		return 0, 0, errors.New("SRT inverse transform failed: invalid data")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	// init arrays
	freqs := [256]int{}
	headerSize, err := this.decodeHeader(src, freqs[:])

	if err != nil {
		return 0, 0, err
	}

	dataLength := len(src) - headerSize

	// Empty input is represented by a zero-length transform block, not a header-only block.
	if dataLength <= 0 {
		return 0, 0, errors.New("SRT inverse transform failed: invalid data")
	}

	var totalFreq uint64

	for _, freq := range freqs {
		if freq < 0 {
			return 0, 0, errors.New("SRT inverse transform failed: invalid data")
		}

		totalFreq += uint64(freq)
	}

	if totalFreq != uint64(dataLength) {
		return 0, 0, errors.New("SRT inverse transform failed: invalid data")
	}

	src = src[headerSize:]

	if len(src) > len(dst) {
		return 0, 0, errors.New("SRT inverse transform failed: invalid data")
	}

	symbols := [256]byte{}
	nbSymbols := this.preprocess(freqs[:], symbols[:])
	buckets := [256]int{}
	bucketEnds := [256]int{}
	r2s := [256]byte{}

	for i, bucketPos := 0, 0; i < nbSymbols; i++ {
		c := symbols[i]

		if freqs[c] <= 0 || bucketPos < 0 || bucketPos >= len(src) {
			return 0, 0, errors.New("SRT inverse transform failed: invalid data")
		}

		r2s[src[bucketPos]] = c
		buckets[c] = bucketPos + 1
		bucketPos += int(freqs[c])

		if bucketPos > len(src) {
			return 0, 0, errors.New("SRT inverse transform failed: invalid data")
		}

		bucketEnds[c] = bucketPos

		if i == nbSymbols-1 && bucketPos != len(src) {
			return 0, 0, errors.New("SRT inverse transform failed: invalid data")
		}
	}

	// decoding
	c := r2s[0]

	for i := 0; i < dataLength; {
		pos := buckets[c]
		if pos < bucketEnds[c] && src[pos] == 0 {
			end := srtRunEnd(src, pos+1, bucketEnds[c], 0)
			run := end - pos
			if run >= dataLength-i {
				return 0, 0, errors.New("SRT inverse transform failed: invalid data")
			}
			word := uint64(c) * 0x0101010101010101
			limit := i + run
			for limit-i >= 8 {
				binary.LittleEndian.PutUint64(dst[i:], word)
				i += 8
			}
			for i < limit {
				dst[i] = c
				i++
			}
			buckets[c] = end
			pos = end
		}

		dst[i] = c
		i++
		if pos < bucketEnds[c] {
			r := src[pos]
			buckets[c]++
			copy(r2s[:r], r2s[1:int(r)+1])
			r2s[r] = c
			c = r2s[0]
		} else if nbSymbols > 1 {
			nbSymbols--
			copy(r2s[:nbSymbols], r2s[1:nbSymbols+1])
			c = r2s[0]
		}
	}

	return uint(len(src) + headerSize), uint(len(src)), nil
}

func (this SRT) encodeHeader(freqs []int, dst []byte) int {
	n := 0

	for _, f := range freqs {
		for f >= 128 {
			dst[n] = byte(0x80 | (f & 0x7F))
			n++
			f >>= 7
		}

		dst[n] = byte(f)
		n++
	}

	return n
}

func (this SRT) decodeHeader(src []byte, freqs []int) (int, error) {
	n := 0

	for i := range freqs {
		if n >= len(src) {
			return 0, errors.New("SRT inverse transform failed: truncated header")
		}

		val := int(src[n])
		n++

		if val < 128 {
			freqs[i] = val
			continue
		}

		res := val & 0x7F

		if n >= len(src) {
			return 0, errors.New("SRT inverse transform failed: truncated header")
		}

		val = int(src[n])
		n++
		res |= ((val & 0x7F) << 7)

		if val >= 128 {
			if n >= len(src) {
				return 0, errors.New("SRT inverse transform failed: truncated header")
			}

			val = int(src[n])
			n++
			res |= ((val & 0x7F) << 14)

			if val >= 128 {
				if n >= len(src) {
					return 0, errors.New("SRT inverse transform failed: truncated header")
				}

				val = int(src[n])
				n++
				res |= ((val & 0x7F) << 21)

				if val >= 128 {
					if n >= len(src) {
						return 0, errors.New("SRT inverse transform failed: truncated header")
					}

					val = int(src[n])
					n++

					if val >= 128 || val > math.MaxInt>>28 {
						return 0, errors.New("SRT inverse transform failed: invalid header")
					}

					res |= val << 28
				}
			}
		}

		freqs[i] = res
	}

	return n, nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *SRT) MaxEncodedLen(srcLen int) int {
	if srcLen > math.MaxInt-_SRT_MAX_HEADER_SIZE {
		return math.MaxInt
	}

	return srcLen + _SRT_MAX_HEADER_SIZE
}
