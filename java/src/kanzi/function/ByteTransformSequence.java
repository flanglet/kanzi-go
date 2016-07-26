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


// Encapsulates a sequence of transforms or functions in a function 
public class ByteTransformSequence implements ByteFunction
{
   public static final int SKIP_MASK = 0x0F;
   
   private final ByteTransform[] transforms; // transforms or functions
   private byte skipFlags; // skip transforms: 0b0000yyyy with yyyy=flags
  
   
   public ByteTransformSequence(ByteTransform[] transforms) 
   {
      if (transforms == null)
         throw new NullPointerException("Invalid null transforms parameter");
      
      if ((transforms.length == 0) || (transforms.length > 4))
         throw new NullPointerException("Only 1 to 4 transforms allowed");
      
      this.transforms = transforms;
   }


   @Override
   public boolean forward(IndexedByteArray src, IndexedByteArray dst, int length)
   {  
      // Check for null buffers. Let individual transforms decide on buffer equality
      if ((src == null) || (dst == null))
         return false;

      if (length == 0)
         return true;
      
      if ((length < 0) || (length+src.index > src.array.length))
         return false;

      final int blockSize = length;
      final int savedIIdx0 = src.index; 
      final int savedOIdx0 = dst.index;
      IndexedByteArray input = dst;
      IndexedByteArray output = src;
      final int requiredSize = this.getMaxEncodedLength(length);
      this.skipFlags = 0;
      
      // Process transforms sequentially
      for (int i=0; i<this.transforms.length; i++)
      {         
         if (input == src)
         {
            input = dst;
            output = src;
            input.index = savedOIdx0;
            output.index = savedIIdx0;
         }
         else
         {
            input = src;
            output = dst;
            input.index = savedIIdx0;
            output.index = savedOIdx0;
         }

         // Check that the output buffer has enough room. If not, allocate a new one.
         if (output.array.length < requiredSize)
             output.array = new byte[requiredSize];
         
         final int savedIIdx = input.index;
         final int savedOIdx = output.index;
         ByteTransform transform = this.transforms[i];                 

         // Apply forward transform                 
         if (transform.forward(input, output, length) == false)
         {
            // Transform failed (probably due to lack of space in output). Revert
            if (input.array != output.array)
               System.arraycopy(input.array, savedIIdx, output.array, savedOIdx, length);

            output.index = savedOIdx + length;
            this.skipFlags |= (1<<(3-i));
         }

         length = output.index - savedOIdx;
      } 
      
      for (int i=this.transforms.length; i<4; i++)
          this.skipFlags |= (1<<(3-i));
      
      if (output != dst)
         System.arraycopy(src.array, savedIIdx0, dst.array, savedOIdx0, length);
            
      src.index = savedIIdx0 + blockSize;
      dst.index = savedOIdx0 + length;     
      return this.skipFlags != SKIP_MASK;
   }


   @Override
   public boolean inverse(IndexedByteArray src, IndexedByteArray dst, int length)
   {      
      if (length == 0)
         return true;

      if (this.skipFlags == SKIP_MASK)
      {
         if (src.array != dst.array)
            System.arraycopy(src.array, 0, dst.array, src.index, length);

         src.index += length;
         dst.index += length;
         return true;         
      }
      
      if ((length < 0) || (length+src.index > src.array.length))
         return false;
      
      final int blockSize = length;
      boolean res = true;
      final int savedIIdx0 = src.index; 
      final int savedOIdx0 = dst.index;
      IndexedByteArray input = dst;
      IndexedByteArray output = src;
     
      // Process transforms sequentially in reverse order
      for (int i=this.transforms.length-1; i>=0; i--)
      {         
         if ((this.skipFlags & (1<<(3-i))) != 0)
            continue;
         
         if (input == src)
         {
            input = dst;
            output = src;
            input.index = savedOIdx0;
            output.index = savedIIdx0;
         }
         else
         {
            input = src;
            output = dst;
            input.index = savedIIdx0;
            output.index = savedOIdx0;
         }

         final int savedOIdx = output.index;
         ByteTransform transform = this.transforms[i];                 
                  
         // Apply inverse transform
         res = transform.inverse(input, output, length);                  
         length = output.index - savedOIdx;

         // All inverse transforms must succeed
         if (res == false)
            break;
      } 
      
      if (output != dst)
         System.arraycopy(src.array, savedIIdx0, dst.array, savedOIdx0, length);
      
      src.index = savedIIdx0 + blockSize;
      dst.index = savedOIdx0 + length;
      return res;
   }


   @Override
   public int getMaxEncodedLength(int srcLength)
   {
      int requiredSize = srcLength;

      for (ByteTransform transform : this.transforms)
      {
         if (transform instanceof ByteFunction)
         {
            int reqSize = ((ByteFunction) transform).getMaxEncodedLength(srcLength);

            if (reqSize > requiredSize)
               requiredSize = reqSize;
         }
      }
      
      return requiredSize;
   }

   
   public byte getSkipFlags()
   {
      return this.skipFlags;
   }
   
   
   public boolean setSkipFlags(byte flags)
   {
      this.skipFlags = flags;
      return true;
   }
   
}
