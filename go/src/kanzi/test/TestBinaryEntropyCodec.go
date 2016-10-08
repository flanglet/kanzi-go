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
	"kanzi/entropy"
	"kanzi/util"
	"math/rand"
	"os"
	"strings"
	"time"
)

func main() {
	var name = flag.String("type", "ALL", "Type of predictor (all, CM, FPAQ or PAQ)")

	// Parse
	flag.Parse()
	name_ := strings.ToUpper(*name)

	if name_ == "ALL" {
		fmt.Printf("\n\nTestFPAQEntropyCoder")
		TestCorrectness("FPAQ")
		TestSpeed("FPAQ")
		fmt.Printf("\n\nTestCMEntropyCoder")
		TestCorrectness("CM")
		TestSpeed("CM")
		fmt.Printf("\n\nTestPAQEntropyCoder")
		TestCorrectness("PAQ")
		TestSpeed("PAQ")
		fmt.Printf("\n\nTestTPAQEntropyCoder")
		TestCorrectness("TPAQ")
		TestSpeed("TPAQ")
	} else if name_ != "" {
		fmt.Printf("\n\nTest%vEntropyCoder", name_)
		TestCorrectness(name_)
		TestSpeed(name_)
	} else {
		fmt.Println("Usage: TestBinaryEntropyCoder -type=???")
	}

}

func getPredictor(name string) entropy.Predictor {
	switch name {
	case "PAQ":
		res, _ := entropy.NewPAQPredictor()
		return res

	case "FPAQ":
		res, _ := entropy.NewFPAQPredictor()
		return res

	case "TPAQ":
		res, _ := entropy.NewTPAQPredictor()
		return res

	case "CM":
		res, _ := entropy.NewCMPredictor()
		return res

	default:
		panic(fmt.Errorf("Unsupported type: '%s'", name))
	}
}

func TestCorrectness(name string) {
	fmt.Printf("\n\nCorrectness test for %v\n", name)

	// Test behavior
	for ii := 1; ii < 20; ii++ {
		fmt.Printf("\n\nTest %v", ii)
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

		fmt.Printf("\nOriginal: \n")

		for i := range values {
			fmt.Printf("%d ", values[i])
		}

		println()
		fmt.Printf("\nEncoded: \n")
		var bs util.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbgbs.ShowByte(true)
		//dbgbs.Mark(true)
		fc, _ := entropy.NewBinaryEntropyEncoder(dbgbs, getPredictor(name))

		if _, err := fc.Encode(values); err != nil {
			fmt.Printf("Error during encoding: %s", err)
			os.Exit(1)
		}

		fc.Dispose()
		dbgbs.Close()
		println()
		fmt.Printf("\nDecoded: \n")

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		fd, _ := entropy.NewBinaryEntropyDecoder(ibs, getPredictor(name))

		ok := true
		values2 := make([]byte, len(values))
		if _, err := fd.Decode(values2); err != nil {
			fmt.Printf("Error during decoding: %s", err)
			os.Exit(1)
		}

		fd.Dispose()

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

		ibs.Close()
		bs.Close()
		println()
	}
}

func TestSpeed(name string) {
	fmt.Printf("\n\nSpeed test for %v\n", name)
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		fmt.Printf("\nTest %v\n", jj+1)
		delta1 := int64(0)
		delta2 := int64(0)
		iter := 100
		size := 500000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs util.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(rand.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			fc, _ := entropy.NewBinaryEntropyEncoder(obs, getPredictor(name))

			// Encode
			before1 := time.Now()

			if _, err := fc.Encode(values1); err != nil {
				fmt.Printf("An error occured during encoding: %v\n", err)
				os.Exit(1)
			}

			fc.Dispose()

			after1 := time.Now()
			delta1 += after1.Sub(before1).Nanoseconds()

			if _, err := obs.Close(); err != nil {
				fmt.Printf("Error during close: %v\n", err)
				os.Exit(1)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			fd, _ := entropy.NewBinaryEntropyDecoder(ibs, getPredictor(name))

			// Decode
			before2 := time.Now()

			if _, err := fd.Decode(values2); err != nil {
				fmt.Printf("An error occured during decoding: %v\n", err)
				os.Exit(1)
			}

			fd.Dispose()

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

		fmt.Printf("Encode [ms]      : %d\n", delta1/1000000)
		fmt.Printf("Throughput [KB/s]: %d\n", (int64(iter*size))*1000000/delta1*1000/1024)
		fmt.Printf("Decode [ms]      : %d\n", delta2/1000000)
		fmt.Printf("Throughput [KB/s]: %d\n", (int64(iter*size))*1000000/delta2*1000/1024)
	}
}
