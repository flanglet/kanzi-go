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
	"github.com/flanglet/kanzi/util/hash"
	"os"
	"time"
)

func main() {
	var filename = flag.String("input", "c:\\temp\\rt.jar", "name of the input file")
	flag.Parse()
	iter := 500
	fmt.Printf("Processing %v\n", *filename)
	fmt.Printf("%v iterations\n", iter)

	{
		fmt.Printf("XXHash32 speed test\n")
		file, err := os.Open(*filename)

		if err != nil {
			fmt.Printf("Cannot open %s", *filename)

			return
		}

		defer file.Close()
		buffer := make([]byte, 16384)
		hash, err := hash.NewXXHash32(0)

		if err != nil {
			fmt.Printf("Failed to create hash: %v\n", err)
			return
		}

		length, err := file.Read(buffer)
		size := int64(0)
		res := uint32(0)
		sum := int64(0)

		for length > 0 {
			if err != nil {
				fmt.Printf("Failed to read the next chunk of input file '%v': %v\n", *filename, err)
				return
			}

			before := time.Now()

			for i := 0; i < iter; i++ {
				hash.SetSeed(0)
				res += hash.Hash(buffer[0:length])
			}

			after := time.Now()
			sum += after.Sub(before).Nanoseconds()
			size += int64(length * iter)
			length, err = file.Read(buffer)
		}

		sum /= 1000000 // convert to ms
		fmt.Printf("XXHash32 res=%x\n", res)
		fmt.Printf("Elapsed [ms]: %v\n", sum)
		fmt.Printf("Throughput [MB/s]: %v\n", (size/1024*1000/1024)/sum)
	}

	fmt.Printf("\n")

	{
		fmt.Printf("XXHash64 speed test\n")
		file, err := os.Open(*filename)

		if err != nil {
			fmt.Printf("Cannot open %s", *filename)

			return
		}

		defer file.Close()
		buffer := make([]byte, 16384)
		hash, err := hash.NewXXHash64(0)

		if err != nil {
			fmt.Printf("Failed to create hash: %v\n", err)
			return
		}

		length, err := file.Read(buffer)
		size := int64(0)
		res := uint64(0)
		sum := int64(0)

		for length > 0 {
			if err != nil {
				fmt.Printf("Failed to read the next chunk of input file '%v': %v\n", *filename, err)
				return
			}

			before := time.Now()

			for i := 0; i < iter; i++ {
				hash.SetSeed(0)
				res += hash.Hash(buffer[0:length])
			}

			after := time.Now()
			sum += after.Sub(before).Nanoseconds()
			size += int64(length * iter)
			length, err = file.Read(buffer)
		}

		sum /= 1000000 // convert to ms
		fmt.Printf("XXHash64 res=%x\n", res)
		fmt.Printf("Elapsed [ms]: %v\n", sum)
		fmt.Printf("Throughput [MB/s]: %v\n", (size/1024*1000/1024)/sum)
	}

	fmt.Printf("\n")

	{
		fmt.Printf("MurmurHash3 speed test\n")
		file, err := os.Open(*filename)

		if err != nil {
			fmt.Printf("Cannot open %s", *filename)

			return
		}

		defer file.Close()
		buffer := make([]byte, 16384)
		hash, err := hash.NewMurMurHash3(uint32(0))

		if err != nil {
			fmt.Printf("Failed to create hash: %v\n", err)
			return
		}

		length, err := file.Read(buffer)
		size := int64(0)
		res := uint32(0)
		sum := int64(0)

		for length > 0 {
			if err != nil {
				fmt.Printf("Failed to read the next chunk of input file '%v': %v\n", *filename, err)
				return
			}

			before := time.Now()

			for i := 0; i < iter; i++ {
				hash.SetSeed(0)
				res += hash.Hash(buffer[0:length])
			}

			after := time.Now()
			sum += after.Sub(before).Nanoseconds()
			size += int64(length * iter)
			length, err = file.Read(buffer)
		}

		sum /= 1000000 // convert to ms
		fmt.Printf("MurmurHash3 res=%x\n", res)
		fmt.Printf("Elapsed [ms]: %v\n", sum)
		fmt.Printf("Throughput [MB/s]: %v\n", (size/1024*1000/1024)/sum)
	}

	fmt.Printf("\n")

	{
		fmt.Printf("SipHash_2_4 speed test\n")
		file, err := os.Open(*filename)

		if err != nil {
			fmt.Printf("Cannot open %s", *filename)

			return
		}

		defer file.Close()
		buffer := make([]byte, 16384)
		hash, err := hash.NewSipHash()

		if err != nil {
			fmt.Printf("Failed to create hash: %v\n", err)
			return
		}

		length, err := file.Read(buffer)
		size := int64(0)
		res := uint64(0)
		sum := int64(0)

		for length > 0 {
			if err != nil {
				fmt.Printf("Failed to read the next chunk of input file '%v': %v\n", *filename, err)
				return
			}

			before := time.Now()

			for i := 0; i < iter; i++ {
				hash.SetSeedFromLongs(0, 0)
				res += hash.Hash(buffer[0:length])
			}

			after := time.Now()
			sum += after.Sub(before).Nanoseconds()
			size += int64(length * iter)
			length, err = file.Read(buffer)
		}

		sum /= 1000000 // convert to ms
		fmt.Printf("SipHash_2_4 res=%x\n", res)
		fmt.Printf("Elapsed [ms]: %v\n", sum)
		fmt.Printf("Throughput [MB/s]: %v\n", (size/1024*1000/1024)/sum)
	}
}
