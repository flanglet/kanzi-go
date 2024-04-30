/*
Copyright 2011-2024 Frederic Langlet
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

package io

import (
	"fmt"
	"github.com/flanglet/kanzi-go/v2/internal"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestCompressedStream(b *testing.T) {
	fmt.Println("Correctness Test")
	values := make([]byte, 65536<<6)
	incompressible := make([]byte, 65536<<6)
	sum := 0

	for test := 1; test <= 20; test++ {
		length := 65536 << uint(test%7)
		fmt.Printf("\nIteration %v\n", test)

		for i := range values {
			values[i] = byte(rand.Intn(4*test + 1))
			incompressible[i] = byte(rand.Intn(256))
		}

		if res := compress(values[0:length], "HUFFMAN", "LZ"); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Printf("Failure %v\n", res)
			sum += res
			break
		}

		if res := compress(values[0:length], "NONE", "ROLZ"); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Printf("Failure %v\n", res)
			sum += res
			break
		}

		if res := compress(values[0:length], "FPAQ", "BWT"); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Printf("Failure %v\n", res)
			sum += res
			break
		}

		if res := compress(incompressible[0:length], "HUFFMAN", "LZ"); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Printf("Failure %v\n", res)
			sum += res
		}
	}

	if res := compressAfterWriteClose(values[0:65536]); res == 0 {
		fmt.Println("Success")
	} else {
		fmt.Println("Failure")
		sum += res
	}

	if res := compressAfterReadClose(values[0:65536]); res == 0 {
		fmt.Println("Success")
	} else {
		fmt.Printf("Failure %v\n", res)
		sum += res
	}

	fmt.Println()

	if sum != 0 {
		b.Error()
	}
}

func compress(block []byte, entropy, transform string) int {
	jobs := uint(rand.Intn(4) + 1)
	var blockSize uint

	if n := rand.Intn(3); n == 1 {
		blockSize = uint(len(block))
	} else {
		blockSize = uint((len(block) / (n + 1)) & -16)
	}

	fmt.Printf("Block size: %v, jobs: %v \n", blockSize, jobs)

	{
		// Create an io.WriteCloser
		outputName := filepath.Join(os.TempDir(), "compressed.knz")
		output, err := os.Create(outputName)

		if err != nil {
			fmt.Printf("%v\n", err)
			return 1
		}

		// Create a Writer
		w, err2 := NewWriter(output, transform, entropy, blockSize, jobs, true, 0, false)

		if err2 != nil {
			fmt.Printf("%v\n", err2)
			return 2
		}

		// Compress block
		_, err = w.Write(block)

		if err != nil {
			fmt.Printf("%v\n", err)
			return 3
		}

		// Close Writer
		err = w.Close()

		if err != nil {
			fmt.Printf("%v\n", err)
			return 4
		}
	}

	{
		// Create an io.ReadCloser
		inputName := filepath.Join(os.TempDir(), "compressed.knz")
		input, err := os.Open(inputName)

		if err != nil {
			fmt.Printf("%v\n", err)
			return 5
		}

		// Create a Reader
		r, err2 := NewReader(input, 4)

		if err2 != nil {
			fmt.Printf("%v\n", err2)
			return 6
		}

		// Decompress block
		_, err = r.Read(block)

		if err != nil {
			fmt.Printf("%v\n", err)
			return 7
		}

		// Close Reader
		err = r.Close()

		if err != nil {
			fmt.Printf("%v\n", err)
			return 8
		}
	}

	// If we made it until here, the roundtrip is valid.
	// The checksum verification guarantees that the data
	// has been decompressed correctly.
	return 0
}

func compressAfterWriteClose(block []byte) int {
	fmt.Println("Test - write after close")
	buf := make([]byte, len(block))
	copy(buf, block)
	bs := internal.NewBufferStream()

	os, err := NewWriter(bs, "NONE", "HUFFMAN", uint(len(block)), 1, false, 0, false)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}

	_, err = os.Write(block)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 2
	}

	if err = os.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return 3
	}

	_, err = os.Write(block)

	if err != nil {
		fmt.Printf("OK - expected error: %v\n", err)
		return 0
	}

	return 4
}

func compressAfterReadClose(block []byte) int {
	fmt.Println("Test - read after close")
	bs := internal.NewBufferStream()

	os, err := NewWriter(bs, "NONE", "NONE", uint(len(block)), 1, false, 0, false)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}

	_, err = os.Write(block)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 2
	}

	if err = os.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return 3
	}

	is, err := NewReader(bs, 1)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 4
	}

	_, err = is.Read(block)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 5
	}

	if err = is.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return 6
	}

	_, err = is.Read(block)

	if err != nil {
		fmt.Printf("OK - expected error: %v\n", err)
		return 0
	}

	return 7
}
