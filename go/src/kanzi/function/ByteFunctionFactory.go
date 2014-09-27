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
	"kanzi/transform"
	"strings"
)

const (
	// Transform: 4 lsb
	NULL_TRANSFORM_TYPE = byte(0)
	BWT_TYPE            = byte(1)
	BWTS_TYPE           = byte(2)
	LZ4_TYPE            = byte(3)
	SNAPPY_TYPE         = byte(4)
	RLT_TYPE            = byte(5)

	// GST: 3 msb
)

func NewByteFunction(size uint, functionType byte) (kanzi.ByteFunction, error) {
	switch functionType & 0x0F {

	case SNAPPY_TYPE:
		return NewSnappyCodec(size)

	case LZ4_TYPE:
		return NewLZ4Codec(size)

	case RLT_TYPE:
		return NewRLT(size, 3)

	case BWT_TYPE:
		bwt, err := transform.NewBWT(size)

		if err != nil {
			return nil, err
		}

		return NewBlockCodec(bwt, int(functionType)>>4, size) // raw BWT

	case BWTS_TYPE:
		bwts, err := transform.NewBWTS(size)

		if err != nil {
			return nil, err
		}

		return NewBlockCodec(bwts, int(functionType)>>4, size) // raw BWTS

	case NULL_TRANSFORM_TYPE:
		return NewNullFunction(size)
	}

	errMsg := fmt.Sprintf("Unsupported function type: '%c'", functionType)
	return nil, errors.New(errMsg)
}

func getGSTType(args string) (byte, error) {
	switch strings.ToUpper(args) {
	case "MTF":
		return GST_MODE_MTF, nil

	case "RANK":
		return GST_MODE_RANK, nil

	case "TIMESTAMP":
		return GST_MODE_TIMESTAMP, nil

	case "":
		return GST_MODE_RAW, nil

	case "NONE":
		return GST_MODE_RAW, nil
	}

	errMsg := fmt.Sprintf("Unknown GST type: '%v'", args)
	return byte(255), errors.New(errMsg)
}

func getGSTName(gstType int) (string, error) {
	switch gstType {
	case GST_MODE_MTF:
		return "MTF", nil

	case GST_MODE_RANK:
		return "RANK", nil

	case GST_MODE_TIMESTAMP:
		return "TIMESTAMP", nil

	case GST_MODE_RAW:
		return "", nil

	}

	errMsg := fmt.Sprintf("Unknown GST type: '%v'", gstType)
	return "Unknown", errors.New(errMsg)
}

func GetByteFunctionName(functionType byte) (string, error) {
	switch byte(functionType & 0x0F) {

	case SNAPPY_TYPE:
		return "SNAPPY", nil

	case LZ4_TYPE:
		return "LZ4", nil

	case RLT_TYPE:
		return "RLT", nil

	case BWT_TYPE:	
		gstName, _ := getGSTName(int(functionType) >> 4)

		if len(gstName) == 0 {
			return "BWT", nil
		} else {
			return "BWT+" + gstName, nil
		}

	case BWTS_TYPE:
		gstName, _ := getGSTName(int(functionType) >> 4)

		if len(gstName) == 0 {
			return "BWTS", nil
		} else {
			return "BWTS+" + gstName, nil
		}

	case NULL_TRANSFORM_TYPE:
		return "NONE", nil
	}

	errMsg := fmt.Sprintf("Unsupported function type: '%c'", functionType)
	return "", errors.New(errMsg)
}

func GetByteFunctionType(functionName string) (byte, error) {
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
		return SNAPPY_TYPE, nil

	case "LZ4":
		return LZ4_TYPE, nil

	case "RLT":
		return RLT_TYPE, nil

	case "BWT":
		gst, _ := getGSTType(args)
		return byte((gst << 4) | BWT_TYPE), nil

	case "BWTS":
		gst, _ := getGSTType(args)
		return byte((gst << 4) | BWTS_TYPE), nil

	case "NONE":
		return NULL_TRANSFORM_TYPE, nil
	}

	errMsg := fmt.Sprintf("Unsupported function type: '%s'", functionName)
	return byte(0), errors.New(errMsg)
}
