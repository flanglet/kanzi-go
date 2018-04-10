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
	kanzi "github.com/flanglet/kanzi"
	"github.com/flanglet/kanzi/bitstream"
	"github.com/flanglet/kanzi/entropy"
	"github.com/flanglet/kanzi/io"
	"math/rand"
	"os"
	"strings"
	"time"
)

func main() {
	var name = flag.String("type", "ALL", "Type of codec (all, Huffman, ANS, Range, FPAQ, CM, PAQ, TPAQ, ExpGolomg or RiceGolomb)")

	// Parse
	flag.Parse()
	name_ := strings.ToUpper(*name)

	if name_ == "ALL" {
		fmt.Printf("\n\nTestHuffmanCodec")
		TestCorrectness("HUFFMAN")
		TestSpeed("HUFFMAN")
		fmt.Printf("\n\nTestANS0Codec")
		TestCorrectness("ANS0")
		TestSpeed("ANS0")
		fmt.Printf("\n\nTestANS1Codec")
		TestCorrectness("ANS1")
		TestSpeed("ANS1")
		fmt.Printf("\n\nTestRangeCodec")
		TestCorrectness("RANGE")
		TestSpeed("RANGE")
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
		fmt.Printf("\n\nTestExpGolombCodec")
		TestCorrectness("EXPGOLOMB")
		TestSpeed("EXPGOLOMB")
		fmt.Printf("\n\nTestRiceGolombCodec")
		TestCorrectness("RICEGOLOMB")
		TestSpeed("RICEGOLOMB")
	} else if name_ != "" {
		fmt.Printf("\n\nTest%vCodec", name_)
		TestCorrectness(name_)
		TestSpeed(name_)
	}
}

func getPredictor(name string) kanzi.Predictor {
	switch name {
	case "PAQ":
		res, _ := entropy.NewPAQPredictor()
		return res

	case "FPAQ":
		res, _ := entropy.NewFPAQPredictor()
		return res

	case "TPAQ":
		res, _ := entropy.NewTPAQPredictor(nil)
		return res

	case "CM":
		res, _ := entropy.NewCMPredictor()
		return res

	default:
		panic(fmt.Errorf("Unsupported type: '%s'", name))
	}
}

func getEncoder(name string, obs kanzi.OutputBitStream) kanzi.EntropyEncoder {
	switch name {
	case "PAQ":
		res, _ := entropy.NewBinaryEntropyEncoder(obs, getPredictor(name))
		return res

	case "FPAQ":
		res, _ := entropy.NewBinaryEntropyEncoder(obs, getPredictor(name))
		return res

	case "TPAQ":
		res, _ := entropy.NewBinaryEntropyEncoder(obs, getPredictor(name))
		return res

	case "CM":
		res, _ := entropy.NewBinaryEntropyEncoder(obs, getPredictor(name))
		return res

	case "HUFFMAN":
		res, _ := entropy.NewHuffmanEncoder(obs)
		return res

	case "ANS0":
		res, _ := entropy.NewANSRangeEncoder(obs, 0)
		return res

	case "ANS1":
		res, _ := entropy.NewANSRangeEncoder(obs, 1)
		return res

	case "RANGE":
		res, _ := entropy.NewRangeEncoder(obs)
		return res

	case "EXPGOLOMB":
		res, _ := entropy.NewExpGolombEncoder(obs, true)
		return res

	case "RICEGOLOMB":
		res, _ := entropy.NewRiceGolombEncoder(obs, true, 4)
		return res

	default:
		panic(fmt.Errorf("No such entropy encoder: '%s'", name))
	}

	return nil
}

func getDecoder(name string, ibs kanzi.InputBitStream) kanzi.EntropyDecoder {
	switch name {
	case "PAQ":
		pred := getPredictor(name)

		if pred == nil {
			panic(fmt.Errorf("No such entropy decoder: '%s'", name))
		}

		res, _ := entropy.NewBinaryEntropyDecoder(ibs, pred)
		return res

	case "FPAQ":
		pred := getPredictor(name)

		if pred == nil {
			panic(fmt.Errorf("No such entropy decoder: '%s'", name))
		}

		res, _ := entropy.NewBinaryEntropyDecoder(ibs, pred)
		return res

	case "TPAQ":
		pred := getPredictor(name)

		if pred == nil {
			panic(fmt.Errorf("No such entropy decoder: '%s'", name))
		}

		res, _ := entropy.NewBinaryEntropyDecoder(ibs, pred)
		return res

	case "CM":
		pred := getPredictor(name)

		if pred == nil {
			panic(fmt.Errorf("No such entropy decoder: '%s'", name))
		}

		res, _ := entropy.NewBinaryEntropyDecoder(ibs, pred)
		return res

	case "HUFFMAN":
		res, _ := entropy.NewHuffmanDecoder(ibs)
		return res

	case "ANS0":
		res, _ := entropy.NewANSRangeDecoder(ibs, 0)
		return res

	case "ANS1":
		res, _ := entropy.NewANSRangeDecoder(ibs, 1)
		return res

	case "RANGE":
		res, _ := entropy.NewRangeDecoder(ibs)
		return res

	case "EXPGOLOMB":
		res, _ := entropy.NewExpGolombDecoder(ibs, true)
		return res

	case "RICEGOLOMB":
		res, _ := entropy.NewRiceGolombDecoder(ibs, true, 4)
		return res

	default:
		panic(fmt.Errorf("No such entropy decoder: '%s'", name))
	}

	return nil
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
		var bs io.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbgbs.ShowByte(true)
		//dbgbs.Mark(true)
		ec := getEncoder(name, dbgbs)

		if ec == nil {
			os.Exit(1)
		}

		if _, err := ec.Encode(values); err != nil {
			fmt.Printf("Error during encoding: %s", err)
			os.Exit(1)
		}

		ec.Dispose()
		dbgbs.Close()
		println()
		fmt.Printf("\nDecoded: \n")

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		ed := getDecoder(name, ibs)

		if ed == nil {
			os.Exit(1)
		}

		ok := true
		values2 := make([]byte, len(values))

		if _, err := ed.Decode(values2); err != nil {
			fmt.Printf("Error during decoding: %s", err)
			os.Exit(1)
		}

		ed.Dispose()

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
		var bs io.BufferStream

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
			ec := getEncoder(name, obs)

			if ec == nil {
				os.Exit(1)
			}

			// Encode
			before1 := time.Now()

			if _, err := ec.Encode(values1); err != nil {
				fmt.Printf("An error occured during encoding: %v\n", err)
				os.Exit(1)
			}

			ec.Dispose()

			after1 := time.Now()
			delta1 += after1.Sub(before1).Nanoseconds()

			if _, err := obs.Close(); err != nil {
				fmt.Printf("Error during close: %v\n", err)
				os.Exit(1)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			ed := getDecoder(name, ibs)

			if ed == nil {
				os.Exit(1)
			}
			// Decode
			before2 := time.Now()

			if _, err := ed.Decode(values2); err != nil {
				fmt.Printf("An error occured during decoding: %v\n", err)
				os.Exit(1)
			}

			ed.Dispose()

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
