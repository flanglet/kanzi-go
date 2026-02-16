/*
Copyright 2011-2026 Frederic Langlet
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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

// An implementation of BlockListener to display block information (verbose option
// of the BlockCompressor/BlockDecompressor)

const (
	// COMPRESSION event type
	COMPRESSION = 0
	// DECOMPRESSION event type
	DECOMPRESSION = 1
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
	writer             io.Writer
	infoType           uint
	infos              map[int32]blockInfo
	orderedPending     map[int]*kanzi.Event
	thresholds         []int
	mapLock            sync.RWMutex
	lock               sync.RWMutex
	level              uint
	headerInfo         uint
	orderedPhase       int
	lastEmittedBlockId atomic.Int32
}

// NewInfoPrinter creates a new instance of InfoPrinter
func NewInfoPrinter(infoLevel, infoType, firstBlockId uint, writer io.Writer) (*InfoPrinter, error) {
	if writer == nil {
		return nil, errors.New("invalid null writer parameter")
	}

	this := &InfoPrinter{}
	this.infoType = infoType & 3
	this.level = infoLevel
	this.headerInfo = 0
	this.writer = writer
	this.lastEmittedBlockId.Store(int32(firstBlockId - 1))
	this.infos = make(map[int32]blockInfo)
	this.orderedPending = make(map[int]*kanzi.Event)

	if this.infoType == COMPRESSION {
		this.thresholds = []int{
			kanzi.EVT_COMPRESSION_START,
			kanzi.EVT_BEFORE_TRANSFORM,
			kanzi.EVT_AFTER_TRANSFORM,
			kanzi.EVT_BEFORE_ENTROPY,
			kanzi.EVT_AFTER_ENTROPY,
			kanzi.EVT_COMPRESSION_END,
		}
		this.orderedPhase = kanzi.EVT_AFTER_ENTROPY
	} else {
		this.thresholds = []int{
			kanzi.EVT_DECOMPRESSION_START,
			kanzi.EVT_BEFORE_ENTROPY,
			kanzi.EVT_AFTER_ENTROPY,
			kanzi.EVT_BEFORE_TRANSFORM,
			kanzi.EVT_AFTER_TRANSFORM,
			kanzi.EVT_DECOMPRESSION_END,
		}
		this.orderedPhase = kanzi.EVT_BEFORE_ENTROPY
	}

	return this, nil
}

// ProcessEvent receives an event and writes a log record to the internal writer
func (this *InfoPrinter) ProcessEvent(evt *kanzi.Event) {
	if this.infoType == INFO {
		this.processHeaderInfo(evt)
		return
	}

	if evt.Type() == this.orderedPhase {
		this.processOrderedPhase(evt)
		return
	}

	this.processEventOrdered(evt)
}

func (this *InfoPrinter) processOrderedPhase(evt *kanzi.Event) {
	this.lock.Lock()
	this.orderedPending[evt.ID()] = evt
	this.lock.Unlock()

	for {
		nextId := this.lastEmittedBlockId.Load() + 1
		this.lock.Lock()
		next, found := this.orderedPending[int(nextId)]

		if found == false {
			this.lock.Unlock()
			return
		}

		delete(this.orderedPending, int(nextId))
		this.lock.Unlock()

		// Now it is safe to advance
		this.lastEmittedBlockId.Store(nextId)

		// Compression: AFTER_ENTROPY emitted in-order
		// Decompression: BEFORE_TRANSFORM emitted in-order
		this.processEventOrdered(next)
	}
}

func (this *InfoPrinter) processEventOrdered(evt *kanzi.Event) {
	currentBlockID := int32(evt.ID())
	//fmt.Printf("%v\n", evt)

	if evt.Type() == this.thresholds[1] {
		// Register initial block size
		bi := blockInfo{time0: evt.Time()}
		bi.stage0Size = evt.Size()

		this.mapLock.Lock()
		this.infos[currentBlockID] = bi
		this.mapLock.Unlock()

		if this.level >= 5 {
			fmt.Fprintln(this.writer, evt)
		}
	} else if evt.Type() == this.thresholds[2] {
		this.mapLock.RLock()
		bi, exists := this.infos[currentBlockID]
		this.mapLock.RUnlock()

		if exists == true {
			bi.time1 = evt.Time()

			this.mapLock.Lock()
			this.infos[currentBlockID] = bi
			this.mapLock.Unlock()

			if this.level >= 5 {
				durationMS := bi.time1.Sub(bi.time0).Nanoseconds() / int64(time.Millisecond)
				fmt.Fprintf(this.writer, "%s [%d ms]\n", evt, durationMS)
			}
		}
	} else if evt.Type() == this.thresholds[3] {
		this.mapLock.RLock()
		bi, exists := this.infos[currentBlockID]
		this.mapLock.RUnlock()

		if exists == true {
			bi.time2 = evt.Time()
			bi.stage1Size = evt.Size()
			this.mapLock.Lock()
			this.infos[currentBlockID] = bi
			this.mapLock.Unlock()

			if this.level >= 5 {
				durationMS := bi.time2.Sub(bi.time1).Nanoseconds() / int64(time.Millisecond)
				fmt.Fprintf(this.writer, "%s [%d ms]\n", evt, durationMS)
			}
		}
	} else if evt.Type() == this.thresholds[4] {
		this.mapLock.RLock()
		bi, exists := this.infos[currentBlockID]
		this.mapLock.RUnlock()

		if exists == false || this.level < 3 {
			return
		}

		this.mapLock.Lock()
		delete(this.infos, currentBlockID)
		this.mapLock.Unlock()
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
			if this.infoType == COMPRESSION {
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
		info := evt.Info()

		if info == nil {
			return
		}

		if this.level >= 5 {
			// JSON text
			fmt.Fprintln(this.writer, evt.String())
		} else {
			// Raw text
			var sb strings.Builder

			sb.WriteString("\n")
			sb.WriteString("Bitstream version: ")
			sb.WriteString(strconv.Itoa(info.BsVersion))
			sb.WriteString("\n")

			c := info.ChecksumSize
			sb.WriteString("Block checksum: ")

			if c == 0 {
				sb.WriteString("NONE\n")
			} else {
				sb.WriteString(strconv.Itoa(c))
				sb.WriteString(" bits\n")
			}

			sb.WriteString("Block size: ")
			sb.WriteString(strconv.Itoa(info.BlockSize))
			sb.WriteString(" bytes\n")

			e := info.EntropyType

			if e == "" {
				e = "no"
			}

			sb.WriteString("Using ")
			sb.WriteString(e)
			sb.WriteString(" entropy codec (stage 1)\n")

			t := info.TransformType

			if t == "" {
				t = "no"
			}

			sb.WriteString("Using ")
			sb.WriteString(t)
			sb.WriteString(" transform (stage 2)\n")

			if info.OriginalSize >= 0 {
				sb.WriteString("Original size: ")
				sb.WriteString(strconv.FormatInt(info.OriginalSize, 10))
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

	info := evt.Info()

	if info == nil {
		return
	}

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

	inputName := info.InputName
	idx := strings.LastIndex(inputName, string(os.PathSeparator))

	if idx >= 0 {
		inputName = inputName[idx+1:]
	}

	if len(inputName) > 20 {
		inputName = inputName[0:18] + ".."
	}

	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf("%-20s", inputName))
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf("%3s", strconv.Itoa(info.BsVersion)))
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf("%5s", strconv.Itoa(info.ChecksumSize)))
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf("%10s", strconv.Itoa(info.BlockSize)))
	sb.WriteString("|")

	if info.FileSize >= 0 {
		sb.WriteString(fmt.Sprintf("%12s", this.formatSize(float64(info.FileSize))))
		sb.WriteString("|")
	}

	if info.OriginalSize >= 0 {
		sb.WriteString(fmt.Sprintf("%12s", this.formatSize(float64(info.OriginalSize))))
		sb.WriteString("|")
	}

	if info.FileSize >= 0 && info.OriginalSize >= 0 {
		if info.FileSize < 0 || info.OriginalSize < 0 {
			sb.WriteString("  N/A  |")
		} else {
			compSz := float64(info.FileSize)
			origSz := float64(info.OriginalSize)
			ratio := compSz / origSz
			sb.WriteString(fmt.Sprintf(" %.3f ", ratio))
			sb.WriteString("|")
		}
	} else {
		sb.WriteString("  N/A  |")
	}

	if this.level >= 4 {
		sb.WriteString(fmt.Sprintf("%8s", info.EntropyType))
		sb.WriteString("|")
	}

	if this.level >= 4 {
		t := info.TransformType

		if len(t) > 26 {
			t = t[0:24] + ".."
		}

		sb.WriteString(fmt.Sprintf("%26s", t))
		sb.WriteString("|")
	}

	fmt.Fprintln(this.writer, sb.String())
	this.headerInfo++
}

func (this *InfoPrinter) formatSize(size float64) string {
	var s string

	if size >= float64(1<<30) {
		size /= float64(1 << 30)
		s = fmt.Sprintf("%.2f", size) + " GiB"
	} else if size >= float64(1<<20) {
		size /= float64(1 << 20)
		s = fmt.Sprintf("%.2f", size) + " MiB"
	} else if size >= float64(1<<10) {
		size /= float64(1 << 10)
		s = fmt.Sprintf("%.2f", size) + " KiB"
	} else {
		s = fmt.Sprintf("%f", size)
	}

	return s
}
