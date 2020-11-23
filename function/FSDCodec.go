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
	_FSD_MIN_LENGTH   = 128
	_FSD_ESCAPE_TOKEN = 0xFF
	_FSD_DELTA_CODING = byte(0)
	_FSD_XOR_CODING   = byte(1)
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
	count10 := count / 10
	var in []byte
	dst1 := dst[0*count5 : 1*count5]
	dst2 := dst[1*count5 : 2*count5]
	dst3 := dst[2*count5 : 3*count5]
	dst4 := dst[3*count5 : 4*count5]
	dst8 := dst[4*count5 : count]

	// Check several step values on a sub-block (no memory allocation)
	// Sample 2 sub-blocks
	in = src[3*count5 : 3*count5+count]

	for i := 0; i < count10; i++ {
		b := in[i]
		dst1[i] = b ^ in[i-1]
		dst2[i] = b ^ in[i-2]
		dst3[i] = b ^ in[i-3]
		dst4[i] = b ^ in[i-4]
		dst8[i] = b ^ in[i-8]
	}

	in = src[1*count5 : 1*count5+count10]

	for i := count10; i < count5; i++ {
		b := in[i]
		dst1[i] = b ^ in[i-1]
		dst2[i] = b ^ in[i-2]
		dst3[i] = b ^ in[i-3]
		dst4[i] = b ^ in[i-4]
		dst8[i] = b ^ in[i-8]
	}

	// Find if entropy is lower post transform
	var histo [256]int
	var ent [6]int
	count3 := count / 3
	kanzi.ComputeHistogram(src[count3:2*count3], histo[:], true, false)
	ent[0] = kanzi.ComputeFirstOrderEntropy1024(count3, histo[:])
	kanzi.ComputeHistogram(dst1[0:count5], histo[:], true, false)
	ent[1] = kanzi.ComputeFirstOrderEntropy1024(count5, histo[:])
	kanzi.ComputeHistogram(dst2[0:count5], histo[:], true, false)
	ent[2] = kanzi.ComputeFirstOrderEntropy1024(count5, histo[:])
	kanzi.ComputeHistogram(dst3[0:count5], histo[:], true, false)
	ent[3] = kanzi.ComputeFirstOrderEntropy1024(count5, histo[:])
	kanzi.ComputeHistogram(dst4[0:count5], histo[:], true, false)
	ent[4] = kanzi.ComputeFirstOrderEntropy1024(count5, histo[:])
	kanzi.ComputeHistogram(dst8[0:count5], histo[:], true, false)
	ent[5] = kanzi.ComputeFirstOrderEntropy1024(count5, histo[:])

	minIdx := 0

	for i := 1; i < len(ent); i++ {
		if ent[i] < ent[minIdx] {
			minIdx = i
		}
	}

	// If not 'better enough', quick exit
	if this.isFast == true && ent[minIdx] >= (123*ent[0])>>7 {
		return 0, 0, fmt.Errorf("FSD forward transform skip")
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

	var err error

	if srcIdx != count {
		err = errors.New("FSD forward transform skip: output buffer too small")
	} else {
		// Extra check that the transform makes sense
		length := dstIdx

		if this.isFast == true {
			length = dstIdx >> 1
		}

		kanzi.ComputeHistogram(dst[(dstIdx-length)>>1:(dstIdx+length)>>1], histo[:], true, false)

		if entropy := kanzi.ComputeFirstOrderEntropy1024(length, histo[:]); entropy >= ent[0] {
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
