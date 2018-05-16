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

import (
	"errors"
	"fmt"
	kanzi "github.com/flanglet/kanzi-go"
)

// Zero Length Encoding is a simple encoding algorithm by Wheeler
// closely related to Run Length Encoding. The main difference is
// that only runs of 0 values are processed. Also, the length is
// encoded in a different way (each digit in a different byte)
// This algorithm is well adapted to process post BWT/MTFT data

const (
	ZRLT_MAX_RUN = 0x7FFFFFFF
)

type ZRLT struct {
}

func NewZRLT() (*ZRLT, error) {
	this := new(ZRLT)
	return this, nil
}

func (this *ZRLT) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	srcEnd, dstEnd := uint(len(src)), uint(len(dst))
	dstEnd2 := dstEnd - 2
	runLength := uint32(1)
	srcIdx, dstIdx := uint(0), uint(0)
	var err error

	if dstIdx < dstEnd {
		for srcIdx < srcEnd {
			if src[srcIdx] == 0 {
				runLength++
				srcIdx++

				if srcIdx < srcEnd && runLength < ZRLT_MAX_RUN {
					continue
				}
			}

			if runLength > 1 {
				// Encode length
				log2, _ := kanzi.Log2(runLength)

				if dstIdx >= dstEnd-uint(log2) {
					break
				}

				// Write every bit as a byte except the most significant one
				for log2 > 0 {
					log2--
					dst[dstIdx] = byte((runLength >> log2) & 1)
					dstIdx++
				}

				runLength = 1
				continue
			}

			if src[srcIdx] >= 0xFE {
				if dstIdx >= dstEnd2 {
					break
				}

				dst[dstIdx] = 0xFF
				dstIdx++
				dst[dstIdx] = src[srcIdx] - 0xFE
			} else {
				if dstIdx >= dstEnd {
					break
				}

				dst[dstIdx] = src[srcIdx] + 1
			}

			srcIdx++
			dstIdx++

			if dstIdx >= dstEnd {
				break
			}
		}
	}

	if srcIdx != srcEnd || runLength != 1 {
		err = errors.New("Output buffer is too small")
	}

	return srcIdx, dstIdx, err
}

func (this *ZRLT) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	srcEnd, dstEnd := len(src), len(dst)
	runLength := 1
	srcIdx, dstIdx := 0, 0
	var err error

	if srcIdx < srcEnd {
		for dstIdx < dstEnd {
			if runLength > 1 {
				runLength--
				dst[dstIdx] = 0
				dstIdx++
				continue
			}

			if srcIdx >= srcEnd {
				break
			}

			if src[srcIdx] <= 1 {
				// Generate the run length bit by bit (but force MSB)
				runLength = 1

				for src[srcIdx] <= 1 {
					runLength += (runLength + int(src[srcIdx]))
					srcIdx++

					if srcIdx >= srcEnd {
						break
					}
				}

				continue
			}

			// Regular data processing
			if src[srcIdx] == 0xFF {
				srcIdx++

				if srcIdx >= srcEnd {
					break
				}

				dst[dstIdx] = 0xFE + src[srcIdx]
			} else {
				dst[dstIdx] = src[srcIdx] - 1
			}

			srcIdx++
			dstIdx++
		}
	}

	// If runLength is not 1, add trailing 0s
	end := dstIdx + runLength - 1

	if end > dstEnd {
		err = errors.New("Output buffer is too small")
	} else {
		for dstIdx < end {
			dst[dstIdx] = 0
			dstIdx++
		}

		if srcIdx < srcEnd {
			err = errors.New("Output buffer is too small")
		}
	}

	return uint(srcIdx), uint(dstIdx), err
}

// Required encoding output buffer size unknown
func (this ZRLT) MaxEncodedLen(srcLen int) int {
	return srcLen
}
