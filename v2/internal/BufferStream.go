/*
Copyright 2011-2026 Frederic Langlet
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

package internal

import (
	"bytes"
	"errors"
)

// BufferStream a closable read/write stream of bytes backed by a bytes.Buffer
type BufferStream struct {
	buf    *bytes.Buffer
	closed bool
}

// StagedBuffer a closable write-only byte stream with access to the backing slice.
type StagedBuffer struct {
	buf    []byte
	length int
	closed bool
}

// NewBufferStream creates a new instance of BufferStream
func NewBufferStream(args ...[]byte) *BufferStream {
	this := &BufferStream{}

	if len(args) == 1 {
		this.buf = bytes.NewBuffer(args[0])
	} else {
		this.buf = bytes.NewBuffer(make([]byte, 0))
	}

	return this
}

// NewStagedBuffer creates a new instance of StagedBuffer backed by the provided slice.
func NewStagedBuffer(buf []byte) *StagedBuffer {
	return &StagedBuffer{buf: buf[:0]}
}

// Write returns an error if the stream is closed, otherwise writes the given
// data to the internal buffer (growing the buffer as needed).
// Returns the number of bytes written.
func (this *BufferStream) Write(b []byte) (int, error) {
	if this.closed == true {
		return 0, errors.New("Stream closed")
	}

	return this.buf.Write(b)
}

// Read returns an error if the stream is closed, otherwise reads data from
// the internal buffer at the read offset position.
// Returns the number of bytes read or (0, io.EOF) when no more data remains.
func (this *BufferStream) Read(b []byte) (int, error) {
	if this.closed == true {
		return 0, errors.New("Stream closed")
	}

	return this.buf.Read(b)
}

// Close makes the stream unavailable for future reads or writes.
func (this *BufferStream) Close() error {
	this.closed = true
	return nil
}

// Len returns the size of the stream
func (this *BufferStream) Len() int {
	return this.buf.Len()
}

// Available returns the number of bytes available for read
func (this *BufferStream) Available() int {
	if this.closed == true {
		return 0
	}

	return this.buf.Available()
}

// Write returns an error if the stream is closed, otherwise writes the given
// data to the internal buffer (growing the buffer as needed).
// Returns the number of bytes written.
func (this *StagedBuffer) Write(b []byte) (int, error) {
	if this.closed == true {
		return 0, errors.New("Stream closed")
	}

	required := this.length + len(b)

	if cap(this.buf) < required {
		newBuf := make([]byte, growStagedBufferSize(cap(this.buf), required))
		copy(newBuf, this.buf[:this.length])
		this.buf = newBuf[:this.length]
	}

	if len(this.buf) < required {
		this.buf = this.buf[:required]
	}

	copy(this.buf[this.length:], b)
	this.length += len(b)
	return len(b), nil
}

// Close makes the stream unavailable for future writes.
func (this *StagedBuffer) Close() error {
	this.closed = true
	return nil
}

// Bytes returns the bytes written so far.
func (this *StagedBuffer) Bytes() []byte {
	return this.buf[:this.length]
}

// Backing returns the full backing slice capacity.
func (this *StagedBuffer) Backing() []byte {
	if cap(this.buf) == 0 {
		return this.buf
	}

	return this.buf[:cap(this.buf)]
}

func growStagedBufferSize(current, required int) int {
	grownSize := current + max(current>>2, 1<<20)
	return max(required, max(grownSize, 1024))
}
