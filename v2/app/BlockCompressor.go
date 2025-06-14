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
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	kanzi "github.com/flanglet/kanzi-go/v2"
	"github.com/flanglet/kanzi-go/v2/internal"
	kio "github.com/flanglet/kanzi-go/v2/io"
	"github.com/flanglet/kanzi-go/v2/transform"
)

const (
	_COMP_DEFAULT_BUFFER_SIZE = 65536
	_COMP_DEFAULT_BLOCK_SIZE  = 4 * 1024 * 1024
	_COMP_MIN_BLOCK_SIZE      = 1024
	_COMP_MAX_BLOCK_SIZE      = 1024 * 1024 * 1024
	_COMP_MAX_CONCURRENCY     = 64
	_COMP_NONE                = "NONE"
	_COMP_STDIN               = "STDIN"
	_COMP_STDOUT              = "STDOUT"
)

// BlockCompressor main block compressor struct
type BlockCompressor struct {
	verbosity     uint
	overwrite     bool
	checksum      uint
	skipBlocks    bool
	fileReorder   bool
	removeSource  bool
	noDotFiles    bool
	noLinks       bool
	autoBlockSize bool
	inputName     string
	outputName    string
	entropyCodec  string
	transform     string
	blockSize     uint
	jobs          uint
	listeners     []kanzi.Listener
	cpuProf       string
}

type fileCompressResult struct {
	code    int
	read    uint64
	written uint64
	err     error
}

// NewBlockCompressor creates a new instance of BlockCompressor given
// a map of argument name/value pairs.
func NewBlockCompressor(argsMap map[string]any) (*BlockCompressor, error) {
	this := &BlockCompressor{}
	this.listeners = make([]kanzi.Listener, 0)
	level := -1

	if lvl, prst := argsMap["level"]; prst == true {
		level = lvl.(int)

		if level < 0 || level > 9 {
			return nil, fmt.Errorf("Invalid compression level (must be in[0..9]), got %d ", level)
		}

		delete(argsMap, "level")
		tranformAndCodec := getTransformAndCodec(level)
		tokens := strings.Split(tranformAndCodec, "&")
		this.transform = tokens[0]
		this.entropyCodec = tokens[1]
	} else {
		codec, prstC := argsMap["entropy"]
		transf, prstF := argsMap["transform"]

		if prstC == false && prstF == false {
			// Default to level 3
			tranformAndCodec := getTransformAndCodec(3)
			tokens := strings.Split(tranformAndCodec, "&")
			this.transform = tokens[0]
			this.entropyCodec = tokens[1]
		} else {
			if prstC == true {
				this.entropyCodec = codec.(string)
				delete(argsMap, "entropy")
			} else {
				this.entropyCodec = "NONE"
			}

			if prstF == true {
				strTransf := transf.(string)
				delete(argsMap, "transform")

				// Extract transform names. Curate input (EG. NONE+NONE+xxxx => xxxx)
				name, err := transform.GetType(strTransf)

				if err != nil {
					return nil, err
				}

				if strTransf, err = transform.GetName(name); err != nil {
					return nil, err
				}

				this.transform = strTransf
			} else {
				this.transform = "NONE"
			}
		}
	}

	if force, prst := argsMap["overwrite"]; prst == true {
		this.overwrite = force.(bool)
		delete(argsMap, "overwrite")
	} else {
		this.overwrite = false
	}

	if skip, prst := argsMap["skipBlocks"]; prst == true {
		this.skipBlocks = skip.(bool)
		delete(argsMap, "skipBlocks")
	} else {
		this.skipBlocks = false
	}

	if skip, prst := argsMap["autoBlock"]; prst == true {
		this.autoBlockSize = skip.(bool)
		delete(argsMap, "autoBlock")
	} else {
		this.autoBlockSize = false
	}

	this.inputName = argsMap["inputName"].(string)
	delete(argsMap, "inputName")

	if internal.IsReservedName(this.inputName) {
		return nil, fmt.Errorf("'%s' is a reserved name", this.inputName)
	}

	if len(this.inputName) == 0 {
		this.inputName = _COMP_STDIN
	}

	this.outputName = argsMap["outputName"].(string)
	delete(argsMap, "outputName")

	if internal.IsReservedName(this.outputName) {
		return nil, fmt.Errorf("'%s' is a reserved name", this.outputName)
	}

	if len(this.outputName) == 0 && this.inputName == _COMP_STDIN {
		this.outputName = _COMP_STDOUT
	}

	if block, prst := argsMap["blockSize"]; prst == true {
		szBlk := block.(uint)
		this.blockSize = ((szBlk + 15) >> 4) << 4
		delete(argsMap, "blockSize")

		if this.blockSize < _COMP_MIN_BLOCK_SIZE {
			return nil, fmt.Errorf("Minimum block size is %d KiB (%d bytes), got %d bytes", _COMP_MIN_BLOCK_SIZE/1024, _COMP_MIN_BLOCK_SIZE, this.blockSize)
		}

		if this.blockSize > _COMP_MAX_BLOCK_SIZE {
			return nil, fmt.Errorf("Maximum block size is %d GiB (%d bytes), got %d bytes", _COMP_MAX_BLOCK_SIZE/(1024*1024*1024), _COMP_MAX_BLOCK_SIZE, this.blockSize)
		}
	} else {
		switch level {
		case 6:
			this.blockSize = 2 * _COMP_DEFAULT_BLOCK_SIZE
		case 7:
			this.blockSize = 4 * _COMP_DEFAULT_BLOCK_SIZE
		case 8:
			this.blockSize = 4 * _COMP_DEFAULT_BLOCK_SIZE
		case 9:
			this.blockSize = 8 * _COMP_DEFAULT_BLOCK_SIZE
		default:
			this.blockSize = _COMP_DEFAULT_BLOCK_SIZE
		}
	}

	if check, prst := argsMap["checksum"]; prst == true {
		this.checksum = check.(uint)
		delete(argsMap, "checksum")
	} else {
		this.checksum = 0
	}

	if rmSrc, prst := argsMap["remove"]; prst == true {
		this.removeSource = rmSrc.(bool)
		delete(argsMap, "remove")
	} else {
		this.removeSource = false
	}

	if check, prst := argsMap["fileReorder"]; prst == true {
		this.fileReorder = check.(bool)
		delete(argsMap, "fileReorder")
	} else {
		this.fileReorder = true
	}

	if check, prst := argsMap["noDotFiles"]; prst == true {
		this.noDotFiles = check.(bool)
		delete(argsMap, "noDotFiles")
	} else {
		this.noDotFiles = false
	}

	if check, prst := argsMap["noLinks"]; prst == true {
		this.noLinks = check.(bool)
		delete(argsMap, "noLinks")
	} else {
		this.noLinks = false
	}

	this.verbosity = argsMap["verbosity"].(uint)
	delete(argsMap, "verbosity")
	concurrency := uint(1)

	if c, prst := argsMap["jobs"].(uint); prst == true {
		delete(argsMap, "jobs")
		concurrency = c

		if c == 0 {
			concurrency = uint(runtime.NumCPU()) // use all cores
		} else if c > _COMP_MAX_CONCURRENCY {
			msg := fmt.Sprintf("Warning: the number of jobs is too high, defaulting to %d\n", _COMP_MAX_CONCURRENCY)
			log.Println(msg, this.verbosity > 0)
			concurrency = _COMP_MAX_CONCURRENCY
		}
	} else if runtime.NumCPU() > 1 {
		concurrency = uint(runtime.NumCPU() / 2) // defaults to half the cores
	}

	this.jobs = min(concurrency, _COMP_MAX_CONCURRENCY)

	if prof, prst := argsMap["cpuProf"]; prst == true {
		this.cpuProf = prof.(string)
		delete(argsMap, "cpuProf")
	} else {
		this.cpuProf = ""
	}

	if this.verbosity > 3 && len(argsMap) > 0 {
		for k := range argsMap {
			log.Println("Warning: ignoring invalid option ["+k+"]", this.verbosity > 0)
		}
	}

	return this, nil
}

// AddListener adds an event listener to this compressor.
// Returns true if the listener has been added.
func (this *BlockCompressor) AddListener(bl kanzi.Listener) bool {
	if bl == nil {
		return false
	}

	this.listeners = append(this.listeners, bl)
	return true
}

// RemoveListener removes an event listener from this compressor.
// Returns true if the listener has been removed.
func (this *BlockCompressor) RemoveListener(bl kanzi.Listener) bool {
	for i, e := range this.listeners {
		if e == bl {
			this.listeners = append(this.listeners[:i-1], this.listeners[i+1:]...)
			return true
		}
	}

	return false
}

// CPUProf returns the name of the CPU profile data file (maybe be empty)
func (this *BlockCompressor) CPUProf() string {
	return this.cpuProf
}

func fileCompressWorker(tasks <-chan fileCompressTask, cancel <-chan bool, results chan<- fileCompressResult) {
	// Pull tasks from channel and run them
	more := true

	for more {
		select {
		case t, m := <-tasks:
			more = m

			if more {
				res, read, written, err := t.call()
				results <- fileCompressResult{code: res, read: read, written: written, err: err}
				more = res == 0
			}

		case c := <-cancel:
			more = !c
		}
	}
}

// Compress is the main function to compress the files or files based on the
// input name provided at construction. Files may be processed concurrently
// depending on the number of jobs provided at construction.
// Returns exit code, number of bits written.
func (this *BlockCompressor) Compress() (int, uint64) {
	var err error
	before := time.Now()
	files := make([]internal.FileData, 0, 256)
	nbFiles := 1
	var msg string
	isStdIn := strings.EqualFold(this.inputName, _COMP_STDIN)

	if isStdIn == false {
		suffix := string([]byte{os.PathSeparator, '.'})
		target := this.inputName
		isRecursive := len(target) <= 2 || target[len(target)-len(suffix):] != suffix

		if isRecursive == false {
			target = target[0 : len(target)-1]
		}

		files, err = internal.CreateFileList(target, files, isRecursive, this.noLinks, this.noDotFiles)

		if err != nil {
			if ioerr, isIOErr := err.(kio.IOError); isIOErr == true {
				fmt.Printf("%s\n", ioerr.Error())
				return ioerr.ErrorCode(), 0
			}

			fmt.Printf("An unexpected condition happened. Exiting ...\n%s\n", err.Error())
			return kanzi.ERR_OPEN_FILE, 0
		}

		if len(files) == 0 {
			fmt.Println("Cannot find any file to compress")
			return kanzi.ERR_OPEN_FILE, 0
		}

		nbFiles = len(files)

		if nbFiles > 1 {
			msg = fmt.Sprintf("%d files to compress\n", nbFiles)
		} else {
			msg = fmt.Sprintf("%d file to compress\n", nbFiles)
		}

		log.Println(msg, this.verbosity > 0)
	}

	isStdOut := strings.EqualFold(this.outputName, _COMP_STDOUT)

	// Limit verbosity level when output is stdout
	// Logic is duplicated here to avoid dependency to Kanzi.go
	if isStdOut == true {
		this.verbosity = 0
	}

	// Limit verbosity level when files are processed concurrently
	if this.jobs > 1 && nbFiles > 1 && this.verbosity > 1 {
		log.Println("Warning: limiting verbosity to 1 due to concurrent processing of input files.\n", true)
		this.verbosity = 1
	}

	if this.verbosity > 2 {
		if this.autoBlockSize == true {
			msg = "Block size: 'auto'"
		} else {
			msg = fmt.Sprintf("Block size: %d bytes", this.blockSize)
		}

		log.Println(msg, true)
		msg = fmt.Sprintf("Verbosity: %d", this.verbosity)
		log.Println(msg, true)
		msg = fmt.Sprintf("Overwrite: %t", this.overwrite)
		log.Println(msg, true)
		chksum := "NONE"

		if this.checksum == 32 {
			chksum = "32 bits"
		} else if this.checksum == 64 {
			chksum = "64 bits"
		}

		msg = fmt.Sprintf("Block checksum: %s", chksum)
		log.Println(msg, true)
		w1 := "no"

		if this.transform != _COMP_NONE {
			w1 = this.transform
		}

		msg = fmt.Sprintf("Using %s transform (stage 1)", w1)
		log.Println(msg, true)
		w2 := "no"

		if this.entropyCodec != _COMP_NONE {
			w2 = this.entropyCodec
		}

		msg = fmt.Sprintf("Using %s entropy codec (stage 2)", w2)
		log.Println(msg, true)

		if this.jobs > 1 {
			msg = fmt.Sprintf("Using %d jobs", this.jobs)
			log.Println(msg, true)
		} else {
			log.Println("Using 1 job", true)
		}
	}

	if this.verbosity > 2 {
		if listener, err2 := NewInfoPrinter(this.verbosity, ENCODING, os.Stdout); err2 == nil {
			this.AddListener(listener)
		}
	}

	read := uint64(0)
	written := uint64(0)
	inputIsDir := false
	formattedOutName := this.outputName
	formattedInName := this.inputName
	specialOutput := strings.EqualFold(this.outputName, _COMP_NONE) || strings.EqualFold(this.outputName, _COMP_STDOUT)

	if isStdIn == false {
		fi, err := os.Stat(formattedInName)

		if err != nil {
			fmt.Println("Cannot find any file to compress")
			return kanzi.ERR_OPEN_FILE, 0
		}

		if fi.IsDir() {
			inputIsDir = true

			if len(formattedInName) > 1 && formattedInName[len(formattedInName)-1] == '.' {
				formattedInName = formattedInName[0 : len(formattedInName)-1]
			}

			if len(formattedInName) > 0 && formattedInName[len(formattedInName)-1] != os.PathSeparator {
				formattedInName += string([]byte{os.PathSeparator})
			}

			if len(formattedOutName) > 0 && specialOutput == false {
				fi, err = os.Stat(formattedOutName)

				if err != nil {
					fmt.Println("Output must be an existing directory (or 'NONE')")
					return kanzi.ERR_OPEN_FILE, 0
				}

				if !fi.IsDir() {
					fmt.Println("Output must be a directory (or 'NONE')")
					return kanzi.ERR_CREATE_FILE, 0
				}

				if formattedOutName[len(formattedOutName)-1] != os.PathSeparator {
					formattedOutName += string([]byte{os.PathSeparator})
				}
			}
		} else {
			if len(formattedOutName) > 0 && specialOutput == false {
				fi, err = os.Stat(formattedOutName)

				if err == nil && fi.IsDir() {
					fmt.Println("Output must be a file (or 'NONE')")
					return kanzi.ERR_CREATE_FILE, 0
				}
			}
		}
	}

	ctx := make(map[string]any)
	ctx["verbosity"] = this.verbosity
	ctx["remove"] = this.removeSource
	ctx["overwrite"] = this.overwrite
	ctx["skipBlocks"] = this.skipBlocks
	ctx["checksum"] = this.checksum
	ctx["entropy"] = this.entropyCodec
	ctx["transform"] = this.transform
	var res int

	if nbFiles == 1 {
		oName := formattedOutName
		iName := _COMP_STDIN

		if isStdIn == true {
			if len(oName) == 0 {
				oName = _COMP_STDOUT
			}
		} else {
			iName = files[0].FullPath
			ctx["fileSize"] = files[0].Size

			if this.autoBlockSize == true && this.jobs > 0 {
				bl := files[0].Size / int64(this.jobs)
				bl = (bl + 63) & ^63

				if bl > _COMP_MAX_BLOCK_SIZE {
					bl = _COMP_MAX_BLOCK_SIZE
				} else if bl < _COMP_MIN_BLOCK_SIZE {
					bl = _COMP_MIN_BLOCK_SIZE
				}

				this.blockSize = uint(bl)
			}

			if len(oName) == 0 {
				oName = iName + ".knz"
			} else if inputIsDir == true && specialOutput == false {
				oName = formattedOutName + iName[len(formattedInName):] + ".knz"
			}
		}

		ctx["inputName"] = iName
		ctx["outputName"] = oName
		ctx["blockSize"] = this.blockSize
		ctx["jobs"] = this.jobs
		task := fileCompressTask{ctx: ctx, listeners: this.listeners}
		res, read, written, err = task.call()
	} else {
		// Create channels for task synchronization
		tasks := make(chan fileCompressTask, nbFiles)
		results := make(chan fileCompressResult, nbFiles)
		cancel := make(chan bool, 1)

		// When nbFiles > 1, this.jobs are distributed among tasks by ComputeJobsPerTask.
		// Each task then receives a portion of these jobs for its internal parallel block processing.
		jobsPerTask, _ := internal.ComputeJobsPerTask(make([]uint, nbFiles), this.jobs, uint(nbFiles))

		if this.fileReorder == true {
			sort.Sort(internal.NewFileCompare(files, true))
		}

		// Create one task per file
		for i, f := range files {
			iName := f.FullPath
			oName := formattedOutName

			if len(oName) == 0 {
				oName = iName + ".knz"
			} else if inputIsDir == true && specialOutput == false {
				oName = formattedOutName + iName[len(formattedInName):] + ".knz"
			}

			taskCtx := make(map[string]any)

			for k, v := range ctx {
				taskCtx[k] = v
			}

			if this.autoBlockSize == true && this.jobs > 0 {
				bl := f.Size / int64(this.jobs)
				bl = (bl + 63) & ^63
				bl = min(bl, _COMP_MAX_BLOCK_SIZE)
				bl = max(bl, _COMP_MIN_BLOCK_SIZE)
				this.blockSize = uint(bl)
			}

			taskCtx["fileSize"] = f.Size
			taskCtx["inputName"] = iName
			taskCtx["outputName"] = oName
			taskCtx["blockSize"] = this.blockSize
			taskCtx["jobs"] = jobsPerTask[i]
			task := fileCompressTask{ctx: taskCtx, listeners: this.listeners}

			// Push task to channel. The workers are the consumers.
			tasks <- task
		}

		close(tasks)

		// Create one worker per job. A worker calls several tasks sequentially.
		for j := uint(0); j < this.jobs; j++ {
			go fileCompressWorker(tasks, cancel, results)
		}

		res = 0

		// Wait for all task results
		for i := 0; i < nbFiles; i++ {
			result := <-results
			read += result.read
			written += result.written

			if result.code != 0 {
				// Exit early
				res = result.code
				break
			}
		}

		cancel <- true
		close(cancel)
		close(results)
	}

	after := time.Now()

	if nbFiles > 1 {
		delta := after.Sub(before).Nanoseconds() / 1000000 // convert to ms
		log.Println("", this.verbosity > 0)

		if delta >= 100000 {
			msg = fmt.Sprintf("%.1f s", float64(delta)/1000)
		} else {
			msg = fmt.Sprintf("%.0f ms", float64(delta))
		}

		msg = fmt.Sprintf("Total compression time: %s", msg)
		log.Println(msg, this.verbosity > 0)

		if written > 1 {
			msg = fmt.Sprintf("Total output size: %d bytes", written)
		} else {
			msg = fmt.Sprintf("Total output size: %d byte", written)
		}

		log.Println(msg, this.verbosity > 0)

		if read > 0 {
			msg = fmt.Sprintf("Compression ratio: %f", float64(written)/float64(read))
			log.Println(msg, this.verbosity > 0)
		}
	}

	return res, written
}

func notifyBCListeners(listeners []kanzi.Listener, evt *kanzi.Event) {
	defer func() {
		//lint:ignore SA9003 Ignore panics in listeners
		// nolint:staticcheck
		if r := recover(); r != nil {
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
		return "PACK+LZ&NONE"

	case 2:
		return "DNA+LZ&HUFFMAN"

	case 3:
		return "TEXT+UTF+PACK+MM+LZX&HUFFMAN"

	case 4:
		return "TEXT+UTF+EXE+PACK+MM+ROLZ&NONE"

	case 5:
		return "TEXT+UTF+BWT+RANK+ZRLT&ANS0"

	case 6:
		return "TEXT+UTF+BWT+SRT+ZRLT&FPAQ"

	case 7:
		return "LZP+TEXT+UTF+BWT+LZP&CM"

	case 8:
		return "EXE+RLT+TEXT+UTF+DNA&TPAQ"

	case 9:
		return "EXE+RLT+TEXT+UTF+DNA&TPAQX"

	default:
		return "Unknown&Unknown"
	}
}

type fileCompressTask struct {
	ctx       map[string]any
	listeners []kanzi.Listener
}

func (this *fileCompressTask) call() (int, uint64, uint64, error) {
	var msg string
	removeSource := this.ctx["remove"].(bool)
	verbosity := this.ctx["verbosity"].(uint)
	inputName := this.ctx["inputName"].(string)
	outputName := this.ctx["outputName"].(string)

	if verbosity > 2 {
		log.Println("Input file name: '"+inputName+"'", true)
		log.Println("Output file name: '"+outputName+"'", true)
	}

	overwrite := this.ctx["overwrite"].(bool)
	var output io.WriteCloser

	if strings.EqualFold(outputName, _COMP_NONE) == true {
		output, _ = kio.NewNullOutputStream()
	} else if strings.EqualFold(outputName, _COMP_STDOUT) == true {
		output = os.Stdout
	} else {
		var err error

		if output, err = os.OpenFile(outputName, os.O_RDWR, 0666); err == nil {
			// File exists
			if err = output.Close(); err != nil {
				fmt.Printf("Cannot create output file '%s': error closing existing file\n", outputName)
				return kanzi.ERR_OVERWRITE_FILE, 0, 0, err
			}

			if overwrite == false {
				fmt.Printf("File '%s' exists and the 'force' command ", outputName)
				fmt.Println("line option has not been provided")
				return kanzi.ERR_OVERWRITE_FILE, 0, 0, err
			}

			path1, _ := filepath.Abs(inputName)
			path2, _ := filepath.Abs(outputName)

			if path1 == path2 {
				fmt.Print("The input and output files must be different")
				return kanzi.ERR_CREATE_FILE, 0, 0, err
			}
		}

		output, err = os.Create(outputName)

		if err != nil {
			if overwrite {
				// Attempt to create the full folder hierarchy to file
				if err = os.MkdirAll(path.Dir(strings.ReplaceAll(outputName, "\\", "/")), os.ModePerm); err == nil {
					output, err = os.Create(outputName)
				}
			}

			if err != nil {
				fmt.Printf("Cannot open output file '%s' for writing: %v\n", outputName, err)
				return kanzi.ERR_CREATE_FILE, 0, 0, err
			}
		}

		defer output.Close()
	}

	cos, err := kio.NewWriterWithCtx(output, this.ctx)

	if err != nil {
		if ioerr, isIOErr := err.(kio.IOError); isIOErr == true {
			fmt.Printf("%s\n", ioerr.Error())
			return ioerr.ErrorCode(), 0, 0, err
		}

		fmt.Printf("Cannot create compressed stream: %s\n", err.Error())
		return kanzi.ERR_CREATE_COMPRESSOR, 0, 0, err
	}

	defer cos.Close()

	var input io.ReadCloser

	if strings.EqualFold(inputName, _COMP_STDIN) {
		input = os.Stdin
	} else {
		var err error

		if input, err = os.Open(inputName); err != nil {
			fmt.Printf("Cannot open input file '%s': %v\n", inputName, err)
			return kanzi.ERR_OPEN_FILE, 0, 0, err
		}

		defer input.Close()
	}

	for _, bl := range this.listeners {
		cos.AddListener(bl)
	}

	// Encode
	log.Println("\nCompressing "+inputName+" ...", verbosity > 1)
	log.Println("", verbosity > 3)
	read := uint64(0)
	buffer := make([]byte, _COMP_DEFAULT_BUFFER_SIZE)

	if len(this.listeners) > 0 {
		inputSize := int64(0)

		if val, hasKey := this.ctx["fileSize"]; hasKey {
			inputSize = val.(int64)
		}

		evt := kanzi.NewEvent(kanzi.EVT_COMPRESSION_START, -1, inputSize, 0, kanzi.EVT_HASH_NONE, time.Now())
		notifyBCListeners(this.listeners, evt)
	}

	before := time.Now()

	for {
		var length int

		if length, err = input.Read(buffer); err != nil {
			if errors.Is(err, io.EOF) == false {
				// Ignore EOF (see comment in io.Copy:
				// Because Copy is defined to read from src until EOF, it does not
				// treat EOF from Read an an error to be reported)
				fmt.Printf("Failed to read block from file '%s': %v\n", inputName, err)
				return kanzi.ERR_READ_FILE, read, cos.GetWritten(), err
			}
		}

		if length > 0 {
			read += uint64(length)

			if _, err = cos.Write(buffer[0:length]); err != nil {
				if ioerr, isIOErr := err.(kio.IOError); isIOErr == true {
					fmt.Printf("%s\n", ioerr.Error())
					return ioerr.ErrorCode(), read, cos.GetWritten(), err
				}

				fmt.Printf("An unexpected condition happened. Exiting ...\n%v\n", err.Error())
				return kanzi.ERR_PROCESS_BLOCK, read, cos.GetWritten(), err
			}
		}

		if length < len(buffer) {
			break
		}
	}

	// Close streams to ensure all data are flushed
	// Deferred close is fallback for error paths
	if err := cos.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return kanzi.ERR_PROCESS_BLOCK, read, cos.GetWritten(), err
	}

	after := time.Now()
	delta := after.Sub(before).Nanoseconds() / 1000000 // convert to ms

	if verbosity >= 1 {
		log.Println("", verbosity > 1)

		if delta >= 100000 {
			msg = fmt.Sprintf("%.1f s", float64(delta)/1000)
		} else {
			msg = fmt.Sprintf("%.0f ms", float64(delta))
		}

		if verbosity > 1 {
			msg = fmt.Sprintf("Compression time:  %s", msg)
			log.Println(msg, true)
			msg = fmt.Sprintf("Input size:        %d", read)
			log.Println(msg, true)
			msg = fmt.Sprintf("Output size:       %d", cos.GetWritten())
			log.Println(msg, true)

			if read != 0 {
				msg = fmt.Sprintf("Compression ratio: %f", float64(cos.GetWritten())/float64(read))
				log.Println(msg, true)
			}
		} else if verbosity == 1 {
			if read == 0 {
				msg = fmt.Sprintf("Compressed %s: %d => %d in %s", inputName, read, cos.GetWritten(), msg)
			} else {
				f := float64(cos.GetWritten()) / float64(read)
				msg = fmt.Sprintf("Compressed %s: %d => %d (%.2f%%) in %s", inputName, read, cos.GetWritten(), 100*f, msg)
			}

			log.Println(msg, true)
		}

		if verbosity > 1 && delta != 0 && read != 0 {
			msg = fmt.Sprintf("Throughput (KiB/s): %d", ((int64(read*1000))>>10)/delta)
			log.Println(msg, true)
		}

		log.Println("", verbosity > 1)
	}

	if len(this.listeners) > 0 {
		evt := kanzi.NewEvent(kanzi.EVT_COMPRESSION_END, -1, int64(cos.GetWritten()), 0, kanzi.EVT_HASH_NONE, time.Now())
		notifyBCListeners(this.listeners, evt)
	}

	if removeSource == true {
		// Close input prior to deletion
		// Close will return an error if it has already been called.
		// The deferred call does not check for error.
		if err := input.Close(); err != nil {
			msg := fmt.Sprintf("Warning: %v\n", err)
			log.Println(msg, verbosity > 0)
		}

		// Delete input file
		if inputName == "STDIN" {
			log.Println("Warning: ignoring remove option with STDIN", verbosity > 0)
		} else if os.Remove(inputName) != nil {
			log.Println("Warning: input file could not be deleted", verbosity > 0)
		}
	}

	return 0, read, cos.GetWritten(), nil
}
