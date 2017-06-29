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
	"flag"
	"fmt"
	"io"
	kio "kanzi/io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

const (
	COMP_DEFAULT_BUFFER_SIZE = 32768
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
	jobs         uint
	listeners    []kio.BlockListener
	cpuProf      string
}

func NewBlockCompressor() (*BlockCompressor, error) {
	this := new(BlockCompressor)

	// Define flags
	var help = flag.Bool("help", false, "display the help message")
	var verbose = flag.Int("verbose", 1, "set the verbosity level [0..4]")
	var overwrite = flag.Bool("overwrite", false, "overwrite the output file if it already exists")
	var inputName = flag.String("input", "", "mandatory name of the input file to encode or 'stdin'")
	var outputName = flag.String("output", "", "optional name of the output file (defaults to <input.knz>), or 'none' or 'stdout'")
	var blockSize = flag.String("block", "1048576", "size of the input blocks, multiple of 16, max 1 GB (transform dependent), min 1 KB, default 1 MB")
	var entropy = flag.String("entropy", "Huffman", "entropy codec to use [None|Huffman*|ANS|Range|PAQ|FPAQ|CM]")
	var transforms = flag.String("transform", "BWT+MTFT+ZRLT", "transform to use [None|BWT*|BWTS|Snappy|LZ4|RLT|ZRLT|MTFT|RANK|TIMESTAMP]")
	var cksum = flag.Bool("checksum", false, "enable block checksum")
	var tasks = flag.Int("jobs", 1, "number of concurrent jobs")
	var cpuprofile = flag.String("cpuprof", "", "write cpu profile to file")

	// Parse
	flag.Parse()

	if *help == true {
		bc_printOut("-help                : display this message", true)
		bc_printOut("-verbose=<level>     : set the verbosity level [0..4]", true)
		bc_printOut("                       0=silent, 1=default, 2=display block size (byte rounded)", true)
		bc_printOut("                       3=display timings, 4=display extra information", true)
		bc_printOut("-overwrite           : overwrite the output file if it already exists", true)
		bc_printOut("-input=<inputName>   : mandatory name of the input file to encode or 'stdin'", true)
		bc_printOut("-output=<outputName> : optional name of the output file (defaults to <input.knz>) or 'none' or 'stdout'", true)
		bc_printOut("-block=<size>        : size of the input blocks, multiple of 16, max 1 GB (transform dependent), min 1 KB, default 1 MB", true)
		bc_printOut("-entropy=<codec>     : entropy codec to use [None|Huffman*|ANS|Range|PAQ|FPAQ|TPAQ|CM]", true)
		bc_printOut("-transform=<codec>   : transform to use [None|BWT*|BWTS|Snappy|LZ4|RLT|ZRLT|MTFT|RANK|TEXT|TIMESTAMP]", true)
		bc_printOut("                       EG: BWT+RANK or RLT+BWTS+MTFT+ZRLT (default is BWT+MTFT+ZRLT)", true)
		bc_printOut("-checksum            : enable block checksum", true)
		bc_printOut("-jobs=<jobs>         : number of concurrent jobs", true)
		bc_printOut("", true)
		bc_printOut("EG. BlockCompressor -input=foo.txt -output=foo.knz -overwrite -transform=BWT+MTFT+ZRLT -block=4m -entropy=FPAQ -verbose=2 -jobs=4", true)
		bc_printOut("EG. Kanzi -compress -input=foo.txt -output=foo.knz -overwrite -transform=BWT+MTFT+ZRLT -block=4m -entropy=FPAQ -verbose=2 -jobs=4", true)
		os.Exit(0)
	}

	if *verbose < 0 {
		fmt.Printf("Invalid verbosity level provided on command line: %v\n", *verbose)
		os.Exit(kio.ERR_INVALID_PARAM)
	}

	if len(*inputName) == 0 {
		fmt.Printf("Missing input file name, exiting ...\n")
		os.Exit(kio.ERR_MISSING_PARAM)
	}

	if len(*outputName) == 0 {
		*outputName = *inputName + ".knz"
	}

	if *tasks < 1 {
		fmt.Printf("Invalid number of jobs provided on command line: %v\n", *tasks)
		os.Exit(kio.ERR_INVALID_PARAM)
	}

	if strings.ToUpper(*outputName) == "STDOUT" {
		// Overwrite verbosity if the output goes to stdout
		this.verbosity = 0
	} else {
		this.verbosity = uint(*verbose)
	}

	this.overwrite = *overwrite
	this.inputName = *inputName
	this.outputName = *outputName
	strBlockSize := strings.ToUpper(*blockSize)

	// Process K or M suffix
	scale := 1
	lastChar := strBlockSize[len(strBlockSize)-1]

	if lastChar == 'K' {
		strBlockSize = strBlockSize[0 : len(strBlockSize)-1]
		scale = 1024
	} else if lastChar == 'M' {
		strBlockSize = strBlockSize[0 : len(strBlockSize)-1]
		scale = 1024 * 1024
	} else if lastChar == 'G' {
		strBlockSize = strBlockSize[0 : len(strBlockSize)-1]
		scale = 1024 * 1024 * 1024
	}

	bSize, err := strconv.Atoi(strBlockSize)

	if err != nil || bSize <= 0 {
		fmt.Printf("Invalid block size provided on command line: %v\n", *blockSize)
		os.Exit(kio.ERR_BLOCK_SIZE)
	}

	// Extract transform names. Curate input (EG. NONE+NONE+xxxx => xxxx)
	tName := strings.ToUpper(*transforms)
	this.transform = kio.GetByteFunctionName(kio.GetByteFunctionType(tName))
	this.blockSize = uint(scale * bSize)
	this.entropyCodec = strings.ToUpper(*entropy)
	this.checksum = *cksum
	this.jobs = uint(*tasks)
	this.listeners = make([]kio.BlockListener, 0)
	this.cpuProf = *cpuprofile

	if this.verbosity > 1 {
		if listener, err := kio.NewInfoPrinter(this.verbosity, kio.ENCODING, os.Stdout); err == nil {
			this.AddListener(listener)
		}
	}

	return this, nil
}

func (this *BlockCompressor) AddListener(bl kio.BlockListener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

func (this *BlockCompressor) RemoveListener(bl kio.BlockListener) bool {
	for i, e := range this.listeners {
		if e == bl {
			this.listeners = append(this.listeners[:i-1], this.listeners[i+1:]...)
			return true
		}
	}

	return false
}

func BlockCompressor_main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	code := 0
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("An unexpected error occured during compression: %v\n", r.(error))
			code = kio.ERR_UNKNOWN
		}

		os.Exit(code)
	}()

	bc, err := NewBlockCompressor()

	if err != nil {
		fmt.Printf("Failed to create block compressor: %v\n", err)
		os.Exit(kio.ERR_CREATE_COMPRESSOR)
	}

	if len(bc.cpuProf) != 0 {
		if f, err := os.Create(bc.cpuProf); err != nil {
			fmt.Printf("Warning: cpu profile unavailable: %v\n", err)
		} else {
			pprof.StartCPUProfile(f)

			defer func() {
				pprof.StopCPUProfile()
				f.Close()
			}()
		}
	}

	code, _ = bc.call()
}

// Return exit code, number of bits written
func (this *BlockCompressor) call() (int, uint64) {
	var msg string
	printFlag := this.verbosity > 1
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
	prefix := ""

	if this.jobs > 1 {
		prefix = "s"
	}

	msg = fmt.Sprintf("Using %d job%s", this.jobs, prefix)
	bc_printOut(msg, printFlag)
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
				fmt.Print("The output file exists and the 'overwrite' command ")
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

	verboseWriter := os.Stdout

	if printFlag == false {
		verboseWriter = nil
	}

	cos, err := kio.NewCompressedOutputStream(this.entropyCodec, this.transform,
		output, this.blockSize, this.checksum, verboseWriter, this.jobs)

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
	len := 0
	read := int64(0)
	silent := this.verbosity < 1
	bc_printOut("Encoding ...", !silent)
	written = cos.GetWritten()
	buffer := make([]byte, COMP_DEFAULT_BUFFER_SIZE)
	before := time.Now()
	len, err = input.Read(buffer)

	for len > 0 {
		if err != nil {
			fmt.Printf("Failed to read block from file '%v': %v\n", this.inputName, err)
			return kio.ERR_READ_FILE, written
		}

		read += int64(len)

		if _, err = cos.Write(buffer[0:len]); err != nil {
			if ioerr, isIOErr := err.(kio.IOError); isIOErr == true {
				fmt.Printf("%s\n", ioerr.Error())
				return ioerr.ErrorCode(), written
			}

			fmt.Printf("An unexpected condition happened. Exiting ...\n%v\n", err.Error())
			return kio.ERR_PROCESS_BLOCK, written
		}

		len, err = input.Read(buffer)
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

	bc_printOut("", !silent)
	msg = fmt.Sprintf("Encoding:          %d ms", delta)
	bc_printOut(msg, !silent)
	msg = fmt.Sprintf("Input size:        %d", read)
	bc_printOut(msg, !silent)
	msg = fmt.Sprintf("Output size:       %d", cos.GetWritten())
	bc_printOut(msg, !silent)
	msg = fmt.Sprintf("Ratio:             %f", float64(cos.GetWritten())/float64(read))
	bc_printOut(msg, !silent)

	if delta > 0 {
		msg = fmt.Sprintf("Throughput (KB/s): %d", ((read*int64(1000))>>10)/delta)
		bc_printOut(msg, !silent)
	}

	bc_printOut("", !silent)
	return 0, cos.GetWritten()
}

func bc_printOut(msg string, print bool) {
	if print == true {
		fmt.Println(msg)
	}
}
