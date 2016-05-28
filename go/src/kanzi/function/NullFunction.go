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

package function

import (
	"errors"
	"kanzi"
)

type NullFunction struct {
}

func NewNullFunction() (*NullFunction, error) {
	this := new(NullFunction)
	return this, nil
}

func doCopy(src, dst []byte, sz uint) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	length := len(src)

	if sz > 0 {
		length = int(sz)

		if length > len(src) {
			return uint(0), uint(0), errors.New("Source buffer too small")
		}
	}

	if length > len(dst) {
		return uint(0), uint(0), errors.New("Destination buffer too small")
	}

	if kanzi.SameByteSlices(src, dst, false) == false {
		copy(dst, src[0:length])
	}

	return uint(length), uint(length), nil
}

func (this *NullFunction) Forward(src, dst []byte, length uint) (uint, uint, error) {
	return doCopy(src, dst, length)
}

func (this *NullFunction) Inverse(src, dst []byte, length uint) (uint, uint, error) {
	return doCopy(src, dst, length)
}

func (this NullFunction) MaxEncodedLen(srcLen int) int {
	return srcLen
}
