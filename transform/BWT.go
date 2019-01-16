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
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	BWT_MAX_BLOCK_SIZE = 1024 * 1024 * 1024 // 1 GB
	BWT_MAX_CHUNKS     = 8
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
//
// This implementation extends the canonical algorithm to use up to MAX_CHUNKS primary
// indexes (based on input block size). Each primary index corresponds to a data chunk.
// Chunks may be inverted concurrently.

type BWT struct {
	buffer1        []uint32 // inverse regular blocks
	buffer2        []byte   // inverse big blocks
	buffer3        []int32  // forward
	primaryIndexes [8]uint
	saAlgo         *DivSufSort
	jobs           uint
}

func NewBWT() (*BWT, error) {
	this := new(BWT)
	this.buffer1 = make([]uint32, 0)
	this.buffer2 = make([]byte, 0)
	this.buffer3 = make([]int32, 0)
	this.primaryIndexes = [8]uint{}
	this.jobs = 1
	return this, nil
}

func NewBWTWithCtx(ctx *map[string]interface{}) (*BWT, error) {
	this := new(BWT)
	this.buffer1 = make([]uint32, 0)
	this.buffer2 = make([]byte, 0)
	this.buffer3 = make([]int32, 0)
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
	if len(this.buffer3) < count {
		this.buffer3 = make([]int32, count)
	}

	sa := this.buffer3
	this.saAlgo.ComputeSuffixArray(src[0:count], sa[0:count])
	n := 0
	chunks := GetBWTChunks(count)

	if chunks == 1 {
		for n < count {
			if sa[n] == 0 {
				this.SetPrimaryIndex(0, uint(n))
				break
			}

			dst[n] = src[sa[n]-1]
			n++
		}

		dst[n] = src[count-1]
		n++

		for n < count {
			dst[n] = src[sa[n]-1]
			n++
		}
	} else {
		step := int32(count / chunks)

		if int(step)*chunks != count {
			step++
		}

		for n < count {
			if sa[n]%step == 0 {
				this.SetPrimaryIndex(int(sa[n]/step), uint(n))

				if sa[n] == 0 {
					break
				}
			}

			dst[n] = src[sa[n]-1]
			n++
		}

		dst[n] = src[count-1]
		n++

		for n < count {
			if sa[n]%step == 0 {
				this.SetPrimaryIndex(int(sa[n]/step), uint(n))
			}

			dst[n] = src[sa[n]-1]
			n++
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
		errMsg := fmt.Sprintf("Block size is %v, output buffer length is %v", count, len(dst))
		return 0, 0, errors.New(errMsg)
	}

	if count < 2 {
		if count == 1 {
			dst[0] = src[0]
		}

		return uint(count), uint(count), nil
	}

	// Find the fastest way to implement inverse based on block size
	if count < 1<<24 {
		return this.inverseRegularBlock(src, dst, count)
	}

	if 5*uint64(count) >= uint64(1)<<31 {
		return this.inverseHugeBlock(src, dst, count)
	}

	return this.inverseBigBlock(src, dst, count)
}

// When count < 1<<24
func (this *BWT) inverseRegularBlock(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocation
	if len(this.buffer1) < count {
		this.buffer1 = make([]uint32, count)
	}

	// Aliasing
	data := this.buffer1
	buckets := [256]uint32{}
	chunks := GetBWTChunks(count)

	// Build array of packed index + value (assumes block size < 2^24)
	// Start with the primary index position
	pIdx := int(this.PrimaryIndex(0))
	val0 := uint32(src[pIdx])
	data[pIdx] = val0
	buckets[val0]++

	for i := 0; i < pIdx; i++ {
		val := uint32(src[i])
		data[i] = (buckets[val] << 8) | val
		buckets[val]++
	}

	for i := pIdx + 1; i < count; i++ {
		val := uint32(src[i])
		data[i] = (buckets[val] << 8) | val
		buckets[val]++
	}

	sum := uint32(0)

	for i, b := range &buckets {
		buckets[i] = sum
		sum += b
	}

	idx := count - 1

	// Build inverse
	if chunks == 1 || this.jobs == 1 {
		// Shortcut for 1 chunk scenario
		ptr := data[pIdx]
		dst[idx] = byte(ptr)
		idx--

		for idx >= 0 {
			ptr = data[(ptr>>8)+buckets[ptr&0xFF]]
			dst[idx] = byte(ptr)
			idx--
		}
	} else {
		// Several chunks may be decoded concurrently (depending on the availaibility
		// of jobs for this block).
		step := count / chunks

		if step*chunks != count {
			step++
		}

		nbTasks := int(this.jobs)

		if nbTasks > chunks {
			nbTasks = chunks
		}

		jobsPerTask := kanzi.ComputeJobsPerTask(make([]uint, nbTasks), uint(chunks), uint(nbTasks))
		c := chunks
		var wg sync.WaitGroup

		// Create one task per job
		for j := 0; j < nbTasks; j++ {
			// Each task decodes jobsPerTask[j] chunks
			wg.Add(1)
			nc := c - int(jobsPerTask[j])
			end := nc * step

			go func(dst []byte, buckets []uint32, pIdx, idx, step, startChunk, endChunk int) {
				this.inverseChunkRegularBlock(dst, buckets, pIdx, idx, step, startChunk, endChunk)
				wg.Done()
			}(dst, buckets[:], pIdx, idx, step, c-1, nc-1)

			c = nc
			pIdx = int(this.PrimaryIndex(c))
			idx = end - 1
		}

		wg.Wait()
	}

	return uint(count), uint(count), nil
}

// When count >= 1<<24 and 5*count < 1<<31
func (this *BWT) inverseBigBlock(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocations
	if len(this.buffer2) < count {
		this.buffer2 = make([]byte, 5*count)
	}

	// Aliasing
	data := this.buffer2

	buckets := [256]uint32{}
	chunks := GetBWTChunks(count)

	// Build arrays
	// Start with the primary index position
	pIdx := int(this.PrimaryIndex(0))
	val0 := src[pIdx]
	binary.LittleEndian.PutUint32(data[pIdx*5:], buckets[val0])
	data[pIdx*5+4] = val0
	buckets[val0]++

	for i := 0; i < pIdx; i++ {
		val := src[i]
		binary.LittleEndian.PutUint32(data[i*5:], buckets[val])
		data[i*5+4] = val
		buckets[val]++
	}

	for i := pIdx + 1; i < count; i++ {
		val := src[i]
		binary.LittleEndian.PutUint32(data[i*5:], buckets[val])
		data[i*5+4] = val
		buckets[val]++
	}

	sum := uint32(0)

	// Create cumulative histogram
	for i, b := range &buckets {
		buckets[i] = sum
		sum += b
	}

	idx := count - 1

	// Build inverse
	if chunks == 1 || this.jobs == 1 {
		// Shortcut for 1 chunk scenario
		val := data[pIdx*5+4]
		dst[idx] = val
		idx--
		n := binary.LittleEndian.Uint32(data[pIdx*5:]) + buckets[val]

		for idx >= 0 {
			val = data[n*5+4]
			dst[idx] = val
			idx--
			n = binary.LittleEndian.Uint32(data[n*5:]) + buckets[val]
		}
	} else {
		// Several chunks may be decoded concurrently (depending on the availaibility
		// of jobs for this block).
		step := count / chunks

		if step*chunks != count {
			step++
		}

		nbTasks := int(this.jobs)

		if nbTasks > chunks {
			nbTasks = chunks
		}

		jobsPerTask := kanzi.ComputeJobsPerTask(make([]uint, nbTasks), uint(chunks), uint(nbTasks))
		c := chunks
		var wg sync.WaitGroup

		for j := 0; j < nbTasks; j++ {
			wg.Add(1)
			nc := c - int(jobsPerTask[j])
			end := nc * step

			go func(dst []byte, buckets []uint32, pIdx, idx, step, startChunk, endChunk int) {
				this.inverseChunkBigBlock(dst, buckets, pIdx, idx, step, startChunk, endChunk)
				wg.Done()
			}(dst, buckets[:], pIdx, idx, step, c-1, nc-1)

			c = nc
			pIdx = int(this.PrimaryIndex(c))
			idx = end - 1
		}

		wg.Wait()
	}

	return uint(count), uint(count), nil
}

// When 5*count >= 1<<31
func (this *BWT) inverseHugeBlock(src, dst []byte, count int) (uint, uint, error) {
	// Lazy dynamic memory allocations
	if len(this.buffer1) < count {
		this.buffer1 = make([]uint32, count)
	}

	if len(this.buffer2) < count {
		this.buffer2 = make([]byte, count)
	}

	// Aliasing
	data1 := this.buffer1
	data2 := this.buffer2

	buckets := [256]uint32{}
	chunks := GetBWTChunks(count)

	// Build arrays
	// Start with the primary index position
	pIdx := int(this.PrimaryIndex(0))
	val0 := src[pIdx]
	data1[pIdx] = buckets[val0]
	data2[pIdx] = val0
	buckets[val0]++

	for i := 0; i < pIdx; i++ {
		val := src[i]
		data1[i] = buckets[val]
		data2[i] = val
		buckets[val]++
	}

	for i := pIdx + 1; i < count; i++ {
		val := src[i]
		data1[i] = buckets[val]
		data2[i] = val
		buckets[val]++
	}

	sum := uint32(0)

	// Create cumulative histogram
	for i, b := range buckets {
		buckets[i] = sum
		sum += b
	}

	idx := count - 1

	// Build inverse
	if chunks == 1 || this.jobs == 1 {
		// Shortcut for 1 chunk scenario
		val := data2[pIdx]
		dst[idx] = val
		idx--
		n := data1[pIdx] + buckets[val]

		for idx >= 0 {
			val = data2[n]
			dst[idx] = val
			idx--
			n = data1[n] + buckets[val]
		}
	} else {
		// Several chunks may be decoded concurrently (depending on the availaibility
		// of jobs for this block).
		step := count / chunks

		if step*chunks != count {
			step++
		}

		nbTasks := int(this.jobs)

		if nbTasks > chunks {
			nbTasks = chunks
		}

		jobsPerTask := kanzi.ComputeJobsPerTask(make([]uint, nbTasks), uint(chunks), uint(nbTasks))
		c := chunks
		var wg sync.WaitGroup

		for j := 0; j < nbTasks; j++ {
			wg.Add(1)
			nc := c - int(jobsPerTask[j])
			end := nc * step

			go func(dst []byte, buckets []uint32, pIdx, idx, step, startChunk, endChunk int) {
				this.inverseChunkHugeBlock(dst, buckets, pIdx, idx, step, startChunk, endChunk)
				wg.Done()
			}(dst, buckets[:], pIdx, idx, step, c-1, nc-1)

			c = nc
			pIdx = int(this.PrimaryIndex(c))
			idx = end - 1
		}

		wg.Wait()
	}

	return uint(count), uint(count), nil
}

func MaxBWTBlockSize() int {
	return BWT_MAX_BLOCK_SIZE
}

func (this *BWT) inverseChunkRegularBlock(dst []byte, buckets []uint32, pIdx, idx, step, startChunk, endChunk int) {
	data := this.buffer1

	for i := startChunk; i > endChunk; i-- {
		endIdx := i * step
		ptr := data[pIdx]
		dst[idx] = byte(ptr)
		idx--

		for idx >= endIdx {
			ptr = data[(ptr>>8)+buckets[ptr&0xFF]]
			dst[idx] = byte(ptr)
			idx--
		}

		pIdx = int(this.PrimaryIndex(i))
	}
}

func (this *BWT) inverseChunkBigBlock(dst []byte, buckets []uint32, pIdx, idx, step, startChunk, endChunk int) {
	data := this.buffer2

	for i := startChunk; i > endChunk; i-- {
		endIdx := i * step
		val := data[pIdx*5+4]
		dst[idx] = val
		idx--
		n := binary.LittleEndian.Uint32(data[pIdx*5:]) + buckets[val]

		for idx >= endIdx {
			val = data[n*5+4]
			dst[idx] = val
			idx--
			n = binary.LittleEndian.Uint32(data[n*5:]) + buckets[val]
		}

		pIdx = int(this.PrimaryIndex(i))
	}
}

func (this *BWT) inverseChunkHugeBlock(dst []byte, buckets []uint32, pIdx, idx, step, startChunk, endChunk int) {
	data1 := this.buffer1
	data2 := this.buffer2

	for i := startChunk; i > endChunk; i-- {
		endIdx := i * step
		val := data2[pIdx]
		dst[idx] = val
		idx--
		n := data1[pIdx] + buckets[val]

		for idx >= endIdx {
			val = data2[n]
			dst[idx] = val
			idx--
			n = data1[n] + buckets[val]
		}

		pIdx = int(this.PrimaryIndex(i))
	}
}

func GetBWTChunks(size int) int {
	if size < 1<<23 { // 8 MB
		return 1
	}

	res := (size + (1 << 22)) >> 23

	if res > BWT_MAX_CHUNKS {
		return BWT_MAX_CHUNKS
	}

	return res
}
