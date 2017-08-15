/*
Copyright 2011-2017 Frederic Langlet
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

package main

import (
	"fmt"
	"io"
	"kanzi"
	kio "kanzi/io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	COMP_DEFAULT_BUFFER_SIZE = 32768
	COMP_DEFAULT_BLOCK_SIZE  = 1024 * 1024
	WARN_EMPTY_INPUT         = -128
)

// Main block compressor struct
type BlockCompressor struct {
	verbosity    uint
	overwrite    bool
	checksum     bool
	inputName    string
	outputName   string
	entropyCodec string
	transform    string
	blockSize    uint
	level        int // command line compression level
	jobs         uint
	listeners    []kanzi.Listener
	cpuProf      string
}

func NewBlockCompressor(argsMap map[string]interface{}) (*BlockCompressor, error) {
	this := new(BlockCompressor)
	this.listeners = make([]kanzi.Listener, 0)
	this.level = argsMap["level"].(int)
	delete(argsMap, "level")

	this.verbosity = argsMap["verbose"].(uint)
	delete(argsMap, "verbose")

	if force, prst := argsMap["overwrite"]; prst == true {
		this.overwrite = force.(bool)
		delete(argsMap, "overwrite")
	} else {
		this.overwrite = false
	}

	this.inputName = argsMap["inputName"].(string)
	delete(argsMap, "inputName")
	this.outputName = argsMap["outputName"].(string)
	delete(argsMap, "outputName")
	strTransf := ""
	strCodec := ""

	if this.level >= 0 {
		tranformAndCodec := getTransformAndCodec(this.level)
		tokens := strings.Split(tranformAndCodec, "&")
		strTransf = tokens[0]
		strCodec = tokens[1]
	} else {
		if codec, prst := argsMap["entropy"]; prst == true {
			strCodec = codec.(string)
			delete(argsMap, "entropy")
		} else {
			strCodec = "HUFFMAN"
		}
	}

	this.entropyCodec = strCodec

	if block, prst := argsMap["block"]; prst == true {
		this.blockSize = block.(uint)
		delete(argsMap, "block")
	} else {
		this.blockSize = COMP_DEFAULT_BLOCK_SIZE
	}

	if len(strTransf) == 0 {
		if transf, prst := argsMap["transform"]; prst == true {
			strTransf = transf.(string)
			delete(argsMap, "transform")
		} else {
			strTransf = "BWT+MTFT+ZRLT"
		}
	}

	// Extract transform names. Curate input (EG. NONE+NONE+xxxx => xxxx)
	this.transform = kio.GetByteFunctionName(kio.GetByteFunctionType(strTransf))

	if check, prst := argsMap["checksum"]; prst == true {
		this.checksum = check.(bool)
		delete(argsMap, "checksum")
	} else {
		this.checksum = false
	}

	this.jobs = argsMap["jobs"].(uint)
	delete(argsMap, "jobs")

	if prof, prst := argsMap["cpuProf"]; prst == true {
		this.cpuProf = prof.(string)
		delete(argsMap, "cpuProf")
	} else {
		this.cpuProf = ""
	}

	if this.verbosity > 2 {
		if listener, err := kio.NewInfoPrinter(this.verbosity, kio.ENCODING, os.Stdout); err == nil {
			this.AddListener(listener)
		}
	}

	if this.verbosity > 0 && len(argsMap) > 0 {
		for k, _ := range argsMap {
			bc_printOut("Ignoring invalid option ["+k+"]", this.verbosity > 0)
		}
	}

	return this, nil
}

func (this *BlockCompressor) AddListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

func (this *BlockCompressor) RemoveListener(bl kanzi.Listener) bool {
	for i, e := range this.listeners {
		if e == bl {
			this.listeners = append(this.listeners[:i-1], this.listeners[i+1:]...)
			return true
		}
	}

	return false
}

func (this *BlockCompressor) CpuProf() string {
	return this.cpuProf
}

// Return exit code, number of bits written
func (this *BlockCompressor) Call() (int, uint64) {
	var msg string
	printFlag := this.verbosity > 2
	bc_printOut("Kanzi 1.1 (C) 2017,  Frederic Langlet", this.verbosity >= 1)
	bc_printOut("Input file name set to '"+this.inputName+"'", printFlag)
	bc_printOut("Output file name set to '"+this.outputName+"'", printFlag)
	msg = fmt.Sprintf("Block size set to %d bytes", this.blockSize)
	bc_printOut(msg, printFlag)
	msg = fmt.Sprintf("Verbosity set to %v", this.verbosity)
	bc_printOut(msg, printFlag)
	msg = fmt.Sprintf("Overwrite set to %t", this.overwrite)
	bc_printOut(msg, printFlag)
	msg = fmt.Sprintf("Checksum set to %t", this.checksum)
	bc_printOut(msg, printFlag)

	if this.level < 0 {
		w1 := "no"

		if this.transform != "NONE" {
			w1 = this.transform
		}

		msg = fmt.Sprintf("Using %s transform (stage 1)", w1)
		bc_printOut(msg, printFlag)
		w2 := "no"

		if this.entropyCodec != "NONE" {
			w2 = this.entropyCodec
		}

		msg = fmt.Sprintf("Using %s entropy codec (stage 2)", w2)
		bc_printOut(msg, printFlag)
	} else {
		msg = fmt.Sprintf("Compression level set to %v", this.level)
		bc_printOut(msg, printFlag)
	}

	if this.jobs > 0 {
		prefix := ""

		if this.jobs > 1 {
			prefix = "s"
		}

		msg = fmt.Sprintf("Using %d job%s", this.jobs, prefix)
		bc_printOut(msg, printFlag)
	}

	written := uint64(0)
	var output io.WriteCloser

	if strings.ToUpper(this.outputName) == "NONE" {
		output, _ = kio.NewNullOutputStream()
	} else if strings.ToUpper(this.outputName) == "STDOUT" {
		output = os.Stdout
	} else {
		var err error

		if output, err = os.OpenFile(this.outputName, os.O_RDWR, 666); err == nil {
			// File exists
			output.Close()

			if this.overwrite == false {
				fmt.Print("The output file exists and the 'force' command ")
				fmt.Println("line option has not been provided")
				return kio.ERR_OVERWRITE_FILE, written
			}

			path1, _ := filepath.Abs(this.inputName)
			path2, _ := filepath.Abs(this.outputName)

			if path1 == path2 {
				fmt.Print("The input and output files must be different")
				return kio.ERR_CREATE_FILE, written
			}
		}

		output, err = os.Create(this.outputName)

		if err != nil {
			fmt.Printf("Cannot open output file '%v' for writing: %v\n", this.outputName, err)
			return kio.ERR_CREATE_FILE, written
		}

		defer func() {
			output.Close()
		}()

	}

	ctx := make(map[string]interface{})
	ctx["blockSize"] = this.blockSize
	ctx["checksum"] = this.checksum
	ctx["jobs"] = this.jobs
	ctx["codec"] = this.entropyCodec
	ctx["transform"] = this.transform
	cos, err := kio.NewCompressedOutputStream(output, ctx)

	if err != nil {
		if ioerr, isIOErr := err.(kio.IOError); isIOErr == true {
			fmt.Printf("%s\n", ioerr.Error())
			return ioerr.ErrorCode(), written
		}

		fmt.Printf("Cannot create compressed stream: %s\n", err.Error())
		return kio.ERR_CREATE_COMPRESSOR, written

	}

	defer func() {
		cos.Close()
	}()

	var input io.ReadCloser

	if strings.ToUpper(this.inputName) == "STDIN" {
		input = os.Stdin
	} else {
		var err error

		if input, err = os.Open(this.inputName); err != nil {
			fmt.Printf("Cannot open input file '%v': %v\n", this.inputName, err)
			return kio.ERR_OPEN_FILE, written
		}

		defer func() {
			input.Close()
		}()
	}

	for _, bl := range this.listeners {
		cos.AddListener(bl)
	}

	// Encode
	length := 0
	read := int64(0)
	printFlag = this.verbosity > 1
	bc_printOut("Encoding ...", printFlag)
	written = cos.GetWritten()

	buffer := make([]byte, COMP_DEFAULT_BUFFER_SIZE)

	if len(this.listeners) > 0 {
		evt := kanzi.NewEvent(kanzi.EVT_COMPRESSION_START, -1, 0, 0, false)
		bc_notifyListeners(this.listeners, evt)
	}

	before := time.Now()
	length, err = input.Read(buffer)

	for length > 0 {
		if err != nil {
			fmt.Printf("Failed to read block from file '%v': %v\n", this.inputName, err)
			return kio.ERR_READ_FILE, written
		}

		read += int64(length)

		if _, err = cos.Write(buffer[0:length]); err != nil {
			if ioerr, isIOErr := err.(kio.IOError); isIOErr == true {
				fmt.Printf("%s\n", ioerr.Error())
				return ioerr.ErrorCode(), written
			}

			fmt.Printf("An unexpected condition happened. Exiting ...\n%v\n", err.Error())
			return kio.ERR_PROCESS_BLOCK, written
		}

		length, err = input.Read(buffer)
	}

	if read == 0 {
		fmt.Println("Empty input file ... nothing to do")
		return WARN_EMPTY_INPUT, written
	}

	// Close streams to ensure all data are flushed
	// Deferred close is fallback for error paths
	if err := cos.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return kio.ERR_PROCESS_BLOCK, written
	}

	after := time.Now()
	delta := after.Sub(before).Nanoseconds() / 1000000 // convert to ms

	bc_printOut("", this.verbosity >= 1)
	msg = fmt.Sprintf("Encoding:          %d ms", delta)
	bc_printOut(msg, printFlag)
	msg = fmt.Sprintf("Input size:        %d", read)
	bc_printOut(msg, printFlag)
	msg = fmt.Sprintf("Output size:       %d", cos.GetWritten())
	bc_printOut(msg, printFlag)
	msg = fmt.Sprintf("Ratio:             %f", float64(cos.GetWritten())/float64(read))
	bc_printOut(msg, printFlag)
	msg = fmt.Sprintf("Encoding: %v => %v bytes in %v ms", read, cos.GetWritten(), delta)
	bc_printOut(msg, this.verbosity == 1)

	if delta > 0 {
		msg = fmt.Sprintf("Throughput (KB/s): %d", ((read*int64(1000))>>10)/delta)
		bc_printOut(msg, printFlag)
	}

	bc_printOut("", this.verbosity >= 1)

	if len(this.listeners) > 0 {
		evt := kanzi.NewEvent(kanzi.EVT_COMPRESSION_END, -1, int64(cos.GetWritten()), 0, false)
		bc_notifyListeners(this.listeners, evt)
	}

	return 0, cos.GetWritten()
}

func bc_printOut(msg string, print bool) {
	if print == true {
		fmt.Println(msg)
	}
}

func bc_notifyListeners(listeners []kanzi.Listener, evt *kanzi.Event) {
	defer func() {
		if r := recover(); r != nil {
			// Ignore exceptions in listeners
		}
	}()

	for _, bl := range listeners {
		bl.ProcessEvent(evt)
	}
}

func getTransformAndCodec(level int) string {
	switch level {
	case 0:
		return "NONE&NONE"

	case 1:
		return "TEXT+LZ4&HUFFMAN"

	case 2:
		return "BWT+RANK+ZRLT&ANS0"

	case 3:
		return "BWT+RANK+ZRLT&FPAQ"

	case 4:
		return "BWT&CM"

	case 5:
		return "RLT+TEXT&TPAQ"

	default:
		return "Unknown&Unknown"
	}
}
