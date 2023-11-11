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
	"errors"
	"fmt"
	"io"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

// DebugOutputBitStream is an implementation of OutputBitStream used for debugging.
type DebugOutputBitStream struct {
	delegate  kanzi.OutputBitStream
	out       io.Writer
	mark      bool
	hexa      bool
	current   byte
	width     int
	lineIndex int
}

// NewDebugOutputBitStream creates a DebugOutputBitStream wrapped around 'obs'.
// All calls are delegated to the 'obs' OutputBitStream and written bits are logged
// to the provided io.Writer.
func NewDebugOutputBitStream(obs kanzi.OutputBitStream, writer io.Writer) (*DebugOutputBitStream, error) {
	if obs == nil {
		return nil, errors.New("The delegate cannot be null")
	}

	if writer == nil {
		return nil, errors.New("The writer cannot be null")
	}

	this := &DebugOutputBitStream{}
	this.delegate = obs
	this.out = writer
	this.width = 80
	return this, nil
}

// WriteBit writes the least significant bit of the input integer
// Panics if closed or an IO error is received.
// Calls WriteBit() on the underlying bitstream delegate.
func (this *DebugOutputBitStream) WriteBit(bit int) {
	bit &= 1
	fmt.Fprintf(this.out, "%d", bit)
	this.current <<= 1
	this.current |= byte(bit)
	this.lineIndex++

	if this.mark == true {
		fmt.Fprintf(this.out, "w")
	}

	if this.width > 7 && (this.lineIndex-1)%this.width == this.width-1 {
		if this.hexa == true {
			this.printByte(this.current)
		}

		fmt.Fprintf(this.out, "\n")
		this.lineIndex = 0
	} else if this.lineIndex&7 == 0 {
		if this.hexa == true {
			this.printByte(this.current)
		} else {
			fmt.Fprintf(this.out, " ")
		}
	}

	this.delegate.WriteBit(bit)
}

// WriteBits writes the least significant bits of 'bits' to the bitstream.
// Length is the number of bits to write (in [1..64]).
// Returns the number of bits written.
// Panics if closed or an IO error is received.
// Calls WriteBits() on the underlying bitstream delegate.
func (this *DebugOutputBitStream) WriteBits(bits uint64, length uint) uint {
	res := this.delegate.WriteBits(bits, length)

	for i := uint(1); i <= length; i++ {
		bit := (bits >> (length - i)) & 1
		this.current <<= 1
		this.current |= byte(bit)
		this.lineIndex++
		fmt.Fprintf(this.out, "%d", bit)

		if this.mark == true && i == length {
			fmt.Fprintf(this.out, "w")
		}

		if this.width > 7 && this.lineIndex%this.width == 0 {
			if this.hexa == true {
				this.printByte(this.current)
			}

			fmt.Fprintf(this.out, "\n")
			this.lineIndex = 0
		} else if this.lineIndex&7 == 0 {
			if this.hexa == true {
				this.printByte(this.current)
			} else {
				fmt.Fprintf(this.out, " ")
			}
		}
	}

	return res
}

// WriteArray writes bits out of the byte slice. Length is the number of bits.
// Returns the number of bits written.
// Panics if closed or an IO error is received.
// Calls WriteArray() on the underlying bitstream delegate.
func (this *DebugOutputBitStream) WriteArray(bits []byte, count uint) uint {
	res := this.delegate.WriteArray(bits, count)

	for i := uint(0); i < (count >> 3); i++ {
		for j := uint(8); j > 0; j-- {
			bit := (bits[i] >> (j - 1)) & 1
			this.current <<= 1
			this.current |= byte(bit)
			this.lineIndex++
			fmt.Fprintf(this.out, "%d", bit)

			if this.mark == true && i == count {
				fmt.Fprintf(this.out, "w")
			}

			if this.width > 7 && this.lineIndex%this.width == 0 {
				if this.hexa == true {
					this.printByte(this.current)
				}

				fmt.Fprintf(this.out, "\n")
				this.lineIndex = 0
			} else if this.lineIndex&7 == 0 {
				if this.hexa == true {
					this.printByte(this.current)
				} else {
					fmt.Fprintf(this.out, " ")
				}
			}
		}
	}

	return res
}

func (this *DebugOutputBitStream) printByte(val byte) {
	if val < 10 {
		fmt.Fprintf(this.out, " [00%1d] ", val)
	} else if val < 100 {
		fmt.Fprintf(this.out, " [0%2d] ", val)
	} else {
		fmt.Fprintf(this.out, " [%3d] ", val)
	}
}

// Close makes the bitstream unavailable for further writes.
// Calls Close() on the underlying bitstream delegate.
func (this *DebugOutputBitStream) Close() (bool, error) {
	return this.delegate.Close()
}

// Written returns the number of bits written
// Calls Written() on the underlying bitstream delegate.
func (this *DebugOutputBitStream) Written() uint64 {
	return this.delegate.Written()
}

// Mark sets the internal mark state. When true. displays 'w'
// after each bit  or bit sequence read from the bitstream delegate.
func (this *DebugOutputBitStream) Mark(mark bool) {
	this.mark = mark
}

// ShowByte sets the internal show byte state. When true, displays
// the hexadecimal value after the bits.
func (this *DebugOutputBitStream) ShowByte(show bool) {
	this.hexa = show
}
