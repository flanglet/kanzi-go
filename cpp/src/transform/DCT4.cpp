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

#include "DCT4.hpp"

using namespace kanzi;

DCT4::DCT4()
{
    _fShift = 8;
    _iShift = 20;
}

bool DCT4::forward(SliceArray<int>& input, SliceArray<int>& output, int length)
{
    if (length != 16)
       return false;
    
    if (!SliceArray<int>::isValid(input))
       return false;

    if (&input != &output)
    {
       if (!SliceArray<int>::isValid(output))
         return false;

       if (output._index + 16 > output._length)
         return false;   
    }

    computeForward(&input._array[input._index], _data, 4);
    computeForward(_data, &output._array[output._index], _fShift - 4);
    input._index += 16;
    output._index += 16;
    return true;
}

void DCT4::computeForward(int input[], int output[], const int shift)
{
    const int round = (1 << shift) >> 1;

    const int x0 = input[0];
    const int x1 = input[1];
    const int x2 = input[2];
    const int x3 = input[3];
    const int x4 = input[4];
    const int x5 = input[5];
    const int x6 = input[6];
    const int x7 = input[7];
    const int x8 = input[8];
    const int x9 = input[9];
    const int x10 = input[10];
    const int x11 = input[11];
    const int x12 = input[12];
    const int x13 = input[13];
    const int x14 = input[14];
    const int x15 = input[15];

    const int a0 = x0 + x3;
    const int a1 = x1 + x2;
    const int a2 = x0 - x3;
    const int a3 = x1 - x2;
    const int a4 = x4 + x7;
    const int a5 = x5 + x6;
    const int a6 = x4 - x7;
    const int a7 = x5 - x6;
    const int a8 = x8 + x11;
    const int a9 = x9 + x10;
    const int a10 = x8 - x11;
    const int a11 = x9 - x10;
    const int a12 = x12 + x15;
    const int a13 = x13 + x14;
    const int a14 = x12 - x15;
    const int a15 = x13 - x14;

    output[0] = ((W0 * a0) + (W1 * a1) + round) >> shift;
    output[1] = ((W0 * a4) + (W1 * a5) + round) >> shift;
    output[2] = ((W0 * a8) + (W1 * a9) + round) >> shift;
    output[3] = ((W0 * a12) + (W1 * a13) + round) >> shift;
    output[4] = ((W4 * a2) + (W5 * a3) + round) >> shift;
    output[5] = ((W4 * a6) + (W5 * a7) + round) >> shift;
    output[6] = ((W4 * a10) + (W5 * a11) + round) >> shift;
    output[7] = ((W4 * a14) + (W5 * a15) + round) >> shift;
    output[8] = ((W8 * a0) + (W9 * a1) + round) >> shift;
    output[9] = ((W8 * a4) + (W9 * a5) + round) >> shift;
    output[10] = ((W8 * a8) + (W9 * a9) + round) >> shift;
    output[11] = ((W8 * a12) + (W9 * a13) + round) >> shift;
    output[12] = ((W12 * a2) + (W13 * a3) + round) >> shift;
    output[13] = ((W12 * a6) + (W13 * a7) + round) >> shift;
    output[14] = ((W12 * a10) + (W13 * a11) + round) >> shift;
    output[15] = ((W12 * a14) + (W13 * a15) + round) >> shift;
}

bool DCT4::inverse(SliceArray<int>& input, SliceArray<int>& output, int length)
{
    if (length != 16)
       return false;
    
    if (!SliceArray<int>::isValid(input))
       return false;

    if (&input != &output)
    {
       if (!SliceArray<int>::isValid(output))
         return false;

       if (output._index + 16 > output._length)
         return false;   
    }

    computeInverse(&input._array[input._index], _data, 10);
    computeInverse(_data, &output._array[output._index], _iShift - 10);
    input._index += 16;
    output._index += 16;
    return true;
}

void DCT4::computeInverse(int input[], int output[], const int shift)
{
    const int round = (1 << shift) >> 1;

    const int x0 = input[0];
    const int x1 = input[1];
    const int x2 = input[2];
    const int x3 = input[3];
    const int x4 = input[4];
    const int x5 = input[5];
    const int x6 = input[6];
    const int x7 = input[7];
    const int x8 = input[8];
    const int x9 = input[9];
    const int x10 = input[10];
    const int x11 = input[11];
    const int x12 = input[12];
    const int x13 = input[13];
    const int x14 = input[14];
    const int x15 = input[15];

    const int a0 = (W4 * x4) + (W12 * x12);
    const int a1 = (W5 * x4) + (W13 * x12);
    const int a2 = (W0 * x0) + (W8 * x8);
    const int a3 = (W1 * x0) + (W9 * x8);
    const int a4 = (W4 * x5) + (W12 * x13);
    const int a5 = (W5 * x5) + (W13 * x13);
    const int a6 = (W0 * x1) + (W8 * x9);
    const int a7 = (W1 * x1) + (W9 * x9);
    const int a8 = (W4 * x6) + (W12 * x14);
    const int a9 = (W5 * x6) + (W13 * x14);
    const int a10 = (W0 * x2) + (W8 * x10);
    const int a11 = (W1 * x2) + (W9 * x10);
    const int a12 = (W4 * x7) + (W12 * x15);
    const int a13 = (W5 * x7) + (W13 * x15);
    const int a14 = (W0 * x3) + (W8 * x11);
    const int a15 = (W1 * x3) + (W9 * x11);

    const int b0 = (a2 + a0 + round) >> shift;
    const int b1 = (a3 + a1 + round) >> shift;
    const int b2 = (a3 - a1 + round) >> shift;
    const int b3 = (a2 - a0 + round) >> shift;
    const int b4 = (a6 + a4 + round) >> shift;
    const int b5 = (a7 + a5 + round) >> shift;
    const int b6 = (a7 - a5 + round) >> shift;
    const int b7 = (a6 - a4 + round) >> shift;
    const int b8 = (a10 + a8 + round) >> shift;
    const int b9 = (a11 + a9 + round) >> shift;
    const int b10 = (a11 - a9 + round) >> shift;
    const int b11 = (a10 - a8 + round) >> shift;
    const int b12 = (a14 + a12 + round) >> shift;
    const int b13 = (a15 + a13 + round) >> shift;
    const int b14 = (a15 - a13 + round) >> shift;
    const int b15 = (a14 - a12 + round) >> shift;

    output[0] = (b0 >= MAX_VAL) ? MAX_VAL : ((b0 <= MIN_VAL) ? MIN_VAL : b0);
    output[1] = (b1 >= MAX_VAL) ? MAX_VAL : ((b1 <= MIN_VAL) ? MIN_VAL : b1);
    output[2] = (b2 >= MAX_VAL) ? MAX_VAL : ((b2 <= MIN_VAL) ? MIN_VAL : b2);
    output[3] = (b3 >= MAX_VAL) ? MAX_VAL : ((b3 <= MIN_VAL) ? MIN_VAL : b3);
    output[4] = (b4 >= MAX_VAL) ? MAX_VAL : ((b4 <= MIN_VAL) ? MIN_VAL : b4);
    output[5] = (b5 >= MAX_VAL) ? MAX_VAL : ((b5 <= MIN_VAL) ? MIN_VAL : b5);
    output[6] = (b6 >= MAX_VAL) ? MAX_VAL : ((b6 <= MIN_VAL) ? MIN_VAL : b6);
    output[7] = (b7 >= MAX_VAL) ? MAX_VAL : ((b7 <= MIN_VAL) ? MIN_VAL : b7);
    output[8] = (b8 >= MAX_VAL) ? MAX_VAL : ((b8 <= MIN_VAL) ? MIN_VAL : b8);
    output[9] = (b9 >= MAX_VAL) ? MAX_VAL : ((b9 <= MIN_VAL) ? MIN_VAL : b9);
    output[10] = (b10 >= MAX_VAL) ? MAX_VAL : ((b10 <= MIN_VAL) ? MIN_VAL : b10);
    output[11] = (b11 >= MAX_VAL) ? MAX_VAL : ((b11 <= MIN_VAL) ? MIN_VAL : b11);
    output[12] = (b12 >= MAX_VAL) ? MAX_VAL : ((b12 <= MIN_VAL) ? MIN_VAL : b12);
    output[13] = (b13 >= MAX_VAL) ? MAX_VAL : ((b13 <= MIN_VAL) ? MIN_VAL : b13);
    output[14] = (b14 >= MAX_VAL) ? MAX_VAL : ((b14 <= MIN_VAL) ? MIN_VAL : b14);
    output[15] = (b15 >= MAX_VAL) ? MAX_VAL : ((b15 <= MIN_VAL) ? MIN_VAL : b15);
}