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
	"sort"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	HUF_DECODING_BATCH_SIZE = 12 // in bits
	HUF_DECODING_MASK       = (1 << HUF_DECODING_BATCH_SIZE) - 1
	HUF_MAX_DECODING_INDEX  = (HUF_DECODING_BATCH_SIZE << 8) | 0xFF
	HUF_MAX_CHUNK_SIZE      = uint(1 << 16)
	HUF_SYMBOL_ABSENT       = (1 << 31) - 1
	HUF_MAX_SYMBOL_SIZE     = 20
	HUF_BUFFER_SIZE         = (HUF_MAX_SYMBOL_SIZE << 8) + 256
)

// Utilities

type codeLengthComparator struct {
	ranks []int
	sizes []byte
}

func byIncreasingFrequency(ranks []int, frequencies []int) frequencyComparator {
	return frequencyComparator{ranks: ranks, frequencies: frequencies}
}

type frequencyComparator struct {
	ranks       []int
	frequencies []int
}

func (this frequencyComparator) Less(i, j int) bool {
	// Check frequency (natural order) as first key
	ri := this.ranks[i]
	rj := this.ranks[j]

	if this.frequencies[ri] != this.frequencies[rj] {
		return this.frequencies[ri] < this.frequencies[rj]
	}

	// Check index (natural order) as second key
	return ri < rj
}

func (this frequencyComparator) Len() int {
	return len(this.ranks)
}

func (this frequencyComparator) Swap(i, j int) {
	this.ranks[i], this.ranks[j] = this.ranks[j], this.ranks[i]
}

// Return the number of codes generated
func generateCanonicalCodes(sizes []byte, codes []uint, symbols []int) int {
	count := len(symbols)

	// Sort by increasing size (first key) and increasing value (second key)
	if count > 1 {
		var buf [HUF_BUFFER_SIZE]byte

		for i := 0; i < count; i++ {
			buf[(int(sizes[symbols[i]]-1)<<8)|symbols[i]] = 1
		}

		n := 0

		for i := range buf {
			if buf[i] != 0 {
				symbols[n] = i & 0xFF
				n++

				if n == count {
					break
				}
			}
		}
	}

	code := uint(0)
	length := sizes[symbols[0]]

	for _, s := range symbols {
		if sizes[s] > length {
			code <<= (sizes[s] - length)
			length = sizes[s]

			// Max length reached
			if length > HUF_MAX_SYMBOL_SIZE {
				return -1
			}
		}

		codes[s] = code
		code++
	}

	return count
}

// HuffmanEncoder  Implementation of a static Huffman encoder.
// Uses in place generation of canonical codes instead of a tree
type HuffmanEncoder struct {
	bitstream kanzi.OutputBitStream
	codes     [256]uint
	alphabet  [256]int
	sranks    [256]int
	chunkSize int
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats.
// Since the number of args is variable, this function can be called like this:
// NewHuffmanEncoder(bs) or NewHuffmanEncoder(bs, 16384)
func NewHuffmanEncoder(bs kanzi.OutputBitStream, args ...uint) (*HuffmanEncoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("At most one chunk size can be provided")
	}

	chkSize := HUF_MAX_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > HUF_MAX_CHUNK_SIZE {
		return nil, fmt.Errorf("The chunk size must be at most %d", HUF_MAX_CHUNK_SIZE)
	}

	this := new(HuffmanEncoder)
	this.bitstream = bs
	this.codes = [256]uint{}
	this.alphabet = [256]int{}
	this.sranks = [256]int{}
	this.chunkSize = int(chkSize)

	// Default frequencies, sizes and codes
	for i := 0; i < 256; i++ {
		this.codes[i] = uint(i)
	}

	return this, nil
}

// Rebuild Huffman codes
func (this *HuffmanEncoder) updateFrequencies(frequencies []int) (int, error) {
	if frequencies == nil || len(frequencies) != 256 {
		return 0, errors.New("Invalid frequencies parameter")
	}

	count := 0
	var sizes [256]byte

	for i := range this.codes {
		this.codes[i] = 0

		if frequencies[i] > 0 {
			this.alphabet[count] = i
			count++
		}
	}

	symbols := this.alphabet[0:count]
	EncodeAlphabet(this.bitstream, symbols)

	// Transmit code lengths only, frequencies and codes do not matter
	// Unary encode the length differences
	if err := this.computeCodeLengths(frequencies, sizes[:], count); err != nil {
		return count, err
	}

	egenc, err := NewExpGolombEncoder(this.bitstream, true)

	if err != nil {
		return count, err
	}

	prevSize := byte(2)

	for _, s := range symbols {
		currSize := sizes[s]
		egenc.EncodeByte(currSize - prevSize)
		prevSize = currSize
	}

	// Create canonical codes
	if generateCanonicalCodes(sizes[:], this.codes[:], this.sranks[0:count]) < 0 {
		return count, fmt.Errorf("Could not generate codes: max code length (%v bits) exceeded", HUF_MAX_SYMBOL_SIZE)
	}

	// Pack size and code (size <= HUF_MAX_SYMBOL_SIZE bits)
	for _, s := range symbols {
		this.codes[s] |= (uint(sizes[s]) << 24)
	}

	return count, nil
}

// See [In-Place Calculation of Minimum-Redundancy Codes]
// by Alistair Moffat & Jyrki Katajainen
func (this *HuffmanEncoder) computeCodeLengths(frequencies []int, sizes []byte, count int) error {
	if count == 1 {
		this.sranks[0] = this.alphabet[0]
		sizes[this.alphabet[0]] = 1
		return nil
	}

	// Sort ranks by increasing frequency
	copy(this.sranks[:], this.alphabet[0:count])

	// Sort by increasing frequencies (first key) and increasing value (second key)
	sort.Sort(byIncreasingFrequency(this.sranks[0:count], frequencies))
	var buffer [256]int
	buf := buffer[0:count]

	for i := range buf {
		buf[i] = frequencies[this.sranks[i]]
	}

	computeInPlaceSizesPhase1(buf)
	computeInPlaceSizesPhase2(buf)
	var err error

	for i := range buf {
		codeLen := byte(buf[i])

		if codeLen == 0 || codeLen > HUF_MAX_SYMBOL_SIZE {
			err = fmt.Errorf("Could not generate codes: max code length (%v bits) exceeded", HUF_MAX_SYMBOL_SIZE)
			break
		}

		sizes[this.sranks[i]] = codeLen
	}

	return err
}

func computeInPlaceSizesPhase1(data []int) {
	n := len(data)

	for s, r, t := 0, 0, 0; t < n-1; t++ {
		sum := 0

		for i := 0; i < 2; i++ {
			if s >= n || (r < t && data[r] < data[s]) {
				sum += data[r]
				data[r] = t
				r++
			} else {
				sum += data[s]

				if s > t {
					data[s] = 0
				}

				s++
			}
		}

		data[t] = sum
	}
}

func computeInPlaceSizesPhase2(data []int) {
	n := len(data)
	levelTop := n - 2 //root
	depth := 1
	i := n
	totalNodesAtLevel := 2

	for i > 0 {
		k := levelTop

		for k > 0 && data[k-1] >= levelTop {
			k--
		}

		internalNodesAtLevel := levelTop - k
		leavesAtLevel := totalNodesAtLevel - internalNodesAtLevel

		for j := 0; j < leavesAtLevel; j++ {
			i--
			data[i] = depth
		}

		totalNodesAtLevel = internalNodesAtLevel << 1
		levelTop = k
		depth++
	}
}

// Dynamically compute the frequencies for every chunk of data in the block
func (this *HuffmanEncoder) Encode(block []byte) (int, error) {
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
		sizeChunk = end
	}

	for startChunk < end {
		endChunk := startChunk + sizeChunk

		if endChunk > len(block) {
			endChunk = len(block)
		}

		var frequencies [256]int
		kanzi.ComputeHistogram(block[startChunk:endChunk], frequencies[:], true, false)

		// Rebuild Huffman codes
		if _, err := this.updateFrequencies(frequencies[:]); err != nil {
			return 0, err
		}

		c := this.codes
		bs := this.bitstream
		endChunk3 := 3*((endChunk-startChunk)/3) + startChunk

		for i := startChunk; i < endChunk3; i += 3 {
			// Pack 3 codes into 1 uint64
			code1 := c[block[i]]
			codeLen1 := uint(code1 >> 24)
			code2 := c[block[i+1]]
			codeLen2 := uint(code2 >> 24)
			code3 := c[block[i+2]]
			codeLen3 := uint(code3 >> 24)
			st := (uint64(code1&0xFFFFFF) << (codeLen2 + codeLen3)) |
				(uint64(code2&((1<<codeLen2)-1)) << codeLen3) |
				uint64(code3&((1<<codeLen3)-1))
			bs.WriteBits(st, codeLen1+codeLen2+codeLen3)
		}

		for i := endChunk3; i < endChunk; i++ {
			code := c[block[i]]
			bs.WriteBits(uint64(code), code>>24)
		}

		startChunk = endChunk
	}

	return len(block), nil
}

func (this *HuffmanEncoder) Dispose() {
}

func (this *HuffmanEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// HuffmanDecoder Implementation of a static Huffman decoder.
// Uses tables to decode symbols instead of a tree
type HuffmanDecoder struct {
	bitstream  kanzi.InputBitStream
	codes      [256]uint
	alphabet   [256]int
	sizes      [256]byte
	fdTable    []uint16  // Fast decoding table
	sdTable    [256]uint // Slow decoding table
	sdtIndexes []int     // Indexes for slow decoding table (can be negative)
	chunkSize  int
	state      uint64 // holds bits read from bitstream
	bits       uint   // holds number of unused bits in 'state'
	minCodeLen int8
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats.
// Since the number of args is variable, this function can be called like this:
// NewHuffmanDecoder(bs) or NewHuffmanDecoder(bs, 16384)
func NewHuffmanDecoder(bs kanzi.InputBitStream, args ...uint) (*HuffmanDecoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("At most one chunk size can be provided")
	}

	chkSize := HUF_MAX_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > HUF_MAX_CHUNK_SIZE {
		return nil, fmt.Errorf("The chunk size must be at most %d", HUF_MAX_CHUNK_SIZE)
	}

	this := new(HuffmanDecoder)
	this.bitstream = bs
	this.sizes = [256]byte{}
	this.codes = [256]uint{}
	this.alphabet = [256]int{}
	this.fdTable = make([]uint16, 1<<HUF_DECODING_BATCH_SIZE)
	this.sdTable = [256]uint{}
	this.sdtIndexes = make([]int, HUF_MAX_SYMBOL_SIZE+1)
	this.chunkSize = int(chkSize)
	this.minCodeLen = 8

	// Default lengths & canonical codes
	for i := 0; i < 256; i++ {
		this.sizes[i] = 8
		this.codes[i] = uint(i)
	}

	return this, nil
}

func (this *HuffmanDecoder) ReadLengths() (int, error) {
	count, err := DecodeAlphabet(this.bitstream, this.alphabet[:])

	if count == 0 || err != nil {
		return count, err
	}

	egdec, err := NewExpGolombDecoder(this.bitstream, true)

	if err != nil {
		return 0, err
	}

	var currSize int8
	this.minCodeLen = HUF_MAX_SYMBOL_SIZE // max code length
	prevSize := int8(2)
	symbols := this.alphabet[0:count]

	// Read lengths
	for i, s := range symbols {
		if s > len(this.codes) {
			return 0, fmt.Errorf("Invalid bitstream: incorrect Huffman symbol %v", s)
		}

		this.codes[s] = 0
		currSize = prevSize + int8(egdec.DecodeByte())

		if currSize <= 0 || currSize > HUF_MAX_SYMBOL_SIZE {
			return 0, fmt.Errorf("Invalid bitstream: incorrect size %v for Huffman symbol %v", currSize, i)
		}

		if this.minCodeLen > currSize {
			this.minCodeLen = currSize
		}

		this.sizes[s] = byte(currSize)
		prevSize = currSize
	}

	// Create canonical codes
	if generateCanonicalCodes(this.sizes[:], this.codes[:], symbols) < 0 {
		return count, fmt.Errorf("Could not generate codes: max code length (%v bits) exceeded", HUF_MAX_SYMBOL_SIZE)
	}

	this.buildDecodingTables(count)
	return count, nil
}

// Build decoding tables
// The slow decoding table contains the codes in natural order.
// The fast decoding table contains all the prefixes with DECODING_BATCH_SIZE bits.
func (this *HuffmanDecoder) buildDecodingTables(count int) {
	for i := range this.fdTable {
		this.fdTable[i] = 0
	}

	for i := range this.sdTable {
		this.sdTable[i] = 0
	}

	for i := range this.sdtIndexes {
		this.sdtIndexes[i] = HUF_SYMBOL_ABSENT
	}

	length := byte(0)

	for i := 0; i < count; i++ {
		s := uint(this.alphabet[i])
		code := this.codes[s]

		if this.sizes[s] > length {
			length = this.sizes[s]
			this.sdtIndexes[length] = i - int(code)
		}

		// Fill slow decoding table
		val := (uint(this.sizes[s]) << 8) | s
		this.sdTable[i] = val

		// Fill fast decoding table
		// Find location index in table
		if length < HUF_DECODING_BATCH_SIZE {
			idx := code << (HUF_DECODING_BATCH_SIZE - length)
			end := idx + (1 << (HUF_DECODING_BATCH_SIZE - length))

			// All DECODING_BATCH_SIZE bit values read from the bit stream and
			// starting with the same prefix point to symbol r
			for idx < end {
				this.fdTable[idx] = uint16(val)
				idx++
			}
		} else {
			idx := code >> (length - HUF_DECODING_BATCH_SIZE)
			this.fdTable[idx] = uint16(val)
		}

	}
}

// Use fastDecodeByte until the near end of chunk or block.
func (this *HuffmanDecoder) Decode(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	if this.minCodeLen == 0 {
		return 0, errors.New("Invalid minimum code length: 0")
	}

	end := len(block)
	startChunk := 0
	sizeChunk := this.chunkSize

	if sizeChunk == 0 {
		sizeChunk = len(block)
	}

	for startChunk < end {
		// Reinitialize the Huffman tables
		if r, err := this.ReadLengths(); r == 0 || err != nil {
			return startChunk, err
		}

		endChunk := startChunk + sizeChunk

		if endChunk > end {
			endChunk = end
		}

		// Compute minimum number of bits required in bitstream for fast decoding
		endPaddingSize := 64 / int(this.minCodeLen)

		if int(this.minCodeLen)*endPaddingSize != 64 {
			endPaddingSize++
		}

		endChunk8 := (endChunk - endPaddingSize) & -8

		if endChunk8 < 0 {
			endChunk8 = 0
		}

		for i := startChunk; i < endChunk8; i += 8 {
			// Fast decoding (read HUF_DECODING_BATCH_SIZE bits at a time)
			block[i] = this.fastDecodeByte()
			block[i+1] = this.fastDecodeByte()
			block[i+2] = this.fastDecodeByte()
			block[i+3] = this.fastDecodeByte()
			block[i+4] = this.fastDecodeByte()
			block[i+5] = this.fastDecodeByte()
			block[i+6] = this.fastDecodeByte()
			block[i+7] = this.fastDecodeByte()
		}

		for i := endChunk8; i < endChunk; i++ {
			// Fallback to regular decoding (read one bit at a time)
			block[i] = this.slowDecodeByte(0, 0)
		}

		startChunk = endChunk
	}

	return len(block), nil
}

func (this *HuffmanDecoder) slowDecodeByte(code int, codeLen uint) byte {
	for codeLen < HUF_MAX_SYMBOL_SIZE {
		codeLen++
		code <<= 1

		if this.bits == 0 {
			code |= this.bitstream.ReadBit()
		} else {
			// Consume remaining bits in 'state'
			this.bits--
			code |= int((this.state >> this.bits) & 1)
		}

		idx := this.sdtIndexes[codeLen]

		if idx == HUF_SYMBOL_ABSENT { // No code with this length ?
			continue
		}

		if this.sdTable[idx+code]>>8 == codeLen {
			return byte(this.sdTable[idx+code])
		}
	}

	panic(errors.New("Invalid bitstream: incorrect Huffman code"))
}

// 64 bits must be available in the bitstream
func (this *HuffmanDecoder) fastDecodeByte() byte {
	if this.bits < HUF_DECODING_BATCH_SIZE {
		// Fetch more bits from bitstream
		read := this.bitstream.ReadBits(64 - this.bits)
		// No need to mask this.state because uint64(xyz) << 64 = 0
		this.state = (this.state << (64 - this.bits)) | read
		this.bits = 64
	}

	// Retrieve symbol from fast decoding table
	val := this.fdTable[int(this.state>>(this.bits-HUF_DECODING_BATCH_SIZE))&HUF_DECODING_MASK]

	if val > HUF_MAX_DECODING_INDEX {
		this.bits -= HUF_DECODING_BATCH_SIZE
		return this.slowDecodeByte(int(this.state>>this.bits)&HUF_DECODING_MASK, HUF_DECODING_BATCH_SIZE)
	}

	this.bits -= uint(val >> 8)
	return byte(val)
}

func (this *HuffmanDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *HuffmanDecoder) Dispose() {
}
