/*
Copyright 2011-2017 Frederic Langlet
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

package function

import (
	"encoding/binary"
	"errors"
	"fmt"
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
	_ROLZ_MIN_MATCH       = 3
	_ROLZ_MAX_MATCH       = _ROLZ_MIN_MATCH + 255 + 7
	_ROLZ_LOG_POS_CHECKS1 = 4
	_ROLZ_LOG_POS_CHECKS2 = 5
	_ROLZ_CHUNK_SIZE      = 1 << 26 // 64 MB
	_ROLZ_HASH_MASK       = int32(^(_ROLZ_CHUNK_SIZE - 1))
	_ROLZ_LITERAL_FLAG    = 0
	_ROLZ_MATCH_FLAG      = 1
	_ROLZ_HASH            = int32(200002979)
	_ROLZ_MAX_BLOCK_SIZE  = 1 << 30 // 1 GB
	_ROLZ_TOP             = uint64(0x00FFFFFFFFFFFFFF)
	MASK_24_56            = uint64(0x00FFFFFFFF000000)
	MASK_0_24             = uint64(0x0000000000FFFFFF)
	MASK_0_56             = uint64(0x00FFFFFFFFFFFFFF)
	MASK_0_32             = uint64(0x00000000FFFFFFFF)
)

func getKey(p []byte) uint32 {
	return uint32(binary.LittleEndian.Uint16(p))
}

func hash(p []byte) int32 {
	return ((int32(binary.LittleEndian.Uint32(p)) & 0x00FFFFFF) * _ROLZ_HASH) & _ROLZ_HASH_MASK
}

func emitCopy(buf []byte, dstIdx, ref, matchLen int) int {
	buf[dstIdx] = buf[ref]
	buf[dstIdx+1] = buf[ref+1]
	buf[dstIdx+2] = buf[ref+2]
	dstIdx += 3
	ref += 3

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
	delegate kanzi.ByteFunction
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
	var d kanzi.ByteFunction

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
	var d kanzi.ByteFunction

	if val, containsKey := (*ctx)["transform"]; containsKey {
		transform := val.(string)

		if strings.Contains(transform, "ROLZX") {
			d, err = newROLZCodec2(_ROLZ_LOG_POS_CHECKS2)
			this.delegate = d
		}
	}

	if this.delegate == nil && err == nil {
		d, err = newROLZCodec1(_ROLZ_LOG_POS_CHECKS1)
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

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("ROLZ codec: Input and output buffers cannot be equal")
	}

	if len(src) > _ROLZ_MAX_BLOCK_SIZE {
		// Not a recoverable error: instead of a (silent) failure,
		// issue a fatal error.
		panic(fmt.Errorf("The max ROLZ codec block size is %v, got %v", _ROLZ_MAX_BLOCK_SIZE, len(src)))
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
		panic(fmt.Errorf("The max ROLZ codec block size is %v, got %v", _ROLZ_MAX_BLOCK_SIZE, len(src)))
	}

	return this.delegate.Inverse(src, dst)
}

func (this *ROLZCodec) MaxEncodedLen(srcLen int) int {
	return this.delegate.MaxEncodedLen(srcLen)
}

// Use ANS to encode/decode literals and matches
type rolzCodec1 struct {
	matches      []int32
	counters     []int32
	logPosChecks uint
	maskChecks   int32
	posChecks    int32
}

func newROLZCodec1(logPosChecks uint) (*rolzCodec1, error) {
	this := &rolzCodec1{}

	if (logPosChecks < 2) || (logPosChecks > 8) {
		return nil, fmt.Errorf("ROLZ codec: Invalid logPosChecks parameter: %v (must be in [2..8])", logPosChecks)
	}

	this.logPosChecks = logPosChecks
	this.posChecks = 1 << logPosChecks
	this.maskChecks = this.posChecks - 1
	this.counters = make([]int32, 1<<16)
	this.matches = make([]int32, _ROLZ_HASH_SIZE<<logPosChecks)
	return this, nil
}

// findMatch returns match position index (logPosChecks bits) + length (8 bits) or -1
func (this *rolzCodec1) findMatch(buf []byte, pos int) (int, int) {
	key := getKey(buf[pos-2:])

	if this.posChecks == 0 {
		// Ahem terrible hack ... Do not try this at home, kids.
		// This impossible branch speeds up the code (due to speculative
		// memory fetch in the other branch probably)
		return -1, -1
	}

	m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
	hash32 := hash(buf[pos : pos+4])
	counter := this.counters[key]
	bestLen := _ROLZ_MIN_MATCH - 1
	bestIdx := -1
	curBuf := buf[pos:]
	maxMatch := _ROLZ_MAX_MATCH

	if maxMatch > len(buf)-pos {
		maxMatch = len(buf) - pos
	}

	// Check all recorded positions
	for i := counter; i > counter-this.posChecks; i-- {
		ref := m[i&this.maskChecks]

		if ref == 0 {
			break
		}

		// Hash check may save a memory access ...
		if ref&_ROLZ_HASH_MASK != hash32 {
			continue
		}

		ref &= ^_ROLZ_HASH_MASK

		if buf[ref] != curBuf[0] {
			continue
		}

		refBuf := buf[ref:]
		n := 1

		if (n < maxMatch-4) && (binary.LittleEndian.Uint32(refBuf[n:]) == binary.LittleEndian.Uint32(curBuf[n:])) {
			n += 4
		}

		for (n < maxMatch) && (refBuf[n] == curBuf[n]) {
			n++
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
	this.counters[key]++
	m[(counter+1)&this.maskChecks] = hash32 | int32(pos)

	if bestLen < _ROLZ_MIN_MATCH {
		return -1, -1
	}

	return bestIdx, bestLen - _ROLZ_MIN_MATCH
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec1) Forward(src, dst []byte) (uint, uint, error) {
	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("ROLZ codec: Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	srcIdx := 0
	dstIdx := 0
	srcEnd := len(src) - 4
	binary.BigEndian.PutUint32(dst[dstIdx:], uint32(len(src)))
	dstIdx += 4
	sizeChunk := len(src)

	if sizeChunk > _ROLZ_CHUNK_SIZE {
		sizeChunk = _ROLZ_CHUNK_SIZE
	}

	startChunk := 0
	litBuf := make([]byte, this.MaxEncodedLen(sizeChunk))
	lenBuf := make([]byte, sizeChunk/2)
	mIdxBuf := make([]byte, sizeChunk/2)
	var err error

	for i := range this.counters {
		this.counters[i] = 0
	}

	litOrder := uint(1)

	if len(src) < 1<<17 {
		litOrder = 0
	}

	dst[dstIdx] = byte(litOrder)
	dstIdx++

	// Main loop
	for startChunk < srcEnd {
		litIdx := 0
		lenIdx := 0
		mIdx := 0

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
		litBuf[litIdx] = buf[srcIdx]
		litIdx++
		srcIdx++

		if startChunk+1 < srcEnd {
			litBuf[litIdx] = buf[srcIdx]
			litIdx++
			srcIdx++
		}

		firstLitIdx := srcIdx

		// Next chunk
		for srcIdx < sizeChunk {
			matchIdx, matchLen := this.findMatch(buf, srcIdx)

			if matchIdx == -1 {
				srcIdx++
				continue
			}

			// Emit match and literal lengths
			litLen := srcIdx - firstLitIdx
			lenIdx += emitLengths(lenBuf[lenIdx:], litLen, matchLen)

			// Emit literals
			if litLen > 0 {
				copy(litBuf[litIdx:], buf[firstLitIdx:firstLitIdx+litLen])
				litIdx += litLen
			}

			// Emit match index
			mIdxBuf[mIdx] = byte(matchIdx)
			mIdx++
			srcIdx += (matchLen + _ROLZ_MIN_MATCH)
			firstLitIdx = srcIdx
		}

		// Emit last chunk literals
		litLen := srcIdx - firstLitIdx
		lenIdx += emitLengths(lenBuf[lenIdx:], litLen, 0)

		for i := 0; i < litLen; i++ {
			litBuf[litIdx+i] = buf[firstLitIdx+i]
		}

		litIdx += litLen
		var os util.BufferStream

		// Scope to deallocate resources early
		{
			// Encode literal, length and match index buffers
			var obs kanzi.OutputBitStream

			if obs, err = bitstream.NewDefaultOutputBitStream(&os, 65536); err != nil {
				break
			}

			obs.WriteBits(uint64(litIdx), 32)
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
			var lenEnc *entropy.ANSRangeEncoder

			if lenEnc, err = entropy.NewANSRangeEncoder(obs, 0); err != nil {
				goto End
			}

			if _, err = lenEnc.Write(lenBuf[0:lenIdx]); err != nil {
				goto End
			}

			lenEnc.Dispose()
			var mIdxEnc *entropy.ANSRangeEncoder

			if mIdxEnc, err = entropy.NewANSRangeEncoder(obs, 0); err != nil {
				goto End
			}

			if _, err = mIdxEnc.Write(mIdxBuf[0:mIdx]); err != nil {
				goto End
			}

			mIdxEnc.Dispose()
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

	srcIdx := 4
	dstIdx := 0
	litBuf := make([]byte, this.MaxEncodedLen(sizeChunk))
	lenBuf := make([]byte, sizeChunk/2)
	mIdxBuf := make([]byte, sizeChunk/2)
	var err error

	for i := range this.counters {
		this.counters[i] = 0
	}

	litOrder := uint(src[srcIdx])
	srcIdx++

	// Main loop
	for startChunk < dstEnd {
		mIdx := 0
		lenIdx := 0
		litIdx := 0

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
				break
			}

			if ibs, err = bitstream.NewDefaultInputBitStream(&is, 65536); err != nil {
				break
			}

			litLen := int(ibs.ReadBits(32))
			mLenLen := int(ibs.ReadBits(32))
			mIdxLen := int(ibs.ReadBits(32))

			if litLen <= sizeChunk {
				{
					var litDec *entropy.ANSRangeDecoder

					if litDec, err = entropy.NewANSRangeDecoder(ibs, litOrder); err != nil {
						goto End
					}

					if _, err = litDec.Read(litBuf[0:litLen]); err != nil {
						goto End
					}

					litDec.Dispose()
				}

				if mLenLen <= sizeChunk {
					{
						var lenDec *entropy.ANSRangeDecoder

						if lenDec, err = entropy.NewANSRangeDecoder(ibs, 0); err != nil {
							goto End
						}

						if _, err = lenDec.Read(lenBuf[0:mLenLen]); err != nil {
							goto End
						}

						lenDec.Dispose()
					}

					if mIdxLen <= sizeChunk {
						var mIdxDec *entropy.ANSRangeDecoder

						if mIdxDec, err = entropy.NewANSRangeDecoder(ibs, 0); err != nil {
							goto End
						}

						if _, err = mIdxDec.Read(mIdxBuf[0:mIdxLen]); err != nil {
							goto End
						}

						mIdxDec.Dispose()
					}
				}
			}

			srcIdx += int((ibs.Read() + 7) >> 3)
			ibs.Close()

			if litLen > sizeChunk {
				err = fmt.Errorf("ROLZ codec: Invalid length: got %v, must be less than or equal to %v", litLen, sizeChunk)
				goto End
			}

			if mLenLen > sizeChunk {
				err = fmt.Errorf("ROLZ codec: Invalid length: got %v, must be less than or equal to %v", mLenLen, sizeChunk)
				goto End
			}

			if mIdxLen > sizeChunk {
				err = fmt.Errorf("ROLZ codec: Invalid length: got %v, must be less than or equal to %v", mIdxLen, sizeChunk)
				goto End
			}
		}

		dstIdx = 0
		buf[dstIdx] = litBuf[litIdx]
		dstIdx++
		litIdx++

		if startChunk+1 < dstEnd {
			buf[dstIdx] = litBuf[litIdx]
			dstIdx++
			litIdx++
		}

		// Next chunk
		for dstIdx < sizeChunk {
			litLen, matchLen, deltaIdx := this.readLengths(lenBuf[lenIdx:])
			lenIdx += deltaIdx
			this.emitLiterals(litBuf[litIdx:litIdx+litLen], buf, dstIdx)
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

			// Sanity check
			if dstIdx+matchLen+_ROLZ_MIN_MATCH > dstEnd {
				err = errors.New("ROLZ codec: Invalid input data")
				goto End
			}

			matchIdx := int32(mIdxBuf[mIdx] & 0xFF)
			mIdx++
			key := getKey(buf[dstIdx-2:])
			m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
			ref := m[(this.counters[key]-matchIdx)&this.maskChecks]
			savedIdx := int32(dstIdx)
			dstIdx = emitCopy(buf, dstIdx, int(ref), matchLen)
			this.counters[key]++
			m[this.counters[key]&this.maskChecks] = savedIdx
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

func emitLengths(litBuf []byte, litLen, mLen int) int {
	// mode LLLLLMMM -> L lit length, M match length
	var mode byte

	if litLen < 0x1F {
		mode = byte(litLen << 3)
	} else {
		mode = 0xF8
	}

	var idx int

	if mLen >= 7 {
		litBuf[0] = mode | 0x07
		litBuf[1] = byte(mLen - 7)
		idx = 2
	} else {
		litBuf[0] = mode | byte(mLen)
		idx = 1
	}

	if litLen >= 0x1F {
		litLen -= 0x1F

		if litLen >= 1<<7 {
			if litLen >= 1<<14 {
				if litLen >= 1<<21 {
					litBuf[idx] = byte(0x80 | ((litLen >> 21) & 0x7F))
					idx++
				}

				litBuf[idx] = byte(0x80 | ((litLen >> 14) & 0x7F))
				idx++
			}

			litBuf[idx] = byte(0x80 | ((litLen >> 7) & 0x7F))
			idx++
		}

		litBuf[idx] = byte(litLen & 0x7F)
		idx++
	}

	return idx
}

func (this rolzCodec1) readLengths(lenBuf []byte) (int, int, int) {
	// mode LLLLLMMM -> L lit length, M match length
	mode := lenBuf[0]
	idx := 1
	mLen := int(mode & 0x07)

	if mLen == 7 {
		mLen += int(lenBuf[1])
		idx = 2
	}

	if mode < 0xF8 {
		return int(mode >> 3), mLen, idx
	}

	next := lenBuf[idx]
	idx++
	litLen := int(next & 0x7F)

	if next&0x80 != 0 {
		next = lenBuf[idx]
		idx++
		litLen = (litLen << 7) | int(next&0x7F)

		if next&0x80 != 0 {
			next = lenBuf[idx]
			idx++
			litLen = (litLen << 7) | int(next&0x7F)

			if next&0x80 != 0 {
				next = lenBuf[idx]
				idx++
				litLen = (litLen << 7) | int(next&0x7F)
			}
		}
	}

	return litLen + 31, mLen, idx
}

func (this rolzCodec1) emitLiterals(litBuf, dst []byte, dstIdx int) {
	d := dst[dstIdx-2:]
	copy(d[2:], litBuf)

	for n := range litBuf {
		key := getKey(d[n:])
		m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
		this.counters[key]++
		m[this.counters[key]&this.maskChecks] = int32(dstIdx + n)
	}
}

// Use CM (ROLZEncoder/ROLZDecoder) to encode/decode literals and matches
// Code loosely based on 'balz' by Ilya Muravyov
type rolzCodec2 struct {
	matches        []int32
	counters       []int32
	logPosChecks   uint
	maskChecks     int32
	posChecks      int32
	litPredictor   *rolzPredictor
	matchPredictor *rolzPredictor
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
	this.matches = make([]int32, _ROLZ_HASH_SIZE<<logPosChecks)
	this.litPredictor, _ = newRolzPredictor(9)
	this.matchPredictor, _ = newRolzPredictor(logPosChecks)
	return this, nil
}

// findMatch returns match position index and length or -1
func (this *rolzCodec2) findMatch(buf []byte, pos int) (int, int) {
	key := getKey(buf[pos-2:])

	if this.posChecks == 0 {
		// Ahem terrible hack ... Do not try this at home, kids.
		// This impossible branch speeds up the code (due to speculative
		// memory fetch in the other branch probably)
		return -1, -1
	}

	m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
	hash32 := hash(buf[pos : pos+4])
	counter := this.counters[key]
	bestLen := _ROLZ_MIN_MATCH - 1
	bestIdx := -1
	curBuf := buf[pos:]
	maxMatch := _ROLZ_MAX_MATCH

	if maxMatch > len(buf)-pos {
		maxMatch = len(buf) - pos
	}

	// Check all recorded positions
	for i := counter; i > counter-this.posChecks; i-- {
		ref := m[i&this.maskChecks]

		if ref == 0 {
			break
		}

		// Hash check may save a memory access ...
		if ref&_ROLZ_HASH_MASK != hash32 {
			continue
		}

		ref &= ^_ROLZ_HASH_MASK

		if buf[ref] != curBuf[0] {
			continue
		}

		refBuf := buf[ref:]
		n := 1

		if (n < maxMatch-4) && (binary.LittleEndian.Uint32(refBuf[n:]) == binary.LittleEndian.Uint32(curBuf[n:])) {
			n += 4
		}

		for (n < maxMatch) && (refBuf[n] == curBuf[n]) {
			n++
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
	this.counters[key]++
	m[(counter+1)&this.maskChecks] = hash32 | int32(pos)

	if bestLen < _ROLZ_MIN_MATCH {
		return -1, -1
	}

	return bestIdx, bestLen - _ROLZ_MIN_MATCH
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec2) Forward(src, dst []byte) (uint, uint, error) {
	if n := this.MaxEncodedLen(len(src)); len(dst) < n {
		return 0, 0, fmt.Errorf("ROLZX codec: Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	srcIdx := 0
	dstIdx := 0
	srcEnd := len(src) - 4
	sizeChunk := len(src)

	if sizeChunk > _ROLZ_CHUNK_SIZE {
		sizeChunk = _ROLZ_CHUNK_SIZE
	}

	startChunk := 0
	binary.BigEndian.PutUint32(dst[dstIdx:], uint32(len(src)))
	dstIdx += 4
	this.litPredictor.reset()
	this.matchPredictor.reset()
	predictors := [2]kanzi.Predictor{this.litPredictor, this.matchPredictor}
	re, _ := newRolzEncoder(predictors[:], dst, &dstIdx)

	for i := range this.counters {
		this.counters[i] = 0
	}

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
		buf := src[startChunk:endChunk]
		srcIdx = 0
		this.litPredictor.setContext(0)
		re.setContext(_ROLZ_LITERAL_FLAG)
		re.encodeBit(_ROLZ_LITERAL_FLAG)
		re.encodeByte(buf[srcIdx])
		srcIdx++

		if startChunk+1 < srcEnd {
			re.encodeBit(_ROLZ_LITERAL_FLAG)
			re.encodeByte(buf[srcIdx])
			srcIdx++
		}

		// Next chunk
		for srcIdx < sizeChunk {
			this.litPredictor.setContext(buf[srcIdx-1])
			re.setContext(_ROLZ_LITERAL_FLAG)
			matchIdx, matchLen := this.findMatch(buf, srcIdx)

			if matchIdx < 0 {
				re.encodeBit(_ROLZ_LITERAL_FLAG)
				re.encodeByte(buf[srcIdx])
				srcIdx++
			} else {
				re.encodeBit(_ROLZ_MATCH_FLAG)
				re.encodeByte(byte(matchLen))
				this.matchPredictor.setContext(buf[srcIdx-1])
				re.setContext(_ROLZ_MATCH_FLAG)

				for shift := this.logPosChecks; shift > 0; shift-- {
					re.encodeBit(byte(matchIdx>>(shift-1)) & 1)
				}

				srcIdx += (matchLen + _ROLZ_MIN_MATCH)
			}
		}

		startChunk = endChunk
	}

	// Emit last literals
	srcIdx += (startChunk - sizeChunk)
	re.setContext(_ROLZ_LITERAL_FLAG)

	for i := 0; i < 4; i++ {
		this.litPredictor.setContext(src[srcIdx-1])
		re.encodeBit(_ROLZ_LITERAL_FLAG)
		re.encodeByte(src[srcIdx])
		srcIdx++
	}

	re.dispose()
	var err error

	if srcIdx != len(src) {
		err = errors.New("ROLZX codec: Destination buffer too small")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *rolzCodec2) Inverse(src, dst []byte) (uint, uint, error) {
	srcIdx := 0
	dstIdx := 0
	dstEnd := int(binary.BigEndian.Uint32(src[srcIdx:]))

	srcIdx += 4
	sizeChunk := len(dst)

	if sizeChunk > _ROLZ_CHUNK_SIZE {
		sizeChunk = _ROLZ_CHUNK_SIZE
	}

	startChunk := 0
	this.litPredictor.reset()
	this.matchPredictor.reset()
	predictors := [2]kanzi.Predictor{this.litPredictor, this.matchPredictor}
	rd, _ := newRolzDecoder(predictors[:], src, &srcIdx)

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
		dstIdx = 0
		this.litPredictor.setContext(0)
		rd.setContext(_ROLZ_LITERAL_FLAG)
		bit := rd.decodeBit()

		if bit == _ROLZ_LITERAL_FLAG {
			buf[dstIdx] = rd.decodeByte()
			dstIdx++

			if startChunk+1 < dstEnd {
				bit = rd.decodeBit()

				if bit == _ROLZ_LITERAL_FLAG {
					buf[dstIdx] = rd.decodeByte()
					dstIdx++
				}
			}
		}

		// Sanity check
		if bit == _ROLZ_MATCH_FLAG {
			dstIdx += startChunk
			break
		}

		// Next chunk
		for dstIdx < sizeChunk {
			savedIdx := dstIdx
			key := getKey(buf[dstIdx-2:])
			m := this.matches[key<<this.logPosChecks : (key+1)<<this.logPosChecks]
			this.litPredictor.setContext(buf[dstIdx-1])
			rd.setContext(_ROLZ_LITERAL_FLAG)

			if rd.decodeBit() == _ROLZ_MATCH_FLAG {
				// Match flag
				matchLen := int(rd.decodeByte())
				// Sanity check
				if matchLen+3 > dstEnd {
					dstIdx += startChunk
					break
				}

				this.matchPredictor.setContext(buf[dstIdx-1])
				rd.setContext(_ROLZ_MATCH_FLAG)
				matchIdx := int32(0)

				for shift := this.logPosChecks; shift != 0; shift-- {
					matchIdx |= int32(rd.decodeBit() << (shift - 1))
				}

				ref := m[(this.counters[key]-matchIdx)&this.maskChecks]
				dstIdx = emitCopy(buf, dstIdx, int(ref), matchLen)
			} else {
				// Literal flag
				buf[dstIdx] = rd.decodeByte()
				dstIdx++
			}

			// Update
			this.counters[key]++
			m[this.counters[key]&this.maskChecks] = int32(savedIdx)
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
	if srcLen >= _ROLZ_CHUNK_SIZE {
		return srcLen
	}

	if srcLen <= 512 {
		return srcLen + 64
	}

	return srcLen + srcLen/8
}

type rolzPredictor struct {
	probs     []int
	p         []int
	logSize   uint
	threshold int
	c1        int
}

func newRolzPredictor(logPosChecks uint) (*rolzPredictor, error) {
	this := &rolzPredictor{}
	this.logSize = logPosChecks
	this.threshold = 1 << (logPosChecks - 1)
	this.probs = make([]int, 256<<logPosChecks)
	this.reset()
	return this, nil
}

func (this *rolzPredictor) reset() {
	this.c1 = 1
	this.p = this.probs[0:256]

	for i := range this.probs {
		this.probs[i] = 1 << 15
	}
}

func (this *rolzPredictor) Update(bit byte) {
	b := int(bit)
	this.p[this.c1] -= (((this.p[this.c1] - (-b & 0xFFFF)) >> 5) + b)

	if this.c1 >= this.threshold {
		this.c1 = 1
	} else {
		this.c1 = (this.c1 << 1) + b
	}
}

func (this *rolzPredictor) Get() int {
	return this.p[this.c1] >> 4
}

func (this *rolzPredictor) setContext(ctx byte) {
	this.p = this.probs[int(ctx)<<this.logSize:]
}

type rolzEncoder struct {
	predictors []kanzi.Predictor
	predictor  kanzi.Predictor
	buf        []byte
	idx        *int
	low        uint64
	high       uint64
}

func newRolzEncoder(predictors []kanzi.Predictor, buf []byte, idx *int) (*rolzEncoder, error) {
	this := &rolzEncoder{}
	this.low = 0
	this.high = _ROLZ_TOP
	this.buf = buf
	this.idx = idx
	this.predictors = predictors
	this.predictor = predictors[0]
	return this, nil
}

func (this *rolzEncoder) setContext(n int) {
	this.predictor = this.predictors[n]
}

func (this *rolzEncoder) encodeByte(val byte) {
	this.encodeBit((val >> 7) & 1)
	this.encodeBit((val >> 6) & 1)
	this.encodeBit((val >> 5) & 1)
	this.encodeBit((val >> 4) & 1)
	this.encodeBit((val >> 3) & 1)
	this.encodeBit((val >> 2) & 1)
	this.encodeBit((val >> 1) & 1)
	this.encodeBit(val & 1)
}

func (this *rolzEncoder) encodeBit(bit byte) {
	// Calculate interval split
	split := (((this.high - this.low) >> 4) * uint64(this.predictor.Get())) >> 8

	// Update fields with new interval bounds
	if bit == 0 {
		this.low += (split + 1)
	} else {
		this.high = this.low + split
	}

	// Update predictor
	this.predictor.Update(bit)

	// Write unchanged first 32 bits to bitstream
	for (this.low^this.high)&MASK_24_56 == 0 {
		binary.BigEndian.PutUint32(this.buf[*this.idx:*this.idx+4], uint32(this.high>>32))
		*this.idx += 4
		this.low <<= 32
		this.high = (this.high << 32) | MASK_0_32
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
	predictors []kanzi.Predictor
	predictor  kanzi.Predictor
	buf        []byte
	idx        *int
	low        uint64
	high       uint64
	current    uint64
}

func newRolzDecoder(predictors []kanzi.Predictor, buf []byte, idx *int) (*rolzDecoder, error) {
	this := new(rolzDecoder)
	this.low = 0
	this.high = _ROLZ_TOP
	this.buf = buf
	this.idx = idx
	this.current = uint64(0)

	for i := 0; i < 8; i++ {
		this.current = (this.current << 8) | (uint64(this.buf[*this.idx+i]) & 0xFF)
	}

	*this.idx += 8
	this.predictors = predictors
	this.predictor = predictors[0]
	return this, nil
}

func (this *rolzDecoder) setContext(n int) {
	this.predictor = this.predictors[n]
}

func (this *rolzDecoder) decodeByte() byte {
	return (this.decodeBit() << 7) |
		(this.decodeBit() << 6) |
		(this.decodeBit() << 5) |
		(this.decodeBit() << 4) |
		(this.decodeBit() << 3) |
		(this.decodeBit() << 2) |
		(this.decodeBit() << 1) |
		this.decodeBit()
}

func (this *rolzDecoder) decodeBit() byte {
	// Calculate interval split
	split := this.low + ((((this.high - this.low) >> 4) * uint64(this.predictor.Get())) >> 8)
	var bit byte

	// Update predictor
	if split >= this.current {
		bit = 1
		this.high = split
		this.predictor.Update(1)
	} else {
		bit = 0
		this.low = -^split
		this.predictor.Update(0)
	}

	// Read 32 bits from bitstream
	for (this.low^this.high)&MASK_24_56 == 0 {
		this.low = (this.low << 32) & MASK_0_56
		this.high = ((this.high << 32) | MASK_0_32) & MASK_0_56
		val := uint64(binary.BigEndian.Uint32(this.buf[*this.idx : *this.idx+4]))
		this.current = ((this.current << 32) | val) & MASK_0_56
		*this.idx += 4
	}

	return bit
}

func (this *rolzDecoder) dispose() {
}
