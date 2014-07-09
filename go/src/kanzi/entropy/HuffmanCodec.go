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
	"container/list"
	"errors"
	"fmt"
	"kanzi"
	"kanzi/util"
	"sort"
)

const (
	DECODING_BATCH_SIZE        = 10            // in bits
	DEFAULT_HUFFMAN_CHUNK_SIZE = uint(1 << 16) // 64 KB by default
)

// ---- Utilities

// Huffman node
type HuffmanNode struct {
	symbol byte
	weight uint
	left   *HuffmanNode
	right  *HuffmanNode
}

type HuffmanCacheData struct {
	value *HuffmanNode
	next  *HuffmanCacheData
}

type SizeComparator struct {
	ranks []uint
	sizes []uint8
}

func ByDecreasingSize(ranks []uint, sizes []uint8) SizeComparator {
	return SizeComparator{ranks: ranks, sizes: sizes}
}

func (this SizeComparator) Less(i, j int) bool {
	// Check size (reverse order) as first key
	if this.sizes[this.ranks[i]] != this.sizes[this.ranks[j]] {
		return this.sizes[this.ranks[i]] > this.sizes[this.ranks[j]]
	}

	// Check index (natural order) as second key
	if this.ranks[i] < this.ranks[j] {
		return true
	}

	return false
}

func (this SizeComparator) Len() int {
	return len(this.ranks)
}

func (this SizeComparator) Swap(i, j int) {
	this.ranks[i], this.ranks[j] = this.ranks[j], this.ranks[i]
}

type FrequencyComparator struct {
	ranks []uint
	freqs []uint
}

func ByIncreasingFrequency(ranks []uint, frequencies []uint) FrequencyComparator {
	return FrequencyComparator{ranks: ranks, freqs: frequencies}
}

func (this FrequencyComparator) Less(i, j int) bool {
	if this.freqs[this.ranks[i]] != this.freqs[this.ranks[j]] {
		return this.freqs[this.ranks[i]] < this.freqs[this.ranks[j]]
	}

	// Make the sort stable
	if this.ranks[i] < this.ranks[j] {
		return true
	}

	return false
}

func (this FrequencyComparator) Len() int {
	return len(this.ranks)
}

func (this FrequencyComparator) Swap(i, j int) {
	this.ranks[i], this.ranks[j] = this.ranks[j], this.ranks[i]
}

// Return the number of codes generated
func GenerateCanonicalCodes(sizes []uint8, codes, ranks []uint) int {
	// Sort by decreasing size (first key) and increasing value (second key)
	if len(ranks) > 1 {
		sort.Sort(ByDecreasingSize(ranks, sizes))
	}

	code := uint(0)
	length := sizes[ranks[0]]
	count := len(ranks)

	for i := 0; i < count; i++ {
		currentSize := sizes[ranks[i]]

		if length > currentSize {
			code >>= (length - currentSize)
			length = currentSize
		}

		codes[ranks[i]] = code
		code++
	}

	return len(ranks)
}

// ---- Encoder

type HuffmanEncoder struct {
	bitstream kanzi.OutputBitStream
	buffer    []uint
	codes     []uint
	sizes     []uint8
	ranks     []uint
	chunkSize int
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewHuffmanEncoder(bs) or NewHuffmanEncoder(bs, 16384)
// The default chunk size is 65536 bytes.
func NewHuffmanEncoder(bs kanzi.OutputBitStream, chunkSizes ...uint) (*HuffmanEncoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(chunkSizes) > 1 {
		return nil, errors.New("At most one chunk size can be provided")
	}

	chkSize := DEFAULT_HUFFMAN_CHUNK_SIZE

	if len(chunkSizes) == 1 {
		chkSize = chunkSizes[0]
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
	this.sizes = make([]uint8, 256)
	this.ranks = make([]uint, 256)
	this.chunkSize = int(chkSize)

	// Default frequencies, sizes and codes
	for i := 0; i < 256; i++ {
		this.buffer[i] = 1
		this.sizes[i] = 8
		this.codes[i] = uint(i)
	}

	return this, nil
}

func createTreeFromFrequencies(frequencies []uint, sizes_ []uint8, ranks []uint) *HuffmanNode {
	// Sort by frequency
	if len(ranks) > 1 {
		sort.Sort(ByIncreasingFrequency(ranks, frequencies))
	}

	// Create Huffman tree of (present) symbols
	queue1 := list.New()
	queue2 := list.New()
	nodes := make([]*HuffmanNode, 2)

	for i := len(ranks) - 1; i >= 0; i-- {
		queue1.PushFront(&HuffmanNode{symbol: uint8(ranks[i]), weight: frequencies[ranks[i]]})
	}

	for queue1.Len()+queue2.Len() > 1 {
		// Extract 2 minimum nodes
		for i := range nodes {
			if queue2.Len() == 0 {
				nodes[i] = queue1.Front().Value.(*HuffmanNode)
				queue1.Remove(queue1.Front())
				continue
			}

			if queue1.Len() == 0 {
				nodes[i] = queue2.Front().Value.(*HuffmanNode)
				queue2.Remove(queue2.Front())
				continue
			}

			if queue1.Front().Value.(*HuffmanNode).weight <= queue2.Front().Value.(*HuffmanNode).weight {
				nodes[i] = queue1.Front().Value.(*HuffmanNode)
				queue1.Remove(queue1.Front())
			} else {
				nodes[i] = queue2.Front().Value.(*HuffmanNode)
				queue2.Remove(queue2.Front())
			}
		}

		// Merge minimum nodes and enqueue result
		lNode := nodes[0]
		rNode := nodes[1]
		queue2.PushBack(&HuffmanNode{weight: lNode.weight + rNode.weight, left: lNode, right: rNode})
	}

	var rootNode *HuffmanNode

	if queue1.Len() == 0 {
		rootNode = queue2.Front().Value.(*HuffmanNode)
	} else {
		rootNode = queue1.Front().Value.(*HuffmanNode)
	}

	if len(ranks) == 1 {
		sizes_[rootNode.symbol] = uint8(1)
	} else {
		fillTree(rootNode, 0, sizes_)
	}

	return rootNode
}

// Fill size and code arrays
func fillTree(node *HuffmanNode, depth uint, sizes_ []uint8) {
	if node.left == nil && node.right == nil {
		idx := node.symbol
		sizes_[idx] = uint8(depth)
		return
	}

	if node.left != nil {
		fillTree(node.left, depth+1, sizes_)
	}

	if node.right != nil {
		fillTree(node.right, depth+1, sizes_)
	}
}

// Rebuild Huffman tree
func (this *HuffmanEncoder) UpdateFrequencies(frequencies []uint) error {
	if frequencies == nil || len(frequencies) != 256 {
		return errors.New("Invalid frequencies parameter")
	}

	count := 0

	for i := range this.ranks {
		this.sizes[i] = 0
		this.codes[i] = 0

		if frequencies[i] > 0 {
			this.ranks[count] = uint(i)
			count++
		}
	}

	// Create tree from frequencies
	createTreeFromFrequencies(frequencies, this.sizes, this.ranks[0:count])

	// Create canonical codes
	GenerateCanonicalCodes(this.sizes, this.codes, this.ranks[0:count])
	egenc, err := NewExpGolombEncoder(this.bitstream, true)

	if err != nil {
		return err
	}

	// Transmit code lengths only, frequencies and codes do not matter
	// Unary encode the length difference
	prevSize := uint8(2)
	zeros := -1

	for i := 0; i < 256; i++ {
		currSize := this.sizes[i]
		egenc.EncodeByte(currSize - prevSize)

		if currSize == 0 {
			zeros++
		} else {
			zeros = 0
		}

		// If there is one zero size symbol, save a few bits by avoiding the
		// encoding of a big size difference twice
		// EG: 13 13 0 13 14 ... encoded as 0 -13 0 +1 instead of 0 -13 +13 0 +1
		// If there are several zero size symbols in a row, use regular encoding
		if zeros != 1 {
			prevSize = currSize
		}
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

	buf := this.buffer // aliasing
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
		for i := range buf {
			buf[i] = 0
		}

		for i := startChunk; i < endChunk; i++ {
			buf[block[i]]++
		}

		// Rebuild Huffman tree
		this.UpdateFrequencies(buf)

		for i := startChunk; i < endChunk; i++ {
			if err := this.EncodeByte(block[i]); err != nil {
				return i, err
			}
		}

		startChunk = endChunk

		if startChunk+sizeChunk >= end {
			sizeChunk = end - startChunk
		}

		endChunk = startChunk + sizeChunk
	}

	return len(block), nil
}

// Frequencies of the data block must have been previously set
func (this *HuffmanEncoder) EncodeByte(val byte) error {
	_, err := this.bitstream.WriteBits(uint64(this.codes[val]), uint(this.sizes[val]))
	return err
}

func (this *HuffmanEncoder) Dispose() {
}

func (this *HuffmanEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// ---- Decoder

type HuffmanDecoder struct {
	bitstream     kanzi.InputBitStream
	codes         []uint
	sizes         []uint8
	root          *HuffmanNode
	decodingCache []*HuffmanCacheData
	current       *HuffmanCacheData
	ranks         []uint
	chunkSize     int
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewHuffmanDecoder(bs) or NewHuffmanDecoder(bs, 16384)
// The default chunk size is 65536 bytes.
func NewHuffmanDecoder(bs kanzi.InputBitStream, chunkSizes ...uint) (*HuffmanDecoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(chunkSizes) > 1 {
		return nil, errors.New("At most one chunk size can be provided")
	}

	chkSize := DEFAULT_HUFFMAN_CHUNK_SIZE

	if len(chunkSizes) == 1 {
		chkSize = chunkSizes[0]
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("The chunk size must be at most 2^30")
	}

	this := new(HuffmanDecoder)
	this.bitstream = bs
	this.sizes = make([]uint8, 256)
	this.codes = make([]uint, 256)
	this.ranks = make([]uint, 256)
	this.chunkSize = int(chkSize)

	// Default lengths & canonical codes
	for i := uint(0); i < 256; i++ {
		this.sizes[i] = 8
		this.codes[i] = i
	}

	// Create tree from code sizes
	this.root = this.createTreeFromSizes(8)
	this.decodingCache = this.buildDecodingCache(make([]*HuffmanCacheData, 1<<DECODING_BATCH_SIZE))
	this.current = this.decodingCache[0] // point to root
	return this, nil
}

func (this *HuffmanDecoder) buildDecodingCache(cache []*HuffmanCacheData) []*HuffmanCacheData {
	rootNode := this.root
	end := 1 << DECODING_BATCH_SIZE
	var previousData *HuffmanCacheData

	if cache[0] == nil {
		previousData = &HuffmanCacheData{value: rootNode}
	} else {
		previousData = cache[0]
	}

	// Create an array storing a list of tree nodes (shortcuts) for each input value
	for val := 0; val < end; val++ {
		shift := DECODING_BATCH_SIZE - 1
		firstAdded := false

		for shift >= 0 {
			// Start from root
			currentNode := rootNode

			// Process next bit
			for currentNode.left != nil || currentNode.right != nil {

				if (val>>uint(shift))&1 == 0 {
					currentNode = currentNode.left
				} else {
					currentNode = currentNode.right
				}

				shift--

				// Current node is null only if there is only 1 symbol (Huffman code 0).
				// In this case, trying to map an impossible value (non zero) to
				// a node fails.
				if shift < 0 || currentNode == nil {
					break
				}
			}

			// If there is only 1 Huffman symbol to decode (0), no need to create
			// a big cache. Stop here and return.
			if currentNode == nil {
				previousData.next = nil
				return cache
			}

			// Reuse cache data objects when recreating the cache
			if previousData.next == nil {
				previousData.next = &HuffmanCacheData{value: currentNode}
			} else {
				previousData.next.value = currentNode
			}

			// The cache is made of linked nodes
			previousData = previousData.next

			if firstAdded == false {
				// Add first node of list to array (whether it is a leaf or not)
				cache[val] = previousData
				firstAdded = true
			}
		}

		// Reuse cache data objects when recreating the cache
		if previousData.next == nil {
			previousData.next = &HuffmanCacheData{value: rootNode}
		} else {
			previousData.next.value = rootNode
		}

		previousData = previousData.next
	}

	return cache
}

func (this *HuffmanDecoder) createTreeFromSizes(maxSize uint) *HuffmanNode {
	codeMap := make(map[int]*HuffmanNode)
	sum := uint(1 << maxSize)
	tree, _ := util.NewIntBTree() //use binary tree
	codeMap[0] = &HuffmanNode{symbol: byte(0), weight: sum}
	tree.Add(0) // add key(0,0)

	// Create node for each (present) symbol and add to map
	for i := range this.sizes {
		size := this.sizes[i]

		if size == 0 {
			continue
		}

		key := (int(size) << 24) | int(this.codes[i])
		tree.Add(key)
		value := &HuffmanNode{symbol: byte(i), weight: sum >> size}
		codeMap[key] = value
	}

	// Process each element of the map except the root node
	for tree.Size() > 1 {
		key, _ := tree.Max()
		tree.Remove(key)
		node := codeMap[key]
		l := (key >> 24) & 0xFF
		c := key & 0xFFFFFF
		upKey := ((l - 1) << 24) | (c >> 1)
		upNode := codeMap[upKey]

		// Create superior node if it does not exist (length gap > 1)
		if upNode == nil {
			upNode = &HuffmanNode{symbol: byte(0), weight: sum >> uint(l-1)}
			codeMap[upKey] = upNode
			tree.Add(upKey)
		}

		// Add the current node to its parent at the correct place
		if c&1 == 1 {
			upNode.right = node
		} else {
			upNode.left = node
		}
	}

	// Return the last element of the map (root node)
	return codeMap[0]
}

func (this *HuffmanDecoder) ReadLengths() error {
	buf := this.sizes // alias
	egdec, err := NewExpGolombDecoder(this.bitstream, true)

	if err != nil {
		return err
	}

	delta, err := egdec.DecodeByte()

	if err != nil {
		return err
	}

	currSize := int8(delta) + 2

	if currSize < 0 {
		return fmt.Errorf("Invalid bitstream: incorrect size %v for Huffman symbol 0", currSize)
	}

	maxSize := currSize
	prevSize := currSize
	buf[0] = uint8(currSize)
	zeros := 0

	// Read lengths
	for i := 1; i < 256; i++ {
		if delta, err = egdec.DecodeByte(); err != nil {
			return err
		}

		currSize = int8(delta) + prevSize

		if currSize < 0 {
			return fmt.Errorf("Invalid bitstream: incorrect size %v for Huffman symbol %v", currSize, i)
		}

		buf[i] = uint8(currSize)

		if currSize == 0 {
			zeros++
		} else {
			zeros = 0
		}

		if maxSize < currSize {
			maxSize = currSize
		}

		// If there is one zero size symbol, save a few bits by avoiding the
		// encoding of a big size difference twice
		// EG: 13 13 0 13 14 ... encoded as 0 -13 0 +1 instead of 0 -13 +13 0 +1
		// If there are several zero size symbols in a row, use regular decoding
		if zeros != 1 {
			prevSize = currSize
		}
	}

	count := 0

	for i := range this.codes {
		this.codes[i] = 0

		if this.sizes[i] > 0 {
			this.ranks[count] = uint(i)
			count++
		}
	}

	// Create canonical codes
	GenerateCanonicalCodes(buf, this.codes, this.ranks[0:count])

	// Create tree from code sizes
	this.root = this.createTreeFromSizes(uint(maxSize))
	this.buildDecodingCache(this.decodingCache)
	this.current = &HuffmanCacheData{value: this.root} // point to root
	return nil
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

	if startChunk+sizeChunk >= end {
		sizeChunk = end - startChunk
	}

	endChunk := startChunk + sizeChunk

	for startChunk < end {
		// Reinitialize the Huffman tree
		this.ReadLengths()
		endChunk1 := endChunk - DECODING_BATCH_SIZE
		i := startChunk
		var err error

		for i < endChunk1 {
			// Fast decoding by reading several bits at a time from the bitstream
			if block[i], err = this.fastDecodeByte(); err != nil {
				return i, err
			}

			i++
		}

		for i < endChunk {
			// Regular decoding by reading one bit at a time from the bitstream
			if block[i], err = this.DecodeByte(); err != nil {
				return i, err
			}

			i++
		}

		startChunk = endChunk

		if startChunk+sizeChunk >= end {
			sizeChunk = end - startChunk
		}

		endChunk = startChunk + sizeChunk
	}

	return len(block), nil
}

func (this *HuffmanDecoder) DecodeByte() (byte, error) {
	// Empty cache
	currNode := this.current.value

	if currNode != this.root {
		this.current = this.current.next
	}

	for currNode.left != nil || currNode.right != nil {
		r, err := this.bitstream.ReadBit()

		if err != nil {
			return 0, err
		}

		if r == 0 {
			currNode = currNode.left
		} else {
			currNode = currNode.right
		}
	}

	return currNode.symbol, nil
}

// DECODING_BATCH_SIZE bits must be available in the bitstream
func (this *HuffmanDecoder) fastDecodeByte() (byte, error) {
	currNode := this.current.value

	// Use the cache to find a good starting point in the tree
	if currNode == this.root {
		// Read more bits from the bitstream and fetch starting point from cache
		idx, _ := this.bitstream.ReadBits(DECODING_BATCH_SIZE)
		this.current = this.decodingCache[idx]
		currNode = this.current.value
	}

	// The node symbol is 0 only if the node is not a leaf or it codes the value 0.
	// We need to check if it is a leaf only if the symbol is 0.
	if currNode.symbol == 0 {
		for currNode.left != nil || currNode.right != nil {
			r, err := this.bitstream.ReadBit()

			if err != nil {
				return 0, err
			}

			if r == 0 {
				currNode = currNode.left
			} else {
				currNode = currNode.right
			}
		}
	}

	// Move to next starting point in cache
	this.current = this.current.next
	return currNode.symbol, nil
}

func (this *HuffmanDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *HuffmanDecoder) Dispose() {
}
