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

import kanzi.IntSorter;


// A MergeSort is conceptually very simple (divide and merge) but usually not
// very performant except for almost sorted data
public class MergeSort implements IntSorter
{
   private static final int SMALL_ARRAY_THRESHOLD = 32;
   
   private int[] buffer;
   private final IntSorter insertionSort;


   public MergeSort()
   {
      this.buffer = new int[0];
      this.insertionSort = new InsertionSort();
   }

   @Override
   public boolean sort(int[] data, int start, int count)
   {
      if ((data == null) || (count < 0) || (start < 0))
         return false;
      
      if (start+count > data.length)
         return false;
      
      if (count < 2)
         return true;
 
      if (this.buffer.length < count)
          this.buffer = new int[count];
                 
      return this.mergesort(data, 0, count-1);
   }


   private boolean mergesort(int[] data, int low, int high)
   {
      if (low < high)
      {
         int count = high - low + 1;
         
         // Insertion sort on smallest arrays
         if (count < SMALL_ARRAY_THRESHOLD)
            return this.insertionSort.sort(data, low, count);
         
         int middle = low + count / 2;
         this.mergesort(data, low, middle);
         this.mergesort(data, middle + 1, high);
         this.merge(data, low, middle, high);
      }
      
      return true;
   }


   private void merge(int[] data, int low, int middle, int high)
   {
      int count = high - low + 1;
      
      if (count < 16)
      {
         for (int ii=low; ii<=high; ii++)
            this.buffer[ii] = data[ii];
      }
      else
      {
         System.arraycopy(data, low, this.buffer, low, count);
      }
      
      int i = low;
      int j = middle + 1;
      int k = low;
               
      while ((i <= middle) && (j <= high))
      {
         if (this.buffer[i] <= this.buffer[j])
            data[k] = this.buffer[i++];
         else
            data[k] = this.buffer[j++];

         k++;
      }
      
      count = middle - i + 1;
      
      if (count < 16)
      {
         while (i <= middle)
            data[k++] = this.buffer[i++];
      }
      else
      {
         System.arraycopy(this.buffer, i, data, k, count);
      }
   }
}

