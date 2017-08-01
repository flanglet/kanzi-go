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

package function

import (
	"errors"
	"fmt"
	"kanzi/transform"
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

type BWTBlockCodec struct {
	bwt *transform.BWT
}

func NewBWTBlockCodec() (*BWTBlockCodec, error) {

	this := new(BWTBlockCodec)
	var err error
	this.bwt, err = transform.NewBWT()
	return this, err
}

func (this *BWTBlockCodec) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	blockSize := len(src)

	if len(dst) < this.MaxEncodedLen(blockSize) {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d",
			len(dst), this.MaxEncodedLen(blockSize))
	}

	chunks := transform.GetBWTChunks(blockSize)
	log := uint(1)

	for 1<<log <= len(src) {
		log++
	}

	log--

	// Estimate header size based on block size
	headerSizeBytes1 := (uint(chunks)*(2+log) + 7) >> 3

	// Apply forward Transform
	iIdx, oIdx, err := this.bwt.Forward(src, dst[headerSizeBytes1:])

	if err != nil {
		return iIdx, oIdx, err
	}

	headerSizeBytes2 := uint(0)

	for i := 0; i < chunks; i++ {
		primaryIndex := this.bwt.PrimaryIndex(i)
		pIndexSizeBits := uint(6)

		for 1<<pIndexSizeBits <= primaryIndex {
			pIndexSizeBits++
		}

		// Compute block size based on primary index
		headerSizeBytes2 += (2 + pIndexSizeBits)
	}

	headerSizeBytes2 = (headerSizeBytes2 + 7) >> 3

	if headerSizeBytes2 != headerSizeBytes1 {
		// Adjust space for header
		copy(dst[headerSizeBytes2:], dst[headerSizeBytes1:headerSizeBytes1+oIdx])
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
		oIdx += headerSizeBytes2

		for i := uint(1); i < pIndexSizeBytes; i++ {
			shift -= 8
			dst[idx] = byte(primaryIndex >> shift)
			idx++
		}
	}

	return iIdx, oIdx, nil
}

func (this *BWTBlockCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	srcIdx := uint(0)
	blockSize := uint(len(src))
	chunks := transform.GetBWTChunks(len(src))

	for i := 0; i < chunks; i++ {
		// Read block header (mode + primary index). See top of file for format
		blockMode := uint(src[srcIdx])
		srcIdx++
		pIndexSizeBytes := 1 + ((blockMode >> 6) & 0x03)

		if blockSize < pIndexSizeBytes {
			return 0, 0, errors.New("Invalid compressed length in stream")
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

		this.bwt.SetPrimaryIndex(i, primaryIndex)
	}

	// Apply inverse Transform
	return this.bwt.Inverse(src[srcIdx:srcIdx+blockSize], dst)
}

func (this BWTBlockCodec) MaxEncodedLen(srcLen int) int {
	// Return input buffer size + max header size
	return srcLen + 4*8
}
