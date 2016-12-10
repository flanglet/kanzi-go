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

import kanzi.ByteSorter;
import kanzi.IntSorter;


// Bucket sort is a very simple very fast sorting algorithm based on bucket partition
// It is a simplified radix sort with buckets of width one
// Due to this design, the max value of the data to sort is limited to 0xFFFF
// For bigger values use a radix sort
public class BucketSort implements IntSorter, ByteSorter
{
    private final int[] count;
    
    
    public BucketSort()
    {
        this.count = new int[256];
    }

    
    // Limit size to handle shorts
    public BucketSort(int logMaxValue)
    {
        if (logMaxValue < 2)
            throw new IllegalArgumentException("The log data size parameter must be at least 2");
        
        if (logMaxValue > 16)
            throw new IllegalArgumentException("The log data size parameter must be at most 16");

        this.count = new int[1 << logMaxValue];
    }
    
    
    // Not thread safe
    // all input data must be smaller than 1 << logMaxValue
    @Override
    public boolean sort(int[] input, int blkptr, int len)
    {
        if ((blkptr < 0) || (len <= 0) || (blkptr+len > input.length))
            return false;

        if (len == 1)
           return true;
        
        final int len8 = len & -8;
        final int end8 = blkptr + len8;
        final int[] c = this.count;
        final int length = c.length;

        // Unroll loop
        for (int i=blkptr; i<end8; i+=8)
        {
            c[input[i]]++;
            c[input[i+1]]++;
            c[input[i+2]]++;
            c[input[i+3]]++;
            c[input[i+4]]++;
            c[input[i+5]]++;
            c[input[i+6]]++;
            c[input[i+7]]++;
        }

        for (int i=len8; i<len; i++)
            c[input[blkptr+i]]++;

        for (int i=0, j=blkptr; i<length; i++)
        {
            final int val = c[i];

            if (val == 0)
                continue;

            c[i] = 0;
            int val8 = val & -8;

            for (int k=val; k>val8; k--)
                input[j++] = i;

            while (val8 > 0)
            {
                input[j]    = i;
                input[j+1]  = i;
                input[j+2]  = i;
                input[j+3]  = i;
                input[j+4]  = i;
                input[j+5]  = i;
                input[j+6]  = i;
                input[j+7]  = i;
                j += 8;
                val8 -= 8;
            }
        }
        
        return true;
    }


    // Not thread safe
    @Override
    public boolean sort(byte[] input, int blkptr, int len)
    {
        if ((blkptr < 0) || (len <= 0) || (blkptr+len > input.length))
            return false;

        if (len == 1)
           return true;
        
        final int len8 = len & -8;
        final int end8 = blkptr + len8;
        final int[] c = this.count;
        final int length = c.length;

        // Unroll loop
        for (int i=blkptr; i<end8; i+=8)
        {
            c[input[i] & 0xFF]++;
            c[input[i+1] & 0xFF]++;
            c[input[i+2] & 0xFF]++;
            c[input[i+3] & 0xFF]++;
            c[input[i+4] & 0xFF]++;
            c[input[i+5] & 0xFF]++;
            c[input[i+6] & 0xFF]++;
            c[input[i+7] & 0xFF]++;
        }

        for (int i=len8; i<len; i++)
            c[input[blkptr+i] & 0xFF]++;

        for (int i=0, j=blkptr; i<length; i++)
        {
            final int val = c[i];

            if (val == 0)
                continue;
            
            int val8 = val & -8;
            c[i] = 0;

            for (int k=val; k>val8; k--)
                input[j++] = (byte) i;

            while (val8 > 0)
            {
                input[j]    = (byte) i;
                input[j+1]  = (byte) i;
                input[j+2]  = (byte) i;
                input[j+3]  = (byte) i;
                input[j+4]  = (byte) i;
                input[j+5]  = (byte) i;
                input[j+6]  = (byte) i;
                input[j+7]  = (byte) i;
                input[j+8]  = (byte) i;
                j += 8;
                val8 -= 8;
            }
        }
        
        return true;
    }
    
}