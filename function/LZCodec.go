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

package function

import (
	"encoding/binary"
	"errors"
	"fmt"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	_LZX_HASH_SEED          = 0x1E35A7BD
	_LZX_HASH_LOG1          = 16
	_LZX_HASH_SHIFT1        = 40 - _LZX_HASH_LOG1
	_LZX_HASH_MASK1         = (1 << _LZX_HASH_LOG1) - 1
	_LZX_HASH_LOG2          = 21
	_LZX_HASH_SHIFT2        = 48 - _LZX_HASH_LOG2
	_LZX_HASH_MASK2         = (1 << _LZX_HASH_LOG2) - 1
	_LZX_MAX_DISTANCE1      = (1 << 17) - 2
	_LZX_MAX_DISTANCE2      = (1 << 24) - 2
	_LZX_MIN_MATCH          = 5
	_LZX_MAX_MATCH          = 32767 + _LZX_MIN_MATCH
	_LZX_MIN_BLOCK_LENGTH   = 24
	_LZX_MIN_MATCH_MIN_DIST = 1 << 16
	_LZP_HASH_SEED          = 0x7FEB352D
	_LZP_HASH_LOG           = 16
	_LZP_HASH_SHIFT         = 32 - _LZP_HASH_LOG
	_LZP_MIN_MATCH          = 64
	_LZP_MATCH_FLAG         = 0xFC
	_LZP_MIN_BLOCK_LENGTH   = 128
)

// LZCodec encapsulates an implementation of a Lempel-Ziv codec
type LZCodec struct {
	delegate kanzi.ByteFunction
}

// NewLZCodec creates a new instance of LZCodec
func NewLZCodec() (*LZCodec, error) {
	this := &LZCodec{}
	d, err := NewLZXCodec()
	this.delegate = d
	return this, err
}

// MaxEncodedLen returns the max size required for the encoding output mBuf
func (this *LZCodec) MaxEncodedLen(srcLen int) int {
	return this.delegate.MaxEncodedLen(srcLen)
}

// NewLZCodecWithCtx creates a new instance of LZCodec using a
// configuration map as parameter.
func NewLZCodecWithCtx(ctx *map[string]interface{}) (*LZCodec, error) {
	this := &LZCodec{}

	var err error
	var d kanzi.ByteFunction

	if val, containsKey := (*ctx)["lz"]; containsKey {
		lzType := val.(uint64)

		if lzType == LZP_TYPE {
			d, err = NewLZPCodecWithCtx(ctx)
			this.delegate = d
		}
	}

	if this.delegate == nil && err == nil {
		d, err = NewLZXCodecWithCtx(ctx)
		this.delegate = d
	}

	return this, err
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output mBufs cannot be equal")
	}

	return this.delegate.Forward(src, dst)
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output mBufs cannot be equal")
	}

	return this.delegate.Inverse(src, dst)
}

// LZXCodec Simple byte oriented LZ77 implementation.
// It is a based on a heavily modified LZ4 with a bigger window, a bigger
// hash map, 3+n*8 bit literal lengths and 17 or 24 bit match lengths.
type LZXCodec struct {
	hashes []int32
	mBuf   []byte
	tkBuf  []byte
	extra  bool
}

// NewLZXCodec creates a new instance of LZXCodec
func NewLZXCodec() (*LZXCodec, error) {
	this := &LZXCodec{}
	this.hashes = make([]int32, 0)
	this.mBuf = make([]byte, 0)
	this.tkBuf = make([]byte, 0)
	this.extra = false
	return this, nil
}

// NewLZXCodecWithCtx creates a new instance of LZXCodec using a
// configuration map as parameter.
func NewLZXCodecWithCtx(ctx *map[string]interface{}) (*LZXCodec, error) {
	this := &LZXCodec{}
	this.hashes = make([]int32, 0)
	this.mBuf = make([]byte, 0)
	this.tkBuf = make([]byte, 0)
	this.extra = false

	if val, containsKey := (*ctx)["lz"]; containsKey {
		lzType := val.(uint64)

		this.extra = lzType == LZX_TYPE
	}

	return this, nil
}

func emitLengthLZ(block []byte, length int) int {
	if length < 254 {
		block[0] = byte(length)
		return 1
	}

	if length < 65536+254 {
		length -= 254
		block[0] = byte(254)
		block[1] = byte(length >> 8)
		block[2] = byte(length)
		return 3
	}

	length -= 255
	block[0] = byte(255)
	block[1] = byte(length >> 16)
	block[2] = byte(length >> 8)
	block[3] = byte(length)
	return 4
}

func readLengthLZ(block []byte) (int, int) {
	res := int(block[0])
	idx := 1

	if res < 254 {
		return res, idx
	}

	if res == 254 {
		res += (int(block[idx]) << 8)
		res += int(block[idx+1])
		return res, idx + 2
	}

	res += (int(block[idx]) << 16)
	res += (int(block[idx+1]) << 8)
	res += int(block[idx+2])
	return res, idx + 3
}

func emitLiteralsLZ(src, dst []byte) {
	length := len(src)

	for i := 0; i < length; i += 16 {
		copy(dst[i:], src[i:i+16])
	}
}

func (this *LZXCodec) hash(p []byte) uint32 {
	if this.extra == true {
		return uint32((binary.LittleEndian.Uint64(p)*_LZX_HASH_SEED)>>_LZX_HASH_SHIFT2) & _LZX_HASH_MASK2
	} else {
		return uint32((binary.LittleEndian.Uint64(p)*_LZX_HASH_SEED)>>_LZX_HASH_SHIFT1) & _LZX_HASH_MASK1
	}
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZXCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output mBuf is too small - size: %d, required %d", len(dst), n)
	}

	// If too small, skip
	if count < _LZX_MIN_BLOCK_LENGTH {
		return 0, 0, errors.New("Block too small, skip")
	}

	if len(this.hashes) == 0 {
		if this.extra == true {
			this.hashes = make([]int32, 1<<_LZX_HASH_LOG2)
		} else {
			this.hashes = make([]int32, 1<<_LZX_HASH_LOG1)
		}
	} else {
		for i := range this.hashes {
			this.hashes[i] = 0
		}
	}

	minBufSize := count / 5

	if minBufSize < 256 {
		minBufSize = 256
	}

	if len(this.mBuf) < minBufSize {
		this.mBuf = make([]byte, minBufSize)
	}

	if len(this.tkBuf) < minBufSize {
		this.tkBuf = make([]byte, minBufSize)
	}

	srcEnd := count - 16 - 1
	maxDist := _LZX_MAX_DISTANCE2
	dst[8] = 1

	if srcEnd < 4*_LZX_MAX_DISTANCE1 {
		maxDist = _LZX_MAX_DISTANCE1
		dst[8] = 0
	}

	srcIdx := 0
	dstIdx := 9
	anchor := 0
	mIdx := 0
	tkIdx := 0
	repd := 0

	for srcIdx < srcEnd {
		var minRef int

		if srcIdx < maxDist {
			minRef = 0
		} else {
			minRef = srcIdx - maxDist
		}

		h := this.hash(src[srcIdx:])
		ref := int(this.hashes[h])
		this.hashes[h] = int32(srcIdx)
		bestLen := 0

		// Find a match
		if ref > minRef {
			maxMatch := srcEnd - srcIdx

			if maxMatch > _LZX_MAX_MATCH {
				maxMatch = _LZX_MAX_MATCH
			}

			bestLen = findMatch(src, srcIdx, ref, maxMatch)
		}

		// No good match ?
		if bestLen < _LZX_MIN_MATCH || (bestLen == _LZX_MIN_MATCH && srcIdx-ref >= _LZX_MIN_MATCH_MIN_DIST) {
			srcIdx++
			continue
		}

		// Check if better match at next position
		h2 := this.hash(src[srcIdx+1:])
		ref2 := int(this.hashes[h2])
		this.hashes[h2] = int32(srcIdx + 1)
		bestLen2 := 0

		// Find a match
		if ref2 > minRef+1 {
			maxMatch := srcEnd - srcIdx - 1

			if maxMatch > _LZX_MAX_MATCH {
				maxMatch = _LZX_MAX_MATCH
			}

			bestLen2 = findMatch(src, srcIdx+1, ref2, maxMatch)
		}

		// Select best match
		if (bestLen2 > bestLen) || ((bestLen2 == bestLen) && (srcIdx-ref2 < srcIdx-ref)) {
			ref = ref2
			bestLen = bestLen2
			srcIdx++
		}

		// Emit token
		// Token: 3 bits litLen + 1 bit flag + 4 bits mLen (LLLFMMMM)
		// flag = if maxDist = _LZX_MAX_DISTANCE1, then highest bit of distance
		//        else 1 if dist needs 3 bytes (> 0xFFFF) and 0 otherwise
		mLen := bestLen - _LZX_MIN_MATCH
		d := srcIdx - ref
		var dist int

		if d == repd {
			dist = 0
		} else {
			dist = d + 1
		}

		repd = d
		var token int

		if dist > 0xFFFF {
			token = 0x10
		} else {
			token = 0
		}

		if mLen < 15 {
			token += mLen
		} else {
			token += 15
		}

		// Literals to process ?
		if anchor == srcIdx {
			this.tkBuf[tkIdx] = byte(token)
			tkIdx++
		} else {
			// Process literals
			litLen := srcIdx - anchor

			// Emit literal length
			if litLen >= 7 {
				if litLen >= 1<<24 {
					return 0, 0, errors.New("Too many literals, skip")
				}

				this.tkBuf[tkIdx] = byte((7 << 5) | token)
				tkIdx++
				dstIdx += emitLengthLZ(dst[dstIdx:], litLen-7)
			} else {
				this.tkBuf[tkIdx] = byte((litLen << 5) | token)
				tkIdx++
			}

			// Emit literals
			emitLiteralsLZ(src[anchor:anchor+litLen], dst[dstIdx:])
			dstIdx += litLen
		}

		// Emit match length
		if mLen >= 15 {
			mIdx += emitLengthLZ(this.mBuf[mIdx:], mLen-15)
		}

		// Emit distance
		if maxDist == _LZX_MAX_DISTANCE2 && dist > 0xFFFF {
			this.mBuf[mIdx] = byte(dist >> 16)
			mIdx++
		}

		this.mBuf[mIdx] = byte(dist >> 8)
		this.mBuf[mIdx+1] = byte(dist)
		mIdx += 2

		if mIdx >= len(this.mBuf)-4 {
			// Expand match mBuf
			extraBuf := make([]byte, len(this.mBuf))
			this.mBuf = append(this.mBuf, extraBuf...)
		}

		// Fill this.hashes and update positions
		anchor = srcIdx + bestLen
		srcIdx++

		for srcIdx < anchor {
			this.hashes[this.hash(src[srcIdx:])] = int32(srcIdx)
			srcIdx++
		}
	}

	if dstIdx+tkIdx+mIdx > len(dst) {
		return 0, 0, errors.New("Output block too small")
	}

	// Emit last literals
	litLen := count - anchor

	if litLen >= 7 {
		this.tkBuf[tkIdx] = byte(7 << 5)
		tkIdx++
		dstIdx += emitLengthLZ(dst[dstIdx:], litLen-7)
	} else {
		this.tkBuf[tkIdx] = byte(litLen << 5)
		tkIdx++
	}

	copy(dst[dstIdx:], src[anchor:anchor+litLen])
	dstIdx += litLen

	// Emit buffers: literals + tokens + matches
	binary.LittleEndian.PutUint32(dst[0:], uint32(dstIdx))
	binary.LittleEndian.PutUint32(dst[4:], uint32(tkIdx))
	copy(dst[dstIdx:], this.tkBuf[0:tkIdx])
	dstIdx += tkIdx
	copy(dst[dstIdx:], this.mBuf[0:mIdx])
	dstIdx += mIdx

	var err error

	if dstIdx >= count {
		err = errors.New("No compression")
	}

	return uint(count), uint(dstIdx), err
}

func findMatch(src []byte, srcIdx, ref, maxMatch int) int {
	bestLen := 0

	if binary.LittleEndian.Uint32(src[srcIdx:]) == binary.LittleEndian.Uint32(src[ref:]) {
		bestLen = 4

		for bestLen+4 < maxMatch && binary.LittleEndian.Uint32(src[srcIdx+bestLen:]) == binary.LittleEndian.Uint32(src[ref+bestLen:]) {
			bestLen += 4
		}

		for bestLen < maxMatch && src[ref+bestLen] == src[srcIdx+bestLen] {
			bestLen++
		}
	}

	return bestLen
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZXCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	count := len(src)
	tkIdx := int(binary.LittleEndian.Uint32(src[0:]))
	mIdx := tkIdx + int(binary.LittleEndian.Uint32(src[4:]))

	if tkIdx > count || mIdx > count {
		return 0, 0, errors.New("LZCodec: inverse transform failed, invalid data")
	}

	srcEnd := tkIdx - 9
	dstEnd := len(dst) - 16
	maxDist := _LZX_MAX_DISTANCE2

	if src[8] == 0 {
		maxDist = _LZX_MAX_DISTANCE1
	}

	srcIdx := 9
	dstIdx := 0
	repd := 0

	for {
		token := int(src[tkIdx])
		tkIdx++

		if token >= 32 {
			// Get literal length
			litLen := token >> 5

			if litLen == 7 {
				ll, delta := readLengthLZ(src[srcIdx:])
				litLen += ll
				srcIdx += delta
			}

			// Emit literals
			emitLiteralsLZ(src[srcIdx:srcIdx+litLen], dst[dstIdx:])
			srcIdx += litLen
			dstIdx += litLen

			if dstIdx > dstEnd || srcIdx >= srcEnd {
				break
			}
		}

		// Get match length
		mLen := token & 0x0F

		if mLen == 15 {
			ll, delta := readLengthLZ(src[mIdx:])
			mLen += ll
			mIdx += delta
		}

		mLen += _LZX_MIN_MATCH
		mEnd := dstIdx + mLen

		// Sanity check
		if mEnd > dstEnd+16 {
			return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: invalid match length decoded: %d", mLen)
		}

		// Get distance
		d := (int(src[mIdx]) << 8) | int(src[mIdx+1])
		mIdx += 2

		if (token & 0x10) != 0 {
			if maxDist == _LZX_MAX_DISTANCE1 {
				d += 65536
			} else {
				d = (d << 8) | int(src[mIdx])
				mIdx++
			}
		}

		var dist int

		if d == 0 {
			dist = repd
		} else {
			dist = d - 1
			repd = dist
		}

		// Sanity check
		if dstIdx < dist || dist > maxDist {
			return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: invalid distance decoded: %d", dist)
		}

		ref := dstIdx - dist

		// Copy match
		if dist >= 16 {
			for {
				// No overlap
				copy(dst[dstIdx:], dst[ref:ref+16])
				ref += 16
				dstIdx += 16

				if dstIdx >= mEnd {
					break
				}
			}
		} else {
			for i := 0; i < mLen; i++ {
				dst[dstIdx+i] = dst[ref+i]
			}
		}

		dstIdx = mEnd
	}

	var err error

	if srcIdx != srcEnd+9 {
		err = errors.New("LZCodec: inverse transform failed")
	}

	return uint(mIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output mBuf
func (this LZXCodec) MaxEncodedLen(srcLen int) int {
	if srcLen <= 1024 {
		return srcLen + 16
	}

	return srcLen + srcLen/64
}

// LZPCodec an implementation of the Lempel Ziv Predict algorithm
type LZPCodec struct {
	hashes []int32
}

// NewLZPCodec creates a new instance of LZXCodec
func NewLZPCodec() (*LZPCodec, error) {
	this := &LZPCodec{}
	this.hashes = make([]int32, 0)
	return this, nil
}

// NewLZPCodecWithCtx creates a new instance of LZXCodec using a
// configuration map as parameter.
func NewLZPCodecWithCtx(ctx *map[string]interface{}) (*LZPCodec, error) {
	this := &LZPCodec{}
	this.hashes = make([]int32, 0)
	return this, nil
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZPCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output mBuf is too small - size: %d, required %d", len(dst), n)
	}

	// If too small, skip
	if count < _LZP_MIN_BLOCK_LENGTH {
		return 0, 0, fmt.Errorf("Block too small, skip")
	}

	srcEnd := count - 8
	dstEnd := len(dst) - 4

	if len(this.hashes) == 0 {
		this.hashes = make([]int32, 1<<_LZP_HASH_LOG)
	} else {
		for i := range this.hashes {
			this.hashes[i] = 0
		}
	}

	dst[0] = src[0]
	dst[1] = src[1]
	dst[2] = src[2]
	dst[3] = src[3]
	ctx := binary.LittleEndian.Uint32(src[:])
	srcIdx := 4
	dstIdx := 4
	minRef := 4

	for (srcIdx < srcEnd) && (dstIdx < dstEnd) {
		h := (_LZP_HASH_SEED * ctx) >> _LZP_HASH_SHIFT
		ref := int(this.hashes[h])
		this.hashes[h] = int32(srcIdx)
		bestLen := 0

		// Find a match
		if ref > minRef && binary.LittleEndian.Uint32(src[srcIdx:]) == binary.LittleEndian.Uint32(src[ref:]) {
			maxMatch := srcEnd - srcIdx
			bestLen = 4

			for bestLen < maxMatch && binary.LittleEndian.Uint32(src[srcIdx+bestLen:]) == binary.LittleEndian.Uint32(src[ref+bestLen:]) {
				bestLen += 4
			}

			for (bestLen < maxMatch) && (src[ref+bestLen] == src[srcIdx+bestLen]) {
				bestLen++
			}
		}

		// No good match ?
		if bestLen < _LZP_MIN_MATCH {
			val := uint32(src[srcIdx])
			ctx = (ctx << 8) | val
			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++

			if ref != 0 {
				if val == _LZP_MATCH_FLAG {
					dst[dstIdx] = byte(0xFF)
					dstIdx++
				}

				if minRef < bestLen {
					minRef = srcIdx + bestLen
				}
			}

			continue
		}

		srcIdx += bestLen
		ctx = binary.LittleEndian.Uint32(src[srcIdx-4:])
		dst[dstIdx] = _LZP_MATCH_FLAG
		dstIdx++
		bestLen -= _LZP_MIN_MATCH

		// Emit match length
		for bestLen >= 254 {
			bestLen -= 254
			dst[dstIdx] = 0xFE
			dstIdx++

			if dstIdx >= dstEnd {
				break
			}
		}

		dst[dstIdx] = byte(bestLen)
		dstIdx++
	}

	for (srcIdx < srcEnd+8) && (dstIdx < dstEnd) {
		h := (_LZP_HASH_SEED * ctx) >> _LZP_HASH_SHIFT
		ref := this.hashes[h]
		this.hashes[h] = int32(srcIdx)
		val := uint32(src[srcIdx])
		ctx = (ctx << 8) | val
		dst[dstIdx] = src[srcIdx]
		srcIdx++
		dstIdx++

		if (ref != 0) && (val == _LZP_MATCH_FLAG) && (dstIdx < dstEnd) {
			dst[dstIdx] = 0xFF
			dstIdx++
		}
	}

	var err error

	if (srcIdx != count) || (dstIdx >= count-(count>>6)) {
		err = errors.New("LZP forward transform failed: output mBuf too small")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZPCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if len(src) < 4 {
		return 0, 0, errors.New("Block too small, skip")
	}

	if len(this.hashes) == 0 {
		this.hashes = make([]int32, 1<<_LZP_HASH_LOG)
	} else {
		for i := range this.hashes {
			this.hashes[i] = 0
		}
	}

	srcEnd := len(src)
	dst[0] = src[0]
	dst[1] = src[1]
	dst[2] = src[2]
	dst[3] = src[3]
	ctx := binary.LittleEndian.Uint32(src[:])
	srcIdx := 4
	dstIdx := 4

	for srcIdx < srcEnd {
		h := (_LZP_HASH_SEED * ctx) >> _LZP_HASH_SHIFT
		ref := int(this.hashes[h])
		this.hashes[h] = int32(dstIdx)

		if ref == 0 || src[srcIdx] != _LZP_MATCH_FLAG {
			dst[dstIdx] = src[srcIdx]
			ctx = (ctx << 8) | uint32(dst[dstIdx])
			srcIdx++
			dstIdx++
			continue
		}

		srcIdx++

		if src[srcIdx] == 0xFF {
			dst[dstIdx] = _LZP_MATCH_FLAG
			ctx = (ctx << 8) | uint32(_LZP_MATCH_FLAG)
			srcIdx++
			dstIdx++
			continue
		}

		mLen := _LZP_MIN_MATCH

		for srcIdx < srcEnd && src[srcIdx] == 0xFE {
			srcIdx++
			mLen += 254
		}

		if srcIdx >= srcEnd {
			break
		}

		mLen += int(src[srcIdx])
		srcIdx++

		for i := 0; i < mLen; i++ {
			dst[dstIdx+i] = dst[ref+i]
		}

		dstIdx += mLen
		ctx = binary.LittleEndian.Uint32(dst[dstIdx-4:])
	}

	var err error

	if srcIdx != srcEnd {
		err = errors.New("LZP inverse transform failed: output mBuf too small")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output mBuf
func (this LZPCodec) MaxEncodedLen(srcLen int) int {
	if srcLen <= 1024 {
		return srcLen + 16
	}

	return srcLen + srcLen/64
}
