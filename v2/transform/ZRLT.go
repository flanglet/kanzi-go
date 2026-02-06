/*
Copyright 2011-2025 Frederic Langlet
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

package transform

import (
	"errors"
	"fmt"

	internal "github.com/flanglet/kanzi-go/v2/internal"
)

// ZRLT Zero Run Length Transform
// Zero Length Encoding is a simple encoding algorithm by Wheeler
// closely related to Run Length Encoding. The main difference is
// that only runs of 0 values are processed. Also, the length is
// encoded in a different way (each digit in a different byte)
// This algorithm is well adapted to process post BWT/MTFT data
// ZRLT encodes zero runs as follows:
//   - Each zero run is encoded as a sequence of bytes representing
//     the run length in binary
//   - The most significant bit is implied (always 1)
//   - Each subsequent bit is stored as a separate byte (0 or 1)
//   - Non-zero values are encoded as value+1, except for values >= 0xFE
//
// which are encoded as 0xFF followed by value-0xFE
type ZRLT struct {
}

// NewZRLT creates a new instance of ZRLT
func NewZRLT() (*ZRLT, error) {
	this := &ZRLT{}
	return this, nil
}

// NewZRLTWithCtx creates a new instance of ZRLT using a
// configuration map as parameter.
func NewZRLTWithCtx(ctx *map[string]any) (*ZRLT, error) {
	this := &ZRLT{}
	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *ZRLT) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	srcEnd := uint(len(src))
	dstEnd := uint(len(src)) // do not expand, hence len(src)
	srcIdx, dstIdx := uint(0), uint(0)
	res := true

	for srcIdx < srcEnd {
		if src[srcIdx] == 0 {
			runStart := srcIdx - 1
			srcIdx++

			for srcIdx+1 < srcEnd && src[srcIdx]|src[srcIdx+1] == 0 {
				srcIdx += 2
			}

			for srcIdx < srcEnd && src[srcIdx] == 0 {
				srcIdx++
			}

			// Encode length
			runLength := srcIdx - runStart
			log2 := internal.Log2NoCheck(uint32(runLength))

			if dstIdx >= dstEnd-uint(log2) {
				res = false
				break
			}

			// Write every bit as a byte except the most significant one
			for log2 > 0 {
				log2--
				dst[dstIdx] = byte((runLength >> log2) & 1)
				dstIdx++
			}

			continue
		}

		if src[srcIdx] >= 0xFE {
			if dstIdx >= dstEnd-1 {
				res = false
				break
			}

			dst[dstIdx] = 0xFF
			dstIdx++
			dst[dstIdx] = src[srcIdx] - 0xFE
		} else {
			if dstIdx >= dstEnd {
				res = false
				break
			}

			dst[dstIdx] = src[srcIdx] + 1
		}

		srcIdx++
		dstIdx++
	}

	var err error

	if srcIdx != srcEnd || res == false {
		err = errors.New("ZRLT forward transform failed: output buffer is too small")
	}

	return srcIdx, dstIdx, err
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *ZRLT) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	srcEnd, dstEnd := uint(len(src)), uint(len(dst))
	srcIdx, dstIdx := uint(0), uint(0)
	runLength := uint(0)
	var err error

	for {
		if src[srcIdx] <= 1 {
			// Generate the run length bit by bit (but force MSB)
			runLength = 1

			for src[srcIdx] <= 1 {
				runLength += (runLength + uint(src[srcIdx]))
				srcIdx++

				if srcIdx >= srcEnd {
					goto End
				}
			}

			runLength--

			if runLength >= dstEnd-dstIdx {
				break
			}

			for runLength > 0 {
				runLength--
				dst[dstIdx] = 0
				dstIdx++
			}
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

		if srcIdx >= srcEnd || dstIdx >= dstEnd {
			break
		}
	}

End:
	if runLength > 0 {
		runLength--

		// If runLength is not 1, add trailing 0s
		if runLength > dstEnd-dstIdx {
			err = errors.New("ZRLT inverse transform failed: output buffer is too small")
		} else {
			for runLength > 0 {
				runLength--
				dst[dstIdx] = 0
				dstIdx++
			}
		}
	}

	if srcIdx < srcEnd {
		err = errors.New("ZRLT inverse transform failed: output buffer is too small")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *ZRLT) MaxEncodedLen(srcLen int) int {
	return srcLen
}
