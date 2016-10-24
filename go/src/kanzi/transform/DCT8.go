/*
Copyright 2011-2013 Frederic Langlet
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
	"kanzi"
)

// Implementation of Discrete Cosine Transform of dimension 8
// Due to rounding errors, the reconstruction may not be perfect

const (
	W8_0  = 64
	W8_1  = 64
	W8_8  = 89
	W8_9  = 75
	W8_10 = 50
	W8_11 = 18
	W8_16 = 83
	W8_17 = 36
	W8_24 = 75
	W8_25 = -18
	W8_26 = -89
	W8_27 = -50
	W8_32 = 64
	W8_33 = -64
	W8_40 = 50
	W8_41 = -89
	W8_42 = 18
	W8_43 = 75
	W8_48 = 36
	W8_49 = -83
	W8_56 = 18
	W8_57 = -50
	W8_58 = 75
	W8_59 = -89

	MAX_VAL_DCT8 = 1 << 16
	MIN_VAL_DCT8 = -(MAX_VAL_DCT8 + 1)
)

type DCT8 struct {
	fShift uint  // default 10
	iShift uint  // default 20
	data   []int // int[64]
}

func NewDCT8() (*DCT8, error) {
	this := new(DCT8)
	this.fShift = 10
	this.iShift = 20
	this.data = make([]int, 64)
	return this, nil
}

func (this *DCT8) Forward(src, dst []int) (uint, uint, error) {
	if len(src) != 64 {
		return 0, 0, errors.New("Input size must be 64")
	}

	if len(dst) < 64 {
		return 0, 0, errors.New("Output size must be at least 64")
	}

	computeForwardDCT8(src, this.data, 5)
	computeForwardDCT8(this.data, dst, this.fShift-5)
	return 64, 64, nil
}

func computeForwardDCT8(input, output []int, shift uint) {
	iIdx := 0
	round := (1 << shift) >> 1
	in := input[0:64]
	out := output[0:64]

	for i := 0; i < 8; i++ {
		x0 := in[iIdx]
		x1 := in[iIdx+1]
		x2 := in[iIdx+2]
		x3 := in[iIdx+3]
		x4 := in[iIdx+4]
		x5 := in[iIdx+5]
		x6 := in[iIdx+6]
		x7 := in[iIdx+7]

		a0 := x0 + x7
		a1 := x1 + x6
		a2 := x0 - x7
		a3 := x1 - x6
		a4 := x2 + x5
		a5 := x3 + x4
		a6 := x2 - x5
		a7 := x3 - x4

		b0 := a0 + a5
		b1 := a1 + a4
		b2 := a0 - a5
		b3 := a1 - a4

		out[i] = ((W8_0 * b0) + (W8_1 * b1) + round) >> shift
		out[i+8] = ((W8_8 * a2) + (W8_9 * a3) + (W8_10 * a6) + (W8_11 * a7) + round) >> shift
		out[i+16] = ((W8_16 * b2) + (W8_17 * b3) + round) >> shift
		out[i+24] = ((W8_24 * a2) + (W8_25 * a3) + (W8_26 * a6) + (W8_27 * a7) + round) >> shift
		out[i+32] = ((W8_32 * b0) + (W8_33 * b1) + round) >> shift
		out[i+40] = ((W8_40 * a2) + (W8_41 * a3) + (W8_42 * a6) + (W8_43 * a7) + round) >> shift
		out[i+48] = ((W8_48 * b2) + (W8_49 * b3) + round) >> shift
		out[i+56] = ((W8_56 * a2) + (W8_57 * a3) + (W8_58 * a6) + (W8_59 * a7) + round) >> shift

		iIdx += 8
	}
}

func (this *DCT8) Inverse(src, dst []int) (uint, uint, error) {
	if len(src) != 64 {
		return 0, 0, errors.New("Input size must be 64")
	}

	if len(dst) < 64 {
		return 0, 0, errors.New("Output size must be at least 64")
	}

	computeInverseDCT8(src, this.data, 10)
	computeInverseDCT8(this.data, dst, this.iShift-10)
	return 64, 64, nil
}

func computeInverseDCT8(input []int, output []int, shift uint) {
	oIdx := 0
	round := (1 << shift) >> 1
	in := input[0:64]
	out := output[0:64]

	for i := 0; i < 8; i++ {
		x0 := in[i]
		x1 := in[i+8]
		x2 := in[i+16]
		x3 := in[i+24]
		x4 := in[i+32]
		x5 := in[i+40]
		x6 := in[i+48]
		x7 := in[i+56]

		a0 := (W8_8 * x1) + (W8_24 * x3) + (W8_40 * x5) + (W8_56 * x7)
		a1 := (W8_9 * x1) + (W8_25 * x3) + (W8_41 * x5) + (W8_57 * x7)
		a2 := (W8_10 * x1) + (W8_26 * x3) + (W8_42 * x5) + (W8_58 * x7)
		a3 := (W8_11 * x1) + (W8_27 * x3) + (W8_43 * x5) + (W8_59 * x7)
		a4 := (W8_16 * x2) + (W8_48 * x6)
		a5 := (W8_17 * x2) + (W8_49 * x6)
		a6 := (W8_0 * x0) + (W8_32 * x4)
		a7 := (W8_1 * x0) + (W8_33 * x4)

		b0 := a6 + a4
		b1 := a7 + a5
		b2 := a6 - a4
		b3 := a7 - a5

		c0 := (b0 + a0 + round) >> shift
		c1 := (b1 + a1 + round) >> shift
		c2 := (b3 + a2 + round) >> shift
		c3 := (b2 + a3 + round) >> shift
		c4 := (b2 - a3 + round) >> shift
		c5 := (b3 - a2 + round) >> shift
		c6 := (b1 - a1 + round) >> shift
		c7 := (b0 - a0 + round) >> shift

		out[oIdx] = kanzi.Clamp(c0, MIN_VAL_DCT8, MAX_VAL_DCT8)
		out[oIdx+1] = kanzi.Clamp(c1, MIN_VAL_DCT8, MAX_VAL_DCT8)
		out[oIdx+2] = kanzi.Clamp(c2, MIN_VAL_DCT8, MAX_VAL_DCT8)
		out[oIdx+3] = kanzi.Clamp(c3, MIN_VAL_DCT8, MAX_VAL_DCT8)
		out[oIdx+4] = kanzi.Clamp(c4, MIN_VAL_DCT8, MAX_VAL_DCT8)
		out[oIdx+5] = kanzi.Clamp(c5, MIN_VAL_DCT8, MAX_VAL_DCT8)
		out[oIdx+6] = kanzi.Clamp(c6, MIN_VAL_DCT8, MAX_VAL_DCT8)
		out[oIdx+7] = kanzi.Clamp(c7, MIN_VAL_DCT8, MAX_VAL_DCT8)

		oIdx += 8
	}
}
