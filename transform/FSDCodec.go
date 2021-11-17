/*
Copyright 2011-2021 Frederic Langlet
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

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	_FSD_MIN_LENGTH   = 128
	_FSD_ESCAPE_TOKEN = 0xFF
	_FSD_DELTA_CODING = byte(0)
	_FSD_XOR_CODING   = byte(1)
)

// FSDCodec Fixed Step Delta codec is used to decorrelate values separated
// by a constant distance (step) and encode residuals
type FSDCodec struct {
	ctx *map[string]interface{}
}

// NewFSDCodec creates a new instance of FSDCodec
func NewFSDCodec() (*FSDCodec, error) {
	this := &FSDCodec{}
	return this, nil
}

// NewFSDCodecWithCtx creates a new instance of FSDCodec using a
// configuration map as parameter.
func NewFSDCodecWithCtx(ctx *map[string]interface{}) (*FSDCodec, error) {
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
	if count < _FSD_MIN_LENGTH {
		return 0, 0, fmt.Errorf("Block too small, skip")
	}

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt := val.(kanzi.DataType)

			if dt != kanzi.DT_UNDEFINED && dt != kanzi.DT_MULTIMEDIA {
				return 0, 0, fmt.Errorf("FSD forward transform skip")
			}
		}
	}

	dstEnd := this.MaxEncodedLen(count)
	count5 := count / 5
	count10 := count / 10
	var in []byte
	var histo [6][256]int

	// Check several step values on a sub-block (no memory allocation)
	// Sample 2 sub-blocks
	in = src[3*count5 : 3*count5+count]

	for i := 0; i < count10; i++ {
		b := in[i]
		histo[0][b]++
		histo[1][b^in[i-1]]++
		histo[2][b^in[i-2]]++
		histo[3][b^in[i-3]]++
		histo[4][b^in[i-4]]++
		histo[5][b^in[i-8]]++
	}

	in = src[1*count5 : 1*count5+count10]

	for i := count10; i < count5; i++ {
		b := in[i]
		histo[0][b]++
		histo[1][b^in[i-1]]++
		histo[2][b^in[i-2]]++
		histo[3][b^in[i-3]]++
		histo[4][b^in[i-4]]++
		histo[5][b^in[i-8]]++
	}

	// Find if entropy is lower post transform
	var ent [6]int
	ent[0] = kanzi.ComputeFirstOrderEntropy1024(count5, histo[0][:])

	minIdx := 0

	for i := 1; i < len(ent); i++ {
		ent[i] = kanzi.ComputeFirstOrderEntropy1024(count5, histo[i][:])

		if ent[i] < ent[minIdx] {
			minIdx = i
		}
	}

	// If not better, quick exit
	if ent[minIdx] >= ent[0] {
		return 0, 0, fmt.Errorf("FSD forward transform skip")
	}

	if this.ctx != nil {
		(*this.ctx)["dataType"] = kanzi.DT_MULTIMEDIA
	}

	dist := minIdx

	if dist > 4 {
		dist = 8
	}

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
		for srcIdx < count && dstIdx < dstEnd {
			delta := int32(src[srcIdx]) - int32(src[srcIdx-dist])

			if delta >= -127 && delta <= 127 {
				dst[dstIdx] = byte((delta >> 31) ^ (delta << 1)) // zigzag encode delta
				srcIdx++
				dstIdx++
				continue
			}

			if dstIdx == dstEnd-1 {
				break
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

	length := dstIdx
	isFast := true

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["fullFSD"]; containsKey {
			isFast = val.(bool)
		}
	}

	if isFast == true {
		length = dstIdx >> 1
	}

	// Extra check that the transform makes sense
	for i := range histo[0] {
		histo[0][i] = 0
	}

	kanzi.ComputeHistogram(dst[(dstIdx-length)>>1:(dstIdx+length)>>1], histo[0][:], true, false)
	var err error

	if entropy := kanzi.ComputeFirstOrderEntropy1024(length, histo[0][:]); entropy >= ent[0] {
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
	if (dist < 1) || ((dist > 4) && (dist != 8)) {
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
				delta := int32(src[srcIdx]>>1) ^ -int32(src[srcIdx]&1) // zigzag decode delta
				dst[dstIdx] = byte(int32(dst[dstIdx-dist]) + delta)
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
