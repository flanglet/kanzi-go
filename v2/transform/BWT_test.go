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

package transform

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

func TestBWT(b *testing.T) {
	if err := testCorrectnessBWT(true, testing.Verbose()); err != nil {
		b.Errorf(err.Error())
	}
}

func TestBWTS(b *testing.T) {
	if err := testCorrectnessBWT(false, testing.Verbose()); err != nil {
		b.Errorf(err.Error())
	}
}

func testCorrectnessBWT(isBWT, verbose bool) error {
	if verbose {
		if isBWT {
			fmt.Println("Test BWT")
		} else {
			fmt.Println("Test BWTS")
		}
	}

	// Test behavior
	for ii := 1; ii <= 20; ii++ {
		if verbose {
			fmt.Printf("\nTest %v\n", ii)
		}

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
		} else if ii < 19 {
			buf1 = make([]byte, 128)

			for i := range buf1 {
				buf1[i] = byte(65 + rnd.Intn(4*ii))
			}
		} else if ii < 20 {
			buf1 = make([]byte, _BWT_BLOCK_SIZE_THRESHOLD2/2)

			for i := range buf1 {
				buf1[i] = byte(i)
			}
		} else {
			buf1 = make([]byte, _BWT_BLOCK_SIZE_THRESHOLD2*2)

			for i := range buf1 {
				buf1[i] = byte(i)
			}
		}

		buf2 = make([]byte, len(buf1))
		buf3 = make([]byte, len(buf1))
		var tf kanzi.ByteTransform

		if isBWT {
			tf, _ = NewBWT()
		} else {
			tf, _ = NewBWTS()
		}

		str1 := string(buf1)

		if verbose {
			if len(str1) < 512 {
				fmt.Printf("Input:   %s\n", str1)
			}
		}

		_, _, err1 := tf.Forward(buf1, buf2)

		if err1 != nil {
			return fmt.Errorf("Error: %v\n", err1)
		}

		str2 := string(buf2)

		if verbose {
			if len(str2) < 512 {
				fmt.Printf("Encoded: %s", str2)
			}
		}

		if isBWT {
			bwt := tf.(*BWT)
			chunks := GetBWTChunks(len(buf1))
			pi := make([]uint, chunks)

			for i := range pi {
				pi[i] = bwt.PrimaryIndex(i)

				if verbose {
					fmt.Printf("(Primary index=%v)\n", pi[i])
				}
			}

			tf, _ = NewBWT()
			bwt = tf.(*BWT)

			for i := range pi {
				bwt.SetPrimaryIndex(i, pi[i])
			}
		} else {
			tf, _ = NewBWTS()

			if verbose {
				if len(str2) < 512 {
					fmt.Printf("Encoded: %s\n", str2)
				}
			}
		}

		_, _, err2 := tf.Inverse(buf2, buf3)

		if err2 != nil {
			return fmt.Errorf("Error: %v\n", err2)
		}

		str3 := string(buf3)

		if verbose {
			if len(str3) < 512 {
				fmt.Printf("Output:  %s\n", str3)
			}
		}

		if str1 == str3 {
			if verbose {
				fmt.Println("Identical")
			}
		} else {
			idx := -1

			for i := range buf1 {
				if buf1[i] != buf3[i] {
					idx = i
					break
				}
			}

			return fmt.Errorf("Different at index %v %v <-> %v", idx, buf1[idx], buf3[idx])
		}
	}

	return error(nil)
}
