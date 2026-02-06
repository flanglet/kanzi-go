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
	"bytes"
	"math/rand"
	"testing"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/transform"
)

func BenchmarkBWTSmallBlock(b *testing.B) {
	benchmarkBWTRoundTrip(b, true, 256*1024)
}

func BenchmarkBWTBigBlock(b *testing.B) {
	benchmarkBWTRoundTrip(b, true, 10*1024*1024)
}

func BenchmarkBWTS(b *testing.B) {
	benchmarkBWTRoundTrip(b, false, 256*1024)
}

func benchmarkBWTRoundTrip(b *testing.B, isBWT bool, size int) {
	buf1 := make([]byte, size)
	buf2 := make([]byte, size)
	buf3 := make([]byte, size)
	r := rand.New(rand.NewSource(1234567))

	for i := range buf1 {
		buf1[i] = byte(r.Intn(255) + 1)
	}

	var tf kanzi.ByteTransform
	var err error

	if isBWT {
		tf, err = transform.NewBWT()
	} else {
		tf, err = transform.NewBWTS()
	}

	if err != nil {
		b.Fatalf("cannot create transform: %v", err)
	}

	if _, _, err = tf.Forward(buf1, buf2); err != nil {
		b.Fatalf("forward preflight failed: %v", err)
	}

	if _, _, err = tf.Inverse(buf2, buf3); err != nil {
		b.Fatalf("inverse preflight failed: %v", err)
	}

	if !bytes.Equal(buf1, buf3) {
		b.Fatalf("preflight mismatch")
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(buf1)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, _, err = tf.Forward(buf1, buf2); err != nil {
			b.Fatalf("forward failed: %v", err)
		}

		if _, _, err = tf.Inverse(buf2, buf3); err != nil {
			b.Fatalf("inverse failed: %v", err)
		}
	}
}
