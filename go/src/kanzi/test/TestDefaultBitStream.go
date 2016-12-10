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

package main

import (
	"flag"
	"fmt"
	"kanzi"
	"kanzi/bitstream"
	"kanzi/io"
	"math/rand"
	"os"
	"time"
)

func main() {
	testCorrectnessAligned()
	testCorrectnessMisaligned()
	testSpeed() // Writes big output.bin file to local dir !!!
}

func testCorrectnessAligned() {
	fmt.Printf("Correctness Test - byte aligned\n")
	values := make([]int, 100)
	rand.Seed(time.Now().UTC().UnixNano())

	for test := 0; test < 10; test++ {
		var bs io.BufferStream
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

			if i%50 == 49 {
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

			if i%50 == 49 {
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
		}

		println()
		println()
	}

}

func testCorrectnessMisaligned() {
	fmt.Printf("Correctness Test - not byte aligned\n")
	values := make([]int, 100)
	rand.Seed(time.Now().UTC().UnixNano())

	for test := 0; test < 10; test++ {
		var bs io.BufferStream
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

			if i%50 == 49 {
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

			if i%50 == 49 {
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
		}

		println()
		println()
	}
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

func testSpeed() {
	fmt.Printf("Speed Test\n")
	var filename = flag.String("filename", "r:\\output.bin", "Ouput file name for speed test")
	flag.Parse()

	values := []uint64{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3,
		31, 14, 41, 15, 59, 92, 26, 65, 53, 35, 58, 89, 97, 79, 93, 32}
	iter := 150
	read := uint64(0)
	written := uint64(0)
	delta1 := int64(0)
	delta2 := int64(0)
	nn := 100000 * len(values)
	defer os.Remove(*filename)

	for test := 0; test < iter; test++ {
		file1, err := os.Create(*filename)

		if err != nil {
			fmt.Printf("Cannot create %s", *filename)

			return
		}

		bos := file1
		obs, _ := bitstream.NewDefaultOutputBitStream(bos, 16*1024)
		before := time.Now()
		for i := 0; i < nn; i++ {
			obs.WriteBits(values[i%len(values)], 1+uint(i&63))
		}

		// Close first to force flush()
		obs.Close()
		delta1 += time.Now().Sub(before).Nanoseconds()
		written += obs.Written()
		file1.Close()

		file2, err := os.Open(*filename)

		if err != nil {
			fmt.Printf("Cannot open %s", *filename)

			return
		}

		bis := file2
		ibs, _ := bitstream.NewDefaultInputBitStream(bis, 1024*1024)
		before = time.Now()

		for i := 0; i < nn; i++ {
			ibs.ReadBits(1 + uint(i&63))
		}

		ibs.Close()
		delta2 += time.Now().Sub(before).Nanoseconds()
		read += ibs.Read()
		file2.Close()
	}

	println()
	fmt.Printf("%v bits written (%v MB)\n", written, written/1024/8192)
	fmt.Printf("%v bits read (%v MB)\n", read, read/1024/8192)
	println()
	fmt.Printf("Write [ms]        : %v\n", delta1/1000000)
	fmt.Printf("Throughput [MB/s] : %d\n", (written/1024*1000/8192)/uint64(delta1/1000000))
	fmt.Printf("Read [ms]         : %v\n", delta2/1000000)
	fmt.Printf("Throughput [MB/s] : %d\n", (read/1024*1000/8192)/uint64(delta2/1000000))
}
