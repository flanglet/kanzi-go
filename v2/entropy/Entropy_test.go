/*
Copyright 2011-2024 Frederic Langlet
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
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/bitstream"
	"github.com/flanglet/kanzi-go/v2/internal"
)

func TestHuffman(b *testing.T) {
	if err := testEntropyCorrectness("HUFFMAN"); err != nil {
		b.Errorf(err.Error())
	}
}

func TestFPAQCodecSpecificPatterns(t *testing.T) {
	type testCase struct {
		name  string
		input []byte
	}

	testCases := []testCase{
		{
			name:  "FPAQ_RepeatingPattern_LMN",
			input: []byte(strRepeat("LMN", 20)), // 60 bytes
		},
		{
			name:  "FPAQ_ChangingPattern_P30Q30R30PP",
			input: []byte(strRepeat("P", 30) + strRepeat("Q", 30) + strRepeat("R", 30) + "PP"),
		},
		{
			name:  "FPAQ_AlternatingSymbols_STST",
			input: []byte(strRepeat("ST", 30)), // 60 bytes
		},
		{
			name:  "FPAQ_AllSame_W50",
			input: []byte(strRepeat("W", 50)),
		},
		{
			name:  "FPAQ_AlmostAllSame_X50Y1",
			input: []byte(strRepeat("X", 50) + "Y"),
		},
		{
			name:  "FPAQ_SingleSymbol_V",
			input: []byte("V"),
		},
		{
			name:  "FPAQ_TwoDifferentSymbols_IJ",
			input: []byte("IJ"),
		},
		{
			name:  "FPAQ_TwoSameSymbols_KK",
			input: []byte("KK"),
		},
		{
			name:  "FPAQ_EmptyInput",
			input: []byte{},
		},
		{
			name:  "FPAQ_DistantRepetition_ABCDEFXABC",
			// Using 5 repeats to make it reasonably long for FPAQ's context modeling
			input: []byte(strRepeat("ABCDEFXABC", 5)), 
		},
		{
			name: "FPAQ_AllByteValues",
			input: func() []byte {
				res := make([]byte, 256)
				for i := 0; i < 256; i++ {
					res[i] = byte(i)
				}
				return res
			}(),
		},
		{
			name:  "FPAQ_MixedFrequencies",
			input: []byte(strRepeat("L",50) + strRepeat("M",20) + strRepeat("N",5) + strRepeat("O",1) + strRepeat("L",20)),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing FPAQ Codec with pattern: %s (length %d)", tc.name, len(tc.input))

			bufferStream := internal.NewBufferStream()
			obs, err := bitstream.NewDefaultOutputBitStream(bufferStream, 16384)
			if err != nil {
				t.Fatalf("Failed to create OutputBitStream: %v", err)
			}

			encoder := getEncoder("FPAQ", obs)
			if encoder == nil {
				t.Fatalf("Cannot create FPAQ encoder")
			}

			originalLength := len(tc.input)
			byteWritten, err := encoder.Write(tc.input)
			if err != nil {
				t.Fatalf("Error during FPAQ encoding: %v", err)
			}
			if byteWritten != originalLength {
				t.Fatalf("FPAQ Encoder.Write returned %d, expected %d", byteWritten, originalLength)
			}

			encoder.Dispose()
			if err := obs.Close(); err != nil {
				t.Fatalf("Error closing OutputBitStream for FPAQ: %v", err)
			}

			encodedLength := int((obs.Written() + 7) / 8)
			t.Logf("Original length: %d, Encoded length (FPAQ): %d", originalLength, encodedLength)

			if originalLength > 10 { // Only check for reasonable length inputs
				isPredictable := tc.name == "FPAQ_RepeatingPattern_LMN" ||
								tc.name == "FPAQ_AlternatingSymbols_STST" ||
								tc.name == "FPAQ_AllSame_W50" ||
								tc.name == "FPAQ_DistantRepetition_ABCDEFXABC"
				
				if isPredictable && encodedLength >= originalLength {
					t.Logf("Warning: Predictable pattern '%s' for FPAQ did not compress. Original: %d, Encoded: %d. This might be acceptable for some short patterns or specific FPAQ configurations.", tc.name, originalLength, encodedLength)
				}
			}

			ibs, err := bitstream.NewDefaultInputBitStream(bufferStream, 16384)
			if err != nil {
				t.Fatalf("Failed to create InputBitStream for FPAQ: %v", err)
			}

			decoder := getDecoder("FPAQ", ibs)
			if decoder == nil {
				t.Fatalf("Cannot create FPAQ decoder")
			}

			decodedBytes := make([]byte, originalLength)
			if originalLength > 0 {
				bytesRead, err := decoder.Read(decodedBytes)
				if err != nil {
					t.Fatalf("Error during FPAQ decoding: %v", err)
				}
				if bytesRead != originalLength {
					t.Fatalf("FPAQ Decoder.Read returned %d, expected %d", bytesRead, originalLength)
				}
			}

			decoder.Dispose()
			if err := ibs.Close(); err != nil {
				t.Fatalf("Error closing InputBitStream for FPAQ: %v", err)
			}

			if !bytes.Equal(tc.input, decodedBytes) {
				t.Errorf("FPAQ: Decoded data does not match original for test case: %s.\nOriginal (len %d): %q\nDecoded  (len %d): %q",
					tc.name, len(tc.input), string(tc.input), len(decodedBytes), string(decodedBytes))
			}
		})
	}
}

func TestTPAQCodecSpecificPatterns(t *testing.T) {
	type testCase struct {
		name  string
		input []byte
	}

	testCases := []testCase{
		{
			name:  "TPAQ_RepeatingPattern_XYZ",
			input: []byte(strRepeat("XYZ", 20)), // 60 bytes
		},
		{
			name:  "TPAQ_ChangingPattern_D30E30F30DD",
			input: []byte(strRepeat("D", 30) + strRepeat("E", 30) + strRepeat("F", 30) + "DD"),
		},
		{
			name:  "TPAQ_AlternatingSymbols_UVUV",
			input: []byte(strRepeat("UV", 30)), // 60 bytes
		},
		{
			name:  "TPAQ_AllSame_K50",
			input: []byte(strRepeat("K", 50)),
		},
		{
			name:  "TPAQ_AlmostAllSame_G50H1",
			input: []byte(strRepeat("G", 50) + "H"),
		},
		{
			name:  "TPAQ_SingleSymbol_Q",
			input: []byte("Q"),
		},
		{
			name:  "TPAQ_TwoDifferentSymbols_RS",
			input: []byte("RS"),
		},
		{
			name:  "TPAQ_TwoSameSymbols_TT",
			input: []byte("TT"),
		},
		{
			name:  "TPAQ_EmptyInput",
			input: []byte{},
		},
		{
			name:  "TPAQ_DistantRepetition_DEFGHIDEF",
			input: []byte(strRepeat("DEFGHIDEF", 5)), // e.g., DEFGHIDEFDEFGHIDEF...
		},
		{
			name: "TPAQ_AllByteValues",
			input: func() []byte {
				res := make([]byte, 256)
				for i := 0; i < 256; i++ {
					res[i] = byte(i)
				}
				return res
			}(),
		},
		{
			name:  "TPAQ_MixedFrequencies",
			input: []byte(strRepeat("X",50) + strRepeat("Y",20) + strRepeat("Z",5) + strRepeat("W",1) + strRepeat("X",20)),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing TPAQ Codec with pattern: %s (length %d)", tc.name, len(tc.input))

			bufferStream := internal.NewBufferStream()
			obs, err := bitstream.NewDefaultOutputBitStream(bufferStream, 16384)
			if err != nil {
				t.Fatalf("Failed to create OutputBitStream: %v", err)
			}

			encoder := getEncoder("TPAQ", obs)
			if encoder == nil {
				t.Fatalf("Cannot create TPAQ encoder")
			}

			originalLength := len(tc.input)
			byteWritten, err := encoder.Write(tc.input)
			if err != nil {
				t.Fatalf("Error during TPAQ encoding: %v", err)
			}
			if byteWritten != originalLength {
				t.Fatalf("TPAQ Encoder.Write returned %d, expected %d", byteWritten, originalLength)
			}

			encoder.Dispose()
			if err := obs.Close(); err != nil {
				t.Fatalf("Error closing OutputBitStream for TPAQ: %v", err)
			}

			encodedLength := bufferStream.Len()
			t.Logf("Original length: %d, Encoded length (TPAQ): %d", originalLength, encodedLength)

			if originalLength > 10 {
				isPredictable := tc.name == "TPAQ_RepeatingPattern_XYZ" ||
								tc.name == "TPAQ_AlternatingSymbols_UVUV" ||
								tc.name == "TPAQ_AllSame_K50" ||
								tc.name == "TPAQ_DistantRepetition_DEFGHIDEF"
				
				if isPredictable && encodedLength >= originalLength {
					t.Logf("Warning: Predictable pattern '%s' for TPAQ did not compress. Original: %d, Encoded: %d. This might be acceptable for some short patterns or specific TPAQ configurations.", tc.name, originalLength, encodedLength)
				}
			}

			ibs, err := bitstream.NewDefaultInputBitStream(bufferStream, 16384)
			if err != nil {
				t.Fatalf("Failed to create InputBitStream for TPAQ: %v", err)
			}

			decoder := getDecoder("TPAQ", ibs)
			if decoder == nil {
				t.Fatalf("Cannot create TPAQ decoder")
			}

			decodedBytes := make([]byte, originalLength)
			if originalLength > 0 {
				bytesRead, err := decoder.Read(decodedBytes)
				if err != nil {
					t.Fatalf("Error during TPAQ decoding: %v", err)
				}
				if bytesRead != originalLength {
					t.Fatalf("TPAQ Decoder.Read returned %d, expected %d", bytesRead, originalLength)
				}
			}

			decoder.Dispose()
			if err := ibs.Close(); err != nil {
				t.Fatalf("Error closing InputBitStream for TPAQ: %v", err)
			}

			if !bytes.Equal(tc.input, decodedBytes) {
				t.Errorf("TPAQ: Decoded data does not match original for test case: %s.\nOriginal (len %d): %q\nDecoded  (len %d): %q",
					tc.name, len(tc.input), string(tc.input), len(decodedBytes), string(decodedBytes))
			}
		})
	}
}

func TestANS0(b *testing.T) {
	if err := testEntropyCorrectness("ANS0"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestANS1(b *testing.T) {
	if err := testEntropyCorrectness("ANS1"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestRange(b *testing.T) {
	if err := testEntropyCorrectness("RANGE"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestFPAQ(b *testing.T) {
	if err := testEntropyCorrectness("FPAQ"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestCM(b *testing.T) {
	if err := testEntropyCorrectness("CM"); err != nil {
		b.Errorf(err.Error())
	}
}
func TestTPAQ(b *testing.T) {
	if err := testEntropyCorrectness("TPAQ"); err != nil {
		b.Errorf(err.Error())
	}
}

func getEncoder(name string, obs kanzi.OutputBitStream) kanzi.EntropyEncoder {
	ctx := make(map[string]any)
	ctx["entropy"] = name
	ctx["bsVersion"] = uint(4)
	eType, _ := GetType(name)

	res, err := NewEntropyEncoder(obs, ctx, eType)

	if err != nil {
		panic(err.Error())
	}

	return res
}

func getDecoder(name string, ibs kanzi.InputBitStream) kanzi.EntropyDecoder {
	ctx := make(map[string]any)
	ctx["entropy"] = name
	ctx["bsVersion"] = uint(4)
	eType, _ := GetType(name)

	res, err := NewEntropyDecoder(ibs, ctx, eType)

	if err != nil {
		panic(err.Error())
	}

	return res
}

func testEntropyCorrectness(codecName string) error { // Renamed 'name' to 'codecName' for clarity
	fmt.Println()
	fmt.Printf("=== Testing %v ===\n", codecName)

	type entropyTestCase struct {
		name  string
		input []byte
		// ii serves to preserve the old random generation logic for ii dependent cases
		// It's not strictly necessary if we generate all inputs directly, but helps map old default cases.
		ii int
	}

	generateRandomData := func(loop_ii int, size int) []byte {
		data := make([]byte, size)
		// This replicates the logic from the old 'default' case in the switch
		baseVal := 64 + 4*loop_ii
		randRange := 8*loop_ii + 1
		if randRange <= 0 { // prevent panic with rand.Intn if loop_ii is 0 or negative (not expected here)
			randRange = 1
		}
		for i := range data {
			data[i] = byte(baseVal + rand.Intn(randRange))
		}
		return data
	}

	testCases := []entropyTestCase{
		{name: "all_identical_32_bytes_of_2", input: func() []byte {
			v := make([]byte, 32)
			for i := range v {
				v[i] = byte(2)
			}
			return v
		}(), ii: 1},
		{name: "specific_sequence_16_bytes_ascii_like", input: []byte{0x3d, 0x4d, 0x54, 0x47, 0x5a, 0x36, 0x39, 0x26, 0x72, 0x6f, 0x6c, 0x65, 0x3d, 0x70, 0x72, 0x65}, ii: 2},
		{name: "specific_sequence_16_bytes_mixed", input: []byte{0, 0, 32, 15, 252, 16, 0, 16, 0, 7, 255, 252, 224, 0, 31, 255}, ii: 3},
		{name: "alternating_two_symbols_32_bytes", input: func() []byte {
			v := make([]byte, 32)
			for i := range v {
				v[i] = byte(2 + (i & 1))
			}
			return v
		}(), ii: 4},
		{name: "single_byte_42", input: []byte{42}, ii: 5},
		{name: "two_bytes_42_42", input: []byte{42, 42}, ii: 6},
		// Cases previously covered by 'default' (ii from 7 to 19)
		{name: "random_data_256_bytes_ii_7", input: generateRandomData(7, 256), ii: 7},
		{name: "random_data_256_bytes_ii_8", input: generateRandomData(8, 256), ii: 8},
		{name: "random_data_256_bytes_ii_9", input: generateRandomData(9, 256), ii: 9},
		{name: "random_data_256_bytes_ii_10", input: generateRandomData(10, 256), ii: 10},
		{name: "random_data_256_bytes_ii_11", input: generateRandomData(11, 256), ii: 11},
		{name: "random_data_256_bytes_ii_12", input: generateRandomData(12, 256), ii: 12},
		{name: "random_data_256_bytes_ii_13", input: generateRandomData(13, 256), ii: 13},
		{name: "random_data_256_bytes_ii_14", input: generateRandomData(14, 256), ii: 14},
		{name: "random_data_256_bytes_ii_15", input: generateRandomData(15, 256), ii: 15},
		{name: "random_data_256_bytes_ii_16", input: generateRandomData(16, 256), ii: 16},
		{name: "random_data_256_bytes_ii_17", input: generateRandomData(17, 256), ii: 17},
		{name: "random_data_256_bytes_ii_18", input: generateRandomData(18, 256), ii: 18},
		{name: "random_data_256_bytes_ii_19", input: generateRandomData(19, 256), ii: 19},
		// End of 'default' cases
		{name: "empty_input", input: []byte{}, ii: 20},
		{name: "all_256_byte_values", input: func() []byte {
			v := make([]byte, 256)
			for i := 0; i < 256; i++ {
				v[i] = byte(i)
			}
			return v
		}(), ii: 21},
		{name: "single_byte_repeated_1024_times_42", input: func() []byte {
			v := make([]byte, 1024)
			for i := 0; i < 1024; i++ {
				v[i] = byte(42)
			}
			return v
		}(), ii: 22},
		{name: "alternating_AB_1024_bytes", input: func() []byte {
			v := make([]byte, 1024)
			for i := 0; i < 1024; i++ {
				if i%2 == 0 {
					v[i] = byte('A')
				} else {
					v[i] = byte('B')
				}
			}
			return v
		}(), ii: 23},
		{name: "random_data_4096_bytes", input: func() []byte {
			v := make([]byte, 4096)
			for i := 0; i < 4096; i++ {
				v[i] = byte(rand.Intn(256))
			}
			return v
		}(), ii: 24},
		{name: "sparse_data_4096_bytes_mostly_zeros", input: func() []byte {
			v := make([]byte, 4096)
			for i := 1; i < 256; i++ {
				if i*16 < len(v) {
					v[i*16] = byte(i)
				}
			}
			return v
		}(), ii: 25},
	}

	// Test behavior
	for _, tc := range testCases {
		fmt.Printf("\n\nTest %s (Codec: %s, original ii: %d)", tc.name, codecName, tc.ii)
		values := tc.input // Use tc.input as 'values' for minimal changes to core logic

		fmt.Printf("\nOriginal: \n")
		if len(values) > 0 {
			for i := range values {
				fmt.Printf("%d ", values[i])
			}
		} else {
			fmt.Printf("(empty)")
		}

		println()
		fmt.Printf("\nEncoded: \n")
		bs := internal.NewBufferStream()
		obs, _ := bitstream.NewDefaultOutputBitStream(bs, 16384)
		dbgbs, _ := bitstream.NewDebugOutputBitStream(obs, os.Stdout)
		dbgbs.ShowByte(true)
		//dbgbs.Mark(true)
		ec := getEncoder(codecName, dbgbs)

		if ec == nil {
			return errors.New("Cannot create entropy encoder")
		}

		if _, err := ec.Write(values); err != nil {
			fmt.Printf("Error during encoding: %s", err)
			return err
		}

		ec.Dispose()
		dbgbs.Close()
		println()
		fmt.Printf("\nDecoded: \n")

		ibs, _ := bitstream.NewDefaultInputBitStream(bs, 16384)
		ed := getDecoder(codecName, ibs)

		if ed == nil {
			return errors.New("Cannot create entropy decoder")
		}

		ok := true
		values2 := make([]byte, len(values))

		// For empty input, Read might not be called or might behave differently
		// depending on the encoder/decoder implementation.
		// The core idea is that encoding an empty slice and then decoding it
		// should result in an empty slice.
		if len(values) > 0 {
			if _, err := ed.Read(values2); err != nil {
				fmt.Printf("Error during decoding: %s", err)
				return err
			}
		} else if len(values2) != 0 {
			// If input was empty, values2 should also be empty.
			// Some decoders might initialize values2, ensure it's reset or handled.
			// Alternatively, ensure Read handles 0-length buffer correctly.
			// For robustness, if values is empty, values2 should remain/be empty.
			// If Read was called and populated values2 for an empty input, that's a mismatch.
			// However, typical behavior is Read won't read anything if 0 bytes were encoded.
			// Let's assume Read handles it, but if not, values2 might need to be explicitly empty.
			// values2 = []byte{} // If Read doesn't handle it, this might be needed.
		}


		ed.Dispose()

		if len(values) > 0 {
			for i := range values2 {
				fmt.Printf("%v ", values2[i])

				if values[i] != values2[i] {
					ok = false
				}
			}
		} else if len(values2) != 0 {
			// If original was empty, decoded should be empty too.
			ok = false
			fmt.Printf("(decoded non-empty for empty input)")
		}


		if ok == true {
			fmt.Printf("\nIdentical")
		} else {
			fmt.Printf("\n! *** Different *** !")
			return errors.New("Input and inverse are different")
		}

		ibs.Close()
		bs.Close()
		println()
	}

	return nil // Error type remains error, but return nil for success
}

// Helper function to repeat string for test data generation (similar to strings.Repeat)
func strRepeat(s string, count int) string {
	if count <= 0 {
		return ""
	}
	var res []byte
	for i := 0; i < count; i++ {
		res = append(res, s...)
	}
	return string(res)
}

func TestCMCodecSpecificPatterns(t *testing.T) {
	type testCase struct {
		name  string
		input []byte
	}

	// strRepeat is now a global helper function

	testCases := []testCase{
		{
			name:  "RepeatingPattern_ABC",
			// Longer to give CM more context and see better compression. CM uses a rolling context.
			input: []byte(strRepeat("ABC", 20)), // "ABCABC...ABC" (60 bytes)
		},
		{
			name: "ChangingPattern_A30B30C30AA",
			// Tests adaptation to new statistics then re-adaptation to old ones.
			input: []byte(strRepeat("A", 30) + strRepeat("B", 30) + strRepeat("C", 30) + "AA"),
		},
		{
			name:  "AlternatingSymbols_ABAB",
			input: []byte(strRepeat("AB", 30)), // "ABAB...AB" (60 bytes)
		},
		{
			name:  "AllSame_Z50",
			input: []byte(strRepeat("Z", 50)),
		},
		{
			name:  "AlmostAllSame_A50B1",
			// Tests how a single different byte affects compression after a long same-byte run.
			input: []byte(strRepeat("A", 50) + "B"),
		},
		{
			name:  "SingleSymbol",
			input: []byte("X"),
		},
		{
			name:  "TwoDifferentSymbols", // More descriptive
			input: []byte("XY"),
		},
		{
			name:  "TwoSameSymbols",
			input: []byte("XX"),
		},
		{
			name: "EmptyInput",
			input: []byte{},
		},
		{
			// A pattern that might be tricky if context depth is small
			name:  "DistantRepetition_ABCAXYZABC",
			input: []byte("ABCAXYZABCAXYZABCAXYZABCAXYZ"),
		},
		{
			// Test with all 256 byte values, somewhat random but fixed
			name: "AllByteValues",
			input: func() []byte {
				res := make([]byte, 256)
				for i := 0; i < 256; i++ {
					res[i] = byte(i)
				}
				return res
			}(),
		},
		{
			// Test with a mix of high and low frequency symbols
			name: "MixedFrequencies",
			input: []byte(strRepeat("A",50) + strRepeat("B",20) + strRepeat("C",5) + strRepeat("D",1) + strRepeat("A",20)),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing CM Codec with pattern: %s (length %d)", tc.name, len(tc.input))

			bufferStream := internal.NewBufferStream()
			obs, err := bitstream.NewDefaultOutputBitStream(bufferStream, 16384)
			if err != nil {
				t.Fatalf("Failed to create OutputBitStream: %v", err)
			}

			encoder := getEncoder("CM", obs)
			if encoder == nil {
				t.Fatalf("Cannot create CM encoder")
			}

			originalLength := len(tc.input)
			byteWritten, err := encoder.Write(tc.input)
			if err != nil {
				t.Fatalf("Error during encoding: %v", err)
			}
			if byteWritten != originalLength {
				t.Fatalf("Encoder.Write returned %d, expected %d", byteWritten, originalLength)
			}


			encoder.Dispose()
			// It's critical to Close the OutputBitStream to flush any buffered bits to the underlying stream.
			if err := obs.Close(); err != nil {
				t.Fatalf("Error closing OutputBitStream: %v", err)
			}

			encodedLength := bufferStream.Len()
			t.Logf("Original length: %d, Encoded length: %d", originalLength, encodedLength)

			// Soft check for compression on predictable patterns
			// Allow for some overhead, especially for short inputs.
			// For CM, highly predictable patterns should generally compress.
			if originalLength > 10 { // Only check for reasonable length inputs
				isPredictable := tc.name == "RepeatingPattern_ABC" ||
								tc.name == "AlternatingSymbols_ABAB" ||
								tc.name == "AllSame_Z50" ||
								tc.name == "DistantRepetition_ABCAXYZABC"
				
				if isPredictable && encodedLength >= originalLength {
					t.Logf("Warning: Predictable pattern '%s' did not compress. Original: %d, Encoded: %d. This might be acceptable for some short patterns or specific CM configurations.", tc.name, originalLength, encodedLength)
				}
				if tc.name == "AllByteValues" && encodedLength < originalLength {
					t.Logf("Info: 'AllByteValues' (random-like) compressed from %d to %d. This is good.", originalLength, encodedLength)
				}
			}


			// Prepare for decoding
			ibs, err := bitstream.NewDefaultInputBitStream(bufferStream, 16384)
			if err != nil {
				t.Fatalf("Failed to create InputBitStream: %v", err)
			}

			decoder := getDecoder("CM", ibs)
			if decoder == nil {
				t.Fatalf("Cannot create CM decoder")
			}

			decodedBytes := make([]byte, originalLength)
			if originalLength > 0 {
				bytesRead, err := decoder.Read(decodedBytes)
				if err != nil {
					t.Fatalf("Error during decoding: %v", err)
				}
				if bytesRead != originalLength {
					t.Fatalf("Decoder.Read returned %d, expected %d", bytesRead, originalLength)
				}
			} else {
				// For 0 length input, Read shouldn't be called with a non-empty buffer,
				// or if it is, it should read 0 bytes and not error.
				// The current make([]byte, 0) handles this.
				// If Read were called, it should return 0.
			}


			decoder.Dispose()
			if err := ibs.Close(); err != nil { // Input stream should also be closed
				t.Fatalf("Error closing InputBitStream: %v", err)
			}

			// Verify
			if !bytes.Equal(tc.input, decodedBytes) {
				t.Errorf("Decoded data does not match original for test case: %s.\nOriginal (len %d): %q\nDecoded  (len %d): %q",
					tc.name, len(tc.input), string(tc.input), len(decodedBytes), string(decodedBytes))
				// For very long strings, you might not want to print the full content.
				// Consider printing lengths and first/last N bytes if mismatch.
			}
		})
	}
}

