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

package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

const (
	//_ARG_IDX_COMPRESS   = 0
	//_ARG_IDX_DECOMPRESS = 1
	_ARG_IDX_INPUT     = 2
	_ARG_IDX_OUTPUT    = 3
	_ARG_IDX_BLOCK     = 4
	_ARG_IDX_TRANSFORM = 5
	_ARG_IDX_ENTROPY   = 6
	_ARG_IDX_JOBS      = 7
	_ARG_IDX_VERBOSE   = 8
	_ARG_IDX_LEVEL     = 9
	//_ARG_IDX_FROM      = 10
	//_ARG_IDX_TO        = 11
	_ARG_IDX_PROFILE = 14
	_KANZI_VERSION   = "2.3"
	_APP_HEADER      = "Kanzi " + _KANZI_VERSION + " (c) Frederic Langlet"
	_ARG_INPUT       = "--input="
	_ARG_OUTPUT      = "--output="
	_ARG_LEVEL       = "--level="
	_ARG_COMPRESS    = "--compress"
	_ARG_DECOMPRESS  = "--decompress"
	_ARG_ENTROPY     = "--entropy="
	_ARG_TRANSFORM   = "--transform="
	_ARG_VERBOSE     = "--verbose="
	_ARG_JOBS        = "--jobs="
	_ARG_BLOCK       = "--block="
	_ARG_FROM        = "--from="
	_ARG_TO          = "--to="
	_ARG_CPUPROF     = "--cpuProf="
	_ARG_FORCE       = "--force"
	_ARG_SKIP        = "--skip"
	_ARG_CHECKSUM    = "--checksum"
)

var (
	_CMD_LINE_ARGS = []string{
		"-c", "-d", "-i", "-o", "-b", "-t", "-e", "-j",
		"-v", "-l", "-s", "-x", "-f", "-h", "-p",
	}

	mutex         sync.Mutex
	log           = Printer{os: bufio.NewWriter(os.Stdout)}
	pathSeparator = string([]byte{os.PathSeparator})
)

func main() {
	argsMap := make(map[string]any)

	if status := processCommandLine(os.Args, argsMap); status != 0 {
		// Command line processing error ?
		if status < 0 {
			os.Exit(0)
		}

		os.Exit(status)
	}

	// Help mode only ?
	if argsMap["mode"] == nil {
		os.Exit(0)
	}

	mode := argsMap["mode"].(string)
	delete(argsMap, "mode")
	status := 1

	if mode == "c" {
		status = compress(argsMap)
	} else if mode == "d" {
		status = decompress(argsMap)
	} else {
		println("Missing arguments: try --help or -h")
	}

	os.Exit(status)
}

func compress(argsMap map[string]any) int {
	runtime.GOMAXPROCS(runtime.NumCPU())
	code := 0
	verbose := argsMap["verbosity"].(uint)

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("An unexpected error occurred during compression: %v\n", r.(error))
			code = kanzi.ERR_UNKNOWN
		}

		os.Exit(code)
	}()

	bc, err := NewBlockCompressor(argsMap)

	if err != nil {
		fmt.Printf("Failed to create block compressor: %v\n", err)
		return kanzi.ERR_CREATE_COMPRESSOR
	}

	if len(bc.CPUProf()) != 0 {
		if f, err := os.Create(bc.CPUProf()); err != nil {
			msg := fmt.Sprintf("Warning: cpu profile unavailable: %v", err)
			log.Println(msg, verbose > 0)
		} else {
			if err := pprof.StartCPUProfile(f); err != nil {
				msg := fmt.Sprintf("Warning: cpu profile unavailable: %v", err)
				log.Println(msg, verbose > 0)
			}

			defer func() {
				pprof.StopCPUProfile()
				f.Close()
			}()
		}
	}

	code, _ = bc.Compress()
	return code
}

func decompress(argsMap map[string]any) int {
	runtime.GOMAXPROCS(runtime.NumCPU())
	code := 0
	verbose := argsMap["verbosity"].(uint)

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("An unexpected error occurred during decompression: %v\n", r.(error))
			code = kanzi.ERR_UNKNOWN
		}

		os.Exit(code)
	}()

	bd, err := NewBlockDecompressor(argsMap)

	if err != nil {
		fmt.Printf("Failed to create block decompressor: %v\n", err)
		return kanzi.ERR_CREATE_DECOMPRESSOR
	}

	if len(bd.CPUProf()) != 0 {
		if f, err := os.Create(bd.CPUProf()); err != nil {
			msg := fmt.Sprintf("Warning: cpu profile unavailable: %v", err)
			log.Println(msg, verbose > 0)
		} else {
			if err := pprof.StartCPUProfile(f); err != nil {
				msg := fmt.Sprintf("Warning: cpu profile unavailable: %v", err)
				log.Println(msg, verbose > 0)
			}

			defer func() {
				pprof.StopCPUProfile()
				f.Close()
			}()
		}
	}

	code, _ = bd.Decompress()
	return code
}

func processCommandLine(args []string, argsMap map[string]any) int {
	blockSize := -1
	verbose := 1
	overwrite := false
	checksum := false
	skip := false
	fileReorder := true
	noDotFiles := false
	noLinks := false
	from := -1
	to := -1
	remove := false
	inputName := ""
	outputName := ""
	codec := ""
	transform := ""
	tasks := -1
	cpuProf := ""
	ctx := -1
	level := -1
	mode := " "
	autoBlockSize := false
	showHeader := true

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if strings.HasPrefix(arg, _ARG_OUTPUT) || arg == "-o" {
			ctx = _ARG_IDX_OUTPUT
			continue
		}

		if strings.HasPrefix(arg, _ARG_INPUT) || arg == "-i" {
			ctx = _ARG_IDX_INPUT
			continue
		}

		if strings.HasPrefix(arg, _ARG_VERBOSE) || arg == "-v" {
			ctx = _ARG_IDX_VERBOSE
			continue
		}

		// Extract verbosity, output and mode first
		if arg == _ARG_COMPRESS || arg == "-c" {
			if mode == "d" {
				fmt.Println("Both compression and decompression options were provided.")
				return kanzi.ERR_INVALID_PARAM
			}

			mode = "c"
			continue
		}

		if arg == _ARG_DECOMPRESS || arg == "-d" {
			if mode == "c" {
				fmt.Println("Both compression and decompression options were provided.")
				return kanzi.ERR_INVALID_PARAM
			}

			mode = "d"
			continue
		}

		if strings.HasPrefix(arg, _ARG_VERBOSE) || ctx == _ARG_IDX_VERBOSE {
			var verboseLevel string
			var err error

			if strings.HasPrefix(arg, _ARG_VERBOSE) {
				verboseLevel = strings.TrimPrefix(arg, _ARG_VERBOSE)
			} else {
				verboseLevel = arg
			}

			verboseLevel = strings.TrimSpace(verboseLevel)

			if verbose, err = strconv.Atoi(verboseLevel); err != nil {
				fmt.Printf("Invalid verbosity level provided on command line: %v\n", arg)
				return kanzi.ERR_INVALID_PARAM
			}

			if verbose < 0 || verbose > 5 {
				fmt.Printf("Invalid verbosity level provided on command line: %v\n", arg)
				return kanzi.ERR_INVALID_PARAM
			}
		} else if ctx == _ARG_IDX_OUTPUT {
			outputName = strings.TrimSpace(arg)
		} else if ctx == _ARG_IDX_INPUT {
			inputName = strings.TrimSpace(arg)
		}

		ctx = -1
	}

	// Overwrite verbosity if the output goes to stdout
	if len(inputName) == 0 && len(outputName) == 0 {
		verbose = 0
	} else {
		outputName = strings.ToUpper(outputName)

		if outputName == "STDOUT" {
			verbose = 0
		}
	}

	if verbose >= 1 {
		log.Println("\n"+_APP_HEADER+"\n", true)
		showHeader = false
	}

	inputName = ""
	outputName = ""
	ctx = -1
	warningNoValOpt := "Warning: ignoring option [%s] with no value."
	warningCompressOpt := "Warning: ignoring option [%s]. Only applicable in compress mode."
	warningDecompressOpt := "Warning: ignoring option [%s]. Only applicable in decompress mode."
	warningDupOpt := "Warning: ignoring duplicate %s (%s)"
	warningInvalidOpt := "Invalid %s provided on command line: %s"

	if len(args) == 1 {
		printHelp(mode, showHeader)
		return 0
	}

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if arg == "--help" || arg == "-h" {
			printHelp(mode, showHeader)
			return 0
		}

		if arg == _ARG_COMPRESS || arg == "-c" || arg == _ARG_DECOMPRESS || arg == "-d" {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			ctx = -1
			continue
		}

		if arg == _ARG_FORCE || arg == "-f" {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			overwrite = true
			ctx = -1
			continue
		}

		if arg == _ARG_SKIP || arg == "-s" {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			skip = true
			ctx = -1
			continue
		}

		if arg == _ARG_CHECKSUM || arg == "-x" {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			checksum = true
			ctx = -1
			continue
		}

		if arg == "--rm" {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			remove = true
			ctx = -1
			continue
		}

		if arg == "--no-file-reorder" {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			ctx = -1

			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, arg), verbose > 0)
				continue
			}

			fileReorder = false
			continue
		}

		if arg == "--no-dot-file" {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			ctx = -1

			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, arg), verbose > 0)
				continue
			}

			noDotFiles = true
			continue
		}

		if arg == "--no-link" {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			ctx = -1

			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, arg), verbose > 0)
				continue
			}

			noLinks = true
			continue
		}

		if ctx == -1 {
			idx := -1

			for i, v := range _CMD_LINE_ARGS {
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

		if strings.HasPrefix(arg, _ARG_OUTPUT) || ctx == _ARG_IDX_OUTPUT {
			name := ""

			if strings.HasPrefix(arg, _ARG_OUTPUT) {
				name = strings.TrimPrefix(arg, _ARG_OUTPUT)
			} else {
				name = arg
			}

			if outputName != "" {
				log.Println(fmt.Sprintf(warningDupOpt, "output name", name), verbose > 0)
			} else {
				outputName = name
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, _ARG_INPUT) || ctx == _ARG_IDX_INPUT {
			name := ""

			if strings.HasPrefix(arg, _ARG_INPUT) {
				name = strings.TrimPrefix(arg, _ARG_INPUT)
			} else {
				name = arg
			}

			if inputName != "" {
				log.Println(fmt.Sprintf(warningDupOpt, "input name", name), verbose > 0)
			} else {
				inputName = name
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, _ARG_ENTROPY) || ctx == _ARG_IDX_ENTROPY {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "entropy"), verbose > 0)
				ctx = -1
				continue
			}

			name := ""

			if strings.HasPrefix(arg, _ARG_ENTROPY) {
				name = strings.TrimPrefix(arg, _ARG_ENTROPY)
			} else {
				name = arg
			}

			if codec != "" {
				log.Println(fmt.Sprintf(warningDupOpt, "entropy", name), verbose > 0)
				ctx = -1
				continue
			} else {
				codec = strings.ToUpper(name)
			}

			if len(codec) == 0 {
				fmt.Println(fmt.Sprintf(warningInvalidOpt, "entropy", "[]"))
				return kanzi.ERR_INVALID_PARAM
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, _ARG_TRANSFORM) || ctx == _ARG_IDX_TRANSFORM {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "transform"), verbose > 0)
				ctx = -1
				continue
			}

			name := ""

			if strings.HasPrefix(arg, _ARG_TRANSFORM) {
				name = strings.TrimPrefix(arg, _ARG_TRANSFORM)
			} else {
				name = arg
			}

			if transform != "" {
				log.Println(fmt.Sprintf(warningDupOpt, "transform", name), verbose > 0)
				ctx = -1
				continue
			} else {
				transform = strings.ToUpper(name)
			}

			for len(transform) > 0 && transform[0] == '+' {
				transform = transform[1:]
			}

			for len(transform) > 0 && transform[len(transform)-1] == '+' {
				transform = transform[0 : len(transform)-1]
			}

			if len(transform) == 0 {
				fmt.Println(fmt.Sprintf(warningInvalidOpt, "transform", "[]"))
				return kanzi.ERR_INVALID_PARAM
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, _ARG_LEVEL) || ctx == _ARG_IDX_LEVEL {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "level"), verbose > 0)
				ctx = -1
				continue
			}

			var str string
			var err error

			if strings.HasPrefix(arg, _ARG_LEVEL) {
				str = strings.TrimPrefix(arg, _ARG_LEVEL)
			} else {
				str = arg
			}

			str = strings.TrimSpace(str)

			if level != -1 {
				log.Println(fmt.Sprintf(warningDupOpt, "compression level", str), verbose > 0)
				ctx = -1
				continue
			}

			level, err = strconv.Atoi(str)

			if err != nil || level < 0 || level > 9 {
				fmt.Println(fmt.Sprintf(warningInvalidOpt, "compression level", str))
				return kanzi.ERR_INVALID_PARAM
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, _ARG_CPUPROF) || ctx == _ARG_IDX_PROFILE {
			name := ""

			if strings.HasPrefix(arg, _ARG_CPUPROF) {
				name = strings.TrimPrefix(arg, _ARG_CPUPROF)
			} else {
				name = arg
			}

			if cpuProf != "" {
				log.Println(fmt.Sprintf(warningDupOpt, "profile name", name), verbose > 0)
			} else {
				cpuProf = name
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, _ARG_BLOCK) || ctx == _ARG_IDX_BLOCK {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "block size"), verbose > 0)
				ctx = -1
				continue
			}

			var strBlockSize string

			if strings.HasPrefix(arg, _ARG_BLOCK) {
				strBlockSize = strings.TrimPrefix(arg, _ARG_BLOCK)
			} else {
				strBlockSize = arg
			}

			strBlockSize = strings.ToUpper(strBlockSize)

			if blockSize != -1 || autoBlockSize == true {
				log.Println(fmt.Sprintf(warningDupOpt, "block size", strBlockSize), verbose > 0)
				ctx = -1
				continue
			}

			if strings.Compare(strBlockSize, "AUTO") == 0 {
				autoBlockSize = true
			} else {
				// Process K or M suffix
				scale := 1
				lastChar := byte(0)

				if len(strBlockSize) > 0 {
					lastChar = strBlockSize[len(strBlockSize)-1]
				}

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

				var err error

				if blockSize, err = strconv.Atoi(strBlockSize); err != nil || blockSize <= 0 {
					fmt.Println(fmt.Sprintf(warningInvalidOpt, "block size", strBlockSize))
					return kanzi.ERR_BLOCK_SIZE
				}

				blockSize = scale * blockSize
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, _ARG_JOBS) || ctx == _ARG_IDX_JOBS {
			var strTasks string
			var err error

			if strings.HasPrefix(arg, _ARG_JOBS) {
				strTasks = strings.TrimPrefix(arg, _ARG_JOBS)
			} else {
				strTasks = arg
			}

			if tasks != -1 {
				log.Println(fmt.Sprintf(warningDupOpt, "jobs", strTasks), verbose > 0)
				ctx = -1
				continue
			}

			if tasks, err = strconv.Atoi(strTasks); err != nil || tasks < 0 {
				fmt.Println(fmt.Sprintf(warningInvalidOpt, "number of jobs", strTasks))
				return kanzi.ERR_BLOCK_SIZE
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, _ARG_FROM) && ctx == -1 {
			if mode != "d" {
				log.Println(fmt.Sprintf(warningDecompressOpt, "start block"), verbose > 0)
				continue
			}

			var strFrom string
			var err error

			if strings.HasPrefix(arg, _ARG_FROM) {
				strFrom = strings.TrimPrefix(arg, _ARG_FROM)
			} else {
				strFrom = arg
			}

			if from != -1 {
				log.Println(fmt.Sprintf(warningDupOpt, "start block", strFrom), verbose > 0)
				continue
			}

			if from, err = strconv.Atoi(strFrom); err != nil || from <= 0 {
				fmt.Println(fmt.Sprintf(warningInvalidOpt, "start block", strFrom))

				if from == 0 {
					fmt.Printf("The first block ID is 1.\n")
				}

				return kanzi.ERR_INVALID_PARAM
			}

			continue
		}

		if strings.HasPrefix(arg, _ARG_TO) && ctx == -1 {
			if mode != "d" {
				log.Println(fmt.Sprintf(warningDecompressOpt, "end block"), verbose > 0)
				continue
			}

			var strTo string
			var err error

			if strings.HasPrefix(arg, _ARG_TO) {
				strTo = strings.TrimPrefix(arg, _ARG_TO)
			} else {
				strTo = arg
			}

			if to != -1 {
				log.Println(fmt.Sprintf(warningDupOpt, "end block", strTo), verbose > 0)
				continue
			}

			if to, err = strconv.Atoi(strTo); err != nil || to <= 0 {
				fmt.Println(fmt.Sprintf(warningInvalidOpt, "end block", strTo))
				return kanzi.ERR_INVALID_PARAM
			}

			continue
		}

		if !strings.HasPrefix(arg, _ARG_VERBOSE) && !strings.HasPrefix(arg, _ARG_OUTPUT) &&
			ctx == -1 && !strings.HasPrefix(arg, _ARG_CPUPROF) {
			log.Println("Warning: ignoring unknown option ["+arg+"]", verbose > 0)
		}

		ctx = -1
	}

	if ctx != -1 {
		log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
	}

	if level >= 0 {
		if len(codec) != 0 {
			log.Println("Warning: providing the 'level' option forces the entropy codec. Ignoring ["+codec+"]", verbose > 0)
		}

		if len(transform) != 0 {
			log.Println("Warning: providing the 'level' option forces the transform. Ignoring ["+transform+"]", verbose > 0)
		}
	}

	if blockSize != -1 {
		argsMap["blockSize"] = uint(blockSize)
	}

	if autoBlockSize == true {
		argsMap["autoBlock"] = true
	}

	argsMap["verbosity"] = uint(verbose)
	argsMap["mode"] = mode

	if overwrite == true {
		argsMap["overwrite"] = true
	}

	argsMap["inputName"] = inputName
	argsMap["outputName"] = outputName

	if mode == "c" && level != -1 {
		argsMap["level"] = level
	}

	if len(codec) > 0 {
		argsMap["entropy"] = codec
	}

	if len(transform) > 0 {
		argsMap["transform"] = transform
	}

	if checksum == true {
		argsMap["checksum"] = true
	}

	if skip == true {
		argsMap["skipBlocks"] = true
	}

	if remove == true {
		argsMap["remove"] = true
	}

	if fileReorder == false {
		argsMap["fileReorder"] = false
	}

	if noDotFiles == true {
		argsMap["noDotFiles"] = true
	}

	if noLinks == true {
		argsMap["noLinks"] = true
	}

	if tasks >= 0 {
		argsMap["jobs"] = uint(tasks)
	}

	if len(cpuProf) > 0 {
		argsMap["cpuProf"] = cpuProf
	}

	if from >= 0 {
		argsMap["from"] = from
	}

	if to >= 0 {
		argsMap["to"] = to
	}

	return 0
}

func printHelp(mode string, showHeader bool) {
	if showHeader == true {
		log.Println("", true)
		log.Println(_APP_HEADER, true)
	}

	log.Println("", true)
	log.Println("Credits: Matt Mahoney, Yann Collet, Jan Ondrus, Yuta Mori, Ilya Muravyov,", true)
	log.Println("         Neal Burns, Fabian Giesen, Jarek Duda, Ilya Grebnov", true)
	log.Println("", true)
	log.Println("   -h, --help", true)
	log.Println("        Display this message\n", true)

	if mode != "c" && mode != "d" {
		log.Println("   -c, --compress", true)
		log.Println("        Compress mode", true)
		log.Println("", true)
		log.Println("   -d, --decompress", true)
		log.Println("        Decompress mode", true)
		log.Println("", true)
	}

	log.Println("   -i, --input=<inputName>", true)
	log.Println("        Mandatory name of the input file or directory or 'stdin'", true)
	log.Println("        When the source is a directory, all files in it will be processed.", true)
	msg := fmt.Sprintf("        Provide %c. at the end of the directory name to avoid recursion.", os.PathSeparator)
	log.Println(msg, true)
	msg = fmt.Sprintf("        (EG: myDir%c. => no recursion)\n", os.PathSeparator)
	log.Println(msg, true)
	log.Println("   -o, --output=<outputName>", true)

	if mode == "c" {
		log.Println("        Optional name of the output file or directory (defaults to", true)
		log.Println("        <inputName.knz>) or 'none' or 'stdout'. 'stdout' is not valid", true)
		log.Println("        when the number of jobs is greater than 1.\n", true)
	} else if mode == "d" {
		log.Println("        Optional name of the output file or directory (defaults to", true)
		log.Println("        <inputName.bak>) or 'none' or 'stdout'. 'stdout' is not valid", true)
		log.Println("        when the number of jobs is greater than 1.\n", true)

	} else {
		log.Println("        optional name of the output file or 'none' or 'stdout'.\n", true)
	}

	if mode == "c" {
		log.Println("   -b, --block=<size>", true)
		log.Println("        Size of blocks (default 4|8|16|32 MB based on level, max 1 GB, min 1 KB).", true)
		log.Println("        'auto' means that the compressor derives the best value'", true)
		log.Println("        based on input size (when available) and number of jobs.\n", true)
		log.Println("   -l, --level=<compression>", true)
		log.Println("        Set the compression level [0..9]", true)
		log.Println("        Providing this option forces entropy and transform.", true)
		log.Println("        Defaults to level 3 if not provided.", true)
		log.Println("        0=NONE&NONE (store)", true)
		log.Println("        1=PACK+LZ&NONE", true)
		log.Println("        2=PACK+LZ&HUFFMAN", true)
		log.Println("        3=TEXT+UTF+PACK+MM+LZX&HUFFMAN", true)
		log.Println("        4=TEXT+UTF+EXE+PACK+MM+ROLZ&NONE", true)
		log.Println("        5=TEXT+UTF+BWT+RANK+ZRLT&ANS0", true)
		log.Println("        6=TEXT+UTF+BWT+SRT+ZRLT&FPAQ", true)
		log.Println("        7=LZP+TEXT+UTF+BWT+LZP&CM", true)
		log.Println("        8=EXE+RLT+TEXT+UTF&TPAQ", true)
		log.Println("        9=EXE+RLT+TEXT+UTF&TPAQX\n", true)
		log.Println("   -e, --entropy=<codec>", true)
		log.Println("        Entropy codec [None|Huffman|ANS0|ANS1|Range|FPAQ|TPAQ|TPAQX|CM]\n", true)
		log.Println("   -t, --transform=<codec>", true)
		log.Println("        Transform [None|BWT|BWTS|LZ|LZX|LZP|ROLZ|ROLZX|RLT|ZRLT]", true)
		log.Println("                  [MTFT|RANK|SRT|TEXT|MM|EXE|UTF|PACK]", true)
		log.Println("        EG: BWT+RANK or BWTS+MTFT\n", true)
		log.Println("   -x, --checksum", true)
		log.Println("        Enable block checksum\n", true)
		log.Println("   -s, --skip", true)
		log.Println("        Copy blocks with high entropy instead of compressing them.\n", true)

	}

	log.Println("   -j, --jobs=<jobs>", true)
	log.Println("        Maximum number of jobs the program may start concurrently", true)
	log.Println("        If 0 is provided, use all available cores (maximum is 64).", true)
	log.Println("        (default is half of available cores).\n", true)
	log.Println("   -v, --verbose=<level>", true)
	log.Println("        Set the verbosity level [0..5]", true)
	log.Println("        0=silent, 1=default, 2=display details, 3=display configuration,", true)
	log.Println("        4=display block size and timings, 5=display extra information", true)
	log.Println("        Verbosity is reduced to 1 when files are processed concurrently", true)
	log.Println("        Verbosity is reduced to 0 when the output is 'stdout'\n", true)
	log.Println("   -f, --force", true)
	log.Println("        Overwrite the output file if it already exists\n", true)
	log.Println("   --rm", true)
	log.Println("        Remove the input file after successful (de)compression.", true)
	log.Println("        If the input is a folder, all processed files under the folder are removed.\n", true)
	log.Println("   --no-link", true)
	log.Println("        Skip links\n", true)
	log.Println("   --no-dot-file", true)
	log.Println("        Skip dot files\n", true)

	if mode == "d" {
		log.Println("   --from=blockID", true)
		log.Println("        Decompress starting at the provided block (included).", true)
		log.Println("        The first block ID is 1.\n", true)
		log.Println("   --to=blockID", true)
		log.Println("        Decompress ending at the provided block (excluded).\n", true)
		log.Println("", true)
		log.Println("EG. Kanzi -d -i foo.knz -f -v 2 -j 2\n", true)
		log.Println("EG. Kanzi --decompress --input=foo.knz --force --verbose=2 --jobs=2\n", true)
	}

	if mode == "c" {
		log.Println("", true)
		log.Println("EG. Kanzi -c -i foo.txt -o none -b 4m -l 4 -v 3\n", true)
		log.Println("EG. Kanzi -c -i foo.txt -f -t BWT+MTFT+ZRLT -b 4m -e FPAQ -j 4\n", true)
		log.Println("EG. Kanzi --compress --input=foo.txt --output=foo.knz --block=4m --force", true)
		log.Println("          --transform=BWT+MTFT+ZRLT --entropy=FPAQ --jobs=4\n", true)
	}
}

// FileData a basic structure encapsulating a file path and size
type FileData struct {
	FullPath string
	Path     string
	Name     string
	Size     int64
}

// NewFileData creates an instance of FileData from a file path and size
func NewFileData(fullPath string, size int64) *FileData {
	this := &FileData{}
	this.FullPath = fullPath
	this.Size = size

	idx := strings.LastIndexByte(this.FullPath, byte(os.PathSeparator))

	if idx > 0 {
		b := []byte(this.FullPath)
		this.Path = string(b[0 : idx+1])
		this.Name = string(b[idx+1:])
	} else {
		this.Path = ""
		this.Name = this.FullPath
	}

	return this
}

// FileCompare a structure used to sort files by path and size
type FileCompare struct {
	data       []FileData
	sortBySize bool
}

// Len returns the size of the internal file data buffer
func (this FileCompare) Len() int {
	return len(this.data)
}

// Swap swaps two file data in the internal buffer
func (this FileCompare) Swap(i, j int) {
	this.data[i], this.data[j] = this.data[j], this.data[i]
}

// Less returns true if the path at index i in the internal
// file data buffer is less than file data buffer at index j.
// The order is defined by lexical order of the parent directory
// path then file size.
func (this FileCompare) Less(i, j int) bool {
	if this.sortBySize == false {
		return strings.Compare(this.data[i].FullPath, this.data[j].FullPath) < 0
	}

	// First compare parent directory paths
	res := strings.Compare(this.data[i].Path, this.data[j].Path)

	if res != 0 {
		return res < 0
	}

	// Then, compare file sizes (decreasing order)
	return this.data[i].Size > this.data[j].Size
}

func createFileList(target string, fileList []FileData, isRecursive, ignoreLinks, ignoreDotFiles bool) ([]FileData, error) {
	fi, err := os.Stat(target)

	if err != nil {
		return fileList, err
	}

	if ignoreDotFiles == true {
		shortName := target

		if idx := strings.LastIndex(shortName, pathSeparator); idx > 0 {
			shortName = shortName[idx+1:]
		}

		if len(shortName) > 0 && shortName[0] == '.' {
			return fileList, nil
		}
	}

	if fi.Mode().IsRegular() || ((ignoreLinks == false) && (fi.Mode()&fs.ModeSymlink != 0)) {
		fileList = append(fileList, *NewFileData(target, fi.Size()))
		return fileList, nil
	}

	if isRecursive {
		if target[len(target)-1] != os.PathSeparator {
			target = target + pathSeparator
		}

		err = filepath.Walk(target, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if ignoreDotFiles == true {
				shortName := path

				if idx := strings.LastIndex(shortName, pathSeparator); idx > 0 {
					shortName = shortName[idx+1:]
				}

				if len(shortName) > 0 && shortName[0] == '.' {
					return nil
				}
			}

			if fi.Mode().IsRegular() || ((ignoreLinks == false) && (fi.Mode()&fs.ModeSymlink != 0)) {
				fileList = append(fileList, *NewFileData(path, fi.Size()))
			}

			return err
		})
	} else {
		var files []fs.DirEntry
		files, err = os.ReadDir(target)

		if err == nil {
			for _, de := range files {
				if de.Type().IsRegular() {
					var fi fs.FileInfo

					if fi, err = de.Info(); err != nil {
						break
					}

					if ignoreDotFiles == true {
						shortName := de.Name()

						if idx := strings.LastIndex(shortName, pathSeparator); idx > 0 {
							shortName = shortName[idx+1:]
						}

						if len(shortName) > 0 && shortName[0] == '.' {
							continue
						}
					}

					if fi.Mode().IsRegular() || ((ignoreLinks == false) && (fi.Mode()&fs.ModeSymlink != 0)) {
						fileList = append(fileList, *NewFileData(target+de.Name(), fi.Size()))
					}
				}
			}
		}
	}

	return fileList, err
}

// Printer a buffered printer (required in concurrent code)
type Printer struct {
	os *bufio.Writer
}

// Println concurrently safe version (order wise) of Println
func (this *Printer) Println(msg string, printFlag bool) {
	if printFlag == true {
		mutex.Lock()

		// Best effort, ignore error
		if w, _ := this.os.Write([]byte(msg + "\n")); w > 0 {
			_ = this.os.Flush()
		}

		mutex.Unlock()
	}
}
