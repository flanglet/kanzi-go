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

import kanzi.Global;


// Bilinear upsampling guided by a full resolution frame.
// Typical use: upsampling of U and V channels in YUV_20 with Y channel as guide.
public class GuidedBilinearUpSampler implements UpSampler
{
   private final int width;
   private final int height;
   private final int stride;
   private final int offset;
   private int[] guide;
   private BilinearUpSampler delegate;


   public GuidedBilinearUpSampler(int width, int height)
   {
      this(width, height, width, 0, null);
   }

    
   public GuidedBilinearUpSampler(int width, int height, int[] guide)
   {
      this(width, height, width, 0, guide);
   }

    
   public GuidedBilinearUpSampler(int width, int height, int stride, int offset, int[] guide)
   {
      if (height < 8)
         throw new IllegalArgumentException("The height must be at least 8");

      if (width < 8)
         throw new IllegalArgumentException("The width must be at least 8");

      if (offset < 0)
         throw new IllegalArgumentException("The offset must be at least 0");

      if (stride < width)
         throw new IllegalArgumentException("The stride must be at least as big as the width");

      if ((height & 7) != 0)
         throw new IllegalArgumentException("The height must be a multiple of 8");

      if ((width & 7) != 0)
         throw new IllegalArgumentException("The width must be a multiple of 8");

      if ((guide != null) && (guide.length != 4*width*height))
         throw new IllegalArgumentException("The guide must be of dimensions " +
            width + "*" + height);

      this.height = height;
      this.width = width;
      this.stride = stride;
      this.offset = offset;
      this.guide = (guide == null) ? new int[0] : guide;
   }


   public boolean setGuide(int[] guide)
   {
      if (guide == null)
         return false;
       
      if (guide.length < 4*this.width*this.height)
         return false;
       
      if (this.guide.length < 4*this.width*this.height)
         this.guide = new int[4*this.width*this.height];
      
      System.arraycopy(guide, 0, this.guide, 0, 4*this.width*this.height);
      return true;
   }
    
    
   @Override
   // Supports in place resampling
   // guide is full resolution: Y0 Y1 Y2 Y3 Y4
   // data is half resolution:  u0 .. u2 .. u4
   // Interpolation is guided by values Y1, Y3, ....
   // EG. u1=(k1*u0+k2*u2)/(k1+k2) with k1=abs(Y2-Y1) and k2=abs(Y1-Y0)
   public void superSampleVertical(int[] input, int[] output)
   {
      if (this.guide.length == 0)
      {
         // Fallback to unguided super sampling
         if (this.delegate == null)
            this.delegate = new BilinearUpSampler(this.width, this.height, this.stride, this.offset, 2);

         this.delegate.superSampleVertical(input, output);
         return;
      }
      
      final int sw = this.width;
      final int sh = this.height;
      final int st = this.stride;
      final int dw = sw;
      final int dh = sh << 1;
      int iOffs = st*(sh-1) + this.offset;
      int oOffs = dw*(dh-1);
      System.arraycopy(input, iOffs, output, oOffs, sw);
      oOffs -= dw;
      System.arraycopy(input, iOffs, output, oOffs, sw);           
      oOffs -= dw;
      iOffs -= st;
     
      for (int j=sh-2; j>0; j--)
      {
         // Interpolate odd lines
         for (int i=0; i<sw; i++)
         {
            final int idx  = oOffs + i;
            final int idx1 = iOffs + i;
            final int idx2 = idx1  + st;
            final int k2 = Global.abs(this.guide[idx-st]-this.guide[idx]);
            final int k1 = Global.abs(this.guide[idx+st]-this.guide[idx]);            
            output[idx] = (k1+k2 == 0) ? input[idx1] : (k1*input[idx1] + k2*input[idx2]) / (k1+k2);
         } 

         // Copy even lines
         oOffs -= dw;
         System.arraycopy(input, iOffs, output, oOffs, sw);

         oOffs -= dw;
         iOffs -= st;
      }

      System.arraycopy(input, iOffs, output, oOffs, sw);
      oOffs -= dw;  
      System.arraycopy(input, iOffs, output, oOffs, sw);
   }


   @Override
   // Supports in place resampling
   // guide is full resolution: Y0 Y1 Y2 Y3 Y4
   // data is half resolution:  u0 .. u2 .. u4
   // Interpolation is guided by values Y1, Y3, ....
   // EG. u1=(k1*u0+k2*u2)/(k1+k2) with k1=abs(Y2-Y1) and k2=abs(Y1-Y0)
   public void superSampleHorizontal(int[] input, int[] output)
   {
      if (this.guide.length == 0)
      {
         // Fallback to unguided super sampling
         if (this.delegate == null)
            this.delegate = new BilinearUpSampler(this.width, this.height, this.stride, this.offset, 2);

         this.delegate.superSampleHorizontal(input, output);
         return;
      }
      
      final int sw = this.width;
      final int sh = this.height;
      final int st = this.stride;
      final int dw = sw << 1;
      final int dh = sh;
      int iOffs = st*(sh-1) + this.offset;
      int oOffs = dw*(dh-1);

      for (int j=sh-1; j>=0; j--)
      {
         // Columns w-1, w-2
         int prv = input[iOffs+sw-1];
         output[oOffs+dw-1] = prv;
         output[oOffs+dw-2] = prv;

         for (int i=sw-2; i>=0; i--)
         {
            final int idx = oOffs + (i<<1);
            final int val = input[iOffs+i];

            // Copy even columns
            output[idx] = val;

            // Using guide to interpolate odd columns:
            final int k2 = Global.abs(this.guide[idx+1]-this.guide[idx]);
            final int k1 = Global.abs(this.guide[idx+2]-this.guide[idx+1]);
            output[idx+1] = (k1+k2 == 0) ? prv : (k1*prv + k2*val) / (k1+k2);
            prv = val;
         }

         iOffs -= st;
         oOffs -= dw;
      }
   }


   @Override
   // Supports in place resampling
   public void superSample(int[] input, int[] output)
   {
      if (this.guide.length == 0)
      {
         // Fallback to unguided super sampling
         if (this.delegate == null)
            this.delegate = new BilinearUpSampler(this.width, this.height, this.stride, this.offset, 2);

         this.delegate.superSample(input, output);
         return;
      }
      
      final int st = this.stride;
      final int dw = this.width << 1;
      final int dw2 = dw + dw;
      final int dh = this.height << 1;
      final int sh = this.height;
      final int sw = this.width;
      int iOffs = st*(sh-1) + this.offset;
      int oOffs = dw*(dh-2);

      // Last 2 lines, only horizontal interpolation
      for (int i=sw-1; i>=0; i--)
      {
         int k = oOffs + (i<<1);
         final int valA = input[iOffs+i];
         final int valB = (i == sw-1) ? valA : input[iOffs+i+1];
         final int k2 = Global.abs(this.guide[k+1]-this.guide[k]);
         final int k1 = Global.abs(this.guide[k+2]-this.guide[k+1]);
         final int valAB = (k1+k2 == 0) ? valA : (k1*valA + k2*valB) / (k1+k2);
         output[k]    = valA;
         output[k+1]  = valAB;
         k += dw;
         output[k]    = valA;
         output[k+1]  = valAB;         
      }

      iOffs -= st;
      oOffs -= dw2;

      // Grid (lower case: interpolated pixels, upper case: copied pixels)
      //  A a B
      //  b c .
      //  C . D
      for (int j=sh-2; j>=0; j--)
      {
         // Last pixels of the line, only vertical interpolation
         int valB = input[iOffs+sw-1];
         int valD = input[iOffs+st+sw-1];
         int k = oOffs + dw;
         output[k-2] = valB;
         output[k-1] = valB;
         k += dw;
         output[k-2] = (valB + valD) >> 1;
         output[k-1] = (valB + valD) >> 1;

         for (int i=sw-2; i>=0; i--)
         {
            final int valA = input[iOffs+i];
            final int valC = input[iOffs+st+i];
            k = oOffs + (i<<1);
            output[k] = valA;
            int k1, k2, k3, k4;
            k2 = Global.abs(this.guide[k+1]-this.guide[k]);
            k1 = Global.abs(this.guide[k+2]-this.guide[k+1]);
            output[k+1] = (k1+k2 == 0) ? valA : (k1*valA + k2*valB) / (k1+k2); // a              
            k += dw;
            final int guide_A = this.guide[k-dw];
            final int guide_C = this.guide[k+dw];
            final int guide_b = this.guide[k];
            k2 = Global.abs(guide_A-guide_b);
            k1 = Global.abs(guide_C-guide_b);
            output[k] = (k1+k2 == 0) ? valA : (k1*valA + k2*valC) / (k1+k2); // b;              
            final int guide_c = this.guide[k+1];
            k2 = Global.abs(guide_A-guide_c);            // YA-Yc
            k1 = Global.abs(this.guide[k-dw+2]-guide_c); // YB-Yc
            k4 = Global.abs(guide_C-guide_c);            // YC-Yc        
            k3 = Global.abs(this.guide[k+dw+2]-guide_c); // YD-Yc
            
            // Horizontal and vertical mix
            // H: A-B and C-D (k1*valA + k2*valB + k3*valC + k4*valD)
            // V: A-C and B-D (k4*valA + k3*valB + k2*valC + k1*valD)
            output[k+1] = (k1+k2+k3+k4 == 0) ? valA : 
               ((k1+k4)*valA + (k2+k3)*valB + (k3+k2)*valC + (k4+k1)*valD) / ((k1+k2+k3+k4)<<1); // c
            valB = valA;
            valD = valC;
        }

        iOffs -= st;
        oOffs -= dw2;
      }
   }


   @Override
   public boolean supportsScalingFactor(int factor)
   {
      return (factor == 2);
   }
}