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

// Implementation of Discrete Sine Transform of dimension 4

const (
	W4_29 = 29
	W4_74 = 74
	W4_55 = 55

	MAX_VAL_DST4 = 1 << 16
	MIN_VAL_DST4 = -(MAX_VAL_DST4 + 1)
)

type DST4 struct {
	fShift uint  // default 8
	iShift uint  // default 20
	data   []int // int[16]
}

func NewDST4() (*DST4, error) {
	this := new(DST4)
	this.fShift = 8
	this.iShift = 20
	this.data = make([]int, 16)
	return this, nil
}

func (this *DST4) Forward(src, dst []int) (uint, uint, error) {
	if len(src) != 16 {
		return 0, 0, errors.New("Input size must be 16")
	}

	if len(dst) < 16 {
		return 0, 0, errors.New("Output size must be at least 16")
	}

	computeForwardDST4(src, this.data, 4)
	computeForwardDST4(this.data, dst, this.fShift-4)
	return 16, 16, nil
}

func computeForwardDST4(input []int, output []int, shift uint) {
	round := (1 << shift) >> 1
	in := input[0:16]
	out := output[0:16]

	x0 := in[0]
	x1 := in[1]
	x2 := in[2]
	x3 := in[3]
	x4 := in[4]
	x5 := in[5]
	x6 := in[6]
	x7 := in[7]
	x8 := in[8]
	x9 := in[9]
	x10 := in[10]
	x11 := in[11]
	x12 := in[12]
	x13 := in[13]
	x14 := in[14]
	x15 := in[15]

	a0 := x0 + x3
	a1 := x1 + x3
	a2 := x0 - x1
	a3 := W4_74 * x2
	a4 := x4 + x7
	a5 := x5 + x7
	a6 := x4 - x5
	a7 := W4_74 * x6
	a8 := x8 + x11
	a9 := x9 + x11
	a10 := x8 - x9
	a11 := W4_74 * x10
	a12 := x12 + x15
	a13 := x13 + x15
	a14 := x12 - x13
	a15 := W4_74 * x14

	out[0] = ((W4_29 * a0) + (W4_55 * a1) + a3 + round) >> shift
	out[1] = ((W4_29 * a4) + (W4_55 * a5) + a7 + round) >> shift
	out[2] = ((W4_29 * a8) + (W4_55 * a9) + a11 + round) >> shift
	out[3] = ((W4_29 * a12) + (W4_55 * a13) + a15 + round) >> shift
	out[4] = (W4_74*(x0+x1-x3) + round) >> shift
	out[5] = (W4_74*(x4+x5-x7) + round) >> shift
	out[6] = (W4_74*(x8+x9-x11) + round) >> shift
	out[7] = (W4_74*(x12+x13-x15) + round) >> shift
	out[8] = ((W4_29 * a2) + (W4_55 * a0) - a3 + round) >> shift
	out[9] = ((W4_29 * a6) + (W4_55 * a4) - a7 + round) >> shift
	out[10] = ((W4_29 * a10) + (W4_55 * a8) - a11 + round) >> shift
	out[11] = ((W4_29 * a14) + (W4_55 * a12) - a15 + round) >> shift
	out[12] = ((W4_55 * a2) - (W4_29 * a1) + a3 + round) >> shift
	out[13] = ((W4_55 * a6) - (W4_29 * a5) + a7 + round) >> shift
	out[14] = ((W4_55 * a10) - (W4_29 * a9) + a11 + round) >> shift
	out[15] = ((W4_55 * a14) - (W4_29 * a13) + a15 + round) >> shift
}

func (this *DST4) Inverse(src, dst []int) (uint, uint, error) {
	if len(src) != 16 {
		return 0, 0, errors.New("Input size must be 16")
	}

	if len(dst) < 16 {
		return 0, 0, errors.New("Output size must be at least 16")
	}

	computeInverseDST4(src, this.data, 10)
	computeInverseDST4(this.data, dst, this.iShift-10)
	return 16, 16, nil
}

func computeInverseDST4(input, output []int, shift uint) {
	round := (1 << shift) >> 1
	in := input[0:16]
	out := output[0:16]

	x0 := in[0]
	x1 := in[1]
	x2 := in[2]
	x3 := in[3]
	x4 := in[4]
	x5 := in[5]
	x6 := in[6]
	x7 := in[7]
	x8 := in[8]
	x9 := in[9]
	x10 := in[10]
	x11 := in[11]
	x12 := in[12]
	x13 := in[13]
	x14 := in[14]
	x15 := in[15]

	a0 := x0 + x8
	a1 := x8 + x12
	a2 := x0 - x12
	a3 := W4_74 * x4
	a4 := x1 + x9
	a5 := x9 + x13
	a6 := x1 - x13
	a7 := W4_74 * x5
	a8 := x2 + x10
	a9 := x10 + x14
	a10 := x2 - x14
	a11 := W4_74 * x6
	a12 := x3 + x11
	a13 := x11 + x15
	a14 := x3 - x15
	a15 := W4_74 * x7

	b0 := ((W4_29 * a0) + (W4_55 * a1) + a3 + round) >> shift
	b1 := ((W4_55 * a2) - (W4_29 * a1) + a3 + round) >> shift
	b2 := (W4_74*(x0-x8+x12) + round) >> shift
	b3 := ((W4_55 * a0) + (W4_29 * a2) - a3 + round) >> shift
	b4 := ((W4_29 * a4) + (W4_55 * a5) + a7 + round) >> shift
	b5 := ((W4_55 * a6) - (W4_29 * a5) + a7 + round) >> shift
	b6 := (W4_74*(x1-x9+x13) + round) >> shift
	b7 := ((W4_55 * a4) + (W4_29 * a6) - a7 + round) >> shift
	b8 := ((W4_29 * a8) + (W4_55 * a9) + a11 + round) >> shift
	b9 := ((W4_55 * a10) - (W4_29 * a9) + a11 + round) >> shift
	b10 := (W4_74*(x2-x10+x14) + round) >> shift
	b11 := ((W4_55 * a8) + (W4_29 * a10) - a11 + round) >> shift
	b12 := ((W4_29 * a12) + (W4_55 * a13) + a15 + round) >> shift
	b13 := ((W4_55 * a14) - (W4_29 * a13) + a15 + round) >> shift
	b14 := (W4_74*(x3-x11+x15) + round) >> shift
	b15 := ((W4_55 * a12) + (W4_29 * a14) - a15 + round) >> shift

	out[0] = kanzi.Clamp(b0, MIN_VAL_DST4, MAX_VAL_DST4)
	out[1] = kanzi.Clamp(b1, MIN_VAL_DST4, MAX_VAL_DST4)
	out[2] = kanzi.Clamp(b2, MIN_VAL_DST4, MAX_VAL_DST4)
	out[3] = kanzi.Clamp(b3, MIN_VAL_DST4, MAX_VAL_DST4)
	out[4] = kanzi.Clamp(b4, MIN_VAL_DST4, MAX_VAL_DST4)
	out[5] = kanzi.Clamp(b5, MIN_VAL_DST4, MAX_VAL_DST4)
	out[6] = kanzi.Clamp(b6, MIN_VAL_DST4, MAX_VAL_DST4)
	out[7] = kanzi.Clamp(b7, MIN_VAL_DST4, MAX_VAL_DST4)
	out[8] = kanzi.Clamp(b8, MIN_VAL_DST4, MAX_VAL_DST4)
	out[9] = kanzi.Clamp(b9, MIN_VAL_DST4, MAX_VAL_DST4)
	out[10] = kanzi.Clamp(b10, MIN_VAL_DST4, MAX_VAL_DST4)
	out[11] = kanzi.Clamp(b11, MIN_VAL_DST4, MAX_VAL_DST4)
	out[12] = kanzi.Clamp(b12, MIN_VAL_DST4, MAX_VAL_DST4)
	out[13] = kanzi.Clamp(b13, MIN_VAL_DST4, MAX_VAL_DST4)
	out[14] = kanzi.Clamp(b14, MIN_VAL_DST4, MAX_VAL_DST4)
	out[15] = kanzi.Clamp(b15, MIN_VAL_DST4, MAX_VAL_DST4)
}
