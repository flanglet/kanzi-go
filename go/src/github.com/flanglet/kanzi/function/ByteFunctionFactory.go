/*
Copyright 2011-2017 Frederic Langlet
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

package function

import (
	"fmt"
	kanzi "github.com/flanglet/kanzi"
	"github.com/flanglet/kanzi/transform"
	"strings"
)

const (
	BFF_ONE_SHIFT = uint(6)                 // bits per transform
	BFF_MAX_SHIFT = (8 - 1) * BFF_ONE_SHIFT // 8 transforms
	BFF_MASK      = (1 << BFF_ONE_SHIFT) - 1

	// Up to 64 transforms can be declared (6 bit index)
	NONE_TYPE   = uint64(0)  // copy
	BWT_TYPE    = uint64(1)  // Burrows Wheeler
	BWTS_TYPE   = uint64(2)  // Burrows Wheeler Scott
	LZ4_TYPE    = uint64(3)  // LZ4
	SNAPPY_TYPE = uint64(4)  // Snappy
	RLT_TYPE    = uint64(5)  // Run Length
	ZRLT_TYPE   = uint64(6)  // Zero Run Length
	MTFT_TYPE   = uint64(7)  // Move To Front
	RANK_TYPE   = uint64(8)  // Rank
	X86_TYPE    = uint64(9)  // X86 codec
	DICT_TYPE   = uint64(10) // Text codec
)

func NewByteFunction(ctx map[string]interface{}, functionType uint64) (*ByteTransformSequence, error) {
	nbtr := 0

	// Several transforms
	for i := uint(0); i < 8; i++ {
		if (functionType>>(BFF_MAX_SHIFT-BFF_ONE_SHIFT*i))&BFF_MASK != NONE_TYPE {
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
		t := (functionType >> (BFF_MAX_SHIFT - BFF_ONE_SHIFT*uint(i))) & BFF_MASK

		if t != NONE_TYPE || i == 0 {
			if transforms[nbtr], err = newByteFunctionToken(ctx, t); err != nil {
				return nil, err
			}
		}

		nbtr++
	}

	return NewByteTransformSequence(transforms)
}

func newByteFunctionToken(ctx map[string]interface{}, functionType uint64) (kanzi.ByteTransform, error) {
	switch functionType {

	case SNAPPY_TYPE:
		return NewSnappyCodec()

	case LZ4_TYPE:
		return NewLZ4Codec()

	case BWT_TYPE:
		return NewBWTBlockCodec()

	case BWTS_TYPE:
		return transform.NewBWTS()

	case MTFT_TYPE:
		return transform.NewMTFT()

	case ZRLT_TYPE:
		return NewZRLT()

	case RLT_TYPE:
		return NewRLT(2)

	case RANK_TYPE:
		return transform.NewSBRT(transform.SBRT_MODE_RANK)

	case DICT_TYPE:
		return NewTextCodecFromMap(ctx)

	case X86_TYPE:
		return NewX86Codec()

	case NONE_TYPE:
		return NewNullFunction()

	default:
		return nil, fmt.Errorf("Unknown transform type: '%v'", functionType)
	}
}

func GetName(functionType uint64) string {
	var s string

	for i := uint(0); i < 8; i++ {
		t := (functionType >> (BFF_MAX_SHIFT - BFF_ONE_SHIFT*i)) & BFF_MASK

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

	case LZ4_TYPE:
		return "LZ4"

	case BWT_TYPE:
		return "BWT"

	case BWTS_TYPE:
		return "BWTS"

	case SNAPPY_TYPE:
		return "SNAPPY"

	case MTFT_TYPE:
		return "MTFT"

	case ZRLT_TYPE:
		return "ZRLT"

	case RLT_TYPE:
		return "RLT"

	case RANK_TYPE:
		return "RANK"

	case X86_TYPE:
		return "X86"

	case DICT_TYPE:
		return "TEXT"

	case NONE_TYPE:
		return "NONE"

	default:
		panic(fmt.Errorf("Unknown transform type: '%v'", functionType))
	}
}

// The returned type contains 8  transform values
func GetType(name string) uint64 {
	if strings.IndexByte(name, byte('+')) < 0 {
		return getByteFunctionTypeToken(name) << BFF_MAX_SHIFT
	}

	tokens := strings.Split(name, "+")

	if len(tokens) == 0 {
		panic(fmt.Errorf("Unknown transform type: '%v'", name))
	}

	if len(tokens) > 8 {
		panic(fmt.Errorf("Only 8 transforms allowed: '%v'", name))
	}

	res := uint64(0)
	shift := BFF_MAX_SHIFT

	for _, token := range tokens {
		tkType := getByteFunctionTypeToken(token)

		// Skip null transform
		if tkType != NONE_TYPE {
			res |= (tkType << shift)
			shift -= BFF_ONE_SHIFT
		}
	}

	return res
}

func getByteFunctionTypeToken(name string) uint64 {
	name = strings.ToUpper(name)

	switch name {

	case "BWT":
		return BWT_TYPE

	case "BWTS":
		return BWTS_TYPE

	case "SNAPPY":
		return SNAPPY_TYPE

	case "LZ4":
		return LZ4_TYPE

	case "MTFT":
		return MTFT_TYPE

	case "ZRLT":
		return ZRLT_TYPE

	case "RLT":
		return RLT_TYPE

	case "RANK":
		return RANK_TYPE

	case "X86":
		return X86_TYPE

	case "TEXT":
		return DICT_TYPE

	case "NONE":
		return NONE_TYPE

	default:
		panic(fmt.Errorf("Unknown transform type: '%v'", name))
	}
}
