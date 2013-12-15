/*
Copyright 2011-2013 Frederic Langlet
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

package function

import (
	"errors"
	"kanzi"
)

// Distance coder / decoder
// Can be used to replace the Move-To-Front + Run Length Encoding post BWT
//
// The algorithm is explained in the following example
// Example: input = "caracaras", BTW(input) = "rccrsaaaa"
//
// The symbols to be processed will be displayed with upper case
// The ones already processed will be displayed with lower case
// The symbol being processed will be displayed within brackets
//   alphabet | BWT data  | EOF
//   acrs     | rccrsaaaa | # (13 bytes + EOF)
//
// Forward:
// For each symbol, find the next ocurrence (not in a run) and store the distance
// (ignoring already processed symbols).
// STEP 1 : Processing a:  [a]crs RCCRS[a]AAA    #  d=6 (number of upper case symbols + 1)
// STEP 2 : Processing c:  a[c]rs R[c]CRSaAAA    #  d=2
// STEP 3 : Processing r:  ac[r]s [r]cCRSaAAA    #  d=1
// STEP 4 : Processing s:  acr[s] rcCR[s]aAAA    #  d=3
// STEP 5 : Processing r:  acrs   [r]cC[r]saAAA  #  d=2
// STEP 6 : Processing c:  acrs   r[c][c]rsaAAA  #  skip (it is a run)
// STEP 7 : Processing c:  acrs   rc[c]rsaAAA   [#] d=0 (no more 'c')
// STEP 8 : Processing r:  acrs   rcc[r]saAAA   [#] d=0 (no more 'r')
// STEP 9 : Processing s:  acrs   rccr[s]aAAA   [#] d=0 (no more 's')
// STEP 10: Processing a:  acrs   rccrs[a][a]AA  #  skip (it is a run)
// STEP 11: Processing a:  acrs   rccrsa[a][a]A  #  skip (it is a run)
// STEP 12: Processing a:  acrs   rccrsaa[a][a]  #  skip (it is a run)
//
// Result: DC(BTW(input)) = 62132000
//
// Inverse:
//
// STEP 1 : Processing a, d=6 => [a]crs ?????[a]???  (count unprocessed symbols)
// STEP 2 : Processing c, d=2 => a[c]rs ?[c]???a???
// STEP 3 : Processing r, d=1 => ac[r]s [r]c???a???
// STEP 4 : Processing s, d=3 => acr[s] rc??[s]a???
// STEP 5 : Processing r, d=2 => acrs   [r]c?[r]sa???
// STEP 6 : Processing c, d=0 => acrs   r[c]?rsa???   (unchanged, no more 'c')
// STEP 7 : Processing ?,     => acrs   rc[?]rsa???   unknown symbol, repeat previous symbol)
// STEP 8 : Processing r, d=0 => acrs   rcc[r]sa???   (unchanged, no more 'r')
// STEP 9 : Processing s, d=0 => acrs   rccr[s]a???   (unchanged, no more 's')
// STEP 10: Processing ?,     => acrs   rccrsa[?]??   unknown byte, repeat previous byte)
// STEP 11: Processing ?,     => acrs   rccrsaa[?]?   unknown byte, repeat previous byte)
// STEP 12: Processing ?,     => acrs   rccrsaaa[?]   unknown byte, repeat previous byte)
//
// Result: invDC(DC(BTW(input))) = "rccrsaaaa" = BWT(input)

const (
	DEFAULT_DISTANCE_THRESHOLD = 0x80
)

type DistanceCodec struct {
	size             uint
	data             []byte
	buffer           []int
	logDistThreshold uint
}

func NewDistanceCodec(size uint, distanceThreshold int) (*DistanceCodec, error) {
	if distanceThreshold < 4 {
		return nil, errors.New("The distance threshold cannot be less than 4")
	}

	if distanceThreshold > 0x80 {
		return nil, errors.New("The distance threshold cannot more than 128")
	}

	if distanceThreshold&(distanceThreshold-1) != 0 {
		return nil, errors.New("The distance threshold must be a multiple of 2")
	}

	this := new(DistanceCodec)
	this.size = size
	this.buffer = make([]int, 256)
	this.data = make([]byte, 0)
	log2 := uint(0)
	distanceThreshold++

	for distanceThreshold > 1 {
		distanceThreshold >>= 1
		log2++
	}

	this.logDistThreshold = log2
	return this, nil
}

// Determine the alphabet and encode it in the destination array
// Encode the distance for each symbol in the alphabet (of bytes)
// The header is either:
// 0 + 256 * encoded distance for each character
// or (if aplhabet size >= 32)
// alphabet size (byte) + 32 bytes (bit encoded presence of symbol) +
// n (<256) * encoded distance for each symbol
// else
// alphabet size (byte) + m (<32) alphabet symbols +
// n (<256) * encoded distance for each symbol
// The distance is encoded as 1 byte if less than 'distance threshold' or
// else several bytes (with a mask to indicate continuation)
// Return success or failure
func (this *DistanceCodec) encodeHeader(src, dst, significanceFlags []byte) (uint, uint, bool) {
	positions := this.buffer // aliasing
	inLength := this.size

	if inLength == 0 {
		inLength = uint(len(src))
	}

	eof := int(inLength + 1)

	// Set all the positions to 'unknown'
	for i := 0; i < len(positions); i++ {
		positions[i] = eof
	}

	srcIdx := uint(0)
	dstIdx := uint(0)
	current := byte(^src[srcIdx])
	alphabetSize := 0

	// Record the position of the first occurence of each symbol
	for alphabetSize < 256 && srcIdx < inLength {
		// Skip run
		for srcIdx < inLength && src[srcIdx] == current {
			srcIdx++
		}

		// Fill distances array by finding first occurence of each symbol
		if srcIdx < inLength {
			current = src[srcIdx]
			idx := current & 0xFF

			if positions[idx] == eof {
				// distance = alphabet size + index
				positions[idx] = int(srcIdx)
				alphabetSize++
			}

			srcIdx++
		}
	}

	// Check if alphabet is complete (256 symbols), if so encode 0
	if alphabetSize == 256 {
		dst[dstIdx] = 0
		dstIdx++
	} else {
		// Encode size, then encode the alphabet
		dst[dstIdx] = byte(alphabetSize)
		dstIdx++

		if alphabetSize >= 32 {
			// Big alphabet, encode symbol presence bit by bit
			for i := 0; i < 256; i += 8 {
				val := byte(0)
				mask := byte(1)

				for j := 0; j < 8; j++ {
					if positions[i+j] != eof {
						val |= mask
					}

					mask <<= 1
				}

				dst[dstIdx] = val
				dstIdx++
			}
		} else { // small alphabet, spell each symbol
			previous := 0

			for i := 0; i < 256; i++ {
				if positions[i] != eof {
					// Encode symbol as delta
					dst[dstIdx] = byte(i - previous)
					dstIdx++
					previous = i
				}
			}
		}
	}

	distThreshold := 1 << this.logDistThreshold
	distMask := distThreshold - 1

	// For each symbol in the alphabet, encode distance
	for i := 0; i < 256; i++ {
		position := positions[i]

		if position != eof {
			distance := 1

			// Calculate distance
			for j := 0; j < position; j++ {
				distance += int(significanceFlags[j])
			}

			// Mark symbol as already used
			significanceFlags[position] = 0

			// Encode distance over one or several bytes with mask distThreshold
			// to indicate a continuation
			for distance >= distThreshold {
				dst[dstIdx] = byte(distThreshold | (distance & distMask))
				dstIdx++
				distance >>= this.logDistThreshold
			}

			dst[dstIdx] = byte(distance)
			dstIdx++
		}
	}

	return srcIdx, dstIdx, true
}

func (this *DistanceCodec) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	inLength := this.size

	if inLength == 0 {
		inLength = uint(len(src))
	}

	if len(this.data) < int(inLength) {
		this.data = make([]byte, inLength)
	}

	unprocessedFlags := this.data // aliasing

	for i := 0; i < len(unprocessedFlags); i++ {
		unprocessedFlags[i] = 1
	}

	// Encode header
	srcIdx, dstIdx, res := this.encodeHeader(src, dst, unprocessedFlags)

	if res == false {
		return 0, 0, errors.New("Failed to encode header")
	}

	// Encode body (corresponding to input data only)
	distThreshold := 1 << this.logDistThreshold
	distMask := distThreshold - 1

	for srcIdx < inLength {
		first := src[srcIdx]
		current := first
		distance := 1

		// Skip initial run
		for srcIdx < inLength && src[srcIdx] == current {
			srcIdx++
		}

		// Save index of next (different) symbol
		nextIdx := srcIdx

		for srcIdx < inLength {
			current = src[srcIdx]

			// Next occurence of first symbol found => exit
			if current == first {
				break
			}

			// Already processed symbols are ignored (flag = 0)
			distance += int(unprocessedFlags[srcIdx])
			srcIdx++
		}

		if srcIdx == inLength {
			// The symbol has not been found, encode 0
			if current != first {
				dst[dstIdx] = 0
				dstIdx++
			}
		} else {
			// Mark symbol as already used (first of run only)
			unprocessedFlags[srcIdx] = 0

			// Encode distance over one or several bytes with mask distThreshold
			// to indicate a continuation
			for distance >= distThreshold {
				dst[dstIdx] = byte(distThreshold | (distance & distMask))
				dstIdx++
				distance >>= this.logDistThreshold
			}

			dst[dstIdx] = byte(distance)
			dstIdx++
		}

		// Move to next symbol
		srcIdx = nextIdx
	}

	return srcIdx, dstIdx, nil
}

func (this *DistanceCodec) decodeHeader(src, dst, unprocessedFlags []byte) (uint, uint, bool) {
	alphabet := this.buffer // aliasing
	srcIdx := uint(0)
	alphabetSize := int(src[srcIdx])
	srcIdx++

	if alphabetSize != 0 {
		// Reset list of present symbols
		for i := 0; i < len(alphabet); i++ {
			alphabet[i] = 0
		}

		if alphabetSize >= 32 {
			// Big alphabet, decode symbol presence mask
			for i := 0; i < 256; i += 8 {
				val := src[srcIdx]
				srcIdx++
				mask := byte(1)

				for j := 0; j < 8; j++ {
					alphabet[i+j] = int(val & mask)
					mask <<= 1
				}
			}
		} else { // small alphabet, list all present symbols
			previous := 0

			for i := 0; i < alphabetSize; i++ {
				delta := int(src[srcIdx])
				srcIdx++
				previous += delta
				alphabet[previous] = 1
			}
		}
	}

	distThreshold := 1 << this.logDistThreshold
	distMask := distThreshold - 1

	// Process alphabet (find first occurence of each symbol)
	for i := 0; i < 256; i++ {
		if alphabetSize == 0 || alphabet[i] != 0 {
			val := int(src[srcIdx])
			srcIdx++
			distance := 0
			shift := uint(0)

			// Decode distance
			for val >= distThreshold {
				distance |= ((val & distMask) << shift)
				shift += this.logDistThreshold
				val = int(src[srcIdx])
				srcIdx++
			}

			// Distance cannot be 0 since the symbol is present in the alphabet
			distance |= int(val << shift)
			idx := 0

			for distance > 0 {
				distance -= int(unprocessedFlags[idx])
				idx++
			}

			// Output next occurence
			dst[idx] = byte(i)
			unprocessedFlags[idx] = 0
		}
	}

	return srcIdx, uint(0), true
}

func (this *DistanceCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	end := this.size

	if end == 0 {
		end = uint(len(src))
	}

	if len(this.data) < len(dst) {
		this.data = make([]byte, len(dst))
	}

	unprocessedFlags := this.data // aliasing

	for i := 0; i < len(unprocessedFlags); i++ {
		unprocessedFlags[i] = 1
	}

	// Decode header
	srcIdx, dstIdx, res := this.decodeHeader(src, dst, unprocessedFlags)

	if res == false {
		return 0, 0, errors.New("Failed to decode header")
	}

	current := dst[dstIdx]
	distThreshold := 1 << this.logDistThreshold
	distMask := distThreshold - 1

	// Decode body
	for true {
		// If the current symbol is unknown, duplicate previous
		if unprocessedFlags[dstIdx] != 0 {
			dst[dstIdx] = current
			dstIdx++
			continue
		}

		// Get current symbol
		current = dst[dstIdx]
		dstIdx++

		if srcIdx >= end {
			break
		}

		// For the current symbol, get distance to the next occurence
		distance := int(src[srcIdx])
		srcIdx++

		// Last occurence of current symbol
		if distance == 0 {
			continue
		}

		if distance >= distThreshold {
			val := distance
			distance = 0
			shift := uint(0)

			// Decode distance
			for val >= distThreshold {
				distance |= int((val & distMask) << shift)
				shift += this.logDistThreshold
				val = int(src[srcIdx])
				srcIdx++
			}

			distance |= int(val << shift)
		}

		// Skip run
		for unprocessedFlags[dstIdx] != 0 {
			dst[dstIdx] = current
			dstIdx++
		}

		idx := dstIdx

		// Compute index
		for distance > 0 {
			idx++
			distance -= int(unprocessedFlags[idx])
		}

		// Output next occurence
		dst[idx] = current
		unprocessedFlags[idx] = 0
	}

	// Repeat last symbol if needed
	for dstIdx < uint(len(dst)) {
		dst[dstIdx] = current
		dstIdx++
	}

	return uint(srcIdx), uint(dstIdx), nil
}

func (this *DistanceCodec) SetSize(sz uint) bool {
	this.size = sz
	return true
}

// Not thread safe
func (this *DistanceCodec) Size() uint {
	return this.size
}

// Required encoding output buffer size unknown
func  (this DistanceCodec) MaxEncodedLen(srcLen int) int {
	return -1
}
