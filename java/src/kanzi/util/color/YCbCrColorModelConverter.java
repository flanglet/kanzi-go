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
import kanzi.util.sampling.DownSampler;
import kanzi.util.sampling.UpSampler;

// Reference:  http://www.poynton.com/notes/colour_and_gamma/ColorFAQ.html#RTFToC29
// One pass converter using a fast bilinear resampler with in-place supersampling
// A custom resampler can also be provided
// Implementation follows JPEG/JFIF conversion (https://www.w3.org/Graphics/JPEG/jfif3.pdf)
public final class YCbCrColorModelConverter implements ColorModelConverter
{
    private final int height;
    private final int width;
    private final int offset;
    private final int stride;
    private final DownSampler downSampler;
    private final UpSampler upSampler;


    public YCbCrColorModelConverter(int width, int height)
    {
        this(width, height, 0, width, null, null);
    }


    // rgbOffs is the offset in the RGB frame while stride is the width of the RGB frame
    // width and height are the dimension of the YUV frame
    public YCbCrColorModelConverter(int width, int height, int rgbOffset, int stride)
    {
        this(width, height, rgbOffset, stride, null, null);
    }


    // Down/up samplers can be provided in place of the default one (bilinear resampling)
    // If so, the size of the data arrays rgb[], y[], u[] and v[] may depend on
    // the custom resamplers (the supersampler may not support in-place supersampling).
    // Also, specialized down/up samplers requires more than one pass: sampling and
    // color calculation.
    public YCbCrColorModelConverter(int width, int height, DownSampler downSampler, UpSampler upSampler)
    {
       this(width, height, 0, width, downSampler, upSampler);
    }


    // Down/up samplers can be provided in place of the default one (bilinear resampling)
    // If so, the size of the data arrays rgb[], y[], u[] and v[] may depend on
    // the custom resamplers (the supersampler may not support in-place supersampling).
    // Also, specialized down/up samplers requires more than one pass: sampling and
    // color calculation.
    // This constructor provides a way to work on a subset of the whole frame.
    // rgbOffs is the offset in the RGB frame while stride is the width of the RGB frame
    // width and height are the dimension of the YUV frame
    public YCbCrColorModelConverter(int width, int height, int rgbOffset, int stride,
                    DownSampler downSampler, UpSampler upSampler)
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

        if ((downSampler != null) && (downSampler.supportsScalingFactor(2) == false))
            throw new IllegalArgumentException("The provided down sampler does not support a scaling of 1/2");

        if ((upSampler != null) && (upSampler.supportsScalingFactor(2) == false))
            throw new IllegalArgumentException("The provided up sampler does not support a scaling of 2");

        this.height = height;
        this.width = width;
        this.offset = rgbOffset;
        this.stride = stride;
        this.upSampler = upSampler;
        this.downSampler = downSampler;
    }


    @Override
    public boolean convertRGBtoYUV(int[] rgb, int[] y, int[] u, int[] v, ColorModelType type)
    {
       if (type == ColorModelType.YUV444)
          return this.convertRGBtoYUV444(rgb, y, u, v);

       if (type == ColorModelType.YUV420)
          return this.convertRGBtoYUV420(rgb, y, u, v);

       if (type == ColorModelType.YUV422)
          return this.convertRGBtoYUV422(rgb, y, u, v);

       // Other types not supported
       return false;
    }


    @Override
    public boolean convertYUVtoRGB(int[] y, int[] u, int[] v, int[] rgb, ColorModelType type)
    {
       if (type == ColorModelType.YUV444)
          return this.convertYUV444toRGB(y, u, v, rgb);

       if (type == ColorModelType.YUV420)
          return this.convertYUV420toRGB(y, u, v, rgb);

       if (type == ColorModelType.YUV422)
          return this.convertYUV422toRGB(y, u, v, rgb);

       // Other types not supported
       return false;
    }


    // conversion matrix (JFIF)
    //  0.2990  0.5870  0.1140
    // -0.1687 -0.3313  0.5000
    //  0.5000 -0.4187 -0.0813
    private boolean convertRGBtoYUV444(int[] rgb, int[] y, int[] u, int[] v)
    {
        int startLine  = this.offset;
        int startLine2 = 0;

        for (int j=0; j<this.height; j++)
        {
            final int end = startLine + this.width;

            for (int k=startLine, i=startLine2; k<end; i++)
            {
                // ------- fromRGB 'Macro'
                final int rgbVal = rgb[k++];
                final int r = (rgbVal >> 16) & 0xFF;
                final int g = (rgbVal >> 8) & 0xFF;
                final int b =  rgbVal & 0xFF;
                final int yVal =  19595*r + 38470*g +  7471*b;
                final int uVal = -11056*r - 21712*g + 32768*b;
                final int vVal =  32768*r - 27440*g -  5328*b;

                y[i] = ((yVal + 32768) >> 16);
                u[i] = ((uVal + 32768) >> 16) + 128;
                v[i] = ((vVal + 32768) >> 16) + 128;
                // ------- fromRGB 'Macro'  END
            }

            startLine2 += this.width;
            startLine  += this.stride;
        }

        return true;
    }


    // conversion matrix (JFIF)
    // 1.00000  0.00000  1.40200    
    // 1.00000 -0.34414 -0.71414
    // 1.00000  1.77200  0.00000    
    private boolean convertYUV444toRGB(int[] y, int[] u, int[] v, int[] rgb)
    {
        int startLine = 0;
        int startLine2 = this.offset;

        for (int j=0; j<this.height; j++)
        {
            final int end = startLine + this.width;

            for (int i=startLine, k=startLine2; i<end; i++)
            {
                // ------- toRGB 'Macro'
                final int yVal = y[i] << 16; 
                final int uVal = u[i] - 128; 
                final int vVal = v[i] - 128;
                
                int r = yVal               +  91881*vVal;
                int g = yVal -  22554*uVal -  46802*vVal;
                int b = yVal + 116130*uVal;

                if (r >= 16678912) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 32768) >> 16; r <<= 16; }
               
                if (g >= 16678912) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 32768) >> 16; g <<= 8; }

                if (b >= 16678912) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 32768) >> 16; }
                // ------- toRGB 'Macro' END

                rgb[k++] = r | g | b;
            }

            startLine  += this.width;
            startLine2 += this.stride;
        }

        return true;
    }

    
    // In YUV420 format the U and V color components are subsampled 1:2 horizontally
    // and 1:2 vertically
    private boolean convertYUV420toRGB(int[] y, int[] u, int[] v, int[] rgb)
    {
        if (this.upSampler != null)
        {
           // Requires u & v of same size as y           
           this.upSampler.superSample(u, u);
           this.upSampler.superSample(v, v);
           return this.convertYUV444toRGB(y, u, v, rgb);
        }

        // In-place one-loop super sample and color conversion
        final int sw = this.width >> 1;
        final int sh = this.height >> 1;
        final int stride2 = this.stride << 1;
        final int rgbOffs = this.offset;
        int oOffs = this.stride;
        int iOffs = sw;
        int r, g, b;
        int yVal, uVal, vVal;

        for (int j=sh-1; j>=0; j--)
        {
            // The last iteration repeats the source line
            // EG: src lines 254 & 255 => dest lines 508 & 509
            //     src lines 255 & 255 => dest lines 510 & 511
            if (j == 0)
               iOffs -= sw;

            int offs = oOffs;
            int endi = iOffs + sw;
            int uVal0, uVal1, uVal2, uVal3;
            int vVal0, vVal1, vVal2, vVal3;
            uVal0 = u[iOffs-sw];
            vVal0 = v[iOffs-sw];
            uVal2 = u[iOffs];
            vVal2 = v[iOffs];

            for (int i=iOffs+1; i<endi; i++)
            {
                uVal1 = u[i-sw];
                vVal1 = v[i-sw];
                uVal3 = u[i];
                vVal3 = v[i];
                final int idx = offs - this.stride;

                // ------- toRGB 'Macro'
                yVal = y[idx] << 16; uVal = uVal0 - 128; vVal = vVal0 - 128;               
                r = yVal +  91881*vVal;
                g = yVal -  22554*uVal - 46802*vVal;
                b = yVal + 116130*uVal;

                if (r >= 16678912) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 32768) >> 16; r <<= 16; }
               
                if (g >= 16678912) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 32768) >> 16; g <<= 8; }

                if (b >= 16678912) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 32768) >> 16; }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs] = r | g | b;

                int uu, vv;
                uu = (uVal0 + uVal1) >> 1;
                vv = (vVal0 + vVal1) >> 1;

                // ------- toRGB 'Macro'
                yVal = y[idx+1] << 16; uVal = uu - 128; vVal = vv - 128;               
                r = yVal +  91881*vVal;
                g = yVal -  22554*uVal - 46802*vVal;
                b = yVal + 116130*uVal;

                if (r >= 16678912) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 32768) >> 16; r <<= 16; }
               
                if (g >= 16678912) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 32768) >> 16; g <<= 8; }

                if (b >= 16678912) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 32768) >> 16; }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs+1] = r | g | b;
                uu = (uVal0 + uVal2) >> 1;
                vv = (vVal0 + vVal2) >> 1;

                // ------- toRGB 'Macro'
                yVal = y[offs] << 16; uVal = uu - 128; vVal = vv - 128;               
                r = yVal +  91881*vVal;
                g = yVal -  22554*uVal - 46802*vVal;
                b = yVal + 116130*uVal;

                if (r >= 16678912) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 32768) >> 16; r <<= 16; }
               
                if (g >= 16678912) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 32768) >> 16; g <<= 8; }

                if (b >= 16678912) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 32768) >> 16; }
                // ------- toRGB 'Macro' END

                rgb[offs+rgbOffs] = r | g | b;
                uu = (uVal0 + uVal1 + uVal2 + uVal3 + 2) >> 2;
                vv = (vVal0 + vVal1 + vVal2 + vVal3 + 2) >> 2;

                // ------- toRGB 'Macro'
                yVal = y[offs+1] << 16; uVal = uu - 128; vVal = vv - 128;               
                r = yVal +  91881*vVal;
                g = yVal -  22554*uVal - 46802*vVal;
                b = yVal + 116130*uVal;

                if (r >= 16678912) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 32768) >> 16; r <<= 16; }
               
                if (g >= 16678912) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 32768) >> 16; g <<= 8; }

                if (b >= 16678912) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 32768) >> 16; }
                // ------- toRGB 'Macro' END

                rgb[offs+rgbOffs+1] = r | g | b;
                offs += 2;
                uVal0 = uVal1;
                vVal0 = vVal1;
                uVal2 = uVal3;
                vVal2 = vVal3;
            }

            final int idx = offs - this.stride;

            // ------- toRGB 'Macro'
            yVal = y[idx] << 16; uVal = uVal0 - 128; vVal = vVal0 - 128;         
            r = yVal +  91881*vVal;
            g = yVal -  22554*uVal - 46802*vVal;
            b = yVal + 116130*uVal;

            if (r >= 16678912) r = 0x00FF0000;
            else { r &= ~(r >> 31); r = (r + 32768) >> 16; r <<= 16; }

            if (g >= 16678912) g = 0x0000FF00;
            else { g &= ~(g >> 31); g = (g + 32768) >> 16; g <<= 8; }

            if (b >= 16678912) b = 0x000000FF;
            else { b &= ~(b >> 31); b = (b + 32768) >> 16; }
            // ------- toRGB 'Macro' END

            rgb[idx+rgbOffs]   = r | g | b;
            rgb[idx+rgbOffs+1] = r | g | b;

            final int uu = (uVal0 + uVal2) >> 1;
            final int vv = (vVal0 + vVal2) >> 1;

            // ------- toRGB 'Macro'
            yVal = y[offs] << 16; uVal = uu - 128; vVal = vv - 128;         
            r = yVal +  91881*vVal;
            g = yVal -  22554*uVal - 46802*vVal;
            b = yVal + 116130*uVal;

            if (r >= 16678912) r = 0x00FF0000;
            else { r &= ~(r >> 31); r = (r + 32768) >> 16; r <<= 16; }

            if (g >= 16678912) g = 0x0000FF00;
            else { g &= ~(g >> 31); g = (g + 32768) >> 16; g <<= 8; }

            if (b >= 16678912) b = 0x000000FF;
            else { b &= ~(b >> 31); b = (b + 32768) >> 16; }
            // ------- toRGB 'Macro' END

            rgb[offs+rgbOffs]   =  r | g | b;
            rgb[offs+rgbOffs+1] =  r | g | b;
            oOffs += stride2;
            iOffs += sw;
        }

        return true;
    }


    // In YUV420 format the U and V color components are subsampled 1:2 horizontally
    // and 1:2 vertically
    private boolean convertRGBtoYUV420(int[] rgb, int[] y, int[] u, int[] v)
    {
        if (this.downSampler != null)
        {
           // Requires u & v of same size as y
           boolean res = this.convertRGBtoYUV444(rgb, y, u, v);
           this.downSampler.subSample(u, u);
           this.downSampler.subSample(v, v);
           return res;
        }

        int startLine = 0;
        int offs = 0;
        final int rgbOffs = this.offset;

        for (int j=this.height-1; j>=0; j-=2)
        {
            final int nextLine = startLine + this.stride;

            for (int i=0; i<this.width; )
            {
                int r, g, b;
                final int val0 = rgb[startLine+rgbOffs+i];

                // ------- fromRGB 'Macro'
                r = (val0 >> 16) & 0xFF;
                g = (val0 >> 8)  & 0xFF;
                b =  val0 & 0xFF;
                final int yVal0 =  19595*r + 38470*g +  7471*b;
                final int uVal0 = -11056*r - 21712*g + 32768*b;
                final int vVal0 =  32768*r - 27440*g -  5328*b;
                y[startLine+i] = ((yVal0 + 32768) >> 16);

                final int val1 = rgb[nextLine+rgbOffs+i];
                r = (val1 >> 16) & 0xFF;
                g = (val1 >> 8)  & 0xFF;
                b =  val1 & 0xFF;
                final int yVal1 = 19595*r + 38470*g + 7471*b;
                y[nextLine+i] = ((yVal1 + 32768) >> 16);
                i++;

                final int val2 = rgb[startLine+rgbOffs+i];
                r = (val2 >> 16) & 0xFF;
                g = (val2 >> 8)  & 0xFF;
                b =  val2 & 0xFF;
                final int yVal2 = 19595*r + 38470*g + 7471*b;
                y[startLine+i] = ((yVal2 + 32768) >> 16);

                final int val3 = rgb[nextLine+rgbOffs+i];
                r = (val3 >> 16) & 0xFF;
                g = (val3 >> 8)  & 0xFF;
                b =  val3 & 0xFF;
                final int yVal3 = 19595*r + 38470*g + 7471*b;
                y[nextLine+i] = ((yVal3 + 32768) >> 16);
                i++;

                // Decimate u, v (use position 0)
                u[offs] = ((uVal0 + 32768) >> 16) + 128;
                v[offs] = ((vVal0 + 32768) >> 16) + 128;
                offs++;
                // ------- fromRGB 'Macro'  END
            }

            startLine = nextLine + this.stride;
        }

        return true;
    }


    // In YUV422 format the U and V color components are subsampled 1:2 horizontally
    private boolean convertYUV422toRGB(int[] y, int[] u, int[] v, int[] rgb)
    {
        if (this.upSampler != null)
        {
           // Requires u & v of same size as y
           this.upSampler.superSampleHorizontal(u, u);
           this.upSampler.superSampleHorizontal(v, v);
           return this.convertYUV444toRGB(y, u, v, rgb);
        }

        final int half = this.width >> 1;
        final int rgbOffs = this.offset;
        int oOffs = 0;
        int iOffs = 0;
        int k = 0;

        for (int j=0; j<this.height; j++)
        {
            for (int i=0; i<half; i++)
            {
                int r, g, b, yVal, uVal, vVal;
                final int idx = oOffs + i + i;

                // ------- toRGB 'Macro'
                yVal = y[k++] << 16; uVal = u[iOffs+i] - 128; vVal = v[iOffs+i] - 128;         
                r = yVal +  91881*vVal;
                g = yVal -  22554*uVal - 46802*vVal;
                b = yVal + 116130*uVal;

                if (r >= 16678912) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 32768) >> 16; r <<= 16; }

                if (g >= 16678912) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 32768) >> 16; g <<= 8; }

                if (b >= 16678912) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 32768) >> 16; }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs] = r | g | b;

                // ------- toRGB 'Macro'
                yVal = y[k++] << 16; 
                r = yVal +  91881*vVal;
                g = yVal -  22554*uVal - 46802*vVal;
                b = yVal + 116130*uVal;

                if (r >= 16678912) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 32768) >> 16; r <<= 16; }

                if (g >= 16678912) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 32768) >> 16; g <<= 8; }

                if (b >= 16678912) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 32768) >> 16; }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs+1] = r | g | b;
            }

            oOffs += this.stride;
            iOffs += half;
        }

        return true;
    }


    // In YUV422 format the U and V color components are subsampled 1:2 horizontally
    private boolean convertRGBtoYUV422(int[] rgb, int[] y, int[] u, int[] v)
    {
        if (this.downSampler != null)
        {
           // Requires u & v of same size as y
           boolean res = this.convertRGBtoYUV444(rgb, y, u, v);
           this.downSampler.subSampleHorizontal(u, u);
           this.downSampler.subSampleHorizontal(v, v);
           return res;
        }

        int iOffs = this.offset;
        int oOffs = 0;
        int half = this.width >> 1;

        for (int j=0; j<this.height; j++)
        {
            final int end = iOffs + this.width;

            for (int k=iOffs, i=oOffs; k<end; i++)
            {
                int rgbVal, r, g, b;

                // ------- fromRGB 'Macro'
                rgbVal = rgb[k++];
                r = (rgbVal >> 16) & 0xFF;
                g = (rgbVal >> 8) & 0xFF;
                b =  rgbVal & 0xFF;
                final int yVal1 =  19595*r + 38470*g +  7471*b;
                final int uVal1 = -11056*r - 21712*g + 32768*b;
                final int vVal1 =  32768*r - 27440*g -  5328*b;

                rgbVal = rgb[k++];
                r = (rgbVal >> 16) & 0xFF;
                g = (rgbVal >> 8) & 0xFF;
                b =  rgbVal & 0xFF;

                final int yVal2 = 19595*r + 38470*g +  7471*b;

                // Decimate u, v
                u[i] = ((uVal1 + 32768) >> 16) + 128;
                v[i] = ((vVal1 + 32768) >> 16) + 128;
                y[i+i]   = ((yVal1 + 32768) >> 16);
                y[i+i+1] = ((yVal2 + 32768) >> 16);
                // ------- fromRGB 'Macro'  END
            }

            oOffs += half;
            iOffs += this.stride;
        }

        return true;
    }
    
    
    
    @Override
    public String toString() 
    {
       return "YCbCr";
    }    
}
