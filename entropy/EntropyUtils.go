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
	// INCOMPRESSIBLE_THRESHOLD Any block with entropy*1024 greater than this threshold is considered incompressible
	INCOMPRESSIBLE_THRESHOLD = 973

	_FULL_ALPHABET            = 0 // Flag for full alphabet encoding
	_PARTIAL_ALPHABET         = 1 // Flag for partial alphabet encoding
	_ALPHABET_256             = 0 // Flag for alphabet with 256 symbols
	_ALPHABET_NOT_256         = 1 // Flag for alphabet not with 256 symbols
	_DELTA_ENCODED_ALPHABET   = 0 // Flag for full alphabet delta encoding
	_BIT_ENCODED_ALPHABET_256 = 1 // Flag for full alphabet bit encoding
	_PRESENT_SYMBOLS_MASK     = 0 // Flag for present symbol
	_ABSENT_SYMBOLS_MASK      = 1 // Flag for absent symbol
)

type freqSortData struct {
	frequencies []int
	errors      []int
	symbol      int
}

type freqSortPriorityQueue []*freqSortData

func (this freqSortPriorityQueue) Len() int {
	return len(this)
}

func (this freqSortPriorityQueue) Less(i, j int) bool {
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

func (this freqSortPriorityQueue) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

func (this *freqSortPriorityQueue) Push(data interface{}) {
	*this = append(*this, data.(*freqSortData))
}

func (this *freqSortPriorityQueue) Pop() interface{} {
	old := *this
	n := len(old)
	data := old[n-1]
	*this = old[0 : n-1]
	return data
}

// EncodeAlphabet writes the alphabet to the bitstream and return the number
// of symbols written or an error.
// alphabet must be sorted in increasing order
// alphabet size must be a power of 2 up to 256
func EncodeAlphabet(obs kanzi.OutputBitStream, alphabet []int) (int, error) {
	alphabetSize := cap(alphabet)
	count := len(alphabet)

	// Alphabet length must be a power of 2
	if alphabetSize&(alphabetSize-1) != 0 {
		return 0, fmt.Errorf("The alphabet length must be a power of 2, got %v", alphabetSize)
	}

	if alphabetSize > 256 {
		return 0, fmt.Errorf("The max alphabet length is 256, got %v", alphabetSize)
	}

	// First, push alphabet encoding mode
	if alphabetSize > 0 && count == alphabetSize {
		// Full alphabet
		obs.WriteBit(_FULL_ALPHABET)

		if count == 256 {
			obs.WriteBit(_ALPHABET_256) // shortcut
		} else {
			log := uint(1)

			for 1<<log <= count {
				log++
			}

			// Write alphabet size
			obs.WriteBit(_ALPHABET_NOT_256)
			obs.WriteBits(uint64(log-1), 3)
			obs.WriteBits(uint64(count), log)
		}

		return count, nil
	}

	obs.WriteBit(_PARTIAL_ALPHABET)

	if alphabetSize == 256 && count >= 32 && count <= 224 {
		// Regular alphabet of symbols less than 256
		obs.WriteBit(_BIT_ENCODED_ALPHABET_256)
		masks := [4]uint64{}

		for i := 0; i < count; i++ {
			masks[alphabet[i]>>6] |= (1 << uint(alphabet[i]&63))
		}

		obs.WriteBits(masks[0], 64)
		obs.WriteBits(masks[1], 64)
		obs.WriteBits(masks[2], 64)
		obs.WriteBits(masks[3], 64)
		return count, nil
	}

	obs.WriteBit(_DELTA_ENCODED_ALPHABET)
	diffs := make([]int, count)

	if alphabetSize-count < count {
		// Encode all missing symbols
		count = alphabetSize - count
		log := uint(1)

		for 1<<log <= count {
			log++
		}

		// Write length
		obs.WriteBits(uint64(log-1), 3)
		obs.WriteBits(uint64(count), log)

		if count == 0 {
			return 0, nil
		}

		obs.WriteBit(_ABSENT_SYMBOLS_MASK)
		log = 1

		for 1<<log <= alphabetSize {
			log++
		}

		// Write log(alphabet size)
		obs.WriteBits(uint64(log-1), 4)
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
		obs.WriteBits(uint64(log-1), 3)
		obs.WriteBits(uint64(count), log)

		if count == 0 {
			return 0, nil
		}

		obs.WriteBit(_PRESENT_SYMBOLS_MASK)
		previous := 0

		// Create deltas of present symbols
		for i := 0; i < count; i++ {
			diffs[i] = int(alphabet[i]) - previous
			previous = alphabet[i] + 1
		}
	}

	ckSize := (count + 3) >> 2

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

		obs.WriteBits(uint64(log-1), 3)

		// Write deltas for this chunk
		for j := i; j < count && j < i+ckSize; j++ {
			obs.WriteBits(uint64(diffs[j]), log)
		}
	}

	return alphabetSize, nil
}

// DecodeAlphabet reads the alphabet from the bitstream and return the number of symbols
// read or an error
func DecodeAlphabet(ibs kanzi.InputBitStream, alphabet []int) (int, error) {
	// Read encoding mode from bitstream
	alphabetType := ibs.ReadBit()

	if alphabetType == _FULL_ALPHABET {
		var alphabetSize int

		if ibs.ReadBit() == _ALPHABET_256 {
			alphabetSize = 256
		} else {
			log := uint(1 + ibs.ReadBits(3))
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

	if mode == _BIT_ENCODED_ALPHABET_256 {
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
	log := uint(1 + ibs.ReadBits(3))
	count = int(ibs.ReadBits(log))

	if count == 0 {
		return 0, nil
	}

	ckSize := (count + 3) >> 2
	n := 0
	symbol := 0

	if ibs.ReadBit() == _ABSENT_SYMBOLS_MASK {
		alphabetSize := 1 << uint(ibs.ReadBits(4))

		if alphabetSize > len(alphabet) {
			return alphabetSize, fmt.Errorf("Invalid bitstream: incorrect alphabet size: %v", alphabetSize)
		}

		// Read missing symbols
		for i := 0; i < count; i += ckSize {
			log = uint(1 + ibs.ReadBits(3))

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
			log = uint(1 + ibs.ReadBits(3))

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

// ComputeFirstOrderEntropy1024 computes the order 0 entropy of the block
// and scales the result by 1024 (result in the [0..1024] range)
// Fills in the histogram with order 0 frequencies. Incoming array size must be at least 256
func ComputeFirstOrderEntropy1024(block []byte, histo []int) int {
	if len(block) == 0 {
		return 0
	}

	kanzi.ComputeHistogram(block, histo, true, false)
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

// NormalizeFrequencies scales the frequencies so that their sum equals 'scale'.
// Returns the size of the alphabet or an error.
// The alphabet and freqs parameters are updated.
func NormalizeFrequencies(freqs []int, alphabet []int, totalFreq, scale int) (int, error) {
	if len(alphabet) > 256 {
		return 0, fmt.Errorf("Invalid alphabet size parameter: %v (must be less than or equal to 256)", len(alphabet))
	}

	if scale < 256 || scale > 65536 {
		return 0, fmt.Errorf("Invalid range parameter: %v (must be in [256..65536])", scale)
	}

	if len(alphabet) == 0 || totalFreq == 0 {
		return 0, nil
	}

	alphabetSize := 0

	// Shortcut
	if totalFreq == scale {
		for i := 0; i < 256; i++ {
			if freqs[i] != 0 {
				alphabet[alphabetSize] = i
				alphabetSize++
			}
		}

		return alphabetSize, nil
	}

	var errors [256]int
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

			queue := make(freqSortPriorityQueue, 0, alphabetSize)

			// Create sorted queue of present symbols (except those with 'quantum frequency')
			for i := 0; i < alphabetSize; i++ {
				if errors[alphabet[i]] > 0 && freqs[alphabet[i]] != -inc {
					heap.Push(&queue, &freqSortData{errors: errors[:], frequencies: freqs, symbol: alphabet[i]})
				}
			}

			for sumScaledFreq != scale && len(queue) > 0 {
				// Remove symbol with highest error
				fsd := heap.Pop(&queue).(*freqSortData)

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

// WriteVarInt writes the provided value to the bitstream as a VarInt.
// Returns the number of bytes written.
func WriteVarInt(bs kanzi.OutputBitStream, value uint32) int {
	res := 0

	for value >= 128 {
		bs.WriteBits(uint64(0x80|(value&0x7F)), 8)
		value >>= 7
		res++
	}

	bs.WriteBits(uint64(value), 8)
	return res
}

// ReadVarInt reads a VarInt from the bitstream and returns it as an uint32.
func ReadVarInt(bs kanzi.InputBitStream) uint32 {
	value := uint32(bs.ReadBits(8))

	if value < 128 {
		return value
	}

	res := value & 0x7F
	value = uint32(bs.ReadBits(8))
	res |= ((value & 0x7F) << 7)

	if value >= 128 {
		value = uint32(bs.ReadBits(8))
		res |= ((value & 0x7F) << 14)

		if value >= 128 {
			value = uint32(bs.ReadBits(8))
			res |= ((value & 0x7F) << 21)
		}
	}

	return res
}
