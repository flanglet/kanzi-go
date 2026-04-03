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

package bitstream

import (
	"bytes"
	"errors"
	"fmt"
	stdio "io"
	"math/rand"
	"os"
	"testing"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_SUCCESS = "Success"
	_FAILURE = "Failure"
	_READ    = "Read: "
)

func TestBitStreamAligned(b *testing.T) {
	if err := testCorrectnessAligned1(); err != nil {
		b.Error(err)
	}

	if err := testCorrectnessAligned2(); err != nil {
		b.Error(err)
	}
}

func TestBitStreamMisaligned(b *testing.T) {
	if err := testCorrectnessMisaligned1(); err != nil {
		b.Error(err)
	}

	if err := testCorrectnessMisaligned2(); err != nil {
		b.Error(err)
	}
}

func testCorrectnessAligned1() error {
	fmt.Printf("Correctness Test - write long - byte aligned\n")
	values := make([]int, 100)

	// Check correctness of read() and written()
	for t := 1; t <= 32; t++ {
		bs := internal.NewBufferStream()
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		fmt.Println()
		obs.WriteBits(0x0123456789ABCDEF, uint(t))
		fmt.Printf("Written (before close): %v\n", obs.Written())
		obs.Close()
		fmt.Printf("Written (after close): %v\n", obs.Written())

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)
		dbgibs.ReadBits(uint(t))

		if dbgibs.Read() == uint64(t) {
			fmt.Println("\nOK")
		} else {
			fmt.Println("\nKO")
			return errors.New("Invalid number of bits read")
		}

		fmt.Printf("Read (before close): %v\n", dbgibs.Read())
		dbgibs.Close()
		fmt.Printf("Read (after close): %v\n", dbgibs.Read())
	}

	for test := 1; test <= 10; test++ {
		bs := internal.NewBufferStream(make([]byte, 0, 16384))
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)

		for i := range values {
			if test < 5 {
				values[i] = rand.Intn(test*1000 + 100)
			} else {
				values[i] = rand.Intn(1 << 31)
			}

			fmt.Printf("%v ", values[i])

			if i%20 == 19 {
				println()
			}
		}

		println()
		println()

		for i := range values {
			dbgobs.WriteBits(uint64(values[i]), 32)
		}

		// Close first to force flush()
		dbgobs.Close()

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)
		println()
		fmt.Println(_READ)
		ok := true

		for i := range values {
			x := dbgibs.ReadBits(32)
			fmt.Printf("%v", x)

			if int(x) == values[i] {
				fmt.Printf(" ")
			} else {
				fmt.Printf("* ")
				ok = false
			}

			if i%20 == 19 {
				println()
			}
		}

		dbgobs.Close()
		dbgibs.Close()
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgobs.Written())
		fmt.Printf("Bits read: %v\n", dbgibs.Read())
		println()

		if ok {
			fmt.Println(_SUCCESS)
		} else {
			fmt.Println(_FAILURE)
			return fmt.Errorf("Bits written: %v, bits read: %v", dbgobs.Written(), dbgibs.Read())
		}

		println()
		println()
	}

	return error(nil)
}

func testCorrectnessMisaligned1() error {
	fmt.Printf("Correctness Test - write long - not byte aligned\n")
	values := make([]int, 100)

	// Check correctness of read() and written()
	for t := 1; t <= 32; t++ {
		bs := internal.NewBufferStream(make([]byte, 16384))
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)
		fmt.Println()
		dbgobs.WriteBit(1)
		dbgobs.WriteBits(0x0123456789ABCDEF, uint(t))
		fmt.Printf("Written (before close): %v\n", obs.Written())
		dbgobs.Close()
		obs.Close()
		fmt.Printf("Written (after close): %v\n", obs.Written())

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)
		dbgibs.ReadBit()
		dbgibs.ReadBits(uint(t))

		if dbgibs.Read() == uint64(t+1) {
			fmt.Println("OK")
		} else {
			fmt.Println("KO")
			return errors.New("Invalid number of bits read")
		}

		dbgibs.Close()
		bs.Close()
	}

	for test := 1; test <= 10; test++ {
		bs := internal.NewBufferStream()
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)

		for i := range values {
			if test < 5 {
				values[i] = rand.Intn(test*1000 + 100)
			} else {
				values[i] = rand.Intn(1 << 31)
			}

			mask := (1 << (1 + uint(i&63))) - 1
			values[i] &= mask
			fmt.Printf("%v ", values[i])

			if i%20 == 19 {
				println()
			}
		}

		println()
		println()

		for i := range values {
			dbgobs.WriteBits(uint64(values[i]), 1+uint(i&63))
		}

		// Close first to force flush()
		dbgobs.Close()
		obs.Close()
		testWritePostClose(dbgobs)

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)
		println()
		fmt.Println(_READ)
		ok := true

		for i := range values {
			x := dbgibs.ReadBits(1 + uint(i&63))
			fmt.Printf("%v", x)

			if int(x) == values[i] {
				fmt.Printf(" ")
			} else {
				fmt.Printf("* ")
				ok = false
			}

			if i%20 == 19 {
				println()
			}
		}

		dbgibs.Close()
		testReadPostClose(dbgibs)
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgobs.Written())
		fmt.Printf("Bits read: %v\n", dbgibs.Read())
		println()

		if ok {
			fmt.Println(_SUCCESS)
		} else {
			fmt.Println(_FAILURE)
			return fmt.Errorf("Bits written: %v, bits read: %v", dbgobs.Written(), dbgibs.Read())
		}

		println()
		println()
	}

	return error(nil)
}

func testCorrectnessAligned2() error {
	fmt.Printf("Correctness Test - write array - byte aligned\n")
	input := make([]byte, 100)
	output := make([]byte, 100)

	for test := 1; test <= 10; test++ {
		bs := internal.NewBufferStream()
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)
		println()

		for i := range input {
			if test < 5 {
				input[i] = byte(rand.Intn(test*1000 + 100))
			} else {
				input[i] = byte(rand.Intn(1 << 31))
			}

			fmt.Printf("%v ", input[i])

			if i%20 == 19 {
				println()
			}
		}

		count := uint(8 + test*(20+(test&1)) + (test & 3))
		println()
		println()
		dbgobs.WriteArray(input, count)

		// Close first to force flush()
		dbgobs.Close()

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)

		println()
		fmt.Println(_READ)
		r := dbgibs.ReadArray(output, count)
		ok := r == count

		if ok == true {
			for i := 0; i < int(r>>3); i++ {
				fmt.Printf("%v", output[i])

				if output[i] == input[i] {
					fmt.Printf(" ")
				} else {
					fmt.Printf("* ")
					ok = false
				}

				if i%20 == 19 {
					println()
				}
			}
		}

		dbgibs.Close()
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgobs.Written())
		fmt.Printf("Bits read: %v\n", dbgibs.Read())

		if ok {
			fmt.Printf("\nSuccess\n")
		} else {
			fmt.Printf("\nFailure\n")
			return fmt.Errorf("Bits written: %v, bits read: %v", dbgobs.Written(), dbgibs.Read())
		}

		println()
		println()
	}

	return error(nil)
}

func testCorrectnessMisaligned2() error {
	fmt.Printf("Correctness Test - write array - not byte aligned\n")
	input := make([]byte, 100)
	output := make([]byte, 100)

	for test := 1; test <= 10; test++ {
		bs := internal.NewBufferStream()
		obs, _ := NewDefaultOutputBitStream(bs, 16384)
		dbgobs, _ := NewDebugOutputBitStream(obs, os.Stdout)
		dbgobs.ShowByte(true)
		dbgobs.Mark(true)
		println()

		for i := range input {
			if test < 5 {
				input[i] = byte(rand.Intn(test*1000 + 100))
			} else {
				input[i] = byte(rand.Intn(1 << 31))
			}

			fmt.Printf("%v ", input[i])

			if i%20 == 19 {
				println()
			}
		}

		count := uint(8 + test*(20+(test&1)) + (test & 3))
		println()
		println()
		dbgobs.WriteBit(0)
		dbgobs.WriteArray(input[1:], count)

		// Close first to force flush()
		dbgobs.Close()

		ibs, _ := NewDefaultInputBitStream(bs, 16384)
		dbgibs, _ := NewDebugInputBitStream(ibs, os.Stdout)
		dbgibs.ShowByte(true)
		dbgibs.Mark(true)

		println()
		fmt.Println(_READ)
		dbgibs.ReadBit()
		r := dbgibs.ReadArray(output[1:], count)
		ok := r == count

		if ok == true {
			for i := 1; i < 1+int(r>>3); i++ {
				fmt.Printf("%v", output[i])

				if output[i] == input[i] {
					fmt.Printf(" ")
				} else {
					fmt.Printf("* ")
					ok = false
				}

				if i%20 == 19 {
					println()
				}
			}
		}

		dbgibs.Close()
		bs.Close()
		println()
		println()
		fmt.Printf("Bits written: %v\n", dbgobs.Written())
		fmt.Printf("Bits read: %v\n", dbgibs.Read())

		if ok {
			fmt.Printf("\nSuccess\n")
		} else {
			fmt.Printf("\nFailure\n")
			return fmt.Errorf("Bits written: %v, bits read: %v", dbgobs.Written(), dbgibs.Read())
		}

		println()
		println()
	}

	return error(nil)
}

func testWritePostClose(obs kanzi.OutputBitStream) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Error: %v\n", r.(error).Error())
		}
	}()

	fmt.Printf("\nTrying to write to closed stream\n")
	obs.WriteBit(1)
}

func testReadPostClose(ibs kanzi.InputBitStream) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Error: %v\n", r.(error).Error())
		}
	}()

	fmt.Printf("\nTrying to read from closed stream\n")
	ibs.ReadBit()
}

func TestBitStreamWriteArrayPartialTail(t *testing.T) {
	tests := []struct {
		name    string
		prefix  int
		src     byte
		count   uint
		want    uint64
		wantLen uint
	}{
		{name: "aligned", prefix: -1, src: 0xAA, count: 1, want: 0x1, wantLen: 1},
		{name: "misaligned", prefix: 0, src: 0xA0, count: 3, want: 0x5, wantLen: 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bs := internal.NewBufferStream()
			obs, err := NewDefaultOutputBitStream(bs, 1024)
			if err != nil {
				t.Fatalf("create output bitstream: %v", err)
			}

			if tc.prefix >= 0 {
				obs.WriteBit(tc.prefix)
			}

			if got := obs.WriteArray([]byte{tc.src}, tc.count); got != tc.count {
				t.Fatalf("WriteArray()=%d, want %d", got, tc.count)
			}

			if err := obs.Close(); err != nil {
				t.Fatalf("close output bitstream: %v", err)
			}

			ibs, err := NewDefaultInputBitStream(bs, 1024)
			if err != nil {
				t.Fatalf("create input bitstream: %v", err)
			}

			if got := ibs.ReadBits(tc.wantLen); got != tc.want {
				t.Fatalf("ReadBits()=%b, want %b", got, tc.want)
			}

			if err := ibs.Close(); err != nil {
				t.Fatalf("close input bitstream: %v", err)
			}

			if err := bs.Close(); err != nil {
				t.Fatalf("close buffer stream: %v", err)
			}

		})
	}
}

func TestDebugBitStreamArrayPartialTail(t *testing.T) {
	bs := internal.NewBufferStream()
	obs, err := NewDefaultOutputBitStream(bs, 1024)

	if err != nil {
		t.Fatalf("create output bitstream: %v", err)
	}

	var out bytes.Buffer
	dbgobs, err := NewDebugOutputBitStream(obs, &out)

	if err != nil {
		t.Fatalf("create debug output bitstream: %v", err)
	}

	dbgobs.Mark(true)

	if got := dbgobs.WriteArray([]byte{0xA0}, 3); got != 3 {
		t.Fatalf("WriteArray()=%d, want 3", got)
	}

	if out.String() != "101w" {
		t.Fatalf("invalid debug output log: got %q, want %q", out.String(), "101w")
	}

	if err = dbgobs.Close(); err != nil {
		t.Fatalf("close debug output bitstream: %v", err)
	}

	ibs, err := NewDefaultInputBitStream(bs, 1024)

	if err != nil {
		t.Fatalf("create input bitstream: %v", err)
	}

	var in bytes.Buffer
	dbgibs, err := NewDebugInputBitStream(ibs, &in)

	if err != nil {
		t.Fatalf("create debug input bitstream: %v", err)
	}

	dbgibs.Mark(true)
	dst := make([]byte, 1)

	if got := dbgibs.ReadArray(dst, 3); got != 3 {
		t.Fatalf("ReadArray()=%d, want 3", got)
	}

	if dst[0] != 0xA0 {
		t.Fatalf("invalid decoded byte: got 0x%02x, want 0xa0", dst[0])
	}

	if in.String() != "101r" {
		t.Fatalf("invalid debug input log: got %q, want %q", in.String(), "101r")
	}
}

type partialReadCloser struct {
	data []byte
	read bool
	err  error
}

func (this *partialReadCloser) Read(buf []byte) (int, error) {
	if this.read == true {
		return 0, this.err
	}

	this.read = true
	copy(buf, this.data)
	return len(this.data), this.err
}

func (this *partialReadCloser) Close() error {
	return nil
}

func TestInputBitStreamDefersReadErrorUntilBufferedBytesAreConsumed(t *testing.T) {
	ibs, err := NewDefaultInputBitStream(&partialReadCloser{data: []byte{0x80}, err: stdio.EOF}, 1024)

	if err != nil {
		t.Fatalf("create input bitstream: %v", err)
	}

	if got := ibs.ReadBit(); got != 1 {
		t.Fatalf("ReadBit()=%d, want 1", got)
	}

	for i := 0; i < 7; i++ {
		if got := ibs.ReadBit(); got != 0 {
			t.Fatalf("ReadBit()=%d, want 0", got)
		}
	}

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok == true && errors.Is(err, stdio.EOF) {
				return
			}

			t.Fatalf("unexpected panic: %v", r)
		}

		t.Fatal("expected EOF panic")
	}()

	ibs.ReadBit()
}

func TestInputBitStreamHasMoreToReadDefersEOFUntilBufferedBytesAreConsumed(t *testing.T) {
	ibs, err := NewDefaultInputBitStream(&partialReadCloser{data: []byte{0x80}, err: stdio.EOF}, 1024)

	if err != nil {
		t.Fatalf("create input bitstream: %v", err)
	}

	more, err := ibs.HasMoreToRead()

	if more != true || err != nil {
		t.Fatalf("HasMoreToRead()=(%v,%v), want (true,<nil>)", more, err)
	}

	if got := ibs.ReadBit(); got != 1 {
		t.Fatalf("ReadBit()=%d, want 1", got)
	}

	for i := 0; i < 6; i++ {
		more, err = ibs.HasMoreToRead()

		if more != true || err != nil {
			t.Fatalf("HasMoreToRead()=(%v,%v), want (true,<nil>)", more, err)
		}

		if got := ibs.ReadBit(); got != 0 {
			t.Fatalf("ReadBit()=%d, want 0", got)
		}
	}

	more, err = ibs.HasMoreToRead()

	if more != true || err != nil {
		t.Fatalf("HasMoreToRead()=(%v,%v), want (true,<nil>)", more, err)
	}

	if got := ibs.ReadBit(); got != 0 {
		t.Fatalf("ReadBit()=%d, want 0", got)
	}

	more, err = ibs.HasMoreToRead()

	if more != false || errors.Is(err, stdio.EOF) == false {
		t.Fatalf("HasMoreToRead()=(%v,%v), want (false,EOF)", more, err)
	}
}
