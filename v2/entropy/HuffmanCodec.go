/*
Copyright 2011-2024 Frederic Langlet
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
	_HUF_MAX_CHUNK_SIZE     = uint(1 << 14)
	_HUF_MAX_SYMBOL_SIZE_V3 = 14
	_HUF_MAX_SYMBOL_SIZE_V4 = 12
	_HUF_BUFFER_SIZE        = (_HUF_MAX_SYMBOL_SIZE_V3 << 8) + 256
	_HUF_DECODING_MASK_V3   = (1 << _HUF_MAX_SYMBOL_SIZE_V3) - 1
	_HUF_DECODING_MASK_V4   = (1 << _HUF_MAX_SYMBOL_SIZE_V4) - 1
)

// Return the number of codes generated
func generateCanonicalCodes(sizes []byte, codes []uint16, symbols []int, maxSymbolSize int) (int, error) {
	count := len(symbols)

	if count == 0 {
		return 0, nil
	}

	if count > 1 {
		var buf [_HUF_BUFFER_SIZE]byte

		for _, s := range symbols {
			if s > 255 {
				return -1, errors.New("Could not generate Huffman codes: invalid code length")
			}

			// Max length reached
			if sizes[s] > byte(maxSymbolSize) {
				return -1, fmt.Errorf("Could not generate Huffman codes: max code length (%d bits) exceeded", maxSymbolSize)
			}

			buf[(int(sizes[s]-1)<<8)|s] = 1
		}

		for i, n := 0, 0; n < count; i++ {
			symbols[n] = i & 0xFF
			n += int(buf[i])
		}
	}

	code := uint16(0)
	curLen := sizes[symbols[0]]

	for _, s := range symbols {
		if sizes[s] > curLen {
			code <<= (sizes[s] - curLen)
			curLen = sizes[s]
		}

		codes[s] = code
		code++
	}

	return count, nil
}

// HuffmanEncoder  Implementation of a static Huffman encoder.
// Uses in place generation of canonical codes instead of a tree
type HuffmanEncoder struct {
	bitstream kanzi.OutputBitStream
	codes     [256]uint16
	buffer    []byte
	chunkSize int
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

		if chkSize < _HUF_MIN_CHUNK_SIZE {
			return nil, fmt.Errorf("Huffman codec: The chunk size must be at least %d", _HUF_MIN_CHUNK_SIZE)
		}

		if chkSize > _HUF_MAX_CHUNK_SIZE {
			return nil, fmt.Errorf("Huffman codec: The chunk size must be at most %d", _HUF_MAX_CHUNK_SIZE)
		}
	}

	this := &HuffmanEncoder{}
	this.bitstream = bs
	this.chunkSize = int(chkSize)

	// Default frequencies, sizes and codes
	for i := 0; i < 256; i++ {
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
		retries := uint(0)
		var ranks [256]int

		for {
			// Sort ranks by increasing freqs (first key) and increasing value (second key)
			for i := range symbols {
				ranks[i] = (freqs[symbols[i]] << 8) | symbols[i]
			}

			var maxCodeLen int
			var err error

			if maxCodeLen, err = this.computeCodeLengths(sizes[:], ranks[0:count]); err != nil {
				return count, err
			}

			if maxCodeLen <= _HUF_MAX_SYMBOL_SIZE_V4 {
				// Usual case
				if _, err := generateCanonicalCodes(sizes[:], this.codes[:], ranks[0:count], _HUF_MAX_SYMBOL_SIZE_V4); err != nil {
					return count, err
				}

				break
			}

			// Sometimes, codes exceed the budget for the max code length => normalize
			// frequencies (boost the smallest frequencies) and try once more.
			if retries > 2 {
				return count, fmt.Errorf("Could not generate Huffman codes: max code length (%d bits) exceeded, ", _HUF_MAX_SYMBOL_SIZE_V4)
			}

			retries++
			var f [256]int
			var alpha [256]int
			totalFreq := 0

			for i := range symbols {
				f[i] = freqs[symbols[i]]
				totalFreq += f[i]
			}

			// Normalize to a smaller scale
			if _, err := NormalizeFrequencies(f[:count], alpha[:count], totalFreq, int(_HUF_MAX_CHUNK_SIZE>>(retries+1))); err != nil {
				return count, err
			}

			for i := range symbols {
				freqs[symbols[i]] = f[i]
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

	if maxCodeLen <= _HUF_MAX_SYMBOL_SIZE_V4 {
		for i := range freqs {
			sizes[ranks[i]] = byte(freqs[i])
		}
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

	end := len(block)
	startChunk := 0
	minBufLen := this.chunkSize + (this.chunkSize >> 3)

	if minBufLen > 2*len(block) {
		minBufLen = 2 * len(block)
	}

	if minBufLen < 65536 {
		minBufLen = 65536
	}

	if len(this.buffer) < minBufLen {
		this.buffer = make([]byte, minBufLen)
	}

	for startChunk < end {
		endChunk := startChunk + this.chunkSize

		if endChunk > len(block) {
			endChunk = len(block)
		}

		var freqs [256]int
		internal.ComputeHistogram(block[startChunk:endChunk], freqs[:], true, false)
		count, err := this.updateFrequencies(freqs[:])

		if err != nil {
			return startChunk, err
		}

		if count <= 1 {
			// Skip chunk if only one symbol
			startChunk = endChunk
			continue
		}

		endChunk4 := ((endChunk - startChunk) & -4) + startChunk
		c := this.codes
		idx := 0
		state := uint64(0)
		bits := 0 // number of accumulated bits

		// Encode chunk
		for i := startChunk; i < endChunk4; i += 4 {
			var code uint16
			code = c[block[i]]
			codeLen0 := (c[block[i]] >> 12)
			state = (state << codeLen0) | uint64(code&0x0FFF)
			code = c[block[i+1]]
			codeLen1 := (code >> 12)
			state = (state << codeLen1) | uint64(code&0x0FFF)
			code = c[block[i+2]]
			codeLen2 := (code >> 12)
			state = (state << codeLen2) | uint64(code&0x0FFF)
			code = c[block[i+3]]
			codeLen3 := (code >> 12)
			state = (state << codeLen3) | uint64(code&0x0FFF)
			bits += int(codeLen0 + codeLen1 + codeLen2 + codeLen3)
			binary.BigEndian.PutUint64(this.buffer[idx:idx+8], state<<uint(64-bits))
			idx += (bits >> 3)
			bits &= 7
		}

		for i := endChunk4; i < endChunk; i++ {
			code := c[block[i]]
			codeLen := (code >> 12)
			state = (state << codeLen) | uint64(code&0x0FFF)
			bits += int(codeLen)
		}

		nbBits := (idx * 8) + bits

		for bits >= 8 {
			bits -= 8
			this.buffer[idx] = byte(state >> uint(bits))
			idx++
		}

		if bits > 0 {
			this.buffer[idx] = byte(state << uint(8-bits))
			idx++
		}

		// Write number of streams (0->1, 1->4, 2->8, 3->32)
		this.bitstream.WriteBits(0, 2)

		// Write chunk size in bits
		WriteVarInt(this.bitstream, uint32(nbBits))

		// Write compressed data to the stream
		this.bitstream.WriteArray(this.buffer[0:], uint(nbBits))

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
	bitstream     kanzi.InputBitStream
	codes         [256]uint16
	alphabet      [256]int
	sizes         [256]byte
	buffer        []byte
	table         []uint16 // decoding table: code -> size, symbol
	chunkSize     int
	isBsVersion3  bool
	maxSymbolSize int
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

	this := &HuffmanDecoder{}
	this.bitstream = bs
	this.isBsVersion3 = false
	this.maxSymbolSize = _HUF_MAX_SYMBOL_SIZE_V4
	this.table = make([]uint16, 1<<this.maxSymbolSize)
	this.chunkSize = int(chkSize)
	this.buffer = make([]byte, 0)

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
	if bs == nil {
		return nil, errors.New("Huffman codec: Invalid null bitstream parameter")
	}

	bsVersion := uint(4)

	if ctx != nil {
		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			bsVersion = val.(uint)
		}
	}

	this := &HuffmanDecoder{}
	this.bitstream = bs
	this.isBsVersion3 = bsVersion < 4

	this.maxSymbolSize = _HUF_MAX_SYMBOL_SIZE_V4

	if this.isBsVersion3 {
		this.maxSymbolSize = _HUF_MAX_SYMBOL_SIZE_V3
	}

	this.table = make([]uint16, 1<<this.maxSymbolSize)
	this.chunkSize = int(_HUF_MAX_CHUNK_SIZE)
	this.buffer = make([]byte, 0)

	// Default lengths & canonical codes
	for i := 0; i < 256; i++ {
		this.sizes[i] = 8
		this.codes[i] = uint16(i)
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
func (this *HuffmanDecoder) buildDecodingTable(count int) {
	for i := range this.table {
		this.table[i] = 0
	}

	length := 0
	shift := this.maxSymbolSize
	symbols := this.alphabet[0:count]

	for _, s := range symbols {
		if this.sizes[s] > byte(length) {
			length = int(this.sizes[s])
		}

		// code -> size, symbol
		val := (uint16(s) << 8) | uint16(this.sizes[s])
		code := this.codes[s]

		// All DECODING_BATCH_SIZE bit values read from the bit stream and
		// starting with the same prefix point to symbol s
		idx := code << (shift - length)
		end := idx + (1 << (shift - length))
		t := this.table[idx:end]

		for j := range t {
			t[j] = val
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
		endChunk := startChunk + this.chunkSize

		if endChunk > end {
			endChunk = end
		}

		// For each chunk, read code lengths, rebuild codes, rebuild decoding table
		alphabetSize, err := this.readLengths()

		if alphabetSize == 0 || err != nil {
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

		this.buildDecodingTable(alphabetSize)

		if this.isBsVersion3 == true {
			// Compute minimum number of bits required in bitstream for fast decoding
			minCodeLen := int(this.sizes[this.alphabet[0]]) // not 0
			padding := 64 / minCodeLen

			if minCodeLen*padding != 64 {
				padding++
			}

			endChunk2 := startChunk
			szChunk := endChunk - startChunk - padding

			if szChunk > 0 {
				endChunk2 += (szChunk & -2)
			}

			bits := byte(0)
			st := uint64(0)

			for i := startChunk; i < endChunk2; i += 2 {
				if bits < 32 {
					st = (st << 32) | this.bitstream.ReadBits(32)
					bits += 32
				}

				val0 := this.table[int(st>>(bits-_HUF_MAX_SYMBOL_SIZE_V3))&_HUF_DECODING_MASK_V3]
				bits -= byte(val0)
				val1 := this.table[int(st>>(bits-_HUF_MAX_SYMBOL_SIZE_V3))&_HUF_DECODING_MASK_V3]
				bits -= byte(val1)
				block[i] = byte(val0 >> 8)
				block[i+1] = byte(val1 >> 8)
			}

			// Fallback to slow decoding
			for i := endChunk2; i < endChunk; i++ {
				code := 0
				codeLen := uint8(0)

				for {
					codeLen++

					if bits == 0 {
						code = (code << 1) | this.bitstream.ReadBit()
					} else {
						bits--
						code = (code << 1) | int((st>>bits)&1)
					}

					idx := code << (_HUF_MAX_SYMBOL_SIZE_V3 - codeLen)

					if uint8(this.table[idx]) == codeLen {
						block[i] = byte(this.table[idx] >> 8)
						break
					}

					if codeLen >= _HUF_MAX_SYMBOL_SIZE_V3 {
						panic(errors.New("Invalid bitstream: incorrect Huffman code"))
					}
				}
			}
		} else {
			// bsVersion >= 4
			// Read number of streams. Only 1 stream supported for now
			if this.bitstream.ReadBits(2) != 0 {
				return startChunk, errors.New("Invalid Huffman data: number streams not supported in this version")
			}

			// Read chunk size
			szBits := ReadVarInt(this.bitstream)

			// Read compressed data from the bitstream
			if szBits != 0 {
				sz := int(szBits+7) >> 3
				minLenBuf := sz + (sz >> 3)

				if minLenBuf < 1024 {
					minLenBuf = 1024
				}

				if len(this.buffer) < int(minLenBuf) {
					this.buffer = make([]byte, minLenBuf)
				}

				this.bitstream.ReadArray(this.buffer, uint(szBits))
				state := uint64(0)
				bits := uint8(0)
				idx := 0
				n := startChunk

				for idx < sz-8 {
					shift := uint8((56 - bits) & 0xF8)
					state = (state << shift) | (binary.BigEndian.Uint64(this.buffer[idx:idx+8]) >> 1 >> (63 - shift)) // handle shift = 0
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
				nbBits := idx * 8

				for n < endChunk {
					for (bits < _HUF_MAX_SYMBOL_SIZE_V4) && (idx < sz) {
						state = (state << 8) | uint64(this.buffer[idx]&0xFF)
						idx++

						if idx == sz {
							nbBits = int(szBits)
						} else {
							nbBits += 8
						}

						// 'bits' may overshoot when idx == sz due to padding state bits
						// It is necessary to compute proper table indexes
						// and has no consequences (except bits != 0 at the end of chunk)
						bits += 8
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
			}
		}

		startChunk = endChunk
	}

	return len(block), nil
}

// BitStream returns the underlying bitstream
func (this *HuffmanDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Dispose this implementation does nothing
func (this *HuffmanDecoder) Dispose() {
}
