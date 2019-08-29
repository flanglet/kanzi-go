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
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	kanzi "github.com/flanglet/kanzi-go"
)

// An implementation of BlockListener to display block information (verbose option
// of the BlockCompressor/BlockDecompressor)

const (
	// ENCODING event type
	ENCODING = 0
	// DECODING event type
	DECODING = 1
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
}

// NewInfoPrinter creates a new instance of InfoPrinter
func NewInfoPrinter(infoLevel, infoType uint, writer io.Writer) (*InfoPrinter, error) {
	if writer == nil {
		return nil, errors.New("Invalid null writer parameter")
	}

	this := &InfoPrinter{}
	this.infoType = infoType & 1
	this.level = infoLevel
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
	currentBlockID := int32(evt.ID())

	if evt.Type() == this.thresholds[1] {
		// Register initial block size
		bi := blockInfo{time0: evt.Time()}

		if this.infoType == ENCODING {
			bi.stage0Size = evt.Size()
		}

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

			if this.infoType == DECODING {
				bi.stage0Size = evt.Size()
			}

			this.lock.Lock()
			this.infos[currentBlockID] = bi
			this.lock.Unlock()

			if this.level >= 5 {
				durationMS := bi.time1.Sub(bi.time0).Nanoseconds() / int64(time.Millisecond)
				fmt.Fprintln(this.writer, fmt.Sprintf("%s [%d ms]", evt, durationMS))
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
				fmt.Fprintln(this.writer, fmt.Sprintf("%s [%d ms]", evt, durationMS))
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
			fmt.Fprintln(this.writer, fmt.Sprintf("%s [%d ms]", evt, duration2MS))
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
			if evt.Hashing() == true {
				msg += fmt.Sprintf("  [%x]", evt.Hash())
			}

			fmt.Fprintln(this.writer, msg)
		}
	} else if evt.Type() == kanzi.EVT_AFTER_HEADER_DECODING && this.level >= 3 {
		fmt.Fprintln(this.writer, evt)
	} else if this.level >= 5 {
		fmt.Fprintln(this.writer, evt)
	}
}
