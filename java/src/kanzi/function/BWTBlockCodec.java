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
import kanzi.ByteTransform;
import kanzi.IndexedByteArray;
import kanzi.Sizeable;
import kanzi.transform.BWT;
import kanzi.transform.MTFT;
import kanzi.transform.SBRT;


// Utility class to compress/decompress a data block
// Fast reversible block coder/decoder based on a pipeline of transformations:
// Forward: (Bijective) Burrows-Wheeler -> Move to Front -> Zero Run Length
// Inverse: Zero Run Length -> Move to Front -> (Bijective) Burrows-Wheeler
// The block size determines the balance between speed and compression ratio

// BWT stream format: Header (m bytes) Data (n bytes)
// Header: mode (8 bits) + BWT primary index (8, 16 or 24 bits)
// mode: bits 7-6 contain the size in bits of the primary index :
//           00: primary index size <=  6 bits (fits in mode byte)
//           01: primary index size <= 14 bits (1 extra byte)
//           10: primary index size <= 22 bits (2 extra bytes)
//           11: primary index size  > 22 bits (3 extra bytes)
//       bits 5-0 contain 6 most significant bits of primary index
// primary index: remaining bits (up to 3 bytes)
// Bijective BWT stream format: Data (n bytes)

public class BWTBlockCodec implements ByteFunction, Sizeable
{
   public static final int MODE_RAW = 0;
   public static final int MODE_MTF = 1;
   public static final int MODE_RANK = 2;
   public static final int MODE_TIMESTAMP = 3;
 
   private static final int BWT_MAX_HEADER_SIZE  = 4;
   private static final int MAX_BLOCK_SIZE = 256*1024*1024; // 30 bits

   private final int mode;
   private final boolean isBWT;
   private final ByteTransform transform;
   private int size;

   
   public BWTBlockCodec()
   {
      this(new BWT(), MODE_MTF, 4*1024*1024);
   }

   
   public BWTBlockCodec(int postTransformMode, int blockSize)
   {
      this(new BWT(), postTransformMode, blockSize);
   }

   
   // Base on the mode, the forward transform is followed by a Global Structure 
   // Transform and ZRLT, else a raw transform is performed.
   public BWTBlockCodec(ByteTransform transform, int mode, int blockSize)
   {
      if (transform == null)
        throw new NullPointerException("Invalid null transform parameter");

      if ((transform instanceof Sizeable) == false)
        throw new IllegalArgumentException("The transform must implement the Sizeable interface");

      if ((mode != MODE_RAW) && (mode != MODE_MTF) && (mode != MODE_RANK) && (mode != MODE_TIMESTAMP))
        throw new IllegalArgumentException("Invalid GST mode parameter");
     
      if (blockSize < 0)
         throw new IllegalArgumentException("The block size cannot be negative");
        
      this.mode = mode;
      this.transform = transform;
      this.size = blockSize;      
      this.isBWT = (transform instanceof BWT);  

      if (blockSize > this.maxBlockSize())
         throw new IllegalArgumentException("The max block size for the BWT" +
                 ((this.isBWT) ? "" : "S") + " is " + this.maxBlockSize());
   }


   protected ByteTransform createGST(int blockSize) 
   {
      // SBRT can perform MTFT but the dedicated class is faster
      if (this.mode == MODE_RAW)
         return null;
      
      if (this.mode == MODE_MTF) 
         return new MTFT(blockSize);      
      
      return new SBRT(this.mode, blockSize);            
   }
   
   
   private int maxBlockSize() 
   {
      return (this.isBWT == true) ? MAX_BLOCK_SIZE - BWT_MAX_HEADER_SIZE : MAX_BLOCK_SIZE;      
   }
   
   
   @Override
   public int size()
   {
       return this.size;
   }


   @Override
   public boolean setSize(int size)
   {
       if ((size < 0) || (size > this.maxBlockSize()))
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

      if ((blockSize < 0) || (blockSize > this.maxBlockSize()))
         return false;

      if (blockSize + input.index > input.array.length)
         return false;

      final int savedIIdx = input.index; 
      final int savedOIdx = output.index;
      
      ((Sizeable) this.transform).setSize(blockSize);

      // Apply forward transform
      this.transform.forward(input, output);
      
      int headerSizeBytes = 0;
      int pIndexSizeBits = 0;
      int primaryIndex = 0;
      
      if (this.isBWT == true)
      {
         primaryIndex = ((BWT) this.transform).getPrimaryIndex();
         pIndexSizeBits = 6;

         while ((1<<pIndexSizeBits) <= primaryIndex)
            pIndexSizeBits++;          

         headerSizeBytes = (2 + pIndexSizeBits + 7) >> 3;
      }
     
      if (this.mode != MODE_RAW)
      {
         input.index = savedIIdx;
         output.index = savedOIdx;

         // Apply Post Transform
         ByteTransform gst = this.createGST(blockSize);         
         gst.forward(output, input);

         input.index = savedIIdx;
         output.index = savedOIdx + headerSizeBytes;
         ZRLT zrlt = new ZRLT(blockSize);

         // Apply Zero Run Length Encoding (changes the index of input & output)
         if (zrlt.forward(input, output) == false)
         {
            // Compression failed, recover source data
            input.index = savedIIdx;
            output.index = savedOIdx;
            gst.inverse(input, output);
            input.index = savedIIdx;
            output.index = savedOIdx;
            this.transform.inverse(output, input);
            return false;
         }
      }      
      else if (headerSizeBytes > 0)
      {
         // Shift output data to leave space for header
         System.arraycopy(output.array, savedOIdx, output.array, savedOIdx+headerSizeBytes, blockSize);
         output.index += headerSizeBytes;
      } 
      
      if (this.isBWT == true)
      {
         // Write block header (mode + primary index). See top of file for format 
         int shift = (headerSizeBytes - 1) << 3;
         int blockMode = (pIndexSizeBits + 1) >> 3;
         blockMode = (blockMode << 6) | ((primaryIndex >> shift) & 0x3F);
         output.array[savedOIdx] = (byte) blockMode;

         for (int i=1; i<headerSizeBytes; i++)
         {
            shift -= 8;
            output.array[savedOIdx+i] = (byte) (primaryIndex >> shift);
         }
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
      int primaryIndex = 0;
      int blockSize = compressedLength;

      if (this.isBWT == true)
      {
         // Read block header (mode + primary index). See top of file for format
         int blockMode = input.array[input.index++] & 0xFF;
         int headerSizeBytes = 1 + ((blockMode >> 6) & 0x03);

         if (compressedLength < headerSizeBytes)
             return false;

         if (compressedLength == headerSizeBytes)
             return true;

         compressedLength -= headerSizeBytes;
         int shift = (headerSizeBytes - 1) << 3;
         primaryIndex = (blockMode & 0x3F) << shift;
         blockSize = compressedLength;

         // Extract BWT primary index
         for (int i=1; i<headerSizeBytes; i++)
         {
            shift -= 8;
            primaryIndex |= ((input.array[input.index++] & 0xFF) << shift);
         }
      }
      
      if ((blockSize < 0) || (blockSize > this.maxBlockSize()))
         return false;

      if (this.mode != MODE_RAW)
      {
         final int savedOIdx = output.index;
         ZRLT zrlt = new ZRLT(compressedLength);

         // Apply Zero Run Length Decoding (changes the index of input & output)
         if (zrlt.inverse(input, output) == false)
            return false;

         blockSize = output.index - savedOIdx;
         input.index = savedIIdx;
         output.index = savedOIdx;

         // Apply inverse Pre Transform
         ByteTransform gst = this.createGST(blockSize);
         gst.inverse(output, input);

         input.index = savedIIdx;
         output.index = savedOIdx;
      }
      
      if (this.isBWT == true)
         ((BWT) this.transform).setPrimaryIndex(primaryIndex);
      
      ((Sizeable) this.transform).setSize(blockSize);     

      // Apply inverse Transform            
      return this.transform.inverse(input, output);
   }
   
     
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      // Return input buffer size + max header size
      // If forward() fails due to output buffer size, the block is returned 
      // unmodified with an error
      return srcLen + (this.isBWT ? 4 : 0); 
   }
}