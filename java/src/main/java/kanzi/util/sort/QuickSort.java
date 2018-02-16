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

package kanzi.util.sort;

import kanzi.ArrayComparator;
import kanzi.IntSorter;



// Implementation of the Dual-Pivot Quicksort algorithm by
// Vladimir Yaroslavskiy, Jon Bentley and Josh Bloch.
// See http://cr.openjdk.java.net/~alanb/DualPivotSortUpdate/webrev.01/raw_files/
// new/src/java.base/share/classes/java/util/DualPivotQuicksort.java

public class QuickSort implements IntSorter
{
    private static final int HEAP_SORT_THRESHOLD = 69;
    private static final int NANO_INSERTION_SORT_THRESHOLD = 36;
    private static final int PAIR_INSERTION_SORT_THRESHOLD = 88;
    private static final int MERGING_SORT_THRESHOLD = 2048;
    private static final int MAX_RECURSION_DEPTH = 100;
    private static final int LEFTMOST_BITS = MAX_RECURSION_DEPTH << 1;
    
    
   private final ArrayComparator cmp;


   public QuickSort()
   {
      this(null);
   }


   public QuickSort(ArrayComparator cmp)
   {
      this.cmp = cmp;
   }


   protected ArrayComparator getComparator()
   {
      return this.cmp;
   }


   @Override
   public boolean sort(int[] input, int blkptr, int len)
   {
      if ((blkptr < 0) || (len <= 0) || (blkptr+len > input.length))
         return false;

      if (len == 1)
         return true;

      if (this.cmp == null)          
         recursiveSort(input, LEFTMOST_BITS, blkptr, blkptr+len);       
      else
         recursiveSort(input, LEFTMOST_BITS, blkptr, blkptr+len, this.cmp); 
      
      return true;
   }


   private static void recursiveSort(int[] block, int bits, int low, int high) 
   {
      final int end = high - 1;
      final int length = high - low;

      if ((bits & 1) != 0) 
      {
         if (length < NANO_INSERTION_SORT_THRESHOLD) 
         {      
            if (length > 1)
               nanoInsertionSort(block, low, high);
            
            return;
         }

         if (length < PAIR_INSERTION_SORT_THRESHOLD) 
         {
            pairInsertionSort(block, low, end);
            return;
         }
      }

      bits -= 2;
      
      // Switch to heap sort on the leftmost part or
      // if the execution time is becoming quadratic
      if ((length < HEAP_SORT_THRESHOLD) || (bits < 0)) 
      {
         if (length > 1)
            heapSort(block, low, end);
         
         return;
      }

      // Check if the array is nearly sorted
      if (mergingSort(block, low, high))
         return;

      // Splitting step using approximation of the golden ratio
      final int step = (length >> 3) * 3 + 3;

      // Use 5 elements for pivot selection
      final int e1 = low + step;
      final int e5 = end - step;
      final int e3 = (e1 + e5) >>> 1;
      final int e2 = (e1 + e3) >>> 1;
      final int e4 = (e3 + e5) >>> 1;

      // Sort these elements in place by the combination of 5-element 
      // sorting network and insertion sort.
      if (block[e5] < block[e3]) 
         swap(block, e3, e5);

      if (block[e4] < block[e2]) 
         swap(block, e2, e4);

      if (block[e5] < block[e4]) 
         swap(block, e4, e5);

      if (block[e3] < block[e2]) 
         swap(block, e2, e3);

      if (block[e4] < block[e3]) 
          swap(block, e3, e4);

      if (block[e1] > block[e2]) 
      { 
         final int t = block[e1]; 
         block[e1] = block[e2]; 
         block[e2] = t;

         if (t > block[e3]) 
         { 
            block[e2] = block[e3]; 
            block[e3] = t;

            if (t > block[e4]) 
            { 
               block[e3] = block[e4];
               block[e4] = t;

               if (t > block[e5]) 
               { 
                  block[e4] = block[e5];
                  block[e5] = t; 
               }
            }
         }
      }

      // Index of the last element of the left part
      int lower = low; 

      // Index of the first element of the right part
      int upper = end; 

      if ((block[e1] < block[e2]) && (block[e2] < block[e3]) && 
         (block[e3] < block[e4]) && (block[e4] < block[e5])) 
      {
         // Partitioning with two pivots
         // Use the first and the fifth elements as the pivots.             
         final int pivot1 = block[e1];
         final int pivot2 = block[e5];

         // The first and the last elements to be sorted are moved to the
         // locations formerly occupied by the pivots. When partitioning
         // is completed, the pivots are swapped back into their final
         // positions, and excluded from subsequent sorting.
         block[e1] = block[lower];
         block[e5] = block[upper];

         // Skip elements, which are less or greater than the pivots.
         lower++;
         upper--;

         while (block[lower] < pivot1)
            lower++;

         while (block[upper] > pivot2)
            upper--;

         lower--;
         upper++;
         
         for (int k=upper; --k>lower; ) 
         {
            final int ak = block[k];

            if (ak < pivot1) 
            { 
               // Move block[k] to the left side
               while (block[++lower] < pivot1) {}

               if (lower > k) 
               {
                  lower = k;
                  break;
               }

               if (block[lower] > pivot2) 
               { 
                  // block[lower] >= pivot1
                  upper--;
                  block[k] = block[upper];
                  block[upper] = block[lower];
               } 
               else 
               { 
                  // pivot1 <= block[lower] <= pivot2
                  block[k] = block[lower];
               }

               block[lower] = ak;
            } 
            else if (ak > pivot2) 
            { 
               // Move block[k] to the right side
               upper--;
               block[k] = block[upper];
               block[upper] = ak;
            }
         }

         // Swap the pivots back into their final positions
         block[low] = block[lower]; 
         block[lower] = pivot1;
         block[end] = block[upper]; 
         block[upper] = pivot2;

         // Recursion
         recursiveSort(block, bits|1, upper+1, high);
         recursiveSort(block, bits, low, lower);
         recursiveSort(block, bits|1, lower+1, upper);
     } 
     else 
     { 
         // Partitioning with one pivot

         // Use the third element as the pivotas an approximation of the median.
         final int pivot = block[e3];

         // The first element to be sorted is moved to the location
         // formerly occupied by the pivot. When partitioning is
         // completed, the pivot is swapped back into its final
         // position, and excluded from subsequent sorting.
         block[e3] = block[lower];
         upper++;

         for (int k=upper-1; k>lower; k--) 
         {
            if (block[k] == pivot) 
                continue;

            final int ak = block[k];

            if (ak < pivot) 
            { 
               // Move block[k] to the left side
               lower++;
               
               while (block[lower] < pivot)
                  lower++;

               if (lower > k) 
               {
                  lower = k;
                  break;
               }
               
               block[k] = pivot;

               if (block[lower] > pivot) 
               {
                  upper--;
                  block[upper] = block[lower];
               }
               
               block[lower] = ak;
            } 
            else 
            { 
               // Move block[k] to the right side
               block[k] = pivot;
               upper--;
               block[upper] = ak;
            }
         }

         // Swap the pivot into its final position.
         block[low] = block[lower]; 
         block[lower] = pivot;

         // Recursion
         recursiveSort(block, bits|1, upper, high);
         recursiveSort(block, bits, low, lower);
      }   
   }
   
   
   private static void swap(int[] block, int idx0, int idx1)
   {
      final int t = block[idx0];
      block[idx0] = block[idx1];
      block[idx1] = t;
   }
   

   private static void nanoInsertionSort(int[] block, int low, final int high)
   {
      // In the context of Quicksort, the elements from the left part
      // play the role of sentinels. Therefore expensive check of the
      // left range on each iteration can be skipped.
      while (low < high)
      {
         int k = low;
         final int ak = block[k];
         k--;

         while (ak < block[k])
         {
            block[k+1] = block[k];
            k--;
         }

         block[k+1] = ak;
         low++;
      }
   } 
   
   
   private static void pairInsertionSort(int[] block, int left, final int right) 
   {
      // Align left boundary
      left -= ((left ^ right) & 1);

      // Two elements are inserted at once on each iteration.
      // At first, we insert the greater element (a2) and then
      // insert the less element (a1), but from position where
      // the greater element was inserted. In the context of a
      // Dual-Pivot Quicksort, the elements from the left part
      // play the role of sentinels. Therefore expensive check
      // of the left range on each iteration can be skipped.
      left++;
      
      while (left < right) 
      {
         left++;
         int k = left;
         int a1 = block[k];

         if (block[k-2] > block[k-1]) 
         {
            k--;
            int a2 = block[k];

            if (a1 > a2)
            {
               a2 = a1; 
               a1 = block[k];
            }

            k--;
            
            while (a2 < block[k]) 
            {
               block[k+2] = block[k];
               k--;
            }

            k++;
            block[k+1] = a2;
         }

         k--;
         
         while (a1 < block[k]) 
         {
            block[k+1] = block[k];
            k--;
         }
         
         block[k+1] = a1;
         left++;
      }
   }
 
  
   private static void heapSort(int[] block, int left, int right) 
   {
      for (int k=(left+1+right)>>>1; k>left; ) 
      {
         k--;
         pushDown(block, k, block[k], left, right);
      }
      
      for (int k=right; k>left; k--) 
      {
         final int max = block[left];
         pushDown(block, left, block[k], left, k);
         block[k] = max;
      }
   }

  
   private static void pushDown(int[] block, int p, int value, int left, int right)
   {
      while (true)
      {
         int k = (p<<1) - left + 2;

         if ((k > right) || (block[k-1] > block[k])) 
            k--;

         if ((k > right) || (block[k] <= value)) 
         {
            block[p] = value;
            return;
         }
         
         block[p] = block[k];
         p = k;
      }
   }   
   
    
   private static boolean mergingSort(int[] block, int low, int high) 
   {
      final int length = high - low;

      if (length < MERGING_SORT_THRESHOLD)
         return false;

      final int max = (length > 2048000) ? 2000 : (length >> 10) | 5;
      final int[] run = new int[max+1];
      int count = 0;
      int last = low;
      run[0] = low;
      
      // Check if the array is highly structured.
      for (int k=low+1; (k<high) && (count<max); ) 
      {
         if (block[k-1] < block[k]) 
         {
            // Identify ascending sequence
            while (++k < high)
            {
               if (block[k-1] > block[k])
                  break;
            }
         }
         else if (block[k-1] > block[k]) 
         {
            // Identify descending sequence
            while (++k < high)
            {
               if (block[k-1] < block[k])
                  break;
            }

            // Reverse the run into ascending order
            for (int i=last-1, j=k; ((++i < --j) && (block[i] > block[j])); ) 
               swap(block, i, j);
         } 
         else 
         { 
            // Sequence with equal elements
            final int ak = block[k]; 
            
            while (++k < high)
            {
               if (ak != block[k])
                  break;
            }

            if (k < high)
               continue;
         }

         if ((count == 0) || (block[last-1] > block[last]))
            count++;

         last = k;
         run[count] = k;
      }

      // The array is highly structured => merge all runs
      if ((count < max) && (count > 1)) 
         merge(block, new int[length], true, low, run, 0, count);
      
      return count < max;
   }

    
   private static int[] merge(int[] block1, int[] block2, boolean isSource,
            int offset, int[] run, int lo, int hi) 
   {
      if (hi - lo == 1)
      {
         if (isSource == true) 
            return block1;
      
         for (int i=run[hi], j=i-offset, low=run[lo]; i>low; i--, j--)
            block2[j] = block1[i];
          
         return block2;
      }
      
      final int mi = (lo + hi) >>> 1;
      final int[] a1 = merge(block1, block2, !isSource, offset, run, lo, mi);
      final int[] a2 = merge(block1, block2, true, offset, run, mi, hi);
      
      return merge((a1==block1) ? block2 : block1,
                   (a1==block1) ? run[lo]-offset : run[lo],
                    a1,
                   (a1==block2) ? run[lo]-offset : run[lo],
                   (a1==block2) ? run[mi]-offset : run[mi],
                    a2,
                   (a2==block2) ? run[mi]-offset : run[mi],
                   (a2==block2) ? run[hi]-offset : run[hi]);
   }
   
   
   private static int[] merge(int[] dst, int k,
            int[] block1, int i, int hi, int[] block2, int j, int hj) 
   {
      while (true) 
      {
         dst[k++] = (block1[i] < block2[j]) ? block1[i++] : block2[j++];

         if (i == hi) 
         {
            while (j < hj)
               dst[k++] = block2[j++];

            return dst;
         }
         
         if (j == hj) 
         {
            while (i < hi)
               dst[k++] = block1[i++];

            return dst;
         }
      }
   }   
   
   
   
   private static void recursiveSort(int[] block, int bits, int low, int high, ArrayComparator cmp) 
   {
      final int end = high - 1;
      final int length = high - low;

      if ((bits & 1) != 0) 
      {      
         if (length < NANO_INSERTION_SORT_THRESHOLD) 
         {
            if (length > 1)
               nanoInsertionSort(block, low, high, cmp);
            
            return;
         }

         if (length < PAIR_INSERTION_SORT_THRESHOLD) 
         {
            pairInsertionSort(block, low, end, cmp);
            return;
         }
      }

      bits -= 2;
      
      // Switch to heap sort on the leftmost part or
      // if the execution time is becoming quadratic
      if ((length < HEAP_SORT_THRESHOLD) || (bits < 0)) 
      {
         if (length > 1)
            heapSort(block, low, end, cmp);
         
         return;
      }

      // Check if the array is nearly sorted
      if (mergingSort(block, low, high, cmp))
         return;

      // Splitting step using approximation of the golden ratio
      final int step = (length >> 3) * 3 + 3;

      // Use 5 elements for pivot selection
      final int e1 = low + step;
      final int e5 = end - step;
      final int e3 = (e1 + e5) >>> 1;
      final int e2 = (e1 + e3) >>> 1;
      final int e4 = (e3 + e5) >>> 1;

      // Sort these elements in place by the combination of 5-element 
      // sorting network and insertion sort.
      if (cmp.compare(block[e5], block[e3]) < 0) 
         swap(block, e3, e5);

      if (cmp.compare(block[e4], block[e2]) < 0) 
         swap(block, e2, e4);

      if (cmp.compare(block[e5], block[e4]) < 0) 
         swap(block, e4, e5);

      if (cmp.compare(block[e3], block[e2]) < 0) 
         swap(block, e2, e3);

      if (cmp.compare(block[e4], block[e3]) < 0) 
          swap(block, e3, e4);

      if (cmp.compare(block[e1], block[e2]) > 0) 
      { 
         final int t = block[e1]; 
         block[e1] = block[e2]; 
         block[e2] = t;

         if (cmp.compare(t, block[e3]) > 0)  
         { 
            block[e2] = block[e3]; 
            block[e3] = t;

            if (cmp.compare(t, block[e4]) > 0)  
            { 
               block[e3] = block[e4];
               block[e4] = t;

               if (cmp.compare(t, block[e5]) > 0)  
               { 
                  block[e4] = block[e5];
                  block[e5] = t; 
               }
            }
         }
      }

      // Index of the last element of the left part
      int lower = low; 

      // Index of the first element of the right part
      int upper = end; 

      if ((cmp.compare(block[e1], block[e2]) < 0) && (cmp.compare(block[e2], block[e3]) < 0) &&
          (cmp.compare(block[e3], block[e4]) < 0)  && (cmp.compare(block[e4], block[e5]) < 0))
      {
         // Partitioning with two pivots
         // Use the first and the fifth elements as the pivots.             
         final int pivot1 = block[e1];
         final int pivot2 = block[e5];

         // The first and the last elements to be sorted are moved to the
         // locations formerly occupied by the pivots. When partitioning
         // is completed, the pivots are swapped back into their final
         // positions, and excluded from subsequent sorting.
         block[e1] = block[lower];
         block[e5] = block[upper];

         // Skip elements, which are less or greater than the pivots.
         lower++;
         upper--;

         while (cmp.compare(block[lower], pivot1) < 0)  
            lower++;

         while (cmp.compare(block[upper], pivot2) > 0)  
            upper--;

         lower--;
         upper++;
         
         for (int k=upper; --k>lower; ) 
         {
            final int ak = block[k];

            if (cmp.compare(ak, pivot1) < 0)
            { 
               // Move block[k] to the left side
               while (cmp.compare(block[++lower], pivot1) < 0) {}

               if (lower > k) 
               {
                  lower = k;
                  break;
               }

               if (cmp.compare(block[lower], pivot2) > 0)
               { 
                  // block[lower] >= pivot1
                  upper--;
                  block[k] = block[upper];
                  block[upper] = block[lower];
               } 
               else 
               { 
                  // pivot1 <= block[lower] <= pivot2
                  block[k] = block[lower];
               }

               block[lower] = ak;
            } 
            else if (cmp.compare(ak, pivot2) > 0)
            { 
               // Move block[k] to the right side
               upper--;
               block[k] = block[upper];
               block[upper] = ak;
            }
         }

         // Swap the pivots back into their final positions
         block[low] = block[lower]; 
         block[lower] = pivot1;
         block[end] = block[upper]; 
         block[upper] = pivot2;

         // Recursion
         recursiveSort(block, bits|1, upper+1, high, cmp);
         recursiveSort(block, bits, low, lower, cmp);
         recursiveSort(block, bits|1, lower+1, upper, cmp);
     } 
     else 
     { 
         // Partitioning with one pivot

         // Use the third element as the pivotas an approximation of the median.
         final int pivot = block[e3];

         // The first element to be sorted is moved to the location
         // formerly occupied by the pivot. When partitioning is
         // completed, the pivot is swapped back into its final
         // position, and excluded from subsequent sorting.
         block[e3] = block[lower];
         upper++;

         for (int k=upper-1; k>lower; k--) 
         {
            if (cmp.compare(block[k], pivot) == 0)
                continue;

            final int ak = block[k];

            if (cmp.compare(ak, pivot) < 0)
            { 
               // Move block[k] to the left side
               lower++;
               
               while (cmp.compare(block[lower], pivot) < 0)
                  lower++;

               if (lower > k) 
               {
                  lower = k;
                  break;
               }
               
               block[k] = pivot;

               if (cmp.compare(block[lower], pivot) > 0)
               {
                  upper--;
                  block[upper] = block[lower];
               }
               
               block[lower] = ak;
            } 
            else 
            { 
               // Move block[k] to the right side
               block[k] = pivot;
               upper--;
               block[upper] = ak;
            }
         }

         // Swap the pivot into its final position.
         block[low] = block[lower]; 
         block[lower] = pivot;

         // Recursion
         recursiveSort(block, bits|1, upper, high, cmp);
         recursiveSort(block, bits, low, lower, cmp);
      }   
   }
   
   private static void nanoInsertionSort(int[] block, int low, final int high, ArrayComparator cmp)
   {
      // In the context of Quicksort, the elements from the left part
      // play the role of sentinels. Therefore expensive check of the
      // left range on each iteration can be skipped.
      while (low < high)
      {
         int k = low;
         final int ak = block[k];
         k--;

         while (cmp.compare(ak, block[k]) < 0)
         {
            block[k+1] = block[k];
            k--;
         }

         block[k+1] = ak;
         low++;
      }
   } 
   
   
   private static void pairInsertionSort(int[] block, int left, final int right, ArrayComparator cmp) 
   {
      // Align left boundary
      left -= ((left ^ right) & 1);

      // Two elements are inserted at once on each iteration.
      // At first, we insert the greater element (a2) and then
      // insert the less element (a1), but from position where
      // the greater element was inserted. In the context of a
      // Dual-Pivot Quicksort, the elements from the left part
      // play the role of sentinels. Therefore expensive check
      // of the left range on each iteration can be skipped.
      left++;
      
      while (left < right) 
      {
         left++;
         int k = left;
         int a1 = block[k];

         if (cmp.compare(block[k-2], block[k-1]) > 0)
         {
            k--;
            int a2 = block[k];

            if (cmp.compare(a1, a2) > 0)
            {
               a2 = a1; 
               a1 = block[k];
            }

            k--;
            
            while (cmp.compare(a2, block[k]) < 0)
            {
               block[k+2] = block[k];
               k--;
            }

            k++;
            block[k+1] = a2;
         }

         k--;
         
         while (cmp.compare(a1, block[k]) < 0)
         {
            block[k+1] = block[k];
            k--;
         }
         
         block[k+1] = a1;
         left++;
      }
   } 
   
   
   private static void heapSort(int[] block, int left, int right, ArrayComparator cmp) 
   {
      for (int k=(left+1+right)>>>1; k>left; ) 
      {
         k--;
         pushDown(block, k, block[k], left, right, cmp);
      }
      
      for (int k=right; k>left; k--) 
      {
         final int max = block[left];
         pushDown(block, left, block[k], left, k, cmp);
         block[k] = max;
      }
   }

  
   private static void pushDown(int[] block, int p, int value, int left, int right,
      ArrayComparator cmp)
   {
      while (true)
      {
         int k = (p<<1) - left + 2;

         if ((k > right) || (cmp.compare(block[k-1], block[k]) > 0)) 
            k--;

         if ((k > right) || (cmp.compare(block[k], value) <= 0))
         {
            block[p] = value;
            return;
         }
         
         block[p] = block[k];
         p = k;
      }
   }   
   
    
   private static boolean mergingSort(int[] block, int low, int high, ArrayComparator cmp) 
   {
      final int length = high - low;

      if (length < MERGING_SORT_THRESHOLD)
         return false;

      final int max = (length > 2048000) ? 2000 : (length >> 10) | 5;
      final int[] run = new int[max+1];
      int count = 0;
      int last = low;
      run[0] = low;
      
      // Check if the array is highly structured.
      for (int k=low+1; (k<high) && (count<max); ) 
      {
         if (cmp.compare(block[k-1], block[k]) < 0)
         {
            // Identify ascending sequence
            while (++k < high)
            {
               if (cmp.compare(block[k-1], block[k]) > 0)
                  break;
            }
         }
         else if (cmp.compare(block[k-1], block[k]) > 0)
         {
            // Identify descending sequence
            while (++k < high)
            {
               if (cmp.compare(block[k-1], block[k]) < 0)
                  break;
            }

            // Reverse the run into ascending order
            for (int i=last-1, j=k; ((++i < --j) && (cmp.compare(block[i], block[j]) > 0)); ) 
               swap(block, i, j);
         } 
         else 
         { 
            // Sequence with equal elements
            final int ak = block[k]; 
            
            while (++k < high)
            {
               if (cmp.compare(ak, block[k]) != 0)
                  break;               
            }

            if (k < high)
               continue;
         }

         if ((count == 0) || (cmp.compare(block[last-1], block[last]) > 0))
            count++;

         last = k;
         run[count] = k;
      }

      // The array is highly structured => merge all runs
      if ((count < max) && (count > 1)) 
         merge(block, new int[length], true, low, run, 0, count, cmp);
      
      return count < max;
   }

    
   private static int[] merge(int[] block1, int[] block2, boolean isSource,
            int offset, int[] run, int lo, int hi, ArrayComparator cmp) 
   {
      if (hi - lo == 1)
      {
         if (isSource == true) 
            return block1;
      
         for (int i=run[hi], j=i-offset, low=run[lo]; i>low; i--, j--)
            block2[j] = block1[i];
          
         return block2;
      }
      
      final int mi = (lo + hi) >>> 1;
      final int[] a1 = merge(block1, block2, !isSource, offset, run, lo, mi, cmp);
      final int[] a2 = merge(block1, block2, true, offset, run, mi, hi, cmp);
      
      return merge((a1==block1) ? block2 : block1,
                   (a1==block1) ? run[lo]-offset : run[lo],
                    a1,
                   (a1==block2) ? run[lo]-offset : run[lo],
                   (a1==block2) ? run[mi]-offset : run[mi],
                    a2,
                   (a2==block2) ? run[mi]-offset : run[mi],
                   (a2==block2) ? run[hi]-offset : run[hi],
                   cmp);
   }
   
   
   private static int[] merge(int[] dst, int k,
            int[] block1, int i, int hi, int[] block2, int j, int hj, ArrayComparator cmp)  
   {
      while (true) 
      {
         dst[k++] = (cmp.compare(block1[i], block2[j]) < 0) ? block1[i++] : block2[j++];

         if (i == hi) 
         {
            while (j < hj)
               dst[k++] = block2[j++];

            return dst;
         }
         
         if (j == hj) 
         {
            while (i < hi)
               dst[k++] = block1[i++];

            return dst;
         }
      }
   }   
     
}
