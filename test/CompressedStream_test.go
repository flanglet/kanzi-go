/*
Copyright 2011-2021 Frederic Langlet
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

	kio "github.com/flanglet/kanzi-go/io"
	"github.com/flanglet/kanzi-go/util"
)

func TestCorrectness(b *testing.T) {
	fmt.Println("Correctness Test")
	values := make([]byte, 65536<<6)
	incompressible := make([]byte, 65536<<6)
	sum := 0

	for test := 1; test < 40; test++ {
		length := 65536 << (test % 7)
		fmt.Printf("\nIteration %v (size %v)\n", test, length)
		rand.Seed(time.Now().UTC().UnixNano())

		for i := range values {
			values[i] = byte(rand.Intn(4*test + 1))
			incompressible[i] = byte(rand.Intn(256))
		}

		if res := compress1(values[0:length]); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Println("Failure")
			sum += res
		}

		if res := compress2(values[0:length]); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Println("Failure")
			sum += res
		}

		if res := compress3(values[0:length]); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Println("Failure")
			sum += res
		}

		if test == 1 {
			if res := compress4(values[0:length]); res == 0 {
				fmt.Println("Success")
			} else {
				fmt.Println("Failure")
				sum += res
			}

			if res := compress5(values[0:length]); res == 0 {
				fmt.Println("Success")
			} else {
				fmt.Println("Failure")
				sum += res
			}
		}
	}

	fmt.Println()

	if sum != 0 {
		b.Error()
	}
}

func compress1(block []byte) int {
	fmt.Println("Test - Regular")
	buf := make([]byte, len(block))
	copy(buf, block)
	var bs util.BufferStream
	blockSize := uint((len(block) / (rand.Intn(3) + 1)) & -16)

	os, err := kio.NewCompressedOutputStream(&bs, "HUFFMAN", "RLT", blockSize, 1, false)

	if err != nil {
		return 1
	}

	written, err := os.Write(block)

	if err == nil {
		if err = os.Close(); err != nil {
			return 2
		}
	}

	for i := range block {
		block[i] = 0
	}

	is, err := kio.NewCompressedInputStream(&bs, 1)

	if err != nil {
		return 3
	}

	read, err := is.Read(block)

	if err == nil {
		if err = is.Close(); err != nil {
			return 4
		}
	}

	for i := range block {
		if buf[i] != block[i] {
			return 5
		}
	}

	return read ^ written
}

func compress2(block []byte) int {
	jobs := uint(rand.Intn(4) + 1)
	check := false

	if rand.Intn(2) == 0 {
		check = true
		fmt.Printf("Test - %v job(s) - checksum\n", jobs)
	} else {
		fmt.Printf("Test - %v job(s)\n", jobs)
	}

	buf := make([]byte, len(block))
	copy(buf, block)
	var bs util.BufferStream
	blockSize := uint((len(block) / (rand.Intn(3) + 1)) & -16)

	os, err := kio.NewCompressedOutputStream(&bs, "ANS0", "LZX", blockSize, jobs, check)

	if err != nil {
		return 1
	}

	written, err := os.Write(block)

	if err == nil {
		if err = os.Close(); err != nil {
			return 2
		}
	}

	for i := range block {
		block[i] = 0
	}

	is, err := kio.NewCompressedInputStream(&bs, jobs)

	if err != nil {
		return 3
	}

	read, err := is.Read(block)

	if err == nil {
		if err = is.Close(); err != nil {
			return 4
		}
	}

	for i := range block {
		if buf[i] != block[i] {
			return 5
		}
	}

	return read ^ written
}

func compress3(block []byte) int {
	fmt.Println("Test - Incompressible")
	buf := make([]byte, len(block))
	copy(buf, block)
	var bs util.BufferStream
	blockSize := uint((len(block) / (rand.Intn(3) + 1)) & -16)

	os, err := kio.NewCompressedOutputStream(&bs, "FPAQ", "LZP+ZRLT", blockSize, 1, false)

	if err != nil {
		return 1
	}

	written, err := os.Write(block)

	if err == nil {
		if err = os.Close(); err != nil {
			return 2
		}
	}

	for i := range block {
		block[i] = 0
	}

	is, err := kio.NewCompressedInputStream(&bs, 1)

	if err != nil {
		return 3
	}

	read, err := is.Read(block)

	if err == nil {
		if err = is.Close(); err != nil {
			return 4
		}
	}

	for i := range block {
		if buf[i] != block[i] {
			return 5
		}
	}

	return read ^ written
}

func compress4(block []byte) int {
	fmt.Println("Test - write after close")
	buf := make([]byte, len(block))
	copy(buf, block)
	var bs util.BufferStream

	os, err := kio.NewCompressedOutputStream(&bs, "HUFFMAN", "NONE", uint(len(block)), 1, false)

	if err != nil {
		return 1
	}

	_, err = os.Write(block)

	if err == nil {
		if err = os.Close(); err != nil {
			return 2
		}
	}

	_, err = os.Write(block)

	if err != nil {
		fmt.Printf("OK - expected error: %v\n", err)
		return 0
	}

	return 4
}

func compress5(block []byte) int {
	fmt.Println("Test - read after close")
	var bs util.BufferStream

	os, err := kio.NewCompressedOutputStream(&bs, "NONE", "NONE", uint(len(block)), 1, false)

	if err != nil {
		return 1
	}

	_, err = os.Write(block)

	if err == nil {
		if err = os.Close(); err != nil {
			return 2
		}
	}

	is, err := kio.NewCompressedInputStream(&bs, 1)

	if err != nil {
		return 3
	}

	_, err = is.Read(block)

	if err == nil {
		if err = is.Close(); err != nil {
			return 4
		}
	}

	_, err = is.Read(block)

	if err != nil {
		fmt.Printf("OK - expected error: %v\n", err)
		return 0
	}

	return 5
}
