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
// and fast access to minimum and maximum values (almost always constant time).

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

func NewIntBTree() (*IntBTree, error) {
	this := new(IntBTree)
	this.flags = MIN_DIRTY | MAX_DIRTY
	return this, nil
}

func (this *IntBTree) Size() int {
	return this.size
}

func (this *IntBTree) Add(val int) {
	node := &IntBTNode{value: val, count: 1}

	if this.root == nil {
		this.root = node
	} else {
		this.scanAndAdd(this.root, node)
	}

	if val < this.min {
		this.min = val
	}

	if val > this.max {
		this.max = val
	}

	this.size++
}

func (this *IntBTree) scanAndAdd(parent, node *IntBTNode) {
	if parent == nil {
		return
	}

	if node.value < parent.value {
		if parent.left == nil {
			parent.left = node
		} else {
			this.scanAndAdd(parent.left, node)
		}
	} else if node.value > parent.value {
		if parent.right == nil {
			parent.right = node
		} else {
			this.scanAndAdd(parent.right, node)
		}
	} else {
		parent.count++
	}

}

func (this *IntBTree) Remove(value int) *IntBTNode {
	if this.root == nil {
		return nil
	}

	res := this.scanAndRemove(this.root, nil, value, true)

	if res == this.root && this.root.count == 0 {
		if res.right != nil {
			this.root = res.right
		} else if res.left != nil {
			this.root = res.left
		} else {
			this.root = nil
		}
	}

	if res != nil {
		if this.root == nil {
			this.size = 0
		} else {
			this.size--
		}

		// Force recomputation of cached fields
		if this.min == value {
			this.flags |= MIN_DIRTY
		}

		if this.max == value {
			this.flags |= MAX_DIRTY
		}

	}

	return res
}

func (this *IntBTree) scanAndRemove(current, prev *IntBTNode, value int, right bool) *IntBTNode {
	if current == nil {
		return nil
	}

	if value < current.value {
		if current.left == nil {
			return nil
		} else {
			return this.scanAndRemove(current.left, current, value, false)
		}
	} else if value > current.value {
		if current.right == nil {
			return nil
		} else {
			return this.scanAndRemove(current.right, current, value, true)
		}
	}

	current.count--

	if current.count == 0 && prev != nil {
		if right {
			if current.left != nil {
				prev.right = current.left

				if current.right != nil {
					// Re-insert right branch
					this.scanAndAdd(this.root, current.right)
				}
			} else {
				prev.right = current.right
			}
		} else {
			prev.left = current.left

			if current.right != nil {
				// Re-insert right branch
				this.scanAndAdd(this.root, current.right)
			}
		}
	}

	return current
}

func (this *IntBTree) Scan(callback func(node *IntBTNode), reverse bool) {
	if callback == nil {
		return
	}

	scanAndCall(this.root, callback, reverse) // visitor pattern
}

func scanAndCall(current *IntBTNode, callback func(node *IntBTNode), reverse bool) {
	if current == nil {
		return
	}

	if reverse == false {
		if current.left != nil {
			scanAndCall(current.left, callback, reverse)
		}

		for i := current.count; i > 0; i-- {
			callback(current)
		}

		if current.right != nil {
			scanAndCall(current.right, callback, reverse)
		}
	} else {
		if current.right != nil {
			scanAndCall(current.right, callback, reverse)
		}

		for i := current.count; i > 0; i-- {
			callback(current)
		}

		if current.left != nil {
			scanAndCall(current.left, callback, reverse)
		}
	}
}

func (this *IntBTree) Min() (int, error) {
	if this.root == nil {
		return 0, errors.New("Tree is empty")
	}

	if this.flags&MIN_DIRTY == 0 {
		return this.min, nil
	}

	// Dynamically scan tree to leftmost position
	node := this.root
	minimum := node.value

	for node != nil {
		minimum = node.value
		node = node.left
	}

	this.min = minimum
	this.flags &= ^MIN_DIRTY
	return minimum, nil
}

func (this *IntBTree) Max() (int, error) {
	if this.root == nil {
		return 0, errors.New("Tree is empty")
	}

	if this.flags&MAX_DIRTY == 0 {
		return this.max, nil
	}

	// Dynamically scan tree to rightmost position
	node := this.root
	maximum := node.value

	for node != nil {
		maximum = node.value
		node = node.right
	}

	this.max = maximum
	this.flags &= ^MAX_DIRTY
	return maximum, nil
}
