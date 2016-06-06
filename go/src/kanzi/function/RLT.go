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

package function

// Simple implementation of a Run Length Codec
// Length is transmitted as 1 or 2 bytes (minus 1 bit for the mask that indicates
// whether a second byte is used). The run threshold can be provided.
// For a run threshold of 2:
// EG input: 0x10 0x11 0x11 0x17 0x13 0x13 0x13 0x13 0x13 0x13 0x12 (160 times) 0x14
//   output: 0x10 0x11 0x11 0x17 0x13 0x13 0x13 0x05 0x12 0x12 0x80 0xA0 0x14

import (
	"errors"
	"kanzi"
)

const (
	TWO_BYTE_RLE_MASK1    = 0x80
	TWO_BYTE_RLE_MASK2    = 0x7F
	RLT_MAX_RUN           = 0x7FFF
	DEFAULT_RLE_THRESHOLD = 3
)

type RLT struct {
	runThreshold uint
}

func NewRLT(threshold uint) (*RLT, error) {
	if threshold < 2 {
		return nil, errors.New("Invalid run threshold parameter (must be at least 2)")
	}

	if threshold > 256 {
		return nil, errors.New("Invalid run threshold parameter (must be at most 256)")
	}

	this := new(RLT)
	this.runThreshold = threshold
	return this, nil
}

func (this *RLT) RunTheshold() uint {
	return this.runThreshold
}

func (this *RLT) Forward(src, dst []byte, length uint) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	srcEnd := length
	dstEnd := uint(len(dst))
	dstEnd3 := dstEnd - 3
	run := 1
	threshold := int(this.runThreshold)
	maxThreshold := RLT_MAX_RUN + int(this.runThreshold)
	srcIdx := uint(0)
	dstIdx := uint(0)
	var err error

	// Initialize with a value different from the first data
	prev := ^src[srcIdx]

	for srcIdx < srcEnd && dstIdx < dstEnd {
		val := byte(src[srcIdx])
		srcIdx++

		// Encode up to 0x7FFF repetitions in the 'length' information
		if prev == val && run < maxThreshold {
			run++

			if run < threshold {
				dst[dstIdx] = prev
				dstIdx++
			}

			continue
		}

		if run >= threshold {
			if dstIdx >= dstEnd3 {
				err = errors.New("Not enough space in destination buffer")
				break
			}

			dst[dstIdx] = prev
			dstIdx++
			run -= threshold

			// Force MSB to indicate a 2 byte encoding of the length
			if run >= TWO_BYTE_RLE_MASK1 {
				dst[dstIdx] = byte((run >> 8) | TWO_BYTE_RLE_MASK1)
				dstIdx++
			}

			dst[dstIdx] = byte(run)
			dstIdx++
			run = 1
		}

		dst[dstIdx] = val
		dstIdx++

		if prev != val {
			prev = val
			run = 1
		}
	}

	// Fill up the destination array
	if run >= threshold {
		if dstIdx >= dstEnd3 {
			err = errors.New("Not enough space in destination buffer")
		} else {
			dst[dstIdx] = prev
			dstIdx++
			run -= threshold

			// Force MSB to indicate a 2 byte encoding of the length
			if run >= TWO_BYTE_RLE_MASK1 {
				dst[dstIdx] = byte((run >> 8) | TWO_BYTE_RLE_MASK1)
				dstIdx++
			}

			dst[dstIdx] = byte(run)
			dstIdx++
		}
	}

	if srcIdx != srcEnd {
		err = errors.New("Not enough space in destination buffer")
	}

	return srcIdx, dstIdx, err
}

func (this *RLT) Inverse(src, dst []byte, length uint) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	srcEnd := length
	dstEnd := uint(len(dst))
	run := uint(0)
	threshold := this.runThreshold
	srcIdx := uint(0)
	dstIdx := uint(0)
	var err error

	// Initialize with a value different from the first data
	prev := ^src[srcIdx]

	for srcIdx < srcEnd && dstIdx < dstEnd {
		val := src[srcIdx]
		srcIdx++

		if prev == val {
			run++

			if run >= threshold {
				// Read the length
				run = uint(src[srcIdx])
				srcIdx++

				// If the length is encoded in 2 bytes, process next byte
				if run&TWO_BYTE_RLE_MASK1 != 0 {
					run = ((run & TWO_BYTE_RLE_MASK2) << 8) | uint(src[srcIdx])
					srcIdx++
				}

				if dstIdx >= dstEnd+run {
					err = errors.New("Not enough space in destination buffer")
					break
				}
				// Emit length times the previous byte
				for run > 0 {
					dst[dstIdx] = prev
					dstIdx++
					run--
				}
			}
		} else {
			prev = val
			run = 1
		}

		dst[dstIdx] = val
		dstIdx++
	}

	if srcIdx != srcEnd {
		err = errors.New("Not enough space in destination buffer")
	}

	return srcIdx, dstIdx, err
}

// Required encoding output buffer size unknown
func (this RLT) MaxEncodedLen(srcLen int) int {
	return -1
}
