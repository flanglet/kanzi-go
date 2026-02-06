/*
Copyright 2011-2025 Frederic Langlet
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

	kanzi "github.com/flanglet/kanzi-go/v2"
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
func NewEntropyDecoder(ibs kanzi.InputBitStream, ctx map[string]any,
	entropyType uint32) (kanzi.EntropyDecoder, error) {
	switch entropyType {

	case HUFFMAN_TYPE:
		return NewHuffmanDecoderWithCtx(ibs, &ctx)

	case ANS0_TYPE:
		return NewANSRangeDecoderWithCtx(ibs, &ctx, 0)

	case ANS1_TYPE:
		return NewANSRangeDecoderWithCtx(ibs, &ctx, 1)

	case RANGE_TYPE:
		return NewRangeDecoderWithCtx(ibs, &ctx)

	case FPAQ_TYPE:
		return NewFPAQDecoderWithCtx(ibs, &ctx)

	case CM_TYPE:
		predictor, _ := NewCMPredictor(&ctx)
		return NewBinaryEntropyDecoder(ibs, predictor)

	case TPAQ_TYPE, TPAQX_TYPE:
		predictor, _ := NewTPAQPredictor(&ctx)
		return NewBinaryEntropyDecoder(ibs, predictor)

	case NONE_TYPE:
		return NewNullEntropyDecoder(ibs)

	default:
		return nil, fmt.Errorf("Unsupported entropy codec type: '%d'", entropyType)
	}
}

// NewEntropyEncoder creates a new entropy encoder using the provided type and bitstream
func NewEntropyEncoder(obs kanzi.OutputBitStream, ctx map[string]any,
	entropyType uint32) (kanzi.EntropyEncoder, error) {
	switch entropyType {

	case HUFFMAN_TYPE:
		return NewHuffmanEncoder(obs)

	case ANS0_TYPE:
		return NewANSRangeEncoderWithCtx(obs, &ctx, 0)

	case ANS1_TYPE:
		return NewANSRangeEncoderWithCtx(obs, &ctx, 1)

	case RANGE_TYPE:
		return NewRangeEncoderWithCtx(obs, &ctx)

	case FPAQ_TYPE:
		return NewFPAQEncoderWithCtx(obs, &ctx)

	case CM_TYPE:
		predictor, _ := NewCMPredictor(&ctx)
		return NewBinaryEntropyEncoder(obs, predictor)

	case TPAQ_TYPE, TPAQX_TYPE:
		predictor, _ := NewTPAQPredictor(&ctx)
		return NewBinaryEntropyEncoder(obs, predictor)

	case NONE_TYPE:
		return NewNullEntropyEncoder(obs)

	default:
		return nil, fmt.Errorf("Unsupported entropy codec type: '%d'", entropyType)
	}
}

// GetName returns the name of the entropy codec given its type
func GetName(entropyType uint32) (string, error) {
	switch entropyType {

	case HUFFMAN_TYPE:
		return "HUFFMAN", nil

	case ANS0_TYPE:
		return "ANS0", nil

	case ANS1_TYPE:
		return "ANS1", nil

	case RANGE_TYPE:
		return "RANGE", nil

	case FPAQ_TYPE:
		return "FPAQ", nil

	case CM_TYPE:
		return "CM", nil

	case TPAQ_TYPE:
		return "TPAQ", nil

	case TPAQX_TYPE:
		return "TPAQX", nil

	case NONE_TYPE:
		return "NONE", nil

	default:
		return "", fmt.Errorf("Unsupported entropy codec type: '%d'", entropyType)
	}
}

// GetType returns the type of the entropy codec given its name
func GetType(entropyName string) (uint32, error) {
	switch strings.ToUpper(entropyName) {

	case "HUFFMAN":
		return HUFFMAN_TYPE, nil

	case "ANS0":
		return ANS0_TYPE, nil

	case "ANS1":
		return ANS1_TYPE, nil

	case "RANGE":
		return RANGE_TYPE, nil

	case "FPAQ":
		return FPAQ_TYPE, nil

	case "CM":
		return CM_TYPE, nil

	case "TPAQ":
		return TPAQ_TYPE, nil

	case "TPAQX":
		return TPAQX_TYPE, nil

	case "NONE":
		return NONE_TYPE, nil

	default:
		return 0, fmt.Errorf("Unsupported entropy codec type: '%v'", entropyName)
	}
}
