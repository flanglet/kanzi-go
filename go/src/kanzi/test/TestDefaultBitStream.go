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

package main

import (
	"flag"
	"fmt"
	"kanzi/bitstream"
	"kanzi/io"
	"kanzi/util"
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
		buffer := make([]byte, 16384)
		os_, _ := util.NewByteArrayOutputStream(buffer, true)
		obs, _ := bitstream.NewDefaultOutputBitStream(os_, 16384)
		dbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbs.ShowByte(true)

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
			dbs.WriteBits(uint64(values[i]), 32)
		}

		// Close first to force flush()
		dbs.Close()

		is_, _ := util.NewByteArrayInputStream(buffer, true)
		ibs, _ := bitstream.NewDefaultInputBitStream(is_, 16384)
		fmt.Printf("\nRead:\n")
		ok := true

		for i := range values {
			x, _ := ibs.ReadBits(32)
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
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbs.Written())
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
		buffer := make([]byte, 16384)
		os_, _ := util.NewByteArrayOutputStream(buffer, false)
		obs, _ := bitstream.NewDefaultOutputBitStream(os_, 16384)
		dbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbs.ShowByte(true)

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
			dbs.WriteBits(uint64(values[i]), 1+uint(i&63))
		}

		// Close first to force flush()
		dbs.Close()

		fmt.Printf("\nTrying to write to closed stream\n")
		errWClosed := dbs.WriteBit(1)
		
		if errWClosed != nil { 
		   fmt.Printf("Error: %v\n", errWClosed.Error())
		}
		
		is_, _ := util.NewByteArrayInputStream(buffer, false)
		ibs, _ := bitstream.NewDefaultInputBitStream(is_, 16384)
		fmt.Printf("\nRead:\n")
		ok := true

		for i := range values {
			x, _ := ibs.ReadBits(1 + uint(i&63))
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
		
		fmt.Printf("\nTrying to read from closed stream\n")
		_, errRClosed := ibs.ReadBit()
		
		if errRClosed != nil { 
		   fmt.Printf("Error: %v\n", errRClosed.Error())
		}
		
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbs.Written())
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

func testSpeed() {
	fmt.Printf("Speed Test\n")
	var filename = flag.String("filename", "output.bin", "Ouput file name for speed test")
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

		bos, _ := io.NewBufferedOutputStream(file1)
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

		bis, _ := io.NewBufferedInputStream(file2)
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
