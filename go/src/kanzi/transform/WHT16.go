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

type WHT16 struct {
	fScale uint
	iScale uint
	data   []int
}

// For perfect reconstruction, forward results are scaled by 8 unless the
// parameter is set to false (in which case rounding may roduce errors)
func NewWHT16(scale bool) (*WHT16, error) {
	this := new(WHT16)
	this.data = make([]int, 256)

	if scale == true {
		this.fScale = 0
		this.iScale = 8
	} else {
		this.fScale = 4
		this.iScale = 4
	}

	return this, nil
}

// For perfect reconstruction, forward results are scaled by 16 unless
// parameter is set to false (in which case rounding may introduce errors)
func (this *WHT16) Forward(src, dst []int) (uint, uint, error) {
	return this.compute(src, dst, this.fScale)
}

func (this *WHT16) compute(input, output []int, shift uint) (uint, uint, error) {
	dataptr := 0
	buffer := this.data // alias

	// Pass 1: process rows.
	for i := 0; i < 256; i += 16 {
		// Aliasing for speed
		x0 := input[i]
		x1 := input[i+1]
		x2 := input[i+2]
		x3 := input[i+3]
		x4 := input[i+4]
		x5 := input[i+5]
		x6 := input[i+6]
		x7 := input[i+7]
		x8 := input[i+8]
		x9 := input[i+9]
		x10 := input[i+10]
		x11 := input[i+11]
		x12 := input[i+12]
		x13 := input[i+13]
		x14 := input[i+14]
		x15 := input[i+15]

		a0 := x0 + x1
		a1 := x2 + x3
		a2 := x4 + x5
		a3 := x6 + x7
		a4 := x8 + x9
		a5 := x10 + x11
		a6 := x12 + x13
		a7 := x14 + x15
		a8 := x0 - x1
		a9 := x2 - x3
		a10 := x4 - x5
		a11 := x6 - x7
		a12 := x8 - x9
		a13 := x10 - x11
		a14 := x12 - x13
		a15 := x14 - x15

		b0 := a0 + a1
		b1 := a2 + a3
		b2 := a4 + a5
		b3 := a6 + a7
		b4 := a8 + a9
		b5 := a10 + a11
		b6 := a12 + a13
		b7 := a14 + a15
		b8 := a0 - a1
		b9 := a2 - a3
		b10 := a4 - a5
		b11 := a6 - a7
		b12 := a8 - a9
		b13 := a10 - a11
		b14 := a12 - a13
		b15 := a14 - a15

		a0 = b0 + b1
		a1 = b2 + b3
		a2 = b4 + b5
		a3 = b6 + b7
		a4 = b8 + b9
		a5 = b10 + b11
		a6 = b12 + b13
		a7 = b14 + b15
		a8 = b0 - b1
		a9 = b2 - b3
		a10 = b4 - b5
		a11 = b6 - b7
		a12 = b8 - b9
		a13 = b10 - b11
		a14 = b12 - b13
		a15 = b14 - b15

		buffer[dataptr] = a0 + a1
		buffer[dataptr+1] = a2 + a3
		buffer[dataptr+2] = a4 + a5
		buffer[dataptr+3] = a6 + a7
		buffer[dataptr+4] = a8 + a9
		buffer[dataptr+5] = a10 + a11
		buffer[dataptr+6] = a12 + a13
		buffer[dataptr+7] = a14 + a15
		buffer[dataptr+8] = a0 - a1
		buffer[dataptr+9] = a2 - a3
		buffer[dataptr+10] = a4 - a5
		buffer[dataptr+11] = a6 - a7
		buffer[dataptr+12] = a8 - a9
		buffer[dataptr+13] = a10 - a11
		buffer[dataptr+14] = a12 - a13
		buffer[dataptr+15] = a14 - a15

		dataptr += 16
	}

	dataptr = 0
	adjust := (1 << shift) >> 1

	// Pass 2: process columns.
	for i := 0; i < 16; i++ {
		// Aliasing for speed
		x0 := buffer[dataptr]
		x1 := buffer[dataptr+16]
		x2 := buffer[dataptr+32]
		x3 := buffer[dataptr+48]
		x4 := buffer[dataptr+64]
		x5 := buffer[dataptr+80]
		x6 := buffer[dataptr+96]
		x7 := buffer[dataptr+112]
		x8 := buffer[dataptr+128]
		x9 := buffer[dataptr+144]
		x10 := buffer[dataptr+160]
		x11 := buffer[dataptr+176]
		x12 := buffer[dataptr+192]
		x13 := buffer[dataptr+208]
		x14 := buffer[dataptr+224]
		x15 := buffer[dataptr+240]

		a0 := x0 + x1
		a1 := x2 + x3
		a2 := x4 + x5
		a3 := x6 + x7
		a4 := x8 + x9
		a5 := x10 + x11
		a6 := x12 + x13
		a7 := x14 + x15
		a8 := x0 - x1
		a9 := x2 - x3
		a10 := x4 - x5
		a11 := x6 - x7
		a12 := x8 - x9
		a13 := x10 - x11
		a14 := x12 - x13
		a15 := x14 - x15

		b0 := a0 + a1
		b1 := a2 + a3
		b2 := a4 + a5
		b3 := a6 + a7
		b4 := a8 + a9
		b5 := a10 + a11
		b6 := a12 + a13
		b7 := a14 + a15
		b8 := a0 - a1
		b9 := a2 - a3
		b10 := a4 - a5
		b11 := a6 - a7
		b12 := a8 - a9
		b13 := a10 - a11
		b14 := a12 - a13
		b15 := a14 - a15

		a0 = b0 + b1
		a1 = b2 + b3
		a2 = b4 + b5
		a3 = b6 + b7
		a4 = b8 + b9
		a5 = b10 + b11
		a6 = b12 + b13
		a7 = b14 + b15
		a8 = b0 - b1
		a9 = b2 - b3
		a10 = b4 - b5
		a11 = b6 - b7
		a12 = b8 - b9
		a13 = b10 - b11
		a14 = b12 - b13
		a15 = b14 - b15

		output[i] = (a0 + a1 + adjust) >> shift
		output[i+16] = (a2 + a3 + adjust) >> shift
		output[i+32] = (a4 + a5 + adjust) >> shift
		output[i+48] = (a6 + a7 + adjust) >> shift
		output[i+64] = (a8 + a9 + adjust) >> shift
		output[i+80] = (a10 + a11 + adjust) >> shift
		output[i+96] = (a12 + a13 + adjust) >> shift
		output[i+112] = (a14 + a15 + adjust) >> shift
		output[i+128] = (a0 - a1 + adjust) >> shift
		output[i+144] = (a2 - a3 + adjust) >> shift
		output[i+160] = (a4 - a5 + adjust) >> shift
		output[i+176] = (a6 - a7 + adjust) >> shift
		output[i+192] = (a8 - a9 + adjust) >> shift
		output[i+208] = (a10 - a11 + adjust) >> shift
		output[i+224] = (a12 - a13 + adjust) >> shift
		output[i+240] = (a14 - a15 + adjust) >> shift

		dataptr++
	}

	return 256, 256, nil
}

// The transform is symmetric (except, potentially, for scaling)
func (this *WHT16) Inverse(src, dst []int) (uint, uint, error) {
	return this.compute(src, dst, this.iScale)
}
