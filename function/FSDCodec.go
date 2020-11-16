/*
Copyright 2011-2017 Frederic Langlet
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

package function

import (
	"errors"
	"fmt"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	_FSD_MIN_LENGTH   = 4096
	_FSD_ESCAPE_TOKEN = 0xFF
)

// FSDCodec Fixed Step Delta codec is used to decorrelate values separated
// by a constant distance (step) and encode residuals
type FSDCodec struct {
	isFast bool
}

// NewFSDCodec creates a new instance of FSDCodec
func NewFSDCodec() (*FSDCodec, error) {
	this := &FSDCodec{}
	this.isFast = true
	return this, nil
}

// NewFSDCodecWithCtx creates a new instance of FSDCodec using a
// configuration map as parameter.
func NewFSDCodecWithCtx(ctx *map[string]interface{}) (*FSDCodec, error) {
	this := &FSDCodec{}
	this.isFast = true

	if val, containsKey := (*ctx)["fullFSD"]; containsKey {
		fullFSD := val.(int)

		if fullFSD == 1 {
			this.isFast = false
		}
	}

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

	dstEnd := this.MaxEncodedLen(count)
	count5 := count / 5
	length1 := count5

	if this.isFast == true {
		length1 = count5 >> 1
	}

	dst1 := dst[0*count5 : 1*count5]
	dst2 := dst[1*count5 : 2*count5]
	dst3 := dst[2*count5 : 3*count5]
	dst4 := dst[3*count5 : 4*count5]
	dst8 := dst[4*count5 : count]

	// Check several step values on a sub-block (no memory allocation)
	for i := 8; i < length1; i++ {
		b := src[i]
		dst1[i] = b ^ src[i-1]
		dst2[i] = b ^ src[i-2]
		dst3[i] = b ^ src[i-3]
		dst4[i] = b ^ src[i-4]
		dst8[i] = b ^ src[i-8]
	}

	// Find if entropy is lower post transform
	var histo [256]int
	var ent [6]int
	ent[0] = kanzi.ComputeFirstOrderEntropy1024(src[8:length1], histo[:])
	ent[1] = kanzi.ComputeFirstOrderEntropy1024(dst1[8:length1], histo[:])
	ent[2] = kanzi.ComputeFirstOrderEntropy1024(dst2[8:length1], histo[:])
	ent[3] = kanzi.ComputeFirstOrderEntropy1024(dst3[8:length1], histo[:])
	ent[4] = kanzi.ComputeFirstOrderEntropy1024(dst4[8:length1], histo[:])
	ent[5] = kanzi.ComputeFirstOrderEntropy1024(dst8[8:length1], histo[:])

	minIdx := 0

	for i := 1; i < len(ent); i++ {
		if ent[i] < ent[minIdx] {
			minIdx = i
		}
	}

	// If not 'better enough', quick exit
	if ent[minIdx] >= (123*ent[0])>>7 {
		return 0, 0, fmt.Errorf("FSD forward transform skip")
	}

	// Emit step value
	dist := minIdx

	if dist > 4 {
		dist = 8
	}

	dst[0] = byte(dist)
	dstIdx := 1
	srcIdx := 0

	// Emit first bytes
	for i := 0; i < dist; i++ {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++
	}

	// Emit modified bytes
	for srcIdx < count && dstIdx < dstEnd {
		delta := int32(src[srcIdx]) - int32(src[srcIdx-dist])

		if delta >= -127 && delta <= 127 {
			dst[dstIdx] = byte((delta >> 31) ^ (delta << 1)) // zigzag encode delta
			srcIdx++
			dstIdx++
			continue
		}

		// Skip delta, direct encode
		dst[dstIdx] = _FSD_ESCAPE_TOKEN
		dst[dstIdx+1] = src[srcIdx]
		srcIdx++
		dstIdx += 2
	}

	var err error

	if srcIdx != count {
		err = errors.New("FSD forward transform skip: output buffer too small")
	} else {
		// Extra check that the transform makes sense
		length2 := dstIdx

		if this.isFast == true {
			length2 = dstIdx >> 1
		}

		entropy := kanzi.ComputeFirstOrderEntropy1024(dst8[(dstIdx-length2)>>1:length2], histo[:])

		if entropy >= ent[0] {
			err = errors.New("FSD forward transform skip: no improvement")
		}
	}

	return uint(srcIdx), uint(dstIdx), err
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

	// Retrieve step value
	dist := int(dst[0])

	// Sanity check
	if (dist < 1) || ((dist > 4) && (dist != 8)) {
		return 0, 0, errors.New("FSD inverse transform failed: invalid data")
	}

	dstEnd := len(dst)
	dstIdx := 1
	srcIdx := 0

	// Emit first bytes
	for i := 0; i < dist; i++ {
		dst[dstIdx] = src[srcIdx]
		dstIdx++
		srcIdx++
	}

	// Emit modified bytes
	for dstIdx < dstEnd {
		if src[srcIdx] != _FSD_ESCAPE_TOKEN {
			delta := int32(src[srcIdx]>>1) ^ -int32(src[srcIdx]&1)
			dst[dstIdx] = byte(int32(dst[dstIdx-dist]) + delta)
			srcIdx++
			dstIdx++
			continue
		}

		dst[dstIdx] = src[srcIdx+1]
		srcIdx += 2
		dstIdx++
	}

	var err error

	if dstIdx != dstEnd {
		err = errors.New("FSD inverse transform failed: output buffer too small")
	}

	return uint(srcIdx), uint(dstIdx), err
}
