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

package function

import (
	"errors"
	"fmt"
	"kanzi"
	"kanzi/transform"
)

// Utility class to compress/decompress a data block
// Fast reversible block coder/decoder based on a pipeline of transformations:
// Forward: Burrows-Wheeler -> Move to Front -> Zero Length
// Inverse: Zero Length -> Move to Front -> Burrows-Wheeler
// The block size determines the balance between speed and compression ratio

// Stream format: Header (m bytes) Data (n bytes)
// Header: mode (8 bits) + BWT primary index (8, 16 or 24 bits)
// mode: bits 7-6 contain the size in bits of the primary index :
//           00: primary index size <=  6 bits (fits in mode byte)
//           01: primary index size <= 14 bits (1 extra byte)
//           10: primary index size <= 22 bits (2 extra bytes)
//           11: primary index size  > 22 bits (3 extra bytes)
//       bits 5-0 contain 6 most significant bits of primary index
// primary index: remaining bits (up to 3 bytes)

const (
	MAX_BLOCK_HEADER_SIZE = 4
	MAX_BLOCK_SIZE        = 32*1024*1024 - MAX_BLOCK_HEADER_SIZE
)

type BlockCodec struct {
	mtft           *transform.MTFT
	bwt            *transform.BWT
	size           uint
	postProcessing bool
}

// If postProcessing is true, forward BWT is followed by a Global Structure
// Transform (here MTFT) and ZLT, else a raw BWT is performed.
func NewBlockCodec(size uint, postProcessing bool) (*BlockCodec, error) {
	if size > MAX_BLOCK_SIZE {
		errMsg := fmt.Sprintf("The block size must be at most %d\n", MAX_BLOCK_SIZE)
		return nil, errors.New(errMsg)
	}

	var err error
	this := new(BlockCodec)
	this.postProcessing = postProcessing
	this.bwt, err = transform.NewBWT(0)

	if err != nil {
		return nil, err
	}

	if postProcessing == true {
		this.mtft, err = transform.NewMTFT(0)

		if err != nil {
			return nil, err
		}
	}

	this.size = size
	return this, nil
}

func (this *BlockCodec) Size() uint {
	return this.size
}

func (this *BlockCodec) SetSize(sz uint) bool {
	if sz > MAX_BLOCK_SIZE {
		return false
	}

	this.size = sz
	return true
}

// Return no error if the compression chain succeeded. In this case, the input data
// may be modified. If the compression failed, the input data is returned unmodified.
func (this *BlockCodec) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	blockSize := this.size

	if this.size == 0 {
		blockSize = uint(len(src))
	}

	if blockSize > MAX_BLOCK_SIZE {
		errMsg := fmt.Sprintf("Block size is %v, max value is %v", blockSize, MAX_BLOCK_SIZE)
		return 0, 0, errors.New(errMsg)
	}

	if blockSize > uint(len(src)) {
		errMsg := fmt.Sprintf("Block size is %v, input buffer length is %v", blockSize, len(src))
		return 0, 0, errors.New(errMsg)
	}

	// Apply Burrows-Wheeler Transform
	this.bwt.SetSize(blockSize)
	iIdx, oIdx, _ := this.bwt.Forward(src, dst)
	primaryIndex := this.bwt.PrimaryIndex()
	pIndexSizeBits := uint(6)

	for 1<<pIndexSizeBits <= primaryIndex {
		pIndexSizeBits++
	}

	headerSizeBytes := (2 + pIndexSizeBits + 7) >> 3

	if this.postProcessing == true {
		// Apply Move-To-Front Transform
		this.mtft.SetSize(blockSize)
		this.mtft.Forward(dst, src)

		zlt, err := NewZLT(blockSize)

		if err != nil {
			return 0, 0, err
		}

		// Apply Zero Length Encoding
		iIdx, oIdx, err = zlt.Forward(src, dst[headerSizeBytes:])

		if err != nil {
			// Compression failed, recover source data
			this.mtft.Inverse(src, dst)
			this.bwt.Inverse(dst, src)
			return 0, 0, err
		}
	} else {
		// Shift output data to leave space for header
		hs := int(headerSizeBytes)

		for i := int(blockSize - 1); i >= 0; i-- {
			dst[i+hs] = dst[i]
		}
	}

	oIdx += headerSizeBytes

	// Write block header (mode + primary index). See top of file for format
	shift := (headerSizeBytes - 1) << 3
	mode := (pIndexSizeBits + 1) >> 3
	mode = (mode << 6) | ((primaryIndex >> shift) & 0x3F)
	dst[0] = byte(mode)

	for i := uint(1); i < headerSizeBytes; i++ {
		shift -= 8
		dst[i] = byte(primaryIndex >> shift)
	}

	return iIdx, oIdx, nil
}

func (this *BlockCodec) Inverse(src, dst []byte) (uint, uint, error) {
	compressedLength := this.size

	if compressedLength == 0 {
		return 0, 0, nil
	}

	// Read block header (mode + primary index). See top of file for format
	mode := uint(src[0])
	headerSizeBytes := 1 + ((mode >> 6) & 0x03)

	if compressedLength < headerSizeBytes {
		return 0, 0, errors.New("Invalid compressed length in stream")
	}

	if compressedLength == 0 {
		return 0, 0, nil
	}

	compressedLength -= headerSizeBytes
	shift := (headerSizeBytes - 1) << 3
	primaryIndex := (mode & 0x3F) << shift
	blockSize := compressedLength
	srcIdx := headerSizeBytes

	// Extract BWT primary index
	for i := uint(1); i < headerSizeBytes; i++ {
		shift -= 8
		primaryIndex |= uint(src[i]) << shift
	}

	if this.postProcessing == true {
		// Apply Zero Length Decoding
		zlt, err := NewZLT(compressedLength)

		if err != nil {
			return 0, 0, err
		}

		iIdx, oIdx, err := zlt.Inverse(src[srcIdx:], dst)
		iIdx += headerSizeBytes

		if err != nil {
			return iIdx, oIdx, err
		}

		srcIdx = 0
		blockSize = oIdx

		// Apply Move-To-Front Inverse Transform
		this.mtft.SetSize(blockSize)
		this.mtft.Inverse(dst, src)
	}

	// Apply Burrows-Wheeler Inverse Transform
	this.bwt.SetPrimaryIndex(primaryIndex)
	this.bwt.SetSize(blockSize)
	return this.bwt.Inverse(src[srcIdx:], dst)
}

func (this BlockCodec) MaxEncodedLen(srcLen int) int {
	// Return input buffer size + max header size
	// If forward() fails due to output buffer size, the block is returned
	// unmodified with an error
	return srcLen + 4
}
