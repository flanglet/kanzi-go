/*
Copyright 2011-2017 Frederic Langlet
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
	"kanzi"
)

// Hybrid Discrete Wavelet Transform / Discrete Cosine Transform for 2D signals
// May not be exact due to integer rounding errors.
type DWT_DCT struct {
	dwt    kanzi.IntTransform
	dct    kanzi.IntTransform
	dim    uint
	buffer []int
}

func NewDWT_DCT(dim uint) (*DWT_DCT, error) {
	var tr kanzi.IntTransform
	var err error

	switch dim {
	case 8:
		tr, err = NewDCT4()
		break
	case 16:
		tr, err = NewDCT8()
		break
	case 32:
		tr, err = NewDCT16()
		break
	case 64:
		tr, err = NewDCT32()
		break
	default:
		err = errors.New("Invalid transform dimension (must be 8, 16, 32 or 64)")
	}

	this := new(DWT_DCT)

	if err != nil {
		return this, err
	}

	this.buffer = make([]int, dim*dim)
	this.dct = tr
	this.dwt, err = NewDWT(dim, dim, 1)
	this.dim = dim
	return this, err
}

func (this *DWT_DCT) Forward(src, dst []int) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if kanzi.SameIntSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count != int(this.dim*this.dim) {
		errMsg := fmt.Sprintf("Input buffer size must be %v", this.dim*this.dim)
		return 0, 0, errors.New(errMsg)
	}

	if len(dst) < count {
		return 0, 0, errors.New("The output buffer is too small")
	}

	d2 := this.dim >> 1

	// Forward DWT
	if _, _, err := this.dwt.Forward(src, dst); err != nil {
		return 0, 0, err
	}

	// Copy and compact DWT results for LL band
	for j := uint(0); j < d2; j++ {
		copy(this.buffer[j*d2:j*d2+d2], dst[j*this.dim:j*this.dim+d2])
	}

	// Forward DCT of LL band
	if _, _, err := this.dct.Forward(this.buffer, this.buffer); err != nil {
		return 0, 0, err
	}

	// Copy back DCT results
	for j := uint(0); j < d2; j++ {
		copy(dst[j*this.dim:j*this.dim+d2], this.buffer[j*d2:j*d2+d2])
	}

	return this.dim * this.dim, this.dim * this.dim, nil
}

func (this *DWT_DCT) Inverse(src, dst []int) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if kanzi.SameIntSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count != int(this.dim*this.dim) {
		errMsg := fmt.Sprintf("Input buffer size must be %v", this.dim*this.dim)
		return 0, 0, errors.New(errMsg)
	}

	if len(dst) < count {
		return 0, 0, errors.New("The output buffer is too small")
	}

	d2 := this.dim >> 1

	// Copy and compact LL band
	for j := uint(0); j < d2; j++ {
		copy(this.buffer[j*d2:j*d2+d2], src[j*this.dim:j*this.dim+d2])
	}

	// Reverse DCT of LL band
	if _, _, err := this.dct.Inverse(this.buffer, this.buffer); err != nil {
		return 0, 0, err
	}

	// Copy and expand DCT results for LL band
	for j := uint(0); j < d2; j++ {
		copy(src[j*this.dim:j*this.dim+d2], this.buffer[j*d2:j*d2+d2])
	}

	return this.dwt.Inverse(src, dst)
}
