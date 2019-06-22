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
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	kanzi "github.com/flanglet/kanzi-go"
	"github.com/flanglet/kanzi-go/transform"
)

func BenchmarkBWT(b *testing.B) {
	if err := testBWTSpeed(true); err != nil {
		b.Errorf(err.Error())
	}
}

func BenchmarkBWTS(b *testing.B) {
	if err := testBWTSpeed(false); err != nil {
		b.Errorf(err.Error())
	}
}

func testBWTSpeed(isBWT bool) error {
	iter := 10
	size := 256 * 1024
	buf1 := make([]byte, size)
	buf2 := make([]byte, size)
	buf3 := make([]byte, size)

	for jj := 0; jj < 3; jj++ {
		var bwt kanzi.ByteTransform

		if isBWT {
			bwt, _ = transform.NewBWT()
		} else {
			bwt, _ = transform.NewBWTS()
		}

		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < iter; i++ {
			for i := range buf1 {
				buf1[i] = byte(rnd.Intn(255) + 1)
			}

			_, _, err1 := bwt.Forward(buf1, buf2)

			if err1 != nil {
				fmt.Printf("Error: %v\n", err1)
				return err1
			}

			_, _, err2 := bwt.Inverse(buf2, buf3)

			if err2 != nil {
				fmt.Printf("Error: %v\n", err2)
				return err2
			}

			// Sanity check
			for i := range buf1 {
				if buf1[i] != buf3[i] {
					msg := fmt.Sprintf("Error at index %v: %v<->%v\n", i, buf1[i], buf3[i])
					return errors.New(msg)
				}
			}
		}

	}

	return error(nil)
}
