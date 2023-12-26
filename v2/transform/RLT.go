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

package transform

// Implementation of an escaped RLE
// Run length encoding:
// RUN_LEN_ENCODE1 = 224 => RUN_LEN_ENCODE2 = 31*224 = 6944
// 4    <= runLen < 224+4      -> 1 byte
// 228  <= runLen < 6944+228   -> 2 bytes
// 7172 <= runLen < 65535+7172 -> 3 bytes

import (
	"errors"
	"fmt"
	"strings"

	internal "github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_RLT_RUN_LEN_ENCODE1  = 224                               // used to encode run length
	_RLT_RUN_LEN_ENCODE2  = (255 - _RLT_RUN_LEN_ENCODE1) << 8 // used to encode run length
	_RLT_RUN_THRESHOLD    = 3
	_RLT_MAX_RUN          = 0xFFFF + _RLT_RUN_LEN_ENCODE2 + _RLT_RUN_THRESHOLD - 1
	_RLT_MAX_RUN4         = _RLT_MAX_RUN - 4
	_RLT_MIN_BLOCK_LENGTH = 16
	_RLT_DEFAULT_ESCAPE   = 0xFB
)

// RLT a Run Length Transform with escape symbol
type RLT struct {
	ctx *map[string]any
}

// NewRLT creates a new instance of RLT
func NewRLT() (*RLT, error) {
	this := &RLT{}
	return this, nil
}

// NewRLTWithCtx creates a new instance of RLT using a
// configuration map as parameter.
func NewRLTWithCtx(ctx *map[string]any) (*RLT, error) {
	this := &RLT{}
	this.ctx = ctx
	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *RLT) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if len(src) < _RLT_MIN_BLOCK_LENGTH {
		return 0, 0, errors.New("Input buffer is too small, skip")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	dt := internal.DT_UNDEFINED
	findBestEscape := true

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt = val.(internal.DataType)

			if dt == internal.DT_DNA || dt == internal.DT_BASE64 || dt == internal.DT_UTF8 {
				return 0, 0, fmt.Errorf("RLT forward transform skip")
			}
		}

		if val, containsKey := (*this.ctx)["entropy"]; containsKey {
			entropyType := strings.ToUpper(val.(string))

			// Fast track if fast entropy coder is used
			if entropyType == "NONE" || entropyType == "ANS0" ||
				entropyType == "HUFFMAN" || entropyType == "RANGE" {
				findBestEscape = false
			}
		}
	}

	escape := byte(_RLT_DEFAULT_ESCAPE)

	if findBestEscape == true {
		freqs := [256]int{}
		internal.ComputeHistogram(src, freqs[:], true, false)

		if dt == internal.DT_UNDEFINED {
			dt = internal.DetectSimpleType(len(src), freqs[:])

			if this.ctx != nil && dt != internal.DT_UNDEFINED {
				(*this.ctx)["dataType"] = dt
			}

			if dt == internal.DT_DNA || dt == internal.DT_BASE64 || dt == internal.DT_UTF8 {
				return 0, 0, fmt.Errorf("RLT forward transform skip")
			}
		}

		minIdx := 0

		// Select escape symbol
		if freqs[minIdx] > 0 {
			for i, f := range &freqs {
				if f < freqs[minIdx] {
					minIdx = i

					if f == 0 {
						break
					}
				}
			}
		}

		escape = byte(minIdx)
	}

	srcIdx := 0
	dstIdx := 0
	srcEnd := len(src)
	srcEnd4 := srcEnd - 4
	dstEnd := len(dst)
	run := 0
	var err error
	prev := src[srcIdx]
	srcIdx++
	dst[dstIdx] = escape
	dstIdx++
	dst[dstIdx] = prev
	dstIdx++

	if prev == escape {
		dst[dstIdx] = 0
		dstIdx++
	}

	// Main loop
	for {
		if prev == src[srcIdx] {
			srcIdx++
			run++

			if prev == src[srcIdx] {
				srcIdx++
				run++

				if prev == src[srcIdx] {
					srcIdx++
					run++

					if prev == src[srcIdx] {
						srcIdx++
						run++

						if run < _RLT_MAX_RUN4 && srcIdx < srcEnd4 {
							continue
						}
					}
				}
			}
		}

		if run > _RLT_RUN_THRESHOLD {
			if dstIdx+6 >= dstEnd {
				err = errors.New("Output buffer is too small")
				break
			}

			dst[dstIdx] = prev
			dstIdx++

			if prev == escape {
				dst[dstIdx] = 0
				dstIdx++
			}

			dst[dstIdx] = escape
			dstIdx++

			dstIdx += emitRunLength(dst[dstIdx:dstEnd], run)
		} else if prev != escape {
			if dstIdx+run >= dstEnd {
				err = errors.New("Output buffer is too small")
				break
			}

			for run > 0 {
				dst[dstIdx] = prev
				dstIdx++
				run--
			}
		} else { // escape literal
			if dstIdx+2*run >= dstEnd {
				err = errors.New("Output buffer is too small")
				break
			}

			for run > 0 {
				dst[dstIdx] = escape
				dst[dstIdx+1] = 0
				dstIdx += 2
				run--
			}
		}

		prev = src[srcIdx]
		srcIdx++
		run = 1

		if srcIdx >= srcEnd4 {
			break
		}
	}

	if err == nil {
		// run == 1
		if prev != escape {
			if dstIdx+run < dstEnd {
				for run > 0 {
					dst[dstIdx] = prev
					dstIdx++
					run--
				}
			}
		} else { // escape literal
			if dstIdx+2*run < dstEnd {
				for run > 0 {
					dst[dstIdx] = escape
					dst[dstIdx+1] = 0
					dstIdx += 2
					run--
				}
			}
		}

		// Emit the last few bytes
		for srcIdx < srcEnd && dstIdx < dstEnd {
			if src[srcIdx] == escape {
				if dstIdx+2 >= dstEnd {
					break
				}

				dst[dstIdx] = escape
				dst[dstIdx+1] = 0
				dstIdx += 2
				srcIdx++
				continue
			}

			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++
		}

		if srcIdx != srcEnd {
			err = errors.New("Output buffer is too small")
		} else if dstIdx >= srcIdx {
			err = errors.New("Input not compressed")
		}
	}

	return uint(srcIdx), uint(dstIdx), err
}

func emitRunLength(dst []byte, run int) int {
	run -= _RLT_RUN_THRESHOLD

	// Encode run length
	if run < _RLT_RUN_LEN_ENCODE1 {
		dst[0] = byte(run)
		return 1
	}

	var dstIdx int

	if run < _RLT_RUN_LEN_ENCODE2 {
		run -= _RLT_RUN_LEN_ENCODE1
		dst[0] = byte(_RLT_RUN_LEN_ENCODE1 + (run >> 8))
		dstIdx = 1
	} else {
		run -= _RLT_RUN_LEN_ENCODE2
		dst[0] = byte(0xFF)
		dst[1] = byte(run >> 8)
		dstIdx = 2
	}

	dst[dstIdx] = byte(run)
	return dstIdx + 1
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *RLT) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	srcIdx := 0
	dstIdx := 0
	srcEnd := len(src)
	dstEnd := len(dst)
	escape := src[srcIdx]
	srcIdx++
	var err error

	if src[srcIdx] == escape {
		srcIdx++

		// The data cannot start with a run but may start with an escape literal
		if srcIdx < srcEnd && src[srcIdx] != 0 {
			return uint(srcIdx), uint(dstIdx), errors.New("Invalid input data: input starts with a run")
		}

		srcIdx++
		dst[dstIdx] = escape
		dstIdx++
	}

	// Main loop
	for srcIdx < srcEnd {
		if src[srcIdx] != escape {
			// Literal
			if dstIdx >= dstEnd {
				err = errors.New("Invalid input data")
				break
			}

			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++
			continue
		}

		srcIdx++

		if srcIdx >= srcEnd {
			err = errors.New("Invalid input data")
			break
		}

		run := int(src[srcIdx])
		srcIdx++

		if run == 0 {
			// Just an escape symbol, not a run
			if dstIdx >= dstEnd {
				err = errors.New("Invalid input data")
				break
			}

			dst[dstIdx] = escape
			dstIdx++
			continue
		}

		// Decode the length
		if run == 0xFF {
			if srcIdx+1 >= srcEnd {
				err = errors.New("Invalid input data")
				break
			}

			run = (int(src[srcIdx]) << 8) | int(src[srcIdx+1])
			srcIdx += 2
			run += _RLT_RUN_LEN_ENCODE2
		} else if run >= _RLT_RUN_LEN_ENCODE1 {
			if srcIdx >= srcEnd {
				err = errors.New("Invalid input data")
				break
			}

			run = ((run - _RLT_RUN_LEN_ENCODE1) << 8) | int(src[srcIdx])
			run += _RLT_RUN_LEN_ENCODE1
			srcIdx++
		}

		run += (_RLT_RUN_THRESHOLD - 1)

		// Sanity check
		if run > _RLT_MAX_RUN || dstIdx+run >= dstEnd {
			err = errors.New("Invalid run length")
			break
		}

		// Emit 'run' times the previous byte
		val := dst[dstIdx-1]
		d := dst[dstIdx : dstIdx+run]

		for i := range d {
			d[i] = val
		}

		dstIdx += run
	}

	if err == nil && srcIdx != srcEnd {
		err = errors.New("Invalid input data")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this RLT) MaxEncodedLen(srcLen int) int {
	if srcLen <= 512 {
		return srcLen + 32
	}

	return srcLen
}
