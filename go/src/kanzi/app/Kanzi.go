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
	kio "kanzi/io"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
)

const (
	ARG_IDX_COMPRESS   = 0
	ARG_IDX_DECOMPRESS = 1
	ARG_IDX_INPUT      = 2
	ARG_IDX_OUTPUT     = 3
	ARG_IDX_BLOCK      = 4
	ARG_IDX_TRANSFORM  = 5
	ARG_IDX_ENTROPY    = 6
	ARG_IDX_JOBS       = 7
	ARG_IDX_VERBOSE    = 8
	ARG_IDX_LEVEL      = 9
	ARG_IDX_PROFILE    = 13
)

var (
	CMD_LINE_ARGS = []string{
		"-c", "-d", "-i", "-o", "-b", "-t", "-e", "-j",
		"-v", "-l", "-x", "-f", "-h", "-p",
	}
)

func main() {
	argsMap := make(map[string]interface{})
	processCommandLine(os.Args, argsMap)
	mode := argsMap["mode"].(string)
	delete(argsMap, "mode")

	if mode == "c" {
		runtime.GOMAXPROCS(runtime.NumCPU())
		code := 0
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("An unexpected error occured during compression: %v\n", r.(error))
				code = kio.ERR_UNKNOWN
			}

			os.Exit(code)
		}()

		bc, err := NewBlockCompressor(argsMap)

		if err != nil {
			fmt.Printf("Failed to create block compressor: %v\n", err)
			os.Exit(kio.ERR_CREATE_COMPRESSOR)
		}

		if len(bc.CpuProf()) != 0 {
			if f, err := os.Create(bc.CpuProf()); err != nil {
				fmt.Printf("Warning: cpu profile unavailable: %v\n", err)
			} else {
				pprof.StartCPUProfile(f)

				defer func() {
					pprof.StopCPUProfile()
					f.Close()
				}()
			}
		}

		code, _ = bc.Call()
		return
	}

	if mode == "d" {
		runtime.GOMAXPROCS(runtime.NumCPU())
		code := 0

		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("An unexpected error occured during decompression: %v\n", r.(error))
				code = kio.ERR_UNKNOWN
			}

			os.Exit(code)
		}()

		bd, err := NewBlockDecompressor(argsMap)

		if err != nil {
			fmt.Printf("Failed to create block decompressor: %v\n", err)
			os.Exit(kio.ERR_CREATE_DECOMPRESSOR)
		}

		if len(bd.CpuProf()) != 0 {
			if f, err := os.Create(bd.CpuProf()); err != nil {
				fmt.Printf("Warning: cpu profile unavailable: %v\n", err)
			} else {
				pprof.StartCPUProfile(f)

				defer func() {
					pprof.StopCPUProfile()
					f.Close()
				}()
			}
		}

		code, _ = bd.Call()
		os.Exit(code)
	}

	if _, prst := argsMap["help"]; prst == true {
		print(os.Args[0])
		println(" --compress | --decompress | --help")
		os.Exit(0)
	}

	println("Missing arguments: try --help or -h")
	os.Exit(1)
}

func processCommandLine(args []string, argsMap map[string]interface{}) {
	blockSize := -1
	verbose := 2
	overwrite := false
	checksum := false
	inputName := ""
	outputName := ""
	codec := ""
	transform := ""
	tasks := 0
	cpuProf := ""
	ctx := -1
	level := -1
	mode := " "

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if arg == "-o" {
			ctx = ARG_IDX_OUTPUT
			continue
		}

		if arg == "-v" {
			ctx = ARG_IDX_VERBOSE
			continue
		}

		// Extract verbosity, output and mode first
		if arg == "--compress" || arg == "-c" {
			if mode == "d" {
				fmt.Println("Both compression and decompression options were provided.")
				os.Exit(kio.ERR_INVALID_PARAM)
			}

			mode = "c"
			continue
		}

		if arg == "--decompress" || arg == "-d" {
			if mode == "c" {
				fmt.Println("Both compression and decompression options were provided.")
				os.Exit(kio.ERR_INVALID_PARAM)
			}

			mode = "d"
			continue
		}

		if strings.HasPrefix(arg, "--verbose=") || ctx == ARG_IDX_VERBOSE {
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

			if verbose < 0 || verbose > 5 {
				fmt.Printf("Invalid verbosity level provided on command line: %v\n", arg)
				os.Exit(kio.ERR_INVALID_PARAM)
			}
		} else if strings.HasPrefix(arg, "--output=") || ctx == ARG_IDX_OUTPUT {
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
			printOut("", true)
			printOut("   -h, --help", true)
			printOut("        display this message\n", true)
			printOut("   -v, --verbose=<level>", true)
			printOut("        set the verbosity level [0..5]", true)
			printOut("        0=silent, 1=compact, 2=default, 3=display configuration,", true)
			printOut("        4=display block size and timings, 5=display extra information\n", true)
			printOut("   -f, --force", true)
			printOut("        overwrite the output file if it already exists\n", true)
			printOut("   -i, --input=<inputName>", true)
			printOut("        mandatory name of the input file or 'stdin'\n", true)
			printOut("   -o, --output=<outputName>", true)
			printOut("        optional name of the output file (defaults to <input.knz>) or 'none'", true)
			printOut("        or 'stdout'\n", true)

			if mode != "d" {
				printOut("   -b, --block=<size>", true)
				printOut("        size of blocks, multiple of 16, max 1 GB, min 1 KB, default 1 MB\n", true)
				printOut("   -l, --level=<compression>", true)
				printOut("        set the compression level [0..5]", true)
				printOut("        Providing this option forces entropy and transform.", true)
				printOut("        0=None&None (store), 1=TEXT+LZ4&HUFFMAN, 2=BWT+RANK+ZRLT&ANS0", true)
				printOut("        3=BWT+RANK+ZRLT&FPAQ, 4=BWT&CM, 5=RLT+TEXT&TPAQ\n", true)
				printOut("   -e, --entropy=<codec>", true)
				printOut("        entropy codec [None|Huffman|ANS0|ANS1|Range|PAQ|FPAQ|TPAQ|CM]", true)
				printOut("        (default is ANS0)\n", true)
				printOut("   -t, --transform=<codec>", true)
				printOut("        transform [None|BWT|BWTS|SNAPPY|LZ4|RLT|ZRLT|MTFT|RANK|TEXT|TIMESTAMP]", true)
				printOut("        EG: BWT+RANK or BWTS+MTFT (default is BWT+RANK+ZRLT)\n", true)
				printOut("   -x, --checksum", true)
				printOut("        enable block checksum\n", true)
			}

			printOut("   -j, --jobs=<jobs>", true)
			printOut("        number of concurrent jobs\n", true)
			printOut("", true)

			if mode != "d" {
				printOut("EG. Kanzi -c -i foo.txt -o none -b 4m -l 4 -v 3\n", true)
				printOut("EG. Kanzi -c -i foo.txt -o foo.knz -f -t BWT+MTFT+ZRLT -b 4m -e FPAQ -v 3 -j 4\n", true)
				printOut("EG. Kanzi --compress --input=foo.txt --output=foo.knz --block=4m --force", true)
				printOut("          --transform=BWT+MTFT+ZRLT --entropy=FPAQ --verbose=3 --jobs=4\n", true)
			}

			if mode != "c" {
				printOut("EG. Kanzi -d -i foo.knz -f -v 2 -j 2\n", true)
				printOut("EG. Kanzi --decompress --input=foo.knz --force --verbose=2 --jobs=2\n", true)
			}

			os.Exit(0)
		}

		if arg == "--compress" || arg == "-c" || arg == "--decompress" || arg == "-d" {
			if ctx != -1 {
				printOut("Warning: ignoring option ["+CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			ctx = -1
			continue
		}

		if arg == "--force" || arg == "-f" {
			if ctx != -1 {
				printOut("Warning: ignoring option ["+CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			overwrite = true
			ctx = -1
			continue
		}

		if arg == "--checksum" || arg == "-x" {
			if ctx != -1 {
				printOut("Warning: ignoring option ["+CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			checksum = true
			ctx = -1
			continue
		}

		if ctx == -1 {
			idx := -1

			for i, v := range CMD_LINE_ARGS {
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

		if strings.HasPrefix(arg, "--input=") || ctx == ARG_IDX_INPUT {
			if strings.HasPrefix(arg, "--input=") {
				inputName = strings.TrimPrefix(arg, "--input=")
			} else {
				inputName = arg
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--entropy=") || ctx == ARG_IDX_ENTROPY {
			if strings.HasPrefix(arg, "--entropy=") {
				codec = strings.TrimPrefix(arg, "--entropy=")
			} else {
				codec = arg
			}

			codec = strings.ToUpper(codec)
			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--transform=") || ctx == ARG_IDX_TRANSFORM {
			if strings.HasPrefix(arg, "--transform=") {
				transform = strings.TrimPrefix(arg, "--transform=")
			} else {
				transform = arg
			}

			transform = strings.ToUpper(transform)
			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--level=") || ctx == ARG_IDX_LEVEL {
			var str string
			var err error

			if strings.HasPrefix(arg, "--level=") {
				str = strings.TrimPrefix(arg, "--level=")
			} else {
				str = arg
			}

			str = strings.TrimSpace(str)

			if level, err = strconv.Atoi(str); err != nil {
				fmt.Printf("Invalid compression level provided on command line: %v\n", arg)
				os.Exit(kio.ERR_INVALID_PARAM)
			}

			if level < 0 || level > 5 {
				fmt.Printf("Invalid compression level provided on command line: %v\n", arg)
				os.Exit(kio.ERR_INVALID_PARAM)
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--cpuProf=") || ctx == ARG_IDX_PROFILE {
			if strings.HasPrefix(arg, "--cpuProf=") {
				cpuProf = strings.TrimPrefix(arg, "--cpuProf=")
			} else {
				cpuProf = arg
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--block=") || ctx == ARG_IDX_BLOCK {
			var strBlockSize string

			if strings.HasPrefix(arg, "--block=") {
				strBlockSize = strings.TrimPrefix(arg, "--block=")
			} else {
				strBlockSize = arg
			}

			strBlockSize = strings.ToUpper(strBlockSize)

			// Process K or M suffix
			scale := 1
			lastChar := byte(0)

			if len(strBlockSize) > 0 {
				lastChar = strBlockSize[len(strBlockSize)-1]
			}

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

		if strings.HasPrefix(arg, "--jobs=") || ctx == ARG_IDX_JOBS {
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

		if !strings.HasPrefix(arg, "--verbose=") && !strings.HasPrefix(arg, "--output=") &&
			ctx == -1 && !strings.HasPrefix(arg, "--cpuProf=") {
			printOut("Warning: ignoring unknown option ["+arg+"]", verbose > 0)
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
		printOut("Warning: ignoring option with missing value ["+CMD_LINE_ARGS[ctx]+"]", verbose > 0)
	}

	if level >= 0 {
		if len(codec) != 0 {
			printOut("Warning: providing the 'level' option forces the entropy codec. Ignoring ["+codec+"]", verbose > 0)
		}

		if len(transform) != 0 {
			printOut("Warning: providing the 'level' option forces the transform. Ignoring ["+transform+"]", verbose > 0)
		}
	}

	if blockSize != -1 {
		argsMap["block"] = uint(blockSize)
	}

	argsMap["verbose"] = uint(verbose)
	argsMap["mode"] = mode

	if overwrite == true {
		argsMap["overwrite"] = overwrite
	}

	argsMap["inputName"] = inputName
	argsMap["outputName"] = outputName

	if mode == "c" {
		argsMap["level"] = level
	}

	if len(codec) > 0 {
		argsMap["entropy"] = codec
	}

	if len(transform) > 0 {
		argsMap["transform"] = transform
	}

	if checksum == true {
		argsMap["checksum"] = checksum
	}

	argsMap["jobs"] = uint(tasks)

	if len(cpuProf) > 0 {
		argsMap["cpuProf"] = cpuProf
	}
}

func printOut(msg string, print bool) {
	if print == true {
		fmt.Println(msg)
	}
}
