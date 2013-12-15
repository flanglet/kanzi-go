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

// Simple implementation of a Run Length Codec
// Length is transmitted as 1 or 2 bytes (minus 1 bit for the mask that indicates
// whether a second byte is used). The run threshold can be provided.
// For a run threshold of 2:
// EG input: 0x10 0x11 0x11 0x17 0x13 0x13 0x13 0x13 0x13 0x13 0x12 (160 times) 0x14
//   output: 0x10 0x11 0x11 0x17 0x13 0x13 0x13 0x05 0x12 0x12 0x80 0xA0 0x14

public class RLT implements ByteFunction
{
   private static final int TWO_BYTE_RLE_MASK = 0x80;
   private static final int MAX_RUN_VALUE = 0x7FFF;

   private final int size;
   private final int runThreshold;


   public RLT()
   {
      this(0);
   }


   public RLT(int size)
   {
      this(size, 3);
   }


   public RLT(int size, int runThreshold)
   {
      if (size < 0)
         throw new IllegalArgumentException("Invalid size parameter (must be at least 0)");

      if (runThreshold < 2)
         throw new IllegalArgumentException("Invalid run threshold parameter (must be at least 2)");

      this.size = size;
      this.runThreshold = runThreshold;
   }


   public int size()
   {
      return this.size;
   }


   public int getRunThreshold()
   {
      return this.runThreshold;
   }


   @Override
   public boolean forward(IndexedByteArray source, IndexedByteArray destination)
   {
      if ((source == null) || (destination == null) || (source.array == destination.array))
         return false;

      int srcIdx = source.index;
      int dstIdx = destination.index;
      final byte[] src = source.array;
      final byte[] dst = destination.array;
      final int srcEnd = (this.size == 0) ? src.length : srcIdx + this.size;
      final int dstEnd = dst.length;
      boolean res = true;
      int run = 0;
      final int threshold = this.runThreshold;
      
      // Initialize with a value different from the first data
      byte prev = (byte) (~src[srcIdx]);

      try
      {
         while ((srcIdx < srcEnd) && (dstIdx < dstEnd))
         {
            final byte val = src[srcIdx++];

            // Encode up to 0x7FFF repetitions in the 'length' information
            if ((prev == val) && (run < MAX_RUN_VALUE))
            {
               if (++run < threshold)
                  dst[dstIdx++] = prev;

               continue;
            }

            if (run >= threshold)
            {
               dst[dstIdx++] = prev;
               run -= threshold;

               // Force MSB to indicate a 2 byte encoding of the length
               if (run >= TWO_BYTE_RLE_MASK)
                  dst[dstIdx++] = (byte) ((run >> 8) | TWO_BYTE_RLE_MASK);

               dst[dstIdx++] = (byte) (run & 0xFF);
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
            dst[dstIdx++] = prev;
            run -= threshold;

            // Force MSB to indicate a 2 byte encoding of the length
            if (run >= TWO_BYTE_RLE_MASK)
               dst[dstIdx++] = (byte) ((run >> 8) | TWO_BYTE_RLE_MASK);

            dst[dstIdx++] = (byte) (run & 0xFF);
         }
      }
      catch (ArrayIndexOutOfBoundsException e)
      {
         res = false;
      }

      source.index = srcIdx;
      destination.index = dstIdx;
      return res;
   }


   @Override
   public boolean inverse(IndexedByteArray source, IndexedByteArray destination)
   {
      if ((source == null) || (destination == null) || (source.array == destination.array))
         return false;

      int srcIdx = source.index;
      int dstIdx = destination.index;
      final byte[] src = source.array;
      final byte[] dst = destination.array;
      final int srcEnd = (this.size == 0) ? src.length : srcIdx + this.size;
      final int dstEnd = dst.length;
      int run = 0;
      final int threshold = this.runThreshold;
      boolean res = true;

      // Initialize with a value different from the first data
      byte prev = (byte) (~src[srcIdx]);

      try
      {
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
                  if ((run & TWO_BYTE_RLE_MASK) != 0)
                  {
                     run = ((run & ~TWO_BYTE_RLE_MASK) << 8) | (src[srcIdx++] & 0xFF);
                  }

                  // Emit length times the previous byte
                  while (--run >= 0)
                     dst[dstIdx++] = prev;
               }
            }
            else
            {
               prev = val;
               run = 1;
            }

            dst[dstIdx++] = val;
         }
      }
      catch (ArrayIndexOutOfBoundsException e)
      {
         res = false;
      }

      source.index = srcIdx;
      destination.index = dstIdx;
      return res;
   }
   
   
   // Required encoding output buffer size unknown
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      return -1;
   }
}