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

// Behemoth Rank Transform
// A strong rank transform based on https://github.com/loxxous/Behemoth-Rank-Coding
// by Lucas Marsh. Typically used post BWT to reduce the variance of the data
// prior to entropy coding.

const (
	BRT_HEADER_SIZE = 1024 + 1 // 4*256 freqs + 1 nbSymbols
)

type BRT struct {
}

func NewBRT() (*BRT, error) {
	this := new(BRT)
	return this, nil
}

func (this *BRT) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	length := len(src)
	freqs := [256]int32{0}
	ranks := [256]int32{0}
	buckets := [256]int32{0}
	sortedMap := [256]byte{0}

	for i := range ranks {
		ranks[i] = int32(i)
	}

	nbSymbols := this.computeFrequencies(src, freqs[:], ranks[:])
	idx := this.encodeHeader(dst, freqs[:], nbSymbols)

	if idx < 0 {
		return 0, 0, errors.New("BRT forward failed: cannot encode header")
	}

	sortBRTMap(freqs[:], sortedMap[:])

	for i, bucketPos := 0, int32(0); i < nbSymbols; i++ {
		val := sortedMap[i]
		buckets[val] = bucketPos
		bucketPos += freqs[val]
	}

	dst = dst[idx:]

	for i := 0; i < length; i++ {
		s := src[i]
		r := ranks[s]
		dst[buckets[s]] = byte(r)
		buckets[s]++

		if r != 0 {
			for j := 0; j < 256; j += 8 {
				ranks[j] -= ((ranks[j] - r) >> 31)
				ranks[j+1] -= ((ranks[j+1] - r) >> 31)
				ranks[j+2] -= ((ranks[j+2] - r) >> 31)
				ranks[j+3] -= ((ranks[j+3] - r) >> 31)
				ranks[j+4] -= ((ranks[j+4] - r) >> 31)
				ranks[j+5] -= ((ranks[j+5] - r) >> 31)
				ranks[j+6] -= ((ranks[j+6] - r) >> 31)
				ranks[j+7] -= ((ranks[j+7] - r) >> 31)
			}

			ranks[s] = 0
		}
	}

	return uint(length), uint(length + idx), nil
}

func (this *BRT) computeFrequencies(block []byte, freqs, ranks []int32) int {
	n := 0
	nbSymbols := 0

	// Slow loop
	for n < len(block) {
		s := block[n]

		if freqs[s] == 0 {
			ranks[s] = int32(nbSymbols)
			nbSymbols++

			if nbSymbols == 256 {
				break
			}
		}

		freqs[s]++
		n++
	}

	// Fast loop
	for n < len(block) {
		freqs[block[n]]++
		n++
	}

	return nbSymbols
}

func (this *BRT) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	length := len(src)
	freqs := [256]int32{0}
	ranks := [256]int32{0}
	buckets := [256]int32{0}
	bucketEnds := [256]int32{0}
	sortedMap := [256]byte{0}
	total := 0
	nbSymbols := 0

	idx := this.decodeHeader(src, freqs[:], &nbSymbols, &total)

	if idx < 0 {
		return 0, 0, errors.New("BRT inverse failed: cannot decode header")
	}

	if total+idx != length {
		return 0, 0, errors.New("BRT inverse failed: invalid header")
	}

	if nbSymbols == 0 {
		return uint(idx), 0, nil
	}

	sortBRTMap(freqs[:], sortedMap[:])
	src = src[idx:]

	for i, bucketPos := 0, int32(0); i < nbSymbols; i++ {
		s := sortedMap[i]
		ranks[src[bucketPos]] = int32(s)
		buckets[s] = bucketPos + 1
		bucketPos += freqs[s]
		bucketEnds[s] = bucketPos
	}

	s := ranks[0]
	count := length - idx

	for i := 0; i < count; i++ {
		dst[i] = byte(s)
		r := 0xFF

		if buckets[s] < bucketEnds[s] {
			r = int(src[buckets[s]])
			buckets[s]++

			if r == 0 {
				continue
			}
		}

		ranks[0] = ranks[1]

		for j := 1; j < r; j++ {
			ranks[j] = ranks[j+1]
		}

		ranks[r] = s
		s = ranks[0]
	}

	return uint(length + idx), uint(length), nil
}

func sortBRTMap(freqs []int32, sortedMap []byte) {
	var newFreqs [256]int32
	copy(newFreqs[:], freqs[0:256])

	for j := range newFreqs {
		max := newFreqs[0]
		bsym := 0

		for i := 1; i < 256; i++ {
			if newFreqs[i] <= max {
				continue
			}

			bsym = i
			max = newFreqs[i]
		}

		if max == 0 {
			break
		}

		sortedMap[j] = byte(bsym)
		newFreqs[bsym] = 0
	}
}

func (this BRT) encodeHeader(block []byte, freqs []int32, nbSymbols int) int {
	// Require enough space in output block
	if len(block) < 4*len(freqs)+1 {
		return -1
	}

	blkptr := 0
	block[blkptr] = byte(nbSymbols - 1)
	blkptr++

	for _, f := range freqs {
		for f >= 0x80 {
			block[blkptr] = byte(0x80 | (f & 0x7F))
			blkptr++
			f >>= 7
		}

		block[blkptr] = byte(f)
		blkptr++

		if f > 0 {
			nbSymbols--

			if nbSymbols == 0 {
				break
			}
		}
	}

	return blkptr
}

func (this BRT) decodeHeader(block []byte, freqs []int32, nbSymbols, total *int) int {
	// Require enough space in arrays
	if (len(freqs) < 256) || (len(block) == 0) {
		return -1
	}

	blkptr := 0
	symbs := 1 + int(block[blkptr])
	blkptr++
	*nbSymbols = symbs
	tot := int32(0)

	for i := 0; i < 256; i++ {
		f := int32(block[blkptr])
		blkptr++
		res := f & 0x7F
		shift := uint(7)

		for (f >= 0x80) && (shift <= 28) {
			f = int32(block[blkptr])
			blkptr++
			res |= ((f & 0x7F) << shift)
			shift += 7
		}

		if (freqs[i] == 0) && (res != 0) {
			symbs--

			if symbs == 0 {
				freqs[i] = res
				tot += res
				break
			}
		}

		freqs[i] = res
		tot += res
	}

	*total = int(tot)
	return blkptr
}

func (this BRT) MaxEncodedLen(srcLen int) int {
	return srcLen + BRT_HEADER_SIZE
}
