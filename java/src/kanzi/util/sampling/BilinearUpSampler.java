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


public class BilinearUpSampler implements UpSampler
{
   private final int width;
   private final int height;
   private final int stride;
   private final int offset;
   private final int factor;


   public BilinearUpSampler(int width, int height)
   {
        this(width, height, width, 0, 2);
   }


   public BilinearUpSampler(int width, int height, int factor)
   {
       this(width, height, width, 0, factor);
   }
    
    
   public BilinearUpSampler(int width, int height, int stride, int offset, int factor)
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

      if ((factor != 2) && (factor != 4))
         throw new IllegalArgumentException("This implementation only supports "+
                    "a scaling factor equal to 2 or 4");

      this.height = height;
      this.width = width;
      this.stride = stride;
      this.offset = offset;
      this.factor = factor;
   }


    @Override
    // Supports in place resampling
    public void superSampleVertical(int[] input, int[] output)
    {
      final int sw = this.width;
      final int sh = this.height;
      final int st = this.stride;
      final int dw = sw;
      final int dh = sh * this.factor;
      final int dw2 = dw + dw;
      int iOffs = st * (sh - 1) + this.offset;
      int oOffs = dw * (dh - 1);

      if (this.factor == 2)
      {
         System.arraycopy(input, iOffs, output, oOffs, sw);
         oOffs -= dw;
         System.arraycopy(input, iOffs, output, oOffs, sw);           
         oOffs -= dw;
         iOffs -= st;

         for (int j=sh-2; j>0; j--)
         {
            // Interpolate odd lines
            for (int i=0; i<sw; i+=4)
            {
               final int idx1 = iOffs + i;
               final int idx2 = idx1 + st;
               output[oOffs+i]   = (input[idx1]   + input[idx2])   >> 1;
               output[oOffs+i+1] = (input[idx1+1] + input[idx2+1]) >> 1;
               output[oOffs+i+2] = (input[idx1+2] + input[idx2+2]) >> 1;
               output[oOffs+i+3] = (input[idx1+3] + input[idx2+3]) >> 1;
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
      else // factor = 4
      {
         System.arraycopy(input, iOffs, output, oOffs, sw);
         oOffs -= dw;
         System.arraycopy(input, iOffs, output, oOffs, sw);
         oOffs -= dw;
         System.arraycopy(input, iOffs, output, oOffs, sw);
         oOffs -= dw;
         System.arraycopy(input, iOffs, output, oOffs, sw);
         oOffs -= dw;
         iOffs -= st;

         for (int j=sh-2; j>0; j--)
         {
            // Interpolate
            for (int i=0; i<sw; i++)
            {
               final int val10 = input[iOffs+i];
               final int val20 = input[iOffs+i+st];
               output[oOffs+i] = (val10 + val20) >> 1;
               output[oOffs-dw+i] = (val10 + val10 + val10 + val20 + 2) >> 2;
               output[oOffs+dw+i] = (val10 + val20 + val20 + val20 + 2) >> 2;
            }

            // Copy
            oOffs -= dw2;
            System.arraycopy(input, iOffs, output, oOffs, sw);
              
            oOffs -= dw2;
            iOffs -= st;
         }
      }           
   }


   @Override
   // Supports in place resampling
   public void superSampleHorizontal(int[] input, int[] output)
   {
      final int sw = this.width;
      final int sh = this.height;
      final int st = this.stride;
      final int dw = sw * this.factor;
      final int dh = sh;
      int iOffs = st * (sh - 1) + this.offset;
      int oOffs = dw * (dh - 1);

      if (this.factor == 2)
      {
         for (int j=sh-1; j>=0; j--)
         {
            // Columns w-1, w-2
            int prv = input[iOffs+sw-1];
            output[oOffs+dw-1] = prv;
            output[oOffs+dw-2] = prv;

            for (int i=sw-2; i>=0; i--)
            {
               final int idx = oOffs + (i << 1);
               final int val = input[iOffs+i];

               // Copy even columns
               output[idx] = val;

               // Interpolate odd columns
               output[idx+1] = (val + prv) >> 1;
               prv = val;
            }
              
            iOffs -= st;
            oOffs -= dw;
         }
      }
      else // factor 4
      {
         for (int j=sh-1; j>=0; j--)
         {
            // Columns w-1, w-2, w-3, w-4
            int prv = input[iOffs+sw-1];
            output[oOffs+dw-1] = prv;
            output[oOffs+dw-2] = prv;
            output[oOffs+dw-3] = prv;
            output[oOffs+dw-4] = prv;
             
            for (int i=sw-2; i>=0; i--)
            {
               final int idx = oOffs + (i << 2);
               final int val = input[iOffs+i];

               // Copy (column is multiple of 4)
               output[idx] = val;

               // Interpolate
               output[idx+1] = (val + val + val + prv + 2) >> 2;
               output[idx+2] = (val + prv) >> 1;
               output[idx+3] = (val + prv + prv + prv + 2) >> 2;
               prv = val;
            }

            iOffs -= st;
            oOffs -= dw;
         }
      }
   }

   
   @Override
   // Supports in place resampling
   public void superSample(int[] input, int[] output)
   {
      final int st = this.stride;
      final int dw = this.width * this.factor;
      final int dw2 = dw + dw;
      final int dh = this.height * this.factor;
      final int sh = this.height;
      final int sw = this.width;
      int iOffs = st * (sh - 1) + this.offset;
      int oOffs = dw * (dh - this.factor);

      if (this.factor == 2)
      {
         // Last 2 lines, only horizontal interpolation
         for (int i=sw-1; i>=0; i--)
         {
            final int valA = input[iOffs+i];
            final int valB = (i == sw-1) ? valA : input[iOffs+i+1];
            final int valAB = (valA + valB) >> 1;
            int k = oOffs + (i << 1);
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
               k = oOffs + (i << 1);
               final int valA = input[iOffs+i];
               final int valC = input[iOffs+st+i];
               output[k]   = valA;
               output[k+1] = (valA + valB) >> 1; // a
               k += dw;
               output[k]   = (valA + valC) >> 1; // b
               output[k+1] = (valA + valB + valC + valD + 2) >> 2; // c
               valB = valA;
               valD = valC;
            }

            iOffs -= st;
            oOffs -= dw2;
         }
      }
      else // factor = 4
      {
         final int dw4 = dw2 + dw2;
          
         // Last 4 lines, only horizontal interpolation
         for (int i=sw-1; i>=0; i--)
         {
            final int valA = input[iOffs+i];
            final int valB = (i == sw-1) ? valA : input[iOffs+i+1];
            final int val3AB = (valA + valA + valA + valB + 2) >> 2;
            final int valA3B = (valA + valB + valB + valB + 2) >> 2;
            int k = oOffs + (i << 2);
            output[k]    = valA;
            output[k+1]  = val3AB;
            output[k+2]  = valA3B;
            output[k+3]  = valB;
            k += dw;
            output[k]    = valA;
            output[k+1]  = val3AB;
            output[k+2]  = valA3B;
            output[k+3]  = valB;
            k += dw;
            output[k]   = valA;
            output[k+1] = val3AB;
            output[k+2] = valA3B;
            output[k+3] = valB;
            k += dw;
            output[k]   = valA;
            output[k+1] = val3AB;
            output[k+2] = valA3B;
            output[k+3] = valB;
         }

         iOffs -= st;
         oOffs -= dw4;

         // Grid (lower case: interpolated pixels, upper case: copied pixels)
         //  A a b c B
         //  d e f g .
         //  h i j k .
         //  l m n o .
         //  C . . . D
         for (int j=sh-2; j>=0; j--)
         {
            // Last pixels of the line, only vertical interpolation
            int valB = input[iOffs+sw-1];
            int valD = input[iOffs+st+sw-1];
            int val3B = (valB << 1) + valB;
            int val3D = (valD << 1) + valD;
            int k = oOffs + dw;
            output[k-4] = valB;
            output[k-3] = valB;
            output[k-2] = valB;
            output[k-1] = valB;
            k += dw;
            output[k-4] = (val3B + valD + 2) >> 2;
            output[k-3] = (val3B + valD + 2) >> 2;
            output[k-2] = (val3B + valD + 2) >> 2;
            output[k-1] = (val3B + valD + 2) >> 2;
            k += dw;
            output[k-4] = (valB + val3D + 2) >> 2;
            output[k-3] = (valB + val3D + 2) >> 2;
            output[k-2] = (valB + val3D + 2) >> 2;
            output[k-1] = (valB + val3D + 2) >> 2;
            k += dw;
            output[k-4] = valD;
            output[k-3] = valD;
            output[k-2] = valD;
            output[k-1] = valD;

            for (int i=sw-2; i>=0; i--)
            {
               k = oOffs + (i << 2);
               final int valA = input[iOffs+i];
               final int valC = input[iOffs+st+i];
               final int val3A = (valA << 1) + valA;
               final int val3C = (valC << 1) + valC;
               output[k]   = valA;
               output[k+1] = (val3A + valB + 2) >> 2; // a
               output[k+2] = (valA + valB) >> 1; // b
               output[k+3] = (valA + val3B + 2) >> 2; // c
               k += dw;
               output[k]   = (val3A + valC + 2) >> 2; // d
               output[k+1] = (val3A + valB + valB + valC + valC + valD + 4) >> 3; // e
               output[k+2] = (val3A + val3B + valC + valD + 4) >> 3; //f
               output[k+3] = (valA + valA + val3B + valC + valD + valD + 4) >> 3; // g
               k += dw;
               output[k]   = (valA + valC) >> 1; // h
               output[k+1] = (val3A + valB + val3C + valD + 4) >> 3; // i
               output[k+2] = (valA + valB + valC + valD + 2) >> 2; // j
               output[k+3] = (valA + val3B + valC + val3D + 4) >> 3; //k
               k += dw;
               output[k]   = (valA + val3C + 2) >> 2; // l
               output[k+1] = (valA + valA + valB + val3C + valD + valD + 4) >> 3; // m
               output[k+2] = (valA + valB + val3C + val3D + 4) >> 3; // n
               output[k+3] = (valA + valB + valB + valC + valC + val3D + 4) >> 3; // o
               valB = valA;
               valD = valC;
               val3B = val3A;
               val3D = val3C;
            }

            iOffs -= st;
            oOffs -= dw4;
         }
      }
   }


   @Override
   public boolean supportsScalingFactor(int factor)
   {
     return ((factor == 2) || (factor == 4));
   }
}