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

package entropy

import (
	"container/heap"
	"fmt"
	"kanzi"
)

const (
	FULL_ALPHABET            = 0
	PARTIAL_ALPHABET         = 1
	ALPHABET_256             = 0
	ALPHABET_NOT_256         = 1
	DELTA_ENCODED_ALPHABET   = 0
	BIT_ENCODED_ALPHABET_256 = 1
	PRESENT_SYMBOLS_MASK     = 0
	ABSENT_SYMBOLS_MASK      = 1
)

type ErrorComparator struct {
	symbols []int
	errors  []int
}

func ByIncreasingError(symbols []int, errors []int) ErrorComparator {
	return ErrorComparator{symbols: symbols, errors: errors}
}

func (this ErrorComparator) Less(i, j int) bool {
	// Check errors (natural order) as first key
	ri := this.symbols[i]
	rj := this.symbols[j]

	if this.errors[ri] != this.errors[rj] {
		return this.errors[ri] < this.errors[rj]
	}

	// Check index (natural order) as second key
	return ri < rj
}

func (this ErrorComparator) Len() int {
	return len(this.symbols)
}

func (this ErrorComparator) Swap(i, j int) {
	this.symbols[i], this.symbols[j] = this.symbols[j], this.symbols[i]
}

type FreqSortData struct {
	frequencies []int
	errors      []int
	symbol      int
}

type FreqSortPriorityQueue []*FreqSortData

func (this FreqSortPriorityQueue) Len() int {
	return len(this)
}

func (this FreqSortPriorityQueue) Less(i, j int) bool {
	di := this[i]
	dj := this[j]

	// Decreasing error
	if di.errors[di.symbol] != dj.errors[dj.symbol] {
		return di.errors[di.symbol] > dj.errors[dj.symbol]
	}

	// Decreasing frequency
	if di.frequencies[di.symbol] != dj.frequencies[dj.symbol] {
		return di.frequencies[di.symbol] > dj.frequencies[dj.symbol]
	}

	// Decreasing symbol
	return dj.symbol < di.symbol
}

func (this FreqSortPriorityQueue) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

func (this *FreqSortPriorityQueue) Push(data interface{}) {
	*this = append(*this, data.(*FreqSortData))
}

func (this *FreqSortPriorityQueue) Pop() interface{} {
	old := *this
	n := len(old)
	data := old[n-1]
	*this = old[0 : n-1]
	return data
}

type EntropyUtils struct {
	errors []int
}

func NewEntropyUtils() (*EntropyUtils, error) {
	this := new(EntropyUtils)
	this.errors = make([]int, 0)
	return this, nil
}

// alphabet must be sorted in increasing order
// alphabet length must be a power of 2
func EncodeAlphabet(obs kanzi.OutputBitStream, alphabet []int) int {
	alphabetSize := cap(alphabet)
	count := len(alphabet)

	// Alphabet length must be a power of 2
	if alphabetSize&(alphabetSize-1) != 0 {
		return -1
	}

	// First, push alphabet encoding mode
	if alphabetSize > 0 && count == alphabetSize {
		// Full alphabet
		obs.WriteBit(FULL_ALPHABET)

		if count == 256 {
			obs.WriteBit(ALPHABET_256) // shortcut
		} else {
			log := uint(1)

			for 1<<log <= count {
				log++
			}

			// Write alphabet size
			obs.WriteBit(ALPHABET_NOT_256)
			obs.WriteBits(uint64(log-1), 5)
			obs.WriteBits(uint64(count), log)
		}

		return count
	}

	obs.WriteBit(PARTIAL_ALPHABET)

	if alphabetSize == 256 && count >= 32 && count <= 224 {
		// Regular alphabet of symbols less than 256
		obs.WriteBit(BIT_ENCODED_ALPHABET_256)
		masks := make([]uint64, 4)

		for i := 0; i < count; i++ {
			masks[alphabet[i]>>6] |= (1 << uint(alphabet[i]&63))
		}

		for i := range masks {
			obs.WriteBits(masks[i], 64)
		}

		return count
	}

	obs.WriteBit(DELTA_ENCODED_ALPHABET)
	diffs := make([]int, count)

	if alphabetSize-count < count {
		// Encode all missing symbols
		count = alphabetSize - count
		log := uint(1)

		for 1<<log <= count {
			log++
		}

		// Write length
		obs.WriteBits(uint64(log-1), 4)
		obs.WriteBits(uint64(count), log)

		if count == 0 {
			return 0
		}

		obs.WriteBit(ABSENT_SYMBOLS_MASK)
		log = 1

		for 1<<log <= alphabetSize {
			log++
		}

		// Write log(alphabet size)
		obs.WriteBits(uint64(log-1), 5)
		symbol := 0
		previous := 0

		// Create deltas of missing symbols
		for i, n := 0, 0; n < count; {
			if symbol == alphabet[i] {
				if i < alphabetSize-1-count {
					i++
				}

				symbol++
				continue
			}

			diffs[n] = symbol - previous
			symbol++
			previous = symbol
			n++
		}
	} else {
		// Encode all present symbols
		log := uint(1)

		for 1<<log <= count {
			log++
		}

		// Write length
		obs.WriteBits(uint64(log-1), 4)
		obs.WriteBits(uint64(count), log)

		if count == 0 {
			return 0
		}

		obs.WriteBit(PRESENT_SYMBOLS_MASK)
		previous := 0

		// Create deltas of present symbols
		for i := 0; i < count; i++ {
			diffs[i] = int(alphabet[i]) - int(previous)
			previous = alphabet[i] + 1
		}
	}

	ckSize := 16

	if count <= 64 {
		ckSize = 8
	}

	// Encode all deltas by chunks
	for i := 0; i < count; i += ckSize {
		max := 0

		// Find log(max(deltas)) for this chunk
		for j := i; j < count && j < i+ckSize; j++ {
			if max < diffs[j] {
				max = diffs[j]
			}
		}

		log := uint(1)

		for 1<<log <= max {
			log++
		}

		obs.WriteBits(uint64(log-1), 4)

		// Write deltas for this chunk
		for j := i; j < count && j < i+ckSize; j++ {
			encodeSize(obs, log, uint64(diffs[j]))
		}
	}

	return alphabetSize
}

func encodeSize(obs kanzi.OutputBitStream, log uint, val uint64) {
	obs.WriteBits(val, log)
}

func decodeSize(ibs kanzi.InputBitStream, log uint) uint64 {
	return ibs.ReadBits(log)
}

func DecodeAlphabet(ibs kanzi.InputBitStream, alphabet []int) (int, error) {
	// Read encoding mode from bitstream
	aphabetType := ibs.ReadBit()

	if aphabetType == FULL_ALPHABET {
		var alphabetSize int

		if ibs.ReadBit() == ALPHABET_256 {
			alphabetSize = 256
		} else {
			log := uint(1 + ibs.ReadBits(5))
			alphabetSize = int(ibs.ReadBits(log))
		}

		// Full alphabet
		for i := 0; i < alphabetSize; i++ {
			alphabet[i] = int(i)
		}

		return alphabetSize, nil
	}

	count := 0
	mode := ibs.ReadBit()

	if mode == BIT_ENCODED_ALPHABET_256 {
		// Decode presence flags
		for i := 0; i < 256; i += 64 {
			val := ibs.ReadBits(64)

			for j := 0; j < 64; j++ {
				if val&(uint64(1)<<uint(j)) != 0 {
					alphabet[count] = int(i + j)
					count++
				}
			}
		}

		return count, nil
	}
	// DELTA_ENCODED_ALPHABET
	log := uint(1 + ibs.ReadBits(4))
	count = int(ibs.ReadBits(log))

	if count == 0 {
		return 0, nil
	}

	ckSize := 16

	if count <= 64 {
		ckSize = 8
	}

	n := 0
	symbol := 0

	if ibs.ReadBit() == ABSENT_SYMBOLS_MASK {
		alphabetSize := 1 << uint(ibs.ReadBits(5))

		// Read missing symbols
		for i := 0; i < count; i += ckSize {
			log = uint(1 + ibs.ReadBits(4))

			// Read deltas for this chunk
			for j := i; j < count && j < i+ckSize; j++ {
				next := symbol + int(decodeSize(ibs, log))

				for symbol < next && n < alphabetSize {
					alphabet[n] = symbol
					symbol++
					n++
				}

				symbol++
			}
		}

		count = alphabetSize - count

		for n < count {
			alphabet[n] = symbol
			n++
			symbol++
		}

	} else {
		// Read present symbols
		for i := 0; i < count; i += ckSize {
			log = uint(1 + ibs.ReadBits(4))

			// Read deltas for this chunk
			for j := i; j < count && j < i+ckSize; j++ {
				symbol += int(decodeSize(ibs, log))
				alphabet[j] = symbol
				symbol++
			}
		}
	}

	return count, nil
}

// Returns the size of the alphabet
// The alphabet and freqs parameters are updated
func (this *EntropyUtils) NormalizeFrequencies(freqs []int, alphabet []int, totalFreq, scale int) (int, error) {
	if len(alphabet) > 1<<16 {
		return 0, fmt.Errorf("Invalid alphabet size parameter: %v (must be less than 65536)", len(alphabet))
	}

	if scale < 1<<8 || scale > 1<<16 {
		return 0, fmt.Errorf("Invalid range parameter: %v (must be in [256..65536])", scale)
	}

	if len(alphabet) == 0 || totalFreq == 0 {
		return 0, nil
	}

	alphabetSize := 0

	// shortcut
	if totalFreq == scale {
		for i := range freqs {
			if freqs[i] != 0 {
				alphabet[alphabetSize] = int(i)
				alphabetSize++
			}
		}

		return alphabetSize, nil
	}

	if len(this.errors) < len(alphabet) {
		this.errors = make([]int, len(alphabet))
	}

	sumScaledFreq := 0
	freqMax := 0
	idxMax := -1

	// Scale frequencies by stretching distribution over complete range
	for i := range alphabet {
		alphabet[i] = 0
		this.errors[i] = 0
		f := freqs[i]

		if f == 0 {
			continue
		}

		if f > freqMax {
			freqMax = f
			idxMax = i
		}

		sf := int64(freqs[i]) * int64(scale)
		var scaledFreq int

		if sf <= int64(totalFreq) {
			// Quantum of frequency
			scaledFreq = 1
		} else {
			// Find best frequency rounding value
			scaledFreq = int(sf / int64(totalFreq))
			errCeiling := int64(scaledFreq+1)*int64(totalFreq) - sf
			errFloor := sf - int64(scaledFreq)*int64(totalFreq)

			if errCeiling < errFloor {
				scaledFreq++
				this.errors[i] = int(errCeiling)
			} else {
				this.errors[i] = int(errFloor)
			}
		}

		alphabet[alphabetSize] = i
		alphabetSize++
		sumScaledFreq += scaledFreq
		freqs[i] = scaledFreq
	}

	if alphabetSize == 0 {
		return 0, nil
	}

	if alphabetSize == 1 {
		freqs[alphabet[0]] = scale
		return 1, nil
	}

	if sumScaledFreq != scale {
		if freqs[idxMax] > sumScaledFreq-scale {
			// Fast path: just adjust the max frequency
			freqs[idxMax] += (scale - sumScaledFreq)
		} else {
			// Slow path: spread error across frequencies
			var inc int

			if sumScaledFreq > scale {
				inc = -1
			} else {
				inc = 1
			}

			queue := make(FreqSortPriorityQueue, 0)

			// Create sorted queue of present symbols (except those with 'quantum frequency')
			for i := 0; i < alphabetSize; i++ {
				if this.errors[alphabet[i]] > 0 && freqs[alphabet[i]] != -inc {
					heap.Push(&queue, &FreqSortData{errors: this.errors, frequencies: freqs, symbol: alphabet[i]})
				}
			}

			for sumScaledFreq != scale && len(queue) > 0 {
				// Remove symbol with highest error
				fsd := heap.Pop(&queue).(*FreqSortData)

				// Do not zero out any frequency
				if freqs[fsd.symbol] == -inc {
					continue
				}

				// Distort frequency and error
				freqs[fsd.symbol] += inc
				this.errors[fsd.symbol] -= scale
				sumScaledFreq += inc
				heap.Push(&queue, fsd)
			}
		}
	}

	return alphabetSize, nil
}
