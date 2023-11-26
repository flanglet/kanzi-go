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

import (
	"errors"
	"fmt"
	"sort"

	internal "github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_UTF_MIN_BLOCKSIZE = 1024
)

var (
	_UTF_SIZES = []int{1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 2, 2, 3, 4}
)

type sdUTF struct {
	sym  int32 // symbol
	freq int32 // frequency
}

type sortUTFByFreq []*sdUTF

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

	// First (possibly) invalid symbols (due to block truncation)
	for (start < 4) && (_UTF_SIZES[src[start]>>4] == 0) {
		start++
	}

	if (mustValidate == true) && (validateUTF(src[start:count-4]) == false) {
		return 0, 0, errors.New("UTF forward transform skip: not UTF")
	}

	// 1-3 bit size + (7 or 11 or 16 or 21) bit payload
	// 3 MSBs indicate symbol size (limit map size to 22 bits)
	// 000 -> 7 bits
	// 001 -> 11 bits
	// 010 -> 16 bits
	// 1xx -> 21 bits
	aliasMap := make([]int32, 1<<22)
	symb := [32768]*sdUTF{}
	n := 0
	var err error

	for i := start; i < count-4; {
		var val uint32
		s := packUTF(src[i:], &val)

		if s == 0 {
			err = errors.New("UTF forward transform skip: invalid UTF")
			break
		}

		if aliasMap[val] == 0 {
			symb[n] = &sdUTF{sym: int32(val)}
			n++

			if n >= 32768 {
				err = errors.New("UTF forward transform skip: too many symbols")
				break
			}
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

	dstEnd := count - (count / 10)

	if (3*n + 6) >= dstEnd {
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

	if estimate >= dstEnd {
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

	if dstIdx >= dstEnd {
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
	start := int(src[0])
	adjust := int(src[1]) // adjust end of regular processing
	n := (int(src[2]) << 8) + int(src[3])

	// Protect against invalid map size value
	if (n >= 32768) || (3*n >= count) {
		return 0, 0, errors.New("UTF inverse transform skip: invalid data")
	}

	bsVersion := uint(4)

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["bsVersion"]; containsKey {
			bsVersion = val.(uint)
		}
	}

	isBsVersion3 := bsVersion < 4
	m := [32768]utfSymbol{}
	srcIdx := 4

	// Build inverse mapping
	for i := 0; i < n; i++ {
		s := (uint32(src[srcIdx]) << 16) | (uint32(src[srcIdx+1]) << 8) | uint32(src[srcIdx+2])

		if isBsVersion3 == true {
			sl := unpackUTF0(s, m[i].value[:])

			if sl == 0 {
				return 0, 0, errors.New("UTF inverse transform skip: invalid data")
			}

			m[i].length = uint8(sl)
		} else {
			sl := unpackUTF1(s, m[i].value[:])

			if sl == 0 {
				return 0, 0, errors.New("UTF inverse transform skip: invalid data")
			}

			m[i].length = uint8(sl)
		}

		srcIdx += 3
	}

	dstIdx := 0
	srcEnd := count - 4 + adjust

	for i := 0; i < start; i++ {
		dst[dstIdx] = src[srcIdx]
		srcIdx++
		dstIdx++
	}

	// Emit data
	for srcIdx < srcEnd {
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

	for i := srcEnd; i < count; i++ {
		dst[dstIdx] = src[srcIdx]
		srcIdx++
		dstIdx++
	}

	return uint(srcIdx), uint(dstIdx), nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this UTFCodec) MaxEncodedLen(srcLen int) int {
	return srcLen + 8192
}

func validateUTF(block []byte) bool {
	var freqs0 [256]int
	var freqs [256][256]int
	freqs1 := freqs[0:256]
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
	}

	for i := end4; i < count; i++ {
		cur := block[i]
		freqs0[cur]++
		freqs1[prv][cur]++
		prv = cur
	}

	// Check UTF-8
	// See Unicode 14 Standard - UTF-8 Table 3.7
	// U+0000..U+007F          00..7F
	// U+0080..U+07FF          C2..DF 80..BF
	// U+0800..U+0FFF          E0 A0..BF 80..BF
	// U+1000..U+CFFF          E1..EC 80..BF 80..BF
	// U+D000..U+D7FF          ED 80..9F 80..BF 80..BF
	// U+E000..U+FFFF          EE..EF 80..BF 80..BF
	// U+10000..U+3FFFF        F0 90..BF 80..BF 80..BF
	// U+40000..U+FFFFF        F1..F3 80..BF 80..BF 80..BF
	// U+100000..U+10FFFF      F4 80..8F 80..BF 80..BF
	if freqs0[0xC0] > 0 || freqs0[0xC1] > 0 {
		return false
	}

	for i := 0xF5; i <= 0xFF; i++ {
		if freqs0[i] > 0 {
			return false
		}
	}

	sum := 0

	for i := 0; i < 256; i++ {
		// Exclude < 0xE0A0 || > 0xE0BF
		if (i < 0xA0 || i > 0xBF) && (freqs[0xE0][i] > 0) {
			return false
		}

		// Exclude < 0xED80 || > 0xEDE9F
		if (i < 0x80 || i > 0x9F) && (freqs[0xED][i] > 0) {
			return false
		}

		// Exclude < 0xF090 || > 0xF0BF
		if (i < 0x90 || i > 0xBF) && (freqs[0xF0][i] > 0) {
			return false
		}

		// Exclude < 0xF480 || > 0xF4BF
		if (i < 0x80 || i > 0xBF) && (freqs[0xF4][i] > 0) {
			return false
		}

		// Count non-primary bytes
		if i >= 0x80 && i <= 0xBF {
			sum += freqs0[i]
		}
	}

	// Ad-hoc threshold
	return sum >= (count / 4)
}

func packUTF(in []byte, out *uint32) int {
	s := _UTF_SIZES[uint8(in[0])>>4]

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
