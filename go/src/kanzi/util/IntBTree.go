/*
Copyright 2011-2013 Frederic Langlet
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
	MAX_DIRTY = uint8(1)
	MIN_DIRTY = uint8(2)
)

type IntBTNode struct {
	value int
	count uint
	left  *IntBTNode
	right *IntBTNode
}

func (this *IntBTNode) Value() int {
	return this.value
}

type IntBTree struct {
	root  *IntBTNode
	size  int
	flags uint8
	min   int
	max   int
}

// Visitor pattern. Must return the node value
type IntBTreeCallback func(node *IntBTNode) int

func NewIntBTree() (*IntBTree, error) {
	this := new(IntBTree)
	return this, nil
}

func (this *IntBTree) Size() int {
	return this.size
}

func (this *IntBTree) Add(val int) {
	node := &IntBTNode{value: val, count: 1}

	if this.root == nil {
		this.root = node
		this.min = val
		this.max = val
	} else {
		addNode(this.root, node)

		if val < this.min {
			this.min = val
		}

		if val > this.max {
			this.max = val
		}
	}

	this.size++
}

func addNode(parent, node *IntBTNode) {
	value := node.value

	for value != parent.value {
		if value < parent.value {
			if parent.left == nil {
				parent.left = node
				return
			}

			parent = parent.left
		} else {
			if parent.right == nil {
				parent.right = node
				return
			}

			parent = parent.right
		}
	}

	parent.count++
}

// Return the number of matches
func (this *IntBTree) Contains(value int) uint {
	res := findNode(this.root, value)

	if res == nil {
		return 0
	}

	return res.count
}

func findNode(current *IntBTNode, value int) *IntBTNode {
	if current == nil {
		return nil
	}

	for value != current.value {
		if value < current.value {
			if current.left == nil {
				return nil
			}

			current = current.left
		} else {
			if current.right == nil {
				return nil
			}

			current = current.right
		}
	}

	return current
}

func (this *IntBTree) Remove(value int) bool {
	if this.root == nil {
		return false
	}

	res := this.removeNode(value)

	if res == nil {
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

	for value != current.value {
		if value < current.value {
			if current.left == nil {
				return nil
			}

			prev = current
			current = current.left
		} else {
			if current.right == nil {
				return nil
			}

			prev = current
			current = current.right
		}
	}

	// Found target
	current.count--

	if current.count != 0 {
		return current
	}

	if current == this.root {
		// First, try easy substitutions of root
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

		for i := current.count; i > 0; i-- {
			array[index] = callback(current)
			index++
		}

		if current.right != nil {
			index = scanAndCall(current.right, array, index, callback, false)
		}
	} else {
		if current.right != nil {
			index = scanAndCall(current.right, array, index, callback, true)
		}

		for i := current.count; i > 0; i-- {
			array[index] = callback(current)
			index++
		}

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

		this.min = node.value
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

		this.max = node.value
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
	scanAndCall(this.root, res, 0, emptyCallback, false)
	return res
}

func emptyCallback(node *IntBTNode) int {
	return node.value
}
