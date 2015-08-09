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
	"container/heap"
	"errors"
	"fmt"
	"kanzi"
	"sort"
)

const (
	DECODING_BATCH_SIZE        = 10 // in bits
	DECODING_MASK              = (1 << DECODING_BATCH_SIZE) - 1
	MAX_DECODING_INDEX         = (DECODING_BATCH_SIZE << 8) | 0xFF
	DEFAULT_HUFFMAN_CHUNK_SIZE = uint(1 << 16) // 64 KB by default
	ABSENT                     = (1 << 31) - 1
)

// ---- Utilities

// Huffman node
type HuffmanNode struct {
	symbol byte
	weight uint
	left   *HuffmanNode
	right  *HuffmanNode
}

type SizeComparator struct {
	ranks []byte
	sizes []byte
}

func ByIncreasingSize(ranks, sizes []byte) SizeComparator {
	return SizeComparator{ranks: ranks, sizes: sizes}
}

func (this SizeComparator) Less(i, j int) bool {
	// Check size (natural order) as first key
	ri := this.ranks[i]
	rj := this.ranks[j]

	if this.sizes[ri] != this.sizes[rj] {
		return this.sizes[ri] < this.sizes[rj]
	}

	// Check index (natural order) as second key
	return ri < rj
}

func (this SizeComparator) Len() int {
	return len(this.ranks)
}

func (this SizeComparator) Swap(i, j int) {
	this.ranks[i], this.ranks[j] = this.ranks[j], this.ranks[i]
}

type HuffmanPriorityQueue []*HuffmanNode

func (this HuffmanPriorityQueue) Len() int {
	return len(this)
}

func (this HuffmanPriorityQueue) Less(i, j int) bool {
	ni := this[i]
	nj := this[j]

	if ni.weight != nj.weight {
		return ni.weight < nj.weight
	}

	if ni.left == nil {
		if nj.left != nil {
			return true
		}
	} else if nj.left == nil {
		return false
	}

	return ni.symbol < nj.symbol
}

func (this HuffmanPriorityQueue) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

func (this *HuffmanPriorityQueue) Push(node interface{}) {
	*this = append(*this, node.(*HuffmanNode))
}

func (this *HuffmanPriorityQueue) Pop() interface{} {
	old := *this
	n := len(old)
	node := old[n-1]
	*this = old[0 : n-1]
	return node
}

// Return the number of codes generated
func generateCanonicalCodes(sizes []byte, codes []uint, ranks []byte) int {
	count := len(ranks)

	// Sort by increasing size (first key) and increasing value (second key)
	if count > 1 {
		sort.Sort(ByIncreasingSize(ranks, sizes))
	}

	code := uint(0)
	length := sizes[ranks[0]]

	for i := 0; i < count; i++ {
		r := ranks[i]

		if sizes[r] > length {
			code <<= (sizes[r] - length)
			length = sizes[r]

			// Max length reached
			if length > 24 {
				return -1
			}
		}

		codes[r] = code
		code++
	}

	return count
}

// ---- Encoder

type HuffmanEncoder struct {
	bitstream kanzi.OutputBitStream
	buffer    []uint
	codes     []uint
	sizes     []byte
	ranks     []byte
	chunkSize int
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewHuffmanEncoder(bs) or NewHuffmanEncoder(bs, 16384)
// The default chunk size is 65536 bytes.
func NewHuffmanEncoder(bs kanzi.OutputBitStream, args ...uint) (*HuffmanEncoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("At most one chunk size can be provided")
	}

	chkSize := DEFAULT_HUFFMAN_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("The chunk size must be at most 2^30")
	}

	this := new(HuffmanEncoder)
	this.bitstream = bs
	this.buffer = make([]uint, 256)
	this.codes = make([]uint, 256)
	this.sizes = make([]byte, 256)
	this.ranks = make([]byte, 256)
	this.chunkSize = int(chkSize)

	// Default frequencies, sizes and codes
	for i := 0; i < 256; i++ {
		this.buffer[i] = 1
		this.sizes[i] = 8
		this.codes[i] = uint(i)
	}

	return this, nil
}

func createTreeFromFrequencies(frequencies []uint, sizes_ []byte, ranks []byte) error {
	// Create Huffman tree of (present) symbols
	queue := make(HuffmanPriorityQueue, 0)
	heap.Init(&queue)

	for i := range ranks {
		heap.Push(&queue, &HuffmanNode{symbol: ranks[i], weight: frequencies[ranks[i]]})
	}

	for queue.Len() > 1 {
		// Extract 2 minimum nodes, merge them and enqueue result
		lNode := heap.Pop(&queue).(*HuffmanNode)
		rNode := heap.Pop(&queue).(*HuffmanNode)

		// Setting the symbol is critical to resolve ties during node sorting !
		heap.Push(&queue, &HuffmanNode{weight: lNode.weight + rNode.weight, left: lNode, right: rNode, symbol: lNode.symbol})
	}

	rootNode := heap.Pop(&queue).(*HuffmanNode)
	var err error

	if len(ranks) == 1 {
		sizes_[rootNode.symbol] = byte(1)
	} else {
		err = fillSizes(rootNode, 0, sizes_)
	}

	return err
}

// Recursively fill sizes
func fillSizes(node *HuffmanNode, depth uint, sizes_ []byte) error {
	if depth > 24 {
		return fmt.Errorf("Cannot code symbol '%v'", node.symbol)
	}

	if node.left == nil && node.right == nil {
		sizes_[node.symbol] = byte(depth)
		return nil
	}

	if node.left != nil {
		fillSizes(node.left, depth+1, sizes_)
	}

	if node.right != nil {
		fillSizes(node.right, depth+1, sizes_)
	}

	return nil
}

// Rebuild Huffman tree
func (this *HuffmanEncoder) UpdateFrequencies(frequencies []uint) error {
	if frequencies == nil || len(frequencies) != 256 {
		return errors.New("Invalid frequencies parameter")
	}

	alphabetSize := 0

	for i := range this.sizes {
		this.sizes[i] = 0
		this.codes[i] = 0

		if frequencies[i] > 0 {
			this.ranks[alphabetSize] = byte(i)
			alphabetSize++
		}
	}

	// Create tree from frequencies
	err := createTreeFromFrequencies(frequencies, this.sizes, this.ranks[0:alphabetSize])

	if err != nil {
		return err
	}

	EncodeAlphabet(this.bitstream, this.ranks[0:alphabetSize])

	// Transmit code lengths only, frequencies and codes do not matter
	// Unary encode the length difference
	egenc, err := NewExpGolombEncoder(this.bitstream, true)

	if err != nil {
		return err
	}

	prevSize := byte(2)

	for i := 0; i < alphabetSize; i++ {
		currSize := this.sizes[this.ranks[i]]
		egenc.EncodeByte(currSize - prevSize)
		prevSize = currSize
	}

	// Create canonical codes (reorders ranks)
	if generateCanonicalCodes(this.sizes, this.codes, this.ranks[0:alphabetSize]) < 0 {
		return errors.New("Could not generate codes: max code length (24 bits) exceeded")
	}

	// Pack size and code (size <= 24 bits)
	for i := 0; i < alphabetSize; i++ {
		r := this.ranks[i]
		this.codes[r] = (uint(this.sizes[r]) << 24) | this.codes[r]
	}

	return nil
}

// Dynamically compute the frequencies for every chunk of data in the block
func (this *HuffmanEncoder) Encode(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	frequencies := this.buffer // aliasing
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

		for i := range frequencies {
			frequencies[i] = 0
		}

		for i := startChunk; i < endChunk; i++ {
			frequencies[block[i]]++
		}

		// Rebuild Huffman tree
		this.UpdateFrequencies(frequencies)

		for i := startChunk; i < endChunk; i++ {
			val := this.codes[block[i]]
			this.bitstream.WriteBits(uint64(val), val>>24)
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

// ---- Decoder

type HuffmanDecoder struct {
	bitstream  kanzi.InputBitStream
	codes      []uint
	ranks      []byte
	sizes      []byte
	fdTable    []uint // Fast decoding table
	sdTable    []uint // Slow decoding table
	sdtIndexes []int  // Indexes for slow decoding table (can be negative)
	chunkSize  int
	state      uint64 // holds bits read from bitstream
	bits       uint   // hold number of unused bits in 'state'
	minCodeLen int8
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewHuffmanDecoder(bs) or NewHuffmanDecoder(bs, 16384)
// The default chunk size is 65536 bytes.
func NewHuffmanDecoder(bs kanzi.InputBitStream, args ...uint) (*HuffmanDecoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 1 {
		return nil, errors.New("At most one chunk size can be provided")
	}

	chkSize := DEFAULT_HUFFMAN_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("The chunk size must be at most 2^30")
	}

	this := new(HuffmanDecoder)
	this.bitstream = bs
	this.sizes = make([]byte, 256)
	this.codes = make([]uint, 256)
	this.ranks = make([]byte, 256)
	this.fdTable = make([]uint, 1<<DECODING_BATCH_SIZE)
	this.sdTable = make([]uint, 256)
	this.sdtIndexes = make([]int, 24)
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
	count, err := DecodeAlphabet(this.bitstream, this.ranks)

	if err != nil {
		return 0, err
	}

	egdec, err := NewExpGolombDecoder(this.bitstream, true)

	if err != nil {
		return 0, err
	}

	var currSize int8
	this.minCodeLen = 24 // max code length
	prevSize := int8(2)

	// Read lengths
	for i := 0; i < count; i++ {
		r := this.ranks[i]
		this.codes[r] = 0
		currSize = int8(egdec.DecodeByte()) + prevSize

		if currSize < 0 {
			return 0, fmt.Errorf("Invalid bitstream: incorrect size %v for Huffman symbol %v", currSize, i)
		}

		if currSize != 0 {
			if currSize > 24 {
				return 0, fmt.Errorf("Invalid bitstream: incorrect size %v for Huffman symbol %v", currSize, i)
			}

			if this.minCodeLen > currSize {
				this.minCodeLen = currSize
			}
		}

		this.sizes[r] = byte(currSize)
		prevSize = currSize
	}

	if count == 0 {
		return 0, nil
	}

	// Create canonical codes
	if generateCanonicalCodes(this.sizes, this.codes, this.ranks[0:count]) < 0 {
		return 0, errors.New("Could not generate codes: max code length (24 bits) exceeded")
	}

	// Build decoding tables
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
		this.sdtIndexes[i] = ABSENT
	}

	length := byte(0)

	for i := 0; i < count; i++ {
		r := uint(this.ranks[i])
		code := this.codes[r]

		if this.sizes[r] > length {
			length = this.sizes[r]
			this.sdtIndexes[length] = i - int(code)
		}

		// Fill slow decoding table
		val := (uint(this.sizes[r]) << 8) | r
		this.sdTable[i] = val
		var idx, end uint

		// Fill fast decoding table
		// Find location index in table
		if length < DECODING_BATCH_SIZE {
			idx = code << (DECODING_BATCH_SIZE - length)
			end = idx + (1 << (DECODING_BATCH_SIZE - length))
		} else {
			idx = code >> (length - DECODING_BATCH_SIZE)
			end = idx + 1
		}

		// All DECODING_BATCH_SIZE bit values read from the bit stream and
		// starting with the same prefix point to symbol r
		for idx < end {
			this.fdTable[idx] = val
			idx++
		}
	}
}

// Rebuild the Huffman tree for each chunk of data in the block
// Use fastDecodeByte until the near end of chunk or block.
func (this *HuffmanDecoder) Decode(block []byte) (int, error) {
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
		// Reinitialize the Huffman tables
		if r, err := this.ReadLengths(); r == 0 || err != nil {
			return startChunk, err
		}

		endChunk := startChunk + sizeChunk

		if endChunk > end {
			endChunk = end
		}

		// Compute minimum number of bits requires in bitstream for fast decoding
		endPaddingSize := int(64 / this.minCodeLen)

		if this.minCodeLen*(64/this.minCodeLen) != 64 {
			endPaddingSize++
		}

		endChunk1 := endChunk - endPaddingSize
		i := startChunk

		for i < endChunk1 {
			// Fast decoding (read DECODING_BATCH_SIZE bits at a time)
			block[i] = this.fastDecodeByte()
			i++
		}

		for i < endChunk {
			// Fallback to regular decoding (read one bit at a time)
			block[i] = this.slowDecodeByte(0, 0)
			i++
		}

		startChunk = endChunk
	}

	return len(block), nil
}

func (this *HuffmanDecoder) slowDecodeByte(code int, codeLen uint) byte {
	for codeLen < 23 {
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

		if idx == ABSENT {
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
	if this.bits < DECODING_BATCH_SIZE {
		// Fetch more bits from bitstream
		read := this.bitstream.ReadBits(64 - this.bits)
		// No need to mask this.state because uint64(xyz) << 64 = 0
		this.state = (this.state << (64 - this.bits)) | read
		this.bits = 64
	}

	// Retrieve symbol from fast decoding table
	idx := int(this.state>>(this.bits-DECODING_BATCH_SIZE)) & DECODING_MASK
	val := this.fdTable[idx]

	if val > MAX_DECODING_INDEX {
		this.bits -= DECODING_BATCH_SIZE
		return this.slowDecodeByte(idx, DECODING_BATCH_SIZE)
	}

	this.bits -= (val >> 8)
	return byte(val)
}

func (this *HuffmanDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *HuffmanDecoder) Dispose() {
}
