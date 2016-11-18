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

package kanzi.util.color;

import kanzi.ColorModelType;

// One pass reversible converter
public final class ReversibleYUVColorModelConverter implements ColorModelConverter
{
    private final int height;
    private final int width;
    private final int rgbOffset;
    private final int stride;


    public ReversibleYUVColorModelConverter(int width, int height)
    {
        this(width, height, 0, width);
    }


    public ReversibleYUVColorModelConverter(int width, int height, int rgbOffset, int stride)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");

        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

        if ((height & 7) != 0)
            throw new IllegalArgumentException("The height must be a multiple of 8");

        if ((width & 7) != 0)
            throw new IllegalArgumentException("The width must be a multiple of 8");

        if ((stride & 7) != 0)
            throw new IllegalArgumentException("The stride must be a multiple of 8");

        this.height = height;
        this.width = width;
        this.rgbOffset = rgbOffset;
        this.stride = stride;
    }


    // Only YUV444 supported. Other types cannot be exactly reversed
    @Override
    public boolean convertRGBtoYUV(int[] rgb, int[] y, int[] u, int[] v, ColorModelType type)
    {
        if (type != ColorModelType.YUV444)
            return false;

        int startLine  = this.rgbOffset;
        int startLine2 = 0;

        for (int j=0; j<this.height; j++)
        {
            final int end = startLine + this.width;

            for (int k=startLine, i=startLine2; k<end; i++)
            {
                // ------- fromRGB 'Macro' (y, u, v sign must be preserved)
                final int rgbVal = rgb[k++];
                final int r = (rgbVal >> 16) & 0xFF;
                final int g = (rgbVal >> 8) & 0xFF;
                final int b =  rgbVal & 0xFF;
                
                y[i] = (r + g + g + b) >> 2;
                u[i] = r - g;
                v[i] = b - g;
                // ------- fromRGB 'Macro' (y, u, v sign must be preserved) END
            }

            startLine2 += this.width;
            startLine  += this.stride;
        }

        return true;
    }


    // Only YUV444 supported. Other types cannot be exactly reversed
    @Override
    public boolean convertYUVtoRGB(int[] y, int[] u, int[] v, int[] rgb, ColorModelType type)
    {
        if (type != ColorModelType.YUV444)
            return false;

        int startLine = 0;
        int startLine2 = this.rgbOffset;

        for (int j=0; j<this.height; j++)
        {
            final int end = startLine + this.width;

            for (int i=startLine, k=startLine2; i<end; i++)
            {
                // ------- toRGB 'Macro'
                final int g = y[i] - ((u[i] + v[i]) >> 2);
                final int r = u[i] + g;
                final int b = v[i] + g;
              
                rgb[k++] = (r << 16) | (g << 8) | b;
                // ------- toRGB 'Macro' END
            }

            startLine  += this.width;
            startLine2 += this.stride;
        }

        return true;
    }
   
    
    @Override
    public String toString() 
    {
       return "Reversible YUV";
    }    
}
