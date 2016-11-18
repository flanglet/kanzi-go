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

package kanzi.util.sampling;

import kanzi.SliceIntArray;
import kanzi.IntTransform;
import kanzi.transform.DWT_CDF_9_7;
import kanzi.transform.DWT_Haar;


/**
 *
 * @author fred
 */
public class DWTUpSampler implements UpSampler
{
   private final int width;
   private final int height;
   private final int stride;
   private final int shift;
   private final IntTransform dwt;

   
   public DWTUpSampler(int w, int h)
   {
      this(w, h, w, 0, false);
   }
   
   
   public DWTUpSampler(int width, int height, int stride, int shift)
   {
      this(width, height, stride, shift, false);
   }
   
   
   // If shift > 0, the input values are rescaled (shifted by scaling factor)
   // It allows the input values to be in the byte range.
   public DWTUpSampler(int width, int height, int stride, int shift, boolean isHaar)
   {
      if (height < 8)
          throw new IllegalArgumentException("The height must be at least 8");

      if (width < 8)
          throw new IllegalArgumentException("The width must be at least 8");

      if (stride < width)
          throw new IllegalArgumentException("The stride must be at least as big as the width");

      if (shift < 0)
          throw new IllegalArgumentException("The rescaling factor must be positive or null");

      this.width = width;
      this.height = height;
      this.stride = stride;
      this.shift = shift;
      this.dwt = (isHaar) ? new DWT_Haar(this.width<<1, this.height<<1, 1, false) :
          new DWT_CDF_9_7(this.width<<1, this.height<<1, 1);
   }

   
   @Override
   public void superSampleHorizontal(int[] input, int[] output) 
   {
      throw new UnsupportedOperationException("Not supported yet.");       
   }

   
   @Override
   public void superSampleVertical(int[] input, int[] output) 
   {
      throw new UnsupportedOperationException("Not supported yet."); 
   }
   

   // input and output must be of same dimensions (width*height)
   @Override
   public void superSample(int[] input, int[] output) 
   {
      SliceIntArray src = new SliceIntArray(input, 0);
      SliceIntArray dst = new SliceIntArray(output, 0);
      
      if (this.shift > 0)
      {
         int offs = 0;
         final int sh = this.shift;
         final int h = this.height;
         final int w = this.width;

         for (int j=0; j<h; j++)
         {
            for (int i=0; i<w; i++) 
               input[offs+i] <<= sh;

            offs += this.stride;
         }         
      }
      
      this.dwt.inverse(src, dst);
   }

   
   @Override
   public boolean supportsScalingFactor(int factor) 
   {
      return (factor == 2);
   }
}
