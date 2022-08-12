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

package bitstream

import (
	"errors"
	"fmt"
	"io"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

// DebugInputBitStream is an implementation of InputBitStream used for debugging.
type DebugInputBitStream struct {
	delegate  kanzi.InputBitStream
	out       io.Writer
	mark      bool
	hexa      bool
	current   byte
	width     int
	lineIndex int
}

// NewDebugInputBitStream creates a DebugInputBitStream wrapped around 'ibs'.
// All calls are delegated to the 'ibs' InputBitStream and read bits are logged
// to the provided io.Writer.
func NewDebugInputBitStream(ibs kanzi.InputBitStream, writer io.Writer) (*DebugInputBitStream, error) {
	if ibs == nil {
		return nil, errors.New("The delegate cannot be null")
	}

	if writer == nil {
		return nil, errors.New("The writer cannot be null")
	}

	this := new(DebugInputBitStream)
	this.delegate = ibs
	this.out = writer
	this.width = 80
	return this, nil
}

// ReadBit returns the next bit in the bitstream. Panics if closed or EOS is reached.
// Calls ReadBit() on the underlying bitstream delegate.
func (this *DebugInputBitStream) ReadBit() int {
	res := this.delegate.ReadBit()

	this.current <<= 1
	this.current |= byte(res)
	this.lineIndex++

	if res&1 == 1 {
		fmt.Fprintf(this.out, "1")
	} else {
		fmt.Fprintf(this.out, "0")
	}

	if this.mark == true {
		fmt.Fprintf(this.out, "r")
	}

	if this.width > 7 {
		if (this.lineIndex-1)%this.width == this.width-1 {
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
	} else if this.lineIndex&7 == 0 {
		if this.hexa == true {
			this.printByte(this.current)
		} else {
			fmt.Fprintf(this.out, " ")
		}
	}

	return res
}

// ReadBits reads 'length' (in [1..64]) bits from the bitstream .
// Returns the bits read as an uint64.
// Panics if closed or EOS is reached.
// Calls ReadBits() on the underlying bitstream delegate.
func (this *DebugInputBitStream) ReadBits(length uint) uint64 {
	res := this.delegate.ReadBits(length)

	for i := uint(1); i <= length; i++ {
		bit := (res >> (length - i)) & 1
		this.current <<= 1
		this.current |= byte(bit)
		this.lineIndex++
		fmt.Fprintf(this.out, "%d", bit)

		if this.mark == true && i == length {
			fmt.Fprintf(this.out, "r")
		}

		if this.width > 7 {
			if this.lineIndex%this.width == 0 {
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

// ReadArray reads 'length' bits from the bitstream and put them in the byte slice.
// Returns the number of bits read.
// Panics if closed or EOS is reached.
// Calls ReadArray() on the underlying bitstream delegate.
func (this *DebugInputBitStream) ReadArray(bits []byte, count uint) uint {
	count = this.delegate.ReadArray(bits, count)

	for i := uint(0); i < count>>3; i++ {
		for j := 7; j >= 0; j-- {
			bit := (bits[i] >> uint(j)) & 1
			this.current <<= 1
			this.current |= byte(bit)
			this.lineIndex++
			fmt.Fprintf(this.out, "%d", bit)

			if this.mark == true && j == int(count) {
				fmt.Fprintf(this.out, "r")
			}

			if this.width > 7 {
				if this.lineIndex%this.width == 0 {
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
			} else if this.lineIndex&7 == 0 {
				if this.hexa == true {
					this.printByte(this.current)
				} else {
					fmt.Fprintf(this.out, " ")
				}
			}
		}
	}

	return count
}

// HasMoreToRead returns false when the bitstream is closed or the EOS has been reached
// Calls HasMoreToRead() on the underlying bitstream delegate.
func (this *DebugInputBitStream) HasMoreToRead() (bool, error) {
	return this.delegate.HasMoreToRead()
}

func (this *DebugInputBitStream) printByte(val byte) {
	if val < 10 {
		fmt.Fprintf(this.out, " [00%1d] ", val)
	} else if val < 100 {
		fmt.Fprintf(this.out, " [0%2d] ", val)
	} else {
		fmt.Fprintf(this.out, " [%3d] ", val)
	}
}

// Close makes the bitstream unavailable for further reads.
// Calls Close() on the underlying bitstream delegate.
func (this *DebugInputBitStream) Close() (bool, error) {
	return this.delegate.Close()
}

// Read returns the number of bits read
// Calls Read() on the underlying bitstream delegate.
func (this *DebugInputBitStream) Read() uint64 {
	return this.delegate.Read()
}

// Mark sets the internal mark state. When true. displays 'r'
// after each bit  or bit sequence read from the bitstream delegate.
func (this *DebugInputBitStream) Mark(mark bool) {
	this.mark = mark
}

// ShowByte sets the internal show byte state. When true, displays
// the hexadecimal value after the bits.
func (this *DebugInputBitStream) ShowByte(show bool) {
	this.hexa = show
}
