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
import kanzi.ByteTransform;
import kanzi.SliceByteArray;


// Encapsulates a sequence of transforms or functions in a function 
public class ByteTransformSequence implements ByteFunction
{
   private static final int SKIP_MASK = 0xFF;
   
   private final ByteTransform[] transforms; // transforms or functions
   private byte skipFlags; // skip transforms
  
   
   public ByteTransformSequence(ByteTransform[] transforms) 
   {
      if (transforms == null)
         throw new NullPointerException("Invalid null transforms parameter");
      
      if ((transforms.length == 0) || (transforms.length > 8))
         throw new NullPointerException("Only 1 to 8 transforms allowed");
      
      this.transforms = transforms;
   }


   @Override
   public boolean forward(SliceByteArray src, SliceByteArray dst)
   {  
      // Check for null buffers. Let individual transforms decide on buffer equality
      if ((src == null) || (dst == null))
         return false;

      if ((src.array == null) || (dst.array == null))
         return false;

      int count = src.length;
      
      if (count == 0)
         return true;
      
      if ((count < 0) || (count+src.index > src.array.length))
         return false;

      final int blockSize = count;
      SliceByteArray[] sa = new SliceByteArray[] 
      { 
         new SliceByteArray(src.array, src.length, src.index),
         new SliceByteArray(dst.array, dst.length, dst.index)
      };
      
      int saIdx = 0;
      
      final int requiredSize = this.getMaxEncodedLength(count);
      this.skipFlags = 0;
      
      // Process transforms sequentially
      for (int i=0; i<this.transforms.length; i++)
      {        
         SliceByteArray sa1 = sa[saIdx];
         SliceByteArray sa2 = sa[saIdx^1];
            
         // Check that the output buffer has enough room. If not, allocate a new one.
         if (sa2.length < requiredSize)
         {
            sa2.length = requiredSize;
            
            if (sa2.array.length < sa2.length)
               sa2.array = new byte[sa2.length];
         }
         
         final int savedIIdx = sa1.index;
         final int savedOIdx = sa2.index;
         ByteTransform transform = this.transforms[i];                 
         sa1.length = count;
         
         // Apply forward transform            
         if (transform.forward(sa1, sa2) == false)
         {
            // Transform failed (probably due to lack of space in output). Revert
            if (sa1.array != sa2.array)
               System.arraycopy(sa1.array, savedIIdx, sa2.array, savedOIdx, count);

            sa2.index = savedOIdx + count;
            this.skipFlags |= (1<<(7-i));
         }

         count = sa2.index - savedOIdx;
         sa1.index = savedIIdx;
         sa2.index = savedOIdx;
         saIdx ^= 1;
      } 
      
      for (int i=this.transforms.length; i<8; i++)
          this.skipFlags |= (1<<(7-i));
            
      if (saIdx != 1)
         System.arraycopy(sa[0].array, sa[0].index, sa[1].array, sa[1].index, count);
            
      src.index += blockSize;
      dst.index += count;     
      return this.skipFlags != SKIP_MASK;
   }


   @Override
   public boolean inverse(SliceByteArray src, SliceByteArray dst)
   {      
      if ((src == null) || (dst == null))
         return false;

      if ((src.array == null) || (dst.array == null))
         return false;

      int count = src.length;
     
      if (count == 0)
         return true;

      if ((count < 0) || (count+src.index > src.array.length))
         return false;
      
      if (this.skipFlags == SKIP_MASK)
      {
         if (src.array != dst.array)
            System.arraycopy(src.array, src.index, dst.array, dst.index, count);

         src.index += count;
         dst.index += count;
         return true;         
      }     
      
      final int blockSize = count;
      boolean res = true;
      SliceByteArray[] sa = new SliceByteArray[] 
      { 
         new SliceByteArray(src.array, src.length, src.index),
         new SliceByteArray(dst.array, dst.length, dst.index)
      };
      
      int saIdx = 0;
     
      // Process transforms sequentially in reverse order
      for (int i=this.transforms.length-1; i>=0; i--)
      {         
         if ((this.skipFlags & (1<<(7-i))) != 0)
            continue;        

         SliceByteArray sa1 = sa[saIdx];
         saIdx ^= 1;
         SliceByteArray sa2 = sa[saIdx];
         final int savedIIdx = sa1.index;
         final int savedOIdx = sa2.index;
         ByteTransform transform = this.transforms[i];                 
                  
         // Apply inverse transform
         sa1.length = count; 
         sa2.length = dst.length;
                 
         if (sa2.array.length < sa2.length)
            sa2.array = new byte[sa2.length];
         
         res = transform.inverse(sa1, sa2);                  
         count = sa2.index - savedOIdx;
         sa1.index = savedIIdx;
         sa2.index = savedOIdx;
         
         // All inverse transforms must succeed
         if (res == false)
            break;  
      } 
      
      if (saIdx != 1)
         System.arraycopy(sa[0].array, sa[0].index, sa[1].array, sa[1].index, count);
      
      if (count > dst.length)
         return false;
     
      src.index += blockSize;
      dst.index += count;
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

   
   public int getNbFunctions()
   {
      return this.transforms.length;
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
