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

#include "DCT16.hpp"

using namespace kanzi;

DCT16::DCT16()
{
    _fShift = 12;
    _iShift = 20;
}

bool DCT16::forward(SliceArray<int>& input, SliceArray<int>& output, int length)
{
    if (length != 256)
       return false;
    
    if (!SliceArray<int>::isValid(input))
       return false;

    if (&input != &output)
    {
       if (!SliceArray<int>::isValid(output))
         return false;

       if (output._index + 256 > output._length)
         return false;   
    }

    computeForward(&input._array[input._index], _data, 4);
    computeForward(_data, &output._array[output._index], _fShift - 4);
    input._index += 256;
    output._index += 256;
    return true;
}

void DCT16::computeForward(int input[], int output[], const int shift)
{
    const int round = (1 << shift) >> 1;

    for (int i = 0; i < 16; i++) {
        const int si = i << 4;
        const int x0 = input[si];
        const int x1 = input[si + 1];
        const int x2 = input[si + 2];
        const int x3 = input[si + 3];
        const int x4 = input[si + 4];
        const int x5 = input[si + 5];
        const int x6 = input[si + 6];
        const int x7 = input[si + 7];
        const int x8 = input[si + 8];
        const int x9 = input[si + 9];
        const int x10 = input[si + 10];
        const int x11 = input[si + 11];
        const int x12 = input[si + 12];
        const int x13 = input[si + 13];
        const int x14 = input[si + 14];
        const int x15 = input[si + 15];

        const int a0 = x0 + x15;
        const int a1 = x1 + x14;
        const int a2 = x0 - x15;
        const int a3 = x1 - x14;
        const int a4 = x2 + x13;
        const int a5 = x3 + x12;
        const int a6 = x2 - x13;
        const int a7 = x3 - x12;
        const int a8 = x4 + x11;
        const int a9 = x5 + x10;
        const int a10 = x4 - x11;
        const int a11 = x5 - x10;
        const int a12 = x6 + x9;
        const int a13 = x7 + x8;
        const int a14 = x6 - x9;
        const int a15 = x7 - x8;

        const int b0 = a0 + a13;
        const int b1 = a1 + a12;
        const int b2 = a0 - a13;
        const int b3 = a1 - a12;
        const int b4 = a4 + a9;
        const int b5 = a5 + a8;
        const int b6 = a4 - a9;
        const int b7 = a5 - a8;

        const int c0 = b0 + b5;
        const int c1 = b1 + b4;
        const int c2 = b0 - b5;
        const int c3 = b1 - b4;

        const int di = i;
        output[di] = ((W0 * c0) + (W1 * c1) + round) >> shift;
        output[di + 16] = ((W16 * a2) + (W17 * a3) + (W18 * a6) + (W19 * a7) + (W20 * a10) + (W21 * a11) + (W22 * a14) + (W23 * a15) + round) >> shift;
        output[di + 32] = ((W32 * b2) + (W33 * b3) + (W34 * b6) + (W35 * b7) + round) >> shift;
        output[di + 48] = ((W48 * a2) + (W49 * a3) + (W50 * a6) + (W51 * a7) + (W52 * a10) + (W53 * a11) + (W54 * a14) + (W55 * a15) + round) >> shift;
        output[di + 64] = ((W64 * c2) + (W65 * c3) + round) >> shift;
        output[di + 80] = ((W80 * a2) + (W81 * a3) + (W82 * a6) + (W83 * a7) + (W84 * a10) + (W85 * a11) + (W86 * a14) + (W87 * a15) + round) >> shift;
        output[di + 96] = ((W96 * b2) + (W97 * b3) + (W98 * b6) + (W99 * b7) + round) >> shift;
        output[di + 112] = ((W112 * a2) + (W113 * a3) + (W114 * a6) + (W115 * a7) + (W116 * a10) + (W117 * a11) + (W118 * a14) + (W119 * a15) + round) >> shift;
        output[di + 128] = ((W128 * c0) + (W129 * c1) + round) >> shift;
        output[di + 144] = ((W144 * a2) + (W145 * a3) + (W146 * a6) + (W147 * a7) + (W148 * a10) + (W149 * a11) + (W150 * a14) + (W151 * a15) + round) >> shift;
        output[di + 160] = ((W160 * b2) + (W161 * b3) + (W162 * b6) + (W163 * b7) + round) >> shift;
        output[di + 176] = ((W176 * a2) + (W177 * a3) + (W178 * a6) + (W179 * a7) + (W180 * a10) + (W181 * a11) + (W182 * a14) + (W183 * a15) + round) >> shift;
        output[di + 192] = ((W192 * c2) + (W193 * c3) + round) >> shift;
        output[di + 208] = ((W208 * a2) + (W209 * a3) + (W210 * a6) + (W211 * a7) + (W212 * a10) + (W213 * a11) + (W214 * a14) + (W215 * a15) + round) >> shift;
        output[di + 224] = ((W224 * b2) + (W225 * b3) + (W226 * b6) + (W227 * b7) + round) >> shift;
        output[di + 240] = ((W240 * a2) + (W241 * a3) + (W242 * a6) + (W243 * a7) + (W244 * a10) + (W245 * a11) + (W246 * a14) + (W247 * a15) + round) >> shift;
    }
}

bool DCT16::inverse(SliceArray<int>& input, SliceArray<int>& output, int length)
{
    if (length != 256)
       return false;
    
    if (!SliceArray<int>::isValid(input))
       return false;

    if (&input != &output)
    {
       if (!SliceArray<int>::isValid(output))
         return false;

       if (output._index + 256 > output._length)
         return false;   
    }

    computeInverse(&input._array[input._index], _data, 10);
    computeInverse(_data, &output._array[output._index], _iShift - 10);
    input._index += 256;
    output._index += 256;
    return true;
}

void DCT16::computeInverse(int input[], int output[], const int shift)
{
    const int round = (1 << shift) >> 1;

    for (int i = 0; i < 16; i++) {
        const int si = i;
        const int x0 = input[si];
        const int x1 = input[si + 16];
        const int x2 = input[si + 32];
        const int x3 = input[si + 48];
        const int x4 = input[si + 64];
        const int x5 = input[si + 80];
        const int x6 = input[si + 96];
        const int x7 = input[si + 112];
        const int x8 = input[si + 128];
        const int x9 = input[si + 144];
        const int x10 = input[si + 160];
        const int x11 = input[si + 176];
        const int x12 = input[si + 192];
        const int x13 = input[si + 208];
        const int x14 = input[si + 224];
        const int x15 = input[si + 240];

        const int a0 = (W16 * x1) + (W48 * x3) + (W80 * x5) + (W112 * x7) + (W144 * x9) + (W176 * x11) + (W208 * x13) + (W240 * x15);
        const int a1 = (W17 * x1) + (W49 * x3) + (W81 * x5) + (W113 * x7) + (W145 * x9) + (W177 * x11) + (W209 * x13) + (W241 * x15);
        const int a2 = (W18 * x1) + (W50 * x3) + (W82 * x5) + (W114 * x7) + (W146 * x9) + (W178 * x11) + (W210 * x13) + (W242 * x15);
        const int a3 = (W19 * x1) + (W51 * x3) + (W83 * x5) + (W115 * x7) + (W147 * x9) + (W179 * x11) + (W211 * x13) + (W243 * x15);
        const int a4 = (W20 * x1) + (W52 * x3) + (W84 * x5) + (W116 * x7) + (W148 * x9) + (W180 * x11) + (W212 * x13) + (W244 * x15);
        const int a5 = (W21 * x1) + (W53 * x3) + (W85 * x5) + (W117 * x7) + (W149 * x9) + (W181 * x11) + (W213 * x13) + (W245 * x15);
        const int a6 = (W22 * x1) + (W54 * x3) + (W86 * x5) + (W118 * x7) + (W150 * x9) + (W182 * x11) + (W214 * x13) + (W246 * x15);
        const int a7 = (W23 * x1) + (W55 * x3) + (W87 * x5) + (W119 * x7) + (W151 * x9) + (W183 * x11) + (W215 * x13) + (W247 * x15);

        const int b0 = (W32 * x2) + (W96 * x6) + (W160 * x10) + (W224 * x14);
        const int b1 = (W33 * x2) + (W97 * x6) + (W161 * x10) + (W225 * x14);
        const int b2 = (W34 * x2) + (W98 * x6) + (W162 * x10) + (W226 * x14);
        const int b3 = (W35 * x2) + (W99 * x6) + (W163 * x10) + (W227 * x14);
        const int b4 = (W0 * x0) + (W128 * x8) + (W64 * x4) + (W192 * x12);
        const int b5 = (W0 * x0) + (W128 * x8) - (W64 * x4) - (W192 * x12);
        const int b6 = (W1 * x0) + (W129 * x8) + (W65 * x4) + (W193 * x12);
        const int b7 = (W1 * x0) + (W129 * x8) - (W65 * x4) - (W193 * x12);

        const int c0 = b4 + b0;
        const int c1 = b6 + b1;
        const int c2 = b7 + b2;
        const int c3 = b5 + b3;
        const int c4 = b5 - b3;
        const int c5 = b7 - b2;
        const int c6 = b6 - b1;
        const int c7 = b4 - b0;

        const int d0 = (c0 + a0 + round) >> shift;
        const int d1 = (c1 + a1 + round) >> shift;
        const int d2 = (c2 + a2 + round) >> shift;
        const int d3 = (c3 + a3 + round) >> shift;
        const int d4 = (c4 + a4 + round) >> shift;
        const int d5 = (c5 + a5 + round) >> shift;
        const int d6 = (c6 + a6 + round) >> shift;
        const int d7 = (c7 + a7 + round) >> shift;
        const int d8 = (c7 - a7 + round) >> shift;
        const int d9 = (c6 - a6 + round) >> shift;
        const int d10 = (c5 - a5 + round) >> shift;
        const int d11 = (c4 - a4 + round) >> shift;
        const int d12 = (c3 - a3 + round) >> shift;
        const int d13 = (c2 - a2 + round) >> shift;
        const int d14 = (c1 - a1 + round) >> shift;
        const int d15 = (c0 - a0 + round) >> shift;

        const int di = i << 4;
        output[di] = (d0 >= MAX_VAL) ? MAX_VAL : ((d0 <= MIN_VAL) ? MIN_VAL : d0);
        output[di + 1] = (d1 >= MAX_VAL) ? MAX_VAL : ((d1 <= MIN_VAL) ? MIN_VAL : d1);
        output[di + 2] = (d2 >= MAX_VAL) ? MAX_VAL : ((d2 <= MIN_VAL) ? MIN_VAL : d2);
        output[di + 3] = (d3 >= MAX_VAL) ? MAX_VAL : ((d3 <= MIN_VAL) ? MIN_VAL : d3);
        output[di + 4] = (d4 >= MAX_VAL) ? MAX_VAL : ((d4 <= MIN_VAL) ? MIN_VAL : d4);
        output[di + 5] = (d5 >= MAX_VAL) ? MAX_VAL : ((d5 <= MIN_VAL) ? MIN_VAL : d5);
        output[di + 6] = (d6 >= MAX_VAL) ? MAX_VAL : ((d6 <= MIN_VAL) ? MIN_VAL : d6);
        output[di + 7] = (d7 >= MAX_VAL) ? MAX_VAL : ((d7 <= MIN_VAL) ? MIN_VAL : d7);
        output[di + 8] = (d8 >= MAX_VAL) ? MAX_VAL : ((d8 <= MIN_VAL) ? MIN_VAL : d8);
        output[di + 9] = (d9 >= MAX_VAL) ? MAX_VAL : ((d9 <= MIN_VAL) ? MIN_VAL : d9);
        output[di + 10] = (d10 >= MAX_VAL) ? MAX_VAL : ((d10 <= MIN_VAL) ? MIN_VAL : d10);
        output[di + 11] = (d11 >= MAX_VAL) ? MAX_VAL : ((d11 <= MIN_VAL) ? MIN_VAL : d11);
        output[di + 12] = (d12 >= MAX_VAL) ? MAX_VAL : ((d12 <= MIN_VAL) ? MIN_VAL : d12);
        output[di + 13] = (d13 >= MAX_VAL) ? MAX_VAL : ((d13 <= MIN_VAL) ? MIN_VAL : d13);
        output[di + 14] = (d14 >= MAX_VAL) ? MAX_VAL : ((d14 <= MIN_VAL) ? MIN_VAL : d14);
        output[di + 15] = (d15 >= MAX_VAL) ? MAX_VAL : ((d15 <= MIN_VAL) ? MIN_VAL : d15);
    }
}