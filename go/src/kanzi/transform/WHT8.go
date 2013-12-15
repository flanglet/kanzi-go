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

type WHT8 struct {
	fScale uint
	iScale uint
	data   []int
}

// For perfect reconstruction, forward results are scaled by 8 unless the
// parameter is set to false (in which case rounding may introduce errors)
func NewWHT8(scale bool) (*WHT8, error) {
	this := new(WHT8)
	this.data = make([]int, 64)

	if scale == true {
		this.fScale = 0
		this.iScale = 6
	} else {
		this.fScale = 3
		this.iScale = 3
	}

	return this, nil
}

// For perfect reconstruction, forward results are scaled by 8*sqrt(2) unless
// the parameter is set to false (scaled by sqrt(2), in which case rounding
// may introduce errors)
func (this *WHT8) Forward(src, dst []int) (uint, uint, error) {
	return this.compute(src, dst, this.fScale)
}

func (this *WHT8) compute(input, output []int, shift uint) (uint, uint, error) {
	dataptr := 0
	buffer := this.data // alias

	// Pass 1: process rows.
	for i := 0; i < 64; i += 8 {
		// Aliasing for speed
		x0 := input[i]
		x1 := input[i+1]
		x2 := input[i+2]
		x3 := input[i+3]
		x4 := input[i+4]
		x5 := input[i+5]
		x6 := input[i+6]
		x7 := input[i+7]

		a0 := x0 + x1
		a1 := x2 + x3
		a2 := x4 + x5
		a3 := x6 + x7
		a4 := x0 - x1
		a5 := x2 - x3
		a6 := x4 - x5
		a7 := x6 - x7

		b0 := a0 + a1
		b1 := a2 + a3
		b2 := a4 + a5
		b3 := a6 + a7
		b4 := a0 - a1
		b5 := a2 - a3
		b6 := a4 - a5
		b7 := a6 - a7

		buffer[dataptr] = b0 + b1
		buffer[dataptr+1] = b2 + b3
		buffer[dataptr+2] = b4 + b5
		buffer[dataptr+3] = b6 + b7
		buffer[dataptr+4] = b0 - b1
		buffer[dataptr+5] = b2 - b3
		buffer[dataptr+6] = b4 - b5
		buffer[dataptr+7] = b6 - b7

		dataptr += 8
	}

	dataptr = 0
	adjust := (1 << shift) >> 1

	// Pass 2: process columns.
	for i := 0; i < 8; i++ {
		// Aliasing for speed
		x0 := buffer[dataptr]
		x1 := buffer[dataptr+8]
		x2 := buffer[dataptr+16]
		x3 := buffer[dataptr+24]
		x4 := buffer[dataptr+32]
		x5 := buffer[dataptr+40]
		x6 := buffer[dataptr+48]
		x7 := buffer[dataptr+56]

		a0 := x0 + x1
		a1 := x2 + x3
		a2 := x4 + x5
		a3 := x6 + x7
		a4 := x0 - x1
		a5 := x2 - x3
		a6 := x4 - x5
		a7 := x6 - x7

		b0 := a0 + a1
		b1 := a2 + a3
		b2 := a4 + a5
		b3 := a6 + a7
		b4 := a0 - a1
		b5 := a2 - a3
		b6 := a4 - a5
		b7 := a6 - a7

		output[i] = (b0 + b1 + adjust) >> shift
		output[i+8] = (b2 + b3 + adjust) >> shift
		output[i+16] = (b4 + b5 + adjust) >> shift
		output[i+24] = (b6 + b7 + adjust) >> shift
		output[i+32] = (b0 - b1 + adjust) >> shift
		output[i+40] = (b2 - b3 + adjust) >> shift
		output[i+48] = (b4 - b5 + adjust) >> shift
		output[i+56] = (b6 - b7 + adjust) >> shift

		dataptr++
	}

	return 64, 64, nil
}

// The transform is symmetric (except, potentially, for scaling)
func (this *WHT8) Inverse(src, dst []int) (uint, uint, error) {
	return this.compute(src, dst, this.iScale)
}
