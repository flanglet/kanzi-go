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
	"os"
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
	_KANZI_VERSION   = "2.4.0"
	_APP_HEADER      = "Kanzi " + _KANZI_VERSION + " (c) Frederic Langlet"
	_APP_SUB_HEADER  = "Fast lossless data compressor."
	_APP_USAGE       = "Usage: Kanzi [-c|-d] [flags and files in any order]"
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
	_ARG_CHECKSUM    = "--checksum="
)

var (
	_CMD_LINE_ARGS = []string{
		"-c", "-d", "-i", "-o", "-b", "-t", "-e", "-j",
		"-v", "-l", "-s", "-x", "-f", "-h", "-p",
	}

	mutex sync.Mutex
	log   = Printer{os: bufio.NewWriter(os.Stdout)}
)

func main() {
	argsMap := make(map[string]any)

	if status := processCommandLine(os.Args, argsMap); status != 0 {
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
	checksum := 0
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
	verboseLevel := ""
	ctx := -1
	level := -1
	mode := " "
	autoBlockSize := false
	showHelp := false
	warningNoValOpt := "Warning: ignoring option [%s] with no value."
	warningCompressOpt := "Warning: ignoring option [%s]. Only applicable in compress mode."
	warningDecompressOpt := "Warning: ignoring option [%s]. Only applicable in decompress mode."
	warningDupOpt := "Warning: ignoring duplicate option [%s] (%s)"
	warningInvalidOpt := "Invalid %s provided on command line: %s"

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if arg == "-o" {
			ctx = _ARG_IDX_OUTPUT
			continue
		}

		if arg == "-i" {
			ctx = _ARG_IDX_INPUT
			continue
		}

		if arg == "-v" {
			ctx = _ARG_IDX_VERBOSE
			continue
		}

		// Extract verbosity, output and mode first
		if arg == "-c" || arg == _ARG_COMPRESS {
			if mode == "d" {
				fmt.Println("Both compression and decompression options were provided.")
				return kanzi.ERR_INVALID_PARAM
			}

			mode = "c"
			continue
		}

		if arg == "-d" || arg == _ARG_DECOMPRESS {
			if mode == "c" {
				fmt.Println("Both compression and decompression options were provided.")
				return kanzi.ERR_INVALID_PARAM
			}

			mode = "d"
			continue
		}

		if ctx == _ARG_IDX_VERBOSE || strings.HasPrefix(arg, _ARG_VERBOSE) {
			if verboseLevel != "" {
				log.Println(fmt.Sprintf(warningDupOpt, "verbose", verboseLevel), verbose > 0)
			} else {

				if ctx == _ARG_IDX_VERBOSE {
					verboseLevel = arg
				} else {
					verboseLevel = strings.TrimPrefix(arg, _ARG_VERBOSE)
				}

				verboseLevel = strings.TrimSpace(verboseLevel)
				var err error

				if verbose, err = strconv.Atoi(verboseLevel); err != nil {
					fmt.Printf("Invalid verbosity level provided on command line: %v\n", arg)
					return kanzi.ERR_INVALID_PARAM
				}

				if verbose < 0 || verbose > 5 {
					fmt.Printf("Invalid verbosity level provided on command line: %v\n", arg)
					return kanzi.ERR_INVALID_PARAM
				}
			}
		} else if ctx == _ARG_IDX_OUTPUT || strings.HasPrefix(arg, _ARG_OUTPUT) {
			if ctx == _ARG_IDX_OUTPUT {
				outputName = arg
			} else {
				outputName = strings.TrimPrefix(arg, _ARG_OUTPUT)
			}

			outputName = strings.TrimSpace(outputName)
		} else if ctx == _ARG_IDX_INPUT || strings.HasPrefix(arg, _ARG_INPUT) {
			if ctx == _ARG_IDX_INPUT {
				inputName = arg
			} else {
				inputName = strings.TrimPrefix(arg, _ARG_INPUT)
			}

			inputName = strings.TrimSpace(inputName)
		} else if arg == "-h" || arg == "--help" {
			showHelp = true
		}

		ctx = -1
	}

	if showHelp == true || len(args) == 1 {
		printHelp(mode, true)
		return 0
	}

	// Overwrite verbosity if the output goes to stdout
	if (len(inputName) == 0 && len(outputName) == 0) || strings.EqualFold(outputName, "STDOUT") == true {
		verbose = 0
	}

	log.Println("\n"+_APP_HEADER+"\n", verbose >= 1)
	log.Println(_APP_SUB_HEADER, verbose > 1)
	inputName = ""
	outputName = ""
	ctx = -1

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if arg == "-c" || arg == "-d" || arg == _ARG_COMPRESS || arg == _ARG_DECOMPRESS {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			ctx = -1
			continue
		}

		if arg == "-f" || arg == _ARG_FORCE {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			overwrite = true
			ctx = -1
			continue
		}

		if arg == "-s" || arg == _ARG_SKIP {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			skip = true
			ctx = -1
			continue
		}

		if arg == "-x" || arg == "-x32" || arg == "-x64" {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "checksum"), verbose > 0)
			} else if checksum > 0 {
				log.Println(fmt.Sprintf(warningDupOpt, "checksum", "true"), verbose > 0)
			}

			if arg == "-x64" {
				checksum = 64
			} else {
				checksum = 32
			}

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

			noDotFiles = true
			ctx = -1
			continue
		}

		if arg == "--no-link" {
			if ctx != -1 {
				log.Println(fmt.Sprintf(warningNoValOpt, _CMD_LINE_ARGS[ctx]), verbose > 0)
			}

			noLinks = true
			ctx = -1
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

		if ctx == _ARG_IDX_OUTPUT || strings.HasPrefix(arg, _ARG_OUTPUT) {
			name := ""

			if ctx == _ARG_IDX_OUTPUT {
				name = arg
			} else {
				name = strings.TrimPrefix(arg, _ARG_OUTPUT)
			}

			if outputName != "" {
				log.Println(fmt.Sprintf(warningDupOpt, "output name", name), verbose > 0)
			} else {
				outputName = name
			}

			ctx = -1
			continue
		}

		if ctx == _ARG_IDX_INPUT || strings.HasPrefix(arg, _ARG_INPUT) {
			name := ""

			if ctx == _ARG_IDX_INPUT {
				name = arg
			} else {
				name = strings.TrimPrefix(arg, _ARG_INPUT)
			}

			if inputName != "" {
				log.Println(fmt.Sprintf(warningDupOpt, "input name", name), verbose > 0)
			} else {
				inputName = name
			}

			ctx = -1
			continue
		}

		if ctx == _ARG_IDX_ENTROPY || strings.HasPrefix(arg, _ARG_ENTROPY) {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "entropy"), verbose > 0)
				ctx = -1
				continue
			}

			name := ""

			if ctx == _ARG_IDX_ENTROPY {
				name = arg
			} else {
				name = strings.TrimPrefix(arg, _ARG_ENTROPY)
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

		if ctx == _ARG_IDX_TRANSFORM || strings.HasPrefix(arg, _ARG_TRANSFORM) {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "transform"), verbose > 0)
				ctx = -1
				continue
			}

			name := ""

			if ctx == _ARG_IDX_TRANSFORM {
				name = arg
			} else {
				name = strings.TrimPrefix(arg, _ARG_TRANSFORM)
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

		if ctx == _ARG_IDX_LEVEL || strings.HasPrefix(arg, _ARG_LEVEL) {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "level"), verbose > 0)
				ctx = -1
				continue
			}

			var str string
			var err error

			if ctx == _ARG_IDX_LEVEL {
				str = arg
			} else {
				str = strings.TrimPrefix(arg, _ARG_LEVEL)
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

		if strings.HasPrefix(arg, _ARG_CHECKSUM) {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "checksum"), verbose > 0)
				ctx = -1
				continue
			}

			str := strings.TrimPrefix(arg, _ARG_CHECKSUM)
			str = strings.TrimSpace(str)

			if checksum != 0 {
				log.Println(fmt.Sprintf(warningDupOpt, "checksum", str), verbose > 0)
				ctx = -1
				continue
			}

			var err error
			checksum, err = strconv.Atoi(str)

			if err != nil || (checksum != 32 && checksum != 64) {
				fmt.Println(fmt.Sprintf(warningInvalidOpt, "checksum", str))
				return kanzi.ERR_INVALID_PARAM
			}

			ctx = -1
			continue
		}

		if ctx == _ARG_IDX_PROFILE || strings.HasPrefix(arg, _ARG_CPUPROF) {
			name := ""

			if ctx == _ARG_IDX_PROFILE {
				name = arg
			} else {
				name = strings.TrimPrefix(arg, _ARG_CPUPROF)
			}

			if cpuProf != "" {
				log.Println(fmt.Sprintf(warningDupOpt, "profile name", name), verbose > 0)
			} else {
				cpuProf = name
			}

			ctx = -1
			continue
		}

		if ctx == _ARG_IDX_BLOCK || strings.HasPrefix(arg, _ARG_BLOCK) {
			if mode != "c" {
				log.Println(fmt.Sprintf(warningCompressOpt, "block size"), verbose > 0)
				ctx = -1
				continue
			}

			var strBlockSize string

			if ctx == _ARG_IDX_BLOCK {
				strBlockSize = arg
			} else {
				strBlockSize = strings.TrimPrefix(arg, _ARG_BLOCK)
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

		if ctx == _ARG_IDX_JOBS || strings.HasPrefix(arg, _ARG_JOBS) {
			var strTasks string
			var err error

			if ctx == _ARG_IDX_JOBS {
				strTasks = arg
			} else {
				strTasks = strings.TrimPrefix(arg, _ARG_JOBS)
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

		if ctx == -1 && strings.HasPrefix(arg, _ARG_FROM) {
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
					fmt.Println("The first block ID is 1.")
				}

				return kanzi.ERR_INVALID_PARAM
			}

			continue
		}

		if ctx == -1 && strings.HasPrefix(arg, _ARG_TO) {
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
	argsMap["inputName"] = inputName
	argsMap["outputName"] = outputName

	if overwrite == true {
		argsMap["overwrite"] = true
	}

	if mode == "c" && level != -1 {
		argsMap["level"] = level
	}

	if len(codec) > 0 {
		argsMap["entropy"] = codec
	}

	if len(transform) > 0 {
		argsMap["transform"] = transform
	}

	if checksum != 0 {
		argsMap["checksum"] = uint(checksum)
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
		log.Println("\n"+_APP_HEADER+"\n", true)
		log.Println(_APP_SUB_HEADER, true)
		log.Println(_APP_USAGE, true)
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
	log.Println("        Name of the input file or directory or 'stdin'.", true)
	log.Println("        When the source is a directory, all files in it will be processed.", true)
	msg := fmt.Sprintf("        Provide %c. at the end of the directory name to avoid recursion.", os.PathSeparator)
	log.Println(msg, true)
	msg = fmt.Sprintf("        (EG: myDir%c. => no recursion)", os.PathSeparator)
	log.Println(msg, true)
	log.Println("        If this option is not provided, Kanzi reads data from stdin.\n", true)
	log.Println("   -o, --output=<outputName>", true)

	if mode == "c" {
		log.Println("        Optional name of the output file or directory (defaults to", true)
		log.Println("        <inputName.knz> if input is <inputName> or 'stdout' if input is 'stdin').", true)
		log.Println("        or 'none' or 'stdout'.", true)
	} else if mode == "d" {
		log.Println("        Optional name of the output file or directory (defaults to", true)
		log.Println("        <inputName> if input is <inputName.knz> or 'stdout' if input is 'stdin').", true)
		log.Println("        or 'none' or 'stdout'.", true)

	} else {
		log.Println("        optional name of the output file or 'none' or 'stdout'.\n", true)
	}

	if mode == "c" {
		log.Println("   -b, --block=<size>", true)
		log.Println("        Size of blocks (default 4|8|16|32 MiB based on level, max 1 GiB, min 1 KiB).", true)
		log.Println("        'auto' means that the compressor derives the best value'", true)
		log.Println("        based on input size (when available) and number of jobs.\n", true)
		log.Println("   -l, --level=<compression>", true)
		log.Println("        Set the compression level [0..9]", true)
		log.Println("        Providing this option forces entropy and transform.", true)
		log.Println("        Defaults to level 3 if not provided.", true)
		log.Println("        0=NONE&NONE (store)", true)
		log.Println("        1=LZX&NONE", true)
		log.Println("        2=DNA+LZ&HUFFMAN", true)
		log.Println("        3=TEXT+UTF+PACK+MM+LZX&HUFFMAN", true)
		log.Println("        4=TEXT+UTF+EXE+PACK+MM+ROLZ&NONE", true)
		log.Println("        5=TEXT+UTF+BWT+RANK+ZRLT&ANS0", true)
		log.Println("        6=TEXT+UTF+BWT+SRT+ZRLT&FPAQ", true)
		log.Println("        7=LZP+TEXT+UTF+BWT+LZP&CM", true)
		log.Println("        8=EXE+RLT+TEXT+UTF+DNA&TPAQ", true)
		log.Println("        9=EXE+RLT+TEXT+UTF+DNA&TPAQX\n", true)
		log.Println("   -e, --entropy=<codec>", true)
		log.Println("        Entropy codec [None|Huffman|ANS0|ANS1|Range|FPAQ|TPAQ|TPAQX|CM]\n", true)
		log.Println("   -t, --transform=<codec>", true)
		log.Println("        Transform [None|BWT|BWTS|LZ|LZX|LZP|ROLZ|ROLZX|RLT|ZRLT]", true)
		log.Println("                  [MTFT|RANK|SRT|TEXT|MM|EXE|UTF|PACK]", true)
		log.Println("        EG: BWT+RANK or BWTS+MTFT\n", true)
		log.Println("   -x, -x32, -x64, --checksum=<size>", true)
		log.Println("        Enable block checksum (32 or 64 bits).", true)
		log.Println("        -x is equivalent to -x32.\n", true)
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
