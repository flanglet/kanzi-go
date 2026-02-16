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

package bitstream

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_SUCCESS = "Success"
	_FAILURE = "Failure"
	_READ    = "Read: "
)

func TestBitStreamAligned(b *testing.T) {
	if err := testCorrectnessAligned1(); err != nil {
		b.Errorf(err.Error())
	}

	if err := testCorrectnessAligned2(); err != nil {
		b.Errorf(err.Error())
	}
}

func TestBitStreamMisaligned(b *testing.T) {
	if err := testCorrectnessMisaligned1(); err != nil {
		b.Errorf(err.Error())
	}

	if err := testCorrectnessMisaligned2(); err != nil {
		b.Errorf(err.Error())
	}
}

func testCorrectnessAligned1() error {
	fmt.Printf("Correctness Test - write long - byte aligned\n")
	values := make([]int, 100)

	// Check correctness of read() and written()
	for t := 1; t <= 32; t++ {
		bs := internal.NewBufferStream()
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		fmt.Println()
		obs.WriteBits(0x0123456789ABCDEF, uint(t))
		fmt.Printf("Written (before close): %v\n", obs.Written())
		obs.Close()
		fmt.Printf("Written (after close): %v\n", obs.Written())

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)
		dbgibs.ReadBits(uint(t))

		if dbgibs.Read() == uint64(t) {
			fmt.Println("\nOK")
		} else {
			fmt.Println("\nKO")
			return errors.New("Invalid number of bits read")
		}

		fmt.Printf("Read (before close): %v\n", dbgibs.Read())
		dbgibs.Close()
		fmt.Printf("Read (after close): %v\n", dbgibs.Read())
	}

	for test := 1; test <= 10; test++ {
		bs := internal.NewBufferStream(make([]byte, 0, 16384))
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)

		for i := range values {
			if test < 5 {
				values[i] = rand.Intn(test*1000 + 100)
			} else {
				values[i] = rand.Intn(1 << 31)
			}

			fmt.Printf("%v ", values[i])

			if i%20 == 19 {
				println()
			}
		}

		println()
		println()

		for i := range values {
			dbgobs.WriteBits(uint64(values[i]), 32)
		}

		// Close first to force flush()
		dbgobs.Close()

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)
		println()
		fmt.Println(_READ)
		ok := true

		for i := range values {
			x := dbgibs.ReadBits(32)
			fmt.Printf("%v", x)

			if int(x) == values[i] {
				fmt.Printf(" ")
			} else {
				fmt.Printf("* ")
				ok = false
			}

			if i%20 == 19 {
				println()
			}
		}

		dbgobs.Close()
		dbgibs.Close()
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgobs.Written())
		fmt.Printf("Bits read: %v\n", dbgibs.Read())
		println()

		if ok {
			fmt.Println(_SUCCESS)
		} else {
			fmt.Println(_FAILURE)
			return fmt.Errorf("Bits written: %v, bits read: %v", dbgobs.Written(), dbgibs.Read())
		}

		println()
		println()
	}

	return error(nil)
}

func testCorrectnessMisaligned1() error {
	fmt.Printf("Correctness Test - write long - not byte aligned\n")
	values := make([]int, 100)

	// Check correctness of read() and written()
	for t := 1; t <= 32; t++ {
		bs := internal.NewBufferStream(make([]byte, 16384))
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)
		fmt.Println()
		dbgobs.WriteBit(1)
		dbgobs.WriteBits(0x0123456789ABCDEF, uint(t))
		fmt.Printf("Written (before close): %v\n", obs.Written())
		dbgobs.Close()
		obs.Close()
		fmt.Printf("Written (after close): %v\n", obs.Written())

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)
		dbgibs.ReadBit()
		dbgibs.ReadBits(uint(t))

		if dbgibs.Read() == uint64(t+1) {
			fmt.Println("OK")
		} else {
			fmt.Println("KO")
			return errors.New("Invalid number of bits read")
		}

		dbgibs.Close()
		bs.Close()
	}

	for test := 1; test <= 10; test++ {
		bs := internal.NewBufferStream()
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)

		for i := range values {
			if test < 5 {
				values[i] = rand.Intn(test*1000 + 100)
			} else {
				values[i] = rand.Intn(1 << 31)
			}

			mask := (1 << (1 + uint(i&63))) - 1
			values[i] &= mask
			fmt.Printf("%v ", values[i])

			if i%20 == 19 {
				println()
			}
		}

		println()
		println()

		for i := range values {
			dbgobs.WriteBits(uint64(values[i]), 1+uint(i&63))
		}

		// Close first to force flush()
		dbgobs.Close()
		obs.Close()
		testWritePostClose(dbgobs)

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)
		println()
		fmt.Println(_READ)
		ok := true

		for i := range values {
			x := dbgibs.ReadBits(1 + uint(i&63))
			fmt.Printf("%v", x)

			if int(x) == values[i] {
				fmt.Printf(" ")
			} else {
				fmt.Printf("* ")
				ok = false
			}

			if i%20 == 19 {
				println()
			}
		}

		dbgibs.Close()
		testReadPostClose(dbgibs)
		bs.Close()

		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgobs.Written())
		fmt.Printf("Bits read: %v\n", dbgibs.Read())
		println()

		if ok {
			fmt.Println(_SUCCESS)
		} else {
			fmt.Println(_FAILURE)
			return fmt.Errorf("Bits written: %v, bits read: %v", dbgobs.Written(), dbgibs.Read())
		}

		println()
		println()
	}

	return error(nil)
}

func testCorrectnessAligned2() error {
	fmt.Printf("Correctness Test - write array - byte aligned\n")
	input := make([]byte, 100)
	output := make([]byte, 100)

	for test := 1; test <= 10; test++ {
		bs := internal.NewBufferStream()
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)
		println()

		for i := range input {
			if test < 5 {
				input[i] = byte(rand.Intn(test*1000 + 100))
			} else {
				input[i] = byte(rand.Intn(1 << 31))
			}

			fmt.Printf("%v ", input[i])

			if i%20 == 19 {
				println()
			}
		}

		count := uint(8 + test*(20+(test&1)) + (test & 3))
		println()
		println()
		dbgobs.WriteArray(input, count)

		// Close first to force flush()
		dbgobs.Close()

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)

		println()
		fmt.Println(_READ)
		r := dbgibs.ReadArray(output, count)
		ok := r == count

		if ok == true {
			for i := 0; i < int(r>>3); i++ {
				fmt.Printf("%v", output[i])

				if output[i] == input[i] {
					fmt.Printf(" ")
				} else {
					fmt.Printf("* ")
					ok = false
				}

				if i%20 == 19 {
					println()
				}
			}
		}

		dbgibs.Close()
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgobs.Written())
		fmt.Printf("Bits read: %v\n", dbgibs.Read())

		if ok {
			fmt.Printf("\nSuccess\n")
		} else {
			fmt.Printf("\nFailure\n")
			return fmt.Errorf("Bits written: %v, bits read: %v", dbgobs.Written(), dbgibs.Read())
		}

		println()
		println()
	}

	return error(nil)
}

func testCorrectnessMisaligned2() error {
	fmt.Printf("Correctness Test - write array - not byte aligned\n")
	input := make([]byte, 100)
	output := make([]byte, 100)

	for test := 1; test <= 10; test++ {
		bs := internal.NewBufferStream()
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)
		println()

		for i := range input {
			if test < 5 {
				input[i] = byte(rand.Intn(test*1000 + 100))
			} else {
				input[i] = byte(rand.Intn(1 << 31))
			}

			fmt.Printf("%v ", input[i])

			if i%20 == 19 {
				println()
			}
		}

		count := uint(8 + test*(20+(test&1)) + (test & 3))
		println()
		println()
		dbgobs.WriteBit(0)
		dbgobs.WriteArray(input[1:], count)

		// Close first to force flush()
		dbgobs.Close()

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)

		println()
		fmt.Println(_READ)
		dbgibs.ReadBit()
		r := dbgibs.ReadArray(output[1:], count)
		ok := r == count

		if ok == true {
			for i := 1; i < 1+int(r>>3); i++ {
				fmt.Printf("%v", output[i])

				if output[i] == input[i] {
					fmt.Printf(" ")
				} else {
					fmt.Printf("* ")
					ok = false
				}

				if i%20 == 19 {
					println()
				}
			}
		}

		dbgibs.Close()
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgobs.Written())
		fmt.Printf("Bits read: %v\n", dbgibs.Read())

		if ok {
			fmt.Printf("\nSuccess\n")
		} else {
			fmt.Printf("\nFailure\n")
			return fmt.Errorf("Bits written: %v, bits read: %v", dbgobs.Written(), dbgibs.Read())
		}

		println()
		println()
	}

	return error(nil)
}

func testWritePostClose(obs kanzi.OutputBitStream) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Error: %v\n", r.(error).Error())
		}
	}()

	fmt.Printf("\nTrying to write to closed stream\n")
	obs.WriteBit(1)
}

func testReadPostClose(ibs kanzi.InputBitStream) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Error: %v\n", r.(error).Error())
		}
	}()

	fmt.Printf("\nTrying to read from closed stream\n")
	ibs.ReadBit()
}
