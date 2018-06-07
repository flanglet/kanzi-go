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
	kanzi "github.com/flanglet/kanzi-go"
	"github.com/flanglet/kanzi-go/function"
	"math/rand"
	"os"
	"strings"
	"time"
)

func main() {
	var name = flag.String("type", "ALL", "Type of function (all, LZ4, ROLZ, SNAPPY, RLT or ZRLT)")

	// Parse
	flag.Parse()
	name_ := strings.ToUpper(*name)
	fmt.Printf("Transform %v", name)

	if name_ == "ALL" {
		fmt.Printf("\n\nTestLZ4")

		if err := TestCorrectness("LZ4"); err != nil {
			os.Exit(1)
		}

		TestSpeed("LZ4")
		fmt.Printf("\n\nTestROLZ")

		if err := TestCorrectness("ROLZ"); err != nil {
			os.Exit(1)
		}

		TestSpeed("ROLZ")
		fmt.Printf("\n\nTestSnappy")

		if err := TestCorrectness("SNAPPY"); err != nil {
			os.Exit(1)
		}

		TestSpeed("SNAPPY")
		fmt.Printf("\n\nTestZRLT")

		if err := TestCorrectness("ZRLT"); err != nil {
			os.Exit(1)
		}

		TestSpeed("ZRLT")
		fmt.Printf("\n\nTestRLT")

		if err := TestCorrectness("RLT"); err != nil {
			os.Exit(1)
		}

		TestSpeed("RLT")
	} else if name_ != "" {
		fmt.Printf("Test%v", name_)

		if err := TestCorrectness(name_); err != nil {
			os.Exit(1)
		}

		TestSpeed(name_)
	}
}

func getByteFunction(name string) (kanzi.ByteFunction, error) {
	switch name {
	case "LZ4":
		res, err := function.NewLZ4Codec()
		return res, err

	case "ROLZ":
		res, err := function.NewROLZCodec(function.ROLZ_LOG_POS_CHECKS)
		return res, err

	case "SNAPPY":
		res, err := function.NewSnappyCodec()
		return res, err

	case "ZRLT":
		res, err := function.NewZRLT()
		return res, err

	case "RLT":
		res, err := function.NewRLT(3)
		return res, err

	default:
		panic(fmt.Errorf("No such byte function: '%s'", name))
	}
}

func TestCorrectness(name string) error {
	fmt.Printf("Correctness test for %v\n", name)
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
			arr = make([]int, 66000)

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

		for i := range arr {
			fmt.Printf("%v ", input[i])
		}

		srcIdx, dstIdx, err := f.Forward(input, output)

		if err != nil {
			// ZRLT may fail if the input data has too few 0s
			if srcIdx != uint(size) {
				fmt.Printf("\nNo compression (ratio > 1.0), skip reverse")
				continue
			}

			fmt.Printf("\nEncoding error : %v\n", err)
			return err
		}

		if srcIdx != uint(size) {
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

		for i := range reverse {
			fmt.Printf("%v ", reverse[i])
		}

		fmt.Printf("\n")

		// Check
		for i := range reverse {
			if input[i] != reverse[i] {
				fmt.Printf("Different (index %v - %v)\n", input[i], reverse[i])
				return err
			}
		}

		fmt.Printf("Identical\n")
	}

	return error(nil)
}

func TestSpeed(name string) {
	iter := 50000

	if name == "ROLZ" {
		iter = 2000
	}

	size := 50000
	fmt.Printf("\n\nSpeed test for %v\n", name)
	fmt.Printf("Iterations: %v\n", iter)
	rng := 256

	if name == "ZRLT" {
		rng = 5
	}

	for jj := 0; jj < 3; jj++ {
		bf, err2 := getByteFunction(name)

		if err2 != nil {
			fmt.Printf("\nCannot create transform '%v': %v\n", name, err2)
			return
		}

		input := make([]byte, size)
		output := make([]byte, bf.MaxEncodedLen(size))
		reverse := make([]byte, size)
		rand.Seed(int64(jj))

		// Generate random data with runs
		// Leave zeros at the beginning for ZRLT to succeed
		n := iter / 20
		delta1 := int64(0)
		delta2 := int64(0)

		for n < len(input) {
			val := byte(rand.Intn(rng))
			input[n] = val
			n++
			run := rand.Intn(256)
			run -= 220

			for run > 0 && n < len(input) {
				input[n] = val
				n++
				run--
			}
		}

		var err error
		var dstIdx uint

		for ii := 0; ii < iter; ii++ {
			f, err2 := getByteFunction(name)

			if err2 != nil {
				fmt.Printf("\nCannot create transform '%v': %v\n", name, err2)
				return
			}

			before := time.Now()

			_, dstIdx, err = f.Forward(input, output)

			if err != nil {
				fmt.Printf("Encoding error : %v\n", err)
				continue
			}

			after := time.Now()
			delta1 += after.Sub(before).Nanoseconds()
		}

		for ii := 0; ii < iter; ii++ {
			f, err2 := getByteFunction(name)

			if err2 != nil {
				fmt.Printf("\nCannot create transform '%v': %v\n", name, err2)
				return
			}

			before := time.Now()

			if _, _, err = f.Inverse(output[0:dstIdx], reverse); err != nil {
				fmt.Printf("Decoding error : %v\n", err)
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

		fmt.Printf("\n%v encoding [ms]: %v", name, delta1/1000000)
		fmt.Printf("\nThroughput [MB/s]: %d", (int64(iter*size))*1000000/delta1*1000/(1024*1024))
		fmt.Printf("\n%v decoding [ms]: %v", name, delta2/1000000)
		fmt.Printf("\nThroughput [MB/s]: %d", (int64(iter*size))*1000000/delta2*1000/(1024*1024))
	}

	println()
}
