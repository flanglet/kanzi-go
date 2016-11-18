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

// Port to (proper) Java of the DivSufSort algorithm by Yuta Mori.
// DivSufSort is a fast two-stage suffix sorting algorithm.
// The original C code is here: https://code.google.com/p/libdivsufsort/
// See also https://code.google.com/p/libdivsufsort/source/browse/wiki/SACA_Benchmarks.wiki
// for comparison of different suffix array construction algorithms.
// It is used to implement the forward stage of the BWT in linear time.

public final class DivSufSort
{
    private static final int SS_INSERTIONSORT_THRESHOLD = 8;
    private static final int SS_BLOCKSIZE = 1024;
    private static final int SS_MISORT_STACKSIZE = 16;
    private static final int SS_SMERGE_STACKSIZE = 32;
    private static final int TR_STACKSIZE = 64;
    private static final int TR_INSERTIONSORT_THRESHOLD = 8;


    private final static int[] SQQ_TABLE =
    {
         0, 16, 22, 27, 32, 35, 39, 42, 45, 48, 50, 53, 55, 57, 59, 61, 64, 65, 67, 69,
        71, 73, 75, 76, 78, 80, 81, 83, 84, 86, 87, 89, 90, 91, 93, 94, 96, 97, 98, 99,
        101, 102, 103, 104, 106, 107, 108, 109, 110, 112, 113, 114, 115, 116, 117, 118,
        119, 120, 121, 122, 123, 124, 125, 126, 128, 128, 129, 130, 131, 132, 133, 134,
        135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 144, 145, 146, 147, 148, 149,
        150, 150, 151, 152, 153, 154, 155, 155, 156, 157, 158, 159, 160, 160, 161, 162,
        163, 163, 164, 165, 166, 167, 167, 168, 169, 170, 170, 171, 172, 173, 173, 174,
        175, 176, 176, 177, 178, 178, 179, 180, 181, 181, 182, 183, 183, 184, 185, 185,
        186, 187, 187, 188, 189, 189, 190, 191, 192, 192, 193, 193, 194, 195, 195, 196,
        197, 197, 198, 199, 199, 200, 201, 201, 202, 203, 203, 204, 204, 205, 206, 206,
        207, 208, 208, 209, 209, 210, 211, 211, 212, 212, 213, 214, 214, 215, 215, 216,
        217, 217, 218, 218, 219, 219, 220, 221, 221, 222, 222, 223, 224, 224, 225, 225,
        226, 226, 227, 227, 228, 229, 229, 230, 230, 231, 231, 232, 232, 233, 234, 234,
        235, 235, 236, 236, 237, 237, 238, 238, 239, 240, 240, 241, 241, 242, 242, 243,
        243, 244, 244, 245, 245, 246, 246, 247, 247, 248, 248, 249, 249, 250, 250, 251,
        251, 252, 252, 253, 253, 254, 254, 255
    };

    private final static int[] LOG_TABLE =
    {
        -1, 0, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
        4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
        5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
        6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
        6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 7, 7, 7, 7, 7, 7, 7,
        7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
        7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
        7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
        7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
        7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7
    };

    private int[] sa;
    private short[] buffer;
    private final int[] bucketA;
    private final int[] bucketB;
    private final Stack ssStack;
    private final Stack trStack;
    private final Stack mergeStack;


    public DivSufSort()
    {
        this.bucketA = new int[256];
        this.bucketB = new int[65536];
        this.sa = new int[0];
        this.buffer = new short[0];
        this.ssStack = new Stack(SS_MISORT_STACKSIZE);
        this.trStack = new Stack(TR_STACKSIZE);
        this.mergeStack = new Stack(SS_SMERGE_STACKSIZE);
    }


    public void reset()
    {
       this.ssStack.index = 0;
       this.trStack.index = 0;
       this.mergeStack.index = 0;

       for (int i=this.bucketA.length-1; i>=0; i--)
          this.bucketA[i] = 0;

       for (int i=this.bucketB.length-1; i>=0; i--)
          this.bucketB[i] = 0;
    }



    // Not thread safe
    public int[] computeSuffixArray(byte[] input, int start, int length)
    {
        // Lazy dynamic memory allocation 
        if (this.sa.length < length)
           this.sa = new int[length];
        
        if (this.buffer.length < length+1)
           this.buffer = new short[length+1];

        for (int i=0; i<length; i++)
           this.buffer[i] = (short) (input[start+i] & 0xFF);

        this.buffer[length] = input[start];
        int m = this.sortTypeBstar(this.bucketA, this.bucketB, length);
        this.constructSuffixArray(this.bucketA, this.bucketB, length, m);
        return this.sa;
    }


    private void constructSuffixArray(int[] bucket_A, int[] bucket_B, int n, int m)
    {
        if (m > 0)
        {
            for (int c1=254; c1>=0; c1--)
            {
                final int idx = c1 << 8;
                final int i = bucket_B[idx+c1+1];
                int k = 0;
                int c2 = -1;

                for (int j=bucket_A[c1+1]-1; j>=i; j--)
                {
                    int s = this.sa[j];
                    this.sa[j] = ~s;

                    if (s <= 0)
                       continue;
                    
                    s--;
                    final int c0 = this.buffer[s];

                    if ((s > 0) && (this.buffer[s-1] > c0))
                        s = ~s;

                    if (c0 != c2)
                    {
                        if (c2 >= 0)
                            bucket_B[idx+c2] = k;

                        c2 = c0;
                        k = bucket_B[idx+c2];
                    }

                    this.sa[k--] = s;
                }
            }
        }

        int c2 = this.buffer[n-1];
        int k = bucket_A[c2];
        this.sa[k++] = (this.buffer[n-2] < c2) ? ~(n-1) : (n-1);

        // Scan the suffix array from left to right.
        for (int i=0; i < n; i++)
        {
            int s = this.sa[i];

            if (s <= 0)
            {
               this.sa[i] = ~s;
               continue;
            }
             
            s--;
            final int c0 = this.buffer[s];

            if ((s == 0) || (this.buffer[s-1] < c0))
                s = ~s;

            if (c0 != c2)
            {
                bucket_A[c2] = k;
                c2 = c0;
                k = bucket_A[c2];
            }

            this.sa[k++] = s;
        }
    }


    private int sortTypeBstar(int[] bucket_A, int[] bucket_B, final int n)
    {
        int m = n;
        int c0 = this.buffer[n-1];
        final int[] arr = this.sa;

        for (int i=n-1; i>=0; )
        {
            int c1;
            
            do
            {
                c1 = c0;
                bucket_A[c1]++;
                i--;
            }
            while ((i >= 0) && ((c0=this.buffer[i]) >= c1));

            if (i < 0)
               break;

            bucket_B[(c0<<8)+c1]++;
            m--;
            arr[m] = i;
            i--;
            c1 = c0;

            while ((i >= 0) && ((c0=this.buffer[i]) <= c1))
            {
               bucket_B[(c1<<8)+c0]++;
               c1 = c0;
               i--;
            }
        }

        m = n - m;
        c0 = 0;

        // Calculate the index of start/end point of each bucket.
        for (int i=0, j=0; c0<256; c0++)
        {
            final int t = i + bucket_A[c0];
            bucket_A[c0] = i + j; // start point
            final int idx = c0 << 8;
            i = t + bucket_B[idx+c0];

            for (int c1=c0+1; c1<256; c1++)
            {
                j += bucket_B[idx+c1];
                bucket_B[idx+c1] = j; // end point
                i += bucket_B[(c1<<8)+c0];
            }
        }

        if (m > 0)
        {
            // Sort the type B* suffixes by their first two characters.
            final int pab = n - m;

            for (int i=m-2; i>=0; i--)
            {
                final int t = arr[pab+i];
                final int idx = (this.buffer[t]<<8) + this.buffer[t+1];
                bucket_B[idx]--;
                arr[bucket_B[idx]] = i;
            }

            final int t = arr[pab+m-1];
            c0 = (this.buffer[t]<<8) + this.buffer[t+1];
            bucket_B[c0]--;
            arr[bucket_B[c0]] = m - 1;

            // Sort the type B* substrings using ssSort.
            final int bufSize = n - m - m;
            c0 = 254;

            for (int j=m; j>0; c0--)
            {
                final int idx = c0 << 8;
               
                for (int c1=255; c1>c0; c1--)
                {
                    final int i = bucket_B[idx+c1];

                    if (j > i + 1)
                        this.ssSort(pab, i, j, m, bufSize, 2, n, arr[i] == m - 1);

                    j = i;
                }
            }

            // Compute ranks of type B* substrings.
            for (int i=m-1; i >= 0; i--)
            {
                if (arr[i] >= 0)
                {
                    final int i0 = i;

                    do
                    {
                        arr[m+arr[i]] = i;
                        i--;
                    }
                    while ((i >= 0) && (arr[i] >= 0));

                    arr[i+1] = i - i0;

                    if (i <= 0)
                        break;
                }

                final int i0 = i;

                do
                {
                    arr[i] = ~arr[i];
                    arr[m+arr[i]] = i0;
                    i--;
                }
                while (arr[i] < 0);

                arr[m+arr[i]] = i0;
            }

            // Construct the inverse suffix array of type B* suffixes using trsort.
            this.trSort(m, 1);
            
            // Set the sorted order of type B* suffixes.
            c0 = this.buffer[n-1];
            int c1;

            for (int i=n-1, j=m; i>=0; )
            {
                i--;
                c1 = c0;

                for ( ; (i >= 0) && ((c0=this.buffer[i]) >= c1); i--)
                {
                   c1 = c0;
                }

                if (i >= 0)
                {
                    final int tt = i;
                    i--;
                    c1 = c0;

                    for (; (i >= 0) && ((c0=this.buffer[i]) <= c1); i--)
                    {
                       c1 = c0;
                    }

                    j--;
                    arr[arr[m+j]] = ((tt == 0) || (tt-i > 1)) ? tt : ~tt;
                }
            }

            // Calculate the index of start/end point of each bucket.
            bucket_B[bucket_B.length-1] = n; // end
            c0 = 254;

            for (int k=m-1; c0>=0; c0--)
            {
                int i = bucket_A[c0+1] - 1;
                final int idx = c0 << 8;

                for (c1=255; c1>c0; c1--)
                {
                    final int tt = i - bucket_B[(c1<<8)+c0];
                    bucket_B[(c1<<8)+c0] = i; // end point
                    i = tt;

                    // Move all type B* suffixes to the correct position.
                    // Typically very small number of copies, no need for arraycopy
                    for (int j=bucket_B[idx+c1]; j<=k; i--, k--)
                        arr[i] = arr[k];
                }

                bucket_B[idx+c0+1] = i - bucket_B[idx+c0] + 1;
                bucket_B[idx+c0] = i; // end point
            }
        }

        return m;
    }


    private void ssSort(final int pa, int first, int last, int buf, int bufSize,
        int depth, int n, boolean lastSuffix)
    {
        if (lastSuffix == true)
          first++;

        int limit = 0;
        int middle = last;

        if ((bufSize < SS_BLOCKSIZE) && (bufSize < last-first))
        {
           limit = ssIsqrt(last-first);

           if (bufSize < limit)
           {
               if (limit > SS_BLOCKSIZE)
                   limit = SS_BLOCKSIZE;

               middle = last - limit;
               buf = middle;
               bufSize = limit;
           }
           else
           {
              limit = 0;
           }
        }

        int a;
        int i = 0;

        for (a=first; middle-a>SS_BLOCKSIZE; a+=SS_BLOCKSIZE, i++)
        {
            this.ssMultiKeyIntroSort(pa, a, a+SS_BLOCKSIZE, depth);
            int curBufSize = last - (a + SS_BLOCKSIZE);
            final int curBuf;

            if (curBufSize > bufSize)
            {
               curBuf = a + SS_BLOCKSIZE;
            }
            else
            {
                curBufSize = bufSize;
                curBuf = buf;
            }
            
            int k = SS_BLOCKSIZE;
            int b = a;

            for (int j=i; (j&1) != 0; j>>=1)
            {
               this.ssSwapMerge(pa, b-k, b, b+k, curBuf, curBufSize, depth);
               b -= k;
               k <<= 1;
            }
        }

        this.ssMultiKeyIntroSort(pa, a, middle, depth);

        for (int k=SS_BLOCKSIZE; i!=0; k<<=1, i>>=1)
        {
            if ((i & 1) == 0)
               continue;

            this.ssSwapMerge(pa, a-k, a, middle, buf, bufSize, depth);
            a -= k;
        }

        if (limit != 0)
        {
            this.ssMultiKeyIntroSort(pa, middle, last, depth);
            this.ssInplaceMerge(pa, first, middle, last, depth);
        }

        if (lastSuffix == true)
        {
            i = this.sa[first-1];
            final int p1 = this.sa[pa+i];
            final int p11 = n - 2;

            for (a=first; (a<last) && ((this.sa[a]<0) || (this.ssCompare(p1, p11, pa+this.sa[a], depth)>0)); a++)
                this.sa[a-1] = this.sa[a];

            this.sa[a-1] = i;
        }
    }


    private int ssCompare(int pa, int pb, int p2, int depth)
    {
        int u1 = depth + pa;
        int u2 = depth + this.sa[p2];
        final int u1n = pb + 2;
        final int u2n = this.sa[p2+1] + 2;

        if (u1n - u1 > u2n - u2)
        {
           while ((u2 < u2n) && (this.buffer[u1] == this.buffer[u2]))
           {
              u1++;
              u2++;
           }
        }
        else
        {
           while ((u1 < u1n) && (this.buffer[u1] == this.buffer[u2]))
           {
              u1++;
              u2++;
           }
        }

        return (u1 < u1n) ? ((u2 < u2n) ? this.buffer[u1] - this.buffer[u2] : 1) : ((u2 < u2n) ? -1 : 0);
    }


    private int ssCompare(int p1, int p2, int depth)
    {
        int u1 = depth + this.sa[p1];
        int u2 = depth + this.sa[p2];
        final int u1n = this.sa[p1+1] + 2;
        final int u2n = this.sa[p2+1] + 2;

        if (u1n - u1 > u2n - u2)
        {
           while ((u2 < u2n) && (this.buffer[u1] == this.buffer[u2]))
           {
              u1++;
              u2++;
           }
        }
        else
        {
           while ((u1 < u1n) && (this.buffer[u1] == this.buffer[u2]))
           {
              u1++;
              u2++;
           }
        }

        return (u1 < u1n) ? ((u2 < u2n) ? this.buffer[u1] - this.buffer[u2] : 1) : ((u2 < u2n) ? -1 : 0);
    }


    private void ssInplaceMerge(int pa, int first, int middle, int last, int depth)
    {
        final int[] arr = this.sa;

        while (true)
        {
            int p, x;

            if (arr[last-1] < 0)
            {
                x = 1;
                p = pa + ~arr[last-1];
            }
            else
            {
                x = 0;
                p = pa + arr[last-1];
            }

            int a = first;
            int r = -1;

            for (int len=middle-first, half=(len>>1); len>0; len=half, half>>=1)
            {
                final int b = a + half;
                final int q = ssCompare(pa + ((arr[b] >= 0) ? arr[b] : ~arr[b]), p, depth);

                if (q < 0)
                {
                    a = b + 1;
                    half -= ((len & 1) ^ 1);
                }
                else
                    r = q;
            }

            if (a < middle)
            {
                if (r == 0)
                    arr[a] = ~arr[a];

                this.ssRotate(a, middle, last);
                last -= (middle - a);
                middle = a;

                if (first == middle)
                    break;
            }

            last--;

            if (x != 0)
            {
                last--;

                while (arr[last] < 0)
                   last--;
            }

            if (middle == last)
                break;
        }
    }


    private void ssRotate(int first, int middle, int last)
    {
        int l = middle - first;
        int r = last - middle;
        final int[] arr = this.sa;

        while ((l > 0) && (r > 0))
        {
            if (l == r)
            {
                ssBlockSwap(first, middle, l);
                break;
            }

            if (l < r)
            {
                int a = last - 1;
                int b = middle - 1;
                int t = arr[a];

                while (true)
                {
                    arr[a--] = arr[b];
                    arr[b--] = arr[a];

                    if (b < first)
                    {
                        arr[a] = t;
                        last = a;
                        r -= (l+1);

                        if (r <= l)
                            break;

                        a--;
                        b = middle - 1;
                        t = arr[a];
                    }
                }
            }
            else
            {
                int a = first;
                int b = middle;
                int t = arr[a];

                while (true)
                {
                    arr[a++] = arr[b];
                    arr[b++] = arr[a];

                    if (last <= b)
                    {
                        arr[a] = t;
                        first = a + 1;
                        l -= (r+1);

                        if (l <= r)
                            break;

                        a++;
                        b = middle;
                        t = arr[a];
                    }
                }
            }
        }
    }


    private void ssBlockSwap(int a, int b, int n)
    {
        while (n > 0)
        {
            final int t = this.sa[a];
            this.sa[a] = this.sa[b];
            this.sa[b] = t;
            n--;
            a++;
            b++;
        }
    }


    private static int getIndex(int a)
    {
        return (a >= 0) ? a : ~a;
    }


    private void ssSwapMerge(int pa, int first, int middle, int last, int buf,
        int bufSize, int depth)
    {
        final int[] arr = this.sa;
        int check = 0;

        while (true)
        {
            if (last - middle <= bufSize)
            {
                if ((first < middle) && (middle < last))
                    this.ssMergeBackward(pa, first, middle, last, buf, depth);

                if (((check & 1) != 0)
                     || (((check & 2) != 0) && (this.ssCompare(pa+getIndex(this.sa[first-1]),
                       pa+arr[first], depth) == 0)))
                {
                    arr[first] = ~arr[first];
                }

                if (((check & 4) != 0)
                    && ((this.ssCompare(pa+getIndex(arr[last-1]), pa+arr[last], depth) == 0)))
                {
                    arr[last] = ~arr[last];
                }

                StackElement se = this.mergeStack.pop();

                if (se == null)
                   return;

                first = se.a;
                middle = se.b;
                last = se.c;
                check = se.d;
                continue;
            }

            if (middle - first <= bufSize)
            {
                if (first < middle)
                    this.ssMergeForward(pa, first, middle, last, buf, depth);

                if (((check & 1) != 0)
                    || (((check & 2) != 0) && (ssCompare(pa+getIndex(arr[first-1]),
                        pa+arr[first], depth) == 0)))
                {
                    arr[first] = ~arr[first];
                }

                if (((check & 4) != 0)
                    && ((ssCompare(pa+getIndex(arr[last-1]), pa+arr[last], depth) == 0)))
                {
                    arr[last] = ~arr[last];
                }

                StackElement se = this.mergeStack.pop();

                if (se == null)
                   return;

                first = se.a;
                middle = se.b;
                last = se.c;
                check = se.d;
                continue;
            }

            int len = (middle - first < last - middle) ? middle - first : last - middle;
            int m = 0;

            for (int half=len>>1; len>0; len=half, half>>=1)
            {
                if (ssCompare(pa+getIndex(arr[middle+m+half]), pa+getIndex(arr[middle-m-half-1]), depth) < 0)
                {
                    m += (half + 1);
                    half -= ((len & 1) ^ 1);
                }
            }

            if (m > 0)
            {
                int lm = middle - m;
                int rm = middle + m;
                this.ssBlockSwap(lm, middle, m);
                int l = middle;
                int r = l;
                int next = 0;

                if (rm < last)
                {
                    if (arr[rm] < 0)
                    {
                        arr[rm] = ~arr[rm];

                        if (first < lm)
                        {
                            l--;

                            while (arr[l] < 0)
                               l--;

                            next |= 4;
                        }

                        next |= 1;
                    }
                    else if (first < lm)
                    {
                        while (arr[r] < 0)
                           r++;

                        next |= 2;
                    }
                }

                if (l - first <= last - r)
                {
                    this.mergeStack.push(r, rm, last, (next & 3) | (check & 4), 0);
                    middle = lm;
                    last = l;
                    check = (check & 3) | (next & 4);
                }
                else
                {
                    if ((r == middle) && ((next & 2) != 0))
                        next ^= 6;

                    this.mergeStack.push(first, lm, l, (check & 3) | (next & 4), 0);
                    first = r;
                    middle = rm;
                    check = (next & 3) | (check & 4);
                }
            }
            else
            {
                if (this.ssCompare(pa+getIndex(arr[middle-1]), pa + arr[middle], depth) == 0)
                {
                    arr[middle] = ~arr[middle];
                }

                if (((check & 1) != 0)
                    || (((check & 2) != 0) && (this.ssCompare(pa+getIndex(this.sa[first-1]),
                        pa+arr[first], depth) == 0)))
                {
                    arr[first] = ~arr[first];
                }

                if (((check & 4) != 0)
                    && ((this.ssCompare(pa+getIndex(arr[last-1]), pa+arr[last], depth) == 0)))
                {
                    arr[last] = ~arr[last];
                }

                StackElement se = this.mergeStack.pop();

                if (se == null)
                   return;

                first = se.a;
                middle = se.b;
                last = se.c;
                check = se.d;
            }
        }
    }


    private  void ssMergeForward(int pa, int first, int middle, int last, int buf,
        int depth)
    {
        final int[] arr = this.sa;
        final int bufEnd = buf + middle - first - 1;
        this.ssBlockSwap(buf, first, middle - first);
        int a = first;
        int b = buf;
        int c = middle;
        final int t = arr[a];

        while (true)
        {
            final int r = ssCompare(pa+arr[b], pa+arr[c], depth);

            if (r < 0)
            {
                do
                {
                    arr[a++] = arr[b];

                    if (bufEnd <= b)
                    {
                        arr[bufEnd] = t;
                        return;
                    }

                    arr[b++] = arr[a];
                }
                while (arr[b] < 0);
            }
            else if (r > 0)
            {
                do
                {
                    arr[a++] = arr[c];
                    arr[c++] = arr[a];

                    if (last <= c)
                    {
                        while (b < bufEnd)
                        {
                            arr[a++] = arr[b];
                            arr[b++] = arr[a];
                        }

                        arr[a] = arr[b];
                        arr[b] = t;
                        return;
                    }
                }
                while (arr[c] < 0);
            }
            else
            {
                arr[c] = ~arr[c];

                do
                {
                    arr[a++] = arr[b];

                    if (bufEnd <= b)
                    {
                        arr[bufEnd] = t;
                        return;
                    }

                    arr[b++] = arr[a];
                }
                while (arr[b] < 0);

                do
                {
                    arr[a++] = arr[c];
                    arr[c++] = arr[a];

                    if (last <= c)
                    {
                        while (b < bufEnd)
                        {
                            arr[a++] = arr[b];
                            arr[b++] = arr[a];
                        }

                        arr[a] = arr[b];
                        arr[b] = t;
                        return;
                    }
                }
                while (arr[c] < 0);
            }
        }
    }


    private  void ssMergeBackward(int pa, int first, int middle, int last, int buf,
        int depth)
    {
        final int[] arr = this.sa;
        final int bufEnd = buf + last - middle - 1;
        this.ssBlockSwap(buf, middle, last-middle);
        int x = 0;
        int p1, p2;

        if (arr[bufEnd] < 0)
        {
            p1 = pa + ~arr[bufEnd];
            x |= 1;
        }
        else
            p1 = pa + arr[bufEnd];

        if (arr[middle-1] < 0)
        {
            p2 = pa + ~arr[middle-1];
            x |= 2;
        }
        else
            p2 = pa + arr[middle-1];

        int a = last - 1;
        int b = bufEnd;
        int c = middle - 1;
        final int t = arr[a];

        while (true)
        {
            final int r = this.ssCompare(p1, p2, depth);

            if (r > 0)
            {
                if ((x & 1) != 0)
                {
                    do
                    {
                        arr[a--] = arr[b];
                        arr[b--] = arr[a];
                    }
                    while (arr[b] < 0);

                    x ^= 1;
                }

                arr[a--] = arr[b];

                if (b <= buf)
                {
                    arr[buf] = t;
                    break;
                }

                arr[b--] = arr[a];

                if (arr[b] < 0)
                {
                    p1 = pa + ~arr[b];
                    x |= 1;
                }
                else
                    p1 = pa + arr[b];
            }
            else if (r < 0)
            {
                if ((x & 2) != 0)
                {
                    do
                    {
                        arr[a--] = arr[c];
                        arr[c--] = arr[a];
                    }
                    while (arr[c] < 0);

                    x ^= 2;
                }

                arr[a--] = arr[c];
                arr[c--] = arr[a];

                if (c < first)
                {
                    while (buf < b)
                    {
                        arr[a--] = arr[b];
                        arr[b--] = arr[a];
                    }

                    arr[a] = arr[b];
                    arr[b] = t;
                    break;
                }

                if (arr[c] < 0)
                {
                    p2 = pa + ~arr[c];
                    x |= 2;
                }
                else
                    p2 = pa + arr[c];
            }
            else // r = 0
            {
                if ((x & 1) != 0)
                {
                    do
                    {
                        arr[a--] = arr[b];
                        arr[b--] = arr[a];
                    }
                    while (arr[b] < 0);

                    x ^= 1;
                }

                arr[a--] = ~arr[b];

                if (b <= buf)
                {
                    arr[buf] = t;
                    break;
                }

                arr[b--] = arr[a];

                if ((x & 2) != 0)
                {
                    do
                    {
                        arr[a--] = arr[c];
                        arr[c--] = arr[a];
                    }
                    while (arr[c] < 0);

                    x ^= 2;
                }

                arr[a--] = arr[c];
                arr[c--] = arr[a];

                if (c < first)
                {
                    while (buf < b)
                    {
                        arr[a--] = arr[b];
                        arr[b--] = arr[a];
                    }

                    arr[a] = arr[b];
                    arr[b] = t;
                    break;
                }

                if (arr[b] < 0)
                {
                    p1 = pa + ~arr[b];
                    x |= 1;
                }
                else
                    p1 = pa + arr[b];

                if (arr[c] < 0)
                {
                    p2 = pa + ~arr[c];
                    x |= 2;
                }
                else
                    p2 = pa + arr[c];
            }
        }
    }

    
    private void ssInsertionSort(int pa, int first, int last, int depth)
    {
        final int[] arr = this.sa;

        for (int i=last-2; i >= first; i--)
        {
            final int t = pa + arr[i];
            int j = i + 1;
            int r;

            while ((r = this.ssCompare(t, pa+arr[j], depth)) > 0)
            {
                do
                {
                    arr[j-1] = arr[j];
                    j++;
                }
                while ((j < last) && (arr[j] < 0));

                if (j >= last)
                    break;
            }

            if (r == 0)
                arr[j] = ~arr[j];

            arr[j-1] = t - pa;
        }
    }


    private static int ssIsqrt(int x)
    {
        if (x >= (SS_BLOCKSIZE * SS_BLOCKSIZE))
            return SS_BLOCKSIZE;

        final int e = ((x & 0xFFFF0000) != 0) ? (((x & 0xFF000000) != 0) ? 24 + LOG_TABLE[(x>>24) & 0xFF]
            : 16 + LOG_TABLE[(x>>16) & 0xFF])
            : (((x & 0x0000FF00) != 0) ? 8 + LOG_TABLE[(x>>8) & 0xFF]
                : LOG_TABLE[x & 0xFF]);

        if (e < 8)
           return SQQ_TABLE[x] >> 4;

        int y;

        if (e >= 16)
        {
            y = SQQ_TABLE[x>>((e-6)-(e&1))] << ((e>>1)-7);

            if (e >= 24)
            {
                y = (y + 1 + x / y) >> 1;
            }

            y = (y + 1 + x / y) >> 1;
        }
        else
        {
            y = (SQQ_TABLE[x>>((e-6)-(e&1))] >> (7-(e>>1))) + 1;
        }

        return (x < y*y) ? y-1 : y;
    }


    private void ssMultiKeyIntroSort(final int pa, int first, int last, int depth)
    {
        int limit = ssIlg(last-first);
        int x = 0;

        while (true)
        {
            if (last - first <= SS_INSERTIONSORT_THRESHOLD)
            {
                if (last - first > 1)
                    this.ssInsertionSort(pa, first, last, depth);

                StackElement se = this.ssStack.pop();

                if (se == null)
                   return;

                first = se.a;
                last = se.b;
                depth = se.c;
                limit = se.d;
                continue;
            }

            final int idx = depth;
            
            if (limit == 0)
                this.ssHeapSort(idx, pa, first, last - first);

            limit--;
            int a;

            if (limit < 0)
            {
                int v = this.buffer[idx+this.sa[pa+this.sa[first]]];

                for (a=first+1; a<last; a++)
                {
                    if ((x=this.buffer[idx+this.sa[pa+this.sa[a]]]) != v)
                    {
                       if (a - first > 1)
                            break;

                        v = x;
                        first = a;
                    }
                }

                if (this.buffer[idx+this.sa[pa+this.sa[first]]-1] < v)
                    first = this.ssPartition(pa, first, a, depth);

                if (a - first <= last - a)
                {
                    if (a - first > 1)
                    {
                        this.ssStack.push(a, last, depth, -1, 0);
                        last = a;
                        depth++;
                        limit = ssIlg(a-first);
                    }
                    else
                    {
                        first = a;
                        limit = -1;
                    }
                }
                else
                {
                    if (last - a > 1)
                    {
                        this.ssStack.push(first, a, depth+1, ssIlg(a-first), 0);
                        first = a;
                        limit = -1;
                    }
                    else
                    {
                        last = a;
                        depth++;
                        limit = ssIlg(a-first);
                    }
                }

                continue;
            }

            // choose pivot
            a = this.ssPivot(idx, pa, first, last);
            final int v = this.buffer[idx+this.sa[pa+this.sa[a]]];
            this.swapInSA(first, a);
            int b = first;

            // partition
            while (++b < last)
            {
               if ((x=this.buffer[idx+this.sa[pa+this.sa[b]]]) != v)
                  break;
            }

            a = b;

            if ((a < last) && (x < v))
            {
                while (++b < last)
                {
                    if ((x=this.buffer[idx+this.sa[pa+this.sa[b]]]) > v)
                       break;
                    
                    if (x == v)
                    {
                        this.swapInSA(b, a);
                        a++;
                    }
                }
            }

            int c = last;

            while (--c > b)
            {
               if ((x=this.buffer[idx+this.sa[pa+this.sa[c]]]) != v)
                  break;
            }

            int d = c;

            if ((b < d) && (x > v))
            {
                while (--c > b)
                {
                    if ((x=this.buffer[idx+this.sa[pa+this.sa[c]]]) < v)
                       break;
                    
                    if (x == v)
                    {
                        this.swapInSA(c, d);
                        d--;
                    }
                }
            }

            while (b < c)
            {
                this.swapInSA(b, c);

                while (++b < c)
                {
                    if ((x=this.buffer[idx+this.sa[pa+this.sa[b]]]) > v)
                       break;
                    
                    if (x == v)
                    {
                        this.swapInSA(b, a);
                        a++;
                    }
                }

                while (--c > b)
                {
                    if ((x=this.buffer[idx+this.sa[pa+this.sa[c]]]) < v)
                       break;
                    
                    if (x == v)
                    {
                        this.swapInSA(c, d);
                        d--;
                    }
                }
            }

            if (a <= d)
            {
                c = b - 1;
                int s = (a - first > b - a) ? b - a : a - first;

                for (int e=first, f=b-s; s>0; s--, e++, f++)
                    this.swapInSA(e, f);

                s = (d - c > last - d - 1) ? last - d - 1 : d - c;

                for (int e=b, f=last-s; s>0; s--, e++, f++)
                    this.swapInSA(e, f);

                a = first + (b - a);
                c = last - (d - c);
                b = (v <= this.buffer[idx+this.sa[pa+this.sa[a]]-1]) ? a : this.ssPartition(pa, a, c, depth);

                if (a - first <= last - c)
                {
                    if (last - c <= c - b)
                    {
                        this.ssStack.push(b, c, depth+1, ssIlg(c-b), 0);
                        this.ssStack.push(c, last, depth, limit, 0);
                        last = a;
                    }
                    else if (a - first <= c - b)
                    {
                        this.ssStack.push(c, last, depth, limit, 0);
                        this.ssStack.push(b, c, depth+1, ssIlg(c-b), 0);
                        last = a;
                    }
                    else
                    {
                        this.ssStack.push(c, last, depth, limit, 0);
                        this.ssStack.push(first, a, depth, limit, 0);
                        first = b;
                        last = c;
                        depth++;
                        limit = ssIlg(c-b);
                    }
                }
                else
                {
                    if (a - first <= c - b)
                    {
                        this.ssStack.push(b, c, depth+1, ssIlg(c-b), 0);
                        this.ssStack.push(first, a, depth, limit, 0);
                        first = c;
                    }
                    else if (last - c <= c - b)
                    {
                        this.ssStack.push(first, a, depth, limit, 0);
                        this.ssStack.push(b, c, depth + 1, ssIlg(c-b), 0);
                        first = c;
                    }
                    else
                    {
                        this.ssStack.push(first, a, depth, limit, 0);
                        this.ssStack.push(c, last, depth, limit, 0);
                        first = b;
                        last = c;
                        depth++;
                        limit = ssIlg(c-b);
                    }
                }
            }
            else
            {
                if (this.buffer[idx+this.sa[pa+this.sa[first]]-1] < v)
                {
                    first = this.ssPartition(pa, first, last, depth);
                    limit = ssIlg(last-first);
                }
                else
                {
                    limit++;
                }
                
                depth++;
            }
        }
    }


    private int ssPivot(int td, int pa, int first, int last)
    {
        int t = last - first;
        int middle = first + (t>>1);

        if (t <= 512)
        {
           return (t <= 32) ? this.ssMedian3(td, pa, first, middle, last-1) :
                    this.ssMedian5(td, pa, first, first+(t>>2), middle, last-1-(t>>2), last-1);
        }

        t >>= 3;
        first = this.ssMedian3(td, pa, first, first+t, first+(t<<1));
        middle = this.ssMedian3(td, pa, middle-t, middle, middle+t);
        last = this.ssMedian3(td, pa, last-1-(t<<1), last-1-t, last-1);
        return this.ssMedian3(td, pa, first, middle, last);
    }


    private int ssMedian5(final int idx, int pa, int v1, int v2, int v3, int v4, int v5)
    {
        final int b1 = this.buffer[idx+this.sa[pa+this.sa[v1]]];
        final int b2 = this.buffer[idx+this.sa[pa+this.sa[v2]]];
        final int b3 = this.buffer[idx+this.sa[pa+this.sa[v3]]];
        final int b4 = this.buffer[idx+this.sa[pa+this.sa[v4]]];
        
        if (b2 > b3)
        {
            final int t = v2;
            v2 = v3;
            v3 = t;
        }

        if (b4 > this.buffer[idx+this.sa[pa+this.sa[v5]]])
        {
            final int t = v4;
            v4 = v5;
            v5 = t;
        }

        if (b2 > b4)
        {
            final int t1 = v2;
            v2 = v4;
            v4 = t1;
            final int t2 = v3;
            v3 = v5;
            v5 = t2;
        }

        if (b1 > b3)
        {
            final int t = v1;
            v1 = v3;
            v3 = t;
        }

        if (b1 > b4)
        {
            final int t1 = v1;
            v1 = v4;
            v4 = t1;
            final int t2 = v3;
            v3 = v5;
            v5 = t2;
        }

        if (b3 > b4)
            return v4;

        return v3;
    }


    private int ssMedian3(int idx, int pa, int v1, int v2, int v3)
    {
        final int b1 = this.buffer[idx+this.sa[pa+this.sa[v1]]];
        final int b2 = this.buffer[idx+this.sa[pa+this.sa[v2]]];
        final int b3 = this.buffer[idx+this.sa[pa+this.sa[v3]]];

        if (b1 > b2)
        {
            final int t = v1;
            v1 = v2;
            v2 = t;
        }

        if (b2 > b3)
        {
            if (b1 > b3)
                return v1;

            return v3;
        }

        return v2;
    }


    private int ssPartition(int pa, int first, int last, int depth)
    {
        final int[] arr = this.sa;
        int a = first - 1;
        int b = last;
        final int d = depth - 1;
        final int pb = pa + 1;

        while (true)
        {
            a++;
           
            for (; (a < b) && (arr[pa+arr[a]]+d >= arr[pb+arr[a]]); )
            {
                arr[a] = ~arr[a];
                a++;
            }

            b--;
            
            for (; (b > a) && (arr[pa+arr[b]]+d < arr[pb+arr[b]]); )
                b--;
            
            if (b <= a)
                break;

            final int t = ~arr[b];
            arr[b] = arr[a];
            arr[a] = t;
        }

        if (first < a)
            arr[first] = ~arr[first];

        return a;
    }


    private void ssHeapSort(int idx, int pa, int saIdx, int size)
    {
	int m = size;

        if ((size & 1) == 0)
        {
            m--;

            if (this.buffer[idx+this.sa[pa+this.sa[saIdx+(m>>1)]]] < this.buffer[idx+this.sa[pa+this.sa[saIdx+m]]])
                this.swapInSA(saIdx+m, saIdx+(m>>1));
        }

        for (int i=(m>>1)-1; i>=0; i--)
            this.ssFixDown(idx, pa, saIdx, i, m);

        if ((size & 1) == 0)
        {
            this.swapInSA(saIdx, saIdx + m);
            this.ssFixDown(idx, pa, saIdx, 0, m);
        }

        for (int i=m-1; i>0; i--)
        {
            int t = this.sa[saIdx];
            this.sa[saIdx] = this.sa[saIdx+i];
            this.ssFixDown(idx, pa, saIdx, 0, i);
            this.sa[saIdx+i] = t;
        }
    }


    private void ssFixDown(int idx, int pa, int saIdx, int i, int size)
    {
        final int[] arr = this.sa;
        final int v = arr[saIdx+i];
        final int c = this.buffer[idx+arr[pa+v]];
        int j = (i << 1) + 1;

        while (j < size)
        {
            int k = j;
            j++;
            int d = this.buffer[idx+arr[pa+arr[saIdx+k]]];
            final int e = this.buffer[idx+arr[pa+arr[saIdx+j]]];

            if (d < e)
            {
                k = j;
                d = e;
            }

            if (d <= c)
               break;

            arr[saIdx+i] = arr[saIdx+k];
            i = k;
            j = (i << 1) + 1;
        }

        arr[i+saIdx] = v;
    }


    private static int ssIlg(int n)
    {
        return ((n & 0xFF00) != 0) ? 8 + LOG_TABLE[(n>>8) & 0xFF]
            : LOG_TABLE[n & 0xFF];
    }


    private void swapInSA(int a, int b)
    {
        final int tmp = this.sa[a];
        this.sa[a] = this.sa[b];
        this.sa[b] = tmp;
    }


    private void trSort(int n, int depth)
    {
        final int[] arr = this.sa;
        TRBudget budget = new TRBudget(trIlg(n)*2/3, n);

        for (int isad=n+depth; arr[0]>-n; isad+=(isad-n))
        {
            int first = 0;
            int skip = 0;
            int unsorted = 0;

            do
            {
                final int t = arr[first];

                if (t < 0)
                {
                    first -= t;
                    skip += t;
                }
                else
                {
                    if (skip != 0)
                    {
                        arr[first+skip] = skip;
                        skip = 0;
                    }

                    final int last = arr[n+t] + 1;

                    if (last - first > 1)
                    {
                        budget.count = 0;
                        this.trIntroSort(n, isad, first, last, budget);

                        if (budget.count != 0)
                            unsorted += budget.count;
                        else
                            skip = first - last;
                    }
                    else if (last - first == 1)
                        skip = -1;

                    first = last;
                }
            }
            while (first < n);

            if (skip != 0)
                arr[first+skip] = skip;

            if (unsorted == 0)
                break;
        }
    }


    private long trPartition(int isad, int first, int middle, int last, int v)
    {
        int x = 0;
        int b = middle;

        while (b < last)
        {
           x = this.sa[isad + this.sa[b]];

           if (x != v)
              break;

           b++;
        }

        int a = b;

        if ((a < last) && (x < v))
        {
            while ((++b < last) && ((x = this.sa[isad+this.sa[b]]) <= v))
            {
                if (x == v)
                {
                    this.swapInSA(a, b);
                    a++;
                }
            }
        }

        int c = last - 1;

        while (c > b)
        {
            x = this.sa[isad+this.sa[c]];

            if (x != v)
               break;

            c--;
        }

        int d = c;

        if ((b < d) && (x > v))
        {
            while ((--c > b) && ((x = this.sa[isad+this.sa[c]]) >= v))
            {
                if (x == v)
                {
                    this.swapInSA(c, d);
                    d--;
                }
            }
        }

        while (b < c)
        {
            this.swapInSA(c, b);

            while ((++b < c) && ((x = this.sa[isad+this.sa[b]]) <= v))
            {
                if (x == v)
                {
                    this.swapInSA(a, b);
                    a++;
                }
            }

            while ((--c > b) && ((x = this.sa[isad+this.sa[c]]) >= v))
            {
                if (x == v)
                {
                    this.swapInSA(c, d);
                    d--;
                }
            }
        }

        if (a <= d)
        {
            c = b - 1;
            int s = a - first;

            if (s > b - a)
                s = b - a;

            for (int e=first, f=b-s; s>0; s--, e++, f++)
                this.swapInSA(e, f);

            s = d - c;

            if (s >= last - d)
                s = last - d - 1;

            for (int e=b, f=last-s; s>0; s--, e++, f++)
                this.swapInSA(e, f);

            first += (b - a);
            last -= (d - c);
        }

        return (((long) first) << 32) | (((long) last) & 0xFFFFFFFFL);
    }


    private void trIntroSort(int isa, int isad, int first, int last, TRBudget budget)
    {
        final int incr = isad - isa;
        final int[] arr = this.sa;
        int limit = trIlg(last - first);
        int trlink = -1;

        while (true)
        {
            if (limit < 0)
            {
                if (limit == -1)
                {
                    // tandem repeat partition
                    long res = this.trPartition(isad-incr, first, first, last, last-1);
                    final int a = (int) (res >> 32);
                    final int b = (int) res;

                    // update ranks
                    if (a < last)
                    {
                        for (int c=first, v=a-1; c < a; c++)
                            arr[isa+arr[c]] = v;
                    }

                    if (b < last)
                    {
                        for (int c=a, v=b-1; c < b; c++)
                            arr[isa+arr[c]] = v;
                    }

                    // push
                    if (b - a > 1)
                    {
                        this.trStack.push(0, a, b, 0, 0);
                        this.trStack.push(isad-incr, first, last, -2, trlink);
                        trlink = this.trStack.size() - 2;
                    }

                    if (a - first <= last - b)
                    {
                        if (a - first > 1)
                        {
                            this.trStack.push(isad, b, last, trIlg(last-b), trlink);
                            last = a;
                            limit = trIlg(a - first);
                        }
                        else if (last - b > 1)
                        {
                            first = b;
                            limit = trIlg(last - b);
                        }
                        else
                        {
                            StackElement se = this.trStack.pop();

                            if (se == null)
                               return;

                            isad = se.a;
                            first = se.b;
                            last = se.c;
                            limit = se.d;
                            trlink = se.e;
                        }
                    }
                    else
                    {
                        if (last - b > 1)
                        {
                            this.trStack.push(isad, first, a, trIlg(a - first), trlink);
                            first = b;
                            limit = trIlg(last - b);
                        }
                        else if (a - first > 1)
                        {
                            last = a;
                            limit = trIlg(a - first);
                        }
                        else
                        {
                            StackElement se = this.trStack.pop();

                            if (se == null)
                               return;

                            isad = se.a;
                            first = se.b;
                            last = se.c;
                            limit = se.d;
                            trlink = se.e;
                        }
                    }
                }
                else if (limit == -2)
                {
                    // tandem repeat copy
                    StackElement se = this.trStack.pop();
                    
                    if (se.d == 0)
                    {
                       this.trCopy(isa, first, se.b, se.c, last, isad - isa);
                    }
                    else
                    {
                       if (trlink >= 0)
                           this.trStack.get(trlink).d = -1;

                       this.trPartialCopy(isa, first, se.b, se.c, last, isad - isa);
                    }

                    se = this.trStack.pop();

                    if (se == null)
                       return;

                    isad = se.a;
                    first = se.b;
                    last = se.c;
                    limit = se.d;
                    trlink = se.e;
                }
                else
                {
                    // sorted partition
                    if (arr[first] >= 0)
                    {
                        int a = first;

                        do
                        {
                            arr[isa+arr[a]] = a;
                            a++;
                        }
                        while ((a < last) && (arr[a] >= 0));

                        first = a;
                    }

                    if (first < last)
                    {
                        int a = first;

                        do
                        {
                            arr[a] = ~arr[a];
                            a++;
                        }
                        while (arr[a] < 0);

                        int next = (arr[isa+arr[a]] != arr[isad+arr[a]]) ? trIlg(a-first+1) : -1;
                        a++;

                        if (a < last)
                        {
                            final int v = a - 1;

                            for (int b=first; b < a; b++)
                                arr[isa+arr[b]] = v;
                        }

                        // push
                        if (budget.check(a - first) == true)
                        {
                            if (a - first <= last - a)
                            {
                                this.trStack.push(isad, a, last, -3, trlink);
                                isad += incr;
                                last = a;
                                limit = next;
                            }
                            else
                            {
                                if (last - a > 1)
                                {
                                    this.trStack.push(isad+incr, first, a, next, trlink);
                                    first = a;
                                    limit = -3;
                                }
                                else
                                {
                                    isad += incr;
                                    last = a;
                                    limit = next;
                                }
                            }
                        }
                        else
                        {
                            if (trlink >= 0)
                                this.trStack.get(trlink).d = -1;

                            if (last - a > 1)
                            {
                                first = a;
                                limit = -3;
                            }
                            else
                            {
                                StackElement se = this.trStack.pop();

                                if (se == null)
                                   return;

                                isad = se.a;
                                first = se.b;
                                last = se.c;
                                limit = se.d;
                                trlink = se.e;
                            }
                        }
                    }
                    else
                    {
                        StackElement se = this.trStack.pop();

                        if (se == null)
                           return;

                        isad = se.a;
                        first = se.b;
                        last = se.c;
                        limit = se.d;
                        trlink = se.e;
                    }
                }

                continue;
            }

            if (last - first <= TR_INSERTIONSORT_THRESHOLD)
            {
                this.trInsertionSort(isad, first, last);
                limit = -3;
                continue;
            }

            if (limit == 0)
            {
                this.trHeapSort(isad, first, last - first);
                int a = last - 1;

                while (first < a)
                {
                    int b = a - 1;

                    for (int x=arr[isad+arr[a]]; (first<=b) && (arr[isad+arr[b]] == x); b--)
                       arr[b] = ~arr[b];

                    a = b;
                }

                limit = -3;
                continue;
            }

            limit--;

            // choose pivot
            this.swapInSA(first, trPivot(this.sa, isad, first, last));
            int v = arr[isad + arr[first]];

            // partition
            long res = this.trPartition(isad, first, first+1, last, v);
            final int a = (int) (res >> 32);
            final int b = (int) (res & 0xFFFFFFFFL);

            if (last - first != b - a)
            {
                final int next = (arr[isa+arr[a]] != v) ? trIlg(b-a) : -1;
                v = a - 1;

                // update ranks
                for (int c=first; c < a; c++)
                    arr[isa+arr[c]] = v;

                if (b < last)
                {
                    v = b - 1;

                    for (int c=a; c < b; c++)
                       arr[isa+arr[c]] = v;
                }

                // push
                if ((b - a > 1) && (budget.check(b-a) == true))
                {
                    if (a - first <= last - b)
                    {
                        if (last - b <= b - a)
                        {
                            if (a - first > 1)
                            {
                                this.trStack.push(isad+incr, a, b, next, trlink);
                                this.trStack.push(isad, b, last, limit, trlink);
                                last = a;
                            }
                            else if (last - b > 1)
                            {
                                this.trStack.push(isad+incr, a, b, next, trlink);
                                first = b;
                            }
                            else
                            {
                                isad += incr;
                                first = a;
                                last = b;
                                limit = next;
                            }
                        }
                        else if (a - first <= b - a)
                        {
                            if (a - first > 1)
                            {
                                this.trStack.push(isad, b, last, limit, trlink);
                                this.trStack.push(isad+incr, a, b, next, trlink);
                                last = a;
                            }
                            else
                            {
                                this.trStack.push(isad, b, last, limit, trlink);
                                isad += incr;
                                first = a;
                                last = b;
                                limit = next;
                            }
                        }
                        else
                        {
                            this.trStack.push(isad, b, last, limit, trlink);
                            this.trStack.push(isad, first, a, limit, trlink);
                            isad += incr;
                            first = a;
                            last = b;
                            limit = next;
                        }
                    }
                    else
                    {
                        if (a - first <= b - a)
                        {
                            if (last - b > 1)
                            {
                                this.trStack.push(isad+incr, a, b, next, trlink);
                                this.trStack.push(isad, first, a, limit, trlink);
                                first = b;
                            }
                            else if (a - first > 1)
                            {
                                this.trStack.push(isad+incr, a, b, next, trlink);
                                last = a;
                            }
                            else
                            {
                                isad += incr;
                                first = a;
                                last = b;
                                limit = next;
                            }
                        }
                        else if (last - b <= b - a)
                        {
                            if (last - b > 1)
                            {
                                this.trStack.push(isad, first, a, limit, trlink);
                                this.trStack.push(isad+incr, a, b, next, trlink);
                                first = b;
                            }
                            else
                            {
                                this.trStack.push(isad, first, a, limit, trlink);
                                isad += incr;
                                first = a;
                                last = b;
                                limit = next;
                            }
                        }
                        else
                        {
                            this.trStack.push(isad, first, a, limit, trlink);
                            this.trStack.push(isad, b, last, limit, trlink);
                            isad += incr;
                            first = a;
                            last = b;
                            limit = next;
                        }
                    }
                }
                else
                {
                    if ((b - a > 1) && (trlink >= 0))
                        this.trStack.get(trlink).d = -1;

                    if (a - first <= last - b)
                    {
                        if (a - first > 1)
                        {
                            this.trStack.push(isad, b, last, limit, trlink);
                            last = a;
                        }
                        else if (last - b > 1)
                        {
                            first = b;
                        }
                        else
                        {
                           StackElement se = this.trStack.pop();

                           if (se == null)
                              return;

                           isad = se.a;
                           first = se.b;
                           last = se.c;
                           limit = se.d;
                           trlink = se.e;
                        }
                    }
                    else
                    {
                        if (last - b > 1)
                        {
                            this.trStack.push(isad, first, a, limit, trlink);
                            first = b;
                        }
                        else if (a - first > 1)
                        {
                            last = a;
                        }
                        else
                        {
                            StackElement se = this.trStack.pop();

                            if (se == null)
                               return;

                            isad = se.a;
                            first = se.b;
                            last = se.c;
                            limit = se.d;
                            trlink = se.e;
                        }
                    }
                }
            }
            else
            {
                if (budget.check(last - first) == true)
                {
                    limit = trIlg(last - first);
                    isad += incr;
                }
                else
                {
                    if (trlink >= 0)
                        this.trStack.get(trlink).d = -1;

                    StackElement se = this.trStack.pop();

                    if (se == null)
                       return;

                    isad = se.a;
                    first = se.b;
                    last = se.c;
                    limit = se.d;
                    trlink = se.e;
                }
            }
        }
    }


    private static int trPivot(int[] arr, int isad, int first, int last)
    {
        int t = last - first;
        int middle = first + (t>>1);

        if (t <= 512)
        {
            if (t <= 32)
               return trMedian3(arr, isad, first, middle, last - 1);

            t >>= 2;
            return trMedian5(arr, isad, first, first+t, middle, last-1-t, last-1);
        }

        t >>= 3;
        first = trMedian3(arr, isad, first, first+t, first+(t<<1));
        middle = trMedian3(arr, isad, middle-t, middle, middle+t);
        last = trMedian3(arr, isad, last-1-(t<<1), last-1-t, last-1);
        return trMedian3(arr, isad, first, middle, last);
    }


    private static int trMedian5(int[] arr, int isad, int v1, int v2, int v3, int v4, int v5)
    {
        if (arr[isad+arr[v2]] > arr[isad+arr[v3]])
        {
            int t = v2;
            v2 = v3;
            v3 = t;
        }

        if (arr[isad+arr[v4]] > arr[isad+arr[v5]])
        {
            final int t = v4;
            v4 = v5;
            v5 = t;
        }

        if (arr[isad+arr[v2]] > arr[isad+arr[v4]])
        {
            final int t1 = v2;
            v2 = v4;
            v4 = t1;
            final int t2 = v3;
            v3 = v5;
            v5 = t2;
        }

        if (arr[isad+arr[v1]] > arr[isad+arr[v3]])
        {
            final int t = v1;
            v1 = v3;
            v3 = t;
        }

        if (arr[isad+arr[v1]] > arr[isad+arr[v4]])
        {
            final int t1 = v1;
            v1 = v4;
            v4 = t1;
            final int t2 = v3;
            v3 = v5;
            v5 = t2;
        }

        if (arr[isad+arr[v3]] > arr[isad+arr[v4]])
            return v4;

        return v3;
    }


    private static int trMedian3(int[] arr, int isad, int v1, int v2, int v3)
    {
        if (arr[isad+arr[v1]] > arr[isad+arr[v2]])
        {
            final int t = v1;
            v1 = v2;
            v2 = t;
        }

        if (arr[isad+arr[v2]] > arr[isad+arr[v3]])
        {
            if (arr[isad+arr[v1]] > arr[isad+arr[v3]])
                return v1;

            return v3;
        }

        return v2;
    }


    private void trHeapSort(int isad, int saIdx, int size)
    {
        final int[] arr = this.sa;
        int m = size;

        if ((size & 1) == 0)
        {
            m--;

            if (arr[isad + arr[saIdx+(m>>1)]] < arr[isad + arr[saIdx+m]])
                this.swapInSA(saIdx+m, saIdx+(m>>1));
        }

        for (int i=(m>>1)-1; i>=0; i--)
            this.trFixDown(isad, saIdx, i, m);

        if ((size & 1) == 0)
        {
            this.swapInSA(saIdx, saIdx+m);
            this.trFixDown(isad, saIdx, 0, m);
        }

        for (int i=m-1; i>0; i--)
        {
            final int t = arr[saIdx];
            arr[saIdx] = arr[saIdx+i];
            this.trFixDown(isad, saIdx, 0, i);
            arr[saIdx+i] = t;
        }
    }


    private void trFixDown(int isad, int saIdx, int i, int size)
    {
        final int[] arr = this.sa;
        final int v = arr[saIdx+i];
        final int c = arr[isad+v];
        int j = (i << 1) + 1;

        while (j < size)
        {
            int k = j;
            j++;
            int d = arr[isad+arr[saIdx+k]];
            final int e = arr[isad+arr[saIdx+j]];

            if (d < e)
            {
                k = j;
                d = e;
            }

            if (d <= c)
                break;

            arr[saIdx+i] = arr[saIdx+k];
            i = k;
            j = (i << 1) + 1;
        }

        arr[saIdx+i] = v;
    }


    private void trInsertionSort(int isad, int first, int last)
    {
        final int[] arr = this.sa;

        for (int a=first+1; a<last; a++)
        {
            int b = a - 1;
            final int t = arr[a];
            int r = arr[isad+t] - arr[isad+arr[b]];

            while (r < 0)
            {
                do
                {
                    arr[b+1] = arr[b];
                    b--;
                }
                while ((b >= first) && (arr[b] < 0));

                if (b < first)
                    break;

                r = arr[isad+t] - arr[isad+arr[b]];
            }

            if (r == 0)
                arr[b] = ~arr[b];

            arr[b+1] = t;
        }
    }


    private void trPartialCopy(int isa, int first, int a, int b, int last, int depth)
    {
        final int[] arr = this.sa;
	int v = b - 1;
	int lastRank = -1;
	int newRank = -1;
	int d = a - 1;

        for (int c=first; c<=d; c++)
        {
            final int s = arr[c] - depth;

            if ((s >= 0) && (arr[isa+s] == v))
            {
                d++;
                arr[d] = s;
                final int rank = arr[isa+s+depth];

                if (lastRank != rank)
                {
                    lastRank = rank;
                    newRank = d;
                }

                arr[isa+s] = newRank;
            }
        }

        lastRank = -1;

        for (int e=d; first<=e; e--)
        {
            final int rank = arr[isa+arr[e]];

            if (lastRank != rank)
            {
                lastRank = rank;
                newRank = e;
            }

            if (newRank != rank)
            {
                arr[isa+arr[e]] = newRank;
            }
        }

        lastRank = -1;
        d = b;

        for (int c=last-1, e=d+1; e<d; c--)
        {
            final int s = arr[c] - depth;

            if ((s >= 0) && (arr[isa+s] == v))
            {
                d--;
                arr[d] = s;
                final int rank = arr[isa+s+depth];

                if (lastRank != rank)
                {
                    lastRank = rank;
                    newRank = d;
                }

                arr[isa+s] = newRank;
            }
        }
    }


    private void trCopy(int isa, int first, int a, int b, int last, int depth)
    {
        final int[] arr = this.sa;
        int v = b - 1;
        int d = a - 1;

        for (int c=first; c<=d; c++)
        {
            int s = arr[c] - depth;

            if ((s >= 0) && (arr[isa+s] == v))
            {
                d++;
                arr[d] = s;
                arr[isa+s] = d;
            }
        }

        final int e = d + 1;
        d = b;

        for (int c=last-1; d>e; c--)
        {
            final int s = arr[c] - depth;

            if ((s >= 0) && (arr[isa+s] == v))
            {
                d--;
                arr[d] = s;
                arr[isa+s] = d;
            }
        }
    }


    private static int trIlg(int n)
    {
        return ((n & 0xFFFF0000) != 0) ? (((n & 0xFF000000) != 0) ? 24 + LOG_TABLE[(n>>24) & 0xFF]
            : 16 + LOG_TABLE[(n>>16) & 0xFF])
            : (((n & 0x0000FF00) != 0) ? 8 + LOG_TABLE[(n>>8) & 0xFF]
                : LOG_TABLE[n & 0xFF]);
    }




    private static class StackElement
    {
        int a, b, c, d, e;
    }


    // A stack of pre-allocated elements
    private static class Stack
    {
       private final StackElement[] array;
       private int index;

       Stack(int size)
       {
          this.array = new StackElement[size];

          for (int i=0; i<size; i++)
             this.array[i] = new StackElement();
       }

       StackElement get(int idx)
       {
          return this.array[idx];
       }

       int size()
       {
          return this.index;
       }

       void push(int a, int b, int c, int d, int e)
       {
          StackElement elt = this.array[this.index];
          elt.a = a;
          elt.b = b;
          elt.c = c;
          elt.d = d;
          elt.e = e;
          this.index++;
       }

       StackElement pop()
       {
          return (this.index == 0) ? null : this.array[--this.index];
       }
    }


    private static class TRBudget
    {
        int chance;
        int remain;
        int incVal;
        int count;

        private TRBudget(int chance, int incval)
        {
            this.chance = chance;
            this.remain = incval;
            this.incVal = incval;
        }

        private boolean check(int size)
        {
            if (size <= this.remain)
            {
                this.remain -= size;
                return true;
            }

            if (this.chance == 0)
            {
                this.count += size;
                return false;
            }

            this.remain += (this.incVal - size);
            this.chance--;
            return true;
        }
    }

}
