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

package entropy

const (
	PSCALE = 4096
)

// Derived from fpaq0r by Matt Mahoney & Alexander Ratushnyak.
// See http://mattmahoney.net/dc/#fpaq0.
// Simple (and fast) adaptive order 0 entropy coder predictor
type FPAQPredictor struct {
	probs  []int // probability of bit=1
	ctxIdx byte  // previous bits
	p      int
}

func NewFPAQPredictor() (*FPAQPredictor, error) {
	this := new(FPAQPredictor)
	this.ctxIdx = 1
	this.probs = make([]int, 256)
	this.p = PSCALE >> 1

	for i := range this.probs {
		this.probs[i] = PSCALE >> 1
	}

	return this, nil
}

// Update the probability model
// bit == 1 -> prob += (3*((PSCALE-(prob+16))) >> 7);
// bit == 0 -> prob -= (3*(prob+16)) >> 7);
func (this *FPAQPredictor) Update(bit byte) {
	this.probs[this.ctxIdx] -= ((3 * ((this.probs[this.ctxIdx] + 16) - (PSCALE & -int(bit)))) >> 7)

	// Update context by registering the current bit (or wrapping after 8 bits)
	if this.ctxIdx < 128 {
		this.ctxIdx = (this.ctxIdx << 1) | bit
	} else {
		this.ctxIdx = 1
	}

	this.p = this.probs[this.ctxIdx]
}

// Return the split value representing the probability of 1 in the [0..4095] range.
func (this *FPAQPredictor) Get() int {
	return this.p
}
