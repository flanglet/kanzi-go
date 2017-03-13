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


public final class SharpenFilter implements IntFilter
{
    // Type of input filter: use RGB for 3 channels, or 1 channel (use B for Y, or U or V)
    public static final int THREE_CHANNELS = 0x0310; // 3 channels with shifts 16,8,0
    public static final int R_CHANNEL = 0x0110;
    public static final int G_CHANNEL = 0x0108;
    public static final int B_CHANNEL = 0x0100;
    
    private final int width;
    private final int height;
    private final int stride;
    private final int channels;
    private final boolean processBoundaries;


    public SharpenFilter(int width, int height)
    {
       this(width, height, width, THREE_CHANNELS, true);
    }


    public SharpenFilter(int width, int height, int stride)
    {
       this(width, height, stride, THREE_CHANNELS, true);
    }


    public SharpenFilter(int width, int height, int stride, boolean processBoundaries)
    {
       this(width, height, stride, THREE_CHANNELS, processBoundaries);
    }


    // If 'processBoundaries' is false, the first & last lines, first & last rows
    // are not processed. Otherwise, these boundaries get a copy of the nearest
    // pixels (since a 3x3 Sharpen kernel cannot be applied).
    public SharpenFilter(int width, int height, int stride, int channels, boolean processBoundaries)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");
        
        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

        if ((channels != THREE_CHANNELS) && (channels != R_CHANNEL) && 
                (channels != G_CHANNEL) && (channels != B_CHANNEL))
            throw new IllegalArgumentException("Invalid input channel parameter (must be RGB or R or G or B)");

        this.height = height;
        this.width = width;
        this.stride = stride;
        this.channels = channels;
        this.processBoundaries = processBoundaries;
    }

    
    //   Filter
    //    0 -1  0        pix00 pix01 pix02 
    //   -1  5 -1  <-->  pix10 pix11 pix12 
    //    0 -1  0        pix20 pix21 pix22
    // Implementation focused on speed through reduction of array access
    // This implementation requires around 4*w*h accesses
    @Override
    public boolean apply(SliceIntArray input, SliceIntArray output)
    {
       if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
          return false;
      
       if (this.channels == THREE_CHANNELS)
          return this.applyThreeChannels(input, output);
       
       return this.applyOneChannel(input, output);
    }
    
    
    private boolean applyOneChannel(SliceIntArray input, SliceIntArray output)
    {
      final int[] src = input.array;
      final int[] dst = output.array;
      final int h = this.height;
      final int w = this.width;
      final int st = this.stride;
      int srcStart = input.index;
      int dstStart = output.index;
      final int shift = this.channels & 0xFF;

      for (int y=h-2; y>0; y--)
      {
         final int srcLine = srcStart + st;
         final int endLine = srcLine + st;
         final int dstLine = dstStart + st;
         final int pixel01 = src[srcStart+1];
         final int pixel10 = src[srcLine];
         final int pixel11 = src[srcLine+1];
         final int pixel21 = src[endLine+1];
         int val01 = (pixel01 >> shift) & 0xFF;
         int val10 = (pixel10 >> shift) & 0xFF;
         int val11 = (pixel11 >> shift) & 0xFF;
         int val21 = (pixel21 >> shift) & 0xFF;

         for (int x=2; x<w; x++)
         {
           final int pixel02 = src[srcStart+x];
           final int pixel12 = src[srcLine+x];
           final int pixel22 = src[endLine+x];
           int val02 = (pixel02 >> shift) & 0xFF;
           int val12 = (pixel12 >> shift) & 0xFF;
           int val22 = (pixel22 >> shift) & 0xFF;
           int val = - val01 - val10 + 5*val11 - val21 - val12; 
           val &= ~(val >> 31); 
           dst[dstLine+x-1] &= ~(255 << shift);
           dst[dstLine+x-1] |= (val >= 255) ? 255<<shift : val<<shift;

           // Slide the 3x3 window (reassign 6 pixels: left + center columns)
           val01 = val02;
           val10 = val11;
           val11 = val12;
           val21 = val22;
        }

        if (this.processBoundaries == true)
        {
           // Boundary processing (first and last row pixels), just duplicate pixels
           dst[dstLine] = dst[dstLine+1];
           dst[dstLine+w-1] = dst[dstLine+w-2];
        }

        srcStart = srcLine;
        dstStart = dstLine;
     }

       final int firstLine = output.index;
       final int lastLine = output.index + st * (h - 1);

       if (this.processBoundaries == true)
       {
          // Duplicate first and last lines
          System.arraycopy(dst, firstLine+st, dst, firstLine, w);
          System.arraycopy(dst, lastLine-st, dst, lastLine, w);
       }

       return true;
    }    


    private boolean applyThreeChannels(SliceIntArray input, SliceIntArray output)
    {
      final int[] src = input.array;
      final int[] dst = output.array;
      final int h = this.height;
      final int w = this.width;
      final int st = this.stride;
      int srcStart = input.index;
      int dstStart = output.index;

      for (int y=h-2; y>0; y--)
      {
         final int srcLine = srcStart + st;
         final int endLine = srcLine + st;
         final int dstLine = dstStart + st;
         final int pixel01 = src[srcStart+1];
         final int pixel10 = src[srcLine];
         final int pixel11 = src[srcLine+1];
         final int pixel21 = src[endLine+1];
         int r01 = (pixel01>> 16) & 0xFF;
         int g01 = (pixel01>>  8) & 0xFF;
         int b01 =  pixel01       & 0xFF;
         int r10 = (pixel10>> 16) & 0xFF;
         int g10 = (pixel10>> 8)  & 0xFF;
         int b10 =  pixel10       & 0xFF;
         int r11 = (pixel11>> 16) & 0xFF;
         int g11 = (pixel11>>  8) & 0xFF;
         int b11 =  pixel11       & 0xFF;
         int r21 = (pixel21>> 16) & 0xFF;
         int g21 = (pixel21>>  8) & 0xFF;
         int b21 =  pixel21       & 0xFF;

         for (int x=2; x<w; x++)
         {
           final int pixel02 = src[srcStart+x];
           final int pixel12 = src[srcLine+x];
           final int pixel22 = src[endLine+x];
           int r02 = (pixel02 >> 16) & 0xFF;
           int g02 = (pixel02 >>  8) & 0xFF;
           int b02 =  pixel02        & 0xFF;
           int r12 = (pixel12 >> 16) & 0xFF;
           int g12 = (pixel12 >>  8) & 0xFF;
           int b12 =  pixel12        & 0xFF;
           int r22 = (pixel22 >> 16) & 0xFF;
           int g22 = (pixel22 >>  8) & 0xFF;
           int b22 =  pixel22        & 0xFF;
           int r = - r01 - r10 + 5*r11 - r21 - r12; 
           
           if (r >= 255) r = 255; 
           else r &= ~(r >> 31); 
           
           int g = - g01 - g10 + 5*g11 - g21 - g12; 
           
           if (g >= 255) g = 255;
           else g &= ~(g >> 31); 
                      
           int b = - b01 - b10 + 5*b11 - b21 - b12; 
           
           if (b >= 255) b = 255;
           else b &= ~(b >> 31); 
           
           dst[dstLine+x-1] = (r<<16) | (g<<8) | b;

           // Slide the 3x3 window (reassign 6 pixels: left + center columns)
           r01 = r02;
           g01 = g02;
           b01 = b02;
           r10 = r11;
           g10 = g11;
           b10 = b11;
           r11 = r12;
           g11 = g12;
           b11 = b12;
           r21 = r22;
           g21 = g22;
           b21 = b22;
        }

        if (this.processBoundaries == true)
        {
           // Boundary processing (first and last row pixels), just duplicate pixels
           dst[dstLine] = dst[dstLine+1];
           dst[dstLine+w-1] = dst[dstLine+w-2];
        }

        srcStart = srcLine;
        dstStart = dstLine;
     }

       final int firstLine = output.index;
       final int lastLine = output.index + st * (h - 1);

       if (this.processBoundaries == true)
       {
          // Duplicate first and last lines
          System.arraycopy(dst, firstLine+st, dst, firstLine, w);
          System.arraycopy(dst, lastLine-st, dst, lastLine, w);
       }

       return true;       
    }    
}
