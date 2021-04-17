/*
Copyright 2011-2021 Frederic Langlet
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
	"sync"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	_BWT_MAX_BLOCK_SIZE        = 1024 * 1024 * 1024 // 1 GB
	_BWT_NB_FASTBITS           = 17
	_BWT_MASK_FASTBITS         = 1 << _BWT_NB_FASTBITS
	_BWT_BLOCK_SIZE_THRESHOLD1 = 256
	_BWT_BLOCK_SIZE_THRESHOLD2 = 8 * 1024 * 1024
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
// BWT[i] = input[SA[i]-1] => BWT(input) = ipssmpissii (+ primary index 5)
// The suffix array and permutation vector are equal when the input is 0 terminated
// The insertion of a guard is done internally and is entirely transparent.
//
// This implementation extends the canonical algorithm to use up to MAX_CHUNKS primary
// indexes (based on input block size). Each primary index corresponds to a data chunk.
// Chunks may be inverted concurrently.

// BWT Burrows Wheeler Transform
type BWT struct {
	buffer         []int32
	primaryIndexes [8]uint
	saAlgo         *DivSufSort
	jobs           uint
}

// NewBWT creates a new BWT instance with 1 job
func NewBWT() (*BWT, error) {
	this := new(BWT)
	this.buffer = make([]int32, 0)
	this.primaryIndexes = [8]uint{}
	this.jobs = 1
	return this, nil
}

// NewBWTWithCtx creates a new BWT instance. The number of jobs is extracted
// from the provided map or arguments.
func NewBWTWithCtx(ctx *map[string]interface{}) (*BWT, error) {
	this := new(BWT)
	this.buffer = make([]int32, 0)
	this.primaryIndexes = [8]uint{}

	if _, containsKey := (*ctx)["jobs"]; containsKey {
		this.jobs = (*ctx)["jobs"].(uint)
	} else {
		this.jobs = 1
	}

	return this, nil
}

// PrimaryIndex returns the primary index for the n-th chunk
func (this *BWT) PrimaryIndex(n int) uint {
	return this.primaryIndexes[n]
}

// SetPrimaryIndex sets the primary index for of n-th chunk
func (this *BWT) SetPrimaryIndex(n int, primaryIndex uint) bool {
	if n < 0 || n >= len(this.primaryIndexes) {
		return false
	}

	this.primaryIndexes[n] = primaryIndex
	return true
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *BWT) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count > MaxBWTBlockSize() {
		// Not a recoverable error: instead of silently fail the transform,
		// issue a fatal error.
		errMsg := fmt.Sprintf("The max BWT block size is %v, got %v", MaxBWTBlockSize(), count)
		panic(errors.New(errMsg))
	}

	if count > len(dst) {
		errMsg := fmt.Sprintf("Block size is %v, output buffer length is %v", count, len(dst))
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

	// Lazy dynamic memory allocation
	if len(this.buffer) < count {
		this.buffer = make([]int32, count)
	}

	sa := this.buffer
	chunks := GetBWTChunks(count)

	if chunks == 1 {
		pIdx := this.saAlgo.ComputeBWT(src[0:count], dst, sa[0:count])
		this.SetPrimaryIndex(0, uint(pIdx))
	} else {
		this.saAlgo.ComputeSuffixArray(src[0:count], sa[0:count])
		step := int32(count / chunks)

		if int(step)*chunks != count {
			step++
		}

		dst[0] = src[count-1]
		idx := 0

		for i := range sa {
			if (sa[i] % step) != 0 {
				continue
			}

			if this.SetPrimaryIndex(int(sa[i]/step), uint(i+1)) {
				idx++

				if idx == chunks {
					break
				}
			}
		}

		pIdx0 := int(this.PrimaryIndex(0))

		for i := 0; i < pIdx0-1; i++ {
			dst[i+1] = src[sa[i]-1]
		}

		for i := pIdx0; i < count; i++ {
			dst[i] = src[sa[i]-1]
		}
	}

	return uint(count), uint(count), nil
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *BWT) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count > MaxBWTBlockSize() {
		// Not a recoverable error: instead of silently fail the transform,
		// issue a fatal error.
		errMsg := fmt.Sprintf("The max BWT block size is %v, got %v", MaxBWTSBlockSize(), count)
		panic(errors.New(errMsg))
	}

	if count > len(dst) {
		errMsg := fmt.Sprintf("BWT inverse failed: output buffer size is %v, expected %v", count, len(dst))
		return 0, 0, errors.New(errMsg)
	}

	if count < 2 {
		if count == 1 {
			dst[0] = src[0]
		}

		return uint(count), uint(count), nil
	}

	// Find the fastest way to implement inverse based on block size
	if count <= _BWT_BLOCK_SIZE_THRESHOLD2 && this.jobs == 1 {
		return this.inverseMergeTPSI(src, dst, count)
	}

	return this.inverseBiPSIv2(src, dst, count)
}

// When count <= _BWT_BLOCK_SIZE_THRESHOLD2, mergeTPSI algo. Always in one chunk
func (this *BWT) inverseMergeTPSI(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocation
	if len(this.buffer) < count {
		if count <= 64 {
			this.buffer = make([]int32, 64)
		} else {
			this.buffer = make([]int32, count)
		}
	}

	// Aliasing
	data := this.buffer

	// Build array of packed index + value (assumes block size < 2^24)
	pIdx := int(this.PrimaryIndex(0))

	if pIdx > len(src) {
		return 0, 0, errors.New("Invalid input: corrupted BWT primary index")
	}

	buckets := [256]int{}
	kanzi.ComputeHistogram(src[0:count], buckets[:], true, false)
	sum := 0

	for i, b := range &buckets {
		tmp := b
		buckets[i] = sum
		sum += tmp
	}

	for i := 0; i < pIdx; i++ {
		val := int32(src[i])
		data[buckets[val]] = int32((i-1)<<8) | val
		buckets[val]++
	}

	for i := pIdx; i < count; i++ {
		val := int32(src[i])
		data[buckets[val]] = int32((i)<<8) | val
		buckets[val]++
	}

	if count < _BWT_BLOCK_SIZE_THRESHOLD1 {
		t := int32(pIdx - 1)

		for i := range src {
			ptr := data[t]
			dst[i] = byte(ptr)
			t = ptr >> 8
		}
	} else {
		ckSize := count >> 3

		if ckSize*8 != count {
			ckSize++
		}

		t0 := int32(this.PrimaryIndex(0) - 1)
		t1 := int32(this.PrimaryIndex(1) - 1)
		t2 := int32(this.PrimaryIndex(2) - 1)
		t3 := int32(this.PrimaryIndex(3) - 1)
		t4 := int32(this.PrimaryIndex(4) - 1)
		t5 := int32(this.PrimaryIndex(5) - 1)
		t6 := int32(this.PrimaryIndex(6) - 1)
		t7 := int32(this.PrimaryIndex(7) - 1)
		n := 0

		for {
			ptr0 := data[t0]
			dst[n] = byte(ptr0)
			t0 = ptr0 >> 8
			ptr1 := data[t1]
			dst[n+ckSize*1] = byte(ptr1)
			t1 = ptr1 >> 8
			ptr2 := data[t2]
			dst[n+ckSize*2] = byte(ptr2)
			t2 = ptr2 >> 8
			ptr3 := data[t3]
			dst[n+ckSize*3] = byte(ptr3)
			t3 = ptr3 >> 8
			ptr4 := data[t4]
			dst[n+ckSize*4] = byte(ptr4)
			t4 = ptr4 >> 8
			ptr5 := data[t5]
			dst[n+ckSize*5] = byte(ptr5)
			t5 = ptr5 >> 8
			ptr6 := data[t6]
			dst[n+ckSize*6] = byte(ptr6)
			t6 = ptr6 >> 8
			ptr7 := data[t7]
			dst[n+ckSize*7] = byte(ptr7)
			t7 = ptr7 >> 8
			n++

			if ptr7 < 0 {
				break
			}
		}

		for n < ckSize {
			ptr0 := data[t0]
			dst[n] = byte(ptr0)
			t0 = ptr0 >> 8
			ptr1 := data[t1]
			dst[n+ckSize*1] = byte(ptr1)
			t1 = ptr1 >> 8
			ptr2 := data[t2]
			dst[n+ckSize*2] = byte(ptr2)
			t2 = ptr2 >> 8
			ptr3 := data[t3]
			dst[n+ckSize*3] = byte(ptr3)
			t3 = ptr3 >> 8
			ptr4 := data[t4]
			dst[n+ckSize*4] = byte(ptr4)
			t4 = ptr4 >> 8
			ptr5 := data[t5]
			dst[n+ckSize*5] = byte(ptr5)
			t5 = ptr5 >> 8
			ptr6 := data[t6]
			dst[n+ckSize*6] = byte(ptr6)
			t6 = ptr6 >> 8
			n++
		}
	}

	return uint(count), uint(count), nil
}

// When count > _BWT_BLOCK_SIZE_THRESHOLD2, biPSIv2 algo
func (this *BWT) inverseBiPSIv2(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocations
	if len(this.buffer) < count+1 {
		if count+1 <= 64 {
			this.buffer = make([]int32, 64)
		} else {
			this.buffer = make([]int32, count+1)
		}
	}

	pIdx := int(this.PrimaryIndex(0))

	if pIdx > len(src) {
		return 0, 0, errors.New("Invalid input: corrupted BWT primary index")
	}

	freqs := [256]int{}
	kanzi.ComputeHistogram(src[0:count], freqs[:], true, false)
	buckets := make([]int, 65536)

	for c, sum := 0, 1; c < 256; c++ {
		f := sum
		sum += int(freqs[c])
		freqs[c] = f

		if f != sum {
			ptr := buckets[c<<8 : (c+1)<<8]
			var hi, lo int

			if sum < pIdx {
				hi = sum
			} else {
				hi = pIdx
			}

			if f-1 > pIdx {
				lo = f - 1
			} else {
				lo = pIdx
			}

			for i := f; i < hi; i++ {
				ptr[src[i]]++
			}

			for i := lo; i < sum-1; i++ {
				ptr[src[i]]++
			}
		}
	}

	lastc := int(src[0])
	fastBits := make([]uint16, _BWT_MASK_FASTBITS+1)
	shift := uint(0)

	for (count >> shift) > _BWT_MASK_FASTBITS {
		shift++
	}

	for c, v, sum := 0, 0, 1; c < 256; c++ {
		if c == lastc {
			sum++
		}

		ptr := buckets[c:]

		for d := 0; d < 256; d++ {
			s := sum
			sum += ptr[d<<8]
			ptr[d<<8] = s

			if s != sum {
				for v <= ((sum - 1) >> shift) {
					fastBits[v] = uint16((c << 8) | d)
					v++
				}
			}
		}
	}

	data := this.buffer

	for i := 0; i < pIdx; i++ {
		c := int(src[i])
		p := freqs[c]
		freqs[c]++

		if p < pIdx {
			idx := (c << 8) | int(src[p])
			data[buckets[idx]] = int32(i)
			buckets[idx]++
		} else if p > pIdx {
			idx := (c << 8) | int(src[p-1])
			data[buckets[idx]] = int32(i)
			buckets[idx]++
		}
	}

	for i := pIdx; i < count; i++ {
		c := int(src[i])
		p := freqs[c]
		freqs[c]++

		if p < pIdx {
			idx := (c << 8) | int(src[p])
			data[buckets[idx]] = int32(i + 1)
			buckets[idx]++
		} else if p > pIdx {
			idx := (c << 8) | int(src[p-1])
			data[buckets[idx]] = int32(i + 1)
			buckets[idx]++
		}
	}

	for c := 0; c < 256; c++ {
		c256 := c << 8

		for d := 0; d < c; d++ {
			buckets[(d<<8)|c], buckets[c256|d] = buckets[c256|d], buckets[(d<<8)|c]
		}
	}

	chunks := GetBWTChunks(count)

	// Build inverse
	// Several chunks may be decoded concurrently (depending on the availability
	// of jobs for this block).
	ckSize := count / chunks

	if ckSize*chunks != count {
		ckSize++
	}

	nbTasks := int(this.jobs)

	if nbTasks > chunks {
		nbTasks = chunks
	}

	jobsPerTask := kanzi.ComputeJobsPerTask(make([]uint, nbTasks), uint(chunks), uint(nbTasks))
	var wg sync.WaitGroup

	for j, c := 0, 0; j < nbTasks; j++ {
		wg.Add(1)
		start := c * ckSize

		go func(dst []byte, buckets []int, fastBits []uint16, indexes []uint, total, start, ckSize, firstChunk, lastChunk int) {
			this.inverseBiPSIv2Task(dst, buckets, fastBits, indexes, total, start, ckSize, firstChunk, lastChunk)
			wg.Done()
		}(dst, buckets[:], fastBits, this.primaryIndexes[:], count, start, ckSize, c, c+int(jobsPerTask[j]))

		c += int(jobsPerTask[j])
	}

	wg.Wait()

	dst[count-1] = byte(lastc)
	return uint(count), uint(count), nil
}

func (this *BWT) inverseBiPSIv2Task(dst []byte, buckets []int, fastBits []uint16, indexes []uint, total, start, ckSize, firstChunk, lastChunk int) {
	data := this.buffer
	shift := uint(0)

	for (total >> shift) > _BWT_MASK_FASTBITS {
		shift++
	}

	for c := firstChunk; c < lastChunk; c++ {
		end := start + ckSize

		if end > total-1 {
			end = total - 1
		}

		p := int(indexes[c])

		for i := start + 1; i <= end; i += 2 {
			s := fastBits[p>>shift]

			for buckets[s] <= p {
				s++
			}

			dst[i-1] = byte(s >> 8)
			dst[i] = byte(s)
			p = int(data[p])
		}

		start = end
	}
}

// MaxBWTBlockSize returns the maximum size of a block to transform
func MaxBWTBlockSize() int {
	return _BWT_MAX_BLOCK_SIZE
}

// GetBWTChunks returns the number of chunks for a given block size
func GetBWTChunks(size int) int {
	if size < _BWT_BLOCK_SIZE_THRESHOLD1 {
		return 1
	}

	return 8
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this BWT) MaxEncodedLen(srcLen int) int {
	return srcLen
}
