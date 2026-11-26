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
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"slices"

	internal "github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_ALIAS_MIN_BLOCKSIZE = 1024
)

// aliasMinHeap tracks the top n0 (< 240) digrams using packed uint64 keys
// (freq << 16) | val so items[0] is always the weakest retained entry.
type aliasMinHeap struct {
	items  [256]uint64
	size   int
	cap    int
	minKey uint64
}

func (h *aliasMinHeap) siftDown(i int) {
	for {
		left := 2*i + 1
		if left >= h.size {
			break
		}
		smallest := left
		if right := left + 1; right < h.size && h.items[right] < h.items[left] {
			smallest = right
		}
		if h.items[i] <= h.items[smallest] {
			break
		}
		h.items[i], h.items[smallest] = h.items[smallest], h.items[i]
		i = smallest
	}
}

func (h *aliasMinHeap) push(val, freq int) {
	key := (uint64(freq) << 16) | uint64(val)

	if h.size < h.cap {
		i := h.size
		h.items[i] = key
		h.size++
		for i > 0 {
			p := (i - 1) >> 1
			if h.items[p] <= h.items[i] {
				break
			}
			h.items[p], h.items[i] = h.items[i], h.items[p]
			i = p
		}
		h.minKey = h.items[0]
	} else if key > h.minKey {
		h.items[0] = key
		h.siftDown(0)
		h.minKey = h.items[0]
	}
}

// Two nonempty byte slices overlap if either slice starts inside the other.
func aliasBuffersOverlap(src, dst []byte) bool {
	srcStart := reflect.ValueOf(&src[0]).Pointer()
	dstStart := reflect.ValueOf(&dst[0]).Pointer()

	if srcStart <= dstStart {
		return dstStart-srcStart < uintptr(len(src))
	}

	return srcStart-dstStart < uintptr(len(dst))
}

// encodeDigram8 encodes src in 8 parallel streams to break the serial s += (1 or 2)
// dependency chain. At split m, chunk k-1 ends at m or m+1; advancing from both
// until they meet at conv[k] syncs chunk k cleanly (or returns -1 if not met in
// 64 steps). Staging chunk k at dst[dstIdx+conv[k]:] is safe because output <= input.
func encodeDigram8(src, dst []byte, dstIdx, count int, map16 *[65536]int16) int {
	srcEnd := count - 1
	step := srcEnd >> 3
	var end [8]int
	var conv [8]int
	conv[0] = 0
	src = src[:count:count]
	_ = dst[dstIdx+count-1]

	// Chunk k - 1 stops at or just past split, so chunk k can safely start
	// wherever parses from split and split + 1 first meet.
	for k := 1; k < 8; k++ {
		split := k * step
		end[k-1] = split
		pos0 := split
		pos1 := split + 1

		for steps := 64; pos0 != pos1 && steps > 0 && pos0 < srcEnd && pos1 < srcEnd; steps-- {
			if pos0 < pos1 {
				pos0 += int(map16[binary.LittleEndian.Uint16(src[pos0:])] >> 8)
			} else {
				pos1 += int(map16[binary.LittleEndian.Uint16(src[pos1:])] >> 8)
			}
		}

		if pos0 != pos1 || pos0 >= srcEnd {
			return -1
		}

		conv[k] = pos0
	}

	end[7] = srcEnd

	s0, s1, s2, s3 := 0, conv[1], conv[2], conv[3]
	s4, s5, s6, s7 := conv[4], conv[5], conv[6], conv[7]
	d0, d1, d2, d3 := dstIdx, dstIdx+conv[1], dstIdx+conv[2], dstIdx+conv[3]
	d4, d5, d6, d7 := dstIdx+conv[4], dstIdx+conv[5], dstIdx+conv[6], dstIdx+conv[7]
	e0, e1, e2, e3 := end[0], end[1], end[2], end[3]
	e4, e5, e6, e7 := end[4], end[5], end[6], end[7]

	// Run all 8 streams in parallel while every chunk has input
	for s0 < e0 && s1 < e1 && s2 < e2 && s3 < e3 && s4 < e4 && s5 < e5 && s6 < e6 && s7 < e7 {
		a0 := map16[binary.LittleEndian.Uint16(src[s0:])]
		dst[d0] = byte(a0)
		d0++
		s0 += int(a0 >> 8)
		a1 := map16[binary.LittleEndian.Uint16(src[s1:])]
		dst[d1] = byte(a1)
		d1++
		s1 += int(a1 >> 8)
		a2 := map16[binary.LittleEndian.Uint16(src[s2:])]
		dst[d2] = byte(a2)
		d2++
		s2 += int(a2 >> 8)
		a3 := map16[binary.LittleEndian.Uint16(src[s3:])]
		dst[d3] = byte(a3)
		d3++
		s3 += int(a3 >> 8)
		a4 := map16[binary.LittleEndian.Uint16(src[s4:])]
		dst[d4] = byte(a4)
		d4++
		s4 += int(a4 >> 8)
		a5 := map16[binary.LittleEndian.Uint16(src[s5:])]
		dst[d5] = byte(a5)
		d5++
		s5 += int(a5 >> 8)
		a6 := map16[binary.LittleEndian.Uint16(src[s6:])]
		dst[d6] = byte(a6)
		d6++
		s6 += int(a6 >> 8)
		a7 := map16[binary.LittleEndian.Uint16(src[s7:])]
		dst[d7] = byte(a7)
		d7++
		s7 += int(a7 >> 8)
	}

	s := [8]int{s0, s1, s2, s3, s4, s5, s6, s7}
	d := [8]int{d0, d1, d2, d3, d4, d5, d6, d7}

	// Drain the leftover tail of each chunk
	for k := 0; k < 8; k++ {
		sk, dk, ek := s[k], d[k], end[k]
		for sk < ek {
			a := map16[binary.LittleEndian.Uint16(src[sk:])]
			dst[dk] = byte(a)
			dk++
			sk += int(a >> 8)
		}
		s[k], d[k] = sk, dk
	}

	// Stitch chunks together: encode the short bridge [s[k-1], conv[k]) and shift chunk k down
	curDst := d[0]
	for k := 1; k < 8; k++ {
		pos := s[k-1]
		cEnd := conv[k]
		for pos < cEnd {
			a := map16[binary.LittleEndian.Uint16(src[pos:])]
			dst[curDst] = byte(a)
			curDst++
			pos += int(a >> 8)
		}
		ds := dstIdx + conv[k]
		n := d[k] - ds
		copy(dst[curDst:curDst+n], dst[ds:d[k]])
		curDst += n
	}

	if s[7] != count {
		dst[1] = 1
		dst[curDst] = src[s[7]]
		curDst++
	}

	return curDst
}

// AliasCodec is a simple codec replacing 2-byte symbols with 1-byte aliases whenever possible
type AliasCodec struct {
	ctx     *map[string]any
	onlyDNA bool
}

// NewAliasCodec creates a new instance of AliasCodec
func NewAliasCodec() (*AliasCodec, error) {
	this := &AliasCodec{}
	this.onlyDNA = false
	return this, nil
}

// NewAliasCodecWithCtx creates a new instance of AliasCodec using a
// configuration map as parameter.
func NewAliasCodecWithCtx(ctx *map[string]any) (*AliasCodec, error) {
	this := &AliasCodec{}
	this.ctx = ctx
	this.onlyDNA = false

	if ctx != nil {
		if val, containsKey := (*this.ctx)["packOnlyDNA"]; containsKey {
			onlyDNA, ok := val.(bool)

			if ok == false {
				return nil, errors.New("Alias codec: invalid onlyDNA type")
			}

			this.onlyDNA = onlyDNA
		}
	}

	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *AliasCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	if len(src) < _ALIAS_MIN_BLOCKSIZE {
		return 0, 0, fmt.Errorf("Input block is too small - size: %d, required %d", len(src), _ALIAS_MIN_BLOCKSIZE)
	}

	dt := internal.DT_UNDEFINED

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			var ok bool
			dt, ok = val.(internal.DataType)

			if ok == false {
				return 0, 0, errors.New("Alias codec forward transform failed: invalid data type")
			}
		}

		if (dt == internal.DT_MULTIMEDIA) || (dt == internal.DT_UTF8) {
			return 0, 0, errors.New("Alias Codec: forward transform skip, binary data")
		}

		if (dt == internal.DT_EXE) || (dt == internal.DT_BIN) {
			return 0, 0, errors.New("Alias Codec: forward transform skip, binary data")
		}

		if (this.onlyDNA == true) && (dt != internal.DT_UNDEFINED) && (dt != internal.DT_DNA) {
			return 0, 0, errors.New("DNA Alias Codec: forward transform skip, not DNA data")
		}
	}

	// Find missing 1-byte symbols
	var freqs0 [256]int
	internal.ComputeHistogram(src[:], freqs0[:], true, false)
	n0 := 0
	var absent [256]int
	var present [256]int
	nPres := 0

	for i := range &freqs0 {
		if freqs0[i] == 0 {
			absent[n0] = i
			n0++
		} else {
			present[nPres] = i
			nPres++
		}
	}

	if n0 < 16 {
		return 0, 0, errors.New("Alias Codec: forward transform skip, not enough free slots")
	}

	if dt == internal.DT_UNDEFINED {
		dt = internal.DetectSimpleType(len(src), freqs0[:])

		if (this.ctx != nil) && (dt != internal.DT_UNDEFINED) {
			(*this.ctx)["dataType"] = dt
		}

		if (dt != internal.DT_DNA) && (this.onlyDNA == true) {
			return 0, 0, errors.New("DNA Alias Codec: forward transform skip, not DNA data")
		}
	}

	var srcIdx int
	var dstIdx int
	count := len(src)

	if n0 >= 240 {
		// Small alphabet => pack bits
		dst[0] = byte(n0)

		if n0 == 255 {
			// One symbol
			dst[1] = src[0]
			binary.LittleEndian.PutUint32(dst[2:], uint32(count))
			srcIdx = count
			dstIdx = 6
		} else {
			var map8 [256]byte
			srcIdx = 0
			dstIdx = 1
			j := 0

			for i := range &freqs0 {
				if freqs0[i] != 0 {
					dst[dstIdx] = byte(i)
					dstIdx++
					map8[i] = byte(j)
					j++
				}
			}

			if n0 >= 252 {
				// 4 symbols or less
				c3 := count & 3
				dst[dstIdx] = byte(c3)
				dstIdx++
				copy(dst[dstIdx:], src[srcIdx:srcIdx+c3])
				srcIdx += c3
				dstIdx += c3

				end16 := count - 15
				for srcIdx < end16 {
					b0 := uint32((map8[int(src[srcIdx+0])] << 6) | (map8[int(src[srcIdx+1])] << 4) |
						(map8[int(src[srcIdx+2])] << 2) | map8[int(src[srcIdx+3])])
					b1 := uint32((map8[int(src[srcIdx+4])] << 6) | (map8[int(src[srcIdx+5])] << 4) |
						(map8[int(src[srcIdx+6])] << 2) | map8[int(src[srcIdx+7])])
					b2 := uint32((map8[int(src[srcIdx+8])] << 6) | (map8[int(src[srcIdx+9])] << 4) |
						(map8[int(src[srcIdx+10])] << 2) | map8[int(src[srcIdx+11])])
					b3 := uint32((map8[int(src[srcIdx+12])] << 6) | (map8[int(src[srcIdx+13])] << 4) |
						(map8[int(src[srcIdx+14])] << 2) | map8[int(src[srcIdx+15])])
					binary.LittleEndian.PutUint32(dst[dstIdx:], b0|(b1<<8)|(b2<<16)|(b3<<24))
					srcIdx += 16
					dstIdx += 4
				}

				for srcIdx < count {
					dst[dstIdx] = (map8[int(src[srcIdx+0])] << 6) | (map8[int(src[srcIdx+1])] << 4) |
						(map8[int(src[srcIdx+2])] << 2) | map8[int(src[srcIdx+3])]
					srcIdx += 4
					dstIdx++
				}
			} else {
				// 16 symbols or less
				dst[dstIdx] = byte(count & 1)
				dstIdx++

				if (count & 1) != 0 {
					dst[dstIdx] = src[srcIdx]
					srcIdx++
					dstIdx++
				}

				end8 := count - 7
				for srcIdx < end8 {
					b0 := uint32((map8[int(src[srcIdx+0])] << 4) | map8[int(src[srcIdx+1])])
					b1 := uint32((map8[int(src[srcIdx+2])] << 4) | map8[int(src[srcIdx+3])])
					b2 := uint32((map8[int(src[srcIdx+4])] << 4) | map8[int(src[srcIdx+5])])
					b3 := uint32((map8[int(src[srcIdx+6])] << 4) | map8[int(src[srcIdx+7])])
					binary.LittleEndian.PutUint32(dst[dstIdx:], b0|(b1<<8)|(b2<<16)|(b3<<24))
					srcIdx += 8
					dstIdx += 4
				}

				for srcIdx < count {
					dst[dstIdx] = (map8[int(src[srcIdx])] << 4) | map8[int(src[srcIdx+1])]
					srcIdx += 2
					dstIdx++
				}
			}
		}
	} else {
		// Digram encoding
		h := aliasMinHeap{cap: n0}

		{
			var freqs1 [65536]int
			internal.ComputeHistogram(src[:], freqs1[:], false, false)

			// Scan present symbol pairs to keep only the top n0 most frequent digrams
			for hiIdx := 0; hiIdx < nPres; hiIdx++ {
				hi := present[hiIdx]
				base := hi << 8
				row := freqs1[base : base+256]

				for loIdx := 0; loIdx < nPres; loIdx++ {
					lo := present[loIdx]
					if row[lo] != 0 {
						h.push(base|lo, row[lo])
					}
				}
			}

			if freqs0[0] == 0 && freqs1[int(src[0])] != 0 {
				h.push(int(src[0]), freqs1[int(src[0])])
			}
		}

		if h.size < n0 {
			// Fewer distinct 2-byte symbols than 1-byte symbols
			n0 = h.size

			if n0 < 16 {
				return 0, 0, errors.New("Alias Codec: forward transform skip, not enough free slots")
			}
		}

		savings := 0
		for i := 0; i < n0; i++ {
			savings += int(h.items[i] >> 16)
		}

		// Worth it ?
		if savings < count/20 {
			return 0, 0, errors.New("Alias Codec: forward transform skip, not enough savings")
		}

		// Order top digrams descending so the most frequent get the first aliases
		slices.SortFunc(h.items[:n0], func(a, b uint64) int {
			return cmp.Compare(b, a)
		})

		// Build branchless symbol -> alias lookup (upper byte holds src advance: 1 or 2)
		var map16 [65536]int16
		var row0 [256]int16
		for j := 0; j < 256; j++ {
			row0[j] = int16(0x100 | j)
		}
		for j := 0; j < nPres; j++ {
			base := present[j] << 8
			copy(map16[base:base+256], row0[:])
		}

		dst[0] = byte(n0)
		dst[1] = 0
		srcIdx = 0
		dstIdx = 2

		// Header: emit map data
		for i := 0; i < n0; i++ {
			idx := int(h.items[i] & 0xFFFF)
			hi := idx >> 8
			lo := idx & 0xFF
			map16[(lo<<8)|hi] = int16(0x200 | absent[i])
			dst[dstIdx] = byte(hi)
			dst[dstIdx+1] = byte(lo)
			dst[dstIdx+2] = byte(absent[i])
			dstIdx += 3
		}

		encodedLen := -1
		// The staged encoder writes ahead of the current output position.
		if count >= 4096 && !aliasBuffersOverlap(src, dst[:this.MaxEncodedLen(count)]) {
			encodedLen = encodeDigram8(src, dst, dstIdx, count, &map16)
		}

		if encodedLen >= 0 {
			dstIdx = encodedLen
			srcIdx = count
		} else {
			srcEnd := count - 1

			for srcIdx < srcEnd {
				alias := map16[binary.LittleEndian.Uint16(src[srcIdx:])]
				dst[dstIdx] = byte(alias)
				srcIdx += int(alias >> 8)
				dstIdx++
			}

			if srcIdx != count {
				dst[1] = 1
				dst[dstIdx] = src[srcIdx]
				srcIdx++
				dstIdx++
			}
		}
	}

	if dstIdx >= count {
		return 0, 0, errors.New("Alias Codec: forward transform skip, not enough savings")
	}

	return uint(srcIdx), uint(dstIdx), nil
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *AliasCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	if len(src) < 2 {
		return 0, 0, fmt.Errorf("Input block is too small - size: %d, required %d", len(src), 2)
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	n := int(src[0])

	if n < 16 {
		return 0, 0, errors.New("Alias codec inverse transform failed: invalid data (incorrect number of slots)")
	}

	var srcIdx int
	dstIdx := 0

	if n >= 240 {
		srcEnd := len(src)
		n = 256 - n
		srcIdx = 1

		if n == 1 {
			// One symbol
			if len(src) < 6 {
				return 0, 0, errors.New("Alias codec inverse transform failed: truncated header")
			}

			val := src[1]
			oSize := int(binary.LittleEndian.Uint32(src[2:]))

			if oSize > len(dst) {
				return 0, 0, errors.New("Alias codec inverse transform failed: invalid data (incorrect output size)")
			}

			for i := range dst[0:oSize] {
				dst[i] = val
			}

			srcIdx = srcEnd
			dstIdx = oSize
		} else {
			// Rebuild map alias -> symbol
			if len(src) < srcIdx+n+1 {
				return 0, 0, errors.New("Alias codec inverse transform failed: truncated header")
			}

			var idx2symb [16]byte

			for i := 0; i < n; i++ {
				idx2symb[i] = src[srcIdx]
				srcIdx++
			}

			adjust := int(src[srcIdx])
			srcIdx++

			if adjust < 0 || adjust > 3 {
				return 0, 0, errors.New("Alias codec inverse transform failed: invalid data")
			}

			if n <= 4 {
				if adjust > srcEnd-srcIdx || adjust > len(dst)-dstIdx || srcEnd-srcIdx-adjust > (len(dst)-dstIdx-adjust)/4 {
					return 0, 0, errors.New("Alias codec inverse transform failed: invalid data")
				}

				// 4 symbols or less
				var decodeMap [256]uint32

				for i := 0; i < 256; i++ {
					var val uint32
					val = uint32(idx2symb[(i>>0)&0x03])
					val <<= 8
					val |= uint32(idx2symb[(i>>2)&0x03])
					val <<= 8
					val |= uint32(idx2symb[(i>>4)&0x03])
					val <<= 8
					val |= uint32(idx2symb[(i>>6)&0x03])
					decodeMap[i] = val
				}

				copy(dst[dstIdx:], src[srcIdx:srcIdx+adjust])
				srcIdx += adjust
				dstIdx += adjust

				srcEnd2 := srcEnd - 1
				for srcIdx < srcEnd2 {
					q := uint64(decodeMap[int(src[srcIdx+0])]) | (uint64(decodeMap[int(src[srcIdx+1])]) << 32)
					binary.LittleEndian.PutUint64(dst[dstIdx:], q)
					srcIdx += 2
					dstIdx += 8
				}

				if srcIdx < srcEnd {
					binary.LittleEndian.PutUint32(dst[dstIdx:], decodeMap[int(src[srcIdx])])
					srcIdx++
					dstIdx += 4
				}
			} else {
				if adjust > srcEnd-srcIdx || adjust > len(dst)-dstIdx || srcEnd-srcIdx-adjust > (len(dst)-dstIdx-adjust)/2 {
					return 0, 0, errors.New("Alias codec inverse transform failed: invalid data")
				}

				// 16 symbols or less
				var decodeMap [256]uint16

				for i := 0; i < 256; i++ {
					val := uint16(idx2symb[i&0x0F])
					val <<= 8
					val |= uint16(idx2symb[i>>4])
					decodeMap[i] = val
				}

				if adjust != 0 {
					dst[dstIdx] = src[srcIdx]
					srcIdx++
					dstIdx++
				}

				srcEnd4 := srcEnd - 3
				for srcIdx < srcEnd4 {
					q := uint64(decodeMap[int(src[srcIdx+0])]) |
						(uint64(decodeMap[int(src[srcIdx+1])]) << 16) |
						(uint64(decodeMap[int(src[srcIdx+2])]) << 32) |
						(uint64(decodeMap[int(src[srcIdx+3])]) << 48)
					binary.LittleEndian.PutUint64(dst[dstIdx:], q)
					srcIdx += 4
					dstIdx += 8
				}

				for srcIdx < srcEnd {
					val := decodeMap[int(src[srcIdx])]
					srcIdx++
					binary.LittleEndian.PutUint16(dst[dstIdx:], val)
					dstIdx += 2
				}
			}
		}
	} else {
		// Rebuild map alias -> symbol
		var map16 [256]int
		adjust := int(src[1])

		if adjust > 1 || len(src) < 2+3*n+adjust {
			return 0, 0, errors.New("Alias codec inverse transform failed: truncated header")
		}

		srcEnd := len(src) - adjust
		srcIdx = 2

		for i := range &map16 {
			map16[i] = 0x10000 | int(i)
		}

		for i := 0; i < n; i++ {
			map16[int(src[srcIdx+2])] = 0x20000 | int(src[srcIdx]) | (int(src[srcIdx+1]) << 8)
			srcIdx += 3
		}

		nbAliases := srcEnd - srcIdx
		dstAvail := len(dst) - dstIdx

		if nbAliases <= (dstAvail >> 1) {
			// Safe to prefix-sum and write 16 bits unconditionally for each 8-symbol batch
			srcEnd8 := srcEnd - 7
			for srcIdx < srcEnd8 {
				v0 := map16[int(src[srcIdx+0])]
				v1 := map16[int(src[srcIdx+1])]
				v2 := map16[int(src[srcIdx+2])]
				v3 := map16[int(src[srcIdx+3])]
				v4 := map16[int(src[srcIdx+4])]
				v5 := map16[int(src[srcIdx+5])]
				v6 := map16[int(src[srcIdx+6])]
				v7 := map16[int(src[srcIdx+7])]
				o1 := v0 >> 16
				o2 := o1 + (v1 >> 16)
				o3 := o2 + (v2 >> 16)
				o4 := o3 + (v3 >> 16)
				o5 := o4 + (v4 >> 16)
				o6 := o5 + (v5 >> 16)
				o7 := o6 + (v6 >> 16)
				binary.LittleEndian.PutUint16(dst[dstIdx:], uint16(v0))
				binary.LittleEndian.PutUint16(dst[dstIdx+o1:], uint16(v1))
				binary.LittleEndian.PutUint16(dst[dstIdx+o2:], uint16(v2))
				binary.LittleEndian.PutUint16(dst[dstIdx+o3:], uint16(v3))
				binary.LittleEndian.PutUint16(dst[dstIdx+o4:], uint16(v4))
				binary.LittleEndian.PutUint16(dst[dstIdx+o5:], uint16(v5))
				binary.LittleEndian.PutUint16(dst[dstIdx+o6:], uint16(v6))
				binary.LittleEndian.PutUint16(dst[dstIdx+o7:], uint16(v7))
				dstIdx += o7 + (v7 >> 16)
				srcIdx += 8
			}

			for srcIdx < srcEnd {
				val := map16[int(src[srcIdx])]
				srcIdx++
				binary.LittleEndian.PutUint16(dst[dstIdx:], uint16(val))
				dstIdx += (val >> 16)
			}
		} else {
			for srcIdx < srcEnd && dstIdx+1 < len(dst) {
				val := map16[int(src[srcIdx])]
				srcIdx++
				dst[dstIdx] = byte(val)
				dst[dstIdx+1] = byte(val >> 8)
				dstIdx += (val >> 16)
			}

			for srcIdx < srcEnd {
				val := map16[int(src[srcIdx])]
				srcIdx++
				inc := val >> 16

				if dstIdx+inc > len(dst) {
					return 0, 0, errors.New("Alias codec inverse transform failed: invalid data")
				}

				dst[dstIdx+inc-1] = byte(val >> 8)
				dst[dstIdx] = byte(val)
				dstIdx += inc
			}
		}

		if adjust != 0 {
			if dstIdx >= len(dst) {
				return 0, 0, errors.New("Alias codec inverse transform failed: invalid data")
			}

			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++
		}
	}

	return uint(srcIdx), uint(dstIdx), nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *AliasCodec) MaxEncodedLen(srcLen int) int {
	return srcLen + 1024
}
