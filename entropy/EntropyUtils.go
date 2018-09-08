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
	kanzi "github.com/flanglet/kanzi-go"
)

const (
	INCOMPRESSIBLE_THRESHOLD = 973
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
	buffer []int
}

func NewEntropyUtils() (*EntropyUtils, error) {
	this := new(EntropyUtils)
	this.buffer = make([]int, 0)
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
		masks := [4]uint64{}

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
			diffs[i] = int(alphabet[i]) - previous
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
			// Encode size
			obs.WriteBits(uint64(diffs[j]), log)
		}
	}

	return alphabetSize
}

func DecodeAlphabet(ibs kanzi.InputBitStream, alphabet []int) (int, error) {
	// Read encoding mode from bitstream
	alphabetType := ibs.ReadBit()

	if alphabetType == FULL_ALPHABET {
		var alphabetSize int

		if ibs.ReadBit() == ALPHABET_256 {
			alphabetSize = 256
		} else {
			log := uint(1 + ibs.ReadBits(5))
			alphabetSize = int(ibs.ReadBits(log))
		}

		if alphabetSize > len(alphabet) {
			return alphabetSize, fmt.Errorf("Invalid bitstream: incorrect alphabet size: %v", alphabetSize)
		}

		// Full alphabet
		for i := 0; i < alphabetSize; i++ {
			alphabet[i] = i
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
					alphabet[count] = i + j
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

		if alphabetSize > len(alphabet) {
			return alphabetSize, fmt.Errorf("Invalid bitstream: incorrect alphabet size: %v", alphabetSize)
		}

		// Read missing symbols
		for i := 0; i < count; i += ckSize {
			log = uint(1 + ibs.ReadBits(4))

			// Read deltas for this chunk
			for j := i; j < count && j < i+ckSize; j++ {
				next := symbol + int(ibs.ReadBits(log))

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
				symbol += int(ibs.ReadBits(log))
				alphabet[j] = symbol
				symbol++
			}
		}
	}

	return count, nil
}

// Return the first order entropy in the [0..1024] range
// Fills in the histogram with order 0 frequencies. Incoming array size must be 256
func ComputeFirstOrderEntropy1024(block []byte, histo []int) int {
	if len(block) == 0 {
		return 0
	}

	for i := 0; i < 256; i++ {
		histo[i] = 0
	}

	end8 := len(block) & -8

	for i := 0; i < end8; i += 8 {
		histo[block[i]]++
		histo[block[i+1]]++
		histo[block[i+2]]++
		histo[block[i+3]]++
		histo[block[i+4]]++
		histo[block[i+5]]++
		histo[block[i+6]]++
		histo[block[i+7]]++
	}

	for i := end8; i < len(block); i++ {
		histo[block[i]]++
	}

	sum := uint64(0)
	logLength1024, _ := kanzi.Log2_1024(uint32(len(block)))

	for i := 0; i < 256; i++ {
		if histo[i] == 0 {
			continue
		}

		log1024, _ := kanzi.Log2_1024(uint32(histo[i]))
		sum += ((uint64(histo[i]) * uint64(logLength1024-log1024)) >> 3)
	}

	return int(sum / uint64(len(block)))
}

// Return the size of the alphabet
// The alphabet and freqs parameters are updated
func (this *EntropyUtils) NormalizeFrequencies(freqs []int, alphabet []int, totalFreq, scale int) (int, error) {
	if len(alphabet) > 1<<8 {
		return 0, fmt.Errorf("Invalid alphabet size parameter: %v (must be less than or equal to 256)", len(alphabet))
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
		for i := 0; i < 256; i++ {
			if freqs[i] != 0 {
				alphabet[alphabetSize] = i
				alphabetSize++
			}
		}

		return alphabetSize, nil
	}

	if len(this.buffer) < len(alphabet) {
		this.buffer = make([]int, len(alphabet))
	}

	errors := this.buffer
	sumScaledFreq := 0
	freqMax := 0
	idxMax := -1

	// Scale frequencies by stretching distribution over complete range
	for i := range alphabet {
		alphabet[i] = 0
		errors[i] = 0
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
				errors[i] = int(errCeiling)
			} else {
				errors[i] = int(errFloor)
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

			queue := make(FreqSortPriorityQueue, 0, alphabetSize)

			// Create sorted queue of present symbols (except those with 'quantum frequency')
			for i := 0; i < alphabetSize; i++ {
				if errors[alphabet[i]] > 0 && freqs[alphabet[i]] != -inc {
					heap.Push(&queue, &FreqSortData{errors: errors, frequencies: freqs, symbol: alphabet[i]})
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
				errors[fsd.symbol] -= scale
				sumScaledFreq += inc
				heap.Push(&queue, fsd)
			}
		}
	}

	return alphabetSize, nil
}

func WriteVarInt(bs kanzi.OutputBitStream, value int) int {
	if bs == nil {
		panic("Invalid null bitstream parameter")
	}

	w := 0

	for {
		if value >= 128 {
			bs.WriteBits(uint64(0x80|(value&0x7F)), 8)
		} else {
			bs.WriteBits(uint64(value), 8)
		}

		more := value >= 128
		value >>= 7
		w++

		if more == false || w >= 4 {
			break
		}
	}

	return w
}

func ReadVarInt(bs kanzi.InputBitStream) int {
	if bs == nil {
		panic("Invalid null bitstream parameter")
	}

	res := 0
	shift := uint(0)

	for {
		val := int(bs.ReadBits(8))
		res = ((val & 0x7F) << shift) | res
		more := val >= 128
		shift += 7

		if more == false || shift >= 28 {
			break
		}
	}

	return res
}
