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

package benchmark

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/flanglet/kanzi-go/transform"
)

func BenchmarkRANK(b *testing.B) {
	iter := b.N
	size := 50000

	for jj := 0; jj < 3; jj++ {
		input := make([]byte, size)
		output := make([]byte, size)
		reverse := make([]byte, size)
		rand.Seed(int64(jj))
		n := 0

		for n < len(input) {
			val := byte(rand.Intn(255))
			input[n] = val
			n++
			run := rand.Intn(55)
			run -= 20

			for run > 0 && n < len(input) {
				input[n] = val
				n++
				run--
			}
		}

		var dstIdx uint
		var err error

		for ii := 0; ii < iter; ii++ {
			f, _ := transform.NewSBRT(transform.SBRT_MODE_RANK)

			_, dstIdx, err = f.Forward(input, output)

			if err != nil {
				msg := fmt.Sprintf("Encoding error : %v\n", err)
				b.Fatalf(msg)
			}
		}

		for ii := 0; ii < iter; ii++ {
			f, _ := transform.NewSBRT(transform.SBRT_MODE_RANK)

			if _, _, err = f.Inverse(output[0:dstIdx], reverse); err != nil {
				msg := fmt.Sprintf("Decoding error : %v\n", err)
				b.Fatalf(msg)
			}
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
			msg := fmt.Sprintf("Failure at index %v (%v <-> %v)\n", idx, input[idx], reverse[idx])
			b.Fatalf(msg)
		}

	}
}

func BenchmarkMTFT(b *testing.B) {
	iter := b.N
	size := 50000

	for jj := 0; jj < 3; jj++ {
		input := make([]byte, size)
		output := make([]byte, size)
		reverse := make([]byte, size)
		rand.Seed(int64(jj))
		n := 0

		for n < len(input) {
			val := byte(rand.Intn(255))
			input[n] = val
			n++
			run := rand.Intn(55)
			run -= 20

			for run > 0 && n < len(input) {
				input[n] = val
				n++
				run--
			}
		}

		var dstIdx uint
		var err error

		for ii := 0; ii < iter; ii++ {
			f, _ := transform.NewSBRT(transform.SBRT_MODE_MTF)

			_, dstIdx, err = f.Forward(input, output)

			if err != nil {
				msg := fmt.Sprintf("Encoding error : %v\n", err)
				b.Fatalf(msg)
			}
		}

		for ii := 0; ii < iter; ii++ {
			f, _ := transform.NewSBRT(transform.SBRT_MODE_MTF)

			if _, _, err = f.Inverse(output[0:dstIdx], reverse); err != nil {
				msg := fmt.Sprintf("Decoding error : %v\n", err)
				b.Fatalf(msg)
			}
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
			msg := fmt.Sprintf("Failure at index %v (%v <-> %v)\n", idx, input[idx], reverse[idx])
			b.Fatalf(msg)
		}

	}
}
