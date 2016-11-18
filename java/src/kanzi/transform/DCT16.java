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

package kanzi.transform;

import kanzi.SliceIntArray;
import kanzi.IntTransform;


// Implementation of Discrete Cosine Transform of dimension 16
// Due to rounding errors, the reconstruction may not be perfect
public final class DCT16 implements IntTransform
{
    // Weights
    private static final int W0   = 64;
    private static final int W1   = 64;
    private static final int W16  = 90;
    private static final int W17  = 87;
    private static final int W18  = 80;
    private static final int W19  = 70;
    private static final int W20  = 57;
    private static final int W21  = 43;
    private static final int W22  = 25;
    private static final int W23  = 9;
    private static final int W32  = 89;
    private static final int W33  = 75;
    private static final int W34  = 50;
    private static final int W35  = 18;
    private static final int W48  = 87;
    private static final int W49  = 57;
    private static final int W50  = 9;
    private static final int W51  = -43;
    private static final int W52  = -80;
    private static final int W53  = -90;
    private static final int W54  = -70;
    private static final int W55  = -25;
    private static final int W64  = 83;
    private static final int W65  = 36;
    private static final int W80  = 80;
    private static final int W81  = 9;
    private static final int W82  = -70;
    private static final int W83  = -87;
    private static final int W84  = -25;
    private static final int W85  = 57;
    private static final int W86  = 90;
    private static final int W87  = 43;
    private static final int W96  = 75;
    private static final int W97  = -18;
    private static final int W98  = -89;
    private static final int W99  = -50;
    private static final int W112 = 70;
    private static final int W113 = -43;
    private static final int W114 = -87;
    private static final int W115 = 9;
    private static final int W116 = 90;
    private static final int W117 = 25;
    private static final int W118 = -80;
    private static final int W119 = -57;
    private static final int W128 = 64;
    private static final int W129 = -64;
    private static final int W144 = 57;
    private static final int W145 = -80;
    private static final int W146 = -25;
    private static final int W147 = 90;
    private static final int W148 = -9;
    private static final int W149 = -87;
    private static final int W150 = 43;
    private static final int W151 = 70;
    private static final int W160 = 50;
    private static final int W161 = -89;
    private static final int W162 = 18;
    private static final int W163 = 75;
    private static final int W176 = 43;
    private static final int W177 = -90;
    private static final int W178 = 57;
    private static final int W179 = 25;
    private static final int W180 = -87;
    private static final int W181 = 70;
    private static final int W182 = 9;
    private static final int W183 = -80;
    private static final int W192 = 36;
    private static final int W193 = -83;
    private static final int W208 = 25;
    private static final int W209 = -70;
    private static final int W210 = 90;
    private static final int W211 = -80;
    private static final int W212 = 43;
    private static final int W213 = 9;
    private static final int W214 = -57;
    private static final int W215 = 87;
    private static final int W224 = 18;
    private static final int W225 = -50;
    private static final int W226 = 75;
    private static final int W227 = -89;
    private static final int W240 = 9;
    private static final int W241 = -25;
    private static final int W242 = 43;
    private static final int W243 = -57;
    private static final int W244 = 70;
    private static final int W245 = -80;
    private static final int W246 = 87;
    private static final int W247 = -90;
    
    private static final int MAX_VAL = 1<<16;
    private static final int MIN_VAL = -(MAX_VAL+1);
            
    private final int fShift;
    private final int iShift;
    private final SliceIntArray data;

    
    public DCT16()
    {
       this.fShift = 12;
       this.iShift = 20;
       this.data = new SliceIntArray(new int[256], 0);
    }


    @Override
    public boolean forward(SliceIntArray src, SliceIntArray dst)
    {
       if (!SliceIntArray.isValid(src))
          return false;

       if (src.length != 256)
          return false;
       
       if (src != dst)
       {
          if (!SliceIntArray.isValid(dst))
            return false;

          if (dst.index + 256 > dst.array.length)
            return false;   
       }
       
       this.data.index = 0;
       computeForward(src, this.data, 6);
       computeForward(this.data, dst, this.fShift-6);
       src.index += 256;
       dst.index += 256;       
       return true;
    }
    
    
    private static void computeForward(SliceIntArray src, SliceIntArray dst, int shift)
    {       
       final int[] input = src.array;
       final int[] output = dst.array;
       final int srcIdx = src.index;
       final int dstIdx = dst.index;        
       final int round = (1 << shift) >> 1;

       for (int i=0; i<16; i++)
       {
          final int si = srcIdx + (i << 4);
          final int x0  = input[si];
          final int x1  = input[si+1];
          final int x2  = input[si+2];
          final int x3  = input[si+3];
          final int x4  = input[si+4];
          final int x5  = input[si+5];
          final int x6  = input[si+6];
          final int x7  = input[si+7];
          final int x8  = input[si+8];
          final int x9  = input[si+9];
          final int x10 = input[si+10];
          final int x11 = input[si+11];
          final int x12 = input[si+12];
          final int x13 = input[si+13];
          final int x14 = input[si+14];
          final int x15 = input[si+15];
     
          final int a0   = x0 + x15;
          final int a1   = x1 + x14;
          final int a2   = x0 - x15;
          final int a3   = x1 - x14;
          final int a4   = x2 + x13;
          final int a5   = x3 + x12;
          final int a6   = x2 - x13;
          final int a7   = x3 - x12;
          final int a8   = x4 + x11;
          final int a9   = x5 + x10;
          final int a10  = x4 - x11;
          final int a11  = x5 - x10;
          final int a12  = x6 + x9;
          final int a13  = x7 + x8;
          final int a14  = x6 - x9;
          final int a15  = x7 - x8;

          final int b0 = a0 + a13; 
          final int b1 = a1 + a12;
          final int b2 = a0 - a13;
          final int b3 = a1 - a12;
          final int b4 = a4 + a9;
          final int b5 = a5 + a8;
          final int b6 = a4 - a9;
          final int b7 = a5 - a8;

          final int c0 = b0 + b5;
          final int c1 = b1 + b4;
          final int c2 = b0 - b5;
          final int c3 = b1 - b4;
         
          final int di = dstIdx + i;
          output[di]     = ((W0  *c0)  + (W1  *c1)  + round) >> shift;
          output[di+16]  = ((W16 *a2)  + (W17 *a3)  + (W18 *a6)  + (W19 *a7)  + 
                           (W20 *a10) + (W21 *a11) + (W22 *a14) + (W23 *a15) + round) >> shift;
          output[di+32]  = ((W32 *b2)  + (W33 *b3)  + (W34 *b6)  + (W35 *b7)  + round) >> shift;
          output[di+48]  = ((W48 *a2)  + (W49 *a3)  + (W50 *a6)  + (W51 *a7)  + 
                           (W52 *a10) + (W53 *a11) + (W54 *a14) + (W55 *a15) + round) >> shift;
          output[di+64]  = ((W64 *c2)  + (W65 *c3)  + round) >> shift;
          output[di+80]  = ((W80 *a2)  + (W81 *a3)  + (W82 *a6)  + (W83 *a7)  + 
                           (W84 *a10) + (W85 *a11) + (W86 *a14) + (W87 *a15) + round) >> shift;
          output[di+96]  = ((W96 *b2)  + (W97 *b3)  + (W98 *b6)  + (W99 *b7)  + round) >> shift;
          output[di+112] = ((W112*a2)  + (W113*a3)  + (W114*a6)  + (W115*a7)  + 
                           (W116*a10) + (W117*a11) + (W118*a14) + (W119*a15) + round) >> shift;
          output[di+128] = ((W128*c0)  + (W129*c1)  + round) >> shift;
          output[di+144] = ((W144*a2)  + (W145*a3)  + (W146*a6)  + (W147*a7)  + 
                           (W148*a10) + (W149*a11) + (W150*a14) + (W151*a15) + round) >> shift;
          output[di+160] = ((W160*b2)  + (W161*b3)  + (W162*b6)  + (W163*b7)  + round) >> shift;
          output[di+176] = ((W176*a2)  + (W177*a3)  + (W178*a6)  + (W179*a7)  + 
                           (W180*a10) + (W181*a11) + (W182*a14) + (W183*a15) + round) >> shift;
          output[di+192] = ((W192*c2)  + (W193*c3)  + round) >> shift;
          output[di+208] = ((W208*a2)  + (W209*a3)  + (W210*a6)  + (W211*a7)  + 
                           (W212*a10) + (W213*a11) + (W214*a14) + (W215*a15) + round) >> shift;
          output[di+224] = ((W224*b2)  + (W225*b3)  + (W226*b6)  + (W227*b7)  + round) >> shift;
          output[di+240] = ((W240*a2)  + (W241*a3)  + (W242*a6)  + (W243*a7)  + 
                           (W244*a10) + (W245*a11) + (W246*a14) + (W247*a15) + round) >> shift;
       }
    }


    @Override
    public boolean inverse(SliceIntArray src, SliceIntArray dst)
    {
       if (!SliceIntArray.isValid(src))
          return false;

       if (src.length != 256)
          return false;
       
       if (src != dst)
       {
          if (!SliceIntArray.isValid(dst))
            return false;

          if (dst.index + 256 > dst.array.length)
            return false;   
       } 
       
       this.data.index = 0;
       computeInverse(src, this.data, 10);
       computeInverse(this.data, dst, this.iShift-10);
       src.index += 256;
       dst.index += 256;
       return true;
    }
    
    
    private static void computeInverse(SliceIntArray src, SliceIntArray dst, int shift)
    {
       final int[] input = src.array;
       final int[] output = dst.array;
       final int srcIdx = src.index;
       final int dstIdx = dst.index;        
       final int round = (1 << shift) >> 1;

       for (int i=0; i<16; i++)
       {
          final int si = srcIdx + i;          
          final int x0  = input[si];
          final int x1  = input[si+16];
          final int x2  = input[si+32];
          final int x3  = input[si+48];
          final int x4  = input[si+64];
          final int x5  = input[si+80];
          final int x6  = input[si+96];
          final int x7  = input[si+112];
          final int x8  = input[si+128];
          final int x9  = input[si+144];
          final int x10 = input[si+160];
          final int x11 = input[si+176];
          final int x12 = input[si+192];
          final int x13 = input[si+208];
          final int x14 = input[si+224];
          final int x15 = input[si+240];
          
          final int a0 = (W16 *x1) + (W48 *x3)  + (W80 *x5)  + (W112*x7) +
                         (W144*x9) + (W176*x11) + (W208*x13) + (W240*x15);
          final int a1 = (W17 *x1) + (W49 *x3)  + (W81 *x5)  + (W113*x7) +
                         (W145*x9) + (W177*x11) + (W209*x13) + (W241*x15);
          final int a2 = (W18 *x1) + (W50 *x3)  + (W82 *x5)  + (W114*x7) +
                         (W146*x9) + (W178*x11) + (W210*x13) + (W242*x15);
          final int a3 = (W19 *x1) + (W51 *x3)  + (W83 *x5)  + (W115*x7) +
                         (W147*x9) + (W179*x11) + (W211*x13) + (W243*x15);
          final int a4 = (W20 *x1) + (W52 *x3)  + (W84 *x5)  + (W116*x7) +
                         (W148*x9) + (W180*x11) + (W212*x13) + (W244*x15);
          final int a5 = (W21 *x1) + (W53 *x3)  + (W85 *x5)  + (W117*x7) +
                         (W149*x9) + (W181*x11) + (W213*x13) + (W245*x15);
          final int a6 = (W22 *x1) + (W54 *x3)  + (W86 *x5)  + (W118*x7) +
                         (W150*x9) + (W182*x11) + (W214*x13) + (W246*x15);
          final int a7 = (W23 *x1) + (W55 *x3)  + (W87 *x5)  + (W119*x7) +
                         (W151*x9) + (W183*x11) + (W215*x13) + (W247*x15);
          
          final int b0 = (W32*x2) + (W96*x6)  + (W160*x10) + (W224*x14);
          final int b1 = (W33*x2) + (W97*x6)  + (W161*x10) + (W225*x14);
          final int b2 = (W34*x2) + (W98*x6)  + (W162*x10) + (W226*x14);
          final int b3 = (W35*x2) + (W99*x6)  + (W163*x10) + (W227*x14);
          final int b4 = (W0*x0)  + (W128*x8) + (W64*x4)   + (W192*x12);
          final int b5 = (W0*x0)  + (W128*x8) - (W64*x4)   - (W192*x12);
          final int b6 = (W1*x0)  + (W129*x8) + (W65*x4)   + (W193*x12);
          final int b7 = (W1*x0)  + (W129*x8) - (W65*x4)   - (W193*x12);

          final int c0 = b4 + b0;
          final int c1 = b6 + b1;
          final int c2 = b7 + b2;
          final int c3 = b5 + b3;
          final int c4 = b5 - b3;
          final int c5 = b7 - b2;
          final int c6 = b6 - b1;
          final int c7 = b4 - b0;

          final int d0  = (c0 + a0 + round) >> shift;
          final int d1  = (c1 + a1 + round) >> shift;
          final int d2  = (c2 + a2 + round) >> shift;
          final int d3  = (c3 + a3 + round) >> shift;
          final int d4  = (c4 + a4 + round) >> shift;
          final int d5  = (c5 + a5 + round) >> shift;
          final int d6  = (c6 + a6 + round) >> shift;
          final int d7  = (c7 + a7 + round) >> shift;
          final int d8  = (c7 - a7 + round) >> shift;
          final int d9  = (c6 - a6 + round) >> shift;
          final int d10 = (c5 - a5 + round) >> shift;
          final int d11 = (c4 - a4 + round) >> shift;
          final int d12 = (c3 - a3 + round) >> shift;
          final int d13 = (c2 - a2 + round) >> shift;
          final int d14 = (c1 - a1 + round) >> shift;
          final int d15 = (c0 - a0 + round) >> shift;

          final int di = dstIdx + (i << 4);
          output[di]    = (d0  >= MAX_VAL) ? MAX_VAL : ((d0  <= MIN_VAL) ? MIN_VAL : d0);
          output[di+1]  = (d1  >= MAX_VAL) ? MAX_VAL : ((d1  <= MIN_VAL) ? MIN_VAL : d1);
          output[di+2]  = (d2  >= MAX_VAL) ? MAX_VAL : ((d2  <= MIN_VAL) ? MIN_VAL : d2);
          output[di+3]  = (d3  >= MAX_VAL) ? MAX_VAL : ((d3  <= MIN_VAL) ? MIN_VAL : d3);
          output[di+4]  = (d4  >= MAX_VAL) ? MAX_VAL : ((d4  <= MIN_VAL) ? MIN_VAL : d4);
          output[di+5]  = (d5  >= MAX_VAL) ? MAX_VAL : ((d5  <= MIN_VAL) ? MIN_VAL : d5);
          output[di+6]  = (d6  >= MAX_VAL) ? MAX_VAL : ((d6  <= MIN_VAL) ? MIN_VAL : d6);
          output[di+7]  = (d7  >= MAX_VAL) ? MAX_VAL : ((d7  <= MIN_VAL) ? MIN_VAL : d7);
          output[di+8]  = (d8  >= MAX_VAL) ? MAX_VAL : ((d8  <= MIN_VAL) ? MIN_VAL : d8);
          output[di+9]  = (d9  >= MAX_VAL) ? MAX_VAL : ((d9  <= MIN_VAL) ? MIN_VAL : d9);
          output[di+10] = (d10 >= MAX_VAL) ? MAX_VAL : ((d10 <= MIN_VAL) ? MIN_VAL : d10);
          output[di+11] = (d11 >= MAX_VAL) ? MAX_VAL : ((d11 <= MIN_VAL) ? MIN_VAL : d11);
          output[di+12] = (d12 >= MAX_VAL) ? MAX_VAL : ((d12 <= MIN_VAL) ? MIN_VAL : d12);
          output[di+13] = (d13 >= MAX_VAL) ? MAX_VAL : ((d13 <= MIN_VAL) ? MIN_VAL : d13);
          output[di+14] = (d14 >= MAX_VAL) ? MAX_VAL : ((d14 <= MIN_VAL) ? MIN_VAL : d14);
          output[di+15] = (d15 >= MAX_VAL) ? MAX_VAL : ((d15 <= MIN_VAL) ? MIN_VAL : d15);
       }
    }
}