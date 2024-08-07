/*
Copyright 2011-2024 Frederic Langlet
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
	"errors"
	"fmt"
)

const (
	_BWT_MAX_HEADER_SIZE = 8 * 4
)

// Utility class to en/de-code a BWT data block and its associated primary index(es)

// BWT stream format: Header (m bytes) Data (n bytes)
// Header: For each primary index,
//   mode (8 bits) + primary index (8,16 or 24 bits)
//   mode: bits 7-6 contain the size in bits of the primary index :
//             00: primary index size <=  6 bits (fits in mode byte)
//             01: primary index size <= 14 bits (1 extra byte)
//             10: primary index size <= 22 bits (2 extra bytes)
//             11: primary index size  > 22 bits (3 extra bytes)
//         bits 5-0 contain 6 most significant bits of primary index
//   primary index: remaining bits (up to 3 bytes)

// BWTBlockCodec a codec that encapsulates a Burrows Wheeler Transform and
// takes care of encoding/decoding information about the primary indexes in a header.
type BWTBlockCodec struct {
	bwt *BWT
}

// NewBWTBlockCodec creates a new instance of BWTBlockCodec
func NewBWTBlockCodec() (*BWTBlockCodec, error) {
	this := &BWTBlockCodec{}
	var err error
	this.bwt, err = NewBWT()
	return this, err
}

// NewBWTBlockCodecWithCtx creates a new instance of BWTBlockCodec
func NewBWTBlockCodecWithCtx(ctx *map[string]any) (*BWTBlockCodec, error) {
	this := &BWTBlockCodec{}
	var err error
	this.bwt, err = NewBWTWithCtx(ctx)
	return this, err
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *BWTBlockCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	blockSize := len(src)
	chunks := GetBWTChunks(blockSize)
	log := uint(1)

	for 1<<log <= len(src) {
		log++
	}

	// Estimate header size based on block size
	headerSizeBytes1 := uint(chunks) * ((2 + log + 7) >> 3)

	// Apply forward Transform
	iIdx, oIdx, err := this.bwt.Forward(src, dst[headerSizeBytes1:])

	if err != nil {
		return iIdx, oIdx, err
	}

	oIdx += headerSizeBytes1
	headerSizeBytes2 := uint(0)

	for i := 0; i < chunks; i++ {
		primaryIndex := this.bwt.PrimaryIndex(i)
		pIndexSizeBits := uint(6)

		for 1<<pIndexSizeBits <= primaryIndex {
			pIndexSizeBits++
		}

		// Compute block size based on primary index
		headerSizeBytes2 += ((2 + pIndexSizeBits + 7) >> 3)
	}

	if headerSizeBytes2 != headerSizeBytes1 {
		// Adjust space for header
		copy(dst[headerSizeBytes2:], dst[headerSizeBytes1:headerSizeBytes1+uint(blockSize)])
		oIdx = oIdx - headerSizeBytes1 + headerSizeBytes2
	}

	idx := 0

	for i := 0; i < chunks; i++ {
		primaryIndex := this.bwt.PrimaryIndex(i)
		pIndexSizeBits := uint(6)

		for 1<<pIndexSizeBits <= primaryIndex {
			pIndexSizeBits++
		}

		// Compute primary index size
		pIndexSizeBytes := (2 + pIndexSizeBits + 7) >> 3

		// Write block header (mode + primary index). See top of file for format
		shift := (pIndexSizeBytes - 1) << 3
		blockMode := (pIndexSizeBits + 1) >> 3
		blockMode = (blockMode << 6) | ((primaryIndex >> shift) & 0x3F)
		dst[idx] = byte(blockMode)
		idx++

		for shift >= 8 {
			shift -= 8
			dst[idx] = byte(primaryIndex >> shift)
			idx++
		}
	}

	return iIdx, oIdx, nil
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *BWTBlockCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	srcIdx := uint(0)
	blockSize := uint(len(src))
	chunks := GetBWTChunks(len(src))

	for i := 0; i < chunks; i++ {
		// Read block header (mode + primary index). See top of file for format
		blockMode := uint(src[srcIdx])
		srcIdx++
		pIndexSizeBytes := 1 + ((blockMode >> 6) & 0x03)

		if blockSize < pIndexSizeBytes {
			return 0, 0, errors.New("BWT inverse transform failed: invalid compressed length in bitstream")
		}

		blockSize -= pIndexSizeBytes
		shift := (pIndexSizeBytes - 1) << 3
		primaryIndex := (blockMode & 0x3F) << shift

		// Extract BWT primary index
		for i := uint(1); i < pIndexSizeBytes; i++ {
			shift -= 8
			primaryIndex |= uint(src[srcIdx]) << shift
			srcIdx++
		}

		if this.bwt.SetPrimaryIndex(i, primaryIndex) == false {
			return 0, 0, errors.New("BWT inverse transform failed: invalid primary index in bitstream")
		}
	}

	// Apply inverse Transform
	return this.bwt.Inverse(src[srcIdx:srcIdx+blockSize], dst)
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *BWTBlockCodec) MaxEncodedLen(srcLen int) int {
	return srcLen + _BWT_MAX_HEADER_SIZE
}
