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
import kanzi.SliceByteArray;

// Simple implementation of a Run Length Codec
// Length is transmitted as 1 or 2 bytes (minus 1 bit for the mask that indicates
// whether a second byte is used). The run threshold can be provided.
// For a run threshold of 2:
// EG input: 0x10 0x11 0x11 0x17 0x13 0x13 0x13 0x13 0x13 0x13 0x12 (160 times) 0x14
//   output: 0x10 0x11 0x11 0x17 0x13 0x13 0x13 0x05 0x12 0x12 0x80 0xA0 0x14

public class RLT implements ByteFunction
{
   private static final int TWO_BYTE_RLE_MASK1 = 0x80;
   private static final int TWO_BYTE_RLE_MASK2 = 0x7F;
   private static final int MAX_RUN_VALUE = 0x7FFF;

   private final int runThreshold;


   public RLT()
   {
      this(3);
   }


   public RLT(int runThreshold)
   {
      if (runThreshold < 2)
         throw new IllegalArgumentException("Invalid run threshold parameter (must be at least 2)");

      this.runThreshold = runThreshold;
   }


   public int getRunThreshold()
   {
      return this.runThreshold;
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
      final int dstEnd3 = dstEnd - 3;
      boolean res = true;
      int run = 0;
      final int threshold = this.runThreshold;
      final int maxThreshold = MAX_RUN_VALUE + this.runThreshold;
      
      // Initialize with a value different from the first data
      byte prev = (byte) ~src[srcIdx];
      
      while ((srcIdx < srcEnd) && (dstIdx < dstEnd))
      {
         final byte val = src[srcIdx++];

         // Encode up to 0x7FFF repetitions in the 'length' information
         if ((prev == val) && (run < maxThreshold))
         {
            if (++run < threshold)
               dst[dstIdx++] = prev;

            continue;
         }

         if (run >= threshold)
         {
            if (dstIdx >= dstEnd3)
            {
               res = false;
               break;
            }
            
            dst[dstIdx++] = prev;
            run -= threshold;

            // Force MSB to indicate a 2 byte encoding of the length
            if (run >= TWO_BYTE_RLE_MASK1)
               dst[dstIdx++] = (byte) ((run >> 8) | TWO_BYTE_RLE_MASK1);

            dst[dstIdx++] = (byte) run;
            run = 1;
         }

         dst[dstIdx++] = val;

         if (prev != val)
         {
            prev = val;
            run = 1;
         }
      }

      // Fill up the destination array
      if (run >= threshold)
      {
         if (dstIdx >= dstEnd3)
         {
            res = false;
         }
         else
         {
            dst[dstIdx++] = prev;
            run -= threshold;

            // Force MSB to indicate a 2 byte encoding of the length
            if (run >= TWO_BYTE_RLE_MASK1)
               dst[dstIdx++] = (byte) ((run >>> 8) | TWO_BYTE_RLE_MASK1);

            dst[dstIdx++] = (byte) run;
         }
      }

      input.index = srcIdx;
      output.index = dstIdx;
      return res && (srcIdx == srcEnd);
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
      int run = 0;
      final int threshold = this.runThreshold;
      boolean res = true;

      // Initialize with a value different from the first data
      byte prev = (byte) ~src[srcIdx];

      while ((srcIdx < srcEnd) && (dstIdx < dstEnd))
      { 
         final byte val = src[srcIdx++];

         if (prev == val)
         {
            if (++run >= threshold)
            {
               // Read the length
               run = src[srcIdx++] & 0xFF;

               // If the length is encoded in 2 bytes, process next byte
               if ((run & TWO_BYTE_RLE_MASK1) != 0)
               {
                  run = ((run & TWO_BYTE_RLE_MASK2) << 8) | (src[srcIdx++] & 0xFF);
               }

               if (dstIdx >= dstEnd + run)
               {
                  res = false;
                  break;
               }
               
               // Emit length times the previous byte
               while (--run >= 0)
                  dst[dstIdx++] = prev;

               run = 0;
            }
         }
         else
         {
            prev = val;
            run = 1;
         }

         dst[dstIdx++] = val;
      }

      input.index = srcIdx;
      output.index = dstIdx;
      return res && (srcIdx == srcEnd);
   }
   
   
   // Required encoding output buffer size unknown => guess
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      return srcLen;
   }
}