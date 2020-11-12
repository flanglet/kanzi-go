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
	"math/rand"
	"testing"
	"time"

	kanzi "github.com/flanglet/kanzi-go"
	"github.com/flanglet/kanzi-go/function"
)

func getByteFunction(name string) (kanzi.ByteFunction, error) {
	switch name {
	case "LZ":
		res, err := function.NewLZCodec()
		return res, err

	case "ZRLT":
		res, err := function.NewZRLT()
		return res, err

	case "RLT":
		res, err := function.NewRLT()
		return res, err

	case "SRT":
		res, err := function.NewSRT()
		return res, err

	case "ROLZ":
		res, err := function.NewROLZCodecWithFlag(false)
		return res, err

	case "ROLZX":
		res, err := function.NewROLZCodecWithFlag(true)
		return res, err

	default:
		panic(fmt.Errorf("No such byte function: '%s'", name))
	}
}

func TestLZ(b *testing.T) {
	if err := testFunctionCorrectness("LZ"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestROLZ(b *testing.T) {
	if err := testFunctionCorrectness("ROLZ"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestZRLT(b *testing.T) {
	if err := testFunctionCorrectness("ZRLT"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestRLT(b *testing.T) {
	if err := testFunctionCorrectness("RLT"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestSRT(b *testing.T) {
	if err := testFunctionCorrectness("SRT"); err != nil {
		b.Errorf(err.Error())
	}
}

// func TestROLZX(b *testing.T) {
// 	if err := testFunctionCorrectness("ROLZX"); err != nil {
// 		b.Errorf(err.Error())
// 	}
// }

func testFunctionCorrectness(name string) error {
	rng := 256

	if name == "ZRLT" {
		rng = 5
	}

	for ii := 0; ii < 20; ii++ {
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		fmt.Printf("\nTest %v\n\n", ii)
		var arr []int

		if ii == 0 {
			arr = []int{0, 1, 2, 2, 2, 2, 7, 9, 9, 16, 16, 16, 1, 3,
				3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
		} else if ii == 1 {
			arr = make([]int, 80000)

			for i := range arr {
				arr[i] = 8
			}

			arr[0] = 1
		} else if ii == 2 {
			arr = []int{0, 0, 1, 1, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3}
		} else if ii < 6 {
			// Lots of zeros
			arr = make([]int, 1<<uint(ii+6))

			if rng > 100 {
				rng = 100
			}

			for i := range arr {
				val := rand.Intn(rng)

				if val >= 33 {
					val = 0
				}

				arr[i] = val
			}
		} else if ii == 6 {
			// Totally random
			arr = make([]int, 512)

			// Leave zeros at the beginning for ZRLT to succeed
			for i := 20; i < len(arr); i++ {
				arr[i] = rand.Intn(rng)
			}
		} else {
			arr = make([]int, 1024)
			// Leave zeros at the beginning for ZRLT to succeed
			idx := 20

			for idx < len(arr) {
				length := rnd.Intn(40)

				if length%3 == 0 {
					length = 1
				}

				val := rand.Intn(rng)
				end := idx + length

				if end >= len(arr) {
					end = len(arr) - 1
				}

				for j := idx; j < end; j++ {
					arr[j] = val
				}

				idx += length

			}
		}

		size := len(arr)
		f, err := getByteFunction(name)

		if err != nil {
			fmt.Printf("\nCannot create transform '%v': %v\n", name, err)
			return err
		}

		input := make([]byte, size)
		output := make([]byte, f.MaxEncodedLen(size))
		reverse := make([]byte, size)

		for i := range output {
			output[i] = 0xAA
		}

		for i := range arr {
			input[i] = byte(arr[i])
		}

		f, err = getByteFunction(name)

		if err != nil {
			fmt.Printf("\nCannot create transform '%v': %v\n", name, err)
			return err
		}

		fmt.Printf("\nOriginal: \n")

		if ii == 1 {
			fmt.Printf("1 8 (%v times)", len(input)-2)
		} else {
			for i := range arr {
				fmt.Printf("%v ", input[i])
			}
		}

		srcIdx, dstIdx, err := f.Forward(input, output)

		if err != nil {
			// Function may fail when compression ratio > 1.0
			if srcIdx != uint(size) || srcIdx < dstIdx {
				fmt.Printf("\nNo compression (ratio > 1.0), skip reverse")
				continue
			}

			fmt.Printf("\nEncoding error : %v\n", err)
			return err
		}

		if srcIdx != uint(size) || srcIdx < dstIdx {
			fmt.Printf("\nNo compression (ratio > 1.0), skip reverse")
			continue
		}

		fmt.Printf("\nCoded: \n")

		for i := uint(0); i < dstIdx; i++ {
			fmt.Printf("%v ", output[i])
		}

		fmt.Printf(" (Compression ratio: %v%%)\n", int(dstIdx)*100/size)

		f, err = getByteFunction(name)

		if err != nil {
			fmt.Printf("\nCannot create transform '%v': %v\n", name, err)
			return err
		}

		_, _, err = f.Inverse(output[0:dstIdx], reverse)

		if err != nil {
			fmt.Printf("Decoding error : %v\n", err)
			return err
		}

		fmt.Printf("Decoded: \n")
		idx := -1

		// Check
		for i := range reverse {
			if input[i] != reverse[i] {
				idx = i
				break
			}
		}

		if idx == -1 {
			if ii == 1 {
				fmt.Printf("1 8 (%v times)", len(input)-2)
			} else {
				for i := range reverse {
					fmt.Printf("%v ", reverse[i])
				}
			}

			fmt.Printf("\n")
		} else {
			fmt.Printf("Different (index %v - %v)\n", input[idx], reverse[idx])
			return err
		}

		fmt.Printf("Identical\n")
	}

	return error(nil)
}
