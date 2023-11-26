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

package transform

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

func getTransform(name string) (kanzi.ByteTransform, error) {
	ctx := make(map[string]any)
	ctx["transform"] = name
	ctx["bsVersion"] = uint(4)

	switch name {
	case "LZ":
		ctx["lz"] = LZ_TYPE
		res, err := NewLZCodecWithCtx(&ctx)
		return res, err

	case "LZX":
		ctx["lz"] = LZX_TYPE
		res, err := NewLZCodecWithCtx(&ctx)
		return res, err

	case "LZP":
		ctx["lz"] = LZP_TYPE
		res, err := NewLZCodecWithCtx(&ctx)
		return res, err

	case "ALIAS":
		res, err := NewAliasCodecWithCtx(&ctx)
		return res, err

	case "NONE":
		res, err := NewNullTransformWithCtx(&ctx)
		return res, err

	case "ZRLT":
		res, err := NewZRLTWithCtx(&ctx)
		return res, err

	case "RLT":
		res, err := NewRLTWithCtx(&ctx)
		return res, err

	case "SRT":
		res, err := NewSRTWithCtx(&ctx)
		return res, err

	case "ROLZ", "ROLZX":
		res, err := NewROLZCodecWithCtx(&ctx)
		return res, err

	case "RANK":
		res, err := NewSBRT(SBRT_MODE_RANK)
		return res, err

	case "MTFT":
		res, err := NewSBRT(SBRT_MODE_MTF)
		return res, err

	case "MM":
		res, err := NewFSDCodecWithCtx(&ctx)
		return res, err

	default:
		panic(fmt.Errorf("No such transform: '%s'", name))
	}
}

func TestLZ(b *testing.T) {
	if err := testTransformCorrectness("LZ"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestLZX(b *testing.T) {
	if err := testTransformCorrectness("LZX"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestLZP(b *testing.T) {
	if err := testTransformCorrectness("LZP"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestROLZ(b *testing.T) {
	if err := testTransformCorrectness("ROLZ"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestROLZX(b *testing.T) {
	if err := testTransformCorrectness("ROLZX"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestCopy(b *testing.T) {
	if err := testTransformCorrectness("NONE"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestAlias(b *testing.T) {
	if err := testTransformCorrectness("ALIAS"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestZRLT(b *testing.T) {
	if err := testTransformCorrectness("ZRLT"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestRLT(b *testing.T) {
	if err := testTransformCorrectness("RLT"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestSRT(b *testing.T) {
	if err := testTransformCorrectness("SRT"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestMM(b *testing.T) {
	if err := testTransformCorrectness("MM"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestRank(b *testing.T) {
	if err := testTransformCorrectness("RANK"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestMTFT(b *testing.T) {
	if err := testTransformCorrectness("MTFT"); err != nil {
		b.Errorf(err.Error())
	}
}

func testTransformCorrectness(name string) error {
	rng := 256
	fmt.Println()
	fmt.Printf("=== Testing %v ===\n", name)

	if name == "ZRLT" {
		rng = 5
	}

	for ii := 0; ii < 50; ii++ {
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		fmt.Printf("\nTest %v\n\n", ii)
		var arr []int

		if ii == 0 {
			//arr = []int{19, 0, 0, 0, 0, 0, 0, 0, 30, 8, 0, 32, 0, 26, 2, 0, 0, 0, 0, 16, 0, 0, 1, 0, 4, 0, 3, 0, 14, 5, 0, 15, 9, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 25, 0, 0, 15, 0, 0, 13, 0, 0, 14, 0, 28, 0, 14, 0, 7, 0, 0, 12, 10, 22, 5, 0, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 12, 0, 23, 15, 15, 11, 0, 0, 24, 0, 27, 0, 28, 0, 0, 0, 0, 0, 0, 0, 0, 0, 15, 0, 0, 0, 23, 0, 12, 3, 0, 25, 22, 26, 0, 0, 0, 12, 0, 20, 0, 0, 29, 15, 0, 0, 0, 9, 0, 0, 29, 21, 0, 0, 0, 0, 0, 0, 0, 0, 0, 28, 13, 0, 0, 0, 0, 18, 0, 0, 9, 0, 0, 0, 0, 26, 0, 0, 15, 24, 5, 0, 6, 0, 0, 1, 10, 0, 0, 0, 27, 0, 11, 0, 0, 13, 0, 32, 0, 0, 15, 0, 0, 25, 7, 0, 30, 0, 23, 0, 0, 0, 0, 17, 0, 0, 0, 0, 16, 18, 0, 0, 13, 0, 0, 0, 17, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 21, 0, 4, 8, 0, 12, 0, 6, 0, 0, 14, 3, 0, 0, 22, 0, 21, 0, 0, 0, 0, 0, 12, 0, 0, 0, 22, 0, 0, 0, 29, 0, 0, 19, 16, 0, 0, 0, 0, 0, 0, 20, 0, 0, 0, 0, 0, 0, 27, 0, 0, 32, 22, 0, 0, 10, 0, 0, 29, 28, 0, 12, 0, 0, 0, 0, 0, 0, 10, 0, 24, 0, 0, 0, 7, 0, 17, 24, 0, 7, 0, 2, 30, 17, 0, 0, 9, 0, 17, 0, 15, 0, 2, 0, 0, 0, 13, 0, 18, 0, 0, 0, 0, 0, 0, 25, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 13, 0, 0, 0, 0, 7, 0, 17, 0, 0, 0, 3, 21, 0, 19, 0, 0, 0, 0, 0, 0, 0, 0, 0, 14, 0, 7, 0, 0, 0, 0, 0, 4, 0, 9, 0, 0, 0, 6, 0, 1, 25, 0, 0, 22, 0, 0, 0, 0, 0, 0, 7, 0, 29, 0, 0, 16, 0, 0, 0, 8, 2, 0, 30, 0, 0, 7, 0, 0, 0, 15, 0, 5, 0, 0, 0, 20, 0, 26, 0, 0, 0, 0, 0, 0, 0, 0, 11, 7, 23, 0, 0, 0, 0, 0, 0, 0, 0, 15, 19, 0, 32, 0, 21, 26, 0, 0, 7, 32, 14, 0, 0, 0, 0, 0, 0, 9, 0, 0, 0, 14, 0, 0, 18, 5, 15, 0, 17, 0, 22, 0, 0, 3, 0, 0, 1, 0, 0, 0, 0, 0, 0, 10, 0, 0, 0, 0, 13, 0, 13, 13, 0, 0, 15, 11, 0, 10, 0, 0, 0, 0, 0, 0, 0, 25, 0, 0, 0, 15, 0, 0, 0, 27, 0, 0, 0, 26, 0, 0, 0, 1, 0, 0, 30, 0, 7, 0, 13, 18, 0, 0, 0, 0, 0, 27, 0, 0, 0, 0, 0, 0, 15, 0, 12, 22, 0, 14, 0, 25, 16, 8, 0, 0, 0, 5, 0, 0, 0, 0, 27, 0, 0, 0, 0, 10, 0, 14, 0, 9, 0, 3, 26, 0, 0, 22, 0, 0, 7, 0, 31, 0, 31, 7, 0, 0, 0, 0, 3, 31, 4, 0, 20, 27, 17, 8, 0, 0, 0, 15, 10, 14, 0, 0, 24, 0, 5, 5, 0, 0, 6, 0, 5, 27, 28, 0, 28, 0, 12, 0, 0, 0, 11, 0, 0, 0, 0, 0, 0, 1, 0, 3, 0, 23, 0, 20, 14, 0, 17, 0, 17, 2, 2, 0, 0, 28, 0, 0, 0, 0, 0, 0, 4, 10, 4, 0, 0, 21, 0, 15, 0, 0, 32, 0, 9, 13, 0, 3, 16, 0, 0, 0, 0, 0, 24, 1, 0, 15, 0, 0, 0, 0, 0, 0, 32, 0, 0, 8, 0, 12, 23, 0, 0, 0, 8, 0, 0, 21, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 18, 16, 10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 12, 0, 0, 0, 31, 0, 10, 22, 0, 28, 0, 13, 0, 0, 0, 0, 0, 0, 0, 0, 30, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 30, 0, 30, 0, 0, 0, 0, 1, 13, 19, 0, 30, 20, 0, 4, 0, 0, 26, 28, 17, 21, 0, 31, 0, 0, 0, 28, 0, 0, 0, 5, 0, 0, 0, 0, 0, 17, 6, 0, 0, 0, 5, 0, 0, 0, 26, 0, 14, 0, 27, 0, 0, 0, 0, 0, 13, 32, 2, 0, 0, 31, 0, 0, 0, 17, 0, 1, 0, 0, 0, 0, 7, 0, 30, 13, 0, 0, 0, 3, 3, 0, 0, 7, 0, 6, 15, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 7, 17, 0, 0, 10, 0, 0, 28, 15, 0, 0, 0, 15, 0, 0, 0, 0, 0, 0, 29, 0, 0, 23, 29, 0, 0, 31, 30, 19, 0, 0, 18, 0, 0, 11, 28, 0, 24, 0, 0, 0, 0, 0, 0, 0, 31, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 14, 0, 13, 0, 0, 22, 0, 0, 0, 0, 24, 0, 0, 0, 0, 0, 6, 19, 21, 4, 0, 0, 0, 8, 0, 0, 0, 19, 0, 14, 23, 0, 0, 14, 0, 0, 28, 0, 0, 0, 0, 30, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 14, 27, 0, 15, 0, 0, 16, 27, 14, 8, 0, 0, 0, 0, 0, 0, 12, 0, 0, 30, 23, 0, 0, 0, 0, 13, 22, 28, 0, 0, 0, 0, 0, 0, 0, 0, 0, 25, 0, 4, 19, 0, 0, 0, 28, 29, 30, 29, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 0, 16, 0, 0, 0, 0, 10, 0, 0, 29, 0, 1, 0, 0, 0, 0, 14, 5, 0, 31, 28, 0, 17, 0, 20, 0, 20, 0, 0, 0, 0, 6, 0, 0, 16, 0, 0, 28, 0, 0, 11, 0, 0, 0, 10, 0, 0, 0, 0, 0, 1, 9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 29, 0, 0, 0, 9, 26, 0, 9, 0, 0, 12, 0, 19, 0, 0, 0, 0, 10, 16, 0, 0, 0, 12, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 28, 0, 0, 0, 0, 0, 0, 0, 15, 12, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 6, 29, 0, 0, 0, 0, 0, 2, 0, 0, 22, 0, 18, 21, 9, 0, 9, 7, 12, 0, 0, 29, 31, 8, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 21, 0, 0, 21, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 28, 0, 0, 0, 13, 0, 0, 30, 0, 0, 0, 5, 0, 21, 0, 15, 3, 0, 29, 7, 0, 0, 25, 10, 0, 15, 6, 0, 2, 0, 0, 0, 0, 29, 0, 0, 0, 0, 0, 13, 0, 22, 0, 8, 4, 0, 11, 0, 27, 0, 0, 0, 0, 0, 30, 0, 0, 0, 26, 0, 4, 0, 26, 0, 0, 0, 17, 26, 0, 27, 0, 0, 0, 0, 28, 0, 0, 22, 0, 10, 0, 12, 0, 21, 21, 0, 12, 0, 0, 25, 3, 0, 0, 0, 0, 0, 0, 0, 31, 0, 12, 0, 13, 0, 0, 0, 0, 0, 8, 27, 0, 0, 0, 0, 0, 0, 26, 5, 24, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 18, 0, 0, 29, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 16, 0, 4, 0, 0, 16, 0, 11, 0, 0, 0, 0, 30, 2, 29, 14, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 25, 0, 0, 0, 0, 30, 0, 0, 7, 23, 17, 0, 0, 0, 22, 0, 0, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 25, 0, 30, 19, 20, 30, 0, 15, 7, 0, 0, 0, 0, 26, 13, 0, 0, 0, 0, 0, 0, 0, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 25, 0, 0, 0, 18, 0, 0, 0, 0, 0, 0, 0, 0, 6, 0, 14, 11, 31, 0, 0, 0, 17, 0, 2, 22, 21, 0, 0, 0, 6, 0, 0, 0, 0, 11, 0, 8, 30, 0, 0, 0, 0, 1, 0, 2, 0, 0, 14, 0, 27, 0, 0, 0, 0, 0, 0, 0, 11, 0, 10, 26, 11, 0, 0, 0, 0, 30, 0, 0, 3, 0, 0, 8, 0, 5, 22, 0, 2, 0, 13, 0, 0, 30, 0, 19, 0, 0, 0, 0, 14, 29, 0, 0, 2, 10, 0, 16, 0, 18, 9, 0, 0, 28, 0, 17, 0, 0, 24, 0, 0, 4, 0, 0, 0, 6, 0, 0, 20, 0, 0, 21, 14, 0, 11, 0, 0, 0, 31, 0, 17, 21, 24, 21, 32, 24, 0, 0, 14, 27, 0, 17, 0, 0, 5, 0, 0, 10, 0, 16, 24, 30, 30, 0, 10, 0, 0, 0, 0, 14, 0, 16, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 31, 0, 0, 0, 0, 22, 5, 3, 10, 27, 0, 3, 0, 15, 0, 22, 0, 19, 0, 25, 0, 5, 0, 0, 12, 0, 0, 0, 0, 0, 0, 0, 29, 27, 4, 0, 0, 0, 0, 0, 23, 0, 17, 27, 0, 27, 13, 0, 13, 0, 27, 28, 0, 15, 0, 0, 32, 0, 0, 24, 0, 0, 0, 0, 0, 0, 27, 0, 0, 27, 0, 11, 0, 14, 0, 3, 1, 0, 0, 0, 14, 27, 26, 0, 0, 0, 0, 6, 31, 0, 0, 0, 0, 23, 23, 11, 1, 0, 0, 21, 9, 0, 0, 0, 0, 5, 0, 0, 0, 20, 0, 0, 5, 2, 0, 0, 25, 25, 0, 20, 0, 0, 0, 0, 0, 0, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 28, 0, 0, 0, 0, 0, 0, 0, 19, 25, 0, 0, 0, 0, 25, 21, 0, 0, 0, 0, 0, 27, 0, 0, 0, 0, 0, 21, 10, 11, 19, 32, 22, 0, 0, 0, 0, 0, 0, 0, 0, 15, 0, 0, 0, 0, 0, 0, 20, 25, 0, 31, 0, 11, 31, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 30, 0, 15, 0, 0, 0, 0, 0, 15, 0, 2, 0, 0, 9, 19, 0, 0, 0, 0, 0, 0, 0, 32, 0, 11, 28, 0, 0, 0, 30, 29, 0, 32, 0, 18, 0, 0, 0, 0, 0, 0, 0, 0, 0, 30, 0, 0, 2, 0, 0, 15, 8, 0, 0, 20, 0, 0, 0, 0, 0, 0, 0, 1, 0, 27, 0, 28, 32, 26, 28, 2, 0, 32, 0, 0, 0, 0, 0, 0, 17, 0, 26, 0, 0, 18, 0, 6, 0, 17, 0, 29, 0, 0, 0, 0, 16, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 21, 22, 0, 0, 0, 6, 28, 0, 0, 0, 21, 0, 12, 19, 0, 0, 0, 0, 0, 0, 0, 23, 7, 0, 11, 0, 0, 0, 0, 0, 0, 0, 0, 23, 0, 0, 0, 0, 28, 0, 29, 0, 21, 6, 31, 0, 3, 28, 0, 7, 0, 0, 14, 0, 0, 0, 0, 10, 0, 0, 0, 0, 24, 14, 32, 0, 0, 0, 25, 0, 0, 0, 13, 28, 0, 27, 0, 19, 0, 15, 23, 4, 10, 23, 0, 0, 0, 29, 0, 0, 0, 0, 0, 0, 0, 0, 0, 12, 0, 0, 2, 0, 16, 0, 0, 0, 15, 0}

			arr = []int{0, 1, 2, 2, 2, 2, 7, 9, 9, 16, 16, 16, 1, 3,
				3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
		} else if ii == 1 {
			arr = make([]int, 80000)

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
				length := rnd.Intn(120) // above LZP min match threshold

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
		f, err := getTransform(name)

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

		f, err = getTransform(name)

		if err != nil {
			fmt.Printf("\nCannot create transform '%v': %v\n", name, err)
			return err
		}

		fmt.Printf("\nOriginal: \n")

		if ii == 1 {
			fmt.Printf("1 8 (%v times)", len(input)-1)
		} else {
			for i := range arr {
				fmt.Printf("%v ", input[i])
			}
		}

		srcIdx, dstIdx, err := f.Forward(input, output)

		if err != nil {
			// Function may fail when compression ratio > 1.0
			fmt.Printf("\nNo compression (ratio > 1.0), skip reverse")
			continue
		}

		if name != "MM" && (srcIdx != uint(size) || srcIdx < dstIdx) {
			fmt.Printf("\nNo compression (ratio > 1.0), skip reverse")
			continue
		}

		fmt.Printf("\nCoded: \n")

		for i := uint(0); i < dstIdx; i++ {
			fmt.Printf("%v ", output[i])
		}

		fmt.Printf(" (Compression ratio: %v%%)\n", int(dstIdx)*100/size)

		f, err = getTransform(name)

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
		idx := -1

		// Check
		for i := range reverse {
			if input[i] != reverse[i] {
				idx = i
				break
			}
		}

		if idx == -1 {
			if ii == 1 {
				fmt.Printf("1 8 (%v times)", len(input)-1)
			} else {
				for i := range reverse {
					fmt.Printf("%v ", reverse[i])
				}
			}

			fmt.Printf("\n")
		} else {
			err := fmt.Errorf("Failure at index %v of %v (%v <-> %v)", idx, len(input), input[idx], reverse[idx])
			return err
		}

		fmt.Printf("Identical\n")
	}

	fmt.Println()
	return error(nil)
}
