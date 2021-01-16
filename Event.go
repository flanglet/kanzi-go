/*
Copyright 2011-2021 Frederic Langlet
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
	EVT_COMPRESSION_START     = 0 // Compression starts
	EVT_DECOMPRESSION_START   = 1 // Decompression starts
	EVT_BEFORE_TRANSFORM      = 2 // Transform forward/inverse starts
	EVT_AFTER_TRANSFORM       = 3 // Transform forward/inverse ends
	EVT_BEFORE_ENTROPY        = 4 // Entropy encoding/decoding starts
	EVT_AFTER_ENTROPY         = 5 // Entropy encoding/decoding ends
	EVT_COMPRESSION_END       = 6 // Compression ends
	EVT_DECOMPRESSION_END     = 7 // Decompression ends
	EVT_AFTER_HEADER_DECODING = 8 // Compression header decoding ends
)

// Event a compression/decompression event
type Event struct {
	eventType int
	id        int
	size      int64
	hash      uint32
	hashing   bool
	eventTime time.Time
	msg       string
}

// NewEventFromString creates a new Event instance that wraps a message
func NewEventFromString(evtType, id int, msg string, evtTime time.Time) *Event {
	if evtTime.IsZero() {
		evtTime = time.Now()
	}

	return &Event{eventType: evtType, id: id, size: 0, msg: msg, eventTime: evtTime}
}

// NewEvent creates a new Event instance with size and hash info
func NewEvent(evtType, id int, size int64, hash uint32, hashing bool, evtTime time.Time) *Event {
	if evtTime.IsZero() {
		evtTime = time.Now()
	}

	return &Event{eventType: evtType, id: id, size: size, hash: hash,
		hashing: hashing, eventTime: evtTime}
}

// Type returns the type info
func (this *Event) Type() int {
	return this.eventType
}

// ID returns the id info
func (this *Event) ID() int {
	return this.id
}

// Time returns the time info
func (this *Event) Time() time.Time {
	return this.eventTime
}

// Size returns the size info
func (this *Event) Size() int64 {
	return this.size
}

// Hash returns the hash info
func (this *Event) Hash() uint32 {
	return this.hash
}

// Hashing returns true if the event contains a hash info
func (this *Event) Hashing() bool {
	return this.hashing
}

// String returns a string representation of this event.
// If the event wraps a message, the the message is returned.
// Owtherwise a string is built from the fields.
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
		t = "COMPRESSION_START"

	case EVT_DECOMPRESSION_START:
		t = "DECOMPRESSION_START"

	case EVT_COMPRESSION_END:
		t = "COMPRESSION_END"

	case EVT_DECOMPRESSION_END:
		t = "DECOMPRESSION_END"
	}

	return fmt.Sprintf("{ \"type\":\"%s\"%s, \"size\":%d, \"time\":%d%s }", t, id, this.size,
		this.eventTime.UnixNano()/1000000, hash)
}

// Listener is an interface implemented by event processors
type Listener interface {
	ProcessEvent(evt *Event)
}
