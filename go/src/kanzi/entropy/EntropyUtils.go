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

package entropy

import (
	"container/heap"
	"fmt"
	"kanzi"
)

const (
	FULL_ALPHABET          = 0
	PARTIAL_ALPHABET       = 1
	SEVEN_BIT_ALPHABET     = 0
	EIGHT_BIT_ALPHABET     = 1
	DELTA_ENCODED_ALPHABET = 0
	BIT_ENCODED_ALPHABET   = 1
	PRESENT_SYMBOLS_MASK   = 0
	ABSENT_SYMBOLS_MASK    = 1
)

type ErrorComparator struct {
	symbols []byte
	errors  []int
}

func ByIncreasingError(symbols []byte, errors []int) ErrorComparator {
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
	symbol      byte
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
	ranks  []byte
	errors []int
}

func NewEntropyUtils() (*EntropyUtils, error) {
	this := new(EntropyUtils)
	this.ranks = make([]byte, 0)
	this.errors = make([]int, 0)
	return this, nil
}

// alphabet must be sorted in increasing order
// alphabetSize <= 256
func EncodeAlphabet(obs kanzi.OutputBitStream, alphabet []byte) int {
	alphabetSize := len(alphabet)

	if alphabetSize > 256 {
		return -1
	}

	// First, push alphabet encoding mode
	if alphabetSize == 256 {
		// Full alphabet
		obs.WriteBit(FULL_ALPHABET)
		obs.WriteBit(EIGHT_BIT_ALPHABET)
		return 256
	}

	if alphabetSize == 128 {
		flag := true

		for i := byte(0); i < 128; i++ {
			if alphabet[i] != i {
				flag = false
				break
			}
		}

		if flag == true {
			obs.WriteBit(FULL_ALPHABET)
			obs.WriteBit(SEVEN_BIT_ALPHABET)
			return 128
		}
	}

	obs.WriteBit(PARTIAL_ALPHABET)

	diffs := make([]int, 32)
	maxSymbolDiff := 1

	if (alphabetSize != 0) && ((alphabetSize < 32) || (alphabetSize > 224)) {
		obs.WriteBit(DELTA_ENCODED_ALPHABET)

		if alphabetSize >= 224 {
			// Big alphabet, encode all missing symbols
			alphabetSize = 256 - alphabetSize
			obs.WriteBits(uint64(alphabetSize), 5)
			obs.WriteBit(ABSENT_SYMBOLS_MASK)
			symbol := uint8(0)
			previous := uint8(0)

			for i, n := 0, 0; n < alphabetSize; {
				if symbol == alphabet[i] {
					if i < len(alphabet)-1 {
						i++
					}

					symbol++
					continue
				}

				diffs[n] = int(symbol) - int(previous)
				symbol++
				previous = symbol

				if diffs[n] > maxSymbolDiff {
					maxSymbolDiff = diffs[n]
				}

				n++
			}
		} else {
			// Small alphabet, encode all present symbols
			obs.WriteBits(uint64(alphabetSize), 5)
			obs.WriteBit(PRESENT_SYMBOLS_MASK)
			previous := uint8(0)

			for i := 0; i < alphabetSize; i++ {
				diffs[i] = int(alphabet[i]) - int(previous)
				previous = alphabet[i] + 1

				if diffs[i] > maxSymbolDiff {
					maxSymbolDiff = diffs[i]
				}
			}
		}

		// Write log(max(diff)) to bitstream
		log := uint(1)

		for 1<<log <= maxSymbolDiff {
			log++
		}

		obs.WriteBits(uint64(log-1), 3) // delta size

		// Write all symbols with delta encoding
		for i := 0; i < alphabetSize; i++ {
			encodeSize(obs, log, uint64(diffs[i]))
		}
	} else {
		// Regular (or empty) alphabet
		obs.WriteBit(BIT_ENCODED_ALPHABET)
		masks := make([]uint64, 4)

		for i := 0; i < alphabetSize; i++ {
			masks[alphabet[i]>>6] |= (1 << uint(alphabet[i]&63))
		}

		for i := range masks {
			obs.WriteBits(masks[i], 64)
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

func DecodeAlphabet(ibs kanzi.InputBitStream, alphabet []byte) (int, error) {
	// Read encoding mode from bitstream
	aphabetType := ibs.ReadBit()

	if aphabetType == FULL_ALPHABET {
		mode := ibs.ReadBit()
		var alphabetSize int

		if mode == EIGHT_BIT_ALPHABET {
			alphabetSize = 256
		} else {
			alphabetSize = 128
		}

		// Full alphabet
		for i := 0; i < alphabetSize; i++ {
			alphabet[i] = byte(i)
		}

		return alphabetSize, nil
	}

	alphabetSize := 0
	mode := ibs.ReadBit()

	if mode == BIT_ENCODED_ALPHABET {
		// Decode presence flags
		for i := 0; i < 256; i += 64 {
			val := ibs.ReadBits(64)

			for j := 0; j < 64; j++ {
				if val&(uint64(1)<<uint(j)) != 0 {
					alphabet[alphabetSize] = byte(i + j)
					alphabetSize++
				}
			}
		}
	} else { // DELTA_ENCODED_ALPHABET
		val := int(ibs.ReadBits(6))
		log := uint(1 + ibs.ReadBits(3)) // log(max(diff))
		alphabetSize = val >> 1
		n := 0
		symbol := uint8(0)

		if val&1 == ABSENT_SYMBOLS_MASK {
			for i := 0; i < alphabetSize; i++ {
				next := symbol + byte(decodeSize(ibs, log))

				for symbol < next {
					alphabet[n] = symbol
					symbol++
					n++
				}

				symbol++
			}

			alphabetSize = 256 - alphabetSize

			for n < alphabetSize {
				alphabet[n] = symbol
				n++
				symbol++
			}

		} else {
			for i := 0; i < alphabetSize; i++ {
				symbol += uint8(decodeSize(ibs, log))
				alphabet[i] = symbol
				symbol++
			}
		}
	}

	return alphabetSize, nil
}

// Returns the size of the alphabet
// The alphabet and freqs parameters are updated
func (this *EntropyUtils) NormalizeFrequencies(freqs []int, alphabet []byte, count int, scale int) (int, error) {
	if count == 0 {
		return 0, nil
	}

	if scale < 1<<8 || scale > 1<<16 {
		return 0, fmt.Errorf("Invalid range parameter: %v (must be in [256..65536])", scale)
	}

	alphabetSize := 0

	// range == count shortcut
	if count == scale {
		for i := range freqs {
			if freqs[i] != 0 {
				alphabet[alphabetSize] = byte(i)
				alphabetSize++
			}
		}

		return alphabetSize, nil
	}

	if len(this.ranks) < len(alphabet) {
		this.ranks = make([]byte, len(alphabet))
	}

	if len(this.errors) < len(alphabet) {
		this.errors = make([]int, len(alphabet))
	}

	sum := -scale

	// Scale frequencies by stretching distribution over complete range
	for i := range alphabet {
		alphabet[i] = 0
		this.errors[i] = -1
		this.ranks[i] = byte(i)

		if freqs[i] == 0 {
			continue
		}

		sf := int64(freqs[i]) * int64(scale)
		scaledFreq := int(sf / int64(count))

		if scaledFreq == 0 {
			// Quantum of frequency
			scaledFreq = 1
		} else {
			// Find best frequency rounding value
			errCeiling := int64(scaledFreq+1)*int64(count) - sf
			errFloor := sf - int64(scaledFreq)*int64(count)

			if errCeiling < errFloor {
				scaledFreq++
				this.errors[i] = int(errCeiling)
			} else {
				this.errors[i] = int(errFloor)
			}
		}

		alphabet[alphabetSize] = byte(i)
		alphabetSize++
		sum += scaledFreq
		freqs[i] = scaledFreq
	}

	if alphabetSize == 0 {
		return 0, nil
	}

	if alphabetSize == 1 {
		freqs[alphabet[0]] = scale
		return 1, nil
	}

	if sum != 0 {
		// Need to normalize frequency sum to range
		var inc int

		if sum > 0 {
			inc = -1
		} else {
			inc = 1
		}

		queue := make(FreqSortPriorityQueue, 0)

		// Create sorted queue of present symbols (except those with 'quantum frequency')
		for i := 0; i < alphabetSize; i++ {
			if this.errors[alphabet[i]] >= 0 {
				heap.Push(&queue, &FreqSortData{errors: this.errors, frequencies: freqs, symbol: alphabet[i]})
			}
		}

		for sum != 0 && len(queue) > 0 {
			// Remove symbol with highest error
			fsd := heap.Pop(&queue).(*FreqSortData)

			// Do not zero out any frequency
			if freqs[fsd.symbol] == -inc {
				continue
			}

			// Distort frequency and error
			freqs[fsd.symbol] += inc
			this.errors[fsd.symbol] -= scale
			sum += inc
			heap.Push(&queue, fsd)
		}
	}

	return alphabetSize, nil
}
