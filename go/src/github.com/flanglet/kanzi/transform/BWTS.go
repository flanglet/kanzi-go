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

package transform

import (
	"errors"
	"fmt"
	kanzi "github.com/flanglet/kanzi"
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
	buffer1 []int
	buffer2 []int
	buckets []int
	saAlgo  *DivSufSort
}

func NewBWTS() (*BWTS, error) {
	this := new(BWTS)
	this.buffer1 = make([]int, 0)
	this.buffer2 = make([]int, 0)
	this.buckets = make([]int, 256)
	return this, nil
}

func (this *BWTS) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count > maxBWTBlockSize() {
		errMsg := fmt.Sprintf("Block size is %v, max value is %v", count, maxBWTBlockSize())
		return 0, 0, errors.New(errMsg)
	}

	if count > len(dst) {
		errMsg := fmt.Sprintf("Block size is %v, output buffer length is %v", count, len(src))
		return 0, 0, errors.New(errMsg)
	}

	if count < 2 {
		if count == 1 {
			dst[0] = src[0]
		}

		return uint(count), uint(count), nil
	}

	if this.saAlgo == nil {
		var err error

		if this.saAlgo, err = NewDivSufSort(); err != nil {
			return 0, 0, err
		}
	}

	// Lazy dynamic memory allocations
	if len(this.buffer1) < count {
		this.buffer1 = make([]int, count)
	}

	if len(this.buffer2) < count {
		this.buffer2 = make([]int, count)
	}

	// Aliasing
	sa := this.buffer1[0:count]
	isa := this.buffer2[0:count]

	this.saAlgo.ComputeSuffixArray(src[0:count], sa)

	for i := range isa {
		isa[sa[i]] = i
	}

	min := isa[0]
	idxMin := 0

	for i := 1; i < count && min > 0; i++ {
		if isa[i] >= min {
			continue
		}

		refRank := this.moveLyndonWordHead(sa, isa, src, count, idxMin, i-idxMin, min)

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

func (this *BWTS) moveLyndonWordHead(sa, isa []int, data []byte, count, start, size, rank int) int {
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

func (this *BWTS) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count > maxBWTBlockSize() {
		errMsg := fmt.Sprintf("Block size is %v, max value is %v", count, maxBWTBlockSize())
		return 0, 0, errors.New(errMsg)
	}

	if count > len(dst) {
		errMsg := fmt.Sprintf("Block size is %v, output buffer length is %v", count, len(src))
		return 0, 0, errors.New(errMsg)
	}

	if count < 2 {
		if count == 1 {
			dst[0] = src[0]
		}

		return uint(count), uint(count), nil
	}

	// Lazy dynamic memory allocation
	if len(this.buffer1) < count {
		this.buffer1 = make([]int, count)
	}

	// Aliasing
	buckets_ := this.buckets
	lf := this.buffer1

	// Initialize histogram
	for i := range this.buckets {
		buckets_[i] = 0
	}

	for i := 0; i < count; i++ {
		buckets_[src[i]]++
	}

	sum := 0

	// Histogram
	for i := range buckets_ {
		sum += buckets_[i]
		buckets_[i] = sum - buckets_[i]
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
