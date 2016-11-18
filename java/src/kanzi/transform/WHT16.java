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


// Implementation of Walsh-Hadamard transform of dimension 16 using only additions,
// subtractions and shifts.
public final class WHT16 implements IntTransform
{
    private final int[] data;
    private final int fScale;
    private final int iScale;


    // For perfect reconstruction, forward results are scaled by 16
    public WHT16()
    {
       this.fScale = 0;
       this.iScale = 8;
       this.data = new int[256];
    }


    // For perfect reconstruction, forward results are scaled by 16 unless the
    // parameter is set to false (in wich case rounding may introduce errors)
    public WHT16(boolean scale)
    {
       this.fScale = (scale == false) ? 4 : 0;
       this.iScale = (scale == false) ? 4 : 8;
       this.data = new int[256];
    }
    

    // Not thread safe
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
       
       return compute(src, dst, this.data, this.fScale);
    }


    // Not thread safe
    // Result multiplied by 16 if 'scale' is set to false
    private static boolean compute(SliceIntArray src, SliceIntArray dst, int[] buffer, int shift)
    {
        final int[] input = src.array;
        final int[] output = dst.array;
        final int srcIdx = src.index;
        final int dstIdx = dst.index;        
        int dataptr = 0;

        // Pass 1: process rows.
        for (int i=0; i<256; i+=16)
        {
            // Aliasing for speed
            final int si = srcIdx + i;
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

            int a0  = x0  + x1;
            int a1  = x2  + x3;
            int a2  = x4  + x5;
            int a3  = x6  + x7;
            int a4  = x8  + x9;
            int a5  = x10 + x11;
            int a6  = x12 + x13;
            int a7  = x14 + x15;
            int a8  = x0  - x1;
            int a9  = x2  - x3;
            int a10 = x4  - x5;
            int a11 = x6  - x7;
            int a12 = x8  - x9;
            int a13 = x10 - x11;
            int a14 = x12 - x13;
            int a15 = x14 - x15;

            final int b0  = a0  + a1;
            final int b1  = a2  + a3;
            final int b2  = a4  + a5;
            final int b3  = a6  + a7;
            final int b4  = a8  + a9;
            final int b5  = a10 + a11;
            final int b6  = a12 + a13;
            final int b7  = a14 + a15;
            final int b8  = a0  - a1;
            final int b9  = a2  - a3;
            final int b10 = a4  - a5;
            final int b11 = a6  - a7;
            final int b12 = a8  - a9;
            final int b13 = a10 - a11;
            final int b14 = a12 - a13;
            final int b15 = a14 - a15;

            a0  = b0  + b1;
            a1  = b2  + b3;
            a2  = b4  + b5;
            a3  = b6  + b7;
            a4  = b8  + b9;
            a5  = b10 + b11;
            a6  = b12 + b13;
            a7  = b14 + b15;
            a8  = b0  - b1;
            a9  = b2  - b3;
            a10 = b4  - b5;
            a11 = b6  - b7;
            a12 = b8  - b9;
            a13 = b10 - b11;
            a14 = b12 - b13;
            a15 = b14 - b15;

            buffer[dataptr]    = a0  + a1;
            buffer[dataptr+1]  = a2  + a3;
            buffer[dataptr+2]  = a4  + a5;
            buffer[dataptr+3]  = a6  + a7;
            buffer[dataptr+4]  = a8  + a9;
            buffer[dataptr+5]  = a10 + a11;
            buffer[dataptr+6]  = a12 + a13;
            buffer[dataptr+7]  = a14 + a15;
            buffer[dataptr+8]  = a0  - a1;
            buffer[dataptr+9]  = a2  - a3;
            buffer[dataptr+10] = a4  - a5;
            buffer[dataptr+11] = a6  - a7;
            buffer[dataptr+12] = a8  - a9;
            buffer[dataptr+13] = a10 - a11;
            buffer[dataptr+14] = a12 - a13;
            buffer[dataptr+15] = a14 - a15;

            dataptr += 16;
        }

        dataptr = 0;
        final int adjust = (1 << shift) >> 1;

        // Pass 2: process columns.
        for (int i=0; i<16; i++)
        {
            // Aliasing for speed
            final int x0  = buffer[dataptr];
            final int x1  = buffer[dataptr+16];
            final int x2  = buffer[dataptr+32];
            final int x3  = buffer[dataptr+48];
            final int x4  = buffer[dataptr+64];
            final int x5  = buffer[dataptr+80];
            final int x6  = buffer[dataptr+96];
            final int x7  = buffer[dataptr+112];
            final int x8  = buffer[dataptr+128];
            final int x9  = buffer[dataptr+144];
            final int x10 = buffer[dataptr+160];
            final int x11 = buffer[dataptr+176];
            final int x12 = buffer[dataptr+192];
            final int x13 = buffer[dataptr+208];
            final int x14 = buffer[dataptr+224];
            final int x15 = buffer[dataptr+240];

            int a0  = x0  + x1;
            int a1  = x2  + x3;
            int a2  = x4  + x5;
            int a3  = x6  + x7;
            int a4  = x8  + x9;
            int a5  = x10 + x11;
            int a6  = x12 + x13;
            int a7  = x14 + x15;
            int a8  = x0  - x1;
            int a9  = x2  - x3;
            int a10 = x4  - x5;
            int a11 = x6  - x7;
            int a12 = x8  - x9;
            int a13 = x10 - x11;
            int a14 = x12 - x13;
            int a15 = x14 - x15;

            final int b0  = a0  + a1;
            final int b1  = a2  + a3;
            final int b2  = a4  + a5;
            final int b3  = a6  + a7;
            final int b4  = a8  + a9;
            final int b5  = a10 + a11;
            final int b6  = a12 + a13;
            final int b7  = a14 + a15;
            final int b8  = a0  - a1;
            final int b9  = a2  - a3;
            final int b10 = a4  - a5;
            final int b11 = a6  - a7;
            final int b12 = a8  - a9;
            final int b13 = a10 - a11;
            final int b14 = a12 - a13;
            final int b15 = a14 - a15;

            a0  = b0  + b1;
            a1  = b2  + b3;
            a2  = b4  + b5;
            a3  = b6  + b7;
            a4  = b8  + b9;
            a5  = b10 + b11;
            a6  = b12 + b13;
            a7  = b14 + b15;
            a8  = b0  - b1;
            a9  = b2  - b3;
            a10 = b4  - b5;
            a11 = b6  - b7;
            a12 = b8  - b9;
            a13 = b10 - b11;
            a14 = b12 - b13;
            a15 = b14 - b15;

            final int di = dstIdx + i;
            output[di]      = (a0  + a1  + adjust) >> shift;
            output[di+16]   = (a2  + a3  + adjust) >> shift;
            output[di+32]   = (a4  + a5  + adjust) >> shift;
            output[di+48]   = (a6  + a7  + adjust) >> shift;
            output[di+64]   = (a8  + a9  + adjust) >> shift;
            output[di+80]   = (a10 + a11 + adjust) >> shift;
            output[di+96]   = (a12 + a13 + adjust) >> shift;
            output[di+112]  = (a14 + a15 + adjust) >> shift;
            output[di+128]  = (a0  - a1  + adjust) >> shift;
            output[di+144]  = (a2  - a3  + adjust) >> shift;
            output[di+160]  = (a4  - a5  + adjust) >> shift;
            output[di+176]  = (a6  - a7  + adjust) >> shift;
            output[di+192]  = (a8  - a9  + adjust) >> shift;
            output[di+208]  = (a10 - a11 + adjust) >> shift;
            output[di+224]  = (a12 - a13 + adjust) >> shift;
            output[di+240]  = (a14 - a15 + adjust) >> shift;

            dataptr++;
        }

        src.index += 256;
        dst.index += 256;
        return true;
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
       return compute(src, dst, this.data, this.iScale);
    }

}