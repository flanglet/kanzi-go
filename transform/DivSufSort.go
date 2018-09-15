/*
Copyright 2011-2017 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License")
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

const (
	SS_INSERTIONSORT_THRESHOLD = 8
	SS_BLOCKSIZE               = 1024
	SS_MISORT_STACKSIZE        = 16
	SS_SMERGE_STACKSIZE        = 32
	TR_STACKSIZE               = 64
	TR_INSERTIONSORT_THRESHOLD = 8
	MASK_FFFF0000              = -65536    // make 32 bit systems happy
	MASK_FF000000              = -16777216 // make 32 bit systems happy
	MASK_0000FF00              = 65280     // make 32 bit systems happy
)

var SQQ_TABLE = []int{
	0, 16, 22, 27, 32, 35, 39, 42, 45, 48, 50, 53, 55, 57, 59, 61, 64, 65, 67, 69,
	71, 73, 75, 76, 78, 80, 81, 83, 84, 86, 87, 89, 90, 91, 93, 94, 96, 97, 98, 99,
	101, 102, 103, 104, 106, 107, 108, 109, 110, 112, 113, 114, 115, 116, 117, 118,
	119, 120, 121, 122, 123, 124, 125, 126, 128, 128, 129, 130, 131, 132, 133, 134,
	135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 144, 145, 146, 147, 148, 149,
	150, 150, 151, 152, 153, 154, 155, 155, 156, 157, 158, 159, 160, 160, 161, 162,
	163, 163, 164, 165, 166, 167, 167, 168, 169, 170, 170, 171, 172, 173, 173, 174,
	175, 176, 176, 177, 178, 178, 179, 180, 181, 181, 182, 183, 183, 184, 185, 185,
	186, 187, 187, 188, 189, 189, 190, 191, 192, 192, 193, 193, 194, 195, 195, 196,
	197, 197, 198, 199, 199, 200, 201, 201, 202, 203, 203, 204, 204, 205, 206, 206,
	207, 208, 208, 209, 209, 210, 211, 211, 212, 212, 213, 214, 214, 215, 215, 216,
	217, 217, 218, 218, 219, 219, 220, 221, 221, 222, 222, 223, 224, 224, 225, 225,
	226, 226, 227, 227, 228, 229, 229, 230, 230, 231, 231, 232, 232, 233, 234, 234,
	235, 235, 236, 236, 237, 237, 238, 238, 239, 240, 240, 241, 241, 242, 242, 243,
	243, 244, 244, 245, 245, 246, 246, 247, 247, 248, 248, 249, 249, 250, 250, 251,
	251, 252, 252, 253, 253, 254, 254, 255,
}

var LOG_TABLE = []int{
	-1, 0, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
	4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
}

type DivSufSort struct {
	sa         []int
	buffer     []int
	bucketA    [256]int
	bucketB    [65536]int
	ssStack    *Stack
	trStack    *Stack
	mergeStack *Stack
}

func NewDivSufSort() (*DivSufSort, error) {
	this := new(DivSufSort)
	this.sa = make([]int, 0)
	this.buffer = make([]int, 0)
	this.ssStack = newStack(SS_MISORT_STACKSIZE)
	this.trStack = newStack(TR_STACKSIZE)
	this.mergeStack = newStack(SS_SMERGE_STACKSIZE)
	return this, nil
}

func (this *DivSufSort) reset() {
	this.ssStack.index = 0
	this.trStack.index = 0
	this.mergeStack.index = 0

	for i := range this.bucketA {
		this.bucketA[i] = 0
	}

	for i := range this.bucketB {
		this.bucketB[i] = 0
	}
}

func (this *DivSufSort) ComputeSuffixArray(src []byte, sa []int) {
	length := len(src)

	// Lazy dynamic memory allocation
	if len(this.buffer) < length+1 {
		this.buffer = make([]int, length+1)
	}

	for i := range src {
		this.buffer[i] = int(src[i])
	}

	this.sa = sa
	this.reset()
	m := this.sortTypeBstar(this.bucketA[:], this.bucketB[:], length)
	this.constructSuffixArray(this.bucketA[:], this.bucketB[:], length, m)
}

func (this *DivSufSort) constructSuffixArray(bucket_A, bucket_B []int, n, m int) {
	if m > 0 {
		for c1 := 254; c1 >= 0; c1-- {
			idx := c1 << 8
			i := bucket_B[idx+c1+1]
			k := 0
			c2 := -1

			// Scan the suffix array from right to left.
			for j := bucket_A[c1+1] - 1; j >= i; j-- {
				s := this.sa[j]
				this.sa[j] = ^s

				if s <= 0 {
					continue
				}

				s--
				c0 := this.buffer[s]

				if s > 0 && this.buffer[s-1] > c0 {
					s = ^s
				}

				if c0 != c2 {
					if c2 >= 0 {
						bucket_B[idx+c2] = k
					}

					c2 = c0
					k = bucket_B[idx+c2]
				}

				this.sa[k] = s
				k--
			}
		}
	}

	c2 := this.buffer[n-1]
	k := bucket_A[c2]

	if this.buffer[n-2] < c2 {
		this.sa[k] = ^(n - 1)
	} else {
		this.sa[k] = n - 1
	}

	k++

	// Scan the suffix array from left to right.
	for i := 0; i < n; i++ {
		s := this.sa[i]

		if s <= 0 {
			this.sa[i] = ^s
			continue
		}

		s--
		c0 := this.buffer[s]

		if s == 0 || this.buffer[s-1] < c0 {
			s = ^s
		}

		if c0 != c2 {
			bucket_A[c2] = k
			c2 = c0
			k = bucket_A[c2]
		}

		this.sa[k] = s
		k++
	}
}

func (this *DivSufSort) ComputeBWT(src []byte, sa []int) int {
	length := len(src)

	// Lazy dynamic memory allocation
	if len(this.buffer) < length+1 {
		this.buffer = make([]int, length+1)
	}

	for i := 0; i < length; i++ {
		this.buffer[i] = int(src[i])
	}

	this.sa = sa
	this.reset()
	m := this.sortTypeBstar(this.bucketA[:], this.bucketB[:], length)
	return this.constructBWT(this.bucketA[:], this.bucketB[:], length, m)
}

func (this *DivSufSort) constructBWT(bucket_A, bucket_B []int, n, m int) int {
	pIdx := -1

	if m > 0 {
		for c1 := 254; c1 >= 0; c1-- {
			idx := c1 << 8
			i := bucket_B[idx+c1+1]
			k := 0
			c2 := -1

			// Scan the suffix array from right to left.
			for j := bucket_A[c1+1] - 1; j >= i; j-- {
				s := this.sa[j]

				if s <= 0 {
					if s != 0 {
						this.sa[j] = ^s
					}

					continue
				}

				s--
				c0 := this.buffer[s]
				this.sa[j] = ^c0

				if s > 0 && this.buffer[s-1] > c0 {
					s = ^s
				}

				if c0 != c2 {
					if c2 >= 0 {
						bucket_B[idx+c2] = k
					}

					c2 = c0
					k = bucket_B[idx+c2]
				}

				this.sa[k] = s
				k--
			}
		}
	}

	c2 := this.buffer[n-1]
	k := bucket_A[c2]

	if this.buffer[n-2] < c2 {
		this.sa[k] = ^this.buffer[n-2]
	} else {
		this.sa[k] = n - 1
	}

	k++

	// Scan the suffix array from left to right.
	for i := 0; i < n; i++ {
		s := this.sa[i]

		if s <= 0 {
			if s != 0 {
				this.sa[i] = ^s
			} else {
				pIdx = i
			}

			continue
		}

		s--
		c0 := this.buffer[s]
		this.sa[i] = c0

		if s > 0 && this.buffer[s-1] < c0 {
			s = ^this.buffer[s-1]
		}

		if c0 != c2 {
			bucket_A[c2] = k
			c2 = c0
			k = bucket_A[c2]
		}

		this.sa[k] = s
		k++
	}

	return pIdx
}

func (this *DivSufSort) sortTypeBstar(bucket_A, bucket_B []int, n int) int {
	m := n
	c0 := this.buffer[n-1]
	arr := this.sa

	// Count the number of occurrences of the first one or two characters of each
	// type A, B and B* suffix. Moreover, store the beginning position of all
	// type B* suffixes into the array SA.
	for i := n - 1; i >= 0; {
		c1 := c0

		for c0 >= c1 {
			c1 = c0
			bucket_A[c1]++
			i--

			if i < 0 {
				break
			}

			c0 = this.buffer[i]
		}

		if i < 0 {
			break
		}

		bucket_B[(c0<<8)+c1]++
		m--
		arr[m] = i
		i--
		c1 = c0

		for i >= 0 {
			c0 = this.buffer[i]

			if c0 > c1 {
				break
			}

			bucket_B[(c1<<8)+c0]++
			c1 = c0
			i--
		}
	}

	m = n - m
	c0 = 0

	// A type B* suffix is lexicographically smaller than a type B suffix that
	// begins with the same first two characters.

	// Calculate the index of start/end point of each bucket.
	for i, j := 0, 0; c0 < 256; c0++ {
		t := i + bucket_A[c0]
		bucket_A[c0] = i + j // start point
		idx := c0 << 8
		i = t + bucket_B[idx+c0]

		for c1 := c0 + 1; c1 < 256; c1++ {
			j += bucket_B[idx+c1]
			bucket_B[idx+c1] = j // end point
			i += bucket_B[(c1<<8)+c0]
		}
	}

	if m > 0 {
		// Sort the type B* suffixes by their first two characters.
		pab := n - m

		for i := m - 2; i >= 0; i-- {
			t := arr[pab+i]
			idx := (this.buffer[t] << 8) + this.buffer[t+1]
			bucket_B[idx]--
			arr[bucket_B[idx]] = i
		}

		t := arr[pab+m-1]
		c0 = (this.buffer[t] << 8) + this.buffer[t+1]
		bucket_B[c0]--
		arr[bucket_B[c0]] = m - 1

		// Sort the type B* substrings using ssSort.
		bufSize := n - m - m
		c0 = 254

		for j := m; j > 0; c0-- {
			idx := c0 << 8

			for c1 := 255; c1 > c0; c1-- {
				i := bucket_B[idx+c1]

				if j-i > 1 {
					this.ssSort(pab, i, j, m, bufSize, 2, n, arr[i] == m-1)
				}

				j = i
			}
		}

		// Compute ranks of type B* substrings.
		for i := m - 1; i >= 0; i-- {
			if arr[i] >= 0 {
				j := i

				for {
					arr[m+arr[i]] = i
					i--

					if i < 0 || arr[i] < 0 {
						break
					}
				}

				arr[i+1] = i - j

				if i <= 0 {
					break
				}
			}

			j := i

			for {
				arr[i] = ^arr[i]
				arr[m+arr[i]] = j
				i--

				if arr[i] >= 0 {
					break
				}
			}

			arr[m+arr[i]] = j
		}

		// Construct the inverse suffix array of type B* suffixes using trSort.
		this.trSort(m, 1)

		// Set the sorted order of type B* suffixes.
		c0 = this.buffer[n-1]
		var c1 int

		for i, j := n-1, m; i >= 0; {
			i--
			c1 = c0

			for i >= 0 {
				c0 = this.buffer[i]

				if c0 < c1 {
					break
				}

				c1 = c0
				i--
			}

			if i >= 0 {
				tt := i
				i--
				c1 = c0

				for i >= 0 {
					c0 = this.buffer[i]

					if c0 > c1 {
						break
					}

					c1 = c0
					i--
				}

				j--

				if tt == 0 || tt-i > 1 {
					arr[arr[m+j]] = tt
				} else {
					arr[arr[m+j]] = ^tt
				}
			}
		}

		// Calculate the index of start/end point of each bucket.
		bucket_B[len(bucket_B)-1] = n // end
		k := m - 1

		for c0 = 254; c0 >= 0; c0-- {
			i := bucket_A[c0+1] - 1
			c2 := c0 << 8

			for c1 := 255; c1 > c0; c1-- {
				tt := i - bucket_B[(c1<<8)+c0]
				bucket_B[(c1<<8)+c0] = i // end point
				i = tt

				// Move all type B* suffixes to the correct position.
				// Typically very small number of copies
				for j := bucket_B[c2+c1]; j <= k; {
					arr[i] = arr[k]
					i--
					k--
				}
			}

			bucket_B[c2+c0+1] = i - bucket_B[c2+c0] + 1 //start point
			bucket_B[c2+c0] = i                         // end point
		}
	}

	return m
}

// Sub String Sort
func (this *DivSufSort) ssSort(pa, first, last, buf, bufSize, depth, n int, lastSuffix bool) {
	if lastSuffix == true {
		first++
	}

	limit := 0
	middle := last

	if bufSize < SS_BLOCKSIZE && bufSize < last-first {
		limit = ssIsqrt(last - first)

		if bufSize < limit {
			if limit > SS_BLOCKSIZE {
				limit = SS_BLOCKSIZE
			}

			middle = last - limit
			buf = middle
			bufSize = limit
		} else {
			limit = 0
		}
	}

	var a int
	i := 0

	for a = first; middle-a > SS_BLOCKSIZE; a += SS_BLOCKSIZE {
		this.ssMultiKeyIntroSort(pa, a, a+SS_BLOCKSIZE, depth)
		curBufSize := last - (a + SS_BLOCKSIZE)
		var curBuf int

		if curBufSize > bufSize {
			curBuf = a + SS_BLOCKSIZE
		} else {
			curBufSize = bufSize
			curBuf = buf
		}

		k := SS_BLOCKSIZE
		b := a

		for j := i; j&1 != 0; j >>= 1 {
			this.ssSwapMerge(pa, b-k, b, b+k, curBuf, curBufSize, depth)
			b -= k
			k <<= 1
		}

		i++
	}

	this.ssMultiKeyIntroSort(pa, a, middle, depth)
	k := SS_BLOCKSIZE

	for i != 0 {
		if i&1 != 0 {
			this.ssSwapMerge(pa, a-k, a, middle, buf, bufSize, depth)
			a -= k
		}

		k <<= 1
		i >>= 1
	}

	if limit != 0 {
		this.ssMultiKeyIntroSort(pa, middle, last, depth)
		this.ssInplaceMerge(pa, first, middle, last, depth)
	}

	if lastSuffix == true {
		i = this.sa[first-1]
		p1 := this.sa[pa+i]
		p11 := n - 2

		for a = first; a < last && (this.sa[a] < 0 || this.ssCompare4(p1, p11, pa+this.sa[a], depth) > 0); a++ {
			this.sa[a-1] = this.sa[a]
		}

		this.sa[a-1] = i
	}
}

func (this *DivSufSort) ssCompare4(pa, pb, p2, depth int) int {
	u1n := pb + 2
	u1 := pa + depth
	u2n := this.sa[p2+1] + 2
	u2 := this.sa[p2] + depth

	if u1n-u1 > u2n-u2 {
		for u2 < u2n && this.buffer[u1] == this.buffer[u2] {
			u1++
			u2++
		}
	} else {
		for u1 < u1n && this.buffer[u1] == this.buffer[u2] {
			u1++
			u2++
		}
	}

	if u1 < u1n {
		if u2 < u2n {
			return this.buffer[u1] - this.buffer[u2]
		}

		return 1
	}

	if u2 < u2n {
		return -1
	}

	return 0
}

func (this *DivSufSort) ssCompare3(p1, p2, depth int) int {
	u1n := this.sa[p1+1] + 2
	u1 := this.sa[p1] + depth
	u2n := this.sa[p2+1] + 2
	u2 := this.sa[p2] + depth
	buf := this.buffer

	if u1n-u1 > u2n-u2 {
		for u2 < u2n && buf[u1] == buf[u2] {
			u1++
			u2++
		}
	} else {
		for u1 < u1n && buf[u1] == buf[u2] {
			u1++
			u2++
		}
	}

	if u1 < u1n {
		if u2 < u2n {
			return buf[u1] - buf[u2]
		}

		return 1
	}

	if u2 < u2n {
		return -1
	}

	return 0
}

func (this *DivSufSort) ssInplaceMerge(pa, first, middle, last, depth int) {
	arr := this.sa

	for {
		var p, x int

		if arr[last-1] < 0 {
			x = 1
			p = pa + ^arr[last-1]
		} else {
			x = 0
			p = pa + arr[last-1]
		}

		a := first
		r := -1
		half := (middle - first) >> 1

		for len := middle - first; len > 0; len = half {
			b := a + half
			var c int

			if arr[b] >= 0 {
				c = arr[b]
			} else {
				c = ^arr[b]
			}

			q := this.ssCompare3(pa+c, p, depth)

			if q < 0 {
				a = b + 1
				half -= ((len & 1) ^ 1)
			} else {
				r = q
			}

			half >>= 1
		}

		if a < middle {
			if r == 0 {
				arr[a] = ^arr[a]
			}

			this.ssRotate(a, middle, last)
			last -= (middle - a)
			middle = a

			if first == middle {
				break
			}
		}

		last--

		if x != 0 {
			last--

			for arr[last] < 0 {
				last--
			}
		}

		if middle == last {
			break
		}
	}
}

func (this *DivSufSort) ssRotate(first, middle, last int) {
	l := middle - first
	r := last - middle
	arr := this.sa

	for l > 0 && r > 0 {
		if l == r {
			this.ssBlockSwap(first, middle, l)
			break
		}

		if l < r {
			a := last - 1
			b := middle - 1
			t := arr[a]

			for {
				arr[a] = arr[b]
				a--
				arr[b] = arr[a]
				b--

				if b < first {
					arr[a] = t
					last = a
					r -= (l + 1)

					if r <= l {
						break
					}

					a--
					b = middle - 1
					t = arr[a]
				}
			}
		} else {
			a := first
			b := middle
			t := arr[a]

			for {
				arr[a] = arr[b]
				a++
				arr[b] = arr[a]
				b++

				if last <= b {
					arr[a] = t
					first = a + 1
					l -= (r + 1)

					if l <= r {
						break
					}

					a++
					b = middle
					t = arr[a]
				}
			}
		}
	}
}

func (this *DivSufSort) ssBlockSwap(a, b, n int) {
	for n > 0 {
		this.sa[a], this.sa[b] = this.sa[b], this.sa[a]
		n--
		a++
		b++
	}
}

func getIndex(a int) int {
	if a >= 0 {
		return a
	}

	return ^a
}

func (this *DivSufSort) ssSwapMerge(pa, first, middle, last, buf, bufSize, depth int) {
	arr := this.sa
	check := 0

	for {
		if last-middle <= bufSize {
			if first < middle && middle < last {
				this.ssMergeBackward(pa, first, middle, last, buf, depth)
			}

			if check&1 != 0 || (check&2 != 0 && this.ssCompare3(pa+getIndex(this.sa[first-1]),
				pa+arr[first], depth) == 0) {
				arr[first] = ^arr[first]
			}

			if check&4 != 0 && this.ssCompare3(pa+getIndex(arr[last-1]), pa+arr[last], depth) == 0 {
				arr[last] = ^arr[last]
			}

			se := this.mergeStack.pop()

			if se == nil {
				return
			}

			first = se.a
			middle = se.b
			last = se.c
			check = se.d
			continue
		}

		if middle-first <= bufSize {
			if first < middle {
				this.ssMergeForward(pa, first, middle, last, buf, depth)
			}

			if check&1 != 0 || (check&2 != 0 && this.ssCompare3(pa+getIndex(arr[first-1]),
				pa+arr[first], depth) == 0) {
				arr[first] = ^arr[first]
			}

			if check&4 != 0 && this.ssCompare3(pa+getIndex(arr[last-1]), pa+arr[last], depth) == 0 {
				arr[last] = ^arr[last]
			}

			se := this.mergeStack.pop()

			if se == nil {
				return
			}

			first = se.a
			middle = se.b
			last = se.c
			check = se.d
			continue
		}

		m := 0
		var len int

		if middle-first < last-middle {
			len = middle - first
		} else {
			len = last - middle
		}

		for half := len >> 1; len > 0; len, half = half, half>>1 {
			if this.ssCompare3(pa+getIndex(arr[middle+m+half]), pa+getIndex(arr[middle-m-half-1]), depth) < 0 {
				m += (half + 1)
				half -= ((len & 1) ^ 1)
			}
		}

		if m > 0 {
			lm := middle - m
			rm := middle + m
			this.ssBlockSwap(lm, middle, m)
			l := middle
			r := l
			next := 0

			if rm < last {
				if arr[rm] < 0 {
					arr[rm] = ^arr[rm]

					if first < lm {
						l--

						for arr[l] < 0 {
							l--
						}

						next |= 4
					}

					next |= 1
				} else if first < lm {
					for arr[r] < 0 {
						r++
					}

					next |= 2
				}
			}

			if l-first <= last-r {
				this.mergeStack.push(r, rm, last, (next&3)|(check&4), 0)
				middle = lm
				last = l
				check = (check & 3) | (next & 4)
			} else {
				if r == middle && (next&2) != 0 {
					next ^= 6
				}

				this.mergeStack.push(first, lm, l, (check&3)|(next&4), 0)
				first = r
				middle = rm
				check = (next & 3) | (check & 4)
			}
		} else {
			if this.ssCompare3(pa+getIndex(arr[middle-1]), pa+arr[middle], depth) == 0 {
				arr[middle] = ^arr[middle]
			}

			if check&1 != 0 || (check&2 != 0 && this.ssCompare3(pa+getIndex(this.sa[first-1]),
				pa+arr[first], depth) == 0) {
				arr[first] = ^arr[first]
			}

			if check&4 != 0 && this.ssCompare3(pa+getIndex(arr[last-1]), pa+arr[last], depth) == 0 {
				arr[last] = ^arr[last]
			}

			se := this.mergeStack.pop()

			if se == nil {
				return
			}

			first = se.a
			middle = se.b
			last = se.c
			check = se.d
		}
	}
}

func (this *DivSufSort) ssMergeForward(pa, first, middle, last, buf, depth int) {
	arr := this.sa
	bufEnd := buf + middle - first - 1
	this.ssBlockSwap(buf, first, middle-first)
	a := first
	b := buf
	c := middle
	t := arr[a]

	for {
		if r := this.ssCompare3(pa+arr[b], pa+arr[c], depth); r < 0 {
			for {
				arr[a] = arr[b]
				a++

				if bufEnd <= b {
					arr[bufEnd] = t
					return
				}

				arr[b] = arr[a]
				b++

				if arr[b] >= 0 {
					break
				}
			}
		} else if r > 0 {
			for {
				arr[a] = arr[c]
				a++
				arr[c] = arr[a]
				c++

				if last <= c {
					for b < bufEnd {
						arr[a] = arr[b]
						a++
						arr[b] = arr[a]
						b++
					}

					arr[a] = arr[b]
					arr[b] = t
					return
				}

				if arr[c] >= 0 {
					break
				}
			}
		} else {
			arr[c] = ^arr[c]

			for {
				arr[a] = arr[b]
				a++

				if bufEnd <= b {
					arr[bufEnd] = t
					return
				}

				arr[b] = arr[a]
				b++

				if arr[b] >= 0 {
					break
				}
			}

			for {
				arr[a] = arr[c]
				a++
				arr[c] = arr[a]
				c++

				if last <= c {
					for b < bufEnd {
						arr[a] = arr[b]
						a++
						arr[b] = arr[a]
						b++
					}

					arr[a] = arr[b]
					arr[b] = t
					return
				}

				if arr[c] >= 0 {
					break
				}
			}
		}
	}
}

func (this *DivSufSort) ssMergeBackward(pa, first, middle, last, buf, depth int) {
	arr := this.sa
	bufEnd := buf + last - middle - 1
	this.ssBlockSwap(buf, middle, last-middle)
	x := 0
	var p1, p2 int

	if arr[bufEnd] < 0 {
		p1 = pa + ^arr[bufEnd]
		x |= 1
	} else {
		p1 = pa + arr[bufEnd]
	}

	if arr[middle-1] < 0 {
		p2 = pa + ^arr[middle-1]
		x |= 2
	} else {
		p2 = pa + arr[middle-1]
	}

	a := last - 1
	b := bufEnd
	c := middle - 1
	t := arr[a]

	for {
		if r := this.ssCompare3(p1, p2, depth); r > 0 {
			if x&1 != 0 {
				for {
					arr[a] = arr[b]
					a--
					arr[b] = arr[a]
					b--

					if arr[b] >= 0 {
						break
					}
				}

				x ^= 1
			}

			arr[a] = arr[b]
			a--

			if b <= buf {
				arr[buf] = t
				break
			}

			arr[b] = arr[a]
			b--

			if arr[b] < 0 {
				p1 = pa + ^arr[b]
				x |= 1
			} else {
				p1 = pa + arr[b]
			}
		} else if r < 0 {
			if x&2 != 0 {
				for {
					arr[a] = arr[c]
					a--
					arr[c] = arr[a]
					c--

					if arr[c] >= 0 {
						break
					}
				}

				x ^= 2
			}

			arr[a] = arr[c]
			a--
			arr[c] = arr[a]
			c--

			if c < first {
				for buf < b {
					arr[a] = arr[b]
					a--
					arr[b] = arr[a]
					b--
				}

				arr[a] = arr[b]
				arr[b] = t
				break
			}

			if arr[c] < 0 {
				p2 = pa + ^arr[c]
				x |= 2
			} else {
				p2 = pa + arr[c]
			}
		} else { // r = 0
			if x&1 != 0 {
				for {
					arr[a] = arr[b]
					a--
					arr[b] = arr[a]
					b--

					if arr[b] >= 0 {
						break
					}
				}

				x ^= 1
			}

			arr[a] = ^arr[b]
			a--

			if b <= buf {
				arr[buf] = t
				break
			}

			arr[b] = arr[a]
			b--

			if x&2 != 0 {
				for {
					arr[a] = arr[c]
					a--
					arr[c] = arr[a]
					c--

					if arr[c] >= 0 {
						break
					}
				}

				x ^= 2
			}

			arr[a] = arr[c]
			a--
			arr[c] = arr[a]
			c--

			if c < first {
				for buf < b {
					arr[a] = arr[b]
					a--
					arr[b] = arr[a]
					b--
				}

				arr[a] = arr[b]
				arr[b] = t
				break
			}

			if arr[b] < 0 {
				p1 = pa + ^arr[b]
				x |= 1
			} else {
				p1 = pa + arr[b]
			}

			if arr[c] < 0 {
				p2 = pa + ^arr[c]
				x |= 2
			} else {
				p2 = pa + arr[c]
			}
		}
	}
}

func (this *DivSufSort) ssInsertionSort(pa, first, last, depth int) {
	arr := this.sa

	for i := last - 2; i >= first; i-- {
		t := pa + arr[i]
		j := i + 1
		var r int

		for r = this.ssCompare3(t, pa+arr[j], depth); r > 0; {
			for {
				arr[j-1] = arr[j]
				j++

				if j >= last || arr[j] >= 0 {
					break
				}
			}

			if j >= last {
				break
			}

			r = this.ssCompare3(t, pa+arr[j], depth)
		}

		if r == 0 {
			arr[j] = ^arr[j]
		}

		arr[j-1] = t - pa
	}
}

func ssIsqrt(x int) int {
	if x >= SS_BLOCKSIZE*SS_BLOCKSIZE {
		return SS_BLOCKSIZE
	}

	var e int

	if x&MASK_FFFF0000 != 0 {
		if x&MASK_FF000000 != 0 {
			e = 24 + LOG_TABLE[(x>>24)&0xFF]
		} else {
			e = 16 + LOG_TABLE[(x>>16)&0xFF]
		}
	} else {
		if x&MASK_0000FF00 != 0 {
			e = 8 + LOG_TABLE[(x>>8)&0xFF]
		} else {
			e = LOG_TABLE[x&0xFF]
		}
	}

	if e < 8 {
		return SQQ_TABLE[x] >> 4
	}

	var y int

	if e >= 16 {
		y = SQQ_TABLE[x>>uint((e-6)-(e&1))] << uint((e>>1)-7)

		if e >= 24 {
			y = (y + 1 + x/y) >> 1
		}

		y = (y + 1 + x/y) >> 1
	} else {
		y = (SQQ_TABLE[x>>uint((e-6)-(e&1))] >> uint(7-(e>>1))) + 1
	}

	if x < y*y {
		return y - 1
	}

	return y
}

func (this *DivSufSort) ssMultiKeyIntroSort(pa, first, last, depth int) {
	limit := ssIlg(last - first)
	x := 0

	for {
		if last-first <= SS_INSERTIONSORT_THRESHOLD {
			if last-first > 1 {
				this.ssInsertionSort(pa, first, last, depth)
			}

			se := this.ssStack.pop()

			if se == nil {
				return
			}

			first = se.a
			last = se.b
			depth = se.c
			limit = se.d
			continue
		}

		idx := depth

		// Create slice aliases
		// NOTE: buf1 can only replace this.buffer when the index is guaranteed
		// to be positive or zero (not in a pattern like this.buffer[...-1]) !!!
		buf1 := this.buffer[idx:len(this.buffer)]
		buf2 := this.sa[pa:len(this.sa)]

		if limit == 0 {
			this.ssHeapSort(idx, pa, first, last-first)
		}

		limit--
		var a int

		if limit < 0 {
			v := buf1[buf2[this.sa[first]]]

			for a = first + 1; a < last; a++ {
				if x = buf1[buf2[this.sa[a]]]; x != v {
					if a-first > 1 {
						break
					}

					v = x
					first = a
				}
			}

			if this.buffer[idx+buf2[this.sa[first]]-1] < v {
				first = this.ssPartition(pa, first, a, depth)
			}

			if a-first <= last-a {
				if a-first > 1 {
					this.ssStack.push(a, last, depth, -1, 0)
					last = a
					depth++
					limit = ssIlg(a - first)
				} else {
					first = a
					limit = -1
				}
			} else {
				if last-a > 1 {
					this.ssStack.push(first, a, depth+1, ssIlg(a-first), 0)
					first = a
					limit = -1
				} else {
					last = a
					depth++
					limit = ssIlg(a - first)
				}
			}

			continue
		}

		// choose pivot
		a = this.ssPivot(idx, pa, first, last)
		v := buf1[buf2[this.sa[a]]]
		this.sa[a], this.sa[first] = this.sa[first], this.sa[a]
		b := first + 1

		// partition
		for b < last {
			if x = buf1[buf2[this.sa[b]]]; x != v {
				break
			}

			b++
		}

		a = b

		if a < last && x < v {
			b++

			for b < last {
				if x = buf1[buf2[this.sa[b]]]; x > v {
					break
				}

				if x == v {
					this.sa[a], this.sa[b] = this.sa[b], this.sa[a]
					a++
				}

				b++
			}
		}

		c := last - 1

		for c > b {
			if x = buf1[buf2[this.sa[c]]]; x != v {
				break
			}

			c--
		}

		d := c

		if b < d && x > v {
			c--

			for c > b {
				if x = buf1[buf2[this.sa[c]]]; x < v {
					break
				}

				if x == v {
					this.sa[c], this.sa[d] = this.sa[d], this.sa[c]
					d--
				}

				c--
			}
		}

		for b < c {
			this.sa[b], this.sa[c] = this.sa[c], this.sa[b]
			b++

			for b < c {
				if x = buf1[buf2[this.sa[b]]]; x > v {
					break
				}

				if x == v {
					this.sa[a], this.sa[b] = this.sa[b], this.sa[a]
					a++
				}

				b++
			}

			c--

			for c > b {
				if x = buf1[buf2[this.sa[c]]]; x < v {
					break
				}

				if x == v {
					this.sa[c], this.sa[d] = this.sa[d], this.sa[c]
					d--
				}

				c--
			}
		}

		if a <= d {
			c = b - 1
			s := a - first
			t := b - a

			if s > t {
				s = t
			}

			for e, f := first, b-s; s > 0; s-- {
				this.sa[e], this.sa[f] = this.sa[f], this.sa[e]
				e++
				f++
			}

			s = d - c
			t = last - d - 1

			if s > t {
				s = t
			}

			for e, f := b, last-s; s > 0; s-- {
				this.sa[e], this.sa[f] = this.sa[f], this.sa[e]
				e++
				f++
			}

			a = first + (b - a)
			c = last - (d - c)

			if v <= this.buffer[idx+buf2[this.sa[a]]-1] {
				b = a
			} else {
				b = this.ssPartition(pa, a, c, depth)
			}

			if a-first <= last-c {
				if last-c <= c-b {
					this.ssStack.push(b, c, depth+1, ssIlg(c-b), 0)
					this.ssStack.push(c, last, depth, limit, 0)
					last = a
				} else if a-first <= c-b {
					this.ssStack.push(c, last, depth, limit, 0)
					this.ssStack.push(b, c, depth+1, ssIlg(c-b), 0)
					last = a
				} else {
					this.ssStack.push(c, last, depth, limit, 0)
					this.ssStack.push(first, a, depth, limit, 0)
					first = b
					last = c
					depth++
					limit = ssIlg(c - b)
				}
			} else {
				if a-first <= c-b {
					this.ssStack.push(b, c, depth+1, ssIlg(c-b), 0)
					this.ssStack.push(first, a, depth, limit, 0)
					first = c
				} else if last-c <= c-b {
					this.ssStack.push(first, a, depth, limit, 0)
					this.ssStack.push(b, c, depth+1, ssIlg(c-b), 0)
					first = c
				} else {
					this.ssStack.push(first, a, depth, limit, 0)
					this.ssStack.push(c, last, depth, limit, 0)
					first = b
					last = c
					depth++
					limit = ssIlg(c - b)
				}
			}
		} else {
			if this.buffer[idx+buf2[this.sa[first]]-1] < v {
				first = this.ssPartition(pa, first, last, depth)
				limit = ssIlg(last - first)
			} else {
				limit++
			}

			depth++
		}
	}
}

func (this *DivSufSort) ssPivot(td, pa, first, last int) int {
	t := last - first
	middle := first + (t >> 1)
	buf0 := this.buffer[td:]
	buf1 := this.sa[pa:]

	if t <= 512 {
		if t <= 32 {
			return this.ssMedian3(buf0, buf1, first, middle, last-1)
		}

		return this.ssMedian5(buf0, buf1, first, first+(t>>2), middle, last-1-(t>>2), last-1)
	}

	t >>= 3
	first = this.ssMedian3(buf0, buf1, first, first+t, first+(t<<1))
	middle = this.ssMedian3(buf0, buf1, middle-t, middle, middle+t)
	last = this.ssMedian3(buf0, buf1, last-1-(t<<1), last-1-t, last-1)
	return this.ssMedian3(buf0, buf1, first, middle, last)
}

func (this *DivSufSort) ssMedian5(buf0, buf1 []int, v1, v2, v3, v4, v5 int) int {
	if buf0[buf1[this.sa[v2]]] > buf0[buf1[this.sa[v3]]] {
		v2, v3 = v3, v2
	}

	if buf0[buf1[this.sa[v4]]] > buf0[buf1[this.sa[v5]]] {
		v4, v5 = v5, v4
	}

	if buf0[buf1[this.sa[v2]]] > buf0[buf1[this.sa[v4]]] {
		v2, v4 = v4, v2
		v3, v5 = v5, v3
	}

	if buf0[buf1[this.sa[v1]]] > buf0[buf1[this.sa[v3]]] {
		v1, v3 = v3, v1
	}

	if buf0[buf1[this.sa[v1]]] > buf0[buf1[this.sa[v4]]] {
		v1, v4 = v4, v1
		v3, v5 = v5, v3
	}

	if buf0[buf1[this.sa[v3]]] > buf0[buf1[this.sa[v4]]] {
		return v4
	}

	return v3
}

func (this *DivSufSort) ssMedian3(buf0, buf1 []int, v1, v2, v3 int) int {
	if buf0[buf1[this.sa[v1]]] > buf0[buf1[this.sa[v2]]] {
		t := v1
		v1 = v2
		v2 = t
	}

	if buf0[buf1[this.sa[v2]]] > buf0[buf1[this.sa[v3]]] {
		if buf0[buf1[this.sa[v1]]] > buf0[buf1[this.sa[v3]]] {
			return v1
		}

		return v3
	}

	return v2
}

func (this *DivSufSort) ssPartition(pa, first, last, depth int) int {
	buf1 := this.sa
	buf2 := this.sa[pa:]
	a := first - 1
	b := last
	d := depth - 1

	for {
		a++

		for a < b && buf2[buf1[a]]+d >= buf2[buf1[a]+1] {
			buf1[a] = ^buf1[a]
			a++
		}

		b--

		for b > a && buf2[buf1[b]]+d < buf2[buf1[b]+1] {
			b--
		}

		if b <= a {
			break
		}

		buf1[a], buf1[b] = ^buf1[b], buf1[a]
	}

	if first < a {
		buf1[first] = ^buf1[first]
	}

	return a
}

func (this *DivSufSort) ssHeapSort(idx, pa, saIdx, size int) {
	m := size

	if size&1 == 0 {
		m--

		if this.buffer[idx+this.sa[pa+this.sa[saIdx+(m>>1)]]] < this.buffer[idx+this.sa[pa+this.sa[saIdx+m]]] {
			this.sa[saIdx+(m>>1)], this.sa[saIdx+m] = this.sa[saIdx+m], this.sa[saIdx+(m>>1)]
		}
	}

	buf1 := this.buffer[idx:]
	buf2 := this.sa[pa:]
	buf3 := this.sa[saIdx:]

	for i := (m >> 1) - 1; i >= 0; i-- {
		this.ssFixDown(buf1, buf2, buf3, i, m)
	}

	if size&1 == 0 {
		this.sa[saIdx], this.sa[saIdx+m] = this.sa[saIdx+m], this.sa[saIdx]
		this.ssFixDown(buf1, buf2, buf3, 0, m)
	}

	for i := m - 1; i > 0; i-- {
		t := this.sa[saIdx]
		this.sa[saIdx] = this.sa[saIdx+i]
		this.ssFixDown(buf1, buf2, buf3, 0, i)
		this.sa[saIdx+i] = t
	}
}

func (this *DivSufSort) ssFixDown(buf1, buf2, buf3 []int, i, size int) {
	v := buf3[i]
	c := buf1[buf2[v]]
	j := (i << 1) + 1

	for j < size {
		k := j
		j++
		d := buf1[buf2[buf3[k]]]
		e := buf1[buf2[buf3[j]]]

		if d < e {
			k = j
			d = e
		}

		if d <= c {
			break
		}

		buf3[i] = buf3[k]
		i = k
		j = (i << 1) + 1
	}

	buf3[i] = v
}

func ssIlg(n int) int {
	if n&0xFF00 != 0 {
		return 8 + LOG_TABLE[(n>>8)&0xFF]
	}

	return LOG_TABLE[n&0xFF]
}

// Tandem Repeat Sort
func (this *DivSufSort) trSort(n, depth int) {
	arr := this.sa
	budget := &TRBudget{chance: trIlg(n) * 2 / 3, remain: n, incVal: n}

	for isad := n + depth; arr[0] > -n; isad += (isad - n) {
		first := 0
		skip := 0
		unsorted := 0

		for {
			t := arr[first]

			if t < 0 {
				first -= t
				skip += t
			} else {
				if skip != 0 {
					arr[first+skip] = skip
					skip = 0
				}

				last := arr[n+t] + 1

				if last-first > 1 {
					budget.count = 0
					this.trIntroSort(n, isad, first, last, budget)

					if budget.count != 0 {
						unsorted += budget.count
					} else {
						skip = first - last
					}
				} else if last-first == 1 {
					skip = -1
				}

				first = last
			}

			if first >= n {
				break
			}
		}

		if skip != 0 {
			arr[first+skip] = skip
		}

		if unsorted == 0 {
			break
		}
	}
}

func (this *DivSufSort) trPartition(isad, first, middle, last, v int) (int, int) {
	x := 0
	b := middle
	arr := this.sa[isad:len(this.sa)]

	for b < last {
		if x = arr[this.sa[b]]; x != v {
			break
		}

		b++
	}

	a := b

	if a < last && x < v {
		b++

		for b < last {
			if x = arr[this.sa[b]]; x > v {
				break
			}

			if x == v {
				this.sa[a], this.sa[b] = this.sa[b], this.sa[a]
				a++
			}

			b++
		}
	}

	c := last - 1

	for c > b {
		if x = arr[this.sa[c]]; x != v {
			break
		}

		c--
	}

	d := c

	if b < d && x > v {
		c--

		for c > b {
			if x = arr[this.sa[c]]; x < v {
				break
			}

			if x == v {
				this.sa[c], this.sa[d] = this.sa[d], this.sa[c]
				d--
			}

			c--
		}
	}

	for b < c {
		this.sa[b], this.sa[c] = this.sa[c], this.sa[b]
		b++

		for b < c {
			if x = arr[this.sa[b]]; x > v {
				break
			}

			if x == v {
				this.sa[a], this.sa[b] = this.sa[b], this.sa[a]
				a++
			}

			b++
		}

		c--

		for c > b {
			if x = arr[this.sa[c]]; x < v {
				break
			}

			if x == v {
				this.sa[c], this.sa[d] = this.sa[d], this.sa[c]
				d--
			}

			c--
		}
	}

	if a <= d {
		c = b - 1
		s := a - first

		if s > b-a {
			s = b - a
		}

		for e, f := first, b-s; s > 0; s-- {
			this.sa[e], this.sa[f] = this.sa[f], this.sa[e]
			e++
			f++
		}

		s = d - c

		if s >= last-d {
			s = last - d - 1
		}

		for e, f := b, last-s; s > 0; s-- {
			this.sa[e], this.sa[f] = this.sa[f], this.sa[e]
			e++
			f++
		}

		first += (b - a)
		last -= (d - c)
	}

	return first, last
}

func (this *DivSufSort) trIntroSort(isa, isad, first, last int, budget *TRBudget) {
	incr := isad - isa
	arr := this.sa
	limit := trIlg(last - first)
	trlink := -1

	for {
		if limit < 0 {
			if limit == -1 {
				// tandem repeat partition
				a, b := this.trPartition(isad-incr, first, first, last, last-1)

				// update ranks
				if a < last {
					for c, v := first, a-1; c < a; c++ {
						arr[isa+arr[c]] = v
					}
				}

				if b < last {
					for c, v := a, b-1; c < b; c++ {
						arr[isa+arr[c]] = v
					}
				}

				// push
				if b-a > 1 {
					this.trStack.push(0, a, b, 0, 0)
					this.trStack.push(isad-incr, first, last, -2, trlink)
					trlink = this.trStack.size() - 2
				}

				if a-first <= last-b {
					if a-first > 1 {
						this.trStack.push(isad, b, last, trIlg(last-b), trlink)
						last = a
						limit = trIlg(a - first)
					} else if last-b > 1 {
						first = b
						limit = trIlg(last - b)
					} else {
						se := this.trStack.pop()

						if se == nil {
							return
						}

						isad = se.a
						first = se.b
						last = se.c
						limit = se.d
						trlink = se.e
					}
				} else {
					if last-b > 1 {
						this.trStack.push(isad, first, a, trIlg(a-first), trlink)
						first = b
						limit = trIlg(last - b)
					} else if a-first > 1 {
						last = a
						limit = trIlg(a - first)
					} else {
						se := this.trStack.pop()

						if se == nil {
							return
						}

						isad = se.a
						first = se.b
						last = se.c
						limit = se.d
						trlink = se.e
					}
				}
			} else if limit == -2 {
				// tandem repeat copy
				se := this.trStack.pop()

				if se.d == 0 {
					this.trCopy(isa, first, se.b, se.c, last, isad-isa)
				} else {
					if trlink >= 0 {
						this.trStack.get(trlink).d = -1
					}

					this.trPartialCopy(isa, first, se.b, se.c, last, isad-isa)
				}

				if se = this.trStack.pop(); se == nil {
					return
				}

				isad = se.a
				first = se.b
				last = se.c
				limit = se.d
				trlink = se.e
			} else {
				// sorted partition
				if arr[first] >= 0 {
					a := first

					for {
						arr[isa+arr[a]] = a
						a++

						if a >= last || arr[a] < 0 {
							break
						}
					}

					first = a
				}

				if first < last {
					a := first

					for {
						arr[a] = ^arr[a]
						a++

						if arr[a] >= 0 {
							break
						}
					}

					next := -1

					if arr[isa+arr[a]] != arr[isad+arr[a]] {
						next = trIlg(a - first + 1)
					}

					a++

					if a < last {
						v := a - 1

						for b := first; b < a; b++ {
							arr[isa+arr[b]] = v
						}
					}

					// push
					if budget.check(a-first) == true {
						if a-first <= last-a {
							this.trStack.push(isad, a, last, -3, trlink)
							isad += incr
							last = a
							limit = next
						} else {
							if last-a > 1 {
								this.trStack.push(isad+incr, first, a, next, trlink)
								first = a
								limit = -3
							} else {
								isad += incr
								last = a
								limit = next
							}
						}
					} else {
						if trlink >= 0 {
							this.trStack.get(trlink).d = -1
						}

						if last-a > 1 {
							first = a
							limit = -3
						} else {
							se := this.trStack.pop()

							if se == nil {
								return
							}

							isad = se.a
							first = se.b
							last = se.c
							limit = se.d
							trlink = se.e
						}
					}
				} else {
					se := this.trStack.pop()

					if se == nil {
						return
					}

					isad = se.a
					first = se.b
					last = se.c
					limit = se.d
					trlink = se.e
				}
			}

			continue
		}

		if last-first <= TR_INSERTIONSORT_THRESHOLD {
			this.trInsertionSort(isad, first, last)
			limit = -3
			continue
		}

		if limit == 0 {
			this.trHeapSort(isad, first, last-first)
			a := last - 1

			for first < a {
				b := a - 1
				x := arr[isad+arr[a]]

				for first <= b && arr[isad+arr[b]] == x {
					arr[b] = ^arr[b]
					b--
				}

				a = b
			}

			limit = -3
			continue
		}

		limit--

		// choose pivot
		pvt := trPivot(this.sa, isad, first, last)
		this.sa[first], this.sa[pvt] = this.sa[pvt], this.sa[first]

		v := arr[isad+arr[first]]

		// partition
		a, b := this.trPartition(isad, first, first+1, last, v)

		if last-first != b-a {
			next := -1

			if arr[isa+arr[a]] != v {
				next = trIlg(b - a)
			}

			v = a - 1

			// update ranks
			for c := first; c < a; c++ {
				arr[isa+arr[c]] = v
			}

			if b < last {
				v = b - 1

				for c := a; c < b; c++ {
					arr[isa+arr[c]] = v
				}
			}

			// push
			if b-a > 1 && budget.check(b-a) == true {
				if a-first <= last-b {
					if last-b <= b-a {
						if a-first > 1 {
							this.trStack.push(isad+incr, a, b, next, trlink)
							this.trStack.push(isad, b, last, limit, trlink)
							last = a
						} else if last-b > 1 {
							this.trStack.push(isad+incr, a, b, next, trlink)
							first = b
						} else {
							isad += incr
							first = a
							last = b
							limit = next
						}
					} else if a-first <= b-a {
						if a-first > 1 {
							this.trStack.push(isad, b, last, limit, trlink)
							this.trStack.push(isad+incr, a, b, next, trlink)
							last = a
						} else {
							this.trStack.push(isad, b, last, limit, trlink)
							isad += incr
							first = a
							last = b
							limit = next
						}
					} else {
						this.trStack.push(isad, b, last, limit, trlink)
						this.trStack.push(isad, first, a, limit, trlink)
						isad += incr
						first = a
						last = b
						limit = next
					}
				} else {
					if a-first <= b-a {
						if last-b > 1 {
							this.trStack.push(isad+incr, a, b, next, trlink)
							this.trStack.push(isad, first, a, limit, trlink)
							first = b
						} else if a-first > 1 {
							this.trStack.push(isad+incr, a, b, next, trlink)
							last = a
						} else {
							isad += incr
							first = a
							last = b
							limit = next
						}
					} else if last-b <= b-a {
						if last-b > 1 {
							this.trStack.push(isad, first, a, limit, trlink)
							this.trStack.push(isad+incr, a, b, next, trlink)
							first = b
						} else {
							this.trStack.push(isad, first, a, limit, trlink)
							isad += incr
							first = a
							last = b
							limit = next
						}
					} else {
						this.trStack.push(isad, first, a, limit, trlink)
						this.trStack.push(isad, b, last, limit, trlink)
						isad += incr
						first = a
						last = b
						limit = next
					}
				}
			} else {
				if b-a > 1 && trlink >= 0 {
					this.trStack.get(trlink).d = -1
				}

				if a-first <= last-b {
					if a-first > 1 {
						this.trStack.push(isad, b, last, limit, trlink)
						last = a
					} else if last-b > 1 {
						first = b
					} else {
						se := this.trStack.pop()

						if se == nil {
							return
						}

						isad = se.a
						first = se.b
						last = se.c
						limit = se.d
						trlink = se.e
					}
				} else {
					if last-b > 1 {
						this.trStack.push(isad, first, a, limit, trlink)
						first = b
					} else if a-first > 1 {
						last = a
					} else {
						se := this.trStack.pop()

						if se == nil {
							return
						}

						isad = se.a
						first = se.b
						last = se.c
						limit = se.d
						trlink = se.e
					}
				}
			}
		} else {
			if budget.check(last-first) == true {
				limit = trIlg(last - first)
				isad += incr
			} else {
				if trlink >= 0 {
					this.trStack.get(trlink).d = -1
				}

				se := this.trStack.pop()

				if se == nil {
					return
				}

				isad = se.a
				first = se.b
				last = se.c
				limit = se.d
				trlink = se.e
			}
		}
	}
}

func trPivot(buf1 []int, isad, first, last int) int {
	t := last - first
	middle := first + (t >> 1)
	buf2 := buf1[isad:]

	if t <= 512 {
		if t <= 32 {
			return trMedian3(buf1, buf2, first, middle, last-1)
		}

		t >>= 2
		return trMedian5(buf1, buf2, first, first+t, middle, last-1-t, last-1)
	}

	t >>= 3
	first = trMedian3(buf1, buf2, first, first+t, first+(t<<1))
	middle = trMedian3(buf1, buf2, middle-t, middle, middle+t)
	last = trMedian3(buf1, buf2, last-1-(t<<1), last-1-t, last-1)
	return trMedian3(buf1, buf2, first, middle, last)
}

func trMedian5(buf1, buf2 []int, v1, v2, v3, v4, v5 int) int {
	if buf2[buf1[v2]] > buf2[buf1[v3]] {
		v2, v3 = v3, v2
	}

	if buf2[buf1[v4]] > buf2[buf1[v5]] {
		v4, v5 = v5, v4
	}

	if buf2[buf1[v2]] > buf2[buf1[v4]] {
		v2, v4 = v4, v2
		v3, v5 = v5, v3
	}

	if buf2[buf1[v1]] > buf2[buf1[v3]] {
		v1, v3 = v3, v1
	}

	if buf2[buf1[v1]] > buf2[buf1[v4]] {
		v1, v4 = v4, v1
		v3, v5 = v5, v3
	}

	if buf2[buf1[v3]] > buf2[buf1[v4]] {
		return v4
	}

	return v3
}

func trMedian3(buf1, buf2 []int, v1, v2, v3 int) int {
	if buf2[buf1[v1]] > buf2[buf1[v2]] {
		v1, v2 = v2, v1
	}

	if buf2[buf1[v2]] > buf2[buf1[v3]] {
		if buf2[buf1[v1]] > buf2[buf1[v3]] {
			return v1
		}

		return v3
	}

	return v2
}

func (this *DivSufSort) trHeapSort(isad, saIdx, size int) {
	arr := this.sa
	m := size

	if size&1 == 0 {
		m--

		if arr[isad+arr[saIdx+(m>>1)]] < arr[isad+arr[saIdx+m]] {
			this.sa[saIdx+(m>>1)], this.sa[saIdx+m] = this.sa[saIdx+m], this.sa[saIdx+(m>>1)]
		}
	}

	buf1 := this.sa[isad:]
	buf2 := this.sa[saIdx:]

	for i := (m >> 1) - 1; i >= 0; i-- {
		this.trFixDown(buf1, buf2, i, m)
	}

	if size&1 == 0 {
		this.sa[saIdx], this.sa[saIdx+m] = this.sa[saIdx+m], this.sa[saIdx]
		this.trFixDown(buf1, buf2, 0, m)
	}

	for i := m - 1; i > 0; i-- {
		t := arr[saIdx]
		arr[saIdx] = arr[saIdx+i]
		this.trFixDown(buf1, buf2, 0, i)
		arr[saIdx+i] = t
	}
}

func (this *DivSufSort) trFixDown(buf1, buf2 []int, i, size int) {
	v := buf2[i]
	c := buf1[v]
	j := (i << 1) + 1

	for j < size {
		k := j
		j++
		d := buf1[buf2[k]]
		e := buf1[buf2[j]]

		if d < e {
			k = j
			d = e
		}

		if d <= c {
			break
		}

		buf2[i] = buf2[k]
		i = k
		j = (i << 1) + 1
	}

	buf2[i] = v
}

func (this *DivSufSort) trInsertionSort(isad, first, last int) {
	buf1 := this.sa
	buf2 := this.sa[isad:]

	for a := first + 1; a < last; a++ {
		b := a - 1
		t := buf1[a]
		r := buf2[t] - buf2[buf1[b]]

		for r < 0 {
			for {
				buf1[b+1] = buf1[b]
				b--

				if b < first || buf1[b] >= 0 {
					break
				}
			}

			if b < first {
				break
			}

			r = buf2[t] - buf2[buf1[b]]
		}

		if r == 0 {
			buf1[b] = ^buf1[b]
		}

		buf1[b+1] = t
	}
}

func (this *DivSufSort) trPartialCopy(isa, first, a, b, last, depth int) {
	buf1 := this.sa
	buf2 := this.sa[isa:]
	v := b - 1
	lastRank := -1
	newRank := -1
	d := a - 1

	for c := first; c <= d; c++ {
		s := buf1[c] - depth

		if s >= 0 && buf2[s] == v {
			d++
			buf1[d] = s
			rank := buf2[s+depth]

			if lastRank != rank {
				lastRank = rank
				newRank = d
			}

			buf2[s] = newRank
		}
	}

	lastRank = -1

	for e := d; first <= e; e-- {
		rank := buf2[buf1[e]]

		if lastRank != rank {
			lastRank = rank
			newRank = e
		}

		if newRank != rank {
			buf2[buf1[e]] = newRank
		}
	}

	lastRank = -1
	e := d + 1
	d = b

	for c := last - 1; d > e; c-- {
		s := buf1[c] - depth

		if s >= 0 && buf2[s] == v {
			d--
			buf1[d] = s
			rank := buf2[s+depth]

			if lastRank != rank {
				lastRank = rank
				newRank = d
			}

			buf2[s] = newRank
		}
	}
}

func (this *DivSufSort) trCopy(isa, first, a, b, last, depth int) {
	buf1 := this.sa
	buf2 := this.sa[isa:]
	v := b - 1
	d := a - 1

	for c := first; c <= d; c++ {
		s := buf1[c] - depth

		if s >= 0 && buf2[s] == v {
			d++
			buf1[d] = s
			buf2[s] = d
		}
	}

	e := d + 1
	d = b

	for c := last - 1; d > e; c-- {
		s := buf1[c] - depth

		if s >= 0 && buf2[s] == v {
			d--
			buf1[d] = s
			buf2[s] = d
		}
	}
}

func trIlg(n int) int {
	if n&MASK_FFFF0000 != 0 {
		if n&MASK_FF000000 != 0 {
			return 24 + LOG_TABLE[(n>>24)&0xFF]
		}

		return 16 + LOG_TABLE[(n>>16)&0xFF]
	}

	if n&MASK_0000FF00 != 0 {
		return 8 + LOG_TABLE[(n>>8)&0xFF]
	}

	return LOG_TABLE[n&0xFF]
}

type StackElement struct {
	a, b, c, d, e int
}

type TRBudget struct {
	chance int
	remain int
	incVal int
	count  int
}

// A stack of pre-allocated elements
type Stack struct {
	array []*StackElement
	index int
}

func newStack(size int) *Stack {
	this := new(Stack)
	this.array = make([]*StackElement, size)

	for i := range this.array {
		this.array[i] = &StackElement{0, 0, 0, 0, 0}
	}

	return this
}

func (this *Stack) get(idx int) *StackElement {
	return this.array[idx]
}

func (this *Stack) size() int {
	return this.index
}

func (this *Stack) push(a, b, c, d, e int) {
	elt := this.array[this.index]
	elt.a = a
	elt.b = b
	elt.c = c
	elt.d = d
	elt.e = e
	this.index++
}

func (this *Stack) pop() *StackElement {
	if this.index == 0 {
		return nil
	}

	this.index--
	return this.array[this.index]
}

func (this *TRBudget) check(size int) bool {
	if size <= this.remain {
		this.remain -= size
		return true
	}

	if this.chance == 0 {
		this.count += size
		return false
	}

	this.remain += (this.incVal - size)
	this.chance--
	return true
}
