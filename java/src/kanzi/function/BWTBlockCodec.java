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
import kanzi.transform.BWT;


// Utility class to en/de-code a BWT data block and its associated primary index(es)

// BWT stream format: Header (m bytes) Data (n bytes)
// Header: For each primary index,
//   mode (8 bits) + primary index (8,16 or 24 bits)
//   mode: bits 7-6 contain the size in bits of the primary index :
//             00: primary index size <=  6 bits (fits in mode byte)
//             01: primary index size <= 14 bits (1 extra byte)
//             10: primary index size <= 22 bits (2 extra bytes)
//             11: primary index size  > 22 bits (3 extra bytes)
//         bits 5-0 contain 6 most significant bits of primary index
//   primary index: remaining bits (up to 3 bytes)

public class BWTBlockCodec implements ByteFunction
{
   private final BWT bwt;

   
   public BWTBlockCodec()
   {
      this.bwt = new BWT();   
   }
   

   // Return true if the compression chain succeeded. In this case, the input data 
   // may be modified. If the compression failed, the input data is returned unmodified.
   @Override
   public boolean forward(SliceByteArray input, SliceByteArray output)
   {
      if ((!SliceByteArray.isValid(input)) || (!SliceByteArray.isValid(output)))
         return false;

      if (input.array == output.array)
         return false;
      
      final int blockSize = input.length;

      if (output.length - output.index < getMaxEncodedLength(blockSize))
         return false;
      
      final int savedOIdx = output.index;
      final int chunks = BWT.getBWTChunks(blockSize);
      int log = 1;

      while (1<<log <= blockSize)
         log++; 
              
      log--;
      
      // Estimate header size based on block size
      final int headerSizeBytes1 = (chunks*(2+log)+7) >>> 3;
      output.index += headerSizeBytes1;
      output.length -= headerSizeBytes1;
     
      // Apply forward transform
      if (this.bwt.forward(input, output) == false)
         return false;

      int headerSizeBytes2 = 0;
      
      for (int i=0; i<chunks; i++)
      {
         int primaryIndex = this.bwt.getPrimaryIndex(i);
         int pIndexSizeBits = 6;

         while ((1<<pIndexSizeBits) <= primaryIndex)
            pIndexSizeBits++;          

         // Compute block size based on primary index
         headerSizeBytes2 += (2+pIndexSizeBits);
      }
      
      headerSizeBytes2 = (headerSizeBytes2+7) >>> 3;
      
      if (headerSizeBytes2 != headerSizeBytes1)
      {
         // Adjust space for header
         System.arraycopy(output.array, savedOIdx+headerSizeBytes1, 
            output.array, savedOIdx+headerSizeBytes2, blockSize);

         output.index = output.index - headerSizeBytes1 + headerSizeBytes2;
      }     
      
      int idx = savedOIdx;
      
      for (int i=0; i<chunks; i++)
      {
         int primaryIndex = this.bwt.getPrimaryIndex(i);
         int pIndexSizeBits = 6;

         while ((1<<pIndexSizeBits) <= primaryIndex)
            pIndexSizeBits++;          

         // Compute primary index size
         final int pIndexSizeBytes = (2+pIndexSizeBits+7) >>> 3;
   
         // Write block header (mode + primary index). See top of file for format 
         int shift = (pIndexSizeBytes - 1) << 3;
         int blockMode = (pIndexSizeBits + 1) >>> 3;
         blockMode = (blockMode << 6) | ((primaryIndex >>> shift) & 0x3F);
         output.array[idx++] = (byte) blockMode;

         for (int n=1; n<pIndexSizeBytes; n++)
         {
            shift -= 8;
            output.array[idx++] = (byte) (primaryIndex >> shift);
         }
      }
      
      return true;
   }


   @Override
   public boolean inverse(SliceByteArray input, SliceByteArray output)
   {
      if ((!SliceByteArray.isValid(input)) || (!SliceByteArray.isValid(output)))
         return false;

      if (input.array == output.array)
         return false;
      
      int blockSize = input.length;
      final int chunks = BWT.getBWTChunks(blockSize);

      for (int i=0; i<chunks; i++)
      {
         // Read block header (mode + primary index). See top of file for format
         final int blockMode = input.array[input.index++] & 0xFF;
         final int pIndexSizeBytes = 1 + ((blockMode >>> 6) & 0x03);

         if (input.length < pIndexSizeBytes)
             return false;
        
         input.length -= pIndexSizeBytes;
         int shift = (pIndexSizeBytes - 1) << 3;
         int primaryIndex = (blockMode & 0x3F) << shift;

         // Extract BWT primary index
         for (int n=1; n<pIndexSizeBytes; n++)
         {
            shift -= 8;
            primaryIndex |= ((input.array[input.index++] & 0xFF) << shift);
         }

         this.bwt.setPrimaryIndex(i, primaryIndex);
      }
      
      // Apply inverse Transform            
      return this.bwt.inverse(input, output);      
   }
   
     
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      // Return input buffer size + max header size
      return srcLen + 4*8; 
   }
}