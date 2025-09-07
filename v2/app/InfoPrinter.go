/*
Copyright 2011-2025 Frederic Langlet
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
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

// An implementation of BlockListener to display block information (verbose option
// of the BlockCompressor/BlockDecompressor)

const (
	// ENCODING event type
	ENCODING = 0
	// DECODING event type
	DECODING = 1
	// INFO event type
	INFO = 2
)

type blockInfo struct {
	time0      time.Time
	time1      time.Time
	time2      time.Time
	time3      time.Time
	stage0Size int64
	stage1Size int64
}

// InfoPrinter contains all the data required to print one event
type InfoPrinter struct {
	writer     io.Writer
	infoType   uint
	infos      map[int32]blockInfo
	thresholds []int
	lock       sync.RWMutex
	level      uint
	headerInfo uint
}

// NewInfoPrinter creates a new instance of InfoPrinter
func NewInfoPrinter(infoLevel, infoType uint, writer io.Writer) (*InfoPrinter, error) {
	if writer == nil {
		return nil, errors.New("invalid null writer parameter")
	}

	this := &InfoPrinter{}
	this.infoType = infoType & 3
	this.level = infoLevel
	this.headerInfo = 0
	this.writer = writer
	this.infos = make(map[int32]blockInfo)

	if this.infoType == ENCODING {
		this.thresholds = []int{
			kanzi.EVT_COMPRESSION_START,
			kanzi.EVT_BEFORE_TRANSFORM,
			kanzi.EVT_AFTER_TRANSFORM,
			kanzi.EVT_BEFORE_ENTROPY,
			kanzi.EVT_AFTER_ENTROPY,
			kanzi.EVT_COMPRESSION_END,
		}
	} else {
		this.thresholds = []int{
			kanzi.EVT_DECOMPRESSION_START,
			kanzi.EVT_BEFORE_ENTROPY,
			kanzi.EVT_AFTER_ENTROPY,
			kanzi.EVT_BEFORE_TRANSFORM,
			kanzi.EVT_AFTER_TRANSFORM,
			kanzi.EVT_DECOMPRESSION_END,
		}
	}

	return this, nil
}

// ProcessEvent receives an event and writes a log record to the internal writer
func (this *InfoPrinter) ProcessEvent(evt *kanzi.Event) {
	if this.infoType == INFO {
		this.processHeaderInfo(evt)
		return
	}

	currentBlockID := int32(evt.ID())

	if evt.Type() == this.thresholds[1] {
		// Register initial block size
		bi := blockInfo{time0: evt.Time()}
		bi.stage0Size = evt.Size()

		this.lock.Lock()
		this.infos[currentBlockID] = bi
		this.lock.Unlock()

		if this.level >= 5 {
			fmt.Fprintln(this.writer, evt)
		}
	} else if evt.Type() == this.thresholds[2] {
		this.lock.RLock()
		bi, exists := this.infos[currentBlockID]
		this.lock.RUnlock()

		if exists == true {
			bi.time1 = evt.Time()

			this.lock.Lock()
			this.infos[currentBlockID] = bi
			this.lock.Unlock()

			if this.level >= 5 {
				durationMS := bi.time1.Sub(bi.time0).Nanoseconds() / int64(time.Millisecond)
				fmt.Fprintf(this.writer, "%s [%d ms]\n", evt, durationMS)
			}
		}
	} else if evt.Type() == this.thresholds[3] {
		this.lock.RLock()
		bi, exists := this.infos[currentBlockID]
		this.lock.RUnlock()

		if exists == true {
			bi.time2 = evt.Time()
			bi.stage1Size = evt.Size()
			this.lock.Lock()
			this.infos[currentBlockID] = bi
			this.lock.Unlock()

			if this.level >= 5 {
				durationMS := bi.time2.Sub(bi.time1).Nanoseconds() / int64(time.Millisecond)
				fmt.Fprintf(this.writer, "%s [%d ms]\n", evt, durationMS)
			}
		}
	} else if evt.Type() == this.thresholds[4] {
		this.lock.RLock()
		bi, exists := this.infos[currentBlockID]
		this.lock.RUnlock()

		if exists == false || this.level < 3 {
			return
		}

		this.lock.Lock()
		delete(this.infos, currentBlockID)
		this.lock.Unlock()
		bi.time3 = evt.Time()
		duration1MS := bi.time1.Sub(bi.time0).Nanoseconds() / int64(time.Millisecond)
		duration2MS := bi.time3.Sub(bi.time2).Nanoseconds() / int64(time.Millisecond)

		// Get block size after stage 2
		stage2Size := evt.Size()

		// Display block info
		var msg string

		if this.level >= 5 {
			fmt.Fprintf(this.writer, "%s [%d ms]\n", evt, duration2MS)
		}

		// Display block info
		if this.level >= 4 {
			msg = fmt.Sprintf("Block %d: %d => %d [%d ms] => %d [%d ms]", currentBlockID,
				bi.stage0Size, bi.stage1Size, duration1MS, stage2Size, duration2MS)

			// Add compression ratio for encoding
			if this.infoType == ENCODING {
				if bi.stage0Size != 0 {
					msg += fmt.Sprintf(" (%d%%)", uint64(stage2Size)*100/uint64(bi.stage0Size))
				}
			}

			// Optionally add hash
			if evt.HashType() != kanzi.EVT_HASH_NONE {
				msg += fmt.Sprintf("  [%x]", evt.Hash())
			}

			fmt.Fprintln(this.writer, msg)
		}
	} else if evt.Type() == kanzi.EVT_AFTER_HEADER_DECODING && this.level >= 3 {
		// Special CSV format
		s := evt.String()
		r := csv.NewReader(strings.NewReader(s))
		tokens, _ := r.Read()
		nbTokens := len(tokens)
		var sb strings.Builder

		if this.level >= 5 {
			// JSON text
			sb.WriteString("{ \"type\":\"")
			sb.WriteString(evt.TypeAsString())
			sb.WriteString("\"")

			if nbTokens > 1 {
				sb.WriteString(", \"bsversion\":")
				sb.WriteString(tokens[1])
			}

			if nbTokens > 2 {
				sb.WriteString(", \"checksize\":")
				sb.WriteString(tokens[2])
			}

			if nbTokens > 3 {
				sb.WriteString(", \"blocksize\":")
				sb.WriteString(tokens[3])
			}

			if nbTokens > 4 {
				e := tokens[4]

				if e == "" {
					e = "NONE"
				}

				sb.WriteString(", \"entropy\":")
				sb.WriteString("\"")
				sb.WriteString(e)
				sb.WriteString("\"")
			}

			if nbTokens > 5 {
				t := tokens[5]

				if t == "" {
					t = "NONE"
				}

				sb.WriteString(", \"transforms\":")
				sb.WriteString("\"")
				sb.WriteString(t)
				sb.WriteString("\"")
			}

			if nbTokens > 6 && tokens[6] != "" {
				sb.WriteString(", \"compressed\":")
				sb.WriteString(tokens[6])
			}

			if nbTokens > 7 && tokens[7] != "" {
				sb.WriteString(", \"original\":")
				sb.WriteString(tokens[7])
			}

			sb.WriteString(" }")
			fmt.Fprintln(this.writer, sb.String())
		} else {
			// Raw text
			if nbTokens > 1 {
				sb.WriteString("\n")
				sb.WriteString("Bitstream version: ")
				sb.WriteString(tokens[1])
				sb.WriteString("\n")
			}

			if nbTokens > 2 {
				c := tokens[2]
				sb.WriteString("Block checksum: ")

				if c == "0" {
					sb.WriteString("NONE\n")
				} else {
					sb.WriteString(c)
					sb.WriteString(" bits\n")
				}
			}

			if nbTokens > 3 {
				sb.WriteString("Block size: ")
				sb.WriteString(tokens[3])
				sb.WriteString(" bytes\n")
			}

			if nbTokens > 4 {
				e := tokens[4]

				if e == "" {
					e = "no"
				}

				sb.WriteString("Using ")
				sb.WriteString(e)
				sb.WriteString(" entropy codec (stage 1)\n")
			}

			if nbTokens > 5 {
				t := tokens[5]

				if t == "" {
					t = "no"
				}

				sb.WriteString("Using ")
				sb.WriteString(t)
				sb.WriteString(" transform (stage 2)\n")
			}

			if nbTokens > 7 && tokens[7] != "" {
				sb.WriteString("Original size: ")
				sb.WriteString(tokens[7])
				sb.WriteString(" byte(s)\n")
			}

			fmt.Fprintln(this.writer, sb.String())

		}
	} else if this.level >= 5 {
		fmt.Fprintln(this.writer, evt)
	}
}

func (this *InfoPrinter) processHeaderInfo(evt *kanzi.Event) {
	if this.level == 0 || evt.Type() != kanzi.EVT_AFTER_HEADER_DECODING {
		return
	}

	s := evt.String()
	r := csv.NewReader(strings.NewReader(s))
	tokens, _ := r.Read()
	nbTokens := len(tokens)
	var sb strings.Builder

	if this.headerInfo == 0 {
		// Display header
		sb.WriteString("\n")
		sb.WriteString("|     File Name      ")
		sb.WriteString("|Ver")
		sb.WriteString("|Check")
		sb.WriteString("|Block Size")
		sb.WriteString("|  File Size ")
		sb.WriteString("| Orig. Size ")
		sb.WriteString("| Ratio ")

		if this.level >= 4 {
			sb.WriteString("| Entropy")
			sb.WriteString("|        Transforms        ")
		}

		sb.WriteString("|\n")
	}

	if nbTokens > 0 {
		inputName := tokens[0]
		idx := strings.LastIndex(inputName, string(os.PathSeparator))

		if idx >= 0 {
			inputName = inputName[idx+1:]
		}

		if len(inputName) > 20 {
			inputName = inputName[18:] + ".."
		}

		sb.WriteString("|")
		sb.WriteString(fmt.Sprintf("%-20s", inputName))
		sb.WriteString("|") // inputName
	}

	if nbTokens > 1 {
		sb.WriteString(fmt.Sprintf("%3s", tokens[1]))
		sb.WriteString("|") //bsVersion
	}

	if nbTokens > 2 {
		sb.WriteString(fmt.Sprintf("%5s", tokens[2]))
		sb.WriteString("|") // checksum
	}

	if nbTokens > 3 {
		sb.WriteString(fmt.Sprintf("%10s", tokens[3]))
		sb.WriteString("|") // block size
	}

	if nbTokens > 6 {
		sb.WriteString(fmt.Sprintf("%12s", this.formatSize(tokens[6])))
		sb.WriteString("|") // compressed size
	}

	if nbTokens > 7 {
		sb.WriteString(fmt.Sprintf("%12s", this.formatSize(tokens[7])))
		sb.WriteString("|") // original size
	}

	if tokens[6] != "" && tokens[7] != "" {
		compStr := tokens[6]
		origStr := tokens[7]
		compSz, err1 := strconv.ParseFloat(compStr, 64)
		origSz, err2 := strconv.ParseFloat(origStr, 64)

		if err1 != nil || err2 != nil {
			sb.WriteString("  N/A  |")
		} else {
			ratio := compSz / origSz
			sb.WriteString(fmt.Sprintf(" %.3f ", ratio))
			sb.WriteString("|") // compression ratio
		}
	} else {
		sb.WriteString("  N/A  |")
	}

	if this.level >= 4 && nbTokens > 4 {
		if tokens[4] == "" {
			sb.WriteString(fmt.Sprintf("%9s", "NONE|"))
		} else {
			sb.WriteString(fmt.Sprintf("%8s", tokens[4]))
			sb.WriteString("|")
		}
	}

	if this.level >= 4 && nbTokens > 5 {
		t := tokens[5]

		if len(t) > 26 {
			t = t[0:24] + ".."
		}

		if t == "" {
			sb.WriteString(fmt.Sprintf("%27s", "NONE|"))
		} else {
			sb.WriteString(fmt.Sprintf("%26s", t))
			sb.WriteString("|") // transforms
		}
	}

	fmt.Fprintln(this.writer, sb.String())
	this.headerInfo++
}

func (this *InfoPrinter) formatSize(input string) string {
	size, err := strconv.ParseFloat(input, 64)

	if err != nil {
		return "N/A"
	}

	s := input

	if size >= float64(1<<30) {
		size /= float64(1 << 30)
		s = fmt.Sprintf("%.2f", size) + " GiB"
	} else if size >= float64(1<<20) {
		size /= float64(1 << 20)
		s = fmt.Sprintf("%.2f", size) + " MiB"
	} else if size >= float64(1<<10) {
		size /= float64(1 << 10)
		s = fmt.Sprintf("%.2f", size) + " KiB"
	}

	return s
}
