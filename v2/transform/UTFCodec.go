/*
Copyright 2011-2026 Frederic Langlet
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
	"math/bits"
	"slices"

	internal "github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_UTF_MIN_BLOCKSIZE = 1024
)

var (
	_UTF_SIZES = [256]uint8{
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
		4, 4, 4, 4, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
)

type sdUTF struct {
	sym  int32 // symbol
	freq int32 // frequency
}

type utfSymbol struct {
	value  [4]byte
	length uint8
}

// UTFCodec is a simple one-pass UTF8 codec that replaces code points with indexes.
type UTFCodec struct {
	ctx      *map[string]any
	aliasMap []int32
	tokens   []uint32
}

// NewUTFCodec creates a new instance of UTFCodec
func NewUTFCodec() (*UTFCodec, error) {
	this := &UTFCodec{}
	return this, nil
}

// NewUTFCodecWithCtx creates a new instance of UTFCodec using a
// configuration map as parameter.
func NewUTFCodecWithCtx(ctx *map[string]any) (*UTFCodec, error) {
	this := &UTFCodec{}
	this.ctx = ctx
	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *UTFCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if len(src) < _UTF_MIN_BLOCKSIZE {
		return 0, 0, fmt.Errorf("Input block is too small - size: %d, required %d", len(src), _UTF_MIN_BLOCKSIZE)
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	count := len(src)
	mustValidate := true

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt, ok := val.(internal.DataType)

			if !ok {
				return 0, 0, errors.New("UTF forward transform failed: invalid data type")
			}

			if dt != internal.DT_UNDEFINED && dt != internal.DT_UTF8 {
				return 0, 0, errors.New("UTF forward transform skip: not UTF")
			}

			mustValidate = dt != internal.DT_UTF8
		}
	}

	start := 0

	if src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		start = 3
	} else {
		// Skip up to three leading continuation bytes from a sequence split across blocks.
		for start < 3 && src[start]&0xC0 == 0x80 {
			start++
		}
	}

	srcEnd := count - 4
	if mustValidate && countHighBitBytes(src, start, srcEnd) < uint64((count-start-4)/8) {
		return 0, 0, errors.New("UTF forward transform skip: not UTF")
	}

	if this.aliasMap == nil {
		this.aliasMap = make([]int32, 1<<22)
	}
	if cap(this.tokens) < count {
		this.tokens = make([]uint32, count)
	} else {
		this.tokens = this.tokens[:count]
	}

	asciiFreq := [128]uint32{}
	mbKeys := make([]uint32, 0, max(count>>9, 256))
	defer func() {
		clear(this.aliasMap[:128])
		for _, key := range mbKeys {
			this.aliasMap[key] = 0
		}
	}()

	tokens := this.tokens
	ntok := 0
	var contBytes uint
	i := start

	// Validate sequences, count frequencies, and record packed tokens.
	for i < srcEnd {
		b0 := src[i]
		if b0 < 0x80 {
			if i+8 <= srcEnd && binary.LittleEndian.Uint64(src[i:])&0x8080808080808080 == 0 {
				for k := 0; k < 8; k++ {
					c := src[i+k]
					asciiFreq[c]++
					tokens[ntok] = uint32(c)
					ntok++
				}
				i += 8
				continue
			}

			asciiFreq[b0]++
			tokens[ntok] = uint32(b0)
			ntok++
			i++
			continue
		}

		var val uint32
		var size int
		if b0 < 0xE0 {
			b1 := src[i+1]
			if b0 < 0xC2 || b1&0xC0 != 0x80 {
				return 0, 0, errors.New("UTF forward transform skip: invalid or too complex")
			}
			val = (1 << 19) | (uint32(b0) << 8) | uint32(b1)
			size = 2
			contBytes++
		} else if b0 < 0xF0 {
			b1, b2 := src[i+1], src[i+2]
			slot := b0 & 0x1F
			if b1 < utfMinB1[slot] || b1-utfMinB1[slot] > utfSpanB1[slot] || b2&0xC0 != 0x80 {
				return 0, 0, errors.New("UTF forward transform skip: invalid or too complex")
			}
			val = (2 << 19) | (uint32(b0&0x0F) << 12) | (uint32(b1&0x3F) << 6) | uint32(b2&0x3F)
			size = 3
			contBytes += 2
		} else {
			b1, b2, b3 := src[i+1], src[i+2], src[i+3]
			if b0 > 0xF4 {
				return 0, 0, errors.New("UTF forward transform skip: invalid or too complex")
			}
			slot := b0 & 0x1F
			if b1 < utfMinB1[slot] || b1-utfMinB1[slot] > utfSpanB1[slot] || b2&0xC0 != 0x80 || b3&0xC0 != 0x80 {
				return 0, 0, errors.New("UTF forward transform skip: invalid or too complex")
			}
			val = (4 << 19) | (uint32(b0&0x07) << 18) | (uint32(b1&0x3F) << 12) | (uint32(b2&0x3F) << 6) | uint32(b3&0x3F)
			size = 4
			contBytes += 3
		}

		if this.aliasMap[val] == 0 {
			if len(mbKeys) >= 32767 {
				return 0, 0, errors.New("UTF forward transform skip: invalid or too complex")
			}
			mbKeys = append(mbKeys, val)
		}
		this.aliasMap[val]++
		tokens[ntok] = val
		ntok++
		i += size
	}

	if mustValidate && contBytes < uint((count-start-4)/8) {
		return 0, 0, errors.New("UTF forward transform skip: not UTF")
	}

	items := make([]uint64, 0, len(mbKeys)+128)
	for c, freq := range asciiFreq {
		if freq != 0 {
			items = append(items, uint64(freq)<<24|uint64(c))
		}
	}
	for _, val := range mbKeys {
		items = append(items, uint64(uint32(this.aliasMap[val]))<<24|uint64(val))
	}
	n := len(items)
	maxTarget := count - count/10
	if n == 0 || n >= 32768 {
		return 0, 0, errors.New("UTF forward transform skip: invalid or too complex")
	}
	if this.ctx != nil {
		(*this.ctx)["dataType"] = internal.DT_UTF8
	}
	if 3*n+6 >= maxTarget {
		return 0, 0, errors.New("UTF forward transform skip: no improvement")
	}

	// Ascending sort, then emit in reverse: decreasing frequency and decreasing symbol.
	slices.Sort(items)
	dstIdx := 2
	dst[dstIdx] = byte(n >> 8)
	dstIdx++
	dst[dstIdx] = byte(n)
	dstIdx++
	estimate := dstIdx + 3*n + start + 1
	for rank := 0; rank < n; rank++ {
		item := items[n-1-rank]
		freq := int(item >> 24)
		sym := uint32(item & 0xFFFFFF)
		if rank < 128 {
			estimate += freq
			this.aliasMap[sym] = int32(rank)
		} else {
			estimate += 2 * freq
			this.aliasMap[sym] = int32(0x10080 | ((rank << 1) & 0xFF00) | (rank & 0x7F))
		}
		dst[dstIdx] = byte(sym >> 16)
		dst[dstIdx+1] = byte(sym >> 8)
		dst[dstIdx+2] = byte(sym)
		dstIdx += 3
	}
	if estimate >= maxTarget {
		return 0, uint(dstIdx), errors.New("UTF forward transform skip: no improvement")
	}

	copy(dst[dstIdx:], src[:start])
	dstIdx += start
	for k := 0; k < ntok; k++ {
		alias := uint32(this.aliasMap[tokens[k]])
		dst[dstIdx] = byte(alias)
		dstIdx++
		if alias>>16 != 0 {
			dst[dstIdx] = byte(alias >> 8)
			dstIdx++
		}
	}
	dst[0] = byte(start)
	dst[1] = byte(i - srcEnd)
	copy(dst[dstIdx:], src[i:])
	dstIdx += count - i

	if dstIdx >= maxTarget {
		return uint(count), uint(dstIdx), errors.New("UTF forward transform skip: no improvement")
	}
	return uint(count), uint(dstIdx), nil
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *UTFCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}
	if len(src) < 4 {
		return 0, 0, fmt.Errorf("Input block is too small - size: %d, required %d", len(src), 4)
	}
	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)
	start := int(src[0]) & 0x03
	adjust := int(src[1]) & 0x03
	n := (int(src[2]) << 8) + int(src[3])
	if n == 0 || n >= 32768 || 3*n > count-4 {
		return 0, 0, errors.New("UTF inverse transform: invalid map size")
	}

	isBsVersion3 := false
	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["bsVersion"]; containsKey {
			bsVersion, ok := val.(uint)
			if !ok {
				return 0, 0, errors.New("UTF inverse transform failed: invalid bitstream version type")
			}
			isBsVersion3 = bsVersion < 4
		}
	}

	m := [32768]utfSymbol{}
	srcIdx := 4
	maxLen := 1
	for j := 0; j < n; j++ {
		sym := (uint32(src[srcIdx]) << 16) | (uint32(src[srcIdx+1]) << 8) | uint32(src[srcIdx+2])
		var size int
		if isBsVersion3 {
			size = unpackUTF0(sym, m[j].value[:])
		} else {
			size = unpackUTF1(sym, m[j].value[:])
		}
		if size == 0 {
			return 0, 0, errors.New("UTF inverse transform failed: invalid UTF alias")
		}
		m[j].length = uint8(size)
		if size > maxLen {
			maxLen = size
		}
		srcIdx += 3
	}

	srcEnd := count - 4 + adjust
	dstIdx := 0
	if len(dst) < 4 {
		return 0, 0, errors.New("UTF inverse transform failed: invalid output block size")
	}
	if srcEnd < srcIdx || srcEnd > count || srcIdx+start > srcEnd || start > len(dst) {
		return 0, 0, errors.New("UTF inverse transform failed: invalid data")
	}
	copy(dst[:start], src[srcIdx:srcIdx+start])
	srcIdx += start
	dstIdx += start

	if n <= 128 {
		for srcIdx < srcEnd && len(dst)-dstIdx >= 4 {
			safeSteps := min(srcEnd-srcIdx, (len(dst)-dstIdx-4)/maxLen)
			if safeSteps < 4 {
				break
			}
			batchEnd := srcIdx + (safeSteps &^ 3)
			for srcIdx < batchEnd {
				aliases := [4]int{int(src[srcIdx]), int(src[srcIdx+1]), int(src[srcIdx+2]), int(src[srcIdx+3])}
				srcIdx += 4
				for _, alias := range aliases {
					if alias >= n {
						return 0, 0, errors.New("UTF inverse transform failed: invalid data")
					}
					sym := m[alias]
					copy(dst[dstIdx:dstIdx+4], sym.value[:])
					dstIdx += int(sym.length)
				}
			}
		}
		for srcIdx < srcEnd {
			alias := int(src[srcIdx])
			srcIdx++
			if alias >= n {
				return 0, 0, errors.New("UTF inverse transform failed: invalid data")
			}
			sym := m[alias]
			length := int(sym.length)
			if dstIdx+4 <= len(dst) {
				copy(dst[dstIdx:dstIdx+4], sym.value[:])
			} else if dstIdx+length <= len(dst) {
				copy(dst[dstIdx:dstIdx+length], sym.value[:length])
			} else {
				return 0, 0, errors.New("UTF inverse transform failed: output buffer too small")
			}
			dstIdx += length
		}
	} else {
		pairStride := 2 * maxLen
		for srcEnd-srcIdx >= 4 && len(dst)-dstIdx >= 4 {
			safePairs := min((srcEnd-srcIdx)>>2, (len(dst)-dstIdx-4)/pairStride)
			if safePairs == 0 {
				break
			}
			for p := 0; p < safePairs; p++ {
				w4 := binary.LittleEndian.Uint32(src[srcIdx:])
				if p+1 < safePairs && w4&0x80808080 == 0 {
					for k := 0; k < 4; k++ {
						sym := m[src[srcIdx+k]]
						copy(dst[dstIdx:dstIdx+4], sym.value[:])
						dstIdx += int(sym.length)
					}
					srcIdx += 4
					p++
					continue
				}
				for k := 0; k < 2; k++ {
					alias := int(src[srcIdx])
					srcIdx++
					if alias >= 128 {
						if srcIdx >= srcEnd {
							return 0, 0, errors.New("UTF inverse transform failed: invalid data")
						}
						alias = (int(src[srcIdx]) << 7) + (alias & 0x7F)
						srcIdx++
					}
					if alias >= n {
						return 0, 0, errors.New("UTF inverse transform failed: invalid data")
					}
					sym := m[alias]
					copy(dst[dstIdx:dstIdx+4], sym.value[:])
					dstIdx += int(sym.length)
				}
			}
		}
		for srcIdx < srcEnd {
			alias := int(src[srcIdx])
			srcIdx++
			if alias >= 128 {
				if srcIdx >= srcEnd {
					return 0, 0, errors.New("UTF inverse transform failed: invalid data")
				}
				alias = (int(src[srcIdx]) << 7) + (alias & 0x7F)
				srcIdx++
			}
			if alias >= n {
				return 0, 0, errors.New("UTF inverse transform failed: invalid data")
			}
			sym0 := m[alias]
			len0 := int(sym0.length)
			if srcIdx >= srcEnd {
				if dstIdx+4 <= len(dst) {
					copy(dst[dstIdx:dstIdx+4], sym0.value[:])
				} else if dstIdx+len0 <= len(dst) {
					copy(dst[dstIdx:dstIdx+len0], sym0.value[:len0])
				} else {
					return 0, 0, errors.New("UTF inverse transform failed: output buffer too small")
				}
				dstIdx += len0
				break
			}
			alias = int(src[srcIdx])
			srcIdx++
			if alias >= 128 {
				if srcIdx >= srcEnd {
					return 0, 0, errors.New("UTF inverse transform failed: invalid data")
				}
				alias = (int(src[srcIdx]) << 7) + (alias & 0x7F)
				srcIdx++
			}
			if alias >= n {
				return 0, 0, errors.New("UTF inverse transform failed: invalid data")
			}
			sym1 := m[alias]
			len1 := int(sym1.length)
			if len(dst)-dstIdx >= len0+len1+4 {
				copy(dst[dstIdx:dstIdx+4], sym0.value[:])
				dstIdx += len0
				copy(dst[dstIdx:dstIdx+4], sym1.value[:])
				dstIdx += len1
			} else {
				if len(dst)-dstIdx < len0+len1 {
					return 0, 0, errors.New("UTF inverse transform failed: output buffer too small")
				}
				copy(dst[dstIdx:dstIdx+len0], sym0.value[:len0])
				dstIdx += len0
				copy(dst[dstIdx:dstIdx+len1], sym1.value[:len1])
				dstIdx += len1
			}
		}
	}

	if srcIdx != srcEnd || srcIdx+4-adjust > count || dstIdx+4-adjust > len(dst) {
		return 0, 0, errors.New("UTF inverse transform failed: invalid data")
	}
	copy(dst[dstIdx:dstIdx+4-adjust], src[srcIdx:srcIdx+4-adjust])
	srcIdx += 4 - adjust
	dstIdx += 4 - adjust
	return uint(srcIdx), uint(dstIdx), nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *UTFCodec) MaxEncodedLen(srcLen int) int {
	maxInt := int(^uint(0) >> 1)
	if srcLen > maxInt-8192 {
		return maxInt
	}
	return srcLen + 8192
}

var utfMinB1 = [32]byte{0xA0, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x90, 0x80, 0x80, 0x80, 0x80, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
var utfSpanB1 = [32]byte{0x1F, 0x3F, 0x3F, 0x3F, 0x3F, 0x3F, 0x3F, 0x3F, 0x3F, 0x3F, 0x3F, 0x3F, 0x3F, 0x1F, 0x3F, 0x3F, 0x2F, 0x3F, 0x3F, 0x3F, 0x0F, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func countHighBitBytes(src []byte, start, end int) uint64 {
	var count uint64
	i := start
	for i+8 <= end {
		count += uint64(bits.OnesCount64(binary.LittleEndian.Uint64(src[i:]) & 0x8080808080808080))
		i += 8
	}
	for i < end {
		if src[i]&0x80 != 0 {
			count++
		}
		i++
	}
	return count
}

// A quick partial validation
// A more complete validation is done during processing for the remaining cases
// (rules for 3 and 4 byte sequences)
// Faster than utf8.Valid([]byte), especially at rejecting a block
func validateUTF(block []byte) bool {
	var freqs0 [256]int
	var freqs1 [256][256]int
	count := len(block)
	end4 := count & -4
	prv := byte(0)

	// Unroll loop
	for i := 0; i < end4; i += 4 {
		cur0 := block[i]
		cur1 := block[i+1]
		cur2 := block[i+2]
		cur3 := block[i+3]
		freqs0[cur0]++
		freqs0[cur1]++
		freqs0[cur2]++
		freqs0[cur3]++
		freqs1[prv][cur0]++
		freqs1[cur0][cur1]++
		freqs1[cur1][cur2]++
		freqs1[cur2][cur3]++
		prv = cur3

		if i&0x0FFF == 0 {
			// Early check rules for 1 byte
			sum := freqs0[0xC0] + freqs0[0xC1]

			for _, f := range freqs0[0xF5:] {
				sum += f
			}

			if sum != 0 {
				return false
			}
		}
	}

	if end4 != count {
		for i := end4; i < count; i++ {
			cur := block[i]
			freqs0[cur]++
			freqs1[prv][cur]++
			prv = cur
		}

		// Check rules for 1 byte
		sum := freqs0[0xC0] + freqs0[0xC1]

		for _, f := range freqs0[0xF5:] {
			sum += f
		}

		if sum != 0 {
			return false
		}
	}

	// Valid UTF-8 sequences
	// See Unicode 16 Standard - UTF-8 Table 3.7
	// U+0000..U+007F          00..7F
	// U+0080..U+07FF          C2..DF 80..BF
	// U+0800..U+0FFF          E0 A0..BF 80..BF
	// U+1000..U+CFFF          E1..EC 80..BF 80..BF
	// U+D000..U+D7FF          ED 80..9F 80..BF 80..BF
	// U+E000..U+FFFF          EE..EF 80..BF 80..BF
	// U+10000..U+3FFFF        F0 90..BF 80..BF 80..BF
	// U+40000..U+FFFFF        F1..F3 80..BF 80..BF 80..BF
	// U+100000..U+10FFFF      F4 80..8F 80..BF 80..BF
	sum := 0
	sum2 := 0

	// Check rules for first 2 bytes
	for i := 0; i < 256; i++ {
		// Exclude < 0xE0A0 || > 0xE0BF
		if i < 0xA0 || i > 0xBF {
			sum += freqs1[0xE0][i]
		}

		// Exclude < 0xED80 || > 0xEDE9F
		if i < 0x80 || i > 0x9F {
			sum += freqs1[0xED][i]
		}

		// Exclude < 0xF090 || > 0xF0BF
		if i < 0x90 || i > 0xBF {
			sum += freqs1[0xF0][i]
		}

		// Exclude < 0xF480 || > 0xF48F
		if i < 0x80 || i > 0x8F {
			sum += freqs1[0xF4][i]
		}

		if i < 0x80 || i > 0xBF {
			// Exclude < 0x??80 || > 0x??BF with ?? in [C2..DF]
			for j := 0xC2; j <= 0xDF; j++ {
				sum += freqs1[j][i]
			}

			// Exclude < 0x??80 || > 0x??BF with ?? in [E1..EC]
			for j := 0xE1; j <= 0xEC; j++ {
				sum += freqs1[j][i]
			}

			// Exclude < 0x??80 || > 0x??BF with ?? in [F1..F3]
			sum += freqs1[0xF1][i]
			sum += freqs1[0xF2][i]
			sum += freqs1[0xF3][i]

			// Exclude < 0xEE80 || > 0xEEBF
			sum += freqs1[0xEE][i]

			// Exclude < 0xEF80 || > 0xEFBF
			sum += freqs1[0xEF][i]
		} else {
			// Count non-primary bytes
			sum2 += freqs0[i]
		}

		if sum != 0 {
			return false
		}
	}

	// Ad-hoc threshold
	return sum2 >= (count / 8)
}

func packUTF(in []byte, out *uint32) int {
	s := int(_UTF_SIZES[in[0]])

	switch s {
	case 1:
		*out = uint32(in[0])

	case 2:
		*out = (1 << 19) | (uint32(in[0]) << 8) | uint32(in[1])
		s = 2

	case 3:
		*out = (2 << 19) | ((uint32(in[0]) & 0x0F) << 12) | ((uint32(in[1]) & 0x3F) << 6) | (uint32(in[2]) & 0x3F)
		s = 3

	case 4:
		*out = (4 << 19) | ((uint32(in[0]) & 0x07) << 18) | ((uint32(in[1]) & 0x3F) << 12) | ((uint32(in[2]) & 0x3F) << 6) | (uint32(in[3]) & 0x3F)
		s = 4

	default:
		*out = 0
		s = 0 // signal invalid value
	}

	return s
}

func unpackUTF0(in uint32, out []byte) int {
	s := int(in>>21) + 1

	switch s {
	case 1:
		out[0] = byte(in)

	case 2:
		out[0] = byte(in >> 8)
		out[1] = byte(in)

	case 3:
		out[0] = byte(((in >> 12) & 0x0F) | 0xE0)
		out[1] = byte(((in >> 6) & 0x3F) | 0x80)
		out[2] = byte((in & 0x3F) | 0x80)

	case 4:
		out[0] = byte(((in >> 18) & 0x07) | 0xF0)
		out[1] = byte(((in >> 12) & 0x3F) | 0x80)
		out[2] = byte(((in >> 6) & 0x3F) | 0x80)
		out[3] = byte((in & 0x3F) | 0x80)

	default:
		s = 0 // signal invalid value
	}

	return s
}

// Since Kanzi 2.2 (bitstream v4)
func unpackUTF1(in uint32, out []byte) int {
	var s int
	sz := in >> 19

	switch {
	case sz == 0:
		out[0] = byte(in)
		s = 1

	case sz == 1:
		out[0] = byte(in >> 8)
		out[1] = byte(in)
		s = 2

	case sz == 2:
		out[0] = byte(((in >> 12) & 0x0F) | 0xE0)
		out[1] = byte(((in >> 6) & 0x3F) | 0x80)
		out[2] = byte((in & 0x3F) | 0x80)
		s = 3

	case sz >= 4 && sz <= 7:
		out[0] = byte(((in >> 18) & 0x07) | 0xF0)
		out[1] = byte(((in >> 12) & 0x3F) | 0x80)
		out[2] = byte(((in >> 6) & 0x3F) | 0x80)
		out[3] = byte((in & 0x3F) | 0x80)
		s = 4

	default:
		s = 0 // signal invalid value
	}

	return s
}
