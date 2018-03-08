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

import java.util.Arrays;
import kanzi.Global;
import kanzi.IntSorter;
import kanzi.SliceIntArray;

// Implementation of SpreadSort
// See https://en.wikipedia.org/wiki/Spreadsort
// See [The Spreadsort High-performance General-case Sorting Algorithm] by Steven Ross
public class SpreadSort implements IntSorter
{
   private static final int MAX_SPLITS = 11;
   private static final int LOG_MEAN_BIN_SIZE = 2;
   private static final int LOG_MIN_SPLIT_COUNT = 9;
   private static final int LOG_CONST = 4;

   
   @Override
   public boolean sort(int[] array, int idx, int count)
   {
      return _sort(array, idx, count);
   }
   
   
   private static boolean _sort(int[] array, int idx, int count)
   {
      if (count < 2)
         return true;		
         
      // Array containing min, max and bin count
      final int[] minMaxCount = new int[3];
      SliceIntArray sia = new SliceIntArray(array, idx);
      final Bin[] bins = spreadSortCore(sia, count, minMaxCount);
         
      if (bins == null)
         return false;
         
      final int maxCount = getMaxCount(roughLog2(minMaxCount[1]-minMaxCount[0]), count);
      spreadSortBins(sia, minMaxCount, bins, maxCount);      
      return true;
   }
   
   
   private static int roughLog2(int x) 
   {
      return Global.log2(x);
   }
   
   
   private static int getMaxCount(int logRange, int count)
   {
      int logSize = roughLog2(count);
      
      if (logSize > MAX_SPLITS) 
         logSize = MAX_SPLITS;
      
      int relativeWidth = (LOG_CONST*logRange) / logSize;
      
      // Don't try to bitshift more than the size of an element
      if (relativeWidth >= 4)
         relativeWidth = 3;
      
      final int shift = (relativeWidth < LOG_MEAN_BIN_SIZE+LOG_MIN_SPLIT_COUNT) ? 
         LOG_MEAN_BIN_SIZE + LOG_MIN_SPLIT_COUNT : relativeWidth;
      
      return 1 << shift;
   }

   
   private static void findExtremes(SliceIntArray sia, int count, int[] minMax)
   {
      final int[] input = sia.array;
      final int end = sia.index + count;
      int min = input[sia.index];
      int max = min;
      
      for (int i=sia.index; i<end; i++) 
      {
         final int val = input[i];
         
         if (val > max)
            max = val;
         else if (val < min)
            min = val;
      }
      
      minMax[0] = min;
      minMax[1] = max;
   }	


   private static Bin[] spreadSortCore(SliceIntArray sia, int count, int[] minMaxCount)
   {
      // This step is roughly 10% of runtime but it helps avoid worst-case
      // behavior and improves behavior with real data.  If you know the
      // maximum and minimum ahead of time, you can pass those values in
      // and skip this step for the first iteration
      findExtremes(sia, count, minMaxCount);
      final int max = minMaxCount[1];
      final int min = minMaxCount[0];

      if (max == min)
         return null;

      final int logRange = roughLog2(max-min);
      int logDivisor = logRange - roughLog2(count) + LOG_MEAN_BIN_SIZE;

      if (logDivisor < 0)
         logDivisor = 0;

      // The below if statement is only necessary on systems with high memory
      // latency relative to processor speed (most modern processors)
      if (logRange-logDivisor > MAX_SPLITS)
         logDivisor = logRange - MAX_SPLITS;

      final int divMin = min >> logDivisor;
      final int divMax = max >> logDivisor;
      final int binCount = divMax - divMin + 1;

      // Allocate the bins and determine their sizes
      final Bin[] bins = new Bin[binCount];
      
      for (int i=0; i<binCount; i++)
         bins[i] = new Bin();
      
      final int[] array = sia.array;
      final int count8 = count & -8;
      final int end8 = sia.index + count8;
      
      // Calculating the size of each bin
      for (int i=sia.index; i<end8; i+=8)
      {
         bins[(array[i]  >>logDivisor)-divMin].count++;
         bins[(array[i+1]>>logDivisor)-divMin].count++;
         bins[(array[i+2]>>logDivisor)-divMin].count++;
         bins[(array[i+3]>>logDivisor)-divMin].count++;
         bins[(array[i+4]>>logDivisor)-divMin].count++;
         bins[(array[i+5]>>logDivisor)-divMin].count++;
         bins[(array[i+6]>>logDivisor)-divMin].count++;
         bins[(array[i+7]>>logDivisor)-divMin].count++;
      }
      
      for (int i=count8; i<count; i++)
         bins[(array[sia.index+i]>>logDivisor)-divMin].count++;

      // Assign the bin positions
      bins[0].position = sia.index;

      for (int i=0; i<binCount-1; i++) 
      {
         bins[i+1].position = bins[i].position + bins[i].count;
         bins[i].count = bins[i].position - sia.index;
      }
      
      bins[binCount-1].count = bins[binCount-1].position - sia.index;

      // Swap into place.  This dominates runtime, especially in the swap
      for (int i=0; i<count; i++) 
      {
         Bin currBin;
         final int idx = sia.index + i;
         
         for (currBin=bins[(array[idx]>>logDivisor)-divMin]; currBin.count>i; ) 
         {
               final int tmp = array[currBin.position];
               array[currBin.position] = array[idx];
               array[idx] = tmp;               
               currBin.position++;
               currBin = bins[(array[idx]>>logDivisor)-divMin];
         }
         
         // Now that we have found the item belonging in this position,
         // increment the bucket count
         if (currBin.position == idx)
             currBin.position++;
      }

      minMaxCount[0] = min;
      minMaxCount[1] = max;
      minMaxCount[2] = binCount;

      // If we have bucket sorted, the array is sorted and we should skip recursion
      if (logDivisor == 0) 
         return null;

      return bins;
   }
   

   private static void spreadSortBins(SliceIntArray sia, int[] minMaxCount, Bin[] bins, int maxCount)
   {
      final int binCount = minMaxCount[2];
      
      for (int i=0; i<binCount; i++) 
      {
         final int n = (bins[i].position - sia.index) - bins[i].count;

         // Don't sort unless there are at least two items to compare
         if (n < 2)
            continue;

         if (n < maxCount)
            Arrays.sort(sia.array, sia.index+bins[i].count, bins[i].position);
         else
            _sort(sia.array, sia.index+bins[i].count, n);
      }
   }
 
   
   private static class Bin 
   {
      int position;
      int count;
   }
}
