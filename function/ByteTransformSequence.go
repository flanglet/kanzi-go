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
	if len(src) == 0 {
		return 0, 0, nil
	}

	blockSize := len(src)
	length := uint(blockSize)
	requiredSize := this.MaxEncodedLen(blockSize)
	this.skipFlags = 0
	sa := [2]*[]byte{&src, &dst}
	saIdx := 0
	var err error

	for i, t := range this.transforms {
		in := *sa[saIdx]
		out := *sa[saIdx^1]

		// Check that the output buffer has enough room. If not, allocate a new one.
		if len(out) < requiredSize {
			buf := make([]byte, requiredSize)
			sa[saIdx^1] = &buf
			out = *sa[saIdx^1]
		}

		var err1 error
		var oIdx uint

		// Apply forward transform
		if _, oIdx, err1 = t.Forward(in[0:length], out); err1 != nil {
			// Transform failed. Either it does not apply to this type
			// of data or a recoverable error occurred => revert
			if &src != &dst {
				copy(out[0:length], in[0:length])
			}

			oIdx = length
			this.skipFlags |= (1 << (7 - uint(i)))

			if err == nil {
				err = err1
			}
		}

		length = oIdx
		saIdx ^= 1
	}

	for i := len(this.transforms); i < 8; i++ {
		this.skipFlags |= (1 << (7 - uint(i)))
	}

	if saIdx != 1 {
		in := *sa[0]
		out := *sa[1]
		copy(out, in[0:length])
	}

	if this.skipFlags != _TRANSFORM_SKIP_MASK {
		err = nil
	}

	return uint(blockSize), length, err
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Runs Inverse on each transform in the sequence.
// Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *ByteTransformSequence) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	blockSize := len(src)
	length := uint(blockSize)

	if this.skipFlags == _TRANSFORM_SKIP_MASK {
		if &src[0] != &dst[0] {
			copy(dst, src)
		}

		return length, length, nil
	}

	sa := [2]*[]byte{&src, &dst}
	saIdx := 0
	var res error

	// Process transforms sequentially in reverse order
	for i := len(this.transforms) - 1; i >= 0; i-- {
		if this.skipFlags&(1<<(7-uint(i))) != 0 {
			continue
		}

		t := this.transforms[i]
		in := *sa[saIdx]
		saIdx ^= 1
		out := *sa[saIdx]

		// Apply inverse transform
		_, length, res = t.Inverse(in[0:length], out[0:cap(out)])

		if res != nil {
			break
		}
	}

	if saIdx != 1 {
		in := *sa[0]
		out := *sa[1]
		copy(out, in[0:length])
	}

	return uint(blockSize), length, res
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
