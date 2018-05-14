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
	kanzi "github.com/flanglet/kanzi-go"
)

// Implementation of Discrete Cosine Transform of dimension 16
// Due to rounding errors, the reconstruction may not be perfect

const (
	W16_0   = 64
	W16_1   = 64
	W16_16  = 90
	W16_17  = 87
	W16_18  = 80
	W16_19  = 70
	W16_20  = 57
	W16_21  = 43
	W16_22  = 25
	W16_23  = 9
	W16_32  = 89
	W16_33  = 75
	W16_34  = 50
	W16_35  = 18
	W16_48  = 87
	W16_49  = 57
	W16_50  = 9
	W16_51  = -43
	W16_52  = -80
	W16_53  = -90
	W16_54  = -70
	W16_55  = -25
	W16_64  = 83
	W16_65  = 36
	W16_80  = 80
	W16_81  = 9
	W16_82  = -70
	W16_83  = -87
	W16_84  = -25
	W16_85  = 57
	W16_86  = 90
	W16_87  = 43
	W16_96  = 75
	W16_97  = -18
	W16_98  = -89
	W16_99  = -50
	W16_112 = 70
	W16_113 = -43
	W16_114 = -87
	W16_115 = 9
	W16_116 = 90
	W16_117 = 25
	W16_118 = -80
	W16_119 = -57
	W16_128 = 64
	W16_129 = -64
	W16_144 = 57
	W16_145 = -80
	W16_146 = -25
	W16_147 = 90
	W16_148 = -9
	W16_149 = -87
	W16_150 = 43
	W16_151 = 70
	W16_160 = 50
	W16_161 = -89
	W16_162 = 18
	W16_163 = 75
	W16_176 = 43
	W16_177 = -90
	W16_178 = 57
	W16_179 = 25
	W16_180 = -87
	W16_181 = 70
	W16_182 = 9
	W16_183 = -80
	W16_192 = 36
	W16_193 = -83
	W16_208 = 25
	W16_209 = -70
	W16_210 = 90
	W16_211 = -80
	W16_212 = 43
	W16_213 = 9
	W16_214 = -57
	W16_215 = 87
	W16_224 = 18
	W16_225 = -50
	W16_226 = 75
	W16_227 = -89
	W16_240 = 9
	W16_241 = -25
	W16_242 = 43
	W16_243 = -57
	W16_244 = 70
	W16_245 = -80
	W16_246 = 87
	W16_247 = -90

	MAX_VAL_DCT16 = 1 << 16
	MIN_VAL_DCT16 = -(MAX_VAL_DCT16 + 1)
)

type DCT16 struct {
	fShift uint  // default 12
	iShift uint  // default 20
	data   []int // int[256]
}

func NewDCT16() (*DCT16, error) {
	this := new(DCT16)
	this.fShift = 12
	this.iShift = 20
	this.data = make([]int, 256)
	return this, nil
}

func (this *DCT16) Forward(src, dst []int) (uint, uint, error) {
	if len(src) != 256 {
		return 0, 0, errors.New("Input size must be 256")
	}

	if len(dst) < 256 {
		return 0, 0, errors.New("Output size must be at least 256")
	}

	computeForwardDCT16(src, this.data, 6)
	computeForwardDCT16(this.data, dst, this.fShift-6)
	return 256, 256, nil
}

func computeForwardDCT16(input, output []int, shift uint) {
	iIdx := 0
	round := (1 << shift) >> 1
	in := input[0:256]
	out := output[0:256]

	for i := 0; i < 16; i++ {
		x0 := in[iIdx]
		x1 := in[iIdx+1]
		x2 := in[iIdx+2]
		x3 := in[iIdx+3]
		x4 := in[iIdx+4]
		x5 := in[iIdx+5]
		x6 := in[iIdx+6]
		x7 := in[iIdx+7]
		x8 := in[iIdx+8]
		x9 := in[iIdx+9]
		x10 := in[iIdx+10]
		x11 := in[iIdx+11]
		x12 := in[iIdx+12]
		x13 := in[iIdx+13]
		x14 := in[iIdx+14]
		x15 := in[iIdx+15]

		a0 := x0 + x15
		a1 := x1 + x14
		a2 := x0 - x15
		a3 := x1 - x14
		a4 := x2 + x13
		a5 := x3 + x12
		a6 := x2 - x13
		a7 := x3 - x12
		a8 := x4 + x11
		a9 := x5 + x10
		a10 := x4 - x11
		a11 := x5 - x10
		a12 := x6 + x9
		a13 := x7 + x8
		a14 := x6 - x9
		a15 := x7 - x8

		b0 := a0 + a13
		b1 := a1 + a12
		b2 := a0 - a13
		b3 := a1 - a12
		b4 := a4 + a9
		b5 := a5 + a8
		b6 := a4 - a9
		b7 := a5 - a8

		c0 := b0 + b5
		c1 := b1 + b4
		c2 := b0 - b5
		c3 := b1 - b4

		out[i] = ((W16_0 * c0) + (W16_1 * c1) + round) >> shift
		out[i+16] = ((W16_16 * a2) + (W16_17 * a3) + (W16_18 * a6) + (W16_19 * a7) +
			(W16_20 * a10) + (W16_21 * a11) + (W16_22 * a14) + (W16_23 * a15) + round) >> shift
		out[i+32] = ((W16_32 * b2) + (W16_33 * b3) + (W16_34 * b6) + (W16_35 * b7) + round) >> shift
		out[i+48] = ((W16_48 * a2) + (W16_49 * a3) + (W16_50 * a6) + (W16_51 * a7) +
			(W16_52 * a10) + (W16_53 * a11) + (W16_54 * a14) + (W16_55 * a15) + round) >> shift
		out[i+64] = ((W16_64 * c2) + (W16_65 * c3) + round) >> shift
		out[i+80] = ((W16_80 * a2) + (W16_81 * a3) + (W16_82 * a6) + (W16_83 * a7) +
			(W16_84 * a10) + (W16_85 * a11) + (W16_86 * a14) + (W16_87 * a15) + round) >> shift
		out[i+96] = ((W16_96 * b2) + (W16_97 * b3) + (W16_98 * b6) + (W16_99 * b7) + round) >> shift
		out[i+112] = ((W16_112 * a2) + (W16_113 * a3) + (W16_114 * a6) + (W16_115 * a7) +
			(W16_116 * a10) + (W16_117 * a11) + (W16_118 * a14) + (W16_119 * a15) + round) >> shift
		out[i+128] = ((W16_128 * c0) + (W16_129 * c1) + round) >> shift
		out[i+144] = ((W16_144 * a2) + (W16_145 * a3) + (W16_146 * a6) + (W16_147 * a7) +
			(W16_148 * a10) + (W16_149 * a11) + (W16_150 * a14) + (W16_151 * a15) + round) >> shift
		out[i+160] = ((W16_160 * b2) + (W16_161 * b3) + (W16_162 * b6) + (W16_163 * b7) + round) >> shift
		out[i+176] = ((W16_176 * a2) + (W16_177 * a3) + (W16_178 * a6) + (W16_179 * a7) +
			(W16_180 * a10) + (W16_181 * a11) + (W16_182 * a14) + (W16_183 * a15) + round) >> shift
		out[i+192] = ((W16_192 * c2) + (W16_193 * c3) + round) >> shift
		out[i+208] = ((W16_208 * a2) + (W16_209 * a3) + (W16_210 * a6) + (W16_211 * a7) +
			(W16_212 * a10) + (W16_213 * a11) + (W16_214 * a14) + (W16_215 * a15) + round) >> shift
		out[i+224] = ((W16_224 * b2) + (W16_225 * b3) + (W16_226 * b6) + (W16_227 * b7) + round) >> shift
		out[i+240] = ((W16_240 * a2) + (W16_241 * a3) + (W16_242 * a6) + (W16_243 * a7) +
			(W16_244 * a10) + (W16_245 * a11) + (W16_246 * a14) + (W16_247 * a15) + round) >> shift

		iIdx += 16
	}

}

func (this *DCT16) Inverse(src, dst []int) (uint, uint, error) {
	if len(src) != 256 {
		return 0, 0, errors.New("Input size must be 256")
	}

	if len(dst) < 256 {
		return 0, 0, errors.New("Output size must be at least 256")
	}

	computeInverseDCT16(src, this.data, 10)
	computeInverseDCT16(this.data, dst, this.iShift-10)
	return 256, 256, nil
}

func computeInverseDCT16(input, output []int, shift uint) {
	oIdx := 0
	round := (1 << shift) >> 1
	in := input[0:256]
	out := output[0:256]

	for i := 0; i < 16; i++ {
		x0 := in[i]
		x1 := in[i+16]
		x2 := in[i+32]
		x3 := in[i+48]
		x4 := in[i+64]
		x5 := in[i+80]
		x6 := in[i+96]
		x7 := in[i+112]
		x8 := in[i+128]
		x9 := in[i+144]
		x10 := in[i+160]
		x11 := in[i+176]
		x12 := in[i+192]
		x13 := in[i+208]
		x14 := in[i+224]
		x15 := in[i+240]

		a0 := (W16_16 * x1) + (W16_48 * x3) + (W16_80 * x5) + (W16_112 * x7) +
			(W16_144 * x9) + (W16_176 * x11) + (W16_208 * x13) + (W16_240 * x15)
		a1 := (W16_17 * x1) + (W16_49 * x3) + (W16_81 * x5) + (W16_113 * x7) +
			(W16_145 * x9) + (W16_177 * x11) + (W16_209 * x13) + (W16_241 * x15)
		a2 := (W16_18 * x1) + (W16_50 * x3) + (W16_82 * x5) + (W16_114 * x7) +
			(W16_146 * x9) + (W16_178 * x11) + (W16_210 * x13) + (W16_242 * x15)
		a3 := (W16_19 * x1) + (W16_51 * x3) + (W16_83 * x5) + (W16_115 * x7) +
			(W16_147 * x9) + (W16_179 * x11) + (W16_211 * x13) + (W16_243 * x15)
		a4 := (W16_20 * x1) + (W16_52 * x3) + (W16_84 * x5) + (W16_116 * x7) +
			(W16_148 * x9) + (W16_180 * x11) + (W16_212 * x13) + (W16_244 * x15)
		a5 := (W16_21 * x1) + (W16_53 * x3) + (W16_85 * x5) + (W16_117 * x7) +
			(W16_149 * x9) + (W16_181 * x11) + (W16_213 * x13) + (W16_245 * x15)
		a6 := (W16_22 * x1) + (W16_54 * x3) + (W16_86 * x5) + (W16_118 * x7) +
			(W16_150 * x9) + (W16_182 * x11) + (W16_214 * x13) + (W16_246 * x15)
		a7 := (W16_23 * x1) + (W16_55 * x3) + (W16_87 * x5) + (W16_119 * x7) +
			(W16_151 * x9) + (W16_183 * x11) + (W16_215 * x13) + (W16_247 * x15)

		b0 := (W16_32 * x2) + (W16_96 * x6) + (W16_160 * x10) + (W16_224 * x14)
		b1 := (W16_33 * x2) + (W16_97 * x6) + (W16_161 * x10) + (W16_225 * x14)
		b2 := (W16_34 * x2) + (W16_98 * x6) + (W16_162 * x10) + (W16_226 * x14)
		b3 := (W16_35 * x2) + (W16_99 * x6) + (W16_163 * x10) + (W16_227 * x14)
		b4 := (W16_0 * x0) + (W16_128 * x8) + (W16_64 * x4) + (W16_192 * x12)
		b5 := (W16_0 * x0) + (W16_128 * x8) - (W16_64 * x4) - (W16_192 * x12)
		b6 := (W16_1 * x0) + (W16_129 * x8) + (W16_65 * x4) + (W16_193 * x12)
		b7 := (W16_1 * x0) + (W16_129 * x8) - (W16_65 * x4) - (W16_193 * x12)

		c0 := b4 + b0
		c1 := b6 + b1
		c2 := b7 + b2
		c3 := b5 + b3
		c4 := b5 - b3
		c5 := b7 - b2
		c6 := b6 - b1
		c7 := b4 - b0

		d0 := (c0 + a0 + round) >> shift
		d1 := (c1 + a1 + round) >> shift
		d2 := (c2 + a2 + round) >> shift
		d3 := (c3 + a3 + round) >> shift
		d4 := (c4 + a4 + round) >> shift
		d5 := (c5 + a5 + round) >> shift
		d6 := (c6 + a6 + round) >> shift
		d7 := (c7 + a7 + round) >> shift
		d8 := (c7 - a7 + round) >> shift
		d9 := (c6 - a6 + round) >> shift
		d10 := (c5 - a5 + round) >> shift
		d11 := (c4 - a4 + round) >> shift
		d12 := (c3 - a3 + round) >> shift
		d13 := (c2 - a2 + round) >> shift
		d14 := (c1 - a1 + round) >> shift
		d15 := (c0 - a0 + round) >> shift

		out[oIdx] = kanzi.Clamp(d0, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+1] = kanzi.Clamp(d1, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+2] = kanzi.Clamp(d2, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+3] = kanzi.Clamp(d3, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+4] = kanzi.Clamp(d4, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+5] = kanzi.Clamp(d5, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+6] = kanzi.Clamp(d6, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+7] = kanzi.Clamp(d7, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+8] = kanzi.Clamp(d8, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+9] = kanzi.Clamp(d9, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+10] = kanzi.Clamp(d10, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+11] = kanzi.Clamp(d11, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+12] = kanzi.Clamp(d12, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+13] = kanzi.Clamp(d13, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+14] = kanzi.Clamp(d14, MIN_VAL_DCT16, MAX_VAL_DCT16)
		out[oIdx+15] = kanzi.Clamp(d15, MIN_VAL_DCT16, MAX_VAL_DCT16)

		oIdx += 16
	}
}
