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
	"io"
)

type DefaultInputBitStream struct {
	closed      bool
	read        uint64
	position    int  // index of current byte (consumed if bitIndex == 63)
	bitIndex    uint // index of current bit to read
	is          io.ReadCloser
	buffer      []byte
	maxPosition int
	current     uint64 // cached bits
}

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
	this.bitIndex = 63
	this.maxPosition = -1
	return this, nil
}

// Return 1 or 0
func (this *DefaultInputBitStream) ReadBit() int {
	if this.bitIndex == 63 {
		this.pullCurrent() // Panic if stream is closed
	}

	bit := int(this.current>>this.bitIndex) & 1
	this.bitIndex = (this.bitIndex + 63) & 63
	return bit
}

func (this *DefaultInputBitStream) ReadBits(count uint) uint64 {
	if count == 0 || count > 64 {
		panic(fmt.Errorf("Invalid count: %v (must be in [1..64])", count))
	}

	var res uint64

	if count <= this.bitIndex+1 {
		// Enough spots available in 'current'
		shift := this.bitIndex + 1 - count

		if this.bitIndex == 63 {
			this.pullCurrent()
			shift += (this.bitIndex - 63) // adjust if bitIndex != 63 (end of stream)
		}

		res = (this.current >> shift) & (0xFFFFFFFFFFFFFFFF >> (64 - count))
		this.bitIndex = (this.bitIndex - count) & 63
	} else {
		// Not enough spots available in 'current'
		remaining := count - this.bitIndex - 1
		res = this.current & (0xFFFFFFFFFFFFFFFF >> (63 - this.bitIndex))
		this.pullCurrent()
		res <<= remaining
		this.bitIndex -= remaining
		res |= (this.current >> (this.bitIndex + 1))
	}

	return res
}

func (this *DefaultInputBitStream) readFromInputStream(count int) (int, error) {
	if this.Closed() {
		return 0, errors.New("Stream closed")
	}

	this.read += uint64((this.maxPosition + 1) << 3)
	size, err := this.is.Read(this.buffer[0:count])
	this.position = 0

	if size <= 0 {
		this.maxPosition = -1
	} else {
		this.maxPosition = size - 1
	}

	if err != nil {
		return size, err
	}

	if size <= 0 {
		return size, errors.New("No more data to read in the bitstream")
	}

	return size, nil
}

func (this *DefaultInputBitStream) HasMoreToRead() (bool, error) {
	if this.Closed() {
		return false, errors.New("Stream closed")
	}

	if this.position < this.maxPosition || this.bitIndex != 63 {
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

	var val uint64

	if this.position+7 > this.maxPosition {
		// End of stream: overshoot max position => adjust bit index
		shift := uint(this.maxPosition-this.position) << 3
		this.bitIndex = shift + 7
		val = 0

		for this.position <= this.maxPosition {
			val |= (uint64(this.buffer[this.position]) << shift)
			this.position++
			shift -= 8
		}
	} else {
		// Regular processing, buffer length is multiple of 8
		buf := this.buffer[this.position:this.position+8]
		val = uint64(buf[0]) << 56
		val |= (uint64(buf[1]) << 48)
		val |= (uint64(buf[2]) << 40)
		val |= (uint64(buf[3]) << 32)
		val |= (uint64(buf[4]) << 24)
		val |= (uint64(buf[5]) << 16)
		val |= (uint64(buf[6]) << 8)
		val |= uint64(buf[7])
		this.bitIndex = 63
		this.position += 8
	}

	this.current = val
}

func (this *DefaultInputBitStream) Close() (bool, error) {
	if this.Closed() {
		return true, nil
	}

	this.closed = true
	this.read += uint64(63)

	// Reset fields to force a readFromInputStream() and trigger an error
	// on ReadBit() or ReadBits()
	this.bitIndex = 63
	this.maxPosition = -1
	return true, nil
}

// Return number of bits read so far
func (this *DefaultInputBitStream) Read() uint64 {
	return this.read + uint64(this.position)<<3 - uint64(this.bitIndex)
}

func (this *DefaultInputBitStream) Closed() bool {
	return this.closed
}
