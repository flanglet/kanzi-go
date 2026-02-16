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

package entropy

import (
	"errors"
	"fmt"

	kanzi "github.com/flanglet/kanzi-go/v2"
	internal "github.com/flanglet/kanzi-go/v2/internal"
)

// Code based on Order 0 range coder by Dmitry Subbotin itself derived from the algorithm
// described by G.N.N Martin in his seminal article in 1979.
// [G.N.N. Martin on the Data Recording Conference, Southampton, 1979]

const (
	_TOP_RANGE                = uint64(0x0FFFFFFFFFFFFFFF)
	_BOTTOM_RANGE             = uint64(0x000000000000FFFF)
	_RANGE_MASK               = uint64(0x0FFFFFFF00000000)
	_DEFAULT_RANGE_CHUNK_SIZE = uint(1 << 15) // 32 KB by default
	_DEFAULT_RANGE_LOG_RANGE  = uint(12)
	_RANGE_MAX_CHUNK_SIZE     = 1 << 30
)

// RangeEncoder a Order 0 Range Entropy Encoder
type RangeEncoder struct {
	low       uint64
	rng       uint64
	alphabet  [256]int
	freqs     [256]int
	cumFreqs  [257]uint64
	bitstream kanzi.OutputBitStream
	chunkSize uint
	logRange  uint
	shift     uint
}

// NewRangeEncoder creates a new instance of RangeEncoder
// The given arguments are either empty or containing a chunk size and
// a log range (to specify the precision of the encoding).
// EG: call NewRangeEncoder(bs) or NewRangeEncoder(bs, 16384, 14)
// The default chunk size is 65536 bytes.
func NewRangeEncoder(bs kanzi.OutputBitStream, args ...uint) (*RangeEncoder, error) {
	if bs == nil {
		return nil, errors.New("Range codec: Invalid null bitstream parameter")
	}

	if len(args) > 2 {
		return nil, errors.New("Range codec: At most one chunk size and one log range can be provided")
	}

	chkSize := _DEFAULT_RANGE_CHUNK_SIZE
	logRange := _DEFAULT_RANGE_LOG_RANGE

	if len(args) == 2 {
		chkSize = args[0]
		logRange = args[1]
	}

	if chkSize < 1024 {
		return nil, errors.New("Range codec: The chunk size must be at least 1024")
	}

	if chkSize > _RANGE_MAX_CHUNK_SIZE {
		return nil, fmt.Errorf("Range codec: The chunk size must be at most %d", _RANGE_MAX_CHUNK_SIZE)
	}

	if logRange < 8 || logRange > 16 {
		return nil, fmt.Errorf("Range codec: Invalid range parameter: %v (must be in [8..16])", logRange)
	}

	this := &RangeEncoder{}
	this.bitstream = bs
	this.alphabet = [256]int{}
	this.freqs = [256]int{}
	this.cumFreqs = [257]uint64{}
	this.logRange = logRange
	this.chunkSize = chkSize
	return this, nil
}

// NewRangeEncoderWithCtx  creates a new instance of RangeEncoder with a context
// The given arguments are either empty or containing a chunk size and
// a log range (to specify the precision of the encoding).
// The default chunk size is 65536 bytes.
func NewRangeEncoderWithCtx(bs kanzi.OutputBitStream, ctx *map[string]any, args ...uint) (*RangeEncoder, error) {
	if bs == nil {
		return nil, errors.New("Range codec: Invalid null bitstream parameter")
	}

	if len(args) > 2 {
		return nil, errors.New("Range codec: At most one chunk size and one log range can be provided")
	}

	chkSize := _DEFAULT_RANGE_CHUNK_SIZE
	logRange := _DEFAULT_RANGE_LOG_RANGE

	if len(args) == 2 {
		chkSize = args[0]
		logRange = args[1]
	}

	if chkSize < 1024 {
		return nil, errors.New("Range codec: The chunk size must be at least 1024")
	}

	if chkSize > _RANGE_MAX_CHUNK_SIZE {
		return nil, fmt.Errorf("Range codec: The chunk size must be at most %d", _RANGE_MAX_CHUNK_SIZE)
	}

	if logRange < 8 || logRange > 16 {
		return nil, fmt.Errorf("Range codec: Invalid range parameter: %v (must be in [8..16])", logRange)
	}

	this := &RangeEncoder{}
	this.bitstream = bs
	this.alphabet = [256]int{}
	this.freqs = [256]int{}
	this.cumFreqs = [257]uint64{}
	this.logRange = logRange
	this.chunkSize = chkSize
	return this, nil
}

func (this *RangeEncoder) updateFrequencies(frequencies []int, size int, lr uint) (int, error) {
	if frequencies == nil || len(frequencies) != 256 {
		return 0, errors.New("Range codec: Invalid frequencies parameter")
	}

	alphabetSize, err := NormalizeFrequencies(frequencies, this.alphabet[:], size, 1<<lr)

	if err != nil {
		return alphabetSize, err
	}

	if alphabetSize > 0 {
		this.cumFreqs[0] = 0

		// Create histogram of frequencies scaled to 'range'
		for i := range frequencies {
			this.cumFreqs[i+1] = this.cumFreqs[i] + uint64(frequencies[i])
		}
	}

	err = this.encodeHeader(this.alphabet[0:alphabetSize], frequencies, lr)
	return alphabetSize, err
}

func (this *RangeEncoder) encodeHeader(alphabet []int, frequencies []int, lr uint) error {
	if _, err := EncodeAlphabet(this.bitstream, alphabet); err != nil {
		return err
	}

	alphabetSize := len(alphabet)

	if alphabetSize == 0 {
		return nil
	}

	this.bitstream.WriteBits(uint64(lr-8), 3) // logRange
	chkSize := 8

	if alphabetSize < 64 {
		chkSize = 6
	}

	llr := uint(3)

	for 1<<llr <= lr {
		llr++
	}

	// Encode all frequencies (but the first one) by chunks
	for i := 1; i < alphabetSize; i += chkSize {
		max := frequencies[alphabet[i]] - 1
		logMax := uint(0)
		endj := min(i+chkSize, alphabetSize)

		// Search for max frequency log size in next chunk
		for j := i + 1; j < endj; j++ {
			if frequencies[alphabet[j]]-1 > max {
				max = frequencies[alphabet[j]] - 1
			}
		}

		for 1<<logMax <= max {
			logMax++
		}

		this.bitstream.WriteBits(uint64(logMax), llr)

		if logMax == 0 {
			// all frequencies equal one in this chunk
			continue
		}

		// Write frequencies
		for j := i; j < endj; j++ {
			this.bitstream.WriteBits(uint64(frequencies[alphabet[j]]-1), logMax)
		}
	}

	return nil
}

// Write encodes the data provided into the bitstream. Return the number of byte
// written to the bitstream. Splits the input into chunks and encode chunks
// sequentially based on local statistics.
func (this *RangeEncoder) Write(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Range codec: Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	sizeChunk := int(this.chunkSize)
	startChunk := 0
	end := len(block)

	for startChunk < end {
		this.rng = _TOP_RANGE
		this.low = 0
		lr := this.logRange
		endChunk := min(startChunk+sizeChunk, end)

		// Lower log range if the size of the data block is small
		for lr > 8 && 1<<lr > endChunk-startChunk {
			lr--
		}

		this.shift = lr
		buf := block[startChunk:endChunk]

		alphabetSize, err := this.rebuildStatistics(buf, lr)

		if err != nil {
			return startChunk, err
		}

		if alphabetSize <= 1 {
			// Skip chunk if only one symbol
			startChunk = endChunk
			continue
		}

		for i := range buf {
			this.encodeByte(buf[i])
		}

		// Flush 'low'
		this.bitstream.WriteBits(this.low, 60)
		startChunk = endChunk
	}

	return len(block), nil
}

// Compute chunk frequencies, cumulated frequencies and encode chunk header
func (this *RangeEncoder) rebuildStatistics(block []byte, lr uint) (int, error) {
	clear(this.freqs[:])
	internal.ComputeHistogram(block, this.freqs[:], true, false)
	return this.updateFrequencies(this.freqs[:], len(block), lr)
}

func (this *RangeEncoder) encodeByte(b byte) {
	// Compute next low and range
	symbol := int(b)
	cumFreq := this.cumFreqs[symbol]
	this.rng >>= this.shift
	this.low += (cumFreq * this.rng)
	this.rng *= (this.cumFreqs[symbol+1] - cumFreq)

	// If the left-most digits are the same throughout the range, write bits to bitstream
	for {
		if (this.low^(this.low+this.rng))&_RANGE_MASK != 0 {
			if this.rng > _BOTTOM_RANGE {
				break
			}

			// Normalize
			this.rng = -this.low & _BOTTOM_RANGE
		}

		this.bitstream.WriteBits(this.low>>32, 28)
		this.rng <<= 28
		this.low <<= 28
	}

}

// BitStream returns the underlying bitstream
func (this *RangeEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// Dispose this implementation does nothing
func (this *RangeEncoder) Dispose() {
}

// RangeDecoder Order 0 Range Entropy Decoder
type RangeDecoder struct {
	code      uint64
	low       uint64
	rng       uint64
	alphabet  [256]int
	freqs     [256]int
	cumFreqs  [257]uint64
	f2s       []uint16 // mapping frequency -> symbol
	bitstream kanzi.InputBitStream
	chunkSize uint
	shift     uint
}

// NewRangeDecoder creates a new instance of RangeDecoder
// The given arguments are either empty or containing a chunk size.
// EG: call NewRangeDecoder(bs) or NewRangeDecoder(bs, 16384)
// The default chunk size is 65536 bytes.
func NewRangeDecoder(bs kanzi.InputBitStream, args ...uint) (*RangeDecoder, error) {
	if bs == nil {
		return nil, errors.New("Range codec: Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("Range codec: At most one chunk size can be provided")
	}

	chkSize := _DEFAULT_RANGE_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize < 1024 {
		return nil, errors.New("Range codec: The chunk size must be at least 1024")
	}

	if chkSize > _RANGE_MAX_CHUNK_SIZE {
		return nil, fmt.Errorf("Range codec: The chunk size must be at most %d", _RANGE_MAX_CHUNK_SIZE)
	}

	this := &RangeDecoder{}
	this.bitstream = bs
	this.alphabet = [256]int{}
	this.freqs = [256]int{}
	this.cumFreqs = [257]uint64{}
	this.f2s = make([]uint16, 0)
	this.chunkSize = chkSize
	return this, nil
}

// NewRangeDecoderWithCtx creates a new instance of RangeDecoder with a context
// The given arguments are either empty or containing a chunk size.
// The default chunk size is 65536 bytes.
func NewRangeDecoderWithCtx(bs kanzi.InputBitStream, ctx *map[string]any, args ...uint) (*RangeDecoder, error) {
	if bs == nil {
		return nil, errors.New("Range codec: Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("Range codec: At most one chunk size can be provided")
	}

	chkSize := _DEFAULT_RANGE_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize < 1024 {
		return nil, errors.New("Range codec: The chunk size must be at least 1024")
	}

	if chkSize > _RANGE_MAX_CHUNK_SIZE {
		return nil, fmt.Errorf("Range codec: The chunk size must be at most %d", _RANGE_MAX_CHUNK_SIZE)
	}

	this := &RangeDecoder{}
	this.bitstream = bs
	this.alphabet = [256]int{}
	this.freqs = [256]int{}
	this.cumFreqs = [257]uint64{}
	this.f2s = make([]uint16, 0)
	this.chunkSize = chkSize
	return this, nil
}

func (this *RangeDecoder) decodeHeader(frequencies []int) (int, error) {
	alphabetSize, err := DecodeAlphabet(this.bitstream, this.alphabet[:])

	if err != nil || alphabetSize == 0 {
		return alphabetSize, err
	}

	if alphabetSize != 256 {
		clear(frequencies)
	}

	// Decode frequencies
	logRange := uint(8 + this.bitstream.ReadBits(3))
	scale := 1 << logRange
	this.shift = logRange
	sum := 0
	chkSize := 8

	if alphabetSize < 64 {
		chkSize = 6
	}

	llr := uint(3)

	for 1<<llr <= logRange {
		llr++
	}

	// Decode all frequencies (but the first one)
	for i := 1; i < alphabetSize; i += chkSize {
		logMax := uint(this.bitstream.ReadBits(llr))

		if 1<<logMax > scale {
			err := fmt.Errorf("Invalid bitstream: incorrect frequency size %v in range decoder", logMax)
			return alphabetSize, err
		}

		endj := min(i+chkSize, alphabetSize)

		// Read frequencies
		for j := i; j < endj; j++ {
			freq := 1

			if logMax > 0 {
				freq = int(1 + this.bitstream.ReadBits(logMax))

				if freq <= 0 || freq >= scale {
					err := fmt.Errorf("Invalid bitstream: incorrect frequency %v for symbol '%v' in range decoder", freq, this.alphabet[j])
					return alphabetSize, err
				}
			}

			frequencies[this.alphabet[j]] = freq
			sum += freq
		}
	}

	// Infer first frequency
	if scale <= sum {
		err := fmt.Errorf("Invalid bitstream: incorrect frequency %v for symbol '%v' in range decoder", frequencies[this.alphabet[0]], this.alphabet[0])
		return alphabetSize, err
	}

	frequencies[this.alphabet[0]] = scale - sum
	this.cumFreqs[0] = 0

	if len(this.f2s) < scale {
		this.f2s = make([]uint16, scale)
	}

	// Create reverse mapping
	for i := range frequencies {
		this.cumFreqs[i+1] = this.cumFreqs[i] + uint64(frequencies[i])
		base := int(this.cumFreqs[i])

		for j := frequencies[i] - 1; j >= 0; j-- {
			this.f2s[base+j] = uint16(i)
		}
	}

	return alphabetSize, nil
}

// Read decodes data from the bitstream and return it in the provided buffer.
// Decode the data chunk by chunk sequentially.
// Return the number of bytes read from the bitstream.
func (this *RangeDecoder) Read(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Range codec: Invalid null block parameter")
	}

	end := len(block)
	startChunk := 0
	sizeChunk := int(this.chunkSize)

	for startChunk < end {
		endChunk := min(startChunk+sizeChunk, end)
		alphabetSize, err := this.decodeHeader(this.freqs[:])

		if err != nil || alphabetSize == 0 {
			return startChunk, err
		}

		if alphabetSize == 1 {
			// Shortcut for chunks with only one symbol
			for i := startChunk; i < endChunk; i++ {
				block[i] = byte(this.alphabet[0])
			}

			startChunk = endChunk
			continue
		}

		this.rng = _TOP_RANGE
		this.low = 0
		this.code = this.bitstream.ReadBits(60)
		buf := block[startChunk:endChunk]

		for i := range buf {
			buf[i] = this.decodeByte()
		}

		startChunk = endChunk
	}

	return len(block), nil
}

func (this *RangeDecoder) decodeByte() byte {
	// Compute next low and range
	this.rng >>= this.shift
	count := int((this.code - this.low) / this.rng)
	symbol := this.f2s[count]
	cumFreq := this.cumFreqs[symbol]
	this.low += (cumFreq * this.rng)
	this.rng *= (this.cumFreqs[symbol+1] - cumFreq)

	// If the left-most digits are the same throughout the range, read bits from bitstream
	for {
		if (this.low^(this.low+this.rng))&_RANGE_MASK != 0 {
			if this.rng > _BOTTOM_RANGE {
				break
			}

			// Normalize
			this.rng = -this.low & _BOTTOM_RANGE
		}

		this.code = (this.code << 28) | this.bitstream.ReadBits(28)
		this.rng <<= 28
		this.low <<= 28
	}

	return byte(symbol)
}

// BitStream returns the underlying bitstream
func (this *RangeDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Dispose this implementation does nothing
func (this *RangeDecoder) Dispose() {
}
