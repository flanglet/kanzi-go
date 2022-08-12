/*
Copyright 2011-2022 Frederic Langlet
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

// BufferStream a closable read/write stream of bytes backed by a slice
type BufferStream struct {
	buf    []byte
	off    int
	closed bool
}

// NewBufferStream creates a new instance of BufferStream backed by the
// provided byte slice.
func NewBufferStream(buf []byte) *BufferStream {
	this := &BufferStream{}
	this.buf = buf
	return this
}

// Write returns an error if the stream is closed, otherwise writes the given
// data to the internal buffer (growing the buffer as needed).
// Returns the number of bytes written.
func (this *BufferStream) Write(b []byte) (int, error) {
	if this.closed == true {
		return 0, errors.New("Stream closed")
	}

	// Write to len(this.buf)
	this.buf = append(this.buf, b...)
	return len(b), nil
}

// Read returns an error if the stream is closed, otherwise reads data from
// the internal buffer at the read offset position.
// Returns the number of bytes read.
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

// Close makes the stream unavailable for further reads or writes.
func (this *BufferStream) Close() error {
	this.closed = true
	return nil
}

// Len returns the size of the stream
func (this *BufferStream) Len() int {
	return len(this.buf)
}

// Offset returns the offset of the read pointer
func (this *BufferStream) Offset() int {
	return this.off
}

// SetOffset sets the offset of the read pointer.
// Returns an error if the offset value is invalid otr the stream is closed.
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
