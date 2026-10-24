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
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	kanzi "github.com/flanglet/kanzi-go/v2"
	internal "github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_HUF_MIN_CHUNK_SIZE     = 1024
	_HUF_MAX_CHUNK_SIZE     = 1 << 14
	_HUF_ORDER1_CHUNK_SHIFT = 5
	_HUF_ORDER1_CONTEXTS    = 256
	_HUF_MAX_SYMBOL_SIZE_V4 = 12
	_HUF_DECODING_MASK_V4   = (1 << _HUF_MAX_SYMBOL_SIZE_V4) - 1
)

// Return the number of codes generated
func generateCanonicalCodes(sizes []byte, codes []uint16, symbols []int, maxSymbolSize int) (int, error) {
	count := len(symbols)

	if count == 0 {
		return 0, nil
	}

	if count > 256 {
		return -1, errors.New("Could not generate Huffman codes: invalid symbol count")
	}

	if count > 1 {
		var present [256]bool
		var offsets [_HUF_MAX_SYMBOL_SIZE_V4 + 1]uint16

		for _, s := range symbols {
			if s < 0 || s > 255 {
				return -1, errors.New("Could not generate Huffman codes: invalid symbol")
			}

			length := sizes[s]

			if length == 0 {
				return -1, errors.New("Could not generate Huffman codes: invalid code length")
			}

			if int(length) > maxSymbolSize || int(length) > _HUF_MAX_SYMBOL_SIZE_V4 {
				return -1, fmt.Errorf("Could not generate Huffman codes: max code length (%d bits) exceeded", maxSymbolSize)
			}

			if present[s] {
				return -1, errors.New("Could not generate Huffman codes: duplicate symbol")
			}

			present[s] = true
			offsets[length]++
		}

		var offset uint16

		for length := 1; length <= _HUF_MAX_SYMBOL_SIZE_V4; length++ {
			n := offsets[length]
			offsets[length] = offset
			offset += n
		}

		// Scanning symbols in value order preserves the previous tie break.
		for s := 0; s < 256; s++ {
			if present[s] {
				length := sizes[s]
				symbols[offsets[length]] = s
				offsets[length]++
			}
		}
	}

	code := uint16(0)
	curLen := sizes[symbols[0]]

	for _, s := range symbols {
		code <<= (sizes[s] - curLen)
		curLen = sizes[s]
		codes[s] = code
		code++
	}

	return count, nil
}

// HuffmanEncoder  Implementation of a static Huffman encoder.
// Uses in place generation of canonical codes instead of a tree
type HuffmanEncoder struct {
	bitstream   kanzi.OutputBitStream
	codes       [256]uint16
	buffer      []byte
	freqs       []int
	codesOrder1 []uint16
	chunkSize   int
	order       int
}

// NewHuffmanEncoder creates an instance of HuffmanEncoder.
// Since the number of args is variable, this function can be called like this:
// NewHuffmanEncoder(bs) or NewHuffmanEncoder(bs, 16384) (the second argument
// being the chunk size)
func NewHuffmanEncoder(bs kanzi.OutputBitStream, args ...int) (*HuffmanEncoder, error) {
	return NewHuffmanEncoderWithOrder(bs, 0, args...)
}

// NewHuffmanEncoderWithOrder creates a Huffman encoder for the requested order.
// The optional chunk size is the order-0 base size and is multiplied by 32 for
// order 1.
func NewHuffmanEncoderWithOrder(bs kanzi.OutputBitStream, order int, args ...int) (*HuffmanEncoder, error) {
	if bs == nil {
		return nil, errors.New("Huffman codec: Invalid null bitstream parameter")
	}

	if order != 0 && order != 1 {
		return nil, errors.New("Huffman codec: The order must be 0 or 1")
	}

	if len(args) > 1 {
		return nil, errors.New("Huffman codec: At most one chunk size can be provided")
	}

	chkSize := _HUF_MAX_CHUNK_SIZE

	if len(args) == 1 {
		chkSize = args[0]

		if chkSize < _HUF_MIN_CHUNK_SIZE {
			return nil, fmt.Errorf("Huffman codec: The chunk size must be at least %d", _HUF_MIN_CHUNK_SIZE)
		}

		if chkSize > _HUF_MAX_CHUNK_SIZE {
			return nil, fmt.Errorf("Huffman codec: The chunk size must be at most %d", _HUF_MAX_CHUNK_SIZE)
		}
	}

	this := &HuffmanEncoder{}
	this.bitstream = bs
	this.chunkSize = chkSize << (order * _HUF_ORDER1_CHUNK_SHIFT)
	this.order = order

	// Default frequencies, sizes and codes
	for i := range &this.codes {
		this.codes[i] = uint16(i)
	}

	return this, nil
}

// Rebuild Huffman codes
func (this *HuffmanEncoder) updateFrequencies(freqs []int) (int, error) {
	if freqs == nil || len(freqs) != 256 {
		return 0, errors.New("Huffman codec: Invalid frequencies parameter")
	}

	count := 0
	var sizes [256]byte
	var alphabet [256]int

	for i := range &this.codes {
		this.codes[i] = 0

		if freqs[i] > 0 {
			alphabet[count] = i
			count++
		}
	}

	symbols := alphabet[0:count]

	if _, err := EncodeAlphabet(this.bitstream, symbols); err != nil {
		return count, err
	}

	if count == 0 {
		return 0, nil
	}

	if count == 1 {
		this.codes[symbols[0]] = 1 << 12
		sizes[symbols[0]] = 1
	} else {
		var ranks [256]int

		// Sort ranks by increasing freqs (first key) and increasing value (second key)
		for i := range symbols {
			ranks[i] = (freqs[symbols[i]] << 8) | symbols[i]
		}

		var maxCodeLen int
		var err error

		if maxCodeLen, err = this.computeCodeLengths(sizes[:], ranks[0:count]); err != nil {
			return count, err
		}

		if maxCodeLen > _HUF_MAX_SYMBOL_SIZE_V4 {
			// Attempt to limit codes max width
			if maxCodeLen, err = this.limitCodeLengths(symbols, freqs, sizes[:], ranks[0:count]); err != nil {
				return count, err
			}
		}

		if maxCodeLen > _HUF_MAX_SYMBOL_SIZE_V4 {
			// Unlikely branch when no codes could be found that fit within _HUF_MAX_SYMBOL_SIZE_V4 width
			for i := 0; i < count; i++ {
				this.codes[alphabet[i]] = uint16(i)
				sizes[alphabet[i]] = 8
			}
		} else {
			if _, err = generateCanonicalCodes(sizes[:], this.codes[:], ranks[0:count], _HUF_MAX_SYMBOL_SIZE_V4); err != nil {
				return count, err
			}
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
		curSize := sizes[s]
		this.codes[s] |= (uint16(curSize) << 12)
		egenc.EncodeByte(curSize - prevSize)
		prevSize = curSize
	}

	egenc.Dispose()
	return count, nil
}

func (this *HuffmanEncoder) limitCodeLengths(symbols []int, freqs []int, sizes []byte, ranks []int) (int, error) {
	n := 0
	debt := 0

	// Fold over-the-limit sizes, skip at-the-limit sizes => incur bit debt
	for sizes[ranks[n]] >= _HUF_MAX_SYMBOL_SIZE_V4 {
		debt += (int(sizes[ranks[n]]) - _HUF_MAX_SYMBOL_SIZE_V4)
		sizes[ranks[n]] = _HUF_MAX_SYMBOL_SIZE_V4
		n++
	}

	// Check (up to) 6 levels; one slice per size delta
	q := make([][]int, 6)
	count := len(ranks)

	for n < count {
		idx := _HUF_MAX_SYMBOL_SIZE_V4 - 1 - sizes[ranks[n]]

		if (idx > 5) || (debt < (1 << idx)) {
			break
		}

		q[idx] = append(q[idx], ranks[n])
		n++
	}

	idx := 5

	// Repay bit debt in a "semi optimized" way
	for (debt > 0) && (idx >= 0) {
		if (len(q[idx]) == 0) || (debt < (1 << idx)) {
			idx--
			continue
		}

		r := q[idx][0]
		sizes[r]++
		debt -= (1 << idx)
		q[idx] = q[idx][1:]
	}

	idx = 0

	// Adjust if necessary
	for (debt > 0) && (idx < 6) {
		if len(q[idx]) == 0 {
			idx++
			continue
		}

		r := q[idx][0]
		sizes[r]++
		debt -= (1 << idx)
		q[idx] = q[idx][1:]
	}

	if debt > 0 {
		// Fallback to slow (more accurate) path if fast path failed to repay the debt
		var f [256]int
		var alpha [256]int
		totalFreq := 0

		for i := range symbols {
			f[i] = freqs[symbols[i]]
			totalFreq += f[i]
		}

		// Renormalize to a smaller scale
		if _, err := NormalizeFrequencies(f[:count], alpha[:count], totalFreq, _HUF_MAX_CHUNK_SIZE>>3); err != nil {
			return 0, err
		}

		for i := range ranks {
			freqs[symbols[i]] = f[i]
			ranks[i] = (f[i] << 8) | symbols[i]
		}

		return this.computeCodeLengths(sizes, ranks)
	}

	return _HUF_MAX_SYMBOL_SIZE_V4, nil
}

// Called only when more than 1 symbol (len(ranks) >= 2)
func (this *HuffmanEncoder) computeCodeLengths(sizes []byte, ranks []int) (int, error) {
	var frequencies [256]int
	freqs := frequencies[0:len(ranks)]
	sort.Ints(ranks)

	for i := range ranks {
		freqs[i] = ranks[i] >> 8
		ranks[i] &= 0xFF

		if freqs[i] == 0 {
			return 0, errors.New("Could not generate Huffman codes: invalid code length 0")
		}
	}

	// See [In-Place Calculation of Minimum-Redundancy Codes]
	// by Alistair Moffat & Jyrki Katajainen
	computeInPlaceSizesPhase1(freqs)
	maxCodeLen := computeInPlaceSizesPhase2(freqs)

	for i := range freqs {
		sizes[ranks[i]] = byte(freqs[i])
	}

	return maxCodeLen, nil
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

// len(data) must be at least 2
func computeInPlaceSizesPhase2(data []int) int {
	if len(data) < 2 {
		return 0
	}

	levelTop := len(data) - 2 //root
	depth := 1
	i := len(data)
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

	return depth - 1
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

	if this.order == 1 {
		return this.writeOrder1(block)
	}

	end := len(block)
	startChunk := 0
	minBufLen := min(this.chunkSize+(this.chunkSize>>3), 2*len(block))
	minBufLen = max(minBufLen, 65536)

	if len(this.buffer) < minBufLen {
		this.buffer = make([]byte, minBufLen)
	}

	for startChunk < end {
		sizeChunk := min(this.chunkSize, end-startChunk)

		if sizeChunk < 32 {
			// Special case for small chunks
			this.bitstream.WriteArray(block[startChunk:], uint(8*sizeChunk))
		} else {
			var freqs [256]int
			internal.ComputeHistogram(block[startChunk:startChunk+sizeChunk], freqs[:], true, false)
			count, err := this.updateFrequencies(freqs[:])

			if err != nil {
				return startChunk, err
			}

			// Skip chunk if only one symbol
			if count > 1 {
				this.encodeChunk(block[startChunk:], sizeChunk)
			}
		}

		startChunk += sizeChunk
	}

	return len(block), nil
}

// writeOrder1 encodes four independent fragments using the preceding byte as context.
func (this *HuffmanEncoder) writeOrder1(block []byte) (int, error) {
	if this.freqs == nil {
		this.freqs = make([]int, _HUF_ORDER1_CONTEXTS*256)
	}

	if this.codesOrder1 == nil {
		this.codesOrder1 = make([]uint16, _HUF_ORDER1_CONTEXTS*256)
	}

	for startChunk := 0; startChunk < len(block); {
		sizeChunk := min(this.chunkSize, len(block)-startChunk)
		src := block[startChunk : startChunk+sizeChunk]

		if sizeChunk < 32 {
			this.bitstream.WriteArray(src, uint(8*sizeChunk))
			startChunk += sizeChunk
			continue
		}

		clear(this.freqs)
		internal.ComputeHistogram(src, this.freqs, false, false)
		fragSize := sizeChunk / 4

		for ctx := 0; ctx < _HUF_ORDER1_CONTEXTS; ctx++ {
			base := ctx << 8

			if _, err := this.updateFrequencies(this.freqs[base : base+256]); err != nil {
				return startChunk, err
			}

			copy(this.codesOrder1[base:base+256], this.codes[:])
		}

		// At most twelve bits per symbol, plus padding and an eight-byte refill guard.
		stride := ((fragSize*_HUF_MAX_SYMBOL_SIZE_V4 + 7) >> 3) + 1
		minBufSize := 4*stride + 8

		if len(this.buffer) < minBufSize {
			this.buffer = make([]byte, minBufSize)
		}

		var nbBits [4]uint32

		for j := 0; j < 4; j++ {
			prev := 0

			if j != 0 {
				prev = int(src[j*fragSize-1])
			}

			state := uint64(0)
			bits := 0
			pos := j * stride
			fragStart := j * fragSize
			fragEnd := fragStart + fragSize
			end4 := fragStart + (fragSize & ^3)

			for i := fragStart; i < end4; i += 4 {
				code0 := this.codesOrder1[(prev<<8)|int(src[i])]
				codeLen0 := int(code0 >> 12)
				prev = int(src[i])
				code1 := this.codesOrder1[(prev<<8)|int(src[i+1])]
				codeLen1 := int(code1 >> 12)
				prev = int(src[i+1])
				code2 := this.codesOrder1[(prev<<8)|int(src[i+2])]
				codeLen2 := int(code2 >> 12)
				prev = int(src[i+2])
				code3 := this.codesOrder1[(prev<<8)|int(src[i+3])]
				codeLen3 := int(code3 >> 12)
				prev = int(src[i+3])

				pack01 := (uint32(code0&0x0FFF) << uint(codeLen1)) | uint32(code1&0x0FFF)
				pack23 := (uint32(code2&0x0FFF) << uint(codeLen3)) | uint32(code3&0x0FFF)
				state = (state << uint(codeLen0+codeLen1)) | uint64(pack01)
				state = (state << uint(codeLen2+codeLen3)) | uint64(pack23)
				bits += codeLen0 + codeLen1 + codeLen2 + codeLen3
				binary.BigEndian.PutUint64(this.buffer[pos:pos+8], state<<uint(64-bits))
				pos += bits >> 3
				bits &= 7
			}

			for i := end4; i < fragEnd; i++ {
				code := this.codesOrder1[(prev<<8)|int(src[i])]
				codeLen := int(code >> 12)
				state = (state << uint(codeLen)) | uint64(code&0x0FFF)
				bits += codeLen
				prev = int(src[i])
			}

			nbBits[j] = uint32((pos-j*stride)*8 + bits)

			for bits >= 8 {
				bits -= 8
				this.buffer[pos] = byte(state >> uint(bits))
				pos++
			}

			if bits != 0 {
				this.buffer[pos] = byte(state << uint(8-bits))
			}
		}

		for j := 0; j < 4; j++ {
			WriteVarInt(this.bitstream, nbBits[j])
		}

		for j := 0; j < 4; j++ {
			this.bitstream.WriteArray(this.buffer[j*stride:], uint(nbBits[j]))
		}

		for i := 4 * fragSize; i < sizeChunk; i++ {
			this.bitstream.WriteBits(uint64(src[i]), 8)
		}

		startChunk += sizeChunk
	}

	return len(block), nil
}

func (this *HuffmanEncoder) encodeChunk(block []byte, count int) {
	nbBits := [4]uint32{0}
	szFrag := count / 4
	szFrag4 := szFrag & ^3
	szBuf := len(this.buffer) / 4

	// Encode chunk
	for j := 0; j < 4; j++ {
		src := block[j*szFrag:]
		buf := this.buffer[j*szBuf:]
		c := this.codes
		idx := 0
		state := uint64(0)
		bits := 0 // number of accumulated bits

		// Encode fragments sequentially
		for i := 0; i < szFrag4; i += 4 {
			var code uint16
			code = c[src[i]]
			codeLen0 := code >> 12
			state = (state << codeLen0) | uint64(code&0x0FFF)
			code = c[src[i+1]]
			codeLen1 := code >> 12
			state = (state << codeLen1) | uint64(code&0x0FFF)
			code = c[src[i+2]]
			codeLen2 := code >> 12
			state = (state << codeLen2) | uint64(code&0x0FFF)
			code = c[src[i+3]]
			codeLen3 := code >> 12
			state = (state << codeLen3) | uint64(code&0x0FFF)
			bits += int(codeLen0 + codeLen1 + codeLen2 + codeLen3)
			binary.BigEndian.PutUint64(buf[idx:idx+8], state<<uint(64-bits)) // bits cannot be 0
			idx += (bits >> 3)
			bits &= 7
		}

		// Fragment last bytes
		for i := szFrag4; i < szFrag; i++ {
			code := c[src[i]]
			codeLen := (code >> 12)
			state = (state << codeLen) | uint64(code&0x0FFF)
			bits += int(codeLen)
		}

		nbBits[j] = uint32((idx * 8) + bits)

		for bits >= 8 {
			bits -= 8
			buf[idx] = byte(state >> uint(bits))
			idx++
		}

		if bits > 0 {
			buf[idx] = byte(state << uint(8-bits))
			idx++
		}
	}

	// Write chunk size in bits
	WriteVarInt(this.bitstream, nbBits[0])
	WriteVarInt(this.bitstream, nbBits[1])
	WriteVarInt(this.bitstream, nbBits[2])
	WriteVarInt(this.bitstream, nbBits[3])

	// Write compressed data to the stream
	this.bitstream.WriteArray(this.buffer[0*szBuf:], uint(nbBits[0]))
	this.bitstream.WriteArray(this.buffer[1*szBuf:], uint(nbBits[1]))
	this.bitstream.WriteArray(this.buffer[2*szBuf:], uint(nbBits[2]))
	this.bitstream.WriteArray(this.buffer[3*szBuf:], uint(nbBits[3]))

	// Chunk last bytes
	count4 := 4 * szFrag

	for i := count4; i < count; i++ {
		this.bitstream.WriteBits(uint64(block[i]), 8)
	}
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
	bitstream     kanzi.InputBitStream
	codes         [256]uint16
	alphabet      [256]int
	sizes         [256]byte
	buffer        []byte
	table         []uint16 // decoding table: code -> size, symbol
	tablesOrder1  []uint16
	chunkSize     int
	order         int
	bsVersion     uint
	maxSymbolSize int
}

// NewHuffmanDecoder creates an instance of HuffmanDecoder.
// Since the number of args is variable, this function can be called like this:
// NewHuffmanDecoder(bs) or NewHuffmanDecoder(bs, 16384) (the second argument
// being the chunk size)
func NewHuffmanDecoder(bs kanzi.InputBitStream, args ...int) (*HuffmanDecoder, error) {
	return NewHuffmanDecoderWithCtxAndOrder(bs, nil, 0, args...)
}

// NewHuffmanDecoderWithOrder creates a Huffman decoder for the requested order.
// The optional chunk size is the order-0 base size and is multiplied by 32 for
// order 1.
func NewHuffmanDecoderWithOrder(bs kanzi.InputBitStream, order int, args ...int) (*HuffmanDecoder, error) {
	return NewHuffmanDecoderWithCtxAndOrder(bs, nil, order, args...)
}

func newHuffmanDecoder(bs kanzi.InputBitStream, ctx *map[string]any, order int, args ...int) (*HuffmanDecoder, error) {
	if bs == nil {
		return nil, errors.New("Huffman codec: Invalid null bitstream parameter")
	}

	if order != 0 && order != 1 {
		return nil, errors.New("Huffman codec: The order must be 0 or 1")
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

	bsVersion := uint(6)

	if ctx != nil {
		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			var ok bool
			bsVersion, ok = val.(uint)

			if ok == false {
				return nil, errors.New("Huffman codec: invalid bitstream version type")
			}
		}
	}

	this := &HuffmanDecoder{
		bitstream:     bs,
		bsVersion:     bsVersion,
		maxSymbolSize: _HUF_MAX_SYMBOL_SIZE_V4,
		table:         make([]uint16, 1<<_HUF_MAX_SYMBOL_SIZE_V4),
		chunkSize:     chkSize << (order * _HUF_ORDER1_CHUNK_SHIFT),
		order:         order,
		buffer:        make([]byte, 0),
	}

	// Default lengths & canonical codes
	for i := 0; i < 256; i++ {
		this.sizes[i] = 8
		this.codes[i] = uint16(i)
	}

	return this, nil
}

// NewHuffmanDecoderWithCtx creates an instance of HuffmanDecoder providing a
// context map.
func NewHuffmanDecoderWithCtx(bs kanzi.InputBitStream, ctx *map[string]any) (*HuffmanDecoder, error) {
	return newHuffmanDecoder(bs, ctx, 0)
}

// NewHuffmanDecoderWithCtxAndOrder creates a Huffman decoder with a context map
// and the requested order.
func NewHuffmanDecoderWithCtxAndOrder(bs kanzi.InputBitStream, ctx *map[string]any,
	order int, args ...int) (*HuffmanDecoder, error) {
	return newHuffmanDecoder(bs, ctx, order, args...)
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

	curSize := int8(2)
	symbols := this.alphabet[0:count]

	// Decode lengths
	for _, s := range symbols {
		if s > 255 {
			return 0, fmt.Errorf("Invalid bitstream: incorrect Huffman symbol %d", s)
		}

		this.codes[s] = 0
		curSize += int8(egdec.DecodeByte())

		if curSize <= 0 || curSize > int8(this.maxSymbolSize) {
			return 0, fmt.Errorf("Invalid bitstream: incorrect size %d for Huffman symbol %d", curSize, s)
		}

		this.sizes[s] = byte(curSize)
	}

	if _, err := generateCanonicalCodes(this.sizes[:], this.codes[:], symbols, this.maxSymbolSize); err != nil {
		return count, err
	}

	egdec.Dispose()
	return count, nil
}

// max(CodeLen) must be <= _HUF_MAX_SYMBOL_SIZE
func (this *HuffmanDecoder) buildDecodingTable(count int) bool {
	// Initialize table with non zero value.
	// If the bitstream is altered, the decoder may access these default table values.
	// The number of consumed bits cannot be 0.
	for i := range this.table {
		this.table[i] = 7
	}

	length := 0
	shift := this.maxSymbolSize
	symbols := this.alphabet[0:count]

	for _, s := range symbols {
		if this.sizes[s] > byte(length) {
			length = int(this.sizes[s])
		}

		// All DECODING_BATCH_SIZE bit values read from the bit stream and
		// starting with the same prefix point to symbol s
		idx := this.codes[s] << (shift - length)
		end := idx + (1 << (shift - length))

		if int(end) > len(this.table) {
			return false
		}

		// code -> size, symbol
		val := (uint16(s) << 8) | uint16(this.sizes[s])
		t := this.table[idx:end]

		for j := range t {
			t[j] = val
		}
	}

	return true
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

	if this.order == 1 {
		return this.decodeOrder1(block)
	}

	if this.bsVersion < 6 {
		return this.decodeV5(block)
	}

	return this.decodeV6(block)
}

func (this *HuffmanDecoder) decodeV5(block []byte) (int, error) {
	end := len(block)
	startChunk := 0

	for startChunk < end {
		sizeChunk := min(this.chunkSize, end-startChunk)

		// For each chunk, read code lengths, rebuild codes, rebuild decoding table
		alphabetSize, err := this.readLengths()

		if alphabetSize == 0 || err != nil {
			return startChunk, err
		}

		if alphabetSize == 1 {
			val := byte(this.alphabet[0])
			b := block[startChunk : startChunk+sizeChunk]

			// Shortcut for chunks with only one symbol
			for i := range b {
				b[i] = val
			}
		} else {
			if this.buildDecodingTable(alphabetSize) == false {
				return 0, errors.New("Invalid bitstream: incorrect symbol size")
			}

			_, err = this.decodeChunkV5(block[startChunk:], sizeChunk)

			if err != nil {
				return startChunk, err
			}
		}

		startChunk += sizeChunk
	}

	return len(block), nil
}

func (this *HuffmanDecoder) decodeV6(block []byte) (int, error) {
	if len(this.buffer) < 2*this.chunkSize {
		this.buffer = make([]byte, 2*this.chunkSize)
	}

	end := len(block)
	startChunk := 0

	for startChunk < end {
		sizeChunk := min(this.chunkSize, end-startChunk)

		if sizeChunk < 32 {
			// Special case for small chunks
			this.bitstream.ReadArray(block[startChunk:], uint(8*sizeChunk))
		} else {
			// For each chunk, read code lengths, rebuild codes, rebuild decoding table
			alphabetSize, err := this.readLengths()

			if alphabetSize == 0 || err != nil {
				return startChunk, err
			}

			if alphabetSize == 1 {
				val := byte(this.alphabet[0])
				b := block[startChunk : startChunk+sizeChunk]

				// Shortcut for chunks with only one symbol
				for i := range b {
					b[i] = val
				}

			} else {
				if this.buildDecodingTable(alphabetSize) == false {
					return startChunk, errors.New("Invalid bitstream: incorrect symbol size")
				}

				_, err = this.decodeChunkV6(block[startChunk:], sizeChunk)

				if err != nil {
					return startChunk, err
				}
			}
		}

		startChunk += sizeChunk
	}

	return len(block), nil
}

func (this *HuffmanDecoder) decodeOrder1(block []byte) (int, error) {
	if this.tablesOrder1 == nil {
		this.tablesOrder1 = make([]uint16, _HUF_ORDER1_CONTEXTS*(1<<_HUF_MAX_SYMBOL_SIZE_V4))
	}

	for startChunk := 0; startChunk < len(block); {
		sizeChunk := min(this.chunkSize, len(block)-startChunk)
		dst := block[startChunk : startChunk+sizeChunk]

		if sizeChunk < 32 {
			this.bitstream.ReadArray(dst, uint(8*sizeChunk))
			startChunk += sizeChunk
			continue
		}

		clear(this.tablesOrder1)

		for ctx := 0; ctx < _HUF_ORDER1_CONTEXTS; ctx++ {
			count, err := this.readLengths()

			if err != nil {
				return startChunk, err
			}

			table := this.tablesOrder1[ctx<<_HUF_MAX_SYMBOL_SIZE_V4 : (ctx+1)<<_HUF_MAX_SYMBOL_SIZE_V4]

			for i := 0; i < count; i++ {
				sym := this.alphabet[i]
				length := int(this.sizes[sym])
				begin := int(this.codes[sym]) << (_HUF_MAX_SYMBOL_SIZE_V4 - length)
				end := begin + (1 << (_HUF_MAX_SYMBOL_SIZE_V4 - length))

				if end > 1<<_HUF_MAX_SYMBOL_SIZE_V4 {
					return startChunk, errors.New("Invalid bitstream: incorrect Huffman symbol size")
				}

				val := uint16((sym << 8) | length)

				for k := begin; k < end; k++ {
					if table[k] != 0 {
						return startChunk, errors.New("Invalid bitstream: duplicate Huffman code")
					}

					table[k] = val
				}
			}
		}

		fragSize := sizeChunk / 4
		var nbBits [4]uint32

		for j := 0; j < 4; j++ {
			nbBits[j] = ReadVarInt(this.bitstream)

			if nbBits[j] < uint32(fragSize) || uint64(nbBits[j]) > uint64(fragSize)*_HUF_MAX_SYMBOL_SIZE_V4 {
				return startChunk, errors.New("Invalid bitstream: incorrect Huffman stream size")
			}
		}

		minBufSize := ((fragSize*_HUF_MAX_SYMBOL_SIZE_V4 + 7) >> 3) + 8

		if len(this.buffer) < minBufSize {
			this.buffer = make([]byte, minBufSize)
		}

		for j := 0; j < 4; j++ {
			payloadBytes := int((nbBits[j] + 7) >> 3)
			this.bitstream.ReadArray(this.buffer, uint(nbBits[j]))
			clear(this.buffer[payloadBytes : payloadBytes+8])

			consumed := uint32(0)
			idx := 0
			state := uint64(0)
			bits := uint8(0)
			prev := 0

			if j != 0 {
				prev = int(dst[j*fragSize-1])
			}

			fragStart := j * fragSize
			fragEnd := fragStart + fragSize
			i := fragStart

			for i < fragEnd-4 {
				bs := this.readState(&state, &idx, &bits)
				val0 := this.tablesOrder1[(prev<<_HUF_MAX_SYMBOL_SIZE_V4)|int((state>>bs)&_HUF_DECODING_MASK_V4)]
				len0 := uint32(val0 & 255)

				if len0 == 0 || consumed+len0 > nbBits[j] {
					return startChunk, errors.New("Invalid bitstream: invalid Huffman context")
				}

				bs -= uint8(len0)
				prev = int(val0 >> 8)
				dst[i] = byte(prev)
				i++

				val1 := this.tablesOrder1[(prev<<_HUF_MAX_SYMBOL_SIZE_V4)|int((state>>bs)&_HUF_DECODING_MASK_V4)]
				len1 := uint32(val1 & 255)

				if len1 == 0 || consumed+len0+len1 > nbBits[j] {
					return startChunk, errors.New("Invalid bitstream: invalid Huffman context")
				}

				bs -= uint8(len1)
				prev = int(val1 >> 8)
				dst[i] = byte(prev)
				i++

				val2 := this.tablesOrder1[(prev<<_HUF_MAX_SYMBOL_SIZE_V4)|int((state>>bs)&_HUF_DECODING_MASK_V4)]
				len2 := uint32(val2 & 255)

				if len2 == 0 || consumed+len0+len1+len2 > nbBits[j] {
					return startChunk, errors.New("Invalid bitstream: invalid Huffman context")
				}

				bs -= uint8(len2)
				prev = int(val2 >> 8)
				dst[i] = byte(prev)
				i++

				val3 := this.tablesOrder1[(prev<<_HUF_MAX_SYMBOL_SIZE_V4)|int((state>>bs)&_HUF_DECODING_MASK_V4)]
				len3 := uint32(val3 & 255)

				if len3 == 0 || consumed+len0+len1+len2+len3 > nbBits[j] {
					return startChunk, errors.New("Invalid bitstream: invalid Huffman context")
				}

				bs -= uint8(len3)
				prev = int(val3 >> 8)
				dst[i] = byte(prev)
				i++
				consumed += len0 + len1 + len2 + len3
				bits = bs + _HUF_MAX_SYMBOL_SIZE_V4
			}

			bs := this.readState(&state, &idx, &bits)

			for i < fragEnd {
				val := this.tablesOrder1[(prev<<_HUF_MAX_SYMBOL_SIZE_V4)|int((state>>bs)&_HUF_DECODING_MASK_V4)]
				length := uint32(val & 255)

				if length == 0 || consumed+length > nbBits[j] {
					return startChunk, errors.New("Invalid bitstream: invalid Huffman context")
				}

				bs -= uint8(length)
				prev = int(val >> 8)
				dst[i] = byte(prev)
				i++
				consumed += length
			}

			used := (idx << 3) - (int(int8(bs)) + _HUF_MAX_SYMBOL_SIZE_V4)

			if consumed != nbBits[j] || used != int(nbBits[j]) {
				return startChunk, errors.New("Invalid bitstream: incorrect Huffman stream size")
			}
		}

		for i := 4 * fragSize; i < sizeChunk; i++ {
			dst[i] = byte(this.bitstream.ReadBits(8))
		}

		startChunk += sizeChunk
	}

	return len(block), nil
}

func (this *HuffmanDecoder) decodeChunkV6(block []byte, count int) (int, error) {
	// Read fragment sizes
	szBits0 := ReadVarInt(this.bitstream)
	szBits1 := ReadVarInt(this.bitstream)
	szBits2 := ReadVarInt(this.bitstream)
	szBits3 := ReadVarInt(this.bitstream)

	if (int(szBits0) < 0) || (int(szBits1) < 0) || (int(szBits2) < 0) || (int(szBits3) < 0) {
		return 0, errors.New("Invalid bitstream: incorrect stream size")
	}

	stride := len(this.buffer) / 4
	base0 := 0 * stride
	base1 := 1 * stride
	base2 := 2 * stride
	base3 := 3 * stride
	idx0 := base0
	idx1 := base1
	idx2 := base2
	idx3 := base3

	// Read all compressed data from bitstream
	this.bitstream.ReadArray(this.buffer[idx0:], uint(szBits0))
	this.bitstream.ReadArray(this.buffer[idx1:], uint(szBits1))
	this.bitstream.ReadArray(this.buffer[idx2:], uint(szBits2))
	this.bitstream.ReadArray(this.buffer[idx3:], uint(szBits3))

	// Ensure deterministic refill words past each stream payload end without
	// clearing the whole reusable buffer.
	if sz0 := idx0 + int((szBits0+7)>>3); sz0 < idx0+stride {
		clear(this.buffer[sz0:min(sz0+8, idx0+stride)])
	}

	if sz1 := idx1 + int((szBits1+7)>>3); sz1 < idx1+stride {
		clear(this.buffer[sz1:min(sz1+8, idx1+stride)])
	}

	if sz2 := idx2 + int((szBits2+7)>>3); sz2 < idx2+stride {
		clear(this.buffer[sz2:min(sz2+8, idx2+stride)])
	}

	if sz3 := idx3 + int((szBits3+7)>>3); sz3 < idx3+stride {
		clear(this.buffer[sz3:min(sz3+8, idx3+stride)])
	}

	// Bits read from bitstream
	state0 := uint64(0)
	state1 := uint64(0)
	state2 := uint64(0)
	state3 := uint64(0)

	// Number of available bits in state
	bits0 := uint8(0)
	bits1 := uint8(0)
	bits2 := uint8(0)
	bits3 := uint8(0)

	var bs0 uint8
	var bs1 uint8
	var bs2 uint8
	var bs3 uint8

	szFrag := count / 4
	block0 := block[0*szFrag:]
	block1 := block[1*szFrag:]
	block2 := block[2*szFrag:]
	block3 := block[3*szFrag:]
	n := 0

	for n < szFrag-4 {
		// Fill 64 bits of state from the bitstream for each stream
		bs0 = this.readState(&state0, &idx0, &bits0)
		bs1 = this.readState(&state1, &idx1, &bits1)
		bs2 = this.readState(&state2, &idx2, &bits2)
		bs3 = this.readState(&state3, &idx3, &bits3)

		// Decompress 4 symbols per stream
		val00 := this.table[(state0>>bs0)&_HUF_DECODING_MASK_V4]
		bs0 -= uint8(val00)
		val10 := this.table[(state1>>bs1)&_HUF_DECODING_MASK_V4]
		bs1 -= uint8(val10)
		val20 := this.table[(state2>>bs2)&_HUF_DECODING_MASK_V4]
		bs2 -= uint8(val20)
		val30 := this.table[(state3>>bs3)&_HUF_DECODING_MASK_V4]
		bs3 -= uint8(val30)
		val01 := this.table[(state0>>bs0)&_HUF_DECODING_MASK_V4]
		bs0 -= uint8(val01)
		val11 := this.table[(state1>>bs1)&_HUF_DECODING_MASK_V4]
		bs1 -= uint8(val11)
		val21 := this.table[(state2>>bs2)&_HUF_DECODING_MASK_V4]
		bs2 -= uint8(val21)
		val31 := this.table[(state3>>bs3)&_HUF_DECODING_MASK_V4]
		bs3 -= uint8(val31)
		val02 := this.table[(state0>>bs0)&_HUF_DECODING_MASK_V4]
		bs0 -= uint8(val02)
		val12 := this.table[(state1>>bs1)&_HUF_DECODING_MASK_V4]
		bs1 -= uint8(val12)
		val22 := this.table[(state2>>bs2)&_HUF_DECODING_MASK_V4]
		bs2 -= uint8(val22)
		val32 := this.table[(state3>>bs3)&_HUF_DECODING_MASK_V4]
		bs3 -= uint8(val32)
		val03 := this.table[(state0>>bs0)&_HUF_DECODING_MASK_V4]
		bs0 -= uint8(val03)
		val13 := this.table[(state1>>bs1)&_HUF_DECODING_MASK_V4]
		bs1 -= uint8(val13)
		val23 := this.table[(state2>>bs2)&_HUF_DECODING_MASK_V4]
		bs2 -= uint8(val23)
		val33 := this.table[(state3>>bs3)&_HUF_DECODING_MASK_V4]
		bs3 -= uint8(val33)

		bits0 = bs0 + _HUF_MAX_SYMBOL_SIZE_V4
		bits1 = bs1 + _HUF_MAX_SYMBOL_SIZE_V4
		bits2 = bs2 + _HUF_MAX_SYMBOL_SIZE_V4
		bits3 = bs3 + _HUF_MAX_SYMBOL_SIZE_V4

		block0[n+0] = byte(val00 >> 8)
		block1[n+0] = byte(val10 >> 8)
		block2[n+0] = byte(val20 >> 8)
		block3[n+0] = byte(val30 >> 8)
		block0[n+1] = byte(val01 >> 8)
		block1[n+1] = byte(val11 >> 8)
		block2[n+1] = byte(val21 >> 8)
		block3[n+1] = byte(val31 >> 8)
		block0[n+2] = byte(val02 >> 8)
		block1[n+2] = byte(val12 >> 8)
		block2[n+2] = byte(val22 >> 8)
		block3[n+2] = byte(val32 >> 8)
		block0[n+3] = byte(val03 >> 8)
		block1[n+3] = byte(val13 >> 8)
		block2[n+3] = byte(val23 >> 8)
		block3[n+3] = byte(val33 >> 8)
		n += 4
	}

	// Fill 64 bits of state from the bitstream for each stream
	bs0 = this.readState(&state0, &idx0, &bits0)
	bs1 = this.readState(&state1, &idx1, &bits1)
	bs2 = this.readState(&state2, &idx2, &bits2)
	bs3 = this.readState(&state3, &idx3, &bits3)

	for n < szFrag {
		// Decompress 1 symbol per stream
		val0 := this.table[(state0>>bs0)&_HUF_DECODING_MASK_V4]
		bs0 -= uint8(val0)
		val1 := this.table[(state1>>bs1)&_HUF_DECODING_MASK_V4]
		bs1 -= uint8(val1)
		val2 := this.table[(state2>>bs2)&_HUF_DECODING_MASK_V4]
		bs2 -= uint8(val2)
		val3 := this.table[(state3>>bs3)&_HUF_DECODING_MASK_V4]
		bs3 -= uint8(val3)

		block0[n] = byte(val0 >> 8)
		block1[n] = byte(val1 >> 8)
		block2[n] = byte(val2 >> 8)
		block3[n] = byte(val3 >> 8)
		n++
	}

	// Process any remaining bytes at the end of the whole chunk
	count4 := 4 * szFrag

	for i := count4; i < count; i++ {
		block[i] = byte(this.bitstream.ReadBits(8))
	}

	if (((idx0-base0)<<3)-int(bs0+_HUF_MAX_SYMBOL_SIZE_V4) != int(szBits0)) ||
		(((idx1-base1)<<3)-int(bs1+_HUF_MAX_SYMBOL_SIZE_V4) != int(szBits1)) ||
		(((idx2-base2)<<3)-int(bs2+_HUF_MAX_SYMBOL_SIZE_V4) != int(szBits2)) ||
		(((idx3-base3)<<3)-int(bs3+_HUF_MAX_SYMBOL_SIZE_V4) != int(szBits3)) {
		return 0, errors.New("Invalid bitstream: incorrect Huffman stream size")
	}

	return count, nil
}

func (this *HuffmanDecoder) readState(state *uint64, idx *int, bits *uint8) uint8 {
	shift := (56 - *bits) & ^uint8(0x07)
	*state = (*state << shift) | (binary.BigEndian.Uint64(this.buffer[*idx:]) >> (64 - shift))
	*idx += int(shift >> 3)
	return *bits + shift - _HUF_MAX_SYMBOL_SIZE_V4
}

func (this *HuffmanDecoder) decodeChunkV5(block []byte, count int) (int, error) {
	// Read number of streams. Only 1 stream supported
	if this.bitstream.ReadBits(2) != 0 {
		return 0, errors.New("Invalid Huffman data: only one stream supported in this version")
	}

	// Read chunk size
	szBits := ReadVarInt(this.bitstream)

	// A non-empty chunk with multiple symbols cannot have an empty
	// encoded payload. Reject malformed input instead of reporting a
	// successful decode while leaving the destination untouched.
	if szBits == 0 {
		return 0, errors.New("Invalid bitstream: empty Huffman payload")
	}

	// Read compressed data from the bitstream
	if szBits != 0 {
		sz := int(szBits+7) >> 3
		minLenBuf := max(sz+(sz>>3), 1024)

		if len(this.buffer) < int(minLenBuf) {
			this.buffer = make([]byte, minLenBuf)
		}

		this.bitstream.ReadArray(this.buffer, uint(szBits))
		state := uint64(0)
		bits := uint8(0)
		idx := 0
		n := 0

		for idx < sz-8 {
			shift := (56 - bits) & ^uint8(0x07)
			state = (state << shift) | (binary.BigEndian.Uint64(this.buffer[idx:idx+8]) >> (64 - shift))
			idx += int(shift >> 3)
			bs := bits + shift - _HUF_MAX_SYMBOL_SIZE_V4
			val0 := this.table[(state>>bs)&_HUF_DECODING_MASK_V4]
			bs -= uint8(val0)
			val1 := this.table[(state>>bs)&_HUF_DECODING_MASK_V4]
			bs -= uint8(val1)
			val2 := this.table[(state>>bs)&_HUF_DECODING_MASK_V4]
			bs -= uint8(val2)
			val3 := this.table[(state>>bs)&_HUF_DECODING_MASK_V4]
			bs -= uint8(val3)
			bits = bs + _HUF_MAX_SYMBOL_SIZE_V4
			block[n+0] = byte(val0 >> 8)
			block[n+1] = byte(val1 >> 8)
			block[n+2] = byte(val2 >> 8)
			block[n+3] = byte(val3 >> 8)
			n += 4
		}

		// Last bytes
		for n < count {
			for (bits < _HUF_MAX_SYMBOL_SIZE_V4) && (idx < sz) {
				state = (state << 8) | uint64(this.buffer[idx]&0xFF)
				idx++

				// 'bits' may overshoot when idx == sz due to padding state bits
				// It is necessary to compute proper table indexes
				// and has no consequences (except bits != 0 at the end of chunk)
				bits += 8
			}

			// Sanity check
			if bits > 64 {
				return n, errors.New("Invalid bitstream: incorrect symbol size")
			}

			var val uint16

			if bits >= _HUF_MAX_SYMBOL_SIZE_V4 {
				val = this.table[(state>>(bits-_HUF_MAX_SYMBOL_SIZE_V4))&_HUF_DECODING_MASK_V4]
			} else {
				val = this.table[(state<<(_HUF_MAX_SYMBOL_SIZE_V4-bits))&_HUF_DECODING_MASK_V4]
			}

			bits -= uint8(val)
			block[n] = byte(val >> 8)
			n++
		}

		if ((idx << 3) - int(bits)) != int(szBits) {
			return 0, errors.New("Invalid bitstream: incorrect Huffman stream size")
		}
	}

	return count, nil
}

// BitStream returns the underlying bitstream
func (this *HuffmanDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Dispose this implementation does nothing
func (this *HuffmanDecoder) Dispose() {
}
