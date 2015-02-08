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

package kanzi.util.color;

import kanzi.ColorModelType;
import kanzi.util.sampling.DownSampler;
import kanzi.util.sampling.UpSampler;

// Reference:  http://www.poynton.com/notes/colour_and_gamma/ColorFAQ.html#RTFToC29
// One pass converter using a fast bilinear resampler with in-place supersampling
// A custom resampler can also be provided
// Implementation follows ITU-R 601 conversion
public final class YCbCrColorModelConverter implements ColorModelConverter
{
    private final int height;
    private final int width;
    private final int rgbOffset;
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
        this.rgbOffset = rgbOffset;
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
    //  0.257  0.504  0.098
    // -0.148 -0.291  0.439
    //  0.439 -0.368 -0.071
    private boolean convertRGBtoYUV444(int[] rgb, int[] y, int[] u, int[] v)
    {
        int startLine  = this.rgbOffset;
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
                final int yVal = 1052*r + 2065*g +  401*b;
                final int uVal = -607*r - 1192*g + 1799*b;
                final int vVal = 1799*r - 1506*g -  293*b;

                y[i] = ((yVal + 2048) >> 12) + 16;
                u[i] = ((uVal + 2048) >> 12) + 128;
                v[i] = ((vVal + 2048) >> 12) + 128;
                // ------- fromRGB 'Macro'  END
            }

            startLine2 += this.width;
            startLine  += this.stride;
        }

        return true;
    }


    // conversion matrix
    // 1.164 0.000 1.596
    // 1.164 0.392 0.813
    // 1.164 2.017 0.000 
    private boolean convertYUV444toRGB(int[] y, int[] u, int[] v, int[] rgb)
    {
        int startLine = 0;
        int startLine2 = this.rgbOffset;

        for (int j=0; j<this.height; j++)
        {
            final int end = startLine + this.width;

            for (int i=startLine, k=startLine2; i<end; i++)
            {
                // ------- toRGB 'Macro'
                final int yVal = 4769 * (y[i] - 16); 
                final int uVal = u[i] - 128; 
                final int vVal = v[i] - 128;
                int r = yVal             + 6537*vVal;
                int g = yVal - 1605*uVal - 3330*vVal;
                int b = yVal + 8263*uVal;

                if (r >= 1046528) r = 0x00FF0000;
                else { r = (r + 2048) >> 12; r &= (-r >> 31); r <<= 16; }

                if (g >= 1046528) g = 0x0000FF00;
                else { g = (g + 2048) >> 12; g &= (-g >> 31); g <<= 8; }

                if (b >= 1046528) b = 0x000000FF;
                else { b = (b + 2048) >> 12; b &= (-b >> 31); }
                // ------- toRGB 'Macro' END

                rgb[k++] = r | g | b;
            }

            startLine  += this.width;
            startLine2 += this.stride;
        }

        return true;
    }


    // In YUV422 format the U and V color components are supersampled 2:1 horizontally
    private boolean convertYUV422toYUV444(int[] y, int[] u, int[] v)
    {
        if (this.upSampler != null)
        {
           this.upSampler.superSampleHorizontal(u, u);
           this.upSampler.superSampleHorizontal(v, v);
           return true;
        }

        // In place super-sampling
        // Must scan Y backwards to avoid overwriting the data array
        final int w = this.width;
        final int half = w >> 1;
        int oOffs = this.width * (this.height - 1);
        int iOffs = half * (this.height - 1);

        for (int j=0; j<this.height; j++)
        {
            // Use 2 loops to support in place interpolation ... a bit slower
            // Odd columns
            for (int i=half-1; i>=0; i--)
            {
                u[oOffs+i+i] = u[iOffs+i];
                v[oOffs+i+i] = v[iOffs+i];
            }

            int uPrev = u[oOffs];
            int vPrev = v[oOffs];

            // Even columns
            for (int i=2; i<w; i+=2)
            {
                final int idx = oOffs + i;
                final int uVal = u[idx];
                final int vVal = v[idx];
                u[idx-1] = (uVal + uPrev) >> 1;
                v[idx-1] = (vVal + vPrev) >> 1;
                uPrev = uVal;
                vPrev = vVal;
            }

            u[oOffs+this.width-1] = uPrev;
            v[oOffs+this.width-1] = vPrev;
            oOffs -= this.stride;
            iOffs -= half;
        }

        return true;
    }


    // In YUV422 format the U and V color components are subsampled 1:2 horizontally
    private boolean convertYUV444toYUV422(int[] y, int[] u, int[] v)
    {
        if (this.downSampler != null)
        {
           this.downSampler.subSampleHorizontal(u, u);
           this.downSampler.subSampleHorizontal(v, v);
           return true;
        }

        int iOffs = 0;
        int oOffs = 0;

        for (int j=0; j<this.height; j++)
        {
            final int end = iOffs + this.width;

            // Simply decimate
            for (int i=iOffs; i<end; i+=8)
            {
                u[oOffs] = u[i];
                v[oOffs] = v[i];
                oOffs++;
                u[oOffs] = u[i+2];
                v[oOffs] = v[i+2];
                oOffs++;
                u[oOffs] = u[i+4];
                v[oOffs] = v[i+4];
                oOffs++;
                u[oOffs] = u[i+6];
                v[oOffs] = v[i+6];
                oOffs++;
            }

            iOffs += this.stride;
        }

        return true;
    }


    // In YUV420 format the U and V color components are supersampled 2:1 horizontally
    // and 2:1 vertically
    private boolean convertYUV420toYUV444(int[] y, int[] u, int[] v)
    {
       if (this.upSampler != null)
       {
          this.upSampler.superSample(u, u);
          this.upSampler.superSample(v, v);
          return true;
       }

       // In-place super sampling
       final int st = this.stride;
       final int w  = this.width;
       final int halfH = this.height >> 1;
       final int halfW = this.width >> 1;
       final int decY = st + st - w;
       int oOffs = ((st * (this.height - 2)) + this.width - 1);
       int iOffs = (halfW * halfH) - 1;
       int uPrev = u[iOffs];
       int vPrev = v[iOffs];
       iOffs--;
       int uVal = uPrev;
       int vVal = vPrev;
       u[oOffs] = uPrev;
       v[oOffs] = vPrev;
       oOffs--;

       // Last 2 lines
       for (int i=halfW-2; i>=0; i--)
       {
          uVal = u[iOffs];
          vVal = v[iOffs];
          iOffs--;
          final int uAvg = (uVal + uPrev) >> 1;
          final int vAvg = (vVal + vPrev) >> 1;
          u[oOffs+st] = uAvg;
          u[oOffs]    = uAvg;
          v[oOffs+st] = vAvg;
          v[oOffs]    = vAvg;
          oOffs--;
          u[oOffs+st] = uVal;
          u[oOffs]    = uVal;
          v[oOffs+st] = vVal;
          v[oOffs]    = vVal;
          oOffs--;
          uPrev = uVal;
          vPrev = vVal;
       }

       u[oOffs+st] = uVal;
       u[oOffs]    = uVal;
       v[oOffs+st] = vVal;
       v[oOffs]    = vVal;
       oOffs--;
       oOffs -= decY;

       // Process 2 lines at once (odd + even)
       //              A   AB    B   BC     C
       // A B C  ===> AD  ABDE  BE  BCEF   CF
       // D E F        D   DE    E   EF     F
       for (int j=halfH-2; j>=0; j--)
       {
          // Last pixel of lines
          int uVal2 = u[iOffs+halfW]; // F
          int uVal1 = u[iOffs]; // C
          int vVal2 = v[iOffs+halfW]; // F
          int vVal1 = v[iOffs]; // C
          iOffs--;
          u[oOffs+st] = (uVal1 + uVal2) >> 1; // CF
          u[oOffs]    = uVal1; // C
          v[oOffs+st] = (vVal1 + vVal2) >> 1; // CF
          v[oOffs]    = vVal1; // C
          iOffs--;
          int uPrev1 = uVal1; // C
          int uPrev2 = uVal2; // F
          int vPrev1 = vVal1; // C
          int vPrev2 = vVal2; // F

          // All columns
          for (int i=halfW-2; i>=0; i--)
          {
              uVal2 = u[iOffs+halfW]; // E
              uVal1 = u[iOffs]; // B
              vVal2 = v[iOffs+halfW]; // E
              vVal1 = v[iOffs]; // B
              iOffs--;
              u[oOffs+st] = (uVal1 + uVal2 + uPrev1 + uPrev2 + 2) >> 2; // BCEF
              u[oOffs]    = (uVal1 + uPrev1) >> 1; // BC
              v[oOffs+st] = (vVal1 + vVal2 + vPrev1 + vPrev2 + 2) >> 2; // BCEF
              v[oOffs]    = (vVal1 + vPrev1) >> 1; // BC
              oOffs--;
              u[oOffs+st] = (uVal1 + uVal2) >> 1; // BE
              u[oOffs]    = uVal1; // B
              v[oOffs+st] = (vVal1 + vVal2) >> 1; // BE
              v[oOffs]    = vVal1; // B
              oOffs--;
              uPrev1 = uVal1; // B
              uPrev2 = uVal2; // E
              vPrev1 = vVal1; // B
              vPrev2 = vVal2; // E
          }

          // First pixel of lines
          u[oOffs+st] = (uVal1 + uVal2) >> 1;
          u[oOffs]    = uVal1;
          v[oOffs+st] = (vVal1 + vVal2) >> 1;
          v[oOffs]    = vVal1;
          oOffs--;
          oOffs -= decY;
       }

        return true;
    }


    // In YUV420 format the U and V color components are subsampled 1:2 horizontally
    // and 1:2 vertically
    private boolean convertYUV444toYUV420(int[] y, int[] u, int[] v)
    {
        if (this.downSampler != null)
        {
           this.downSampler.subSample(u, u);
           this.downSampler.subSample(v, v);
           return true;
        }

        int startLine = 0;
        int offset = 0;

        for (int j=this.height-1; j>=0; j-=2)
        {
            final int nextLine = startLine + this.stride;
            final int end = startLine + this.width;

            // Simply decimate
            for (int i=startLine; i<end; i+=2)
            {
                u[offset] = u[i];
                v[offset] = v[i];
                offset++;
            }

            startLine = nextLine + this.stride;
        }

        return true;
    }


    // In YUV420 format the U and V color components are subsampled 1:2 horizontally
    // and 1:2 vertically
    private boolean convertYUV420toRGB(int[] y, int[] u, int[] v, int[] rgb)
    {
        if ((this.downSampler != null) && (this.upSampler != null))
        {
           this.convertYUV420toYUV444(y, u, v);
           this.convertYUV444toRGB(y, u, v, rgb);
           return true;
        }

        // In-place one-loop super sample and color conversion
        final int sw = this.width >> 1;
        final int sh = this.height >> 1;
        final int stride2 = this.stride << 1;
        final int rgbOffs = this.rgbOffset;
        int dLine = this.stride;
        int sLine = sw;
        int r, g, b;
        int yVal, uVal, vVal;

        for (int j=0; j<sh; j++)
        {
            // The last iteration (j==sh-1) must repeat the source line
            // EG: src lines 254 & 255 => dest lines 508 & 509
            //     src lines 255 & 255 => dest lines 510 & 511
            if (j == sh-1)
               sLine -= sw;

            int offs = dLine;
            int endi = sLine + sw;
            int uVal0, uVal1, uVal2, uVal3;
            int vVal0, vVal1, vVal2, vVal3;
            uVal0 = u[sLine-sw];
            vVal0 = v[sLine-sw];
            uVal2 = u[sLine];
            vVal2 = v[sLine];

            for (int i=sLine+1; i<endi; i++)
            {
                uVal1 = u[i-sw];
                vVal1 = v[i-sw];
                uVal3 = u[i];
                vVal3 = v[i];
                final int idx = offs - this.stride;

                // ------- toRGB 'Macro'
                yVal = 4769 * (y[idx] - 16); uVal = uVal0 - 128; vVal = vVal0 - 128;
                r = yVal + 6537*vVal;
                g = yVal - 1605*uVal - 3330*vVal;
                b = yVal + 8263*uVal;

                if (r >= 1046528) r = 0x00FF0000;
                else { r = (r + 2048) >> 12; r &= (-r >> 31); r <<= 16; }

                if (g >= 1046528) g = 0x0000FF00;
                else { g = (g + 2048) >> 12; g &= (-g >> 31); g <<= 8; }

                if (b >= 1046528) b = 0x000000FF;
                else { b = (b + 2048) >> 12; b &= (-b >> 31); }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs] = r | g | b;

                int uu, vv;
                uu = (uVal0 + uVal1) >> 1;
                vv = (vVal0 + vVal1) >> 1;

                // ------- toRGB 'Macro'
                yVal = 4769 * (y[idx+1] - 16); uVal = uu - 128; vVal = vv - 128;
                r = yVal + 6537*vVal;
                g = yVal - 1605*uVal - 3330*vVal;
                b = yVal + 8263*uVal;

                if (r >= 1046528) r = 0x00FF0000;
                else { r = (r + 2048) >> 12; r &= (-r >> 31); r <<= 16; }

                if (g >= 1046528) g = 0x0000FF00;
                else { g = (g + 2048) >> 12; g &= (-g >> 31); g <<= 8; }

                if (b >= 1046528) b = 0x000000FF;
                else { b = (b + 2048) >> 12; b &= (-b >> 31); }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs+1] = r | g | b;
                uu = (uVal0 + uVal2) >> 1;
                vv = (vVal0 + vVal2) >> 1;

                // ------- toRGB 'Macro'
                yVal = 4769 * (y[offs] - 16); uVal = uu - 128; vVal = vv - 128;
                r = yVal + 6537*vVal;
                g = yVal - 1605*uVal - 3330*vVal;
                b = yVal + 8263*uVal;

                if (r >= 1046528) r = 0x00FF0000;
                else { r = (r + 2048) >> 12; r &= (-r >> 31); r <<= 16; }

                if (g >= 1046528) g = 0x0000FF00;
                else { g = (g + 2048) >> 12; g &= (-g >> 31); g <<= 8; }

                if (b >= 1046528) b = 0x000000FF;
                else { b = (b + 2048) >> 12; b &= (-b >> 31); }
                // ------- toRGB 'Macro' END

                rgb[offs+rgbOffs] = r | g | b;
                uu = (uVal0 + uVal1 + uVal2 + uVal3 + 2) >> 2;
                vv = (vVal0 + vVal1 + vVal2 + vVal3 + 2) >> 2;

                // ------- toRGB 'Macro'
                yVal = 4769 * (y[offs+1] - 16); uVal = uu - 128; vVal = vv - 128;
                r = yVal + 6537*vVal;
                g = yVal - 1605*uVal - 3330*vVal;
                b = yVal + 8263*uVal;

                if (r >= 1046528) r = 0x00FF0000;
                else { r = (r + 2048) >> 12; r &= (-r >> 31); r <<= 16; }

                if (g >= 1046528) g = 0x0000FF00;
                else { g = (g + 2048) >> 12; g &= (-g >> 31); g <<= 8; }

                if (b >= 1046528) b = 0x000000FF;
                else { b = (b + 2048) >> 12; b &= (-b >> 31); }
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
            yVal = 4679 * (y[idx] - 16); uVal = uVal0 - 128; vVal = vVal0 - 128;

            r = yVal + 6537*vVal;
            g = yVal - 1605*uVal - 3330*vVal;
            b = yVal + 8263*uVal;

            if (r >= 1046528) r = 0x00FF0000;
            else { r = (r + 2048) >> 12; r &= (-r >> 31); r <<= 16; }

            if (g >= 1046528) g = 0x0000FF00;
            else { g = (g + 2048) >> 12; g &= (-g >> 31); g <<= 8; }

            if (b >= 1046528) b = 0x000000FF;
            else { b = (b + 2048) >> 12; b &= (-b >> 31); }
            // ------- toRGB 'Macro' END

            rgb[idx+rgbOffs]   = r | g | b;
            rgb[idx+rgbOffs+1] = r | g | b;

            final int uu = (uVal0 + uVal2) >> 1;
            final int vv = (vVal0 + vVal2) >> 1;

            // ------- toRGB 'Macro'
            yVal = 4769 * (y[offs] - 16); uVal = uu - 128; vVal = vv - 128;
            r = yVal + 6537*vVal;
            g = yVal - 1605*uVal - 3330*vVal;
            b = yVal + 8263*uVal;

            if (r >= 1046528) r = 0x00FF0000;
            else { r = (r + 2048) >> 12; r &= (-r >> 31); r <<= 16; }

            if (g >= 1046528) g = 0x0000FF00;
            else { g = (g + 2048) >> 12; g &= (-g >> 31); g <<= 8; }

            if (b >= 1046528) b = 0x000000FF;
            else { b = (b + 2048) >> 12; b &= (-b >> 31); }
            // ------- toRGB 'Macro' END

            rgb[offs+rgbOffs]   =  r | g | b;
            rgb[offs+rgbOffs+1] =  r | g | b;
            dLine += stride2;
            sLine += sw;
        }

        return true;
    }


    // In YUV420 format the U and V color components are subsampled 1:2 horizontally
    // and 1:2 vertically
    private boolean convertRGBtoYUV420(int[] rgb, int[] y, int[] u, int[] v)
    {
        if ((this.downSampler != null) && (this.upSampler != null))
        {
           this.convertRGBtoYUV444(rgb, y, u, v);
           this.convertYUV444toYUV420(y, u, v);
           return true;
        }

        int startLine = 0;
        int offs = 0;
        final int rgbOffs = this.rgbOffset;

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
                final int yVal0 = 1052*r + 2065*g +  401*b;
                final int uVal0 = -607*r - 1192*g + 1799*b;
                final int vVal0 = 1799*r - 1506*g -  293*b;
                y[startLine+i] = ((yVal0 + 2048) >> 12) + 16;

                final int val1 = rgb[nextLine+rgbOffs+i];
                r = (val1 >> 16) & 0xFF;
                g = (val1 >> 8)  & 0xFF;
                b =  val1 & 0xFF;
                final int yVal1 = 1052*r + 2065*g + 401*b;
                y[nextLine+i] = ((yVal1 + 2048) >> 12) + 16;
                i++;

                final int val2 = rgb[startLine+rgbOffs+i];
                r = (val2 >> 16) & 0xFF;
                g = (val2 >> 8)  & 0xFF;
                b =  val2 & 0xFF;
                final int yVal2 = 1052*r + 2065*g + 401*b;
                y[startLine+i] = ((yVal2 + 2048) >> 12) + 16;

                final int val3 = rgb[nextLine+rgbOffs+i];
                r = (val3 >> 16) & 0xFF;
                g = (val3 >> 8)  & 0xFF;
                b =  val3 & 0xFF;
                final int yVal3 = 1052*r + 2065*g + 401*b;
                y[nextLine+i] = ((yVal3 + 2048) >> 12) + 16;
                i++;

                // Decimate u, v (use position 0)
                u[offs] = ((uVal0 + 2048) >> 12) + 128;
                v[offs] = ((vVal0 + 2048) >> 12) + 128;
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
        if ((this.downSampler != null) && (this.upSampler != null))
        {
           this.convertYUV422toYUV444(y, u, v);
           this.convertYUV444toRGB(y, u, v, rgb);
           return true;
        }

        final int half = this.width >> 1;
        final int rgbOffs = this.rgbOffset;
        int oOffs = 0;
        int iOffs = 0;
        int k = 0;

        for (int j=0; j<this.height; j++)
        {
            for (int i=0; i<half; i++)
            {
                int r, g, b, yVal, uVal, vVal;
                int idx = oOffs + i + i;

                // ------- toRGB 'Macro'
                yVal = 4769 * (y[k++] - 16); uVal = u[iOffs+i]; vVal = v[iOffs+i];
                r = yVal + 6537*vVal;
                g = yVal - 1605*uVal - 3330*vVal;
                b = yVal + 8263*uVal;

                if (r >= 1046528) r = 0x00FF0000;
                else { r = (r + 2048) >> 12; r &= (-r >> 31); r <<= 16; }

                if (g >= 1046528) g = 0x0000FF00;
                else { g = (g + 2048) >> 12; g &= (-g >> 31); g <<= 8; }

                if (b >= 1046528) b = 0x000000FF;
                else { b = (b + 2048) >> 12; b &= (-b >> 31); }
                // ------- toRGB 'Macro' END

                rgb[idx+rgbOffs] = r | g | b;

                // ------- toRGB 'Macro'
                yVal = 4769 * (y[k++] - 16);
                r = yVal + 6537*vVal;
                g = yVal - 1605*uVal - 3330*vVal;
                b = yVal + 8263*uVal;

                if (r >= 1046528) r = 0x00FF0000;
                else { r = (r + 2048) >> 12; r &= (-r >> 31); r <<= 16; }

                if (g >= 1046528) g = 0x0000FF00;
                else { g = (g + 2048) >> 12; g &= (-g >> 31); g <<= 8; }

                if (b >= 1046528) b = 0x000000FF;
                else { b = (b + 2048) >> 12; b &= (-b >> 31); }
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
        if ((this.downSampler != null) && (this.upSampler != null))
        {
           this.convertRGBtoYUV444(rgb, y, u, v);
           this.convertYUV444toYUV422(y, u, v);
           return true;
        }

        int iOffs = this.rgbOffset;
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

                final int yVal1 = 1052*r + 2065*g +  401*b;
                final int uVal1 = -607*r - 1192*g + 1799*b;
                final int vVal1 = 1799*r - 1506*g -  293*b;

                rgbVal = rgb[k++];
                r = (rgbVal >> 16) & 0xFF;
                g = (rgbVal >> 8) & 0xFF;
                b =  rgbVal & 0xFF;

                final int yVal2 = 1052*r + 2065*g + 401*b;

                // Decimate u, v
                u[i] = ((uVal1 + 2048) >> 12) + 128;
                v[i] = ((vVal1 + 2048) >> 12) + 128;
                y[i+i]   = ((yVal1 + 2048) >> 12) + 16;
                y[i+i+1] = ((yVal2 + 2048) >> 12) + 16;
                // ------- fromRGB 'Macro'  END
            }

            oOffs += half;
            iOffs += this.stride;
        }

        return true;
    }
}
