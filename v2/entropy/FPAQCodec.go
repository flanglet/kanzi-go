/*
Copyright 2011-2025 Frederic Langlet
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

	kanzi "github.com/flanglet/kanzi-go/v2"
)

const (
	_FPAQ_PSCALE             = 1 << 16
	_FPAQ_DEFAULT_CHUNK_SIZE = 4 * 1024 * 1024
	_FPAQ_ENTROPY_TOP        = uint64(0x00FFFFFFFFFFFFFF)
	_FPAQ_MASK_0_56          = uint64(0x00FFFFFFFFFFFFFF)
	_FPAQ_MASK_0_24          = uint64(0x0000000000FFFFFF)
	_FPAQ_MASK_0_32          = uint64(0x00000000FFFFFFFF)
)

// FPAQEncoder entropy encoder derived from fpaq0r by Matt Mahoney & Alexander Ratushnyak.
// See http://mattmahoney.net/dc/#fpaq0.
// Simple (and fast) adaptive entropy bit encoder
type FPAQEncoder struct {
	low       uint64
	high      uint64
	bitstream kanzi.OutputBitStream
	disposed  bool
	buffer    []byte
	index     int
	probs     [4][]int // probability of bit=1
	ctxIdx    byte     // previous bits
}

// NewFPAQEncoder creates an instance of FPAQEncoder providing a
// context map.
func NewFPAQEncoder(bs kanzi.OutputBitStream) (*FPAQEncoder, error) {
	if bs == nil {
		return nil, errors.New("FPAQ codec: Invalid null bitstream parameter")
	}

	this := &FPAQEncoder{}
	this.low = 0
	this.high = _FPAQ_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	this.ctxIdx = 1

	for i := 0; i < 4; i++ {
		this.probs[i] = make([]int, 256)

		for j := range this.probs[0] {
			this.probs[i][j] = _FPAQ_PSCALE >> 1
		}
	}

	return this, nil
}

// NewFPAQEncoderWithCtx creates an instance of FPAQEncoder
func NewFPAQEncoderWithCtx(bs kanzi.OutputBitStream, ctx *map[string]any) (*FPAQEncoder, error) {
	if bs == nil {
		return nil, errors.New("FPAQ codec: Invalid null bitstream parameter")
	}

	this := &FPAQEncoder{}
	this.low = 0
	this.high = _FPAQ_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	this.ctxIdx = 1

	for i := 0; i < 4; i++ {
		this.probs[i] = make([]int, 256)

		for j := range this.probs[0] {
			this.probs[i][j] = _FPAQ_PSCALE >> 1
		}
	}

	return this, nil
}

func (this *FPAQEncoder) encodeBit(bit byte, p *int) {
	// Calculate interval split
	// Written in a way to maximize accuracy of multiplication/division
	split := (((this.high - this.low) >> 8) * uint64(*p)) >> 8

	// Update probabilities
	if bit == 0 {
		this.low += (split + 1)
		*p -= (*p >> 6)
	} else {
		this.high = this.low + split
		*p -= ((*p - _FPAQ_PSCALE + 64) >> 6)
	}

	// Write unchanged first 32 bits to bitstream
	if (this.low ^ this.high) < (1 << 24) {
		this.flush()
	}
}

// Write encodes the data provided into the bitstream. Return the number of byte
// written to the bitstream. Splits big blocks into chunks and encode the chunks
// byte by byte sequentially into the bitstream.
func (this *FPAQEncoder) Write(block []byte) (int, error) {
	count := len(block)

	if count > 1<<30 {
		return 0, fmt.Errorf("FPAQ codec: Invalid block size parameter (max is 1<<30): got %v", count)
	}

	startChunk := 0
	end := count

	// Split block into chunks, read bit array from bitstream and decode chunk
	for startChunk < end {
		chunkSize := _FPAQ_DEFAULT_CHUNK_SIZE

		if startChunk+_FPAQ_DEFAULT_CHUNK_SIZE >= end {
			chunkSize = end - startChunk
		}

		if len(this.buffer) < (chunkSize + (chunkSize >> 3)) {
			this.buffer = make([]byte, chunkSize+(chunkSize>>3))
		}

		this.index = 0
		buf := block[startChunk : startChunk+chunkSize]
		p := this.probs[0]

		for _, val := range buf {
			bits := int(val) + 256
			this.encodeBit(val&0x80, &p[1])
			this.encodeBit(val&0x40, &p[bits>>7])
			this.encodeBit(val&0x20, &p[bits>>6])
			this.encodeBit(val&0x10, &p[bits>>5])
			this.encodeBit(val&0x08, &p[bits>>4])
			this.encodeBit(val&0x04, &p[bits>>3])
			this.encodeBit(val&0x02, &p[bits>>2])
			this.encodeBit(val&0x01, &p[bits>>1])
			p = this.probs[val>>6]
		}

		WriteVarInt(this.bitstream, uint32(this.index))
		this.bitstream.WriteArray(this.buffer, uint(8*this.index))
		startChunk += chunkSize

		if startChunk < end {
			this.bitstream.WriteBits(this.low|_FPAQ_MASK_0_24, 56)
		}
	}

	return count, nil
}

func (this *FPAQEncoder) flush() {
	binary.BigEndian.PutUint32(this.buffer[this.index:], uint32(this.high>>24))
	this.index += 4
	this.low <<= 32
	this.high = (this.high << 32) | _FPAQ_MASK_0_32
}

// BitStream returns the underlying bitstream
func (this *FPAQEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// Dispose must be called before getting rid of the entropy encoder
// This idempotent implementation writes the last buffered bits into the
// bitstream.
func (this *FPAQEncoder) Dispose() {
	if this.disposed == true {
		return
	}

	this.disposed = true
	this.bitstream.WriteBits(this.low|_FPAQ_MASK_0_24, 56)
}

// FPAQDecoder entropy decoder derived from fpaq0r by Matt Mahoney & Alexander Ratushnyak.
// See http://mattmahoney.net/dc/#fpaq0.
// Simple (and fast) adaptive entropy bit decoder
type FPAQDecoder struct {
	low          uint64
	high         uint64
	current      uint64
	bitstream    kanzi.InputBitStream
	buffer       []byte
	index        int
	probs        [4][]int // probability of bit=1
	p            []int    // pointer to current prob
	ctx          byte     // previous bits
	isBsVersion3 bool
}

// NewFPAQDecoder creates an instance of FPAQDecoder
func NewFPAQDecoder(bs kanzi.InputBitStream) (*FPAQDecoder, error) {
	if bs == nil {
		return nil, errors.New("FPAQ codec: Invalid null bitstream parameter")
	}

	this := &FPAQDecoder{}
	this.low = 0
	this.high = _FPAQ_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	this.ctx = 1
	this.p = this.probs[0]
	this.isBsVersion3 = false
	return this, nil
}

// NewFPAQDecoderWithCtx creates an instance of FPAQDecoder providing a
// context map.
func NewFPAQDecoderWithCtx(bs kanzi.InputBitStream, ctx *map[string]any) (*FPAQDecoder, error) {
	if bs == nil {
		return nil, errors.New("FPAQ codec: Invalid null bitstream parameter")
	}

	this := &FPAQDecoder{}
	this.low = 0
	this.high = _FPAQ_ENTROPY_TOP
	this.bitstream = bs
	this.buffer = make([]byte, 0)
	this.index = 0
	this.ctx = 1
	this.p = this.probs[0]

	for i := 0; i < 4; i++ {
		this.probs[i] = make([]int, 256)

		for j := range this.probs[0] {
			this.probs[i][j] = _FPAQ_PSCALE >> 1
		}
	}

	bsVersion := uint(4)

	if ctx != nil {
		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			bsVersion = val.(uint)
		}
	}

	this.isBsVersion3 = bsVersion < 4
	return this, nil
}

func (this *FPAQDecoder) decodeBitV1(pred int) byte {
	// Calculate interval split
	split := ((((this.high - this.low) >> 4) * uint64(pred)) >> 8) + this.low
	var bit byte

	// Update probabilities
	if split >= this.current {
		bit = 1
		this.high = split
		this.p[this.ctx] -= ((this.p[this.ctx] - _FPAQ_PSCALE + 64) >> 6)
		this.ctx += (this.ctx + 1)
	} else {
		bit = 0
		this.low = -^split
		this.p[this.ctx] -= (this.p[this.ctx] >> 6)
		this.ctx += this.ctx
	}

	// Read 32 bits from bitstream
	for (this.low^this.high)>>24 == 0 {
		this.read()
	}

	return bit
}

func (this *FPAQDecoder) decodeBitV2(p []int) byte {
	// Calculate interval split
	split := ((((this.high - this.low) >> 8) * uint64(p[this.ctx])) >> 8) + this.low
	var bit byte

	// Update probabilities
	if split >= this.current {
		bit = 1
		this.high = split
		p[this.ctx] -= ((p[this.ctx] - _FPAQ_PSCALE + 64) >> 6)
		this.ctx += (this.ctx + 1)
	} else {
		bit = 0
		this.low = -^split
		p[this.ctx] -= (p[this.ctx] >> 6)
		this.ctx += this.ctx
	}

	// Read 32 bits from bitstream
	if (this.low ^ this.high) < (1 << 24) {
		this.read()
	}

	return bit
}

func (this *FPAQDecoder) read() {
	this.low = (this.low << 32) & _FPAQ_MASK_0_56
	this.high = ((this.high << 32) | _FPAQ_MASK_0_32) & _FPAQ_MASK_0_56
	val := uint64(binary.BigEndian.Uint32(this.buffer[this.index:]))
	this.current = ((this.current << 32) | val) & _FPAQ_MASK_0_56
	this.index += 4
}

// Read decodes data from the bitstream and return it in the provided buffer.
// Return the number of bytes read from the bitstream.
// Splits big blocks into chunks and decode the chunks byte by byte sequentially from the bitstream.
func (this *FPAQDecoder) Read(block []byte) (int, error) {
	count := len(block)

	if count > 1<<30 {
		return 0, fmt.Errorf("FPAQ codec: Invalid block size parameter (max is 1<<30): got %v", count)
	}

	startChunk := 0
	end := count

	// Split block into chunks, read bit array from bitstream and decode chunk
	for startChunk < end {
		szBytes := int(ReadVarInt(this.bitstream))

		if szBytes < 0 || szBytes >= 2*len(block) {
			return 0, fmt.Errorf("FPAQ codec: Invalid chunk size (%v)", szBytes)
		}

		bufSize := max(int(szBytes+(szBytes>>2)), 1024)

		if len(this.buffer) < bufSize {
			this.buffer = make([]byte, bufSize)
		}

		this.current = this.bitstream.ReadBits(56)

		if bufSize > szBytes {
			for i := range this.buffer[szBytes:] {
				this.buffer[i] = 0
			}
		}

		this.bitstream.ReadArray(this.buffer, uint(8*szBytes))
		this.index = 0
		chunkSize := min(_FPAQ_DEFAULT_CHUNK_SIZE, end-startChunk)
		buf := block[startChunk : startChunk+chunkSize]

		if this.isBsVersion3 == true {
			this.p = this.probs[0]

			for i := range buf {
				this.ctx = 1
				this.decodeBitV1(this.p[this.ctx] >> 4)
				this.decodeBitV1(this.p[this.ctx] >> 4)
				this.decodeBitV1(this.p[this.ctx] >> 4)
				this.decodeBitV1(this.p[this.ctx] >> 4)
				this.decodeBitV1(this.p[this.ctx] >> 4)
				this.decodeBitV1(this.p[this.ctx] >> 4)
				this.decodeBitV1(this.p[this.ctx] >> 4)
				this.decodeBitV1(this.p[this.ctx] >> 4)
				buf[i] = byte(this.ctx)
				this.p = this.probs[(this.ctx&0xFF)>>6]
			}
		} else {
			p := this.probs[0]

			for i := range buf {
				this.ctx = 1
				this.decodeBitV2(p)
				this.decodeBitV2(p)
				this.decodeBitV2(p)
				this.decodeBitV2(p)
				this.decodeBitV2(p)
				this.decodeBitV2(p)
				this.decodeBitV2(p)
				this.decodeBitV2(p)
				buf[i] = byte(this.ctx)
				p = this.probs[(this.ctx&0xFF)>>6]
			}
		}

		startChunk += chunkSize
	}

	return count, nil
}

// BitStream returns the underlying bitstream
func (this *FPAQDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Dispose must be called before getting rid of the entropy decoder
// This implementation does nothing.
func (this *FPAQDecoder) Dispose() {
}
