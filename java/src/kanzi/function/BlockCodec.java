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
import kanzi.transform.BWT;
import kanzi.transform.MTFT;


// Utility class to compress/decompress a data block
// Fast reversible block coder/decoder based on a pipeline of transformations:
// Forward: Burrows-Wheeler -> Move to Front -> Zero Length
// Inverse: Zero Length -> Move to Front -> Burrows-Wheeler
// The block size determines the balance between speed and compression ratio

// Stream format: Header (m bytes) Data (n bytes)
// Header: mode (8 bits) + BWT primary index (8, 16 or 24 bits)
// mode: bits 7-6 contain the size in bits of the primary index : 
//           00: primary index size <=  6 bits (fits in mode byte)
//           01: primary index size <= 14 bits (1 extra byte)
//           10: primary index size <= 22 bits (2 extra bytes)
//           11: primary index size  > 22 bits (3 extra bytes)
//       bits 5-0 contain 6 most significant bits of primary index
// primary index: remaining bits (up to 3 bytes) 

public class BlockCodec implements ByteFunction
{
   private static final int MAX_HEADER_SIZE  = 4;
   private static final int MAX_BLOCK_SIZE   = (32*1024*1024) - MAX_HEADER_SIZE;

   private final boolean postProcessing;
   private final MTFT mtft;
   private final BWT bwt;
   private int size;

   
   public BlockCodec()
   {
      this(MAX_BLOCK_SIZE, true);
   }

   
   public BlockCodec(int blockSize)
   {
      this(blockSize, true);
   }

   
   // If postProcessing is true, forward BWT is followed by a Global Structure 
   // Transform (here MTFT) and ZLT, else a raw BWT is performed.
   public BlockCodec(int blockSize, boolean postProcessing)
   {
      if (blockSize < 0)
         throw new IllegalArgumentException("The block size cannot be negative");

      if (blockSize > MAX_BLOCK_SIZE)
         throw new IllegalArgumentException("The block size must be at most " + MAX_BLOCK_SIZE);
  
      this.postProcessing = postProcessing;
      this.bwt = new BWT();
      this.mtft = (postProcessing == true) ? new MTFT() : null;
      this.size = blockSize;
   }


   public int size()
   {
       return this.size;
   }


   public boolean setSize(int size)
   {
       if ((size < 0) || (size > MAX_BLOCK_SIZE))
          return false;

       this.size = size;
       return true;
   }


   // Return true if the compression chain succeeded. In this case, the input data 
   // may be modified. If the compression failed, the input data is returned unmodified.
   @Override
   public boolean forward(IndexedByteArray input, IndexedByteArray output)
   {
      if ((input == null) || (output == null) || (input.array == output.array))
         return false;

      final int blockSize = (this.size == 0) ? input.array.length - input.index : this.size;

      if ((blockSize < 0) || (blockSize > MAX_BLOCK_SIZE))
         return false;

      if (blockSize + input.index > input.array.length)
         return false;

      final int savedIIdx = input.index; 
      final int savedOIdx = output.index;
      
      // Apply Burrows-Wheeler Transform
      this.bwt.setSize(blockSize);
      this.bwt.forward(input, output);
      
      int primaryIndex = this.bwt.getPrimaryIndex();
      int pIndexSizeBits = 6;

      while ((1<<pIndexSizeBits) <= primaryIndex)
         pIndexSizeBits++;          

      final int headerSizeBytes = (2 + pIndexSizeBits + 7) >> 3;
     
      if (this.postProcessing == true)
      {
         input.index = savedIIdx;
         output.index = savedOIdx;

         // Apply Move-To-Front Transform
         this.mtft.setSize(blockSize);
         this.mtft.forward(output, input);

         input.index = savedIIdx;
         output.index = savedOIdx + headerSizeBytes;
         ZLT zlt = new ZLT(blockSize);

         // Apply Zero Length Encoding (changes the index of input & output)
         if (zlt.forward(input, output) == false)
         {
            // Compression failed, recover source data
            input.index = savedIIdx;
            output.index = savedOIdx;
            this.mtft.inverse(input, output);
            input.index = savedIIdx;
            output.index = savedOIdx;
            this.bwt.inverse(output, input);
            return false;
         }
      } 
      else
      {
         // Shift output data to leave space for header
         System.arraycopy(output.array, savedOIdx, output.array, savedOIdx+headerSizeBytes, blockSize);
         output.index += headerSizeBytes;
      }
      
      // Write block header (mode + primary index). See top of file for format 
      int shift = (headerSizeBytes - 1) << 3;
      int mode = (pIndexSizeBits + 1) >> 3;
      mode = (mode << 6) | ((primaryIndex >> shift) & 0x3F);
      output.array[savedOIdx] = (byte) mode;

      for (int i=1; i<headerSizeBytes; i++)
      {
         shift -= 8;
         output.array[savedOIdx+i] = (byte) (primaryIndex >>> shift);
      }

      return true;
   }


   @Override
   public boolean inverse(IndexedByteArray input, IndexedByteArray output)
   {
      int compressedLength = this.size;

      if (compressedLength == 0)
         return true;

      final int savedIIdx = input.index; 

      // Read block header (mode + primary index). See top of file for format
      int mode = input.array[input.index++] & 0xFF;
      int headerSizeBytes = 1 + ((mode >> 6) & 0x03);

      if (compressedLength < headerSizeBytes)
          return false;

      if (compressedLength == headerSizeBytes)
          return true;

      compressedLength -= headerSizeBytes;
      int shift = (headerSizeBytes - 1) << 3;
      int primaryIndex = (mode & 0x3F) << shift;
      int blockSize = compressedLength;

      // Extract BWT primary index
      for (int i=1; i<headerSizeBytes; i++)
      {
         shift -= 8;
         primaryIndex |= ((input.array[input.index++] & 0xFF) << shift);
      }
      
      if (this.postProcessing == true)
      {
         final int savedOIdx = output.index;
         ZLT zlt = new ZLT(compressedLength);

         // Apply Zero Length Decoding (changes the index of input & output)
         if (zlt.inverse(input, output) == false)
            return false;

         blockSize = output.index - savedOIdx;
         input.index = savedIIdx;
         output.index = savedOIdx;

         // Apply Move-To-Front Inverse Transform
         this.mtft.setSize(blockSize);
         this.mtft.inverse(output, input);

         input.index = savedIIdx;
         output.index = savedOIdx;
      }
      
      // Apply Burrows-Wheeler Inverse Transform      
      this.bwt.setPrimaryIndex(primaryIndex);
      this.bwt.setSize(blockSize);     
      return this.bwt.inverse(input, output);
   }
   
     
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      // Return input buffer size + max header size
      // If forward() fails due to output buffer size, the block is returned 
      // unmodified with an error
      return srcLen + 4; 
   }
}