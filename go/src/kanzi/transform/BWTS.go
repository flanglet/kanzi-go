/*
Copyright 2011-2013 Frederic Langlet
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
	"kanzi"
	"kanzi/util"
)

// Bijective version of the Burrows-Wheeler Transform
// The main advantage over the regular BWT is that there is no need for a primary
// index (hence the bijectivity). BWTS is about 10% slower than BWT.
// Forward transform based on the code at https://code.google.com/p/mk-bwts/
// by Neal Burns and DivSufSort (port of libDivSufSort by Yuta Mori)

const (
	BWTS_MAX_BLOCK_SIZE = 1024 * 1024 * 1024 // 1 GB (30 bits)
)

type BWTS struct {
	buffer  []int
	buckets []int
	saAlgo  *util.DivSufSort
}

func NewBWTS() (*BWTS, error) {
	this := new(BWTS)
	this.buffer = make([]int, 0)
	this.buckets = make([]int, 256)
	return this, nil
}

func (this *BWTS) Forward(src, dst []byte, length uint) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := int(length)

	if count > maxBWTSBlockSize() {
		errMsg := fmt.Sprintf("Block size is %v, max value is %v", count, maxBWTSBlockSize())
		return 0, 0, errors.New(errMsg)
	}

	if count > len(src) {
		errMsg := fmt.Sprintf("Block size is %v, input buffer length is %v", count, len(src))
		return 0, 0, errors.New(errMsg)
	}

	if count < 2 {
		if count == 1 {
			dst[0] = src[0]
		}

		return uint(count), uint(count), nil
	}

	// Lazy dynamic memory allocations
	if len(this.buffer) < count {
		this.buffer = make([]int, count)
	}

	if this.saAlgo == nil {
		var err error

		if this.saAlgo, err = util.NewDivSufSort(); err != nil {
			return 0, 0, err
		}
	} else {
		this.saAlgo.Reset()
	}

	// Compute suffix array
	sa := this.saAlgo.ComputeSuffixArray(src[0:count])

	// Aliasing
	isa := this.buffer

	for i := 0; i < count; i++ {
		isa[sa[i]] = i
	}

	min := isa[0]
	idxMin := 0

	for i := 1; i < count && min > 0; i++ {
		if isa[i] >= min {
			continue
		}

		headRank := this.moveLyndonWordHead(sa, src, count, idxMin, i-idxMin, min)
		refRank := headRank

		for j := i - 1; j > idxMin; j-- {
			// iterate through the new lyndon word from end to start
			testRank := isa[j]
			startRank := testRank

			for testRank < count-1 {
				nextRankStart := sa[testRank+1]

				if j > nextRankStart || src[j] != src[nextRankStart] || refRank < isa[nextRankStart+1] {
					break
				}

				sa[testRank] = nextRankStart
				isa[nextRankStart] = testRank
				testRank++
			}

			sa[testRank] = j
			isa[j] = testRank
			refRank = testRank

			if startRank == testRank {
				break
			}
		}

		min = isa[i]
		idxMin = i
	}

	min = count

	for i := 0; i < count; i++ {
		if isa[i] >= min {
			dst[isa[i]] = src[i-1]
			continue
		}

		if min < count {
			dst[min] = src[i-1]
		}

		min = isa[i]
	}

	dst[0] = src[count-1]
	return uint(count), uint(count), nil
}

func (this *BWTS) moveLyndonWordHead(sa []int, data []byte, count, start, size, rank int) int {
	isa := this.buffer
	end := start + size

	for rank+1 < count {
		nextStart0 := sa[rank+1]

		if nextStart0 <= end {
			break
		}

		nextStart := nextStart0
		k := 0

		for k < size && nextStart < count && data[start+k] == data[nextStart] {
			k++
			nextStart++
		}

		if k == size && rank < isa[nextStart] {
			break
		}

		if k < size && nextStart < count && data[start+k] < data[nextStart] {
			break
		}

		sa[rank] = nextStart0
		isa[nextStart0] = rank
		rank++
	}

	sa[rank] = start
	isa[start] = rank
	return rank
}

func (this *BWTS) Inverse(src, dst []byte, length uint) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := int(length)

	if count > maxBWTSBlockSize() {
		errMsg := fmt.Sprintf("Block size is %v, max value is %v", length, maxBWTSBlockSize())
		return 0, 0, errors.New(errMsg)
	}

	if count < 2 {
		if count == 1 {
			dst[0] = src[0]
		}

		return uint(count), uint(count), nil
	}

	// Lazy dynamic memory allocation
	if len(this.buffer) < count {
		this.buffer = make([]int, count)
	}

	// Aliasing
	buckets_ := this.buckets
	lf := this.buffer

	// Initialize histogram
	for i := range this.buckets {
		buckets_[i] = 0
	}

	for i := 0; i < count; i++ {
		buckets_[src[i]]++
	}

	// Histogram
	for i, j := 0, 0; i < 256; i++ {
		t := buckets_[i]
		buckets_[i] = j
		j += t
	}

	for i := 0; i < count; i++ {
		lf[i] = buckets_[src[i]]
		buckets_[src[i]]++
	}

	// Build inverse
	for i, j := 0, count-1; j >= 0; i++ {
		if lf[i] < 0 {
			continue
		}

		p := i

		for {
			dst[j] = src[p]
			j--
			t := lf[p]
			lf[p] = -1
			p = t

			if lf[p] < 0 {
				break
			}
		}
	}

	return uint(count), uint(count), nil
}

func maxBWTSBlockSize() int {
	return BWTS_MAX_BLOCK_SIZE
}
