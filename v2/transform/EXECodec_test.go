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

package transform

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func writeInt16LE(buf []byte, value int) {
	buf[0] = byte(value)
	buf[1] = byte(value >> 8)
}

func writeInt32LE(buf []byte, value int) {
	buf[0] = byte(value)
	buf[1] = byte(value >> 8)
	buf[2] = byte(value >> 16)
	buf[3] = byte(value >> 24)
}

func createPEBlock(arch int) []byte {
	const (
		size      = 8192
		codeStart = 512
		codeLen   = 4096
		posPE     = 0x80
	)

	data := bytes.Repeat([]byte{0x90}, size)
	data[0] = 'M'
	data[1] = 'Z'
	writeInt32LE(data[60:], posPE)
	data[posPE] = 'P'
	data[posPE+1] = 'E'
	data[posPE+2] = 0
	data[posPE+3] = 0
	writeInt16LE(data[posPE+4:], arch)
	writeInt32LE(data[posPE+28:], codeLen)
	writeInt32LE(data[posPE+44:], codeStart)
	return data
}

func setPECodeLength(data []byte, codeLen int) {
	writeInt32LE(data[0x80+28:], codeLen)
}

func fillX86Code(data []byte, codeStart, codeLen int) {
	for i := codeStart; i+5 <= codeStart+codeLen; i += 5 {
		data[i] = 0xE8
		data[i+1] = 0
		data[i+2] = 0
		data[i+3] = 0
		data[i+4] = 0
	}
}

func fillX86ExpandedCode(data []byte, codeStart, codeLen int) {
	for i := codeStart; i+8 <= codeStart+codeLen; i += 8 {
		escaped := ((i - codeStart) >> 3) < 24
		data[i] = 0xE8
		data[i+1] = 0
		data[i+2] = 0
		data[i+3] = 0
		data[i+4] = 0

		if escaped {
			data[i+5] = _EXE_X86_ESCAPE
		} else {
			data[i+5] = 0x90
		}

		data[i+6] = 0x90
		data[i+7] = 0x90
	}
}

func addX86BoundaryJCC(data []byte, codeStart, codeLen int) {
	idx := codeStart + codeLen - 5
	data[idx] = _EXE_X86_TWO_BYTE_PREFIX
	data[idx+1] = 0x85
	data[idx+2] = 0
	data[idx+3] = 0
	data[idx+4] = 0
	data[idx+5] = 0
}

func createX86BoundaryBlock() []byte {
	data := createPEBlock(_EXE_WIN_X86_ARCH)
	const (
		codeStart = 512
		codeLen   = 85
	)

	setPECodeLength(data, codeLen)
	fillX86Code(data, codeStart, 16*5)
	addX86BoundaryJCC(data, codeStart, codeLen)
	return data
}

func roundTripEXE(t *testing.T, src []byte) []byte {
	t.Helper()
	codec, err := NewEXECodec()

	if err != nil {
		t.Fatalf("new codec: %v", err)
	}

	encoded := make([]byte, codec.MaxEncodedLen(len(src)))
	decoded := make([]byte, len(src))
	_, written, err := codec.Forward(src, encoded)

	if err != nil {
		t.Fatalf("forward failed: %v", err)
	}

	encoded = encoded[:written]
	_, decodedWritten, err := codec.Inverse(encoded, decoded)

	if err != nil {
		t.Fatalf("inverse failed: %v", err)
	}

	if int(decodedWritten) != len(src) {
		t.Fatalf("decoded size mismatch: got=%d want=%d", decodedWritten, len(src))
	}

	if bytes.Equal(src, decoded) == false {
		t.Fatalf("round-trip mismatch")
	}

	return encoded
}

func TestEXECodecExpandedRoundTrip(t *testing.T) {
	data := createPEBlock(_EXE_WIN_X86_ARCH)
	fillX86ExpandedCode(data, 512, 4096)
	encoded := roundTripEXE(t, data)

	if len(encoded) <= len(data)+9 {
		t.Fatalf("expected expansion beyond header: got=%d src=%d", len(encoded), len(data))
	}
}

func TestEXECodecBoundaryJCCRoundTrip(t *testing.T) {
	roundTripEXE(t, createX86BoundaryBlock())
}

func TestEXECodecLegacyBoundaryJCCRoundTrip(t *testing.T) {
	data := createX86BoundaryBlock()
	encoded := roundTripEXE(t, data)
	codeEnd := int(binary.LittleEndian.Uint32(encoded[5:]))

	if codeEnd >= len(encoded) || encoded[codeEnd] != _EXE_X86_TWO_BYTE_PREFIX {
		t.Fatalf("unexpected boundary layout: codeEnd=%d len=%d byte=%02x", codeEnd, len(encoded), encoded[codeEnd])
	}

	legacy := append([]byte(nil), encoded...)
	binary.LittleEndian.PutUint32(legacy[5:], uint32(codeEnd+1))
	codec, err := NewEXECodec()

	if err != nil {
		t.Fatalf("new codec: %v", err)
	}

	decoded := make([]byte, len(data))
	_, written, err := codec.Inverse(legacy, decoded)

	if err != nil {
		t.Fatalf("inverse legacy failed: %v", err)
	}

	if int(written) != len(data) {
		t.Fatalf("decoded legacy size mismatch: got=%d want=%d", written, len(data))
	}

	if bytes.Equal(data, decoded) == false {
		t.Fatalf("legacy round-trip mismatch")
	}
}
