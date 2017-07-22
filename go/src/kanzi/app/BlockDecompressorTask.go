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
	DECOMP_DEFAULT_BUFFER_SIZE = 32768
	DEC_ARG_IDX_INPUT          = 0
	DEC_ARG_IDX_OUTPUT         = 1
	DEC_ARG_IDX_JOBS           = 5
	DEC_ARG_IDX_VERBOSE        = 6
	DEC_ARG_IDX_PROFILE        = 10
)

var (
	DEC_CMD_LINE_ARGS = []string{
		"-i", "-o", "-b", "-t", "-e", "-j", "-v", "-x", "-f", "-h", "-p",
	}
)

// Main block decompressor struct
type BlockDecompressor struct {
	verbosity  uint
	overwrite  bool
	inputName  string
	outputName string
	jobs       uint
	listeners  []kio.BlockListener
	cpuProf    string
}

func NewBlockDecompressor() (*BlockDecompressor, error) {
	this := new(BlockDecompressor)
	argsMap := make(map[string]interface{})
	processDecoderCommandLine(os.Args, argsMap)

	this.verbosity = argsMap["verbose"].(uint)
	this.overwrite = argsMap["overwrite"].(bool)
	this.inputName = argsMap["inputName"].(string)
	this.outputName = argsMap["outputName"].(string)
	this.jobs = argsMap["jobs"].(uint)
	this.cpuProf = argsMap["cpuProf"].(string)
	this.listeners = make([]kio.BlockListener, 0)

	if this.verbosity > 1 {
		if listener, err := kio.NewInfoPrinter(this.verbosity, kio.DECODING, os.Stdout); err == nil {
			this.AddListener(listener)
		}
	}

	return this, nil
}

func (this *BlockDecompressor) AddListener(bl kio.BlockListener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

func (this *BlockDecompressor) RemoveListener(bl kio.BlockListener) bool {
	for i, e := range this.listeners {
		if e == bl {
			this.listeners = append(this.listeners[:i-1], this.listeners[i+1:]...)
			return true
		}
	}

	return false
}

func BlockDecompressor_main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	code := 0

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("An unexpected error occured during decompression: %v\n", r.(error))
			code = kio.ERR_UNKNOWN
		}

		os.Exit(code)
	}()

	bd, err := NewBlockDecompressor()

	if err != nil {
		fmt.Printf("Failed to create block decompressor: %v\n", err)
		os.Exit(kio.ERR_CREATE_DECOMPRESSOR)
	}

	if len(bd.cpuProf) != 0 {
		if f, err := os.Create(bd.cpuProf); err != nil {
			fmt.Printf("Warning: cpu profile unavailable: %v\n", err)
		} else {
			pprof.StartCPUProfile(f)

			defer func() {
				pprof.StopCPUProfile()
				f.Close()
			}()
		}
	}

	code, _ = bd.call()
}

// Return exit code, number of bits written
func (this *BlockDecompressor) call() (int, uint64) {
	var msg string
	printFlag := this.verbosity > 1
	bd_printOut("Kanzi 1.1 (C) 2017,  Frederic Langlet", this.verbosity >= 1)
	bd_printOut("Input file name set to '"+this.inputName+"'", printFlag)
	bd_printOut("Output file name set to '"+this.outputName+"'", printFlag)
	msg = fmt.Sprintf("Verbosity set to %v", this.verbosity)
	bd_printOut(msg, printFlag)
	msg = fmt.Sprintf("Overwrite set to %t", this.overwrite)
	bd_printOut(msg, printFlag)
	prefix := ""

	if this.jobs > 1 {
		prefix = "s"
	}

	msg = fmt.Sprintf("Using %d job%s", this.jobs, prefix)
	bd_printOut(msg, printFlag)
	var output io.WriteCloser

	if strings.ToUpper(this.outputName) == "NONE" {
		output, _ = kio.NewNullOutputStream()
	} else if strings.ToUpper(this.outputName) == "STDOUT" {
		output = os.Stdout
	} else {
		var err error

		if output, err = os.OpenFile(this.outputName, os.O_RDWR, 666); err == nil {
			// File exists
			if this.overwrite == false {
				fmt.Printf("The output file '%v' exists and the 'overwrite' command ", this.outputName)
				fmt.Println("line option has not been provided")
				output.Close()
				return kio.ERR_OVERWRITE_FILE, 0
			}

			path1, _ := filepath.Abs(this.inputName)
			path2, _ := filepath.Abs(this.outputName)

			if path1 == path2 {
				fmt.Print("The input and output files must be different")
				return kio.ERR_CREATE_FILE, 0
			}
		} else {
			// File does not exist, create
			if output, err = os.Create(this.outputName); err != nil {
				fmt.Printf("Cannot open output file '%v' for writing: %v\n", this.outputName, err)
				return kio.ERR_CREATE_FILE, 0
			}
		}
	}

	defer func() {
		output.Close()
	}()

	// Decode
	read := uint64(0)
	silent := this.verbosity < 1
	bd_printOut("Decoding ...", !silent)
	var input io.ReadCloser

	if strings.ToUpper(this.inputName) == "STDIN" {
		input = os.Stdin
	} else {
		var err error

		if input, err = os.Open(this.inputName); err != nil {
			fmt.Printf("Cannot open input file '%v': %v\n", this.inputName, err)
			return kio.ERR_OPEN_FILE, read
		}

		defer func() {
			input.Close()
		}()
	}

	verboseWriter := os.Stdout

	if printFlag == false {
		verboseWriter = nil
	}

	cis, err := kio.NewCompressedInputStream(input, verboseWriter, this.jobs)

	if err != nil {
		if err.(*kio.IOError) != nil {
			fmt.Printf("%s\n", err.(*kio.IOError).Message())
			return err.(*kio.IOError).ErrorCode(), read
		}

		fmt.Printf("Cannot create compressed stream: %v\n", err)
		return kio.ERR_CREATE_DECOMPRESSOR, read
	}

	for _, bl := range this.listeners {
		cis.AddListener(bl)
	}

	buffer := make([]byte, DECOMP_DEFAULT_BUFFER_SIZE)
	decoded := len(buffer)
	before := time.Now()

	// Decode next block
	for decoded == len(buffer) {
		if decoded, err = cis.Read(buffer); err != nil {
			if ioerr, isIOErr := err.(*kio.IOError); isIOErr == true {
				fmt.Printf("%s\n", ioerr.Message())
				return ioerr.ErrorCode(), read
			}

			fmt.Printf("An unexpected condition happened. Exiting ...\n%v\n", err)
			return kio.ERR_PROCESS_BLOCK, read
		}

		if decoded > 0 {
			_, err = output.Write(buffer[0:decoded])

			if err != nil {
				fmt.Printf("Failed to write decompressed block to file '%v': %v\n", this.outputName, err)
				return kio.ERR_WRITE_FILE, read
			}

			read += uint64(decoded)
		}
	}

	// Close streams to ensure all data are flushed
	// Deferred close is fallback for error paths
	if err := cis.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return kio.ERR_PROCESS_BLOCK, read
	}

	after := time.Now()
	delta := after.Sub(before).Nanoseconds() / 1000000 // convert to ms

	bd_printOut("", !silent)
	msg = fmt.Sprintf("Decoding:          %d ms", delta)
	bd_printOut(msg, !silent)
	msg = fmt.Sprintf("Input size:        %d", cis.GetRead())
	bd_printOut(msg, !silent)
	msg = fmt.Sprintf("Output size:       %d", read)
	bd_printOut(msg, !silent)

	if delta > 0 {
		msg = fmt.Sprintf("Throughput (KB/s): %d", ((read*uint64(1000))>>10)/uint64(delta))
		bd_printOut(msg, !silent)
	}

	bd_printOut("", !silent)
	return 0, cis.GetRead()
}

func processDecoderCommandLine(args []string, argsMap map[string]interface{}) {
	verbose := 1
	overwrite := false
	inputName := ""
	outputName := ""
	tasks := 1
	cpuProf := ""
	ctx := -1

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if arg == "-v" {
			ctx = DEC_ARG_IDX_VERBOSE
			continue
		}

		if arg == "-o" {
			ctx = DEC_ARG_IDX_OUTPUT
			continue
		}

		// Extract verbosity and output first
		if strings.HasPrefix(arg, "--verbose=") || ctx == DEC_ARG_IDX_VERBOSE {
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
		} else if strings.HasPrefix(arg, "--output=") || ctx == DEC_ARG_IDX_OUTPUT {
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
			bd_printOut("-h, --help                : display this message", true)
			bd_printOut("-v, -verbose=<level>      : set the verbosity level [1..4]", true)
			bd_printOut("                            0=silent, 1=default, 2=display block size (byte rounded)", true)
			bd_printOut("                            3=display timings, 4=display extra information", true)
			bd_printOut("-f, --force               : overwrite the output file if it already exists", true)
			bd_printOut("-i, --input=<inputName>   : mandatory name of the input file to encode or 'stdin'", true)
			bd_printOut("-o, --output=<outputName> : optional name of the output file (defaults to <input.knz>) or 'none' or 'stdout'", true)
			bd_printOut("-j, --jobs=<jobs>         : number of concurrent jobs", true)
			bd_printOut("", true)
			bd_printOut("EG. Kanzi --decompress --input=foo.knz --force --verbose=2 --jobs=2", true)
			bd_printOut("EG. Kanzi -d -i foo.knz -f -v 2 -j 2", true)
			os.Exit(0)
		}

		if arg == "--force" || arg == "-f" {
			if ctx != -1 {
				bd_printOut("Warning: ignoring option ["+DEC_CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			overwrite = true
			ctx = -1
			continue
		}

		if ctx == -1 {
			idx := -1

			for i, v := range DEC_CMD_LINE_ARGS {
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

		if strings.HasPrefix(arg, "--input=") || ctx == DEC_ARG_IDX_INPUT {
			if strings.HasPrefix(arg, "--input=") {
				inputName = strings.TrimPrefix(arg, "--input=")
			} else {
				inputName = arg
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--cpuProf=") || ctx == DEC_ARG_IDX_PROFILE {
			if strings.HasPrefix(arg, "--cpuProf=") {
				cpuProf = strings.TrimPrefix(arg, "--cpuProf=")
			} else {
				cpuProf = arg
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--jobs=") || ctx == DEC_ARG_IDX_JOBS {
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
			bd_printOut("Warning: ignoring unknown option ["+arg+"]", verbose > 0)
		}

		ctx = -1
	}

	if inputName == "" {
		fmt.Printf("Missing input file name, exiting ...")
		os.Exit(kio.ERR_MISSING_PARAM)
	}

	if strings.ToUpper(inputName) != "STDIN" && !strings.HasSuffix(inputName, ".knz") {
		bd_printOut("Warning: the input file name does not end with the .KNZ extension", verbose > 0)
	}

	if outputName == "" {
		outputName = inputName + ".knz"
	}

	if ctx != -1 {
		bd_printOut("Warning: ignoring option with missing value ["+DEC_CMD_LINE_ARGS[ctx]+"]", verbose > 0)
	}

	argsMap["verbose"] = uint(verbose)
	argsMap["overwrite"] = overwrite
	argsMap["inputName"] = inputName
	argsMap["outputName"] = outputName
	argsMap["jobs"] = uint(tasks)
	argsMap["cpuProf"] = cpuProf

}

func bd_printOut(msg string, print bool) {
	if print == true {
		fmt.Println(msg)
	}
}
