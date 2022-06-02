/*
Copyright 2011-2021 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License")
you may not use this file except in compliance with the License.
you may obtain a copy of the License at

                http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package transform

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"strings"

	kanzi "github.com/flanglet/kanzi-go"
	"github.com/flanglet/kanzi-go/bitstream"
	"github.com/flanglet/kanzi-go/entropy"
	"github.com/flanglet/kanzi-go/util"
)

// Implementation of a Reduced Offset Lempel Ziv transform
// More information about ROLZ at http://ezcodesample.com/rolz/rolz_article.html

const (
	_ROLZ_HASH_SIZE       = 1 << 16
	_ROLZ_MIN_MATCH3      = 3
	_ROLZ_MIN_MATCH4      = 4
	_ROLZ_MIN_MATCH7      = 7
	_ROLZ_MAX_MATCH1      = _ROLZ_MIN_MATCH3 + 65535
	_ROLZ_MAX_MATCH2      = _ROLZ_MIN_MATCH3 + 255
	_ROLZ_LOG_POS_CHECKS1 = 4
	_ROLZ_LOG_POS_CHECKS2 = 5
	_ROLZ_CHUNK_SIZE      = 1 << 26 // 64 MB
	_ROLZ_HASH_MASK       = ^uint32(_ROLZ_CHUNK_SIZE - 1)
	_ROLZ_MATCH_FLAG      = 0
	_ROLZ_LITERAL_FLAG    = 1
	_ROLZ_MATCH_CTX       = 0
	_ROLZ_LITERAL_CTX     = 1
	_ROLZ_HASH_SEED       = 200002979
	_ROLZ_MAX_BLOCK_SIZE  = 1 << 30 // 1 GB
	_ROLZ_MIN_BLOCK_SIZE  = 64
	_ROLZ_PSCALE          = 0xFFFF
	_ROLZ_TOP             = uint64(0x00FFFFFFFFFFFFFF)
	_MASK_0_56            = uint64(0x00FFFFFFFFFFFFFF)
	_MASK_0_32            = uint64(0x00000000FFFFFFFF)
)

func getKey1(p []byte) uint32 {
	return uint32(binary.LittleEndian.Uint16(p))
}

func getKey2(p []byte) uint32 {
	return uint32((binary.LittleEndian.Uint64(p)*_ROLZ_HASH_SEED)>>40) & 0xFFFF
}

func rolzhash(p []byte) uint32 {
	return ((binary.LittleEndian.Uint32(p) << 8) * _ROLZ_HASH_SEED) & _ROLZ_HASH_MASK
}

func emitCopy(buf []byte, dstIdx, ref, matchLen int) int {
	for matchLen >= 8 {
		buf[dstIdx] = buf[ref]
		buf[dstIdx+1] = buf[ref+1]
		buf[dstIdx+2] = buf[ref+2]
		buf[dstIdx+3] = buf[ref+3]
		buf[dstIdx+4] = buf[ref+4]
		buf[dstIdx+5] = buf[ref+5]
		buf[dstIdx+6] = buf[ref+6]
		buf[dstIdx+7] = buf[ref+7]
		dstIdx += 8
		ref += 8
		matchLen -= 8
	}

	for matchLen != 0 {
		buf[dstIdx] = buf[ref]
		dstIdx++
		ref++
		matchLen--
	}

	return dstIdx
}

// ROLZCodec Reduced Offset Lempel Ziv codec
type ROLZCodec struct {
	delegate kanzi.ByteTransform
}

// NewROLZCodec creates a new instance of ROLZCodec providing
// he log of the number of matches to check for during encoding.
func NewROLZCodec(logPosChecks uint) (*ROLZCodec, error) {
	this := &ROLZCodec{}
	d, err := newROLZCodec1(logPosChecks)
	this.delegate = d
	return this, err
}

// NewROLZCodecWithFlag creates a new instance of ROLZCodec
// If the bool parameter is false, encode literals and matches using ANS.
// Otherwise encode literals and matches using CM and check more match
// positions.
func NewROLZCodecWithFlag(extra bool) (*ROLZCodec, error) {
	this := &ROLZCodec{}
	var err error
	var d kanzi.ByteTransform

	if extra {
		d, err = newROLZCodec2(_ROLZ_LOG_POS_CHECKS2)
	} else {
		d, err = newROLZCodec1(_ROLZ_LOG_POS_CHECKS1)
	}

	this.delegate = d
	return this, err
}

// NewROLZCodecWithCtx creates a new instance of ROLZCodec providing a
// context map. If the map contains a transform name set to "ROLZX"
// encode literals and matches using ANS. Otherwise encode literals
// and matches using CM and check more match positions.
func NewROLZCodecWithCtx(ctx *map[string]interface{}) (*ROLZCodec, error) {
	this := &ROLZCodec{}
	var err error
	var d kanzi.ByteTransform

	if val, containsKey := (*ctx)["transform"]; containsKey {
		transform := val.(string)

		if strings.Contains(transform, "ROLZX") {
			d, err = newROLZCodec2WithCtx(_ROLZ_LOG_POS_CHECKS2, ctx)
			this.delegate = d
		}
	}

	if this.delegate == nil && err == nil {
		d, err = newROLZCodec1WithCtx(_ROLZ_LOG_POS_CHECKS1, ctx)
		this.delegate = d
	}

	return this, err
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *ROLZCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if len(src) < _ROLZ_MIN_BLOCK_SIZE {
		return 0, 0, errors.New("ROLZ codec: Block too small, skip")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("ROLZ codec: Input and output buffers cannot be equal")
	}

	if len(src) > _ROLZ_MAX_BLOCK_SIZE {
		// Not a recoverable error: instead of a (silent) failure,
		// issue a fatal error.
		panic(fmt.Errorf("The max ROLZ codec block size is %d, got %d", _ROLZ_MAX_BLOCK_SIZE, len(src)))
	}

	return this.delegate.Forward(src, dst)
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *ROLZCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("ROLZ codec: Input and output buffers cannot be equal")
	}

	if len(src) > _ROLZ_MAX_BLOCK_SIZE {
		// Not a recoverable error: instead of a (silent) failure,
		// issue a fatal error.
		panic(fmt.Errorf("The max ROLZ codec block size is %d, got %d", _ROLZ_MAX_BLOCK_SIZE, len(src)))
	}

	return this.delegate.Inverse(src, dst)
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *ROLZCodec) MaxEncodedLen(srcLen int) int {
	return this.delegate.MaxEncodedLen(srcLen)
}

// Use ANS to encode/decode literals and matches
type rolzCodec1 struct {
	matches      []uint32
	counters     []int32
	logPosChecks uint
	maskChecks   int32
	posChecks    int32
	minMatch     int
	ctx          *map[string]interface{}
}

func newROLZCodec1(logPosChecks uint) (*rolzCodec1, error) {
	this := &rolzCodec1{}

	if (logPosChecks < 2) || (logPosChecks > 8) {
		return nil, fmt.Errorf("ROLZ codec: Invalid logPosChecks parameter: %d (must be in [2..8])", logPosChecks)
	}

	this.logPosChecks = logPosChecks
	this.posChecks = 1 << logPosChecks
	this.maskChecks = this.posChecks - 1
	this.counters = make([]int32, 1<<16)
	this.matches = make([]uint32, _ROLZ_HASH_SIZE<<logPosChecks)
	return this, nil
}

func newROLZCodec1WithCtx(logPosChecks uint, ctx *map[string]interface{}) (*rolzCodec1, error) {
	this := &rolzCodec1{}

	if (logPosChecks < 2) || (logPosChecks > 8) {
		return nil, fmt.Errorf("ROLZ codec: Invalid logPosChecks parameter: %d (must be in [2..8])", logPosChecks)
	}

	this.logPosChecks = logPosChecks
	this.posChecks = 1 << logPosChecks
	this.maskChecks = this.posChecks - 1
	this.counters = make([]int32, 1<<16)
	this.matches = make([]uint32, _ROLZ_HASH_SIZE<<logPosChecks)
	this.ctx = ctx
	return this, nil
}

// findMatch returns match position index (logPosChecks bits) + length (8 bits) or -1
func (this *rolzCodec1) findMatch(buf []byte, pos int, key uint32) (int, int) {
	maxMatch := _ROLZ_MAX_MATCH1

	if maxMatch > len(buf)-pos {
		maxMatch = len(buf) - pos

		if maxMatch < this.minMatch {
			return -1, -1
		}
	}

	m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
	hash32 := rolzhash(buf[pos : pos+4])
	counter := this.counters[key]
	bestLen := 0
	bestIdx := -1
	curBuf := buf[pos:]

	// Check all recorded positions
	for i := counter; i > counter-this.posChecks; i-- {
		ref := m[i&this.maskChecks]

		// Hash check may save a memory access ...
		if ref&_ROLZ_HASH_MASK != hash32 {
			continue
		}

		ref &= ^_ROLZ_HASH_MASK
		refBuf := buf[ref:]

		if refBuf[bestLen] != curBuf[bestLen] {
			continue
		}

		n := 0

		for n < maxMatch-4 {
			if diff := binary.LittleEndian.Uint32(refBuf[n:]) ^ binary.LittleEndian.Uint32(curBuf[n:]); diff != 0 {
				n += (bits.TrailingZeros32(diff) >> 3)
				break
			}

			n += 4
		}

		if n > bestLen {
			bestIdx = int(counter - i)
			bestLen = n

			if bestLen == maxMatch {
				break
			}
		}
	}

	// Register current position
	this.counters[key] = (this.counters[key] + 1) & this.maskChecks
	m[this.counters[key]] = hash32 | uint32(pos)

	if bestLen < this.minMatch {
		return -1, -1
	}

	return bestIdx, bestLen - this.minMatch
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec1) Forward(src, dst []byte) (uint, uint, error) {
	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("ROLZ codec: Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	srcEnd := len(src) - 4
	binary.BigEndian.PutUint32(dst[0:], uint32(len(src)))
	sizeChunk := len(src)

	if sizeChunk > _ROLZ_CHUNK_SIZE {
		sizeChunk = _ROLZ_CHUNK_SIZE
	}

	startChunk := 0
	litBuf := make([]byte, this.MaxEncodedLen(sizeChunk))
	lenBuf := make([]byte, sizeChunk/5)
	mIdxBuf := make([]byte, sizeChunk/4)
	tkBuf := make([]byte, sizeChunk/4)
	var err error

	for i := range this.counters {
		this.counters[i] = 0
	}

	litOrder := uint(1)

	if len(src) < 1<<17 {
		litOrder = 0
	}

	flags := byte(litOrder)
	this.minMatch = _ROLZ_MIN_MATCH3

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt := val.(kanzi.DataType)

			if dt == kanzi.DT_DNA {
				this.minMatch = _ROLZ_MIN_MATCH7
				flags |= 4
			} else if dt == kanzi.DT_MULTIMEDIA {
				this.minMatch = _ROLZ_MIN_MATCH4
				flags |= 2
			}
		}
	}

	dst[4] = flags
	srcIdx := 0
	dstIdx := 5

	// Main loop
	for startChunk < srcEnd {
		litIdx := 0
		lenIdx := 0
		mIdx := 0
		tkIdx := 0

		for i := range this.matches {
			this.matches[i] = 0
		}

		endChunk := startChunk + sizeChunk

		if endChunk >= srcEnd {
			endChunk = srcEnd
			sizeChunk = endChunk - startChunk
		}

		buf := src[startChunk:endChunk]
		srcIdx = 0
		mm := 8

		if startChunk >= srcEnd {
			mm = srcEnd - startChunk
		}

		for j := 0; j < mm; j++ {
			litBuf[litIdx] = buf[srcIdx]
			litIdx++
			srcIdx++
		}

		firstLitIdx := srcIdx

		// Next chunk
		for srcIdx < sizeChunk {
			var matchIdx, matchLen int

			if this.minMatch == _ROLZ_MIN_MATCH3 {
				matchIdx, matchLen = this.findMatch(buf, srcIdx, getKey1(buf[srcIdx-2:]))
			} else {
				matchIdx, matchLen = this.findMatch(buf, srcIdx, getKey2(buf[srcIdx-8:]))
			}

			if matchIdx < 0 {
				srcIdx++
				continue
			}

			// mode LLLLLMMM -> L lit length, M match length
			litLen := srcIdx - firstLitIdx
			var mode byte

			if litLen < 31 {
				mode = byte(litLen << 3)
			} else {
				mode = 0xF8
			}

			if matchLen >= 7 {
				tkBuf[tkIdx] = mode | 0x07
				tkIdx++
				lenIdx += emitLengthROLZ(lenBuf[lenIdx:], matchLen-7)
			} else {
				tkBuf[tkIdx] = mode | byte(matchLen)
				tkIdx++
			}

			// Emit literals
			if litLen > 0 {
				if litLen >= 31 {
					lenIdx += emitLengthROLZ(lenBuf[lenIdx:], litLen-31)
				}

				copy(litBuf[litIdx:], buf[firstLitIdx:firstLitIdx+litLen])
				litIdx += litLen
			}

			// Emit match index
			mIdxBuf[mIdx] = byte(matchIdx)
			mIdx++
			srcIdx += (matchLen + this.minMatch)
			firstLitIdx = srcIdx
		}

		// Emit last chunk literals
		litLen := srcIdx - firstLitIdx

		if litLen < 31 {
			tkBuf[tkIdx] = byte(litLen << 3)
		} else {
			tkBuf[tkIdx] = 0xF8
			lenIdx += emitLengthROLZ(lenBuf[lenIdx:], litLen-31)
		}

		tkIdx++

		// Emit literals
		if litLen > 0 {
			copy(litBuf[litIdx:], buf[firstLitIdx:firstLitIdx+litLen])
			litIdx += litLen
		}

		var os util.BufferStream

		// Scope to deallocate resources early
		{
			// Encode literal, length and match index buffers
			var obs kanzi.OutputBitStream

			if obs, err = bitstream.NewDefaultOutputBitStream(&os, 65536); err != nil {
				break
			}

			obs.WriteBits(uint64(litIdx), 32)
			obs.WriteBits(uint64(tkIdx), 32)
			obs.WriteBits(uint64(lenIdx), 32)
			obs.WriteBits(uint64(mIdx), 32)
			var litEnc *entropy.ANSRangeEncoder

			if litEnc, err = entropy.NewANSRangeEncoder(obs, litOrder); err != nil {
				goto End
			}

			if _, err = litEnc.Write(litBuf[0:litIdx]); err != nil {
				goto End
			}

			litEnc.Dispose()
			var mEnc *entropy.ANSRangeEncoder

			if mEnc, err = entropy.NewANSRangeEncoder(obs, 0); err != nil {
				goto End
			}

			if _, err = mEnc.Write(tkBuf[0:tkIdx]); err != nil {
				goto End
			}

			if _, err = mEnc.Write(lenBuf[0:lenIdx]); err != nil {
				goto End
			}

			if _, err = mEnc.Write(mIdxBuf[0:mIdx]); err != nil {
				goto End
			}

			mEnc.Dispose()
			obs.Close()
		}

		// Copy bitstream array to output
		bufSize := os.Len()

		if dstIdx+bufSize > len(dst) {
			err = errors.New("ROLZ codec: Destination buffer too small")
			break
		}

		if _, err = os.Read(dst[dstIdx : dstIdx+bufSize]); err != nil {
			break
		}

		dstIdx += bufSize
		startChunk = endChunk
	}

End:
	if err == nil {
		if dstIdx+4 > len(dst) {
			err = errors.New("ROLZ codec: Destination buffer too small")
		} else {
			// Emit last literals
			srcIdx += (startChunk - sizeChunk)
			dst[dstIdx] = src[srcIdx]
			dst[dstIdx+1] = src[srcIdx+1]
			dst[dstIdx+2] = src[srcIdx+2]
			dst[dstIdx+3] = src[srcIdx+3]
			srcIdx += 4
			dstIdx += 4

			if srcIdx != len(src) {
				err = errors.New("ROLZ codec: Destination buffer too small")
			} else if dstIdx >= len(src) {
				err = errors.New("ROLZ codec: No compression")
			}
		}
	}

	return uint(srcIdx), uint(dstIdx), err
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec1) Inverse(src, dst []byte) (uint, uint, error) {
	sizeChunk := len(dst)

	if sizeChunk > _ROLZ_CHUNK_SIZE {
		sizeChunk = _ROLZ_CHUNK_SIZE
	}

	startChunk := 0
	var is util.BufferStream
	dstEnd := int(binary.BigEndian.Uint32(src[0:])) - 4

	if _, err := is.Write(src[4:]); err != nil {
		return 0, 0, err
	}

	srcIdx := 5
	dstIdx := 0
	litBuf := make([]byte, this.MaxEncodedLen(sizeChunk))
	lenBuf := make([]byte, sizeChunk/5)
	mIdxBuf := make([]byte, sizeChunk/4)
	tkBuf := make([]byte, sizeChunk/4)
	var err error

	for i := range this.counters {
		this.counters[i] = 0
	}

	litOrder := uint(src[4] & 1)
	this.minMatch = _ROLZ_MIN_MATCH3
	bsVersion := uint(3)

	if val, containsKey := (*this.ctx)["bsVersion"]; containsKey {
		bsVersion = val.(uint)
	}

	if bsVersion >= 3 {
		if src[4]&6 == 2 {
			this.minMatch = _ROLZ_MIN_MATCH4
		} else if src[4]&6 == 4 {
			this.minMatch = _ROLZ_MIN_MATCH7
		}
	}

	// Main loop
	for startChunk < dstEnd {
		mIdx := 0
		lenIdx := 0
		litIdx := 0
		tkIdx := 0

		for i := range this.matches {
			this.matches[i] = 0
		}

		endChunk := startChunk + sizeChunk

		if endChunk > dstEnd {
			endChunk = dstEnd
		}

		sizeChunk = endChunk - startChunk
		buf := dst[startChunk:endChunk]

		// Scope to deallocate resources early
		{
			// Decode literal, match length and match index buffers
			var ibs kanzi.InputBitStream

			if err = is.SetOffset(srcIdx - 4); err != nil {
				goto End
			}

			if ibs, err = bitstream.NewDefaultInputBitStream(&is, 65536); err != nil {
				goto End
			}

			litLen := int(ibs.ReadBits(32))
			tkLen := int(ibs.ReadBits(32))
			mLenLen := int(ibs.ReadBits(32))
			mIdxLen := int(ibs.ReadBits(32))

			if litLen > sizeChunk {
				err = fmt.Errorf("ROLZ codec: Invalid length: got %d, must be less than or equal to %v", litLen, sizeChunk)
				goto End
			}

			if tkLen > sizeChunk {
				err = fmt.Errorf("ROLZ codec: Invalid length: got %d, must be less than or equal to %v", tkLen, sizeChunk)
				goto End
			}

			if mLenLen > sizeChunk {
				err = fmt.Errorf("ROLZ codec: Invalid length: got %d, must be less than or equal to %v", mLenLen, sizeChunk)
				goto End
			}

			if mIdxLen > sizeChunk {
				err = fmt.Errorf("ROLZ codec: Invalid length: got %d, must be less than or equal to %v", mIdxLen, sizeChunk)
				goto End
			}

			var litDec *entropy.ANSRangeDecoder

			if litDec, err = entropy.NewANSRangeDecoderWithCtx(ibs, litOrder, this.ctx); err != nil {
				goto End
			}

			if _, err = litDec.Read(litBuf[0:litLen]); err != nil {
				goto End
			}

			litDec.Dispose()
			var mDec *entropy.ANSRangeDecoder

			if mDec, err = entropy.NewANSRangeDecoderWithCtx(ibs, 0, this.ctx); err != nil {
				goto End
			}

			if _, err = mDec.Read(tkBuf[0:tkLen]); err != nil {
				goto End
			}

			if _, err = mDec.Read(lenBuf[0:mLenLen]); err != nil {
				goto End
			}

			if _, err = mDec.Read(mIdxBuf[0:mIdxLen]); err != nil {
				goto End
			}

			mDec.Dispose()
			srcIdx += int((ibs.Read() + 7) >> 3)
			ibs.Close()
		}

		dstIdx = 0
		mm := 8

		if startChunk >= dstEnd {
			mm = dstEnd - startChunk
		}

		for j := 0; j < mm; j++ {
			buf[dstIdx] = litBuf[litIdx]
			dstIdx++
			litIdx++
		}

		// Next chunk
		for dstIdx < sizeChunk {
			// mode LLLLLMMM -> L lit length, M match length
			mode := tkBuf[tkIdx]
			tkIdx++
			matchLen := int(mode & 0x07)

			if matchLen == 7 {
				ml, deltaIdx := readLengthROLZ(lenBuf[lenIdx:])
				lenIdx += deltaIdx
				matchLen = ml + 7
			}

			var litLen int

			if mode < 0xF8 {
				litLen = int(mode >> 3)
			} else {
				ll, deltaIdx := readLengthROLZ(lenBuf[lenIdx:])
				lenIdx += deltaIdx
				litLen = ll + 31
			}

			if litLen > 0 {
				lb := litBuf[litIdx : litIdx+litLen]

				if this.minMatch == _ROLZ_MIN_MATCH3 {
					d := buf[dstIdx-2:]
					copy(d[2:], lb)

					for n := range lb {
						key := getKey1(d[n:])
						m := this.matches[key<<this.logPosChecks:]
						this.counters[key] = (this.counters[key] + 1) & this.maskChecks
						m[this.counters[key]] = uint32(dstIdx + n)
					}
				} else {
					d := buf[dstIdx-8:]
					copy(d[8:], lb)

					for n := range lb {
						key := getKey2(d[n:])
						m := this.matches[key<<this.logPosChecks:]
						this.counters[key] = (this.counters[key] + 1) & this.maskChecks
						m[this.counters[key]] = uint32(dstIdx + n)
					}
				}

				litIdx += litLen
				dstIdx += litLen

				if dstIdx >= sizeChunk {
					// Last chunk literals not followed by match
					if dstIdx == sizeChunk {
						break
					}

					err = errors.New("ROLZ codec: Invalid input data")
					goto End
				}
			}

			// Sanity check
			if dstIdx+matchLen+this.minMatch > dstEnd {
				err = errors.New("ROLZ codec: Invalid input data")
				goto End
			}

			matchIdx := int32(mIdxBuf[mIdx] & 0xFF)
			mIdx++
			var key uint32

			if this.minMatch == _ROLZ_MIN_MATCH3 {
				key = getKey1(buf[dstIdx-2:])
			} else {
				key = getKey2(buf[dstIdx-8:])
			}

			m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
			ref := int(m[(this.counters[key]-matchIdx)&this.maskChecks])
			savedIdx := uint32(dstIdx)
			dstIdx = emitCopy(buf, dstIdx, ref, matchLen+this.minMatch)
			this.counters[key] = (this.counters[key] + 1) & this.maskChecks
			m[this.counters[key]] = savedIdx
		}

		startChunk = endChunk
	}

End:
	if err == nil {
		// Emit last literals
		dstIdx += (startChunk - sizeChunk)
		dst[dstIdx] = src[srcIdx]
		dst[dstIdx+1] = src[srcIdx+1]
		dst[dstIdx+2] = src[srcIdx+2]
		dst[dstIdx+3] = src[srcIdx+3]
		srcIdx += 4
		dstIdx += 4

		if srcIdx != len(src) {
			err = errors.New("ROLZ codec: Invalid input data")
		}
	}

	return uint(srcIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this rolzCodec1) MaxEncodedLen(srcLen int) int {
	if srcLen <= 512 {
		return srcLen + 64
	}

	return srcLen
}

func emitLengthROLZ(block []byte, litLen int) int {
	idx := 0

	if litLen >= 1<<7 {
		if litLen >= 1<<14 {
			if litLen >= 1<<21 {
				block[idx] = byte(0x80 | (litLen >> 21))
				idx++
			}

			block[idx] = byte(0x80 | (litLen >> 14))
			idx++
		}

		block[idx] = byte(0x80 | (litLen >> 7))
		idx++
	}

	block[idx] = byte(litLen & 0x7F)
	return idx + 1
}

// return litLen, idx
func readLengthROLZ(lenBuf []byte) (int, int) {
	idx := 0
	next := lenBuf[idx]
	idx++
	litLen := int(next & 0x7F)

	if next >= 128 {
		next = lenBuf[idx]
		idx++
		litLen = (litLen << 7) | int(next&0x7F)

		if next >= 128 {
			next = lenBuf[idx]
			idx++
			litLen = (litLen << 7) | int(next&0x7F)

			if next >= 128 {
				next = lenBuf[idx]
				idx++
				litLen = (litLen << 7) | int(next&0x7F)
			}
		}
	}

	return litLen, idx
}

// Use CM (ROLZEncoder/ROLZDecoder) to encode/decode literals and matches
// Code loosely based on 'balz' by Ilya Muravyov
type rolzCodec2 struct {
	matches      []uint32
	counters     []int32
	logPosChecks uint
	maskChecks   int32
	posChecks    int32
	minMatch     int
	ctx          *map[string]interface{}
}

func newROLZCodec2(logPosChecks uint) (*rolzCodec2, error) {
	this := &rolzCodec2{}

	if (logPosChecks < 2) || (logPosChecks > 8) {
		return nil, fmt.Errorf("ROLZX codec: Invalid logPosChecks parameter: %v (must be in [2..8])", logPosChecks)
	}

	this.logPosChecks = logPosChecks
	this.posChecks = 1 << logPosChecks
	this.maskChecks = this.posChecks - 1
	this.counters = make([]int32, 1<<16)
	this.matches = make([]uint32, _ROLZ_HASH_SIZE<<logPosChecks)
	return this, nil
}

func newROLZCodec2WithCtx(logPosChecks uint, ctx *map[string]interface{}) (*rolzCodec2, error) {
	this := &rolzCodec2{}

	if (logPosChecks < 2) || (logPosChecks > 8) {
		return nil, fmt.Errorf("ROLZX codec: Invalid logPosChecks parameter: %d (must be in [2..8])", logPosChecks)
	}

	this.logPosChecks = logPosChecks
	this.posChecks = 1 << logPosChecks
	this.maskChecks = this.posChecks - 1
	this.counters = make([]int32, 1<<16)
	this.matches = make([]uint32, _ROLZ_HASH_SIZE<<logPosChecks)
	this.ctx = ctx
	return this, nil
}

// findMatch returns match position index and length or -1
func (this *rolzCodec2) findMatch(buf []byte, pos int, key uint32) (int, int) {
	maxMatch := _ROLZ_MAX_MATCH2

	if maxMatch > len(buf)-pos {
		maxMatch = len(buf) - pos

		if maxMatch < this.minMatch {
			return -1, -1
		}
	}

	m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
	hash32 := rolzhash(buf[pos : pos+4])
	counter := this.counters[key]
	bestLen := 0
	bestIdx := -1
	curBuf := buf[pos:]

	// Check all recorded positions
	for i := counter; i > counter-this.posChecks; i-- {
		ref := m[i&this.maskChecks]

		// Hash check may save a memory access ...
		if ref&_ROLZ_HASH_MASK != hash32 {
			continue
		}

		ref &= ^_ROLZ_HASH_MASK
		refBuf := buf[ref:]

		if refBuf[bestLen] != curBuf[bestLen] {
			continue
		}

		n := 0

		for n+4 < maxMatch {
			if diff := binary.LittleEndian.Uint32(refBuf[n:]) ^ binary.LittleEndian.Uint32(curBuf[n:]); diff != 0 {
				n += (bits.TrailingZeros32(diff) >> 3)
				break
			}

			n += 4
		}

		if n > bestLen {
			bestIdx = int(counter - i)
			bestLen = n

			if bestLen == maxMatch {
				break
			}
		}
	}

	// Register current position
	this.counters[key] = (this.counters[key] + 1) & this.maskChecks
	m[this.counters[key]] = hash32 | uint32(pos)

	if bestLen < this.minMatch {
		return -1, -1
	}

	return bestIdx, bestLen - this.minMatch
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec2) Forward(src, dst []byte) (uint, uint, error) {
	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("ROLZX codec: Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	srcEnd := len(src) - 4
	sizeChunk := len(src)

	if sizeChunk > _ROLZ_CHUNK_SIZE {
		sizeChunk = _ROLZ_CHUNK_SIZE
	}

	srcIdx := 0
	dstIdx := 5
	startChunk := 0
	binary.BigEndian.PutUint32(dst[0:], uint32(len(src)))
	re, _ := newRolzEncoder(9, this.logPosChecks, dst, &dstIdx)

	for i := range this.counters {
		this.counters[i] = 0
	}

	this.minMatch = _ROLZ_MIN_MATCH3
	flags := byte(0)

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt := val.(kanzi.DataType)

			if dt == kanzi.DT_DNA {
				this.minMatch = _ROLZ_MIN_MATCH7
				flags = 1
			}
		}
	}

	dst[4] = flags

	// Main loop
	for startChunk < srcEnd {
		for i := range this.matches {
			this.matches[i] = 0
		}

		endChunk := startChunk + sizeChunk

		if endChunk >= srcEnd {
			endChunk = srcEnd
		}

		sizeChunk = endChunk - startChunk
		re.reset()
		buf := src[startChunk:endChunk]
		srcIdx = 0

		// First literals
		mm := 8
		re.setContext(_ROLZ_LITERAL_CTX, 0)

		if startChunk >= srcEnd {
			mm = srcEnd - startChunk
		}

		for j := 0; j < mm; j++ {
			re.encode9Bits((_ROLZ_LITERAL_FLAG << 8) | int(buf[srcIdx]))
			srcIdx++
		}

		// Next chunk
		for srcIdx < sizeChunk {
			re.setContext(_ROLZ_LITERAL_CTX, buf[srcIdx-1])
			var matchIdx, matchLen int

			if this.minMatch == _ROLZ_MIN_MATCH3 {
				matchIdx, matchLen = this.findMatch(buf, srcIdx, getKey1(buf[srcIdx-2:]))
			} else {
				matchIdx, matchLen = this.findMatch(buf, srcIdx, getKey2(buf[srcIdx-8:]))
			}

			if matchIdx < 0 {
				// Emit one literal
				re.encode9Bits((_ROLZ_LITERAL_FLAG << 8) | int(buf[srcIdx]))
				srcIdx++
				continue
			}

			// Emit one match length and index
			re.encode9Bits((_ROLZ_MATCH_FLAG << 8) | int(matchLen))
			re.setContext(_ROLZ_MATCH_CTX, buf[srcIdx-1])
			re.encodeBits(matchIdx, this.logPosChecks)
			srcIdx += (matchLen + this.minMatch)
		}

		startChunk = endChunk
	}

	// Emit last literals
	srcIdx += (startChunk - sizeChunk)

	for i := 0; i < 4; i++ {
		re.setContext(_ROLZ_LITERAL_CTX, src[srcIdx-1])
		re.encode9Bits((_ROLZ_LITERAL_FLAG << 8) | int(src[srcIdx]))
		srcIdx++
	}

	re.dispose()
	var err error

	if srcIdx != len(src) {
		err = errors.New("ROLZX codec: Destination buffer too small")
	} else if dstIdx >= len(src) {
		err = errors.New("ROLZX codec: No compression")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec2) Inverse(src, dst []byte) (uint, uint, error) {
	dstEnd := int(binary.BigEndian.Uint32(src[0:]))
	sizeChunk := len(dst)

	if sizeChunk > _ROLZ_CHUNK_SIZE {
		sizeChunk = _ROLZ_CHUNK_SIZE
	}

	this.minMatch = _ROLZ_MIN_MATCH3
	bsVersion := uint(3)

	if val, containsKey := (*this.ctx)["bsVersion"]; containsKey {
		bsVersion = val.(uint)
	}

	if bsVersion >= 3 && src[4] == 1 {
		this.minMatch = _ROLZ_MIN_MATCH7
	}

	srcIdx := 5
	dstIdx := 0
	startChunk := 0
	rd, _ := newRolzDecoder(9, this.logPosChecks, src, &srcIdx)

	for i := range this.counters {
		this.counters[i] = 0
	}

	// Main loop
	for startChunk < dstEnd {
		for i := range this.matches {
			this.matches[i] = 0
		}

		endChunk := startChunk + sizeChunk

		if endChunk > dstEnd {
			endChunk = dstEnd
			sizeChunk = endChunk - startChunk
		}

		buf := dst[startChunk:endChunk]
		rd.reset()
		dstIdx = 0

		// First literals
		mm := 8
		rd.setContext(_ROLZ_LITERAL_CTX, 0)

		if startChunk >= dstEnd {
			mm = dstEnd - startChunk
		}

		for j := 0; j < mm; j++ {
			val := rd.decode9Bits()

			// Sanity check
			if val>>8 == _ROLZ_MATCH_FLAG {
				dstIdx += startChunk
				break
			}

			buf[dstIdx] = byte(val)
			dstIdx++
		}

		// Next chunk
		for dstIdx < sizeChunk {
			savedIdx := dstIdx
			var key uint32

			if this.minMatch == _ROLZ_MIN_MATCH3 {
				key = getKey1(buf[dstIdx-2:])
			} else {
				key = getKey2(buf[dstIdx-8:])
			}

			m := this.matches[key<<this.logPosChecks:]
			rd.setContext(_ROLZ_LITERAL_CTX, buf[dstIdx-1])
			val := rd.decode9Bits()

			if val>>8 == _ROLZ_LITERAL_FLAG {
				buf[dstIdx] = byte(val)
				dstIdx++
			} else {
				// Read one match length and index
				matchLen := val & 0xFF

				// Sanity check
				if matchLen+3 > dstEnd {
					dstIdx += startChunk
					break
				}

				rd.setContext(_ROLZ_MATCH_CTX, buf[dstIdx-1])
				matchIdx := int32(rd.decodeBits(this.logPosChecks))
				ref := int(m[(this.counters[key]-matchIdx)&this.maskChecks])
				dstIdx = emitCopy(buf, dstIdx, ref, matchLen+this.minMatch)
			}

			// Update map
			this.counters[key] = (this.counters[key] + 1) & this.maskChecks
			m[this.counters[key]] = uint32(savedIdx)
		}

		startChunk = endChunk
	}

	rd.dispose()
	var err error
	dstIdx += (startChunk - sizeChunk)

	if srcIdx != len(src) {
		err = errors.New("ROLZX codec: Invalid input data")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this rolzCodec2) MaxEncodedLen(srcLen int) int {
	// Since we do not check the dst index for each byte (for speed purpose)
	// allocate some extra buffer for incompressible data.
	if srcLen <= 16384 {
		return srcLen + 1024
	}

	return srcLen + srcLen/32
}

type rolzEncoder struct {
	buf     []byte
	idx     *int
	low     uint64
	high    uint64
	probs   [2][]int
	logSize [2]uint
	c1      int
	pIdx    int
	ctx     int
	p       []int
}

func newRolzEncoder(litLogSize, mLogSize uint, buf []byte, idx *int) (*rolzEncoder, error) {
	this := &rolzEncoder{}
	this.low = 0
	this.high = _ROLZ_TOP
	this.buf = buf
	this.idx = idx
	this.pIdx = _ROLZ_LITERAL_CTX
	this.c1 = 1
	this.ctx = 0
	this.logSize[_ROLZ_MATCH_CTX] = mLogSize
	this.probs[_ROLZ_MATCH_CTX] = make([]int, 256<<mLogSize)
	this.logSize[_ROLZ_LITERAL_CTX] = litLogSize
	this.probs[_ROLZ_LITERAL_CTX] = make([]int, 256<<litLogSize)
	this.reset()
	return this, nil
}

func (this *rolzEncoder) reset() {
	for i := range this.probs[_ROLZ_MATCH_CTX] {
		this.probs[_ROLZ_MATCH_CTX][i] = _ROLZ_PSCALE >> 1
	}

	for i := range this.probs[_ROLZ_LITERAL_CTX] {
		this.probs[_ROLZ_LITERAL_CTX][i] = _ROLZ_PSCALE >> 1
	}
}

func (this *rolzEncoder) setContext(n int, ctx byte) {
	this.pIdx = n
	this.ctx = int(ctx) << this.logSize[this.pIdx]
}

func (this *rolzEncoder) encodeBits(val int, n uint) {
	this.c1 = 1
	this.p = this.probs[this.pIdx][this.ctx:]

	for n != 0 {
		n--
		this.encodeBit(val & (1 << n))
	}
}

func (this *rolzEncoder) encode9Bits(val int) {
	this.c1 = 1
	this.p = this.probs[this.pIdx][this.ctx:]
	this.encodeBit(val & 0x100)
	this.encodeBit(val & 0x80)
	this.encodeBit(val & 0x40)
	this.encodeBit(val & 0x20)
	this.encodeBit(val & 0x10)
	this.encodeBit(val & 0x08)
	this.encodeBit(val & 0x04)
	this.encodeBit(val & 0x02)
	this.encodeBit(val & 0x01)
}

func (this *rolzEncoder) encodeBit(bit int) {
	// Calculate interval split
	split := (((this.high - this.low) >> 4) * uint64(this.p[this.c1]>>4)) >> 8

	// Update fields with new interval bounds
	if bit == 0 {
		this.low += (split + 1)
		this.p[this.c1] -= (this.p[this.c1] >> 5)
		this.c1 += this.c1
	} else {
		this.high = this.low + split
		this.p[this.c1] -= ((this.p[this.c1] - _ROLZ_PSCALE + 32) >> 5)
		this.c1 += (this.c1 + 1)
	}

	// Write unchanged first 32 bits to bitstream
	for (this.low^this.high)>>24 == 0 {
		binary.BigEndian.PutUint32(this.buf[*this.idx:*this.idx+4], uint32(this.high>>32))
		*this.idx += 4
		this.low <<= 32
		this.high = (this.high << 32) | _MASK_0_32
	}
}

func (this *rolzEncoder) dispose() {
	for i := 0; i < 8; i++ {
		this.buf[*this.idx+i] = byte(this.low >> 56)
		this.low <<= 8
	}

	*this.idx += 8
}

type rolzDecoder struct {
	buf     []byte
	idx     *int
	low     uint64
	high    uint64
	current uint64
	probs   [2][]int
	logSize [2]uint
	c1      int
	pIdx    int
	ctx     int
	p       []int
}

func newRolzDecoder(litLogSize, mLogSize uint, buf []byte, idx *int) (*rolzDecoder, error) {
	this := &rolzDecoder{}
	this.low = 0
	this.high = _ROLZ_TOP
	this.buf = buf
	this.idx = idx
	this.current = uint64(0)

	for i := 0; i < 8; i++ {
		this.current = (this.current << 8) | (uint64(this.buf[*this.idx+i]) & 0xFF)
	}

	*this.idx += 8
	this.pIdx = _ROLZ_LITERAL_CTX
	this.c1 = 1
	this.ctx = 0
	this.logSize[_ROLZ_MATCH_CTX] = mLogSize
	this.probs[_ROLZ_MATCH_CTX] = make([]int, 256<<mLogSize)
	this.logSize[_ROLZ_LITERAL_CTX] = litLogSize
	this.probs[_ROLZ_LITERAL_CTX] = make([]int, 256<<litLogSize)
	this.reset()
	return this, nil
}

func (this *rolzDecoder) reset() {
	for i := range this.probs[_ROLZ_MATCH_CTX] {
		this.probs[_ROLZ_MATCH_CTX][i] = _ROLZ_PSCALE >> 1
	}

	for i := range this.probs[_ROLZ_LITERAL_CTX] {
		this.probs[_ROLZ_LITERAL_CTX][i] = _ROLZ_PSCALE >> 1
	}
}

func (this *rolzDecoder) setContext(n int, ctx byte) {
	this.pIdx = n
	this.ctx = int(ctx) << this.logSize[this.pIdx]
}

func (this *rolzDecoder) decodeBits(n uint) int {
	this.c1 = 1
	mask := (1 << n) - 1
	this.p = this.probs[this.pIdx][this.ctx:]

	for n != 0 {
		this.decodeBit()
		n--
	}

	return this.c1 & mask
}

func (this *rolzDecoder) decode9Bits() int {
	this.c1 = 1
	this.p = this.probs[this.pIdx][this.ctx:]
	this.decodeBit()
	this.decodeBit()
	this.decodeBit()
	this.decodeBit()
	this.decodeBit()
	this.decodeBit()
	this.decodeBit()
	this.decodeBit()
	this.decodeBit()
	return this.c1 & 0x1FF
}

func (this *rolzDecoder) decodeBit() int {
	// Calculate interval split
	mid := this.low + ((((this.high - this.low) >> 4) * uint64(this.p[this.c1]>>4)) >> 8)
	var bit int

	// Update bounds and predictor
	if mid >= this.current {
		bit = 1
		this.high = mid
		this.p[this.c1] -= ((this.p[this.c1] - _ROLZ_PSCALE + 32) >> 5)
		this.c1 += (this.c1 + 1)
	} else {
		bit = 0
		this.low = mid + 1
		this.p[this.c1] -= (this.p[this.c1] >> 5)
		this.c1 += this.c1
	}

	// Read 32 bits from bitstream
	for (this.low^this.high)>>24 == 0 {
		this.low = (this.low << 32) & _MASK_0_56
		this.high = ((this.high << 32) | _MASK_0_32) & _MASK_0_56
		val := uint64(binary.BigEndian.Uint32(this.buf[*this.idx : *this.idx+4]))
		this.current = ((this.current << 32) | val) & _MASK_0_56
		*this.idx += 4
	}

	return bit
}

func (this *rolzDecoder) dispose() {
}
