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
	"kanzi/util"
)

// The Burrows-Wheeler Transform is a reversible transform based on
// permutation of the data in the original message to reduce the entropy.

// The initial text can be found here:
// Burrows M and Wheeler D, [A block sorting lossless data compression algorithm]
// Technical Report 124, Digital Equipment Corporation, 1994

// See also Peter Fenwick, [Block sorting text compression - final report]
// Technical Report 130, 1996

// This implementation replaces the 'slow' sorting of permutation strings
// with the construction of a suffix array (faster but more complex).
// The suffix array contains the indexes of the sorted suffixes.
//
// E.G.    0123456789A
// Source: mississippi\0
// Suffixes:    rank  sorted
// mississippi\0  0  -> 4
//  ississippi\0  1  -> 3
//   ssissippi\0  2  -> 10
//    sissippi\0  3  -> 8
//     issippi\0  4  -> 2
//      ssippi\0  5  -> 9
//       sippi\0  6  -> 7
//        ippi\0  7  -> 1
//         ppi\0  8  -> 6
//          pi\0  9  -> 5
//           i\0  10 -> 0
// Suffix array        10 7 4 1 0 9 8 6 3 5 2 => ipss\0mpissii (+ primary index 4)
// The suffix array and permutation vector are equal when the input is 0 terminated
// In this example, for a non \0 terminated string the output is pssmipissii.
// The insertion of a guard is done internally and is entirely transparent.
//
// See https://code.google.com/p/libdivsufsort/source/browse/wiki/SACA_Benchmarks.wiki
// for respective performance of different suffix sorting algorithms.

type BWT struct {
	size         uint
	buffer       []int
	buckets      []int
	primaryIndex uint
	saAlgo       *util.DivSufSort
}

func NewBWT(sz uint) (*BWT, error) {
	this := new(BWT)
	this.size = sz
	this.buffer = make([]int, sz+1) // (SA algo requires sz+1 bytes)
	this.buckets = make([]int, 256)
	return this, nil
}

func (this *BWT) PrimaryIndex() uint {
	return this.primaryIndex
}

func (this *BWT) SetPrimaryIndex(primaryIndex uint) bool {
	if primaryIndex < 0 {
		return false
	}

	this.primaryIndex = primaryIndex
	return true
}

func (this *BWT) Size() uint {
	return this.size
}

func (this *BWT) SetSize(sz uint) {
	this.size = sz
}

func (this *BWT) Forward(src, dst []byte) (uint, uint, error) {
	count := int(this.size)

	if this.size == 0 {
		count = len(src)
	}

	if count < 2 {
		if count == 1 {
			dst[0] = src[0]
		}

		return uint(count), uint(count), nil
	}

	// Lazy dynamic memory allocation (SA algo requires count+1 bytes)
	if len(this.buffer) < count+1 {
		this.buffer = make([]int, count+1)
	}

	// Aliasing
	data := this.buffer

	if this.saAlgo == nil {
		err := error(nil)
		this.saAlgo, err = util.NewDivSufSort() // lazy instantiation

		if err != nil {
			return 0, 0, err
		}
	} else {
		this.saAlgo.Reset()
	}

	for i := 0; i < count; i++ {
		data[i] = int(src[i])
	}

	data[count] = data[0]

	// Compute suffix array
	sa := this.saAlgo.ComputeSuffixArray(data[0:count])
	dst[0] = byte(data[count-1])

	i := 0

	for i < count {
		if sa[i] == 0 {
			// Found primary index
			this.SetPrimaryIndex(uint(i))
			i++
			break
		}

		dst[i+1] = src[sa[i]-1]
		i++
	}

	for i < count {
		dst[i] = src[sa[i]-1]
		i++
	}

	return uint(count), uint(count), nil
}

func (this *BWT) Inverse(src, dst []byte) (uint, uint, error) {
	count := int(this.size)

	if this.size == 0 {
		count = len(src)
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
	data := this.buffer

	// Create histogram
	for i := range this.buckets {
		buckets_[i] = 0
	}

	// Build array of packed index + value (assumes block size < 2^24)
	// Start with the primary index position
	pIdx := int(this.PrimaryIndex())
	val := int(src[0])
	data[pIdx] = (buckets_[val] << 8) | val
	buckets_[val]++

	for i := 0; i < pIdx; i++ {
		val = int(src[i+1])
		data[i] = (buckets_[val] << 8) | val
		buckets_[val]++
	}

	for i := pIdx + 1; i < count; i++ {
		val = int(src[i])
		data[i] = (buckets_[val] << 8) | val
		buckets_[val]++
	}

	sum := 0

	// Create cumulative histogram
	for i := range buckets_ {
		tmp := buckets_[i]
		buckets_[i] = sum
		sum += tmp
	}

	idx := pIdx

	// Build inverse
	for i := count - 1; i >= 0; i-- {
		ptr := data[idx]
		dst[i] = byte(ptr)
		idx = (ptr >> 8) + buckets_[ptr&0xFF]
	}

	return uint(count), uint(count), nil
}
