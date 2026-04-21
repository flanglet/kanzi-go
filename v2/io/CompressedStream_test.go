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

package io

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/flanglet/kanzi-go/v2/internal"
	stdio "io"
	"math/rand"
	"os"
	"testing"
)

func TestCompressedStream(b *testing.T) {
	fmt.Println("Correctness Test")
	rng := rand.New(rand.NewSource(0x4B414E5A))
	values := make([]byte, 65536<<6)
	incompressible := make([]byte, 65536<<6)
	sum := 0

	for test := 1; test <= 20; test++ {
		length := 65536 << uint(test%7)
		fmt.Printf("\nIteration %v\n", test)

		for i := range values {
			values[i] = byte(rng.Intn(4*test + 1))
			incompressible[i] = byte(rng.Intn(256))
		}

		if res := compress(values[0:length], "HUFFMAN", "LZ", rng); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Printf("Failure %v\n", res)
			sum += res
			break
		}

		if res := compress(values[0:length], "NONE", "ROLZ", rng); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Printf("Failure %v\n", res)
			sum += res
			break
		}

		if res := compress(values[0:length], "FPAQ", "BWT", rng); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Printf("Failure %v\n", res)
			sum += res
			break
		}

		if res := compress(incompressible[0:length], "HUFFMAN", "LZ", rng); res == 0 {
			fmt.Println("Success")
		} else {
			fmt.Printf("Failure %v\n", res)
			sum += res
		}
	}

	if res := compressAfterWriteClose(values[0:65536]); res == 0 {
		fmt.Println("Success")
	} else {
		fmt.Printf("Failure %v\n", res)
		sum += res
	}

	if res := compressAfterReadClose(values[0:65536]); res == 0 {
		fmt.Println("Success")
	} else {
		fmt.Printf("Failure %v\n", res)
		sum += res
	}

	fmt.Println()

	if sum != 0 {
		b.Error()
	}
}

func compress(block []byte, entropy, transform string, rng *rand.Rand) int {
	jobs := uint(rng.Intn(4) + 1)
	var blockSize uint

	if n := rng.Intn(3); n == 1 {
		blockSize = uint(len(block))
	} else {
		blockSize = uint((len(block) / (n + 1)) & -16)
	}

	fmt.Printf("Block size: %v, transform %v, entropy: %v, jobs: %v \n", blockSize, transform, entropy, jobs)
	outputName := ""

	{
		// Create an io.WriteCloser
		output, err := os.CreateTemp("", "compressed-*.knz")

		if err != nil {
			fmt.Printf("%v\n", err)
			return 1
		}

		outputName = output.Name()
		defer os.Remove(outputName)

		// Create a Writer
		w, err2 := NewWriter(output, transform, entropy, blockSize, jobs, 32, 0, false)

		if err2 != nil {
			fmt.Printf("%v\n", err2)
			return 2
		}

		// Compress block
		_, err = w.Write(block)

		if err != nil {
			fmt.Printf("%v\n", err)
			return 3
		}

		// Close Writer
		err = w.Close()

		if err != nil {
			fmt.Printf("%v\n", err)
			return 4
		}
	}

	{
		// Create an io.ReadCloser
		input, err := os.Open(outputName)

		if err != nil {
			fmt.Printf("%v\n", err)
			return 5
		}

		// Create a Reader
		r, err2 := NewReader(input, 4)

		if err2 != nil {
			fmt.Printf("%v\n", err2)
			return 6
		}

		// Decompress block
		_, err = r.Read(block)

		if err != nil {
			fmt.Printf("%v\n", err)
			return 7
		}

		// Close Reader
		err = r.Close()

		if err != nil {
			fmt.Printf("%v\n", err)
			return 8
		}
	}

	// If we made it until here, the roundtrip is valid.
	// The checksum verification guarantees that the data
	// has been decompressed correctly.
	return 0
}

func compressAfterWriteClose(block []byte) int {
	fmt.Println("Test - write after close")
	buf := make([]byte, len(block))
	copy(buf, block)
	bs := internal.NewBufferStream()

	os, err := NewWriter(bs, "NONE", "HUFFMAN", uint(len(block)), 1, 0, 0, false)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}

	_, err = os.Write(block)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 2
	}

	if err = os.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return 3
	}

	_, err = os.Write(block)

	if err != nil {
		fmt.Printf("OK - expected error: %v\n", err)
		return 0
	}

	return 4
}

func compressAfterReadClose(block []byte) int {
	fmt.Println("Test - read after close")
	dst := &memoryWriteCloser{}

	os, err := NewWriter(dst, "NONE", "NONE", uint(len(block)), 1, 0, 0, false)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}

	_, err = os.Write(block)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 2
	}

	if err = os.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return 3
	}

	is, err := NewReader(stdio.NopCloser(bytes.NewReader(dst.data)), 1)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 4
	}

	_, err = is.Read(block)

	if err != nil {
		fmt.Printf("%v\n", err)
		return 5
	}

	if err = is.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return 6
	}

	_, err = is.Read(block)

	if err != nil {
		fmt.Printf("OK - expected error: %v\n", err)
		return 0
	}

	return 7
}

type failingReadCloser struct {
	data   []byte
	offset int
	failed bool
}

type memoryWriteCloser struct {
	data   []byte
	closed bool
}

type failingWriteCloser struct {
	data     []byte
	failures int
}

type trackingReadCloser struct {
	reader *bytes.Reader
	closed bool
}

func (this *memoryWriteCloser) Write(buf []byte) (int, error) {
	this.data = append(this.data, buf...)
	return len(buf), nil
}

func (this *memoryWriteCloser) Close() error {
	this.closed = true
	return nil
}

func (this *failingWriteCloser) Write(buf []byte) (int, error) {
	if this.failures > 0 {
		this.failures--
		return 0, errors.New("temporary write failure")
	}

	this.data = append(this.data, buf...)
	return len(buf), nil
}

func (this *failingWriteCloser) Close() error {
	return nil
}

func (this *failingReadCloser) Read(buf []byte) (int, error) {
	if this.failed == false {
		this.failed = true
		return 0, errors.New("temporary read failure")
	}

	if this.offset >= len(this.data) {
		return 0, stdio.EOF
	}

	n := copy(buf, this.data[this.offset:])
	this.offset += n
	return n, nil
}

func (this *failingReadCloser) Close() error {
	return nil
}

func (this *trackingReadCloser) Read(buf []byte) (int, error) {
	return this.reader.Read(buf)
}

func (this *trackingReadCloser) Close() error {
	this.closed = true
	return nil
}

func TestReaderReadHeaderRetriesAfterFailure(t *testing.T) {
	input := make([]byte, 1024)

	for i := range input {
		input[i] = byte(i)
	}

	dst := &memoryWriteCloser{}
	w, err := NewWriter(dst, "NONE", "NONE", uint(len(input)), 1, 0, 0, false)

	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	if _, err = w.Write(input); err != nil {
		t.Fatalf("write compressed stream: %v", err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	src := &failingReadCloser{data: append([]byte(nil), dst.data...)}
	r, err := NewReader(src, 1)

	if err != nil {
		t.Fatalf("create reader: %v", err)
	}

	buf := make([]byte, len(input))

	if _, err = r.Read(buf); err == nil {
		t.Fatal("expected first read to fail")
	}

	n, err := r.Read(buf)

	if err != nil {
		t.Fatalf("retry read failed: %v", err)
	}

	if n != len(input) {
		t.Fatalf("invalid decoded length: got %d, want %d", n, len(input))
	}

	if bytes.Equal(buf, input) == false {
		t.Fatal("decoded data mismatch after retry")
	}
}

func TestWriterCloseRetriesAfterFailure(t *testing.T) {
	input := make([]byte, 1024)

	for i := range input {
		input[i] = byte(i)
	}

	dst := &failingWriteCloser{failures: 1}
	w, err := NewWriter(dst, "NONE", "NONE", uint(len(input)), 1, 0, 0, false)

	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	if _, err = w.Write(input); err != nil {
		t.Fatalf("write compressed stream: %v", err)
	}

	if err = w.Close(); err == nil {
		t.Fatal("expected first close to fail")
	}

	if _, err = w.Write(input[:1]); err == nil {
		t.Fatal("expected writes to be rejected after close started")
	}

	if err = w.Close(); err != nil {
		t.Fatalf("retry close failed: %v", err)
	}

	r, err := NewReader(stdio.NopCloser(bytes.NewReader(dst.data)), 1)

	if err != nil {
		t.Fatalf("create reader: %v", err)
	}

	buf := make([]byte, len(input))
	n, err := r.Read(buf)

	if err != nil {
		t.Fatalf("read roundtrip data: %v", err)
	}

	if n != len(input) {
		t.Fatalf("invalid decoded length: got %d, want %d", n, len(input))
	}

	if bytes.Equal(buf, input) == false {
		t.Fatal("decoded data mismatch after close retry")
	}
}

func TestWriterCloseClosesWrappedStream(t *testing.T) {
	dst := &memoryWriteCloser{}
	w, err := NewWriter(dst, "NONE", "NONE", 1024, 1, 0, 0, false)

	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	if _, err = w.Write(make([]byte, 1024)); err != nil {
		t.Fatalf("write compressed stream: %v", err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	if dst.closed == false {
		t.Fatal("expected wrapped writer to be closed")
	}
}

func TestReaderCloseClosesWrappedStream(t *testing.T) {
	dst := &memoryWriteCloser{}
	w, err := NewWriter(dst, "NONE", "NONE", 1024, 1, 0, 0, false)

	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	if _, err = w.Write(make([]byte, 1024)); err != nil {
		t.Fatalf("write compressed stream: %v", err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	src := &trackingReadCloser{reader: bytes.NewReader(dst.data)}
	r, err := NewReader(src, 1)

	if err != nil {
		t.Fatalf("create reader: %v", err)
	}

	if err = r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	if src.closed == false {
		t.Fatal("expected wrapped reader to be closed")
	}
}
