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
	"sync"

	internal "github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_BWT_MAX_BLOCK_SIZE        = 1024 * 1024 * 1024 // 1 GB
	_BWT_NB_FASTBITS           = 17
	_BWT_MASK_FASTBITS         = (1 << _BWT_NB_FASTBITS) - 1
	_BWT_BLOCK_SIZE_THRESHOLD1 = 256
	_BWT_BLOCK_SIZE_THRESHOLD2 = 4 * 1024 * 1024
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
	this := &BWT{}
	this.buffer = make([]int32, 0)
	this.primaryIndexes = [8]uint{}
	this.jobs = 1
	return this, nil
}

// NewBWTWithCtx creates a new BWT instance. The number of jobs is extracted
// from the provided map or arguments.
func NewBWTWithCtx(ctx *map[string]any) (*BWT, error) {
	this := &BWT{}
	this.buffer = make([]int32, 0)
	this.primaryIndexes = [8]uint{}
	this.jobs = 1

	if _, containsKey := (*ctx)["jobs"]; containsKey {
		this.jobs = (*ctx)["jobs"].(uint)

		if this.jobs == 0 {
			return nil, errors.New("The number of jobs must be at least 1")
		}
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

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	count := len(src)

	if count > _BWT_MAX_BLOCK_SIZE {
		return 0, 0, fmt.Errorf("The max BWT block size is %d, got %d", _BWT_MAX_BLOCK_SIZE, count)
	}

	if count == 1 {
		dst[0] = src[0]
		return uint(count), uint(count), nil
	}

	if this.saAlgo == nil {
		var err error

		if this.saAlgo, err = NewDivSufSort(); err != nil {
			return 0, 0, err
		}
	}

	// Lazy dynamic memory allocation
	minLenBuf := max(count, 256)

	if len(this.buffer) < minLenBuf {
		this.buffer = make([]int32, minLenBuf)
	}

	this.saAlgo.ComputeBWT(src[0:count], dst, this.buffer[0:count], this.primaryIndexes[:], GetBWTChunks(count))
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

	if count > _BWT_MAX_BLOCK_SIZE {
		return 0, 0, fmt.Errorf("The max BWT block size is %d, got %d", _BWT_MAX_BLOCK_SIZE, count)
	}

	if count > len(dst) {
		return 0, 0, fmt.Errorf("BWT inverse failed: output buffer size is %d, expected %d", count, len(dst))
	}

	if count == 1 {
		dst[0] = src[0]
		return uint(count), uint(count), nil
	}

	// Find the fastest way to implement inverse based on block size
	if count <= _BWT_BLOCK_SIZE_THRESHOLD2 {
		return this.inverseMergeTPSI(src, dst, count)
	}

	return this.inverseBiPSIv2(src, dst, count)
}

// When count <= _BWT_BLOCK_SIZE_THRESHOLD2, mergeTPSI algo. Always in one chunk
func (this *BWT) inverseMergeTPSI(src, dst []byte, count int) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	pIdx := int(this.PrimaryIndex(0))

	if pIdx <= 0 || pIdx > len(src) {
		return 0, 0, errors.New("Invalid input: corrupted BWT primary index")
	}

	// Lazy dynamic memory allocation
	minLenBuf := max(count, 64)

	if len(this.buffer) < minLenBuf {
		this.buffer = make([]int32, minLenBuf)
	}

	// Aliasing
	data := this.buffer

	// Build array of packed index + value (assumes block size < 2^24)
	buckets := [256]int{}
	internal.ComputeHistogram(src[0:count], buckets[:], true, false)
	sum := 0

	for i, b := range &buckets {
		tmp := b
		buckets[i] = sum
		sum += tmp
	}

	data[buckets[src[0]]] = int32(0xFF00) | int32(src[0])
	buckets[src[0]]++

	for i := 1; i < pIdx; i++ {
		val := int32(src[i])
		data[buckets[val]] = int32((i-1)<<8) | val
		buckets[val]++
	}

	for i := pIdx; i < count; i++ {
		val := int32(src[i])
		data[buckets[val]] = int32((i)<<8) | val
		buckets[val]++
	}

	if GetBWTChunks(count) != 8 {
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

		if t0 < 0 || t1 < 0 || t2 < 0 || t3 < 0 || t4 < 0 || t5 < 0 || t6 < 0 || t7 < 0 {
			return 0, 0, errors.New("Invalid input: corrupted BWT primary index")
		}

		if t0 >= int32(len(data)) || t1 >= int32(len(data)) || t2 >= int32(len(data)) || t3 >= int32(len(data)) || t4 >= int32(len(data)) || t5 >= int32(len(data)) || t6 >= int32(len(data)) || t7 >= int32(len(data)) {
			return 0, 0, errors.New("Invalid input: corrupted BWT primary index")
		}

		d0 := dst[0*ckSize : 1*ckSize]
		d1 := dst[1*ckSize : 2*ckSize]
		d2 := dst[2*ckSize : 3*ckSize]
		d3 := dst[3*ckSize : 4*ckSize]
		d4 := dst[4*ckSize : 5*ckSize]
		d5 := dst[5*ckSize : 6*ckSize]
		d6 := dst[6*ckSize : 7*ckSize]
		d7 := dst[7*ckSize : count]

		// Last interval [7*chunk:count] smaller when 8*ckSize != count
		end := count - ckSize*7
		n := 0

		for n < end {
			ptr0 := data[t0]
			d0[n] = byte(ptr0)
			t0 = ptr0 >> 8
			ptr1 := data[t1]
			d1[n] = byte(ptr1)
			t1 = ptr1 >> 8
			ptr2 := data[t2]
			d2[n] = byte(ptr2)
			t2 = ptr2 >> 8
			ptr3 := data[t3]
			d3[n] = byte(ptr3)
			t3 = ptr3 >> 8
			ptr4 := data[t4]
			d4[n] = byte(ptr4)
			t4 = ptr4 >> 8
			ptr5 := data[t5]
			d5[n] = byte(ptr5)
			t5 = ptr5 >> 8
			ptr6 := data[t6]
			d6[n] = byte(ptr6)
			t6 = ptr6 >> 8
			ptr7 := data[t7]
			d7[n] = byte(ptr7)
			t7 = ptr7 >> 8
			n++
		}

		for n < ckSize {
			ptr0 := data[t0]
			d0[n] = byte(ptr0)
			t0 = ptr0 >> 8
			ptr1 := data[t1]
			d1[n] = byte(ptr1)
			t1 = ptr1 >> 8
			ptr2 := data[t2]
			d2[n] = byte(ptr2)
			t2 = ptr2 >> 8
			ptr3 := data[t3]
			d3[n] = byte(ptr3)
			t3 = ptr3 >> 8
			ptr4 := data[t4]
			d4[n] = byte(ptr4)
			t4 = ptr4 >> 8
			ptr5 := data[t5]
			d5[n] = byte(ptr5)
			t5 = ptr5 >> 8
			ptr6 := data[t6]
			d6[n] = byte(ptr6)
			t6 = ptr6 >> 8
			n++
		}
	}

	return uint(count), uint(count), nil
}

// When count > _BWT_BLOCK_SIZE_THRESHOLD2, biPSIv2 algo
func (this *BWT) inverseBiPSIv2(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocations
	minLenBuf := max(count+1, 256)

	if len(this.buffer) < minLenBuf {
		this.buffer = make([]int32, minLenBuf)
	}

	pIdx := int(this.PrimaryIndex(0))

	if pIdx > len(src) {
		return 0, 0, errors.New("Invalid input: corrupted BWT primary index")
	}

	freqs := [256]int{}
	internal.ComputeHistogram(src[0:count], freqs[:], true, false)
	buckets := make([]int, 65536)

	for c, sum := 0, 1; c < 256; c++ {
		f := sum
		sum += int(freqs[c])
		freqs[c] = f

		if f != sum {
			ptr := buckets[c<<8 : (c+1)<<8]
			hi := min(sum, pIdx)
			lo := max(f-1, pIdx)

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
			val := ptr[d<<8]
			ptr[d<<8] = sum
			sum += val

			if val != 0 {
				fb := uint16((c << 8) | d)
				ve := (sum - 1) >> shift

				for v <= ve {
					fastBits[v] = fb
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

	nbTasks := min(int(this.jobs), chunks)
	jobsPerTask, _ := internal.ComputeJobsPerTask(make([]uint, nbTasks), uint(chunks), uint(nbTasks))
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

	c := firstChunk
	dst0 := dst[0:]
	dst1 := dst[ckSize:]
	dst2 := dst[2*ckSize:]
	dst3 := dst[3*ckSize:]
	dst4 := dst[4*ckSize:]
	dst5 := dst[5*ckSize:]
	dst6 := dst[6*ckSize:]
	dst7 := dst[7*ckSize:]

	if start+8*ckSize <= total {
		for c+7 < lastChunk {
			end := start + ckSize
			p0 := int(indexes[c])
			p1 := int(indexes[c+1])
			p2 := int(indexes[c+2])
			p3 := int(indexes[c+3])
			p4 := int(indexes[c+4])
			p5 := int(indexes[c+5])
			p6 := int(indexes[c+6])
			p7 := int(indexes[c+7])

			for i := start + 1; i <= end; i += 2 {
				s0 := fastBits[p0>>shift]
				s1 := fastBits[p1>>shift]
				s2 := fastBits[p2>>shift]
				s3 := fastBits[p3>>shift]
				s4 := fastBits[p4>>shift]
				s5 := fastBits[p5>>shift]
				s6 := fastBits[p6>>shift]
				s7 := fastBits[p7>>shift]

				for buckets[s0] <= p0 {
					s0++
				}

				for buckets[s1] <= p1 {
					s1++
				}

				for buckets[s2] <= p2 {
					s2++
				}

				for buckets[s3] <= p3 {
					s3++
				}

				for buckets[s4] <= p4 {
					s4++
				}

				for buckets[s5] <= p5 {
					s5++
				}

				for buckets[s6] <= p6 {
					s6++
				}

				for buckets[s7] <= p7 {
					s7++
				}

				dst0[i-1] = byte(s0 >> 8)
				dst0[i] = byte(s0)
				dst1[i-1] = byte(s1 >> 8)
				dst1[i] = byte(s1)
				dst2[i-1] = byte(s2 >> 8)
				dst2[i] = byte(s2)
				dst3[i-1] = byte(s3 >> 8)
				dst3[i] = byte(s3)
				dst4[i-1] = byte(s4 >> 8)
				dst4[i] = byte(s4)
				dst5[i-1] = byte(s5 >> 8)
				dst5[i] = byte(s5)
				dst6[i-1] = byte(s6 >> 8)
				dst6[i] = byte(s6)
				dst7[i-1] = byte(s7 >> 8)
				dst7[i] = byte(s7)
				p0 = int(data[p0])
				p1 = int(data[p1])
				p2 = int(data[p2])
				p3 = int(data[p3])
				p4 = int(data[p4])
				p5 = int(data[p5])
				p6 = int(data[p6])
				p7 = int(data[p7])
			}

			start += 8*ckSize
			c += 8
		}
	}

	for c < lastChunk {
		end := min(start+ckSize, total-1)
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
		c++
	}
}

// GetBWTChunks returns the number of chunks for a given block size
func GetBWTChunks(size int) int {
	if size < _BWT_BLOCK_SIZE_THRESHOLD1 {
		return 1
	}

	return 8
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *BWT) MaxEncodedLen(srcLen int) int {
	return srcLen
}
