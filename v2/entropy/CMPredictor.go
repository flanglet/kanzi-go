/*
Copyright 2011-2026 Frederic Langlet
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

package entropy

import "fmt"

const (
	_CM_FAST_RATE       = 2
	_CM_MEDIUM_RATE     = 4
	_CM_SLOW_RATE       = 6
	_CM_PSCALE          = 65536
	_CM_COUNTER1_STRIDE = 257
	_CM_COUNTER2_STRIDE = 17
)

type CMPredictor struct {
	c1           byte
	c2           byte
	ctx          int32
	runMask      int32
	counter1     []int32
	counter2     []int32
	idx          int
	isBsVersion3 bool
}

// NewCMPredictor creates a new instance of CMPredictor
func NewCMPredictor(ctx *map[string]any) (*CMPredictor, error) {
	this := &CMPredictor{}
	this.ctx = 1
	this.runMask = 0
	bsVersion := uint(4)

	if ctx != nil {
		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			var ok bool

			if bsVersion, ok = val.(uint); ok == false {
				return nil, fmt.Errorf("CM predictor: invalid bsVersion parameter type")
			}
		}
	}

	this.isBsVersion3 = bsVersion < 4

	this.counter1 = make([]int32, 256*_CM_COUNTER1_STRIDE)
	this.counter2 = make([]int32, 512*_CM_COUNTER2_STRIDE)

	for i := 0; i < 256; i++ {
		counter1Idx := i * _CM_COUNTER1_STRIDE
		counter2Idx := (i + i) * _CM_COUNTER2_STRIDE

		for j := 0; j <= 256; j++ {
			this.counter1[counter1Idx+j] = _CM_PSCALE >> 1
		}

		for j := 0; j < 16; j++ {
			this.counter2[counter2Idx+j] = int32(j << 12)
			this.counter2[counter2Idx+_CM_COUNTER2_STRIDE+j] = int32(j << 12)
		}

		if this.isBsVersion3 == true {
			this.counter2[counter2Idx+16] = int32(15 << 12)
			this.counter2[counter2Idx+_CM_COUNTER2_STRIDE+16] = int32(15 << 12)
		} else {
			this.counter2[counter2Idx+16] = 65535
			this.counter2[counter2Idx+_CM_COUNTER2_STRIDE+16] = 65535
		}
	}

	return this, nil
}

// Update updates the probability model based on the internal bit counters
func (this *CMPredictor) Update(bit byte) {
	pc2 := int(this.ctx|this.runMask) * _CM_COUNTER2_STRIDE
	pc1 := int(this.ctx) * _CM_COUNTER1_STRIDE
	c1 := pc1 + int(this.c1)

	if bit == 0 {
		this.counter1[pc1+256] -= (this.counter1[pc1+256] >> _CM_FAST_RATE)
		this.counter1[c1] -= (this.counter1[c1] >> _CM_MEDIUM_RATE)
		this.counter2[pc2+this.idx] -= (this.counter2[pc2+this.idx] >> _CM_SLOW_RATE)
		this.counter2[pc2+this.idx+1] -= (this.counter2[pc2+this.idx+1] >> _CM_SLOW_RATE)
		this.ctx += this.ctx
	} else {
		this.counter1[pc1+256] -= ((this.counter1[pc1+256] - _CM_PSCALE + 16) >> _CM_FAST_RATE)
		this.counter1[c1] -= ((this.counter1[c1] - _CM_PSCALE + 16) >> _CM_MEDIUM_RATE)
		this.counter2[pc2+this.idx] -= ((this.counter2[pc2+this.idx] - _CM_PSCALE + 16) >> _CM_SLOW_RATE)
		this.counter2[pc2+this.idx+1] -= ((this.counter2[pc2+this.idx+1] - _CM_PSCALE + 16) >> _CM_SLOW_RATE)
		this.ctx += (this.ctx + 1)
	}

	if this.ctx > 255 {
		this.c2 = this.c1
		this.c1 = byte(this.ctx)
		this.ctx = 1

		if this.c1 == this.c2 {
			this.runMask = 0x100
		} else {
			this.runMask = 0
		}
	}
}

// Get returns the value representing the probability of the next bit being 1
// in the [0..4095] range. The probability is computed from the internal
// bit counters.
func (this *CMPredictor) Get() int {
	pc2 := int(this.ctx|this.runMask) * _CM_COUNTER2_STRIDE
	pc1 := int(this.ctx) * _CM_COUNTER1_STRIDE
	p := int(13*(this.counter1[pc1+256]+this.counter1[pc1+int(this.c1)])+6*this.counter1[pc1+int(this.c2)]) >> 5
	this.idx = p >> 12
	x2 := int(this.counter2[pc2+this.idx+1])
	x1 := int(this.counter2[pc2+this.idx])

	if this.isBsVersion3 == true {
		ssep := x1 + (((x2 - x1) * (p & 4095)) >> 12)
		return (p + 3*ssep + 32) >> 6 // rescale to [0..4095]
	}

	return (p + p + 3*(x1+x2) + 64) >> 7 // rescale to [0..4095]
}
