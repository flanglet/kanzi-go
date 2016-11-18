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


// Reference:  [A NEW COLOR TRANSFORM FOR RGB CODING]
//             by Hyun Mun Kim, Woo-Shik Kim, and Dae-Sung Cho
// One pass converter using a fast bilinear resampler with in-place supersampling
// A custom resampler can also be provided
public final class YSbSrColorModelConverter implements ColorModelConverter
{
    private final int height;
    private final int width;
    private final int offset;
    private final int stride;
    private final int fShift; // forward (RGB to YUV)
    private final int iShift; // inverse (YUV to RGB)    
    private final DownSampler downSampler;
    private final UpSampler upSampler;


    public YSbSrColorModelConverter(int width, int height)
    {
        this(width, height, 0, width, null, null, true);
    }

    
    // By default the transform expand YUV range by 2.
    // If keepRange is set to true, keep YUV in [0..255] range (which increases
    // roundtrip error).
    public YSbSrColorModelConverter(int width, int height, boolean keepRange)
    {
        this(width, height, 0, width, null, null, keepRange);
    }


    // rgbOffs is the offset in the RGB frame while stride is the width of the RGB frame
    // width and height are the dimension of the YUV frame
    public YSbSrColorModelConverter(int width, int height, int rgbOffset, int stride, boolean keepRange)
    {
        this(width, height, rgbOffset, stride, null, null, keepRange);
    }


    // Down/up samplers can be provided in place of the default one (bilinear resampling)
    // If so, the size of the data arrays rgb[], y[], u[] and v[] may depend on
    // the custom resamplers (the supersampler may not support in-place supersampling).
    // Also, specialized down/up samplers requires more than one pass: sampling and
    // color calculation.
    public YSbSrColorModelConverter(int width, int height, 
       DownSampler downSampler, UpSampler upSampler, boolean keepRange)
    {
       this(width, height, 0, width, downSampler, upSampler, keepRange);
    }


    // Down/up samplers can be provided in place of the default one (bilinear resampling)
    // If so, the size of the data arrays rgb[], y[], u[] and v[] may depend on
    // the custom resamplers (the supersampler may not support in-place supersampling).
    // Also, specialized down/up samplers requires more than one pass: sampling and
    // color calculation.
    // This constructor provides a way to work on a subset of the whole frame.
    // rgbOffs is the offset in the RGB frame while stride is the width of the RGB frame
    // width and height are the dimension of the YUV frame
    public YSbSrColorModelConverter(int width, int height, int rgbOffset, int stride,
                   DownSampler downSampler, UpSampler upSampler, boolean keepRange)
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
        this.fShift = keepRange ? 13 : 12;
        this.iShift = keepRange ? 11 : 12;
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


    // conversion matrix (increase range by 2)
    // if keepRange is true, the matrix coefficients are divided by 2
    //  0.6460 0.6880  0.6660
    // -1.0000 0.2120  0.7880
    // -0.3220 1.0000 -0.6780
    private boolean convertRGBtoYUV444(int[] rgb, int[] y, int[] u, int[] v)
    {
        int startLine  = this.offset;
        int startLine2 = 0;
        final int rshift = this.fShift;
        final int adjust = (1 << rshift) >> 1;
        
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

                y[i] = ( 2646*r  + 2818*g  + 2728*b + adjust) >> rshift;
                u[i] = (-(r<<12) +  868*g  + 3228*b + adjust) >> rshift;
                v[i] = (-1319*r  + (g<<12) - 2777*b + adjust) >> rshift;
                // ------- fromRGB 'Macro' END
            }

            startLine2 += this.width;
            startLine  += this.stride;
        }

        return true;
    }


    // conversion matrix
    // if keepRange is true, the matrix coefficients are multiplied by 2
    // 0.5000 -0.6077 -0.2152
    // 0.5000  0.1200  0.6306
    // 0.5000  0.4655 -0.4427
    private boolean convertYUV444toRGB(int[] y, int[] u, int[] v, int[] rgb)
    {
        int startLine = 0;
        int startLine2 = this.offset;
        final int rshift = this.iShift;
        final int adjust = (1 << rshift) >> 1;
        final int maxVal = (255 << rshift) - adjust;
        
        for (int j=0; j<this.height; j++)
        {
            final int end = startLine + this.width;

            for (int i=startLine, k=startLine2; i<end; i++)
            {
                // ------- toRGB 'Macro'
                final int yVal = y[i] << 11; 
                final int uVal = u[i]; 
                final int vVal = v[i];
                int r = yVal - 2489*uVal -  881*vVal;
                int g = yVal +  491*uVal + 2583*vVal;
                int b = yVal + 1907*uVal - 1813*vVal;
                
                if (r >= maxVal) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + adjust) >> rshift; r <<= 16; }
               
                if (g >= maxVal) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + adjust) >> rshift; g <<= 8; }

                if (b >= maxVal) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + adjust) >> rshift; }
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
        final int rshift = this.iShift;
        final int adjust = (1 << rshift) >> 1;
        final int maxVal = (255 << rshift) - adjust;
        int oOffs = this.stride;
        int iOffs = sw;
        int r, g, b;
        int yVal, uVal, vVal;

        for (int j=sh-1; j>=0; j--)
        {
            // The last iteration (j==sh-1) must repeat the source line
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
                yVal = y[idx] << 11; 
                uVal = uVal0; 
                vVal = vVal0;
                r = yVal - 2489*uVal -  881*vVal;
                g = yVal +  491*uVal + 2583*vVal;
                b = yVal + 1907*uVal - 1813*vVal;
                
                if (r >= maxVal) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + adjust) >> rshift; r <<= 16; }
               
                if (g >= maxVal) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + adjust) >> rshift; g <<= 8; }

                if (b >= maxVal) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + adjust) >> rshift; }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs] = r | g | b;

                int uu, vv;
                uu = (uVal0 + uVal1) >> 1;
                vv = (vVal0 + vVal1) >> 1;

                // ------- toRGB 'Macro'
                yVal = y[idx+1] << 11; 
                uVal = uu; 
                vVal = vv;
                r = yVal - 2489*uVal -  881*vVal;
                g = yVal +  491*uVal + 2583*vVal;
                b = yVal + 1907*uVal - 1813*vVal;
                
                if (r >= maxVal) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + adjust) >> rshift; r <<= 16; }
               
                if (g >= maxVal) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + adjust) >> rshift; g <<= 8; }

                if (b >= maxVal) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + adjust) >> rshift; }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs+1] = r | g | b;
                uu = (uVal0 + uVal2) >> 1;
                vv = (vVal0 + vVal2) >> 1;

                // ------- toRGB 'Macro'
                yVal = y[offs] << 11; 
                uVal = uu; 
                vVal = vv;
                r = yVal - 2489*uVal -  881*vVal;
                g = yVal +  491*uVal + 2583*vVal;
                b = yVal + 1907*uVal - 1813*vVal;
                
                if (r >= maxVal) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + adjust) >> rshift; r <<= 16; }
               
                if (g >= maxVal) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + adjust) >> rshift; g <<= 8; }

                if (b >= maxVal) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + adjust) >> rshift; }
                // ------- toRGB 'Macro' END

                rgb[offs+rgbOffs] = r | g | b;
                uu = (uVal0 + uVal1 + uVal2 + uVal3 + 2) >> 2;
                vv = (vVal0 + vVal1 + vVal2 + vVal3 + 2) >> 2;

                // ------- toRGB 'Macro'
                yVal = y[offs+1] << 11; 
                uVal = uu; 
                vVal = vv;
                r = yVal - 2489*uVal -  881*vVal;
                g = yVal +  491*uVal + 2583*vVal;
                b = yVal + 1907*uVal - 1813*vVal;
                
                if (r >= maxVal) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + adjust) >> rshift; r <<= 16; }
               
                if (g >= maxVal) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + adjust) >> rshift; g <<= 8; }

                if (b >= maxVal) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + adjust) >> rshift; }
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
            yVal = y[idx] << 11; 
            uVal = uVal0; 
            vVal = vVal0;
            r = yVal - 2489*uVal -  881*vVal;
            g = yVal +  491*uVal + 2583*vVal;
            b = yVal + 1907*uVal - 1813*vVal;

            if (r >= maxVal) r = 0x00FF0000;
            else { r &= ~(r >> 31); r = (r + adjust) >> rshift; r <<= 16; }

            if (g >= maxVal) g = 0x0000FF00;
            else { g &= ~(g >> 31); g = (g + adjust) >> rshift; g <<= 8; }

            if (b >= maxVal) b = 0x000000FF;
            else { b &= ~(b >> 31); b = (b + adjust) >> rshift; }
            // ------- toRGB 'Macro' END

            rgb[idx+rgbOffs]   = r | g | b;
            rgb[idx+rgbOffs+1] = r | g | b;

            final int uu = (uVal0 + uVal2) >> 1;
            final int vv = (vVal0 + vVal2) >> 1;

            // ------- toRGB 'Macro'
            yVal = y[offs] << 11; 
            uVal = uu; 
            vVal = vv;
            r = yVal - 2489*uVal -  881*vVal;
            g = yVal +  491*uVal + 2583*vVal;
            b = yVal + 1907*uVal - 1813*vVal;

            if (r >= maxVal) r = 0x00FF0000;
            else { r &= ~(r >> 31); r = (r + adjust) >> rshift; r <<= 16; }

            if (g >= maxVal) g = 0x0000FF00;
            else { g &= ~(g >> 31); g = (g + adjust) >> rshift; g <<= 8; }

            if (b >= maxVal) b = 0x000000FF;
            else { b &= ~(b >> 31); b = (b + adjust) >> rshift; }
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
        final int rshift = this.fShift;
        final int adjust = (1 << rshift) >> 1;
        
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
                final int yVal0 =  2646*r  + 2818*g  + 2728*b;
                final int bVal0 = -(r<<12) +  868*g  + 3228*b;
                final int rVal0 = -1319*r  + (g<<12) - 2777*b;
                y[startLine+i] = (yVal0 + adjust) >> rshift;

                final int val1 = rgb[nextLine+rgbOffs+i];
                r = (val1 >> 16) & 0xFF;
                g = (val1 >> 8)  & 0xFF;
                b =  val1 & 0xFF;
                y[nextLine+i] = (2646*r  + 2818*g  + 2728*b + adjust) >> rshift;
                i++;

                final int val2 = rgb[startLine+rgbOffs+i];
                r = (val2 >> 16) & 0xFF;
                g = (val2 >> 8)  & 0xFF;
                b =  val2 & 0xFF;
                y[startLine+i] = (2646*r  + 2818*g  + 2728*b + adjust) >> rshift;

                final int val3 = rgb[nextLine+rgbOffs+i];
                r = (val3 >> 16) & 0xFF;
                g = (val3 >> 8)  & 0xFF;
                b =  val3 & 0xFF;                
                y[nextLine+i] = (2646*r  + 2818*g  + 2728*b + adjust) >> rshift;
                i++;

                // Decimate u, v (use position 0)
                u[offs] = (bVal0 + adjust) >> rshift;
                v[offs] = (rVal0 + adjust) >> rshift;
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
        final int rshift = this.iShift;
        final int adjust = (1 << rshift) >> 1;
        final int maxVal = (255 << rshift) - adjust;
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
                yVal = y[k++] << 11; 
                uVal = u[iOffs+i]; 
                vVal = v[iOffs+i];
                r = yVal - 2489*uVal -  881*vVal;
                g = yVal +  491*uVal + 2583*vVal;
                b = yVal + 1907*uVal - 1813*vVal;
                
                if (r >= maxVal) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + adjust) >> rshift; r <<= 16; }
               
                if (g >= maxVal) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + adjust) >> rshift; g <<= 8; }

                if (b >= maxVal) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + adjust) >> rshift; }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs] = r | g | b;

                // ------- toRGB 'Macro'
                yVal = y[k++] << 11;
                r = yVal - 2489*uVal -  881*vVal;
                g = yVal +  491*uVal + 2583*vVal;
                b = yVal + 1907*uVal - 1813*vVal;
                
                if (r >= maxVal) r = 0x00FF0000;
                else { r &= ~(r >> 31); r = (r + adjust) >> rshift; r <<= 16; }
               
                if (g >= maxVal) g = 0x0000FF00;
                else { g &= ~(g >> 31); g = (g + adjust) >> rshift; g <<= 8; }

                if (b >= maxVal) b = 0x000000FF;
                else { b &= ~(b >> 31); b = (b + adjust) >> rshift; }
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
        final int rshift = this.fShift;
        final int adjust = (1 << rshift) >> 1;
        
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

                final int yVal1 =  2646*r  + 2818*g  + 2728*b;
                final int bVal1 = -(r<<12) +  868*g  + 3228*b;
                final int rVal1 = -1319*r  + (g<<12) - 2777*b;

                rgbVal = rgb[k++];
                r = (rgbVal >> 16) & 0xFF;
                g = (rgbVal >> 8)  & 0xFF;
                b =  rgbVal & 0xFF;

                final int yVal2 =  2646*r  + 2818*g  + 2728*b;

                // Decimate u, v
                u[i] = (bVal1 + adjust) >> rshift;
                v[i] = (rVal1 + adjust) >> rshift;
                // ------- fromRGB 'Macro' END

                y[i+i]   = (yVal1 + adjust) >> rshift;
                y[i+i+1] = (yVal2 + adjust) >> rshift;
            }

            oOffs += half;
            iOffs += this.stride;
        }

        return true;
    }
       
    
    @Override
    public String toString() 
    {
       return "YSbSr";
    }    
}
