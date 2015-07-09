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
	"kanzi"
	"kanzi/transform"
	"math/rand"
	"os"
	"time"
)

func main() {
	var input = flag.String("input", "", "input string")

	// Parse
	flag.Parse()

    if len(*input) > 0 {
    	buf1 := []byte(*input)
    	size := uint(len(buf1))
    	buf2 := make([]byte, size)
    	bwt, _ := transform.NewBWT(size)
    	bwt.Forward(buf1, buf2)
		fmt.Printf("BWT:  %s (%v)\n", buf2, bwt.PrimaryIndex())
    	bwts, _ := transform.NewBWTS(size)
    	bwts.Forward(buf1, buf2)
		fmt.Printf("BWTS: %s\n", buf2)
		os.Exit(0)
    }
        
	fmt.Printf("TestBWT and TestBWTS")
	TestCorrectness(true)
	TestCorrectness(false)
	TestSpeed(true)
	TestSpeed(false)
}

func TestCorrectness(isBWT bool) {
	if isBWT {
		fmt.Printf("\n\nBWT Correctness test")
	} else {
		fmt.Printf("\n\nBWTS Correctness test")
	}

	// Test behavior
	for ii := 1; ii <= 20; ii++ {
		fmt.Printf("\nTest %v\n", ii)
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

		size := uint(0)
		var buf1 []byte
		var buf2 []byte
		var buf3 []byte

		if ii == 1 {
			size = 0
			buf1 = []byte("mississippi")
		} else if ii == 2 {
			buf1 = []byte("3.14159265358979323846264338327950288419716939937510")
		} else {
			size = 128
			buf1 = make([]byte, size)

			for i := 0; i < len(buf1); i++ {
				buf1[i] = byte(65 + rnd.Intn(4*ii))
			}
		}

		buf2 = make([]byte, len(buf1))
		buf3 = make([]byte, len(buf1))
		var bwt kanzi.ByteTransform

		if isBWT {
			bwt, _ = transform.NewBWT(size)
		} else {
			bwt, _ = transform.NewBWTS(size)
		}

		str1 := string(buf1)
		fmt.Printf("Input:   %s\n", str1)
		_, _, err1 := bwt.Forward(buf1, buf2)

		if err1 != nil {
			fmt.Printf("Error: %v\n", err1)
			os.Exit(1)
		}

		str2 := string(buf2)
		fmt.Printf("Encoded: %s", str2)

		if isBWT {
			primaryIndex := bwt.(*transform.BWT).PrimaryIndex()
			fmt.Printf("  (Primary index=%v)\n", primaryIndex)
			bwt.(*transform.BWT).SetPrimaryIndex(primaryIndex)
		} else {
			println()
		}
		
		_, _, err2 := bwt.Inverse(buf2, buf3)

		if err2 != nil {
			fmt.Printf("Error: %v\n", err2)
			os.Exit(1)
		}

		str3 := string(buf3)
		fmt.Printf("Output:  %s\n", str3)

		if str1 == str3 {
			fmt.Printf("Identical\n")
		} else {
			fmt.Printf("Different\n")
			os.Exit(1)
		}
	}
}

func TestSpeed(isBWT bool) {
	if isBWT {
		fmt.Printf("\n\nBWT Speed test")
	} else {
		fmt.Printf("\n\nBWTS Speed test")
	}

	iter := 2000
	size := 256 * 1024
	buf1 := make([]byte, size)
	buf2 := make([]byte, size)
	buf3 := make([]byte, size)
	fmt.Printf("\nIterations: %v", iter)
	fmt.Printf("\nTransform size: %v\n", size)

	for jj := 0; jj < 3; jj++ {
		delta1 := int64(0)
		delta2 := int64(0)
		var bwt kanzi.ByteTransform

		if isBWT {
			bwt, _ = transform.NewBWT(0)
		} else {
			bwt, _ = transform.NewBWTS(0)
		}

		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < iter; i++ {
			for i := range buf1 {
				buf1[i] = byte(rnd.Intn(255) + 1)
			}

			before := time.Now()
			bwt.Forward(buf1, buf2)
			after := time.Now()
			delta1 += after.Sub(before).Nanoseconds()
			before = time.Now()
			bwt.Inverse(buf2, buf3)
			after = time.Now()
			delta2 += after.Sub(before).Nanoseconds()

			// Sanity check
			for i := range buf1 {
				if buf1[i] != buf3[i] {
					fmt.Printf("Error at index %v: %v<->%v\n", i, buf1[i], buf3[i])
					os.Exit(1)
				}
			}
		}

		println()
		prod := int64(iter) * int64(size)
		fmt.Printf("Forward transform [ms] : %v\n", delta1/1000000)
		fmt.Printf("Throughput [KB/s]      : %d\n", prod*1000000/delta1*1000/1024)
		fmt.Printf("Inverse transform [ms] : %v\n", delta2/1000000)
		fmt.Printf("Throughput [KB/s]      : %d\n", prod*1000000/delta2*1000/1024)
		println()
	}
}
