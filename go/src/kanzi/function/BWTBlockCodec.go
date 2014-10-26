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
// Forward: (Bijective) Burrows-Wheeler -> Move to Front -> Zero Run Length
// Inverse: Zero Run Length -> Move to Front -> (Bijective) Burrows-Wheeler
// The block size determines the balance between speed and compression ratio

// BWT stream format: Header (m bytes) Data (n bytes)
// Header: mode (8 bits) + BWT primary index (8, 16 or 24 bits)
// mode: bits 7-6 contain the size in bits of the primary index :
//           00: primary index size <=  6 bits (fits in mode byte)
//           01: primary index size <= 14 bits (1 extra byte)
//           10: primary index size <= 22 bits (2 extra bytes)
//           11: primary index size  > 22 bits (3 extra bytes)
//       bits 5-0 contain 6 most significant bits of primary index
// primary index: remaining bits (up to 3 bytes)
// Bijective BWT stream format: Data (n bytes)

const (
	GST_MODE_RAW        = 0
	GST_MODE_MTF        = 1
	GST_MODE_RANK       = 2
	GST_MODE_TIMESTAMP  = 3
	BWT_MAX_HEADER_SIZE = 4
	MAX_BLOCK_SIZE      = 256 * 1024 * 1024 // 30 bits
)

type BWTBlockCodec struct {
	transform kanzi.ByteTransform
	mode      int
	size      uint
	isBWT     bool
}

// Based on the mode, the forward transform is followed by a Global Structure
// Transform and ZRLT, else a raw transform is performed.
func NewBWTBlockCodec(tr interface{}, mode int, blockSize uint) (*BWTBlockCodec, error) {
	if tr == nil {
		return nil, errors.New("Invalid null transform parameter")
	}

	if _, isTransform := tr.(kanzi.ByteTransform); isTransform == false {
		return nil, errors.New("The transform must implement the ByteTransform interface")
	}

	if _, isSizeable := tr.(kanzi.Sizeable); isSizeable == false {
		return nil, errors.New("The transform must implement the Sizeable interface")
	}

	if mode != GST_MODE_RAW && mode != GST_MODE_MTF && mode != GST_MODE_RANK && mode != GST_MODE_TIMESTAMP {
		return nil, errors.New("Invalid GST mode parameter")
	}

	_, isBWT := tr.(*transform.BWT)

	this := new(BWTBlockCodec)
	this.mode = mode
	this.size = blockSize
	this.transform = tr.(kanzi.ByteTransform)
	this.isBWT = isBWT

	if blockSize > this.maxBlockSize() {
		transformName := "BWT"

		if this.isBWT == false {
			transformName = "BWTS"
		}

		errMsg := fmt.Sprintf("The max block size for the %v is %d", transformName, this.maxBlockSize())
		return nil, errors.New(errMsg)
	}

	return this, nil
}

func (this *BWTBlockCodec) createGST(blockSize uint) (kanzi.ByteTransform, error) {
	// SBRT can perform MTFT but the dedicated class is faster
	if this.mode == GST_MODE_RAW {
		return nil, nil
	}

	if this.mode == GST_MODE_MTF {
		return transform.NewMTFT(blockSize)
	}

	return transform.NewSBRT(this.mode, blockSize)
}

func (this *BWTBlockCodec) maxBlockSize() uint {
	maxSize := uint(MAX_BLOCK_SIZE)

	if this.isBWT == true {
		maxSize -= BWT_MAX_HEADER_SIZE
	}

	return maxSize
}

func (this *BWTBlockCodec) Size() uint {
	return this.size
}

func (this *BWTBlockCodec) SetSize(sz uint) bool {
	if sz > this.maxBlockSize() {
		return false
	}

	this.size = sz
	return true
}

// Return no error if the compression chain succeeded. In this case, the input data
// may be modified. If the compression failed, the input data is returned unmodified.
func (this *BWTBlockCodec) Forward(src, dst []byte) (uint, uint, error) {
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

	if blockSize == 0 {
		blockSize = uint(len(src))

		if blockSize > this.maxBlockSize() {
			errMsg := fmt.Sprintf("Block size is %v, max value is %v", blockSize, this.maxBlockSize())
			return 0, 0, errors.New(errMsg)
		}
	} else if blockSize > uint(len(src)) {
		errMsg := fmt.Sprintf("Block size is %v, input buffer length is %v", blockSize, len(src))
		return 0, 0, errors.New(errMsg)
	}

	this.transform.(kanzi.Sizeable).SetSize(blockSize)

	// Apply forward Transform
	iIdx, oIdx, _ := this.transform.Forward(src, dst)

	headerSizeBytes := uint(0)
	pIndexSizeBits := uint(0)
	primaryIndex := uint(0)

	if this.isBWT {
		primaryIndex = this.transform.(*transform.BWT).PrimaryIndex()
		pIndexSizeBits = uint(6)

		for 1<<pIndexSizeBits <= primaryIndex {
			pIndexSizeBits++
		}

		headerSizeBytes = (2 + pIndexSizeBits + 7) >> 3
	}

	if this.mode != GST_MODE_RAW {
		// Apply Post Transform
		gst, err := this.createGST(blockSize)

		if err != nil {
			return 0, 0, err
		}

		gst.Forward(dst, src)

		if ZRLT, err := NewZRLT(blockSize); err == nil {
			// Apply Zero Run Length Encoding
			iIdx, oIdx, err = ZRLT.Forward(src, dst[headerSizeBytes:])

			if err != nil {
				// Compression failed, recover source data
				gst.Inverse(src, dst)
				this.transform.Inverse(dst, src)
				return 0, 0, err
			}
		}
	} else if headerSizeBytes > 0 {
		// Shift output data to leave space for header
		hs := int(headerSizeBytes)

		for i := int(blockSize - 1); i >= 0; i-- {
			dst[i+hs] = dst[i]
		}
	}

	if this.isBWT {
		oIdx += headerSizeBytes

		// Write block header (mode + primary index). See top of file for format
		shift := (headerSizeBytes - 1) << 3
		blockMode := (pIndexSizeBits + 1) >> 3
		blockMode = (blockMode << 6) | ((primaryIndex >> shift) & 0x3F)
		dst[0] = byte(blockMode)

		for i := uint(1); i < headerSizeBytes; i++ {
			shift -= 8
			dst[i] = byte(primaryIndex >> shift)
		}
	}

	return iIdx, oIdx, nil
}

func (this *BWTBlockCodec) Inverse(src, dst []byte) (uint, uint, error) {
	compressedLength := this.size

	if compressedLength == 0 {
		return 0, 0, nil
	}

	primaryIndex := uint(0)
	blockSize := compressedLength
	headerSizeBytes := uint(0)
	srcIdx := uint(0)

	if this.isBWT {
		// Read block header (mode + primary index). See top of file for format
		blockMode := uint(src[0])
		headerSizeBytes = 1 + ((blockMode >> 6) & 0x03)

		if compressedLength < headerSizeBytes {
			return 0, 0, errors.New("Invalid compressed length in stream")
		}

		if compressedLength == 0 {
			return 0, 0, nil
		}

		compressedLength -= headerSizeBytes
		shift := (headerSizeBytes - 1) << 3
		primaryIndex = (blockMode & 0x3F) << shift
		blockSize = compressedLength
		srcIdx = headerSizeBytes

		// Extract BWT primary index
		for i := uint(1); i < headerSizeBytes; i++ {
			shift -= 8
			primaryIndex |= uint(src[i]) << shift
		}
	}

	if blockSize > this.maxBlockSize() {
		errMsg := fmt.Sprintf("Block size is %v, max value is %v", blockSize, this.maxBlockSize())
		return 0, 0, errors.New(errMsg)
	}

	if this.mode != GST_MODE_RAW {
		// Apply Zero Run Length Decoding
		ZRLT, err := NewZRLT(compressedLength)

		if err != nil {
			return 0, 0, err
		}

		iIdx, oIdx, err := ZRLT.Inverse(src[srcIdx:], dst)
		iIdx += headerSizeBytes

		if err != nil {
			return iIdx, oIdx, err
		}

		srcIdx = 0
		blockSize = oIdx

		// Apply inverse Pre Transform
		gst, err := this.createGST(blockSize)

		if err != nil {
			return 0, 0, err
		}

		gst.Inverse(dst, src)
	}

	if this.isBWT {
		this.transform.(*transform.BWT).SetPrimaryIndex(primaryIndex)
	}

	this.transform.(kanzi.Sizeable).SetSize(blockSize)

	// Apply inverse Transform
	return this.transform.Inverse(src[srcIdx:], dst)
}

func (this BWTBlockCodec) MaxEncodedLen(srcLen int) int {
	// Return input buffer size + max header size
	// If forward() fails due to output buffer size, the block is returned
	// unmodified with an error
	if this.isBWT == true {
		return srcLen + 4
	}

	return srcLen
}
