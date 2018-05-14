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
)

type WHT4 struct {
	fScale uint
	iScale uint
}

// For perfect reconstruction, forward results are scaled by 4 unless the
// parameter is set to false (in which case rounding may introduce errors)
func NewWHT4(scale bool) (*WHT4, error) {
	this := new(WHT4)

	if scale == true {
		this.fScale = 0
		this.iScale = 4
	} else {
		this.fScale = 2
		this.iScale = 2
	}

	return this, nil
}

func (this *WHT4) Forward(src, dst []int) (uint, uint, error) {
	if len(src) != 16 {
		return 0, 0, errors.New("Input size must be 16")
	}

	if len(dst) < 16 {
		return 0, 0, errors.New("Output size must be at least 16")
	}

	return this.compute(src, dst, this.fScale)
}

func (this *WHT4) compute(input, output []int, shift uint) (uint, uint, error) {
	// Pass 1: process rows.
	// Aliasing for speed
	x0 := input[0]
	x1 := input[1]
	x2 := input[2]
	x3 := input[3]
	x4 := input[4]
	x5 := input[5]
	x6 := input[6]
	x7 := input[7]
	x8 := input[8]
	x9 := input[9]
	x10 := input[10]
	x11 := input[11]
	x12 := input[12]
	x13 := input[13]
	x14 := input[14]
	x15 := input[15]

	a0 := x0 + x1
	a1 := x2 + x3
	a2 := x0 - x1
	a3 := x2 - x3
	a4 := x4 + x5
	a5 := x6 + x7
	a6 := x4 - x5
	a7 := x6 - x7
	a8 := x8 + x9
	a9 := x10 + x11
	a10 := x8 - x9
	a11 := x10 - x11
	a12 := x12 + x13
	a13 := x14 + x15
	a14 := x12 - x13
	a15 := x14 - x15

	b0 := a0 + a1
	b1 := a2 + a3
	b2 := a0 - a1
	b3 := a2 - a3
	b4 := a4 + a5
	b5 := a6 + a7
	b6 := a4 - a5
	b7 := a6 - a7
	b8 := a8 + a9
	b9 := a10 + a11
	b10 := a8 - a9
	b11 := a10 - a11
	b12 := a12 + a13
	b13 := a14 + a15
	b14 := a12 - a13
	b15 := a14 - a15

	// Pass 2: process columns.
	a0 = b0 + b4
	a1 = b8 + b12
	a2 = b0 - b4
	a3 = b8 - b12
	a4 = b1 + b5
	a5 = b9 + b13
	a6 = b1 - b5
	a7 = b9 - b13
	a8 = b2 + b6
	a9 = b10 + b14
	a10 = b2 - b6
	a11 = b10 - b14
	a12 = b3 + b7
	a13 = b11 + b15
	a14 = b3 - b7
	a15 = b11 - b15

	adjust := (1 << shift) >> 1

	output[0] = (a0 + a1 + adjust) >> shift
	output[4] = (a2 + a3 + adjust) >> shift
	output[8] = (a0 - a1 + adjust) >> shift
	output[12] = (a2 - a3 + adjust) >> shift
	output[1] = (a4 + a5 + adjust) >> shift
	output[5] = (a6 + a7 + adjust) >> shift
	output[9] = (a4 - a5 + adjust) >> shift
	output[13] = (a6 - a7 + adjust) >> shift
	output[2] = (a8 + a9 + adjust) >> shift
	output[6] = (a10 + a11 + adjust) >> shift
	output[10] = (a8 - a9 + adjust) >> shift
	output[14] = (a10 - a11 + adjust) >> shift
	output[3] = (a12 + a13 + adjust) >> shift
	output[7] = (a14 + a15 + adjust) >> shift
	output[11] = (a12 - a13 + adjust) >> shift
	output[15] = (a14 - a15 + adjust) >> shift

	return 16, 16, nil
}

// The transform is symmetric (except, potentially, for scaling)
func (this *WHT4) Inverse(src, dst []int) (uint, uint, error) {
	if len(src) != 16 {
		return 0, 0, errors.New("Input size must be 16")
	}

	if len(dst) < 16 {
		return 0, 0, errors.New("Output size must be at least 16")
	}

	return this.compute(src, dst, this.iScale)
}
