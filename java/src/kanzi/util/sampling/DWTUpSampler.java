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
   private int[] guide;
   
   
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
      this.guide = new int[0];      
   }

   
   @Override
   public void superSampleHorizontal(int[] input, int[] output) 
   {
      throw new UnsupportedOperationException("Not supported.");       
   }

   
   @Override
   public void superSampleVertical(int[] input, int[] output) 
   {
      throw new UnsupportedOperationException("Not supported."); 
   }
   

   // input and output must be of same dimensions (width*height)
   @Override
   public void superSample(int[] input, int[] output) 
   {
      SliceIntArray src = new SliceIntArray(input, 0);
      SliceIntArray dst = new SliceIntArray(output, 0);
      final int h = this.height;
      final int w = this.width;      
      final int st = this.stride;      
      
//      if (this.guide.length > 0)
//      {
//         // Use guide to estimate, HL, LH and HH coefficients
//         this.dwt.forward(new SliceIntArray(this.guide, w*h*4, 0), dst);
//         int startLine = 0;
//         final int h2 = h << 1;
//         final int w2 = w << 1;
//         
//         for (int j=0; j<h2; j++)
//         {
//            final int offs = (j < h) ? w : 0;
//            System.arraycopy(output, startLine+offs, input, startLine+offs, w2-offs);            
//            startLine += st;
//         }
//      }
      
      if (this.shift > 0)
      {
         // Rescale LL coefficients
         int startLine = 0;
         final int sh = this.shift;

         for (int j=0; j<h; j++)
         {
            for (int i=0; i<w; i++) 
               input[startLine+i] <<= sh;

            startLine += st;
         }         
      }

      src.index = 0;
      dst.index = 0;
      this.dwt.inverse(src, dst);
   }
   

//   public boolean setGuide(int[] guide)
//   {
//      if (guide == null)
//         return false;
//       
//      if (guide.length < 4*this.width*this.height)
//         return false;
//       
//      if (this.guide.length < 4*this.width*this.height)
//         this.guide = new int[4*this.width*this.height];
//      
//      System.arraycopy(guide, 0, this.guide, 0, 4*this.width*this.height);
//      return true;
//   }

   
   @Override
   public boolean supportsScalingFactor(int factor) 
   {
      return (factor == 2);
   }
}
