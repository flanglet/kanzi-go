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

package io

import (
	"fmt"
	"time"
)

const (
	EVT_BEFORE_TRANSFORM = 0
	EVT_AFTER_TRANSFORM  = 1
	EVT_BEFORE_ENTROPY   = 2
	EVT_AFTER_ENTROPY    = 3
)

type BlockEvent struct {
	eventType int
	blockId   int
	blockSize int
	hash      uint32
	hashing   bool
	time_     time.Time
}

func (this *BlockEvent) EventType() int {
	return this.eventType
}

func (this *BlockEvent) BlockId() int {
	return this.blockId
}

func (this *BlockEvent) Time() time.Time {
	return this.time_
}

func (this *BlockEvent) BlockSize() int {
	return this.blockSize
}

func (this *BlockEvent) Hash() uint32 {
	return this.hash
}

func (this *BlockEvent) Hashing() bool {
	return this.hashing
}

func (this *BlockEvent) String() string {
	hash := ""
	type_ := ""

	if this.hashing == true {
		hash = fmt.Sprintf(", \"hash\": %x", this.hash)
	}

	if this.eventType == EVT_BEFORE_TRANSFORM {
		type_ = "BEFORE_TRANSFORM"
	} else if this.eventType == EVT_AFTER_TRANSFORM {
		type_ = "AFTER_TRANSFORM"
	} else if this.eventType == EVT_BEFORE_ENTROPY {
		type_ = "BEFORE_ENTROPY"
	} else if this.eventType == EVT_AFTER_ENTROPY {
		type_ = "AFTER_ENTROPY"
	}

	return fmt.Sprintf("{ \"type\":\"%s\", \"id\":%d, \"size\":%d, \"time\":%d%s}", type_, this.blockId, this.blockSize,
		this.time_.UnixNano()/1000000, hash)
}

type BlockListener interface {
	ProcessEvent(evt *BlockEvent)
}
