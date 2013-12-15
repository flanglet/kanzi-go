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
	"errors"
	"fmt"
	"kanzi"
)

const (
	HUFFMAN_TYPE = byte(72)
	NONE_TYPE    = byte(78)
	FPAQ_TYPE    = byte(70)
	PAQ_TYPE     = byte(80)
	RANGE_TYPE   = byte(82)
)

func NewEntropyDecoder(ibs kanzi.InputBitStream, entropyType byte) (kanzi.EntropyDecoder, error) {
	if ibs == nil {
		return nil, errors.New("Invalid null input bit stream parameter")
	}

	switch entropyType {
	
	case HUFFMAN_TYPE:
		return NewHuffmanDecoder(ibs)

	case RANGE_TYPE:
		return NewRangeDecoder(ibs)

	case PAQ_TYPE:
		predictor, _ := NewPAQPredictor()
		return NewBinaryEntropyDecoder(ibs, predictor)

	case FPAQ_TYPE:
		predictor, _ := NewFPAQPredictor()
		return NewBinaryEntropyDecoder(ibs, predictor)

	case NONE_TYPE:
		return NewNullEntropyDecoder(ibs)
	}
	
	errMsg := fmt.Sprintf("Unsupported entropy codec type: '%c'", entropyType)
	return nil, errors.New(errMsg)
}

func NewEntropyEncoder(obs kanzi.OutputBitStream, entropyType byte) (kanzi.EntropyEncoder, error) {
	if obs == nil {
		return nil, errors.New("Invalid null output bit stream parameter")
	}

	switch byte(entropyType) {
	
	case HUFFMAN_TYPE:
		return NewHuffmanEncoder(obs)

	case RANGE_TYPE:
		return NewRangeEncoder(obs)

	case PAQ_TYPE:
		predictor, _ := NewPAQPredictor()
		return NewBinaryEntropyEncoder(obs, predictor)

	case FPAQ_TYPE:
		predictor, _ := NewFPAQPredictor()
		return NewBinaryEntropyEncoder(obs, predictor)

	case NONE_TYPE:
		return NewNullEntropyEncoder(obs)

	}

	errMsg := fmt.Sprintf("Unsupported entropy codec type: '%c'", entropyType)
	return nil, errors.New(errMsg)
}

func GetEntropyCodecName(entropyType byte) (string, error) {
	switch byte(entropyType) {
	
	case HUFFMAN_TYPE:
		return "HUFFMAN", nil

	case RANGE_TYPE:
		return "RANGE", nil

	case PAQ_TYPE:
		return "PAQ", nil

	case FPAQ_TYPE:
		return "FPAQ", nil

	case NONE_TYPE:
		return "NONE", nil

	}

	errMsg := fmt.Sprintf("Unsupported entropy codec type: '%c'", entropyType)
	return errMsg, errors.New(errMsg)
}
