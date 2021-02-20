/*
Copyright 2011-2021 Frederic Langlet
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

package transform

import (
	"fmt"
	"strings"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	_BFF_ONE_SHIFT = uint(6)                  // bits per transform
	_BFF_MAX_SHIFT = (8 - 1) * _BFF_ONE_SHIFT // 8 transforms
	_BFF_MASK      = (1 << _BFF_ONE_SHIFT) - 1

	// Up to 64 transforms can be declared (6 bit index)
	NONE_TYPE   = uint64(0)  // Copy
	BWT_TYPE    = uint64(1)  // Burrows Wheeler
	BWTS_TYPE   = uint64(2)  // Burrows Wheeler Scott
	LZ_TYPE     = uint64(3)  // Lempel Ziv
	SNAPPY_TYPE = uint64(4)  // Snappy (obsolete)
	RLT_TYPE    = uint64(5)  // Run Length
	ZRLT_TYPE   = uint64(6)  // Zero Run Length
	MTFT_TYPE   = uint64(7)  // Move To Front
	RANK_TYPE   = uint64(8)  // Rank
	X86_TYPE    = uint64(9)  // X86 codec
	DICT_TYPE   = uint64(10) // Text codec
	ROLZ_TYPE   = uint64(11) // ROLZ codec
	ROLZX_TYPE  = uint64(12) // ROLZ Extra codec
	SRT_TYPE    = uint64(13) // Sorted Rank
	LZP_TYPE    = uint64(14) // Lempel Ziv Predict
	FSD_TYPE    = uint64(15) // Fixed Shift Delta codec
	LZX_TYPE    = uint64(16) // Lempel Ziv Extra
)

// New creates a new instance of ByteTransformSequence based on the provided
// function type.
func New(ctx *map[string]interface{}, functionType uint64) (*ByteTransformSequence, error) {
	nbtr := 0

	// Several transforms
	for i := uint(0); i < 8; i++ {
		if (functionType>>(_BFF_MAX_SHIFT-_BFF_ONE_SHIFT*i))&_BFF_MASK != NONE_TYPE {
			nbtr++
		}
	}

	// Only null transforms ? Keep first.
	if nbtr == 0 {
		nbtr = 1
	}

	transforms := make([]kanzi.ByteTransform, nbtr)
	nbtr = 0
	var err error

	for i := range transforms {
		t := (functionType >> (_BFF_MAX_SHIFT - _BFF_ONE_SHIFT*uint(i))) & _BFF_MASK

		if t != NONE_TYPE || i == 0 {
			if transforms[nbtr], err = newToken(ctx, t); err != nil {
				return nil, err
			}
		}

		nbtr++
	}

	return NewByteTransformSequence(transforms)
}

func newToken(ctx *map[string]interface{}, functionType uint64) (kanzi.ByteTransform, error) {
	switch functionType {

	case DICT_TYPE:
		textCodecType := 1

		if val, containsKey := (*ctx)["codec"]; containsKey {
			entropyType := strings.ToUpper(val.(string))

			// Select text encoding based on entropy codec.
			if entropyType == "NONE" || entropyType == "ANS0" ||
				entropyType == "HUFFMAN" || entropyType == "RANGE" {
				textCodecType = 2
			}
		}

		(*ctx)["textcodec"] = textCodecType
		return NewTextCodecWithCtx(ctx)

	case ROLZ_TYPE:
		return NewROLZCodecWithCtx(ctx)

	case ROLZX_TYPE:
		return NewROLZCodecWithCtx(ctx)

	case BWT_TYPE:
		return NewBWTBlockCodecWithCtx(ctx)

	case BWTS_TYPE:
		return NewBWTSWithCtx(ctx)

	case LZ_TYPE:
		(*ctx)["lz"] = LZ_TYPE
		return NewLZCodecWithCtx(ctx)

	case LZX_TYPE:
		(*ctx)["lz"] = LZX_TYPE
		return NewLZCodecWithCtx(ctx)

	case LZP_TYPE:
		(*ctx)["lz"] = LZP_TYPE
		return NewLZCodecWithCtx(ctx)

	case FSD_TYPE:
		return NewFSDCodecWithCtx(ctx)

	case SRT_TYPE:
		return NewSRTWithCtx(ctx)

	case RANK_TYPE:
		(*ctx)["sbrt"] = SBRT_MODE_RANK
		return NewSBRTWithCtx(ctx)

	case MTFT_TYPE:
		(*ctx)["sbrt"] = SBRT_MODE_MTF
		return NewSBRTWithCtx(ctx)

	case ZRLT_TYPE:
		return NewZRLTWithCtx(ctx)

	case RLT_TYPE:
		return NewRLTWithCtx(ctx)

	case X86_TYPE:
		return NewX86CodecWithCtx(ctx)

	case NONE_TYPE:
		return NewNullTransformWithCtx(ctx)

	default:
		return nil, fmt.Errorf("Unknown transform type: '%v'", functionType)
	}
}

// GetName transforms the function type into a function name
func GetName(functionType uint64) string {
	var s string

	for i := uint(0); i < 8; i++ {
		t := (functionType >> (_BFF_MAX_SHIFT - _BFF_ONE_SHIFT*i)) & _BFF_MASK

		if t == NONE_TYPE {
			continue
		}

		name := getByteFunctionNameToken(t)

		if len(s) != 0 {
			s += "+"
		}

		s += name
	}

	if len(s) == 0 {
		s += getByteFunctionNameToken(NONE_TYPE)
	}

	return s
}

func getByteFunctionNameToken(functionType uint64) string {
	switch functionType {

	case DICT_TYPE:
		return "TEXT"

	case ROLZ_TYPE:
		return "ROLZ"

	case ROLZX_TYPE:
		return "ROLZX"

	case BWT_TYPE:
		return "BWT"

	case BWTS_TYPE:
		return "BWTS"

	case LZ_TYPE:
		return "LZ"

	case LZX_TYPE:
		return "LZX"

	case LZP_TYPE:
		return "LZP"

	case X86_TYPE:
		return "X86"

	case FSD_TYPE:
		return "FSD"

	case ZRLT_TYPE:
		return "ZRLT"

	case RLT_TYPE:
		return "RLT"

	case SRT_TYPE:
		return "SRT"

	case RANK_TYPE:
		return "RANK"

	case MTFT_TYPE:
		return "MTFT"

	case NONE_TYPE:
		return "NONE"

	default:
		panic(fmt.Errorf("Unknown transform type: '%v'", functionType))
	}
}

// GetType transforms the function name into a function type.
// The returned type contains 8 transform type values (masks).
func GetType(name string) uint64 {
	if strings.IndexByte(name, byte('+')) < 0 {
		return getByteFunctionTypeToken(name) << _BFF_MAX_SHIFT
	}

	tokens := strings.Split(name, "+")

	if len(tokens) == 0 {
		panic(fmt.Errorf("Unknown transform type: '%v'", name))
	}

	if len(tokens) > 8 {
		panic(fmt.Errorf("Only 8 transforms allowed: '%v'", name))
	}

	res := uint64(0)
	shift := _BFF_MAX_SHIFT

	for _, token := range tokens {
		tkType := getByteFunctionTypeToken(token)

		// Skip null transform
		if tkType != NONE_TYPE {
			res |= (tkType << shift)
			shift -= _BFF_ONE_SHIFT
		}
	}

	return res
}

func getByteFunctionTypeToken(name string) uint64 {
	name = strings.ToUpper(name)

	switch name {

	case "TEXT":
		return DICT_TYPE

	case "BWT":
		return BWT_TYPE

	case "BWTS":
		return BWTS_TYPE

	case "ROLZ":
		return ROLZ_TYPE

	case "ROLZX":
		return ROLZX_TYPE

	case "LZ":
		return LZ_TYPE

	case "LZX":
		return LZX_TYPE

	case "LZP":
		return LZP_TYPE

	case "FSD":
		return FSD_TYPE

	case "SRT":
		return SRT_TYPE

	case "RANK":
		return RANK_TYPE

	case "MTFT":
		return MTFT_TYPE

	case "ZRLT":
		return ZRLT_TYPE

	case "RLT":
		return RLT_TYPE

	case "X86":
		return X86_TYPE

	case "NONE":
		return NONE_TYPE

	default:
		panic(fmt.Errorf("Unknown transform type: '%v'", name))
	}
}
