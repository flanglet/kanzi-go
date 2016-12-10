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

type WHT32 struct {
	fScale uint
	iScale uint
	data   []int
}

// For perfect reconstruction, forward results are scaled by 16*sqrt(2) unless
// the parameter is set to false (scaled by sqrt(2), in which case rounding
// may introduce errors)
func NewWHT32(scale bool) (*WHT32, error) {
	this := new(WHT32)
	this.data = make([]int, 1024)

	if scale == true {
		this.fScale = 0
		this.iScale = 10
	} else {
		this.fScale = 5
		this.iScale = 5
	}

	return this, nil
}

// For perfect reconstruction, forward results are scaled by 16*sqrt(2) unless
// the parameter is set to false (scaled by sqrt(2), in which case rounding
// may introduce errors)
func (this *WHT32) Forward(src, dst []int) (uint, uint, error) {
	if len(src) != 1024 {
		return 0, 0, errors.New("Input size must be 1024")
	}

	if len(dst) < 1024 {
		return 0, 0, errors.New("Output size must be at least 1024")
	}

	return this.compute(src, dst, this.fScale)
}

func (this *WHT32) compute(input, output []int, shift uint) (uint, uint, error) {
	processRows(input, this.data)
	processColumns(this.data, output, shift)
	return 1024, 1024, nil
}

func processRows(input, buffer []int) {
	dataptr := 0

	// Pass 1: process rows.
	for i := 0; i < 1024; i += 32 {
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
		x16 := input[i+16]
		x17 := input[i+17]
		x18 := input[i+18]
		x19 := input[i+19]
		x20 := input[i+20]
		x21 := input[i+21]
		x22 := input[i+22]
		x23 := input[i+23]
		x24 := input[i+24]
		x25 := input[i+25]
		x26 := input[i+26]
		x27 := input[i+27]
		x28 := input[i+28]
		x29 := input[i+29]
		x30 := input[i+30]
		x31 := input[i+31]

		a0 := x0 + x1
		a1 := x2 + x3
		a2 := x4 + x5
		a3 := x6 + x7
		a4 := x8 + x9
		a5 := x10 + x11
		a6 := x12 + x13
		a7 := x14 + x15
		a8 := x16 + x17
		a9 := x18 + x19
		a10 := x20 + x21
		a11 := x22 + x23
		a12 := x24 + x25
		a13 := x26 + x27
		a14 := x28 + x29
		a15 := x30 + x31
		a16 := x0 - x1
		a17 := x2 - x3
		a18 := x4 - x5
		a19 := x6 - x7
		a20 := x8 - x9
		a21 := x10 - x11
		a22 := x12 - x13
		a23 := x14 - x15
		a24 := x16 - x17
		a25 := x18 - x19
		a26 := x20 - x21
		a27 := x22 - x23
		a28 := x24 - x25
		a29 := x26 - x27
		a30 := x28 - x29
		a31 := x30 - x31

		b0 := a0 + a1
		b1 := a2 + a3
		b2 := a4 + a5
		b3 := a6 + a7
		b4 := a8 + a9
		b5 := a10 + a11
		b6 := a12 + a13
		b7 := a14 + a15
		b8 := a16 + a17
		b9 := a18 + a19
		b10 := a20 + a21
		b11 := a22 + a23
		b12 := a24 + a25
		b13 := a26 + a27
		b14 := a28 + a29
		b15 := a30 + a31
		b16 := a0 - a1
		b17 := a2 - a3
		b18 := a4 - a5
		b19 := a6 - a7
		b20 := a8 - a9
		b21 := a10 - a11
		b22 := a12 - a13
		b23 := a14 - a15
		b24 := a16 - a17
		b25 := a18 - a19
		b26 := a20 - a21
		b27 := a22 - a23
		b28 := a24 - a25
		b29 := a26 - a27
		b30 := a28 - a29
		b31 := a30 - a31

		a0 = b0 + b1
		a1 = b2 + b3
		a2 = b4 + b5
		a3 = b6 + b7
		a4 = b8 + b9
		a5 = b10 + b11
		a6 = b12 + b13
		a7 = b14 + b15
		a8 = b16 + b17
		a9 = b18 + b19
		a10 = b20 + b21
		a11 = b22 + b23
		a12 = b24 + b25
		a13 = b26 + b27
		a14 = b28 + b29
		a15 = b30 + b31
		a16 = b0 - b1
		a17 = b2 - b3
		a18 = b4 - b5
		a19 = b6 - b7
		a20 = b8 - b9
		a21 = b10 - b11
		a22 = b12 - b13
		a23 = b14 - b15
		a24 = b16 - b17
		a25 = b18 - b19
		a26 = b20 - b21
		a27 = b22 - b23
		a28 = b24 - b25
		a29 = b26 - b27
		a30 = b28 - b29
		a31 = b30 - b31

		b0 = a0 + a1
		b1 = a2 + a3
		b2 = a4 + a5
		b3 = a6 + a7
		b4 = a8 + a9
		b5 = a10 + a11
		b6 = a12 + a13
		b7 = a14 + a15
		b8 = a16 + a17
		b9 = a18 + a19
		b10 = a20 + a21
		b11 = a22 + a23
		b12 = a24 + a25
		b13 = a26 + a27
		b14 = a28 + a29
		b15 = a30 + a31
		b16 = a0 - a1
		b17 = a2 - a3
		b18 = a4 - a5
		b19 = a6 - a7
		b20 = a8 - a9
		b21 = a10 - a11
		b22 = a12 - a13
		b23 = a14 - a15
		b24 = a16 - a17
		b25 = a18 - a19
		b26 = a20 - a21
		b27 = a22 - a23
		b28 = a24 - a25
		b29 = a26 - a27
		b30 = a28 - a29
		b31 = a30 - a31

		buffer[dataptr] = b0 + b1
		buffer[dataptr+1] = b2 + b3
		buffer[dataptr+2] = b4 + b5
		buffer[dataptr+3] = b6 + b7
		buffer[dataptr+4] = b8 + b9
		buffer[dataptr+5] = b10 + b11
		buffer[dataptr+6] = b12 + b13
		buffer[dataptr+7] = b14 + b15
		buffer[dataptr+8] = b16 + b17
		buffer[dataptr+9] = b18 + b19
		buffer[dataptr+10] = b20 + b21
		buffer[dataptr+11] = b22 + b23
		buffer[dataptr+12] = b24 + b25
		buffer[dataptr+13] = b26 + b27
		buffer[dataptr+14] = b28 + b29
		buffer[dataptr+15] = b30 + b31
		buffer[dataptr+16] = b0 - b1
		buffer[dataptr+17] = b2 - b3
		buffer[dataptr+18] = b4 - b5
		buffer[dataptr+19] = b6 - b7
		buffer[dataptr+20] = b8 - b9
		buffer[dataptr+21] = b10 - b11
		buffer[dataptr+22] = b12 - b13
		buffer[dataptr+23] = b14 - b15
		buffer[dataptr+24] = b16 - b17
		buffer[dataptr+25] = b18 - b19
		buffer[dataptr+26] = b20 - b21
		buffer[dataptr+27] = b22 - b23
		buffer[dataptr+28] = b24 - b25
		buffer[dataptr+29] = b26 - b27
		buffer[dataptr+30] = b28 - b29
		buffer[dataptr+31] = b30 - b31

		dataptr += 32
	}
}

func processColumns(buffer, output []int, shift uint) {
	dataptr := 0
	adjust := (1 << shift) >> 1

	// Pass 2: process columns.
	for i := 0; i < 32; i++ {
		// Aliasing for speed
		x0 := buffer[dataptr]
		x1 := buffer[dataptr+32]
		x2 := buffer[dataptr+64]
		x3 := buffer[dataptr+96]
		x4 := buffer[dataptr+128]
		x5 := buffer[dataptr+160]
		x6 := buffer[dataptr+192]
		x7 := buffer[dataptr+224]
		x8 := buffer[dataptr+256]
		x9 := buffer[dataptr+288]
		x10 := buffer[dataptr+320]
		x11 := buffer[dataptr+352]
		x12 := buffer[dataptr+384]
		x13 := buffer[dataptr+416]
		x14 := buffer[dataptr+448]
		x15 := buffer[dataptr+480]
		x16 := buffer[dataptr+512]
		x17 := buffer[dataptr+544]
		x18 := buffer[dataptr+576]
		x19 := buffer[dataptr+608]
		x20 := buffer[dataptr+640]
		x21 := buffer[dataptr+672]
		x22 := buffer[dataptr+704]
		x23 := buffer[dataptr+736]
		x24 := buffer[dataptr+768]
		x25 := buffer[dataptr+800]
		x26 := buffer[dataptr+832]
		x27 := buffer[dataptr+864]
		x28 := buffer[dataptr+896]
		x29 := buffer[dataptr+928]
		x30 := buffer[dataptr+960]
		x31 := buffer[dataptr+992]

		a0 := x0 + x1
		a1 := x2 + x3
		a2 := x4 + x5
		a3 := x6 + x7
		a4 := x8 + x9
		a5 := x10 + x11
		a6 := x12 + x13
		a7 := x14 + x15
		a8 := x16 + x17
		a9 := x18 + x19
		a10 := x20 + x21
		a11 := x22 + x23
		a12 := x24 + x25
		a13 := x26 + x27
		a14 := x28 + x29
		a15 := x30 + x31
		a16 := x0 - x1
		a17 := x2 - x3
		a18 := x4 - x5
		a19 := x6 - x7
		a20 := x8 - x9
		a21 := x10 - x11
		a22 := x12 - x13
		a23 := x14 - x15
		a24 := x16 - x17
		a25 := x18 - x19
		a26 := x20 - x21
		a27 := x22 - x23
		a28 := x24 - x25
		a29 := x26 - x27
		a30 := x28 - x29
		a31 := x30 - x31

		b0 := a0 + a1
		b1 := a2 + a3
		b2 := a4 + a5
		b3 := a6 + a7
		b4 := a8 + a9
		b5 := a10 + a11
		b6 := a12 + a13
		b7 := a14 + a15
		b8 := a16 + a17
		b9 := a18 + a19
		b10 := a20 + a21
		b11 := a22 + a23
		b12 := a24 + a25
		b13 := a26 + a27
		b14 := a28 + a29
		b15 := a30 + a31
		b16 := a0 - a1
		b17 := a2 - a3
		b18 := a4 - a5
		b19 := a6 - a7
		b20 := a8 - a9
		b21 := a10 - a11
		b22 := a12 - a13
		b23 := a14 - a15
		b24 := a16 - a17
		b25 := a18 - a19
		b26 := a20 - a21
		b27 := a22 - a23
		b28 := a24 - a25
		b29 := a26 - a27
		b30 := a28 - a29
		b31 := a30 - a31

		a0 = b0 + b1
		a1 = b2 + b3
		a2 = b4 + b5
		a3 = b6 + b7
		a4 = b8 + b9
		a5 = b10 + b11
		a6 = b12 + b13
		a7 = b14 + b15
		a8 = b16 + b17
		a9 = b18 + b19
		a10 = b20 + b21
		a11 = b22 + b23
		a12 = b24 + b25
		a13 = b26 + b27
		a14 = b28 + b29
		a15 = b30 + b31
		a16 = b0 - b1
		a17 = b2 - b3
		a18 = b4 - b5
		a19 = b6 - b7
		a20 = b8 - b9
		a21 = b10 - b11
		a22 = b12 - b13
		a23 = b14 - b15
		a24 = b16 - b17
		a25 = b18 - b19
		a26 = b20 - b21
		a27 = b22 - b23
		a28 = b24 - b25
		a29 = b26 - b27
		a30 = b28 - b29
		a31 = b30 - b31

		b0 = a0 + a1
		b1 = a2 + a3
		b2 = a4 + a5
		b3 = a6 + a7
		b4 = a8 + a9
		b5 = a10 + a11
		b6 = a12 + a13
		b7 = a14 + a15
		b8 = a16 + a17
		b9 = a18 + a19
		b10 = a20 + a21
		b11 = a22 + a23
		b12 = a24 + a25
		b13 = a26 + a27
		b14 = a28 + a29
		b15 = a30 + a31
		b16 = a0 - a1
		b17 = a2 - a3
		b18 = a4 - a5
		b19 = a6 - a7
		b20 = a8 - a9
		b21 = a10 - a11
		b22 = a12 - a13
		b23 = a14 - a15
		b24 = a16 - a17
		b25 = a18 - a19
		b26 = a20 - a21
		b27 = a22 - a23
		b28 = a24 - a25
		b29 = a26 - a27
		b30 = a28 - a29
		b31 = a30 - a31

		output[i] = (b0 + b1 + adjust) >> shift
		output[i+32] = (b2 + b3 + adjust) >> shift
		output[i+64] = (b4 + b5 + adjust) >> shift
		output[i+96] = (b6 + b7 + adjust) >> shift
		output[i+128] = (b8 + b9 + adjust) >> shift
		output[i+160] = (b10 + b11 + adjust) >> shift
		output[i+192] = (b12 + b13 + adjust) >> shift
		output[i+224] = (b14 + b15 + adjust) >> shift
		output[i+256] = (b16 + b17 + adjust) >> shift
		output[i+288] = (b18 + b19 + adjust) >> shift
		output[i+320] = (b20 + b21 + adjust) >> shift
		output[i+352] = (b22 + b23 + adjust) >> shift
		output[i+384] = (b24 + b25 + adjust) >> shift
		output[i+416] = (b26 + b27 + adjust) >> shift
		output[i+448] = (b28 + b29 + adjust) >> shift
		output[i+480] = (b30 + b31 + adjust) >> shift
		output[i+512] = (b0 - b1 + adjust) >> shift
		output[i+544] = (b2 - b3 + adjust) >> shift
		output[i+576] = (b4 - b5 + adjust) >> shift
		output[i+608] = (b6 - b7 + adjust) >> shift
		output[i+640] = (b8 - b9 + adjust) >> shift
		output[i+672] = (b10 - b11 + adjust) >> shift
		output[i+704] = (b12 - b13 + adjust) >> shift
		output[i+736] = (b14 - b15 + adjust) >> shift
		output[i+768] = (b16 - b17 + adjust) >> shift
		output[i+800] = (b18 - b19 + adjust) >> shift
		output[i+832] = (b20 - b21 + adjust) >> shift
		output[i+864] = (b22 - b23 + adjust) >> shift
		output[i+896] = (b24 - b25 + adjust) >> shift
		output[i+928] = (b26 - b27 + adjust) >> shift
		output[i+960] = (b28 - b29 + adjust) >> shift
		output[i+992] = (b30 - b31 + adjust) >> shift

		dataptr++
	}
}

// The transform is symmetric (except, potentially, for scaling)
func (this *WHT32) Inverse(src, dst []int) (uint, uint, error) {
	if len(src) != 1024 {
		return 0, 0, errors.New("Input size must be 1024")
	}

	if len(dst) < 1024 {
		return 0, 0, errors.New("Output size must be at least 1024")
	}

	return this.compute(src, dst, this.iScale)
}
