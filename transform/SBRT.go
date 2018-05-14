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
	"fmt"
	kanzi "github.com/flanglet/kanzi-go"
)

// Sort by Rank Transform is a family of transforms typically used after
// a BWT to reduce the variance of the data prior to entropy coding.
// SBR(alpha) is defined by sbr(x, alpha) = (1-alpha)*(t-w1(x,t)) + alpha*(t-w2(x,t))
// where x is an item in the data list, t is the current access time and wk(x,t) is
// the k-th access time to x at time t (with 0 <= alpha <= 1).
// See [Two new families of list update algorihtms] by Frank Schulz for details.
// SBR(0)= Move to Front Transform
// SBR(1)= Time Stamp Transform
// This code implements SBR(0), SBR(1/2) and SBR(1). Code derived from openBWT

const (
	SBRT_MODE_MTF       = 1 // alpha = 0
	SBRT_MODE_RANK      = 2 // alpha = 1/2
	SBRT_MODE_TIMESTAMP = 3 // alpha = 1
)

type SBRT struct {
	mode int
}

func NewSBRT(mode int) (*SBRT, error) {
	if mode != SBRT_MODE_MTF && mode != SBRT_MODE_RANK && mode != SBRT_MODE_TIMESTAMP {
		return nil, errors.New("Invalid mode parameter")
	}

	this := new(SBRT)
	this.mode = mode
	return this, nil
}

func (this *SBRT) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if len(src) == 0 {
		return 0, 0, nil
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count > len(dst) {
		errMsg := fmt.Sprintf("Block size is %v, output buffer length is %v", count, len(src))
		return 0, 0, errors.New(errMsg)
	}

	var mask1, mask2 int
	var shift uint

	if this.mode == SBRT_MODE_TIMESTAMP {
		mask1 = 0
	} else {
		mask1 = -1
	}

	if this.mode == SBRT_MODE_MTF {
		mask2 = 0
	} else {
		mask2 = -1
	}

	if this.mode == SBRT_MODE_RANK {
		shift = 1
	} else {
		shift = 0
	}

	p := [256]int{}
	q := [256]int{}
	s2r := [256]int{}
	r2s := [256]int{}

	for i := 0; i < 256; i++ {
		s2r[i] = i
		r2s[i] = i
	}

	for i := 0; i < count; i++ {
		c := int(src[i])
		r := s2r[c]
		dst[i] = byte(r)
		q[c] = ((i & mask1) + (p[c] & mask2)) >> shift
		p[c] = i
		curVal := q[c]

		// Move up symbol to correct rank
		for r > 0 && q[r2s[r-1]] <= curVal {
			r2s[r] = r2s[r-1]
			s2r[r2s[r]] = r
			r--
		}

		r2s[r] = c
		s2r[c] = r
	}

	return uint(count), uint(count), nil
}

func (this *SBRT) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if len(src) == 0 {
		return 0, 0, nil
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count > len(dst) {
		errMsg := fmt.Sprintf("Block size is %v, output buffer length is %v", count, len(src))
		return 0, 0, errors.New(errMsg)
	}

	var mask1, mask2 int
	var shift uint

	if this.mode == SBRT_MODE_TIMESTAMP {
		mask1 = 0
	} else {
		mask1 = -1
	}

	if this.mode == SBRT_MODE_MTF {
		mask2 = 0
	} else {
		mask2 = -1
	}

	if this.mode == SBRT_MODE_RANK {
		shift = 1
	} else {
		shift = 0
	}

	p := [256]int{}
	q := [256]int{}
	r2s := [256]int{}

	for i := 0; i < 256; i++ {
		r2s[i] = i
	}

	for i := 0; i < count; i++ {
		r := int(src[i])
		c := r2s[r]
		dst[i] = byte(c)
		q[c] = ((i & mask1) + (p[c] & mask2)) >> shift
		p[c] = i
		curVal := q[c]

		// Move up symbol to correct rank
		for r > 0 && q[r2s[r-1]] <= curVal {
			r2s[r] = r2s[r-1]
			r--
		}

		r2s[r] = c
	}

	return uint(count), uint(count), nil
}
