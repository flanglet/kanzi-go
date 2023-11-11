/*
Copyright 2011-2024 Frederic Langlet
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

//nolint (remove unused warnings)
import (
	"errors"

	internal "github.com/flanglet/kanzi-go/v2/internal"
)

// AdaptiveProbMap maps a probability and a context to a new probability
// that the next bit will be 1. After each guess, it updates
// its state to improve future predictions.
type adaptiveProbMapData struct {
	index    int      // last prob, context
	rate     uint     // update rate
	data     []uint16 // prob, context -> prob
	gradient [2]int
}

const (
	LINEAR_APM        = 0
	LOGISTIC_APM      = 1
	FAST_LOGISTIC_APM = 2
)

type AdaptiveProbMap interface {
	Get(bit int, pr int, ctx int) int
}

// LinearAdaptiveProbMap maps a probability and a context into a new probability
// using linear interpolation of probabilities
type LinearAdaptiveProbMap adaptiveProbMapData

// LogisticAdaptiveProbMap maps a probability and a context into a new probability
// using interpolation in the logistic domain
type LogisticAdaptiveProbMap adaptiveProbMapData

// FastLogisticAdaptiveProbMap is similar to LogisticAdaptiveProbMap but works
// faster at the expense of some accuracy
type FastLogisticAdaptiveProbMap adaptiveProbMapData

// NewBinaryEntropyEncoder creates an instance of AdaptiveProbMap
// given the provided type of APM.
func NewAdaptiveProbMap(mapType int, n, rate uint) (AdaptiveProbMap, error) {
	if mapType == LINEAR_APM {
		return newLinearAdaptiveProbMap(n, rate)
	}
	if mapType == LOGISTIC_APM {
		return newLogisticAdaptiveProbMap(n, rate)
	}
	if mapType == FAST_LOGISTIC_APM {
		return newFastLogisticAdaptiveProbMap(n, rate)
	}

	return nil, errors.New("Unknow APM type")
}

func newLogisticAdaptiveProbMap(n, rate uint) (*LogisticAdaptiveProbMap, error) {
	this := &LogisticAdaptiveProbMap{}
	size := n * 33

	if size == 0 {
		size = 33
	}

	this.data = make([]uint16, size)
	this.rate = rate

	for j := 0; j <= 32; j++ {
		this.data[j] = uint16(internal.Squash((j-16)<<7) << 4)
	}

	for i := uint(1); i < n; i++ {
		copy(this.data[i*33:], this.data[0:33])
	}

	this.gradient[0] = 0
	this.gradient[1] = 65528 + (1 << this.rate)
	return this, nil
}

// get returns improved prediction given current bit, prediction and context
func (this *LogisticAdaptiveProbMap) Get(bit int, pr int, ctx int) int {
	// Update probability based on error and learning rate
	g := this.gradient[bit]
	this.data[this.index+1] += uint16((g - int(this.data[this.index+1])) >> this.rate)
	this.data[this.index] += uint16((g - int(this.data[this.index])) >> this.rate)
	pr = internal.STRETCH[pr]

	// Find index: 33*ctx + quantized prediction in [0..32]
	this.index = ((pr + 2048) >> 7) + 33*ctx

	// Return interpolated probability
	w := pr & 127
	return (int(this.data[this.index+1])*w + int(this.data[this.index])*(128-w)) >> 11
}

func newFastLogisticAdaptiveProbMap(n, rate uint) (*FastLogisticAdaptiveProbMap, error) {
	this := &FastLogisticAdaptiveProbMap{}
	this.data = make([]uint16, n*32)
	this.rate = rate

	for j := 0; j < 32; j++ {
		this.data[j] = uint16(internal.Squash((j-16)<<7) << 4)
	}

	for i := uint(1); i < n; i++ {
		copy(this.data[i*32:], this.data[0:32])
	}

	this.gradient[0] = 0
	this.gradient[1] = 65528 + (1 << this.rate)
	return this, nil
}

// get returns improved prediction given current bit, prediction and context
func (this *FastLogisticAdaptiveProbMap) Get(bit int, pr int, ctx int) int {
	// Update probability based on error and learning rate
	g := this.gradient[bit]
	this.data[this.index] += uint16((g - int(this.data[this.index])) >> this.rate)
	this.index = ((internal.STRETCH[pr] + 2048) >> 7) + 32*ctx
	return int(this.data[this.index]) >> 4
}

func newLinearAdaptiveProbMap(n, rate uint) (*LinearAdaptiveProbMap, error) {
	this := &LinearAdaptiveProbMap{}
	size := n * 65

	if size == 0 {
		size = 65
	}

	this.data = make([]uint16, size)
	this.rate = rate

	for j := 0; j <= 64; j++ {
		this.data[j] = uint16(j<<6) << 4
	}

	for i := uint(1); i < n; i++ {
		copy(this.data[i*65:], this.data[0:65])
	}

	this.gradient[0] = 0
	this.gradient[1] = 65528 + (1 << this.rate)
	return this, nil
}

// get returns improved prediction given current bit, prediction and context
func (this *LinearAdaptiveProbMap) Get(bit int, pr int, ctx int) int {
	// Update probability based on error and learning rate
	g := this.gradient[bit]
	this.data[this.index+1] += uint16((g - int(this.data[this.index+1])) >> this.rate)
	this.data[this.index] += uint16((g - int(this.data[this.index])) >> this.rate)

	// Find index: 65*ctx + quantized prediction in [0..64]
	this.index = (pr >> 6) + 65*ctx

	// Return interpolated probability
	w := pr & 127
	return (int(this.data[this.index+1])*w + int(this.data[this.index])*(128-w)) >> 11
}
