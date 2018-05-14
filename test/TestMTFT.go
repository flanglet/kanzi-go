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
	"fmt"
	"github.com/flanglet/kanzi-go/transform"
	"math/rand"
	"os"
	"time"
)

func main() {
	fmt.Printf("\nMTFT Correctness test")

	for ii := 0; ii < 20; ii++ {
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		var input []byte
		if ii == 0 {
			input = []byte{5, 2, 4, 7, 0, 0, 7, 1, 7}
		} else {
			input = make([]byte, 32)

			for i := 0; i < len(input); i++ {
				input[i] = byte(65 + rnd.Intn(5*ii))
			}
		}

		size := len(input)
		mtft, _ := transform.NewMTFT()
		transform := make([]byte, size+20)
		reverse := make([]byte, size)

		fmt.Printf("\nTest %d", (ii + 1))
		fmt.Printf("\nInput     : ")

		for i := 0; i < len(input); i++ {
			fmt.Printf("%d ", input[i])
		}

		start := (ii & 1) * ii
		mtft.Forward(input, transform[start:start+size])
		fmt.Printf("\nTransform : ")

		for i := start; i < start+len(input); i++ {
			fmt.Printf("%d ", transform[i])
		}

		mtft.Inverse(transform[start:start+size], reverse)
		fmt.Printf("\nReverse   : ")

		for i := 0; i < len(input); i++ {
			fmt.Printf("%d ", reverse[i])
		}

		fmt.Printf("\n")
		ok := true

		for i := 0; i < len(input); i++ {
			if reverse[i] != input[i] {
				ok = false
				break
			}
		}

		if ok == true {
			fmt.Printf("Identical\n")
		} else {
			fmt.Printf("Different\n")
		}
	}

	// Speed Test
	iter := 20000
	size := 10000
	fmt.Printf("\n\nMTFT Speed test\n")
	fmt.Printf("Iterations: %v\n", iter)

	for jj := 0; jj < 4; jj++ {
		input := make([]byte, size)
		output := make([]byte, size)
		reverse := make([]byte, size)
		mtft, _ := transform.NewMTFT()
		delta1 := int64(0)
		delta2 := int64(0)

		if jj == 0 {
			println("\n\nPurely random input")
		}

		if jj == 2 {
			println("\n\nSemi random input")
		}

		for ii := 0; ii < iter; ii++ {
			for i := 0; i < len(input); i++ {
				n := 128

				if jj < 2 {
					// Pure random
					input[i] = byte(rand.Intn(256))
				} else {
					// Semi random (a bit more realistic input)
					rng := 5

					if i&7 == 0 {
						rng = 128
					}

					p := (rand.Intn(rng) - rng/2 + n) & 0xFF
					input[i] = byte(p)
					n = p
				}
			}

			before := time.Now()
			mtft.Forward(input, output)
			after := time.Now()
			delta1 += after.Sub(before).Nanoseconds()
			before = time.Now()
			mtft.Inverse(output, reverse)
			after = time.Now()
			delta2 += after.Sub(before).Nanoseconds()

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
		}

		fmt.Printf("MTFT Forward transform [ms]: %v\n", delta1/1000000)
		fmt.Printf("Throughput [KB/s]          : %d\n", (int64(iter*size))*1000000/delta1*1000/1024)
		fmt.Printf("MTFT Reverse transform [ms]: %v\n", delta2/1000000)
		fmt.Printf("Throughput [KB/s]          : %d\n", (int64(iter*size))*1000000/delta2*1000/1024)
		println()
	}
}
