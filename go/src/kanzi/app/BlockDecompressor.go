/*
Copyright 2011-2013 Frederic Langlet
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
	"container/list"
	"flag"
	"fmt"
	"kanzi/io"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	DECOMP_DEFAULT_BUFFER_SIZE = 32768
)

type BlockDecompressor struct {
	verbose    bool
	silent     bool
	overwrite  bool
	inputName  string
	outputName string
	jobs       uint
	listeners  *list.List
}

func NewBlockDecompressor() (*BlockDecompressor, error) {
	this := new(BlockDecompressor)

	// Define flags
	var help = flag.Bool("help", false, "display the help message")
	var verbose = flag.Bool("verbose", false, "display the block size at each stage (in bytes, floor rounding if fractional)")
	var overwrite = flag.Bool("overwrite", false, "overwrite the output file if it already exists")
	var silent = flag.Bool("silent", false, "silent mode, no output (except warnings and errors)")
	var inputName = flag.String("input", "", "mandatory name of the input file to decode")
	var outputName = flag.String("output", "", "optional name of the output file")
	var tasks = flag.Int("jobs", 1, "number of parallel jobs")

	// Parse
	flag.Parse()

	if *help == true {
		printOut("-help                : display this message", true)
		printOut("-verbose             : display the block size at each stage (in bytes, floor rounding if fractional)", true)
		printOut("-overwrite           : overwrite the output file if it already exists", true)
		printOut("-silent              : silent mode, no output (except warnings and errors)", true)
		printOut("-input=<inputName>   : mandatory name of the input file to decode", true)
		printOut("-output=<outputName> : optional name of the output file", true)
		printOut("-jobs=<jobs>         : number of parallel jobs", true)
		os.Exit(0)
	}

	if *silent == true && *verbose == true {
		printOut("Warning: both 'silent' and 'verbose' options were selected, ignoring 'verbose'", true)
		*verbose = false
	}

	if len(*inputName) == 0 {
		fmt.Printf("Missing input file name, exiting ...\n")
		os.Exit(io.ERR_MISSING_FILENAME)
	}

	if strings.HasSuffix(*inputName, ".knz") == false {
		printOut("Warning: the input file name does not end with the .KNZ extension", true)
	}

	if len(*outputName) == 0 {
		if strings.HasSuffix(*inputName, ".knz") == false {
			*outputName = *inputName + ".tmp"
		} else {
			*outputName = strings.TrimRight(*inputName, ".knz")
		}
	}

	this.verbose = *verbose
	this.silent = *silent
	this.inputName = *inputName
	this.outputName = *outputName
	this.overwrite = *overwrite
	this.jobs = uint(*tasks)
	this.listeners = list.New()

	if this.verbose == true {
		listener, _ := io.NewInfoPrinter(io.DECODING, os.Stdout)
		this.listeners.PushFront(listener)
	}

	return this, nil
}

func (this *BlockDecompressor) AddListener(bl io.BlockListener) bool {
	if bl == nil {
		return false
	}

	this.listeners.PushFront(bl)
	return true
}

func (this *BlockDecompressor) RemoveListener(bl io.BlockListener) bool {
	if bl == nil {
		return false
	}

	for e := this.listeners.Front(); e != nil; e = e.Next() {
		if e.Value == bl {
			this.listeners.Remove(e)
			return true
		}
	}

	return false
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	bd, err := NewBlockDecompressor()

	if err != nil {
		fmt.Printf("Failed to create block decompressor: %v\n", err)
		os.Exit(io.ERR_CREATE_DECOMPRESSOR)
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("An unexpected error occured during decompression: %v\n", r.(error))
			os.Exit(io.ERR_UNKNOWN)
		}
	}()

	code, _ := bd.call()
	bd.listeners.Init()
	os.Exit(code)
}

// Return exit code, number of bits written
func (this *BlockDecompressor) call() (int, uint64) {
	var msg string
	printOut("Input file name set to '"+this.inputName+"'", this.verbose)
	printOut("Output file name set to '"+this.outputName+"'", this.verbose)
	msg = fmt.Sprintf("Verbose set to %t", this.verbose)
	printOut(msg, this.verbose)
	msg = fmt.Sprintf("Overwrite set to %t", this.overwrite)
	printOut(msg, this.verbose)
	prefix := ""

	if this.jobs > 1 {
		prefix = "s"
	}

	msg = fmt.Sprintf("Using %d job%s", this.jobs, prefix)
	printOut(msg, this.verbose)

	output, err := os.OpenFile(this.outputName, os.O_RDWR, 666)

	if err == nil {
		// File exists
		if this.overwrite == false {
			fmt.Printf("The output file '%v' exists and the 'overwrite' command ", this.outputName)
			fmt.Println("line option has not been provided")
			output.Close()
			return io.ERR_OVERWRITE_FILE, 0
		}
	} else {
		// File does not exist, create
		output, err = os.Create(this.outputName)

		if err != nil {
			fmt.Printf("Cannot open output file '%v' for writing: %v\n", this.outputName, err)
			return io.ERR_CREATE_FILE, 0
		}
	}

	defer output.Close()

	// Decode
	read := uint64(0)
	printOut("Decoding ...", !this.silent)
	input, err := os.Open(this.inputName)

	if err != nil {
		fmt.Printf("Cannot open input file '%v': %v\n", this.inputName, err)
		return io.ERR_OPEN_FILE, read
	}

	defer input.Close()
	verboseWriter := os.Stdout

	if this.verbose == false {
		verboseWriter = nil
	}

	bis, err := io.NewBufferedInputStream(input)

	if err != nil {
		fmt.Printf("Cannot create compressed stream: %v\n", err)
		return io.ERR_CREATE_DECOMPRESSOR, read
	}

	cis, err := io.NewCompressedInputStream(bis, verboseWriter, this.jobs)

	if err != nil {
		if err.(*io.IOError) != nil {
			fmt.Printf("%s\n", err.(*io.IOError).Message())
			return err.(*io.IOError).ErrorCode(), read
		} else {
			fmt.Printf("Cannot create compressed stream: %v\n", err)
			return io.ERR_CREATE_DECOMPRESSOR, read
		}
	}

	for e := this.listeners.Front(); e != nil; e = e.Next() {
		cis.AddListener(e.Value.(io.BlockListener))
	}

	buffer := make([]byte, DECOMP_DEFAULT_BUFFER_SIZE)
	decoded := len(buffer)
	before := time.Now()

	// Decode next block
	for decoded == len(buffer) {
		if decoded, err = cis.Read(buffer); err != nil {
			if ioerr, isIOErr := err.(*io.IOError); isIOErr == true {
				fmt.Printf("%s\n", ioerr.Message())
				return ioerr.ErrorCode(), read
			} else {
				fmt.Printf("An unexpected condition happened. Exiting ...\n%v\n", err)
				return io.ERR_PROCESS_BLOCK, read
			}
		}

		if decoded > 0 {
			_, err = output.Write(buffer[0:decoded])

			if err != nil {
				fmt.Printf("Failed to write decompressed block to file '%v': %v\n", this.outputName, err)
				return io.ERR_WRITE_FILE, read
			}

			read += uint64(decoded)
		}
	}

	// Close streams to ensure all data are flushed
	// Deferred close is fallback for error paths
	cis.Close()

	after := time.Now()
	delta := after.Sub(before).Nanoseconds() / 1000000 // convert to ms

	printOut("", !this.silent)
	msg = fmt.Sprintf("Decoding:          %d ms", delta)
	printOut(msg, !this.silent)
	msg = fmt.Sprintf("Input size:        %d", cis.GetRead())
	printOut(msg, !this.silent)
	msg = fmt.Sprintf("Output size:       %d", read)
	printOut(msg, !this.silent)

	if delta > 0 {
		msg = fmt.Sprintf("Throughput (KB/s): %d", ((read*uint64(1000))>>10)/uint64(delta))
		printOut(msg, !this.silent)
	}

	printOut("", !this.silent)
	return 0, cis.GetRead()
}

func printOut(msg string, print bool) {
	if print == true {
		fmt.Println(msg)
	}
}
