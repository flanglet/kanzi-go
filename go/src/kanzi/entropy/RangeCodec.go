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

package entropy

import (
	"errors"
	"fmt"
	"kanzi"
)

const (
	TOP_RANGE                = uint64(0x00FFFFFFFFFFFFFF)
	BOTTOM_RANGE             = uint64(0x000000FFFFFFFFFF)
	MAX_RANGE                = BOTTOM_RANGE + 1
	MASK                     = uint64(0x00FF000000000000)
	NB_SYMBOLS               = 257           //256 + EOF
	DEFAULT_RANGE_CHUNK_SIZE = uint(1 << 16) // 64 KB by default
	LAST                     = NB_SYMBOLS - 1
	BASE_LEN                 = NB_SYMBOLS >> 4
)

type RangeEncoder struct {
	low       uint64
	range_    uint64
	disposed  bool
	baseFreq  []int
	deltaFreq []int
	bitstream kanzi.OutputBitStream
	chunkSize int
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block.
// Since the number of args is variable, this function can be called like this:
// NewRangeEncoder(bs) or NewRangeEncoder(bs, 16384)
// The default chunk size is 65536 bytes.
func NewRangeEncoder(bs kanzi.OutputBitStream, args ...uint) (*RangeEncoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("At most one chunk size can be provided")
	}

	chkSize := DEFAULT_RANGE_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("The chunk size must be at most 2^30")
	}

	this := new(RangeEncoder)
	this.range_ = TOP_RANGE
	this.bitstream = bs
	this.chunkSize = int(chkSize)

	// Since the frequency update after each byte encoded is the bottleneck,
	// split the frequency table into an array of absolute frequencies (with
	// indexes multiple of 16) and delta frequencies (relative to the previous
	// absolute frequency) with indexes in the [0..15] range
	this.deltaFreq = make([]int, NB_SYMBOLS+1)
	this.baseFreq = make([]int, BASE_LEN+1)
	this.ResetFrequencies()
	return this, nil
}

func (this *RangeEncoder) ResetFrequencies() {
	for i := 0; i <= NB_SYMBOLS; i++ {
		this.deltaFreq[i] = i & 15 // DELTA
	}

	for i := 0; i <= BASE_LEN; i++ {
		this.baseFreq[i] = i << 4 // BASE
	}

}

// This method is on the speed critical path (called for each byte)
// The speed optimization is focused on reducing the frequency table update
func (this *RangeEncoder) EncodeByte(b byte) {
	value := int(b)
	symbolLow := uint64(this.baseFreq[value>>4] + this.deltaFreq[value])
	symbolHigh := uint64(this.baseFreq[(value+1)>>4] + this.deltaFreq[value+1])
	this.range_ /= uint64(this.baseFreq[BASE_LEN] + this.deltaFreq[NB_SYMBOLS])

	// Encode symbol
	this.low += (symbolLow * this.range_)
	this.range_ *= (symbolHigh - symbolLow)

	// If the left-most digits are the same throughout the range, write bits to bitstream
	for {
		if (this.low^(this.low+this.range_))&MASK != 0 {
			if this.range_ >= MAX_RANGE {
				break
			} else {
				// Normalize
				this.range_ = -this.low & BOTTOM_RANGE
			}
		}

		this.bitstream.WriteBits(this.low>>48, 8)
		this.range_ <<= 8
		this.low <<= 8
	}

	this.updateFrequencies(int(value + 1))
}

func (this *RangeEncoder) updateFrequencies(value int) {
	start := (value + 15) >> 4

	// Update absolute frequencies
	for j := start; j <= BASE_LEN; j++ {
		this.baseFreq[j]++
	}

	// Update relative frequencies (in the 'right' segment only)
	for j := value; j < (start << 4); j++ {
		this.deltaFreq[j]++
	}
}

func (this *RangeEncoder) Encode(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Invalid null block parameter")
	}

	end := len(block)
	startChunk := 0
	sizeChunk := this.chunkSize

	if sizeChunk == 0 {
		sizeChunk = end
	}

	if startChunk+sizeChunk >= end {
		sizeChunk = end - startChunk
	}

	endChunk := startChunk + sizeChunk

	for startChunk < end {
		this.ResetFrequencies()

		for i := startChunk; i < endChunk; i++ {
			this.EncodeByte(block[i])
		}

		startChunk = endChunk

		if startChunk+sizeChunk >= end {
			sizeChunk = end - startChunk
		}

		endChunk = startChunk + sizeChunk
	}

	return len(block), nil
}

func (this *RangeEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

func (this *RangeEncoder) Dispose() {
	if this.disposed == true {
		return
	}

	this.disposed = true
	this.bitstream.WriteBits(this.low, 56)
}

type RangeDecoder struct {
	code        uint64
	low         uint64
	range_      uint64
	baseFreq    []int
	deltaFreq   []int
	initialized bool
	bitstream   kanzi.InputBitStream
	chunkSize   int
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewRangeDecoder(bs) or NewRangeDecoder(bs, 16384)
// The default chunk size is 65536 bytes.
func NewRangeDecoder(bs kanzi.InputBitStream, args ...uint) (*RangeDecoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("At most one chunk size can be provided")
	}

	chkSize := DEFAULT_RANGE_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("The chunk size must be at most 2^30")
	}

	this := new(RangeDecoder)
	this.range_ = TOP_RANGE
	this.bitstream = bs
	this.chunkSize = int(chkSize)

	// Since the frequency update after each byte encoded is the bottleneck,
	// split the frequency table into an array of absolute frequencies (with
	// indexes multiple of 16) and delta frequencies (relative to the previous
	// absolute frequency) with indexes in the [0..15] range
	this.deltaFreq = make([]int, NB_SYMBOLS+1)
	this.baseFreq = make([]int, BASE_LEN+1)
	this.ResetFrequencies()

	return this, nil
}

func (this *RangeDecoder) ResetFrequencies() {
	for i := 0; i <= NB_SYMBOLS; i++ {
		this.deltaFreq[i] = i & 15 // DELTA
	}

	for i := 0; i <= BASE_LEN; i++ {
		this.baseFreq[i] = i << 4 // BASE
	}

}

func (this *RangeDecoder) Initialized() bool {
	return this.initialized
}

func (this *RangeDecoder) Initialize() {
	if this.initialized == true {
		return
	}

	this.initialized = true
	this.code = this.bitstream.ReadBits(56)
}

func (this *RangeDecoder) DecodeByte() byte {
	if this.initialized == false {
		this.Initialize()
	}

	return this.decodeByte_()
}

// This method is on the speed critical path (called for each byte)
// The speed optimization is focused on reducing the frequency table update
func (this *RangeDecoder) decodeByte_() byte {
	bfreq := this.baseFreq  // alias
	dfreq := this.deltaFreq // alias
	this.range_ /= uint64(bfreq[BASE_LEN] + dfreq[NB_SYMBOLS])
	count := int((this.code - this.low) / this.range_)

	// Find first frequency less than 'count'
	value := this.findSymbol(count)

	if value == LAST {
		more, err := this.bitstream.HasMoreToRead()

		if err != nil {
			panic(err)
		}

		if more == false {
			panic(errors.New("End of bitstream"))
		}

		errMsg := fmt.Sprintf("Unknown symbol: %d", value)
		panic(errors.New(errMsg))
	}

	symbolLow := uint64(bfreq[value>>4] + dfreq[value])
	symbolHigh := uint64(bfreq[(value+1)>>4] + dfreq[value+1])

	// Decode symbol
	this.low += (symbolLow * this.range_)
	this.range_ *= (symbolHigh - symbolLow)

	for {
		if (this.low^(this.low+this.range_))&MASK != 0 {
			if this.range_ >= MAX_RANGE {
				break
			} else {
				// Normalize
				this.range_ = -this.low & BOTTOM_RANGE
			}
		}

		this.code = (this.code << 8) | this.bitstream.ReadBits(8)
		this.range_ <<= 8
		this.low <<= 8
	}

	this.updateFrequencies(value + 1)
	return byte(value)
}

func (this *RangeDecoder) findSymbol(freq int) int {
	bfreq := this.baseFreq  // alias
	dfreq := this.deltaFreq // alias
	var value int

	if freq < dfreq[len(bfreq)/2] {
		value = len(bfreq)/2 - 1
	} else {
		value = len(bfreq) - 1
	}

	for value > 0 && freq < bfreq[value] {
		value--
	}

	freq -= bfreq[value]
	value <<= 4

	if freq > 0 {
		end := value

		if freq < dfreq[value+8] {
			if freq < dfreq[value+4] {
				value += 3
			} else {
				value += 7
			}
		} else {
			if freq < dfreq[value+12] {
				value += 11
			} else {
				value += 15
			}
		}

		for value > end && freq < dfreq[value] {
			value--
		}
	}

	return value
}

func (this *RangeDecoder) updateFrequencies(value int) {
	start := (value + 15) >> 4

	// Update absolute frequencies
	for j := start; j <= BASE_LEN; j++ {
		this.baseFreq[j]++
	}

	// Update relative frequencies (in the 'right' segment only)
	for j := value; j < (start << 4); j++ {
		this.deltaFreq[j]++
	}
}

// Initialize once (if necessary) at the beginning, the use the faster decodeByte_()
// Reset frequency stats for each chunk of data in the block
func (this *RangeDecoder) Decode(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Invalid null block parameter")
	}

	// Deferred initialization: the bitstream may not be ready at build time
	// Initialize 'current' with bytes read from the bitstream
	if this.Initialized() == false {
		this.Initialize()
	}

	end := len(block)
	startChunk := 0
	sizeChunk := this.chunkSize

	if sizeChunk == 0 {
		sizeChunk = len(block)
	}

	if startChunk+sizeChunk >= end {
		sizeChunk = end - startChunk
	}

	endChunk := startChunk + sizeChunk

	for startChunk < end {
		this.ResetFrequencies()

		for i := startChunk; i < endChunk; i++ {
			block[i] = this.decodeByte_()
		}

		startChunk = endChunk

		if startChunk+sizeChunk >= end {
			sizeChunk = end - startChunk
		}

		endChunk = startChunk + sizeChunk
	}

	return len(block), nil
}

func (this *RangeDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *RangeDecoder) Dispose() {
}
