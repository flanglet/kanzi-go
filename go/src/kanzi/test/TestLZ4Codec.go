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
	"kanzi/function"
	"math/rand"
	"os"
	"time"
)

func main() {
	fmt.Printf("TestLZ4Codec\n\n")
	TestCorrectness()
	TestSpeed()
}

func TestCorrectness() {
	fmt.Printf("Correctness test\n")

	for ii := 0; ii < 20; ii++ {
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		fmt.Printf("\nTest %v\n\n", ii)
		var arr []int

		if ii == 0 {
			arr = []int{0, 0, 1, 1, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3}
		} else if ii == 1 {
			arr = make([]int, 500)

			for i := range arr {
				arr[i] = 8
			}

			arr[0] = 1
		} else if ii == 2 {
			arr = []int{0, 1, 2, 2, 2, 2, 7, 9, 9, 16, 16, 16, 1, 3,
				3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
		} else {
			arr = make([]int, 1024)
			idx := 0

			for idx < len(arr) {
				length := rnd.Intn(270)

				if length%3 == 0 {
					length = 1
				}

				val := rand.Intn(256) - 128
				end := idx + length

				if end >= len(arr) {
					end = len(arr) - 1
				}

				for j := idx; j < end; j++ {
					arr[j] = val
				}

				idx += length
				fmt.Printf("%v (%v) ", val, length)

			}
		}

		size := len(arr)
		input := make([]byte, size)
		output := make([]byte, 32+size*4/3)
		reverse := make([]byte, size)

		for i := range output {
			output[i] = 0xAA
		}

		for i := range arr {
			input[i] = byte(arr[i])
		}

		lz4, _ := function.NewLZ4Codec()
		fmt.Printf("\nOriginal: \n")

		for i := range arr {
			fmt.Printf("%v ", input[i])
		}

		srcIdx, dstIdx, err := lz4.Forward(input, output)

		if err != nil {
			fmt.Printf("\n===Encoding error===\n%v\n", err)
			os.Exit(1)
		}

		if srcIdx != uint(len(input)) {
			fmt.Printf("\nNo compression (ratio > 1.0), skip reverse")
			continue
		}

		fmt.Printf("\nCoded: \n")

		for i := uint(0); i < dstIdx; i++ {
			fmt.Printf("%v ", output[i])
		}

		// Required to reset internal attributes
		lz4, _ = function.NewLZ4Codec()

		_, _, err = lz4.Inverse(output[0:dstIdx], reverse)

		if err != nil {
			fmt.Printf("\n===Decoding error===\n%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nDecoded: \n")

		for i := range reverse {
			fmt.Printf("%v ", reverse[i])
		}

		fmt.Printf("\n")

		// Check
		for i := range reverse {
			if input[i] != reverse[i] {
				fmt.Printf("Different (index %v - %v)\n", input[i], reverse[i])
				os.Exit(1)
			}
		}

		fmt.Printf("Identical\n")
	}
}

func TestSpeed() {
	iter := 50000
	size := 50000
	fmt.Printf("\n\nSpeed test\n")
	fmt.Printf("Iterations: %v\n", iter)

	for jj := 0; jj < 3; jj++ {
		input := make([]byte, size)
		output := make([]byte, 32+size*4/3)
		reverse := make([]byte, size)

		// Generate random data with runs
		n := 0
		delta1 := int64(0)
		delta2 := int64(0)

		for n < len(input) {
			val := byte(rand.Intn(3))
			input[n] = val
			n++
			run := rand.Intn(255)
			run -= 200
			run--

			for run > 0 && n < len(input) {
				input[n] = val
				n++
				run--
			}
		}

		var dstIdx uint
		var err error

		for ii := 0; ii < iter; ii++ {
			lz4, _ := function.NewLZ4Codec()
			before := time.Now()

			_, dstIdx, err = lz4.Forward(input, output)

			if err != nil {
				fmt.Printf("Encoding error%v\n", err)
				os.Exit(1)
			}

			after := time.Now()
			delta1 += after.Sub(before).Nanoseconds()
		}

		for ii := 0; ii < iter; ii++ {
			lz4, _ := function.NewLZ4Codec()
			before := time.Now()

			if _, _, err = lz4.Inverse(output[0:dstIdx], reverse); err != nil {
				fmt.Printf("Decoding error%v\n", err)
				os.Exit(1)
			}

			after := time.Now()
			delta2 += after.Sub(before).Nanoseconds()
		}

		idx := -1

		// Sanity check
		for i := range input {
			if input[i] != reverse[i] {
				idx = i
				break
			}
		}

		if idx >= 0 {
			fmt.Printf("Failure at index %v (%v <-> %v)\n", idx, input[idx], reverse[idx])
			os.Exit(1)
		}

		fmt.Printf("\nLZ4 encoding [ms]: %v", delta1/1000000)
		fmt.Printf("\nThroughput [MB/s]: %d", (int64(iter*size))*1000000/delta1*1000/(1024*1024))
		fmt.Printf("\nLZ4 decoding [ms]: %v", delta2/1000000)
		fmt.Printf("\nThroughput [MB/s]: %d", (int64(iter*size))*1000000/delta2*1000/(1024*1024))
		println()
	}
}
