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

import java.util.concurrent.ForkJoinPool;
import java.util.concurrent.RecursiveAction;
import java.util.concurrent.RejectedExecutionException;
import kanzi.ByteSorter;
import kanzi.IntSorter;


public class ForkJoinParallelSort implements IntSorter, ByteSorter
{
   private final ForkJoinPool pool;

   
   public ForkJoinParallelSort(ForkJoinPool pool)
   {
      if (pool == null)
         throw new NullPointerException("Invalid null fork/join pool parameter");

      this.pool = pool;
   }

   
   @Override
   public boolean sort(int[] array, int idx, int len) 
   {
      try
      {
         this.pool.invoke(new SortTask(array, idx, len));      
      }
      catch (RejectedExecutionException e)
      {
         return false;
      }
      
      return true;
   }

   
   @Override
   public boolean sort(byte[] array, int idx, int len) 
   {
      try
      {
         this.pool.invoke(new SortTask(array, idx, len));      
      }
      catch (RejectedExecutionException e)
      {
         return false;
      }
      
      return true;
   }
 
   
   static class SortTask extends RecursiveAction
   {
      private static final int MIN_THRESHOLD = 8192;
      private static final int MAX_THRESHOLD = MIN_THRESHOLD << 1;

      private final transient IntSorter iDelegate;
      private final transient ByteSorter bDelegate;
      private final int[] iSrc;
      private final int[] iDst;
      private final byte[] bSrc;
      private final byte[] bDst;
      private final int size;
      private final int startIdx;
      private final int threshold;


      public SortTask(int[] array, int idx, int len)
      {
         this(array, idx, len, new int[len]);
      }


      protected SortTask(int[] array, int idx, int len, int[] buffer)
      {
         this.size = len;
         this.startIdx = idx;
         this.iDelegate = new FlashSort();
         this.iDst = buffer;
         this.iSrc = array;
         this.bDelegate = null;
         this.bDst = null;
         this.bSrc = null;

         while (len >= MAX_THRESHOLD)
             len >>= 1;

         this.threshold = (len < MIN_THRESHOLD) ? MIN_THRESHOLD : len;
      }


      public SortTask(byte[] array, int idx, int len)
      {
         this(array, idx, len, new byte[len]);
      }


      protected SortTask(byte[] array, int idx, int len, byte[] buffer)
      {
         this.size = len;
         this.startIdx = idx;
         this.bDelegate = new BucketSort(8);
         this.bDst = buffer;
         this.bSrc = array;
         this.iDelegate = null;
         this.iDst = null;
         this.iSrc = null;

         while (len >= MAX_THRESHOLD)
             len >>= 1;

         this.threshold = (len < MIN_THRESHOLD) ? MIN_THRESHOLD : len;
      }


      @Override
      protected void compute()
      {
         if (this.iSrc != null)
           this.sortInts();
         else
           this.sortBytes();
      }


      protected void sortBytes()
      {
         if (this.size < this.threshold)
         {
            // Stop recursion, use delegate to sort
            this.bDelegate.sort(this.bSrc, this.startIdx, this.size);
            return;
         }

         final int half = this.size >> 1;
         SortTask lowerHalfTask = new SortTask(this.iSrc, this.startIdx, half, this.iDst);
         SortTask upperHalfTask = new SortTask(this.iSrc, this.startIdx+half, this.size-half, this.iDst);

         // Fork
         invokeAll(lowerHalfTask, upperHalfTask);

         // Join
         final byte[] source = this.bSrc;
         final byte[] dest = this.bDst;
         int idx  = this.startIdx;
         int idx1 = this.startIdx;
         int idx2 = idx1 + half;
         final int end1 = idx2;
         final int end2 = this.startIdx + this.size;

         while ((idx1 < end1) && (idx2 < end2))
         {
            dest[idx++] = (source[idx1] < source[idx2]) ? source[idx1++] : source[idx2++];
         }

         while (idx1 < end1)
             dest[idx++] = source[idx1++];

         while (idx2 < end2)
             dest[idx++] = source[idx2++];

         System.arraycopy(dest, this.startIdx, source, this.startIdx, this.size);
      }


      protected void sortInts()
      {
         if (this.size < this.threshold)
         {
            // Stop recursion, use delegate to sort
            this.iDelegate.sort(this.iSrc, this.startIdx, this.size);
            return;
         }

         final int half = this.size >> 1;
         SortTask lowerHalfTask = new SortTask(this.iSrc, this.startIdx, half, this.iDst);
         SortTask upperHalfTask = new SortTask(this.iSrc, this.startIdx+half, this.size-half, this.iDst);

         // Fork
         invokeAll(lowerHalfTask, upperHalfTask);

         // Join
         final int[] source = this.iSrc;
         final int[] dest = this.iDst;
         int idx  = this.startIdx;
         int idx1 = this.startIdx;
         int idx2 = idx1 + half;
         final int end1 = idx2;
         final int end2 = this.startIdx + this.size;

         while ((idx1 < end1) && (idx2 < end2))
         {
            dest[idx++] = (source[idx1] < source[idx2]) ? source[idx1++] : source[idx2++];
         }

         while (idx1 < end1)
             dest[idx++] = source[idx1++];

         while (idx2 < end2)
             dest[idx++] = source[idx2++];

         System.arraycopy(dest, this.startIdx, source, this.startIdx, this.size);
      }
   }

}