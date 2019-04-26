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

package entropy

import (
	"errors"
	"fmt"

	kanzi "github.com/flanglet/kanzi-go"
)

// Based on Order 0 range coder by Dmitry Subbotin itself derived from the algorithm
// described by G.N.N Martin in his seminal article in 1979.
// [G.N.N. Martin on the Data Recording Conference, Southampton, 1979]
// Optimized for speed.

const (
	TOP_RANGE                = uint64(0x0FFFFFFFFFFFFFFF)
	BOTTOM_RANGE             = uint64(0x000000000000FFFF)
	RANGE_MASK               = uint64(0x0FFFFFFF00000000)
	DEFAULT_RANGE_CHUNK_SIZE = uint(1 << 16) // 64 KB by default
	DEFAULT_RANGE_LOG_RANGE  = uint(13)
)

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

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block.
// Since the number of args is variable, this function can be called like this:
// NewRangeEncoder(bs) or NewRangeEncoder(bs, 16384, 14)
// The default chunk size is 65536 bytes.
func NewRangeEncoder(bs kanzi.OutputBitStream, args ...uint) (*RangeEncoder, error) {
	if bs == nil {
		return nil, errors.New("Range codec: Invalid null bitstream parameter")
	}

	if len(args) > 2 {
		return nil, errors.New("Range codec: At most one chunk size and one log range can be provided")
	}

	chkSize := DEFAULT_RANGE_CHUNK_SIZE
	logRange := DEFAULT_RANGE_LOG_RANGE

	if len(args) == 2 {
		chkSize = args[0]
		logRange = args[1]
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("Range codec: The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("Range codec: The chunk size must be at most 2^30")
	}

	if logRange < 8 || logRange > 16 {
		return nil, fmt.Errorf("Range codec: Invalid range parameter: %v (must be in [8..16])", logRange)
	}

	this := new(RangeEncoder)
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

	err = this.encodeHeader(alphabetSize, this.alphabet[:], frequencies, lr)
	return alphabetSize, err
}

func (this *RangeEncoder) encodeHeader(alphabetSize int, alphabet []int, frequencies []int, lr uint) error {
	if _, err := EncodeAlphabet(this.bitstream, alphabet[0:alphabetSize]); err != nil {
		return err
	}

	if alphabetSize == 0 {
		return nil
	}

	this.bitstream.WriteBits(uint64(lr-8), 3) // logRange
	chkSize := 12

	if alphabetSize < 64 {
		chkSize = 6
	}

	llr := uint(3)

	for 1<<llr <= lr {
		llr++
	}

	/// Encode all frequencies (but the first one) by chunks of size 'inc'
	for i := 1; i < alphabetSize; i += chkSize {
		max := 0
		logMax := uint(1)
		endj := i + chkSize

		if endj > alphabetSize {
			endj = alphabetSize
		}

		// Search for max frequency log size in next chunk
		for j := i; j < endj; j++ {
			if frequencies[alphabet[j]] > max {
				max = frequencies[alphabet[j]]
			}
		}

		for 1<<logMax <= max {
			logMax++
		}

		this.bitstream.WriteBits(uint64(logMax-1), llr)

		// Write frequencies
		for j := i; j < endj; j++ {
			this.bitstream.WriteBits(uint64(frequencies[alphabet[j]]), logMax)
		}
	}

	return nil
}

func (this *RangeEncoder) Write(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Range codec: Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	sizeChunk := int(this.chunkSize)

	if sizeChunk == 0 {
		sizeChunk = len(block)
	}

	startChunk := 0
	end := len(block)

	for startChunk < end {
		this.rng = TOP_RANGE
		this.low = 0
		lr := this.logRange

		endChunk := startChunk + sizeChunk

		if endChunk > end {
			endChunk = end
		}

		// Lower log range if the size of the data block is small
		for lr > 8 && 1<<lr > endChunk-startChunk {
			lr--
		}

		this.shift = lr
		buf := block[startChunk:endChunk]

		if err := this.rebuildStatistics(buf, lr); err != nil {
			return startChunk, err
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
func (this *RangeEncoder) rebuildStatistics(block []byte, lr uint) error {
	kanzi.ComputeHistogram(block, this.freqs[:], true, false)
	_, err := this.updateFrequencies(this.freqs[:], len(block), lr)
	return err
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
		if (this.low^(this.low+this.rng))&RANGE_MASK != 0 {
			if this.rng > BOTTOM_RANGE {
				break
			}

			// Normalize
			this.rng = -this.low & BOTTOM_RANGE
		}

		this.bitstream.WriteBits(this.low>>32, 28)
		this.rng <<= 28
		this.low <<= 28
	}

}

func (this *RangeEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

func (this *RangeEncoder) Dispose() {
}

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

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewRangeDecoder(bs) or NewRangeDecoder(bs, 16384, 14)
// The default chunk size is 65536 bytes.
func NewRangeDecoder(bs kanzi.InputBitStream, args ...uint) (*RangeDecoder, error) {
	if bs == nil {
		return nil, errors.New("Range codec: Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("Range codec: At most one chunk size can be provided")
	}

	chkSize := DEFAULT_RANGE_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("Range codec: The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("Range codec: The chunk size must be at most 2^30")
	}

	this := new(RangeDecoder)
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
		return alphabetSize, nil
	}

	if alphabetSize != 256 {
		for i := range frequencies {
			frequencies[i] = 0
		}
	}

	// Decode frequencies
	logRange := uint(8 + this.bitstream.ReadBits(3))
	scale := 1 << logRange
	this.shift = logRange
	sum := 0
	chkSize := 12

	if alphabetSize < 64 {
		chkSize = 6
	}

	llr := uint(3)

	for 1<<llr <= logRange {
		llr++
	}

	// Decode all frequencies (but the first one)
	for i := 1; i < alphabetSize; i += chkSize {
		logMax := uint(1 + this.bitstream.ReadBits(llr))
		endj := i + chkSize

		if endj > alphabetSize {
			endj = alphabetSize
		}

		// Read frequencies
		for j := i; j < endj; j++ {
			val := int(this.bitstream.ReadBits(logMax))

			if val <= 0 || val >= scale {
				err := fmt.Errorf("Invalid bitstream: incorrect frequency %v for symbol '%v' in range decoder", val, this.alphabet[j])
				return alphabetSize, err
			}

			frequencies[this.alphabet[j]] = val
			sum += val
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

// Reset frequency stats for each chunk of data in the block
func (this *RangeDecoder) Read(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Range codec: Invalid null block parameter")
	}

	end := len(block)
	startChunk := 0
	sizeChunk := int(this.chunkSize)

	if sizeChunk == 0 {
		sizeChunk = len(block)
	}

	for startChunk < end {
		alphabetSize, err := this.decodeHeader(this.freqs[:])

		if err != nil || alphabetSize == 0 {
			return startChunk, err
		}

		this.rng = TOP_RANGE
		this.low = 0
		this.code = this.bitstream.ReadBits(60)
		endChunk := startChunk + sizeChunk

		if endChunk > end {
			endChunk = end
		}

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
		if (this.low^(this.low+this.rng))&RANGE_MASK != 0 {
			if this.rng > BOTTOM_RANGE {
				break
			}

			// Normalize
			this.rng = -this.low & BOTTOM_RANGE
		}

		this.code = (this.code << 28) | this.bitstream.ReadBits(28)
		this.rng <<= 28
		this.low <<= 28
	}

	return byte(symbol)
}

func (this *RangeDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *RangeDecoder) Dispose() {
}
