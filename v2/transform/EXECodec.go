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
	"encoding/binary"
	"errors"
	"fmt"

	internal "github.com/flanglet/kanzi-go/v2/internal"
)

// EXECodec is a codec that replaces relative jumps addresses with
// absolute ones in X86 code (to improve entropy coding).

const (
	_EXE_X86_MASK_JUMP        = 0xFE
	_EXE_X86_INSTRUCTION_JUMP = 0xE8
	_EXE_X86_INSTRUCTION_JCC  = 0x80
	_EXE_X86_TWO_BYTE_PREFIX  = 0x0F
	_EXE_X86_MASK_JCC         = 0xF0
	_EXE_X86_ESCAPE           = 0x9B
	_EXE_NOT_EXE              = 0x80
	_EXE_X86                  = 0x40
	_EXE_ARM64                = 0x20
	_EXE_MASK_DT              = 0x0F
	_EXE_X86_ADDR_MASK        = (1 << 24) - 1
	_EXE_MASK_ADDRESS         = 0xF0F0F0F0
	_EXE_ARM_B_ADDR_MASK      = (1 << 26) - 1
	_EXE_ARM_B_OPCODE_MASK    = 0xFFFFFFFF ^ _EXE_ARM_B_ADDR_MASK
	_EXE_ARM_B_ADDR_SGN_MASK  = 1 << 25
	_EXE_ARM_OPCODE_B         = 0x14000000 // 6 bit opcode
	_EXE_ARM_OPCODE_BL        = 0x94000000 // 6 bit opcode
	_EXE_ARM_CB_REG_BITS      = 5          // lowest bits for register
	_EXE_ARM_CB_ADDR_MASK     = 0x00FFFFE0 // 18 bit addr mask
	_EXE_ARM_CB_ADDR_SGN_MASK = 1 << 18
	_EXE_ARM_CB_OPCODE_MASK   = 0x7F000000
	_EXE_ARM_OPCODE_CBZ       = 0x34000000 // 8 bit opcode
	_EXE_ARM_OPCODE_CBNZ      = 0x3500000  // 8 bit opcode
	_EXE_WIN_PE               = 0x00004550
	_EXE_WIN_X86_ARCH         = 0x014C
	_EXE_WIN_AMD64_ARCH       = 0x8664
	_EXE_WIN_ARM64_ARCH       = 0xAA64
	_EXE_ELF_X86_ARCH         = 0x03
	_EXE_ELF_AMD64_ARCH       = 0x3E
	_EXE_ELF_ARM64_ARCH       = 0xB7
	_EXE_MAC_AMD64_ARCH       = 0x01000007
	_EXE_MAC_ARM64_ARCH       = 0x0100000C
	_EXE_MAC_MH_EXECUTE       = 0x02
	_EXE_MAC_LC_SEGMENT       = 0x01
	_EXE_MAC_LC_SEGMENT64     = 0x19
	_EXE_MIN_BLOCK_SIZE       = 4096
	_EXE_MAX_BLOCK_SIZE       = (1 << (26 + 2)) - 1 // max offset << 2

)

// EXECodec a codec for x86 code
type EXECodec struct {
	ctx          *map[string]any
	isBsVersion2 bool
}

// NewEXECodec creates a new instance of EXECodec
func NewEXECodec() (*EXECodec, error) {
	this := &EXECodec{}
	this.isBsVersion2 = false
	return this, nil
}

// NewEXECodecWithCtx creates a new instance of EXECodec using a
// configuration map as parameter.
func NewEXECodecWithCtx(ctx *map[string]any) (*EXECodec, error) {
	this := &EXECodec{}
	this.ctx = ctx
	bsVersion := uint(2)

	if ctx != nil {
		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			var ok bool
			bsVersion, ok = val.(uint)

			if ok == false {
				return nil, errors.New("Exe codec: invalid bitstream version type")
			}
		}
	}

	this.isBsVersion2 = bsVersion < 3
	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error. If the source data does not represent
// X86 code, an error is returned.
func (this *EXECodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count < _EXE_MIN_BLOCK_SIZE {
		return 0, 0, fmt.Errorf("ExeCodec forward failed: Block too small - size: %d, min %d)", count, _EXE_MIN_BLOCK_SIZE)
	}

	if count > _EXE_MAX_BLOCK_SIZE {
		return 0, 0, fmt.Errorf("ExeCodec forward failed: Block too big - size: %d, max %d", count, _EXE_MAX_BLOCK_SIZE)
	}

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("ExeCodec forward transform skip: Output buffer too small - size: %d, required %d", len(dst), n)
	}

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt := val.(internal.DataType)

			if dt != internal.DT_UNDEFINED && dt != internal.DT_EXE && dt != internal.DT_BIN {
				return 0, 0, fmt.Errorf("ExeCodec forward transform skip: Input is not an executable")
			}
		}
	}

	codeStart := 0
	codeEnd := count - 8
	mode := detectExeType(src[:codeEnd+4], &codeStart, &codeEnd)

	if mode&_EXE_NOT_EXE != 0 {
		if this.ctx != nil {
			(*this.ctx)["dataType"] = internal.DataType(mode & _EXE_MASK_DT)
		}

		return 0, 0, fmt.Errorf("ExeCodec forward transform skip: Input is not an executable")
	}

	mode &= ^byte(_EXE_MASK_DT)

	if this.ctx != nil {
		(*this.ctx)["dataType"] = internal.DT_EXE
	}

	if mode == _EXE_X86 {
		return this.forwardX86(src, dst, codeStart, codeEnd)
	}

	if mode == _EXE_ARM64 {
		return this.forwardARM(src, dst, codeStart, codeEnd)
	}

	return 0, 0, fmt.Errorf("ExeCodec forward transform skip: Input is not a supported executable format")
}

func (this *EXECodec) forwardX86(src, dst []byte, codeStart, codeEnd int) (uint, uint, error) {
	srcIdx := codeStart
	dstIdx := 9
	matches := 0
	dstEnd := len(dst) - 5
	dst[0] = _EXE_X86
	matches = 0

	if codeStart > len(src) || codeEnd > len(src) {
		return 0, 0, fmt.Errorf("ExeCodec forward transform skip: Input is not a supported executable format")
	}

	if codeStart > 0 {
		copy(dst[dstIdx:], src[0:codeStart])
		dstIdx += codeStart
	}

	for srcIdx < codeEnd && dstIdx < dstEnd {
		if src[srcIdx] == _EXE_X86_TWO_BYTE_PREFIX {
			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++

			if (src[srcIdx] & _EXE_X86_MASK_JCC) != _EXE_X86_INSTRUCTION_JCC {
				// Not a relative jump
				if src[srcIdx] == _EXE_X86_ESCAPE {
					dst[dstIdx] = _EXE_X86_ESCAPE
					dstIdx++
				}

				dst[dstIdx] = src[srcIdx]
				srcIdx++
				dstIdx++
				continue
			}
		} else if (src[srcIdx] & _EXE_X86_MASK_JUMP) != _EXE_X86_INSTRUCTION_JUMP {
			// Not a relative call
			if src[srcIdx] == _EXE_X86_ESCAPE {
				dst[dstIdx] = _EXE_X86_ESCAPE
				dstIdx++
			}

			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++
			continue
		}

		// Current instruction is a jump/call.
		sgn := src[srcIdx+4]
		offset := int(binary.LittleEndian.Uint32(src[srcIdx+1:]))

		if (sgn != 0 && sgn != 0xFF) || (offset == 0xFF000000) {
			dst[dstIdx] = _EXE_X86_ESCAPE
			dst[dstIdx+1] = src[srcIdx]
			srcIdx++
			dstIdx += 2
			continue
		}

		// Absolute target address = srcIdx + 5 + offset. Let us ignore the +5
		addr := srcIdx

		if sgn == 0 {
			addr += offset
		} else {
			addr -= (-offset & _EXE_X86_ADDR_MASK)
		}

		dst[dstIdx] = src[srcIdx]
		binary.BigEndian.PutUint32(dst[dstIdx+1:], uint32(addr^_EXE_MASK_ADDRESS))
		srcIdx += 5
		dstIdx += 5
		matches++
	}

	if matches < 16 {
		return uint(srcIdx), uint(dstIdx), errors.New("ExeCodec forward transform skip: Too few calls/jumps")
	}

	count := len(src)

	// Cap expansion due to false positives
	if srcIdx < codeEnd || dstIdx+(count-srcIdx) > dstEnd {
		return uint(srcIdx), uint(dstIdx), errors.New("ExeCodec forward transform skip: Too many false positives")
	}

	binary.LittleEndian.PutUint32(dst[1:], uint32(codeStart))
	binary.LittleEndian.PutUint32(dst[5:], uint32(dstIdx))
	copy(dst[dstIdx:], src[srcIdx:count])
	dstIdx += (count - srcIdx)

	// Cap expansion due to false positives
	if dstIdx > count+(count/50) {
		return uint(srcIdx), uint(dstIdx), errors.New("ExeCodec forward transform skip: Too many false positives")
	}

	return uint(count), uint(dstIdx), nil
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *EXECodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	// Old format
	if this.isBsVersion2 == true {
		return this.inverseV2(src, dst)
	}

	if len(src) < 9 {
		return 0, 0, errors.New("ExeCodec inverse transform failed: invalid data")
	}

	mode := src[0]

	if mode == _EXE_X86 {
		return this.inverseX86(src, dst)
	}

	if mode == _EXE_ARM64 {
		return this.inverseARM(src, dst)
	}

	return 0, 0, errors.New("ExeCodec inverse transform failed: unknown binary type")
}

func (this *EXECodec) inverseX86(src, dst []byte) (uint, uint, error) {
	srcIdx := 9
	dstIdx := 0
	codeStart := int(binary.LittleEndian.Uint32(src[1:]))
	codeEnd := int(binary.LittleEndian.Uint32(src[5:]))

	// Sanity check
	if codeStart+srcIdx > len(src) || codeStart+dstIdx > len(dst) || codeEnd > len(src) {
		return 0, 0, errors.New("ExeCodec inverse transform failed: invalid data")
	}

	if codeStart > 0 {
		copy(dst[dstIdx:], src[srcIdx:srcIdx+codeStart])
		dstIdx += codeStart
		srcIdx += codeStart
	}

	for srcIdx < codeEnd {
		if src[srcIdx] == _EXE_X86_TWO_BYTE_PREFIX {
			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++

			if (src[srcIdx] & _EXE_X86_MASK_JCC) != _EXE_X86_INSTRUCTION_JCC {
				// Not a relative jump
				if src[srcIdx] == _EXE_X86_ESCAPE {
					srcIdx++
				}

				dst[dstIdx] = src[srcIdx]
				srcIdx++
				dstIdx++
				continue
			}
		} else if (src[srcIdx] & _EXE_X86_MASK_JUMP) != _EXE_X86_INSTRUCTION_JUMP {
			// Not a relative call
			if src[srcIdx] == _EXE_X86_ESCAPE {
				srcIdx++
			}

			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++
			continue
		}

		// Current instruction is a jump/call. Decode absolute address
		addr := int(binary.BigEndian.Uint32(src[srcIdx+1:])) ^ _EXE_MASK_ADDRESS
		offset := addr - dstIdx
		dst[dstIdx] = src[srcIdx]
		srcIdx++
		dstIdx++

		if offset >= 0 {
			binary.LittleEndian.PutUint32(dst[dstIdx:], uint32(offset))
		} else {
			binary.LittleEndian.PutUint32(dst[dstIdx:], uint32(-(-offset & _EXE_X86_ADDR_MASK)))
		}

		srcIdx += 4
		dstIdx += 4
	}

	count := len(src)

	if srcIdx < count {
		copy(dst[dstIdx:], src[srcIdx:count])
		dstIdx += (count - srcIdx)
	}

	return uint(count), uint(dstIdx), nil
}

func (this *EXECodec) inverseV2(src, dst []byte) (uint, uint, error) {
	count := len(src)
	srcIdx := 0
	dstIdx := 0
	end := count - 8

	for srcIdx < end {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++

		// Relative jump ?
		if src[srcIdx-1]&_EXE_X86_MASK_JUMP != _EXE_X86_INSTRUCTION_JUMP {
			continue
		}

		if src[srcIdx] == 0xF5 {
			// Not an encoded address. Skip escape symbol
			srcIdx++
			continue
		}

		sgn := src[srcIdx] - 1

		// Invalid sign of jump address difference => false positive ?
		if sgn != 0 && sgn != 0xFF {
			continue
		}

		addr := (0xD5 ^ int32(src[srcIdx+3])) |
			((0xD5 ^ int32(src[srcIdx+2])) << 8) |
			((0xD5 ^ int32(src[srcIdx+1])) << 16) |
			((0xFF & int32(sgn)) << 24)

		addr -= int32(dstIdx)
		dst[dstIdx] = byte(addr)
		dst[dstIdx+1] = byte(addr >> 8)
		dst[dstIdx+2] = byte(addr >> 16)
		dst[dstIdx+3] = sgn
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

func (this *EXECodec) forwardARM(src, dst []byte, codeStart, codeEnd int) (uint, uint, error) {
	srcIdx := codeStart
	dstIdx := 9
	matches := 0
	dstEnd := len(dst) - 8
	dst[0] = _EXE_ARM64
	matches = 0

	if codeStart > len(src) || codeEnd > len(src) {
		return 0, 0, fmt.Errorf("ExeCodec forward failed: Input is not a supported executable format")
	}

	if codeStart > 0 {
		copy(dst[dstIdx:], src[0:codeStart])
		dstIdx += codeStart
	}

	for srcIdx < codeEnd && dstIdx < dstEnd {
		instr := int(binary.LittleEndian.Uint32(src[srcIdx:]))
		opcode1 := instr & _EXE_ARM_B_OPCODE_MASK
		//opcode2 := instr & ARM_CB_OPCODE_MASK
		isBL := (opcode1 == _EXE_ARM_OPCODE_B) || (opcode1 == _EXE_ARM_OPCODE_BL) // unconditional jump
		// disable for now ... isCB = (opcode2 == ARM_OPCODE_CBZ) || (opcode2 == ARM_OPCODE_CBNZ) // conditional jump
		//isCB := false

		if isBL == false { // && isCB == false {
			// Not a relative jump
			copy(dst[dstIdx:], src[srcIdx:srcIdx+4])
			srcIdx += 4
			dstIdx += 4
			continue
		}

		var addr int
		var val int

		if isBL == true {
			// opcode(6) + sgn(1) + offset(25)
			// Absolute target address = srcIdx +/- (offset*4)
			offset := int(int32(instr & _EXE_ARM_B_ADDR_MASK))

			if instr&_EXE_ARM_B_ADDR_SGN_MASK == 0 {
				addr = srcIdx + 4*offset
			} else {
				addr = srcIdx - 4*int(int32(-offset&_EXE_ARM_B_ADDR_MASK))
			}

			if addr < 0 {
				addr = 0
			}

			val = opcode1 | (addr >> 2)
		} else { // isCB == true
			// opcode(8) + sgn(1) + offset(18) + register(5)
			// Absolute target address = srcIdx +/- (offset*4)
			offset := (instr & _EXE_ARM_CB_ADDR_MASK) >> _EXE_ARM_CB_REG_BITS

			if instr&_EXE_ARM_CB_ADDR_SGN_MASK == 0 {
				addr = srcIdx + 4*offset
			} else {
				addr = srcIdx + 4*(0xFFFC0000|offset)
			}

			if addr < 0 {
				addr = 0
			}

			val = (instr & ^_EXE_ARM_CB_ADDR_MASK) | ((addr >> 2) << _EXE_ARM_CB_REG_BITS)
		}

		if addr == 0 {
			binary.LittleEndian.PutUint32(dst[dstIdx:], uint32(val)) // 0 address as escape
			copy(dst[dstIdx+4:], src[srcIdx:srcIdx+4])
			srcIdx += 4
			dstIdx += 8
			continue
		}

		binary.LittleEndian.PutUint32(dst[dstIdx:], uint32(val))
		srcIdx += 4
		dstIdx += 4
		matches++
	}

	if matches < 16 {
		return uint(srcIdx), uint(dstIdx), errors.New("ExeCodec forward transform skip: Too few calls/jumps")
	}

	count := len(src)

	// Cap expansion due to false positives
	if srcIdx < codeEnd || dstIdx+(count-srcIdx) > dstEnd {
		return uint(srcIdx), uint(dstIdx), errors.New("ExeCodec forward transform skip: Too many false positives")
	}

	binary.LittleEndian.PutUint32(dst[1:], uint32(codeStart))
	binary.LittleEndian.PutUint32(dst[5:], uint32(dstIdx))
	copy(dst[dstIdx:], src[srcIdx:count])
	dstIdx += (count - srcIdx)

	// Cap expansion due to false positives
	if dstIdx > count+(count/50) {
		return uint(srcIdx), uint(dstIdx), errors.New("ExeCodec forward transform skip: Too many false positives")
	}

	return uint(count), uint(dstIdx), nil
}

func (this *EXECodec) inverseARM(src, dst []byte) (uint, uint, error) {
	srcIdx := 9
	dstIdx := 0
	codeStart := int(binary.LittleEndian.Uint32(src[1:]))
	codeEnd := int(binary.LittleEndian.Uint32(src[5:]))

	// Sanity check
	if codeStart+srcIdx > len(src) || codeStart+dstIdx > len(dst) || codeEnd > len(src) {
		return 0, 0, errors.New("ExeCodec inverse transform failed: invalid data")
	}

	if codeStart > 0 {
		copy(dst[dstIdx:], src[srcIdx:srcIdx+codeStart])
		dstIdx += codeStart
		srcIdx += codeStart
	}

	for srcIdx < codeEnd {
		instr := int(binary.LittleEndian.Uint32(src[srcIdx:]))
		opcode1 := instr & _EXE_ARM_B_OPCODE_MASK
		//copcode2 := instr & ARM_CB_OPCODE_MASK
		isBL := (opcode1 == _EXE_ARM_OPCODE_B) || (opcode1 == _EXE_ARM_OPCODE_BL) // unconditional jump
		// disable for now ... isCB = (opcode2 == ARM_OPCODE_CBZ) || (opcode2 == ARM_OPCODE_CBNZ); // conditional jump
		//isCB := false

		if isBL == false { //} && isCB == false {
			// Not a relative jump
			copy(dst[dstIdx:], src[srcIdx:srcIdx+4])
			srcIdx += 4
			dstIdx += 4
			continue
		}

		// Decode absolute address
		var addr int
		var val int

		if isBL == true {
			addr = (instr & _EXE_ARM_B_ADDR_MASK) << 2
			offset := (addr - dstIdx) >> 2
			val = opcode1 | (offset & _EXE_ARM_B_ADDR_MASK)
		} else {
			addr = ((instr & _EXE_ARM_CB_ADDR_MASK) >> _EXE_ARM_CB_REG_BITS) << 2
			offset := (addr - dstIdx) >> 2
			val = (instr & ^_EXE_ARM_CB_ADDR_MASK) | (offset << _EXE_ARM_CB_REG_BITS)
		}

		if addr == 0 {
			copy(dst[dstIdx:], src[srcIdx+4:srcIdx+8])
			srcIdx += 8
			dstIdx += 4
			continue
		}

		binary.LittleEndian.PutUint32(dst[dstIdx:], uint32(val))
		srcIdx += 4
		dstIdx += 4
	}

	count := len(src)

	if srcIdx < count {
		copy(dst[dstIdx:], src[srcIdx:count])
		dstIdx += (count - srcIdx)
	}

	return uint(count), uint(dstIdx), nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *EXECodec) MaxEncodedLen(srcLen int) int {
	// Allocate some extra buffer for incompressible data.
	if srcLen <= 256 {
		return srcLen + 32
	}

	return srcLen + srcLen/8
}

func detectExeType(src []byte, codeStart, codeEnd *int) byte {
	// Let us check the first bytes ... but this may not be the first block
	// Best effort
	magic := internal.GetMagicType(src)
	arch := 0

	if parseExeHeader(src, magic, &arch, codeStart, codeEnd) == true {
		if (arch == _EXE_ELF_X86_ARCH) || (arch == _EXE_ELF_AMD64_ARCH) {
			return _EXE_X86
		}

		if (arch == _EXE_WIN_X86_ARCH) || (arch == _EXE_WIN_AMD64_ARCH) {
			return _EXE_X86
		}

		if arch == _EXE_MAC_AMD64_ARCH {
			return _EXE_X86
		}

		if (arch == _EXE_ELF_ARM64_ARCH) || (arch == _EXE_WIN_ARM64_ARCH) {
			return _EXE_ARM64
		}

		if arch == _EXE_MAC_ARM64_ARCH {
			return _EXE_ARM64
		}
	}

	jumpsX86 := 0
	jumpsARM64 := 0
	count := *codeEnd - *codeStart
	var histo [256]int

	for i := *codeStart; i < *codeEnd; i++ {
		histo[src[i]]++

		// X86
		if (src[i] & _EXE_X86_MASK_JUMP) == _EXE_X86_INSTRUCTION_JUMP {
			if (src[i+4] == 0) || (src[i+4] == 0xFF) {
				// Count relative jumps (CALL = E8/ JUMP = E9 .. .. .. 00/FF)
				jumpsX86++
				continue
			}
		} else if src[i] == _EXE_X86_TWO_BYTE_PREFIX {
			i++

			if (src[i] == 0x38) || (src[i] == 0x3A) {
				i++
			}

			// Count relative conditional jumps (0x0F 0x8?) with 16/32 offsets
			if (src[i] & _EXE_X86_MASK_JCC) == _EXE_X86_INSTRUCTION_JCC {
				jumpsX86++
				continue
			}
		}

		// ARM
		if (i & 3) != 0 {
			continue
		}

		instr := binary.LittleEndian.Uint32(src[i:])
		opcode1 := instr & _EXE_ARM_B_OPCODE_MASK
		opcode2 := instr & _EXE_ARM_CB_OPCODE_MASK

		if (opcode1 == _EXE_ARM_OPCODE_B) || (opcode1 == _EXE_ARM_OPCODE_BL) || (opcode2 == _EXE_ARM_OPCODE_CBZ) || (opcode2 == _EXE_ARM_OPCODE_CBNZ) {
			jumpsARM64++
		}
	}

	var dt internal.DataType

	if dt = internal.DetectSimpleType(count, histo[:]); dt != internal.DT_BIN {
		return _EXE_NOT_EXE | byte(dt)
	}

	// Filter out (some/many) multimedia files
	smallVals := 0

	for _, h := range histo[0:16] {
		smallVals += h
	}

	if histo[0] < (count/10) || smallVals > (count/2) || histo[255] < (count/100) {
		return _EXE_NOT_EXE | byte(dt)
	}

	// Ad-hoc thresholds
	if jumpsX86 >= (count/200) && histo[255] >= (count/50) {
		return _EXE_X86
	}

	if jumpsARM64 >= (count / 200) {
		return _EXE_ARM64
	}

	// Number of jump instructions too small => either not an exe or not worth the change, skip.
	return _EXE_NOT_EXE | byte(dt)
}

// Return true if known header
func parseExeHeader(src []byte, magic uint, arch, codeStart, codeEnd *int) bool {
	count := len(src)

	if magic == internal.WIN_MAGIC {
		if count >= 64 {
			posPE := int(binary.LittleEndian.Uint32(src[60:]))

			if (posPE > 0) && (posPE <= count-48) && (int(binary.LittleEndian.Uint32(src[posPE:])) == _EXE_WIN_PE) {
				*codeStart = min(int(binary.LittleEndian.Uint32(src[posPE+44:])), count)
				*codeEnd = min(*codeStart+int(binary.LittleEndian.Uint32(src[posPE+28:])), count)
				*arch = int(binary.LittleEndian.Uint16(src[posPE+4:]))
			}

			return true
		}
	} else if magic == internal.ELF_MAGIC {
		isLittleEndian := src[5] == 1

		if count >= 64 {
			*codeStart = 0

			if isLittleEndian == true {
				// Little Endian
				if src[4] == 2 {
					// 64 bits
					nbEntries := int(binary.LittleEndian.Uint16(src[0x3C:]))
					szEntry := int(binary.LittleEndian.Uint16(src[0x3A:]))
					posSection := int(binary.LittleEndian.Uint64(src[0x28:]))

					for i := 0; i < nbEntries; i++ {
						startEntry := posSection + i*szEntry

						if startEntry+0x28 >= count {
							return false
						}

						typeSection := int(binary.LittleEndian.Uint32(src[startEntry+4:]))
						offSection := int(binary.LittleEndian.Uint64(src[startEntry+0x18:]))
						lenSection := int(binary.LittleEndian.Uint64(src[startEntry+0x20:]))

						if typeSection == 1 && lenSection >= 64 {
							if *codeStart == 0 {
								*codeStart = offSection
							}

							*codeEnd = offSection + lenSection
						}
					}
				} else {
					// 32 bits
					nbEntries := int(binary.LittleEndian.Uint16(src[0x30:]))
					szEntry := int(binary.LittleEndian.Uint16(src[0x2E:]))
					posSection := int(binary.LittleEndian.Uint32(src[0x20:]))

					for i := 0; i < nbEntries; i++ {
						startEntry := posSection + i*szEntry

						if startEntry+0x18 >= count {
							return false
						}

						typeSection := int(binary.LittleEndian.Uint32(src[startEntry+4:]))
						offSection := int(binary.LittleEndian.Uint32(src[startEntry+0x10:]))
						lenSection := int(binary.LittleEndian.Uint32(src[startEntry+0x14:]))

						if typeSection == 1 && lenSection >= 64 {
							if *codeStart == 0 {
								*codeStart = offSection
							}

							*codeEnd = offSection + lenSection
						}
					}
				}

				*arch = int(binary.LittleEndian.Uint16(src[18:]))
			} else {
				// Big Endian
				if src[4] == 2 {
					// 64 bits
					nbEntries := int(binary.BigEndian.Uint16(src[0x3C:]))
					szEntry := int(binary.BigEndian.Uint16(src[0x3A:]))
					posSection := int(binary.BigEndian.Uint64(src[0x28:]))

					for i := 0; i < nbEntries; i++ {
						startEntry := posSection + i*szEntry

						if startEntry+0x28 >= count {
							return false
						}

						typeSection := int(binary.BigEndian.Uint32(src[startEntry+4:]))
						offSection := int(binary.BigEndian.Uint64(src[startEntry+0x18:]))
						lenSection := int(binary.BigEndian.Uint64(src[startEntry+0x20:]))

						if typeSection == 1 && lenSection >= 64 {
							if *codeStart == 0 {
								*codeStart = offSection
							}

							*codeEnd = offSection + lenSection
						}
					}
				} else {
					// 32 bits
					nbEntries := int(binary.BigEndian.Uint16(src[0x30:]))
					szEntry := int(binary.BigEndian.Uint16(src[0x2E:]))
					posSection := int(binary.BigEndian.Uint32(src[0x20:]))

					for i := 0; i < nbEntries; i++ {
						startEntry := posSection + i*szEntry

						if startEntry+0x18 >= count {
							return false
						}

						typeSection := int(binary.BigEndian.Uint32(src[startEntry+4:]))
						offSection := int(binary.BigEndian.Uint32(src[startEntry+0x10:]))
						lenSection := int(binary.BigEndian.Uint32(src[startEntry+0x14:]))

						if typeSection == 1 && lenSection >= 64 {
							if *codeStart == 0 {
								*codeStart = offSection
							}

							*codeEnd = offSection + lenSection
						}
					}
				}

				*arch = int(binary.BigEndian.Uint16(src[18:]))
			}

			*codeStart = min(*codeStart, count)
			*codeEnd = min(*codeEnd, count)
			return true
		}
	} else if (magic == internal.MAC_MAGIC32) || (magic == internal.MAC_CIGAM32) ||
		(magic == internal.MAC_MAGIC64) || (magic == internal.MAC_CIGAM64) {
		is64Bits := magic == internal.MAC_MAGIC64 || magic == internal.MAC_CIGAM64
		*codeStart = 0

		if count >= 64 {
			mode := binary.LittleEndian.Uint32(src[12:])

			if mode != _EXE_MAC_MH_EXECUTE {
				return false
			}

			*arch = int(binary.LittleEndian.Uint32(src[4:]))
			nbCmds := int(binary.LittleEndian.Uint32(src[0x10:]))
			cmd := 0
			pos := 0x1C

			if is64Bits == true {
				pos = 0x20
			}

			for cmd < nbCmds {
				ldCmd := int(binary.LittleEndian.Uint32(src[pos:]))
				szCmd := int(binary.LittleEndian.Uint32(src[pos+4:]))
				szSegHdr := 0x38

				if is64Bits == true {
					szSegHdr = 0x48
				}

				if ldCmd == _EXE_MAC_LC_SEGMENT || ldCmd == _EXE_MAC_LC_SEGMENT64 {
					if pos+14 >= count {
						return false
					}

					nameSegment := binary.BigEndian.Uint64(src[pos+8:]) >> 16

					if nameSegment == 0x5F5F54455854 {
						posSection := pos + szSegHdr

						if posSection+0x34 >= count {
							return false
						}

						nameSection := binary.BigEndian.Uint64(src[posSection:]) >> 16

						if nameSection == 0x5F5F74657874 {
							// Text section in TEXT segment
							if is64Bits == true {
								*codeStart = int(int32(binary.LittleEndian.Uint64(src[posSection+0x30:])))
								*codeEnd = *codeStart + int(int32(binary.LittleEndian.Uint32(src[posSection+0x28:])))
								break
							} else {
								*codeStart = int(int32(binary.LittleEndian.Uint32(src[posSection+0x2C:])))
								*codeEnd = *codeStart + int(int32(binary.LittleEndian.Uint32(src[posSection+0x28:])))
								break
							}
						}
					}
				}

				cmd++
				pos += szCmd
			}

			*codeStart = min(*codeStart, count)
			*codeEnd = min(*codeEnd, count)
			return true
		}
	}

	return false
}
