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

package kanzi.function;

import java.nio.ByteOrder;
import kanzi.ByteFunction;
import kanzi.IndexedByteArray;


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
   private static final int SHIFT1             = (ByteOrder.nativeOrder() == ByteOrder.BIG_ENDIAN) ?  24 : 0;
   private static final int SHIFT2             = (ByteOrder.nativeOrder() == ByteOrder.BIG_ENDIAN) ?  16 : 8;
   private static final int SHIFT3             = (ByteOrder.nativeOrder() == ByteOrder.BIG_ENDIAN) ?  8  : 16;
   private static final int SHIFT4             = (ByteOrder.nativeOrder() == ByteOrder.BIG_ENDIAN) ?  0  : 24;
   private static final int COPY_LENGTH        = 8;
   private static final int MIN_LENGTH         = 14;
   private static final int MAX_LENGTH         = (32*1024*1024) - 4 - MIN_MATCH;
   private static final int DEFAULT_FIND_MATCH_ATTEMPTS = (1 << SKIP_STRENGTH) + 3;

   private int size;
   private final int[] buffer;


   public LZ4Codec()
   {
      this(0);
   }


   public LZ4Codec(int size)
   {
      if (size < 0)
         throw new IllegalArgumentException("Invalid size parameter (must be at least 0)");

      this.size = size;
      this.buffer = new int[1<<HASH_LOG_64K];
   }


   public int size()
   {
      return this.size;
   }


   public boolean setSize(int sz)
   {
      if (size < 0)
         return false;

      this.size = sz;
      return true;
   }


   private static int writeLength(byte[] array, int idx, int len) 
   {   
      while (len >= 0x1FE) 
      {
         array[idx++] = (byte) 0xFF;
         array[idx++] = (byte) 0xFF;
         len -= 0x1FE;
      }

      if (len >= 0xFF) 
      {
         array[idx++] = (byte) 0xFF;
         len -= 0xFF;
      }
    
      array[idx++] = (byte) len;
      return idx;
   }


   private static int emitLiterals(IndexedByteArray source,
           IndexedByteArray destination, int runLen, boolean last)
   {
      int token;

      // Emit literal lengths
      if (runLen >= RUN_MASK)
      {
         token = RUN_MASK << ML_BITS;

         if (last == true)
            destination.array[destination.index++] = (byte) token;

         destination.index = writeLength(destination.array, destination.index, runLen - RUN_MASK);
      }
      else
      {
         token = runLen << ML_BITS;

         if (last == true)
            destination.array[destination.index++] = (byte) token;
      }

      // Emit literals
      if (last == true)
         System.arraycopy(source.array, source.index, destination.array, destination.index, runLen);
      else
         customArrayCopy(source.array, source.index, destination.array, destination.index, runLen);

      source.index += runLen;
      destination.index += runLen;
      return token;
   }

   @Override
   public boolean forward(IndexedByteArray source, IndexedByteArray destination)
   {
      if ((source == null) || (destination == null) || (source.array == destination.array))
         return false;

      final int srcIdx0 = source.index;
      final int dstIdx0 = destination.index;
      final byte[] src = source.array;
      final byte[] dst = destination.array;
      final int count = (this.size > 0) ? this.size : src.length - srcIdx0;

      if (destination.array.length - destination.index < getMaxEncodedLength(count))
         return false;

      if (count <= MIN_LENGTH)
      {
         emitLiterals(source, destination, count, true);
         return true;
      }

      final int base = source.index;
      final int hashLog = (count < LZ4_64K_LIMIT) ? HASH_LOG_64K : HASH_LOG;
      final int hashShift = 32 - hashLog;
      final int srcEnd = srcIdx0 + count;
      final int srcLimit = srcEnd - LAST_LITERALS;
      final int mfLimit = srcEnd - MF_LIMIT;
      int srcIdx = srcIdx0;
      int dstIdx = dstIdx0;
      int anchor = srcIdx;
      srcIdx++;
      final int[] table = this.buffer; // aliasing

      for (int i=(1<<hashLog)-1; i>=0; i--)
         table[i] = 0;

      while (true)
      {
         int attempts = DEFAULT_FIND_MATCH_ATTEMPTS;
         int fwdIdx = srcIdx;
         int ref;

         // Find a match
         do
         {
            srcIdx = fwdIdx;
            fwdIdx += (attempts >>> SKIP_STRENGTH);

            if (fwdIdx > mfLimit)
            {
               source.index = anchor;
               destination.index = dstIdx;
               emitLiterals(source, destination, srcEnd - anchor, true);
               return true;
            }

            attempts++;
            final int h = (readInt(src, srcIdx) * HASH_SEED) >>> hashShift;
            ref = base + table[h];
            table[h] = srcIdx - base;        
         }
         while ((differentInts(src, ref, srcIdx) == true) || (ref <= srcIdx - MAX_DISTANCE));
       
         // Catch up
         while ((ref > srcIdx0) && (srcIdx > anchor) && (src[ref-1] == src[srcIdx-1]))
         {
            ref--;
            srcIdx--;
         }

         // Encode literal length
         final int runLen = srcIdx - anchor;
         int tokenOff = dstIdx;
         dstIdx++;

         source.index = anchor;
         destination.index = dstIdx;
         int token = emitLiterals(source, destination, runLen, false);
         dstIdx = destination.index;

         do
         {       
            // Encode offset
            dst[dstIdx++] = (byte) (srcIdx-ref);
            dst[dstIdx++] = (byte) ((srcIdx-ref) >> 8);

            // Start counting
            srcIdx += MIN_MATCH;
            ref += MIN_MATCH;
            anchor = srcIdx;

            while ((srcIdx < srcLimit) && (src[srcIdx] == src[ref]))
            {
               srcIdx++;
               ref++;
            }

            final int matchLen = srcIdx - anchor;

            // Encode match length
            if (matchLen >= ML_MASK)
            {
               dst[tokenOff] = (byte) (token | ML_MASK);
               dstIdx = writeLength(dst, dstIdx, matchLen-ML_MASK);
            }
            else
            {
               dst[tokenOff] = (byte) (token | matchLen);
            }

            // Test end of chunk
            if (srcIdx > mfLimit)
            {
               source.index = srcIdx;
               destination.index = dstIdx;
               emitLiterals(source, destination, srcEnd - srcIdx, true);
               return true;
            }

            // Test next position
            final int h1 = (readInt(src, srcIdx-2) * HASH_SEED) >>> hashShift;
            final int h2 = (readInt(src, srcIdx) * HASH_SEED) >>> hashShift;
            table[h1] = srcIdx - 2 - base;
            ref = base + table[h2];
            table[h2] = srcIdx - base;

            if ((differentInts(src, ref, srcIdx) == true) || (ref <= srcIdx - MAX_DISTANCE))
               break;

            tokenOff = dstIdx;
            dstIdx++;
            token = 0;
         }
         while (true);

         // Prepare next loop
         anchor = srcIdx;
         srcIdx++;
      }
   }


   @Override
   public boolean inverse(IndexedByteArray source, IndexedByteArray destination)
   {
      if ((source == null) || (destination == null) || (source.array == destination.array))
         return false;

      final int srcIdx0 = source.index;
      final int dstIdx0 = destination.index;
      final byte[] src = source.array;
      final byte[] dst = destination.array;
      final int count = (this.size > 0) ? this.size : src.length - srcIdx0;
      final int srcEnd = srcIdx0 + count;
      final int dstEnd = dst.length;
      final int srcEnd2 = srcEnd - COPY_LENGTH;
      final int dstEnd2 = dstEnd - COPY_LENGTH;
      int srcIdx = srcIdx0;
      int dstIdx = dstIdx0;

      while (srcIdx < srcEnd)
      {
         final int token = src[srcIdx++] & 0xFF;

         // Literals
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

         // Get offset
         final int delta = (src[srcIdx++] & 0xFF) | ((src[srcIdx++] & 0xFF) << 8);
         int matchOffset = dstIdx - delta;
         length = token & ML_MASK;

         // Get match length
         if (length == ML_MASK)
         {
            byte len;

            while (((len = src[srcIdx++]) == (byte) 0xFF) && (srcIdx <= srcEnd))
               length += 0xFF;

            length += (len & 0xFF);

            if (length > MAX_LENGTH)
               throw new IllegalArgumentException("Invalid length decoded: " + length);
         }

         length += MIN_MATCH;
         final int matchEnd = dstIdx + length;

         if (matchEnd > dstEnd2)
         {
            for (int i=0; i<length; i++)
               dst[dstIdx+i] = dst[matchOffset+i];
         }
         else
         {
            // Unroll loop
            do
            {
               dst[dstIdx]   = dst[matchOffset];
               dst[dstIdx+1] = dst[matchOffset+1];
               dst[dstIdx+2] = dst[matchOffset+2];
               dst[dstIdx+3] = dst[matchOffset+3];
               dst[dstIdx+4] = dst[matchOffset+4];
               dst[dstIdx+5] = dst[matchOffset+5];
               dst[dstIdx+6] = dst[matchOffset+6];
               dst[dstIdx+7] = dst[matchOffset+7];
               matchOffset += 8;
               dstIdx += 8;
            }
            while (dstIdx < matchEnd);
         }

         // Correction
         dstIdx = matchEnd;
      }

      destination.index = dstIdx;
      source.index = srcIdx;
      return srcIdx == srcEnd;
   }


   private static void arrayChunkCopy(byte[] src, int srcIdx, byte[] dst, int dstIdx)
   {
      for (int j=0; j<8; j++)
         dst[dstIdx+j] = src[srcIdx+j];
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

   
   private static int readInt(byte[] array, int srcIdx)
   {
      return ((array[srcIdx]   & 0xFF) << SHIFT1) | 
             ((array[srcIdx+1] & 0xFF) << SHIFT2) |
             ((array[srcIdx+2] & 0xFF) << SHIFT3) | 
             ((array[srcIdx+3] & 0xFF) << SHIFT4);
   }
}
