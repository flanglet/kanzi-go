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
	"fmt"
	"kanzi"
	"kanzi/transform"
	"strings"
)

const (
	NULL_TRANSFORM_TYPE = uint16(0)
	BWT_TYPE            = uint16(1)
	BWTS_TYPE           = uint16(2)
	LZ4_TYPE            = uint16(3)
	SNAPPY_TYPE         = uint16(4)
	RLT_TYPE            = uint16(5)
)

func NewByteFunction(size uint, functionType uint16) (kanzi.ByteFunction, error) {
	switch uint16(functionType & 0x0F) {

	case SNAPPY_TYPE:
		return NewSnappyCodec()

	case LZ4_TYPE:
		return NewLZ4Codec()

	case RLT_TYPE:
		return NewRLT(3)

	case BWT_TYPE:
		bwt, err := transform.NewBWT()

		if err != nil {
			return nil, err
		}

		return NewBWTBlockCodec(bwt, int(functionType)>>4, size) // raw BWT

	case BWTS_TYPE:
		bwts, err := transform.NewBWTS()

		if err != nil {
			return nil, err
		}

		return NewBWTBlockCodec(bwts, int(functionType)>>4, size) // raw BWTS

	case NULL_TRANSFORM_TYPE:
		return NewNullFunction()

	default:
		return nil, fmt.Errorf("Unsupported function type: '%c'", functionType)
	}
}

func getGSTType(args string) uint16 {
	switch strings.ToUpper(args) {
	case "MTF":
		return GST_MODE_MTF

	case "RANK":
		return GST_MODE_RANK

	case "TIMESTAMP":
		return GST_MODE_TIMESTAMP

	case "":
		return GST_MODE_RAW

	case "NONE":
		return GST_MODE_RAW

	default:
		panic(fmt.Errorf("Unknown GST type: '%v'", args))
	}
}

func getGSTName(gstType int) string {
	switch gstType {
	case GST_MODE_MTF:
		return "MTF"

	case GST_MODE_RANK:
		return "RANK"

	case GST_MODE_TIMESTAMP:
		return "TIMESTAMP"

	case GST_MODE_RAW:
		return ""

	default:
		panic(fmt.Errorf("Unknown GST type: '%v'", gstType))
	}
}

func GetByteFunctionName(functionType uint16) string {
	switch uint16(functionType & 0x0F) {

	case SNAPPY_TYPE:
		return "SNAPPY"

	case LZ4_TYPE:
		return "LZ4"

	case RLT_TYPE:
		return "RLT"

	case BWT_TYPE:
		gstName := getGSTName(int(functionType) >> 4)

		if len(gstName) == 0 {
			return "BWT"
		} else {
			return "BWT+" + gstName
		}

	case BWTS_TYPE:
		gstName := getGSTName(int(functionType) >> 4)

		if len(gstName) == 0 {
			return "BWTS"
		} else {
			return "BWTS+" + gstName
		}

	case NULL_TRANSFORM_TYPE:
		return "NONE"

	default:
		panic(fmt.Errorf("Unsupported function type: '%c'", functionType))
	}
}

func GetByteFunctionType(functionName string) uint16 {
	args := ""
	functionName = strings.ToUpper(functionName)

	if strings.HasPrefix(functionName, "BWT") {
		tokens := strings.Split(functionName, "+")

		if len(tokens) > 1 {
			functionName = tokens[0]
			args = tokens[1]
		}
	}

	switch functionName {

	case "SNAPPY":
		return SNAPPY_TYPE

	case "LZ4":
		return LZ4_TYPE

	case "RLT":
		return RLT_TYPE

	case "BWT":
		gst := getGSTType(args)
		return uint16((gst << 4) | BWT_TYPE)

	case "BWTS":
		gst := getGSTType(args)
		return uint16((gst << 4) | BWTS_TYPE)

	case "NONE":
		return NULL_TRANSFORM_TYPE

	default:
		panic(fmt.Errorf("Unsupported function type: '%s'", functionName))
	}
}
