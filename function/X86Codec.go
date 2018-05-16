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
)

const (
	X86_INSTRUCTION_MASK = 0xFE
	X86_INSTRUCTION_JUMP = 0xE8
	X86_ADDRESS_MASK     = 0xD5
	X86_ESCAPE           = 0x02
)

// Adapted from MCM: https://github.com/mathieuchartier/mcm/blob/master/X86Binary.hpp
type X86Codec struct {
}

func NewX86Codec() (*X86Codec, error) {
	this := new(X86Codec)
	return this, nil
}

func (this *X86Codec) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	jumps := 0
	end := count - 8

	for i := 0; i < end; i++ {
		if src[i]&X86_INSTRUCTION_MASK == X86_INSTRUCTION_JUMP {
			// Count valid relative jumps (E8/E9 .. .. .. 00/FF)
			if src[i+4] == 0 || src[i+4] == 255 {
				// No encoding conflict ?
				if src[i] != 0 && src[i] != 1 && src[i] != X86_ESCAPE {
					jumps++
				}
			}
		}
	}

	if jumps < (count >> 7) {
		// Number of jump instructions too small => either not a binary
		// or not worth the change => skip. Very crude filter obviously.
		// Also, binaries usually have a lot of 0x88..0x8C (MOV) instructions.
		return 0, 0, errors.New("Not a binary or not enough jumps")
	}

	srcIdx := 0
	dstIdx := 0

	for srcIdx < end {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++

		// Relative jump ?
		if src[srcIdx-1]&X86_INSTRUCTION_MASK != X86_INSTRUCTION_JUMP {
			continue
		}

		cur := src[srcIdx]

		if cur == 0 || cur == 1 || cur == X86_ESCAPE {
			// Conflict prevents encoding the address. Emit escape symbol
			dst[dstIdx] = X86_ESCAPE
			dst[dstIdx+1] = cur
			srcIdx++
			dstIdx += 2
			continue
		}

		sgn := src[srcIdx+3]

		// Invalid sign of jump address difference => false positive ?
		if sgn != 0 && sgn != 255 {
			continue
		}

		addr := int32(src[srcIdx]) | (int32(src[srcIdx+1]) << 8) |
			(int32(src[srcIdx+2]) << 16) | (int32(sgn) << 24)

		addr += int32(srcIdx)
		dst[dstIdx] = byte(sgn + 1)
		dst[dstIdx+1] = X86_ADDRESS_MASK ^ byte(addr>>16)
		dst[dstIdx+2] = X86_ADDRESS_MASK ^ byte(addr>>8)
		dst[dstIdx+3] = X86_ADDRESS_MASK ^ byte(addr)
		srcIdx += 4
		dstIdx += 4
	}

	for srcIdx < count {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++
	}

	return uint(srcIdx), uint(dstIdx), nil
}

func (this *X86Codec) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)
	srcIdx := 0
	dstIdx := 0
	end := count - 8

	for srcIdx < end {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++

		// Relative jump ?
		if src[srcIdx-1]&X86_INSTRUCTION_MASK != X86_INSTRUCTION_JUMP {
			continue
		}

		sgn := src[srcIdx]

		if sgn == X86_ESCAPE {
			// Not an encoded address. Skip escape symbol
			srcIdx++
			continue
		}

		// Invalid sign of jump address difference => false positive ?
		if sgn != 1 && sgn != 0 {
			continue
		}

		addr := (X86_ADDRESS_MASK ^ int32(src[srcIdx+3])) |
			((X86_ADDRESS_MASK ^ int32(src[srcIdx+2])) << 8) |
			((X86_ADDRESS_MASK ^ int32(src[srcIdx+1])) << 16) |
			((0xFF & int32(sgn-1)) << 24)

		addr -= int32(dstIdx)
		dst[dstIdx] = byte(addr)
		dst[dstIdx+1] = byte(addr >> 8)
		dst[dstIdx+2] = byte(addr >> 16)
		dst[dstIdx+3] = byte(sgn - 1)
		srcIdx += 4
		dstIdx += 4
	}

	for srcIdx < count {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++
	}

	return uint(srcIdx), uint(dstIdx), nil
}

func (this X86Codec) MaxEncodedLen(srcLen int) int {
	return (srcLen * 5) >> 2
}
