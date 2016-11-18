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

//
// Karl-Dietrich Neubert's Flashsort1 Algorithm
// See [http://www.neubert.net/Flapaper/9802n.htm]
// Fast distribution based non stable sort

public class FlashSort implements IntSorter
{
   private static final int SHIFT = 15;

   private int[] buffer;


    public FlashSort()
    {
       this.buffer = new int[0];
    }


    // Not thread safe 
    @Override
    public boolean sort(int[] input, int blkptr, int len)
    {
       if ((blkptr < 0) || (len <= 0) || (blkptr+len > input.length))
          return false;

        if (len == 1)
           return true;
        
       final int m = (len * 215) >> 9; // speed optimum m = 0.42 n

       if (this.buffer.length < m)
          this.buffer = new int[(m<32) ? 32 : (m+7) & -8];

       this.partialFlashSort(input, blkptr, len);
       return new InsertionSort().sort(input, blkptr, len);
    }


    private void partialFlashSort(int[] input, int blkptr, int count)
    {
        int min = input[blkptr];
        int max = min;
        int idxMax = blkptr;
        final int end = blkptr + count;

        for (int i=blkptr+1; i<end; i++)
        {
           int val = input[i];

           if (val < min)
              min = val;

           if (val > max)
           {
              max = val;
              idxMax = i;
           }
        }

        if (min == max)
           return;

        // Aliasing for speed
        final int[] buf = this.buffer;
        final int len8 = buf.length;
        final long delta = max - min;
        final long delta1 = delta + 1;

        // Reset buckets buffer
        for (int i=0; i<len8; i+=8)
        {
           buf[i]   = 0;
           buf[i+1] = 0;
           buf[i+2] = 0;
           buf[i+3] = 0;
           buf[i+4] = 0;
           buf[i+5] = 0;
           buf[i+6] = 0;
           buf[i+7] = 0;
        }

        int shiftL = SHIFT;
        final int threshold = Integer.MAX_VALUE >> 1;
        long c1 = 0;
        long num = 0;

        // Find combinations, shiftL, shiftR and c1
        while ((c1 == 0) && (num < threshold))
        {
           shiftL++;
           num = ((long) len8) << shiftL;
           c1 = num / delta1;
        }

        int shiftR = shiftL;

        while (c1 == 0)
        {
           final long denum = delta >>> (shiftR - shiftL);
           c1 = num / denum;
           shiftR++;
        }

        // Create the buckets
        for (int i=blkptr; i<end; i++)
        {
           final long k = (c1 * (input[i] - min)) >>> shiftR;
           buf[(int) k]++;
        }

        // Create distribution
        for (int i=1; i<len8; i++)
           buf[i] += buf[i-1];

        input[idxMax] = input[blkptr];
        input[blkptr] = max;
        int j = 0;
        int k = len8 - 1;
        int nmove = 1;
        final int offs = blkptr - 1;

        while (nmove < count)
        {
            while (j >= buf[k])
            {
                j++;
                final long kl = (c1 * (input[blkptr+j] - min)) >>> shiftR;
                k = (int) kl;
            }

            int flash = input[blkptr+j];

            // Speed critical section
            while (buf[k] != j)
            {
                final long kl = (c1 * (flash - min)) >>> shiftR;
                k = (int) kl;
                final int idx = offs + buf[k];
                final int hold = input[idx];
                input[idx] = flash;
                flash = hold;
                buf[k]--;
                nmove++;
            }
        }
    }

}