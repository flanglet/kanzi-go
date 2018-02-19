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

package kanzi.transform;

import kanzi.SliceIntArray;

// Compute suffix array using the SA_IS algorithm.
// This implementation is based on the SA_IS (Suffix Array Induction Sorting) algorithm.
// This is a port of sais.c by Yuta Mori (http://sites.google.com/site/yuta256/sais)
// See original publication of the algorithm here:
// [Ge Nong, Sen Zhang and Wai Hong Chan, Two Efficient Algorithms for
// Linear Suffix Array Construction, 2008]
// Another good read: http://labh-curien.univ-st-etienne.fr/~bellet/misc/SA_report.pdf
//
// Overview of the algorithm:
// Step 1 - Problem reduction: the input string is reduced into a smaller string.
// Step 2 - Recursion: the suffix array of the reduced problem is recursively computed.
// Step 3 - Problem induction: based on the suffix array of the reduced problem, that of the
//          unreduced problem is induced

public class SA_IS 
{
      // find the start or end of each bucket
      private static void getCounts(SliceIntArray src, SliceIntArray dst, int n, int k)
      {
        final int[] dstArray = dst.array;
        final int[] srcArray = src.array;
        final int dstIdx = dst.index;
        final int srcIdx = src.index;

        for (int i=dstIdx+k-1; i>=dstIdx; i--)
           dstArray[i] = 0;

        for (int i=srcIdx+n-1; i>=srcIdx; i--)
           dstArray[dstIdx+srcArray[i]]++;
      }


      private static void getBuckets(SliceIntArray src, SliceIntArray dst, int k, boolean end)
      {
        int sum = 0;
        final int[] dstArray = dst.array;
        final int[] srcArray = src.array;
        final int dstIdx = dst.index;
        final int srcIdx = src.index;

        if (end == true)
        {
           for (int i=0; i<k; i++)
           {
              sum += srcArray[srcIdx+i];
              dstArray[dstIdx+i] = sum;
           }
        }
        else
        {
           for (int i=0; i<k; i++)
           {
              // The temp variable is required if srcArray == dstArray
              final int tmp = srcArray[srcIdx+i];
              dstArray[dstIdx+i] = sum;
              sum += tmp;
           }
        }
      }


      // sort all type LMS suffixes
      private static void sortLMSSuffixes(SliceIntArray src, int[] sa, SliceIntArray C,
              SliceIntArray B, int n, int k)
      {
        // compute sal
        if (C == B)
           getCounts(src, C, n, k);

        // find starts of buckets
        getBuckets(C, B, k, false);

        int j = n - 1;
        final int[] srcArray = src.array;
        final int srcIdx = src.index;
        final int[] array = B.array;
        final int bIdx = B.index;
        int c1 = srcArray[srcIdx+j];
        int b = array[bIdx+c1];
        j--;
        sa[b++] = (srcArray[srcIdx+j] < c1) ? ~j : j;

        for (int i=0; i<n; i++)
        {
          j = sa[i];

          if (j > 0)
          {
            final int c0 = srcArray[srcIdx+j];

            if (c0 != c1)
            {
               array[bIdx+c1] = b;
               c1 = c0;
               b = array[bIdx+c1];
            }

            j--;
            sa[b++] = (srcArray[srcIdx+j] < c1) ? ~j : j;
            sa[i] = 0;
          }
          else if (j < 0)
            sa[i] = ~j;
        }

        // compute sas
        if (C == B)
           getCounts(src, C, n, k);

        // find ends of buckets
        getBuckets(C, B, k, true);
        c1 = 0;
        b = array[bIdx+c1];

        for (int i=n-1; i>=0; i--)
        {
          j = sa[i];

          if (j <= 0)
             continue;
          
          final int c0 = srcArray[srcIdx+j];

          if (c0 != c1)
          {
             array[bIdx+c1] = b;
             c1 = c0;
             b = array[bIdx+c1];
          }

          j--;
          b--;
          sa[b] = (srcArray[srcIdx+j] > c1) ? ~(j + 1) : j;
          sa[i] = 0;
        }
      }


      private static int postProcessLMS(SliceIntArray src, int[] sa, int n, int m)
      {
        int i = 0;
        int j;
        final int index = src.index;
        final int[] array = src.array;

        // compact all the sorted substrings into the first m items of sa
        // 2*m must be not larger than n
        for (int p; (p=sa[i])<0; i++)
           sa[i] = ~p;

        if (i < m)
        {
          j = i;
          i++;

          do
          {
            final int p = sa[i++];

            if (p >= 0)
               continue;

            sa[j++] = ~p;
            sa[i-1] = 0;
          }
          while (j != m);               
          
        }

        // store the length of all substrings
        i = index + n - 1;
        j = n - 1;
        int c0 = array[i];
        int c1;

        do
        {
          c1 = c0;
          i--;
        }
        while ((i >= index) && ((c0 = array[i]) >= c1));

        while (i >= index)
        {
          do
          {
            c1 = c0;
            i--;
          }
          while ((i >= index) && ((c0 = array[i]) <= c1));

          if (i < index)
             break;

          sa[m+((i-index+1)>>1)] = j - i + index;
          j = i - index + 1;

          do
          {
            c1 = c0;
            i--;
          }
          while ((i >= index) && ((c0 = array[i]) >= c1));
        }

        // find the lexicographic names of all substrings
        int name = 0;
        int q = n;
        int qlen = 0;

        for (int ii=0; ii<m; ii++)
        {
          final int p = sa[ii];
          final int plen = sa[m+(p>>1)];
          boolean diff = true;

          if ((plen == qlen) && ((q + plen) < n))
          {
            int jj = index;
            final int plen2 = index + plen;

            while ((jj<plen2) && (array[p+jj] == array[q+jj]))
               jj++;

            if (jj == plen2)
               diff = false;
          }

          if (diff == true)
          {
             name++;
             q = p;
             qlen = plen;
          }

          sa[m+(p>>1)] = name;
        }

        return name;
      }


      private static void induceSuffixArray(SliceIntArray src, int[] sa, SliceIntArray buf1,
              SliceIntArray buf2, int n, int k)
      {
        // compute sal
        if (buf1 == buf2)
           getCounts(src, buf1, n, k);

        // find starts of buckets
        getBuckets(buf1, buf2, k, false);

        final int srcIdx = src.index;
        final int[] srcArray = src.array;
        final int bufIdx = buf2.index;
        final int[] bufArray = buf2.array;
        int j = n - 1;
        int c1 = srcArray[srcIdx+j];
        int b = bufArray[bufIdx+c1];
        sa[b++] = ((j > 0) && (srcArray[srcIdx+j-1] < c1)) ? ~j : j;

        for (int i=0; i<n; i++)
        {
          j = sa[i];
          sa[i] = ~j;

          if (j <= 0)
             continue;
          
          j--;
          final int c0 = srcArray[srcIdx+j];

          if (c0 != c1)
          {
             bufArray[bufIdx+c1] = b;
             c1 = c0;
             b = bufArray[bufIdx+c1];
          }

          sa[b++] = ((j > 0) && (srcArray[srcIdx+j-1] < c1)) ? ~j : j;          
        }

        // compute sas
        if (buf1 == buf2)
           getCounts(src, buf1, n, k);

        // find ends of buckets
        getBuckets(buf1, buf2, k, true);
        c1 = 0;
        b = bufArray[bufIdx+c1];

        for (int i=n-1; i>=0; i--)
        {
          j = sa[i];

          if (j <= 0)
          {
             sa[i] = ~j;
             continue;
          }
          
          j--;
          final int c0 = srcArray[srcIdx+j];

          if (c0 != c1)
          {
             bufArray[bufIdx+c1] = b;
             c1 = c0;
             b = bufArray[bufIdx+c1];
          }

          b--;
          sa[b] = ((j == 0) || (srcArray[srcIdx+j-1] > c1)) ? ~j : j;
        }
      }


      private static int computeBWT(SliceIntArray data, int[] sa, SliceIntArray iia1,
              SliceIntArray iia2, int n, int k)
      {
        // compute sal
        if (iia1 == iia2)
           getCounts(data, iia1, n, k);

        // find starts of buckets
        getBuckets(iia1, iia2, k, false);
        int[] array = data.array;
        int[] buffer = iia2.array;
        int arrayIdx = data.index;
        int bufferIdx = iia2.index;
        int j = n - 1;
        int c1 = array[arrayIdx+j];
        int b = buffer[bufferIdx+c1];
        sa[b++] = ((j > 0) && (array[arrayIdx+j-1] < c1)) ? ~j : j;

        for (int i=0; i<n; i++)
        {
          j = sa[i];

          if (j > 0)
          {
            j--;
            final int c0 = array[arrayIdx+j];
            sa[i] = ~c0;

            if (c0 != c1)
            {
               buffer[bufferIdx+c1] = b;
               c1 = c0;
               b = buffer[bufferIdx+c1];
            }

            sa[b++] = ((j > 0) && (array[arrayIdx+j-1] < c1)) ? ~j : j;
          }
          else if (j != 0)
            sa[i] = ~j;
        }

        // compute sas
        if (iia1 == iia2)
           getCounts(data, iia1, n, k);

        // find ends of buckets
        getBuckets(iia1, iia2, k, true);
        c1 = 0;
        b = buffer[bufferIdx+c1];
        int pidx = -1;

        for (int i=n-1; i>=0; i--)
        {
          j = sa[i];

          if (j > 0)
          {
            j--;
            final int c0 = array[arrayIdx+j];
            sa[i] = c0;

            if (c0 != c1)
            {
               buffer[bufferIdx+c1] = b;
               c1 = c0;
               b = buffer[bufferIdx+c1];
            }

            b--;
            sa[b] = ((j > 0) && (array[arrayIdx+j-1] > c1)) ? ~(array[arrayIdx+j-1]) : j;
          }
          else if (j != 0)
            sa[i] = ~j;
          else
            pidx = i;
        }

        return pidx;
      }


      // Find the suffix array sa of data[0..n-1] in {0..k-1}^n
      // Return the primary index if isbwt is true (0 otherwise)
      public static int computeSuffixArray(SliceIntArray data, int[] sa, int fs, 
              int n, int k, boolean isBWT)
      {
        SliceIntArray C;
        SliceIntArray B;
        int flags;

        if (k <= 256)
        {
          C = new SliceIntArray(new int[k], 0);

          if (k <= fs)
          {
             B = new SliceIntArray(sa, n+fs-k);
             flags = 1;
          }
          else
          {
             B = new SliceIntArray(new int[k], 0);
             flags = 3;
          }
        }
        else if (k <= fs)
        {
          C = new SliceIntArray(sa, n+fs-k);

          if (k <= (fs-k))
          {
             B = new SliceIntArray(sa, n+fs-(k+k));
             flags = 0;
          }
          else if (k <= 1024)
          {
             B = new SliceIntArray(new int[k], 0);
             flags = 2;
          }
          else
          {
             B = C;
             flags = 8;
          }
        }
        else
        {
           B = new SliceIntArray(new int[k], 0);
           C = B;
           flags = 12;
        }

        // stage 1: reduce the problem by at least 1/2, sort all the LMS-substrings
        // find ends of buckets
        getCounts(data, C, n, k);
        getBuckets(C, B, k, true);

        for (int ii=0; ii<n; ii++)
           sa[ii] = 0;

        final int[] array = data.array;
        final int arrayIdx = data.index;
        int b = -1;
        int i = arrayIdx + n - 1;
        int j = n;
        int m = 0;
        int c0 = array[i];
        int c1;

        do
        {
           c1 = c0;
           i--;
        }
        while ((i >= arrayIdx) && ((c0 = array[i]) >= c1));

        final int[] buffer = B.array;
        final int bufferIdx = B.index;

        while (i >= arrayIdx)
        {
          do
          {
             c1 = c0;
             i--;
          }
          while ((i >= arrayIdx) && ((c0 = array[i]) <= c1));

          if (i < arrayIdx)
             break;
          
          if (b >= 0)
             sa[b] = j;

          buffer[bufferIdx+c1]--;
          b = buffer[bufferIdx+c1];
          j = i - arrayIdx;
          m++;

          do
          {
            c1 = c0;
            i--;
          }
          while ((i >= arrayIdx) && ((c0 = array[i]) >= c1));
        }

        int name = 0;
        
        if (m > 1)
        {
          sortLMSSuffixes(data, sa, C, B, n, k);
          name = postProcessLMS(data, sa, n, m);
        }
        else if (m == 1)
        {
          sa[b] = j + 1;
          name = 1;
        }

        // stage 2: solve the reduced problem, recurse if names are not yet unique
        if (name < m)
        {
          int newfs = (n+fs) - (m+m);

          if ((flags & 13) == 0)
          {
            if ((k + name) <= newfs)
              newfs -= k;
            else
              flags |= 8;
          }

          j = m + m + newfs - 1;

          for (int ii=m+(n>>1)-1; ii>=m; ii--)
          {
            if (sa[ii] != 0)
              sa[j--] = sa[ii] - 1;
          }

          computeSuffixArray(new SliceIntArray(sa, m + newfs), sa, newfs, m, name, false);

          i = arrayIdx + n - 1;
          j = m + m - 1;
          c0 = array[i];

          do
          {
            c1 = c0;
            i--;
          }
          while ((i >= arrayIdx) && ((c0 = array[i]) >= c1));

          while (i >= arrayIdx)
          {
            do
            {
              c1 = c0;
              i--;
            }
            while ((i >= arrayIdx) && ((c0 = array[i]) <= c1));

            if (i < arrayIdx)
               break;
            
            sa[j--] = i - arrayIdx + 1;

            do
            {
              c1 = c0;
              i--;
            }
            while ((i >= arrayIdx) && ((c0 = array[i]) >= c1));
          }

          for (int ii=0; ii<m; ii++)
             sa[ii] = sa[m+sa[ii]];

          if ((flags & 4) != 0)
          {
             B = new SliceIntArray(new int[k], 0);
             C = B;
          }
          else if ((flags & 2) != 0)
             B = new SliceIntArray(new int[k], 0);
        }

        // stage 3: induce the result for the original problem
        if ((flags & 8) != 0)
           getCounts(data, C, n, k);

        // put all left-most S characters into their buckets
        if (m > 1)
        {
          // find ends of buckets
          getBuckets(C, B, k, true);
          i = m - 1;
          j = n;
          int p = sa[m-1];
          c1 = array[arrayIdx+p];

          do
          {
            c0 = c1;
            final int q = B.array[B.index+c0];

            while (q < j)
               sa[--j] = 0;

            do
            {
              sa[--j] = p;

              if (--i < 0)
                 break;

              p = sa[i];
              c1 = array[arrayIdx+p];
            }
            while (c1 == c0);
          }
          while (i >= 0);

          while (j > 0)
             sa[--j] = 0;
        }

        if (isBWT == false)
        {
           induceSuffixArray(data, sa, C, B, n, k);
           return 0;
        }

        return computeBWT(data, sa, C, B, n, k);
     }   
}
