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


// Down sampling by a factor or 2 or 4 based on a 6-tap asymetric filter
// 1/4 pixel	(3, -15, 111, 37, -10, 2)/128
// 2/4 pixel	(3, -17,  78, 78, -17, 3)/128
// 3/4 pixel	(2, -10, 37, 111, -15, 3)/128

public class SixTapDownSampler implements DownSampler
{
    private static final int DIR_VERTICAL   = 1;
    private static final int DIR_HORIZONTAL = 2;

    private static final int SHIFT  = 7; // because the filter coeffs add to 2^7
    private static final int ADJUST = 1 << (SHIFT - 1);
    private static final int[] CENTRAL_FILTER_128 = { 5, 5, 5, 22, 22, 5 };
    private static final int[] FILTER_128 = { 3, -15, 111, 37, -10, 2, 3, -17, 78 };
    // alternate values                     { 1, -4, 28, 9, -3, 1, 1, -5, 20 } / 32
    private final int width;
    private final int height;
    private final int stride;
    private final int offset;
    private final int factor;


    public SixTapDownSampler(int width, int height)
    {
        this(width, height, width, 0, 2);
    }


    public SixTapDownSampler(int width, int height, int factor)
    {
        this(width, height, width, 0, factor);
    }


    public SixTapDownSampler(int width, int height, int stride, int offset, int factor)
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
    }


    @Override
    public boolean supportsScalingFactor(int factor)
    {
        return ((factor == 2) || (factor == 4)) ? true : false;
    }


    // Grid:
    // A   o   B   o   o   C
    // o   o   o   o   o   o
    // o   o   o   o   o   o
    // o   o   o   o   o   o
    // o   D   E   F   G   o
    // o   o   o   o   o   o
    // o   o   o   o   o   o
    // o   o   o   o   o   o
    // H   I   JabcK   L   M
    // o   o   defgo   o   o
    // o   o   hijko   o   o
    // o   o   npqro   o   o
    // o   N   O   P   Q   o
    // o   o   o   o   o   o
    // o   o   o   o   o   o
    // o   o   o   o   o   o
    // o   R   S   T   U   o
    // o   o   o   o   o   o
    // o   o   o   o   o   o
    // o   o   o   o   o   o
    // V   o   W   o   o   X
    //
    // Formulas:
    // Horizontal
    // a = (w00*H + w01*I + w02*J + w03*K + w04*L + w05*M + ADJUST) >> SHIFT;
    // b = (w06*H + w07*I + w08*J + w09*K + w10*L + w11*M + ADJUST) >> SHIFT;
    // c = (w12*H + w13*I + w14*J + w15*K + w16*L + w17*M + ADJUST) >> SHIFT;
    // Vertical
    // d = (w00*B + w01*E + w02*J + w03*O + w04*S + w05*W + ADJUST) >> SHIFT;
    // h = (w06*B + w07*E + w08*J + w09*O + w10*S + w11*W + ADJUST) >> SHIFT;
    // n = (w12*B + w13*E + w14*J + w15*O + w16*S + w17*W + ADJUST) >> SHIFT;
    // Diagonal
    // e = (w00*A + w01*D + w02*J + w03*P + w04*U + w05*X + ADJUST) >> SHIFT;
    // g = (w00*C + w01*G + w02*K + w03*O + w04*R + w05*V + ADJUST) >> SHIFT;
    // p = (w12*A + w13*D + w14*J + w15*P + w16*U + w17*X + ADJUST) >> SHIFT;
    // r = (w12*C + w13*G + w14*K + w15*O + w16*R + w17*V + ADJUST) >> SHIFT;
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
        final int w06 = FILTER_128[6];
        final int w07 = FILTER_128[7];
        final int w08 = FILTER_128[8];
        final int w09 = w08;
        final int w10 = w07;
        final int w11 = w06;
        final int w12 = w05;
        final int w13 = w04;
        final int w14 = w03;
        final int w15 = w02;
        final int w16 = w01;
        final int w17 = w00;

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

        for (int y=dh-yInc; y>=0; y-=yInc)
        {
            final int sy = y << yIncShift;
            final int y0, y1, y2, y3, y4, y5;

            if (sy+3 <= sh-1)
            {
               if (sy >= 2)
               {
                  y0 = (sy - 2) * this.stride;
                  y1 = y0 + this.stride;
               }
               else
               {
                  y0 = 0;
                  y1 = (sy >= 1) ? this.stride : 0;
               }

               y2 = y1 + this.stride;
               y3 = y2 + this.stride;
               y4 = y3 + this.stride;
               y5 = y4 + this.stride;
            }
            else
            {
               y0 = (sy-2 <= sh-1) ? (sy-2)*this.stride : (sh-1)*this.stride;
               y1 = (sy-1 <= sh-1) ? ((y0+this.stride)) : y0;
               y2 = (sy   <= sh-1) ? ((y1+this.stride)) : y1;
               y3 = (sy+1 <= sh-1) ? ((y2+this.stride)) : y2;
               y4 = (sy+2 <= sh-1) ? ((y3+this.stride)) : y3;
               y5 = y4;
            }

            for (int x=dw-xInc; x>=0; x-=xInc)
            {
                final int sx = (x << xIncShift) + this.offset;
                int x0, x1, x2, x3, x4, x5;

                if (sx+3 < sw-1)
                {
                   if (sx >= 2)
                   {
                      x0 = sx - 2;
                      x1 = x0 + 1;
                   }
                   else
                   {
                      x0 = 0;
                      x1 = (sx >= 1) ? 1 : 0;
                   }

                   x2 = x1 + 1;
                   x3 = x2 + 1;
                   x4 = x3 + 1;
                   x5 = x4 + 1;
                }
                else
                {
                   x0 = (sx-2 < sw-1) ? sx-2: sw-1;
                   x1 = (sx-1 < sw-1) ? x0+1: sw-1;
                   x2 = (sx   < sw-1) ? x1+1: sw-1;
                   x3 = (sx+1 < sw-1) ? x2+1: sw-1;
                   x4 = (sx+2 < sw-1) ? x3+1: sw-1;
                   x5 = sw - 1;
                }

                int pB = input[y0+x2];
                int pE = input[y1+x2];
                int pO = input[y3+x2];
                int pS = input[y4+x2];
                int pH = input[y2+x0];
                int pI = input[y2+x1];
                int pL = input[y2+x4];
                int pM = input[y2+x5];
                int pJ = input[y2+x2];
                int pK = input[y2+x3];
                int pW = input[y5+x2];

                output[line0+x] = pJ;

                if ((direction & DIR_HORIZONTAL) != 0)
                {
                    final int bVal = (w06*pH + w07*pI + w08*pJ + w09*pK + w10*pL + w11*pM + ADJUST) >> SHIFT;

                    if (scale == 2)
                    {
                        output[line0+x+1] = bVal & ~(bVal >> 31);
                    }
                    else
                    {
                        final int aVal = (w00*pH + w01*pI + w02*pJ + w03*pK + w04*pL + w05*pM + ADJUST) >> SHIFT;
                        final int cVal = (w12*pH + w13*pI + w14*pJ + w15*pK + w16*pL + w17*pM + ADJUST) >> SHIFT;
                        output[line0+x+1] = aVal & ~(aVal >> 31);
                        output[line0+x+2] = bVal & ~(bVal >> 31);
                        output[line0+x+3] = cVal & ~(cVal >> 31);
                    }
                }

                if ((direction & DIR_VERTICAL) != 0)
                {
                    final int hVal = (w06*pB + w07*pE + w08*pJ + w09*pO + w10*pS + w11*pW + ADJUST) >> SHIFT;

                    if (scale == 2)
                    {
                        output[line1+x] = hVal & ~(hVal >> 31);
                    }
                    else
                    {
                        final int dVal = (w00*pB + w01*pE + w02*pJ + w03*pO + w04*pS + w05*pW + ADJUST) >> SHIFT;
                        final int nVal = (w12*pB + w13*pE + w14*pJ + w15*pO + w16*pS + w17*pW + ADJUST) >> SHIFT;
                        output[line1+x] = dVal & ~(dVal >> 31);
                        output[line2+x] = hVal & ~(hVal >> 31);
                        output[line3+x] = nVal & ~(nVal >> 31);
                    }
                }

                if (((direction & DIR_VERTICAL) != 0) && ((direction & DIR_HORIZONTAL) != 0))
                {
                    int pA = input[y0+x0];
                    int pC = input[y0+x5];
                    int pD = input[y1+x1];
                    int pF = input[y1+x3];
                    int pG = input[y1+x4];
                    int pN = input[y3+x1];
                    int pP = input[y3+x3];
                    int pQ = input[y3+x4];
                    int pR = input[y4+x1];
                    int pT = input[y4+x3];
                    int pU = input[y4+x4];
                    int pV = input[y5+x0];
                    int pX = input[y5+x5];

                    final int jVal = ((c0*pE + c1*pF) + (c2*pI + c3*pJ + c4*pK + c5*pL) +
                                      (c5*pN + c4*pO + c3*pP + c2*pQ) + (c1*pS + c0*pT) + ADJUST) >> SHIFT;

                    if (scale == 2)
                    {
                        output[line1+x+1] = jVal & ~(jVal >> 31);
                    }
                    else
                    {
                        final int eVal = (w00*pA + w01*pD + w02*pJ + w03*pP + w04*pU + w05*pX + ADJUST) >> SHIFT;
                        final int gVal = (w00*pC + w01*pG + w02*pK + w03*pO + w04*pR + w05*pV + ADJUST) >> SHIFT;
                        final int pVal = (w12*pA + w13*pD + w14*pJ + w15*pP + w16*pU + w17*pX + ADJUST) >> SHIFT;
                        final int rVal = (w12*pC + w13*pG + w14*pK + w15*pO + w16*pR + w17*pV + ADJUST) >> SHIFT;
                        final int fVal = (eVal + gVal) >> 1;
                        final int iVal = (eVal + pVal) >> 1;
                        final int kVal = (gVal + rVal) >> 1;
                        final int qVal = (pVal + rVal) >> 1;
                        output[line1+x+1] = eVal & ~(eVal >> 31);
                        output[line1+x+2] = fVal & ~(fVal >> 31);
                        output[line1+x+3] = gVal & ~(gVal >> 31);
                        output[line2+x+1] = iVal & ~(iVal >> 31);
                        output[line2+x+2] = jVal & ~(jVal >> 31);
                        output[line2+x+3] = kVal & ~(kVal >> 31);
                        output[line3+x+1] = pVal & ~(pVal >> 31);
                        output[line3+x+2] = qVal & ~(qVal >> 31);
                        output[line3+x+3] = rVal & ~(rVal >> 31);
                    }
                }
            }

            line0 -= lineInc;
            line1 = line0 + dw;
            line2 = line1 + dw;
            line3 = line2 + dw;
        }
     }


    @Override
    public void subSampleHorizontal(int[] input, int[] output)
    {
        this.reSample(input, output, DIR_HORIZONTAL);
    }


    @Override
    public void subSampleVertical(int[] input, int[] output)
    {
        this.reSample(input, output, DIR_VERTICAL);
    }


    @Override
    public void subSample(int[] input, int[] output)
    {
        this.reSample(input, output, DIR_HORIZONTAL | DIR_VERTICAL);
    }

}