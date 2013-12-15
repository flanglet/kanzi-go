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
)

const (
	BLOCK_TYPE  = byte(66) // 'B'
	RLT_TYPE    = byte(82) // 'R'
	SNAPPY_TYPE = byte(83) // 'S'
	ZLT_TYPE    = byte(90) // 'Z'
	LZ4_TYPE    = byte(76) // 'L'
	NONE_TYPE   = byte(78) // 'N'
)

func NewByteFunction(size uint, functionType byte) (kanzi.ByteFunction, error) {
	switch functionType {
	case BLOCK_TYPE:
		return NewBlockCodec(size)

	case SNAPPY_TYPE:
		return NewSnappyCodec(size)

	case LZ4_TYPE:
		return NewLZ4Codec(size)

	case RLT_TYPE:
		return NewRLT(size, 3)

	case ZLT_TYPE:
		return NewZLT(size)

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

	case ZLT_TYPE:
		return "ZLT", nil

	case NONE_TYPE:
		return "NONE", nil
	}

	errMsg := fmt.Sprintf("Unsupported function type: '%c'", functionType)
	return errMsg, errors.New(errMsg)
}
