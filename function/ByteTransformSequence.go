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

package function

import (
	"errors"
	"fmt"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	_TRANSFORM_SKIP_MASK = 0xFF
)

// ByteTransformSequence encapsulates a sequence of transforms or functions in a function
type ByteTransformSequence struct {
	transforms []kanzi.ByteTransform // transforms or functions
	skipFlags  byte                  // skip transforms
}

// NewByteTransformSequence creates a new instance of NewByteTransformSequence
// containing the transforms provided as parameter.
func NewByteTransformSequence(transforms []kanzi.ByteTransform) (*ByteTransformSequence, error) {
	if transforms == nil {
		return nil, errors.New("Invalid null transforms parameter")
	}

	if len(transforms) == 0 || len(transforms) > 8 {
		return nil, errors.New("Only 1 to 8 transforms allowed")
	}

	this := new(ByteTransformSequence)
	this.transforms = transforms
	this.skipFlags = 0
	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Runs Forward on each transform in the sequence.
// Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *ByteTransformSequence) Forward(src, dst []byte) (uint, uint, error) {
	this.skipFlags = _TRANSFORM_SKIP_MASK

	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	requiredSize := this.MaxEncodedLen(len(src))

	if len(dst) < requiredSize {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), requiredSize)
	}

	blockSize := uint(len(src))
	length := blockSize
	in, out := src, dst
	var err error
	swaps := 0

	// Process transforms sequentially
	for i, t := range this.transforms {
		savedLength := length

		if len(out) < requiredSize {
			out = make([]byte, requiredSize)
		}

		// Apply forward transform
		if _, length, err = t.Forward((in)[0:length], out); err != nil {
			// Transform failed. Either it does not apply to this type
			// of data or a recoverable error occurred => revert
			length = savedLength
			continue
		}

		this.skipFlags &= ^(1 << (7 - uint(i)))
		in, out = out, in
		swaps++

		if i == this.Len()-1 {
			break
		}
	}

	if swaps&1 == 0 {
		copy(dst, in[0:length])
	}

	return blockSize, length, nil
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Runs Inverse on each transform in the sequence.
// Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *ByteTransformSequence) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	blockSize := uint(len(src))

	if this.skipFlags == _TRANSFORM_SKIP_MASK {
		copy(dst, src)

		return blockSize, blockSize, nil
	}

	length := blockSize
	in, out := src, dst
	var err error
	swaps := 0

	// Process transforms sequentially in reverse order
	for i := this.Len() - 1; i >= 0; i-- {
		if this.skipFlags&(1<<(7-uint(i))) != 0 {
			continue
		}

		if len(out) < len(dst) {
			out = make([]byte, len(dst))
		}

		// Apply inverse transform
		if _, length, err = this.transforms[i].Inverse(in[0:length], out); err != nil {
			// All inverse transforms must succeed
			break
		}

		in, out = out, in
		swaps++
	}

	if err == nil && swaps&1 == 0 {
		copy(dst, in[0:length])
	}

	return blockSize, length, err
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this ByteTransformSequence) MaxEncodedLen(srcLen int) int {
	requiredSize := srcLen

	for _, t := range this.transforms {
		if f, isFunction := t.(kanzi.ByteFunction); isFunction == true {
			reqSize := f.MaxEncodedLen(requiredSize)

			if reqSize > requiredSize {
				requiredSize = reqSize
			}
		}
	}

	return requiredSize
}

// Len returns the number of functions in the sequence (in [0..8])
func (this *ByteTransformSequence) Len() int {
	return len(this.transforms)
}

// SkipFlags returns the flags describing which function to
// skip (bit set to 1)
func (this *ByteTransformSequence) SkipFlags() byte {
	return this.skipFlags
}

// SetSkipFlags sets the flags describing which function to skip
func (this *ByteTransformSequence) SetSkipFlags(flags byte) bool {
	this.skipFlags = flags
	return true
}
