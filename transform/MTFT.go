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

package transform

import (
	"errors"
	"fmt"
	kanzi "github.com/flanglet/kanzi-go"
)

const (
	RESET_THRESHOLD = 64
	LIST_LENGTH     = 17
)

type Payload struct {
	previous *Payload
	next     *Payload
	value    byte
}

type MTFT struct {
	lengths [16]int
	buckets [256]byte
	heads   [16]*Payload
	anchor  *Payload
}

func NewMTFT() (*MTFT, error) {
	this := new(MTFT)
	this.heads = [16]*Payload{}
	this.lengths = [16]int{}
	this.buckets = [256]byte{}
	return this, nil
}

func (this *MTFT) Inverse(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if len(src) == 0 {
		return 0, 0, nil
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count > len(dst) {
		errMsg := fmt.Sprintf("Block size is %v, output buffer length is %v", count, len(src))
		return 0, 0, errors.New(errMsg)
	}

	indexes := this.buckets

	for i := range indexes {
		indexes[i] = byte(i)
	}

	value := byte(0)

	for i := 0; i < count; i++ {
		if src[i] == 0 {
			dst[i] = value
			continue
		}

		idx := int(src[i])
		value = indexes[idx]
		dst[i] = value

		if idx <= 16 {
			for j := idx - 1; j >= 0; j-- {
				indexes[j+1] = indexes[j]
			}
		} else {
			copy(indexes[1:], indexes[0:idx])
		}

		indexes[0] = value
	}

	return uint(count), uint(count), nil
}

// Initialize the linked lists: 1 item in bucket 0 and LIST_LENGTH in each other
// Used by forward() only
func (this *MTFT) initLists() {
	array := make([]*Payload, 257)
	array[0] = &Payload{value: 0}
	previous := array[0]
	this.heads[0] = previous
	this.lengths[0] = 1
	this.buckets[0] = 0
	listIdx := byte(0)

	for i := 1; i < 256; i++ {
		array[i] = &Payload{value: byte(i)}

		if (i-1)%LIST_LENGTH == 0 {
			listIdx++
			this.heads[listIdx] = array[i]
			this.lengths[listIdx] = LIST_LENGTH
		}

		this.buckets[i] = listIdx
		previous.next = array[i]
		array[i].previous = previous
		previous = array[i]
	}

	// Create a fake end payload so that every payload in every list has a successor
	array[256] = &Payload{value: 0}
	this.anchor = array[256]
	previous.next = this.anchor
}

// Recreate one list with 1 item and 15 lists with LIST_LENGTH items
// Update lengths and buckets accordingly.
// Used by forward() only
func (this *MTFT) balanceLists(resetValues bool) {
	this.lengths[0] = 1
	p := this.heads[0].next
	val := byte(0)

	if resetValues == true {
		this.heads[0].value = byte(0)
		this.buckets[0] = 0
	}

	for listIdx := byte(1); listIdx < 16; listIdx++ {
		this.heads[listIdx] = p
		this.lengths[listIdx] = LIST_LENGTH

		for n := 0; n < LIST_LENGTH; n++ {
			if resetValues == true {
				val++
				p.value = val
			}

			this.buckets[int(p.value)] = listIdx
			p = p.next
		}
	}
}

func (this *MTFT) Forward(src, dst []byte) (uint, uint, error) {
	if src == nil {
		return 0, 0, errors.New("Input buffer cannot be null")
	}

	if dst == nil {
		return 0, 0, errors.New("Output buffer cannot be null")
	}

	if len(src) == 0 {
		return 0, 0, nil
	}

	if kanzi.SameByteSlices(src, dst, false) {
		return 0, 0, errors.New("Input and output buffers cannot be equal")
	}

	count := len(src)

	if count > len(dst) {
		errMsg := fmt.Sprintf("Block size is %v, output buffer length is %v", count, len(src))
		return 0, 0, errors.New(errMsg)
	}

	if this.anchor == nil {
		this.initLists()
	} else {
		this.balanceLists(true)
	}

	previous := this.heads[0].value

	for ii := 0; ii < count; ii++ {
		current := src[ii]

		if current == previous {
			dst[ii] = byte(0)
			continue
		}

		// Find list index
		listIdx := int(this.buckets[int(current)])
		p := this.heads[listIdx]
		idx := 0

		for i := 0; i < listIdx; i++ {
			idx += this.lengths[i]
		}

		// Find index in list (less than RESET_THRESHOLD iterations)
		for p.value != current {
			p = p.next
			idx++
		}

		dst[ii] = byte(idx)

		// Unlink (the end anchor ensures p.next != nil)
		p.previous.next = p.next
		p.next.previous = p.previous

		// Add to head of first list
		p.next = this.heads[0]
		p.next.previous = p
		this.heads[0] = p

		// Update list information
		if listIdx != 0 {
			// Update head if needed
			if p == this.heads[listIdx] {
				this.heads[listIdx] = p.previous.next
			}

			this.buckets[int(current)] = 0

			if this.lengths[0] >= RESET_THRESHOLD {
				this.balanceLists(false)
			} else {
				this.lengths[listIdx]--
				this.lengths[0]++
			}
		}

		previous = current
	}

	return uint(count), uint(count), nil
}
