/*
Copyright 2011-2026 Frederic Langlet
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

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/bitstream"
	"github.com/flanglet/kanzi-go/v2/entropy"
	"github.com/flanglet/kanzi-go/v2/internal"
)

func BenchmarkExpGolomb(b *testing.B) {
	if err := testEntropySpeed(b, "EXPGOLOMB"); err != nil {
		b.Errorf(err.Error())
	}
}

func BenchmarkHuffman(b *testing.B) {
	if err := testEntropySpeed(b, "HUFFMAN"); err != nil {
		b.Errorf(err.Error())
	}
}

func BenchmarkANS0(b *testing.B) {
	if err := testEntropySpeed(b, "ANS0"); err != nil {
		b.Errorf(err.Error())
	}
}

func BenchmarkANS1(b *testing.B) {
	if err := testEntropySpeed(b, "ANS1"); err != nil {
		b.Errorf(err.Error())
	}
}

func BenchmarkFPAQ(b *testing.B) {
	if err := testEntropySpeed(b, "FPAQ"); err != nil {
		b.Errorf(err.Error())
	}
}

func BenchmarkCM(b *testing.B) {
	if err := testEntropySpeed(b, "CM"); err != nil {
		b.Errorf(err.Error())
	}
}

func BenchmarkTPAQ(b *testing.B) {
	if err := testEntropySpeed(b, "TPAQ"); err != nil {
		b.Errorf(err.Error())
	}
}

func getEncoder(name string, obs kanzi.OutputBitStream) kanzi.EntropyEncoder {
	ctx := make(map[string]any)
	ctx["entropy"] = name
	ctx["bsVersion"] = uint(6)
	eType, _ := entropy.GetType(name)

	res, err := entropy.NewEntropyEncoder(obs, ctx, eType)

	if err != nil {
		panic(err.Error())
	}

	return res
}

func getDecoder(name string, ibs kanzi.InputBitStream) kanzi.EntropyDecoder {
	ctx := make(map[string]any)
	ctx["entropy"] = name
	ctx["bsVersion"] = uint(6)
	eType, _ := entropy.GetType(name)

	res, err := entropy.NewEntropyDecoder(ibs, ctx, eType)

	if err != nil {
		panic(err.Error())
	}

	return res
}

func testEntropySpeed(b *testing.B, name string) error {
	// Initialize with a fixed seed to get consistent results
	r := rand.New(rand.NewSource(1234567))
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		iter := b.N
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		bs := internal.NewBufferStream(make([]byte, 0, size))

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(r.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(bs, uint(size))
			ec := getEncoder(name, obs)

			// Encode
			if _, err := ec.Write(values1); err != nil {
				msg := fmt.Sprintf("An error occurred during encoding: %v\n", err)
				b.Fatalf(msg)
			}

			ec.Dispose()

			if err := obs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(bs, uint(size))
			ed := getDecoder(name, ibs)

			// Decode
			if _, err := ed.Read(values2); err != nil {
				msg := fmt.Sprintf("An error occurred during decoding: %v\n", err)
				b.Fatalf(msg)
			}

			ed.Dispose()

			if err := ibs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}
		}

		bs.Close()
	}

	return nil
}
