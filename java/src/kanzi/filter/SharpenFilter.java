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
      
        final int[] src = input.array;
        final int[] dst = output.array;
        final int nbChans = this.channels >> 8;
        final int maxShift = this.channels & 0xFF;
        final int h = this.height;
        final int w = this.width;
        final int st = this.stride;

        for (int i=0; i<nbChans; i++)
        {
            int srcStart = input.index;
            int dstStart = output.index;
            final int shift = maxShift - 8*i;
            
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
