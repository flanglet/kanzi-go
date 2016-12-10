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


// HeapSort is a comparison sort with O(n ln n) complexity. Practically, it is
// usually slower than QuickSort.
public final class HeapSort implements IntSorter
{
    private final ArrayComparator cmp;


    public HeapSort()
    {
        this(null);
    }


    public HeapSort(ArrayComparator cmp)
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
        
        for (int k=len>>1; k>0; k--)
        {
            doSort(input, blkptr, k, len, this.cmp);
        }

        for (int i=len-1; i>0; i--)
        {
            final int temp = input[blkptr];
            input[blkptr] = input[blkptr+i];
            input[blkptr+i] = temp;
            doSort(input, blkptr, 1, i, this.cmp);
        }
        
        return true;
    }


    private static void doSort(int[] array, int blkptr, int idx, int count,
            ArrayComparator cmp)
    {
        int k = idx;
        final int temp = array[blkptr+k-1];
        final int n = count >> 1;

        if (cmp != null)
        {
           while (k <= n)
           {
               int j = k << 1;

               if ((j < count) && (cmp.compare(array[blkptr+j-1], array[blkptr+j]) < 0))
                   j++;

               if (temp >= array[blkptr+j-1])
                   break;

               array[blkptr+k-1] = array[blkptr+j-1];
               k = j;
           }
        }
        else
        {
           while (k <= n)
           {
               int j = k << 1;

               if ((j < count) && (array[blkptr+j-1] < array[blkptr+j]))
                   j++;

               if (temp >= array[blkptr+j-1])
                   break;

               array[blkptr+k-1] = array[blkptr+j-1];
               k = j;
           }
        }

        array[blkptr+k-1] = temp;
    }
}
