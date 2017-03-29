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

package function

// Implementation of Mespotine RLE
// See [An overhead-reduced and improved Run-Length-Encoding Method] by Meo Mespotine
// Length is transmitted as 1 to 3 bytes. The run threshold can be provided.
// EG. runThreshold = 2 and RUN_LEN_ENCODE1 = 239 => RUN_LEN_ENCODE2 = 4096
// 2    <= runLen < 239+2      -> 1 byte
// 241  <= runLen < 4096+2     -> 2 bytes
// 4098 <= runLen < 65536+4098 -> 3 bytes

import (
	"errors"
	"fmt"
	"kanzi"
)

const (
	RLT_RUN_LEN_ENCODE1 = 224                                  // used to encode run length
	RLT_RUN_LEN_ENCODE2 = (256 - 1 - RLT_RUN_LEN_ENCODE1) << 8 // used to encode run length
	RLT_MAX_RUN         = 0xFFFF + RLT_RUN_LEN_ENCODE2
)

type RLT struct {
	runThreshold uint
	counters     []int
	flags        []byte
}

func NewRLT(threshold uint) (*RLT, error) {
	if threshold < 2 {
		return nil, errors.New("Invalid run threshold parameter (must be at least 2)")
	}

	this := new(RLT)
	this.runThreshold = threshold
	this.counters = make([]int, 256)
	this.flags = make([]byte, 32)
	return this, nil
}

func (this *RLT) RunTheshold() uint {
	return this.runThreshold
}

func (this *RLT) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	for i := range this.flags {
		this.flags[i] = 0
	}

	for i := range this.counters {
		this.counters[i] = 0
	}

	srcIdx := uint(0)
	dstIdx := uint(0)
	srcEnd := uint(len(src))
	dstEnd := uint(len(dst))
	dstEnd4 := dstEnd - 4
	run := 0
	threshold := int(this.runThreshold)
	maxRun := RLT_MAX_RUN + int(this.runThreshold)
	var err error

	// Initialize with a value different from the first data
	prev := ^src[srcIdx]

	// Step 1: create counters and set compression flags
	for srcIdx < srcEnd {
		val := src[srcIdx]
		srcIdx++

		// Encode up to 0x7FFF repetitions in the 'length' information
		if prev == val && run < RLT_MAX_RUN {
			run++
			continue
		}

		if run >= threshold {
			this.counters[prev] += (run - threshold - 1)
		}

		if prev != val {
			prev = val
			run = 1
		}
	}

	if run >= threshold {
		this.counters[prev] += (run - threshold - 1)
	}

	for i := range this.counters {
		if this.counters[i] > 0 {
			this.flags[i>>3] |= (1 << uint(7-(i&7)))
		}
	}

	// Write flags to output
	for i := range this.flags {
		dst[dstIdx] = this.flags[i]
		dstIdx++
	}

	srcIdx = 0
	prev = ^src[srcIdx]
	run = 0

	// Step 2: output run lengths and literals
	// Note that it is possible to output runs over the threshold (for symbols
	// with an unset compression flag)
	for srcIdx < srcEnd && dstIdx < dstEnd {
		val := src[srcIdx]
		srcIdx++

		// Encode up to 0x7FFF repetitions in the 'length' information
		if prev == val && run < maxRun && this.counters[prev&0xFF] > 0 {
			run++

			if run < threshold {
				dst[dstIdx] = prev
				dstIdx++
			}

			continue
		}

		if run >= threshold {
			run -= threshold

			if dstIdx >= dstEnd4 {
				if run >= RLT_RUN_LEN_ENCODE2 {
					err = errors.New("Not enough space in destination buffer")
					break
				}

				if run >= RLT_RUN_LEN_ENCODE1 && dstIdx > dstEnd4 {
					err = errors.New("Not enough space in destination buffer")
					break
				}
			}

			dst[dstIdx] = prev
			dstIdx++

			// Encode run length
			if run >= RLT_RUN_LEN_ENCODE1 {
				if run < RLT_RUN_LEN_ENCODE2 {
					run -= RLT_RUN_LEN_ENCODE1
					dst[dstIdx] = byte(RLT_RUN_LEN_ENCODE1 + (run >> 8))
					dstIdx++
				} else {
					run -= RLT_RUN_LEN_ENCODE2
					dst[dstIdx] = byte(0xFF)
					dstIdx++
					dst[dstIdx] = byte(run >> 8)
					dstIdx++
				}
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
		run -= threshold

		if dstIdx >= dstEnd4+1 {
			if run >= RLT_RUN_LEN_ENCODE2 {
				err = errors.New("Not enough space in destination buffer")
			} else if run >= RLT_RUN_LEN_ENCODE1 && dstIdx > dstEnd4+1 {
				err = errors.New("Not enough space in destination buffer")
			}
		} else {
			dst[dstIdx] = prev
			dstIdx++

			// Encode run length
			if run >= RLT_RUN_LEN_ENCODE1 {
				if run < RLT_RUN_LEN_ENCODE2 {
					run -= RLT_RUN_LEN_ENCODE1
					dst[dstIdx] = byte(RLT_RUN_LEN_ENCODE1 + (run >> 8))
					dstIdx++
				} else {
					run -= RLT_RUN_LEN_ENCODE2
					dst[dstIdx] = byte(0xFF)
					dstIdx++
					dst[dstIdx] = byte(run >> 8)
					dstIdx++
				}
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

func (this *RLT) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	srcIdx := uint(0)
	dstIdx := uint(0)
	srcEnd := uint(len(src))
	dstEnd := uint(len(dst))
	run := uint(0)
	threshold := this.runThreshold
	maxRun := RLT_MAX_RUN + this.runThreshold
	var err error

	// Read compression flags from input
	for i, j := 0, 0; i < 32; i++ {
		flag := src[srcIdx]
		srcIdx++
		this.flags[i] = flag
		this.counters[j] = int(flag>>7) & 1
		this.counters[j+1] = int(flag>>6) & 1
		this.counters[j+2] = int(flag>>5) & 1
		this.counters[j+3] = int(flag>>4) & 1
		this.counters[j+4] = int(flag>>3) & 1
		this.counters[j+5] = int(flag>>2) & 1
		this.counters[j+6] = int(flag>>1) & 1
		this.counters[j+7] = int(flag) & 1
		j += 8
	}

	// Initialize with a value different from the first data
	prev := ^src[srcIdx]

	for srcIdx < srcEnd && dstIdx < dstEnd {
		val := src[srcIdx]
		srcIdx++

		if prev == val && this.counters[prev&0xFF] > 0 {
			run++

			if run >= threshold {
				// Decode the length
				run = uint(src[srcIdx])
				srcIdx++

				if run == 0xFF {
					run = uint(src[srcIdx])
					srcIdx++
					run = (run << 8) | uint(src[srcIdx])
					srcIdx++
					run += RLT_RUN_LEN_ENCODE2
				} else if run >= RLT_RUN_LEN_ENCODE1 {
					run = ((run - RLT_RUN_LEN_ENCODE1) << 8) | uint(src[srcIdx])
					run += RLT_RUN_LEN_ENCODE1
					srcIdx++
				}

				if dstIdx >= dstEnd+run || run > maxRun {
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

// Required encoding output buffer size unknown => guess
func (this RLT) MaxEncodedLen(srcLen int) int {
	return srcLen + 32
}
