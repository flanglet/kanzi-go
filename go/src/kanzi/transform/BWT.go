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
// This implementation is based on the SA_IS (Suffix Array Induction Sorting) algorithm.
// This is a port of sais.c by Yuta Mori (http://sites.google.com/site/yuta256/sais)
// See original publication of the algorithm here:
// [Ge Nong, Sen Zhang and Wai Hong Chan, Two Efficient Algorithms for
// Linear Suffix Array Construction, 2008]
// Another good read: http://labh-curien.univ-st-etienne.fr/~bellet/misc/SA_report.pdf
//
// Overview of the algorithm:
// Step 1 - Problem reduction: the input string is reduced into a smaller string.
// Step 2 - Recursion: the suffix array of the reduced problem is recursively computed.
// Step 3 - Problem induction: based on the suffix array of the reduced problem, that of the
//          unreduced problem is induced
//
// E.G.
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

type BWT struct {
	size         uint
	data         []int
	buffer1      []int
	buckets      []int
	primaryIndex uint
}

func NewBWT(sz uint) (*BWT, error) {
	this := new(BWT)
	this.size = sz
	this.data = make([]int, sz)
	this.buffer1 = make([]int, sz)
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

	// Lazy dynamic memory allocation
	if len(this.data) < count {
		this.data = make([]int, count)
	}

	// Lazy dynamic memory allocation
	if len(this.buffer1) < count {
		this.buffer1 = make([]int, count)
	}

	data_ := this.data

	for i := 0; i < count; i++ {
		data_[i] = int(src[i])
	}

	// Suffix array
	sa := this.buffer1
	pIdx := computeSuffixArray(this.data, sa, 0, count, 256, true)
	dst[0] = byte(this.data[count-1])

	for i := uint(0); i < pIdx; i++ {
		dst[i+1] = byte(sa[i])
	}

	for i := int(pIdx + 1); i < count; i++ {
		dst[i] = byte(sa[i])
	}

	this.SetPrimaryIndex(pIdx)
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

	// Aliasing
	buckets_ := this.buckets
	data_ := this.data

	// Lazy dynamic memory allocation
	if len(this.data) < count {
		data_ = make([]int, count)
	}

	// Create histogram
	for i := range this.buckets {
		buckets_[i] = 0
	}

	// Build array of packed index + value (assumes block size < 2^24)
	// Start with the primary index position
	pIdx := int(this.PrimaryIndex())
	val := int(src[0])
	data_[pIdx] = (buckets_[val] << 8) | val
	buckets_[val]++

	for i := 0; i < pIdx; i++ {
		val = int(src[i+1])
		data_[i] = (buckets_[val] << 8) | val
		buckets_[val]++
	}

	for i := pIdx + 1; i < count; i++ {
		val = int(src[i])
		data_[i] = (buckets_[val] << 8) | val
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
		ptr := data_[idx]
		dst[i] = byte(ptr)
		idx = (ptr >> 8) + buckets_[ptr&0xFF]
	}

	return uint(count), uint(count), nil
}

func getCounts(src []int, dst []int, n, k int) {
	for i := 0; i < k; i++ {
		dst[i] = 0
	}

	for i := 0; i < n; i++ {
		dst[src[i]]++
	}
}

func getBuckets(src []int, dst []int, k int, end bool) {
	sum := 0

	if end == true {
		for i := 0; i < k; i++ {
			sum += src[i]
			dst[i] = sum
		}
	} else {
		for i := 0; i < k; i++ {
			// The temp variable is required if src == dst
			tmp := src[i]
			dst[i] = sum
			sum += tmp
		}
	}
}

// sort all type LMS suffixes
func sortLMSSuffixes(src []int, sa []int, ptrC *[]int, ptrB *[]int, n, k int) {
	// compute sal
	if ptrC == ptrB {
		getCounts(src, *ptrC, n, k)
	}

	B := *ptrB
	C := *ptrC

	// find starts of buckets
	getBuckets(C, B, k, false)

	j := n - 1
	c1 := src[j]
	b := B[c1]
	j--

	if src[j] < c1 {
		sa[b] = ^j
	} else {
		sa[b] = j
	}

	b++

	for i := 0; i < n; i++ {
		j = sa[i]

		if j > 0 {
			c0 := src[j]

			if c0 != c1 {
				B[c1] = b
				c1 = c0
				b = B[c1]
			}

			j--

			if src[j] < c1 {
				sa[b] = ^j
			} else {
				sa[b] = j
			}

			b++
			sa[i] = 0

		} else if j < 0 {
			sa[i] = ^j
		}
	}

	// compute sas
	if ptrC == ptrB {
		getCounts(src, C, n, k)
	}

	// find ends of buckets
	getBuckets(C, B, k, true)
	c1 = 0
	b = B[c1]

	for i := n - 1; i >= 0; i-- {
		j = sa[i]

		if j <= 0 {
			continue
		}

		c0 := src[j]

		if c0 != c1 {
			B[c1] = b
			c1 = c0
			b = B[c1]
		}

		j--
		b--

		if src[j] > c1 {
			sa[b] = ^(j + 1)
		} else {
			sa[b] = j
		}

		sa[i] = 0
	}
}

func postProcessLMS(src []int, sa []int, n, m int) int {
	i := 0
	j := 0

	// compact all the sorted substrings into the first m items of sa
	// 2*m must be not larger than n
	for p := sa[i]; p < 0; i++ {
		sa[i] = ^p
		p = sa[i+1]
	}

	if i < m {
		j = i
		i++

		for {
			p := sa[i]
			i++

			if p >= 0 {
				continue
			}

			sa[j] = ^p
			sa[i-1] = 0
			j++

			if j == m {
				break
			}
		}
	}

	// store the length of all substrings
	i = n - 2
	j = n - 1
	c0 := src[n-2]
	c1 := src[n-1]

	if i >= 0 {
		for c0 >= c1 {
			c1 = c0
			i--

			if i < 0 {
				break
			}

			c0 = src[i]
		}
	}

	for i >= 0 {
		c1 = c0
		i--

		if i < 0 {
			break
		}

		c0 = src[i]

		for c0 <= c1 {
			c1 = c0
			i--

			if i < 0 {
				break
			}

			c0 = src[i]
		}

		if i < 0 {
			break
		}

		sa[m+((i+1)>>1)] = j - i
		j = i + 1
		c1 = c0
		i--

		if i >= 0 {
			c0 = src[i]

			for c0 >= c1 {
				c1 = c0
				i--

				if i < 0 {
					break
				}

				c0 = src[i]
			}
		}
	}

	// find the lexicographic names of all substrings
	name := 0
	q := n
	qlen := 0

	for ii := 0; ii < m; ii++ {
		p := sa[ii]
		plen := sa[m+(p>>1)]
		diff := true

		if plen == qlen && q+plen < n {
			j = 0

			for j < plen && src[p+j] == src[q+j] {
				j++
			}

			if j == plen {
				diff = false
			}
		}

		if diff == true {
			name++
			q = p
			qlen = plen
		}

		sa[m+(p>>1)] = name
	}

	return name
}

func induceSuffixArray(src []int, sa []int, ptrBuf1 *[]int, ptrBuf2 *[]int, n int, k int) {
	buf1 := *ptrBuf1
	buf2 := *ptrBuf2

	// compute sal
	if ptrBuf1 == ptrBuf2 {
		getCounts(src, buf1, n, k)
	}

	// find starts of buckets
	getBuckets(buf1, buf2, k, false)

	j := n - 1
	c1 := src[j]
	b := buf2[c1]

	if j > 0 && src[j-1] < c1 {
		sa[b] = ^j
	} else {
		sa[b] = j
	}

	b++

	for i := 0; i < n; i++ {
		j = sa[i]
		sa[i] = ^j

		if j <= 0 {
			continue
		}

		j--
		c0 := src[j]

		if c0 != c1 {
			buf2[c1] = b
			c1 = c0
			b = buf2[c1]
		}

		if j > 0 && src[j-1] < c1 {
			sa[b] = ^j
		} else {
			sa[b] = j
		}

		b++
	}

	// compute sas
	if ptrBuf1 == ptrBuf2 {
		getCounts(src, buf1, n, k)
	}

	// find ends of buckets
	getBuckets(buf1, buf2, k, true)
	c1 = 0
	b = buf2[c1]

	for i := n - 1; i >= 0; i-- {
		j = sa[i]

		if j <= 0 {
			sa[i] = ^j
			continue
		}

		j--
		c0 := src[j]

		if c0 != c1 {
			buf2[c1] = b
			c1 = c0
			b = buf2[c1]
		}

		b--

		if j == 0 || src[j-1] > c1 {
			sa[b] = ^j
		} else {
			sa[b] = j
		}
	}
}

func computeBWT(data []int, sa []int, ptrBuf1 *[]int, ptrBuf2 *[]int, n int, k int) int {
	buf1 := *ptrBuf1
	buf2 := *ptrBuf2

	// compute sal
	if ptrBuf1 == ptrBuf2 {
		getCounts(data, buf1, n, k)
	}

	// find starts of buckets
	getBuckets(buf1, buf2, k, false)
	j := n - 1
	c1 := data[j]
	b := buf2[c1]

	if j > 0 && data[j-1] < c1 {
		sa[b] = ^j
	} else {
		sa[b] = j
	}

	b++

	for i := 0; i < n; i++ {
		j = sa[i]

		if j > 0 {
			j--
			c0 := data[j]
			sa[i] = ^c0

			if c0 != c1 {
				buf2[c1] = b
				c1 = c0
				b = buf2[c1]
			}

			if j > 0 && data[j-1] < c1 {
				sa[b] = ^j
			} else {
				sa[b] = j
			}

			b++
		} else if j != 0 {
			sa[i] = ^j
		}
	}

	// compute sas
	if ptrBuf1 == ptrBuf2 {
		getCounts(data, buf1, n, k)
	}

	// find ends of buckets
	getBuckets(buf1, buf2, k, true)
	c1 = 0
	b = buf2[c1]
	pidx := -1

	for i := n - 1; i >= 0; i-- {
		j = sa[i]

		if j > 0 {
			j--
			c0 := data[j]
			sa[i] = c0

			if c0 != c1 {
				buf2[c1] = b
				c1 = c0
				b = buf2[c1]
			}

			b--

			if j > 0 && data[j-1] > c1 {
				sa[b] = ^data[j-1]
			} else {
				sa[b] = j
			}
		} else if j != 0 {
			sa[i] = ^j
		} else {
			pidx = i
		}
	}

	return pidx
}

// Find the suffix array sa of data[0..n-1] in {0..k-1}^n
// Return the primary index if isbwt is true (0 otherwise)
func computeSuffixArray(data []int, sa []int, fs int, n int, k int, isbwt bool) uint {
	var B, C []int
	var ptrB, ptrC *[]int
	flags := 0

	if k <= 256 {
		C = make([]int, k)
		ptrC = &C

		if k <= fs {
			B = sa[n+fs-k:]
			flags = 1
		} else {
			B = make([]int, k)
			flags = 3
		}

		ptrB = &B

	} else if k <= fs {
		C = sa[n+fs-k:]
		ptrC = &C

		if k <= fs-k {
			B = sa[n+fs-(k+k):]
			ptrB = &B
			flags = 0
		} else if k <= 1024 {
			B = make([]int, k)
			ptrB = &B
			flags = 2
		} else {
			ptrB = ptrC
			B = *ptrB
			flags = 8
		}
	} else {
		B = make([]int, k)
		ptrB = &B
		ptrC = ptrB
		C = *ptrC
		flags = 12
	}

	// stage 1: reduce the problem by at least 1/2, sort all the LMS-substrings
	// find ends of buckets
	getCounts(data, C, n, k)
	getBuckets(C, B, k, true)

	for ii := 0; ii < n; ii++ {
		sa[ii] = 0
	}

	b := -1
	i := n - 1
	j := n
	m := 0
	c0 := data[n-1]
	c1 := c0

	for c0 >= c1 {
		c1 = c0
		i--

		if i < 0 {
			break
		}

		c0 = data[i]
	}

	for i >= 0 {
		for {
			c1 = c0
			i--

			if i < 0 {
				break
			}

			c0 = data[i]

			if c0 > c1 {
				break
			}
		}

		if i < 0 {
			break
		}

		if b >= 0 {
			sa[b] = j
		}

		B[c1]--
		b = B[c1]
		j = i
		m++

		for {
			c1 = c0
			i--

			if i < 0 {
				break
			}

			c0 = data[i]

			if c0 < c1 {
				break
			}
		}
	}

	name := 0

	if m > 1 {
		sortLMSSuffixes(data, sa, ptrC, ptrB, n, k)
		name = postProcessLMS(data, sa, n, m)
	} else if m == 1 {
		sa[b] = j + 1
		name = 1
	}

	// stage 2: solve the reduced problem, recurse if names are not yet unique
	if name < m {
		newfs := (n + fs) - (m + m)

		if flags&13 == 0 {
			if k+name <= newfs {
				newfs -= k
			} else {
				flags |= 8
			}
		}

		j = m + m + newfs - 1

		for ii := m + (n >> 1) - 1; ii >= m; ii-- {
			if sa[ii] != 0 {
				sa[j] = sa[ii] - 1
				j--
			}
		}

		computeSuffixArray(sa[m+newfs:], sa, newfs, m, name, false)

		i = n - 1
		j = m + m - 1
		c0 = data[i]

		for {
			c1 = c0
			i--

			if i < 0 {
				break
			}

			c0 = data[i]

			if c0 < c1 {
				break
			}
		}

		for i >= 0 {
			for {
				c1 = c0
				i--

				if i < 0 {
					break
				}

				c0 = data[i]

				if c0 > c1 {
					break
				}
			}

			if i < 0 {
				break
			}

			sa[j] = i + 1
			j--

			for {
				c1 = c0
				i--

				if i < 0 {
					break
				}

				c0 = data[i]

				if c0 < c1 {
					break
				}
			}
		}

		for ii := 0; ii < m; ii++ {
			sa[ii] = sa[m+sa[ii]]
		}

		if flags&4 != 0 {
			B = make([]int, k)
			ptrB = &B
			ptrC = ptrB
			C = *ptrC
		} else if flags&2 != 0 {
			B = make([]int, k)
			ptrB = &B
		}
	}

	// stage 3: induce the result for the original problem
	if flags&8 != 0 {
		getCounts(data, C, n, k)
	}

	// put all left-most S characters into their buckets
	if m > 1 {
		// find ends of buckets
		getBuckets(C, B, k, true)
		i = m - 1
		j = n
		p := sa[m-1]
		c1 = data[p]

		for {
			c0 = c1
			q := B[c0]

			for q < j {
				j--
				sa[j] = 0
			}

			for {
				j--
				sa[j] = p
				i--

				if i < 0 {
					break
				}

				p = sa[i]
				c1 = data[p]

				if c1 != c0 {
					break
				}
			}

			if i < 0 {
				break
			}
		}

		for j > 0 {
			j--
			sa[j] = 0
		}
	}

	if isbwt == false {
		induceSuffixArray(data, sa, ptrC, ptrB, n, k)
		return uint(0)
	}

	pidx := computeBWT(data, sa, ptrC, ptrB, n, k)
	return uint(pidx)
}
