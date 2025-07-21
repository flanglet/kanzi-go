/*
Copyright 2011-2025 Frederic Langlet
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

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/bitstream"
	"github.com/flanglet/kanzi-go/v2/entropy"
	"github.com/flanglet/kanzi-go/v2/internal"
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
	_ROLZ_CHUNK_SIZE      = 16 * 1024 * 1024
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
	if dstIdx >= ref+matchLen {
		copy(buf[dstIdx:], buf[ref:ref+matchLen])
		return dstIdx + matchLen
	}

	// Handle overlapping segments
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
func NewROLZCodecWithCtx(ctx *map[string]any) (*ROLZCodec, error) {
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
		return 0, 0, errors.New("ROLZ codec forward transform skip: block too small")
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if len(src) > _ROLZ_MAX_BLOCK_SIZE {
		return 0, 0, fmt.Errorf("The max ROLZ codec block size is %d, got %d", _ROLZ_MAX_BLOCK_SIZE, len(src))
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
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	if len(src) > _ROLZ_MAX_BLOCK_SIZE {
		return 0, 0, fmt.Errorf("The max ROLZ codec block size is %d, got %d", _ROLZ_MAX_BLOCK_SIZE, len(src))
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
	ctx          *map[string]any
}

func newROLZCodec1(logPosChecks uint) (*rolzCodec1, error) {
	this := &rolzCodec1{}

	if (logPosChecks < 2) || (logPosChecks > 8) {
		return nil, fmt.Errorf("ROLZ codec forward transform failed: Invalid logPosChecks parameter: %d (must be in [2..8])", logPosChecks)
	}

	this.logPosChecks = logPosChecks
	this.posChecks = 1 << logPosChecks
	this.maskChecks = this.posChecks - 1
	this.counters = make([]int32, 1<<16)
	this.matches = make([]uint32, 0)
	return this, nil
}

func newROLZCodec1WithCtx(logPosChecks uint, ctx *map[string]any) (*rolzCodec1, error) {
	this := &rolzCodec1{}

	if (logPosChecks < 2) || (logPosChecks > 8) {
		return nil, fmt.Errorf("ROLZ codec: Invalid logPosChecks parameter: %d (must be in [2..8])", logPosChecks)
	}

	this.logPosChecks = logPosChecks
	this.posChecks = 1 << logPosChecks
	this.maskChecks = this.posChecks - 1
	this.counters = make([]int32, 1<<16)
	this.matches = make([]uint32, 0)
	this.ctx = ctx
	return this, nil
}

// findMatch returns match position index (logPosChecks bits) + length (8 bits) or -1
func (this *rolzCodec1) findMatch(buf []byte, pos int, hash32 uint32, counter int32, matches []uint32) (int, int) {
	maxMatch := min(_ROLZ_MAX_MATCH1, len(buf)-pos)

	if maxMatch < this.minMatch {
		return -1, -1
	}

	maxMatch -= 4
	bestLen := 0
	bestIdx := -1
	curBuf := buf[pos:]

	// Check all recorded positions
	for i := counter; i > counter-this.posChecks; i-- {
		ref := matches[i&this.maskChecks]

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

		for n < maxMatch {
			if diff := binary.LittleEndian.Uint32(refBuf[n:]) ^ binary.LittleEndian.Uint32(curBuf[n:]); diff != 0 {
				n += (bits.TrailingZeros32(diff) >> 3)
				break
			}

			n += 4
		}

		if n > bestLen {
			bestIdx = int(i)
			bestLen = n
		}
	}

	if bestLen < this.minMatch {
		return -1, -1
	}

	return int(counter) - bestIdx, bestLen - this.minMatch
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec1) Forward(src, dst []byte) (uint, uint, error) {
	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("ROLZ codec forward transform failed: output buffer is too small - size: %d, required %d", len(dst), n)
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
	delta := 2

	if this.ctx != nil {
		dt := internal.DT_UNDEFINED

		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt = val.(internal.DataType)
		}

		if dt == internal.DT_UNDEFINED {
			var freqs0 [256]int
			internal.ComputeHistogram(src, freqs0[:], true, false)
			dt = internal.DetectSimpleType(len(src), freqs0[:])

			if dt != internal.DT_UNDEFINED {
				(*this.ctx)["dataType"] = dt
			}
		}

		if dt == internal.DT_EXE {
			delta = 3
			flags |= 8
		} else if dt == internal.DT_DNA {
			delta = 8
			this.minMatch = _ROLZ_MIN_MATCH7
			flags |= 4
		} else if dt == internal.DT_MULTIMEDIA {
			delta = 8
			this.minMatch = _ROLZ_MIN_MATCH4
			flags |= 2
		}
	}

	flags |= byte(this.logPosChecks << 4)
	dst[4] = flags
	srcIdx := 0
	dstIdx := 5

	if len(this.matches) == 0 {
		this.matches = make([]uint32, _ROLZ_HASH_SIZE<<this.logPosChecks)
	}

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
		n := min(srcEnd-startChunk, 8)

		for j := 0; j < n; j++ {
			litBuf[litIdx] = buf[srcIdx]
			litIdx++
			srcIdx++
		}

		firstLitIdx := srcIdx
		srcInc := 0

		// Next chunk
		for srcIdx < sizeChunk {
			var key uint32

			if this.minMatch == _ROLZ_MIN_MATCH3 {
				key = getKey1(buf[srcIdx-delta:])
			} else {
				key = getKey2(buf[srcIdx-delta:])
			}

			m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
			hash32 := rolzhash(buf[srcIdx : srcIdx+4])
			matchIdx, matchLen := this.findMatch(buf, srcIdx, hash32, this.counters[key], m)

			// Register current position
			this.counters[key] = (this.counters[key] + 1) & this.maskChecks
			m[this.counters[key]] = hash32 | uint32(srcIdx)

			if matchIdx < 0 {
				srcIdx++
				srcIdx += (srcInc >> 6)
				srcInc++
				continue
			}

			// Check if better match at next position
			srcIdx1 := srcIdx + 1

			if this.minMatch == _ROLZ_MIN_MATCH3 {
				key = getKey1(buf[srcIdx1-delta:])
			} else {
				key = getKey2(buf[srcIdx1-delta:])
			}

			m = this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
			hash32 = rolzhash(buf[srcIdx1 : srcIdx1+4])
			matchIdx1, matchLen1 := this.findMatch(buf, srcIdx1, hash32, this.counters[key], m)

			if (matchIdx1 >= 0) && (matchLen1 > matchLen) {
				// New match is better
				matchIdx = matchIdx1
				matchLen = matchLen1
				srcIdx = srcIdx1

				// Register current position
				this.counters[key] = (this.counters[key] + 1) & this.maskChecks
				m[this.counters[key]] = hash32 | uint32(srcIdx)
			}

			// token LLLLLMMM -> L lit length, M match length
			litLen := srcIdx - firstLitIdx
			var token byte

			if matchLen >= 7 {
				token = 7
				lenIdx += emitLengthROLZ(lenBuf[lenIdx:], matchLen-7)
			} else {
				token = byte(matchLen)
			}

			// Emit literals
			if litLen > 0 {
				if litLen >= 31 {
					token |= 0xF8
					lenIdx += emitLengthROLZ(lenBuf[lenIdx:], litLen-31)
				} else {
					token |= byte(litLen << 3)
				}

				copy(litBuf[litIdx:], buf[firstLitIdx:firstLitIdx+litLen])
				litIdx += litLen
			}

			tkBuf[tkIdx] = token
			tkIdx++

			// Emit match index
			mIdxBuf[mIdx] = byte(matchIdx)
			mIdx++
			srcIdx += (matchLen + this.minMatch)
			firstLitIdx = srcIdx
			srcInc = 0
		}

		// Emit last chunk literals
		srcIdx = sizeChunk
		litLen := srcIdx - firstLitIdx

		if tkIdx != 0 {
			// At least one match to emit
			if litLen >= 31 {
				tkBuf[tkIdx] = 0xF8
			} else {
				tkBuf[tkIdx] = byte(litLen << 3)
			}

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

		os := internal.NewBufferStream(make([]byte, 0, sizeChunk/4))

		// Scope to deallocate resources early
		{
			// Encode literal, length and match index buffers
			var obs kanzi.OutputBitStream

			if obs, err = bitstream.NewDefaultOutputBitStream(os, 65536); err != nil {
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

			if mEnc, err = entropy.NewANSRangeEncoder(obs, 0, 32768); err != nil {
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
			err = errors.New("ROLZ codec forward transform skip: destination buffer too small")
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
			err = errors.New("ROLZ codec forward transform skip: destination buffer too small")
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
				err = errors.New("ROLZ codec forward transform skip: destination buffer too small")
			} else if dstIdx >= len(src) {
				err = errors.New("ROLZ codec forward transform skip: no compression")
			}
		}
	}

	return uint(srcIdx), uint(dstIdx), err
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec1) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) < 5 {
		return 0, 0, errors.New("ROLZ codec inverse transform failed: invalid input data (input array too small)")
	}

	dstEnd := int(binary.BigEndian.Uint32(src[0:])) - 4

	if dstEnd <= 0 || dstEnd > len(dst) {
		return 0, 0, errors.New("ROLZ codec inverse transform failed: invalid input data")
	}

	startChunk := 0
	srcIdx := 5
	dstIdx := 0
	sizeChunk := min(len(dst), _ROLZ_CHUNK_SIZE)
	litBuf := make([]byte, sizeChunk)
	mLenBuf := make([]byte, sizeChunk/5)
	mIdxBuf := make([]byte, sizeChunk/4)
	tkBuf := make([]byte, sizeChunk/4)
	var err error

	for i := range this.counters {
		this.counters[i] = 0
	}

	flags := src[4]
	litOrder := uint(flags & 1)
	delta := 2
	this.minMatch = _ROLZ_MIN_MATCH3
	bsVersion := uint(6)

	if len(this.matches) < int(this.logPosChecks) {
		this.matches = make([]uint32, _ROLZ_HASH_SIZE<<this.logPosChecks)
	}
	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["bsVersion"]; containsKey {
			bsVersion = val.(uint)
		}
	}

	if bsVersion >= 4 {
		if flags&0x0E == 2 {
			this.minMatch = _ROLZ_MIN_MATCH4
			delta = 8
		} else if flags&0x0E == 4 {
			this.minMatch = _ROLZ_MIN_MATCH7
			delta = 8
		} else if flags&0x0E == 8 {
			delta = 3
		}
	} else if bsVersion >= 3 {
		if flags&6 == 2 {
			this.minMatch = _ROLZ_MIN_MATCH4
		} else if flags&6 == 4 {
			this.minMatch = _ROLZ_MIN_MATCH7
		}
	}

	this.logPosChecks = uint(flags >> 4)

	if this.logPosChecks < 2 || this.logPosChecks > 8 {
		return 0, 0, errors.New("ROLZ codec inverse transform failed: invalid 'logPosChecks' value in bitstream")
	}

	this.posChecks = 1 << this.logPosChecks
	this.maskChecks = this.posChecks - 1

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
		onlyLiterals := false

		// Scope to deallocate resources early
		{
			// Decode literal, match length and match index buffers
			is := internal.NewBufferStream(src[srcIdx:])
			var ibs kanzi.InputBitStream

			if ibs, err = bitstream.NewDefaultInputBitStream(is, 65536); err != nil {
				goto End
			}

			litLen := int(ibs.ReadBits(32))
			tkLen := int(ibs.ReadBits(32))
			mLenLen := int(ibs.ReadBits(32))
			mIdxLen := int(ibs.ReadBits(32))

			if litLen < 0 || litLen > len(litBuf) {
				err = fmt.Errorf("ROLZ codec: Invalid length for literals: got %d, must be positive and less than or equal to %d", litLen, len(litBuf))
				goto End
			}

			if tkLen < 0 || tkLen > len(tkBuf) {
				err = fmt.Errorf("ROLZ codec: Invalid length for tokens: got %d, must be positive and less than or equal to %d", tkLen, len(tkBuf))
				goto End
			}

			if mLenLen < 0 || mLenLen > len(mLenBuf) {
				err = fmt.Errorf("ROLZ codec: Invalid length for match lengths: got %d, must be positive and less than or equal to %d", mLenLen, len(mLenBuf))
				goto End
			}

			if mIdxLen < 0 || mIdxLen > len(mIdxBuf) {
				err = fmt.Errorf("ROLZ codec: Invalid length for match indexes: got %d, must be positive and less than or equal to %d", mIdxLen, len(mIdxBuf))
				goto End
			}

			var litDec *entropy.ANSRangeDecoder

			if litDec, err = entropy.NewANSRangeDecoderWithCtx(ibs, this.ctx, litOrder); err != nil {
				goto End
			}

			if _, err = litDec.Read(litBuf[0:litLen]); err != nil {
				goto End
			}

			litDec.Dispose()
			var mDec *entropy.ANSRangeDecoder

			if mDec, err = entropy.NewANSRangeDecoderWithCtx(ibs, this.ctx, 0, 32768); err != nil {
				goto End
			}

			if _, err = mDec.Read(tkBuf[0:tkLen]); err != nil {
				goto End
			}

			if _, err = mDec.Read(mLenBuf[0:mLenLen]); err != nil {
				goto End
			}

			if _, err = mDec.Read(mIdxBuf[0:mIdxLen]); err != nil {
				goto End
			}

			mDec.Dispose()
			onlyLiterals = tkLen == 0
			srcIdx += int((ibs.Read() + 7) >> 3)
			ibs.Close()
		}

		if onlyLiterals == true {
			// Shortcut when no match
			copy(buf[dstIdx:], litBuf[0:sizeChunk])
			startChunk = endChunk
			dstIdx += sizeChunk
			continue
		}

		dstIdx = 0
		mm := 8

		if bsVersion < 3 {
			mm = 2
		}

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
			// token LLLLLMMM -> L lit length, M match length
			token := tkBuf[tkIdx]
			tkIdx++
			matchLen := int(token & 0x07)

			if matchLen == 7 {
				ml, deltaIdx := readLengthROLZ(mLenBuf[lenIdx : lenIdx+4])
				lenIdx += deltaIdx
				matchLen = ml + 7
			}

			var litLen int

			if token < 0xF8 {
				litLen = int(token >> 3)
			} else {
				ll, deltaIdx := readLengthROLZ(mLenBuf[lenIdx : lenIdx+4])
				lenIdx += deltaIdx
				litLen = ll + 31
			}

			if litLen > 0 {
				if dstIdx+litLen > len(litBuf) {
					err = errors.New("ROLZ codec inverse transform failed: invalid data")
					goto End
				}

				srcInc := 0
				d := buf[dstIdx-delta:]
				copy(d[delta:], litBuf[litIdx:litIdx+litLen])

				if this.minMatch == _ROLZ_MIN_MATCH3 {
					for n := 0; n < litLen; n++ {
						key := getKey1(d[n:])
						c := (this.counters[key] + 1) & this.maskChecks
						this.matches[(key<<this.logPosChecks)+uint32(c)] = uint32(dstIdx + n)
						this.counters[key] = c
						n += (srcInc >> 6)
						srcInc++
					}
				} else {
					for n := 0; n < litLen; n++ {
						key := getKey2(d[n:])
						c := (this.counters[key] + 1) & this.maskChecks
						this.matches[(key<<this.logPosChecks)+uint32(c)] = uint32(dstIdx + n)
						this.counters[key] = c
						n += (srcInc >> 6)
						srcInc++
					}
				}

				litIdx += litLen
				dstIdx += litLen

				if dstIdx >= sizeChunk {
					// Last chunk literals not followed by match
					if dstIdx == sizeChunk {
						break
					}

					err = errors.New("ROLZ codec inverse transform failed: invalid data")
					goto End
				}
			}

			// Sanity check
			if dstIdx+matchLen+this.minMatch > dstEnd {
				err = errors.New("ROLZ codec inverse transform failed: invalid data")
				goto End
			}

			matchIdx := int32(mIdxBuf[mIdx] & 0xFF)
			mIdx++
			var key uint32

			if this.minMatch == _ROLZ_MIN_MATCH3 {
				key = getKey1(buf[dstIdx-delta:])
			} else {
				key = getKey2(buf[dstIdx-delta:])
			}

			m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
			ref := int(m[(this.counters[key]-matchIdx)&this.maskChecks])
			this.counters[key] = (this.counters[key] + 1) & this.maskChecks
			m[this.counters[key]] = uint32(dstIdx)
			dstIdx = emitCopy(buf, dstIdx, ref, matchLen+this.minMatch)
		}

		startChunk = endChunk
	}

End:
	if err == nil {
		// Emit last literals
		dstIdx += (startChunk - sizeChunk)

		if dstIdx+4 > len(dst) && srcIdx+4 > len(src) {
			err = errors.New("ROLZ codec inverse transform failed: invalid input data")
		} else {
			dst[dstIdx] = src[srcIdx]
			dst[dstIdx+1] = src[srcIdx+1]
			dst[dstIdx+2] = src[srcIdx+2]
			dst[dstIdx+3] = src[srcIdx+3]
			srcIdx += 4
			dstIdx += 4
		}

		if srcIdx != len(src) {
			err = errors.New("ROLZ codec inverse transform failed: invalid input data")
		}
	}

	return uint(srcIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *rolzCodec1) MaxEncodedLen(srcLen int) int {
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
	next := lenBuf[0]
	idx := 1
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
	ctx          *map[string]any
}

func newROLZCodec2(logPosChecks uint) (*rolzCodec2, error) {
	this := &rolzCodec2{}

	if (logPosChecks < 2) || (logPosChecks > 8) {
		return nil, fmt.Errorf("ROLZX codec forward transform failed: invalid logPosChecks parameter: %v (must be in [2..8])", logPosChecks)
	}

	this.logPosChecks = logPosChecks
	this.posChecks = 1 << logPosChecks
	this.maskChecks = this.posChecks - 1
	this.counters = make([]int32, 1<<16)
	this.matches = make([]uint32, _ROLZ_HASH_SIZE<<logPosChecks)
	return this, nil
}

func newROLZCodec2WithCtx(logPosChecks uint, ctx *map[string]any) (*rolzCodec2, error) {
	this := &rolzCodec2{}

	if (logPosChecks < 2) || (logPosChecks > 8) {
		return nil, fmt.Errorf("ROLZX codec forward transform failed: invalid logPosChecks parameter: %d (must be in [2..8])", logPosChecks)
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
	maxMatch := min(_ROLZ_MAX_MATCH2, len(buf)-pos)

	if maxMatch < this.minMatch {
		return -1, -1
	}

	maxMatch -= 4
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

		for n < maxMatch {
			if diff := binary.LittleEndian.Uint32(refBuf[n:]) ^ binary.LittleEndian.Uint32(curBuf[n:]); diff != 0 {
				n += (bits.TrailingZeros32(diff) >> 3)
				break
			}

			n += 4
		}

		if n > bestLen {
			bestIdx = int(i)
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

	return int(counter) - bestIdx, bestLen - this.minMatch
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec2) Forward(src, dst []byte) (uint, uint, error) {
	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("ROLZX codec: Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	srcEnd := len(src) - 4
	srcIdx := 0
	dstIdx := 5
	startChunk := 0
	binary.BigEndian.PutUint32(dst[0:], uint32(len(src)))
	re, _ := newRolzEncoder(9, this.logPosChecks, dst, &dstIdx)

	for i := range this.counters {
		this.counters[i] = 0
	}

	this.minMatch = _ROLZ_MIN_MATCH3
	delta := 2
	flags := byte(0)

	if this.ctx != nil {
		dt := internal.DT_UNDEFINED

		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt = val.(internal.DataType)
		}

		if dt == internal.DT_UNDEFINED {
			var freqs0 [256]int
			internal.ComputeHistogram(src, freqs0[:], true, false)
			dt = internal.DetectSimpleType(len(src), freqs0[:])

			if dt == internal.DT_UNDEFINED {
				(*this.ctx)["dataType"] = dt
			}
		}

		if dt == internal.DT_EXE {
			delta = 3
			flags |= 8
		} else if dt == internal.DT_DNA {
			this.minMatch = _ROLZ_MIN_MATCH7
			flags = 1
		}
	}

	dst[4] = flags
	sizeChunk := min(len(src), _ROLZ_CHUNK_SIZE)

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
			var key uint32

			if this.minMatch == _ROLZ_MIN_MATCH3 {
				key = getKey1(buf[srcIdx-delta:])
			} else {
				key = getKey2(buf[srcIdx-delta:])
			}

			matchIdx, matchLen := this.findMatch(buf, srcIdx, key)

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
		err = errors.New("ROLZX codec forward transform skip: destination buffer too small")
	} else if dstIdx >= len(src) {
		err = errors.New("ROLZX codec forward transform skip: no compression")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec2) Inverse(src, dst []byte) (uint, uint, error) {
	dstEnd := int(binary.BigEndian.Uint32(src[0:]))

	if dstEnd <= 0 || dstEnd > len(dst) {
		return 0, 0, errors.New("ROLZX codec inverse transform failed: invalid data")
	}

	this.minMatch = _ROLZ_MIN_MATCH3
	srcIdx := 4
	bsVersion := uint(6)
	flags := src[4]
	delta := 2

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["bsVersion"]; containsKey {
			bsVersion = val.(uint)
		}
	}

	if bsVersion >= 4 {
		if flags&0x0E == 8 {
			delta = 3
		} else if flags&0x0E == 4 {
			delta = 8
			this.minMatch = _ROLZ_MIN_MATCH7
		}

		srcIdx++
	} else if bsVersion >= 3 {
		if flags == 1 {
			this.minMatch = _ROLZ_MIN_MATCH7
		}

		srcIdx++
	}

	dstIdx := 0
	startChunk := 0
	sizeChunk := min(len(dst), _ROLZ_CHUNK_SIZE)
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

		if bsVersion < 3 {
			mm = 2
		}

		rd.setContext(_ROLZ_LITERAL_CTX, 0)

		if startChunk >= dstEnd {
			mm = dstEnd - startChunk
		}

		for j := 0; j < mm; j++ {
			val := rd.decode9Bits()

			// Sanity check
			if val>>8 == _ROLZ_MATCH_FLAG {
				dstIdx += startChunk
				return uint(srcIdx), uint(dstIdx), errors.New("ROLZX codec inverse transform failed: invalid data")
			}

			buf[dstIdx] = byte(val)
			dstIdx++
		}

		// Next chunk
		for dstIdx < sizeChunk {
			savedIdx := dstIdx
			var key uint32

			if this.minMatch == _ROLZ_MIN_MATCH3 {
				key = getKey1(buf[dstIdx-delta:])
			} else {
				key = getKey2(buf[dstIdx-delta:])
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
					return uint(srcIdx), uint(dstIdx), errors.New("ROLZX codec inverse transform failed: invalid data")
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
		err = errors.New("ROLZX codec inverse transform failed: invalid data")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *rolzCodec2) MaxEncodedLen(srcLen int) int {
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
