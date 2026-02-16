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
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/transform"
)

func getTransform(name string) (kanzi.ByteTransform, error) {
	ctx := make(map[string]any)
	ctx["transform"] = name
	ctx["bsVersion"] = uint(6)

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

	case "TEXT":
		res, err := transform.NewTextCodecWithCtx(&ctx)
		return res, err

	case "UTF":
		res, err := transform.NewUTFCodecWithCtx(&ctx)
		return res, err

	default:
		panic(fmt.Errorf("No such transform: '%s'", name))
	}
}

func BenchmarkLZ(b *testing.B) {
	benchmarkTransformRoundTrip(b, "LZ", 50000)
}

func BenchmarkLZP(b *testing.B) {
	benchmarkTransformRoundTrip(b, "LZP", 50000)
}

func BenchmarkLZX(b *testing.B) {
	benchmarkTransformRoundTrip(b, "LZX", 50000)
}

func BenchmarkCopy(b *testing.B) {
	benchmarkTransformRoundTrip(b, "NONE", 50000)
}

func BenchmarkAlias(b *testing.B) {
	benchmarkTransformRoundTrip(b, "ALIAS", 50000)
}

func BenchmarkROLZ(b *testing.B) {
	benchmarkTransformRoundTrip(b, "ROLZ", 50000)
}

func BenchmarkZRLT(b *testing.B) {
	benchmarkTransformRoundTrip(b, "ZRLT", 50000)
}

func BenchmarkRLT(b *testing.B) {
	benchmarkTransformRoundTrip(b, "RLT", 50000)
}

func BenchmarkSRT(b *testing.B) {
	benchmarkTransformRoundTrip(b, "SRT", 50000)
}

func BenchmarkROLZX(b *testing.B) {
	benchmarkTransformRoundTrip(b, "ROLZX", 50000)
}

func BenchmarkRank(b *testing.B) {
	benchmarkTransformRoundTrip(b, "RANK", 50000)
}

func BenchmarkMTFT(b *testing.B) {
	benchmarkTransformRoundTrip(b, "MTFT", 50000)
}

func BenchmarkText(b *testing.B) {
	benchmarkTransformRoundTrip(b, "TEXT", 256*1024)
}

func BenchmarkUTF(b *testing.B) {
	benchmarkTransformRoundTrip(b, "UTF", 256*1024)
}

func benchmarkTransformRoundTrip(b *testing.B, name string, size int) {
	input := makeBenchmarkInput(name, size, 1234567)
	fwd, err := getTransform(name)

	if err != nil {
		b.Fatalf("cannot create forward transform '%s': %v", name, err)
	}

	inv, err := getTransform(name)

	if err != nil {
		b.Fatalf("cannot create inverse transform '%s': %v", name, err)
	}

	output := make([]byte, fwd.MaxEncodedLen(len(input)))
	reverse := make([]byte, len(input))
	_, dstIdx, err := fwd.Forward(input, output)

	if err != nil {
		b.Skipf("forward preflight skipped for %s: %v", name, err)
	}

	if _, _, err = inv.Inverse(output[:dstIdx], reverse); err != nil {
		b.Skipf("inverse preflight skipped for %s: %v", name, err)
	}

	if !bytes.Equal(input, reverse) {
		b.Fatalf("preflight mismatch for %s", name)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, dstIdx, err = fwd.Forward(input, output)

		if err != nil {
			b.Fatalf("forward failed for %s: %v", name, err)
		}

		if _, _, err = inv.Inverse(output[:dstIdx], reverse); err != nil {
			b.Fatalf("inverse failed for %s: %v", name, err)
		}
	}
}

func makeBenchmarkInput(name string, size int, seed int64) []byte {
	input := make([]byte, size)

	switch name {
	case "TEXT":
		sample := []byte("The quick brown fox jumps over the lazy dog. This benchmark line is repeated for text transform validation.\r\n")
		for i := range input {
			input[i] = sample[i%len(sample)]
		}
		return input

	case "UTF":
		// Generate valid UTF-8 with a high ratio of multibyte symbols.
		// Keep rune boundaries intact to avoid trailing partial code points.
		pair := []byte{0xC3, 0xA9} // "é"
		n := len(input) / 2

		for i := 0; i < n; i++ {
			copy(input[2*i:], pair)
		}

		if len(input)&1 != 0 {
			input[len(input)-1] = 'a'
		}
		return input
	}

	// Generic compressible data with runs. Keep leading zeros to help ZRLT.
	for i := 0; i < min(32, len(input)); i++ {
		input[i] = 0
	}

	r := rand.New(rand.NewSource(seed))
	n := min(32, len(input))

	for n < len(input) {
		val := byte(r.Intn(64))

		if val%7 == 0 {
			val = 0
		}

		run := r.Intn(120) - 20
		run = max(run, 1)
		end := min(len(input), n+run)

		for ; n < end; n++ {
			input[n] = val
		}
	}

	return input
}
