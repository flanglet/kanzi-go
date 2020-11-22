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
	_HUF_LOG_MAX_CHUNK_SIZE  = 14
	_HUF_MAX_CHUNK_SIZE      = uint(1 << _HUF_LOG_MAX_CHUNK_SIZE)
	_HUF_MAX_SYMBOL_SIZE     = _HUF_LOG_MAX_CHUNK_SIZE
	_HUF_DECODING_BATCH_SIZE = 14 // ensures decoding table fits in L1 cache
	_HUF_BUFFER_SIZE         = uint(_HUF_MAX_SYMBOL_SIZE<<8) + 256
	_HUF_DECODING_MASK       = (1 << _HUF_DECODING_BATCH_SIZE) - 1
)

// Return the number of codes generated
func generateCanonicalCodes(sizes []byte, codes []uint, symbols []int) (int, error) {
	count := len(symbols)

	// Sort by increasing size (first key) and increasing value (second key)
	if count > 1 {
		var buf [_HUF_BUFFER_SIZE]byte

		for i := 0; i < count; i++ {
			s := symbols[i]

			if s > 255 {
				return -1, errors.New("Could not generate Huffman codes: invalid code length")
			}

			// Max length reached
			if sizes[s] > _HUF_MAX_SYMBOL_SIZE {
				return -1, fmt.Errorf("Could not generate Huffman codes: max code length (%d bits) exceeded", _HUF_MAX_SYMBOL_SIZE)
			}

			buf[(int(sizes[s]-1)<<8)|s] = 1
		}

		n := 0

		for i := range &buf {
			if buf[i] == 0 {
				continue
			}

			symbols[n] = i & 0xFF
			n++

			if n == count {
				break
			}
		}
	}

	code := uint(0)
	length := sizes[symbols[0]]

	for _, s := range symbols {
		if sizes[s] > length {
			code <<= (sizes[s] - length)
			length = sizes[s]
		}

		codes[s] = code
		code++
	}

	return count, nil
}

// HuffmanEncoder  Implementation of a static Huffman encoder.
// Uses in place generation of canonical codes instead of a tree
type HuffmanEncoder struct {
	bitstream  kanzi.OutputBitStream
	codes      [256]uint
	alphabet   [256]int
	sranks     [256]int
	chunkSize  int
	maxCodeLen int
}

// NewHuffmanEncoder creates an instance of HuffmanEncoder.
// Since the number of args is variable, this function can be called like this:
// NewHuffmanEncoder(bs) or NewHuffmanEncoder(bs, 16384) (the second argument
// being the chunk size)
func NewHuffmanEncoder(bs kanzi.OutputBitStream, args ...uint) (*HuffmanEncoder, error) {
	if bs == nil {
		return nil, errors.New("Huffman codec: Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("Huffman codec: At most one chunk size can be provided")
	}

	chkSize := _HUF_MAX_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]

		if chkSize < 1024 {
			return nil, errors.New("Huffman codec: The chunk size must be at least 1024")
		}

		if chkSize > _HUF_MAX_CHUNK_SIZE {
			return nil, fmt.Errorf("Huffman codec: The chunk size must be at most %d", _HUF_MAX_CHUNK_SIZE)
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

	for i := range &this.codes {
		this.codes[i] = 0

		if frequencies[i] > 0 {
			this.alphabet[count] = i
			count++
		}
	}

	symbols := this.alphabet[0:count]

	if _, err := EncodeAlphabet(this.bitstream, symbols); err != nil {
		return count, err
	}

	retries := uint(0)

	for {
		if err := this.computeCodeLengths(frequencies, sizes[:], count); err != nil {
			return count, err
		}

		if this.maxCodeLen <= _HUF_MAX_SYMBOL_SIZE {
			// Usual case
			if _, err := generateCanonicalCodes(sizes[:], this.codes[:], this.sranks[0:count]); err != nil {
				return count, err
			}

			break
		}

		// Rare: some codes exceed the budget for the max code length => normalize
		// frequencies (it boosts the smallest frequencies) and try once more.
		if retries > 2 {
			return count, fmt.Errorf("Could not generate Huffman codes: max code length (%d bits) exceeded, ", _HUF_MAX_SYMBOL_SIZE)
		}

		var f [256]int
		totalFreq := 0

		for i := 0; i < count; i++ {
			f[i] = frequencies[this.alphabet[i]]
			totalFreq += f[i]
		}

		// Copy alphabet (modified by normalizeFrequencies)
		var alphabet [256]int
		copy(alphabet[:], this.alphabet[:count])
		retries++

		// Normalize to a smaller scale
		if _, err := NormalizeFrequencies(f[:count], alphabet[:count], totalFreq, int(_HUF_MAX_CHUNK_SIZE>>(2*retries))); err != nil {
			return count, err
		}

		for i := 0; i < count; i++ {
			frequencies[this.alphabet[i]] = f[i]
		}
	}

	// Transmit code lengths only, frequencies and codes do not matter
	egenc, err := NewExpGolombEncoder(this.bitstream, true)

	if err != nil {
		return count, err
	}

	prevSize := byte(2)

	// Pack size and code (size <= _HUF_MAX_SYMBOL_SIZE bits)
	// Unary encode the length differences
	for _, s := range symbols {
		currSize := sizes[s]
		this.codes[s] |= (uint(currSize) << 24)
		egenc.EncodeByte(currSize - prevSize)
		prevSize = currSize
	}

	return count, nil
}

func (this *HuffmanEncoder) computeCodeLengths(frequencies []int, sizes []byte, count int) error {
	if count == 1 {
		this.sranks[0] = this.alphabet[0]
		sizes[this.alphabet[0]] = 1
		this.maxCodeLen = 1
		return nil
	}

	// Sort ranks by increasing frequencies (first key) and increasing value (second key)
	for i := 0; i < count; i++ {
		this.sranks[i] = (frequencies[this.alphabet[i]] << 8) | this.alphabet[i]
	}

	buf := make([]int, count)
	sort.Ints(this.sranks[0:count])

	for i := range buf {
		buf[i] = this.sranks[i] >> 8
		this.sranks[i] &= 0xFF
	}

	// See [In-Place Calculation of Minimum-Redundancy Codes]
	// by Alistair Moffat & Jyrki Katajainen
	computeInPlaceSizesPhase1(buf)
	computeInPlaceSizesPhase2(buf)
	this.maxCodeLen = 0
	var err error

	for i := range buf {
		codeLen := buf[i]

		if codeLen == 0 {
			err = errors.New("Could not generate Huffman codes: invalid code length 0")
			break
		}

		if this.maxCodeLen < codeLen {
			this.maxCodeLen = codeLen
		}

		sizes[this.sranks[i]] = byte(buf[i])
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
				continue
			}

			sum += data[s]

			if s > t {
				data[s] = 0
			}

			s++
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

// Write encodes the data provided into the bitstream. Return the number of byte
// written to the bitstream.  Dynamically compute the frequencies for every
// chunk of data in the block
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

		// Update frequencies and rebuild Huffman codes
		if _, err := this.updateFrequencies(frequencies[:]); err != nil {
			return 0, err
		}

		c := this.codes
		bs := this.bitstream
		endChunk4 := ((endChunk - startChunk) & -4) + startChunk

		for i := startChunk; i < endChunk4; i += 4 {
			// Pack 4 codes into 1 uint64
			code1 := c[block[i]]
			codeLen1 := uint(code1 >> 24)
			code2 := c[block[i+1]]
			codeLen2 := uint(code2 >> 24)
			code3 := c[block[i+2]]
			codeLen3 := uint(code3 >> 24)
			code4 := c[block[i+3]]
			codeLen4 := uint(code4 >> 24)
			st := (uint64(code1&0xFFFF) << (codeLen2 + codeLen3 + codeLen4)) |
				(uint64(code2&((1<<codeLen2)-1)) << (codeLen3 + codeLen4)) |
				(uint64(code3&((1<<codeLen3)-1)) << codeLen4) |
				uint64(code4&((1<<codeLen4)-1))
			bs.WriteBits(st, codeLen1+codeLen2+codeLen3+codeLen4)
		}

		for i := endChunk4; i < endChunk; i++ {
			code := c[block[i]]
			bs.WriteBits(uint64(code), code>>24)
		}

		startChunk = endChunk
	}

	return len(block), nil
}

// Dispose this implementation does nothing
func (this *HuffmanEncoder) Dispose() {
}

// BitStream returns the underlying bitstream
func (this *HuffmanEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// HuffmanDecoder Implementation of a static Huffman decoder.
// Uses tables to decode symbols
type HuffmanDecoder struct {
	bitstream kanzi.InputBitStream
	codes     [256]uint
	alphabet  [256]int
	sizes     [256]byte
	table     []uint16 // decoding table: code -> size, symbol
	state     uint64   // holds bits read from bitstream
	bits      byte     // holds number of unused bits in 'state'
	chunkSize int
}

// NewHuffmanDecoder creates an instance of HuffmanDecoder.
// Since the number of args is variable, this function can be called like this:
// NewHuffmanDecoder(bs) or NewHuffmanDecoder(bs, 16384) (the second argument
// being the chunk size)
func NewHuffmanDecoder(bs kanzi.InputBitStream, args ...uint) (*HuffmanDecoder, error) {
	if bs == nil {
		return nil, errors.New("Huffman codec: Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("Huffman codec: At most one chunk size can be provided")
	}

	chkSize := _HUF_MAX_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]

		if chkSize < 1024 {
			return nil, errors.New("Huffman codec: The chunk size must be at least 1024")
		}

		if chkSize > _HUF_MAX_CHUNK_SIZE {
			return nil, fmt.Errorf("Huffman codec: The chunk size must be at most %d", _HUF_MAX_CHUNK_SIZE)
		}
	}

	this := new(HuffmanDecoder)
	this.bitstream = bs
	this.table = make([]uint16, 1<<_HUF_DECODING_BATCH_SIZE)
	this.chunkSize = int(chkSize)

	// Default lengths & canonical codes
	for i := 0; i < 256; i++ {
		this.sizes[i] = 8
		this.codes[i] = uint(i)
	}

	return this, nil
}

// readLengths decodes the code lengths from the bitstream and generates
// the Huffman codes for decoding.
func (this *HuffmanDecoder) readLengths() (int, error) {
	count, err := DecodeAlphabet(this.bitstream, this.alphabet[:])

	if count == 0 || err != nil {
		return count, err
	}

	egdec, err := NewExpGolombDecoder(this.bitstream, true)

	if err != nil {
		return 0, err
	}

	currSize := int8(2)
	symbols := this.alphabet[0:count]

	// Decode lengths
	for _, s := range symbols {
		if s&0xFF != s {
			return 0, fmt.Errorf("Invalid bitstream: incorrect Huffman symbol %d", s)
		}

		this.codes[s] = 0
		currSize += int8(egdec.DecodeByte())

		if currSize <= 0 || currSize > _HUF_MAX_SYMBOL_SIZE {
			return 0, fmt.Errorf("Invalid bitstream: incorrect size %d for Huffman symbol %d", currSize, s)
		}

		this.sizes[s] = byte(currSize)
	}

	if _, err := generateCanonicalCodes(this.sizes[:], this.codes[:], symbols); err != nil {
		return count, err
	}

	this.buildDecodingTable(count)
	return count, nil
}

// max(CodeLen) must be <= _HUF_MAX_SYMBOL_SIZE
func (this *HuffmanDecoder) buildDecodingTable(count int) {
	for i := range this.table {
		this.table[i] = 0
	}

	length := byte(0)

	for i := 0; i < count; i++ {
		s := this.alphabet[i]

		if this.sizes[s] > length {
			length = this.sizes[s]
		}

		// code -> size, symbol
		val := (uint16(s) << 8) | uint16(this.sizes[s])
		code := this.codes[s]

		// All DECODING_BATCH_SIZE bit values read from the bit stream and
		// starting with the same prefix point to symbol s
		idx := code << (_HUF_DECODING_BATCH_SIZE - length)
		end := (code + 1) << (_HUF_DECODING_BATCH_SIZE - length)
		t := this.table[0:end]

		for idx+4 < end {
			t[idx] = val
			t[idx+1] = val
			t[idx+2] = val
			t[idx+3] = val
			idx += 4
		}

		for idx < end {
			t[idx] = val
			idx++
		}

	}
}

// Read decodes data from the bitstream and return it in the provided buffer.
// Return the number of bytes read from the bitstream
func (this *HuffmanDecoder) Read(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Huffman codec: Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	end := len(block)
	startChunk := 0

	for startChunk < end {
		// For each chunk, read code lengths, rebuild codes, rebuild decoding table
		alphabetSize, err := this.readLengths()

		if alphabetSize == 0 || err != nil {
			return startChunk, err
		}

		// Compute minimum number of bits required in bitstream for fast decoding
		minCodeLen := int(this.sizes[this.alphabet[0]]) // not 0
		padding := 64 / minCodeLen

		if minCodeLen*padding != 64 {
			padding++
		}

		endChunk := startChunk + this.chunkSize

		if endChunk > end {
			endChunk = end
		}

		endChunk4 := startChunk

		if endChunk > startChunk+padding {
			endChunk4 += ((endChunk - startChunk - padding) & -4)
		}

		for i := startChunk; i < endChunk4; i += 4 {
			this.fetchBits()
			block[i] = this.decodeByte()
			block[i+1] = this.decodeByte()
			block[i+2] = this.decodeByte()
			block[i+3] = this.decodeByte()
		}

		// Fallback to regular decoding
		for i := endChunk4; i < endChunk; i++ {
			block[i] = this.slowDecodeByte()
		}

		startChunk = endChunk
	}

	return len(block), nil
}

func (this *HuffmanDecoder) slowDecodeByte() byte {
	code := 0
	codeLen := uint8(0)

	for codeLen < _HUF_MAX_SYMBOL_SIZE {
		codeLen++

		if this.bits == 0 {
			code = (code << 1) | this.bitstream.ReadBit()
		} else {
			this.bits--
			code = (code << 1) | int((this.state>>this.bits)&1)
		}

		idx := code << (_HUF_DECODING_BATCH_SIZE - codeLen)

		if uint8(this.table[idx]) == codeLen {
			return byte(this.table[idx] >> 8)
		}
	}

	panic(errors.New("Invalid bitstream: incorrect Huffman code"))
}

func (this *HuffmanDecoder) fetchBits() {
	read := this.bitstream.ReadBits(uint(64 - this.bits))
	this.state = (this.state << (64 - this.bits)) | read
	this.bits = 64
}

func (this *HuffmanDecoder) decodeByte() byte {
	val := this.table[int(this.state>>(this.bits-_HUF_DECODING_BATCH_SIZE))&_HUF_DECODING_MASK]
	this.bits -= byte(val)
	return byte(val >> 8)
}

// BitStream returns the underlying bitstream
func (this *HuffmanDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Dispose this implementation does nothing
func (this *HuffmanDecoder) Dispose() {
}
