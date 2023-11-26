/*
Copyright 2011-2024 Frederic Langlet
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

var _FSD_ZIGZAG1 = [256]byte{
	253, 251, 249, 247, 245, 243, 241, 239,
	237, 235, 233, 231, 229, 227, 225, 223,
	221, 219, 217, 215, 213, 211, 209, 207,
	205, 203, 201, 199, 197, 195, 193, 191,
	189, 187, 185, 183, 181, 179, 177, 175,
	173, 171, 169, 167, 165, 163, 161, 159,
	157, 155, 153, 151, 149, 147, 145, 143,
	141, 139, 137, 135, 133, 131, 129, 127,
	125, 123, 121, 119, 117, 115, 113, 111,
	109, 107, 105, 103, 101, 99, 97, 95,
	93, 91, 89, 87, 85, 83, 81, 79,
	77, 75, 73, 71, 69, 67, 65, 63,
	61, 59, 57, 55, 53, 51, 49, 47,
	45, 43, 41, 39, 37, 35, 33, 31,
	29, 27, 25, 23, 21, 19, 17, 15,
	13, 11, 9, 7, 5, 3, 1, 0,
	2, 4, 6, 8, 10, 12, 14, 16,
	18, 20, 22, 24, 26, 28, 30, 32,
	34, 36, 38, 40, 42, 44, 46, 48,
	50, 52, 54, 56, 58, 60, 62, 64,
	66, 68, 70, 72, 74, 76, 78, 80,
	82, 84, 86, 88, 90, 92, 94, 96,
	98, 100, 102, 104, 106, 108, 110, 112,
	114, 116, 118, 120, 122, 124, 126, 128,
	130, 132, 134, 136, 138, 140, 142, 144,
	146, 148, 150, 152, 154, 156, 158, 160,
	162, 164, 166, 168, 170, 172, 174, 176,
	178, 180, 182, 184, 186, 188, 190, 192,
	194, 196, 198, 200, 202, 204, 206, 208,
	210, 212, 214, 216, 218, 220, 222, 224,
	226, 228, 230, 232, 234, 236, 238, 240,
	242, 244, 246, 248, 250, 252, 254, 255,
}

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

// FSDCodec Fixed Step Delta codec is used to decorrelate values separated
// by a constant distance (step) and encode residuals
type FSDCodec struct {
	ctx *map[string]any
}

// NewFSDCodec creates a new instance of FSDCodec
func NewFSDCodec() (*FSDCodec, error) {
	this := &FSDCodec{}
	return this, nil
}

// NewFSDCodecWithCtx creates a new instance of FSDCodec using a
// configuration map as parameter.
func NewFSDCodecWithCtx(ctx *map[string]any) (*FSDCodec, error) {
	this := &FSDCodec{}
	this.ctx = ctx
	return this, nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *FSDCodec) MaxEncodedLen(srcLen int) int {
	padding := (srcLen >> 4)

	if padding < 32 {
		padding = 32
	}

	return srcLen + padding // limit expansion
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *FSDCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	// If too small, skip
	if count < _FSD_MIN_BLOCK_LENGTH {
		return 0, 0, fmt.Errorf("Block too small, skip")
	}

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt := val.(internal.DataType)

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
	count5 := count / 5
	count10 := count / 10
	in0 := src[0*count5:]
	in1 := src[2*count5:]
	in2 := src[4*count5:]
	var histo [7][256]int

	for i := count10; i < count5; i++ {
		b0 := in0[i]
		histo[0][b0]++
		histo[1][b0^in0[i-1]]++
		histo[2][b0^in0[i-2]]++
		histo[3][b0^in0[i-3]]++
		histo[4][b0^in0[i-4]]++
		histo[5][b0^in0[i-8]]++
		histo[6][b0^in0[i-16]]++
		b1 := in1[i]
		histo[0][b1]++
		histo[1][b1^in1[i-1]]++
		histo[2][b1^in1[i-2]]++
		histo[3][b1^in1[i-3]]++
		histo[4][b1^in1[i-4]]++
		histo[5][b1^in1[i-8]]++
		histo[6][b1^in1[i-16]]++
		b2 := in2[i]
		histo[0][b2]++
		histo[1][b2^in2[i-1]]++
		histo[2][b2^in2[i-2]]++
		histo[3][b2^in2[i-3]]++
		histo[4][b2^in2[i-4]]++
		histo[5][b2^in2[i-8]]++
		histo[6][b2^in2[i-16]]++
	}

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

	if this.ctx != nil {
		(*this.ctx)["dataType"] = internal.DT_MULTIMEDIA
	}

	var distances = []int{0, 1, 2, 3, 4, 8, 16}
	dist := distances[minIdx]
	largeDeltas := 0

	// Detect best coding by sampling for large deltas
	for i := 2 * count5; i < 3*count5; i++ {
		delta := int32(src[i]) - int32(src[i-dist])

		if (delta < -127) || (delta > 127) {
			largeDeltas++
		}
	}

	// Delta coding works better for pictures & xor coding better for wav files
	// Select xor coding if large deltas are over 3% (ad-hoc threshold)
	mode := _FSD_DELTA_CODING

	if largeDeltas > (count5 >> 5) {
		mode = _FSD_XOR_CODING
	}

	dstEnd := this.MaxEncodedLen(count)
	dst[0] = mode
	dst[1] = byte(dist)
	srcIdx := 0
	dstIdx := 2

	// Emit first bytes
	for i := 0; i < dist; i++ {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++
	}

	// Emit modified bytes
	if mode == _FSD_DELTA_CODING {
		for srcIdx < count && dstIdx < dstEnd-1 {
			delta := 127 + int32(src[srcIdx]) - int32(src[srcIdx-dist])

			if delta >= 0 && delta < 255 {
				dst[dstIdx] = _FSD_ZIGZAG1[delta] // zigzag encode delta
				srcIdx++
				dstIdx++
				continue
			}

			// Skip delta, encode with escape
			dst[dstIdx] = _FSD_ESCAPE_TOKEN
			dst[dstIdx+1] = src[srcIdx] ^ src[srcIdx-dist]
			srcIdx++
			dstIdx += 2
		}
	} else { // mode == _FSD_XOR_CODING
		for srcIdx < count {
			dst[dstIdx] = src[srcIdx] ^ src[srcIdx-dist]
			srcIdx++
			dstIdx++
		}
	}

	if srcIdx != count {
		return uint(srcIdx), uint(dstIdx), errors.New("FSD forward transform skip: output buffer too small")
	}

	// Extra check that the transform makes sense
	for i := range histo[0] {
		histo[0][i] = 0
	}

	out1 := dst[1*count5 : 1*count5+count10]
	out2 := dst[3*count5 : 3*count5+count10]

	for i := 0; i < count10; i++ {
		histo[0][out1[i]]++
		histo[0][out2[i]]++
	}

	var err error

	if entropy := internal.ComputeFirstOrderEntropy1024(count5, histo[0][:]); entropy >= ent[0] {
		err = errors.New("FSD forward transform skip: no improvement")
	}

	return uint(srcIdx), uint(dstIdx), err // Allowed to expand
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *FSDCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	// Retrieve mode & step value
	mode := src[0]
	dist := int(src[1])

	// Sanity check
	if (dist < 1) || ((dist > 4) && (dist != 8) && (dist != 16)) {
		return 0, 0, errors.New("FSD inverse transform failed: invalid data")
	}

	srcEnd := len(src)
	dstEnd := len(dst)
	srcIdx := 2
	dstIdx := 0

	// Emit first bytes
	for i := 0; i < dist; i++ {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++
	}

	// Recover original bytes
	if mode == _FSD_DELTA_CODING {
		for srcIdx < srcEnd && dstIdx < dstEnd {
			if src[srcIdx] != _FSD_ESCAPE_TOKEN {
				dst[dstIdx] = byte(int(dst[dstIdx-dist]) + _FSD_ZIGZAG2[src[srcIdx]])
				srcIdx++
				dstIdx++
				continue
			}

			srcIdx++
			dst[dstIdx] = src[srcIdx] ^ dst[dstIdx-dist]
			srcIdx++
			dstIdx++
		}
	} else { // mode == _FSD_XOR_CODING
		for srcIdx < srcEnd {
			dst[dstIdx] = src[srcIdx] ^ dst[dstIdx-dist]
			dstIdx++
			srcIdx++
		}
	}

	var err error

	if srcIdx != srcEnd {
		err = errors.New("FSD inverse transform failed: output buffer too small")
	}

	return uint(srcIdx), uint(dstIdx), err
}
