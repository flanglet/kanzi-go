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
// mode: bit 7 is unused for now
//       bits 6-4 (contains the size in bits of the primary index - 1) / 4
//       bits 3-0 4 highest bits of primary index
// primary index: remaining bits (up to 3 bytes)
//
// EG: Mode=0bx.100.xxxx primary index is (4+1)*4=20 bits long
//     Mode=0bx.000.xxxx primary index is (0+1)*4=4 bits long

public class BlockCodec implements ByteFunction
{
   private static final int MAX_HEADER_SIZE  = 3;
   private static final int MAX_BLOCK_SIZE   = (16*1024*1024) - MAX_HEADER_SIZE;

   private final MTFT mtft;
   private final BWT bwt;
   private int size;


   public BlockCodec()
   {
      this(MAX_BLOCK_SIZE);
   }


   public BlockCodec(int blockSize)
   {
      if (blockSize < 0)
         throw new IllegalArgumentException("The block size cannot be negative");

      if (blockSize > MAX_BLOCK_SIZE)
         throw new IllegalArgumentException("The block size must be at most " + MAX_BLOCK_SIZE);

      this.bwt = new BWT();
      this.mtft = new MTFT();
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


   // Return true is the compression chain succeeded. In this case, the input data 
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
      final int primaryIndex = this.bwt.getPrimaryIndex();
      input.index = savedIIdx;
      output.index = savedOIdx;

      // Apply Move-To-Front Transform
      this.mtft.setSize(blockSize);
      this.mtft.forward(output, input);

      int pIndexSizeBits = 4;

      while ((1<<pIndexSizeBits) <= primaryIndex)
         pIndexSizeBits += 4;

      final int headerSizeBytes = (4 + pIndexSizeBits + 7) >> 3;
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
      
      // Write block header (mode + primary index)
      // 'mode' contains size of primaryIndex in bits (bits 6 to 4)
      // the size is divided by 4 and decreased by one
      byte mode = (byte) (((pIndexSizeBits >> 2) - 1) << 4);
      int shift = pIndexSizeBits;

      if ((shift & 7) == 4)
      {
         shift -= 4;
         mode |= (byte) ((primaryIndex >> shift) & 0x0F);
      }

      output.array[savedOIdx] = mode;

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

      // Read block header (mode + primary index)
      // 'mode' contains size of primaryIndex in bits (bits 6 to 4)
      // the size is divided by 4 and decreased by one
      byte mode = input.array[input.index++];
      final int pIndexSizeBits = (((mode & 0x70) >> 4) + 1) << 2;
      final int headerSizeBytes = (4 + pIndexSizeBits + 7) >> 3;
      compressedLength -= headerSizeBytes;

      if (compressedLength <= 0)
          return false;

      int shift = pIndexSizeBits;
      int primaryIndex = 0;

      if ((shift & 7) == 4)
      {
         shift -= 4;
         primaryIndex |= ((mode & 0x0F) << shift);
      }

      // Extract BWT primary index
      for (int i=1; i<headerSizeBytes; i++)
      {
         shift -= 8;
         primaryIndex |= ((input.array[input.index++] & 0xFF) << shift);
      }

      final int savedOIdx = output.index;
      ZLT zlt = new ZLT(compressedLength);

      // Apply Zero Length Decoding (changes the index of input & output)
      if (zlt.inverse(input, output) == false)
         return false;

      final int blockSize = output.index - savedOIdx;
      input.index = savedIIdx;
      output.index = savedOIdx;

      // Apply Move-To-Front Inverse Transform
      this.mtft.setSize(blockSize);
      this.mtft.inverse(output, input);

      // Apply Burrows-Wheeler Inverse Transform
      input.index = savedIIdx;
      output.index = savedOIdx;
      this.bwt.setPrimaryIndex(primaryIndex);
      this.bwt.setSize(blockSize);
      this.bwt.inverse(input, output);
      return true;
   }
   
     
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      // Return input buffer size + max header size
      // If forward() fails due to output buffer size, the block is returned 
      // unmodified with an error
      return srcLen + 32; 
   }
}