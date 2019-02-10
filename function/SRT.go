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

import (
	"errors"
	"fmt"
)

// Sorted Ranks Transform is typically used after a BWT to reduce the variance
// of the data prior to entropy coding.

const (
	SRT_HEADER_SIZE = 4 * 256 // freqs
	SRT_CHUNK_SIZE  = 8 * 1024 * 1024
)

type SRT struct {
}

func NewSRT() (*SRT, error) {
	this := &SRT{}
	return this, nil
}

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
		var j int

		for j = i + 1; (j < count) && (src[j] == c); j++ {
		}

		if freqs[c] == 0 {
			r2s[b] = c
			s2r[c] = byte(b)
			b++
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

	headerSize, err := this.encodeHeader(freqs[:], dst)

	if err != nil {
		return 0, 0, err
	}

	dst = dst[headerSize:]

	// encoding
	for i := 0; i < count; {
		c := src[i]
		r := s2r[c]
		p := buckets[c]
		dst[p] = r
		p++

		if r > 0 {
			for r > 0 {
				r2s[r] = r2s[r-1]
				s2r[r2s[r]] = r
				r--
			}

			r2s[0] = c
			s2r[c] = 0
		}

		j := i + 1

		for (j < count) && (src[j] == c) {
			dst[p] = 0
			p++
			j++
		}

		buckets[c] = p
		i = j
	}

	return uint(count), uint(count + SRT_HEADER_SIZE), nil
}

func (this *SRT) preprocess(freqs []int32, symbols []byte) int {
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

			for b = i - h; (b >= 0) && ((freqs[symbols[b]] < freqs[t]) || ((freqs[t] == freqs[symbols[b]]) && (t < symbols[b]))); b -= h {
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

func (this *SRT) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	// init arrays
	freqs := [256]int32{}
	headerSize, err := this.decodeHeader(src, freqs[:])

	if err != nil {
		return 0, 0, err
	}

	src = src[headerSize:]
	count := len(src)
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

			for s := byte(0); s < r; s++ {
				r2s[s] = r2s[s+1]
			}

			r2s[r] = c
			c = r2s[0]
		} else {
			nbSymbols--

			if nbSymbols <= 0 {
				continue
			}

			for s := 0; s < nbSymbols; s++ {
				r2s[s] = r2s[s+1]
			}

			c = r2s[0]
		}
	}

	return uint(count + SRT_HEADER_SIZE), uint(count), nil
}

func (this SRT) encodeHeader(freqs []int32, dst []byte) (int, error) {
	if len(dst) < SRT_HEADER_SIZE {
		return 0, errors.New("SRT forward failed: cannot encode header")
	}

	for i := range freqs {
		dst[4*i] = byte(freqs[i] >> 24)
		dst[4*i+1] = byte(freqs[i] >> 16)
		dst[4*i+2] = byte(freqs[i] >> 8)
		dst[4*i+3] = byte(freqs[i])
	}

	return SRT_HEADER_SIZE, nil
}

func (this SRT) decodeHeader(src []byte, freqs []int32) (int, error) {
	if len(src) < SRT_HEADER_SIZE {
		return 0, errors.New("SRT inverse failed: cannot decode header")
	}

	for i := range freqs {
		f1 := int32(src[4*i])
		f2 := int32(src[4*i+1])
		f3 := int32(src[4*i+2])
		f4 := int32(src[4*i+3])
		freqs[i] = (f1 << 24) | (f2 << 16) | (f3 << 8) | f4
	}

	return SRT_HEADER_SIZE, nil
}

func (this SRT) MaxEncodedLen(srcLen int) int {
	return srcLen + SRT_HEADER_SIZE
}
