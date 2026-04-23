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

package main

import (
	"bytes"
	"errors"
	stdio "io"
	"os"
	"path/filepath"
	"testing"
	"time"

	kio "github.com/flanglet/kanzi-go/v2/io"
)

func TestOpenOutputFileNoOverwriteRejectsDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.knz")
	link := filepath.Join(dir, "link.knz")

	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	output, err := openOutputFile(link, _COMP_STDIN, false)

	if output != nil {
		output.Close()
	}

	if errors.Is(err, errOutputExists) == false {
		t.Fatalf("openOutputFile() error=%v, want errOutputExists", err)
	}

	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) == false {
		t.Fatalf("dangling symlink target was created or stat failed unexpectedly: %v", err)
	}
}

func TestOpenOutputFileOverwriteRejectsOutputSymlinkToInput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	link := filepath.Join(dir, "output")
	original := []byte("original")

	if err := os.WriteFile(input, original, 0600); err != nil {
		t.Fatalf("create input file: %v", err)
	}

	if err := os.Symlink(input, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	output, err := openOutputFile(link, input, true)

	if output != nil {
		output.Close()
	}

	if errors.Is(err, errSameInputOutput) == false {
		t.Fatalf("openOutputFile() error=%v, want errSameInputOutput", err)
	}

	data, err := os.ReadFile(input)

	if err != nil {
		t.Fatalf("read input file: %v", err)
	}

	if bytes.Equal(data, original) == false {
		t.Fatal("input file was modified")
	}
}

func TestFileCompressTaskReadsShortReadsUntilEOF(t *testing.T) {
	originalStdin := os.Stdin
	reader, writer, err := os.Pipe()

	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	os.Stdin = reader
	defer func() {
		os.Stdin = originalStdin
		reader.Close()
	}()

	output, err := os.CreateTemp("", "kanzi-short-read-*.knz")

	if err != nil {
		t.Fatalf("create output file: %v", err)
	}

	outputName := output.Name()
	output.Close()
	defer os.Remove(outputName)

	first := bytes.Repeat([]byte("a"), 2048)
	second := bytes.Repeat([]byte("b"), 2048)
	expected := append(append([]byte(nil), first...), second...)
	writeDone := make(chan error, 1)

	go func() {
		if _, err := writer.Write(first); err != nil {
			writer.Close()
			writeDone <- err
			return
		}

		time.Sleep(50 * time.Millisecond)

		if _, err := writer.Write(second); err != nil {
			writer.Close()
			writeDone <- err
			return
		}

		writeDone <- writer.Close()
	}()

	ctx := map[string]any{
		"remove":     false,
		"verbosity":  uint(0),
		"inputName":  _COMP_STDIN,
		"outputName": outputName,
		"overwrite":  true,
		"skipBlocks": false,
		"checksum":   uint(32),
		"entropy":    "NONE",
		"transform":  "NONE",
		"blockSize":  uint(1024),
		"jobs":       uint(1),
	}

	task := fileCompressTask{ctx: ctx}
	code, _, _, err := task.call()

	if code != 0 || err != nil {
		t.Fatalf("compress stdin: code=%d err=%v", code, err)
	}

	if err := <-writeDone; err != nil {
		t.Fatalf("write pipe data: %v", err)
	}

	input, err := os.Open(outputName)

	if err != nil {
		t.Fatalf("open compressed output: %v", err)
	}

	defer input.Close()
	cis, err := kio.NewReader(input, 1)

	if err != nil {
		t.Fatalf("create reader: %v", err)
	}

	defer cis.Close()
	actual := make([]byte, len(expected))
	n, err := stdio.ReadFull(cis, actual)

	if err != nil {
		t.Fatalf("read decompressed data: n=%d err=%v", n, err)
	}

	if bytes.Equal(actual, expected) == false {
		t.Fatal("decompressed data does not include all short reads")
	}
}
