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
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	kanzi "github.com/flanglet/kanzi-go"
	"github.com/flanglet/kanzi-go/transform"
)

func TestBWT(b *testing.T) {
	if err := testCorrectnessBWT(false); err != nil {
		b.Errorf(err.Error())
	}
}

func TestBWTS(b *testing.T) {
	if err := testCorrectnessBWT(true); err != nil {
		b.Errorf(err.Error())
	}
}

func testCorrectnessBWT(isBWT bool) error {
	if isBWT {
		fmt.Println("Test BWT")
	} else {
		fmt.Println("Test BWTS")
	}

	// Test behavior
	for ii := 1; ii <= 20; ii++ {
		fmt.Printf("\nTest %v\n", ii)
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

		var buf1 []byte
		var buf2 []byte
		var buf3 []byte

		if ii == 1 {
			buf1 = []byte("mississippi")
		} else if ii == 2 {
			buf1 = []byte("3.14159265358979323846264338327950288419716939937510")
		} else if ii == 3 {
			buf1 = []byte("SIX.MIXED.PIXIES.SIFT.SIXTY.PIXIE.DUST.BOXES")
		} else {
			buf1 = make([]byte, 128)

			for i := 0; i < len(buf1); i++ {
				buf1[i] = byte(65 + rnd.Intn(4*ii))
			}
		}

		buf2 = make([]byte, len(buf1))
		buf3 = make([]byte, len(buf1))
		var bwt kanzi.ByteTransform

		if isBWT {
			bwt, _ = transform.NewBWT()
		} else {
			bwt, _ = transform.NewBWTS()
		}

		str1 := string(buf1)
		fmt.Printf("Input:   %s\n", str1)
		_, _, err1 := bwt.Forward(buf1, buf2)

		if err1 != nil {
			fmt.Printf("Error: %v\n", err1)
			return err1
		}

		str2 := string(buf2)

		if isBWT {
			primaryIndex := bwt.(*transform.BWT).PrimaryIndex(0)
			fmt.Printf("Encoded: %s  (Primary index=%v)\n", str2, primaryIndex)
		} else {
			fmt.Printf("Encoded: %s\n", str2)
		}

		_, _, err2 := bwt.Inverse(buf2, buf3)

		if err2 != nil {
			fmt.Printf("Error: %v\n", err2)
			return err2
		}

		str3 := string(buf3)
		fmt.Printf("Output:  %s\n", str3)

		if str1 != str3 {
			return errors.New("Input and inverse are different")
		}
	}

	return error(nil)
}
