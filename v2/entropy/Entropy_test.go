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

package entropy

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/bitstream"
	"github.com/flanglet/kanzi-go/v2/util"
)

func TestHuffman(b *testing.T) {
	if err := testEntropyCorrectness("HUFFMAN"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestANS0(b *testing.T) {
	if err := testEntropyCorrectness("ANS0"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestANS1(b *testing.T) {
	if err := testEntropyCorrectness("ANS1"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestRange(b *testing.T) {
	if err := testEntropyCorrectness("RANGE"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestFPAQ(b *testing.T) {
	if err := testEntropyCorrectness("FPAQ"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestCM(b *testing.T) {
	if err := testEntropyCorrectness("CM"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestTPAQ(b *testing.T) {
	if err := testEntropyCorrectness("TPAQ"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestExpGolomb(b *testing.T) {
	if err := testEntropyCorrectness("EXPGOLOMB"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestRiceGolomb(b *testing.T) {
	if err := testEntropyCorrectness("RICEGOLOMB"); err != nil {
		b.Errorf(err.Error())
	}
}

func getPredictor(name string) kanzi.Predictor {
	switch name {
	case "TPAQ":
		res, _ := NewTPAQPredictor(nil)
		return res

	case "CM":
		res, _ := NewCMPredictor(nil)
		return res

	default:
		panic(fmt.Errorf("Unsupported type: '%s'", name))
	}
}

func getEncoder(name string, obs kanzi.OutputBitStream) kanzi.EntropyEncoder {
	ctx := make(map[string]interface{})
	ctx["entropy"] = name
	ctx["bsVersion"] = uint(4)
	eType, _ := GetType(name)

	res, err := NewEntropyEncoder(obs, ctx, eType)

	if err != nil {
		panic(err.Error())
	}

	return res
}

func getDecoder(name string, ibs kanzi.InputBitStream) kanzi.EntropyDecoder {
	ctx := make(map[string]interface{})
	ctx["entropy"] = name
	ctx["bsVersion"] = uint(4)
	eType, _ := GetType(name)

	res, err := NewEntropyDecoder(ibs, ctx, eType)

	if err != nil {
		panic(err.Error())
	}

	return res
}

func testEntropyCorrectness(name string) error {
	fmt.Println()
	fmt.Printf("=== Testing %v ===\n", name)

	// Test behavior
	for ii := 1; ii < 20; ii++ {
		fmt.Printf("\n\nTest %v", ii)
		var values []byte

		if ii == 3 {
			values = []byte{0, 0, 32, 15, -4 & 0xFF, 16, 0, 16, 0, 7, -1 & 0xFF, -4 & 0xFF, -32 & 0xFF, 0, 31, -1 & 0xFF}
		} else if ii == 2 {
			values = []byte{0x3d, 0x4d, 0x54, 0x47, 0x5a, 0x36, 0x39, 0x26, 0x72, 0x6f, 0x6c, 0x65, 0x3d, 0x70, 0x72, 0x65}
		} else if ii == 1 {
			values = make([]byte, 32)

			for i := range values {
				values[i] = byte(2) // all identical
			}
		} else if ii == 5 {
			values = make([]byte, 32)

			for i := range values {
				values[i] = byte(2 + (i & 1)) // 2 symbols
			}
		} else {
			values = make([]byte, 256)

			for i := range values {
				values[i] = byte(64 + 4*ii + rand.Intn(8*ii+1))
			}
		}

		fmt.Printf("\nOriginal: \n")

		for i := range values {
			fmt.Printf("%d ", values[i])
		}

		println()
		fmt.Printf("\nEncoded: \n")
		var bs util.BufferStream
		obs, _ := bitstream.NewDefaultOutputBitStream(&bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbgbs.ShowByte(true)
		//dbgbs.Mark(true)
		ec := getEncoder(name, dbgbs)

		if ec == nil {
			return errors.New("Cannot create entropy encoder")
		}

		if _, err := ec.Write(values); err != nil {
			fmt.Printf("Error during encoding: %s", err)
			return err
		}

		ec.Dispose()
		dbgbs.Close()
		println()
		fmt.Printf("\nDecoded: \n")

		ibs, _ := bitstream.NewDefaultInputBitStream(&bs, 16384)
		ed := getDecoder(name, ibs)

		if ed == nil {
			return errors.New("Cannot create entropy decoder")
		}

		ok := true
		values2 := make([]byte, len(values))

		if _, err := ed.Read(values2); err != nil {
			fmt.Printf("Error during decoding: %s", err)
			return err
		}

		ed.Dispose()

		for i := range values2 {
			fmt.Printf("%v ", values2[i])

			if values[i] != values2[i] {
				ok = false
			}
		}

		if ok == true {
			fmt.Printf("\nIdentical")
		} else {
			fmt.Printf("\n! *** Different *** !")
			return errors.New("Input and inverse are different")
		}

		ibs.Close()
		bs.Close()
		println()
	}

	return error(nil)
}
