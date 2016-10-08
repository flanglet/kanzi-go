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
	fmt.Printf("\nTestExpGolombCodec")
	TestCorrectness()
	TestSpeed()
}

func TestCorrectness() {
	fmt.Printf("\n\nCorrectness test")

	// Test behavior
	for ii := 1; ii < 20; ii++ {
		fmt.Printf("\nTest %v", ii)
		var values []byte
		rand.Seed(time.Now().UTC().UnixNano())

		if ii == 1 {
			values = []byte{17, 8, 30, 28, 6, 26, 2, 9, 31, 0, 15, 30, 11, 27, 17, 11, 24, 6, 10, 24, 15, 10, 16, 13, 6, 21, 1, 18, 0, 3, 23, 6}
		} else {
			values = make([]byte, 32)

			for i := range values {
				values[i] = byte(rand.Intn(32) - 16*(ii&1))
			}
		}

		fmt.Printf("\nOriginal: ")

		for i := range values {
			fmt.Printf("%d ", values[i])
		}

		signed := true

		if ii&1 == 0 {
			signed = false
		}

		println()
		fmt.Printf("\nEncoded: ")
		var bs io.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)

		// Alternate signed / unsigned coding
		fpc, _ := entropy.NewExpGolombEncoder(dbgbs, signed)

		if _, err := fpc.Encode(values); err != nil {
			fmt.Printf("Error during encoding: %s", err)
			os.Exit(1)
		}

		fpc.Dispose()
		dbgbs.Close()

		if _, err := dbgbs.Close(); err != nil {
			fmt.Printf("Error during close: %v\n", err)
			os.Exit(1)
		}

		println()
		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		dbgbs2, _ := bitstream.NewDebugInputBitStream(ibs, os.Stdout)
		dbgbs2.Mark(true)

		fpd, _ := entropy.NewExpGolombDecoder(dbgbs2, signed)

		ok := true
		values2 := make([]byte, len(values))

		if _, err := fpd.Decode(values2); err != nil {
			fmt.Printf("Error during decoding: %s", err)
			os.Exit(1)
		}

		println()
		fmt.Printf("\nDecoded: ")

		for i := range values2 {
			fmt.Printf("%v ", values2[i])

			if values[i] != values2[i] {
				ok = false
			}
		}

		if ok == true {
			fmt.Printf("\nIdentical")
		} else {
			fmt.Printf("\n! *** Different *** !")
			os.Exit(1)
		}

		fpd.Dispose()
		dbgbs2.Close()
		bs.Close()
	}
}

func TestSpeed() {
	fmt.Printf("\n\nSpeed test\n")
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		fmt.Printf("Test %v\n", jj+1)
		delta1 := int64(0)
		delta2 := int64(0)
		size := 50000
		iter := 4000
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
					values1[j] = byte(i0)
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			rc, _ := entropy.NewExpGolombEncoder(obs, true)

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
			rd, _ := entropy.NewExpGolombDecoder(ibs, true)

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

			// Sanity check
			for i := 0; i < size; i++ {
				if values1[i] != values2[i] {
					fmt.Printf("Error at index %v (%v<->%v)\n", i, values1[i], values2[i])
					break
				}
			}
		}

		bs.Close()
		prod := int64(iter) * int64(size)
		fmt.Printf("Encode [ms]      : %d\n", delta1/1000000)
		fmt.Printf("Throughput [KB/s]: %d\n", prod*1000000/delta1*1000/1024)
		fmt.Printf("Decode [ms]      : %d\n", delta2/1000000)
		fmt.Printf("Throughput [KB/s]: %d\n", prod*1000000/delta2*1000/1024)
	}
}
