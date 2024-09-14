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
	internal "github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_BWT_MAX_HEADER_SIZE = 8 * 4
)

// Utility class to en/de-code a BWT data block and its associated primary index(es)

// BWT stream format: Header (mode + primary index(es)) | Data (n bytes)
//   mode (8 bits): xxxyyyzz
//   xxx: ignored
//   yyy: log(chunks)
//   zz: primary index size - 1 (in bytes)
//   primary indexes (chunks * (8|16|24|32 bits))

// BWTBlockCodec a codec that encapsulates a Burrows Wheeler Transform and
// takes care of encoding/decoding information about the primary indexes in a header.
type BWTBlockCodec struct {
	bwt       *BWT
	bsVersion uint
}

// NewBWTBlockCodec creates a new instance of BWTBlockCodec
func NewBWTBlockCodec() (*BWTBlockCodec, error) {
	this := &BWTBlockCodec{}
	this.bsVersion = 6
	var err error
	this.bwt, err = NewBWT()
	return this, err
}

// NewBWTBlockCodecWithCtx creates a new instance of BWTBlockCodec
func NewBWTBlockCodecWithCtx(ctx *map[string]any) (*BWTBlockCodec, error) {
	this := &BWTBlockCodec{}
	this.bsVersion = 6

	if val, containsKey := (*ctx)["bsVersion"]; containsKey {
		this.bsVersion = val.(uint)
	}

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
	logBlockSize := internal.Log2NoCheck(uint32(blockSize))

	if blockSize&(blockSize-1) != 0 {
		logBlockSize++
	}

	pIndexSize := int(logBlockSize+7) >> 3

	if pIndexSize <= 0 || pIndexSize >= 5 {
		return 0, 0, errors.New("BWT forward failed: invalid index size")
	}

	chunks := GetBWTChunks(blockSize)
	logNbChunks := internal.Log2NoCheck(uint32(chunks))

	if logNbChunks > 7 {
		return 0, 0, errors.New("BWT forward failed: invalid number of chunks")
	}

	headerSize := chunks*pIndexSize + 1

	// Apply forward Transform
	iIdx, oIdx, err := this.bwt.Forward(src, dst[headerSize:])

	if err != nil {
		return iIdx, oIdx, err
	}

	mode := byte(int(logNbChunks<<2) | (pIndexSize - 1))

	// Emit header
	for i, idx := 0, 1; i < chunks; i++ {
		primaryIndex := this.bwt.PrimaryIndex(i) - 1
		shift := (pIndexSize - 1) << 3

		for shift >= 0 {
			dst[idx] = byte(primaryIndex >> shift)
			idx++
			shift -= 8
		}
	}

	dst[0] = mode
	return iIdx, oIdx + uint(headerSize), nil
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *BWTBlockCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if len(src) == 1 {
		return 0, 0, errors.New("BWT inverse transform failed: invalid size")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	srcIdx := 0
	blockSize := len(src)

	if this.bsVersion > 5 {
		// Number of chunks and primary index size in bitstream since bsVersion 6
		mode := src[0]
		logNbChunks := uint(mode>>2) & 0x07
		pIndexSize := int(mode&0x03) + 1

		if pIndexSize == 0 {
			return 0, 0, errors.New("BWT inverse transform failed: invalid index size")
		}

		chunks := 1 << logNbChunks

		if chunks != GetBWTChunks(blockSize) {
			return 0, 0, errors.New("BWT inverse transform failed: invalid number of chunks")
		}

		headerSize := chunks*pIndexSize + 1

		if len(src) < headerSize || blockSize < headerSize {
			return 0, 0, errors.New("BWT inverse transform failed: invalid header size")
		}

		// Read header
		for i, idx := 0, 1; i < chunks; i++ {
			shift := (pIndexSize - 1) << 3
			primaryIndex := uint(0)

			// Extract BWT primary index
			for shift >= 0 {
				primaryIndex = (primaryIndex << 8) | uint(src[idx])
				idx++
				shift -= 8
			}

			if this.bwt.SetPrimaryIndex(i, primaryIndex+1) == false {
				return 0, 0, errors.New("BWT inverse transform failed: invalid primary index in bitstream")
			}

		}

		srcIdx += headerSize
		blockSize -= headerSize
	} else {
		chunks := GetBWTChunks(len(src))

		for i := 0; i < chunks; i++ {
			// Read block header (mode + primary index). See top of file for format
			blockMode := int(src[srcIdx])
			srcIdx++
			pIndexSizeBytes := 1 + ((blockMode >> 6) & 0x03)

			if blockSize < pIndexSizeBytes {
				return 0, 0, errors.New("BWT inverse transform failed: invalid compressed length in bitstream")
			}

			blockSize -= pIndexSizeBytes
			shift := (pIndexSizeBytes - 1) << 3
			primaryIndex := uint(blockMode&0x3F) << shift

			// Extract BWT primary index
			for i := 1; i < pIndexSizeBytes; i++ {
				shift -= 8
				primaryIndex |= uint(src[srcIdx]) << shift
				srcIdx++
			}

			if this.bwt.SetPrimaryIndex(i, primaryIndex) == false {
				return 0, 0, errors.New("BWT inverse transform failed: invalid primary index in bitstream")
			}
		}
	}

	// Apply inverse Transform
	return this.bwt.Inverse(src[srcIdx:srcIdx+blockSize], dst)
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *BWTBlockCodec) MaxEncodedLen(srcLen int) int {
	return srcLen + _BWT_MAX_HEADER_SIZE
}
