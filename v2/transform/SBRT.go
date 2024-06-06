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

package transform

import (
	"errors"
	"fmt"
)

// Sort by Rank Transform is a family of transforms typically used after
// a BWT to reduce the variance of the data prior to entropy coding.
// SBR(alpha) is defined by sbr(x, alpha) = (1-alpha)*(t-w1(x,t)) + alpha*(t-w2(x,t))
// where x is an item in the data list, t is the current access time and wk(x,t) is
// the k-th access time to x at time t (with 0 <= alpha <= 1).
// See [Two new families of list update algorithms] by Frank Schulz for details.
// SBR(0)= Move to Front Transform
// SBR(1)= Time Stamp Transform
// This code implements SBR(0), SBR(1/2) and SBR(1). Code derived from openBWT

const (
	// SBRT_MODE_MTF mode MoveToFront
	SBRT_MODE_MTF = 1
	// SBRT_MODE_RANK mode Rank
	SBRT_MODE_RANK = 2
	// SBRT_MODE_TIMESTAMP mode TimeStamp
	SBRT_MODE_TIMESTAMP = 3
)

// SBRT Sort By Rank Transform
type SBRT struct {
	mode  int
	mask1 int
	mask2 int
	shift uint
}

// NewSBRT creates a new instance of SBRT
func NewSBRT(mode int) (*SBRT, error) {
	if mode != SBRT_MODE_MTF && mode != SBRT_MODE_RANK && mode != SBRT_MODE_TIMESTAMP {
		return nil, errors.New("Invalid mode parameter")
	}

	this := &SBRT{}
	this.mode = mode

	if this.mode == SBRT_MODE_TIMESTAMP {
		this.mask1 = 0
	} else {
		this.mask1 = -1
	}

	if this.mode == SBRT_MODE_MTF {
		this.mask2 = 0
	} else {
		this.mask2 = -1
	}

	if this.mode == SBRT_MODE_RANK {
		this.shift = 1
	} else {
		this.shift = 0
	}

	return this, nil
}

// NewSBRTWithCtx creates a new instance of SBRT using a
// configuration map as parameter.
func NewSBRTWithCtx(ctx *map[string]any) (*SBRT, error) {
	mode := SBRT_MODE_MTF

	if _, containsKey := (*ctx)["sbrt"]; containsKey {
		mode = (*ctx)["sbrt"].(int)
	}

	if mode != SBRT_MODE_MTF && mode != SBRT_MODE_RANK && mode != SBRT_MODE_TIMESTAMP {
		return nil, errors.New("Invalid mode parameter")
	}

	this := &SBRT{}
	this.mode = mode

	if this.mode == SBRT_MODE_TIMESTAMP {
		this.mask1 = 0
	} else {
		this.mask1 = -1
	}

	if this.mode == SBRT_MODE_MTF {
		this.mask2 = 0
	} else {
		this.mask2 = -1
	}

	if this.mode == SBRT_MODE_RANK {
		this.shift = 1
	} else {
		this.shift = 0
	}

	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *SBRT) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	count := len(src)
	s2r := [256]uint8{}
	r2s := [256]uint8{}

	for i := range s2r {
		s2r[i] = uint8(i)
		r2s[i] = uint8(i)
	}

	m1 := this.mask1
	m2 := this.mask2
	s := this.shift
	p := [256]int{}
	q := [256]int{}

	for i := 0; i < count; i++ {
		c := uint8(src[i])
		r := s2r[c]
		dst[i] = byte(r)
		qc := ((i & m1) + (p[c] & m2)) >> s
		p[c] = i
		q[c] = qc

		// Move up symbol to correct rank
		for r > 0 && q[r2s[r-1]] <= qc {
			t := r2s[r-1]
			r2s[r], s2r[t] = t, r
			r--
		}

		r2s[r] = c
		s2r[c] = r
	}

	return uint(count), uint(count), nil
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *SBRT) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count > len(dst) {
		errMsg := fmt.Sprintf("Block size is %v, output buffer length is %v", count, len(dst))
		return 0, 0, errors.New(errMsg)
	}

	r2s := [256]uint8{}

	for i := range r2s {
		r2s[i] = uint8(i)
	}

	m1 := this.mask1
	m2 := this.mask2
	s := this.shift
	p := [256]int{}
	q := [256]int{}

	for i := 0; i < count; i++ {
		r := src[i]
		c := r2s[r]
		dst[i] = byte(c)
		qc := ((i & m1) + (p[c] & m2)) >> s
		p[c] = i
		q[c] = qc

		// Move up symbol to correct rank
		for r > 0 && q[r2s[r-1]] <= qc {
			r2s[r] = r2s[r-1]
			r--
		}

		r2s[r] = c
	}

	return uint(count), uint(count), nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *SBRT) MaxEncodedLen(srcLen int) int {
	return srcLen + _BWT_MAX_HEADER_SIZE
}
