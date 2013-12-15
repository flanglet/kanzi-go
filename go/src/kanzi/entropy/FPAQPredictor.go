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
	states [256][]uint16 // 256 frequency contexts for each bit
}

func NewFPAQPredictor() (*FPAQPredictor, error) {
	this := new(FPAQPredictor)
	this.ctxIdx = 1

	for i := 255; i >= 0; i-- {
		this.states[i] = make([]uint16, 2)
	}

	return this, nil
}

// Used to update the probability model
func (this *FPAQPredictor) Update(bit byte) {
	// Find the number of registered 0 & 1 given the previous bits (in this.ctxIdx)
	st := this.states[this.ctxIdx]
	st[bit]++

	if st[bit] >= THRESHOLD {
		st[0] >>= SHIFT
		st[1] >>= SHIFT
	}

	// Update context by registering the current bit (or wrapping after 8 bits)
	if this.ctxIdx >= 128 {
		this.ctxIdx = 1
	} else {
		this.ctxIdx = (this.ctxIdx << 1) | int(bit)
	}
}

// Return the split value representing the probability of 1 in the [0..4095] range.
func (this *FPAQPredictor) Get() uint {
	st := this.states[this.ctxIdx]
	numberOfOnes := uint(st[1]+1)
	numberOfBits := uint(st[0]+st[1]+2)
	return (numberOfOnes<<12) / numberOfBits
}
