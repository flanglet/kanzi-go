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
	fmt.Printf("TestRLT\n")
	TestCorrectness()
	TestSpeed()
}

func TestCorrectness() {
	fmt.Printf("Correctness test\n")

	for ii := 0; ii < 20; ii++ {
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		fmt.Printf("\nTest %v\n", ii)
		var arr []int

		if ii == 0 {
			arr = []int{0, 0, 1, 1, 2, 2, 3, 3}
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
		output := make([]byte, size*3/2)
		reverse := make([]byte, size)

		for i := range output {
			output[i] = 0xAA
		}

		for i := range arr {
			input[i] = byte(arr[i])
		}

		rlt, err := function.NewRLT(3)

		if err != nil {
			fmt.Printf("\nError: %v\n", err)
			break
		}

		fmt.Printf("\nOriginal: ")

		for i := range arr {
			fmt.Printf("%v ", input[i])
		}

		fmt.Printf("\nCoded: ")
		srcIdx, dstIdx, err2 := rlt.Forward(input, output)

		if err2 != nil {
			fmt.Printf("\nEncoding error: %v\n", err2)
			continue
		}

		if srcIdx != uint(len(input)) {
			fmt.Printf("\nNo compression (ratio > 1.0), skip reverse")
			continue
		}

		for i := uint(0); i < dstIdx; i++ {
			fmt.Printf("%v ", output[i])
		}

		// Required to reset internal attributes
		rlt, err2 = function.NewRLT(3)

		if err2 != nil {
			fmt.Printf("\nError: %v\n", err2)
			continue
		}

		_, _, err2 = rlt.Inverse(output[0:dstIdx], reverse)

		if err2 != nil {
			fmt.Printf("\nDecoding error: %v\n", err2)
			continue
		}

		fmt.Printf("\nDecoded: ")

		for i := range reverse {
			fmt.Printf("%v ", reverse[i])
		}

		ok := true

		for i := range input {
			if i%100 == 0 {
				fmt.Printf("\n")
			}

			fmt.Printf("%v ", reverse[i])

			if reverse[i] != input[i] {
				ok = false
			}
		}

		if ok == true {
			fmt.Printf("\nIdentical\n")
		} else {
			fmt.Printf("\nDifferent\n")
			os.Exit(1)
		}
	}
}

func TestSpeed() {
	iter := 50000
	size := 50000
	fmt.Printf("\n\nSpeed test\n")
	fmt.Printf("Iterations: %v\n", iter)

	for jj := 0; jj < 3; jj++ {
		input := make([]byte, size)
		output := make([]byte, len(input)*2)
		reverse := make([]byte, len(input))

		// Generate random data with runs
		n := 0
		var compressed uint
		var err error
		delta1 := int64(0)
		delta2 := int64(0)

		for n < len(input) {
			val := byte(rand.Intn(255))
			input[n] = val
			n++
			run := rand.Intn(128)
			run -= 100

			for run > 0 && n < len(input) {
				input[n] = val
				n++
				run--
			}
		}

		for ii := 0; ii < iter; ii++ {
			rlt, _ := function.NewRLT(3)
			before := time.Now()

			if _, compressed, err = rlt.Forward(input, output); err != nil {
				fmt.Printf("Encoding error%v\n", err)
				os.Exit(1)
			}

			after := time.Now()
			delta1 += after.Sub(before).Nanoseconds()
		}

		for ii := 0; ii < iter; ii++ {
			rlt, _ := function.NewRLT(3)
			before := time.Now()

			if _, _, err = rlt.Inverse(output[0:compressed], reverse); err != nil {
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

		prod := int64(iter) * int64(size)
		fmt.Printf("\nRLT encoding [ms]: %v", delta1/1000000)
		fmt.Printf("\nThroughput [MB/s]: %d", prod*1000000/delta1*1000/(1024*1024))
		fmt.Printf("\nRLT decoding [ms]: %v", delta2/1000000)
		fmt.Printf("\nThroughput [MB/s]: %d", prod*1000000/delta2*1000/(1024*1024))
		println()
	}
}
