/*
Copyright 2011-2013 Frederic Langlet
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
	"kanzi"
	"strings"
)

const (
	NONE_TYPE    = byte(0) // No compression
	HUFFMAN_TYPE = byte(1) // Huffman
	FPAQ_TYPE    = byte(2) // Fast PAQ
	PAQ_TYPE     = byte(3) // PAQ
	RANGE_TYPE   = byte(4) // Range
	ANS_TYPE     = byte(5) // Asymetric Numerical System
	CM_TYPE      = byte(6) // Context Model
)

func NewEntropyDecoder(ibs kanzi.InputBitStream, entropyType byte) (kanzi.EntropyDecoder, error) {
	switch entropyType {

	case HUFFMAN_TYPE:
		return NewHuffmanDecoder(ibs)

	case ANS_TYPE:
		return NewANSRangeDecoder(ibs)

	case RANGE_TYPE:
		return NewRangeDecoder(ibs)

	case PAQ_TYPE:
		predictor, _ := NewPAQPredictor()
		return NewBinaryEntropyDecoder(ibs, predictor)

	case FPAQ_TYPE:
		predictor, _ := NewFPAQPredictor()
		return NewBinaryEntropyDecoder(ibs, predictor)

	case CM_TYPE:
		predictor, _ := NewCMPredictor()
		return NewBinaryEntropyDecoder(ibs, predictor)

	case NONE_TYPE:
		return NewNullEntropyDecoder(ibs)

	default:
		return nil, fmt.Errorf("Unsupported entropy codec type: '%c'", entropyType)
	}
}

func NewEntropyEncoder(obs kanzi.OutputBitStream, entropyType byte) (kanzi.EntropyEncoder, error) {
	switch byte(entropyType) {

	case HUFFMAN_TYPE:
		return NewHuffmanEncoder(obs)

	case ANS_TYPE:
		return NewANSRangeEncoder(obs)

	case RANGE_TYPE:
		return NewRangeEncoder(obs)

	case PAQ_TYPE:
		predictor, _ := NewPAQPredictor()
		return NewBinaryEntropyEncoder(obs, predictor)

	case FPAQ_TYPE:
		predictor, _ := NewFPAQPredictor()
		return NewBinaryEntropyEncoder(obs, predictor)

	case CM_TYPE:
		predictor, _ := NewCMPredictor()
		return NewBinaryEntropyEncoder(obs, predictor)

	case NONE_TYPE:
		return NewNullEntropyEncoder(obs)

	default:
		return nil, fmt.Errorf("Unsupported entropy codec type: '%c'", entropyType)
	}
}

func GetEntropyCodecName(entropyType byte) string {
	switch byte(entropyType) {

	case HUFFMAN_TYPE:
		return "HUFFMAN"

	case ANS_TYPE:
		return "ANS"

	case RANGE_TYPE:
		return "RANGE"

	case PAQ_TYPE:
		return "PAQ"

	case FPAQ_TYPE:
		return "FPAQ"

	case CM_TYPE:
		return "CM"

	case NONE_TYPE:
		return "NONE"

	default:
		panic(fmt.Errorf("Unsupported entropy codec type: '%c'", entropyType))
	}
}

func GetEntropyCodecType(entropyName string) byte {
	switch strings.ToUpper(entropyName) {

	case "HUFFMAN":
		return HUFFMAN_TYPE

	case "ANS":
		return ANS_TYPE

	case "RANGE":
		return RANGE_TYPE

	case "PAQ":
		return PAQ_TYPE

	case "FPAQ":
		return FPAQ_TYPE

	case "CM":
		return CM_TYPE

	case "NONE":
		return NONE_TYPE

	default:
		panic(fmt.Errorf("Unsupported entropy codec type: '%s'", entropyName))
	}
}
