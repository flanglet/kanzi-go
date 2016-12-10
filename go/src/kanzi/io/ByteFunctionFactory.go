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

package io

import (
	"fmt"
	"kanzi"
	"kanzi/function"
	"kanzi/transform"
	"strings"
)

const (
	NULL_TRANSFORM_TYPE = uint16(0) // copy
	BWT_TYPE            = uint16(1) // Burrows Wheeler
	BWTS_TYPE           = uint16(2) // Burrows Wheeler Scott
	LZ4_TYPE            = uint16(3) // LZ4
	SNAPPY_TYPE         = uint16(4) // Snappy
	RLT_TYPE            = uint16(5) // Run Length
	ZRLT_TYPE           = uint16(6) // Zero Run Length
	MTFT_TYPE           = uint16(7) // Move To Front
	RANK_TYPE           = uint16(8) // Rank
	TIMESTAMP_TYPE      = uint16(9) // TimeStamp
)

func NewByteFunction(size uint, functionType uint16) (*function.ByteTransformSequence, error) {
	nbtr := 0

	// Several transforms
	for i := uint(0); i < 4; i++ {
		if (functionType>>(12-4*i))&0x0F != NULL_TRANSFORM_TYPE {
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
		t := (functionType >> (12 - uint(4*i))) & 0x0F

		if t != NULL_TRANSFORM_TYPE || i == 0 {
			if transforms[nbtr], err = newByteFunctionToken(size, t); err != nil {
				return nil, err
			}
		}

		nbtr++
	}

	return function.NewByteTransformSequence(transforms)
}

func newByteFunctionToken(size uint, functionType uint16) (kanzi.ByteTransform, error) {
	switch uint16(functionType & 0x0F) {

	case SNAPPY_TYPE:
		return function.NewSnappyCodec()

	case LZ4_TYPE:
		return function.NewLZ4Codec()

	case BWT_TYPE:
		return function.NewBWTBlockCodec()

	case BWTS_TYPE:
		return transform.NewBWTS()

	case MTFT_TYPE:
		return transform.NewMTFT()

	case ZRLT_TYPE:
		return function.NewZRLT()

	case RLT_TYPE:
		return function.NewRLT(3)

	case RANK_TYPE:
		return transform.NewSBRT(transform.SBRT_MODE_RANK)

	case TIMESTAMP_TYPE:
		return transform.NewSBRT(transform.SBRT_MODE_TIMESTAMP)

	case NULL_TRANSFORM_TYPE:
		return function.NewNullFunction()

	default:
		return nil, fmt.Errorf("Unknown transform type: '%v'", functionType)
	}
}

func GetByteFunctionName(functionType uint16) string {
	var s string

	for i := uint(0); i < 4; i++ {
		t := functionType >> (12 - 4*i)

		if t&0x0F == NULL_TRANSFORM_TYPE {
			continue
		}

		name := getByteFunctionNameToken(t)

		if len(s) != 0 {
			s += "+"
		}

		s += name
	}

	if len(s) == 0 {
		s += getByteFunctionNameToken(NULL_TRANSFORM_TYPE)
	}

	return s
}

func getByteFunctionNameToken(functionType uint16) string {
	switch uint16(functionType & 0x0F) {

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

	case TIMESTAMP_TYPE:
		return "TIMESTAMP"

	case NULL_TRANSFORM_TYPE:
		return "NONE"

	default:
		panic(fmt.Errorf("Unknown transform type: '%v'", functionType))
	}
}

// The returned type contains 4 (nibble based) transform values
func GetByteFunctionType(name string) uint16 {
	if strings.IndexByte(name, byte('+')) < 0 {
		return getByteFunctionTypeToken(name) << 12
	}

	tokens := strings.Split(name, "+")

	if len(tokens) == 0 {
		panic(fmt.Errorf("Unknown transform type: '%v'", name))
	}

	if len(tokens) > 4 {
		panic(fmt.Errorf("Only 4 transforms allowed: '%v'", name))
	}

	res := uint16(0)
	shift := uint(12)

	for _, token := range tokens {
		tkType := getByteFunctionTypeToken(token)

		// Skip null transform
		if tkType != NULL_TRANSFORM_TYPE {
			res |= (tkType << shift)
			shift -= 4
		}
	}

	return res
}

func getByteFunctionTypeToken(name string) uint16 {
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

	case "TIMESTAMP":
		return TIMESTAMP_TYPE

	case "NONE":
		return NULL_TRANSFORM_TYPE

	default:
		panic(fmt.Errorf("Unknown transform type: '%v'", name))
	}
}
