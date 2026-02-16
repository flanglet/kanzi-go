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

// Package kanzi defines all the top level interfaces used in the kanzi
// data lossless compressor/decompressor
//
// The implementation of these interfaces are available in sub-folders
// like bitstream, io, transform or entropy.
// In particular, the io package contains the implementation of the
// Writer and Reader used to compress and decompress data.
package kanzi

const (
	ERR_MISSING_PARAM       = 1
	ERR_BLOCK_SIZE          = 2
	ERR_INVALID_CODEC       = 3
	ERR_CREATE_COMPRESSOR   = 4
	ERR_CREATE_DECOMPRESSOR = 5
	ERR_OUTPUT_IS_DIR       = 6
	ERR_OVERWRITE_FILE      = 7
	ERR_CREATE_FILE         = 8
	ERR_CREATE_BITSTREAM    = 9
	ERR_OPEN_FILE           = 10
	ERR_READ_FILE           = 11
	ERR_WRITE_FILE          = 12
	ERR_PROCESS_BLOCK       = 13
	ERR_CREATE_CODEC        = 14
	ERR_INVALID_FILE        = 15
	ERR_STREAM_VERSION      = 16
	ERR_CREATE_STREAM       = 17
	ERR_INVALID_PARAM       = 18
	ERR_CRC_CHECK           = 19
	ERR_UNKNOWN             = 127
)

// IntTransform is a function that transforms the input int slice and writes
// the result in the output int slice. The result may have a different size.
// The transform must be stateless to ensure that the compression results
// are the same regardless of the number of jobs (ie no information is retained
// between to invocations of Forward or Inverse).
type IntTransform interface {
	// Forward applies the function to the source and writes the result
	// to the destination. Returns number of bytes read, number of bytes
	// written and possibly an error.
	Forward(src, dst []int) (uint, uint, error)

	// Inverse applies the reverse function to the source and writes the result
	// to the destination. Returns number of bytes read, number of bytes
	// written and possibly an error.
	Inverse(src, dst []int) (uint, uint, error)

	// MaxEncodedLen returns the max size required for the encoding output buffer
	// If the max size of the output buffer is not known, return -1
	MaxEncodedLen(srcLen int) int
}

// ByteTransform is a function that transforms the input byte slice and writes
// the result in the output byte slice. The result may have a different size.
// The transform must be stateless to ensure that the compression results
// are the same regardless of the number of jobs (ie no information is retained
// between to invocations of Forward or Inverse).
type ByteTransform interface {
	// Forward applies the function to the src and writes the result
	// to the destination. Returns number of bytes read, number of bytes
	// written and possibly an error.
	Forward(src, dst []byte) (uint, uint, error)

	// Inverse applies the reverse function to the src and writes the result
	// to the destination. Returns number of bytes read, number of bytes
	// written and possibly an error.
	Inverse(src, dst []byte) (uint, uint, error)

	// MaxEncodedLen returns the max size required for the encoding output buffer
	MaxEncodedLen(srcLen int) int
}

// InputBitStream is a bitstream reader
type InputBitStream interface {
	// ReadBit returns the next bit in the bitstream. Panics if closed or EOS is reached.
	ReadBit() int

	// ReadBits reads 'length' (in [1..64]) bits from the bitstream.
	// Returns the bits read as an uint64.
	// Panics if closed or EOS is reached.
	ReadBits(length uint) uint64

	// ReadArray reads 'length' bits from the bitstream and put them in the byte slice.
	// Returns the number of bits read.
	// Panics if closed or EOS is reached.
	ReadArray(bits []byte, length uint) uint

	// Close makes the bitstream unavailable for further reads.
	Close() error

	// Read returns the number of bits read
	Read() uint64

	// HasMoreToRead returns false when the bitstream is closed or the EOS has been reached
	HasMoreToRead() (bool, error)
}

// OutputBitStream is a bitstream writer
type OutputBitStream interface {
	// WriteBit writes the least significant bit of the input integer.
	// Panics if closed or an IO error is received.
	WriteBit(bit int)

	// WriteBits writes the least significant bits of 'bits' to the bitstream.
	// Length is the number of bits to write (in [1..64]).
	// Returns the number of bits written.
	// Panics if closed or an IO error is received.
	WriteBits(bits uint64, length uint) uint

	// WriteArray writes bits out of the byte slice. Length is the number of bits.
	// Returns the number of bits written.
	// Panics if closed or an IO error is received.
	WriteArray(bits []byte, length uint) uint

	// Close makes the bitstream unavailable for further writes.
	Close() error

	// Written returns the number of bits written
	Written() uint64
}

// Predictor predicts the probability of the next bit being 1.
type Predictor interface {
	// Update updates the internal probability model based on the observed bit
	Update(bit byte)

	// Get returns the value representing the probability of the next bit being 1
	// in the [0..4095] range.
	// E.G. 410 represents roughly a probability of 10% for 1
	Get() int
}

// EntropyEncoder entropy encodes data to a bitstream
type EntropyEncoder interface {
	// Write encodes the data provided into the bitstream. Return the number of bytes
	// written to the bitstream
	Write(block []byte) (int, error)

	// BitStream returns the underlying bitstream
	BitStream() OutputBitStream

	// Dispose must be called before getting rid of the entropy encoder
	// Trying to encode after a call to dispose gives undefined behavior
	Dispose()
}

// EntropyDecoder entropy decodes data from a bitstream
type EntropyDecoder interface {
	// Read decodes data from the bitstream and return it in the provided buffer.
	// Return the number of bytes read from the bitstream
	Read(block []byte) (int, error)

	// BitStream returns the underlying bitstream
	BitStream() InputBitStream

	// Dispose must be called before getting rid of the entropy decoder
	// Trying to decode after a call to dispose gives undefined behavior
	Dispose()
}
