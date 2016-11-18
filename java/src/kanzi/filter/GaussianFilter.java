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

package kanzi.filter;

import kanzi.SliceIntArray;
import kanzi.IntFilter;

// Fast implementation of a Gaussian filter approximation based on a recursive
// algorithm defined by Rachid Deriche.
// [See Deriche, R.: Recursively implementing the gaussian and its derivatives]
// http://hal.archives-ouvertes.fr/inria-00074778

public class GaussianFilter implements IntFilter
{
    private final int width;
    private final int height;
    private final int stride;
    private final int channels;
    private int[] buffer1;
    private int[] buffer2;
    private final int sigma16;
    private final float b1;
    private final float b2;
    private final float a0;
    private final float a1;
    private final float a2;
    private final float a3;
    private final float coefp;
    private final float coefn;      


    // sigma16 is the blurriness coefficient (multiplied by 16)
    public GaussianFilter(int width, int height, int sigma16)
    {
       this(width, height, width, sigma16, 3);
    }


    // sigma16 is the blurriness coefficient (multiplied by 16)
    public GaussianFilter(int width, int height, int stride, int sigma16, int channels)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");

        if ((sigma16 < 0) || (sigma16 > 255))
            throw new IllegalArgumentException("The sigma coefficient must be in [0..255]");

        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

        if ((channels < 1) || (channels > 3))
            throw new IllegalArgumentException("The number of image channels must be in [1..3]");

        this.height = height;
        this.width = width;
        this.stride = stride;
        this.sigma16 = sigma16;
        this.buffer1 = new int[0];
        this.buffer2 = new int[0];
        this.channels = channels;
        float nsigma = (this.sigma16 < 8) ? 0.5f : this.sigma16 /16.0f;
        float alpha = 1.695f / nsigma;
        float ema = (float) Math.exp(-alpha);
        float ema2 = (float) Math.exp(-2*alpha);
        this.b1 = -2*ema;
        this.b2 = ema2;
        float k = (1- ema)*(1-ema)/(1+2*alpha*ema-ema2);
        this.a0 =  k;
        this.a1 =  k*(alpha-1)*ema;
        this.a2 =  k*(alpha+1)*ema;
        this.a3 = -k*ema2;
        this.coefp = (this.a0+this.a1) / (1+this.b1+this.b2);
        this.coefn = (this.a2+this.a3) / (1+this.b1+this.b2);        
    }


    @Override
    public boolean apply(SliceIntArray input, SliceIntArray output)
    {
       if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
         return false;
      
       final int[] src = input.array;
       final int[] dst = output.array;
       final int srcIdx = input.index;
       final int dstIdx = output.index;
       final int maxIdx = Math.max(srcIdx, dstIdx);
       final int count = this.stride * this.height;
       
       if (this.sigma16 == 0)
       {
          if ((src != dst) || (srcIdx != dstIdx))
             System.arraycopy(src, srcIdx, dst, dstIdx, count+maxIdx);
          
          return true;
       }

       if (this.buffer1.length < count+maxIdx)
          this.buffer1 = new int[count+maxIdx];

       if (this.buffer2.length < count+maxIdx)
          this.buffer2 = new int[count+maxIdx];

       // Aliasing
       final int[] buf1 = this.buffer1;
       final int[] buf2 = this.buffer2;

       for (int channel=0; channel<this.channels; channel++)
       {
          final int shift = channel << 3;
          int offs = 0;
 
          // Extract channel
          for (int j=this.height-1; j>=0; j--)
          {
             final int end = offs + this.width;
             
             for (int i=offs; i<end; i++)
                buf1[i] = (src[srcIdx+i] >> shift) & 0xFF;
             
             offs += this.stride;
          }
          
          this.gaussianRecursiveX(buf1, buf2);
          this.gaussianRecursiveY(buf2, buf1);
          offs = 0;

          // Insert channel
          for (int j=this.height-1; j>=0; j--)
          {
             final int end = offs + this.width;
             
             for (int i=offs; i<end; i++)
             {
                dst[dstIdx+i] &= ~(0xFF << shift); //src and dst can share the same array
                dst[dstIdx+i] |= (buf1[i] & 0xFF) << shift;
             }
             
             offs += this.stride;
          }

       }

       return true;
    }


    private void gaussianRecursiveX(int[] input, int[] output)
    {
       final int w = this.width;
       final int h = this.height;
       int offs = 0;

       for (int y=0; y<h; y++)
       {
          // forward pass
          float xp = input[offs];
          float yb = this.coefp*xp;
          float yp = yb;

          for (int x=0; x<w; x++)
          {
             float xc = input[offs+x];
             float yc = this.a0*xc + this.a1*xp - this.b1*yp - this.b2*yb;            
             output[offs+x] = Math.round(yc);
             xp = xc;
             yb = yp;
             yp = yc;
          }

          // reverse pass: ensure response is symmetrical
          float xn = input[offs+w-1];
          float xa = xn;
          float yn = this.coefn*xn;
          float ya = yn;

          for (int x=w-1; x>=0; x--)
          {
             float xc = input[offs+x];
             float yc = this.a2*xn + this.a3*xa - this.b1*yn - this.b2*ya;
             output[offs+x] += Math.round(yc);
             xa = xn;
             xn = xc;
             ya = yn;
             yn = yc;
          }

          offs += this.stride;
       }
    }


    private void gaussianRecursiveY(int[] input, int[] output)
    {
       final int w = this.width;
       final int h = this.height;

       for (int x=0; x<w; x++)
       {
          // forward pass
          int offs = 0;
          float xp = input[x];
          float yb = this.coefp*xp;
          float yp = yb;

          for (int y=0; y<h; y++)
          {
            float xc = input[offs+x];
            float yc = this.a0*xc + this.a1*xp - this.b1*yp - this.b2*yb;
            output[offs+x] = Math.round(yc);
            xp = xc;
            yb = yp;
            yp = yc;
            offs += this.stride;
          }

          // reverse pass: ensure response is symmetrical
          offs = (h-1) * this.stride;
          float xn = input[offs+x];
          float xa = xn;
          float yn = this.coefn*xn;
          float ya = yn;

          for (int y=h-1; y>=0; y--)
          {
            float xc = input[offs+x];
            float yc = this.a2*xn + this.a3*xa - this.b1*yn - this.b2*ya;
            output[offs+x] += Math.round(yc);
            xa = xn;
            xn = xc;
            ya = yn;
            yn = yc;
            offs -= this.stride;
          }
       }
    }


    public int getSigma()
    {
       return this.sigma16;
    }
}
