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


public final class SobelFilter implements IntFilter
{
    public static final int HORIZONTAL = 1;
    public static final int VERTICAL = 2;

    // Type of Sobel filter
    // Can generate RGB/YCC image or array of costs (cost range = [0..255])
    public static final int IMAGE = 0xFFFFFFFF;
    public static final int COST = 0x0000FF;
    public static final int THREE_CHANNELS = 3;
    public static final int ONE_CHANNEL = 1;

    private final int width;
    private final int height;
    private final int stride;
    private final int direction;
    private final int mask;
    private final int channels;
    private final boolean processBoundaries;


    public SobelFilter(int width, int height)
    {
       this(width, height, width, VERTICAL | HORIZONTAL, THREE_CHANNELS, IMAGE, true);
    }


    public SobelFilter(int width, int height, int stride)
    {
       this(width, height, stride, VERTICAL | HORIZONTAL, THREE_CHANNELS, IMAGE, true);
    }


    public SobelFilter(int width, int height, int stride, boolean processBoundaries)
    {
       this(width, height, stride, VERTICAL | HORIZONTAL, THREE_CHANNELS, IMAGE, processBoundaries);
    }


    // If 'processBoundaries' is false, the first & last lines, first & last rows
    // are not processed. Otherwise, these boundaries get a copy of the nearest
    // pixels (since a 3x3 Sobel kernel cannot be applied).
    public SobelFilter(int width, int height, int stride,
            int direction, int nbChannels, int filterType, boolean processBoundaries)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");
        
        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

        if ((direction & (HORIZONTAL | VERTICAL)) == 0)
            throw new IllegalArgumentException("Invalid direction parameter (must be VERTICAL or HORIZONTAL or both)");

        if ((direction & ~(HORIZONTAL | VERTICAL)) != 0)
            throw new IllegalArgumentException("Invalid direction parameter (must be VERTICAL or HORIZONTAL or both)");

        if ((filterType != COST) && (filterType != IMAGE))
            throw new IllegalArgumentException("Invalid filter type parameter (must be IMAGE or COST)");

        if ((nbChannels != THREE_CHANNELS) && (nbChannels != ONE_CHANNEL))
            throw new IllegalArgumentException("Invalid image type parameter (must be ONE_CHANNEL or THREE_CHANNELS)");

        this.height = height;
        this.width = width;
        this.stride = stride;
        this.direction = direction;
        this.mask = filterType;
        this.channels = nbChannels;
        this.processBoundaries = processBoundaries;
    }


    // Return a picture or a map of costs if costMult64 is not null
    //   Horizontal                               Vertical
    //   -1  0   1        pix00 pix01 pix02        1  2  1
    //   -2  0   2  <-->  pix10 pix11 pix12  <-->  0  0  0
    //   -1  0   1        pix20 pix21 pix22       -1 -2 -1
    // Implementation focused on speed through reduction of array access
    // This implementation requires around 4*w*h accesses
    @Override
    public boolean apply(IndexedIntArray source, IndexedIntArray destination)
    {
        final int[] src = source.array;
        final int[] dst = destination.array;
        int srcStart = source.index;
        int dstStart = destination.index;
        final int mask_ = this.mask;
        final int h = this.height;
        final int w = this.width;
        final boolean isVertical = ((this.direction & VERTICAL) != 0) ? true : false;
        final boolean isHorizontal = ((this.direction & HORIZONTAL) != 0) ? true : false;
        final int maxVal = 0x00FFFFFF & mask_;
        boolean isPacked = (this.channels == 3) ? true : false;
        final int st = this.stride;

        for (int y=h-2; y>0; y--)
        {
           final int srcLine = srcStart + st;
           final int endLine = srcLine + st;
           final int dstLine = dstStart + st;
           final int pixel00 = src[srcStart];
           final int pixel01 = src[srcStart+1];
           final int pixel10 = src[srcLine];
           final int pixel11 = src[srcLine+1];
           final int pixel20 = src[endLine];
           final int pixel21 = src[endLine+1];
           int val00, val01, val10, val11, val20, val21;

           if (isPacked == true)
           {
              // Use Yreversible = (R + G + G + B) >> 2;
              val00 = (((pixel00 >> 16) & 0xFF) + ((pixel00 >> 7) & 0x1FE) + (pixel00 & 0xFF)) >> 2;
              val01 = (((pixel01 >> 16) & 0xFF) + ((pixel01 >> 7) & 0x1FE) + (pixel01 & 0xFF)) >> 2;
              val10 = (((pixel10 >> 16) & 0xFF) + ((pixel10 >> 7) & 0x1FE) + (pixel10 & 0xFF)) >> 2;
              val11 = (((pixel11 >> 16) & 0xFF) + ((pixel11 >> 7) & 0x1FE) + (pixel11 & 0xFF)) >> 2;
              val20 = (((pixel20 >> 16) & 0xFF) + ((pixel20 >> 7) & 0x1FE) + (pixel20 & 0xFF)) >> 2;
              val21 = (((pixel21 >> 16) & 0xFF) + ((pixel21 >> 7) & 0x1FE) + (pixel21 & 0xFF)) >> 2;
           }
           else
           {
              val00 = pixel00 & 0xFF;
              val01 = pixel01 & 0xFF;
              val10 = pixel10 & 0xFF;
              val11 = pixel11 & 0xFF;
              val20 = pixel20 & 0xFF;
              val21 = pixel21 & 0xFF;
           }

           for (int x=2; x<w; x++)
           {
             final int pixel02 = src[srcStart+x];
             final int pixel12 = src[srcLine+x];
             final int pixel22 = src[endLine+x];
             final int val02, val12, val22;
             int val;

             if (isPacked == true)
             {                
                // Use Yreversible = (R + G + G + B) >> 2;
                val02 = (((pixel02 >> 16) & 0xFF) + ((pixel02 >> 7) & 0x1FE) + (pixel02 & 0xFF)) >> 2;
                val12 = (((pixel12 >> 16) & 0xFF) + ((pixel12 >> 7) & 0x1FE) + (pixel12 & 0xFF)) >> 2;
                val22 = (((pixel22 >> 16) & 0xFF) + ((pixel22 >> 7) & 0x1FE) + (pixel22 & 0xFF)) >> 2;
             }
             else
             {
                val02 = pixel02 & 0xFF;
                val12 = pixel12 & 0xFF;
                val22 = pixel22 & 0xFF;
             }
             
             if (isHorizontal == true)
             {
                val = -val00 + val02 - val10 - val10 + val12 + val12 - val20 + val22;
                val = (val + (val >> 31)) ^ (val >> 31);

                if (isVertical == true)
                {
                   int valV = val00 + val01 + val01 + val02 - val20 - val21 - val21 - val22;
                   valV = (valV + (valV >> 31)) ^ (valV >> 31);
                   val = (val + valV) >> 1;
                }
             }
             else // if Horizontal==false, then Vertical==true by construct
             {
                val = val00 + val01 + val01 + val02 - val20 - val21 - val21 - val22;
                val = (val + (val >> 31)) ^ (val >> 31);
             }

             dst[dstLine+x-1] = (val > 255) ? maxVal : ((val << 16) | (val << 8) | val) & mask_;

             // Slide the 3x3 window (reassign 6 pixels: left + center columns)
             val00 = val01;
             val01 = val02;
             val10 = val11;
             val11 = val12;
             val20 = val21;
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

       final int firstLine = destination.index;
       final int lastLine = destination.index + st * (h - 1);

       if (this.processBoundaries == true)
       {
          // Duplicate first and last lines
          System.arraycopy(dst, firstLine+st, dst, firstLine, w);
          System.arraycopy(dst, lastLine-st, dst, lastLine, w);
       }

       return true;
    }
}
