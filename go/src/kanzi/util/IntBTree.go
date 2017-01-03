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

package util

import (
	"errors"
)

// A tree based collection of sorted integers allowing log n time for add/remove
// and fast access to minimum and maximum values (usually constant time, else log n time).

const (
	MAX_DIRTY            = uint8(1)
	MIN_DIRTY            = uint8(2)
	LOG_NODE_BUFFER_SIZE = 4
	NODE_BUFFER_SIZE     = 1 << LOG_NODE_BUFFER_SIZE
	MASK_NODE_BUFFER     = NODE_BUFFER_SIZE - 1
)

type IntBTNode struct {
	base   int // range base
	counts []uint
	left   *IntBTNode
	right  *IntBTNode
}

func (this *IntBTNode) Values(values []int, idx int, reverse bool) int {
	if reverse == true {
		for i := NODE_BUFFER_SIZE - 1; i >= 0; i-- {
			for j := this.counts[i]; j > 0; j-- {
				values[idx] = this.base + i
				idx++
			}
		}
	} else {
		for i := range this.counts {
			for j := this.counts[i]; j > 0; j-- {
				values[idx] = this.base + i
				idx++
			}
		}
	}

	return idx
}

func newIntBTNode(val int) *IntBTNode {
	this := new(IntBTNode)
	this.base = val & -NODE_BUFFER_SIZE
	this.counts = make([]uint, NODE_BUFFER_SIZE)
	this.counts[val&MASK_NODE_BUFFER]++
	return this
}

type IntBTree struct {
	root  *IntBTNode
	size  int
	flags uint8
	min   int
	max   int
}

// Visitor pattern. Must return the node value
type IntBTreeCallback func(node *IntBTNode, values []int, idx int, reverse bool) int

func NewIntBTree() (*IntBTree, error) {
	this := new(IntBTree)
	return this, nil
}

func (this *IntBTree) Size() int {
	return this.size
}

func (this *IntBTree) Add(val int) {
	if this.root == nil {
		this.root = newIntBTNode(val)
		this.min = val
		this.max = val
	} else {
		addValue(this.root, val)

		if val < this.min {
			this.min = val
		}

		if val > this.max {
			this.max = val
		}
	}

	this.size++
}

// Add existing node
func addNode(parent, node *IntBTNode) {
	value := node.base

	for true {
		if value < parent.base {
			if parent.left == nil {
				parent.left = node
				return
			}

			parent = parent.left
		} else if value >= parent.base+NODE_BUFFER_SIZE {
			if parent.right == nil {
				parent.right = node
				return
			}

			parent = parent.right
		} else {
			break
		}
	}

	parent.counts[value&MASK_NODE_BUFFER]++
}

func addValue(parent *IntBTNode, value int) {
	for parent != nil {
		if value < parent.base {
			if parent.left == nil {
				parent.left = newIntBTNode(value)
				break
			}

			parent = parent.left
		} else if value >= parent.base+NODE_BUFFER_SIZE {
			if parent.right == nil {
				parent.right = newIntBTNode(value)
				break
			}

			parent = parent.right
		} else {
			parent.counts[value&MASK_NODE_BUFFER]++
			break
		}
	}
}

// Return the number of matches
func (this *IntBTree) Contains(value int) uint {
	res := findNode(this.root, value)

	if res == nil {
		return 0
	}

	return res.counts[value&MASK_NODE_BUFFER]
}

func findNode(current *IntBTNode, value int) *IntBTNode {

	for current != nil {
		if value < current.base {
			current = current.left
		} else if value >= current.base+NODE_BUFFER_SIZE {
			current = current.right
		} else {
			break
		}
	}

	return current
}

func (this *IntBTree) Remove(value int) bool {
	if this.root == nil {
		return false
	}

	if this.removeNode(value) == nil {
		return false
	}

	this.size--

	// Force recomputation of cached fields
	if this.min == value {
		this.flags |= MIN_DIRTY
	}

	if this.max == value {
		this.flags |= MAX_DIRTY
	}

	return true
}

func (this *IntBTree) removeNode(value int) *IntBTNode {
	current := this.root
	var prev *IntBTNode
	prev = nil

	for true {
		if value < current.base {
			if current.left == nil {
				return nil
			}

			prev = current
			current = current.left
		} else if value >= current.base+NODE_BUFFER_SIZE {
			if current.right == nil {
				return nil
			}

			prev = current
			current = current.right
		} else {
			break
		}
	}

	// Found target
	current.counts[value&MASK_NODE_BUFFER]--

	if current.counts[value&MASK_NODE_BUFFER] != 0 {
		return current
	}

	for i := 0; i < NODE_BUFFER_SIZE; i++ {
		if current.counts[i] != 0 {
			return current
		}
	}

	if current == this.root {
		// First, try cheap substitutions of root
		if current.right == nil {
			this.root = current.left
		} else if current.left == nil {
			this.root = current.right
		} else if value&1 == 0 { // random choice or left or right
			this.root = current.right

			// Re-insert left branch
			addNode(this.root, current.left)
		} else {
			this.root = current.left

			// Re-insert right branch
			addNode(this.root, current.right)
		}
	}

	if prev != nil {
		// Remove current node from previous node
		if prev.right == current {
			if current.left != nil {
				prev.right = current.left

				if current.right != nil {
					// Re-insert right branch
					addNode(prev, current.right)
				}
			} else {
				prev.right = current.right
			}
		} else {
			prev.left = current.left

			if current.right != nil {
				// Re-insert right branch
				addNode(prev, current.right)
			}
		}
	}

	return current
}

func (this *IntBTree) Rank(value int) int {
	if this.root == nil {
		return -1
	}

	if val, _ :=this.Min(); val == value {
		return 0
	}

	rank := findRank(this.root, value, 0)

	if rank == this.size {
		return -1
	} 
		
	return -rank
}

func findRank(current *IntBTNode, value, rank int) int {
	if rank >= 0 && current.left != nil {
		rank = findRank(current.left, value, rank)
	}

	for i := 0; i < NODE_BUFFER_SIZE; i++ {
		if value == current.base+i {
			return -rank
		}

		if rank >= 0 {
			rank += int(current.counts[i])
		}
	}

	if rank >= 0 && current.right != nil {
		rank = findRank(current.right, value, rank)
	}

	return rank
}

func (this *IntBTree) Scan(callback IntBTreeCallback, reverse bool) []int {
	if callback == nil || this.root == nil {
		return nil
	}

	res := make([]int, this.size)
	scanAndCall(this.root, res, 0, callback, reverse) // visitor pattern
	return res
}

func scanAndCall(current *IntBTNode, array []int, index int, callback IntBTreeCallback, reverse bool) int {
	if reverse == false {
		if current.left != nil {
			index = scanAndCall(current.left, array, index, callback, false)
		}

		index = callback(current, array, index, false)

		if current.right != nil {
			index = scanAndCall(current.right, array, index, callback, false)
		}
	} else {
		if current.right != nil {
			index = scanAndCall(current.right, array, index, callback, true)
		}

		index = callback(current, array, index, true)

		if current.left != nil {
			index = scanAndCall(current.left, array, index, callback, true)
		}
	}

	return index
}

func (this *IntBTree) Clear() {
	this.root = nil
	this.size = 0
}

func (this *IntBTree) Min() (int, error) {
	if this.root == nil {
		return 0, errors.New("Tree is empty")
	}

	if this.flags&MIN_DIRTY != 0 {
		// Dynamically scan tree to leftmost position
		node := this.root

		for node.left != nil {
			node = node.left
		}

		for i := 0; i < NODE_BUFFER_SIZE; i++ {
			if node.counts[i] > 0 {
				this.min = node.base + i
				break
			}
		}

		this.flags &= ^MIN_DIRTY
	}

	return this.min, nil
}

func (this *IntBTree) Max() (int, error) {
	if this.root == nil {
		return 0, errors.New("Tree is empty")
	}

	if this.flags&MAX_DIRTY != 0 {
		// Dynamically scan tree to rightmost position
		node := this.root

		for node.right != nil {
			node = node.right
		}

		for i := NODE_BUFFER_SIZE - 1; i >= 0; i-- {
			if node.counts[i] > 0 {
				this.max = node.base + i
				break
			}
		}

		this.flags &= ^MAX_DIRTY
	}

	return this.max, nil
}

func (this *IntBTree) ToArray(array []int) []int {
	if this.root == nil {
		return make([]int, 0)
	}

	if array == nil || len(array) < this.size {
		array = make([]int, this.size)
	}

	res := array
	scanAndCall(this.root, res, 0, defaultCallback, false)
	return res
}

func defaultCallback(node *IntBTNode, values []int, index int, reverse bool) int {
	return node.Values(values, index, reverse)
}
