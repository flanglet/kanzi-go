/*
Copyright 2011-2021 Frederic Langlet
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
)

// NullTransform is a pass through byte function
type NullTransform struct {
}

// NewNullTransform creates a new instance of NullTransform
func NewNullTransform() (*NullTransform, error) {
	this := &NullTransform{}
	return this, nil
}

// NewNullTransformWithCtx creates a new instance of NullTransform using a
// configuration map as parameter.
func NewNullTransformWithCtx(ctx *map[string]interface{}) (*NullTransform, error) {
	this := &NullTransform{}
	return this, nil
}

func doCopy(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if len(src) > len(dst) {
		return uint(0), uint(0), errors.New("Destination buffer too small")
	}

	if &src[0] != &dst[0] {
		copy(dst, src)
	}

	return uint(len(src)), uint(len(src)), nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *NullTransform) Forward(src, dst []byte) (uint, uint, error) {
	return doCopy(src, dst)
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *NullTransform) Inverse(src, dst []byte) (uint, uint, error) {
	return doCopy(src, dst)
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this NullTransform) MaxEncodedLen(srcLen int) int {
	return srcLen
}
