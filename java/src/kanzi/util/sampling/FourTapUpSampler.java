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

package kanzi.util.sampling;


// Up sampling by a factor or 2 or 4 based on a 4-tap filter
// 1/4 pixel	(-13, 112, 38,  -9)/128
// 2/4 pixel	(-18,  82,  82,  -18)/128
// 3/4 pixel	(-9,  38, 112, -13)/128

public class FourTapUpSampler implements UpSampler
{
    private static final int DIR_VERTICAL   = 1;
    private static final int DIR_HORIZONTAL = 2;

    private static final int SHIFT  = 7; // because the filter coeffs add to 2^7
    private static final int ADJUST = 1 << (SHIFT - 1);
    private static final int[] CENTRAL_FILTER_128 = { 5, 5, 5, 22, 22, 5 };
    private static final int[] FILTER_128 = { -13, 112, 38, -9, -18, 82 };
    private final int width;
    private final int height;
    private final int stride;
    private final int offset;
    private final int factor;
    private final int pixelRangeOffset;


    public FourTapUpSampler(int width, int height)
    {
        this(width, height, width, 0, 2, true);
    }


    public FourTapUpSampler(int width, int height, int factor)
    {
        this(width, height, width, 0, factor, true);
    }


    public FourTapUpSampler(int width, int height, int factor, boolean isPixelPositive)
    {
        this(width, height, width, 0, factor, isPixelPositive);
    }


    // 'isPixelPositive' says whether the pixel channel range is [0..255] or [-128..127]
    public FourTapUpSampler(int width, int height, int stride, int offset, int factor, boolean isPixelPositive)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");

        if (offset < 0)
            throw new IllegalArgumentException("The offset must be at least 0");

        if (stride < width)
            throw new IllegalArgumentException("The stride must be at least as big as the width");

        if ((height & 7) != 0)
            throw new IllegalArgumentException("The height must be a multiple of 8");

        if ((width & 7) != 0)
            throw new IllegalArgumentException("The width must be a multiple of 8");

        if ((factor != 2) && (factor != 4))
            throw new IllegalArgumentException("This implementation only supports "+
                    "a scaling factor equal to 2 or 4");

        this.height = height;
        this.width = width;
        this.stride = stride;
        this.offset = offset;
        this.factor = factor;
        this.pixelRangeOffset = (isPixelPositive == true) ? 0 : 128;
    }


    @Override
    public boolean supportsScalingFactor(int factor)
    {
        return ((factor == 2) || (factor == 4)) ? true : false;
    }


    // Grid:
    // D   E   F   G
    // o   o   o   o
    // o   o   o   o
    // o   o   o   o
    // I   JabcK   L
    // o   defgo   o
    // o   hijko   o
    // o   npqro   o
    // N   O   P   Q
    // o   o   o   o
    // o   o   o   o
    // o   o   o   o
    // R   S   T   U
    //
    // Formulas:
    // Horizontal
    // a = (w00*I + w01*J + w02*K + w03*L + ADJUST) >> SHIFT;
    // b = (w04*I + w05*J + w06*K + w07*L + ADJUST) >> SHIFT;
    // c = (w08*I + w09*J + w10*K + w11*L + ADJUST) >> SHIFT;
    // Vertical
    // d = (w00*E + w01*J + w02*O + w03*S + ADJUST) >> SHIFT;
    // h = (w04*E + w05*J + w06*O + w07*S + ADJUST) >> SHIFT;
    // n = (w08*E + w09*J + w10*O + w11*S + ADJUST) >> SHIFT;
    // Diagonal
    // e = (w00*D + w01*J + w02*P + w03*U + ADJUST) >> SHIFT;
    // g = (w00*G + w01*K + w02*O + w03*R + ADJUST) >> SHIFT;
    // p = (w04*D + w05*J + w06*P + w07*U + ADJUST) >> SHIFT;
    // r = (w04*G + w05*K + w06*O + w07*R + ADJUST) >> SHIFT;
    // Combination
    // f = (e+g)>>1
    // i = (e+p)>>1
    // k = (g+r)>>1
    // q = (p+r)>>1
    // Central position (uses a 12-tap symetric non-separable filter)
    // j = ((5*E+*5F) + (5*I+22*J+22*K+5*L) + (5*N+22*O+22*P+5*Q) + (5*S+5*T) + 64) >> 7

    private void reSample(int[] input, int[] output, int direction)
    {
        final int sw = this.width;
        final int sh = this.height;
        final int scale = this.factor;
        final int logScale = scale >> 1; // valid for 2 and 4
        final int xIncShift = ((direction & DIR_HORIZONTAL) != 0) ? logScale : 0;
        final int yIncShift = ((direction & DIR_VERTICAL) != 0) ? logScale : 0;
        final int xInc = scale;
        final int yInc = scale;
        final int dw = sw * scale;
        final int dh = sh * scale;
        final int lineInc = yInc * dw;

        final int w00 = FILTER_128[0];
        final int w01 = FILTER_128[1];
        final int w02 = FILTER_128[2];
        final int w03 = FILTER_128[3];
        final int w04 = FILTER_128[4];
        final int w05 = FILTER_128[5];
        final int w06 = w05;
        final int w07 = w04;
        final int w08 = w03;
        final int w09 = w02;
        final int w10 = w01;
        final int w11 = w00;

        final int c0 = CENTRAL_FILTER_128[0];
        final int c1 = CENTRAL_FILTER_128[1];
        final int c2 = CENTRAL_FILTER_128[2];
        final int c3 = CENTRAL_FILTER_128[3];
        final int c4 = CENTRAL_FILTER_128[4];
        final int c5 = CENTRAL_FILTER_128[5];

        int line3 = (dh-1) * dw;
        int line2 = line3 - dw;
        int line1 = line2 - dw;
        int line0 = (dh-yInc) * dw;

        final int maxSx = sw - 3;
        final int maxSy = sh - 3;
        final int pixel_range_offset = this.pixelRangeOffset;

        for (int y=dh-yInc; y>=0; y-=yInc)
        {
            final int sy = y >> yIncShift;
            final int y0, y1, y2, y3;

            if (sy <= maxSy)
            {
               if (sy >= 1)
               {
                  y0 = (sy - 1) * this.stride;
                  y1 = y0 + this.stride;
               }
               else
               {
                  y0 = 0;
                  y1 = 0;
               }

               y2 = y1 + this.stride;
               y3 = y2 + this.stride;
            }
            else
            {
               y0 = (sy-1 <= sh-1) ? (sy-1)*this.stride : (sh-1)*this.stride;
               y1 = (sy   <= sh-1) ? ((y0+this.stride)) : y0;
               y2 = (sy+1 <= sh-1) ? ((y1+this.stride)) : y1;
               y3 =  y2;
            }

            for (int x=dw-xInc; x>=0; x-=xInc)
            {
                final int sx = (x >> xIncShift) + this.offset;
                int x0, x1, x2, x3;

                if (sx <= maxSx)
                {
                   if (sx >= 1)
                   {
                      x0 = sx - 1;
                      x1 = x0 + 1;
                   }
                   else
                   {
                      x0 = 0;
                      x1 = (sx >= 1) ? 1 : 0;
                   }

                   x2 = x1 + 1;
                   x3 = x2 + 1;
                }
                else
                {
                   x0 = (sx-1 < sw-1) ? sx-1: sw-1;
                   x1 = (sx   < sw-1) ? x0+1: sw-1;
                   x2 = (sx+1 < sw-1) ? x1+1: sw-1;
                   x3 = sw-1;
                }

                // Adjust to positive range
                final int pE = input[y0+x1] + pixel_range_offset;
                final int pO = input[y2+x1] + pixel_range_offset;
                final int pS = input[y3+x1] + pixel_range_offset;
                final int pI = input[y1+x0] + pixel_range_offset;
                final int pJ = input[y1+x1] + pixel_range_offset;
                final int pK = input[y1+x2] + pixel_range_offset;
                final int pL = input[y1+x3] + pixel_range_offset;

                output[line0+x] = pJ - pixel_range_offset;

                if ((direction & DIR_HORIZONTAL) != 0)
                {
                    int bVal = ((w04*pI + w05*pJ + w06*pK + w07*pL) + ADJUST) >> SHIFT;
                    bVal = (bVal > 255) ? 255 : bVal & ~(bVal >> 31);

                    if (scale == 2)
                    {
                       output[line0+x+1] = bVal - pixel_range_offset;
                    }
                    else
                    {
                        int aVal = (w00*pI + w01*pJ + w02*pK + w03*pL + ADJUST) >> SHIFT;
                        aVal = (aVal > 255) ? 255 : aVal & ~(aVal >> 31);
                        int cVal = (w08*pI + w09*pJ + w10*pK + w11*pL + ADJUST) >> SHIFT;
                        cVal = (cVal > 255) ? 255 : cVal & ~(cVal >> 31);
                        output[line0+x+1] = aVal - pixel_range_offset;
                        output[line0+x+2] = bVal - pixel_range_offset;
                        output[line0+x+3] = cVal - pixel_range_offset;
                    }
                }

                if ((direction & DIR_VERTICAL) != 0)
                {
                    int hVal = (w04*pE + w05*pJ + w06*pO + w07*pS + ADJUST) >> SHIFT;
                    hVal = (hVal > 255) ? 255 : hVal & ~(hVal >> 31);

                    if (scale == 2)
                    {
                        output[line1+x] = hVal - pixel_range_offset;
                    }
                    else
                    {
                        int dVal = (w00*pE + w01*pJ + w02*pO + w03*pS + ADJUST) >> SHIFT;
                        dVal = (dVal > 255) ? 255 : dVal & ~(dVal >> 31);
                        int nVal = (w08*pE + w09*pJ + w10*pO + w11*pS + ADJUST) >> SHIFT;
                        nVal = (nVal > 255) ? 255 : nVal & ~(nVal >> 31);
                        output[line1+x] = dVal - pixel_range_offset;
                        output[line2+x] = hVal - pixel_range_offset;
                        output[line3+x] = nVal - pixel_range_offset;
                   }
                }

                if (((direction & DIR_VERTICAL) != 0) && ((direction & DIR_HORIZONTAL) != 0))
                {
                    // Adjust to positive range
                    final int pD = input[y0+x0] + pixel_range_offset;
                    final int pF = input[y0+x2] + pixel_range_offset;
                    final int pG = input[y0+x3] + pixel_range_offset;
                    final int pN = input[y2+x0] + pixel_range_offset;
                    final int pP = input[y2+x2] + pixel_range_offset;
                    final int pQ = input[y2+x3] + pixel_range_offset;
                    final int pR = input[y3+x0] + pixel_range_offset;
                    final int pT = input[y3+x2] + pixel_range_offset;
                    final int pU = input[y3+x3] + pixel_range_offset;

                    int jVal = ((c0*pE + c1*pF) + (c2*pI + c3*pJ + c4*pK + c5*pL) +
                                (c5*pN + c4*pO + c3*pP + c2*pQ) + (c1*pS + c0*pT) + ADJUST) >> SHIFT;
                    jVal = (jVal > 255) ? 255 : jVal & ~(jVal >> 31);

                    if (scale == 2)
                    {
                        output[line1+x+1] = jVal - pixel_range_offset;
                    }
                    else
                    {
                        int eVal = (w00*pD + w01*pJ + w02*pP + w03*pU + ADJUST) >> SHIFT;
                        eVal = (eVal > 255) ? 255 : eVal & ~(eVal >> 31);
                        int gVal = (w00*pG + w01*pK + w02*pO + w03*pR + ADJUST) >> SHIFT;
                        gVal = (gVal > 255) ? 255 : gVal & ~(gVal >> 31);
                        int pVal = (w04*pD + w05*pJ + w06*pP + w07*pU + ADJUST) >> SHIFT;
                        pVal = (pVal > 255) ? 255 : pVal & ~(pVal >> 31);
                        int rVal = (w04*pG + w05*pK + w06*pO + w07*pR + ADJUST) >> SHIFT;
                        rVal = (rVal > 255) ? 255 : rVal & ~(rVal >> 31);
                        final int fVal = (eVal + gVal) >> 1;
                        final int iVal = (eVal + pVal) >> 1;
                        final int kVal = (gVal + rVal) >> 1;
                        final int qVal = (pVal + rVal) >> 1;
                        output[line1+x+1] = eVal - pixel_range_offset;
                        output[line1+x+2] = fVal - pixel_range_offset;
                        output[line1+x+3] = gVal - pixel_range_offset;
                        output[line2+x+1] = iVal - pixel_range_offset;
                        output[line2+x+2] = jVal - pixel_range_offset;
                        output[line2+x+3] = kVal - pixel_range_offset;
                        output[line3+x+1] = pVal - pixel_range_offset;
                        output[line3+x+2] = qVal - pixel_range_offset;
                        output[line3+x+3] = rVal - pixel_range_offset;
                    }
                }
            }

            line0 -= lineInc;
            line1 = line0 + dw;
            line2 = line1 + dw;
            line3 = line2 + dw;
        }
        
        System.arraycopy(output, (dh-2)*dw, output, (dh-1)*dw, dw);
     }


    @Override
    public void superSampleHorizontal(int[] input, int[] output)
    {
        this.reSample(input, output, DIR_HORIZONTAL);
    }


    @Override
    public void superSampleVertical(int[] input, int[] output)
    {
        this.reSample(input, output, DIR_VERTICAL);
    }


    @Override
    public void superSample(int[] input, int[] output)
    {
        this.reSample(input, output, DIR_HORIZONTAL | DIR_VERTICAL);
    }

}