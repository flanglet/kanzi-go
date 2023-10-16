/*
Copyright 2011-2024 Frederic Langlet
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
	"errors"
	"fmt"
)

const (
	_SRT_MAX_HEADER_SIZE = 4 * 256
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
func NewSRTWithCtx(ctx *map[string]interface{}) (*SRT, error) {
	this := &SRT{}
	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *SRT) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	count := len(src)
	s2r := [256]byte{}
	r2s := [256]byte{}
	freqs := [256]int32{}

	// find first symbols and count occurrences
	for i, b := 0, 0; i < count; {
		c := src[i]

		if freqs[c] == 0 {
			r2s[b] = c
			s2r[c] = byte(b)
			b++
		}

		j := i + 1

		for (j < count) && (src[j] == c) {
			j++
		}

		freqs[c] += int32(j - i)
		i = j
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

		for (i < count) && (src[i] == c) {
			dst[p] = 0
			p++
			i++
		}

		buckets[c] = p
	}

	return uint(count), uint(count + headerSize), nil
}

func (this SRT) preprocess(freqs []int32, symbols []byte) int {
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

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	// init arrays
	freqs := [256]int32{}
	headerSize := this.decodeHeader(src, freqs[:])
	src = src[headerSize:]
	symbols := [256]byte{}
	nbSymbols := this.preprocess(freqs[:], symbols[:])
	buckets := [256]int{}
	bucketEnds := [256]int{}
	r2s := [256]byte{}

	for i, bucketPos := 0, 0; i < nbSymbols; i++ {
		c := symbols[i]
		r2s[src[bucketPos]] = c
		buckets[c] = bucketPos + 1
		bucketPos += int(freqs[c])
		bucketEnds[c] = bucketPos
	}

	// decoding
	c := r2s[0]

	for i := range dst {
		dst[i] = c

		if buckets[c] < bucketEnds[c] {
			r := src[buckets[c]]
			buckets[c]++

			if r == 0 {
				continue
			}

			s := 0

			for s+4 < int(r) {
				r2s[s] = r2s[s+1]
				r2s[s+1] = r2s[s+2]
				r2s[s+2] = r2s[s+3]
				r2s[s+3] = r2s[s+4]
				s += 4
			}

			for s < int(r) {
				r2s[s] = r2s[s+1]
				s++
			}

			r2s[r] = c
			c = r2s[0]
		} else {
			if nbSymbols == 1 {
				continue
			}

			nbSymbols--

			for s := 0; s < nbSymbols; s++ {
				r2s[s] = r2s[s+1]
			}

			c = r2s[0]
		}
	}

	return uint(len(src) + headerSize), uint(len(src)), nil
}

func (this SRT) encodeHeader(freqs []int32, dst []byte) int {
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

func (this SRT) decodeHeader(src []byte, freqs []int32) int {
	n := 0

	for i := range freqs {
		val := int32(src[n])
		n++

		if val < 128 {
			freqs[i] = val
			continue
		}

		res := val & 0x7F
		val = int32(src[n])
		n++
		res |= ((val & 0x7F) << 7)

		if val >= 128 {
			val = int32(src[n])
			n++
			res |= ((val & 0x7F) << 14)

			if val >= 128 {
				val = int32(src[n])
				n++
				res |= ((val & 0x7F) << 21)
			}
		}

		freqs[i] = res
	}

	return n
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this SRT) MaxEncodedLen(srcLen int) int {
	return srcLen + _SRT_MAX_HEADER_SIZE
}
