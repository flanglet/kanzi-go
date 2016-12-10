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


// Implementation of Walsh-Hadamard transform of dimension 8 using only additions,
// subtractions and shifts.
public final class WHT8 implements IntTransform
{
    private final int[] data;
    private final int fScale;
    private final int iScale;


    // For perfect reconstruction, forward results are scaled by 8*sqrt(2)
    public WHT8()
    {
       this.fScale = 0;
       this.iScale = 6;
       this.data = new int[64];
    }


    // For perfect reconstruction, forward results are scaled by 8*sqrt(2) unless 
    // the parameter is set to false (scaled by sqrt(2), in wich case rounding
    // may introduce errors)
    public WHT8(boolean scale)
    {
       this.fScale = (scale == false) ? 3 : 0;
       this.iScale = (scale == false) ? 3 : 6;
       this.data = new int[64];
    }


    // Not thread safe
    @Override
    public boolean forward(SliceIntArray src, SliceIntArray dst)
    {
       if (!SliceIntArray.isValid(src))
          return false;

       if (src.length != 64)
          return false;
       
       if (src != dst)
       {
          if (!SliceIntArray.isValid(dst))
            return false;

          if (dst.index + 64 > dst.array.length)
            return false;   
       }
       
       return compute(src, dst, this.data, this.fScale);
    }


    // Not thread safe
    // Result multiplied by sqrt(2) or 8*sqrt(2) if 'scale' is set to false
    private static boolean compute(SliceIntArray src, SliceIntArray dst, int[] buffer, int shift)
    {
        final int[] input = src.array;
        final int[] output = dst.array;
        final int srcIdx = src.index;
        final int dstIdx = dst.index;        
        int dataptr = 0;

        // Pass 1: process rows.
        for (int i=0; i<64; i+=8)
        {
            // Aliasing for speed
            final int si = srcIdx + i;
            final int x0 = input[si];
            final int x1 = input[si+1];
            final int x2 = input[si+2];
            final int x3 = input[si+3];
            final int x4 = input[si+4];
            final int x5 = input[si+5];
            final int x6 = input[si+6];
            final int x7 = input[si+7];

            final int a0 = x0 + x1;
            final int a1 = x2 + x3;
            final int a2 = x4 + x5;
            final int a3 = x6 + x7;
            final int a4 = x0 - x1;
            final int a5 = x2 - x3;
            final int a6 = x4 - x5;
            final int a7 = x6 - x7;

            final int b0 = a0 + a1;
            final int b1 = a2 + a3;
            final int b2 = a4 + a5;
            final int b3 = a6 + a7;
            final int b4 = a0 - a1;
            final int b5 = a2 - a3;
            final int b6 = a4 - a5;
            final int b7 = a6 - a7;

            buffer[dataptr]   = b0 + b1;
            buffer[dataptr+1] = b2 + b3;
            buffer[dataptr+2] = b4 + b5;
            buffer[dataptr+3] = b6 + b7;
            buffer[dataptr+4] = b0 - b1;
            buffer[dataptr+5] = b2 - b3;
            buffer[dataptr+6] = b4 - b5;
            buffer[dataptr+7] = b6 - b7;

            dataptr += 8;
        }

        dataptr = 0;
        final int adjust = (1 << shift) >> 1;

        // Pass 2: process columns.
        for (int i=0; i<8; i++)
        {
            // Aliasing for speed
            final int x0 = buffer[dataptr];
            final int x1 = buffer[dataptr+8];
            final int x2 = buffer[dataptr+16];
            final int x3 = buffer[dataptr+24];
            final int x4 = buffer[dataptr+32];
            final int x5 = buffer[dataptr+40];
            final int x6 = buffer[dataptr+48];
            final int x7 = buffer[dataptr+56];

            final int a0 = x0 + x1;
            final int a1 = x2 + x3;
            final int a2 = x4 + x5;
            final int a3 = x6 + x7;
            final int a4 = x0 - x1;
            final int a5 = x2 - x3;
            final int a6 = x4 - x5;
            final int a7 = x6 - x7;

            final int b0 = a0 + a1;
            final int b1 = a2 + a3;
            final int b2 = a4 + a5;
            final int b3 = a6 + a7;
            final int b4 = a0 - a1;
            final int b5 = a2 - a3;
            final int b6 = a4 - a5;
            final int b7 = a6 - a7;

            final int di = dstIdx + i;
            output[di]    = (b0 + b1 + adjust) >> shift;
            output[di+8]  = (b2 + b3 + adjust) >> shift;
            output[di+16] = (b4 + b5 + adjust) >> shift;
            output[di+24] = (b6 + b7 + adjust) >> shift;
            output[di+32] = (b0 - b1 + adjust) >> shift;
            output[di+40] = (b2 - b3 + adjust) >> shift;
            output[di+48] = (b4 - b5 + adjust) >> shift;
            output[di+56] = (b6 - b7 + adjust) >> shift;

            dataptr++;
        }

        src.index += 64;
        dst.index += 64;
        return true;
    }


    @Override
    public boolean inverse(SliceIntArray src, SliceIntArray dst)
    {
       if (!SliceIntArray.isValid(src))
          return false;

       if (src.length != 64)
          return false;
       
       if (src != dst)
       {
          if (!SliceIntArray.isValid(dst))
            return false;

          if (dst.index + 64 > dst.array.length)
            return false;   
       }
       
       return compute(src, dst, this.data, this.iScale);
    }

}