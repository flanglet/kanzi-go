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

// This codec is just a wrapper around the Snappy Go implementation available at
// code.google.com/p/snappy-go/snappy. It requires the snappy Go package
// The package is installed with the command 'go get code.google.com/p/snappy-go/snappy'

import (
	"code.google.com/p/snappy-go/snappy"
	"errors"
	"fmt"
	"kanzi"
)

type SnappyCodec struct {
}

func NewSnappyCodec() (*SnappyCodec, error) {
	this := new(SnappyCodec)
	return this, nil
}

func (this *SnappyCodec) Forward(src, dst []byte, length uint) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := length

	if n := snappy.MaxEncodedLen(int(count)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	res, err := snappy.Encode(dst, src[0:count])

	if err != nil {
		return 0, 0, fmt.Errorf("Encoding error: %v", err)
	}

	return count, uint(len(res)), nil
}

func (this *SnappyCodec) Inverse(src, dst []byte, length uint) (uint, uint, error) {
	if src == nil {
		return uint(0), uint(0), errors.New("Invalid null source buffer")
	}

	if dst == nil {
		return uint(0), uint(0), errors.New("Invalid null destination buffer")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := length
	res, err := snappy.Decode(dst, src[0:count])

	if err != nil {
		return 0, 0, fmt.Errorf("Decoding error: %v", err)
	}

	if len(res) > len(dst) {
		// Encode returns a newly allocated slice if the provided 'dst' array is too small.
		// There is no way to return this new slice, so treat it as an error
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(res), len(dst))
	}

	return count, uint(len(res)), nil
}

func (this SnappyCodec) MaxEncodedLen(srcLen int) int {
	return snappy.MaxEncodedLen(srcLen)
}
