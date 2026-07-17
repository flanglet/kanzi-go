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
	"fmt"
	stdio "io"
	"math/bits"
	"testing"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/bitstream"
	"github.com/flanglet/kanzi-go/v2/entropy"
	"github.com/flanglet/kanzi-go/v2/hash"
	"github.com/flanglet/kanzi-go/v2/transform"
)

func checksumSizeBitsToToken(bits uint) uint64 {
	switch bits {
	case 32:
		return 1
	case 64:
		return 2
	default:
		return 0
	}
}

func computeHeaderChecksumFixture(bsVersion int, checksumSize uint64, entropyType uint32,
	transformType uint64, blockSize int, szMask uint, outputSize uint64) uint32 {
	seed := uint32(bsVersion)

	if bsVersion >= 6 {
		seed = uint32(0x01030507 * bsVersion)
	}

	hashValue := uint32(0x1E35A7BD)
	checksum := hashValue * seed
	mixFn := mix32v6

	if bsVersion >= 7 {
		mixFn = mix32v7
	}

	if bsVersion >= 6 {
		checksum = mixFn(checksum, hashValue, uint32(checksumSize))
	}

	checksum = mixFn(checksum, hashValue, entropyType)
	checksum = mixFn(checksum, hashValue, uint32(transformType>>32))
	checksum = mixFn(checksum, hashValue, uint32(transformType))
	checksum = mixFn(checksum, hashValue, uint32(blockSize))

	if szMask != 0 {
		checksum = mixFn(checksum, hashValue, uint32(outputSize>>32))
		checksum = mixFn(checksum, hashValue, uint32(outputSize))
	}

	return (checksum >> 23) ^ (checksum >> 3)
}

func computeBlockHeaderChecksumFixture(mode byte, skipFlags byte, length uint32) uint32 {
	hashValue := uint32(0x1E35A7BD)
	checksum := hashValue * uint32(0x01030507)
	checksum = mix32v7(checksum, hashValue, uint32(mode))
	checksum = mix32v7(checksum, hashValue, uint32(skipFlags))
	checksum = mix32v7(checksum, hashValue, length)
	return (checksum >> 23) ^ (checksum >> 3)
}

func buildHeaderFixture(t *testing.T, fileType uint32, bsVersion int, checksumSize uint64,
	entropyType uint32, transformType uint64, blockSize int, szMask uint, outputSize uint64,
	validChecksum bool) []byte {
	t.Helper()
	dst := &memoryWriteCloser{}
	obs, err := bitstream.NewDefaultOutputBitStream(dst, 16384)

	if err != nil {
		t.Fatalf("create header bitstream: %v", err)
	}

	obs.WriteBits(uint64(fileType), 32)
	obs.WriteBits(uint64(bsVersion), 4)

	if bsVersion >= 6 {
		obs.WriteBits(checksumSize, 2)
	} else {
		if checksumSize == 0 {
			obs.WriteBit(0)
		} else {
			obs.WriteBit(1)
		}
	}

	obs.WriteBits(uint64(entropyType), 5)
	obs.WriteBits(transformType, 48)
	obs.WriteBits(uint64(blockSize>>4), 28)
	obs.WriteBits(uint64(szMask), 2)

	if szMask != 0 {
		obs.WriteBits(outputSize, 16*szMask)
	}

	if bsVersion >= 6 {
		obs.WriteBits(0, 15)
	}

	crcSize := uint(16)

	if bsVersion >= 6 {
		crcSize = 24
	}

	checksum := computeHeaderChecksumFixture(bsVersion, checksumSize, entropyType,
		transformType, blockSize, szMask, outputSize)

	if validChecksum == false {
		checksum ^= 1
	}

	obs.WriteBits(uint64(checksum), crcSize)

	if err = obs.Close(); err != nil {
		t.Fatalf("close header bitstream: %v", err)
	}

	return append([]byte(nil), dst.data...)
}

func buildBlockPayloadFixture(t *testing.T, mode byte, encodedSkipFlags *byte, headerSkipFlags byte,
	preTransformLength int, checksumBits uint, checksumValue uint64, payload []byte,
	includeHeaderChecksum bool, validHeaderChecksum bool) []byte {
	t.Helper()
	dst := &memoryWriteCloser{}
	obs, err := bitstream.NewDefaultOutputBitStream(dst, 16384)

	if err != nil {
		t.Fatalf("create block bitstream: %v", err)
	}

	obs.WriteBits(uint64(mode), 8)

	if encodedSkipFlags != nil {
		obs.WriteBits(uint64(*encodedSkipFlags), 8)
	}

	dataSize := uint(1 + ((mode >> 5) & 0x03))
	obs.WriteBits(uint64(preTransformLength), 8*dataSize)

	if includeHeaderChecksum {
		checksum := computeBlockHeaderChecksumFixture(mode, headerSkipFlags, uint32(preTransformLength))

		if validHeaderChecksum == false {
			checksum ^= 1
		}

		obs.WriteBits(uint64(checksum&0xFF), 8)
	}

	if checksumBits == 32 {
		obs.WriteBits(checksumValue, 32)
	} else if checksumBits == 64 {
		obs.WriteBits(checksumValue, 64)
	}

	if len(payload) > 0 {
		obs.WriteArray(payload, uint(len(payload)<<3))
	}

	if err = obs.Close(); err != nil {
		t.Fatalf("close block bitstream: %v", err)
	}

	return append([]byte(nil), dst.data...)
}

func buildStreamFixture(t *testing.T, header []byte, blocks ...[]byte) []byte {
	t.Helper()
	dst := &memoryWriteCloser{}
	obs, err := bitstream.NewDefaultOutputBitStream(dst, 16384)

	if err != nil {
		t.Fatalf("create stream bitstream: %v", err)
	}

	obs.WriteArray(header, uint(len(header)<<3))

	for _, block := range blocks {
		blockBits := uint64(len(block) << 3)
		lw := uint(3)

		if blockBits >= 8 {
			lw = uint(bits.Len64(blockBits>>3) + 3)
		}

		obs.WriteBits(uint64(lw-3), 5)
		obs.WriteBits(blockBits, lw)
		obs.WriteArray(block, uint(blockBits))
	}

	// Terminator block
	obs.WriteBits(0, 5)
	obs.WriteBits(0, 3)

	if err = obs.Close(); err != nil {
		t.Fatalf("close stream bitstream: %v", err)
	}

	return append([]byte(nil), dst.data...)
}

func buildCopyBlockStreamFixture(t *testing.T, bsVersion int, payload []byte, checksumBits uint,
	checksumValue uint64, validBlockHeaderChecksum bool) []byte {
	t.Helper()
	transformType, err := transform.GetType("NONE")

	if err != nil {
		t.Fatalf("get NONE transform type: %v", err)
	}

	dataSize := uint(1)

	if len(payload) >= 256 {
		dataSize = uint(bits.Len(uint(len(payload)-1))/8 + 1)
	}

	mode := byte(_COPY_BLOCK_MASK | byte((dataSize-1)<<5))
	header := buildHeaderFixture(t, _BITSTREAM_TYPE, bsVersion, checksumSizeBitsToToken(checksumBits),
		entropy.NONE_TYPE, transformType, 1024, 0, 0, true)
	block := buildBlockPayloadFixture(t, mode, nil, 0, len(payload), checksumBits, checksumValue,
		payload, bsVersion >= 7, validBlockHeaderChecksum)
	return buildStreamFixture(t, header, block)
}

func buildTransformedCopyStreamFixture(t *testing.T, payload []byte, checksumBits uint,
	checksumValue uint64) []byte {
	t.Helper()
	transformType, err := transform.GetType("NONE")

	if err != nil {
		t.Fatalf("get NONE transform type: %v", err)
	}

	dataSize := uint(1)

	if len(payload) >= 256 {
		dataSize = uint(bits.Len(uint(len(payload)-1))/8 + 1)
	}

	mode := byte(_COPY_BLOCK_MASK | _TRANSFORMS_MASK | byte((dataSize-1)<<5))
	headerSkipFlags := byte((mode << 4) | 0x0F)
	header := buildHeaderFixture(t, _BITSTREAM_TYPE, 7, checksumSizeBitsToToken(checksumBits),
		entropy.NONE_TYPE, transformType, 1024, 0, 0, true)
	block := buildBlockPayloadFixture(t, mode, nil, headerSkipFlags, len(payload), checksumBits,
		checksumValue, payload, true, true)
	return buildStreamFixture(t, header, block)
}

func readFirstBlockModeFixture(t *testing.T, data []byte) byte {
	t.Helper()
	ibs, err := bitstream.NewDefaultInputBitStream(stdio.NopCloser(bytes.NewReader(data)), 16384)

	if err != nil {
		t.Fatalf("create input bitstream: %v", err)
	}

	defer ibs.Close()

	if got := ibs.ReadBits(32); got != _BITSTREAM_TYPE {
		t.Fatalf("invalid stream type: got 0x%x", got)
	}

	bsVersion := uint(ibs.ReadBits(4))

	if bsVersion >= 6 {
		ibs.ReadBits(2)
	} else {
		ibs.ReadBit()
	}

	ibs.ReadBits(5)
	ibs.ReadBits(48)
	ibs.ReadBits(28)
	szMask := uint(ibs.ReadBits(2))

	if szMask != 0 {
		ibs.ReadBits(16 * szMask)
	}

	crcSize := uint(16)

	if bsVersion >= 6 {
		ibs.ReadBits(15)
		crcSize = 24
	}

	ibs.ReadBits(crcSize)
	lw := uint(ibs.ReadBits(5)) + 3

	if blockBits := ibs.ReadBits(lw); blockBits == 0 {
		t.Fatal("missing first block")
	}

	return byte(ibs.ReadBits(8))
}

func expectReadIOError(t *testing.T, data []byte, expectedCode int, expectedMsg string) {
	t.Helper()
	r, err := NewReader(stdio.NopCloser(bytes.NewReader(data)), 1)

	if err != nil {
		t.Fatalf("create reader: %v", err)
	}

	defer r.Close()
	buf := make([]byte, 1)
	_, err = r.Read(buf)

	if err == nil {
		t.Fatal("expected read error")
	}

	ioErr, ok := err.(*IOError)

	if ok == false {
		t.Fatalf("unexpected error type: %T (%v)", err, err)
	}

	if ioErr.ErrorCode() != expectedCode {
		t.Fatalf("unexpected error code: got %d want %d (%v)", ioErr.ErrorCode(), expectedCode, ioErr)
	}

	if expectedMsg != "" && bytes.Contains([]byte(ioErr.Error()), []byte(expectedMsg)) == false {
		t.Fatalf("unexpected error message: %v", ioErr)
	}
}

func compressToBytes(t *testing.T, input []byte, transformName, entropyName string, blockSize, jobs, checksum uint,
	headerless bool) []byte {
	t.Helper()
	dst := &memoryWriteCloser{}
	w, err := NewWriter(dst, transformName, entropyName, blockSize, jobs, checksum, int64(len(input)), headerless)

	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	if _, err = w.Write(input); err != nil {
		t.Fatalf("write input: %v", err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	return append([]byte(nil), dst.data...)
}

func TestMalformedStream(t *testing.T) {
	entropyType := entropy.ANS0_TYPE
	transformType := uint64(transform.LZ_TYPE) << 42

	cases := []struct {
		name string
		data []byte
		code int
		msg  string
	}{
		{
			name: "invalid type",
			data: buildStreamFixture(t, buildHeaderFixture(t, _BITSTREAM_TYPE^1, 7, 0, entropyType,
				transformType, 1024, 0, 0, true)),
			code: kanzi.ERR_INVALID_FILE,
			msg:  "Invalid stream type",
		},
		{
			name: "unsupported version",
			data: buildStreamFixture(t, buildHeaderFixture(t, _BITSTREAM_TYPE, 8, 0, entropyType,
				transformType, 1024, 0, 0, true)),
			code: kanzi.ERR_STREAM_VERSION,
			msg:  "cannot read this version",
		},
		{
			name: "invalid checksum size",
			data: buildStreamFixture(t, buildHeaderFixture(t, _BITSTREAM_TYPE, 7, 3, entropyType,
				transformType, 1024, 0, 0, true)),
			code: kanzi.ERR_INVALID_CODEC,
			msg:  "incorrect checksum size",
		},
		{
			name: "unknown entropy type",
			data: buildStreamFixture(t, buildHeaderFixture(t, _BITSTREAM_TYPE, 7, 0, 31,
				transformType, 1024, 0, 0, true)),
			code: kanzi.ERR_INVALID_CODEC,
			msg:  "incorrect entropy type",
		},
		{
			name: "unknown transform type",
			data: buildStreamFixture(t, buildHeaderFixture(t, _BITSTREAM_TYPE, 7, 0, entropyType,
				uint64(63)<<42, 1024, 0, 0, true)),
			code: kanzi.ERR_INVALID_CODEC,
			msg:  "incorrect transform type",
		},
		{
			name: "invalid block size",
			data: buildStreamFixture(t, buildHeaderFixture(t, _BITSTREAM_TYPE, 7, 0, entropyType,
				transformType, 1008, 0, 0, true)),
			code: kanzi.ERR_BLOCK_SIZE,
			msg:  "incorrect block size",
		},
		{
			name: "header checksum mismatch",
			data: buildStreamFixture(t, buildHeaderFixture(t, _BITSTREAM_TYPE, 7, 0, entropyType,
				transformType, 1024, 0, 0, false)),
			code: kanzi.ERR_CRC_CHECK,
			msg:  "checksum mismatch",
		},
		{
			name: "zero pre-transform length",
			data: func() []byte {
				header := buildHeaderFixture(t, _BITSTREAM_TYPE, 7, 0, entropy.NONE_TYPE,
					uint64(transform.NONE_TYPE)<<42, 1024, 0, 0, true)
				mode := byte(0)
				block := buildBlockPayloadFixture(t, mode, nil, 0x0F, 0, 0, 0, nil, true, true)
				return buildStreamFixture(t, header, block)
			}(),
			code: kanzi.ERR_BLOCK_SIZE,
			msg:  "Invalid compressed block size",
		},
		{
			name: "block header checksum mismatch",
			data: func() []byte {
				header := buildHeaderFixture(t, _BITSTREAM_TYPE, 7, 0, entropy.NONE_TYPE,
					uint64(transform.NONE_TYPE)<<42, 1024, 0, 0, true)
				mode := byte(_COPY_BLOCK_MASK)
				payload := []byte{0x2A}
				block := buildBlockPayloadFixture(t, mode, nil, 0, len(payload), 0, 0, payload, true, false)
				return buildStreamFixture(t, header, block)
			}(),
			code: kanzi.ERR_CRC_CHECK,
			msg:  "block header checksum mismatch",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectReadIOError(t, tc.data, tc.code, tc.msg)
		})
	}
}

func TestReaderAcceptsLegacyV6Stream(t *testing.T) {
	payload := []byte("legacy-v6-stream")
	stream := buildCopyBlockStreamFixture(t, 6, payload, 0, 0, true)
	r, err := NewReader(stdio.NopCloser(bytes.NewReader(stream)), 1)

	if err != nil {
		t.Fatalf("create reader: %v", err)
	}

	defer r.Close()
	dst := make([]byte, len(payload))
	n, err := r.Read(dst)

	if err != nil {
		t.Fatalf("read legacy stream: %v", err)
	}

	if n != len(payload) {
		t.Fatalf("unexpected decoded length: got %d want %d", n, len(payload))
	}

	if bytes.Equal(dst, payload) == false {
		t.Fatal("decoded legacy payload mismatch")
	}
}

func TestHeaderlessRoundTrip(t *testing.T) {
	input := bytes.Repeat([]byte("headerless-roundtrip-"), 64)
	compressed := compressToBytes(t, input, "LZ", "ANS0", 1024, 1, 32, true)
	r, err := NewHeaderlessReader(stdio.NopCloser(bytes.NewReader(compressed)), 1, "LZ", "ANS0", 1024, 32, int64(len(input)), _BITSTREAM_FORMAT_VERSION)

	if err != nil {
		t.Fatalf("create headerless reader: %v", err)
	}

	defer r.Close()
	dst := make([]byte, len(input))
	n, err := r.Read(dst)

	if err != nil {
		t.Fatalf("read headerless stream: %v", err)
	}

	if n != len(input) {
		t.Fatalf("unexpected decoded length: got %d want %d", n, len(input))
	}

	if bytes.Equal(dst, input) == false {
		t.Fatal("headerless roundtrip mismatch")
	}
}

func TestChecksumModes(t *testing.T) {
	input := bytes.Repeat([]byte("checksum-modes-"), 128)

	for _, checksumBits := range []uint{0, 32, 64} {
		t.Run(fmt.Sprintf("checksum-%d", checksumBits), func(t *testing.T) {
			compressed := compressToBytes(t, input, "LZ", "ANS0", 1024, 1, checksumBits, false)
			r, err := NewReader(stdio.NopCloser(bytes.NewReader(compressed)), 1)

			if err != nil {
				t.Fatalf("create reader: %v", err)
			}

			defer r.Close()
			dst := make([]byte, len(input))
			n, err := r.Read(dst)

			if err != nil {
				t.Fatalf("read stream: %v", err)
			}

			if n != len(input) {
				t.Fatalf("unexpected decoded length: got %d want %d", n, len(input))
			}

			if bytes.Equal(dst, input) == false {
				t.Fatal("checksum mode roundtrip mismatch")
			}
		})
	}
}

func TestLargeReadRequest(t *testing.T) {
	input := []byte("large-read-request")
	compressed := compressToBytes(t, input, "NONE", "HUFFMAN", 1024, 1, 0, false)
	r, err := NewReader(stdio.NopCloser(bytes.NewReader(compressed)), 1)

	if err != nil {
		t.Fatalf("create reader: %v", err)
	}

	defer r.Close()
	dst := make([]byte, len(input)*64)
	n, err := r.Read(dst)

	if err != nil {
		t.Fatalf("large read failed: %v", err)
	}

	if n != len(input) {
		t.Fatalf("unexpected decoded length: got %d want %d", n, len(input))
	}

	if bytes.Equal(dst[:n], input) == false {
		t.Fatal("large read decoded payload mismatch")
	}
}

func TestPayloadChecksumMismatch(t *testing.T) {
	payload := bytes.Repeat([]byte{0xA5}, 64)
	hasher, err := hash.NewXXHash32(_BITSTREAM_TYPE)

	if err != nil {
		t.Fatalf("create hasher: %v", err)
	}

	stream := buildCopyBlockStreamFixture(t, 7, payload, 32, uint64(hasher.Hash(payload)^1), true)
	expectReadIOError(t, stream, kanzi.ERR_CRC_CHECK, "Corrupted bitstream")
}

func TestTransformedCopyBlockFixture(t *testing.T) {
	payload := bytes.Repeat([]byte("TC"), 32)
	stream := buildTransformedCopyStreamFixture(t, payload, 0, 0)
	r, err := NewReader(stdio.NopCloser(bytes.NewReader(stream)), 1)

	if err != nil {
		t.Fatalf("create reader: %v", err)
	}

	defer r.Close()
	dst := make([]byte, len(payload))
	n, err := r.Read(dst)

	if err != nil {
		t.Fatalf("read transformed-copy fixture: %v", err)
	}

	if n != len(payload) {
		t.Fatalf("unexpected decoded length: got %d want %d", n, len(payload))
	}

	if bytes.Equal(dst, payload) == false {
		t.Fatal("transformed-copy fixture mismatch")
	}
}

func TestWriterUsesTransformedCopyFallback(t *testing.T) {
	input := []byte("0123456789ABCDEF")
	compressed := compressToBytes(t, input, "NONE", "HUFFMAN", 1024, 1, 0, false)
	mode := readFirstBlockModeFixture(t, compressed)

	if (mode&_COPY_BLOCK_MASK) == 0 || (mode&_TRANSFORMS_MASK) == 0 {
		t.Fatalf("expected transformed-copy block mode, got 0x%02X", mode)
	}

	r, err := NewReader(stdio.NopCloser(bytes.NewReader(compressed)), 1)

	if err != nil {
		t.Fatalf("create reader: %v", err)
	}

	defer r.Close()
	dst := make([]byte, len(input))
	n, err := r.Read(dst)

	if err != nil {
		t.Fatalf("read transformed-copy fallback stream: %v", err)
	}

	if n != len(input) {
		t.Fatalf("unexpected decoded length: got %d want %d", n, len(input))
	}

	if bytes.Equal(dst, input) == false {
		t.Fatal("transformed-copy fallback mismatch")
	}
}
