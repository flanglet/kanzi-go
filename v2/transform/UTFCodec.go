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
	"sort"

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

type sortUTFByFreq []sdUTF

func (this sortUTFByFreq) Len() int {
	return len(this)
}

func (this sortUTFByFreq) Less(i, j int) bool {
	if r := this[i].freq - this[j].freq; r != 0 {
		return r < 0
	}

	return this[i].sym < this[j].sym
}

func (this sortUTFByFreq) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

type utfSymbol struct {
	value  [4]byte
	length uint8
}

// UTFCodec is a simple one-pass UTF8 codec that replaces code points with indexes.
type UTFCodec struct {
	ctx *map[string]any
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
	if len(src) == 0 {
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
			dt := val.(internal.DataType)

			if dt != internal.DT_UNDEFINED && dt != internal.DT_UTF8 {
				return 0, 0, errors.New("UTF forward transform skip: not UTF")
			}

			mustValidate = dt != internal.DT_UTF8
		}
	}

	start := 0

	if binary.BigEndian.Uint32(src[0:])&0x00FFFFFF == 0x00EFBBBF { // Byte Order Mark (BOM)
		start = 3
	} else {
		// First (possibly) invalid symbols (due to block truncation).
		for (start < 4) && (_UTF_SIZES[src[start]] == 0) {
			start++
		}
	}

	if (mustValidate == true) && (validateUTF(src[start:count-4]) == false) {
		return 0, 0, errors.New("UTF forward transform skip: not UTF")
	}

	if this.ctx != nil {
		(*this.ctx)["dataType"] = internal.DT_UTF8
	}

	// 1-3 bit size + (7 or 11 or 16 or 21) bit payload
	// 3 MSBs indicate symbol size (limit map size to 22 bits)
	// 000 -> 7 bits
	// 001 -> 11 bits
	// 010 -> 16 bits
	// 1xx -> 21 bits
	aliasMap := make([]int32, 1<<22)
	symb := [32768]sdUTF{}
	n := 0
	var err error

	for i := start; i < count-4; {
		var val uint32
		s := packUTF(src[i:], &val)
		res := s != 0
		// Validation of longer sequences
		// Third byte in [0x80..0xBF]
		res = res && ((s != 3) || ((src[i+2] & 0xC0) == 0x80))
		// Third and fourth bytes in [0x80..0xBF]
		res = res && ((s != 4) || ((((uint16(src[i+2]) << 8) | uint16(src[i+3])) & 0xC0C0) == 0x8080))

		if aliasMap[val] == 0 {
			symb[n].sym = int32(val)
			n++
			res = res && (n < 32768)
		}

		if res == false {
			return 0, 0, errors.New("UTF forward transform skip: invalid or too complex")
		}

		aliasMap[val]++
		i += s
	}

	if n == 0 {
		return 0, 0, errors.New("UTF forward transform skip: not UTF")
	}

	if err != nil {
		return 0, 0, err
	}

	maxTarget := count - (count / 10)

	if (3*n + 6) >= maxTarget {
		return 0, 0, errors.New("UTF forward transform skip: no improvement")
	}

	for i := 0; i < n; i++ {
		symb[i].freq = aliasMap[symb[i].sym]
	}

	// Sort ranks by increasing frequencies
	sort.Sort(sortUTFByFreq(symb[0:n]))
	dstIdx := 2

	// Emit map length then map data
	dst[dstIdx] = byte(n >> 8)
	dstIdx++
	dst[dstIdx] = byte(n)
	dstIdx++
	estimate := dstIdx + 6

	for i := 0; i < n; i++ {
		r := n - 1 - i
		s := symb[r].sym

		dst[dstIdx] = byte(s >> 16)
		dst[dstIdx+1] = byte(s >> 8)
		dst[dstIdx+2] = byte(s)
		dstIdx += 3

		if i < 128 {
			estimate += int(symb[r].freq)
			aliasMap[s] = int32(i)
		} else {
			estimate += 2 * int(symb[r].freq)
			aliasMap[s] = 0x10080 | int32((i<<1)&0xFF00) | int32(i&0x7F)
		}
	}

	if estimate >= maxTarget {
		return 0, uint(dstIdx), errors.New("UTF forward transform skip: no improvement")
	}

	// Emit first (possibly) invalid symbols (due to block truncation)
	for i := 0; i < start; {
		dst[dstIdx] = src[i]
		i++
		dstIdx++
	}

	srcIdx := start

	// Emit aliases
	for srcIdx < count-4 {
		var val uint32
		srcIdx += packUTF(src[srcIdx:], &val)
		alias := aliasMap[val]
		dst[dstIdx] = byte(alias)
		dstIdx++
		dst[dstIdx] = byte(alias >> 8)
		dstIdx += int(alias >> 16)
	}

	dst[0] = byte(start)
	dst[1] = byte(srcIdx - (count - 4))

	// Emit last (possibly) invalid symbols (due to block truncation)
	for srcIdx < count {
		dst[dstIdx] = src[srcIdx]
		srcIdx++
		dstIdx++
	}

	if dstIdx >= maxTarget {
		return uint(srcIdx), uint(dstIdx), errors.New("UTF forward transform skip: no improvement")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *UTFCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
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
	adjust := int(src[1]) & 0x03 // adjust end of regular processing
	n := (int(src[2]) << 8) + int(src[3])

	// Protect against invalid map size value
	if (n == 0) || (n >= 32768) || (3*n >= count) {
		return 0, 0, errors.New("UTF inverse transform: invalid map size")
	}

	isBsVersion3 := false

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["bsVersion"]; containsKey {
			bsVersion := val.(uint)
			isBsVersion3 = bsVersion < 4
		}
	}

	m := [32768]utfSymbol{}
	srcIdx := 4

	// Build inverse mapping
	if isBsVersion3 == true {
		for i := 0; i < n; i++ {
			s := (uint32(src[srcIdx]) << 16) | (uint32(src[srcIdx+1]) << 8) | uint32(src[srcIdx+2])

			sl := unpackUTF0(s, m[i].value[:])

			if sl == 0 {
				return 0, 0, errors.New("UTF inverse transform failed: invalid UTF alias")
			}

			m[i].length = uint8(sl)
			srcIdx += 3
		}
	} else {
		for i := 0; i < n; i++ {
			s := (uint32(src[srcIdx]) << 16) | (uint32(src[srcIdx+1]) << 8) | uint32(src[srcIdx+2])

			sl := unpackUTF1(s, m[i].value[:])

			if sl == 0 {
				return 0, 0, errors.New("UTF inverse transform failed: invalid UTF alias")
			}

			m[i].length = uint8(sl)
			srcIdx += 3
		}
	}

	srcEnd := count - 4 + adjust
	dstIdx := 0
	dstEnd := len(dst) - 4

	if dstEnd < 0 {
		return 0, 0, errors.New("UTF inverse transform failed: invalid output block size")
	}

	for i := 0; i < start; i++ {
		dst[dstIdx] = src[srcIdx]
		srcIdx++
		dstIdx++
	}

	// Emit data
	for srcIdx < srcEnd && dstIdx < dstEnd {
		alias := int(src[srcIdx])
		srcIdx++

		if alias >= 128 {
			alias = (int(src[srcIdx]) << 7) + (alias & 0x7F)
			srcIdx++
		}

		s := m[alias]
		copy(dst[dstIdx:], s.value[:4])
		dstIdx += int(s.length)
	}

	var err error

	if srcIdx < srcEnd || dstIdx >= dstEnd-count+srcEnd {
		err = errors.New("UTF inverse transform failed: invalid data")
	} else {
		for i := srcEnd; i < count; i++ {
			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++
		}
	}

	return uint(srcIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *UTFCodec) MaxEncodedLen(srcLen int) int {
	return srcLen + 8192
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
