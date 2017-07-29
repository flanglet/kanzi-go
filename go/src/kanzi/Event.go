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

package kanzi

import (
	"fmt"
	"time"
)

const (
	EVT_COMPRESSION_START   = 0
	EVT_DECOMPRESSION_START = 1
	EVT_BEFORE_TRANSFORM    = 2
	EVT_AFTER_TRANSFORM     = 3
	EVT_BEFORE_ENTROPY      = 4
	EVT_AFTER_ENTROPY       = 5
	EVT_COMPRESSION_END     = 6
	EVT_DECOMPRESSION_END   = 7
)

type Event struct {
	eventType int
	id        int
	size      int64
	hash      uint32
	hashing   bool
	time_     time.Time
}

func NewEvent(eventType, id int, size int64, hash uint32, hashing bool) *Event {
	return &Event{eventType: eventType, id: id, size: size, hash: hash,
		hashing: hashing, time_: time.Now()}
}

func (this *Event) EventType() int {
	return this.eventType
}

func (this *Event) Id() int {
	return this.id
}

func (this *Event) Time() time.Time {
	return this.time_
}

func (this *Event) Size() int64 {
	return this.size
}

func (this *Event) Hash() uint32 {
	return this.hash
}

func (this *Event) Hashing() bool {
	return this.hashing
}

func (this *Event) String() string {
	hash := ""
	type_ := ""
	id_ := ""

	if this.hashing == true {
		hash = fmt.Sprintf(", \"hash\": %x", this.hash)
	}

	if this.id >= 0 {
		id_ = fmt.Sprintf(", \"id\": %d", this.id)
	}

	if this.eventType == EVT_BEFORE_TRANSFORM {
		type_ = "BEFORE_TRANSFORM"
	} else if this.eventType == EVT_AFTER_TRANSFORM {
		type_ = "AFTER_TRANSFORM"
	} else if this.eventType == EVT_BEFORE_ENTROPY {
		type_ = "BEFORE_ENTROPY"
	} else if this.eventType == EVT_AFTER_ENTROPY {
		type_ = "AFTER_ENTROPY"
	} else if this.eventType == EVT_COMPRESSION_START {
		type_ = "EVT_COMPRESSION_START"
	} else if this.eventType == EVT_DECOMPRESSION_START {
		type_ = "EVT_DECOMPRESSION_START"
	} else if this.eventType == EVT_COMPRESSION_END {
		type_ = "EVT_COMPRESSION_END"
	} else if this.eventType == EVT_DECOMPRESSION_END {
		type_ = "EVT_DECOMPRESSION_END"
	}

	return fmt.Sprintf("{ \"type\":\"%s\"%s, \"size\":%d, \"time\":%d%s }", type_, id_, this.size,
		this.time_.UnixNano()/1000000, hash)
}

type Listener interface {
	ProcessEvent(evt *Event)
}
