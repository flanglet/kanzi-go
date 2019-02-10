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
	"sync"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	BWT_MAX_BLOCK_SIZE = 1024 * 1024 * 1024 // 1 GB
	BWT_MAX_CHUNKS     = 8
	BWT_NB_FASTBITS    = 17
	BWT_MASK_FASTBITS  = 1 << BWT_NB_FASTBITS
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

type BWT struct {
	buffer1        []uint32
	buffer2        []int32
	primaryIndexes [8]uint
	saAlgo         *DivSufSort
	jobs           uint
}

func NewBWT() (*BWT, error) {
	this := new(BWT)
	this.buffer1 = make([]uint32, 0)
	this.buffer2 = make([]int32, 0)
	this.primaryIndexes = [8]uint{}
	this.jobs = 1
	return this, nil
}

func NewBWTWithCtx(ctx *map[string]interface{}) (*BWT, error) {
	this := new(BWT)
	this.buffer1 = make([]uint32, 0)
	this.buffer2 = make([]int32, 0)
	this.primaryIndexes = [8]uint{}

	if _, containsKey := (*ctx)["jobs"]; containsKey {
		this.jobs = (*ctx)["jobs"].(uint)
	} else {
		this.jobs = 1
	}

	return this, nil
}

func (this *BWT) PrimaryIndex(n int) uint {
	return this.primaryIndexes[n]
}

func (this *BWT) SetPrimaryIndex(n int, primaryIndex uint) bool {
	if n < 0 || n >= len(this.primaryIndexes) {
		return false
	}

	this.primaryIndexes[n] = primaryIndex
	return true
}

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
	if len(this.buffer2) < count {
		this.buffer2 = make([]int32, count)
	}

	sa := this.buffer2
	this.saAlgo.ComputeSuffixArray(src[0:count], sa[0:count])
	chunks := GetBWTChunks(count)

	if chunks == 1 {
		dst[0] = src[count-1]
		n := 0

		for n < count {
			if sa[n] == 0 {
				break
			}

			dst[n+1] = src[sa[n]-1]
			n++
		}

		n++
		this.SetPrimaryIndex(0, uint(n))

		for n < count {
			dst[n] = src[sa[n]-1]
			n++
		}
	} else {
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

			this.SetPrimaryIndex(int(sa[i]/step), uint(i+1))
			idx++

			if idx == chunks {
				break
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
	if count < 4*1024*1024 {
		return this.inverseSmallBlock(src, dst, count)
	}

	return this.inverseBigBlock(src, dst, count)
}

// When count < 4M, mergeTPSI algo. Always in one chunk
func (this *BWT) inverseSmallBlock(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocation
	if len(this.buffer1) < count {
		this.buffer1 = make([]uint32, count)
	}

	// Aliasing
	data := this.buffer1

	// Build array of packed index + value (assumes block size < 2^24)
	pIdx := int(this.PrimaryIndex(0))

	if pIdx >= len(src) {
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
		val := uint32(src[i])
		data[buckets[val]] = uint32((i-1)<<8) | val
		buckets[val]++
	}

	for i := pIdx; i < count; i++ {
		val := uint32(src[i])
		data[buckets[val]] = uint32((i)<<8) | val
		buckets[val]++
	}

	t := uint32(pIdx - 1)

	for i := range src {
		ptr := data[t]
		dst[i] = byte(ptr)
		t = ptr >> 8
	}

	return uint(count), uint(count), nil
}

// When count >= 1<<24, biPSIv2 algo. Possibly multiple chunks
func (this *BWT) inverseBigBlock(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocations
	if len(this.buffer1) < count+1 {
		this.buffer1 = make([]uint32, count+1)
	}

	pIdx := int(this.PrimaryIndex(0))

	if pIdx >= len(src) {
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
	fastBits := make([]uint16, BWT_MASK_FASTBITS+1)
	shift := uint(0)

	for (count >> shift) > BWT_MASK_FASTBITS {
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

	data := this.buffer1

	for i := 0; i < pIdx; i++ {
		c := int(src[i])
		p := freqs[c]
		freqs[c]++

		if p < pIdx {
			idx := (c << 8) | int(src[p])
			data[buckets[idx]] = uint32(i)
			buckets[idx]++
		} else if p > pIdx {
			idx := (c << 8) | int(src[p-1])
			data[buckets[idx]] = uint32(i)
			buckets[idx]++
		}
	}

	for i := pIdx; i < count; i++ {
		c := int(src[i])
		p := freqs[c]
		freqs[c]++

		if p < pIdx {
			idx := (c << 8) | int(src[p])
			data[buckets[idx]] = uint32(i + 1)
			buckets[idx]++
		} else if p > pIdx {
			idx := (c << 8) | int(src[p-1])
			data[buckets[idx]] = uint32(i + 1)
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
	if chunks == 1 {
		// Shortcut for 1 chunk scenariop := pIdx
		for i, p := 1, pIdx; i < count; i += 2 {
			c := fastBits[p>>shift]

			for buckets[c] <= p {
				c++
			}

			dst[i-1] = byte(c >> 8)
			dst[i] = byte(c)
			p = int(data[p])
		}
	} else {
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

			go func(dst []byte, buckets []int, fastBits []uint16, total, start, ckSize, firstChunk, lastChunk int) {
				this.inverseChunkTask(dst, buckets, fastBits, total, start, ckSize, firstChunk, lastChunk)
				wg.Done()
			}(dst, buckets[:], fastBits, count, start, ckSize, c, c+int(jobsPerTask[j]))

			c += int(jobsPerTask[j])
		}

		wg.Wait()
	}

	dst[count-1] = byte(lastc)
	return uint(count), uint(count), nil
}

func (this *BWT) inverseChunkTask(dst []byte, buckets []int, fastBits []uint16, total, start, ckSize, firstChunk, lastChunk int) {
	data := this.buffer1
	shift := uint(0)

	for (total >> shift) > BWT_MASK_FASTBITS {
		shift++
	}

	for c := firstChunk; c < lastChunk; c++ {
		end := start + ckSize

		if end > total-1 {
			end = total - 1
		}

		p := int(this.PrimaryIndex(c))

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

func MaxBWTBlockSize() int {
	return BWT_MAX_BLOCK_SIZE
}

func GetBWTChunks(size int) int {
	if size < 4*1024*1024 {
		return 1
	}

	res := (size + (1 << 21)) >> 22

	if res > BWT_MAX_CHUNKS {
		return BWT_MAX_CHUNKS
	}

	return res
}
