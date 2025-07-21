/*
Copyright 2011-2025 Frederic Langlet
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

import kanzi "github.com/flanglet/kanzi-go/v2"

// NullOutputStream similar to io.Discard but implements io.WriteCloser
type NullOutputStream struct {
	closed bool
}

// NewNullOutputStream creates an instance of NullOutputStream
func NewNullOutputStream() (*NullOutputStream, error) {
	this := &NullOutputStream{}
	this.closed = false
	return this, nil
}

// Write returns an error if the stream is closed else does nothing.
func (this *NullOutputStream) Write(b []byte) (n int, err error) {
	if this.closed == true {
		return 0, &IOError{msg: "Stream closed", code: kanzi.ERR_WRITE_FILE}
	}

	return len(b), nil
}

// Close makes the stream unavailable for future writes
func (this *NullOutputStream) Close() error {
	this.closed = true
	return nil
}
