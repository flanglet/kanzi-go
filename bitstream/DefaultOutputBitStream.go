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

package bitstream

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type DefaultOutputBitStream struct {
	closed    bool
	written   uint64
	position  int    // index of current byte in buffer
	availBits int    // bits not consumed in current
	current   uint64 // cached bits
	os        io.WriteCloser
	buffer    []byte
}

func NewDefaultOutputBitStream(stream io.WriteCloser, bufferSize uint) (*DefaultOutputBitStream, error) {
	if stream == nil {
		return nil, errors.New("Invalid null output stream parameter")
	}

	if bufferSize < 1024 {
		return nil, errors.New("Invalid buffer size parameter (must be at least 1024 bytes)")
	}

	if bufferSize > 1<<29 {
		return nil, errors.New("Invalid buffer size parameter (must be at most 536870912 bytes)")
	}

	if bufferSize&7 != 0 {
		return nil, errors.New("Invalid buffer size (must be a multiple of 8)")
	}

	this := new(DefaultOutputBitStream)
	this.buffer = make([]byte, bufferSize)
	this.os = stream
	this.availBits = 64
	return this, nil
}

// Write least significant bit of the input integer. Panics if stream is closed
func (this *DefaultOutputBitStream) WriteBit(bit int) {
	if this.availBits <= 1 { // availBits = 0 if stream is closed => force pushCurrent() => panic
		this.current |= uint64(bit & 1)
		this.pushCurrent()
	} else {
		this.availBits--
		this.current |= (uint64(bit&1) << uint(this.availBits))
	}

}

// Write 'count' (in [1..64]) bits. Panics if stream is closed.
// Return number of written bits
func (this *DefaultOutputBitStream) WriteBits(value uint64, count uint) uint {
	if count == 0 {
		return 0
	}

	if count > 64 {
		panic(fmt.Errorf("Invalid length: %v (must be in [1..64])", count))
	}

	value &= (0xFFFFFFFFFFFFFFFF >> (64 - count))
	bi := uint(this.availBits)

	// Pad the current position in buffer
	if count < bi {
		// Enough spots available in 'current'
		this.current |= (value << (bi - count))
		this.availBits -= int(count)
	} else {
		// Not enough spots available in 'current'
		remaining := count - bi
		this.current |= (value >> remaining)
		this.pushCurrent()

		if remaining != 0 {
			this.current = (value << (64 - remaining))
			this.availBits -= int(remaining)
		}
	}

	return count
}

func (this *DefaultOutputBitStream) WriteArray(bits []byte, count uint) uint {
	if this.Closed() {
		panic(errors.New("Stream closed"))
	}

	if count == 0 {
		return 0
	}

	if count > uint(len(bits)<<3) {
		panic(fmt.Errorf("Invalid length: %v (must be in [1..%v])", count, len(bits)<<3))
	}

	remaining := int(count)
	start := 0

	// Byte aligned cursor ?
	if this.availBits&7 == 0 {
		// Fill up this.current
		for (this.availBits != 64) && (remaining >= 8) {
			this.WriteBits(uint64(bits[start]), 8)
			start++
			remaining -= 8
		}

		// Copy bits array to internal buffer
		for remaining>>3 >= len(this.buffer)-this.position {
			copy(this.buffer[this.position:], bits[start:start+len(this.buffer)-this.position])
			start += (len(this.buffer) - this.position)
			remaining -= ((len(this.buffer) - this.position) << 3)
			this.position = len(this.buffer)
			this.flush()
		}

		r := (remaining >> 6) << 3

		if r > 0 {
			copy(this.buffer[this.position:], bits[start:start+r])
			start += r
			this.position += r
			remaining -= (r << 3)
		}
	} else {
		// Not byte aligned
		r := 64 - this.availBits

		for remaining >= 64 {
			value := binary.BigEndian.Uint64(bits[start : start+8])
			this.current |= (value >> uint(r))
			this.pushCurrent()
			this.current = (value << uint(64-r))
			this.availBits -= r
			start += 8
			remaining -= 64
		}
	}

	// Last bytes
	for remaining >= 8 {
		this.WriteBits(uint64(bits[start]), 8)
		start++
		remaining -= 8
	}

	if remaining > 0 {
		this.WriteBits(uint64(bits[start])>>uint(8-remaining), uint(remaining))
	}

	return count
}

// Push 64 bits of current value into buffer.
func (this *DefaultOutputBitStream) pushCurrent() {
	binary.BigEndian.PutUint64(this.buffer[this.position:this.position+8], this.current)
	this.availBits = 64
	this.current = 0
	this.position += 8

	if this.position >= len(this.buffer) {
		if err := this.flush(); err != nil {
			panic(err)
		}
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

		this.written += (uint64(this.position) << 3)
		this.position = 0
	}

	return nil
}

func (this *DefaultOutputBitStream) Close() (bool, error) {
	if this.Closed() {
		return true, nil
	}

	savedBitIndex := this.availBits
	savedPosition := this.position
	savedCurrent := this.current

	// Push last bytes (the very last byte may be incomplete)
	size := int((64-this.availBits)+7) >> 3
	this.pushCurrent()
	this.position -= (8 - size)

	if err := this.flush(); err != nil {
		// Revert fields to allow subsequent attempts in case of transient failure
		this.availBits = savedBitIndex
		this.position = savedPosition
		this.current = savedCurrent
		return false, err
	}

	this.closed = true
	this.position = 0

	// Reset fields to force a flush() and trigger an error
	// on WriteBit() or WriteBits()
	this.availBits = 0
	this.buffer = make([]byte, 8)
	this.written -= 64 // adjust for method Written()
	return true, nil
}

// Return number of bits written so far
func (this *DefaultOutputBitStream) Written() uint64 {
	// Number of bits flushed + bytes written in memory + bits written in memory
	return this.written + uint64(this.position<<3) + uint64(64-this.availBits)
}

func (this *DefaultOutputBitStream) Closed() bool {
	return this.closed
}
