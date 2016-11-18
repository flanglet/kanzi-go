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


// Implementation of Discrete Sine Transform of dimension 4 
public final class DST4 implements IntTransform
{
    // Weights
    private final static int W29  = 29;
    private final static int W74  = 74;
    private final static int W55  = 55;
  
    private static final int MAX_VAL = 1<<16;
    private static final int MIN_VAL = -(MAX_VAL+1);
            
    private final int fShift;
    private final int iShift;
    private final SliceIntArray data;
 

    public DST4()
    {
       this.fShift = 8;
       this.iShift = 20;
       this.data = new SliceIntArray(new int[16], 0);
    }


    @Override
    public boolean forward(SliceIntArray src, SliceIntArray dst)
    {
       if (!SliceIntArray.isValid(src))
          return false;

       if (src.length != 16)
          return false;
       
       if (src != dst)
       {
          if (!SliceIntArray.isValid(dst))
            return false;

          if (dst.index + 16 > dst.array.length)
            return false;   
       }
       
       this.data.index = 0;
       computeForward(src, this.data, 4);
       computeForward(this.data, dst, this.fShift-4);     
       src.index += 16;
       dst.index += 16;
       return true;
    }
    
   
    private static void computeForward(SliceIntArray src, SliceIntArray dst, int shift)
    {
       final int[] input = src.array;
       final int[] output = dst.array;
       final int srcIdx = src.index;
       final int dstIdx = dst.index;        
       final int round = (1 << shift) >> 1;
       
       final int x0  = input[srcIdx];
       final int x1  = input[srcIdx+1];
       final int x2  = input[srcIdx+2];
       final int x3  = input[srcIdx+3];
       final int x4  = input[srcIdx+4];
       final int x5  = input[srcIdx+5];
       final int x6  = input[srcIdx+6];
       final int x7  = input[srcIdx+7];
       final int x8  = input[srcIdx+8];
       final int x9  = input[srcIdx+9];
       final int x10 = input[srcIdx+10];
       final int x11 = input[srcIdx+11];
       final int x12 = input[srcIdx+12];
       final int x13 = input[srcIdx+13];
       final int x14 = input[srcIdx+14];
       final int x15 = input[srcIdx+15];
       
       final int a0  = x0 + x3;
       final int a1  = x1 + x3;
       final int a2  = x0 - x1;
       final int a3  = W74 * x2;
       final int a4  = x4 + x7;
       final int a5  = x5 + x7;
       final int a6  = x4 - x5;
       final int a7  = W74 * x6;
       final int a8  = x8 + x11;
       final int a9  = x9 + x11;
       final int a10 = x8 - x9;
       final int a11 = W74 * x10;
       final int a12 = x12 + x15;
       final int a13 = x13 + x15;
       final int a14 = x12 - x13;
       final int a15 = W74 * x14;
  
       output[dstIdx]    = ((W29 * a0)  + (W55 * a1)  + a3   + round) >> shift;
       output[dstIdx+1]  = ((W29 * a4)  + (W55 * a5)  + a7   + round) >> shift;
       output[dstIdx+2]  = ((W29 * a8)  + (W55 * a9)  + a11  + round) >> shift;
       output[dstIdx+3]  = ((W29 * a12) + (W55 * a13) + a15  + round) >> shift;
       output[dstIdx+4]  = (W74  * (x0  + x1          - x3)  + round) >> shift;
       output[dstIdx+5]  = (W74  * (x4  + x5          - x7)  + round) >> shift;
       output[dstIdx+6]  = (W74  * (x8  + x9          - x11) + round) >> shift;
       output[dstIdx+7]  = (W74  * (x12 + x13         - x15) + round) >> shift;
       output[dstIdx+8]  = ((W29 * a2)  + (W55 * a0)  - a3   + round) >> shift;
       output[dstIdx+9]  = ((W29 * a6)  + (W55 * a4)  - a7   + round) >> shift;
       output[dstIdx+10] = ((W29 * a10) + (W55 * a8)  - a11  + round) >> shift;
       output[dstIdx+11] = ((W29 * a14) + (W55 * a12) - a15  + round) >> shift;
       output[dstIdx+12] = ((W55 * a2)  - (W29 * a1)  + a3   + round) >> shift;
       output[dstIdx+13] = ((W55 * a6)  - (W29 * a5)  + a7   + round) >> shift;
       output[dstIdx+14] = ((W55 * a10) - (W29 * a9)  + a11  + round) >> shift;
       output[dstIdx+15] = ((W55 * a14) - (W29 * a13) + a15  + round) >> shift;
    }


    @Override
    public boolean inverse(SliceIntArray src, SliceIntArray dst)
    {
       if (!SliceIntArray.isValid(src))
          return false;

       if (src.length != 16)
          return false;
       
       if (src != dst)
       {
          if (!SliceIntArray.isValid(dst))
            return false;

          if (dst.index + 16 > dst.array.length)
            return false;   
       }
       
       this.data.index = 0;
       computeInverse(src, this.data, 10);
       computeInverse(this.data, dst, this.iShift-10);
       src.index += 16;
       dst.index += 16;
       return true;
    }
    
 
    private static void computeInverse(SliceIntArray src, SliceIntArray dst, int shift)
    {
       final int[] input = src.array;
       final int[] output = dst.array;
       final int srcIdx = src.index;
       final int dstIdx = dst.index;        
       final int round = (1 << shift) >> 1;

       final int x0  = input[srcIdx];
       final int x1  = input[srcIdx+1];
       final int x2  = input[srcIdx+2];
       final int x3  = input[srcIdx+3];
       final int x4  = input[srcIdx+4];
       final int x5  = input[srcIdx+5];
       final int x6  = input[srcIdx+6];
       final int x7  = input[srcIdx+7];
       final int x8  = input[srcIdx+8];
       final int x9  = input[srcIdx+9];
       final int x10 = input[srcIdx+10];
       final int x11 = input[srcIdx+11];
       final int x12 = input[srcIdx+12];
       final int x13 = input[srcIdx+13];
       final int x14 = input[srcIdx+14];
       final int x15 = input[srcIdx+15];

       final int a0  = x0 + x8;
       final int a1  = x8 + x12;
       final int a2  = x0 - x12;
       final int a3  = W74 * x4;
       final int a4  = x1 + x9;
       final int a5  = x9 + x13;
       final int a6  = x1 - x13;
       final int a7  = W74 * x5;
       final int a8  = x2  + x10;
       final int a9  = x10 + x14;
       final int a10 = x2  - x14;
       final int a11 = W74  * x6;
       final int a12 = x3  + x11;
       final int a13 = x11 + x15;
       final int a14 = x3  - x15;
       final int a15 = W74  * x7;
       
       final int b0  = ((W29 * a0)  + (W55 * a1)  + a3  + round) >> shift;
       final int b1  = ((W55 * a2)  - (W29 * a1)  + a3  + round) >> shift;
       final int b2  = (W74  * (x0 - x8 + x12)          + round) >> shift;
       final int b3  = ((W55 * a0)  + (W29 * a2)  - a3  + round) >> shift;      
       final int b4  = ((W29 * a4)  + (W55 * a5)  + a7  + round) >> shift;
       final int b5  = ((W55 * a6)  - (W29 * a5)  + a7  + round) >> shift;
       final int b6  = (W74  * (x1 - x9 + x13)          + round) >> shift;
       final int b7  = ((W55 * a4)  + (W29 * a6)  - a7  + round) >> shift;       
       final int b8  = ((W29 * a8)  + (W55 * a9)  + a11 + round) >> shift;
       final int b9  = ((W55 * a10) - (W29 * a9)  + a11 + round) >> shift;
       final int b10 = (W74  * (x2 - x10 + x14)         + round) >> shift;
       final int b11 = ((W55 * a8)  + (W29 * a10) - a11 + round) >> shift;       
       final int b12 = ((W29 * a12) + (W55 * a13) + a15 + round) >> shift;
       final int b13 = ((W55 * a14) - (W29 * a13) + a15 + round) >> shift;
       final int b14 = (W74  * (x3 - x11 + x15)         + round) >> shift;
       final int b15 = ((W55 * a12) + (W29 * a14) - a15 + round) >> shift;
       
       output[dstIdx]    = (b0  >= MAX_VAL) ? MAX_VAL : ((b0  <= MIN_VAL) ? MIN_VAL : b0);
       output[dstIdx+1]  = (b1  >= MAX_VAL) ? MAX_VAL : ((b1  <= MIN_VAL) ? MIN_VAL : b1);
       output[dstIdx+2]  = (b2  >= MAX_VAL) ? MAX_VAL : ((b2  <= MIN_VAL) ? MIN_VAL : b2);
       output[dstIdx+3]  = (b3  >= MAX_VAL) ? MAX_VAL : ((b3  <= MIN_VAL) ? MIN_VAL : b3);
       output[dstIdx+4]  = (b4  >= MAX_VAL) ? MAX_VAL : ((b4  <= MIN_VAL) ? MIN_VAL : b4);
       output[dstIdx+5]  = (b5  >= MAX_VAL) ? MAX_VAL : ((b5  <= MIN_VAL) ? MIN_VAL : b5);
       output[dstIdx+6]  = (b6  >= MAX_VAL) ? MAX_VAL : ((b6  <= MIN_VAL) ? MIN_VAL : b6);
       output[dstIdx+7]  = (b7  >= MAX_VAL) ? MAX_VAL : ((b7  <= MIN_VAL) ? MIN_VAL : b7);
       output[dstIdx+8]  = (b8  >= MAX_VAL) ? MAX_VAL : ((b8  <= MIN_VAL) ? MIN_VAL : b8);
       output[dstIdx+9]  = (b9  >= MAX_VAL) ? MAX_VAL : ((b9  <= MIN_VAL) ? MIN_VAL : b9);
       output[dstIdx+10] = (b10 >= MAX_VAL) ? MAX_VAL : ((b10 <= MIN_VAL) ? MIN_VAL : b10);
       output[dstIdx+11] = (b11 >= MAX_VAL) ? MAX_VAL : ((b11 <= MIN_VAL) ? MIN_VAL : b11);
       output[dstIdx+12] = (b12 >= MAX_VAL) ? MAX_VAL : ((b12 <= MIN_VAL) ? MIN_VAL : b12);
       output[dstIdx+13] = (b13 >= MAX_VAL) ? MAX_VAL : ((b13 <= MIN_VAL) ? MIN_VAL : b13);
       output[dstIdx+14] = (b14 >= MAX_VAL) ? MAX_VAL : ((b14 <= MIN_VAL) ? MIN_VAL : b14);
       output[dstIdx+15] = (b15 >= MAX_VAL) ? MAX_VAL : ((b15 <= MIN_VAL) ? MIN_VAL : b15);
    }

}