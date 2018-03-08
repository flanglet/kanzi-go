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
import kanzi.Global;
import kanzi.SliceByteArray;

// Zero Run Length Encoding is a simple encoding algorithm by Wheeler
// closely related to Run Length Encoding. The main difference is
// that only runs of 0 values are processed. Also, the length is
// encoded in a different way (each digit in a different byte)
// This algorithm is well adapted to process post BWT/MTFT data

public final class ZRLT implements ByteFunction
{
   private static final int ZRLT_MAX_RUN = Integer.MAX_VALUE;


   public ZRLT()
   {
   }


   @Override
   public boolean forward(SliceByteArray input, SliceByteArray output)
   {
      if ((!SliceByteArray.isValid(input)) || (!SliceByteArray.isValid(output)))
         return false;

      if (input.array == output.array)
         return false;

      final int count = input.length;

      if (output.length - output.index < getMaxEncodedLength(count))
         return false;

      final byte[] src = input.array;
      final byte[] dst = output.array;
      int srcIdx = input.index;
      int dstIdx = output.index;
      final int srcEnd = srcIdx + count;
      final int dstEnd = dst.length;
      final int dstEnd2 = dstEnd - 2;
      int runLength = 1;

      if (dstIdx < dstEnd)
      {
         while (srcIdx < srcEnd)
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
               int log2 = (runLength<=256) ? Global.LOG2[runLength-1] : 31-Integer.numberOfLeadingZeros(runLength);
               
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

               dst[dstIdx] = (byte) 0xFF;
               dstIdx++;
               dst[dstIdx] = (byte) (val - 0xFE);
            }
            else
            {
               if (dstIdx >= dstEnd)
                  break;

               dst[dstIdx] = (byte) (val + 1);
            }

            srcIdx++;
            dstIdx++;

            if (dstIdx >= dstEnd)
               break;
         }
      }

      input.index = srcIdx;
      output.index = dstIdx;
      return (srcIdx == srcEnd) && (runLength == 1);
   }


   @Override
   public boolean inverse(SliceByteArray input, SliceByteArray output)
   {
      if ((!SliceByteArray.isValid(input)) || (!SliceByteArray.isValid(output)))
         return false;

      if (input.array == output.array)
         return false;

      final int count = input.length;
      int srcIdx = input.index;
      int dstIdx = output.index;
      final byte[] src = input.array;
      final byte[] dst = output.array;
      final int srcEnd = srcIdx + count;
      final int dstEnd = dst.length;
      int runLength = 1;

      if (srcIdx < srcEnd)
      {
         while (dstIdx < dstEnd)
         {
            if (runLength > 1)
            {
               runLength--;
               dst[dstIdx++] = 0;
               continue;
            }

            if (srcIdx >= srcEnd)
               break;

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

            srcIdx++;
            dstIdx++;
         }
      }

      // If runLength is not 1, add trailing 0s
      final int end = dstIdx + runLength - 1;
      input.index = srcIdx;
      output.index = dstIdx;

      if (end > dstEnd)
         return false;

      while (dstIdx < end)
         dst[dstIdx++] = 0;

      output.index = dstIdx;
      return srcIdx == srcEnd;
   }


   // Required encoding output buffer size unknown => guess
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      return srcLen;
   }
}