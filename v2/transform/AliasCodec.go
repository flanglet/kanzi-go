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
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

const (
	_ALIAS_MIN_BLOCKSIZE = 1024
)

type sdAlias struct {
	val  int // symbol
	freq int // frequency
}

type sortAliasByFreq []*sdAlias

func (this sortAliasByFreq) Len() int {
	return len(this)
}

func (this sortAliasByFreq) Less(i, j int) bool {
	if r := this[j].freq - this[i].freq; r != 0 {
		return r < 0
	}

	return this[j].val < this[i].val
}

func (this sortAliasByFreq) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

// AliasCodec is a simple codec replacing 2-byte symbols with 1-byte aliases whenever possible
type AliasCodec struct {
	ctx *map[string]interface{}
}

// NewAliasCodec creates a new instance of AliasCodec
func NewAliasCodec() (*AliasCodec, error) {
	this := &AliasCodec{}
	return this, nil
}

// NewAliasCodecWithCtx creates a new instance of AliasCodec using a
// configuration map as parameter.
func NewAliasCodecWithCtx(ctx *map[string]interface{}) (*AliasCodec, error) {
	this := &AliasCodec{}
	this.ctx = ctx
	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *AliasCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
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

	if this.ctx != nil {
		dt := kanzi.DT_UNDEFINED

		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt = val.(kanzi.DataType)
		}

		if (dt == kanzi.DT_MULTIMEDIA) || (dt == kanzi.DT_UTF8) {
			return 0, 0, errors.New("Alias Codec: forward transform skip, binary data")
		}

		if (dt == kanzi.DT_EXE) || (dt == kanzi.DT_BIN) {
			return 0, 0, errors.New("Alias Codec: forward transform skip, binary data")
		}
	}

	// Find missing 1-byte symbols
	var freqs0 [256]int
	kanzi.ComputeHistogram(src[:], freqs0[:], true, false)
	n0 := 0
	var absent [256]int

	for i := range &freqs0 {
		if freqs0[i] == 0 {
			absent[n0] = i
			n0++
		}
	}

	if n0 < 16 {
		return 0, 0, errors.New("Alias Codec: forward transform skip, not enough free slots")
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

			for i := range freqs0 {
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

				for srcIdx < count {
					dst[dstIdx] = (map8[int(src[srcIdx])] << 4) | map8[int(src[srcIdx+1])]
					srcIdx += 2
					dstIdx++
				}
			}
		}
	} else {
		// Digram encoding
		symb := [65536]*sdAlias{}
		n1 := 0

		{
			var freqs1 [65536]int
			kanzi.ComputeHistogram(src[:], freqs1[:], false, false)

			for i := range &freqs1 {
				if freqs1[i] == 0 {
					continue
				}

				symb[n1] = &sdAlias{val: i, freq: freqs1[i]}
				n1++
			}
		}

		if n0 > n1 {
			// Fewer distinct 2-byte symbols than 1-byte symbols
			n0 = n1

			if n0 < 16 {
				return 0, 0, errors.New("Alias Codec: forward transform skip, not enough free slots")
			}
		}

		// Sort by decreasing order 1 frequencies
		sort.Sort(sortAliasByFreq(symb[0:n1]))
		var map16 [65536]int16

		// Build map symbol -> alias
		for i := range &map16 {
			map16[i] = int16(0x100 | (i >> 8))
		}

		savings := 0
		dst[0] = byte(n0)
		srcIdx = 0
		dstIdx = 1

		// Header: emit map length then map data
		for i := 0; i < n0; i++ {
			savings += symb[i].freq // ignore factor 2
			idx := symb[i].val
			map16[idx] = int16(0x200 | absent[i])
			dst[dstIdx] = byte(idx >> 8)
			dst[dstIdx+1] = byte(idx)
			dst[dstIdx+2] = byte(absent[i])
			dstIdx += 3
		}

		// Worth it ?
		if savings*20 < count {
			return 0, 0, errors.New("Alias Codec: forward transform skip, not enough savings")
		}

		srcEnd := count - 1

		// Emit aliased data
		for srcIdx < srcEnd {
			alias := map16[(int(src[srcIdx])<<8)|int(src[srcIdx+1])]
			dst[dstIdx] = byte(alias)
			srcIdx += int(alias >> 8)
			dstIdx++
		}

		if srcIdx != count {
			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++
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
	if len(src) == 0 {
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
		return 0, 0, errors.New("Alias codec: invalid data (incorrect number of slots)")
	}

	var srcIdx int
	dstIdx := 0
	count := len(src)

	if n >= 240 {
		n = 256 - n
		srcIdx = 1

		if n == 1 {
			// One symbol
			val := src[1]
			oSize := int(binary.LittleEndian.Uint32(src[2:]))

			if oSize > len(dst) {
				return 0, 0, errors.New("Alias codec: invalid data (incorrect output size)")
			}

			for i := range dst[0:oSize] {
				dst[i] = val
			}

			srcIdx = count
			dstIdx = oSize
		} else {
			// Rebuild map alias -> symbol
			var idx2symb [16]byte

			for i := 0; i < n; i++ {
				idx2symb[i] = src[srcIdx]
				srcIdx++
			}

			adjust := int(src[srcIdx])
			srcIdx++

			if adjust < 0 || adjust > 3 {
				return 0, 0, errors.New("Alias codec: invalid data")
			}

			if n <= 4 {
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

				for srcIdx < count {
					binary.LittleEndian.PutUint32(dst[dstIdx:], decodeMap[int(src[srcIdx])])
					srcIdx++
					dstIdx += 4
				}
			} else {
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

				for srcIdx < count {
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
		srcIdx = 1

		for i := range &map16 {
			map16[i] = 0x10000 | int(i)
		}

		for i := 0; i < n; i++ {
			map16[int(src[srcIdx+2])] = 0x20000 | int(src[srcIdx]) | (int(src[srcIdx+1]) << 8)
			srcIdx += 3
		}

		srcEnd := len(src)

		for srcIdx < srcEnd {
			val := map16[int(src[srcIdx])]
			srcIdx++
			dst[dstIdx] = byte(val)
			dst[dstIdx+1] = byte(val >> 8)
			dstIdx += (val >> 16)
		}
	}

	return uint(srcIdx), uint(dstIdx), nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this AliasCodec) MaxEncodedLen(srcLen int) int {
	return srcLen + 1024
}
