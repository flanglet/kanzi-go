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
	"errors"
	"fmt"
	"io"
	"kanzi"
)

type DebugOutputBitStream struct {
	delegate  kanzi.OutputBitStream
	out       io.Writer
	mark      bool
	hexa      bool
	current   byte
	width     int
	lineIndex int
}

func NewDebugOutputBitStream(obs kanzi.OutputBitStream, writer io.Writer) (*DebugOutputBitStream, error) {
	if obs == nil {
		return nil, errors.New("The delegate cannot be null")
	}

	if writer == nil {
		return nil, errors.New("The writer cannot be null")
	}

	this := new(DebugOutputBitStream)
	this.delegate = obs
	this.out = writer
	this.width = 80
	return this, nil
}

func (this *DebugOutputBitStream) WriteBit(bit int) {
	bit &= 1
	fmt.Fprintf(this.out, "%d", bit)
	this.current <<= 1
	this.current |= byte(bit)
	this.lineIndex++

	if this.mark == true {
		fmt.Fprintf(this.out, "w")
	}

	if this.width > 7 {
		if (this.lineIndex-1)%this.width == this.width-1 {
			if this.hexa == true {
				fmt.Fprintf(this.out, "[%d] ", this.current)
			}

			fmt.Fprintf(this.out, "\n")
			this.lineIndex = 0
		} else if this.lineIndex&7 == 0 {
			fmt.Fprintf(this.out, " ")

			if this.hexa == true {
				fmt.Fprintf(this.out, "[%d] ", this.current)
			}
		}
	} else if this.lineIndex&7 == 0 {
		fmt.Fprintf(this.out, " ")

		if this.hexa == true {
			fmt.Fprintf(this.out, "[%d] ", this.current)
		}
	}

	this.delegate.WriteBit(bit)
}

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

		if this.width > 7 {
			if this.lineIndex%this.width == 0 {
				if this.hexa == true {
					fmt.Fprintf(this.out, "[%d] ", this.current)
				}

				fmt.Fprintf(this.out, "\n")
				this.lineIndex = 0
			} else if this.lineIndex&7 == 0 {
				fmt.Fprintf(this.out, " ")

				if this.hexa == true {
					fmt.Fprintf(this.out, "[%d] ", this.current)
				}
			}
		} else if this.lineIndex&7 == 0 {
			fmt.Fprintf(this.out, " ")

			if this.hexa == true {
				fmt.Fprintf(this.out, "[%d] ", this.current)
			}
		}
	}

	return res
}

func (this *DebugOutputBitStream) Close() (bool, error) {
	return this.delegate.Close()
}

func (this *DebugOutputBitStream) Written() uint64 {
	return this.delegate.Written()
}

func (this *DebugOutputBitStream) Mark(mark bool) {
	this.mark = mark
}

func (this *DebugOutputBitStream) ShowByte(show bool) {
	this.hexa = show
}
