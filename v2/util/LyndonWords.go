/*
Copyright 2011-2022 Frederic Langlet
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

package util

import (
	"container/list"
)

// Utility class to decompose a string into Lyndon words using the Chen-Fox-Lyndon algorithm

// LyndonWords main structure used to decompose a string into Lyndon words
type LyndonWords struct {
	breakpoints *list.List
}

// NewLyndonWords creates a new instance of LyndonWords
func NewLyndonWords() (*LyndonWords, error) {
	this := &LyndonWords{}
	this.breakpoints = list.New()
	return this, nil
}

func (this *LyndonWords) chenFoxLyndonBreakpoints(s string) *list.List {
	k := 0
	length := len(s)
	this.breakpoints.Init()

	for k < length {
		i := k
		j := k + 1

		for j < length && s[i] <= s[j] {
			if s[i] == s[j] {
				i++
			} else {
				i = k
			}

			j++
		}

		for k <= i {
			k += (j - i)
			this.breakpoints.PushBack(k)
		}
	}

	return this.breakpoints
}

// Split partitions the given sting into Lyndon words
func (this *LyndonWords) Split(s string) []string {
	l := this.chenFoxLyndonBreakpoints(s)
	res := make([]string, l.Len())
	n := 0
	prev := 0

	for bp := l.Front(); bp != nil; bp = bp.Next() {
		cur := bp.Value.(int)
		res[n] = s[prev:cur]
		prev = cur
		n++
	}

	return res
}

// GetPositions reutrns the start index of each Lyndon word in the given string
func (this *LyndonWords) GetPositions(s string) []int32 {
	l := this.chenFoxLyndonBreakpoints(s)
	res := make([]int32, l.Len())
	n := 0

	for bp := l.Front(); bp != nil; bp = bp.Next() {
		res[n] = bp.Value.(int32)
		n++
	}

	return res
}
