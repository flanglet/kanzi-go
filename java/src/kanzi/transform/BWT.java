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

import kanzi.ByteTransform;
import kanzi.SliceByteArray;


// The Burrows-Wheeler Transform is a reversible transform based on
// permutation of the data in the original message to reduce the entropy.

// The initial text can be found here:
// Burrows M and Wheeler D, [A block sorting lossless data compression algorithm]
// Technical Report 124, Digital Equipment Corporation, 1994

// See also Peter Fenwick, [Block sorting text compression - final report]
// Technical Report 130, 1996

// This implementation replaces the 'slow' sorting of permutation strings
// with the construction of a suffix array (faster but more complex).
// The suffix array contains the indexes of the sorted suffixes.
// The BWT may be split in chunks (depending of block size). In this case, 
// several 'primary indexes' (one for each chunk) is kept and the inverse
// can be processed in parallel; each chunk being inverted concurrently.
//
// E.G.    0123456789A
// Source: mississippi\0
// Suffixes:    rank  sorted
// mississippi\0  0  -> 4             i\0
//  ississippi\0  1  -> 3          ippi\0
//   ssissippi\0  2  -> 10      issippi\0
//    sissippi\0  3  -> 8    ississippi\0
//     issippi\0  4  -> 2   mississippi\0 
//      ssippi\0  5  -> 9            pi\0
//       sippi\0  6  -> 7           ppi\0
//        ippi\0  7  -> 1         sippi\0
//         ppi\0  8  -> 6      sissippi\0
//          pi\0  9  -> 5        ssippi\0
//           i\0  10 -> 0     ssissippi\0
// Suffix array SA : 10 7 4 1 0 9 8 6 3 5 2 
// BWT[i] = input[SA[i]-1] => BWT(input) = pssm[i]pissii (+ primary index 4) 
// The suffix array and permutation vector are equal when the input is 0 terminated
// The insertion of a guard is done internally and is entirely transparent.
//
// See https://code.google.com/p/libdivsufsort/source/browse/wiki/SACA_Benchmarks.wiki
// for respective performance of different suffix sorting algorithms.

public class BWT implements ByteTransform
{
   private static final int MAX_BLOCK_SIZE = 1024*1024*1024; // 1 GB (30 bits)
   private static final int BWT_MAX_HEADER_SIZE  = 4;

   private int[] buffer1;   // Only used in inverse
   private byte[] buffer2;  // Only used for big blocks (size >= 1<<24)
   private final int[] buckets;
   private final int[] primaryIndexes;
   private DivSufSort saAlgo;

    
    // Static allocation of memory
    public BWT()
    {
       this.buffer1 = new int[0];  // Allocate empty: only used in inverse
       this.buffer2 = new byte[0]; // Allocate empty: only used for big blocks (size >= 1<<24)
       this.buckets = new int[256];
       this.primaryIndexes = new int[9];
    }


    public int getPrimaryIndex(int n)
    {
       return this.primaryIndexes[n];
    }


    // Not thread safe
    public boolean setPrimaryIndex(int n, int primaryIndex)
    {
       if ((primaryIndex < 0) || (n < 0) || (n >= this.primaryIndexes.length))
          return false;

       this.primaryIndexes[n] = primaryIndex;
       return true;
    }
    
    
    // Not thread safe
    @Override
    public boolean forward(SliceByteArray src, SliceByteArray dst)
    {
        if ((!SliceByteArray.isValid(src)) || (!SliceByteArray.isValid(dst)))
           return false;

        if (src.array == dst.array)
           return false;
        
        final int count = src.length;
        
        if (count > maxBlockSize())
           return false;

        if (dst.index + count > dst.array.length)
           return false;
       
        final byte[] input = src.array;
        final byte[] output = dst.array;
        final int srcIdx = src.index;
        final int dstIdx = dst.index;

        if (count < 2)
        {
           if (count == 1)
              output[dst.index++] = input[src.index++];

           return true;
        }
       
        // Lazy dynamic memory allocation 
        if (this.saAlgo == null)
           this.saAlgo = new DivSufSort(); 

        if (this.buffer1.length < count)
           this.buffer1 = new int[count];
        
        final int[] sa = this.buffer1;       
        this.saAlgo.computeSuffixArray(input, sa, srcIdx, count);
        final int srcIdx2 = srcIdx - 1; 
        int n = 0;
        final int chunks = getBWTChunks(count);
        boolean res = true;  
  
        if (chunks == 1)
        {
            for (; n<count; n++) 
            {                     
               if (sa[n] == 0)
               {
                  res &= this.setPrimaryIndex(0, n); 
                  break;
               }
               
               output[dstIdx+n] = input[srcIdx2+sa[n]];
            }

            output[dstIdx+n] = input[srcIdx2+count];
            n++;

            for (; n<count; n++) 
               output[dstIdx+n] = input[srcIdx2+sa[n]];
        }
        else
        {
            final int step = count / chunks;

            for (; n<count; n++) 
            {      
               if ((sa[n]%step) == 0) 
               {
                   res &= this.setPrimaryIndex(sa[n]/step, n);

                   if (sa[n] == 0)
                      break;
               }

               output[dstIdx+n] = input[srcIdx2+sa[n]];
            }

            output[dstIdx+n] = input[srcIdx2+count];
            n++;

            for (; n<count; n++) 
            {
               if ((sa[n]%step) == 0)
                   res &= this.setPrimaryIndex(sa[n]/step, n);

               output[dstIdx+n] = input[srcIdx2+sa[n]];
            }
        }
        
        src.index += count;
        dst.index += count;
        return res;
    }


    // Not thread safe
    @Override
    public boolean inverse(SliceByteArray src, SliceByteArray dst)
    {
       if ((!SliceByteArray.isValid(src)) || (!SliceByteArray.isValid(dst)))
           return false;

       if (src.array == dst.array)
           return false;
        
       final int count = src.length;
        
       if (count > maxBlockSize())
           return false;
              
       if (dst.index + count > dst.array.length)
           return false;      
       
       if (count < 2)
       {
          if (count == 1)
             dst.array[dst.index++] = src.array[src.index++];

          return true;
       }

       if (count >= 1<<24)
          return this.inverseBigBlock(src, dst, count);
       
       return this.inverseRegularBlock(src, dst, count);
    }
    
    
    // When count < 1<<24
    private boolean inverseRegularBlock(SliceByteArray src, SliceByteArray dst, final int count)
    {
       final byte[] input = src.array;
       final byte[] output = dst.array;
       final int srcIdx = src.index;
       final int dstIdx = dst.index;
       
       // Lazy dynamic memory allocation
       if (this.buffer1.length < count)
          this.buffer1 = new int[count];

       // Aliasing
       final int[] buckets_ = this.buckets;
       final int[] data = this.buffer1;
       
       // Initialize histogram
       for (int i=0; i<256; i++)
          buckets_[i] = 0;
       
       final int chunks = getBWTChunks(count);
    
       // Build array of packed index + value (assumes block size < 2^24)
       // Start with the primary index position
       int pIdx = this.getPrimaryIndex(0);
       final int val0 = input[srcIdx+pIdx] & 0xFF;
       data[pIdx] = val0;
       buckets_[val0]++;

       for (int i=0; i<pIdx; i++)
       {
          final int val = input[srcIdx+i] & 0xFF;
          data[i] = (buckets_[val] << 8) | val;
          buckets_[val]++;
       }
          
       for (int i=pIdx+1; i<count; i++)
       {
          final int val = input[srcIdx+i] & 0xFF;
          data[i] = (buckets_[val] << 8) | val;
          buckets_[val]++;
       }

       // Create cumulative histogram
       for (int i=0, sum=0; i<256; i++)
       {
          final int tmp = buckets_[i];
          buckets_[i] = sum;
          sum += tmp;
       }

       int idx = dstIdx + count - 1;
       
       // Build inverse
       if (chunks == 1)
       {
          int ptr = data[pIdx];
          output[idx--] = (byte) ptr;      

          for (; idx>=dstIdx; idx--)
          {
             ptr = data[(ptr>>>8) + buckets_[ptr&0xFF]];
             output[idx] = (byte) ptr;
          }
       }
       else
       {
           final int step = count / chunks;
           
           for (int i=chunks-1; i>=0; i--)
           {
              int ptr = data[pIdx];
              output[idx--] = (byte) ptr;
              final int endChunk = dstIdx + i*step;
              
              for (; idx>=endChunk; idx--)
              {
                 ptr = data[(ptr>>>8) + buckets_[ptr&0xFF]];
                 output[idx] = (byte) ptr;
              }   
              
              pIdx = this.getPrimaryIndex(i);
              idx = endChunk - 1;
           }
       }
       
       src.index += count;
       dst.index += count;
       return true;
    }

    
    // When count >= 1<<24
    private boolean inverseBigBlock(SliceByteArray src, SliceByteArray dst, int count)
    {
       final byte[] input = src.array;
       final byte[] output = dst.array;
       final int srcIdx = src.index;
       final int dstIdx = dst.index;
       
       // Lazy dynamic memory allocations
       if (this.buffer1.length < count)
          this.buffer1 = new int[count];

       if (this.buffer2.length < count)
          this.buffer2 = new byte[count];

       // Aliasing
       final int[] buckets_ = this.buckets;
       final int[] data1 = this.buffer1;
       final byte[] data2 = this.buffer2;
       
       // Initialize histogram
       for (int i=0; i<256; i++)
          buckets_[i] = 0;

       final int chunks = getBWTChunks(count);

       // Build arrays
       // Start with the primary index position
       int pIdx = this.getPrimaryIndex(0);
       final byte val0 = input[srcIdx+pIdx];
       data1[pIdx] = buckets_[val0&0xFF];
       data2[pIdx] = val0;
       buckets_[val0&0xFF]++;

       for (int i=0; i<pIdx; i++)
       {
          final byte val = input[srcIdx+i];
          data1[i] = buckets_[val&0xFF];
          data2[i] = val;
          buckets_[val&0xFF]++;
       }
       
       for (int i=pIdx+1; i<count; i++)
       {
          final byte val = input[srcIdx+i];
          data1[i] = buckets_[val&0xFF];
          data2[i] = val;
          buckets_[val&0xFF]++;
       }

       // Create cumulative histogram
       for (int i=0, sum=0; i<256; i++)
       {
          final int tmp = buckets_[i];
          buckets_[i] = sum;
          sum += tmp;
       }

       int idx = dstIdx + count - 1;
       
       // Build inverse
       if (chunks == 1)
       {
          int val1 = data1[pIdx];
          byte val2 = data2[pIdx];
          output[idx--] = val2;

          for (; idx>=dstIdx; idx--)
          {
             final int n = val1 + buckets_[val2&0xFF];
             val1 = data1[n];
             val2 = data2[n];
             output[idx] = val2;
          }      
       }
       else
       {
           final int step = count / chunks;
           
           for (int i=chunks-1; i>=0; i--)
           {
              int val1 = data1[pIdx];
              byte val2 = data2[pIdx];
              output[idx--] = val2;
              final int endChunk = dstIdx + i*step;
              
              for (; idx>=endChunk; idx--)
              {
                 final int n = val1 + buckets_[val2&0xFF];
                 val1 = data1[n];
                 val2 = data2[n];
                 output[idx] = val2;
              }   
              
              pIdx = this.getPrimaryIndex(i);
              idx = endChunk - 1;
           }          
       }
            
       src.index += count;
       dst.index += count;
       return true;
    }

    
    private static int maxBlockSize() 
    {
       return MAX_BLOCK_SIZE - BWT_MAX_HEADER_SIZE;      
    } 
    
    
    public static int getBWTChunks(int size)
    {
       // For now, return 1 always !!!!
       return 1;
//       int log = 0;
//       size >>= 10;
//       
//       while ((size>0) && (log<3))
//       {
//          size >>= 2;
//          log++;
//       }
//       
//       return 1<<log;
    }
 
}
