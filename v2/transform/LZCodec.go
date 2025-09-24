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

	kanzi "github.com/flanglet/kanzi-go/v2"
	internal "github.com/flanglet/kanzi-go/v2/internal"
)

const (
	_LZX_HASH_SEED        = 0x1E35A7BD
	_LZX_HASH_LOG1        = 15
	_LZX_HASH_RSHIFT1     = 64 - _LZX_HASH_LOG1
	_LZX_HASH_LSHIFT1     = 24
	_LZX_HASH_LOG2        = 19
	_LZX_HASH_RSHIFT2     = 64 - _LZX_HASH_LOG2
	_LZX_HASH_LSHIFT2     = 24
	_LZX_MAX_DISTANCE1    = (1 << 16) - 2
	_LZX_MAX_DISTANCE2    = (1 << 24) - 2
	_LZX_MIN_MATCH4       = 4
	_LZX_MIN_MATCH6       = 6
	_LZX_MIN_MATCH9       = 9
	_LZX_MAX_MATCH        = 65535 + 254 + _LZX_MIN_MATCH4
	_LZX_MIN_BLOCK_LENGTH = 24
	_LZP_HASH_SEED        = 0x7FEB352D
	_LZP_HASH_LOG         = 16
	_LZP_HASH_SHIFT       = 32 - _LZP_HASH_LOG
	_LZP_MIN_MATCH96      = 96
	_LZP_MIN_MATCH64      = 64
	_LZP_MATCH_FLAG       = 0xFC
	_LZP_MIN_BLOCK_LENGTH = 128
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

// MaxEncodedLen returns the max size required for the encoding output buffer
func (this *LZCodec) MaxEncodedLen(srcLen int) int {
	return this.delegate.MaxEncodedLen(srcLen)
}

// NewLZCodecWithCtx creates a new instance of LZCodec using a
// configuration map as parameter.
func NewLZCodecWithCtx(ctx *map[string]any) (*LZCodec, error) {
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
// It is a based on a heavily modified LZ4 with a bigger window, a bigger
// hash map, 3+n*8 bit literal lengths and 17 or 24 bit match lengths.
type LZXCodec struct {
	hashes    []int32
	mLenBuf   []byte
	mBuf      []byte
	tkBuf     []byte
	extra     bool
	ctx       *map[string]any
	bsVersion uint
}

// NewLZXCodec creates a new instance of LZXCodec
func NewLZXCodec() (*LZXCodec, error) {
	this := &LZXCodec{}
	this.hashes = make([]int32, 0)
	this.mLenBuf = make([]byte, 0)
	this.mBuf = make([]byte, 0)
	this.tkBuf = make([]byte, 0)
	this.extra = false
	this.bsVersion = 6
	return this, nil
}

// NewLZXCodecWithCtx creates a new instance of LZXCodec using a
// configuration map as parameter.
func NewLZXCodecWithCtx(ctx *map[string]any) (*LZXCodec, error) {
	this := &LZXCodec{}
	this.hashes = make([]int32, 0)
	this.mLenBuf = make([]byte, 0)
	this.mBuf = make([]byte, 0)
	this.tkBuf = make([]byte, 0)
	this.extra = false
	this.ctx = ctx
	this.bsVersion = uint(6)

	if ctx != nil {
		if val, containsKey := (*ctx)["lz"]; containsKey {
			lzType := val.(uint64)
			this.extra = lzType == LZX_TYPE
		}

		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			this.bsVersion = val.(uint)
		}
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

	if res < 254 {
		return res, 1
	}

	if res == 254 {
		res += (int(block[1]) << 8)
		res += int(block[2])
		return res, 3
	}

	res += (int(block[1]) << 16)
	res += (int(block[2]) << 8)
	res += int(block[3])
	return res, 4
}

func emitLiteralsLZ(src, dst []byte) {
	copy(dst, src)
}

func (this *LZXCodec) hash(p []byte) uint32 {
	if this.extra == true {
		return uint32(((binary.LittleEndian.Uint64(p) << _LZX_HASH_LSHIFT2) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT2)
	}

	return uint32(((binary.LittleEndian.Uint64(p) << _LZX_HASH_LSHIFT1) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT1)
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
		return 0, 0, fmt.Errorf("LZCodec forward transform skip: output buffer is too small - size: %d, required %d", len(dst), n)
	}

	// If too small, skip
	if count < _LZX_MIN_BLOCK_LENGTH {
		return 0, 0, errors.New("LZCodec forward transform skip: block too small, skip")
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

	minBufSize := max(count/5, 256)

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
	dst[12] = 1

	if srcEnd < 4*_LZX_MAX_DISTANCE1 {
		maxDist = _LZX_MAX_DISTANCE1
		dst[12] = 0
	}

	minMatch := _LZX_MIN_MATCH4

	if this.ctx != nil {
		if val, containsKey := (*this.ctx)["dataType"]; containsKey {
			dt := val.(internal.DataType)

			if dt == internal.DT_DNA {
				// Longer min match for DNA input
				minMatch = _LZX_MIN_MATCH6
			} else if dt == internal.DT_SMALL_ALPHABET {
				return 0, 0, errors.New("LZCodec forward transform skip: Small alphabet")
			}
		}
	}

	// dst[12] = 0000MMMD (4 bits + 3 bits minMatch + 1 bit max distance)
	dst[12] |= byte(((minMatch - 2) & 0x07) << 1) // minMatch in [2..9]
	srcIdx := 0
	dstIdx := 13
	anchor := 0
	mLenIdx := 0
	mIdx := 0
	tkIdx := 0
	var repd = []int{count, count}
	repdIdx := 0
	srcInc := 0

	for srcIdx < srcEnd {
		bestLen := 0
		p := binary.LittleEndian.Uint64(src[srcIdx:])
		srcIdx1 := srcIdx + 1
		maxMatch := min(srcEnd-srcIdx1, _LZX_MAX_MATCH)
		ref := srcIdx1 - repd[repdIdx]
		minRef := max(srcIdx-maxDist, 0)

		// Check repd first
		if ref > minRef && uint32(p>>8) == binary.LittleEndian.Uint32(src[ref:]) {
			bestLen = findMatchLZX(src, srcIdx1, ref, maxMatch)
		}

		if bestLen < minMatch {
			h0 := this.hash(src[srcIdx:])
			ref = int(this.hashes[h0])
			this.hashes[h0] = int32(srcIdx)

			if ref > minRef && uint32(p) == binary.LittleEndian.Uint32(src[ref:]) {
				bestLen = findMatchLZX(src, srcIdx, ref, min(srcEnd-srcIdx, _LZX_MAX_MATCH))
			}

			// No good match ?
			if bestLen < minMatch {
				srcIdx++
				srcIdx += (srcInc >> 6)
				srcInc++
				repdIdx = 0
				continue
			}

			if ref != srcIdx-repd[0] && ref != srcIdx-repd[1] {
				// Check if better match at next position
				srcIdx1 := srcIdx + 1
				h1 := this.hash(src[srcIdx1:])
				ref1 := int(this.hashes[h1])
				this.hashes[h1] = int32(srcIdx1)

				// Find a match
				if ref1 > minRef+1 && binary.LittleEndian.Uint32(src[srcIdx1+bestLen-3:]) == binary.LittleEndian.Uint32(src[ref1+bestLen-3:]) {
					bestLen1 := findMatchLZX(src, srcIdx1, ref1, min(srcEnd-srcIdx1, _LZX_MAX_MATCH))

					// Select best match
					if bestLen1 >= bestLen {
						ref = ref1
						bestLen = bestLen1
						srcIdx = srcIdx1
					}
				}
			}

                        if this.extra == true {
                                // Check if better match at position+2
                                srcIdx2 := srcIdx + 2
                                h2 := this.hash(src[srcIdx2:])
                                ref2 := int(this.hashes[h2])
                                this.hashes[h2] = int32(srcIdx2)

                                // Find a match
                                if ref2 > minRef+2 && binary.LittleEndian.Uint32(src[srcIdx2+bestLen-3:]) == binary.LittleEndian.Uint32(src[ref2+bestLen-3:]) {
                                        bestLen2 := findMatchLZX(src, srcIdx2, ref2, min(srcEnd-srcIdx2, _LZX_MAX_MATCH))

                                        // Select best match
                                        if bestLen2 >= bestLen {
                                                ref = ref2
                                                bestLen = bestLen2
                                                srcIdx = srcIdx2
                                        }
                                }
                        }

                        // Extend backwards
                        for (srcIdx > anchor) && (ref > minRef) && (src[srcIdx-1] == src[ref-1]) {
                                bestLen++
                                ref--
                                srcIdx--
                        }

                        if bestLen > _LZX_MAX_MATCH {
                                srcIdx += (bestLen - _LZX_MAX_MATCH)
                                ref += (bestLen - _LZX_MAX_MATCH)
                                bestLen = _LZX_MAX_MATCH
                        }
		} else {
			h0 := this.hash(src[srcIdx:])
			this.hashes[h0] = int32(srcIdx)

			if src[srcIdx] == src[ref-1] && bestLen < _LZX_MAX_MATCH {
				bestLen++
				ref--
			} else {
				srcIdx++
				h1 := this.hash(src[srcIdx:])
				this.hashes[h1] = int32(srcIdx)
			}
		}

		// Emit match
		srcInc = 0

		// Token: 3 bits litLen + 2 bits flag + 3 bits mLen (LLLFFMMM)
		//    or  3 bits litLen + 3 bits flag + 2 bits mLen (LLLFFFMM)
		// LLL : <= 7 --> LLL == literal length (if 7, remainder encoded outside of token)
		// MMM : <= 7 --> MMM == match length (if 7, remainder encoded outside of token)
		// MM  : <= 3 --> MM  == match length (if 3, remainder encoded outside of token)
		// FF = 01    --> 1 byte dist
		// FF = 10    --> 2 byte dist
		// FF = 11    --> 3 byte dist
		// FFF = 000  --> dist == repd0
		// FFF = 001  --> dist == repd1
		dist := srcIdx - ref
		mLen := bestLen - minMatch
		var token, mLenTh int

		if dist == repd[0] {
			token = 0x00
			mLenTh = 3
		} else if dist == repd[1] {
			token = 0x04
			mLenTh = 3
		} else {
			mLenTh = 7

			// Emit distance since not a repeat
			if dist >= 256 {
				if dist >= 65536 {
					this.mBuf[mIdx] = byte(dist >> 16)
					this.mBuf[mIdx+1] = byte(dist >> 8)
					mIdx += 2
					token = 0x18
				} else {
					this.mBuf[mIdx] = byte(dist >> 8)
					mIdx++
					token = 0x10
				}
			} else {
				token = 0x08
			}

			this.mBuf[mIdx] = byte(dist)
			mIdx++
		}

		// Emit match length
		if mLen >= mLenTh {
			token += mLenTh
			mLenIdx += emitLengthLZ(this.mLenBuf[mLenIdx:], mLen-mLenTh)
		} else {
			token += mLen
		}

		repd[1] = repd[0]
		repd[0] = dist
		repdIdx = 1
		litLen := srcIdx - anchor

		// Emit token
		// Literals to process ?
		if litLen == 0 {
			this.tkBuf[tkIdx] = byte(token)
			tkIdx++
		} else {
			// Emit literal length
			if litLen >= 7 {
				if litLen >= 1<<24 {
					return 0, 0, errors.New("LZCodec forward transform skip: too many literals")
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

		if mIdx >= len(this.mBuf)-8 {
			extraBuf1 := make([]byte, len(this.mBuf)/2)
			this.mBuf = append(this.mBuf, extraBuf1...)

			if mLenIdx >= len(this.mLenBuf)-8 {
				extraBuf2 := make([]byte, len(this.mLenBuf)/2)
				this.mLenBuf = append(this.mLenBuf, extraBuf2...)
			}
		}

		// Fill this.hashes and update positions
		anchor = srcIdx + bestLen
		srcIdx++

		if this.extra == true {
			for srcIdx+4 < anchor {
				v := binary.LittleEndian.Uint64(src[srcIdx:])
				h0 := uint32((((v >> 0) << _LZX_HASH_LSHIFT2) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT2)
				h1 := uint32((((v >> 8) << _LZX_HASH_LSHIFT2) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT2)
				h2 := uint32((((v >> 16) << _LZX_HASH_LSHIFT2) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT2)
				h3 := uint32((((v >> 24) << _LZX_HASH_LSHIFT2) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT2)
				this.hashes[h0] = int32(srcIdx + 0)
				this.hashes[h1] = int32(srcIdx + 1)
				this.hashes[h2] = int32(srcIdx + 2)
				this.hashes[h3] = int32(srcIdx + 3)
				srcIdx += 4
			}
		} else {
			for srcIdx+4 < anchor {
				v := binary.LittleEndian.Uint64(src[srcIdx:])
				h0 := uint32((((v >> 0) << _LZX_HASH_LSHIFT1) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT1)
				h1 := uint32((((v >> 8) << _LZX_HASH_LSHIFT1) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT1)
				h2 := uint32((((v >> 16) << _LZX_HASH_LSHIFT1) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT1)
				h3 := uint32((((v >> 24) << _LZX_HASH_LSHIFT1) * _LZX_HASH_SEED) >> _LZX_HASH_RSHIFT1)
				this.hashes[h0] = int32(srcIdx + 0)
				this.hashes[h1] = int32(srcIdx + 1)
				this.hashes[h2] = int32(srcIdx + 2)
				this.hashes[h3] = int32(srcIdx + 3)
				srcIdx += 4
			}
		}

		for srcIdx < anchor {
			this.hashes[this.hash(src[srcIdx:])] = int32(srcIdx)
			srcIdx++
		}
	}

	// Emit last literals
	litLen := count - anchor

	if dstIdx+litLen+tkIdx+mIdx >= count {
		return uint(count), uint(dstIdx), errors.New("LZCodec forward transform skip: no compression")
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

	if dstIdx > count-count/100 {
		return uint(count), uint(dstIdx), errors.New("LZCodec forward transform skip: no compression")
	}

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
	if this.bsVersion == 3 {
		return this.inverseV3(src, dst)
	}

	if this.bsVersion == 4 || this.bsVersion == 5 {
		return this.inverseV4(src, dst)
	}

	return this.inverseV6(src, dst)
}

func (this *LZXCodec) inverseV6(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	count := len(src)

	if count < 13 {
		return 0, 0, errors.New("LZCodec inverse transform failed: invalid data")
	}

	tkIdx := int(binary.LittleEndian.Uint32(src[0:]))
	mIdx := int(binary.LittleEndian.Uint32(src[4:]))
	mLenIdx := int(binary.LittleEndian.Uint32(src[8:]))

	if (tkIdx < 0) || (mIdx < 0) || (mLenIdx < 0) {
		return 0, 0, errors.New("LZCodec inverse transform failed: invalid data")
	}

	mIdx += tkIdx
	mLenIdx += mIdx

	if (tkIdx > count) || (mIdx > count) || (mLenIdx > count) {
		return 0, 0, errors.New("LZCodec inverse transform failed: invalid data")
	}

	srcEnd := tkIdx - 13
	mFlag := int(src[12]) & 0x01
	dstEnd := len(dst) - 16
	maxDist := _LZX_MAX_DISTANCE2

	if mFlag == 0 {
		maxDist = _LZX_MAX_DISTANCE1
	}

	minMatch := ((int(src[12]) >> 1) & 0x07) + 2
	srcIdx := 13
	dstIdx := 0
	repd0 := 0
	repd1 := 0

	for {
		token := int(src[tkIdx])
		tkIdx++

		if token >= 32 {
			// Get literal length
			var litLen int

			if token >= 0xE0 {
				ll, llIdx := readLengthLZ(src[srcIdx:])
				litLen = 7 + ll
				srcIdx += llIdx
			} else {
				litLen = token >> 5
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

		// Get match length and distance
		var mLen, dist int
		f := token & 0x18

		if f == 0 {
			// Repetition distance, read mLen fully outside of token
			mLen = token & 0x03

			if mLen == 3 {
				ml, mlIdx := readLengthLZ(src[mLenIdx:])
				mLen += (minMatch + ml)
				mLenIdx += mlIdx
			} else {
				mLen += minMatch
			}

			if token&0x04 == 0 {
				dist = repd0
			} else {
				dist = repd1
			}
		} else {
			// Read mLen remainder (if any) outside of token
			mLen = token & 0x07

			if mLen == 7 {
				ml, mlIdx := readLengthLZ(src[mLenIdx:])
				mLen += (minMatch + ml)
				mLenIdx += mlIdx
			} else {
				mLen += minMatch
			}

			dist = int(src[mIdx])
			mIdx++

			if f >= 0x10 {
				dist = (dist << 8) | int(src[mIdx])
				mIdx++

				if f == 0x18 {
					dist = (dist << 8) | int(src[mIdx])
					mIdx++
				}
			}
		}

		repd1 = repd0
		repd0 = dist
		mEnd := dstIdx + mLen
		ref := dstIdx - dist

		// Sanity check
		if ref < 0 || dist > maxDist || mEnd > dstEnd {
			return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: invalid distance decoded: %d", dist)
		}

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
		err = errors.New("LZCodec inverse transform failed")
	}

	return uint(mIdx), uint(dstIdx), err
}

func (this *LZXCodec) inverseV4(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	count := len(src)

	if count < 13 {
		return 0, 0, errors.New("LZCodec inverse transform failed: invalid data")
	}

	tkIdx := int(binary.LittleEndian.Uint32(src[0:]))
	mIdx := int(binary.LittleEndian.Uint32(src[4:]))
	mLenIdx := int(binary.LittleEndian.Uint32(src[8:]))

	if (tkIdx < 0) || (mIdx < 0) || (mLenIdx < 0) {
		return 0, 0, errors.New("LZCodec inverse transform failed: invalid data")
	}

	mIdx += tkIdx
	mLenIdx += mIdx

	if (tkIdx > count) || (mIdx > count) || (mLenIdx > count) {
		return 0, 0, errors.New("LZCodec inverse transform failed: invalid data")
	}

	srcEnd := tkIdx - 13
	mFlag := int(src[12]) & 0x01
	dstEnd := len(dst) - 16
	maxDist := _LZX_MAX_DISTANCE2

	if mFlag == 0 {
		maxDist = _LZX_MAX_DISTANCE1
	}

	mmIdx := (int(src[12]) >> 1) & 0x03
	var minMatches = []int{_LZX_MIN_MATCH4, _LZX_MIN_MATCH9, _LZX_MIN_MATCH6, _LZX_MIN_MATCH6}
	minMatch := minMatches[mmIdx]

	srcIdx := 13
	dstIdx := 0
	repd0 := 0
	repd1 := 0

	for {
		token := int(src[tkIdx])
		tkIdx++

		if token >= 32 {
			// Get literal length
			var litLen int

			if token >= 0xE0 {
				ll, delta := readLengthLZ(src[srcIdx:])
				litLen = 7 + ll
				srcIdx += delta
			} else {
				litLen = token >> 5
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

		// Get match length and distance
		mLen := token & 0x0F
		var dist int

		if mLen == 15 {
			// Repetition distance, read mLen fully outside of token
			ll, delta := readLengthLZ(src[mLenIdx:])
			mLen = minMatch + ll
			mLenIdx += delta

			if token&0x10 == 0 {
				dist = repd0
			} else {
				dist = repd1
			}
		} else {
			// Read mLen remainder (if any) outside of token
			if mLen == 14 {
				ll, delta := readLengthLZ(src[mLenIdx:])
				mLen = 14 + minMatch + ll
				mLenIdx += delta
			} else {
				mLen += minMatch
			}

			dist = int(src[mIdx])
			mIdx++

			if mFlag != 0 {
				dist = (dist << 8) | int(src[mIdx])
				mIdx++
			}

			if token&0x10 != 0 {
				dist = (dist << 8) | int(src[mIdx])
				mIdx++
			}
		}

		repd1 = repd0
		repd0 = dist
		mEnd := dstIdx + mLen
		ref := dstIdx - dist

		// Sanity check
		if ref < 0 || dist > maxDist || mEnd > dstEnd {
			return uint(srcIdx), uint(dstIdx), fmt.Errorf("LZCodec: invalid distance decoded: %d", dist)
		}

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
		err = errors.New("LZCodec inverse transform failed")
	}

	return uint(mIdx), uint(dstIdx), err
}

func (this *LZXCodec) inverseV3(src, dst []byte) (uint, uint, error) {
	if len(src) == 0 {
		return 0, 0, nil
	}

	count := len(src)

	if count < 13 {
		return 0, 0, errors.New("LZCodec inverse transform failed, invalid data")
	}

	tkIdx := int(binary.LittleEndian.Uint32(src[0:]))
	mIdx := int(binary.LittleEndian.Uint32(src[4:]))
	mLenIdx := int(binary.LittleEndian.Uint32(src[8:]))

	// Sanity checks
	if (tkIdx < 0) || (mIdx < 0) || (mLenIdx < 0) {
		return 0, 0, errors.New("LZCodec inverse transform failed, invalid data")
	}

	if (tkIdx > count) || (mIdx > count-tkIdx) || (mLenIdx > count-tkIdx-mIdx) {
		return 0, 0, errors.New("LZCodec inverse transform failed, invalid data")
	}

	mIdx += tkIdx
	mLenIdx += mIdx
	srcEnd := tkIdx - 13
	dstEnd := len(dst) - 16
	maxDist := _LZX_MAX_DISTANCE2

	if src[12]&1 == 0 {
		maxDist = _LZX_MAX_DISTANCE1
	}

	minMatch := _LZX_MIN_MATCH4

	if src[12]&2 != 0 {
		minMatch = _LZX_MIN_MATCH9
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
		err = errors.New("LZCodec inverse transform failed")
	}

	return uint(mIdx), uint(dstIdx), err
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
	hashes       []int32
	isBsVersion3 bool
}

// NewLZPCodec creates a new instance of LZXCodec
func NewLZPCodec() (*LZPCodec, error) {
	this := &LZPCodec{}
	this.hashes = make([]int32, 0)
	this.isBsVersion3 = false
	return this, nil
}

// NewLZPCodecWithCtx creates a new instance of LZXCodec using a
// configuration map as parameter.
func NewLZPCodecWithCtx(ctx *map[string]any) (*LZPCodec, error) {
	this := &LZPCodec{}
	this.hashes = make([]int32, 0)
	bsVersion := uint(4)

	if ctx != nil {
		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			bsVersion = val.(uint)
		}
	}

	this.isBsVersion3 = bsVersion < 4
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
		return 0, 0, fmt.Errorf("Output buffer is too small - size: %d, required %d", len(dst), n)
	}

	// If too small, skip
	if count < _LZP_MIN_BLOCK_LENGTH {
		return 0, 0, fmt.Errorf("Block too small, skip")
	}

	srcEnd := count
	dstEnd := count - (count >> 6)

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
	ctx := binary.LittleEndian.Uint32(src)
	srcIdx := 4
	dstIdx := 4

	for (srcIdx < srcEnd-_LZP_MIN_MATCH64) && (dstIdx < dstEnd) {
		h := (_LZP_HASH_SEED * ctx) >> _LZP_HASH_SHIFT
		ref := int(this.hashes[h])
		this.hashes[h] = int32(srcIdx)
		bestLen := 0

		// Find a match
		if ref != 0 && binary.LittleEndian.Uint64(src[srcIdx+_LZP_MIN_MATCH64-8:]) == binary.LittleEndian.Uint64(src[ref+_LZP_MIN_MATCH64-8:]) {
			bestLen = this.findMatch(src, srcIdx, ref, srcEnd-srcIdx)
		}

		// No good match ?
		if bestLen < _LZP_MIN_MATCH64 {
			val := uint32(src[srcIdx])
			ctx = (ctx << 8) | val
			dst[dstIdx] = src[srcIdx]
			srcIdx++
			dstIdx++

			if ref != 0 && val == _LZP_MATCH_FLAG {
				dst[dstIdx] = byte(0xFF)
				dstIdx++
			}

			continue
		}

		srcIdx += bestLen
		ctx = binary.LittleEndian.Uint32(src[srcIdx-4:])
		dst[dstIdx] = _LZP_MATCH_FLAG
		dstIdx++
		bestLen -= _LZP_MIN_MATCH64

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

		if (ref != 0) && (val == _LZP_MATCH_FLAG) {
			dst[dstIdx] = 0xFF
			dstIdx++
		}
	}

	var err error

	if (srcIdx != count) || (dstIdx >= dstEnd) {
		err = errors.New("LZP forward transform skip: output buffer too small")
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
		return 0, 0, errors.New("LZP inverse transform failed: block too small")
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
	var minMatch int

	if this.isBsVersion3 {
		minMatch = _LZP_MIN_MATCH96
	} else {
		minMatch = _LZP_MIN_MATCH64
	}

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

		mLen := minMatch

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

		if ref+mLen < dstIdx {
			copy(dst[dstIdx:], dst[ref:ref+mLen])
		} else {
			for i := 0; i < mLen; i++ {
				dst[dstIdx+i] = dst[ref+i]
			}
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
