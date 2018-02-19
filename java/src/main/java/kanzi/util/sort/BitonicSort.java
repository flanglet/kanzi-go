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


// See http://www.tools-of-computing.com/tc/CS/Sorts/bitonic_sort.htm
// or http://www.iti.fh-flensburg.de/lang/algorithmen/sortieren/bitonic/bitonicen.htm
// Bitonic sort performs best when implemented as a parallel algorithm (not here)
public class BitonicSort implements IntSorter
{
    private static final int[] POWER_OF_TWO =
    {
        0,  0,  1,  2,  2,  4,  4,  4,  4,  8,  8,  8,  8,  8,  8,  8,
        8, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16,
       16, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32,
       32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32
    };


    public BitonicSort()
    {
    }

    
    // all input data must be smaller than 1 << logDataSize
    @Override
    public boolean sort(int[] input, int blkptr, int len)
    {
        if ((blkptr < 0) || (len <= 0) || (blkptr+len > input.length))
            return false;

        if (len == 1)
           return true;
        
        sort(input, blkptr, len, true);        
        return true;
    }


    private static void sort(int[] array, int lo, int n, boolean up)
    {
        int m = n >> 1;

        if (m > 1)
          sort(array, lo, m, !up);

        if (n - m > 1)
           sort(array, lo+m, n-m, up);

        if (n > 1)
          merge(array, lo, n, up);
    }


    private static void merge(int[] array, int lo, int n, boolean up)
    {
       // Find greatest power of two smaller than n
       int m;

       if (n < POWER_OF_TWO.length) 
       {
          m = POWER_OF_TWO[n];
       } 
       else 
       {
          m = 1;

          while (m < n)
             m <<= 1;

          m >>= 1;
       }

        final int end = lo + n - m;

        for (int i=lo; i<end; i++)
        {
            if ((array[i] > array[i+m]) == up)
            {
                int tmp = array[i];
                array[i] = array[i+m];
                array[i+m] = tmp;
            }
        }

        if (m > 1)
           merge(array, lo, m, up);

        if (n-m > 1)
           merge(array, lo+m, n-m, up);
    }
}