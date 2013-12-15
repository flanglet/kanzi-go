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

package kanzi.util.sort;

import kanzi.IndexedIntArray;
import kanzi.IntSorter;


// A MergeSort is conceptually very simple (divide and merge) but usually not
// very performant ... except for almost sorted data
// This implementation based on OpenJDK avoids the usual trap of many array creations
public class MergeSort implements IntSorter
{
    private static final int SMALL_ARRAY_THRESHOLD = 32;

    private int[] buffer;


    public MergeSort()
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
        
        if (this.buffer.length < len)
            this.buffer = new int[len];

        System.arraycopy(input, blkptr, this.buffer, 0, len);
        IndexedIntArray dst = new IndexedIntArray(input, blkptr);
        IndexedIntArray src = new IndexedIntArray(this.buffer, 0);
        sort(src, dst, blkptr, blkptr+len);
        return true;
    }

    
    private static void sort(IndexedIntArray srcIba, IndexedIntArray dstIba, int start, int end)
    {
        final int length = end - start;
        final int[] src = srcIba.array;
        final int[] dst = dstIba.array;

        // Insertion sort on smallest arrays
        if (length < SMALL_ARRAY_THRESHOLD)
        {
            start += dstIba.index;
            end   += dstIba.index;
            
            for (int i=start; i<end; i++)
            {
                for (int j=i; (j>start) && (dst[j-1]>dst[j]); j--)
                {
                    final int tmp = dst[j-1];
                    dst[j-1] = dst[j];
                    dst[j] = tmp;
                }
            }

            return;
        }

        int mid = (start + end) >>> 1;
        sort(dstIba, srcIba, start, mid);
        sort(dstIba, srcIba, mid, end);
        mid += srcIba.index;
 
        if (src[mid-1] <= src[mid])
        {
           System.arraycopy(src, start, dst, start, length);
           return;
        }

        final int starti = start + dstIba.index;
        final int endi = end + dstIba.index;
        int j = start + srcIba.index;
        int k = mid;
        final int endk = end + srcIba.index;

        for (int i=starti; i<endi; i++)
        {
            if ((k >= endk) || (j < mid) && (src[j] <= src[k]))
                dst[i] = src[j++];
            else
                dst[i] = src[k++];
        }
    }
}

			


