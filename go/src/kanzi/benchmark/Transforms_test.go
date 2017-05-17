/*
Copyright 2011-2013 Frederic Langlet
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
	"kanzi/transform"
	"math/rand"
	"testing"
)

func BenchmarkDCT4(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				dct4, _ := transform.NewDCT4()

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 16)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						dct4.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						dct4.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}

func BenchmarkDCT8(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				dct8, _ := transform.NewDCT8()

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 64)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						dct8.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						dct8.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}

func BenchmarkDCT16(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				dct16, _ := transform.NewDCT16()

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 256)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						dct16.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						dct16.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}

func BenchmarkDCT32(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				dct32, _ := transform.NewDCT32()

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 1024)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						dct32.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						dct32.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}

func BenchmarkDST4(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				dst4, _ := transform.NewDST4()

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 16)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						dst4.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						dst4.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}

func BenchmarkWHT4(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				wht4, _ := transform.NewWHT4(true)

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 16)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						wht4.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						wht4.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}

func BenchmarkWHT8(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				wht8, _ := transform.NewWHT8(true)

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 64)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						wht8.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						wht8.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}

func BenchmarkWHT16(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				wht16, _ := transform.NewWHT16(true)

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 256)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						wht16.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						wht16.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}

func BenchmarkWHT32(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				wht32, _ := transform.NewWHT32(true)

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 1024)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						wht32.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						wht32.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}

func BenchmarkDWT8(b *testing.B) {
	for _, direction := range [...]string{"encode", "decode"} {
		b.Run(direction, func(b *testing.B) {
			iter := b.N

			for times := 0; times < 10; times++ {
				data := make([][]int, 1000)
				dwt8, _ := transform.NewDWT(8, 8, 1)

				for i := 0; i < 1000; i++ {
					data[i] = make([]int, 64)

					for j := 0; j < len(data[i]); j++ {
						data[i][j] = rand.Intn(10 + i + j*10)
					}
				}

				if direction == "encode" {
					for i := 0; i < iter; i++ {
						dwt8.Forward(data[i%100], data[i%100])
					}
				} else {
					for i := 0; i < iter; i++ {
						dwt8.Inverse(data[i%100], data[i%100])
					}
				}

			}
		})
	}
}
