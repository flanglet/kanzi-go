/*
Copyright 2011-2021 Frederic Langlet
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

const (
	_CM_FAST_RATE   = 2
	_CM_MEDIUM_RATE = 4
	_CM_SLOW_RATE   = 6
	_CM_PSCALE      = 65536
)

// CMPredictor context model predictor based on BCM by Ilya Muravyov.
// See https://github.com/encode84/bcm
type CMPredictor struct {
	c1       byte
	c2       byte
	ctx      int32
	idx      int
	runMask  int32
	counter1 [256][]int32
	counter2 [512][]int32
	p        int
}

// NewCMPredictor creates a new instance of CMPredictor
func NewCMPredictor() (*CMPredictor, error) {
	this := &CMPredictor{}
	this.ctx = 1
	this.runMask = 0

	for i := 0; i < 256; i++ {
		this.counter1[i] = make([]int32, 257)
		this.counter2[i+i] = make([]int32, 17)
		this.counter2[i+i+1] = make([]int32, 17)

		for j := 0; j <= 256; j++ {
			this.counter1[i][j] = _CM_PSCALE >> 1
		}

		for j := 0; j < 16; j++ {
			this.counter2[i+i][j] = int32(j << 12)
			this.counter2[i+i+1][j] = int32(j << 12)
		}

		this.counter2[i+i][16] = int32(15 << 12)
		this.counter2[i+i+1][16] = int32(15 << 12)
	}

	pc1 := this.counter1[this.ctx]
	this.p = int(13*(pc1[256]+pc1[this.c1])+6*pc1[this.c2]) >> 5
	this.idx = this.p >> 12
	return this, nil
}

// Update updates the probability model based on the internal bit counters
func (this *CMPredictor) Update(bit byte) {
	pc1 := this.counter1[this.ctx]
	pc2 := this.counter2[this.ctx|this.runMask]
	this.ctx += (this.ctx + int32(bit))

	if bit == 0 {
		pc1[256] -= (pc1[256] >> _CM_FAST_RATE)
		pc1[this.c1] -= (pc1[this.c1] >> _CM_MEDIUM_RATE)
		pc2[this.idx] -= (pc2[this.idx] >> _CM_SLOW_RATE)
		pc2[this.idx+1] -= (pc2[this.idx+1] >> _CM_SLOW_RATE)
	} else {
		pc1[256] -= ((pc1[256] - _CM_PSCALE + 16) >> _CM_FAST_RATE)
		pc1[this.c1] -= ((pc1[this.c1] - _CM_PSCALE + 16) >> _CM_MEDIUM_RATE)
		pc2[this.idx] -= ((pc2[this.idx] - _CM_PSCALE + 16) >> _CM_SLOW_RATE)
		pc2[this.idx+1] -= ((pc2[this.idx+1] - _CM_PSCALE + 16) >> _CM_SLOW_RATE)
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

	pc1 = this.counter1[this.ctx]
	this.p = int(13*(pc1[256]+pc1[this.c1])+6*pc1[this.c2]) >> 5
	this.idx = this.p >> 12
}

// Get returns the value representing the probability of the next bit being 1
// in the [0..4095] range. The probability is computed from the internal
// bit counters.
func (this *CMPredictor) Get() int {
	pc2 := this.counter2[this.ctx|this.runMask]
	x2 := int(pc2[this.idx+1])
	x1 := int(pc2[this.idx])
	ssep := x1 + (((x2 - x1) * (this.p & 4095)) >> 12)
	return (this.p + 3*ssep + 32) >> 6 // rescale to [0..4095]
}
