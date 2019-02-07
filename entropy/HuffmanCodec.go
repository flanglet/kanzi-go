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
	HUF_MAX_CHUNK_SIZE      = uint(1 << 15)
	HUF_MAX_SYMBOL_SIZE     = 18
	HUF_BUFFER_SIZE         = (HUF_MAX_SYMBOL_SIZE << 8) + 256
	HUF_DECODING_MASK0      = (1 << HUF_DECODING_BATCH_SIZE) - 1
	HUF_DECODING_MASK1      = (1 << (HUF_MAX_SYMBOL_SIZE + 1)) - 1
)

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
		return nil, errors.New("Huffman codec: Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("Huffman codec: At most one chunk size can be provided")
	}

	chkSize := HUF_MAX_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]

		if chkSize < 1024 {
			return nil, errors.New("Huffman codec: The chunk size must be at least 1024")
		}

		if chkSize > HUF_MAX_CHUNK_SIZE {
			return nil, fmt.Errorf("Huffman codec: The chunk size must be at most %d", HUF_MAX_CHUNK_SIZE)
		}
	}

	this := new(HuffmanEncoder)
	this.bitstream = bs
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
		return 0, errors.New("Huffman codec: Invalid frequencies parameter")
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

	if generateCanonicalCodes(sizes[:], this.codes[:], this.sranks[0:count]) < 0 {
		return count, fmt.Errorf("Could not generate Huffman codes: max code length (%v bits) exceeded", HUF_MAX_SYMBOL_SIZE)
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

	// Sort ranks by increasing frequencies (first key) and increasing value (second key)
	for i := 0; i < count; i++ {
		this.sranks[i] = (frequencies[this.alphabet[i]] << 8) | this.alphabet[i]
	}

	sort.Ints(this.sranks[0:count])
	buf := make([]int, count)

	for i := range buf {
		buf[i] = this.sranks[i] >> 8
		this.sranks[i] &= 0xFF
	}

	computeInPlaceSizesPhase1(buf)
	computeInPlaceSizesPhase2(buf)
	var err error

	for i := range buf {
		codeLen := byte(buf[i])

		if codeLen == 0 || codeLen > HUF_MAX_SYMBOL_SIZE {
			err = fmt.Errorf("Could not generate Huffman codes: max code length (%v bits) exceeded", HUF_MAX_SYMBOL_SIZE)
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
func (this *HuffmanEncoder) Write(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Huffman codec: Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	end := len(block)
	startChunk := 0

	for startChunk < end {
		endChunk := startChunk + this.chunkSize

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
// Uses tables to decode symbols
type HuffmanDecoder struct {
	bitstream  kanzi.InputBitStream
	codes      [256]uint
	alphabet   [256]int
	sizes      [256]byte
	table0     []uint16 // small decoding table: code -> size, symbol
	table1     []uint16 // big decoding table: code -> size, symbol
	chunkSize  int
	state      uint64 // holds bits read from bitstream
	bits       uint16 // holds number of unused bits in 'state'
	minCodeLen byte
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats.
// Since the number of args is variable, this function can be called like this:
// NewHuffmanDecoder(bs) or NewHuffmanDecoder(bs, 16384)
func NewHuffmanDecoder(bs kanzi.InputBitStream, args ...uint) (*HuffmanDecoder, error) {
	if bs == nil {
		return nil, errors.New("Huffman codec: Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("Huffman codec: At most one chunk size can be provided")
	}

	chkSize := HUF_MAX_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]

		if chkSize < 1024 {
			return nil, errors.New("Huffman codec: The chunk size must be at least 1024")
		}

		if chkSize > HUF_MAX_CHUNK_SIZE {
			return nil, fmt.Errorf("Huffman codec: The chunk size must be at most %d", HUF_MAX_CHUNK_SIZE)
		}
	}

	this := new(HuffmanDecoder)
	this.bitstream = bs
	this.table0 = make([]uint16, 1<<HUF_DECODING_BATCH_SIZE)
	this.table1 = make([]uint16, 1<<(HUF_MAX_SYMBOL_SIZE+1))
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

	prevSize := int8(2)
	symbols := this.alphabet[0:count]

	// Read lengths
	for i, s := range symbols {
		if s > len(this.codes) {
			return 0, fmt.Errorf("Invalid bitstream: incorrect Huffman symbol %v", s)
		}

		this.codes[s] = 0
		currSize := prevSize + int8(egdec.DecodeByte())

		if currSize <= 0 || currSize > HUF_MAX_SYMBOL_SIZE {
			return 0, fmt.Errorf("Invalid bitstream: incorrect size %v for Huffman symbol %v", currSize, i)
		}

		this.sizes[s] = byte(currSize)
		prevSize = currSize
	}

	if generateCanonicalCodes(this.sizes[:], this.codes[:], symbols) < 0 {
		return count, fmt.Errorf("Could not generate Huffman codes: max code length (%v bits) exceeded", HUF_MAX_SYMBOL_SIZE)
	}

	this.buildDecodingTables(count)
	return count, nil
}

func (this *HuffmanDecoder) buildDecodingTables(count int) {
	for i := range this.table0 {
		this.table0[i] = 0
	}

	this.minCodeLen = this.sizes[this.alphabet[0]]
	maxSize := this.sizes[this.alphabet[count-1]]
	t1 := this.table1[0 : 2<<maxSize]

	for i := range t1 {
		t1[i] = 0
	}

	length := byte(0)

	for i := 0; i < count; i++ {
		s := uint(this.alphabet[i])

		if this.sizes[s] > length {
			length = this.sizes[s]
		}

		// code -> size, symbol
		val := (uint(this.sizes[s]) << 8) | s
		code := this.codes[s]
		this.table1[code] = uint16(val)

		// All DECODING_BATCH_SIZE bit values read from the bit stream and
		// starting with the same prefix point to symbol s
		if length <= HUF_DECODING_BATCH_SIZE {
			idx := code << (HUF_DECODING_BATCH_SIZE - length)
			end := (code + 1) << (HUF_DECODING_BATCH_SIZE - length)

			for idx < end {
				this.table0[idx] = uint16(val)
				idx++
			}
		} else {
			idx := code << (HUF_MAX_SYMBOL_SIZE + 1 - length)
			end := (code + 1) << (HUF_MAX_SYMBOL_SIZE + 1 - length)

			for idx < end {
				this.table1[idx] = uint16(val)
				idx++
			}
		}

	}
}

// Use fastDecodeByte until the near end of chunk or block.
func (this *HuffmanDecoder) Read(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Huffman codec: Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	if this.minCodeLen == 0 {
		return 0, errors.New("Huffman codec: Invalid minimum code length: 0")
	}

	end := len(block)
	startChunk := 0

	for startChunk < end {
		// Reinitialize the Huffman tables
		if r, err := this.ReadLengths(); r == 0 || err != nil {
			return startChunk, err
		}

		endChunk := startChunk + this.chunkSize

		if endChunk > end {
			endChunk = end
		}

		// Compute minimum number of bits required in bitstream for fast decoding
		endPaddingSize := 64 / int(this.minCodeLen)

		if int(this.minCodeLen)*endPaddingSize != 64 {
			endPaddingSize++
		}

		endChunk8 := startChunk

		if endChunk > startChunk+endPaddingSize {
			endChunk8 += ((endChunk - endPaddingSize - startChunk) & -8)
		}

		// Fast decoding
		for i := startChunk; i < endChunk8; i += 8 {
			block[i] = this.fastDecodeByte()
			block[i+1] = this.fastDecodeByte()
			block[i+2] = this.fastDecodeByte()
			block[i+3] = this.fastDecodeByte()
			block[i+4] = this.fastDecodeByte()
			block[i+5] = this.fastDecodeByte()
			block[i+6] = this.fastDecodeByte()
			block[i+7] = this.fastDecodeByte()
		}

		// Fallback to regular decoding (read one bit at a time)
		for i := endChunk8; i < endChunk; i++ {
			block[i] = this.slowDecodeByte()
		}

		startChunk = endChunk
	}

	return len(block), nil
}

func (this *HuffmanDecoder) slowDecodeByte() byte {
	code := 0
	codeLen := uint16(0)

	for codeLen < HUF_MAX_SYMBOL_SIZE {
		codeLen++
		code <<= 1

		if this.bits == 0 {
			code |= this.bitstream.ReadBit()
		} else {
			this.bits--
			code |= int((this.state >> this.bits) & 1)
		}

		if (this.table1[code] >> 8) == codeLen {
			return byte(this.table1[code])
		}
	}

	panic(errors.New("Invalid bitstream: incorrect Huffman code"))
}

func (this *HuffmanDecoder) fastDecodeByte() byte {
	if this.bits < HUF_DECODING_BATCH_SIZE {
		read := this.bitstream.ReadBits(uint(64 - this.bits))
		this.state = (this.state << (64 - this.bits)) | read
		this.bits = 64
	}

	// Use small table
	val := this.table0[int(this.state>>(this.bits-HUF_DECODING_BATCH_SIZE))&HUF_DECODING_MASK0]

	if val == 0 {
		if this.bits < HUF_MAX_SYMBOL_SIZE+1 {
			read := this.bitstream.ReadBits(uint(64 - this.bits))
			this.state = (this.state << (64 - this.bits)) | read
			this.bits = 64
		}

		// Fallback to big table
		val = this.table1[int(this.state>>(this.bits-HUF_MAX_SYMBOL_SIZE-1))&HUF_DECODING_MASK1]
	}

	this.bits -= (val >> 8)
	return byte(val)
}

func (this *HuffmanDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *HuffmanDecoder) Dispose() {
}
