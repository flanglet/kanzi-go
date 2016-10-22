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
)

const (
	BWT_MAX_BLOCK_SIZE  = 1024 * 1024 * 1024 // 1 GB (30 bits)
	BWT_MAX_HEADER_SIZE = 4
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
// mississippi\0  0  -> 4             i\0
//  ississippi\0  1  -> 3          ippi\0
//   ssissippi\0  2  -> 10      issippi\0
//    sissippi\0  3  -> 8    ississippi\0
//     issippi\0  4  -> 2   mississippi\0
//      ssippi\0  5  -> 9            pi\0
//       sippi\0  6  -> 7           ppi\0
//        ippi\0  7  -> 1         sippi\0
//         ppi\0  8  -> 6      sissippi\0
//          pi\0  9  -> 5        ssippi\0
//           i\0  10 -> 0     ssissippi\0
// Suffix array SA : 10 7 4 1 0 9 8 6 3 5 2
// BWT[i] = input[SA[i]-1] => BWT(input) = pssm[i]pissii (+ primary index 4)
// The suffix array and permutation vector are equal when the input is 0 terminated
// The insertion of a guard is done internally and is entirely transparent.
//
// See https://code.google.com/p/libdivsufsort/source/browse/wiki/SACA_Benchmarks.wiki
// for respective performance of different suffix sorting algorithms.

type BWT struct {
	buffer1      []int
	buffer2      []byte // Only used for big blocks (size >= 1<<24)
	buckets      []int
	primaryIndex uint
	saAlgo       *DivSufSort
}

func NewBWT() (*BWT, error) {
	this := new(BWT)
	this.buffer1 = make([]int, 0)  // Allocate empty: only used in inverse
	this.buffer2 = make([]byte, 0) // Allocate empty: only used for big blocks (size >= 1<<24)
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

func (this *BWT) Forward(src, dst []byte) (uint, uint, error) {
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

	if this.saAlgo == nil {
		var err error

		if this.saAlgo, err = NewDivSufSort(); err != nil {
			return 0, 0, err
		}
	} else {
		this.saAlgo.Reset()
	}

	// Compute suffix array
	sa := this.saAlgo.ComputeSuffixArray(src[0:count])
	i := 0

	for i < count {
		// Found primary index
		if sa[i] == 0 {
			break
		}

		dst[i] = src[sa[i]-1]
		i++
	}

	dst[i] = src[count-1]
	this.SetPrimaryIndex(uint(i))
	i++

	for i < count {
		dst[i] = src[sa[i]-1]
		i++
	}

	return uint(count), uint(count), nil
}

func (this *BWT) Inverse(src, dst []byte) (uint, uint, error) {
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

	if count < 2 {
		if count == 1 {
			dst[0] = src[0]
		}

		return uint(count), uint(count), nil
	}

	if count >= 1<<24 {
		return this.inverseBigBlock(src, dst, count)
	}

	return this.inverseRegularBlock(src, dst, count)
}

// When count < 1<<24
func (this *BWT) inverseRegularBlock(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocation
	if len(this.buffer1) < count {
		this.buffer1 = make([]int, count)
	}

	// Aliasing
	buckets_ := this.buckets
	data := this.buffer1

	// Create histogram
	for i := range this.buckets {
		buckets_[i] = 0
	}

	// Build array of packed index + value (assumes block size < 2^24)
	// Start with the primary index position
	pIdx := int(this.PrimaryIndex())
	val0 := int(src[pIdx])
	data[pIdx] = val0
	buckets_[val0]++

	for i := 0; i < pIdx; i++ {
		val := int(src[i])
		data[i] = (buckets_[val] << 8) | val
		buckets_[val]++
	}

	for i := pIdx + 1; i < count; i++ {
		val := int(src[i])
		data[i] = (buckets_[val] << 8) | val
		buckets_[val]++
	}

	sum := 0

	for i := range buckets_ {
		sum += buckets_[i]
		buckets_[i] = sum - buckets_[i]
	}

	ptr := data[pIdx]
	dst[count-1] = byte(ptr)

	// Build inverse
	for i := count - 2; i >= 0; i-- {
		ptr = data[uint(ptr>>8)+uint(buckets_[ptr&0xFF])]
		dst[i] = byte(ptr)
	}

	return uint(count), uint(count), nil
}

// When count >= 1<<24
func (this *BWT) inverseBigBlock(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocations
	if len(this.buffer1) < count {
		this.buffer1 = make([]int, count)
	}

	if len(this.buffer2) < count {
		this.buffer2 = make([]byte, count)
	}

	// Aliasing
	buckets_ := this.buckets
	data1 := this.buffer1
	data2 := this.buffer2

	// Create histogram
	for i := range this.buckets {
		buckets_[i] = 0
	}

	// Build arrays
	// Start with the primary index position
	pIdx := int(this.PrimaryIndex())
	val0 := src[pIdx]
	data1[pIdx] = buckets_[val0]
	data2[pIdx] = val0
	buckets_[val0]++

	for i := 0; i < pIdx; i++ {
		val := src[i]
		data1[i] = buckets_[val]
		data2[i] = val
		buckets_[val]++
	}

	for i := pIdx + 1; i < count; i++ {
		val := src[i]
		data1[i] = buckets_[val]
		data2[i] = val
		buckets_[val]++
	}

	sum := 0

	// Create cumulative histogram
	for i := range buckets_ {
		tmp := buckets_[i]
		buckets_[i] = sum
		sum += tmp
	}

	val1 := data1[pIdx]
	val2 := data2[pIdx]
	dst[count-1] = val2

	// Build inverse
	for i := count - 2; i >= 0; i-- {
		idx := val1 + buckets_[val2]
		val1 = data1[idx]
		val2 = data2[idx]
		dst[i] = val2
	}

	return uint(count), uint(count), nil
}

func maxBWTBlockSize() int {
	return BWT_MAX_BLOCK_SIZE - BWT_MAX_HEADER_SIZE
}
