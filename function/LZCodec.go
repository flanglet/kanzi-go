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

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	_LZ_HASH_SEED      = 0x7FEB352D
	_LZX_HASH_LOG      = 18
	_LZX_HASH_SHIFT    = uint(32 - _LZX_HASH_LOG)
	_LZX_MAX_DISTANCE1 = (1 << 17) - 1
	_LZX_MAX_DISTANCE2 = (1 << 24) - 1
	_LZX_MIN_MATCH     = 4
	_LZX_MIN_LENGTH    = 16
	_LZP_HASH_LOG      = 16
	_LZP_HASH_SHIFT    = 32 - _LZP_HASH_LOG
	_LZP_MIN_MATCH     = 64
	_LZP_MIN_LENGTH    = 128
	_LZP_MATCH_FLAG    = 0xFC
)

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

// MaxEncodedLen returns the max size required for the encoding output buffer
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
		return 0, 0, errors.New("Input and output buffers cannot be equal")
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
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	return this.delegate.Inverse(src, dst)
}

// LZXCodec Simple byte oriented LZ77 implementation.
// It is a modified LZ4 with a bigger window, a bigger hash map, 3+n*8 bit
// literal lengths and 17 or 24 bit match lengths.
type LZXCodec struct {
	hashes []int32
}

// NewLZXCodec creates a new instance of LZXCodec
func NewLZXCodec() (*LZXCodec, error) {
	this := &LZXCodec{}
	this.hashes = make([]int32, 0)
	return this, nil
}

// NewLZXCodecWithCtx creates a new instance of LZXCodec using a
// configuration map as parameter.
func NewLZXCodecWithCtx(ctx *map[string]interface{}) (*LZXCodec, error) {
	this := &LZXCodec{}
	this.hashes = make([]int32, 0)
	return this, nil
}

func emitLength(buf []byte, length int) int {
	idx := 0

	for length >= 0xFF {
		buf[idx] = 0xFF
		idx++
		length -= 0xFF
	}

	buf[idx] = byte(length)
	return idx + 1
}

func emitLiterals(src, dst []byte) {
	length := len(src)

	for i := 0; i < length; i += 8 {
		copy(dst[i:], src[i:i+8])
	}
}

func emitLastLiterals(src, dst []byte) int {
	dstIdx := 1
	litLen := len(src)

	if litLen >= 7 {
		dst[0] = byte(7 << 5)
		dstIdx += emitLength(dst[1:], litLen-7)
	} else {
		dst[0] = byte(litLen << 5)
	}

	copy(dst[dstIdx:], src[0:litLen])
	return dstIdx + litLen
}

func lzhash(p []byte) uint32 {
	return (binary.LittleEndian.Uint32(p) * _LZ_HASH_SEED) >> _LZX_HASH_SHIFT
}

// Forward applies the function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZXCodec) Forward(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	// If too small, skip
	if count < _LZX_MIN_LENGTH {
		return 0, 0, fmt.Errorf("Block too small, skip")
	}

	srcEnd := count - 8

	if len(this.hashes) == 0 {
		this.hashes = make([]int32, 1<<_LZX_HASH_LOG)
	} else {
		for i := range this.hashes {
			this.hashes[i] = 0
		}
	}

	maxDist := _LZX_MAX_DISTANCE2
	dst[0] = 1

	if srcEnd < 4*_LZX_MAX_DISTANCE1 {
		maxDist = _LZX_MAX_DISTANCE1
		dst[0] = 0
	}

	srcIdx := 0
	dstIdx := 1
	anchor := 0

	for srcIdx < srcEnd {
		var minRef int

		if srcIdx < maxDist {
			minRef = 0
		} else {
			minRef = srcIdx - maxDist
		}

		h := lzhash(src[srcIdx:])
		ref := int(this.hashes[h])
		bestLen := 0

		// Find a match
		if ref > minRef && binary.LittleEndian.Uint32(src[srcIdx:]) == binary.LittleEndian.Uint32(src[ref:]) {
			maxMatch := srcEnd - srcIdx
			bestLen = 4

			for bestLen+4 < maxMatch && binary.LittleEndian.Uint32(src[srcIdx+bestLen:]) == binary.LittleEndian.Uint32(src[ref+bestLen:]) {
				bestLen += 4
			}

			for bestLen < maxMatch && src[ref+bestLen] == src[srcIdx+bestLen] {
				bestLen++
			}
		}

		// No good match ?
		if bestLen < _LZX_MIN_MATCH {
			this.hashes[h] = int32(srcIdx)
			srcIdx++
			continue
		}

		// Emit token
		// Token: 3 bits litLen + 1 bit flag + 4 bits mLen
		// flag = if maxDist = (1<<17)-1, then highest bit of distance
		//        else 1 if dist needs 3 bytes (> 0xFFFF) and 0 otherwise
		mLen := bestLen - _LZX_MIN_MATCH
		dist := srcIdx - ref
		var token int

		if dist > 0xFFFF {
			token = 0x10
		} else {
			token = 0
		}

		if mLen < 15 {
			token += mLen
		} else {
			token += 0x0F
		}

		// Literals to process ?
		if anchor == srcIdx {
			dst[dstIdx] = byte(token)
			dstIdx++
		} else {
			// Process match
			litLen := srcIdx - anchor

			// Emit literal length
			if litLen >= 7 {
				dst[dstIdx] = byte((7 << 5) | token)
				dstIdx++
				dstIdx += emitLength(dst[dstIdx:], litLen-7)
			} else {
				dst[dstIdx] = byte((litLen << 5) | token)
				dstIdx++
			}

			// Emit literals
			emitLiterals(src[anchor:anchor+litLen], dst[dstIdx:])
			dstIdx += litLen
		}

		// Emit match length
		if mLen >= 0x0F {
			dstIdx += emitLength(dst[dstIdx:], mLen-0x0F)
		}

		// Emit distance
		if maxDist == _LZX_MAX_DISTANCE2 && dist > 0xFFFF {
			dst[dstIdx] = byte(dist >> 16)
			dstIdx++
		}

		dst[dstIdx] = byte(dist >> 8)
		dstIdx++
		dst[dstIdx] = byte(dist)
		dstIdx++

		// Fill _hashes and update positions
		anchor = srcIdx + bestLen
		this.hashes[h] = int32(srcIdx)
		srcIdx++

		for srcIdx < anchor {
			this.hashes[lzhash(src[srcIdx:])] = int32(srcIdx)
			srcIdx++
		}
	}

	// Emit last literals
	dstIdx += emitLastLiterals(src[anchor:srcEnd+8], dst[dstIdx:])
	return uint(srcEnd + 8), uint(dstIdx), nil
}

// Inverse applies the reverse function to the src and writes the result
// to the destination. Returns number of bytes read, number of bytes
// written and possibly an error.
func (this *LZXCodec) Inverse(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)
	srcEnd := count - 8
	dstEnd := len(dst) - 8
	dstIdx := 0
	maxDist := _LZX_MAX_DISTANCE2

	if src[0] == 0 {
		maxDist = _LZX_MAX_DISTANCE1
	}

	srcIdx := 1

	for {
		token := int(src[srcIdx])
		srcIdx++

		if token >= 32 {
			// Get literal length
			litLen := token >> 5

			if litLen == 7 {
				for srcIdx < srcEnd && src[srcIdx] == 0xFF {
					srcIdx++
					litLen += 0xFF
				}

				if srcIdx >= srcEnd+8 {
					return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: Invalid literals length decoded: %d", litLen)
				}

				litLen += int(src[srcIdx])
				srcIdx++
			}

			// Copy literals and exit ?
			if dstIdx+litLen > dstEnd || srcIdx+litLen > srcEnd {
				copy(dst[dstIdx:], src[srcIdx:srcIdx+litLen])
				srcIdx += litLen
				dstIdx += litLen
				break
			}

			// Emit literals
			emitLiterals(src[srcIdx:srcIdx+litLen], dst[dstIdx:])
			srcIdx += litLen
			dstIdx += litLen
		}

		// Get match length
		mLen := token & 0x0F

		if mLen == 15 {
			for srcIdx < srcEnd && src[srcIdx] == 0xFF {
				srcIdx++
				mLen += 0xFF
			}

			if srcIdx < srcEnd {
				mLen += int(src[srcIdx])
				srcIdx++
			}
		}

		mLen += _LZX_MIN_MATCH
		mEnd := dstIdx + mLen

		// Sanity check
		if mEnd > dstEnd+8 {
			return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: Invalid match length decoded: %d", mLen)
		}

		// Get distance
		dist := (int(src[srcIdx]) << 8) | int(src[srcIdx+1])
		srcIdx += 2

		if (token & 0x10) != 0 {
			if maxDist == _LZX_MAX_DISTANCE1 {
				dist += 65536
			} else {
				dist = (dist << 8) | int(src[srcIdx])
				srcIdx++
			}
		}

		// Sanity check
		if dstIdx < dist || dist > maxDist {
			return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: Invalid distance decoded: %d", dist)
		}

		// Copy match
		if dist > 8 {
			ref := dstIdx - dist

			for {
				// No overlap
				copy(dst[dstIdx:], dst[ref:ref+8])
				ref += 8
				dstIdx += 8

				if dstIdx >= mEnd {
					break
				}
			}
		} else {
			ref := dstIdx - dist

			for i := 0; i < mLen; i++ {
				dst[dstIdx+i] = dst[ref+i]
			}
		}

		dstIdx = mEnd
	}

	return uint(srcIdx), uint(dstIdx), nil
}

// MaxEncodedLen returns the max size required for the encoding output buffer
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

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if n := this.MaxEncodedLen(count); len(dst) < n {
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	// If too small, skip
	if count < _LZP_MIN_LENGTH {
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
		h := (_LZ_HASH_SEED * ctx) >> _LZP_HASH_SHIFT
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
		h := (_LZ_HASH_SEED * ctx) >> _LZP_HASH_SHIFT
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

	if &src[0] == &dst[0] {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
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

	count := len(src)
	srcEnd := count
	dst[0] = src[0]
	dst[1] = src[1]
	dst[2] = src[2]
	dst[3] = src[3]
	ctx := binary.LittleEndian.Uint32(src[:])
	srcIdx := 4
	dstIdx := 4

	for srcIdx < srcEnd {
		h := (_LZ_HASH_SEED * ctx) >> _LZP_HASH_SHIFT
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
		err = errors.New("LZP inverse transform failed: output buffer too small")
	}

	return uint(srcIdx), uint(dstIdx), err
}

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this LZPCodec) MaxEncodedLen(srcLen int) int {
	if srcLen <= 1024 {
		return srcLen + 16
	}

	return srcLen + srcLen/64
}
