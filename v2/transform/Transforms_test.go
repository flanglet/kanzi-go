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

package transform

import (
	"bytes" // Added for bytes.Repeat and bytes.Equal
	"fmt"
	"math/rand"
	"testing"
	"time"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

// transformTestCase holds data for a single test case for testTransformCorrectness.
type transformTestCase struct {
	name      string
	inputData []byte
	rng       int  // Specific rng value for this test case, if different from default.
	skip      bool // Flag to skip this test case if necessary
}

// specificTransformTestCase holds data for specific codec tests.
type specificTransformTestCase struct {
	name        string
	transformID string // e.g., "LZ", "RLT"
	input       []byte
	description string // What this test case is trying to achieve
}

func getTransform(name string) (kanzi.ByteTransform, error) {
	ctx := make(map[string]any)
	ctx["transform"] = name
	ctx["bsVersion"] = uint(6)

	switch name {
	case "LZ":
		ctx["lz"] = LZ_TYPE
		res, err := NewLZCodecWithCtx(&ctx)
		return res, err
	case "LZX":
		ctx["lz"] = LZX_TYPE
		res, err := NewLZCodecWithCtx(&ctx)
		return res, err
	case "LZP":
		ctx["lz"] = LZP_TYPE
		res, err := NewLZCodecWithCtx(&ctx)
		return res, err
	case "ALIAS":
		res, err := NewAliasCodecWithCtx(&ctx)
		return res, err
	case "NONE":
		res, err := NewNullTransformWithCtx(&ctx)
		return res, err
	case "ZRLT":
		res, err := NewZRLTWithCtx(&ctx)
		return res, err
	case "RLT":
		res, err := NewRLTWithCtx(&ctx)
		return res, err
	case "SRT":
		res, err := NewSRTWithCtx(&ctx)
		return res, err
	case "ROLZ", "ROLZX":
		res, err := NewROLZCodecWithCtx(&ctx)
		return res, err
	case "RANK":
		res, err := NewSBRT(SBRT_MODE_RANK)
		return res, err
	case "MTFT":
		res, err := NewSBRT(SBRT_MODE_MTF)
		return res, err
	case "MM":
		res, err := NewFSDCodecWithCtx(&ctx)
		return res, err
	case "EXE":
		res, err := NewEXECodecWithCtx(&ctx)
		return res, err
	case "TEXT":
		res, err := NewTextCodecWithCtx(&ctx)
		return res, err
	case "UTF":
		res, err := NewUTFCodecWithCtx(&ctx)
		return res, err
	default:
		panic(fmt.Errorf("No such transform: '%s'", name))
	}
}

// Generic Tests (using testTransformCorrectness)
func TestLZ(t *testing.T)        { testTransformCorrectness("LZ", t) }
func TestLZX(t *testing.T)       { testTransformCorrectness("LZX", t) }
func TestLZP(t *testing.T)       { testTransformCorrectness("LZP", t) }
func TestROLZ(t *testing.T)      { testTransformCorrectness("ROLZ", t) }
func TestROLZX(t *testing.T)     { testTransformCorrectness("ROLZX", t) }
func TestCopy(t *testing.T)      { testTransformCorrectness("NONE", t) }
func TestAlias(t *testing.T)     { testTransformCorrectness("ALIAS", t) }
func TestZRLT(t *testing.T)      { testTransformCorrectness("ZRLT", t) }
func TestRLT(t *testing.T)       { testTransformCorrectness("RLT", t) }
func TestSRT(t *testing.T)       { testTransformCorrectness("SRT", t) }
func TestMM(t *testing.T)        { testTransformCorrectness("MM", t) }
func TestRank(t *testing.T)      { testTransformCorrectness("RANK", t) }
func TestMTFT(t *testing.T)      { testTransformCorrectness("MTFT", t) }
func TestEXECodec(t *testing.T)  { testTransformCorrectness("EXE", t) }
func TestTextCodec(t *testing.T) { testTransformCorrectness("TEXT", t) }
func TestUTFCodec(t *testing.T)  { testTransformCorrectness("UTF", t) }

// generateTransformTestCases creates the suite of generic test cases.
func generateTransformTestCases(transformName string) []transformTestCase {
	testCases := []transformTestCase{}
	defaultRng := 256

	testCases = append(testCases, transformTestCase{name: "EmptyInput", inputData: []byte{}, rng: defaultRng})
	testCases = append(testCases, transformTestCase{name: "SingleByteA", inputData: []byte{'A'}, rng: defaultRng})
	testCases = append(testCases, transformTestCase{name: "TwoIdenticalBytesAA", inputData: []byte{'A', 'A'}, rng: defaultRng})
	testCases = append(testCases, transformTestCase{name: "TwoDifferentBytesAB", inputData: []byte{'A', 'B'}, rng: defaultRng})

	allByteValues := make([]byte, 256)
	for i := 0; i < 256; i++ {
		allByteValues[i] = byte(i)
	}
	testCases = append(testCases, transformTestCase{name: "All256ByteValues", inputData: allByteValues, rng: defaultRng})

	testCases = append(testCases, transformTestCase{
		name:      "Original_SpecificSequence_0",
		inputData: []byte{0, 1, 2, 2, 2, 2, 7, 9, 9, 16, 16, 16, 1, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3},
		rng:       defaultRng,
	})

	input1 := make([]byte, 80000)
	for i := range input1 {
		input1[i] = 8
	}
	input1[0] = 1
	testCases = append(testCases, transformTestCase{name: "Original_AllEights_OneOne_80k_1", inputData: input1, rng: defaultRng})
	testCases = append(testCases, transformTestCase{name: "Original_ShortRepeats_2", inputData: []byte{0, 0, 1, 1, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3}, rng: defaultRng})

	for i := 3; i < 6; i++ {
		size := 1 << uint(i+6)
		input := make([]byte, size)
		currentRng := defaultRng
		if transformName == "ZRLT" {
			currentRng = 5
		} else {
			currentRng = 100
		}
		for j := range input {
			val := byte(rand.Intn(currentRng))
			if val >= 33 {
				val = 0
			}
			input[j] = val
		}
		testCases = append(testCases, transformTestCase{name: fmt.Sprintf("Original_LotsOfZeros_%d_size%d", i, size), inputData: input, rng: currentRng})
	}

	input6 := make([]byte, 512)
	currentRng6 := defaultRng
	if transformName == "ZRLT" {
		currentRng6 = 5
	}
	for i := 0; i < 20 && i < len(input6); i++ {
		input6[i] = 0
	}
	for i := 20; i < len(input6); i++ {
		input6[i] = byte(rand.Intn(currentRng6))
	}
	testCases = append(testCases, transformTestCase{name: "Original_Random_WithInitialZeros_6_size512", inputData: input6, rng: currentRng6})

	rndGen := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 7; i < 50; i++ {
		inputNext := make([]byte, 1024)
		currentRngNext := defaultRng
		if transformName == "ZRLT" {
			currentRngNext = 5
		}
		for j := 0; j < 20 && j < len(inputNext); j++ {
			inputNext[j] = 0
		}
		idx := 20
		for idx < len(inputNext) {
			length := rndGen.Intn(120)
			if length%3 == 0 {
				length = 1
			}
			if length == 0 {
				length = 1
			}
			val := byte(rand.Intn(currentRngNext))
			end := idx + length
			if end > len(inputNext) {
				end = len(inputNext)
			}
			for j := idx; j < end; j++ {
				inputNext[j] = val
			}
			idx += length
			if idx >= len(inputNext) && length > 0 {
				break
			}
		}
		testCases = append(testCases, transformTestCase{name: fmt.Sprintf("Original_RandomLengthsRandomValues_%d_size1024", i), inputData: inputNext, rng: currentRngNext})
	}
	return testCases
}

// testTransformCorrectness is the generic test function.
func testTransformCorrectness(transformName string, t *testing.T) {
	testCases := generateTransformTestCases(transformName)

	if testing.Verbose() {
		fmt.Println()
		fmt.Printf("=== Testing %s (Generic Cases) === ", transformName)
	}

	for _, tc := range testCases {
		if tc.skip {
			if testing.Verbose() {
				fmt.Printf(" Skipping Test Case: %s ", tc.name)
			}
			continue
		}

		currentTestCase := tc
		t.Run(currentTestCase.name, func(t_run *testing.T) {
			if testing.Verbose() {
				fmt.Printf(" Test Case: %s ", currentTestCase.name)
			}

			input := currentTestCase.inputData
			size := len(input)

			fwdTransform, err := getTransform(transformName)
			if err != nil {
				t_run.Errorf("Cannot create transform '%s': %v", transformName, err)
				return
			}

			output := make([]byte, fwdTransform.MaxEncodedLen(size))
			reverse := make([]byte, size)
			for i := range output {
				output[i] = 0xAA
			}

			if testing.Verbose() {
				fmt.Printf("Original (%d bytes): ", size)
				if size > 0 && size <= 64 {
					for i := range input {
						fmt.Printf("%v ", input[i])
					}
					fmt.Println()
				} else if size > 0 {
					fmt.Printf("(data too long to print, first byte: %v) ", input[0])
				} else {
					fmt.Println("(empty input)")
				}
			}

			srcIdx, dstIdx, errFwd := fwdTransform.Forward(input, output)

			if errFwd != nil {
				if testing.Verbose() {
					fmt.Printf("Forward transform error or skip for %s: %v (src: %d, dst: %d) ", transformName, errFwd, srcIdx, dstIdx)
				}
				return
			}

			dataToDecode := output[0:dstIdx]

			if testing.Verbose() {
				fmt.Printf("Coded (%d bytes): ", dstIdx)
				if dstIdx > 0 && dstIdx <= 64 {
					for i := range dataToDecode {
						fmt.Printf("%v ", dataToDecode[i])
					}
					fmt.Println()
				} else if dstIdx > 0 {
					fmt.Printf("(data too long to print, first byte: %v) ", dataToDecode[0])
				} else {
					fmt.Println("(empty output)")
				}
			}

			if testing.Verbose() {
				if size > 0 && dstIdx > 0 {
					fmt.Printf("(Compression ratio: %v%%) ", int(dstIdx)*100/size)
				} else if size == 0 && dstIdx == 0 { /* Correctly transformed empty to empty */
				} else if size > 0 && dstIdx == 0 && errFwd == nil {
					fmt.Printf("(Resulted in empty output from non-empty input without error) ")
				}
			}

			invTransform, err := getTransform(transformName)
			if err != nil {
				t_run.Errorf("Cannot create transform '%s' for inverse: %v", transformName, err)
				return
			}

			_, _, errInv := invTransform.Inverse(dataToDecode, reverse)

			if errInv != nil {
				if len(dataToDecode) == 0 && size == 0 {
					if testing.Verbose() {
						fmt.Printf("Inverse transform error for %s with empty input: %v. This may be acceptable.  ", transformName, errInv)
					}
				} else {
					t_run.Errorf("Decoding error for %s: %v. Input to Inverse was %d bytes.", transformName, errInv, len(dataToDecode))
					return
				}
			}

			if testing.Verbose() {
				fmt.Printf("Decoded (%d bytes): ", len(reverse))
			}

			if len(input) != len(reverse) {
				if !(size > 0 && dstIdx == 0 && errFwd == nil && len(reverse) == 0) {
					t_run.Errorf("Length mismatch for transform %s. Original: %d, Decoded: %d", transformName, len(input), len(reverse))
					return
				} else {
					t_run.Errorf("Data mismatch for transform %s: Original non-empty data resulted in empty data after forward/inverse. Original len: %d", transformName, len(input))
					return
				}
			}

			idx := -1
			for i := range input {
				if input[i] != reverse[i] {
					idx = i
					break
				}
			}

			if idx == -1 {
				if testing.Verbose() {
					if size > 0 && size <= 64 {
						for i := range reverse {
							fmt.Printf("%v ", reverse[i])
						}
						fmt.Println()
					}
					fmt.Println("Identical")
				}
			} else {
				t_run.Errorf("Data mismatch after inverse transform %s at index %v. Original: %v, Decoded: %v", transformName, idx, input[idx], reverse[idx])
				start := idx - 5
				if start < 0 {
					start = 0
				}
				endOrig := idx + 5
				if endOrig > len(input) {
					endOrig = len(input)
				}
				endRev := idx + 5
				if endRev > len(reverse) {
					endRev = len(reverse)
				}

				if testing.Verbose() {
					fmt.Printf("Original around %d: %v ", idx, input[start:endOrig])
					fmt.Printf("Decoded around %d: %v ", idx, reverse[start:endRev])
				}
				return
			}
		})
	}
	if testing.Verbose() {
		fmt.Println()
	}
}

// runSpecificTransformTest executes a single specific test case.
func runSpecificTransformTest(t *testing.T, tc specificTransformTestCase) {
	t.Helper() // Marks this function as a test helper

	if testing.Verbose() {
		fmt.Printf(" Running specific test: %s for %s (%s) ", tc.name, tc.transformID, tc.description)
	}

	fwdTransform, err := getTransform(tc.transformID)
	if err != nil {
		t.Errorf("[%s] Cannot create transform '%s': %v", tc.name, tc.transformID, err)
		return
	}

	input := tc.input
	size := len(input)
	output := make([]byte, fwdTransform.MaxEncodedLen(size))
	reverse := make([]byte, size)

	if testing.Verbose() {
		fmt.Printf("Original (%d bytes): ", size)
		if size <= 128 {
			fmt.Printf("%v ", input)
		} else {
			fmt.Printf("(first 64 bytes: %v...) ", input[:64])
		}
	}

	srcIdx, dstIdx, errFwd := fwdTransform.Forward(input, output)
	if errFwd != nil {
		fmt.Printf("[%s] Forward transform error for %s: %v (src: %d, dst: %d)", tc.name, tc.transformID, errFwd, srcIdx, dstIdx)
		return
	}

	dataToDecode := output[0:dstIdx]

	if testing.Verbose() {
		fmt.Printf("Coded (%d bytes): ", dstIdx)
		if dstIdx <= 128 {
			fmt.Printf("%v ", dataToDecode)
		} else {
			fmt.Printf("(first 64 bytes: %v...) ", dataToDecode[:64])
		}
		if size > 0 && dstIdx > 0 {
			fmt.Printf("(Compression ratio: %v%%) ", int(dstIdx)*100/size)
		}
	}

	// Allow forward to produce empty from non-empty if that's the transform's nature (e.g. ZRLT on non-zeros)
	// The crucial part is whether inverse reconstructs the original.
	if size > 0 && dstIdx == 0 && errFwd == nil && testing.Verbose() {
		fmt.Printf("[%s] Warning: %s transformed non-empty input to empty output without error.  ", tc.name, tc.transformID)
	}

	invTransform, err := getTransform(tc.transformID)
	if err != nil {
		t.Errorf("[%s] Cannot create transform '%s' for inverse: %v", tc.name, tc.transformID, err)
		return
	}

	_, _, errInv := invTransform.Inverse(dataToDecode, reverse)
	if errInv != nil {
		t.Errorf("[%s] Decoding error for %s: %v. Input to Inverse was %d bytes.", tc.name, tc.transformID, errInv, len(dataToDecode))
		return
	}

	if !bytes.Equal(input, reverse) {
		t.Errorf("[%s] Data mismatch after inverse for %s. Expected: %v, Got: %v", tc.name, tc.transformID, input, reverse)
		// Print more details for easier debugging if lengths are same but content differs
		if len(input) == len(reverse) {
			for i := range input {
				if input[i] != reverse[i] {
					if testing.Verbose() {
						fmt.Printf("First mismatch at index %d: Expected %v, Got %v ", i, input[i], reverse[i])
					}

					start := i - 10
					if start < 0 {
						start = 0
					}
					end := i + 10
					if end > len(input) {
						end = len(input)
					}
					if testing.Verbose() {
						fmt.Printf("Expected around mismatch: %v ", input[start:end])
						fmt.Printf("Got around mismatch:      %v ", reverse[start:end])
					}

					break
				}
			}
		} else {
			if testing.Verbose() {
				fmt.Printf("Length mismatch: Expected %d, Got %d ", len(input), len(reverse))
			}
		}
	} else {
		if testing.Verbose() {
			fmt.Printf("Identical reconstruction for %s.  ", tc.name)
		}
	}
}

// TestLZCodecSpecifics provides targeted tests for LZ, LZX, and LZP codecs.
func TestLZCodecSpecifics(t *testing.T) {
	lzTransforms := []string{"LZ", "LZX", "LZP"}

	baseTestCases := []struct {
		name        string
		description string
		input       []byte
	}{
		{
			name:        "HighlyRepetitiveShortPattern",
			description: "Tests compression of short, highly repeated patterns.",
			input:       bytes.Repeat([]byte{'a', 'b', 'c'}, 100), // "abcabcabc..."
		},
		{
			name:        "HighlyRepetitiveLongPattern",
			description: "Tests compression of longer, highly repeated patterns.",
			input:       bytes.Repeat([]byte("abcdefghijklmnop"), 50), // "abcdefghijklmnopabcdefghijklmnop..."
		},
		{
			name:        "DistantMatches",
			description: "Tests if distant matches are found.",
			input:       append(append(bytes.Repeat([]byte{'X'}, 500), []byte("UNIQUEPATTERN")...), bytes.Repeat([]byte{'Y'}, 500)...),
			// Expect "UNIQUEPATTERN" to be potentially non-optimal if dictionary is small or flushed.
			// This test is more about correctness with varying data.
		},
		{
			name:        "LongerDistantMatch",
			description: "A pattern, then lots of other data, then the pattern again.",
			input:       append(append([]byte("REPEAT_ME_PLZ_12345"), bytes.Repeat([]byte{'z'}, 2048)...), []byte("REPEAT_ME_PLZ_12345")...),
		},
		{
			name:        "IncompressibleRandom_1k",
			description: "Tests behavior with incompressible random data (low compression expected).",
			input: func() []byte {
				data := make([]byte, 1024)
				for i := range data {
					data[i] = byte(rand.Intn(256))
				}
				return data
			}(),
		},
		{
			name:        "AllSameBytes_1k",
			description: "Tests with a kilobyte of the same byte (high compression expected).",
			input:       bytes.Repeat([]byte{'Z'}, 1024),
		},
		{
			name:        "AlternatingTwoBytes_1k",
			description: "Tests with alternating two bytes (e.g., ABABAB...).",
			input: func() []byte {
				data := make([]byte, 1024)
				for i := 0; i < len(data); i += 2 {
					data[i] = 'A'
					if i+1 < len(data) {
						data[i+1] = 'B'
					}
				}
				return data
			}(),
		},
		{
			name:        "EmptyInputSpecific",
			description: "Specific test for empty input.",
			input:       []byte{},
		},
		{
			name:        "SingleByteInputSpecific",
			description: "Specific test for a single byte input.",
			input:       []byte{'X'},
		},
	}

	for _, transformID := range lzTransforms {
		t.Run(transformID, func(t_run *testing.T) {
			for _, tc := range baseTestCases {
				// Create a full specificTransformTestCase
				specificTC := specificTransformTestCase{
					name:        fmt.Sprintf("%s_%s", transformID, tc.name),
					transformID: transformID,
					input:       tc.input,
					description: tc.description,
				}
				// Run the test using a sub-test for each case to ensure isolation
				t_run.Run(tc.name, func(t_case *testing.T) {
					runSpecificTransformTest(t_case, specificTC)
				})
			}
		})
	}
}

func makeUTFInput(size int) []byte {
	input := make([]byte, size)
	n := size / 2

	for i := 0; i < n; i++ {
		input[2*i] = 0xC3
		input[2*i+1] = 0xA9 // "é"
	}

	if size&1 != 0 {
		input[size-1] = 'a'
	}

	return input
}

func makeTextInput(size int) []byte {
	input := make([]byte, size)
	sample := []byte("The quick brown fox jumps over the lazy dog. This is a text block used for transform round-trip tests.\r\n")

	for i := range input {
		input[i] = sample[i%len(sample)]
	}

	return input
}

func TestUTFCodecMinBlockAndRoundTrip(t *testing.T) {
	fwd, err := NewUTFCodec()

	if err != nil {
		t.Fatalf("cannot create UTF codec: %v", err)
	}

	small := makeUTFInput(_UTF_MIN_BLOCKSIZE - 1)
	dst := make([]byte, fwd.MaxEncodedLen(len(small)))

	if _, _, err = fwd.Forward(small, dst); err == nil {
		t.Fatalf("expected UTF forward to fail below min block size (%d)", _UTF_MIN_BLOCKSIZE)
	}

	input := makeUTFInput(4 * _UTF_MIN_BLOCKSIZE)
	dst = make([]byte, fwd.MaxEncodedLen(len(input)))
	reverse := make([]byte, len(input))
	_, dstIdx, err := fwd.Forward(input, dst)

	if err != nil {
		t.Fatalf("UTF forward failed for valid input: %v", err)
	}

	inv, err := NewUTFCodec()

	if err != nil {
		t.Fatalf("cannot create UTF codec inverse: %v", err)
	}

	if _, _, err = inv.Inverse(dst[:dstIdx], reverse); err != nil {
		t.Fatalf("UTF inverse failed: %v", err)
	}

	if !bytes.Equal(input, reverse) {
		t.Fatalf("UTF round-trip mismatch")
	}
}

func TestTextCodecMinBlockAndRoundTrip(t *testing.T) {
	fwd, err := NewTextCodec()

	if err != nil {
		t.Fatalf("cannot create Text codec: %v", err)
	}

	small := makeTextInput(_TC_MIN_BLOCK_SIZE - 1)
	dst := make([]byte, fwd.MaxEncodedLen(len(small)))

	if _, _, err = fwd.Forward(small, dst); err == nil {
		t.Fatalf("expected Text forward to fail below min block size (%d)", _TC_MIN_BLOCK_SIZE)
	}

	input := makeTextInput(4 * _TC_MIN_BLOCK_SIZE)
	dst = make([]byte, fwd.MaxEncodedLen(len(input)))
	reverse := make([]byte, len(input))
	_, dstIdx, err := fwd.Forward(input, dst)

	if err != nil {
		t.Fatalf("Text forward failed for valid input: %v", err)
	}

	inv, err := NewTextCodec()

	if err != nil {
		t.Fatalf("cannot create Text codec inverse: %v", err)
	}

	if _, _, err = inv.Inverse(dst[:dstIdx], reverse); err != nil {
		t.Fatalf("Text inverse failed: %v", err)
	}

	if !bytes.Equal(input, reverse) {
		t.Fatalf("Text round-trip mismatch")
	}
}
