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

package entropy

import (
	"fmt"
	"sort"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

const (
	// INCOMPRESSIBLE_THRESHOLD Any block with entropy*1024 greater than this threshold is considered incompressible
	INCOMPRESSIBLE_THRESHOLD = 973

	_FULL_ALPHABET    = 0 // Flag for full alphabet encoding
	_PARTIAL_ALPHABET = 1 // Flag for partial alphabet encoding
	_ALPHABET_256     = 0 // Flag for alphabet with 256 symbols
	_ALPHABET_0       = 1 // Flag for alphabet not with no symbol
)

type freqSortData struct {
	freq   *int
	symbol int
}

type sortByFreq []*freqSortData

func (this sortByFreq) Len() int {
	return len(this)
}

func (this sortByFreq) Less(i, j int) bool {
	di := this[i]
	dj := this[j]

	// Decreasing frequency then decreasing symbol
	if *dj.freq == *di.freq {
		return dj.symbol < di.symbol
	}

	return *dj.freq < *di.freq
}

func (this sortByFreq) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
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

	if count == 0 {
		obs.WriteBit(_FULL_ALPHABET)
		obs.WriteBit(_ALPHABET_0)
	} else if count == 256 {
		obs.WriteBit(_FULL_ALPHABET)
		obs.WriteBit(_ALPHABET_256)
	} else {
		// Partial alphabet
		obs.WriteBit(_PARTIAL_ALPHABET)
		masks := [32]uint8{}

		for i := 0; i < count; i++ {
			masks[alphabet[i]>>3] |= (1 << uint8(alphabet[i]&7))
		}

		lastMask := alphabet[count-1] >> 3
		obs.WriteBits(uint64(lastMask), 5)

		for i := 0; i <= lastMask; i++ {
			obs.WriteBits(uint64(masks[i]), 8)
		}
	}

	return count, nil
}

// DecodeAlphabet reads the alphabet from the bitstream and return the number of symbols
// read or an error
func DecodeAlphabet(ibs kanzi.InputBitStream, alphabet []int) (int, error) {
	// Read encoding mode from bitstream
	if ibs.ReadBit() == _FULL_ALPHABET {
		if ibs.ReadBit() == _ALPHABET_0 {
			return 0, nil
		}

		alphabetSize := 256

		if alphabetSize > len(alphabet) {
			return alphabetSize, fmt.Errorf("Invalid bitstream: incorrect alphabet size: %v", alphabetSize)
		}

		// Full alphabet
		for i := 0; i < alphabetSize; i++ {
			alphabet[i] = i
		}

		return alphabetSize, nil
	}

	// Partial alphabet
	lastMask := int(ibs.ReadBits(5))
	count := 0

	// Decode presence flags
	for i := 0; i <= lastMask; i++ {
		mask := uint8(ibs.ReadBits(8))

		for j := 0; j < 8; j++ {
			if mask&(uint8(1)<<uint(j)) != 0 {
				alphabet[count] = (i << 3) + j
				count++
			}
		}
	}

	return count, nil
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
		for i, f := range freqs {
			if f != 0 {
				alphabet[alphabetSize] = i
				alphabetSize++
			}
		}

		return alphabetSize, nil
	}

	sumScaledFreq := 0
	idxMax := 0

	// Scale frequencies by squeezing/stretching distribution over complete range
	for i := range alphabet {
		alphabet[i] = 0
		f := freqs[i]

		if f == 0 {
			continue
		}

		if f > freqs[idxMax] {
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
		delta := sumScaledFreq - scale
		var inc, absDelta int

		if sumScaledFreq < scale {
			absDelta = -delta
			inc = 1
		} else {
			absDelta = delta
			inc = -1
		}

		if absDelta*10 < freqs[idxMax] {
			// Fast path (small error): just adjust the max frequency
			freqs[idxMax] -= delta
			return alphabetSize, nil
		}

		// Slow path: spread error across frequencies
		queue := make(sortByFreq, alphabetSize)
		n := 0

		// Create queue of present symbols
		for i := 0; i < alphabetSize; i++ {
			if freqs[alphabet[i]] <= 2 {
				// Do not distort small frequencies
				continue
			}

			queue[n] = &freqSortData{freq: &freqs[alphabet[i]], symbol: alphabet[i]}
			n++
		}

		if n == 0 {
			freqs[idxMax] -= delta
			return alphabetSize, nil
		}

		// Sort queue by decreasing frequency
		queue = queue[0:n]
		sort.Sort(queue)

		for sumScaledFreq != scale {
			// Remove symbol with highest frequency
			fsd := queue[0]
			queue = queue[1:]

			// Do not zero out any frequency
			if *fsd.freq == -inc {
				continue
			}

			// Distort frequency and re-enqueue
			*fsd.freq += inc
			sumScaledFreq += inc
			queue = append(queue, fsd)
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
