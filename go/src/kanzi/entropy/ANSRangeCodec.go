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
	"kanzi"
)

// Implementation of an Asymmetric Numeral System codec.
// See "Asymmetric Numeral System" by Jarek Duda at http://arxiv.org/abs/0902.0271
// For alternate C implementation examples, see https://github.com/Cyan4973/FiniteStateEntropy
// and https://github.com/rygorous/ryg_rans

const (
	ANS_TOP                = uint64(1) << 24
	DEFAULT_ANS_CHUNK_SIZE = uint(1 << 16) // 64 KB by default
	DEFAULT_ANS_LOG_RANGE  = uint(13)
)

type ANSRangeEncoder struct {
	bitstream kanzi.OutputBitStream
	freqs     []int
	cumFreqs  []int
	alphabet  []int
	buffer    []int32
	eu        *EntropyUtils
	chunkSize int
	logRange  uint
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewANSRangeEncoder(bs) or NewANSRangeEncoder(bs, 16384, 14)
// The default chunk size is 65536 bytes.
func NewANSRangeEncoder(bs kanzi.OutputBitStream, args ...uint) (*ANSRangeEncoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 2 {
		return nil, errors.New("At most one chunk size and one log range can be provided")
	}

	chkSize := DEFAULT_ANS_CHUNK_SIZE
	logRange := DEFAULT_ANS_LOG_RANGE

	if len(args) == 2 {
		chkSize = args[0]
		logRange = args[1]
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("The chunk size must be at most 2^30")
	}

	if logRange < 8 || logRange > 16 {
		return nil, fmt.Errorf("Invalid range parameter: %v (must be in [8..16])", logRange)
	}

	this := new(ANSRangeEncoder)
	this.bitstream = bs
	this.alphabet = make([]int, 256)
	this.freqs = make([]int, 256)
	this.cumFreqs = make([]int, 257)
	this.buffer = make([]int32, 0)
	this.logRange = logRange
	this.chunkSize = int(chkSize)
	var err error
	this.eu, err = NewEntropyUtils()
	return this, err
}

func (this *ANSRangeEncoder) updateFrequencies(frequencies []int, size int, lr uint) (int, error) {
	if frequencies == nil || len(frequencies) != 256 {
		return 0, errors.New("Invalid frequencies parameter")
	}

	alphabetSize, err := this.eu.NormalizeFrequencies(frequencies, this.alphabet, size, 1<<lr)

	if err == nil {
		if alphabetSize > 0 {
			this.cumFreqs[0] = 0

			// Create histogram of frequencies scaled to 'range'
			for i := range frequencies {
				this.cumFreqs[i+1] = this.cumFreqs[i] + frequencies[i]
			}
		}

		this.encodeHeader(alphabetSize, this.alphabet, frequencies, lr)
	}

	return alphabetSize, err
}

func (this *ANSRangeEncoder) encodeHeader(alphabetSize int, alphabet []int, frequencies []int, lr uint) bool {
	EncodeAlphabet(this.bitstream, alphabet[0:alphabetSize])

	if alphabetSize == 0 {
		return true
	}

	this.bitstream.WriteBits(uint64(lr-8), 3) // logRange
	inc := 16

	if alphabetSize <= 64 {
		inc = 8
	}

	llr := uint(3)

	for 1<<llr <= lr {
		llr++
	}

	/// Encode all frequencies (but the first one) by chunks of size 'inc'
	for i := 1; i < alphabetSize; i += inc {
		max := 0
		logMax := uint(1)
		endj := i + inc

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

	return true
}

// Dynamically compute the frequencies for every chunk of data in the block
func (this *ANSRangeEncoder) Encode(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	sizeChunk := this.chunkSize

	if sizeChunk == 0 {
		sizeChunk = len(block)
	}

	end := len(block)
	startChunk := 0

	if 4*len(this.buffer) < sizeChunk+3 {
		this.buffer = make([]int32, (sizeChunk+3)>>2)
	}

	for startChunk < end {
		endChunk := startChunk + sizeChunk
		lr := this.logRange

		if endChunk > end {
			endChunk = end
		}

		// Lower log range if the size of the data block is small
		for lr > 8 && 1<<lr > endChunk-startChunk {
			lr--
		}

		if err := this.rebuildStatistics(block[startChunk:endChunk], lr); err != nil {
			return startChunk, err
		}

		this.encodeChunk(block[startChunk:endChunk], lr)
		startChunk = endChunk
	}

	return end, nil
}

func (this *ANSRangeEncoder) encodeChunk(block []byte, lr uint) {
	top := (ANS_TOP >> lr) << 32
	st := ANS_TOP
	n := 0

	// Encoding works in reverse
	for i := len(block) - 1; i >= 0; i-- {
		symbol := block[i]
		freq := uint64(this.freqs[symbol])

		// Normalize
		if st >= top*freq {
			this.buffer[n] = int32(st)
			n++
			st >>= 32
		}

		// Compute next ANS state
		// C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
		st = ((st / freq) << lr) + (st % freq) + uint64(this.cumFreqs[symbol])
	}

	// Write final ANS state
	this.bitstream.WriteBits(st, 64)

	// Write encoded data to bitstream
	for n--; n >= 0; n-- {
		this.bitstream.WriteBits(uint64(this.buffer[n]), 32)
	}
}

// Compute chunk frequencies, cumulated frequencies and encode chunk header
func (this *ANSRangeEncoder) rebuildStatistics(block []byte, lr uint) error {
	for i := range this.freqs {
		this.freqs[i] = 0
	}

	for i := range block {
		this.freqs[block[i]]++
	}

	// Rebuild statistics
	if _, err := this.updateFrequencies(this.freqs, len(block), lr); err != nil {
		return err
	}

	return nil
}

func (this *ANSRangeEncoder) Dispose() {
}

func (this *ANSRangeEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

type ANSRangeDecoder struct {
	bitstream kanzi.InputBitStream
	freqs     []int
	cumFreqs  []int
	f2s       []byte // mapping frequency -> symbol
	alphabet  []int
	chunkSize int
	logRange  uint
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewANSRangeDecoder(bs) or NewANSRangeDecoder(bs, 16384, 14)
// The default chunk size is 65536 bytes.
func NewANSRangeDecoder(bs kanzi.InputBitStream, args ...uint) (*ANSRangeDecoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("At most one chunk size can be provided")
	}

	chkSize := DEFAULT_ANS_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("The chunk size must be at most 2^30")
	}

	this := new(ANSRangeDecoder)
	this.bitstream = bs
	this.alphabet = make([]int, 256)
	this.freqs = make([]int, 256)
	this.cumFreqs = make([]int, 257)
	this.f2s = make([]byte, 0)
	this.chunkSize = int(chkSize)
	return this, nil
}

func (this *ANSRangeDecoder) decodeHeader(frequencies []int) (int, error) {
	alphabetSize, err := DecodeAlphabet(this.bitstream, this.alphabet)

	if err != nil || alphabetSize == 0 {
		return alphabetSize, nil
	}

	if alphabetSize != 256 {
		for i := range frequencies {
			frequencies[i] = 0
		}
	}

	// Decode frequencies
	this.logRange = uint(8 + this.bitstream.ReadBits(3))
	scale := 1 << this.logRange
	sum := 0
	inc := 16
	llr := uint(3)

	if alphabetSize <= 64 {
		inc = 8
	}

	for 1<<llr <= this.logRange {
		llr++
	}

	// Decode all frequencies (but the first one) by chunks of size 'inc'
	for i := 1; i < alphabetSize; i += inc {
		logMax := uint(1 + this.bitstream.ReadBits(llr))
		endj := i + inc

		if endj > alphabetSize {
			endj = alphabetSize
		}

		// Read frequencies
		for j := i; j < endj; j++ {
			freq := int(this.bitstream.ReadBits(logMax))

			if freq <= 0 || freq >= scale {
				error := fmt.Errorf("Invalid bitstream: incorrect frequency %v for symbol '%v' in ANS range decoder", freq, this.alphabet[j])
				return alphabetSize, error
			}

			frequencies[this.alphabet[j]] = freq
			sum += freq
		}
	}

	// Infer first frequency
	if scale <= sum {
		error := fmt.Errorf("Invalid bitstream: incorrect frequency %v for symbol '%v' in ANS range decoder", frequencies[this.alphabet[0]], this.alphabet[0])
		return alphabetSize, error
	}

	frequencies[this.alphabet[0]] = scale - sum
	this.cumFreqs[0] = 0

	if len(this.f2s) < scale {
		this.f2s = make([]byte, scale)
	}

	// Create histogram of frequencies scaled to 'range' and reverse mapping
	for i := 0; i < 256; i++ {
		this.cumFreqs[i+1] = this.cumFreqs[i] + frequencies[i]
		base := int(this.cumFreqs[i])

		for j := frequencies[i] - 1; j >= 0; j-- {
			this.f2s[base+j] = byte(i)
		}
	}

	return alphabetSize, nil
}

func (this *ANSRangeDecoder) Decode(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	end := len(block)
	startChunk := 0
	sizeChunk := this.chunkSize

	if sizeChunk == 0 {
		sizeChunk = len(block)
	}

	for startChunk < end {
		alphabetSize, err := this.decodeHeader(this.freqs)

		if err != nil || alphabetSize == 0 {
			return startChunk, err
		}

		endChunk := startChunk + sizeChunk

		if endChunk > end {
			endChunk = end
		}

		this.decodeChunk(block[startChunk:endChunk])
		startChunk = endChunk
	}

	return len(block), nil
}

func (this *ANSRangeDecoder) decodeChunk(block []byte) {
	// Read initial ANS state
	st := this.bitstream.ReadBits(64)
	mask := (uint64(1) << this.logRange) - 1

	for i := range block {
		idx := int(st & mask)
		symbol := this.f2s[idx]
		block[i] = symbol

		// Compute next ANS state
		// D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
		st = uint64(this.freqs[symbol])*(st>>this.logRange) + uint64(idx-this.cumFreqs[symbol])

		// Normalize
		for st < ANS_TOP {
			st = (st << 32) | this.bitstream.ReadBits(32)
		}
	}
}

func (this *ANSRangeDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *ANSRangeDecoder) Dispose() {
}
