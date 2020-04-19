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

// DefaultOutputBitStream is the default implementation of OutputBitStream
type DefaultOutputBitStream struct {
	closed    bool
	written   uint64
	position  int    // index of current byte in buffer
	availBits uint   // bits not consumed in current
	current   uint64 // cached bits
	os        io.WriteCloser
	buffer    []byte
}

var _OBS_MASKS = [65]uint64{
	0x0,
	0x1,
	0x3,
	0x7,
	0xF,
	0x1F,
	0x3F,
	0x7F,
	0xFF,
	0x1FF,
	0x3FF,
	0x7FF,
	0xFFF,
	0x1FFF,
	0x3FFF,
	0x7FFF,
	0xFFFF,
	0x1FFFF,
	0x3FFFF,
	0x7FFFF,
	0xFFFFF,
	0x1FFFFF,
	0x3FFFFF,
	0x7FFFFF,
	0xFFFFFF,
	0x1FFFFFF,
	0x3FFFFFF,
	0x7FFFFFF,
	0xFFFFFFF,
	0x1FFFFFFF,
	0x3FFFFFFF,
	0x7FFFFFFF,
	0xFFFFFFFF,
	0x1FFFFFFFF,
	0x3FFFFFFFF,
	0x7FFFFFFFF,
	0xFFFFFFFFF,
	0x1FFFFFFFFF,
	0x3FFFFFFFFF,
	0x7FFFFFFFFF,
	0xFFFFFFFFFF,
	0x1FFFFFFFFFF,
	0x3FFFFFFFFFF,
	0x7FFFFFFFFFF,
	0xFFFFFFFFFFF,
	0x1FFFFFFFFFFF,
	0x3FFFFFFFFFFF,
	0x7FFFFFFFFFFF,
	0xFFFFFFFFFFFF,
	0x1FFFFFFFFFFFF,
	0x3FFFFFFFFFFFF,
	0x7FFFFFFFFFFFF,
	0xFFFFFFFFFFFFF,
	0x1FFFFFFFFFFFFF,
	0x3FFFFFFFFFFFFF,
	0x7FFFFFFFFFFFFF,
	0xFFFFFFFFFFFFFF,
	0x1FFFFFFFFFFFFFF,
	0x3FFFFFFFFFFFFFF,
	0x7FFFFFFFFFFFFFF,
	0xFFFFFFFFFFFFFFF,
	0x1FFFFFFFFFFFFFFF,
	0x3FFFFFFFFFFFFFFF,
	0x7FFFFFFFFFFFFFFF,
	0xFFFFFFFFFFFFFFFF,
}

// NewDefaultOutputBitStream creates a bitstream for writing, using the provided stream as
// the underlying I/O object.
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

// WriteBit writes the least significant bit of the input integer. Panics if the bitstream is closed
func (this *DefaultOutputBitStream) WriteBit(bit int) {
	if this.availBits <= 1 { // availBits = 0 if stream is closed => force pushCurrent() => panic
		this.current |= uint64(bit & 1)
		this.pushCurrent()
	} else {
		this.availBits--
		this.current |= (uint64(bit&1) << this.availBits)
	}

}

// WriteBits writes 'count' from 'value' to the bitstream.
// Panics if the bitstream is closed or 'count' is outside of [1..64].
// Returns the number of written bits.
func (this *DefaultOutputBitStream) WriteBits(value uint64, count uint) uint {
	if count > 64 {
		panic(fmt.Errorf("Invalid bit count: %d (must be in [1..64])", count))
	}

	// Pad the current position in buffer
	if this.availBits > count {
		// Enough spots available in 'current'
		this.availBits -= count
		this.current |= ((value & _OBS_MASKS[count]) << this.availBits)
	} else {
		// Not enough spots available in 'current'
		remaining := count - this.availBits
		this.current |= ((value & _OBS_MASKS[count]) >> remaining)
		this.pushCurrent()
		this.current = value << (64 - remaining)
		this.availBits -= remaining
	}

	return count
}

// WriteArray writes 'count' bits from 'bits' to the bitstream.
// Panics if the bitstream is closed or 'count' bigger than the number of bits
// in the 'bits' slice. Returns the number of written bits.
func (this *DefaultOutputBitStream) WriteArray(bits []byte, count uint) uint {
	if this.Closed() {
		panic(errors.New("Stream closed"))
	}

	if count > uint(len(bits)<<3) {
		panic(fmt.Errorf("Invalid length: %d (must be in [1..%d])", count, len(bits)<<3))
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

			if err := this.flush(); err != nil {
				panic(err)
			}
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
			this.current |= (value >> r)
			this.pushCurrent()
			this.current = (value << (64 - r))
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

// Close prevents further writes
func (this *DefaultOutputBitStream) Close() (bool, error) {
	if this.Closed() {
		return true, nil
	}

	savedBitIndex := this.availBits
	savedPosition := this.position
	savedCurrent := this.current
	avail := int(this.availBits)

	// Push last bytes (the very last byte may be incomplete)
	for shift := uint(56); avail > 0; shift -= 8 {
		this.buffer[this.position] = byte(this.current >> shift)
		this.position++
		avail -= 8
	}

	this.availBits = 0

	if err := this.flush(); err != nil {
		// Revert fields to allow subsequent attempts in case of transient failure
		this.availBits = savedBitIndex
		this.position = savedPosition
		this.current = savedCurrent
		return false, err
	}

	// Reset fields to force a flush() and trigger an error
	// on WriteBit() or WriteBits()
	this.closed = true
	this.position = 0
	this.availBits = 0
	this.buffer = make([]byte, 8)
	this.written -= 64 // adjust for method Written()
	return true, nil
}

// Written returns the number of bits written so far
func (this *DefaultOutputBitStream) Written() uint64 {
	// Number of bits flushed + bytes written in memory + bits written in memory
	return this.written + uint64(this.position<<3) + uint64(64-this.availBits)
}

// Closed says whether this stream can be written to
func (this *DefaultOutputBitStream) Closed() bool {
	return this.closed
}
