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

package kanzi.filter;

import kanzi.IndexedIntArray;
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
    private int sigma16;


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
        this.buffer1 = new int[width*height];
        this.buffer2 = new int[width*height];
        this.channels = channels;
    }


    @Override
    public boolean apply(IndexedIntArray source, IndexedIntArray destination)
    {
       final int[] src = source.array;
       final int[] dst = destination.array;
       final int srcIdx = source.index;
       final int dstIdx = destination.index;
       
       if (this.sigma16 == 0)
       {
          System.arraycopy(src, srcIdx, dst, dstIdx, this.width*this.height);
          return true;
       }

       final float sigma = (float) this.sigma16 / 16.0f;
       final float nsigma = (sigma < 0.5f) ? 0.5f : sigma;
       final float alpha = 1.695f / nsigma;
       final float ema = (float) Math.exp(-alpha);
       final float ema2 = (float) Math.exp(-2*alpha);
       final float b1 = -2*ema;
       final float b2 = ema2;
       final float k = (1-ema)*(1-ema)/(1+2*alpha*ema-ema2);
       final float a0 = k;
       final float a1 = k*(alpha-1)*ema;
       final float a2 = k*(alpha+1)*ema;
       final float a3 = -k*ema2;
       final float coefp = (a0+a1) / (1+b1+b2);
       final float coefn = (a2+a3) / (1+b1+b2);

       // Aliasing
       final int[] buf1 = this.buffer1;
       final int[] buf2 = this.buffer2;
       final int w = this.width;
       final int h = this.height;
       final int st = this.stride;
       final int len = src.length;

       for (int channel=0; channel<this.channels; channel++)
       {
          final int shift = channel << 3;
          int startLine = srcIdx;
          int idx = 0;

          // Extract channel
          for (int y=0; y<h; y++)
          {
             final int endLine = startLine + w;
             final int endX = (startLine >= len) ? startLine : ((endLine < len) ? endLine : len);

             for (int x=startLine; x<endX; x++)
                buf1[idx++] = (src[x] >> shift) & 0xFF;

             startLine += st;
          }

          this.gaussianRecursiveX(buf1, buf2, a0, a1, a2, a3, b1, b2, coefp, coefn);
          this.gaussianRecursiveY(buf2, buf1, a0, a1, a2, a3, b1, b2, coefp, coefn);

          startLine = dstIdx;
          idx = 0;

          // Insert channel
          for (int y=0; y<h; y++)
          {
             final int endX = startLine + w;

             for (int x=startLine; x<endX; x++)
             {
                dst[x] &= ~(0xFF << shift); //src and dst can share the same array
                dst[x] |= (buf1[idx++] & 0xFF) << shift;
             }

             startLine += st;
          }
       }

       return true;
    }


    private void gaussianRecursiveX(int[] input, int[] output, float a0, float a1,
            float a2, float a3, float b1, float b2, float coefp, float coefn)
    {
       final int w = this.width;
       final int h = this.height;
       int offs = 0;

       for (int y=0; y<h; y++)
       {
          // forward pass
          float xp = input[offs];
          float yb = coefp*xp;
          float yp = yb;

          for (int x=0; x<w; x++)
          {
             float xc = input[offs+x];
             float yc = a0*xc + a1*xp - b1*yp - b2*yb;
             output[offs+x] = (int) (yc + .5);
             xp = xc;
             yb = yp;
             yp = yc;
          }

          // reverse pass: ensure response is symmetrical
          float xn = input[offs+w-1];
          float xa = xn;
          float yn = coefn*xn;
          float ya = yn;

          for (int x=w-1; x>=0; x--)
          {
             float xc = input[offs+x];
             float yc = a2*xn + a3*xa - b1*yn - b2*ya;
             output[offs+x] += yc;
             xa = xn;
             xn = xc;
             ya = yn;
             yn = yc;
          }

          offs += w;
       }
    }


    private void gaussianRecursiveY(int[] input, int[] output, float a0, float a1,
            float a2, float a3, float b1, float b2, float coefp, float coefn)
    {
       final int w = this.width;
       final int h = this.height;

       for (int x=0; x<w; x++)
       {
          // forward pass
          int offs = 0;
          float xp = input[x];
          float yb = coefp*xp;
          float yp = yb;

          for (int y=0; y<h; y++)
          {
            float xc = input[offs+x];
            float yc = a0*xc + a1*xp - b1*yp - b2*yb;
            output[offs+x] = (int) (yc + .5);
            xp = xc;
                 yb = yp;
                 yp = yc;
                 offs += w;
          }

          // reverse pass: ensure response is symmetrical
          offs = (h-1) * w;
          float xn = input[offs+x];
          float xa = xn;
          float yn = coefn*xn;
          float ya = yn;

          for (int y=h-1; y>=0; y--)
          {
            float xc = input[offs+x];
            float yc = a2*xn + a3*xa - b1*yn - b2*ya;
            output[offs+x] += yc;
            xa = xn;
                 xn = xc;
                 ya = yn;
                 yn = yc;
                 offs -= w;
          }
       }
    }


    public int getSigma()
    {
       return this.sigma16;
    }


    // Not thread safe
    public boolean setSigma(int sigma16)
    {
       if ((sigma16 < 0) || (sigma16 > 255))
          return false;

       this.sigma16 = sigma16;
       return true;
    }
}
