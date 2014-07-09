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

package bitstream

import (
	"errors"
	"fmt"
	"kanzi"
)

type DefaultOutputBitStream struct {
	closed   bool
	written  uint64
	position int    // index of current byte in buffer
	bitIndex uint   // index of current bit to write
	current  uint64 // cached bits
	os       kanzi.OutputStream
	buffer   []byte
}

func NewDefaultOutputBitStream(stream kanzi.OutputStream, bufferSize uint) (*DefaultOutputBitStream, error) {
	if stream == nil {
		return nil, errors.New("Invalid null output stream parameter")
	}

	if bufferSize < 1024 {
		return nil, errors.New("Invalid buffer size parameter (must be at least 1024 bytes)")
	}

	if bufferSize&7 != 0 {
		return nil, errors.New("Invalid buffer size (must be a multiple of 8)")
	}

	this := new(DefaultOutputBitStream)
	this.buffer = make([]byte, bufferSize)
	this.os = stream
	this.bitIndex = 63
	return this, nil
}

// Write least significant bit of the input integer. Report error if stream is closed
func (this *DefaultOutputBitStream) WriteBit(bit int) error {
	if this.Closed() {
		return errors.New("Stream closed")
	}

	this.current |= (uint64(bit&1) << this.bitIndex)

	if this.bitIndex == 0 {
		this.pushCurrent()
	} else {
		this.bitIndex--
	}

	return nil
}

// Write 'count' (in [1..64]) bits. Report error if stream is closed
func (this *DefaultOutputBitStream) WriteBits(value uint64, count uint) (uint, error) {
	if this.Closed() {
		return 0, errors.New("Stream closed")
	}

	if count == 0 {
		return 0, nil
	}

	if count > 64 {
		return 0, fmt.Errorf("Invalid length: %v (must be in [1..64])", count)
	}

	value &= (0xFFFFFFFFFFFFFFFF >> (64 - count))

	// Pad the current position in buffer
	if count <= this.bitIndex+1 {
		// Enough spots available in 'current'
		remaining := this.bitIndex + 1 - count

		if remaining == 0 {
			this.current |= value
			this.pushCurrent()
		} else {
			this.current |= (value << remaining)
			this.bitIndex -= count
		}
	} else {
		// Not enough spots available in 'current'
		remaining := count - this.bitIndex - 1
		this.current |= (value >> remaining)
		this.pushCurrent()
		this.current |= (value << (64 - remaining))
		this.bitIndex -= remaining
	}

	return count, nil
}

// Push 64 bits of current value into buffer.
func (this *DefaultOutputBitStream) pushCurrent() {
	this.buffer[this.position] = byte(this.current >> 56)
	this.buffer[this.position+1] = byte(this.current >> 48)
	this.buffer[this.position+2] = byte(this.current >> 40)
	this.buffer[this.position+3] = byte(this.current >> 32)
	this.buffer[this.position+4] = byte(this.current >> 24)
	this.buffer[this.position+5] = byte(this.current >> 16)
	this.buffer[this.position+6] = byte(this.current >> 8)
	this.buffer[this.position+7] = byte(this.current)
	this.bitIndex = 63
	this.current = 0
	this.position += 8

	if this.position >= len(this.buffer) {
		this.flush()
	}
}

// Write buffer into underlying stream
func (this *DefaultOutputBitStream) flush() error {
	if this.Closed() {
		return errors.New("Stream closed")
	}

	if this.position > 0 {
		if _, err := this.os.Write(this.buffer[0:this.position]); err != nil {
			return err
		}

		this.written += uint64(this.position << 3)
		this.position = 0
	}

	return nil
}

func (this *DefaultOutputBitStream) Close() (bool, error) {
	if this.Closed() {
		return true, nil
	}

	savedBitIndex := this.bitIndex
	savedPosition := this.position
	savedCurrent := this.current

	// Push last bytes (the very last byte may be incomplete)
	size := int((63-this.bitIndex)+7) >> 3
	this.pushCurrent()
	this.position -= (8 - size)

	if err := this.flush(); err != nil {
		// Revert fields to allow subsequent attempts in case of transient failure
		this.bitIndex = savedBitIndex
		this.position = savedPosition
		this.current = savedCurrent
		return false, err
	}

	if err := this.os.Sync(); err != nil {
		return false, err
	}

	if err := this.os.Close(); err != nil {
		return false, err
	}

	this.closed = true
	this.position = 0
	this.bitIndex = 63
	return true, nil
}

// Return number of bits written so far
func (this *DefaultOutputBitStream) Written() uint64 {
	// Number of bits flushed + bytes written in memory + bits written in memory
	return this.written + uint64(this.position<<3) + uint64(63-this.bitIndex)
}

func (this *DefaultOutputBitStream) Closed() bool {
	return this.closed
}
