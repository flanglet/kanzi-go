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
	"bufio"
	"fmt"
	kanzi "github.com/flanglet/kanzi-go"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
)

const (
	//ARG_IDX_COMPRESS   = 0
	//ARG_IDX_DECOMPRESS = 1
	ARG_IDX_INPUT     = 2
	ARG_IDX_OUTPUT    = 3
	ARG_IDX_BLOCK     = 4
	ARG_IDX_TRANSFORM = 5
	ARG_IDX_ENTROPY   = 6
	ARG_IDX_JOBS      = 7
	ARG_IDX_VERBOSE   = 8
	ARG_IDX_LEVEL     = 9
	ARG_IDX_PROFILE   = 14
	APP_HEADER        = "Kanzi 1.4 (C) 2018,  Frederic Langlet"
)

var (
	CMD_LINE_ARGS = []string{
		"-c", "-d", "-i", "-o", "-b", "-t", "-e", "-j",
		"-v", "-l", "-s", "-x", "-f", "-h", "-p",
	}
	mutex sync.Mutex
	log   = Printer{os: bufio.NewWriter(os.Stdout)}
)

func main() {
	argsMap := make(map[string]interface{})
	processCommandLine(os.Args, argsMap)
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

func compress(argsMap map[string]interface{}) int {
	runtime.GOMAXPROCS(runtime.NumCPU())
	code := 0

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("An unexpected error occured during compression: %v\n", r.(error))
			code = kanzi.ERR_UNKNOWN
		}

		os.Exit(code)
	}()

	bc, err := NewBlockCompressor(argsMap)

	if err != nil {
		fmt.Printf("Failed to create block compressor: %v\n", err)
		return kanzi.ERR_CREATE_COMPRESSOR
	}

	if len(bc.CpuProf()) != 0 {
		if f, err := os.Create(bc.CpuProf()); err != nil {
			fmt.Printf("Warning: cpu profile unavailable: %v\n", err)
		} else {
			if err := pprof.StartCPUProfile(f); err != nil {
				fmt.Printf("Warning: cpu profile unavailable: %v\n", err)
			}
			defer func() {
				pprof.StopCPUProfile()
				f.Close()
			}()
		}
	}

	code, _ = bc.Call()
	return code
}

func decompress(argsMap map[string]interface{}) int {
	runtime.GOMAXPROCS(runtime.NumCPU())
	code := 0

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("An unexpected error occured during decompression: %v\n", r.(error))
			code = kanzi.ERR_UNKNOWN
		}

		os.Exit(code)
	}()

	bd, err := NewBlockDecompressor(argsMap)

	if err != nil {
		fmt.Printf("Failed to create block decompressor: %v\n", err)
		return kanzi.ERR_CREATE_DECOMPRESSOR
	}

	if len(bd.CpuProf()) != 0 {
		if f, err := os.Create(bd.CpuProf()); err != nil {
			fmt.Printf("Warning: cpu profile unavailable: %v\n", err)
		} else {
			if err := pprof.StartCPUProfile(f); err != nil {
				fmt.Printf("Warning: cpu profile unavailable: %v\n", err)
			}

			defer func() {
				pprof.StopCPUProfile()
				f.Close()
			}()
		}
	}

	code, _ = bd.Call()
	return code
}

func processCommandLine(args []string, argsMap map[string]interface{}) {
	blockSize := -1
	verbose := 1
	overwrite := false
	checksum := false
	skip := false
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
				os.Exit(kanzi.ERR_INVALID_PARAM)
			}

			mode = "c"
			continue
		}

		if arg == "--decompress" || arg == "-d" {
			if mode == "c" {
				fmt.Println("Both compression and decompression options were provided.")
				os.Exit(kanzi.ERR_INVALID_PARAM)
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
				os.Exit(kanzi.ERR_INVALID_PARAM)
			}

			if verbose < 0 || verbose > 5 {
				fmt.Printf("Invalid verbosity level provided on command line: %v\n", arg)
				os.Exit(kanzi.ERR_INVALID_PARAM)
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

	if verbose >= 1 {
		log.Println("\n"+APP_HEADER+"\n", true)
	}

	ctx = -1

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if arg == "--help" || arg == "-h" {
			log.Println("", true)
			log.Println("   -h, --help", true)
			log.Println("        display this message\n", true)
			log.Println("   -v, --verbose=<level>", true)
			log.Println("        set the verbosity level [0..5]", true)
			log.Println("        0=silent, 1=default, 2=display details, 3=display configuration,", true)
			log.Println("        4=display block size and timings, 5=display extra information", true)
			log.Println("        Verbosity is reduced to 1 when files are processed concurrently", true)
			log.Println("        Verbosity is silently reduced to 0 when the output is 'stdout'", true)
			log.Println("        (EG: The source is a directory and the number of jobs > 1).\n", true)
			log.Println("   -f, --force", true)
			log.Println("        overwrite the output file if it already exists\n", true)
			log.Println("   -i, --input=<inputName>", true)
			log.Println("        mandatory name of the input file or directory or 'stdin'", true)
			log.Println("        When the source is a directory, all files in it will be processed.", true)
			msg := fmt.Sprintf("        Provide %c. at the end of the directory name to avoid recursion.", os.PathSeparator)
			log.Println(msg, true)
			msg = fmt.Sprintf("        (EG: myDir%c. => no recursion)\n", os.PathSeparator)
			log.Println(msg, true)
			log.Println("   -o, --output=<outputName>", true)

			if mode == "c" {
				log.Println("        optional name of the output file or directory (defaults to", true)
				log.Println("        <inputName.knz>) or 'none' or 'stdout'. 'stdout' is not valid", true)
				log.Println("        when the number of jobs is greater than 1.\n", true)
			} else if mode == "d" {
				log.Println("        optional name of the output file or directory (defaults to", true)
				log.Println("        <inputName.bak>) or 'none' or 'stdout'. 'stdout' is not valid", true)
				log.Println("        when the number of jobs is greater than 1.\n", true)

			} else {
				log.Println("        optional name of the output file or 'none' or 'stdout'.\n", true)
			}

			if mode != "d" {
				log.Println("   -b, --block=<size>", true)
				log.Println("        size of blocks, multiple of 16 (default 1 MB, max 1 GB, min 1 KB).\n", true)
				log.Println("   -l, --level=<compression>", true)
				log.Println("        set the compression level [0..6]", true)
				log.Println("        Providing this option forces entropy and transform.", true)
				log.Println("        0=None&None (store), 1=TEXT+LZ4&HUFFMAN, 2=TEXT+ROLZ", true)
				log.Println("        3=BWT+RANK+ZRLT&ANS0, 4=BWT+RANK+ZRLT&FPAQ, 5=BWT&CM", true)
				log.Println("        6=X86+RLT+TEXT&TPAQ, 7=X86+RLT+TEXT&TPAQX\n", true)
				log.Println("   -e, --entropy=<codec>", true)
				log.Println("        entropy codec [None|Huffman|ANS0|ANS1|Range|FPAQ|TPAQ|TPAQX|CM]", true)
				log.Println("        (default is ANS0)\n", true)
				log.Println("   -t, --transform=<codec>", true)
				log.Println("        transform [None|BWT|BWTS|SNAPPY|LZ4|ROLZ|RLT|ZRLT|MTFT|RANK|TEXT|X86]", true)
				log.Println("        EG: BWT+RANK or BWTS+MTFT (default is BWT+RANK+ZRLT)\n", true)
				log.Println("   -x, --checksum", true)
				log.Println("        enable block checksum\n", true)
				log.Println("   -s, --skip", true)
				log.Println("        copy blocks with high entropy instead of compressing them.\n", true)
			}

			log.Println("   -j, --jobs=<jobs>", true)
			log.Println("        maximum number of jobs the program may start concurrently", true)
			log.Println("        (default is 1, maximum is 64).\n", true)
			log.Println("", true)

			if mode != "d" {
				log.Println("EG. Kanzi -c -i foo.txt -o none -b 4m -l 4 -v 3\n", true)
				log.Println("EG. Kanzi -c -i foo.txt -f -t BWT+MTFT+ZRLT -b 4m -e FPAQ -v 3 -j 4\n", true)
				log.Println("EG. Kanzi --compress --input=foo.txt --output=foo.knz --block=4m --force", true)
				log.Println("          --transform=BWT+MTFT+ZRLT --entropy=FPAQ --verbose=3 --jobs=4\n", true)
			}

			if mode != "c" {
				log.Println("EG. Kanzi -d -i foo.knz -f -v 2 -j 2\n", true)
				log.Println("EG. Kanzi --decompress --input=foo.knz --force --verbose=2 --jobs=2\n", true)
			}

			os.Exit(0)
		}

		if arg == "--compress" || arg == "-c" || arg == "--decompress" || arg == "-d" {
			if ctx != -1 {
				log.Println("Warning: ignoring option ["+CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			ctx = -1
			continue
		}

		if arg == "--force" || arg == "-f" {
			if ctx != -1 {
				log.Println("Warning: ignoring option ["+CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			overwrite = true
			ctx = -1
			continue
		}

		if arg == "--skip" || arg == "-s" {
			if ctx != -1 {
				log.Println("Warning: ignoring option ["+CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			skip = true
			ctx = -1
			continue
		}

		if arg == "--checksum" || arg == "-x" {
			if ctx != -1 {
				log.Println("Warning: ignoring option ["+CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
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
				os.Exit(kanzi.ERR_INVALID_PARAM)
			}

			if level < 0 || level > 7 {
				fmt.Printf("Invalid compression level provided on command line: %v\n", arg)
				os.Exit(kanzi.ERR_INVALID_PARAM)
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
				os.Exit(kanzi.ERR_BLOCK_SIZE)
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
				os.Exit(kanzi.ERR_BLOCK_SIZE)
			}

			ctx = -1
			continue
		}

		if !strings.HasPrefix(arg, "--verbose=") && !strings.HasPrefix(arg, "--output=") &&
			ctx == -1 && !strings.HasPrefix(arg, "--cpuProf=") {
			log.Println("Warning: ignoring unknown option ["+arg+"]", verbose > 0)
		}

		ctx = -1
	}

	if inputName == "" {
		fmt.Printf("Missing input file name, exiting ...\n")
		os.Exit(kanzi.ERR_MISSING_PARAM)
	}

	if ctx != -1 {
		log.Println("Warning: ignoring option with missing value ["+CMD_LINE_ARGS[ctx]+"]", verbose > 0)
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

	if skip == true {
		argsMap["skipBlocks"] = skip
	}

	argsMap["jobs"] = uint(tasks)

	if len(cpuProf) > 0 {
		argsMap["cpuProf"] = cpuProf
	}
}

type FileData struct {
	Path string
	Size int64
}

type FileCompareByName struct {
	data []FileData
}

func (this FileCompareByName) Len() int {
	return len(this.data)
}

func (this FileCompareByName) Swap(i, j int) {
	this.data[i], this.data[j] = this.data[j], this.data[i]
}

func (this FileCompareByName) Less(i, j int) bool {
	return strings.Compare(this.data[i].Path, this.data[j].Path) < 0
}

func createFileList(target string, fileList []FileData) ([]FileData, error) {
	fi, err := os.Stat(target)

	if err != nil {
		return fileList, err
	}

	if fi.Mode().IsRegular() {
		if fi.Name()[0] != '.' {
			fileList = append(fileList, FileData{Path: target, Size: fi.Size()})
		}

		return fileList, nil
	}

	suffix := string([]byte{os.PathSeparator, '.'})
	isRecursive := len(target) <= 2 || target[len(target)-len(suffix):] != suffix

	if isRecursive {
		if target[len(target)-1] != os.PathSeparator {
			target = target + string([]byte{os.PathSeparator})
		}

		err = filepath.Walk(target, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if fi.Mode().IsRegular() && fi.Name()[0] != '.' {
				fileList = append(fileList, FileData{Path: path, Size: fi.Size()})
			}

			return err
		})
	} else {
		// Remove suffix
		target = target[0 : len(target)-1]

		var files []os.FileInfo
		files, err = ioutil.ReadDir(target)

		if err == nil {
			for _, fi := range files {
				if fi.Mode().IsRegular() && fi.Name()[0] != '.' {
					fileList = append(fileList, FileData{Path: target + fi.Name(), Size: fi.Size()})
				}
			}
		}
	}

	return fileList, err
}

// Buffered printer is required in concurrent code
type Printer struct {
	os *bufio.Writer
}

func (this *Printer) Println(msg string, print bool) {
	if print == true {
		mutex.Lock()

		// Best effort, ignore error
		if w, _ := this.os.Write([]byte(msg + "\n")); w > 0 {
			_ = this.os.Flush()
		}

		mutex.Unlock()
	}
}
