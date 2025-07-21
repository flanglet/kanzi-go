/*
Copyright 2011-2025 Frederic Langlet
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
	"github.com/flanglet/kanzi-go/v2/transform"
)

func getTransform(name string) (kanzi.ByteTransform, error) {
	ctx := make(map[string]any)
	ctx["transform"] = name
	ctx["bsVersion"] = uint(4)

	switch name {
	case "LZ":
		ctx["lz"] = transform.LZ_TYPE
		res, err := transform.NewLZCodecWithCtx(&ctx)
		return res, err

	case "LZX":
		ctx["lz"] = transform.LZX_TYPE
		res, err := transform.NewLZCodecWithCtx(&ctx)
		return res, err

	case "LZP":
		ctx["lz"] = transform.LZP_TYPE
		res, err := transform.NewLZCodecWithCtx(&ctx)
		return res, err

	case "ALIAS":
		res, err := transform.NewAliasCodecWithCtx(&ctx)
		return res, err

	case "NONE":
		res, err := transform.NewNullTransformWithCtx(&ctx)
		return res, err

	case "ZRLT":
		res, err := transform.NewZRLTWithCtx(&ctx)
		return res, err

	case "RLT":
		res, err := transform.NewRLTWithCtx(&ctx)
		return res, err

	case "SRT":
		res, err := transform.NewSRTWithCtx(&ctx)
		return res, err

	case "ROLZ", "ROLZX":
		res, err := transform.NewROLZCodecWithCtx(&ctx)
		return res, err

	case "RANK":
		res, err := transform.NewSBRT(transform.SBRT_MODE_RANK)
		return res, err

	case "MTFT":
		res, err := transform.NewSBRT(transform.SBRT_MODE_MTF)
		return res, err

	default:
		panic(fmt.Errorf("No such transform: '%s'", name))
	}
}

func BenchmarkLZ(b *testing.B) {
	if err := testTransformSpeed("LZ", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func BenchmarkLZP(b *testing.B) {
	if err := testTransformSpeed("LZP", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func BenchmarkLZX(b *testing.B) {
	if err := testTransformSpeed("LZX", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func BenchmarkCopy(b *testing.B) {
	if err := testTransformSpeed("NONE", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func BenchmarkAlias(b *testing.B) {
	if err := testTransformSpeed("ALIAS", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func BenchmarkROLZ(b *testing.B) {
	if err := testTransformSpeed("ROLZ", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func BenchmarkZRLT(b *testing.B) {
	if err := testTransformSpeed("ZRLT", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func BenchmarkRLT(b *testing.B) {
	if err := testTransformSpeed("RLT", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func BenchmarkSRT(b *testing.B) {
	if err := testTransformSpeed("SRT", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func BenchmarkROLZX(b *testing.B) {
	if err := testTransformSpeed("ROLZX", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}
func BenchmarkRank(b *testing.B) {
	if err := testTransformSpeed("RANK", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}
func BenchmarkMTFT(b *testing.B) {
	if err := testTransformSpeed("MTFT", b.N); err != nil {
		b.Fatalf(err.Error())
	}
}

func testTransformSpeed(name string, iter int) error {
	// Initialize with a fixed seed to get consistent results
	r := rand.New(rand.NewSource(1234567))
	size := 50000

	for jj := 0; jj < 3; jj++ {
		input := make([]byte, size)
		output := make([]byte, 8*size)
		reverse := make([]byte, size)

		// Generate random data with runs
		// Leave zeros at the beginning for ZRLT to succeed
		n := iter / 20

		for n < len(input) {
			val := byte(r.Intn(64))

			if val%7 == 0 {
				val = 0
			}

			input[n] = val
			n++
			run := rand.Intn(120) // above LZP min match threshold
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
			f, _ := getTransform(name)

			_, dstIdx, err = f.Forward(input, output)

			if err != nil {
				return err
			}
		}

		for ii := 0; ii < iter; ii++ {
			f, _ := getTransform(name)

			if _, _, err = f.Inverse(output[0:dstIdx], reverse); err != nil {
				return err
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
			err := fmt.Errorf("Failure at index %v of %v (%v <-> %v)", idx, len(input), input[idx], reverse[idx])
			return err
		}

	}

	return nil
}
