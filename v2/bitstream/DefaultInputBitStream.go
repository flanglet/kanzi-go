/*
Copyright 2011-2024 Frederic Langlet
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

// DefaultInputBitStream is the default implementation of InputBitStream
type DefaultInputBitStream struct {
	closed      bool
	read        int64
	position    int  // index of current byte (consumed if bitIndex == -1)
	availBits   uint // bits not consumed in current
	is          io.ReadCloser
	buffer      []byte
	maxPosition int
	current     uint64 // cached bits
}

// NewDefaultInputBitStream creates a bitstream for reading, using the provided stream as
// the underlying I/O object.
func NewDefaultInputBitStream(stream io.ReadCloser, bufferSize uint) (*DefaultInputBitStream, error) {
	if stream == nil {
		return nil, errors.New("Invalid null input stream parameter")
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

	this := new(DefaultInputBitStream)
	this.buffer = make([]byte, bufferSize)
	this.is = stream
	this.availBits = 0
	this.maxPosition = -1
	return this, nil
}

// ReadBit returns the next bit
func (this *DefaultInputBitStream) ReadBit() int {
	if this.availBits == 0 {
		this.pullCurrent() // Panic if stream is closed
	}

	this.availBits--
	return int(this.current>>this.availBits) & 1
}

// ReadBits reads 'count' bits from the stream and returns them as an uint64.
// It panics if the count is outside of the [1..64] range or the stream is closed.
// Returns the number of bits read.
func (this *DefaultInputBitStream) ReadBits(count uint) uint64 {
	if count == 0 || count > 64 {
		panic(fmt.Errorf("Invalid bit count: %d (must be in [1..64])", count))
	}

	if count <= this.availBits {
		// Enough spots available in 'current'
		this.availBits -= count
		return (this.current >> this.availBits) & (0xFFFFFFFFFFFFFFFF >> (64 - count))
	}

	// Not enough spots available in 'current'
	count -= this.availBits
	res := this.current & (0xFFFFFFFFFFFFFFFF >> (64 - this.availBits))
	this.pullCurrent()
	this.availBits -= count
	return (res << count) | (this.current >> this.availBits)
}

// ReadArray reads 'count' bits from the stream and returns them to the 'bits'
// slice. It panics if the stream is closed or the number of bits to read exceeds
// the length of the 'bits' slice. Returns the number of bits read.
func (this *DefaultInputBitStream) ReadArray(bits []byte, count uint) uint {
	if this.Closed() {
		panic(errors.New("Stream closed"))
	}

	if count == 0 {
		return 0
	}

	remaining := int(count)
	start := 0

	// Byte aligned cursor ?
	if this.availBits&7 == 0 {
		if this.availBits == 0 {
			this.pullCurrent()
		}

		// Empty this.current
		for this.availBits != 0 && remaining >= 8 {
			bits[start] = byte(this.ReadBits(8))
			start++
			remaining -= 8
		}

		availBytes := this.maxPosition + 1 - this.position

		// Copy internal buffer to bits array
		for (remaining >> 3) > availBytes {
			copy(bits[start:], this.buffer[this.position:this.maxPosition+1])
			start += availBytes
			remaining -= (availBytes << 3)

			if _, err := this.readFromInputStream(len(this.buffer)); err != nil {
				panic(err)
			}

			availBytes = this.maxPosition + 1 - this.position
		}

		r := (remaining >> 6) << 3

		if r > 0 {
			copy(bits[start:start+r], this.buffer[this.position:this.position+r])
			this.position += r
			start += r
			remaining -= (r << 3)
		}
	} else {
		// Not byte aligned
		r := 64 - this.availBits
		a := this.availBits

		for remaining >= 256 {
			v0 := this.current

			if this.position+32 > this.maxPosition {
				this.pullCurrent()
				this.availBits -= r
				binary.BigEndian.PutUint64(bits[start:start+8], (v0<<r)|(this.current>>uint(this.availBits)))
				start += 8
				remaining -= 64
				continue
			}

			v1 := binary.BigEndian.Uint64(this.buffer[this.position:])
			v2 := binary.BigEndian.Uint64(this.buffer[this.position+8:])
			v3 := binary.BigEndian.Uint64(this.buffer[this.position+16:])
			v4 := binary.BigEndian.Uint64(this.buffer[this.position+24:])
			this.position += 32
			binary.BigEndian.PutUint64(bits[start:], (v0<<r)|(v1>>a))
			binary.BigEndian.PutUint64(bits[start+8:], (v1<<r)|(v2>>a))
			binary.BigEndian.PutUint64(bits[start+16:], (v2<<r)|(v3>>a))
			binary.BigEndian.PutUint64(bits[start+24:], (v3<<r)|(v4>>a))
			start += 32
			remaining -= 256
			this.current = v4
		}

		for remaining >= 64 {
			v := this.current
			this.pullCurrent()
			this.availBits -= r
			binary.BigEndian.PutUint64(bits[start:start+8], (v<<r)|(this.current>>uint(this.availBits)))
			start += 8
			remaining -= 64
		}
	}

	// Last bytes
	for remaining >= 8 {
		bits[start] = byte(this.ReadBits(8))
		start++
		remaining -= 8
	}

	if remaining > 0 {
		bits[start] = byte(this.ReadBits(uint(remaining)) << uint(8-remaining))
	}

	return count
}

func (this *DefaultInputBitStream) readFromInputStream(count int) (int, error) {
	if this.Closed() {
		return 0, errors.New("Stream closed")
	}

	if count == 0 {
		return 0, nil
	}

	this.read += int64((this.maxPosition + 1) << 3)
	size, err := this.is.Read(this.buffer[0:count])
	this.position = 0

	if size <= 0 {
		this.maxPosition = -1
		return 0, errors.New("No more data to read in the bitstream")
	}

	this.maxPosition = size - 1
	return size, err
}

// HasMoreToRead returns false is the stream is closed or there is no
// more bit to read.
func (this *DefaultInputBitStream) HasMoreToRead() (bool, error) {
	if this.Closed() {
		return false, errors.New("Stream closed")
	}

	if this.position < this.maxPosition || this.availBits != 0 {
		return true, nil
	}

	_, err := this.readFromInputStream(len(this.buffer))
	return err == nil, err
}

// Pull 64 bits of current value from buffer.
func (this *DefaultInputBitStream) pullCurrent() {
	if this.position > this.maxPosition {
		if _, err := this.readFromInputStream(len(this.buffer)); err != nil {
			panic(err)
		}
	}

	if this.position+7 > this.maxPosition {
		// End of stream: overshoot max position => adjust bit index
		shift := uint(this.maxPosition-this.position) << 3
		this.availBits = shift + 8
		val := uint64(0)

		for this.position <= this.maxPosition {
			val |= (uint64(this.buffer[this.position]&0xFF) << shift)
			this.position++
			shift -= 8
		}

		this.current = val
	} else {
		// Regular processing, buffer length is multiple of 8
		this.current = binary.BigEndian.Uint64(this.buffer[this.position : this.position+8])
		this.availBits = 64
		this.position += 8
	}

}

// Close prevents further reads (beyond the available bits)
func (this *DefaultInputBitStream) Close() (bool, error) {
	if this.Closed() {
		return true, nil
	}

	this.closed = true

	// Reset fields to force a readFromInputStream() and trigger an error
	// on ReadBit() or ReadBits()
	this.read -= int64(this.availBits) // can be negative
	this.availBits = 0
	this.maxPosition = -1
	return true, nil
}

// Read returns the number of bits read so far
func (this *DefaultInputBitStream) Read() uint64 {
	return uint64(this.read + int64(this.position)<<3 - int64(this.availBits))
}

// Closed says whether this stream can be read from
func (this *DefaultInputBitStream) Closed() bool {
	return this.closed
}
