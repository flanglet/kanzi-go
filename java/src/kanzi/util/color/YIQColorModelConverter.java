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


// YIQ color model: https://en.wikipedia.org/wiki/YIQ
// One pass converter using a fast bilinear resampler with in-place supersampling
// A custom resampler can also be provided
public final class YIQColorModelConverter implements ColorModelConverter
{
    private final int height;
    private final int width;
    private final int offset;
    private final int stride;
    private final DownSampler downSampler;
    private final UpSampler upSampler;


    public YIQColorModelConverter(int width, int height)
    {
        this(width, height, 0, width, null, null);
    }


    // rgbOffs is the offset in the RGB frame while stride is the width of the RGB frame
    // width and height are the dimension of the YUV frame
    public YIQColorModelConverter(int width, int height, int rgbOffset, int stride)
    {
        this(width, height, rgbOffset, stride, null, null);
    }


    // Down/up samplers can be provided in place of the default one (bilinear resampling)
    // If so, the size of the data arrays rgb[], y[], u[] and v[] may depend on
    // the custom resamplers (the supersampler may not support in-place supersampling).
    // Also, specialized down/up samplers requires more than one pass: sampling and
    // color calculation.
    public YIQColorModelConverter(int width, int height, DownSampler downSampler, UpSampler upSampler)
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
    public YIQColorModelConverter(int width, int height, int rgbOffset, int stride,
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


    // conversion matrix
    // 0.299  0.587  0.114
    // 0.596 -0.274 -0.322
    // 0.211 -0.523  0.312
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
                final int g = (rgbVal >> 8)  & 0xFF;
                final int b =  rgbVal & 0xFF;

                y[i] = (1225*r + 2404*g +  467*b + 2048) >> 12;
                u[i] = (2441*r - 1122*g - 1319*b + 2048) >> 12;
                v[i] = ( 864*r - 2142*g + 1278*b + 2048) >> 12;
                // ------- fromRGB 'Macro' END
            }

            startLine2 += this.width;
            startLine  += this.stride;
        }

        return true;
    }


    // conversion matrix
    // 1.000  0.956  0.621
    // 1.000 -0.272 -0.647
    // 1.000 -1.106  1.703
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
                final int yVal = y[i] << 12; 
                final int uVal = u[i]; 
                final int vVal = v[i];
                int r = yVal + 3916*uVal + 2544*vVal;
                int g = yVal - 1114*uVal - 2650*vVal;
                int b = yVal - 4530*uVal + 6976*vVal;
                
                if (r >= 1042432) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 2048) >> 12; r <<= 16; }
               
                if (g >= 1042432) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 2048) >> 12; g <<= 8; }

                if (b >= 1042432) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 2048) >> 12; }
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
            final int end = iOffs + sw;
            int uVal0, uVal1, uVal2, uVal3;
            int vVal0, vVal1, vVal2, vVal3;
            uVal0 = u[iOffs-sw];
            vVal0 = v[iOffs-sw];
            uVal2 = u[iOffs];
            vVal2 = v[iOffs];

            for (int i=iOffs+1; i<end; i++)
            {
                uVal1 = u[i-sw];
                vVal1 = v[i-sw];
                uVal3 = u[i];
                vVal3 = v[i];
                final int idx = offs - this.stride;

                // ------- toRGB 'Macro'
                yVal = y[idx] << 12; 
                uVal = uVal0; 
                vVal = vVal0;
                r = yVal + 3916*uVal + 2544*vVal;
                g = yVal - 1114*uVal - 2650*vVal;
                b = yVal - 4530*uVal + 6976*vVal;
                
                if (r >= 1042432) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 2048) >> 12; r <<= 16; }
               
                if (g >= 1042432) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 2048) >> 12; g <<= 8; }

                if (b >= 1042432) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 2048) >> 12; }                
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs] = r | g | b;

                int uu, vv;
                uu = (uVal0 + uVal1) >> 1;
                vv = (vVal0 + vVal1) >> 1;

                // ------- toRGB 'Macro'
                yVal = y[idx+1] << 12; 
                uVal = uu; 
                vVal = vv;
                r = yVal + 3916*uVal + 2544*vVal;
                g = yVal - 1114*uVal - 2650*vVal;
                b = yVal - 4530*uVal + 6976*vVal;
                
                if (r >= 1042432) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 2048) >> 12; r <<= 16; }
               
                if (g >= 1042432) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 2048) >> 12; g <<= 8; }

                if (b >= 1042432) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 2048) >> 12; }  
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs+1] = r | g | b;
                uu = (uVal0 + uVal2) >> 1;
                vv = (vVal0 + vVal2) >> 1;

                // ------- toRGB 'Macro'
                yVal = y[offs] << 12; 
                uVal = uu; 
                vVal = vv;
                r = yVal + 3916*uVal + 2544*vVal;
                g = yVal - 1114*uVal - 2650*vVal;
                b = yVal - 4530*uVal + 6976*vVal;
                
                if (r >= 1042432) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 2048) >> 12; r <<= 16; }
               
                if (g >= 1042432) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 2048) >> 12; g <<= 8; }

                if (b >= 1042432) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 2048) >> 12; }  
                // ------- toRGB 'Macro' END

                rgb[offs+rgbOffs] = r | g | b;
                uu = (uVal0 + uVal1 + uVal2 + uVal3 + 2) >> 2;
                vv = (vVal0 + vVal1 + vVal2 + vVal3 + 2) >> 2;

                // ------- toRGB 'Macro'
                yVal = y[offs+1] << 12; 
                uVal = uu; 
                vVal = vv;
                r = yVal + 3916*uVal + 2544*vVal;
                g = yVal - 1114*uVal - 2650*vVal;
                b = yVal - 4530*uVal + 6976*vVal;
                
                if (r >= 1042432) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 2048) >> 12; r <<= 16; }
               
                if (g >= 1042432) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 2048) >> 12; g <<= 8; }

                if (b >= 1042432) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 2048) >> 12; }  
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
            yVal = y[idx] << 12; 
            uVal = uVal0; 
            vVal = vVal0;
            r = yVal + 3916*uVal + 2544*vVal;
            g = yVal - 1114*uVal - 2650*vVal;
            b = yVal - 4530*uVal + 6976*vVal;

            if (r >= 1042432) r = 0x00FF0000;
            else { r &= ~(r >> 31); r = (r + 2048) >> 12; r <<= 16; }

            if (g >= 1042432) g = 0x0000FF00;
            else { g &= ~(g >> 31); g = (g + 2048) >> 12; g <<= 8; }

            if (b >= 1042432) b = 0x000000FF;
            else { b &= ~(b >> 31); b = (b + 2048) >> 12; }  
            // ------- toRGB 'Macro' END

            rgb[idx+rgbOffs]   = r | g | b;
            rgb[idx+rgbOffs+1] = r | g | b;

            final int uu = (uVal0 + uVal2) >> 1;
            final int vv = (vVal0 + vVal2) >> 1;

            // ------- toRGB 'Macro'
            yVal = y[offs] << 12; 
            uVal = uu; 
            vVal = vv;
            r = yVal + 3916*uVal + 2544*vVal;
            g = yVal - 1114*uVal - 2650*vVal;
            b = yVal - 4530*uVal + 6976*vVal;

            if (r >= 1042432) r = 0x00FF0000;
            else { r &= ~(r >> 31); r = (r + 2048) >> 12; r <<= 16; }

            if (g >= 1042432) g = 0x0000FF00;
            else { g &= ~(g >> 31); g = (g + 2048) >> 12; g <<= 8; }

            if (b >= 1042432) b = 0x000000FF;
            else { b &= ~(b >> 31); b = (b + 2048) >> 12; }  
            // ------- toRGB 'Macro' END

            rgb[offs+rgbOffs]   =  r | g | b;
            rgb[offs+rgbOffs+1] =  r | g | b;
            iOffs += sw;
            oOffs += stride2;
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
            int nextLine = startLine + this.stride;

            for (int i=0; i<this.width; )
            {
                int r, g, b;
                final int val0 = rgb[startLine+rgbOffs+i];
                r = (val0 >> 16) & 0xFF;
                g = (val0 >> 8)  & 0xFF;
                b =  val0 & 0xFF;
                final int yVal0 = 1225*r + 2404*g +  467*b;
                final int uVal0 = 2441*r - 1122*g - 1319*b;
                final int vVal0 =  864*r - 2142*g + 1278*b;                               
                y[startLine+i] = (yVal0 + 2048) >> 12;

                final int val1 = rgb[nextLine+rgbOffs+i];
                r = (val1 >> 16) & 0xFF;
                g = (val1 >> 8)  & 0xFF;
                b =  val1 & 0xFF;
                y[nextLine+i] = (1225*r + 2404*g +  467*b + 2048) >> 12;
                i++;

                final int val2 = rgb[startLine+rgbOffs+i];
                r = (val2 >> 16) & 0xFF;
                g = (val2 >> 8)  & 0xFF;
                b =  val2 & 0xFF;
                y[startLine+i] = (1225*r + 2404*g +  467*b + 2048) >> 12;

                final int val3 = rgb[nextLine+rgbOffs+i];
                r = (val3 >> 16) & 0xFF;
                g = (val3 >> 8)  & 0xFF;
                b =  val3 & 0xFF;                
                y[nextLine+i] = (1225*r + 2404*g +  467*b + 2048) >> 12;
                i++;

                // Decimate u, v (use position 0)
                u[offs] = (uVal0 + 2048) >> 12;
                v[offs] = (vVal0 + 2048) >> 12;
                offs++;
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
                yVal = y[k++] << 12; 
                uVal = u[iOffs+i]; 
                vVal = v[iOffs+i];
                r = yVal + 3916*uVal + 2544*vVal;
                g = yVal - 1114*uVal - 2650*vVal;
                b = yVal - 4530*uVal + 6976*vVal;
                
                if (r >= 1042432) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 2048) >> 12; r <<= 16; }
               
                if (g >= 1042432) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 2048) >> 12; g <<= 8; }

                if (b >= 1042432) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 2048) >> 12; }  
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs] = r | g | b;

                // ------- toRGB 'Macro'
                yVal = y[k++] << 12;
                r = yVal + 3916*uVal + 2544*vVal;
                g = yVal - 1114*uVal - 2650*vVal;
                b = yVal - 4530*uVal + 6976*vVal;
                
                if (r >= 1042432) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + 2048) >> 12; r <<= 16; }
               
                if (g >= 1042432) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + 2048) >> 12; g <<= 8; }

                if (b >= 1042432) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + 2048) >> 12; }  
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
        final int half = this.width >> 1;

        for (int j=0; j<this.height; j++)
        {
            final int end = iOffs + this.width;

            for (int k=iOffs, i=oOffs; k<end; i++)
            {
                int rgbVal, r, g, b;

                // ------- fromRGB 'Macro'
                rgbVal = rgb[k++];
                r = (rgbVal >> 16) & 0xFF;
                g = (rgbVal >> 8)  & 0xFF;
                b =  rgbVal & 0xFF;

                final int yVal0 = 1225*r + 2404*g +  467*b;
                final int uVal0 = 2441*r - 1122*g - 1319*b;
                final int vVal0 =  864*r - 2142*g + 1278*b; 

                rgbVal = rgb[k++];
                r = (rgbVal >> 16) & 0xFF;
                g = (rgbVal >> 8)  & 0xFF;
                b =  rgbVal & 0xFF;

                final int yVal1 = 1225*r + 2404*g +  467*b;

                // Decimate u, v
                u[i] = (uVal0 + 2048) >> 12;
                v[i] = (vVal0 + 2048) >> 12;
                // ------- fromRGB 'Macro' END

                y[i+i]   = (yVal0 + 2048) >> 12;
                y[i+i+1] = (yVal1 + 2048) >> 12;
            }

            oOffs += half;
            iOffs += this.stride;
        }

        return true;
    }    
    
    
    @Override
    public String toString() 
    {
       return "YIQ";
    }    
}
