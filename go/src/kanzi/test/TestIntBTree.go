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

func printNode(node *util.IntBTNode) {
	fmt.Printf("%v, ", node.Value())
}

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

			fmt.Printf("All nodes in reverse order\n")
			tree.Scan(printNode, true)
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
				fmt.Printf("Remove: %v %v ", tMin, tMax)
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
		iter := 10000
		size := 10000
		delta1 := int64(0)
		delta2 := int64(0)
		delta3 := int64(0)
		array := make([]int, size)

		for ii := 0; ii < iter; ii++ {
	   		tree, _ := util.NewIntBTree()
			r := rand.New(rand.NewSource(time.Now().UnixNano()))

			for i := 0; i < size; i++ {
				array[i] = r.Intn(size / 2)
			}

			before1 := time.Now()

			for i := 0; i < size; i++ {
				tree.Add(array[i])
			}

			after1 := time.Now()
			delta1 += after1.Sub(before1).Nanoseconds()

			// Sanity check
			if tree.Size() != size {
				fmt.Printf("Error: Found size=%v, expected size=%v\n", tree.Size(), size)
				os.Exit(1)
			}

			before2 := time.Now()

			for i := 0; i < size; i++ {
				tree.Remove(array[i])
			}

			after2 := time.Now()
			delta2 += after2.Sub(before2).Nanoseconds()

			// Sanity check
			if tree.Size() != 0 {
				fmt.Printf("Error: Found size=%v, expected size=%v\n", tree.Size(), 0)
				os.Exit(1)
			}

			// Recreate a 'size' array tree
			for i := 0; i < size; i++ {
				tree.Add(array[i])
			}

			for i := 0; i < size; i++ {
				val := rand.Intn(size / 2)
				before3 := time.Now()
				tree.Add(val)
				tree.Remove(val)
				after3 := time.Now()
				delta3 += after3.Sub(before3).Nanoseconds()
			}

			// Sanity check
			if tree.Size() != size {
				fmt.Printf("Error: Found size=%v, expected size=%v\n", tree.Size(), size)
				os.Exit(1)
			}
		}

		fmt.Printf("%v iterations\n", size*iter)
		fmt.Printf("Additions [ms]: %d\n", delta1/1000000)
		fmt.Printf("Deletions [ms]: %d\n", delta2/1000000)
		fmt.Printf("Additions/Deletions at size=%v [ms]: %d\n", size, delta3/1000000)
	}
}
