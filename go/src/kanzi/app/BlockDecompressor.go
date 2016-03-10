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
	"flag"
	"fmt"
	"io"
	kio "kanzi/io"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	DECOMP_DEFAULT_BUFFER_SIZE = 32768
)

// Main block decompressor struct
type BlockDecompressor struct {
	verbosity  uint
	overwrite  bool
	inputName  string
	outputName string
	jobs       uint
	listeners  []kio.BlockListener
}

func NewBlockDecompressor() (*BlockDecompressor, error) {
	this := new(BlockDecompressor)

	// Define flags
	var help = flag.Bool("help", false, "display the help message")
	var verbose = flag.Int("verbose", 1, "set the verbosity level [0..4]")
	var overwrite = flag.Bool("overwrite", false, "overwrite the output file if it already exists")
	var inputName = flag.String("input", "", "mandatory name of the input file to decode")
	var outputName = flag.String("output", "", "optional name of the output file or 'none' for dry-run")
	var tasks = flag.Int("jobs", 1, "number of concurrent jobs")

	// Parse
	flag.Parse()

	if *help == true {
		printOut("-help                : display this message", true)
		printOut("-verbose=<level>     : set the verbosity level [0..4]", true)
		printOut("                       0=silent, 1=default, 2=display block size (byte rounded)", true)
		printOut("                       3=display timings, 4=display extra information", true)
		printOut("-overwrite           : overwrite the output file if it already exists", true)
		printOut("-input=<inputName>   : mandatory name of the input file to decode", true)
		printOut("-output=<outputName> : optional name of the output file or 'none' for dry-run", true)
		printOut("-jobs=<jobs>         : number of concurrent jobs", true)
		printOut("", true)
		printOut("EG. go run BlockDecompressor -input=foo.knz -overwrite -verbose=2 -jobs=2", true)
		os.Exit(0)
	}

	if len(*inputName) == 0 {
		fmt.Printf("Missing input file name, exiting ...\n")
		os.Exit(kio.ERR_MISSING_PARAM)
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

	if *tasks < 1 {
		fmt.Printf("Invalid number of jobs provided on command line: %v\n", *tasks)
		os.Exit(kio.ERR_INVALID_PARAM)
	}

	if *verbose < 0 {
		fmt.Printf("Invalid verbosity level provided on command line: %v\n", *verbose)
		os.Exit(kio.ERR_INVALID_PARAM)
	}

	this.verbosity = uint(*verbose)
	this.inputName = *inputName
	this.outputName = *outputName
	this.overwrite = *overwrite
	this.jobs = uint(*tasks)
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

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	bd, err := NewBlockDecompressor()

	if err != nil {
		fmt.Printf("Failed to create block decompressor: %v\n", err)
		os.Exit(kio.ERR_CREATE_DECOMPRESSOR)
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("An unexpected error occured during decompression: %v\n", r.(error))
			os.Exit(kio.ERR_UNKNOWN)
		}
	}()

	code, _ := bd.call()
	os.Exit(code)
}

// Return exit code, number of bits written
func (this *BlockDecompressor) call() (int, uint64) {
	var msg string
	printFlag := this.verbosity > 1
	printOut("Input file name set to '"+this.inputName+"'", printFlag)
	printOut("Output file name set to '"+this.outputName+"'", printFlag)
	msg = fmt.Sprintf("Verbosity set to %v", this.verbosity)
	printOut(msg, printFlag)
	msg = fmt.Sprintf("Overwrite set to %t", this.overwrite)
	printOut(msg, printFlag)
	prefix := ""

	if this.jobs > 1 {
		prefix = "s"
	}

	msg = fmt.Sprintf("Using %d job%s", this.jobs, prefix)
	printOut(msg, printFlag)
	var output io.WriteCloser

	if strings.ToUpper(this.outputName) == "NONE" {
		output, _ = kio.NewNullOutputStream()
	} else {
		var err error
		output, err = os.OpenFile(this.outputName, os.O_RDWR, 666)

		if err == nil {
			// File exists
			if this.overwrite == false {
				fmt.Printf("The output file '%v' exists and the 'overwrite' command ", this.outputName)
				fmt.Println("line option has not been provided")
				output.Close()
				return kio.ERR_OVERWRITE_FILE, 0
			}
		} else {
			// File does not exist, create
			output, err = os.Create(this.outputName)

			if err != nil {
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
	printOut("Decoding ...", !silent)
	input, err := os.Open(this.inputName)

	if err != nil {
		fmt.Printf("Cannot open input file '%v': %v\n", this.inputName, err)
		return kio.ERR_OPEN_FILE, read
	}

	defer func() {
		input.Close()
	}()

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

	printOut("", !silent)
	msg = fmt.Sprintf("Decoding:          %d ms", delta)
	printOut(msg, !silent)
	msg = fmt.Sprintf("Input size:        %d", cis.GetRead())
	printOut(msg, !silent)
	msg = fmt.Sprintf("Output size:       %d", read)
	printOut(msg, !silent)

	if delta > 0 {
		msg = fmt.Sprintf("Throughput (KB/s): %d", ((read*uint64(1000))>>10)/uint64(delta))
		printOut(msg, !silent)
	}

	printOut("", !silent)
	return 0, cis.GetRead()
}

func printOut(msg string, print bool) {
	if print == true {
		fmt.Println(msg)
	}
}
