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
	"kanzi"
	"kanzi/transform"
	"math/rand"
	"time"
)

func main() {
	fmt.Printf("\nCorrectness test")
	block := []int{
		3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3,
		2, 3, 8, 4, 6, 2, 6, 4, 3, 3, 8, 3, 2, 7, 9, 5,
		0, 2, 8, 8, 4, 1, 9, 7, 1, 6, 9, 3, 9, 9, 3, 7,
		5, 1, 0, 5, 8, 2, 0, 9, 7, 4, 9, 4, 4, 5, 9, 2,
		3, 0, 7, 8, 1, 6, 4, 0, 6, 2, 8, 6, 2, 0, 8, 9,
		9, 8, 6, 2, 8, 0, 3, 4, 8, 2, 5, 3, 4, 2, 1, 1,
		7, 0, 6, 7, 9, 8, 2, 1, 4, 8, 0, 8, 6, 5, 1, 3,
		2, 8, 2, 3, 0, 6, 6, 4, 7, 0, 9, 3, 8, 4, 4, 6,
		0, 9, 5, 5, 0, 5, 8, 2, 2, 3, 1, 7, 2, 5, 3, 5,
		9, 4, 0, 8, 1, 2, 8, 4, 8, 1, 1, 1, 7, 4, 5, 0,
		2, 8, 4, 1, 0, 2, 7, 0, 1, 9, 3, 8, 5, 2, 1, 1,
		0, 5, 5, 5, 9, 6, 4, 4, 6, 2, 2, 9, 4, 8, 9, 5,
		4, 9, 3, 0, 3, 8, 1, 9, 6, 4, 4, 2, 8, 8, 1, 0,
		9, 7, 5, 6, 6, 5, 9, 3, 3, 4, 4, 6, 1, 2, 8, 4,
		7, 5, 6, 4, 8, 2, 3, 3, 7, 8, 6, 7, 8, 3, 1, 6,
		5, 2, 7, 1, 2, 0, 1, 9, 0, 9, 1, 4, 5, 6, 4, 8,
		5, 6, 6, 9, 2, 3, 4, 6, 0, 3, 4, 8, 6, 1, 0, 4,
		5, 4, 3, 2, 6, 6, 4, 8, 2, 1, 3, 3, 9, 3, 6, 0,
		7, 2, 6, 0, 2, 4, 9, 1, 4, 1, 2, 7, 3, 7, 2, 4,
		5, 8, 7, 0, 0, 6, 6, 0, 6, 3, 1, 5, 5, 8, 8, 1,
		7, 4, 8, 8, 1, 5, 2, 0, 9, 2, 0, 9, 6, 2, 8, 2,
		9, 2, 5, 4, 0, 9, 1, 7, 1, 5, 3, 6, 4, 3, 6, 7,
		8, 9, 2, 5, 9, 0, 3, 6, 0, 0, 1, 1, 3, 3, 0, 5,
		3, 0, 5, 4, 8, 8, 2, 0, 4, 6, 6, 5, 2, 1, 3, 8,
		4, 1, 4, 6, 9, 5, 1, 9, 4, 1, 5, 1, 1, 6, 0, 9,
		4, 3, 3, 0, 5, 7, 2, 7, 0, 3, 6, 5, 7, 5, 9, 5,
		9, 1, 9, 5, 3, 0, 9, 2, 1, 8, 6, 1, 1, 7, 3, 8,
		1, 9, 3, 2, 6, 1, 1, 7, 9, 3, 1, 0, 5, 1, 1, 8,
		5, 4, 8, 0, 7, 4, 4, 6, 2, 3, 7, 9, 9, 6, 2, 7,
		4, 9, 5, 6, 7, 3, 5, 1, 8, 8, 5, 7, 5, 2, 7, 2,
		4, 8, 9, 1, 2, 2, 7, 9, 3, 8, 1, 8, 3, 0, 1, 1,
		9, 4, 9, 1, 2, 9, 8, 3, 3, 6, 7, 3, 3, 6, 2, 4,
		4, 0, 6, 5, 6, 6, 4, 3, 0, 8, 6, 0, 2, 1, 3, 9,
		4, 9, 4, 6, 3, 9, 5, 2, 2, 4, 7, 3, 7, 1, 9, 0,
		7, 0, 2, 1, 7, 9, 8, 6, 0, 9, 4, 3, 7, 0, 2, 7,
		7, 0, 5, 3, 9, 2, 1, 7, 1, 7, 6, 2, 9, 3, 1, 7,
		6, 7, 5, 2, 3, 8, 4, 6, 7, 4, 8, 1, 8, 4, 6, 7,
		6, 6, 9, 4, 0, 5, 1, 3, 2, 0, 0, 0, 5, 6, 8, 1,
		2, 7, 1, 4, 5, 2, 6, 3, 5, 6, 0, 8, 2, 7, 7, 8,
		5, 7, 7, 1, 3, 4, 2, 7, 5, 7, 7, 8, 9, 6, 0, 9,
		1, 7, 3, 6, 3, 7, 1, 7, 8, 7, 2, 1, 4, 6, 8, 4,
		4, 0, 9, 0, 1, 2, 2, 4, 9, 5, 3, 4, 3, 0, 1, 4,
		6, 5, 4, 9, 5, 8, 5, 3, 7, 1, 0, 5, 0, 7, 9, 2,
		2, 7, 9, 6, 8, 9, 2, 5, 8, 9, 2, 3, 5, 4, 2, 0,
		1, 9, 9, 5, 6, 1, 1, 2, 1, 2, 9, 0, 2, 1, 9, 6,
		0, 8, 6, 4, 0, 3, 4, 4, 1, 8, 1, 5, 9, 8, 1, 3,
		6, 2, 9, 7, 7, 4, 7, 7, 1, 3, 0, 9, 9, 6, 0, 5,
		1, 8, 7, 0, 7, 2, 1, 1, 3, 4, 9, 9, 9, 9, 9, 9,
		8, 3, 7, 2, 9, 7, 8, 0, 4, 9, 9, 5, 1, 0, 5, 9,
		7, 3, 1, 7, 3, 2, 8, 1, 6, 0, 9, 6, 3, 1, 8, 5,
		9, 5, 0, 2, 4, 4, 5, 9, 4, 5, 5, 3, 4, 6, 9, 0,
		8, 3, 0, 2, 6, 4, 2, 5, 2, 2, 3, 0, 8, 2, 5, 3,
		3, 4, 4, 6, 8, 5, 0, 3, 5, 2, 6, 1, 9, 3, 1, 1,
		8, 8, 1, 7, 1, 0, 1, 0, 0, 0, 3, 1, 3, 7, 8, 3,
		8, 7, 5, 2, 8, 8, 6, 5, 8, 7, 5, 3, 3, 2, 0, 8,
		3, 8, 1, 4, 2, 0, 6, 1, 7, 1, 7, 7, 6, 6, 9, 1,
		4, 7, 3, 0, 3, 5, 9, 8, 2, 5, 3, 4, 9, 0, 4, 2,
		8, 7, 5, 5, 4, 6, 8, 7, 3, 1, 1, 5, 9, 5, 6, 2,
		8, 6, 3, 8, 8, 2, 3, 5, 3, 7, 8, 7, 5, 9, 3, 7,
		5, 1, 9, 5, 7, 7, 8, 1, 8, 5, 7, 7, 8, 0, 5, 3,
		2, 1, 7, 1, 2, 2, 6, 8, 0, 6, 6, 1, 3, 0, 0, 1,
		9, 2, 7, 8, 7, 6, 6, 1, 1, 1, 9, 5, 9, 0, 9, 2,
		1, 6, 4, 2, 0, 1, 9, 8, 9, 3, 8, 0, 9, 5, 2, 5,
		7, 2, 0, 1, 0, 6, 5, 4, 8, 5, 8, 6, 3, 2, 7, 8,
	}

	transforms := make([]kanzi.IntTransform, 5)
	transforms[0], _ = transform.NewDCT4()
	transforms[1], _ = transform.NewDCT8()
	transforms[2], _ = transform.NewDCT16()
	transforms[3], _ = transform.NewDCT32()
	transforms[4], _ = transform.NewDST4()

	for dimIdx := range transforms {
		dim := 4 << uint(dimIdx)
		name := "DCT"

		if dim >= 64 {
			dim = 4
			name = "DST"
		}

		fmt.Printf("\n%v%v correctness\n", name, dim)
		blockSize := dim * dim
		data1 := make([]int, blockSize+20)
		data2 := make([]int, blockSize+20)
		data3 := make([]int, blockSize+20)
		var data []int

		for nn := 0; nn < 20; nn++ {
			fmt.Printf("\nInput %v: ", nn)

			for i := 0; i < blockSize; i++ {
				if nn == 0 {
					data1[i] = block[i]
				} else {
					data1[i] = rand.Intn(nn * 10)
				}

				data2[i] = data1[i]
				fmt.Printf("%v ", data1[i])

			}

			copy(data3, data1)
			fmt.Printf("\nOutput: ")
			start := (nn & 1) * nn

			if nn <= 10 {
				data = data1
			} else {
				data = data2
			}

			transforms[dimIdx].Forward(data1, data[start:])

			for i := start; i < start+blockSize; i++ {
				fmt.Printf("%v ", data[i])
			}

			transforms[dimIdx].Inverse(data[start:], data2)
			fmt.Printf("\nResult: ")

			for i := 0; i < blockSize; i++ {
				fmt.Printf("%v ", data2[i])

				if data3[i] != data2[i] {
					fmt.Printf("! ")
				} else {
					fmt.Printf("= ")
				}
			}

			fmt.Printf("\n")
		}
	}

	fmt.Printf("\nDCT8 speed")
	delta1 := int64(0)
	delta2 := int64(0)
	iter := 500000

	for times := 0; times < 100; times++ {
		data := make([][]int, 1000)
		dct8, _ := transform.NewDCT8()

		for i := 0; i < 1000; i++ {
			data[i] = make([]int, 64)

			for j := 0; j < len(data[i]); j++ {
				data[i][j] = rand.Intn(10 + i + j*10)
			}
		}

		for i := 0; i < iter; i++ {
			before := time.Now()
			dct8.Forward(data[i%100], data[i%100])
			after := time.Now()
			delta1 += after.Sub(before).Nanoseconds()
			before = time.Now()
			dct8.Inverse(data[i%100], data[i%100])
			after = time.Now()
			delta2 += after.Sub(before).Nanoseconds()
		}
	}

	fmt.Printf("\nIterations: %v", iter*100)
	fmt.Printf("\nForward [ms]: %v", delta1/1000000)
	fmt.Printf("\nInverse [ms]: %v", delta2/1000000)
	println()
}
