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
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"

	kanzi "github.com/flanglet/kanzi-go"
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
	_APP_HEADER      = "Kanzi 1.8 (C) 2020,  Frederic Langlet"
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
	argsMap := make(map[string]interface{})

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

func compress(argsMap map[string]interface{}) int {
	runtime.GOMAXPROCS(runtime.NumCPU())
	code := 0

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

	code, _ = bc.Compress()
	return code
}

func decompress(argsMap map[string]interface{}) int {
	runtime.GOMAXPROCS(runtime.NumCPU())
	code := 0

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

	code, _ = bd.Decompress()
	return code
}

func processCommandLine(args []string, argsMap map[string]interface{}) int {
	blockSize := -1
	verbose := 1
	overwrite := false
	checksum := false
	skip := false
	from := -1
	to := -1
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
			ctx = _ARG_IDX_OUTPUT
			continue
		}

		if arg == "-v" {
			ctx = _ARG_IDX_VERBOSE
			continue
		}

		// Extract verbosity, output and mode first
		if arg == "--compress" || arg == "-c" {
			if mode == "d" {
				fmt.Println("Both compression and decompression options were provided.")
				return kanzi.ERR_INVALID_PARAM
			}

			mode = "c"
			continue
		}

		if arg == "--decompress" || arg == "-d" {
			if mode == "c" {
				fmt.Println("Both compression and decompression options were provided.")
				return kanzi.ERR_INVALID_PARAM
			}

			mode = "d"
			continue
		}

		if strings.HasPrefix(arg, "--verbose=") || ctx == _ARG_IDX_VERBOSE {
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
				return kanzi.ERR_INVALID_PARAM
			}

			if verbose < 0 || verbose > 5 {
				fmt.Printf("Invalid verbosity level provided on command line: %v\n", arg)
				return kanzi.ERR_INVALID_PARAM
			}
		} else if strings.HasPrefix(arg, "--output=") || ctx == _ARG_IDX_OUTPUT {
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
		log.Println("\n"+_APP_HEADER+"\n", true)
	}

	outputName = ""
	ctx = -1

	for i, arg := range args {
		if i == 0 {
			continue
		}

		arg = strings.TrimSpace(arg)

		if arg == "--help" || arg == "-h" {
			log.Println("", true)
			log.Println("Credits: Matt Mahoney, Yann Collet, Jan Ondrus, Yuta Mori, Ilya Muravyov,", true)
			log.Println("         Neal Burns, Fabian Giesen, Jarek Duda, Ilya Grebnov", true)
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
				log.Println("        set the compression level [0..8]", true)
				log.Println("        Providing this option forces entropy and transform.", true)
				log.Println("        0=None&None (store), 1=TEXT+LZ&HUFFMAN, 2=TEXT+FSD+ROLZ", true)
				log.Println("        3=TEXT+FSD+ROLZX, 4=TEXT+BWT+RANK+ZRLT&ANS0, 5=TEXT+BWT+SRT+ZRLT&FPAQ", true)
				log.Println("        6=LZP+TEXT+BWT&CM, 7=X86+RLT+TEXT&TPAQ, 8=X86+RLT+TEXT&TPAQX\n", true)
				log.Println("   -e, --entropy=<codec>", true)
				log.Println("        entropy codec [None|Huffman|ANS0|ANS1|Range|FPAQ|TPAQ|TPAQX|CM]", true)
				log.Println("        (default is ANS0)\n", true)
				log.Println("   -t, --transform=<codec>", true)
				log.Println("        transform [None|BWT|BWTS|LZ|LZP|ROLZ|ROLZX|RLT|ZRLT]", true)
				log.Println("                  [MTFT|RANK|SRT|TEXT|X86]", true)
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

			return 0
		}

		if arg == "--compress" || arg == "-c" || arg == "--decompress" || arg == "-d" {
			if ctx != -1 {
				log.Println("Warning: ignoring option ["+_CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			ctx = -1
			continue
		}

		if arg == "--force" || arg == "-f" {
			if ctx != -1 {
				log.Println("Warning: ignoring option ["+_CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			overwrite = true
			ctx = -1
			continue
		}

		if arg == "--skip" || arg == "-s" {
			if ctx != -1 {
				log.Println("Warning: ignoring option ["+_CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			skip = true
			ctx = -1
			continue
		}

		if arg == "--checksum" || arg == "-x" {
			if ctx != -1 {
				log.Println("Warning: ignoring option ["+_CMD_LINE_ARGS[ctx]+"] with no value.", verbose > 0)
			}

			checksum = true
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

		if strings.HasPrefix(arg, "--output=") || ctx == _ARG_IDX_OUTPUT {
			name := ""

			if strings.HasPrefix(arg, "--output=") {
				name = strings.TrimPrefix(arg, "--output=")
			} else {
				name = arg
			}

			if outputName != "" {
				fmt.Printf("Warning: ignoring duplicate output name: %v\n", name)
			} else {
				outputName = name
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--input=") || ctx == _ARG_IDX_INPUT {
			name := ""

			if strings.HasPrefix(arg, "--input=") {
				name = strings.TrimPrefix(arg, "--input=")
			} else {
				name = arg
			}

			if inputName != "" {
				fmt.Printf("Warning: ignoring duplicate input name: %v\n", name)
			} else {
				inputName = name
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--entropy=") || ctx == _ARG_IDX_ENTROPY {
			name := ""

			if strings.HasPrefix(arg, "--entropy=") {
				name = strings.TrimPrefix(arg, "--entropy=")
			} else {
				name = arg
			}

			if codec != "" {
				fmt.Printf("Warning: ignoring duplicate entropy: %v\n", name)
			} else {
				codec = strings.ToUpper(name)
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--transform=") || ctx == _ARG_IDX_TRANSFORM {
			name := ""

			if strings.HasPrefix(arg, "--transform=") {
				name = strings.TrimPrefix(arg, "--transform=")
			} else {
				name = arg
			}

			if transform != "" {
				fmt.Printf("Warning: ignoring duplicate transform: %v\n", name)
			} else {
				transform = strings.ToUpper(name)
			}

			for len(transform) > 0 && transform[0] == '+' {
				transform = transform[1:]
			}

			for len(transform) > 0 && transform[len(transform)-1] == '+' {
				transform = transform[0 : len(transform)-1]
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--level=") || ctx == _ARG_IDX_LEVEL {
			var str string
			var err error

			if strings.HasPrefix(arg, "--level=") {
				str = strings.TrimPrefix(arg, "--level=")
			} else {
				str = arg
			}

			str = strings.TrimSpace(str)

			if level != -1 {
				fmt.Printf("Warning: ignoring duplicate level: %v\n", str)
				ctx = -1
				continue
			}

			if level, err = strconv.Atoi(str); err != nil {
				fmt.Printf("Invalid compression level provided on command line: %v\n", arg)
				return kanzi.ERR_INVALID_PARAM
			}

			if level < 0 || level > 8 {
				fmt.Printf("Invalid compression level provided on command line: %v\n", arg)
				return kanzi.ERR_INVALID_PARAM
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--cpuProf=") || ctx == _ARG_IDX_PROFILE {
			name := ""

			if strings.HasPrefix(arg, "--cpuProf=") {
				name = strings.TrimPrefix(arg, "--cpuProf=")
			} else {
				name = arg
			}

			if cpuProf != "" {
				fmt.Printf("Warning: ignoring duplicate profile file name: %v\n", name)
			} else {
				cpuProf = name
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--block=") || ctx == _ARG_IDX_BLOCK {
			var strBlockSize string

			if strings.HasPrefix(arg, "--block=") {
				strBlockSize = strings.TrimPrefix(arg, "--block=")
			} else {
				strBlockSize = arg
			}

			strBlockSize = strings.ToUpper(strBlockSize)

			if blockSize != -1 {
				fmt.Printf("Warning: ignoring duplicate block size: %v\n", strBlockSize)
				ctx = -1
				continue
			}

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
				return kanzi.ERR_BLOCK_SIZE
			}

			blockSize = scale * blockSize
			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--jobs=") || ctx == _ARG_IDX_JOBS {
			var strTasks string
			var err error

			if strings.HasPrefix(arg, "--jobs=") {
				strTasks = strings.TrimPrefix(arg, "--jobs=")
			} else {
				strTasks = arg
			}

			if tasks != 0 {
				fmt.Printf("Warning: ignoring duplicate jobs: %v\n", strTasks)
				ctx = -1
				continue
			}

			if tasks, err = strconv.Atoi(strTasks); err != nil || tasks < 1 {
				fmt.Printf("Invalid number of jobs provided on command line: %v\n", strTasks)
				return kanzi.ERR_BLOCK_SIZE
			}

			ctx = -1
			continue
		}

		if strings.HasPrefix(arg, "--from=") && ctx == -1 {
			var strFrom string
			var err error

			if strings.HasPrefix(arg, "--from=") {
				strFrom = strings.TrimPrefix(arg, "--from=")
			} else {
				strFrom = arg
			}

			if from != -1 {
				fmt.Printf("Warning: ignoring duplicate start block: %v\n", strFrom)
				continue
			}

			if from, err = strconv.Atoi(strFrom); err != nil || from < 0 {
				fmt.Printf("Invalid start block provided on command line: %v\n", strFrom)
				return kanzi.ERR_INVALID_PARAM
			}

			continue
		}

		if strings.HasPrefix(arg, "--to=") && ctx == -1 {
			var strTo string
			var err error

			if strings.HasPrefix(arg, "--to=") {
				strTo = strings.TrimPrefix(arg, "--to=")
			} else {
				strTo = arg
			}

			if to != -1 {
				fmt.Printf("Warning: ignoring duplicate end block: %v\n", strTo)
				continue
			}

			if to, err = strconv.Atoi(strTo); err != nil || to <= 0 {
				fmt.Printf("Invalid end block provided on command line: %v\n", strTo)
				return kanzi.ERR_INVALID_PARAM
			}

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
		return kanzi.ERR_MISSING_PARAM
	}

	if ctx != -1 {
		log.Println("Warning: ignoring option with missing value ["+_CMD_LINE_ARGS[ctx]+"]", verbose > 0)
	}

	if level >= 0 {
		if len(codec) != 0 {
			log.Println("Warning: providing the 'level' option forces the entropy codec. Ignoring ["+codec+"]", verbose > 0)
		}

		if len(transform) != 0 {
			log.Println("Warning: providing the 'level' option forces the transform. Ignoring ["+transform+"]", verbose > 0)
		}
	}

	if from >= 0 || to >= 0 {
		if mode != "d" {
			log.Println("Warning: ignoring start/end block (only valid for decompression)", verbose > 0)
			from = -1
			to = -1
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

	if mode == "c" || level != -1 {
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

	if from >= 0 {
		argsMap["from"] = from
	}

	if to >= 0 {
		argsMap["to"] = to
	}

	return 0
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

func createFileList(target string, fileList []FileData) ([]FileData, error) {
	fi, err := os.Stat(target)

	if err != nil {
		return fileList, err
	}

	if fi.Mode().IsRegular() {
		if fi.Name()[0] != '.' {
			fileList = append(fileList, *NewFileData(target, fi.Size()))
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
				fileList = append(fileList, *NewFileData(path, fi.Size()))
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
					fileList = append(fileList, *NewFileData(target+fi.Name(), fi.Size()))
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
