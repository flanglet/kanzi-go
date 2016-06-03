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

import kanzi.ByteFunction;
import kanzi.IndexedByteArray;

// Zero Run Length Encoding is a simple encoding algorithm by Wheeler
// closely related to Run Length Encoding. The main difference is
// that only runs of 0 values are processed. Also, the length is
// encoded in a different way (each digit in a different byte)
// This little algorithm is well adapted to process post BWT/MTFT data

public final class ZRLT implements ByteFunction
{
   private static final int ZRLT_MAX_RUN = Integer.MAX_VALUE;

   
   public ZRLT()
   {
   }


   @Override
   public boolean forward(IndexedByteArray source, IndexedByteArray destination, int length)
   {
      if ((source == null) || (destination == null) || (source.array == destination.array))
         return false;

      int srcIdx = source.index;
      int dstIdx = destination.index;
      final byte[] src = source.array;
      final byte[] dst = destination.array;
      final int srcEnd = srcIdx + length;
      final int dstEnd = dst.length;
      final int dstEnd2 = dstEnd - 2;
      int runLength = 1;

      while ((srcIdx < srcEnd) && (dstIdx < dstEnd))
      {
         int val = src[srcIdx];

         if (val == 0)
         {
            runLength++;
            srcIdx++;

            if ((srcIdx < srcEnd) && (runLength < ZRLT_MAX_RUN))
                continue;
         }

         if (runLength > 1)
         {
            // Encode length
            int log2 = 1;

            for (int val2=runLength>>1; val2>1; val2>>=1)
               log2++;

            if (dstIdx >= dstEnd - log2)
               break;

            // Write every bit as a byte except the most significant one
            while (log2 > 0)
            {
               log2--;
               dst[dstIdx++] = (byte) ((runLength >> log2) & 1);
            }

            runLength = 1;
            continue;
         }

         val &= 0xFF;

         if (val >= 0xFE)
         {
            if (dstIdx >= dstEnd2)
               break;

            dst[dstIdx++] = (byte) 0xFF;
            dst[dstIdx++] = (byte) (val - 0xFE);
         }
         else
         {
            dst[dstIdx++] = (byte) (val + 1);
         }

         srcIdx++;
      }

      source.index = srcIdx;
      destination.index = dstIdx;
      return (srcIdx == srcEnd) && (runLength == 1);
   }


   @Override
   public boolean inverse(IndexedByteArray source, IndexedByteArray destination, int length)
   {
      if ((source == null) || (destination == null) || (source.array == destination.array))
         return false;

      int srcIdx = source.index;
      int dstIdx = destination.index;
      final byte[] src = source.array;
      final byte[] dst = destination.array;
      final int srcEnd = srcIdx + length;
      final int dstEnd = dst.length;
      int runLength = 1;

      while ((srcIdx < srcEnd) && (dstIdx < dstEnd))
      {
         if (runLength > 1)
         {
            runLength--;
            dst[dstIdx++] = 0;
            continue;
         }

         int val = src[srcIdx] & 0xFF;

         if (val <= 1)
         {
            // Generate the run length bit by bit (but force MSB)
            runLength = 1;

            do
            {
               runLength = (runLength << 1) | val;
               srcIdx++;

               if (srcIdx >= srcEnd)
                   break;

               val = src[srcIdx] & 0xFF;
            }
            while (val <= 1);

            continue;
         }

         // Regular data processing
         if (val == 0xFF)
         {
            srcIdx++;

            if (srcIdx >= srcEnd)
               break;

            dst[dstIdx] = (byte) (0xFE + src[srcIdx]);
         }
         else
         {
            dst[dstIdx] = (byte) (val - 1);
         }
         
         dstIdx++;
         srcIdx++;
      }

      // If runLength is not 1, add trailing 0s
      final int end = dstIdx + runLength - 1;
      source.index = srcIdx;
      destination.index = dstIdx;

      if (end > dstEnd)
         return false;

      while (dstIdx < end)
         dst[dstIdx++] = 0;

      destination.index = dstIdx;
      return srcIdx == srcEnd;
   }


   // Required encoding output buffer size unknown
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      return -1;
   }
}