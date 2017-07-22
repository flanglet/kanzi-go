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
	ENC_ARG_IDX_INPUT        = 0
	ENC_ARG_IDX_OUTPUT       = 1
	ENC_ARG_IDX_BLOCK        = 2
	ENC_ARG_IDX_TRANSFORM    = 3
	ENC_ARG_IDX_ENTROPY      = 4
	ENC_ARG_IDX_JOBS         = 5
	ENC_ARG_IDX_VERBOSE      = 6
	ENC_ARG_IDX_PROFILE      = 10
)

var (
	ENC_CMD_LINE_ARGS = []string{
		"-i", "-o", "-b", "-t", "-e", "-j", "-v", "-x", "-f", "-h", "-p",
	}
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
	argsMap := make(map[string]interface{})
	processEncoderCommandLine(os.Args, argsMap)

	// Extract transform names. Curate input (EG. NONE+NONE+xxxx => xxxx)
	tName := argsMap["transform"].(string)
	this.transform = kio.GetByteFunctionName(kio.GetByteFunctionType(tName))
	this.verbosity = argsMap["verbose"].(uint)
	this.overwrite = argsMap["overwrite"].(bool)
	this.inputName = argsMap["inputName"].(string)
	this.outputName = argsMap["outputName"].(string)
	this.entropyCodec = argsMap["codec"].(string)
	this.blockSize = argsMap["blockSize"].(uint)
	this.checksum = argsMap["checksum"].(bool)
	this.jobs = argsMap["jobs"].(uint)
	this.cpuProf = argsMap["cpuProf"].(string)
	this.listeners = make([]kio.BlockListener, 0)

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

func processEncoderCommandLine(args []string, argsMap map[string]interface{}) {
	blockSize := 1024 * 1024 // 1 MB
	verbose := 1
	overwrite := false
	checksum := false
	inputName := ""
	outputName := ""
	codec := "HUFFMAN"           // default
	transform := "BWT+MTFT+ZRLT" // default
	tasks := 1
	cpuProf := ""
	ctx := -1

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if arg == "-v" {
			ctx = ENC_ARG_IDX_VERBOSE
			continue
		}

		if arg == "-o" {
			ctx = ENC_ARG_IDX_OUTPUT
			continue
		}

		// Extract verbosity and output first
		if strings.HasPrefix(arg, "--verbose=") || ctx == ENC_ARG_IDX_VERBOSE {
			var verboseLevel string
			var err error

			if strings.HasPrefix(arg, "--verbose=") {
				verboseLevel = strings.TrimPrefix(arg, "--verbose=")
			} else {
				verboseLevel = arg
			}

			verboseLevel = strings.TrimSpace(verboseLevel)

			if verbose, err = strconv.Atoi(verboseLevel); err != nil {
				fmt.Printf("Invalid verbosity level provided on command line: %v\n", arg)
				os.Exit(kio.ERR_INVALID_PARAM)
			}

			if verbose < 0 {
				fmt.Printf("Invalid verbosity level provided on command line: %v\n", arg)
				os.Exit(kio.ERR_INVALID_PARAM)
			}
		} else if strings.HasPrefix(arg, "--output=") || ctx == ENC_ARG_IDX_OUTPUT {
			if strings.HasPrefix(arg, "--output") {
				outputName = strings.TrimPrefix(arg, "--output=")
			} else {
				outputName = arg
			}

			outputName = strings.TrimSpace(outputName)
		}

		ctx = -1
	}

	// Overwrite verbosity if the output goes to stdout
	if strings.ToUpper(outputName) == "STDOUT" {
		verbose = 0
	}

	ctx = -1

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if arg == "--help" || arg == "-h" {
			bc_printOut("-h, --help                : display this message", true)
			bc_printOut("-v, -verbose=<level>      : set the verbosity level [1..4]", true)
			bc_printOut("                            0=silent, 1=default, 2=display block size (byte rounded)", true)
			bc_printOut("                            3=display timings, 4=display extra information", true)
			bc_printOut("-f, --force               : overwrite the output file if it already exists", true)
			bc_printOut("-i, --input=<inputName>   : mandatory name of the input file to encode or 'stdin'", true)
			bc_printOut("-o, --output=<outputName> : optional name of the output file (defaults to <input.knz>) or 'none' or 'stdout'", true)
			bc_printOut("-b, --block=<size>        : size of the input blocks, multiple of 16, max 1 GB (transform dependent), min 1 KB, default 1 MB", true)
			bc_printOut("-e, --entropy=<codec>     : entropy codec to use [None|Huffman*|ANS|Range|PAQ|FPAQ|TPAQ|CM]", true)
			bc_printOut("-t, --transform=<codec>   : transform to use [None|BWT*|BWTS|SNAPPY|LZ4|RLT|ZRLT|MTFT|RANK|TEXT|TIMESTAMP]", true)
			bc_printOut("                            EG: BWT+RANK or BWTS+MTFT (default is BWT+MTFT+ZRLT)", true)
			bc_printOut("-x, --checksum            : enable block checksum", true)
			bc_printOut("-j, --jobs=<jobs>         : number of concurrent jobs", true)
			bc_printOut("", true)
			bc_printOut(`EG. Kanzi --compress --input=foo.txt --output=foo.knz --force 
                                      --transform=BWT+MTFT+ZRLT --block=4m --entropy=FPAQ --verbose=3 --jobs=4`, true)
			bc_printOut("EG. Kanzi -c -i foo.txt -o foo.knz -f -t BWT+MTFT+ZRLT -b 4m -e FPAQ -v 3 -j 4", true)
			os.Exit(0)
		}

		if arg == "--force" || arg == "-f" {
			if ctx != -1 {
				bc_printOut("Warning: ignoring option ["+ENC_CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			overwrite = true
			ctx = -1
			continue
		}

		if arg == "--checksum" || arg == "-x" {
			if ctx != -1 {
				bc_printOut("Warning: ignoring option ["+ENC_CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			checksum = true
			ctx = -1
			continue
		}

		if ctx == -1 {
			idx := -1

			for i, v := range ENC_CMD_LINE_ARGS {
				if arg == v {
					idx = i
					break
				}
			}

			if idx != -1 {
				ctx = idx
				continue
			}
		}

		if strings.HasPrefix(arg, "--input=") || ctx == ENC_ARG_IDX_INPUT {
			if strings.HasPrefix(arg, "--input=") {
				inputName = strings.TrimPrefix(arg, "--input=")
			} else {
				inputName = arg
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--entropy=") || ctx == ENC_ARG_IDX_ENTROPY {
			if strings.HasPrefix(arg, "--entropy=") {
				codec = strings.TrimPrefix(arg, "--entropy=")
			} else {
				codec = arg
			}

			codec = strings.ToUpper(codec)
			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--transform=") || ctx == ENC_ARG_IDX_TRANSFORM {
			if strings.HasPrefix(arg, "--transform=") {
				transform = strings.TrimPrefix(arg, "--transform=")
			} else {
				transform = arg
			}

			transform = strings.ToUpper(transform)
			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--cpuProf=") || ctx == ENC_ARG_IDX_PROFILE {
			if strings.HasPrefix(arg, "--cpuProf=") {
				cpuProf = strings.TrimPrefix(arg, "--cpuProf=")
			} else {
				cpuProf = arg
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--block=") || ctx == ENC_ARG_IDX_BLOCK {
			var strBlockSize string

			if strings.HasPrefix(arg, "--block=") {
				strBlockSize = strings.TrimPrefix(arg, "--block=")
			} else {
				strBlockSize = arg
			}

			strBlockSize = strings.ToUpper(strBlockSize)

			// Process K or M suffix
			scale := 1
			lastChar := strBlockSize[len(strBlockSize)-1]
			var err error

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

			if blockSize, err = strconv.Atoi(strBlockSize); err != nil || blockSize <= 0 {
				fmt.Printf("Invalid block size provided on command line: %v\n", strBlockSize)
				os.Exit(kio.ERR_BLOCK_SIZE)
			}

			blockSize = scale * blockSize
			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--jobs=") || ctx == ENC_ARG_IDX_JOBS {
			var strTasks string
			var err error

			if strings.HasPrefix(arg, "-j") {
				strTasks = strings.TrimPrefix(arg, "-j")
			} else {
				strTasks = strings.TrimPrefix(arg, "--jobs=")
			}

			if tasks, err = strconv.Atoi(strTasks); err != nil || tasks < 1 {
				fmt.Printf("Invalid number of jobs provided on command line: %v\n", strTasks)
				os.Exit(kio.ERR_BLOCK_SIZE)
			}
			ctx = -1
			continue
		}

		if !strings.HasPrefix(arg, "--verbose=") && ctx == -1 && !strings.HasPrefix(arg, "--output=") {
			bc_printOut("Warning: ignoring unknown option ["+arg+"]", verbose > 0)
		}

		ctx = -1
	}

	if inputName == "" {
		fmt.Printf("Missing input file name, exiting ...")
		os.Exit(kio.ERR_MISSING_PARAM)
	}

	if outputName == "" {
		outputName = inputName + ".knz"
	}

	if ctx != -1 {
		bc_printOut("Warning: ignoring option with missing value ["+DEC_CMD_LINE_ARGS[ctx]+"]", verbose > 0)
	}

	argsMap["blockSize"] = uint(blockSize)
	argsMap["verbose"] = uint(verbose)
	argsMap["overwrite"] = overwrite
	argsMap["inputName"] = inputName
	argsMap["outputName"] = outputName
	argsMap["codec"] = codec
	argsMap["transform"] = transform
	argsMap["checksum"] = checksum
	argsMap["jobs"] = uint(tasks)
	argsMap["cpuProf"] = cpuProf

}

func bc_printOut(msg string, print bool) {
	if print == true {
		fmt.Println(msg)
	}
}
