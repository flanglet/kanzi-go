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


public class BicubicUpSampler implements UpSampler
{
    private final int width;
    private final int height;
    private final int srcStride;
    private final int dstStride;
    private final int offset;
    private final boolean isRGB;


    public BicubicUpSampler(int width, int height)
    {
        this(width, height, width, 0);
    }

    
    public BicubicUpSampler(int width, int height, int stride, int offset)
    {
       this(width, height, stride, stride, offset);
    }
    
    
    public BicubicUpSampler(int width, int height, int srcStride, int dstStride, int offset)
    {
       this(width, height, srcStride, dstStride, offset, true);
    }
    
    
    public BicubicUpSampler(int width, int height, int srcStride, int dstStride, int offset, boolean isRGB)
    {
      if (height < 8)
         throw new IllegalArgumentException("The height must be at least 8");

      if (width < 8)
         throw new IllegalArgumentException("The width must be at least 8");

      if (offset < 0)
         throw new IllegalArgumentException("The offset must be at least 0");

      if (srcStride < width)
         throw new IllegalArgumentException("The stride must be at least as big as the width");
      
      if (dstStride < width)
         throw new IllegalArgumentException("The stride must be at least as big as the width");

      if ((height & 7) != 0)
         throw new IllegalArgumentException("The height must be a multiple of 8");

      if ((width & 7) != 0)
         throw new IllegalArgumentException("The width must be a multiple of 8");

      this.height = height;
      this.width = width;
      this.srcStride = srcStride;
      this.dstStride = dstStride;
      this.offset = offset;
      this.isRGB = isRGB;
    }
    
    
    @Override
    // Supports in place resampling
    public void superSampleVertical(int[] input, int[] output)
    {
       this.superSampleVertical(input, output, this.width, this.height, this.srcStride);
    }
    
    
    private void superSampleVertical(int[] input, int[] output, int sw, int sh, int st)
    {
      final int st2 = st + st;
      final int dw = sw;
      final int dh = sh * 2; 
      int iOffs = st * (sh - 1) + this.offset;
      int oOffs = dw * (dh - 1);

      // Rows h-1, h-2, h-3
      System.arraycopy(input, iOffs, output, oOffs, sw);
      oOffs -= dw;                    
      System.arraycopy(input, iOffs, output, oOffs, sw);
      oOffs -= dw;
      iOffs -= st;   
  
      for (int i=0; i<sw; i++)
         output[oOffs+i] = (input[iOffs+i] + input[iOffs+st+i]) >> 1;
      
      oOffs -= dw;                                              
      
      for (int j=sh-3; j>0; j--)
      {
         // Copy 
         System.arraycopy(input, iOffs, output, oOffs, sw);
         oOffs -= dw;    
         iOffs -= st;

         // Interpolate
         for (int i=0; i<sw; i++)
         {
            final int p0 = input[iOffs+i-st];
            final int p1 = input[iOffs+i];
            final int p2 = input[iOffs+i+st];
            final int p3 = input[iOffs+i+st2];

            //output[oOffs+i] = (int) (p1 + 0.5 * x*(p2 - p0 + x*(2.0*p0 - 5.0*p1 + 4.0*p2 - p3 + x*(3.0*(p1 - p2) + p3 - p0))));
            final int val = (p1<<4) + (p2<<2) - (p1<<3) - (p1<<1) + (p2<<3) - (p3<<1) + 
               ((p1<<1) + p1 - (p2<<1) - p2 + p3 - p0);
            output[oOffs+i] = this.getValue(val); 
         }

         oOffs -= dw;        
      }    
      
      // Rows 1, 2, 3
      for (int i=0; i<sw; i++)
         output[oOffs+i] = (input[iOffs+st+i] + input[iOffs+i]) >> 1;
      
      oOffs -= dw;        
      System.arraycopy(input, iOffs, output, oOffs, sw);            
      oOffs -= dw;        
      System.arraycopy(input, iOffs, output, oOffs, sw);               
    }


    @Override
    // Supports in place resampling
    public void superSampleHorizontal(int[] input, int[] output)
    {
       this.superSampleHorizontal(input, output, this.width, this.height, this.srcStride);
    }
    
   
    private void superSampleHorizontal(int[] input, int[] output, int sw, int sh, int st)
    {
       final int dw = sw * 2;
       final int dh = sh;
       int iOffs = st * (sh - 1) + this.offset;
       int oOffs = dw * (dh - 1);

       for (int j=sh-1; j>=0; j--)
       {
         // Columns w-1, w-2, w-3
         int val = input[iOffs+sw-1];
         output[oOffs+dw-1] = val;
         output[oOffs+dw-2] = val;             
         output[oOffs+dw-3] = (val+input[iOffs+sw-2]) >> 1;

         for (int i=sw-3; i>0; i--)
         {
            final int idx = oOffs + (i << 1);
            final int p0 = input[iOffs+i-1];
            final int p1 = input[iOffs+i];
            final int p2 = input[iOffs+i+1];
            final int p3 = input[iOffs+i+2];

            // Copy
            output[idx+2] = p2;
            
            // Interpolate
            //output[idx+1] = (int) (p1 + 0.5 * x*(p2 - p0 + x*(2.0*p0 - 5.0*p1 + 4.0*p2 - p3 + x*(3.0*(p1 - p2) + p3 - p0))));
            val = (p1<<4) + (p2<<2) - (p1<<3) - (p1<<1) + (p2<<3) - (p3<<1) + 
               ((p1<<1) + p1 - (p2<<1) - p2 + p3 - p0);
            output[idx+1] = this.getValue(val); 
         }

         // Columns 1, 2, 3
         val = input[iOffs];
         output[oOffs+2] = (val + input[iOffs+1]) >> 1;             
         output[oOffs+1] = val;             
         output[oOffs] = val;             
         iOffs -= st;
         oOffs -= dw;
       }
    }

    
    private int getValue(int val)
    {
       if (this.isRGB == false)
          return (val + 8 + ((val>>31)<<4)) >> 4;
       
       if (val >= 4072)
          return 255;
       
       val &= ~(val >> 31);
       return (val+8) >> 4;       
    }

    
    @Override
    public void superSample(int[] input, int[] output)
    {
       this.superSampleHorizontal(input, output, this.width, this.height, this.srcStride);
       this.superSampleVertical(output, output, this.width*2, this.height, this.dstStride);
    }


    @Override
    public boolean supportsScalingFactor(int factor)
    {
        return (factor == 2);
    }
}