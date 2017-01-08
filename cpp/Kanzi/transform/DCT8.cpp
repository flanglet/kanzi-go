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

#include "DCT8.hpp"

using namespace kanzi;

DCT8::DCT8()
{
   _fShift = 10;
   _iShift = 20;
}


bool DCT8::forward(SliceArray<int>& input, SliceArray<int>& output, int length)
{
    if (length != 64)
       return false;
    
    if (!SliceArray<int>::isValid(input))
       return false;

    if (&input != &output)
    {
       if (!SliceArray<int>::isValid(output))
         return false;

       if (output._index + 64 > output._length)
         return false;   
    }

    computeForward(&input._array[input._index], _data, 4);
    computeForward(_data, &output._array[output._index], _fShift - 4);
    input._index += 256;
    output._index += 256;
    return true;
}
    
    
void DCT8::computeForward(int input[], int output[], const int shift)
{       
   const int round = (1 << shift) >> 1;
   
   for (int i=0; i<8; i++)
   {
      const int si = i << 3;
      const int x0  = input[si];
      const int x1  = input[si+1];
      const int x2  = input[si+2];
      const int x3  = input[si+3];
      const int x4  = input[si+4];
      const int x5  = input[si+5];
      const int x6  = input[si+6];
      const int x7  = input[si+7];
   
      const int a0 = x0 + x7;
      const int a1 = x1 + x6;
      const int a2 = x0 - x7;
      const int a3 = x1 - x6;
      const int a4 = x2 + x5;
      const int a5 = x3 + x4;
      const int a6 = x2 - x5;
      const int a7 = x3 - x4;

      const int b0 = a0 + a5;
      const int b1 = a1 + a4;
      const int b2 = a0 - a5;
      const int b3 = a1 - a4;
      
      const int di = i;
      output[di]    = ((W0* b0) + (W1 *b1) + round) >> shift;
      output[di+8]  = ((W8* a2) + (W9 *a3) + (W10*a6) + (W11*a7) + round) >> shift;
      output[di+16] = ((W16*b2) + (W17*b3) + round) >> shift;
      output[di+24] = ((W24*a2) + (W25*a3) + (W26*a6) + (W27*a7) + round) >> shift;
      output[di+32] = ((W32*b0) + (W33*b1) + round) >> shift;
      output[di+40] = ((W40*a2) + (W41*a3) + (W42*a6) + (W43*a7) + round) >> shift;
      output[di+48] = ((W48*b2) + (W49*b3) + round) >> shift;
      output[di+56] = ((W56*a2) + (W57*a3) + (W58*a6) + (W59*a7) + round) >> shift;
   }
}


bool DCT8::inverse(SliceArray<int>& input, SliceArray<int>& output, int length)
{
    if (length != 64)
       return false;
    
    if (!SliceArray<int>::isValid(input))
       return false;

    if (&input != &output)
    {
       if (!SliceArray<int>::isValid(output))
         return false;

       if (output._index + 64 > output._length)
         return false;   
    }

    computeInverse(&input._array[input._index], _data, 10);
    computeInverse(_data, &output._array[output._index], _iShift - 10);
    input._index += 64;
    output._index += 64;
    return true;
}
    
    
 void DCT8::computeInverse(int input[], int output[], const int shift)
 {       
   const int round = (1 << shift) >> 1;

   for (int i=0; i<8; i++)
   {
      const int si = i;
      const int x0 = input[si];
      const int x1 = input[si+8];
      const int x2 = input[si+16];
      const int x3 = input[si+24];
      const int x4 = input[si+32];
      const int x5 = input[si+40];
      const int x6 = input[si+48];
      const int x7 = input[si+56];
      
      const int a0 = (W8 *x1) + (W24*x3) + (W40*x5) + (W56*x7);
      const int a1 = (W9 *x1) + (W25*x3) + (W41*x5) + (W57*x7);
      const int a2 = (W10*x1) + (W26*x3) + (W42*x5) + (W58*x7);
      const int a3 = (W11*x1) + (W27*x3) + (W43*x5) + (W59*x7);
      const int a4 = (W16*x2) + (W48*x6);
      const int a5 = (W17*x2) + (W49*x6);
      const int a6 = (W0 *x0) + (W32*x4);
      const int a7 = (W1 *x0) + (W33*x4);

      const int b0 = a6 + a4;
      const int b1 = a7 + a5;
      const int b2 = a6 - a4;
      const int b3 = a7 - a5;

      const int c0 = (b0 + a0 + round) >> shift;
      const int c1 = (b1 + a1 + round) >> shift;
      const int c2 = (b3 + a2 + round) >> shift;
      const int c3 = (b2 + a3 + round) >> shift;
      const int c4 = (b2 - a3 + round) >> shift;
      const int c5 = (b3 - a2 + round) >> shift;
      const int c6 = (b1 - a1 + round) >> shift;
      const int c7 = (b0 - a0 + round) >> shift;
      
      const int di = i << 3;
      output[di]   = (c0 >= MAX_VAL) ? MAX_VAL : ((c0 <= MIN_VAL) ? MIN_VAL : c0);
      output[di+1] = (c1 >= MAX_VAL) ? MAX_VAL : ((c1 <= MIN_VAL) ? MIN_VAL : c1);
      output[di+2] = (c2 >= MAX_VAL) ? MAX_VAL : ((c2 <= MIN_VAL) ? MIN_VAL : c2);
      output[di+3] = (c3 >= MAX_VAL) ? MAX_VAL : ((c3 <= MIN_VAL) ? MIN_VAL : c3);
      output[di+4] = (c4 >= MAX_VAL) ? MAX_VAL : ((c4 <= MIN_VAL) ? MIN_VAL : c4);
      output[di+5] = (c5 >= MAX_VAL) ? MAX_VAL : ((c5 <= MIN_VAL) ? MIN_VAL : c5);
      output[di+6] = (c6 >= MAX_VAL) ? MAX_VAL : ((c6 <= MIN_VAL) ? MIN_VAL : c6);
      output[di+7] = (c7 >= MAX_VAL) ? MAX_VAL : ((c7 <= MIN_VAL) ? MIN_VAL : c7);
   }
 }