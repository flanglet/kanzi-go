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
	"fmt"
	"kanzi/bitstream"
	"kanzi/entropy"
	"kanzi/io"
	"math/rand"
	"os"
	"time"
)

func main() {
	TestCorrectness()
	TestSpeed()
}

func TestCorrectness() {
	fmt.Printf("\n\nCorrectness test")

	// Test behavior
	for ii := 1; ii < 20; ii++ {
		fmt.Printf("\n\nTest %v\n", ii)
		var values []byte
		rand.Seed(time.Now().UTC().UnixNano())

		if ii == 3 {
			values = []byte{0, 0, 32, 15, -4 & 0xFF, 16, 0, 16, 0, 7, -1 & 0xFF, -4 & 0xFF, -32 & 0xFF, 0, 31, -1 & 0xFF}
		} else if ii == 2 {
			values = []byte{0x3d, 0x4d, 0x54, 0x47, 0x5a, 0x36, 0x39, 0x26, 0x72, 0x6f, 0x6c, 0x65, 0x3d, 0x70, 0x72, 0x65}
		} else if ii == 4 {
			values = []byte{65, 71, 74, 66, 76, 65, 69, 77, 74, 79, 68, 75, 73, 72, 77, 68, 78, 65, 79, 79, 78, 66, 77, 71, 64, 70, 74, 77, 64, 67, 71, 64}
		} else if ii == 1 {
			values = make([]byte, 32)

			for i := range values {
				values[i] = byte(2) // all identical
			}
		} else if ii == 5 {
			values = make([]byte, 32)

			for i := range values {
				values[i] = byte(2 + (i & 1)) // 2 symbols
			}
		} else {
			values = make([]byte, 32)

			for i := range values {
				values[i] = byte(64 + 3*ii + rand.Intn(ii+1))
			}
		}

		fmt.Println("Original:")

		for i := range values {
			fmt.Printf("%d ", values[i])
		}

		fmt.Println("\nEncoded:")
		var bs io.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbgbs.ShowByte(true)
		dbgbs.Mark(true)
		hc, _ := entropy.NewANSRangeEncoder(dbgbs)

		if _, err := hc.Encode(values); err != nil {
			fmt.Printf("Error during encoding: %s", err)
			os.Exit(1)
		}

		hc.Dispose()
		dbgbs.Close()
		println()

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		//dbgbs2, _ := bitstream.NewDebugInputBitStream(ibs, os.Stdout)
		//dbgbs2.ShowByte(true)
		//dbgbs2.Mark(true)

		hd, _ := entropy.NewANSRangeDecoder(ibs)

		ok := true
		values2 := make([]byte, len(values))
		if _, err := hd.Decode(values2); err != nil {
			fmt.Printf("Error during decoding: %s", err)
			os.Exit(1)
		}

		hd.Dispose()
		fmt.Println("\nDecoded: ")

		for i := range values2 {
			fmt.Printf("%v ", values2[i])

			if values[i] != values2[i] {
				ok = false
			}
		}

		if ok == true {
			fmt.Println("\nIdentical")
		} else {
			fmt.Println("\n! *** Different *** !")
			os.Exit(1)
		}

		ibs.Close()
		bs.Close()
		println()
	}
}

func TestSpeed() {
	fmt.Printf("\n\nSpeed test\n")
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		fmt.Printf("Test %v\n", jj+1)
		delta1 := int64(0)
		delta2 := int64(0)
		iter := 4000
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs io.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < len(values1); i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F

				if i0+length >= len(values1) {
					length = 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = byte(uint(i0) & 0xFF)
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			rc, _ := entropy.NewANSRangeEncoder(obs)

			// Encode
			before1 := time.Now()

			if _, err := rc.Encode(values1); err != nil {
				fmt.Printf("An error occured during encoding: %v\n", err)
				os.Exit(1)
			}

			rc.Dispose()
			after1 := time.Now()
			delta1 += after1.Sub(before1).Nanoseconds()

			if _, err := obs.Close(); err != nil {
				fmt.Printf("Error during close: %v\n", err)
				os.Exit(1)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			rd, _ := entropy.NewANSRangeDecoder(ibs)

			// Decode
			before2 := time.Now()

			if _, err := rd.Decode(values2); err != nil {
				fmt.Printf("An error occured during decoding: %v\n", err)
				os.Exit(1)
			}

			rd.Dispose()
			after2 := time.Now()
			delta2 += after2.Sub(before2).Nanoseconds()

			if _, err := ibs.Close(); err != nil {
				fmt.Printf("Error during close: %v\n", err)
				os.Exit(1)
			}

		}

		bs.Close()
		fmt.Printf("Encode [ms]      : %d\n", delta1/1000000)
		fmt.Printf("Throughput [KB/s]: %d\n", (int64(iter*size))*1000000/delta1*1000/1024)
		fmt.Printf("Decode [ms]      : %d\n", delta2/1000000)
		fmt.Printf("Throughput [KB/s]: %d\n", (int64(iter*size))*1000000/delta2*1000/1024)
	}
}
