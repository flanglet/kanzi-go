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

package benchmark

import (
	"fmt"
	"kanzi/bitstream"
	"kanzi/entropy"
	"kanzi/io"
	"math/rand"
	"testing"
)

func BenchmarkExpGolomb(b *testing.B) {
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		iter := b.N
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs io.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(rand.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			ec, _ := entropy.NewExpGolombEncoder(obs, true)

			// Encode
			if _, err := ec.Encode(values1); err != nil {
				msg := fmt.Sprintf("An error occured during encoding: %v\n", err)
				b.Fatalf(msg)
			}

			ec.Dispose()

			if _, err := obs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			ed, _ := entropy.NewExpGolombDecoder(ibs, true)

			// Decode
			if _, err := ed.Decode(values2); err != nil {
				msg := fmt.Sprintf("An error occured during decoding: %v\n", err)
				b.Fatalf(msg)
			}

			ed.Dispose()

			if _, err := ibs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}
		}

		bs.Close()
	}
}

func BenchmarkHuffman(b *testing.B) {
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		iter := b.N
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs io.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(rand.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			ec, _ := entropy.NewHuffmanEncoder(obs)

			// Encode
			if _, err := ec.Encode(values1); err != nil {
				msg := fmt.Sprintf("An error occured during encoding: %v\n", err)
				b.Fatalf(msg)
			}

			ec.Dispose()

			if _, err := obs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			ed, _ := entropy.NewHuffmanDecoder(ibs)

			// Decode
			if _, err := ed.Decode(values2); err != nil {
				msg := fmt.Sprintf("An error occured during decoding: %v\n", err)
				b.Fatalf(msg)
			}

			ed.Dispose()

			if _, err := ibs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}
		}

		bs.Close()
	}
}

func BenchmarkANS(b *testing.B) {
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		iter := b.N
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs io.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(rand.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			ec, _ := entropy.NewANSRangeEncoder(obs)

			// Encode
			if _, err := ec.Encode(values1); err != nil {
				msg := fmt.Sprintf("An error occured during encoding: %v\n", err)
				b.Fatalf(msg)
			}

			ec.Dispose()

			if _, err := obs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			ed, _ := entropy.NewANSRangeDecoder(ibs)

			// Decode
			if _, err := ed.Decode(values2); err != nil {
				msg := fmt.Sprintf("An error occured during decoding: %v\n", err)
				b.Fatalf(msg)
			}

			ed.Dispose()

			if _, err := ibs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}
		}

		bs.Close()
	}
}

func BenchmarkRange(b *testing.B) {
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		iter := b.N
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs io.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(rand.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			ec, _ := entropy.NewRangeEncoder(obs)

			// Encode
			if _, err := ec.Encode(values1); err != nil {
				msg := fmt.Sprintf("An error occured during encoding: %v\n", err)
				b.Fatalf(msg)
			}

			ec.Dispose()

			if _, err := obs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			ed, _ := entropy.NewRangeDecoder(ibs)

			// Decode
			if _, err := ed.Decode(values2); err != nil {
				msg := fmt.Sprintf("An error occured during decoding: %v\n", err)
				b.Fatalf(msg)
			}

			ed.Dispose()

			if _, err := ibs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}
		}

		bs.Close()
	}
}

func BenchmarkFPAQ(b *testing.B) {
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		iter := b.N
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs io.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(rand.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			predictor, _ := entropy.NewFPAQPredictor()
			ec, _ := entropy.NewBinaryEntropyEncoder(obs, predictor)

			// Encode
			if _, err := ec.Encode(values1); err != nil {
				msg := fmt.Sprintf("An error occured during encoding: %v\n", err)
				b.Fatalf(msg)
			}

			ec.Dispose()

			if _, err := obs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			predictor, _ = entropy.NewFPAQPredictor()
			ed, _ := entropy.NewBinaryEntropyDecoder(ibs, predictor)

			// Decode
			if _, err := ed.Decode(values2); err != nil {
				msg := fmt.Sprintf("An error occured during decoding: %v\n", err)
				b.Fatalf(msg)
			}

			ed.Dispose()

			if _, err := ibs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}
		}

		bs.Close()
	}
}

func BenchmarkCM(b *testing.B) {
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		iter := b.N
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs io.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(rand.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			predictor, _ := entropy.NewCMPredictor()
			ec, _ := entropy.NewBinaryEntropyEncoder(obs, predictor)

			// Encode
			if _, err := ec.Encode(values1); err != nil {
				msg := fmt.Sprintf("An error occured during encoding: %v\n", err)
				b.Fatalf(msg)
			}

			ec.Dispose()

			if _, err := obs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			predictor, _ = entropy.NewCMPredictor()
			ed, _ := entropy.NewBinaryEntropyDecoder(ibs, predictor)

			// Decode
			if _, err := ed.Decode(values2); err != nil {
				msg := fmt.Sprintf("An error occured during decoding: %v\n", err)
				b.Fatalf(msg)
			}

			ed.Dispose()

			if _, err := ibs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}
		}

		bs.Close()
	}
}

func BenchmarkPAQ(b *testing.B) {
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		iter := b.N
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs io.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(rand.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			predictor, _ := entropy.NewPAQPredictor()
			ec, _ := entropy.NewBinaryEntropyEncoder(obs, predictor)

			// Encode
			if _, err := ec.Encode(values1); err != nil {
				msg := fmt.Sprintf("An error occured during encoding: %v\n", err)
				b.Fatalf(msg)
			}

			ec.Dispose()

			if _, err := obs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			predictor, _ = entropy.NewPAQPredictor()
			ed, _ := entropy.NewBinaryEntropyDecoder(ibs, predictor)

			// Decode
			if _, err := ed.Decode(values2); err != nil {
				msg := fmt.Sprintf("An error occured during decoding: %v\n", err)
				b.Fatalf(msg)
			}

			ed.Dispose()

			if _, err := ibs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}
		}

		bs.Close()
	}
}

func BenchmarkTPAQ(b *testing.B) {
	repeats := []int{3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3}

	for jj := 0; jj < 3; jj++ {
		iter := b.N
		size := 50000
		values1 := make([]byte, size)
		values2 := make([]byte, size)
		var bs io.BufferStream

		for ii := 0; ii < iter; ii++ {
			idx := jj

			for i := 0; i < size; i++ {
				i0 := i

				length := repeats[idx]
				idx = (idx + 1) & 0x0F
				b := byte(rand.Intn(256))

				if i0+length >= size {
					length = size - i0 - 1
				}

				for j := i0; j < i0+length; j++ {
					values1[j] = b
					i++
				}
			}

			obs, _ := bitstream.NewDefaultOutputBitStream(&bs, uint(size))
			predictor, _ := entropy.NewTPAQPredictor()
			ec, _ := entropy.NewBinaryEntropyEncoder(obs, predictor)

			// Encode
			if _, err := ec.Encode(values1); err != nil {
				msg := fmt.Sprintf("An error occured during encoding: %v\n", err)
				b.Fatalf(msg)
			}

			ec.Dispose()

			if _, err := obs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}

			ibs, _ := bitstream.NewDefaultInputBitStream(&bs, uint(size))
			predictor, _ = entropy.NewTPAQPredictor()
			ed, _ := entropy.NewBinaryEntropyDecoder(ibs, predictor)

			// Decode
			if _, err := ed.Decode(values2); err != nil {
				msg := fmt.Sprintf("An error occured during decoding: %v\n", err)
				b.Fatalf(msg)
			}

			ed.Dispose()

			if _, err := ibs.Close(); err != nil {
				msg := fmt.Sprintf("Error during close: %v\n", err)
				b.Fatalf(msg)
			}
		}

		bs.Close()
	}
}
