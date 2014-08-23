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

func EntropyEncodeArray(ee kanzi.EntropyEncoder, block []byte) (int, error) {
	for i := range block {
		ee.EncodeByte(block[i])
	}

	return len(block), nil
}

func EntropyDecodeArray(ed kanzi.EntropyDecoder, block []byte) (int, error) {
	for i := range block {
		block[i] = ed.DecodeByte()
	}

	return len(block), nil
}

// alphabet must be sorted in increasing order
func EncodeAlphabet(obs kanzi.OutputBitStream, alphabet []uint8) int {
	alphabetSize := len(alphabet)

	// First, push alphabet encoding mode
	if alphabetSize == 256 {
		// Full alphabet
		obs.WriteBit(FULL_ALPHABET)
		obs.WriteBit(EIGHT_BIT_ALPHABET)
		return 256
	}

	if alphabetSize == 128 {
		flag := true

		for i := uint8(0); i < 128; i++ {
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
			obs.WriteBits(uint64(diffs[i]), log)
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

func DecodeAlphabet(ibs kanzi.InputBitStream, alphabet []uint8) (int, error) {
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
			alphabet[i] = uint8(i)
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
					alphabet[alphabetSize] = uint8(i + j)
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
				next := symbol + uint8(ibs.ReadBits(log))

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
				symbol += uint8(ibs.ReadBits(log))
				alphabet[i] = symbol
				symbol++
			}
		}
	}

	return alphabetSize, nil
}
