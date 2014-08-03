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

package function

import (
	"errors"
	"fmt"
	"kanzi"
	"strings"
)

const (
	BLOCK_TYPE  = byte(66) // 'B'
	RLT_TYPE    = byte(82) // 'R'
	SNAPPY_TYPE = byte(83) // 'S'
	ZRLT_TYPE   = byte(90) // 'Z'
	LZ4_TYPE    = byte(76) // 'L'
	NONE_TYPE   = byte(78) // 'N'
	BWT_TYPE    = byte(87) // 'W'
)

func NewByteFunction(size uint, functionType byte) (kanzi.ByteFunction, error) {
	switch functionType {
	case BLOCK_TYPE:
		return NewBlockCodec(MODE_MTF, size) // BWT+GST+ZRLT

	case SNAPPY_TYPE:
		return NewSnappyCodec(size)

	case LZ4_TYPE:
		return NewLZ4Codec(size)

	case RLT_TYPE:
		return NewRLT(size, 3)

	case ZRLT_TYPE:
		return NewZRLT(size)

	case BWT_TYPE:
		return NewBlockCodec(MODE_RAW_BWT, size) // raw BWT

	case NONE_TYPE:
		return NewNullFunction(size)
	}

	errMsg := fmt.Sprintf("Unsupported function type: '%c'", functionType)
	return nil, errors.New(errMsg)
}

func GetByteFunctionName(functionType byte) (string, error) {
	switch byte(functionType) {
	case BLOCK_TYPE:
		return "BLOCK", nil

	case SNAPPY_TYPE:
		return "SNAPPY", nil

	case LZ4_TYPE:
		return "LZ4", nil

	case RLT_TYPE:
		return "RLT", nil

	case ZRLT_TYPE:
		return "ZRLT", nil

	case BWT_TYPE:
		return "BWT", nil

	case NONE_TYPE:
		return "NONE", nil
	}

	errMsg := fmt.Sprintf("Unsupported function type: '%c'", functionType)
	return "", errors.New(errMsg)
}

func GetByteFunctionType(functionName string) (byte, error) {
	switch strings.ToUpper(functionName) {
	case "BLOCK":
		return BLOCK_TYPE, nil // BWT+GST+ZRLT

	case "SNAPPY":
		return SNAPPY_TYPE, nil

	case "LZ4":
		return LZ4_TYPE, nil

	case "RLT":
		return RLT_TYPE, nil

	case "ZRLT":
		return ZRLT_TYPE, nil

	case "BWT":
		return BWT_TYPE, nil // raw BWT

	case "NONE":
		return NONE_TYPE, nil
	}

	errMsg := fmt.Sprintf("Unsupported function type: '%s'", functionName)
	return byte(0), errors.New(errMsg)
}
