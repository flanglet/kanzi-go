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
	written   int64
	position  int    // index of current byte in buffer
	availBits uint   // bits not consumed in current
	current   uint64 // cached bits
	os        io.WriteCloser
	buffer    []byte
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

	this := &DefaultOutputBitStream{}
	this.buffer = make([]byte, bufferSize)
	this.os = stream
	this.availBits = 64

	return this, nil
}

// WriteBit writes the least significant bit of the input integer. Panics if the bitstream is closed
func (this *DefaultOutputBitStream) WriteBit(bit int) {
	if this.availBits <= 1 { // availBits = 0 if stream is closed => force push() => panic
		this.push(this.current | uint64(bit&1))
		this.current = 0
		this.availBits = 64
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

	this.current |= ((value << (64 - count)) >> (64 - this.availBits))

	if count >= this.availBits {
		// Not enough spots available in 'current'
		remaining := count - this.availBits
		this.push(this.current)
		this.current = value << (64 - remaining)
		this.availBits = 64 - remaining
	} else {
		this.availBits -= count
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

		maxPos := len(this.buffer) - 8

		// Copy bits array to internal buffer
		for remaining>>3 >= maxPos-this.position {
			copy(this.buffer[this.position:], bits[start:start+maxPos-this.position])
			start += (maxPos - this.position)
			remaining -= ((maxPos - this.position) << 3)
			this.position = maxPos

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
		if remaining >= 64 {
			r := 64 - this.availBits
			a := this.availBits

			for remaining >= 256 {
				val1 := binary.BigEndian.Uint64(bits[start:])
				val2 := binary.BigEndian.Uint64(bits[start+8:])
				val3 := binary.BigEndian.Uint64(bits[start+16:])
				val4 := binary.BigEndian.Uint64(bits[start+24:])
				this.current |= (val1 >> r)

				if this.position >= len(this.buffer)-32 {
					if err := this.flush(); err != nil {
						panic(err)
					}
				}

				binary.BigEndian.PutUint64(this.buffer[this.position:], this.current)
				binary.BigEndian.PutUint64(this.buffer[this.position+8:], (val1<<a)|(val2>>r))
				binary.BigEndian.PutUint64(this.buffer[this.position+16:], (val2<<a)|(val3>>r))
				binary.BigEndian.PutUint64(this.buffer[this.position+24:], (val3<<a)|(val4>>r))
				this.current = val4 << a
				start += 32
				remaining -= 256
				this.availBits = 64
				this.position += 32
			}

			for remaining >= 64 {
				val := binary.BigEndian.Uint64(bits[start:])
				this.push(this.current | (val >> r))
				this.availBits = 64
				this.current = val << a
				start += 8
				remaining -= 64
			}

			this.availBits = a
		}
	}

	// Last bytes
	for remaining >= 8 {
		this.WriteBits(uint64(bits[start]), 8)
		start++
		remaining -= 8
	}

	if remaining > 0 {
		this.WriteBits(binary.BigEndian.Uint64(bits[start:start+8])>>uint(64-remaining), uint(remaining))
	}

	return count
}

// Push 64 bits into buffer.
func (this *DefaultOutputBitStream) push(val uint64) {
	binary.BigEndian.PutUint64(this.buffer[this.position:this.position+8], val)
	this.position += 8

	if this.position >= len(this.buffer)-8 {
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

		this.written += (int64(this.position) << 3)
		this.position = 0
	}

	return nil
}

// Close prevents further writes
func (this *DefaultOutputBitStream) Close() error {
	if this.Closed() {
		return nil
	}

	savedBitIndex := this.availBits
	savedPosition := this.position
	savedCurrent := this.current

	// Push last bytes (the very last byte may be incomplete)
	for shift := uint(56); this.availBits < 64; shift -= 8 {
		this.buffer[this.position] = byte(this.current >> shift)
		this.position++
		this.availBits += 8
	}

	this.written -= int64(this.availBits - 64) // can be negative
	this.availBits = 64

	if err := this.flush(); err != nil {
		// Revert fields to allow subsequent attempts in case of transient failure
		this.availBits = savedBitIndex
		this.position = savedPosition
		this.current = savedCurrent
		return err
	}

	// Reset fields to force a flush() and trigger an error
	// on WriteBit() or WriteBits()
	this.closed = true
	this.position = 0
	this.availBits = 0
	this.written -= 64 // adjust because this.availBits = 0
	this.buffer = make([]byte, 8)
	return nil
}

// Written returns the number of bits written so far
func (this *DefaultOutputBitStream) Written() uint64 {
	// Number of bits flushed + bytes written in memory + bits written in memory
	return uint64(this.written + int64(this.position<<3) + int64(64-this.availBits))
}

// Closed says whether this stream can be written to
func (this *DefaultOutputBitStream) Closed() bool {
	return this.closed
}
