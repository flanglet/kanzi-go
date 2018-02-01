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
	kanzi "github.com/flanglet/kanzi"
	kio "github.com/flanglet/kanzi/io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	COMP_DEFAULT_BUFFER_SIZE = 32768
	COMP_DEFAULT_BLOCK_SIZE  = 1024 * 1024
	COMP_DEFAULT_CONCURRENCY = 1
	COMP_MAX_CONCURRENCY     = 32
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

type FileCompressResult struct {
	code    int
	read    uint64
	written uint64
}

func NewBlockCompressor(argsMap map[string]interface{}) (*BlockCompressor, error) {
	this := new(BlockCompressor)
	this.listeners = make([]kanzi.Listener, 0)
	this.level = argsMap["level"].(int)
	delete(argsMap, "level")

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
			strCodec = "ANS0"
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
			strTransf = "BWT+RANK+ZRLT"
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

	concurrency := argsMap["jobs"].(uint)
	delete(argsMap, "jobs")

	if concurrency == 0 {
		this.jobs = COMP_DEFAULT_CONCURRENCY
	} else {
		if concurrency > COMP_MAX_CONCURRENCY {
			fmt.Printf("Warning: the number of jobs is too high, defaulting to %v\n", COMP_MAX_CONCURRENCY)
			concurrency = COMP_MAX_CONCURRENCY
		}

		this.jobs = concurrency
	}

	if prof, prst := argsMap["cpuProf"]; prst == true {
		this.cpuProf = prof.(string)
		delete(argsMap, "cpuProf")
	} else {
		this.cpuProf = ""
	}

	this.verbosity = argsMap["verbose"].(uint)
	delete(argsMap, "verbose")

	if this.verbosity > 0 && len(argsMap) > 0 {
		for k, _ := range argsMap {
			log.Println("Ignoring invalid option ["+k+"]", this.verbosity > 0)
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

func fileCompressWorker(tasks <-chan FileCompressTask, cancel <-chan bool, results chan<- FileCompressResult) {
	// Pull tasks from channel and run them
	more := true

	for more {
		select {
		case t, m := <-tasks:
			more = m

			if more {
				res, read, written := t.Call()
				results <- FileCompressResult{code: res, read: read, written: written}
				more = res == 0
			}

		case c := <-cancel:
			more = !c
		}
	}
}

// Return exit code, number of bits written
func (this *BlockCompressor) Call() (int, uint64) {
	var err error
	before := time.Now()
	files := make([]FileData, 0, 256)
	files, err = createFileList(this.inputName, files)

	if err != nil {
		if ioerr, isIOErr := err.(kio.IOError); isIOErr == true {
			fmt.Printf("%s\n", ioerr.Error())
			return ioerr.ErrorCode(), 0
		}

		fmt.Printf("An unexpected condition happened. Exiting ...\n%v\n", err.Error())
		return kanzi.ERR_OPEN_FILE, 0
	}

	if len(files) == 0 {
		fmt.Printf("Cannot open input file '%v'\n", this.inputName)
		return kanzi.ERR_OPEN_FILE, 0
	}

	nbFiles := len(files)
	printFlag := this.verbosity > 2
	var msg string

	if nbFiles > 1 {
		msg = fmt.Sprintf("%d files to compress\n", nbFiles)
	} else {
		msg = fmt.Sprintf("%d file to compress\n", nbFiles)
	}

	log.Println(msg, this.verbosity > 0)
	msg = fmt.Sprintf("Block size set to %d bytes", this.blockSize)
	log.Println(msg, printFlag)
	msg = fmt.Sprintf("Verbosity set to %v", this.verbosity)
	log.Println(msg, printFlag)
	msg = fmt.Sprintf("Overwrite set to %t", this.overwrite)
	log.Println(msg, printFlag)
	msg = fmt.Sprintf("Checksum set to %t", this.checksum)
	log.Println(msg, printFlag)

	if this.level < 0 {
		w1 := "no"

		if this.transform != "NONE" {
			w1 = this.transform
		}

		msg = fmt.Sprintf("Using %s transform (stage 1)", w1)
		log.Println(msg, printFlag)
		w2 := "no"

		if this.entropyCodec != "NONE" {
			w2 = this.entropyCodec
		}

		msg = fmt.Sprintf("Using %s entropy codec (stage 2)", w2)
	} else {
		msg = fmt.Sprintf("Compression level set to %v", this.level)
	}

	log.Println(msg, printFlag)

	if this.jobs > 1 {
		msg = fmt.Sprintf("Using %d jobs", this.jobs)
		log.Println(msg, printFlag)

		if strings.ToUpper(this.outputName) == "STDOUT" {
			fmt.Println("Cannot output to STDOUT with multiple jobs")
			return kanzi.ERR_CREATE_FILE, 0
		}
	} else {
		log.Println("Using 1 job", printFlag)
	}

	// Limit verbosity level when files are processed concurrently
	if this.jobs > 1 && nbFiles > 1 && this.verbosity > 1 {
		log.Println("Warning: limiting verbosity to 1 due to concurrent processing of input files.\n", true)
		this.verbosity = 1
	}

	if this.verbosity > 2 {
		if listener, err := NewInfoPrinter(this.verbosity, ENCODING, os.Stdout); err == nil {
			this.AddListener(listener)
		}
	}

	res := 1
	read := uint64(0)
	written := uint64(0)
	var inputIsDir bool
	formattedOutName := this.outputName
	formattedInName := this.inputName
	specialOutput := strings.ToUpper(formattedOutName) == "NONE" || strings.ToUpper(formattedOutName) == "STDOUT"

	fi, err := os.Stat(this.inputName)

	if err != nil {
		fmt.Printf("Cannot access %v\n", formattedInName)
		return kanzi.ERR_OPEN_FILE, 0
	}

	if fi.IsDir() {
		inputIsDir = true

		if formattedInName[len(formattedInName)-1] == '.' {
			formattedInName = formattedInName[0 : len(formattedInName)-1]
		}

		if formattedInName[len(formattedInName)-1] != os.PathSeparator {
			formattedInName = formattedInName + string([]byte{os.PathSeparator})
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
				formattedOutName = formattedOutName + string([]byte{os.PathSeparator})
			}
		}
	} else {
		inputIsDir = false

		if len(formattedOutName) > 0 && specialOutput == false {
			fi, err = os.Stat(formattedOutName)

			if err == nil && fi.IsDir() {
				fmt.Println("Output must be a file (or 'NONE')")
				return kanzi.ERR_CREATE_FILE, 0
			}
		}
	}

	ctx := make(map[string]interface{})
	ctx["verbosity"] = this.verbosity
	ctx["overwrite"] = this.overwrite
	ctx["blockSize"] = this.blockSize
	ctx["checksum"] = this.checksum
	ctx["codec"] = this.entropyCodec
	ctx["transform"] = this.transform

	if nbFiles == 1 {
		oName := formattedOutName
		iName := files[0].Path

		if len(oName) == 0 {
			oName = iName + ".knz"
		} else if inputIsDir == true && specialOutput == false {
			oName = formattedOutName + iName[len(formattedInName):] + ".knz"
		}

		ctx["fileSize"] = files[0].Size
		ctx["inputName"] = iName
		ctx["outputName"] = oName
		ctx["jobs"] = this.jobs
		task := FileCompressTask{ctx: ctx, listeners: this.listeners}
		res, read, written = task.Call()
	} else {
		// Create channels for task synchronization
		tasks := make(chan FileCompressTask, nbFiles)
		results := make(chan FileCompressResult, nbFiles)
		cancel := make(chan bool, 1)

		jobsPerTask := kanzi.ComputeJobsPerTask(make([]uint, nbFiles), this.jobs, uint(nbFiles))
		n := 0
		sort.Sort(FileCompareByName{data: files})

		// Create one task per file
		for _, f := range files {
			iName := f.Path
			oName := formattedOutName

			if len(oName) == 0 {
				oName = iName + ".knz"
			} else if inputIsDir == true && specialOutput == false {
				oName = formattedOutName + iName[len(formattedInName):] + ".knz"
			}

			taskCtx := make(map[string]interface{})

			for k, v := range ctx {
				taskCtx[k] = v
			}

			taskCtx["fileSize"] = f.Size
			taskCtx["inputName"] = iName
			taskCtx["outputName"] = oName
			taskCtx["jobs"] = jobsPerTask[n]
			n++
			task := FileCompressTask{ctx: taskCtx, listeners: this.listeners}

			// Push task to channel. The workers are the consumers.
			tasks <- task
		}

		close(tasks)

		// Create one worker per job. A worker calls several tasks sequentially.
		for j := uint(0); j < this.jobs; j++ {
			go fileCompressWorker(tasks, cancel, results)
		}

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
		msg = fmt.Sprintf("Total encoding time: %d ms", delta)
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
		return "X86+RLT+TEXT&TPAQ"

	default:
		return "Unknown&Unknown"
	}
}

type FileCompressTask struct {
	ctx       map[string]interface{}
	listeners []kanzi.Listener
	cpuProf   string
}

func (this *FileCompressTask) Call() (int, uint64, uint64) {
	var msg string
	verbosity := this.ctx["verbosity"].(uint)
	inputName := this.ctx["inputName"].(string)
	outputName := this.ctx["outputName"].(string)
	printFlag := verbosity > 2
	log.Println("Input file name set to '"+inputName+"'", printFlag)
	log.Println("Output file name set to '"+outputName+"'", printFlag)
	overwrite := this.ctx["overwrite"].(bool)

	var output io.WriteCloser

	if strings.ToUpper(outputName) == "NONE" {
		output, _ = kio.NewNullOutputStream()
	} else if strings.ToUpper(outputName) == "STDOUT" {
		output = os.Stdout
	} else {
		var err error

		if output, err = os.OpenFile(outputName, os.O_RDWR, 0666); err == nil {
			// File exists
			output.Close()

			if overwrite == false {
				fmt.Printf("File '%v' exists and the 'force' command ", outputName)
				fmt.Println("line option has not been provided")
				return kanzi.ERR_OVERWRITE_FILE, 0, 0
			}

			path1, _ := filepath.Abs(inputName)
			path2, _ := filepath.Abs(outputName)

			if path1 == path2 {
				fmt.Print("The input and output files must be different")
				return kanzi.ERR_CREATE_FILE, 0, 0
			}
		}

		output, err = os.Create(outputName)

		if err != nil {
			if overwrite {
				// Attempt to create the full folder hierarchy to file
				if err = os.MkdirAll(path.Dir(strings.Replace(outputName, "\\", "/", -1)), os.ModePerm); err == nil {
					output, err = os.Create(outputName)
				}
			}

			if err != nil {
				fmt.Printf("Cannot open output file '%v' for writing: %v\n", outputName, err)
				return kanzi.ERR_CREATE_FILE, 0, 0
			}
		}

		defer func() {
			output.Close()
		}()

	}

	cos, err := kio.NewCompressedOutputStream(output, this.ctx)

	if err != nil {
		if ioerr, isIOErr := err.(kio.IOError); isIOErr == true {
			fmt.Printf("%s\n", ioerr.Error())
			return ioerr.ErrorCode(), 0, 0
		}

		fmt.Printf("Cannot create compressed stream: %s\n", err.Error())
		return kanzi.ERR_CREATE_COMPRESSOR, 0, 0
	}

	defer func() {
		cos.Close()
	}()

	var input io.ReadCloser

	if strings.ToUpper(inputName) == "STDIN" {
		input = os.Stdin
	} else {
		var err error

		if input, err = os.Open(inputName); err != nil {
			fmt.Printf("Cannot open input file '%v': %v\n", inputName, err)
			return kanzi.ERR_OPEN_FILE, 0, 0
		}

		defer func() {
			input.Close()
		}()
	}

	for _, bl := range this.listeners {
		cos.AddListener(bl)
	}

	// Encode
	printFlag = verbosity > 1
	log.Println("\nEncoding "+inputName+" ...", printFlag)
	log.Println("", verbosity > 3)
	length := 0
	read := uint64(0)

	buffer := make([]byte, COMP_DEFAULT_BUFFER_SIZE)

	if len(this.listeners) > 0 {
		evt := kanzi.NewEvent(kanzi.EVT_COMPRESSION_START, -1, 0, 0, false, time.Now())
		bc_notifyListeners(this.listeners, evt)
	}

	before := time.Now()
	length, err = input.Read(buffer)

	for length > 0 {
		if err != nil {
			fmt.Printf("Failed to read block from file '%v': %v\n", inputName, err)
			return kanzi.ERR_READ_FILE, read, cos.GetWritten()
		}

		read += uint64(length)

		if _, err = cos.Write(buffer[0:length]); err != nil {
			if ioerr, isIOErr := err.(kio.IOError); isIOErr == true {
				fmt.Printf("%s\n", ioerr.Error())
				return ioerr.ErrorCode(), read, cos.GetWritten()
			}

			fmt.Printf("An unexpected condition happened. Exiting ...\n%v\n", err.Error())
			return kanzi.ERR_PROCESS_BLOCK, read, cos.GetWritten()
		}

		length, err = input.Read(buffer)
	}

	if read == 0 {
		msg = fmt.Sprintf("Input file %v is empty ... nothing to do", inputName)
		log.Println(msg, verbosity > 0)
		return 0, read, cos.GetWritten()
	}

	// Close streams to ensure all data are flushed
	// Deferred close is fallback for error paths
	if err := cos.Close(); err != nil {
		fmt.Printf("%v\n", err)
		return kanzi.ERR_PROCESS_BLOCK, read, cos.GetWritten()
	}

	after := time.Now()
	delta := after.Sub(before).Nanoseconds() / 1000000 // convert to ms

	log.Println("", verbosity > 1)
	msg = fmt.Sprintf("Encoding:          %d ms", delta)
	log.Println(msg, printFlag)
	msg = fmt.Sprintf("Input size:        %d", read)
	log.Println(msg, printFlag)
	msg = fmt.Sprintf("Output size:       %d", cos.GetWritten())
	log.Println(msg, printFlag)
	msg = fmt.Sprintf("Compression ratio: %f", float64(cos.GetWritten())/float64(read))
	log.Println(msg, printFlag)
	msg = fmt.Sprintf("Encoding %v: %v => %v bytes in %v ms", inputName, read, cos.GetWritten(), delta)
	log.Println(msg, verbosity == 1)

	if delta > 0 {
		msg = fmt.Sprintf("Throughput (KB/s): %d", ((int64(read*1000))>>10)/delta)
		log.Println(msg, printFlag)
	}

	log.Println("", verbosity > 1)

	if len(this.listeners) > 0 {
		evt := kanzi.NewEvent(kanzi.EVT_COMPRESSION_END, -1, int64(cos.GetWritten()), 0, false, time.Now())
		bc_notifyListeners(this.listeners, evt)
	}

	return 0, read, cos.GetWritten()
}
