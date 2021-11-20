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

package entropy

import (
	"fmt"
	"strings"

	kanzi "github.com/flanglet/kanzi-go"
)

const (
	NONE_TYPE    = uint32(0)  // No compression
	HUFFMAN_TYPE = uint32(1)  // Huffman
	FPAQ_TYPE    = uint32(2)  // Fast PAQ (order 0)
	PAQ_TYPE     = uint32(3)  // Obsolete
	RANGE_TYPE   = uint32(4)  // Range
	ANS0_TYPE    = uint32(5)  // Asymmetric Numerical System order 0
	CM_TYPE      = uint32(6)  // Context Model
	TPAQ_TYPE    = uint32(7)  // Tangelo PAQ
	ANS1_TYPE    = uint32(8)  // Asymmetric Numerical System order 1
	TPAQX_TYPE   = uint32(9)  // Tangelo PAQ Extra
	RESERVED1    = uint32(10) // Reserved
	RESERVED2    = uint32(11) // Reserved
	RESERVED3    = uint32(12) // Reserved
	RESERVED4    = uint32(13) // Reserved
	RESERVED5    = uint32(14) // Reserved
	RESERVED6    = uint32(15) // Reserved
)

// NewEntropyDecoder creates a new entropy decoder using the provided type and bitstream
func NewEntropyDecoder(ibs kanzi.InputBitStream, ctx map[string]interface{},
	entropyType uint32) (kanzi.EntropyDecoder, error) {
	switch entropyType {

	case HUFFMAN_TYPE:
		return NewHuffmanDecoder(ibs)

	case ANS0_TYPE:
		return NewANSRangeDecoderWithCtx(ibs, 0, &ctx)

	case ANS1_TYPE:
		return NewANSRangeDecoderWithCtx(ibs, 1, &ctx)

	case RANGE_TYPE:
		return NewRangeDecoder(ibs)

	case FPAQ_TYPE:
		return NewFPAQDecoder(ibs)

	case CM_TYPE:
		predictor, _ := NewCMPredictor()
		return NewBinaryEntropyDecoder(ibs, predictor)

	case TPAQ_TYPE:
		predictor, _ := NewTPAQPredictor(&ctx)
		return NewBinaryEntropyDecoder(ibs, predictor)

	case TPAQX_TYPE:
		predictor, _ := NewTPAQPredictor(&ctx)
		return NewBinaryEntropyDecoder(ibs, predictor)

	case NONE_TYPE:
		return NewNullEntropyDecoder(ibs)

	default:
		return nil, fmt.Errorf("Unsupported entropy codec type: '%c'", entropyType)
	}
}

// NewEntropyEncoder creates a new entropy encoder using the provided type and bitstream
func NewEntropyEncoder(obs kanzi.OutputBitStream, ctx map[string]interface{},
	entropyType uint32) (kanzi.EntropyEncoder, error) {
	switch entropyType {

	case HUFFMAN_TYPE:
		return NewHuffmanEncoder(obs)

	case ANS0_TYPE:
		return NewANSRangeEncoderWithCtx(obs, 0, &ctx)

	case ANS1_TYPE:
		return NewANSRangeEncoderWithCtx(obs, 1, &ctx)

	case RANGE_TYPE:
		return NewRangeEncoder(obs)

	case FPAQ_TYPE:
		return NewFPAQEncoder(obs)

	case CM_TYPE:
		predictor, _ := NewCMPredictor()
		return NewBinaryEntropyEncoder(obs, predictor)

	case TPAQ_TYPE:
		predictor, _ := NewTPAQPredictor(&ctx)
		return NewBinaryEntropyEncoder(obs, predictor)

	case TPAQX_TYPE:
		predictor, _ := NewTPAQPredictor(&ctx)
		return NewBinaryEntropyEncoder(obs, predictor)

	case NONE_TYPE:
		return NewNullEntropyEncoder(obs)

	default:
		return nil, fmt.Errorf("Unsupported entropy codec type: '%c'", entropyType)
	}
}

// GetName returns the name of the entropy codec given its type
func GetName(entropyType uint32) string {
	switch entropyType {

	case HUFFMAN_TYPE:
		return "HUFFMAN"

	case ANS0_TYPE:
		return "ANS0"

	case ANS1_TYPE:
		return "ANS1"

	case RANGE_TYPE:
		return "RANGE"

	case FPAQ_TYPE:
		return "FPAQ"

	case CM_TYPE:
		return "CM"

	case TPAQ_TYPE:
		return "TPAQ"

	case TPAQX_TYPE:
		return "TPAQX"

	case NONE_TYPE:
		return "NONE"

	default:
		panic(fmt.Errorf("Unsupported entropy codec type: '%c'", entropyType))
	}
}

// GetType returns the type of the entropy codec given its name
func GetType(entropyName string) uint32 {
	switch strings.ToUpper(entropyName) {

	case "HUFFMAN":
		return HUFFMAN_TYPE

	case "ANS0":
		return ANS0_TYPE

	case "ANS1":
		return ANS1_TYPE

	case "RANGE":
		return RANGE_TYPE

	case "FPAQ":
		return FPAQ_TYPE

	case "CM":
		return CM_TYPE

	case "TPAQ":
		return TPAQ_TYPE

	case "TPAQX":
		return TPAQX_TYPE

	case "NONE":
		return NONE_TYPE

	default:
		panic(fmt.Errorf("Unsupported entropy codec type: '%s'", entropyName))
	}
}
