/*
Copyright 2011-2026 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License")
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

const (
	_FSD_MIN_BLOCK_LENGTH = 1024
	_FSD_ESCAPE_TOKEN     = 0xFF
	_FSD_DELTA_CODING     = byte(0)
	_FSD_XOR_CODING       = byte(1)
)

var _FSD_ZIGZAG2 = [256]int{
	0, -1, 1, -2, 2, -3, 3, -4,
	4, -5, 5, -6, 6, -7, 7, -8,
	8, -9, 9, -10, 10, -11, 11, -12,
	12, -13, 13, -14, 14, -15, 15, -16,
	16, -17, 17, -18, 18, -19, 19, -20,
	20, -21, 21, -22, 22, -23, 23, -24,
	24, -25, 25, -26, 26, -27, 27, -28,
	28, -29, 29, -30, 30, -31, 31, -32,
	32, -33, 33, -34, 34, -35, 35, -36,
	36, -37, 37, -38, 38, -39, 39, -40,
	40, -41, 41, -42, 42, -43, 43, -44,
	44, -45, 45, -46, 46, -47, 47, -48,
	48, -49, 49, -50, 50, -51, 51, -52,
	52, -53, 53, -54, 54, -55, 55, -56,
	56, -57, 57, -58, 58, -59, 59, -60,
	60, -61, 61, -62, 62, -63, 63, -64,
	64, -65, 65, -66, 66, -67, 67, -68,
	68, -69, 69, -70, 70, -71, 71, -72,
	72, -73, 73, -74, 74, -75, 75, -76,
	76, -77, 77, -78, 78, -79, 79, -80,
	80, -81, 81, -82, 82, -83, 83, -84,
	84, -85, 85, -86, 86, -87, 87, -88,
	88, -89, 89, -90, 90, -91, 91, -92,
	92, -93, 93, -94, 94, -95, 95, -96,
	96, -97, 97, -98, 98, -99, 99, -100,
	100, -101, 101, -102, 102, -103, 103, -104,
	104, -105, 105, -106, 106, -107, 107, -108,
	108, -109, 109, -110, 110, -111, 111, -112,
	112, -113, 113, -114, 114, -115, 115, -116,
	116, -117, 117, -118, 118, -119, 119, -120,
	120, -121, 121, -122, 122, -123, 123, -124,
	124, -125, 125, -126, 126, -127, 127, -128,
}

const (
	_FSD_SWAR_MASK_01 uint64 = 0x0101010101010101
	_FSD_SWAR_MASK_7F uint64 = 0x7F7F7F7F7F7F7F7F
	_FSD_SWAR_MASK_80 uint64 = 0x8080808080808080
)

func fsdZigzagByte(cur, prev byte) byte {
	residual := uint32(cur - prev)
	return byte((residual << 1) ^ (0 - (residual >> 7)))
}

func fsdZigzagSwar64(a, b uint64) uint64 {
	residual := ((a | _FSD_SWAR_MASK_80) - (b & _FSD_SWAR_MASK_7F)) ^ ((a ^ ^b) & _FSD_SWAR_MASK_80)
	msb := residual & _FSD_SWAR_MASK_80
	sign := msb | (msb - (msb >> 7))
	return ((residual & _FSD_SWAR_MASK_7F) << 1) ^ sign
}

func fsdUnzigzagSwar64(v uint64) uint64 {
	lsb := v & _FSD_SWAR_MASK_01
	m := lsb << 7
	return ((v >> 1) & _FSD_SWAR_MASK_7F) ^ (m | (m - lsb))
}

func fsdAddBytesSwar64(a, b uint64) uint64 {
	return ((a & _FSD_SWAR_MASK_7F) + (b & _FSD_SWAR_MASK_7F)) ^ ((a ^ b) & _FSD_SWAR_MASK_80)
}

func fsdSampleHistograms(src []byte, count10, count5 int, histo *[7][256]int) {
	for s := 0; s < 3; s++ {
		in := src[s*(2*count5):]
		i := count10
		p16 := binary.LittleEndian.Uint32(in[i-16:])
		p12 := binary.LittleEndian.Uint32(in[i-12:])
		p8 := binary.LittleEndian.Uint32(in[i-8:])
		p4 := binary.LittleEndian.Uint32(in[i-4:])
		for ; i+4 <= count5; i += 4 {
			cur := binary.LittleEndian.Uint32(in[i:])
			x0 := cur
			histo[0][byte(x0>>0)]++
			histo[0][byte(x0>>8)]++
			histo[0][byte(x0>>16)]++
			histo[0][byte(x0>>24)]++
			x1 := cur ^ ((cur << 8) | (p4 >> 24))
			histo[1][byte(x1>>0)]++
			histo[1][byte(x1>>8)]++
			histo[1][byte(x1>>16)]++
			histo[1][byte(x1>>24)]++
			x2 := cur ^ ((cur << 16) | (p4 >> 16))
			histo[2][byte(x2>>0)]++
			histo[2][byte(x2>>8)]++
			histo[2][byte(x2>>16)]++
			histo[2][byte(x2>>24)]++
			x3 := cur ^ ((cur << 24) | (p4 >> 8))
			histo[3][byte(x3>>0)]++
			histo[3][byte(x3>>8)]++
			histo[3][byte(x3>>16)]++
			histo[3][byte(x3>>24)]++
			x4 := cur ^ p4
			histo[4][byte(x4>>0)]++
			histo[4][byte(x4>>8)]++
			histo[4][byte(x4>>16)]++
			histo[4][byte(x4>>24)]++
			x5 := cur ^ p8
			histo[5][byte(x5>>0)]++
			histo[5][byte(x5>>8)]++
			histo[5][byte(x5>>16)]++
			histo[5][byte(x5>>24)]++
			x6 := cur ^ p16
			histo[6][byte(x6>>0)]++
			histo[6][byte(x6>>8)]++
			histo[6][byte(x6>>16)]++
			histo[6][byte(x6>>24)]++
			p16 = p12
			p12 = p8
			p8 = p4
			p4 = cur
		}
		for ; i < count5; i++ {
			histo[0][in[i]]++
			histo[1][in[i]^in[i-1]]++
			histo[2][in[i]^in[i-2]]++
			histo[3][in[i]^in[i-3]]++
			histo[4][in[i]^in[i-4]]++
			histo[5][in[i]^in[i-8]]++
			histo[6][in[i]^in[i-16]]++
		}
	}
}

func fsdForwardBucketed(src, dst []byte, dstIdx, dist int) int {
	tileLength := dist * (1 << 15)
	var outs, idxs [8]int
	for tileStart := 0; tileStart < len(src); tileStart += tileLength {
		tileEnd := tileStart + min(tileLength, len(src)-tileStart)
		firstStart := tileStart
		if tileStart == 0 {
			firstStart += dist
		}
		cur := dstIdx
		for lane := 0; lane < dist; lane++ {
			outs[lane] = cur
			fp := firstStart + lane
			if fp < tileEnd {
				cur += (tileEnd-1-fp)/dist + 1
			}
		}
		dstIdx = cur
		pos, outIdx := firstStart, 0
		if dist == 2 {
			for ; pos+8 <= tileEnd; pos += 8 {
				z0 := fsdZigzagSwar64(binary.LittleEndian.Uint64(src[pos+0:]), binary.LittleEndian.Uint64(src[pos+-2:]))
				binary.LittleEndian.PutUint32(dst[outs[0]+outIdx:], (uint32(byte(z0>>0))<<0)|(uint32(byte(z0>>16))<<8)|(uint32(byte(z0>>32))<<16)|(uint32(byte(z0>>48))<<24))
				binary.LittleEndian.PutUint32(dst[outs[1]+outIdx:], (uint32(byte(z0>>8))<<0)|(uint32(byte(z0>>24))<<8)|(uint32(byte(z0>>40))<<16)|(uint32(byte(z0>>56))<<24))
				outIdx += 4
			}
		} else if dist == 4 {
			for ; pos+16 <= tileEnd; pos += 16 {
				z0 := fsdZigzagSwar64(binary.LittleEndian.Uint64(src[pos+0:]), binary.LittleEndian.Uint64(src[pos+-4:]))
				z1 := fsdZigzagSwar64(binary.LittleEndian.Uint64(src[pos+8:]), binary.LittleEndian.Uint64(src[pos+4:]))
				binary.LittleEndian.PutUint32(dst[outs[0]+outIdx:], (uint32(byte(z0>>0))<<0)|(uint32(byte(z0>>32))<<8)|(uint32(byte(z1>>0))<<16)|(uint32(byte(z1>>32))<<24))
				binary.LittleEndian.PutUint32(dst[outs[1]+outIdx:], (uint32(byte(z0>>8))<<0)|(uint32(byte(z0>>40))<<8)|(uint32(byte(z1>>8))<<16)|(uint32(byte(z1>>40))<<24))
				binary.LittleEndian.PutUint32(dst[outs[2]+outIdx:], (uint32(byte(z0>>16))<<0)|(uint32(byte(z0>>48))<<8)|(uint32(byte(z1>>16))<<16)|(uint32(byte(z1>>48))<<24))
				binary.LittleEndian.PutUint32(dst[outs[3]+outIdx:], (uint32(byte(z0>>24))<<0)|(uint32(byte(z0>>56))<<8)|(uint32(byte(z1>>24))<<16)|(uint32(byte(z1>>56))<<24))
				outIdx += 4
			}
		} else if dist == 8 {
			for ; pos+8 <= tileEnd; pos += 8 {
				z0 := fsdZigzagSwar64(binary.LittleEndian.Uint64(src[pos+0:]), binary.LittleEndian.Uint64(src[pos+-8:]))
				dst[outs[0]+outIdx] = byte(z0 >> 0)
				dst[outs[1]+outIdx] = byte(z0 >> 8)
				dst[outs[2]+outIdx] = byte(z0 >> 16)
				dst[outs[3]+outIdx] = byte(z0 >> 24)
				dst[outs[4]+outIdx] = byte(z0 >> 32)
				dst[outs[5]+outIdx] = byte(z0 >> 40)
				dst[outs[6]+outIdx] = byte(z0 >> 48)
				dst[outs[7]+outIdx] = byte(z0 >> 56)
				outIdx += 1
			}
		}
		for lane := range idxs {
			idxs[lane] = outIdx
		}
		for ; pos < tileEnd; pos++ {
			lane := (pos - firstStart) & (dist - 1)
			dst[outs[lane]+idxs[lane]] = fsdZigzagByte(src[pos], src[pos-dist])
			idxs[lane]++
		}
	}
	return dstIdx
}

func fsdInverseLegacy(src, dst []byte, srcIdx, dstIdx, dist int, mode byte) (int, int, error) {
	if mode == _FSD_DELTA_CODING {
		for srcIdx < len(src) && dstIdx < len(dst) {
			if src[srcIdx] != _FSD_ESCAPE_TOKEN {
				dst[dstIdx] = byte(int(dst[dstIdx-dist]) + _FSD_ZIGZAG2[src[srcIdx]])
				srcIdx++
			} else {
				srcIdx++
				if srcIdx == len(src) {
					return srcIdx, dstIdx, errors.New("FSD inverse transform failed: invalid data")
				}
				dst[dstIdx] = src[srcIdx] ^ dst[dstIdx-dist]
				srcIdx++
			}
			dstIdx++
		}
	} else {
		srcIdx, dstIdx = fsdInverseXor(src, dst, srcIdx, dstIdx, dist)
	}
	if srcIdx != len(src) {
		return srcIdx, dstIdx, errors.New("FSD inverse transform failed: output buffer too small")
	}
	return srcIdx, dstIdx, nil
}

func fsdInverseXor(src, dst []byte, srcIdx, dstIdx, dist int) (int, int) {
	if dist == 16 || dist == 8 {
		for srcIdx+dist <= len(src) && dstIdx+dist <= len(dst) {
			for k := 0; k < dist; k += 8 {
				v := binary.LittleEndian.Uint64(src[srcIdx+k:]) ^ binary.LittleEndian.Uint64(dst[dstIdx+k-dist:])
				binary.LittleEndian.PutUint64(dst[dstIdx+k:], v)
			}
			srcIdx += dist
			dstIdx += dist
		}
	}
	for srcIdx < len(src) && dstIdx < len(dst) {
		dst[dstIdx] = src[srcIdx] ^ dst[dstIdx-dist]
		srcIdx++
		dstIdx++
	}
	return srcIdx, dstIdx
}

// FSDCodec Fixed Step Delta codec is used to decorrelate values separated
// by a constant distance (step) and encode residuals
type FSDCodec struct {
	ctx       *map[string]any
	bsVersion uint
}

// NewFSDCodec creates a new instance of FSDCodec
func NewFSDCodec() (*FSDCodec, error) {
	this := &FSDCodec{}
	this.bsVersion = 7
	return this, nil
}

// NewFSDCodecWithCtx creates a new instance of FSDCodec using a
// configuration map as parameter.
func NewFSDCodecWithCtx(ctx *map[string]any) (*FSDCodec, error) {
	this := &FSDCodec{}
	this.ctx = ctx
	this.bsVersion = 7

	if ctx != nil {
		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			version, ok := val.(uint)

			if ok == false {
				return nil, errors.New("FSD codec: invalid bitstream version type")
			}

			this.bsVersion = version
		}
	}

	return this, nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *FSDCodec) MaxEncodedLen(srcLen int) int {
	return srcLen + max(srcLen>>4, 64) // limit expansion
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *FSDCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)
	maxEncodedLen := this.MaxEncodedLen(count)

	if len(dst) < maxEncodedLen {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), maxEncodedLen)
	}

	// If too small, skip
	if count < _FSD_MIN_BLOCK_LENGTH {
		return 0, 0, fmt.Errorf("Block too small, skip")
	}

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt, ok := val.(internal.DataType)

			if ok == false {
				return 0, 0, errors.New("FSD forward transform failed: invalid data type")
			}

			if dt != internal.DT_UNDEFINED && dt != internal.DT_MULTIMEDIA && dt != internal.DT_BIN {
				return 0, 0, fmt.Errorf("FSD forward transform skip")
			}
		}
	}

	magic := internal.GetMagicType(src)

	// Skip detection except for a few candidate types
	switch magic {
	case internal.BMP_MAGIC:
		break
	case internal.RIFF_MAGIC:
		break
	case internal.PBM_MAGIC:
		break
	case internal.PGM_MAGIC:
		break
	case internal.PPM_MAGIC:
		break
	case internal.NO_MAGIC:
		break
	default:
		return 0, 0, fmt.Errorf("FSD forward skip: found %#x magic value header", magic)
	}

	// Check several step values on a few sub-blocks (no memory allocation)
	count10 := count / 10
	count5 := 2 * count10 // count5=count/5 does not guarantee count5=2*count10 !
	var histo [7][256]int
	fsdSampleHistograms(src, count10, count5, &histo)

	// Find if entropy is lower post transform
	var ent [7]int
	minIdx := 0

	for i := range ent {
		ent[i] = internal.ComputeFirstOrderEntropy1024(3*count10, histo[i][:])

		if ent[i] < ent[minIdx] {
			minIdx = i
		}
	}

	// If not better, quick exit
	if ent[minIdx] >= ent[0] {
		if this.ctx != nil {
			(*this.ctx)["dataType"] = internal.DetectSimpleType(3*count10, histo[0][:])
		}

		return 0, 0, fmt.Errorf("FSD forward transform skip")
	}

	var distances = []int{0, 1, 2, 3, 4, 8, 16}
	dist := distances[minIdx]
	coding := _FSD_DELTA_CODING

	// Keep triplet-correlated data interleaved since phase bucketing can
	// disrupt downstream matches for this layout.
	bucketed := dist > 1 && dist != 3 && dist != 16
	mode := coding

	if bucketed {
		mode |= 2
	}

	dst[0] = mode
	dst[1] = byte(dist)
	srcIdx := 0
	dstIdx := 2

	copy(dst[dstIdx:], src[:dist])
	srcIdx += dist
	dstIdx += dist
	if bucketed {
		dstIdx = fsdForwardBucketed(src, dst, dstIdx, dist)
		srcIdx = count
	} else {
		for srcIdx+8 <= count {
			z := fsdZigzagSwar64(binary.LittleEndian.Uint64(src[srcIdx:]), binary.LittleEndian.Uint64(src[srcIdx-dist:]))
			binary.LittleEndian.PutUint64(dst[dstIdx:], z)
			srcIdx += 8
			dstIdx += 8
		}
		for srcIdx < count {
			dst[dstIdx] = fsdZigzagByte(src[srcIdx], src[srcIdx-dist])
			srcIdx++
			dstIdx++
		}
	}

	clear(histo[1][:])
	clear(histo[2][:])
	out1 := dst[count5:]
	out2 := dst[3*count5:]
	oi := 0
	for ; oi+4 <= count10; oi += 4 {
		v1 := binary.LittleEndian.Uint32(out1[oi:])
		v2 := binary.LittleEndian.Uint32(out2[oi:])
		histo[1][byte(v1>>0)]++
		histo[2][byte(v1>>8)]++
		histo[1][byte(v1>>16)]++
		histo[2][byte(v1>>24)]++
		histo[1][byte(v2>>0)]++
		histo[2][byte(v2>>8)]++
		histo[1][byte(v2>>16)]++
		histo[2][byte(v2>>24)]++
	}
	for ; oi < count10; oi++ {
		histo[1][out1[oi]]++
		histo[2][out2[oi]]++
	}
	for b := 0; b < 256; b++ {
		histo[1][b] += histo[2][b]
	}

	var err error

	if entropy := internal.ComputeFirstOrderEntropy1024(count5, histo[1][:]); entropy*100 >= ent[0]*99 {
		if this.ctx != nil {
			(*this.ctx)["dataType"] = internal.DetectSimpleType(3*count10, histo[0][:])
		}
		err = errors.New("FSD forward transform skip: no improvement")
	} else if this.ctx != nil {
		(*this.ctx)["dataType"] = internal.DT_MULTIMEDIA
	}

	return uint(srcIdx), uint(dstIdx), err // Allowed to expand
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *FSDCodec) Inverse(src, dst []byte) (uint, uint, error) {
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
	mode := src[0]
	modeValue := int(mode)
	coding := mode & 1
	bucketed := (mode & 2) != 0
	dist := int(src[1])

	// Sanity check
	if (dist < 1) || ((dist > 4) && (dist != 8) && (dist != 16)) {
		return 0, 0, errors.New("FSD inverse transform failed: invalid distance")
	}

	if (this.bsVersion >= 7 && (modeValue&^3) != 0) || (this.bsVersion < 7 && modeValue > 1) {
		return 0, 0, errors.New("FSD inverse transform failed: invalid mode")
	}

	srcEnd := len(src)
	dstEnd := len(dst)
	dataLength := count - 2
	srcIdx := 2
	dstIdx := 0
	var err error

	if srcEnd-srcIdx < dist {
		return 0, 0, errors.New("FSD inverse transform failed: invalid data")
	}

	if dstEnd < dist {
		return 0, 0, errors.New("FSD inverse transform failed: output buffer too small")
	}

	if this.bsVersion >= 7 && dstEnd < dataLength {
		return 0, 0, errors.New("FSD inverse transform failed: output buffer too small")
	}

	// Emit first bytes
	for i := 0; i < dist; i++ {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++
	}

	if this.bsVersion < 7 {
		srcIdx, dstIdx, err = fsdInverseLegacy(src, dst, srcIdx, dstIdx, dist, mode)
		if err != nil {
			return uint(srcIdx), uint(dstIdx), err
		}
	} else if bucketed {
		tileLength := dist * (1 << 15)
		if coding == _FSD_DELTA_CODING {
			for tileStart := 0; tileStart < dataLength; tileStart += tileLength {
				tileEnd := tileStart + min(tileLength, dataLength-tileStart)
				for lane := 0; lane < dist; lane++ {
					pos := tileStart + lane
					if tileStart == 0 {
						pos += dist
					}
					if pos >= tileEnd {
						continue
					}
					acc := dst[pos-dist]
					for ; pos+3*dist < tileEnd; pos += 4 * dist {
						d0 := _FSD_ZIGZAG2[src[srcIdx+0]]
						d1 := _FSD_ZIGZAG2[src[srcIdx+1]]
						d2 := _FSD_ZIGZAG2[src[srcIdx+2]]
						d3 := _FSD_ZIGZAG2[src[srcIdx+3]]
						acc = byte(int(acc) + d0)
						dst[pos+0*dist] = acc
						acc = byte(int(acc) + d1)
						dst[pos+1*dist] = acc
						acc = byte(int(acc) + d2)
						dst[pos+2*dist] = acc
						acc = byte(int(acc) + d3)
						dst[pos+3*dist] = acc
						srcIdx += 4
					}
					for ; pos < tileEnd; pos += dist {
						acc = byte(int(acc) + _FSD_ZIGZAG2[src[srcIdx]])
						srcIdx++
						dst[pos] = acc
					}
				}
			}
		} else {
			for tileStart := 0; tileStart < dataLength; tileStart += tileLength {
				tileEnd := tileStart + min(tileLength, dataLength-tileStart)
				for lane := 0; lane < dist; lane++ {
					firstPos := tileStart + lane
					if tileStart == 0 {
						firstPos += dist
					}
					for pos := firstPos; pos < tileEnd; pos += dist {
						dst[pos] = src[srcIdx] ^ dst[pos-dist]
						srcIdx++
					}
				}
			}
		}
		dstIdx = dataLength
	} else if coding == _FSD_DELTA_CODING {
		if dist == 16 {
			prev0 := binary.LittleEndian.Uint64(dst[0:])
			prev1 := binary.LittleEndian.Uint64(dst[8:])
			for srcIdx+16 <= srcEnd {
				prev0 = fsdAddBytesSwar64(prev0, fsdUnzigzagSwar64(binary.LittleEndian.Uint64(src[srcIdx+0:])))
				binary.LittleEndian.PutUint64(dst[dstIdx+0:], prev0)
				prev1 = fsdAddBytesSwar64(prev1, fsdUnzigzagSwar64(binary.LittleEndian.Uint64(src[srcIdx+8:])))
				binary.LittleEndian.PutUint64(dst[dstIdx+8:], prev1)
				srcIdx += 16
				dstIdx += 16
			}
		} else if dist == 8 {
			prev0 := binary.LittleEndian.Uint64(dst[0:])
			for srcIdx+8 <= srcEnd {
				prev0 = fsdAddBytesSwar64(prev0, fsdUnzigzagSwar64(binary.LittleEndian.Uint64(src[srcIdx+0:])))
				binary.LittleEndian.PutUint64(dst[dstIdx+0:], prev0)
				srcIdx += 8
				dstIdx += 8
			}
		} else if dist == 3 {
			a0 := dst[0]
			a1 := dst[1]
			a2 := dst[2]
			for srcIdx+6 <= srcEnd {
				d0 := _FSD_ZIGZAG2[src[srcIdx+0]]
				d1 := _FSD_ZIGZAG2[src[srcIdx+1]]
				d2 := _FSD_ZIGZAG2[src[srcIdx+2]]
				d3 := _FSD_ZIGZAG2[src[srcIdx+3]]
				d4 := _FSD_ZIGZAG2[src[srcIdx+4]]
				d5 := _FSD_ZIGZAG2[src[srcIdx+5]]
				a0 = byte(int(a0) + d0)
				dst[dstIdx+0] = a0
				a1 = byte(int(a1) + d1)
				dst[dstIdx+1] = a1
				a2 = byte(int(a2) + d2)
				dst[dstIdx+2] = a2
				a0 = byte(int(a0) + d3)
				dst[dstIdx+3] = a0
				a1 = byte(int(a1) + d4)
				dst[dstIdx+4] = a1
				a2 = byte(int(a2) + d5)
				dst[dstIdx+5] = a2
				srcIdx += 6
				dstIdx += 6
			}
		} else if dist == 1 {
			a0 := dst[0]
			for srcIdx+4 <= srcEnd {
				d0 := _FSD_ZIGZAG2[src[srcIdx+0]]
				d1 := _FSD_ZIGZAG2[src[srcIdx+1]]
				d2 := _FSD_ZIGZAG2[src[srcIdx+2]]
				d3 := _FSD_ZIGZAG2[src[srcIdx+3]]
				a0 = byte(int(a0) + d0)
				dst[dstIdx+0] = a0
				a0 = byte(int(a0) + d1)
				dst[dstIdx+1] = a0
				a0 = byte(int(a0) + d2)
				dst[dstIdx+2] = a0
				a0 = byte(int(a0) + d3)
				dst[dstIdx+3] = a0
				srcIdx += 4
				dstIdx += 4
			}
		}
		for srcIdx < srcEnd {
			dst[dstIdx] = byte(int(dst[dstIdx-dist]) + _FSD_ZIGZAG2[src[srcIdx]])
			srcIdx++
			dstIdx++
		}
	} else {
		srcIdx, dstIdx = fsdInverseXor(src, dst, srcIdx, dstIdx, dist)
	}

	if srcIdx != srcEnd {
		err = errors.New("FSD inverse transform failed: output buffer too small")
	}

	return uint(srcIdx), uint(dstIdx), err
}
