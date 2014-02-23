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
	THRESHOLD = 200
	SHIFT     = 1
)

// Based on fpaq1 by Matt Mahoney
// Simple (and fast) adaptive order 0 entropy coder predictor
type FPAQPredictor struct {
	ctxIdx int           // previous bits
	states []uint16 // 256 frequency contexts for each bit
}

func NewFPAQPredictor() (*FPAQPredictor, error) {
	this := new(FPAQPredictor)
	this.ctxIdx = 1
	this.states = make([]uint16, 512)
	return this, nil
}

// Used to update the probability model
func (this *FPAQPredictor) Update(bit byte) {
	// Find the number of registered 0 & 1 given the previous bits (in this.ctxIdx)
	idx := this.ctxIdx << 1
	b := int(bit) & 1
	this.states[idx+b]++

	if this.states[idx+b] >= THRESHOLD {
		this.states[idx]   >>= SHIFT
		this.states[idx+1] >>= SHIFT
	}

	// Update context by registering the current bit (or wrapping after 8 bits)
	if this.ctxIdx >= 128 {
		this.ctxIdx = 1
	} else {
		this.ctxIdx = (this.ctxIdx << 1) | b
	}
}

// Return the split value representing the probability of 1 in the [0..4095] range.
func (this *FPAQPredictor) Get() uint {
	idx := this.ctxIdx << 1
	numberOfOnes := uint(this.states[idx+1]+1)
	numberOfBits := uint(this.states[idx]+this.states[idx+1]+2)
	return (numberOfOnes<<12) / numberOfBits
}
