/*
Copyright 2011-2021 Frederic Langlet
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

package main

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	kanzi "github.com/flanglet/kanzi-go"
	"github.com/flanglet/kanzi-go/bitstream"
	"github.com/flanglet/kanzi-go/util"
)

func TestBitStreamAligned(b *testing.T) {
	testCorrectnessAligned1()
	testCorrectnessAligned2()
}

func TestBitStreamMisaligned(b *testing.T) {
	testCorrectnessMisaligned1()
	testCorrectnessMisaligned2()
}

func testCorrectnessAligned1() error {
	fmt.Printf("Correctness Test - write long - byte aligned\n")
	values := make([]int, 100)
	rand.Seed(time.Now().UTC().UnixNano())

	// Check correctness of read() and written()
	for t := 1; t <= 32; t++ {
		var bs util.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		fmt.Println()
		obs.WriteBits(0x0123456789ABCDEF, uint(t))
		fmt.Printf("Written (before close): %v\n", obs.Written())
		obs.Close()
		fmt.Printf("Written (after close): %v\n", obs.Written())

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		ibs.ReadBits(uint(t))

		if ibs.Read() == uint64(t) {
			fmt.Println("OK")
		} else {
			fmt.Println("KO")
			return errors.New("Invalid number of bits read")
		}

		fmt.Printf("Read (before close): %v\n", ibs.Read())
		ibs.Close()
		fmt.Printf("Read (after close): %v\n", ibs.Read())
	}

	for test := 1; test <= 10; test++ {
		var bs util.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbgbs.ShowByte(true)
		dbgbs.Mark(true)

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
			dbgbs.WriteBits(uint64(values[i]), 32)
		}

		// Close first to force flush()
		dbgbs.Close()

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		fmt.Printf("\nRead:\n")
		ok := true

		for i := range values {
			x := ibs.ReadBits(32)
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

		ibs.Close()
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgbs.Written())
		fmt.Printf("Bits read: %v\n", ibs.Read())

		if ok {
			fmt.Printf("\nSuccess\n")
		} else {
			fmt.Printf("\nFailure\n")
			return fmt.Errorf("Bits written: %v, its read: %v", dbgbs.Written(), ibs.Read())
		}

		println()
		println()
	}

	return error(nil)
}

func testCorrectnessMisaligned1() error {
	fmt.Printf("Correctness Test - write long - not byte aligned\n")
	values := make([]int, 100)
	rand.Seed(time.Now().UTC().UnixNano())

	// Check correctness of read() and written()
	for t := 1; t <= 32; t++ {
		var bs util.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		fmt.Println()
		obs.WriteBit(1)
		obs.WriteBits(0x0123456789ABCDEF, uint(t))
		fmt.Printf("Written (before close): %v\n", obs.Written())
		obs.Close()
		fmt.Printf("Written (after close): %v\n", obs.Written())

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		ibs.ReadBit()
		ibs.ReadBits(uint(t))

		if ibs.Read() == uint64(t+1) {
			fmt.Println("OK")
		} else {
			fmt.Println("KO")
			return errors.New("Invalid number of bits read")
		}
	}

	for test := 1; test <= 10; test++ {
		var bs util.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbgbs.ShowByte(true)
		dbgbs.Mark(true)

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
			dbgbs.WriteBits(uint64(values[i]), 1+uint(i&63))
		}

		// Close first to force flush()
		dbgbs.Close()
		testWritePostClose(dbgbs)

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		fmt.Printf("\nRead:\n")
		ok := true

		for i := range values {
			x := ibs.ReadBits(1 + uint(i&63))
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

		ibs.Close()
		testReadPostClose(ibs)
		bs.Close()

		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgbs.Written())
		fmt.Printf("Bits read: %v\n", ibs.Read())

		if ok {
			fmt.Printf("\nSuccess\n")
		} else {
			fmt.Printf("\nFailure\n")
			return fmt.Errorf("Bits written: %v, its read: %v", dbgbs.Written(), ibs.Read())
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
	rand.Seed(time.Now().UTC().UnixNano())

	for test := 1; test <= 10; test++ {
		var bs util.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbgbs.ShowByte(true)
		dbgbs.Mark(true)
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
		dbgbs.WriteArray(input, count)

		// Close first to force flush()
		dbgbs.Close()

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		fmt.Printf("\nRead:\n")
		r := ibs.ReadArray(output, count)
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

		ibs.Close()
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgbs.Written())
		fmt.Printf("Bits read: %v\n", ibs.Read())

		if ok {
			fmt.Printf("\nSuccess\n")
		} else {
			fmt.Printf("\nFailure\n")
			return fmt.Errorf("Bits written: %v, its read: %v", dbgbs.Written(), ibs.Read())
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
	rand.Seed(time.Now().UTC().UnixNano())

	for test := 1; test <= 10; test++ {
		var bs util.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbgbs.ShowByte(true)
		dbgbs.Mark(true)
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
		dbgbs.WriteBit(0)
		dbgbs.WriteArray(input[1:], count)

		// Close first to force flush()
		dbgbs.Close()

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		fmt.Printf("\nRead:\n")
		ibs.ReadBit()
		r := ibs.ReadArray(output[1:], count)
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

		ibs.Close()
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgbs.Written())
		fmt.Printf("Bits read: %v\n", ibs.Read())

		if ok {
			fmt.Printf("\nSuccess\n")
		} else {
			fmt.Printf("\nFailure\n")
			return fmt.Errorf("Bits written: %v, its read: %v", dbgbs.Written(), ibs.Read())
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
