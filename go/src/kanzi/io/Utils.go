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

package io

import (
	"errors"
)

// Implements io.WriteCloser
type NullOutputStream struct {
	closed bool
}

func NewNullOutputStream() (*NullOutputStream, error) {
	bos := new(NullOutputStream)
	bos.closed = false
	return bos, nil
}

func (this *NullOutputStream) Write(b []byte) (n int, err error) {
	if this.closed == true {
		panic(errors.New("Stream closed"))
	}

	return len(b), nil
}

func (this *NullOutputStream) Close() error {
	this.closed = true
	return nil
}
