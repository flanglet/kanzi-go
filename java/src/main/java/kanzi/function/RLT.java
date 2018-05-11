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


// Implementation of Mespotine RLE
// See [An overhead-reduced and improved Run-Length-Encoding Method] by Meo Mespotine
// Length is transmitted as 1 to 3 bytes. The run threshold can be provided.
// EG. runThreshold = 2 and RUN_LEN_ENCODE1 = 239 => RUN_LEN_ENCODE2 = 4096
// 2    <= runLen < 239+2      -> 1 byte
// 241  <= runLen < 4096+2     -> 2 bytes
// 4098 <= runLen < 65536+4098 -> 3 bytes

public class RLT implements ByteFunction
{
   private static final int RUN_LEN_ENCODE1 = 224; // used to encode run length
   private static final int RUN_LEN_ENCODE2 = (256-1-RUN_LEN_ENCODE1) << 8; // used to encode run length
   private static final int MAX_RUN = 0xFFFF + RUN_LEN_ENCODE2; 

   private final int runThreshold;
   private final int[] counters;
   private final byte[] flags;

   
   public RLT()
   {
      this(2);
   }


   public RLT(int runThreshold)
   {
      if (runThreshold < 2)
         throw new IllegalArgumentException("Invalid run threshold parameter (must be at least 2)");

      this.runThreshold = runThreshold;
      this.counters = new int[256];
      this.flags = new byte[32];
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
     
      for (int i=0; i<32; i++)
         this.flags[i] = 0;
      
      for (int i=0; i<256; i++)
         this.counters[i] = 0;
      
      final byte[] src = input.array;
      final byte[] dst = output.array;     
      int srcIdx = input.index;
      int dstIdx = output.index;
      final int srcEnd = srcIdx + count;
      final int dstEnd = dst.length;
      final int dstEnd4 = dstEnd - 4;
      boolean res = true;
      int run = 0;
      final int threshold = this.runThreshold;
      final int maxRun = MAX_RUN + this.runThreshold;
      
      // Initialize with a value different from the first data
      byte prev = (byte) ~src[srcIdx];
      
      // Step 1: create counters and set compression flags
      while (srcIdx < srcEnd)
      {
         final byte val = src[srcIdx++];

         if ((prev == val) && (run < MAX_RUN))
         {
            run++;
            continue;
         }

         if (run >= threshold)
            this.counters[prev&0xFF] += (run-threshold-1);

         prev = val;
         run = 1;        
      }

      if (run >= threshold)
         this.counters[prev&0xFF] += (run-threshold-1);

      for (int i=0; i<256; i++)
      {
         if (this.counters[i] > 0)
            this.flags[i>>3] |= (1<<(7-(i&7)));
      }

      // Write flags to output
      for (int i=0; i<32; i++)
         dst[dstIdx++] = this.flags[i];
      
      srcIdx = input.index;
      prev = (byte) ~src[srcIdx];
      run = 0;
      
      // Step 2: output run lengths and literals
      // Note that it is possible to output runs over the threshold (for symbols
      // with an unset compression flag)
      while ((srcIdx < srcEnd) && (dstIdx < dstEnd))
      {
         final byte val = src[srcIdx++];

         // Encode repetitions in the 'length' if the flag of the symbol is set.
         if ((prev == val) && (run < maxRun) && (this.counters[prev&0xFF] > 0))
         {
            if (++run < threshold)
               dst[dstIdx++] = prev;

            continue;
         }

         if (run >= threshold)
         {
            run -= threshold;

            if (dstIdx >= dstEnd4)
            {
               if (run >= RUN_LEN_ENCODE2)
                  break;
               
               if ((run >= RUN_LEN_ENCODE1) && (dstIdx > dstEnd4))
                  break;
            }
            
            dst[dstIdx++] = prev;
        
            // Encode run length
            if (run >= RUN_LEN_ENCODE1)
            {
               if (run < RUN_LEN_ENCODE2)
               {
                  run -= RUN_LEN_ENCODE1;
                  dst[dstIdx++] = (byte) (RUN_LEN_ENCODE1 + (run>>8));                  
               }
               else
               {
                  run -= RUN_LEN_ENCODE2;
                  dst[dstIdx++] = (byte) 0xFF;               
                  dst[dstIdx++] = (byte) (run>>8);                              
               }
            }
            
            dst[dstIdx++] = (byte) run;
         }

         dst[dstIdx++] = val;
         prev = val;
         run = 1;
      }
      
      // Fill up the destination array
      if (run >= threshold)
      {
         run -= threshold;
         
         if (dstIdx >= dstEnd4)
         {
            if (run >= RUN_LEN_ENCODE2)
               res = false;              
            else if ((run >= RUN_LEN_ENCODE1) && (dstIdx > dstEnd4))
               res = false;
         }
         else
         {
            dst[dstIdx++] = prev;

            // Encode run length
            if (run >= RUN_LEN_ENCODE1)
            {
               if (run < RUN_LEN_ENCODE2)
               {
                  run -= RUN_LEN_ENCODE1;
                  dst[dstIdx++] = (byte) (RUN_LEN_ENCODE1 + (run>>8));
               }
               else
               {
                  run -= RUN_LEN_ENCODE2;
                  dst[dstIdx++] = (byte) 0xFF;               
                  dst[dstIdx++] = (byte) (run>>>8);                             
               }
            }
            
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
      final int maxRun = MAX_RUN + this.runThreshold;
      boolean res = true;

      // Read compression flags from input
      for (int i=0, j=0; i<32; i++, j+=8)
      {
         final byte flag = src[srcIdx++];
         this.counters[j]   = (flag>>7) & 1;
         this.counters[j+1] = (flag>>6) & 1;
         this.counters[j+2] = (flag>>5) & 1;
         this.counters[j+3] = (flag>>4) & 1;
         this.counters[j+4] = (flag>>3) & 1;
         this.counters[j+5] = (flag>>2) & 1;
         this.counters[j+6] = (flag>>1) & 1;
         this.counters[j+7] =  flag     & 1;
      }
      
      // Initialize with a value different from the first symbol
      byte prev = (byte) ~src[srcIdx];

      while ((srcIdx < srcEnd))
      { 
         final byte val = src[srcIdx++];

         if ((prev == val) && (this.counters[prev&0xFF] > 0))
         {
            run++;
            
            if (run >= threshold)
            {
               // Decode run length
               run = src[srcIdx++] & 0xFF;

               if (run == 0xFF)
               {
                  if (srcIdx + 1 >= srcEnd)
                     break;

                  run = src[srcIdx++] & 0xFF;
                  run = (run<<8) | (src[srcIdx++] & 0xFF);
                  run += RUN_LEN_ENCODE2;
               }
               else if (run >= RUN_LEN_ENCODE1)
               {
                  if (srcIdx >= srcEnd)
                     break;
                  
                  run = ((run-RUN_LEN_ENCODE1)<<8) | (src[srcIdx++] & 0xFF);
                  run += RUN_LEN_ENCODE1;
               }
               
               if ((dstIdx >= dstEnd + run) || (run > maxRun))
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

         if (dstIdx >= dstEnd)
            break;
            
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
      return srcLen + 32;
   }
}