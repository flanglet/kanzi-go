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

package kanzi.transform;

import kanzi.IndexedIntArray;
import kanzi.IntTransform;


// Hybrid Discrete Wavelet Transform / Discrete Cosine Transform for 2D signals
// May not be exact due to integer rounding errors.
public class DWT_DCT implements IntTransform
{
   private final IntTransform dwt;
   private final IntTransform dct;
   private final int dim;
   private final int[] buffer;
   
   
   public DWT_DCT(int dim) 
   {               
      IntTransform transform;
      
      switch (dim)
      {
         case 8 : 
            transform = new DCT4();
            break;            
         case 16 :
            transform = new DCT8();
            break;            
         case 32 : 
            transform = new DCT16();
            break;            
         case 64 : 
            transform = new DCT32();
            break;            
         default:
            throw new IllegalArgumentException("Invalid transform dimension (must be 8, 16, 32 or 64)");
      }
      
      this.dim = dim;
      this.dct = transform;
      this.dwt = new DWT_CDF_9_7(dim, dim, 1);
      this.buffer = new int[dim*dim];
   }


   // Perform a DWT on the input then a DCT of the LL band.
   @Override
   public boolean forward(IndexedIntArray src, IndexedIntArray dst)
   {
      if ((src == null) || (dst == null))
         return false;
      
      if (src.array.length < this.dim*this.dim)
         return false;

      if (dst.array.length < this.dim*this.dim)
         return false;

      final int d2 = this.dim >> 1;
      IndexedIntArray iia = new IndexedIntArray(this.buffer, 0);

      // Forward DWT
      if (this.dwt.forward(src, dst) == false)
         return false;
      
      // Copy and compact DWT results for LL band
      for (int j=0; j<d2; j++)
         System.arraycopy(dst.array, j*this.dim, iia.array, j*d2, d2);

      // Forward DCT of LL band
      if (this.dct.forward(iia, iia) == false)
         return false;
      
      // Copy back DCT results
      for (int j=0; j<d2; j++)
         System.arraycopy(iia.array, j*d2, dst.array, j*this.dim, d2);
      
      return true;
   }


   // Perform a DWT on the input then a DCT of the LL band.
   @Override
   public boolean inverse(IndexedIntArray src, IndexedIntArray dst)
   {
      if ((src == null) || (dst == null))
         return false;
      
      if (src.array.length < this.dim*this.dim)
         return false;

      if (dst.array.length < this.dim*this.dim)
         return false;

      final int d2 = this.dim >> 1;
      IndexedIntArray iia = new IndexedIntArray(this.buffer, 0);
      
      // Copy and compact LL band
      for (int j=0; j<d2; j++)
         System.arraycopy(src.array, j*this.dim, iia.array, j*d2, d2);
    
      // Reverse DCT of LL band
      if (this.dct.inverse(iia, iia) == false)
         return false;

      // Copy and expand DCT results for LL band
      for (int j=0; j<d2; j++)
         System.arraycopy(iia.array, j*d2, src.array, j*this.dim, d2);
     
      return this.dwt.inverse(src, dst);
   }
}
