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

package util

import (
	"errors"
)

type BufferStream struct {
	buf    []byte
	off    int
	closed bool
}

func (this *BufferStream) Write(b []byte) (int, error) {
	if this.closed == true {
		return 0, errors.New("Stream closed")
	}

	// Write to len(this.buf)
	newBuf := make([]byte, len(this.buf)+len(b))
	copy(newBuf, this.buf)
	copy(newBuf[len(this.buf):], b)
	this.buf = newBuf
	return len(b), nil
}

func (this *BufferStream) Read(b []byte) (int, error) {
	if this.closed == true {
		return 0, errors.New("Stream closed")
	}

	// Read from this.off
	if len(b) < len(this.buf[this.off:]) {
		copy(b, this.buf[this.off:this.off+len(b)])
		this.off += len(b)
		return len(b), nil
	}

	copy(b, this.buf[this.off:])
	old := this.off
	this.off = len(this.buf)
	return len(this.buf) - old, nil
}

func (this *BufferStream) Close() error {
	this.closed = true
	return nil
}

func (this *BufferStream) Len() int {
	return len(this.buf)
}

func (this *BufferStream) Offset() int {
	return this.off
}

func (this *BufferStream) SetOffset(off int) error {
	if this.closed == true {
		return errors.New("Stream closed")
	}

	if off < 0 || off >= this.Len() {
		return errors.New("Invalid offset")
	}

	this.off = off
	return nil
}
