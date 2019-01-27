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

import (
	kanzi "github.com/flanglet/kanzi-go"
)

// APM maps a probability and a context into a new probability
// that the next bit will be 1. After each guess, it updates
// its state to improve future guesses.

type AdaptiveProbMap struct {
	index int      // last prob, context
	rate  uint     // update rate
	data  []uint16 // prob, context -> prob
}

type LinearAdaptiveProbMap AdaptiveProbMap
type LogisticAdaptiveProbMap AdaptiveProbMap
type FastLogisticAdaptiveProbMap AdaptiveProbMap

func newLogisticAdaptiveProbMap(n, rate uint) (*LogisticAdaptiveProbMap, error) {
	this := &LogisticAdaptiveProbMap{}
	this.data = make([]uint16, n*33)
	this.rate = rate

	for j := 0; j <= 32; j++ {
		this.data[j] = uint16(kanzi.Squash((j-16)<<7) << 4)
	}

	for i := uint(1); i < n; i++ {
		copy(this.data[i*33:(i+1)*33], this.data[0:33])
	}

	return this, nil
}

// Return improved prediction given current bit, prediction and context
func (this *LogisticAdaptiveProbMap) get(bit int, pr int, ctx int) int {
	// Update probability based on error and learning rate
	g := (-bit & 65528) + (bit << this.rate)
	this.data[this.index+1] += uint16((g - int(this.data[this.index+1])) >> this.rate)
	this.data[this.index] += uint16((g - int(this.data[this.index])) >> this.rate)
	pr = kanzi.STRETCH[pr]

	// Find index: 33*ctx + quantized prediction in [0..32]
	this.index = ((pr + 2048) >> 7) + 33*ctx

	// Return interpolated probability
	w := pr & 127
	return (int(this.data[this.index+1])*w + int(this.data[this.index])*(128-w)) >> 11
}

func newFastLogisticAdaptiveProbMap(n, rate uint) (*FastLogisticAdaptiveProbMap, error) {
	this := &FastLogisticAdaptiveProbMap{}
	this.data = make([]uint16, n*33)
	this.rate = rate

	for j := 0; j <= 32; j++ {
		this.data[j] = uint16(kanzi.Squash((j-16)<<7) << 4)
	}

	for i := uint(1); i < n; i++ {
		copy(this.data[i*33:(i+1)*33], this.data[0:33])
	}

	return this, nil
}

// Return improved prediction given current bit, prediction and context
func (this *FastLogisticAdaptiveProbMap) get(bit int, pr int, ctx int) int {
	// Update probability based on error and learning rate
	g := (-bit & 65528) + (bit << this.rate)
	this.data[this.index] += uint16((g - int(this.data[this.index])) >> this.rate)
	this.index = ((kanzi.STRETCH[pr] + 2048) >> 7) + 33*ctx
	return int(this.data[this.index]) >> 4
}

func newLinearAdaptiveProbMap(n, rate uint) (*LinearAdaptiveProbMap, error) {
	this := &LinearAdaptiveProbMap{}
	this.data = make([]uint16, n*65)
	this.rate = rate

	for j := 0; j <= 64; j++ {
		this.data[j] = uint16(j<<6) << 4
	}

	for i := uint(1); i < n; i++ {
		copy(this.data[i*65:(i+1)*65], this.data[0:65])
	}

	return this, nil
}

// Return improved prediction given current bit, prediction and context
func (this *LinearAdaptiveProbMap) get(bit int, pr int, ctx int) int {
	// Update probability based on error and learning rate
	g := (-bit & 65528) + (bit << this.rate)
	this.data[this.index+1] += uint16((g - int(this.data[this.index+1])) >> this.rate)
	this.data[this.index] += uint16((g - int(this.data[this.index])) >> this.rate)

	// Find index: 65*ctx + quantized prediction in [0..64]
	this.index = (pr >> 6) + 65*ctx

	// Return interpolated probability
	w := pr & 127
	return (int(this.data[this.index+1])*w + int(this.data[this.index])*(128-w)) >> 11
}
