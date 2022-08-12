/*
Copyright 2011-2022 Frederic Langlet
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

	kanzi "github.com/flanglet/kanzi-go/v2"
)

const (
	_LZX_HASH_SEED          = 0x1E35A7BD
	_LZX_HASH_LOG1          = 17
	_LZX_HASH_SHIFT1        = 40 - _LZX_HASH_LOG1
	_LZX_HASH_MASK1         = (1 << _LZX_HASH_LOG1) - 1
	_LZX_HASH_LOG2          = 21
	_LZX_HASH_SHIFT2        = 48 - _LZX_HASH_LOG2
	_LZX_HASH_MASK2         = (1 << _LZX_HASH_LOG2) - 1
	_LZX_MAX_DISTANCE1      = (1 << 17) - 2
	_LZX_MAX_DISTANCE2      = (1 << 24) - 2
	_LZX_MIN_MATCH1         = 5
	_LZX_MIN_MATCH2         = 9
	_LZX_MAX_MATCH          = 65535 + 254 + 15 + _LZX_MIN_MATCH1
	_LZX_MIN_BLOCK_LENGTH   = 24
	_LZX_MIN_MATCH_MIN_DIST = 1 << 16
	_LZP_HASH_SEED          = 0x7FEB352D
	_LZP_HASH_LOG           = 16
	_LZP_HASH_SHIFT         = 32 - _LZP_HASH_LOG
	_LZP_MIN_MATCH          = 96
	_LZP_MATCH_FLAG         = 0xFC
	_LZP_MIN_BLOCK_LENGTH   = 128
)

// LZCodec encapsulates an implementation of a Lempel-Ziv codec
type LZCodec struct {
	delegate kanzi.ByteTransform
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
	var d kanzi.ByteTransform

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
	hashes       []int32
	mLenBuf      []byte
	mBuf         []byte
	tkBuf        []byte
	extra        bool
	ctx          *map[string]interface{}
	isBsVersion2 bool
}

// NewLZXCodec creates a new instance of LZXCodec
func NewLZXCodec() (*LZXCodec, error) {
	this := &LZXCodec{}
	this.hashes = make([]int32, 0)
	this.mLenBuf = make([]byte, 0)
	this.mBuf = make([]byte, 0)
	this.tkBuf = make([]byte, 0)
	this.extra = false
	this.isBsVersion2 = false // old encoding
	return this, nil
}

// NewLZXCodecWithCtx creates a new instance of LZXCodec using a
// configuration map as parameter.
func NewLZXCodecWithCtx(ctx *map[string]interface{}) (*LZXCodec, error) {
	this := &LZXCodec{}
	this.hashes = make([]int32, 0)
	this.mLenBuf = make([]byte, 0)
	this.mBuf = make([]byte, 0)
	this.tkBuf = make([]byte, 0)
	this.extra = false
	this.ctx = ctx
	bsVersion := uint(3)

	if ctx != nil {
		if val, containsKey := (*ctx)["lz"]; containsKey {
			lzType := val.(uint64)
			this.extra = lzType == LZX_TYPE
		}

		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			bsVersion = val.(uint)
		}
	}

	this.isBsVersion2 = bsVersion < 3
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
	for i := 0; i < len(src); i += 8 {
		copy(dst[i:], src[i:i+8])
	}
}

func (this *LZXCodec) hash(p []byte) uint32 {
	if this.extra == true {
		return uint32((binary.LittleEndian.Uint64(p)*_LZX_HASH_SEED)>>_LZX_HASH_SHIFT2) & _LZX_HASH_MASK2
	}

	return uint32((binary.LittleEndian.Uint64(p)*_LZX_HASH_SEED)>>_LZX_HASH_SHIFT1) & _LZX_HASH_MASK1
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

	if len(this.mLenBuf) < minBufSize {
		this.mLenBuf = make([]byte, minBufSize)
	}

	if len(this.mBuf) < minBufSize {
		this.mBuf = make([]byte, minBufSize)
	}

	if len(this.tkBuf) < minBufSize {
		this.tkBuf = make([]byte, minBufSize)
	}

	srcEnd := count - 16 - 1
	maxDist := _LZX_MAX_DISTANCE2
	dThreshold := 1 << 16
	dst[12] = 1

	if srcEnd < 4*_LZX_MAX_DISTANCE1 {
		maxDist = _LZX_MAX_DISTANCE1
		dThreshold = _LZX_MAX_DISTANCE1 + 1
		dst[12] = 0
	}

	minMatch := _LZX_MIN_MATCH1

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt := val.(kanzi.DataType)

			if dt == kanzi.DT_DNA {
				// Longer min match for DNA input
				minMatch = _LZX_MIN_MATCH2
				dst[12] |= 2
			}
		}
	}

	srcIdx := 0
	dstIdx := 13
	anchor := 0
	mLenIdx := 0
	mIdx := 0
	tkIdx := 0
	repd0 := len(src)
	repd1 := 0

	for srcIdx < srcEnd {
		var minRef int

		if srcIdx < maxDist {
			minRef = 0
		} else {
			minRef = srcIdx - maxDist
		}

		h0 := this.hash(src[srcIdx:])
		ref := srcIdx + 1 - repd0
		bestLen := 0

		if ref > minRef {
			// Check repd0 first
			if binary.LittleEndian.Uint32(src[srcIdx+1:]) == binary.LittleEndian.Uint32(src[ref:]) {
				maxMatch := srcEnd - srcIdx - 5

				if maxMatch > _LZX_MAX_MATCH {
					maxMatch = _LZX_MAX_MATCH
				}

				bestLen = 4 + findMatchLZX(src, srcIdx+5, ref+4, maxMatch)
			}
		}

		if bestLen < minMatch {
			ref = int(this.hashes[h0])
			this.hashes[h0] = int32(srcIdx)

			if ref <= minRef {
				srcIdx++
				continue
			}

			if binary.LittleEndian.Uint32(src[srcIdx:]) == binary.LittleEndian.Uint32(src[ref:]) {
				maxMatch := srcEnd - srcIdx - 4

				if maxMatch > _LZX_MAX_MATCH {
					maxMatch = _LZX_MAX_MATCH
				}

				bestLen = 4 + findMatchLZX(src, srcIdx+4, ref+4, maxMatch)
			}
		} else {
			srcIdx++
			this.hashes[h0] = int32(srcIdx)
		}

		// No good match ?
		if (bestLen < minMatch) || (bestLen == minMatch && srcIdx-ref >= _LZX_MIN_MATCH_MIN_DIST && srcIdx-ref != repd0) {
			srcIdx++
			continue
		}

		if ref != srcIdx-repd0 {
			// Check if better match at next position
			h1 := this.hash(src[srcIdx+1:])
			ref1 := int(this.hashes[h1])
			this.hashes[h1] = int32(srcIdx + 1)

			// Find a match
			if ref1 > minRef+1 {
				maxMatch := srcEnd - srcIdx - 1

				if maxMatch > _LZX_MAX_MATCH {
					maxMatch = _LZX_MAX_MATCH
				}

				bestLen1 := findMatchLZX(src, srcIdx+1, ref1, maxMatch)

				// Select best match
				if (bestLen1 > bestLen) || ((bestLen1 == bestLen) && (srcIdx+1-ref1 < srcIdx-ref)) {
					ref = ref1
					bestLen = bestLen1
					srcIdx++
				}
			}
		}

		d := srcIdx - ref
		var dist int

		if d == repd0 {
			dist = 0
		} else {
			if d == repd1 {
				dist = 1
			} else {
				dist = d + 1
			}

			repd1 = repd0
			repd0 = d
		}

		// Emit token
		// Token: 3 bits litLen + 1 bit flag + 4 bits mLen (LLLFMMMM)
		// flag = if maxDist = _LZX_MAX_DISTANCE1, then highest bit of distance
		//        else 1 if dist needs 3 bytes (> 0xFFFF) and 0 otherwise
		mLen := bestLen - minMatch
		var token int

		if dist > 65535 {
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
			mLenIdx += emitLengthLZ(this.mLenBuf[mLenIdx:], mLen-15)
		}

		// Emit distance
		if dist >= dThreshold {
			this.mBuf[mIdx] = byte(dist >> 16)
			mIdx++
		}

		this.mBuf[mIdx] = byte(dist >> 8)
		this.mBuf[mIdx+1] = byte(dist)
		mIdx += 2

		if mIdx >= len(this.mBuf)-8 {
			// Expand match mBuf
			extraBuf1 := make([]byte, len(this.mBuf))
			this.mBuf = append(this.mBuf, extraBuf1...)

			if mLenIdx >= len(this.mLenBuf)-8 {
				extraBuf2 := make([]byte, len(this.mLenBuf))
				this.mLenBuf = append(this.mBuf, extraBuf2...)
			}
		}

		// Fill this.hashes and update positions
		anchor = srcIdx + bestLen
		srcIdx++

		for srcIdx < anchor {
			this.hashes[this.hash(src[srcIdx:])] = int32(srcIdx)
			srcIdx++
		}
	}

	// Emit last literals
	litLen := count - anchor

	if dstIdx+litLen+tkIdx+mIdx >= count {
		return uint(count), uint(dstIdx), errors.New("No compression")
	}

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
	binary.LittleEndian.PutUint32(dst[8:], uint32(mIdx))
	copy(dst[dstIdx:], this.tkBuf[0:tkIdx])
	dstIdx += tkIdx
	copy(dst[dstIdx:], this.mBuf[0:mIdx])
	dstIdx += mIdx
	copy(dst[dstIdx:], this.mLenBuf[0:mLenIdx])
	dstIdx += mLenIdx
	return uint(count), uint(dstIdx), nil
}

func findMatchLZX(src []byte, srcIdx, ref, maxMatch int) int {
	bestLen := 0

	for bestLen+4 <= maxMatch {
		diff := binary.LittleEndian.Uint32(src[srcIdx+bestLen:]) ^ binary.LittleEndian.Uint32(src[ref+bestLen:])

		if diff != 0 {
			bestLen += (bits.TrailingZeros32(diff) >> 3)
			break
		}

		bestLen += 4
	}

	return bestLen
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZXCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if this.isBsVersion2 == true {
		return this.inverseV2(src, dst)
	}

	return this.inverseV3(src, dst)
}

func (this *LZXCodec) inverseV3(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	count := len(src)

	if count < 13 {
		return 0, 0, errors.New("LZCodec: inverse transform failed, invalid data")
	}

	tkIdx := int(binary.LittleEndian.Uint32(src[0:]))
	mIdx := tkIdx + int(binary.LittleEndian.Uint32(src[4:]))
	mLenIdx := mIdx + int(binary.LittleEndian.Uint32(src[8:]))

	if mLenIdx > count {
		return 0, 0, errors.New("LZCodec: inverse transform failed, invalid data")
	}

	srcEnd := tkIdx - 13
	dstEnd := len(dst) - 16
	maxDist := _LZX_MAX_DISTANCE2

	if src[12]&1 == 0 {
		maxDist = _LZX_MAX_DISTANCE1
	}

	minMatch := _LZX_MIN_MATCH1

	if src[12]&2 != 0 {
		minMatch = _LZX_MIN_MATCH2
	}

	srcIdx := 13
	dstIdx := 0
	repd0 := 0
	repd1 := 0

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
			if dstIdx+litLen >= dstEnd {
				copy(dst[dstIdx:], src[srcIdx:srcIdx+litLen])
			} else {
				emitLiteralsLZ(src[srcIdx:srcIdx+litLen], dst[dstIdx:])
			}

			srcIdx += litLen
			dstIdx += litLen

			if srcIdx >= srcEnd {
				break
			}
		}

		// Get match length
		mLen := token & 0x0F

		if mLen == 15 {
			ll, delta := readLengthLZ(src[mLenIdx:])
			mLen += ll
			mLenIdx += delta
		}

		mLen += minMatch
		mEnd := dstIdx + mLen

		// Get distance
		dist := (int(src[mIdx]) << 8) | int(src[mIdx+1])
		mIdx += 2

		if (token & 0x10) != 0 {
			if maxDist == _LZX_MAX_DISTANCE1 {
				dist += 65536
			} else {
				dist = (dist << 8) | int(src[mIdx])
				mIdx++
			}
		}

		if dist == 0 {
			dist = repd0
		} else {
			if dist == 1 {
				dist = repd1
			} else {
				dist--
			}

			repd1 = repd0
			repd0 = dist
		}

		// Sanity check
		if dstIdx < dist || dist > maxDist || mEnd > dstEnd+16 {
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

	if srcIdx != srcEnd+13 {
		err = errors.New("LZCodec: inverse transform failed")
	}

	return uint(mIdx), uint(dstIdx), err
}

func (this *LZXCodec) inverseV2(src, dst []byte) (uint, uint, error) {
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
			if dstIdx+litLen >= dstEnd {
				copy(dst[dstIdx:], src[srcIdx:srcIdx+litLen])
			} else {
				emitLiteralsLZ(src[srcIdx:srcIdx+litLen], dst[dstIdx:])
			}

			srcIdx += litLen
			dstIdx += litLen

			if srcIdx >= srcEnd {
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

		mLen += 5
		mEnd := dstIdx + mLen

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
		if dstIdx < dist || dist > maxDist || mEnd > dstEnd+16 {
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

	srcEnd := count
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

	for (srcIdx < srcEnd-_LZP_MIN_MATCH) && (dstIdx < dstEnd) {
		h := (_LZP_HASH_SEED * ctx) >> _LZP_HASH_SHIFT
		ref := int(this.hashes[h])
		this.hashes[h] = int32(srcIdx)
		bestLen := 0

		// Find a match
		if ref > minRef && binary.LittleEndian.Uint32(src[srcIdx+_LZP_MIN_MATCH-4:]) == binary.LittleEndian.Uint32(src[ref+_LZP_MIN_MATCH-4:]) {
			bestLen = this.findMatch(src, srcIdx, ref, srcEnd-srcIdx)
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

	for (srcIdx < srcEnd) && (dstIdx < dstEnd) {
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
		err = errors.New("LZP forward transform failed: output buffer too small")
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
	ctx := binary.LittleEndian.Uint32(dst[:])
	srcIdx := 4
	dstIdx := 4
	res := true

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
			res = false
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

	if res == false || (srcIdx != srcEnd) {
		err = errors.New("LZP inverse transform failed: output buffer too small")
	}

	return uint(srcIdx), uint(dstIdx), err
}

func (this *LZPCodec) findMatch(src []byte, srcIdx, ref, maxMatch int) int {
	bestLen := 0

	for bestLen+8 <= maxMatch {
		diff := binary.LittleEndian.Uint64(src[srcIdx+bestLen:]) ^ binary.LittleEndian.Uint64(src[ref+bestLen:])

		if diff != 0 {
			bestLen += (bits.TrailingZeros64(diff) >> 3)
			break
		}

		bestLen += 8
	}

	return bestLen
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this LZPCodec) MaxEncodedLen(srcLen int) int {
	if srcLen <= 1024 {
		return srcLen + 16
	}

	return srcLen + srcLen/64
}
