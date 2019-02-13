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

// IntTransform  An integer function is an operation that takes an array of integers as input and
// and turns it into another array of integers of the same size.
// Return index in src, index in dst and error
type IntTransform interface {
	Forward(src, dst []int) (uint, uint, error)

	Inverse(src, dst []int) (uint, uint, error)
}

// ByteTransform  A byte function is an operation that takes an array of bytes as input and
// turns it into another array of bytes of the same size.
// Return index in src, index in dst and error
type ByteTransform interface {
	Forward(src, dst []byte) (uint, uint, error)

	Inverse(src, dst []byte) (uint, uint, error)
}

// IntFunction  An integer function is an operation that transforms the input int array and writes
// the result in the output int array. The result may have a different size.
// Return index in src, index in dst and error
type IntFunction interface {
	Forward(src, dst []int) (uint, uint, error)

	Inverse(src, dst []int) (uint, uint, error)

	// MaxEncodedLen Return the max size required for the encoding output buffer
	// If the max size of the output buffer is not known, return -1
	MaxEncodedLen(srcLen int) int
}

// ByteFunction A byte function is an operation that transforms the input byte array and writes
// the result in the output byte array.
type ByteFunction interface {
	Forward(src, dst []byte) (uint, uint, error)

	Inverse(src, dst []byte) (uint, uint, error)

	//MaxEncodedLen  Return the max size required for the encoding output buffer
	MaxEncodedLen(srcLen int) int
}

// InputBitStream  A bitstream reader
type InputBitStream interface {
	// ReadBit  Return the next bit in the bitstream. Panic if closed or EOS is reached.
	ReadBit() int

	// ReadBits  Length is the number of bits in [1..64]. Return the bits read as an uint64
	// Panic if closed or EOS is reached.
	ReadBits(length uint) uint64

	// ReadArray  Read bits and put them in the byte array. Length is the number of bits
	// Return the number of bits read. Panic if closed or EOS is reached.
	ReadArray(bits []byte, length uint) uint

	// Close  Make the bitstream unavailable for further reads.
	Close() (bool, error)

	// Read  Number of bits read
	Read() uint64

	// HasMoreToRead  Return false when the bitstream is closed or the EOS has been reached
	HasMoreToRead() (bool, error)
}

// OutputBitStream  A bitstream writer
type OutputBitStream interface {
	// WriteBit  Write the least significant bit of the input integer
	// Panic if closed or an IO error is received.
	WriteBit(bit int)

	// WriteBits  Write the least significant bits of 'bits' in the bitstream.
	// Length is the number of bits in [1..64] to write.
	// Return the number of bits written.
	// Panic if closed or an IO error is received.
	WriteBits(bits uint64, length uint) uint

	// WriteArray  Write bits out of the byte array. Length is the number of bits.
	// Return the number of bits written.
	// Panic if closed or an IO error is received.
	WriteArray(bits []byte, length uint) uint

	// Close  Make the bitstream unavailable for further writes.
	Close() (bool, error)

	// Written  Number of bits written
	Written() uint64
}

// Predictor Predict the probability of the next bit to be 1.
type Predictor interface {
	// Update  Update the probability model
	Update(bit byte)

	// Get  Return the split value representing the probability of 1 in the [0..4095] range.
	// E.G. 410 represents roughly a probability of 10% for 1
	Get() int
}

// EntropyEncoder  Entropy encode data to a bitstream
type EntropyEncoder interface {
	// Write  Encode the data provided into the bitstream. Return the number of byte
	// written to the bitstream
	Write(block []byte) (int, error)

	// BitStream  Return the underlying bitstream
	BitStream() OutputBitStream

	// Dispose  Must be called before getting rid of the entropy encoder
	Dispose()
}

// EntropyDecoder Entropy decode data from a bitstream
type EntropyDecoder interface {
	// Read  Decode data from the bitstream and return it in the provided buffer.
	Read(block []byte) (int, error)

	// BitStream  Return the underlying bitstream
	BitStream() InputBitStream

	// Dispose  Must be called before getting rid of the entropy decoder
	// Trying to encode after a call to dispose gives undefined behavior
	Dispose()
}
