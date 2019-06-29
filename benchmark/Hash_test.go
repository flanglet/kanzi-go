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
	"testing"

	"github.com/flanglet/kanzi-go/util/hash"
)

func BenchmarkXXHash32b(b *testing.B) {
	buffer := make([]byte, 1024*1024)

	for i := range buffer {
		buffer[i] = byte(i * i)
	}

	hash, err := hash.NewXXHash32(0)

	if err != nil {
		msg := fmt.Sprintf("Failed to create XXHash32: %v\n", err)
		b.Errorf(msg)
	}

	res := uint32(0)
	iter := 1000

	for i := 0; i < iter; i++ {
		hash.SetSeed(uint32(i))
		res += hash.Hash(buffer)
	}

	if res != 3520130870 {
		b.Errorf("Incorrect result for XXHash32")
	}
}

func BenchmarkXXHash64(b *testing.B) {
	buffer := make([]byte, 1024*1024)

	for i := range buffer {
		buffer[i] = byte(i * i)
	}

	hash, err := hash.NewXXHash64(0)

	if err != nil {
		msg := fmt.Sprintf("Failed to create XXHash64: %v\n", err)
		b.Errorf(msg)
	}

	res := uint64(0)
	iter := 1000

	for i := 0; i < iter; i++ {
		hash.SetSeed(uint64(i))
		res += hash.Hash(buffer)
	}

	if res != 14337148651243832073 {
		b.Errorf("Incorrect result for XXHash64")
	}
}
