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

package kanzi.function;

import kanzi.ByteFunction;
import kanzi.Memory;
import kanzi.SliceByteArray;


// Pure Java implementation of a LZ4 codec.
// LZ4 is a very fast lossless compression algorithm created by Yann Collet.
// See original code here: https://code.google.com/p/lz4/
// More details on the algorithm are available here:
// http://fastcompression.blogspot.com/2011/05/lz4-explained.html

public final class LZ4Codec implements ByteFunction
{
   private static final int HASH_SEED          = 0x9E3779B1;
   private static final int HASH_LOG           = 12;
   private static final int HASH_LOG_64K       = 13;
   private static final int MAX_DISTANCE       = (1 << 16) - 1;
   private static final int SKIP_STRENGTH      = 6;
   private static final int LAST_LITERALS      = 5;
   private static final int MIN_MATCH          = 4;
   private static final int MF_LIMIT           = 12;
   private static final int LZ4_64K_LIMIT      = MAX_DISTANCE + MF_LIMIT;
   private static final int ML_BITS            = 4;
   private static final int ML_MASK            = (1 << ML_BITS) - 1;
   private static final int RUN_BITS           = 8 - ML_BITS;
   private static final int RUN_MASK           = (1 << RUN_BITS) - 1;
   private static final int COPY_LENGTH        = 8;
   private static final int MIN_LENGTH         = 14;
   private static final int MAX_LENGTH         = (32*1024*1024) - 4 - MIN_MATCH;
   private static final int ACCELERATION       = 1;
   private static final int SKIP_TRIGGER       = 6;
   private static final int SEARCH_MATCH_NB    = ACCELERATION << SKIP_TRIGGER;

   private final int[] buffer;


   public LZ4Codec()
   {
      this.buffer = new int[1<<HASH_LOG_64K];
   }


   private static int writeLength(byte[] array, int idx, int len)
   {
      while (len >= 0x1FE)
      {
         array[idx]   = (byte) 0xFF;
         array[idx+1] = (byte) 0xFF;
         idx += 2;
         len -= 0x1FE;
      }

      if (len >= 0xFF)
      {
         array[idx++] = (byte) 0xFF;
         len -= 0xFF;
      }

      array[idx] = (byte) len;
      return idx + 1;
   }

   
   private static int writeLastLiterals(byte[] src, int srcIdx, byte[] dst, int dstIdx, int runLength)
   {
      if (runLength >= RUN_MASK)
      {
         dst[dstIdx++] = (byte) (RUN_MASK << ML_BITS);
         dstIdx = writeLength(dst, dstIdx, runLength - RUN_MASK);               
      }
      else
      {
         dst[dstIdx++] = (byte) (runLength << ML_BITS);
      } 
            
      System.arraycopy(src, srcIdx, dst, dstIdx, runLength);
      return dstIdx + runLength;
   }
     
   
   // Generates same byte output as LZ4_compress_generic in LZ4 r131 (7/15) 
   // for a 32 bit architecture.
   @Override
   public boolean forward(SliceByteArray input, SliceByteArray output)
   {
      if ((!SliceByteArray.isValid(input)) || (!SliceByteArray.isValid(output)))
         return false;

      if (input.array == output.array)
         return false;
   
      final int count = input.length;
      
      if (output.length - output.index < this.getMaxEncodedLength(count))
         return false;

      final int hashLog = (count < LZ4_64K_LIMIT) ? HASH_LOG_64K : HASH_LOG;
      final int hashShift = 32 - hashLog;
      final int srcIdx0 = input.index;
      final int dstIdx0 = output.index;
      final byte[] src = input.array;
      final byte[] dst = output.array;
      final int srcEnd = srcIdx0 + count;
      final int matchLimit = srcEnd - LAST_LITERALS;
      final int mfLimit = srcEnd - MF_LIMIT;
      int srcIdx = srcIdx0;
      int dstIdx = dstIdx0;
      int anchor = srcIdx0;
      final int[] table = this.buffer; // aliasing

      if (count > MIN_LENGTH)
      {    
         for (int i=(1<<hashLog)-1; i>=0; i--)
            table[i] = 0;

         // First byte
         int h = (Memory.LittleEndian.readInt32(src, srcIdx) * HASH_SEED) >>> hashShift;
         table[h] = srcIdx - srcIdx0;         
         srcIdx++;
         h = (Memory.LittleEndian.readInt32(src, srcIdx) * HASH_SEED) >>> hashShift;

         while (true)
         {
            int fwdIdx = srcIdx;
            int step = 1;
            int searchMatchNb = SEARCH_MATCH_NB;
            int match;

            // Find a match
            do
            {
               srcIdx = fwdIdx;
               fwdIdx += step;

               if (fwdIdx > mfLimit)
               {
                  // Encode last literals
                  output.index = writeLastLiterals(src, anchor, dst, dstIdx, srcEnd-anchor);                                    
                  input.index = srcEnd;
                  return true;
               }

               step = searchMatchNb >> SKIP_STRENGTH;
               searchMatchNb++;
               match = table[h] + srcIdx0;            
               table[h] = srcIdx - srcIdx0;
               h = (Memory.LittleEndian.readInt32(src, fwdIdx) * HASH_SEED) >>> hashShift;
            }
            while ((differentInts(src, match, srcIdx) == true) || (match <= srcIdx - MAX_DISTANCE));

            // Catch up
            while ((match > srcIdx0) && (srcIdx > anchor) && (src[match-1] == src[srcIdx-1]))
            {
               match--;
               srcIdx--;
            }

            // Encode literal length
            final int litLength = srcIdx - anchor;
            int token = dstIdx;
            dstIdx++;
          
            if (litLength >= RUN_MASK)
            {
               dst[token] = (byte) (RUN_MASK << ML_BITS);
               dstIdx = writeLength(dst, dstIdx, litLength-RUN_MASK);               
            }
            else
            {
               dst[token] = (byte) (litLength << ML_BITS);
            }
           
            // Copy literals
            customArrayCopy(src, anchor, dst, dstIdx, litLength);
            dstIdx += litLength;
          
            // Next match
            do
            {
               // Encode offset
               dst[dstIdx++] = (byte) (srcIdx-match);
               dst[dstIdx++] = (byte) ((srcIdx-match) >> 8);

               // Encode match length
               srcIdx += MIN_MATCH;
               match += MIN_MATCH;
               anchor = srcIdx;

               while ((srcIdx < matchLimit) && (src[srcIdx] == src[match]))
               {
                  srcIdx++;
                  match++;
               }

               final int matchLength = srcIdx - anchor;

               // Encode match length
               if (matchLength >= ML_MASK)
               {
                  dst[token] += (byte) ML_MASK;
                  dstIdx = writeLength(dst, dstIdx, matchLength-ML_MASK);                 
               }
               else
               {
                  dst[token] += (byte) matchLength;
               }            

               anchor = srcIdx;

               if (srcIdx > mfLimit)
               {
                  // Encode last literals
                  output.index = writeLastLiterals(src, anchor, dst, dstIdx, srcEnd-anchor);
                  input.index = srcEnd;
                  return true;
               }

               // Fill table
               h = (Memory.LittleEndian.readInt32(src, srcIdx-2) * HASH_SEED) >>> hashShift;
               table[h] = srcIdx - 2 - srcIdx0;

               // Test next position
               h = (Memory.LittleEndian.readInt32(src, srcIdx) * HASH_SEED) >>> hashShift;
               match = table[h] + srcIdx0;
               table[h] = srcIdx - srcIdx0;

               if ((differentInts(src, match, srcIdx) == true) || (match <= srcIdx - MAX_DISTANCE))
                  break;
               
               token = dstIdx;
               dstIdx++;
               dst[token] = 0;
            }
            while (true);
            
            // Prepare next loop
            srcIdx++;
            h = (Memory.LittleEndian.readInt32(src, srcIdx) * HASH_SEED) >>> hashShift;
         }
      }

      dstIdx = writeLastLiterals(src, anchor, dst, dstIdx, srcEnd-anchor);
      
      // Encode last literals
      output.index = dstIdx;
      input.index = srcEnd;
      return true;
   }


   // Reads same byte input as LZ4_decompress_generic in LZ4 r131 (7/15) 
   // for a 32 bit architecture.
   @Override
   public boolean inverse(SliceByteArray input, SliceByteArray output)
   {
      if ((!SliceByteArray.isValid(input)) || (!SliceByteArray.isValid(output)))
         return false;

      if (input.array == output.array)
         return false;

      final int count = input.length;     
      final int srcIdx0 = input.index;
      final int dstIdx0 = output.index;
      final byte[] src = input.array;
      final byte[] dst = output.array;
      final int srcEnd = srcIdx0 + count;
      final int dstEnd = dst.length;
      final int srcEnd2 = srcEnd - COPY_LENGTH;
      final int dstEnd2 = dstEnd - COPY_LENGTH;
      int srcIdx = srcIdx0;
      int dstIdx = dstIdx0;

      while (true)
      {
         // Get literal length 
         final int token = src[srcIdx++] & 0xFF;
         int length = token >> ML_BITS;

         if (length == RUN_MASK)
         {
            byte len;

            while (((len = src[srcIdx++]) == (byte) 0xFF) && (srcIdx <= srcEnd))
               length += 0xFF;

            length += (len & 0xFF);

            if (length > MAX_LENGTH)
               throw new IllegalArgumentException("Invalid length decoded: " + length);
         }
        
         // Copy literals
         if ((dstIdx + length > dstEnd2) || (srcIdx + length > srcEnd2))
         {
            System.arraycopy(src, srcIdx, dst, dstIdx, length);
            srcIdx += length;
            dstIdx += length;
            break;
         }

         customArrayCopy(src, srcIdx, dst, dstIdx, length);
         srcIdx += length;
         dstIdx += length;

         if ((dstIdx > dstEnd2) || (srcIdx > srcEnd2))
            break;

         // Get offset
         final int delta = (src[srcIdx] & 0xFF) | ((src[srcIdx+1] & 0xFF) << 8);
         srcIdx += 2;
         int match = dstIdx - delta;
                  
         if (match < dstIdx0)
            break;
         
         length = token & ML_MASK;

         // Get match length
         if (length == ML_MASK)
         {
            while (((src[srcIdx]) == (byte) 0xFF) && (srcIdx < srcEnd))
            {
               srcIdx++;
               length += 0xFF;
            }

            if (srcIdx < srcEnd)
               length += (src[srcIdx++] & 0xFF);

            if ((length > MAX_LENGTH) || (srcIdx == srcEnd))
               throw new IllegalArgumentException("Invalid length decoded: " + length);
         }

         length += MIN_MATCH;
         final int cpy = dstIdx + length;

         // Copy repeated sequence 
         if (cpy > dstEnd2)
         {
            for (int i=0; i<length; i++)
               dst[dstIdx+i] = dst[match+i]; 
         }
         else
         { 
            // Unroll loop
            do
            {
               dst[dstIdx]   = dst[match];
               dst[dstIdx+1] = dst[match+1];
               dst[dstIdx+2] = dst[match+2];
               dst[dstIdx+3] = dst[match+3];
               dst[dstIdx+4] = dst[match+4];
               dst[dstIdx+5] = dst[match+5];
               dst[dstIdx+6] = dst[match+6];
               dst[dstIdx+7] = dst[match+7];
               match += 8;
               dstIdx += 8;
            }
            while (dstIdx < cpy);
         }
         
         // Correction
         dstIdx = cpy;
      }

      output.index = dstIdx;
      input.index = srcIdx;
      return srcIdx == srcEnd;
   }


   private static void arrayChunkCopy(byte[] src, int srcIdx, byte[] dst, int dstIdx)
   {
      dst[dstIdx]   = src[srcIdx];
      dst[dstIdx+1] = src[srcIdx+1];
      dst[dstIdx+2] = src[srcIdx+2];
      dst[dstIdx+3] = src[srcIdx+3];
      dst[dstIdx+4] = src[srcIdx+4];
      dst[dstIdx+5] = src[srcIdx+5];
      dst[dstIdx+6] = src[srcIdx+6];
      dst[dstIdx+7] = src[srcIdx+7];  
   }


   private static void customArrayCopy(byte[] src, int srcIdx, byte[] dst, int dstIdx, int len)
   {
      for (int i=0; i<len; i+=8)
         arrayChunkCopy(src, srcIdx+i, dst, dstIdx+i);
   }


   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      return srcLen + (srcLen / 255) + 16;
   }


   private static boolean differentInts(byte[] array, int srcIdx, int dstIdx)
   {
      return ((array[srcIdx] != array[dstIdx])     ||
              (array[srcIdx+1] != array[dstIdx+1]) ||
              (array[srcIdx+2] != array[dstIdx+2]) ||
              (array[srcIdx+3] != array[dstIdx+3]));
   }
}
