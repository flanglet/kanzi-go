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

package main

import (
	"fmt"
	"kanzi/util"
	"math/rand"
	"os"
	"time"
)


func main() {
	fmt.Printf("Correctness Test\n")

	{
		tree, _ := util.NewIntBTree()
		r := rand.New(rand.NewSource(time.Now().UnixNano()))

		for ii := 0; ii < 5; ii++ {
			fmt.Printf("\nIteration %v\n", ii)
			max := 0
			min := 1 << 31

			for i := 0; i < 30; i++ {
				val := 64 + r.Intn(5*i+20)

				if val < min {
					min = val
				}

				if val > max {
					max = val
				}

				tree.Add(val)
				tMin, err1 := tree.Min()
				tMax, err2 := tree.Max()

				if err1 != nil {
					fmt.Printf("Error: %v", err1)
					os.Exit(1)
				}

				if err2 != nil {
					fmt.Printf("Error: %v", err2)
					os.Exit(1)
				}

				fmt.Printf("Add: %v\n", val)
				fmt.Printf("Min/max: %v %v\n", tMin, tMax)
				fmt.Printf("Size: %v\n", tree.Size())

				if tMin != min {
					fmt.Printf("Error: Found min=%v, expected min=%v\n", tMin, min)
					os.Exit(1)
				}

				if tMax != max {
					fmt.Printf("Error: Found max=%v, expected max=%v\n", tMax, max)
					os.Exit(1)
				}
			}

			//	fmt.Printf("All nodes in reverse order:\n")
			//	tree.Scan(printNode, true)
			println()
			fmt.Printf("All nodes in natural order\n")
			array := tree.ToArray(make([]int, tree.Size()))

			for i := range array {
				fmt.Printf("%v, ", array[i])
			}

			println()
			fmt.Printf("Size: %v\n", tree.Size())

			for tree.Size() > 0 {
				tMin, err1 := tree.Min()
				tMax, err2 := tree.Max()

				if err1 != nil {
					fmt.Printf("Error: %v\n", err1)
					os.Exit(1)
				}

				if err2 != nil {
					fmt.Printf("Error: %v\n", err2)
					os.Exit(1)
				}

				tree.Remove(tMin)
				tree.Remove(tMax)
				fmt.Printf("Remove: %v %v\n", tMin, tMax)
				array = tree.ToArray(make([]int, tree.Size()))

				for i := range array {
					fmt.Printf("%v, ", array[i])
				}

				println()
				fmt.Printf("Size: %v\n", tree.Size())

				if tree.Size() > 0 {
					tMin, err1 = tree.Min()
					tMax, err2 = tree.Max()

					if err1 != nil {
						fmt.Printf("Error: %v\n", err1)
						os.Exit(1)
					}

					if err2 != nil {
						fmt.Printf("Error: %v\n", err2)
						os.Exit(1)
					}

					fmt.Printf("Min/max: %v %v\n", tMin, tMax)
				}
			}

			fmt.Printf("Success\n\n")
		}
	}

	fmt.Printf("Speed Test\n")

	{
		iter := 5000
		size := 10000
		var before, after time.Time
		delta01 := int64(0)
		delta02 := int64(0)
		delta03 := int64(0)
		delta04 := int64(0)
		array := make([]int, size)

		for ii := 0; ii < iter; ii++ {
			tree1, _ := util.NewIntBTree()
			array[0] = 100000
			r := rand.New(rand.NewSource(time.Now().UnixNano()))

			for i := 0; i < size; i++ {
				array[i] = r.Intn(size / 2)
			}

			before = time.Now()

			for i := 0; i < size; i++ {
				tree1.Add(array[i])
			}

			after = time.Now()
			delta01 += after.Sub(before).Nanoseconds()
			before = time.Now()

			for i := 0; i < size; i++ {
				tree1.Contains(array[size-1-i])
			}

			after = time.Now()
			delta04 += after.Sub(before).Nanoseconds()

			// Sanity check
			if tree1.Size() != size {
				fmt.Printf("Error: Found size=%v, expected size=%v\n", tree1.Size(), size)
				os.Exit(1)
			}

			before = time.Now()

			for i := 0; i < size; i++ {
				tree1.Remove(array[i])
			}

			after := time.Now()
			delta02 += after.Sub(before).Nanoseconds()

			// Sanity check
			if tree1.Size() != 0 {
				fmt.Printf("Error: Found size=%v, expected size=%v\n", tree1.Size(), 0)
				os.Exit(1)
			}

			// Recreate a 'size' array tree
			for i := 0; i < size; i++ {
				tree1.Add(array[i])
			}

			for i := 0; i < size; i++ {
				val := rand.Intn(size / 2)
				before = time.Now()
				tree1.Add(val)
				tree1.Remove(val)
				after = time.Now()
				delta03 += after.Sub(before).Nanoseconds()
			}

			// Sanity check
			if tree1.Size() != size {
				fmt.Printf("Error: Found size=%v, expected size=%v\n", tree1.Size(), size)
				os.Exit(1)
			}
		}

		fmt.Printf("%v iterations\n", size*iter)
		fmt.Printf("Additions [ms]: %d\n", delta01/1000000)
		fmt.Printf("Deletions [ms]: %d\n", delta02/1000000)
		fmt.Printf("Contains  [ms]: %d\n", delta04/1000000)
		fmt.Printf("Additions/Deletions at size=%v [ms]: %d\n", size, delta03/1000000)
	}
}
