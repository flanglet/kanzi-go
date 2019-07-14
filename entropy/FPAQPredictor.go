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

package entropy

const (
	_PSCALE = 1 << 16
)

// FPAQPredictor Fast PAQ Predictor
// Derived from fpaq0r by Matt Mahoney & Alexander Ratushnyak.
// See http://mattmahoney.net/dc/#fpaq0.
// Simple (and fast) adaptive order 0 entropy coder predictor
type FPAQPredictor struct {
	probs  [256]int // probability of bit=1
	ctxIdx byte     // previous bits
}

// NewFPAQPredictor creates a new instance of FPAQPredictor
func NewFPAQPredictor() (*FPAQPredictor, error) {
	this := &FPAQPredictor{}
	this.ctxIdx = 1

	for i := range this.probs {
		this.probs[i] = _PSCALE >> 1
	}

	return this, nil
}

// Update updates the internal probability model based on the observed bit
// bit == 1 -> prob += ((PSCALE-prob) >> 6);
// bit == 0 -> prob -= (prob >> 6);
func (this *FPAQPredictor) Update(bit byte) {
	this.probs[this.ctxIdx] -= (((this.probs[this.ctxIdx] - (-int(bit) & _PSCALE)) >> 6) + int(bit))

	// Update context by registering the current bit (or wrapping after 8 bits)
	if this.ctxIdx < 128 {
		this.ctxIdx = (this.ctxIdx << 1) + bit
	} else {
		this.ctxIdx = 1
	}
}

// Get returns the value representing the probability of the next bit being 1
// in the [0..4095] range.
// E.G. 410 represents roughly a probability of 10% for 1
func (this *FPAQPredictor) Get() int {
	return this.probs[this.ctxIdx] >> 4
}
