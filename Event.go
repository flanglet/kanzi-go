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
	EVT_COMPRESSION_START     = 0
	EVT_DECOMPRESSION_START   = 1
	EVT_BEFORE_TRANSFORM      = 2
	EVT_AFTER_TRANSFORM       = 3
	EVT_BEFORE_ENTROPY        = 4
	EVT_AFTER_ENTROPY         = 5
	EVT_COMPRESSION_END       = 6
	EVT_DECOMPRESSION_END     = 7
	EVT_AFTER_HEADER_DECODING = 8
)

type Event struct {
	eventType int
	id        int
	size      int64
	hash      uint32
	hashing   bool
	eventTime time.Time
	msg       string
}

func NewEventFromString(evtType, id int, msg string, evtTime time.Time) *Event {
	if evtTime.IsZero() {
		evtTime = time.Now()
	}

	return &Event{eventType: evtType, id: id, size: 0, msg: msg, eventTime: evtTime}
}

func NewEvent(evtType, id int, size int64, hash uint32, hashing bool, evtTime time.Time) *Event {
	if evtTime.IsZero() {
		evtTime = time.Now()
	}

	return &Event{eventType: evtType, id: id, size: size, hash: hash,
		hashing: hashing, eventTime: evtTime}
}

func (this *Event) Type() int {
	return this.eventType
}

func (this *Event) Id() int {
	return this.id
}

func (this *Event) Time() time.Time {
	return this.eventTime
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
	if len(this.msg) > 0 {
		return this.msg
	}

	hash := ""
	t := ""
	id := ""

	if this.hashing == true {
		hash = fmt.Sprintf(", \"hash\": %x", this.hash)
	}

	if this.id >= 0 {
		id = fmt.Sprintf(", \"id\": %d", this.id)
	}

	switch this.eventType {
	case EVT_BEFORE_TRANSFORM:
		t = "BEFORE_TRANSFORM"

	case EVT_AFTER_TRANSFORM:
		t = "AFTER_TRANSFORM"

	case EVT_BEFORE_ENTROPY:
		t = "BEFORE_ENTROPY"

	case EVT_AFTER_ENTROPY:
		t = "AFTER_ENTROPY"

	case EVT_COMPRESSION_START:
		t = "EVT_COMPRESSION_START"

	case EVT_DECOMPRESSION_START:
		t = "EVT_DECOMPRESSION_START"

	case EVT_COMPRESSION_END:
		t = "EVT_COMPRESSION_END"

	case EVT_DECOMPRESSION_END:
		t = "EVT_DECOMPRESSION_END"
	}

	return fmt.Sprintf("{ \"type\":\"%s\"%s, \"size\":%d, \"time\":%d%s }", t, id, this.size,
		this.eventTime.UnixNano()/1000000, hash)
}

type Listener interface {
	ProcessEvent(evt *Event)
}
