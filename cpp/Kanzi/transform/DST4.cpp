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

#include "DST4.hpp"

using namespace kanzi;

DST4::DST4()
{
    _fShift = 8;
    _iShift = 20;
}

bool DST4::forward(SliceArray<int>& input, SliceArray<int>& output, int length)
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

void DST4::computeForward(int input[], int output[], const int shift)
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
    const int a1 = x1 + x3;
    const int a2 = x0 - x1;
    const int a3 = W74 * x2;
    const int a4 = x4 + x7;
    const int a5 = x5 + x7;
    const int a6 = x4 - x5;
    const int a7 = W74 * x6;
    const int a8 = x8 + x11;
    const int a9 = x9 + x11;
    const int a10 = x8 - x9;
    const int a11 = W74 * x10;
    const int a12 = x12 + x15;
    const int a13 = x13 + x15;
    const int a14 = x12 - x13;
    const int a15 = W74 * x14;

    output[0] = ((W29 * a0) + (W55 * a1) + a3 + round) >> shift;
    output[1] = ((W29 * a4) + (W55 * a5) + a7 + round) >> shift;
    output[2] = ((W29 * a8) + (W55 * a9) + a11 + round) >> shift;
    output[3] = ((W29 * a12) + (W55 * a13) + a15 + round) >> shift;
    output[4] = (W74 * (x0 + x1 - x3) + round) >> shift;
    output[5] = (W74 * (x4 + x5 - x7) + round) >> shift;
    output[6] = (W74 * (x8 + x9 - x11) + round) >> shift;
    output[7] = (W74 * (x12 + x13 - x15) + round) >> shift;
    output[8] = ((W29 * a2) + (W55 * a0) - a3 + round) >> shift;
    output[9] = ((W29 * a6) + (W55 * a4) - a7 + round) >> shift;
    output[10] = ((W29 * a10) + (W55 * a8) - a11 + round) >> shift;
    output[11] = ((W29 * a14) + (W55 * a12) - a15 + round) >> shift;
    output[12] = ((W55 * a2) - (W29 * a1) + a3 + round) >> shift;
    output[13] = ((W55 * a6) - (W29 * a5) + a7 + round) >> shift;
    output[14] = ((W55 * a10) - (W29 * a9) + a11 + round) >> shift;
    output[15] = ((W55 * a14) - (W29 * a13) + a15 + round) >> shift;
}

bool DST4::inverse(SliceArray<int>& input, SliceArray<int>& output, int length)
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

void DST4::computeInverse(int input[], int output[], const int shift)
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

    const int a0 = x0 + x8;
    const int a1 = x8 + x12;
    const int a2 = x0 - x12;
    const int a3 = W74 * x4;
    const int a4 = x1 + x9;
    const int a5 = x9 + x13;
    const int a6 = x1 - x13;
    const int a7 = W74 * x5;
    const int a8 = x2 + x10;
    const int a9 = x10 + x14;
    const int a10 = x2 - x14;
    const int a11 = W74 * x6;
    const int a12 = x3 + x11;
    const int a13 = x11 + x15;
    const int a14 = x3 - x15;
    const int a15 = W74 * x7;

    const int b0 = ((W29 * a0) + (W55 * a1) + a3 + round) >> shift;
    const int b1 = ((W55 * a2) - (W29 * a1) + a3 + round) >> shift;
    const int b2 = (W74 * (x0 - x8 + x12) + round) >> shift;
    const int b3 = ((W55 * a0) + (W29 * a2) - a3 + round) >> shift;
    const int b4 = ((W29 * a4) + (W55 * a5) + a7 + round) >> shift;
    const int b5 = ((W55 * a6) - (W29 * a5) + a7 + round) >> shift;
    const int b6 = (W74 * (x1 - x9 + x13) + round) >> shift;
    const int b7 = ((W55 * a4) + (W29 * a6) - a7 + round) >> shift;
    const int b8 = ((W29 * a8) + (W55 * a9) + a11 + round) >> shift;
    const int b9 = ((W55 * a10) - (W29 * a9) + a11 + round) >> shift;
    const int b10 = (W74 * (x2 - x10 + x14) + round) >> shift;
    const int b11 = ((W55 * a8) + (W29 * a10) - a11 + round) >> shift;
    const int b12 = ((W29 * a12) + (W55 * a13) + a15 + round) >> shift;
    const int b13 = ((W55 * a14) - (W29 * a13) + a15 + round) >> shift;
    const int b14 = (W74 * (x3 - x11 + x15) + round) >> shift;
    const int b15 = ((W55 * a12) + (W29 * a14) - a15 + round) >> shift;

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