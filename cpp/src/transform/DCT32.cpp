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

#include "DCT32.hpp"

using namespace kanzi;

const int DCT32::W[] = {
    64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64,
    64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64,
    90, 90, 88, 85, 82, 78, 73, 67, 61, 54, 46, 38, 31, 22, 13, 4,
    -4, -13, -22, -31, -38, -46, -54, -61, -67, -73, -78, -82, -85, -88, -90, -90,
    90, 87, 80, 70, 57, 43, 25, 9, -9, -25, -43, -57, -70, -80, -87, -90,
    -90, -87, -80, -70, -57, -43, -25, -9, 9, 25, 43, 57, 70, 80, 87, 90,
    90, 82, 67, 46, 22, -4, -31, -54, -73, -85, -90, -88, -78, -61, -38, -13,
    13, 38, 61, 78, 88, 90, 85, 73, 54, 31, 4, -22, -46, -67, -82, -90,
    89, 75, 50, 18, -18, -50, -75, -89, -89, -75, -50, -18, 18, 50, 75, 89,
    89, 75, 50, 18, -18, -50, -75, -89, -89, -75, -50, -18, 18, 50, 75, 89,
    88, 67, 31, -13, -54, -82, -90, -78, -46, -4, 38, 73, 90, 85, 61, 22,
    -22, -61, -85, -90, -73, -38, 4, 46, 78, 90, 82, 54, 13, -31, -67, -88,
    87, 57, 9, -43, -80, -90, -70, -25, 25, 70, 90, 80, 43, -9, -57, -87,
    -87, -57, -9, 43, 80, 90, 70, 25, -25, -70, -90, -80, -43, 9, 57, 87,
    85, 46, -13, -67, -90, -73, -22, 38, 82, 88, 54, -4, -61, -90, -78, -31,
    31, 78, 90, 61, 4, -54, -88, -82, -38, 22, 73, 90, 67, 13, -46, -85,
    83, 36, -36, -83, -83, -36, 36, 83, 83, 36, -36, -83, -83, -36, 36, 83,
    83, 36, -36, -83, -83, -36, 36, 83, 83, 36, -36, -83, -83, -36, 36, 83,
    82, 22, -54, -90, -61, 13, 78, 85, 31, -46, -90, -67, 4, 73, 88, 38,
    -38, -88, -73, -4, 67, 90, 46, -31, -85, -78, -13, 61, 90, 54, -22, -82,
    80, 9, -70, -87, -25, 57, 90, 43, -43, -90, -57, 25, 87, 70, -9, -80,
    -80, -9, 70, 87, 25, -57, -90, -43, 43, 90, 57, -25, -87, -70, 9, 80,
    78, -4, -82, -73, 13, 85, 67, -22, -88, -61, 31, 90, 54, -38, -90, -46,
    46, 90, 38, -54, -90, -31, 61, 88, 22, -67, -85, -13, 73, 82, 4, -78,
    75, -18, -89, -50, 50, 89, 18, -75, -75, 18, 89, 50, -50, -89, -18, 75,
    75, -18, -89, -50, 50, 89, 18, -75, -75, 18, 89, 50, -50, -89, -18, 75,
    73, -31, -90, -22, 78, 67, -38, -90, -13, 82, 61, -46, -88, -4, 85, 54,
    -54, -85, 4, 88, 46, -61, -82, 13, 90, 38, -67, -78, 22, 90, 31, -73,
    70, -43, -87, 9, 90, 25, -80, -57, 57, 80, -25, -90, -9, 87, 43, -70,
    -70, 43, 87, -9, -90, -25, 80, 57, -57, -80, 25, 90, 9, -87, -43, 70,
    67, -54, -78, 38, 85, -22, -90, 4, 90, 13, -88, -31, 82, 46, -73, -61,
    61, 73, -46, -82, 31, 88, -13, -90, -4, 90, 22, -85, -38, 78, 54, -67,
    64, -64, -64, 64, 64, -64, -64, 64, 64, -64, -64, 64, 64, -64, -64, 64,
    64, -64, -64, 64, 64, -64, -64, 64, 64, -64, -64, 64, 64, -64, -64, 64,
    61, -73, -46, 82, 31, -88, -13, 90, -4, -90, 22, 85, -38, -78, 54, 67,
    -67, -54, 78, 38, -85, -22, 90, 4, -90, 13, 88, -31, -82, 46, 73, -61,
    57, -80, -25, 90, -9, -87, 43, 70, -70, -43, 87, 9, -90, 25, 80, -57,
    -57, 80, 25, -90, 9, 87, -43, -70, 70, 43, -87, -9, 90, -25, -80, 57,
    54, -85, -4, 88, -46, -61, 82, 13, -90, 38, 67, -78, -22, 90, -31, -73,
    73, 31, -90, 22, 78, -67, -38, 90, -13, -82, 61, 46, -88, 4, 85, -54,
    50, -89, 18, 75, -75, -18, 89, -50, -50, 89, -18, -75, 75, 18, -89, 50,
    50, -89, 18, 75, -75, -18, 89, -50, -50, 89, -18, -75, 75, 18, -89, 50,
    46, -90, 38, 54, -90, 31, 61, -88, 22, 67, -85, 13, 73, -82, 4, 78,
    -78, -4, 82, -73, -13, 85, -67, -22, 88, -61, -31, 90, -54, -38, 90, -46,
    43, -90, 57, 25, -87, 70, 9, -80, 80, -9, -70, 87, -25, -57, 90, -43,
    -43, 90, -57, -25, 87, -70, -9, 80, -80, 9, 70, -87, 25, 57, -90, 43,
    38, -88, 73, -4, -67, 90, -46, -31, 85, -78, 13, 61, -90, 54, 22, -82,
    82, -22, -54, 90, -61, -13, 78, -85, 31, 46, -90, 67, 4, -73, 88, -38,
    36, -83, 83, -36, -36, 83, -83, 36, 36, -83, 83, -36, -36, 83, -83, 36,
    36, -83, 83, -36, -36, 83, -83, 36, 36, -83, 83, -36, -36, 83, -83, 36,
    31, -78, 90, -61, 4, 54, -88, 82, -38, -22, 73, -90, 67, -13, -46, 85,
    -85, 46, 13, -67, 90, -73, 22, 38, -82, 88, -54, -4, 61, -90, 78, -31,
    25, -70, 90, -80, 43, 9, -57, 87, -87, 57, -9, -43, 80, -90, 70, -25,
    -25, 70, -90, 80, -43, -9, 57, -87, 87, -57, 9, 43, -80, 90, -70, 25,
    22, -61, 85, -90, 73, -38, -4, 46, -78, 90, -82, 54, -13, -31, 67, -88,
    88, -67, 31, 13, -54, 82, -90, 78, -46, 4, 38, -73, 90, -85, 61, -22,
    18, -50, 75, -89, 89, -75, 50, -18, -18, 50, -75, 89, -89, 75, -50, 18,
    18, -50, 75, -89, 89, -75, 50, -18, -18, 50, -75, 89, -89, 75, -50, 18,
    13, -38, 61, -78, 88, -90, 85, -73, 54, -31, 4, 22, -46, 67, -82, 90,
    -90, 82, -67, 46, -22, -4, 31, -54, 73, -85, 90, -88, 78, -61, 38, -13,
    9, -25, 43, -57, 70, -80, 87, -90, 90, -87, 80, -70, 57, -43, 25, -9,
    -9, 25, -43, 57, -70, 80, -87, 90, -90, 87, -80, 70, -57, 43, -25, 9,
    4, -13, 22, -31, 38, -46, 54, -61, 67, -73, 78, -82, 85, -88, 90, -90,
    90, -90, 88, -85, 82, -78, 73, -67, 61, -54, 46, -38, 31, -22, 13, -4
};

DCT32::DCT32()
{
    _fShift = 14;
    _iShift = 20;
}

bool DCT32::forward(SliceArray<int>& input, SliceArray<int>& output, int length)
{
    if (length != 1024)
       return false;
    
    if (!SliceArray<int>::isValid(input))
       return false;

    if (&input != &output)
    {
       if (!SliceArray<int>::isValid(output))
         return false;

       if (output._index + 1024 > output._length)
         return false;   
    }

    computeForward(&input._array[input._index], _data, 4);
    computeForward(_data, &output._array[output._index], _fShift - 4);
    input._index += 1024;
    output._index += 1024;
    return true;
}

void DCT32::computeForward(int input[], int output[], const int shift)
{
    const int round = (1 << shift) >> 1;

    for (int i = 0; i < 32; i++) {
        const int si = i << 5;
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
        const int x16 = input[si + 16];
        const int x17 = input[si + 17];
        const int x18 = input[si + 18];
        const int x19 = input[si + 19];
        const int x20 = input[si + 20];
        const int x21 = input[si + 21];
        const int x22 = input[si + 22];
        const int x23 = input[si + 23];
        const int x24 = input[si + 24];
        const int x25 = input[si + 25];
        const int x26 = input[si + 26];
        const int x27 = input[si + 27];
        const int x28 = input[si + 28];
        const int x29 = input[si + 29];
        const int x30 = input[si + 30];
        const int x31 = input[si + 31];

        const int a0 = x0 + x31;
        const int a1 = x1 + x30;
        const int a2 = x0 - x31;
        const int a3 = x1 - x30;
        const int a4 = x2 + x29;
        const int a5 = x3 + x28;
        const int a6 = x2 - x29;
        const int a7 = x3 - x28;
        const int a8 = x4 + x27;
        const int a9 = x5 + x26;
        const int a10 = x4 - x27;
        const int a11 = x5 - x26;
        const int a12 = x6 + x25;
        const int a13 = x7 + x24;
        const int a14 = x6 - x25;
        const int a15 = x7 - x24;
        const int a16 = x8 + x23;
        const int a17 = x9 + x22;
        const int a18 = x8 - x23;
        const int a19 = x9 - x22;
        const int a20 = x10 + x21;
        const int a21 = x11 + x20;
        const int a22 = x10 - x21;
        const int a23 = x11 - x20;
        const int a24 = x12 + x19;
        const int a25 = x13 + x18;
        const int a26 = x12 - x19;
        const int a27 = x13 - x18;
        const int a28 = x14 + x17;
        const int a29 = x15 + x16;
        const int a30 = x14 - x17;
        const int a31 = x15 - x16;

        const int di = i;

        for (int n = 32; n < 1024; n += 64) {
            output[di + n] = ((W[n] * a2) + (W[n + 1] * a3) + (W[n + 2] * a6) + (W[n + 3] * a7) + (W[n + 4] * a10) + (W[n + 5] * a11) + (W[n + 6] * a14) + (W[n + 7] * a15) + (W[n + 8] * a18) + (W[n + 9] * a19) + (W[n + 10] * a22) + (W[n + 11] * a23) + (W[n + 12] * a26) + (W[n + 13] * a27) + (W[n + 14] * a30) + (W[n + 15] * a31) + round) >> shift;
        }

        const int b0 = a0 + a29;
        const int b1 = a1 + a28;
        const int b2 = a0 - a29;
        const int b3 = a1 - a28;
        const int b4 = a4 + a25;
        const int b5 = a5 + a24;
        const int b6 = a4 - a25;
        const int b7 = a5 - a24;
        const int b8 = a8 + a21;
        const int b9 = a9 + a20;
        const int b10 = a8 - a21;
        const int b11 = a9 - a20;
        const int b12 = a12 + a17;
        const int b13 = a13 + a16;
        const int b14 = a12 - a17;
        const int b15 = a13 - a16;

        output[di + 64] = ((W[64] * b2) + (W[65] * b3) + (W[66] * b6) + (W[67] * b7) + (W[68] * b10) + (W[69] * b11) + (W[70] * b14) + (W[71] * b15) + round) >> shift;
        output[di + 192] = ((W[192] * b2) + (W[193] * b3) + (W[194] * b6) + (W[195] * b7) + (W[196] * b10) + (W[197] * b11) + (W[198] * b14) + (W[199] * b15) + round) >> shift;
        output[di + 320] = ((W[320] * b2) + (W[321] * b3) + (W[322] * b6) + (W[323] * b7) + (W[324] * b10) + (W[325] * b11) + (W[326] * b14) + (W[327] * b15) + round) >> shift;
        output[di + 448] = ((W[448] * b2) + (W[449] * b3) + (W[450] * b6) + (W[451] * b7) + (W[452] * b10) + (W[453] * b11) + (W[454] * b14) + (W[455] * b15) + round) >> shift;
        output[di + 576] = ((W[576] * b2) + (W[577] * b3) + (W[578] * b6) + (W[579] * b7) + (W[580] * b10) + (W[581] * b11) + (W[582] * b14) + (W[583] * b15) + round) >> shift;
        output[di + 704] = ((W[704] * b2) + (W[705] * b3) + (W[706] * b6) + (W[707] * b7) + (W[708] * b10) + (W[709] * b11) + (W[710] * b14) + (W[711] * b15) + round) >> shift;
        output[di + 832] = ((W[832] * b2) + (W[833] * b3) + (W[834] * b6) + (W[835] * b7) + (W[836] * b10) + (W[837] * b11) + (W[838] * b14) + (W[839] * b15) + round) >> shift;
        output[di + 960] = ((W[960] * b2) + (W[961] * b3) + (W[962] * b6) + (W[963] * b7) + (W[964] * b10) + (W[965] * b11) + (W[966] * b14) + (W[967] * b15) + round) >> shift;

        const int c0 = b0 + b13;
        const int c1 = b1 + b12;
        const int c2 = b0 - b13;
        const int c3 = b1 - b12;
        const int c4 = b4 + b9;
        const int c5 = b5 + b8;
        const int c6 = b4 - b9;
        const int c7 = b5 - b8;

        output[di + 128] = ((W[128] * c2) + (W[129] * c3) + (W[130] * c6) + (W[131] * c7) + round) >> shift;
        output[di + 384] = ((W[384] * c2) + (W[385] * c3) + (W[386] * c6) + (W[387] * c7) + round) >> shift;
        output[di + 640] = ((W[640] * c2) + (W[641] * c3) + (W[642] * c6) + (W[643] * c7) + round) >> shift;
        output[di + 896] = ((W[896] * c2) + (W[897] * c3) + (W[898] * c6) + (W[899] * c7) + round) >> shift;

        const int d0 = c0 + c5;
        const int d1 = c1 + c4;
        const int d2 = c0 - c5;
        const int d3 = c1 - c4;

        output[di] = ((W[0] * d0) + (W[1] * d1) + round) >> shift;
        output[di + 512] = ((W[512] * d0) + (W[513] * d1) + round) >> shift;
        output[di + 256] = ((W[256] * d2) + (W[257] * d3) + round) >> shift;
        output[di + 768] = ((W[768] * d2) + (W[769] * d3) + round) >> shift;
    }
}

bool DCT32::inverse(SliceArray<int>& input, SliceArray<int>& output, int length)
{
    if (length != 1024)
       return false;
    
    if (!SliceArray<int>::isValid(input))
       return false;

    if (&input != &output)
    {
       if (!SliceArray<int>::isValid(output))
         return false;

       if (output._index + 1024 > output._length)
         return false;   
    }

    computeInverse(&input._array[input._index], _data, 10);
    computeInverse(_data, &output._array[output._index], _iShift - 10);
    input._index += 1024;
    output._index += 1024;
    return true;
}

void DCT32::computeInverse(int input[], int output[], const int shift)
{
    const int round = (1 << shift) >> 1;

    for (int i = 0; i < 32; i++) {
        const int si = i;
        const int x0 = input[si];
        const int x1 = input[si + 32];
        const int x2 = input[si + 64];
        const int x3 = input[si + 96];
        const int x4 = input[si + 128];
        const int x5 = input[si + 160];
        const int x6 = input[si + 192];
        const int x7 = input[si + 224];
        const int x8 = input[si + 256];
        const int x9 = input[si + 288];
        const int x10 = input[si + 320];
        const int x11 = input[si + 352];
        const int x12 = input[si + 384];
        const int x13 = input[si + 416];
        const int x14 = input[si + 448];
        const int x15 = input[si + 480];
        const int x16 = input[si + 512];
        const int x17 = input[si + 544];
        const int x18 = input[si + 576];
        const int x19 = input[si + 608];
        const int x20 = input[si + 640];
        const int x21 = input[si + 672];
        const int x22 = input[si + 704];
        const int x23 = input[si + 736];
        const int x24 = input[si + 768];
        const int x25 = input[si + 800];
        const int x26 = input[si + 832];
        const int x27 = input[si + 864];
        const int x28 = input[si + 896];
        const int x29 = input[si + 928];
        const int x30 = input[si + 960];
        const int x31 = input[si + 992];

        const int a0 = (W[32] * x1) + (W[96] * x3) + (W[160] * x5) + (W[224] * x7) + (W[288] * x9) + (W[352] * x11) + (W[416] * x13) + (W[480] * x15) + (W[544] * x17) + (W[608] * x19) + (W[672] * x21) + (W[736] * x23) + (W[800] * x25) + (W[864] * x27) + (W[928] * x29) + (W[992] * x31);
        const int a1 = (W[33] * x1) + (W[97] * x3) + (W[161] * x5) + (W[225] * x7) + (W[289] * x9) + (W[353] * x11) + (W[417] * x13) + (W[481] * x15) + (W[545] * x17) + (W[609] * x19) + (W[673] * x21) + (W[737] * x23) + (W[801] * x25) + (W[865] * x27) + (W[929] * x29) + (W[993] * x31);
        const int a2 = (W[34] * x1) + (W[98] * x3) + (W[162] * x5) + (W[226] * x7) + (W[290] * x9) + (W[354] * x11) + (W[418] * x13) + (W[482] * x15) + (W[546] * x17) + (W[610] * x19) + (W[674] * x21) + (W[738] * x23) + (W[802] * x25) + (W[866] * x27) + (W[930] * x29) + (W[994] * x31);
        const int a3 = (W[35] * x1) + (W[99] * x3) + (W[163] * x5) + (W[227] * x7) + (W[291] * x9) + (W[355] * x11) + (W[419] * x13) + (W[483] * x15) + (W[547] * x17) + (W[611] * x19) + (W[675] * x21) + (W[739] * x23) + (W[803] * x25) + (W[867] * x27) + (W[931] * x29) + (W[995] * x31);
        const int a4 = (W[36] * x1) + (W[100] * x3) + (W[164] * x5) + (W[228] * x7) + (W[292] * x9) + (W[356] * x11) + (W[420] * x13) + (W[484] * x15) + (W[548] * x17) + (W[612] * x19) + (W[676] * x21) + (W[740] * x23) + (W[804] * x25) + (W[868] * x27) + (W[932] * x29) + (W[996] * x31);
        const int a5 = (W[37] * x1) + (W[101] * x3) + (W[165] * x5) + (W[229] * x7) + (W[293] * x9) + (W[357] * x11) + (W[421] * x13) + (W[485] * x15) + (W[549] * x17) + (W[613] * x19) + (W[677] * x21) + (W[741] * x23) + (W[805] * x25) + (W[869] * x27) + (W[933] * x29) + (W[997] * x31);
        const int a6 = (W[38] * x1) + (W[102] * x3) + (W[166] * x5) + (W[230] * x7) + (W[294] * x9) + (W[358] * x11) + (W[422] * x13) + (W[486] * x15) + (W[550] * x17) + (W[614] * x19) + (W[678] * x21) + (W[742] * x23) + (W[806] * x25) + (W[870] * x27) + (W[934] * x29) + (W[998] * x31);
        const int a7 = (W[39] * x1) + (W[103] * x3) + (W[167] * x5) + (W[231] * x7) + (W[295] * x9) + (W[359] * x11) + (W[423] * x13) + (W[487] * x15) + (W[551] * x17) + (W[615] * x19) + (W[679] * x21) + (W[743] * x23) + (W[807] * x25) + (W[871] * x27) + (W[935] * x29) + (W[999] * x31);
        const int a8 = (W[40] * x1) + (W[104] * x3) + (W[168] * x5) + (W[232] * x7) + (W[296] * x9) + (W[360] * x11) + (W[424] * x13) + (W[488] * x15) + (W[552] * x17) + (W[616] * x19) + (W[680] * x21) + (W[744] * x23) + (W[808] * x25) + (W[872] * x27) + (W[936] * x29) + (W[1000] * x31);
        const int a9 = (W[41] * x1) + (W[105] * x3) + (W[169] * x5) + (W[233] * x7) + (W[297] * x9) + (W[361] * x11) + (W[425] * x13) + (W[489] * x15) + (W[553] * x17) + (W[617] * x19) + (W[681] * x21) + (W[745] * x23) + (W[809] * x25) + (W[873] * x27) + (W[937] * x29) + (W[1001] * x31);
        const int a10 = (W[42] * x1) + (W[106] * x3) + (W[170] * x5) + (W[234] * x7) + (W[298] * x9) + (W[362] * x11) + (W[426] * x13) + (W[490] * x15) + (W[554] * x17) + (W[618] * x19) + (W[682] * x21) + (W[746] * x23) + (W[810] * x25) + (W[874] * x27) + (W[938] * x29) + (W[1002] * x31);
        const int a11 = (W[43] * x1) + (W[107] * x3) + (W[171] * x5) + (W[235] * x7) + (W[299] * x9) + (W[363] * x11) + (W[427] * x13) + (W[491] * x15) + (W[555] * x17) + (W[619] * x19) + (W[683] * x21) + (W[747] * x23) + (W[811] * x25) + (W[875] * x27) + (W[939] * x29) + (W[1003] * x31);
        const int a12 = (W[44] * x1) + (W[108] * x3) + (W[172] * x5) + (W[236] * x7) + (W[300] * x9) + (W[364] * x11) + (W[428] * x13) + (W[492] * x15) + (W[556] * x17) + (W[620] * x19) + (W[684] * x21) + (W[748] * x23) + (W[812] * x25) + (W[876] * x27) + (W[940] * x29) + (W[1004] * x31);
        const int a13 = (W[45] * x1) + (W[109] * x3) + (W[173] * x5) + (W[237] * x7) + (W[301] * x9) + (W[365] * x11) + (W[429] * x13) + (W[493] * x15) + (W[557] * x17) + (W[621] * x19) + (W[685] * x21) + (W[749] * x23) + (W[813] * x25) + (W[877] * x27) + (W[941] * x29) + (W[1005] * x31);
        const int a14 = (W[46] * x1) + (W[110] * x3) + (W[174] * x5) + (W[238] * x7) + (W[302] * x9) + (W[366] * x11) + (W[430] * x13) + (W[494] * x15) + (W[558] * x17) + (W[622] * x19) + (W[686] * x21) + (W[750] * x23) + (W[814] * x25) + (W[878] * x27) + (W[942] * x29) + (W[1006] * x31);
        const int a15 = (W[47] * x1) + (W[111] * x3) + (W[175] * x5) + (W[239] * x7) + (W[303] * x9) + (W[367] * x11) + (W[431] * x13) + (W[495] * x15) + (W[559] * x17) + (W[623] * x19) + (W[687] * x21) + (W[751] * x23) + (W[815] * x25) + (W[879] * x27) + (W[943] * x29) + (W[1007] * x31);

        const int b0 = (W[64] * x2) + (W[192] * x6) + (W[320] * x10) + (W[448] * x14) + (W[576] * x18) + (W[704] * x22) + (W[832] * x26) + (W[960] * x30);
        const int b1 = (W[65] * x2) + (W[193] * x6) + (W[321] * x10) + (W[449] * x14) + (W[577] * x18) + (W[705] * x22) + (W[833] * x26) + (W[961] * x30);
        const int b2 = (W[66] * x2) + (W[194] * x6) + (W[322] * x10) + (W[450] * x14) + (W[578] * x18) + (W[706] * x22) + (W[834] * x26) + (W[962] * x30);
        const int b3 = (W[67] * x2) + (W[195] * x6) + (W[323] * x10) + (W[451] * x14) + (W[579] * x18) + (W[707] * x22) + (W[835] * x26) + (W[963] * x30);
        const int b4 = (W[68] * x2) + (W[196] * x6) + (W[324] * x10) + (W[452] * x14) + (W[580] * x18) + (W[708] * x22) + (W[836] * x26) + (W[964] * x30);
        const int b5 = (W[69] * x2) + (W[197] * x6) + (W[325] * x10) + (W[453] * x14) + (W[581] * x18) + (W[709] * x22) + (W[837] * x26) + (W[965] * x30);
        const int b6 = (W[70] * x2) + (W[198] * x6) + (W[326] * x10) + (W[454] * x14) + (W[582] * x18) + (W[710] * x22) + (W[838] * x26) + (W[966] * x30);
        const int b7 = (W[71] * x2) + (W[199] * x6) + (W[327] * x10) + (W[455] * x14) + (W[583] * x18) + (W[711] * x22) + (W[839] * x26) + (W[967] * x30);

        const int c0 = (W[128] * x4) + (W[384] * x12) + (W[640] * x20) + (W[896] * x28);
        const int c1 = (W[129] * x4) + (W[385] * x12) + (W[641] * x20) + (W[897] * x28);
        const int c2 = (W[130] * x4) + (W[386] * x12) + (W[642] * x20) + (W[898] * x28);
        const int c3 = (W[131] * x4) + (W[387] * x12) + (W[643] * x20) + (W[899] * x28);
        const int c4 = (W[256] * x8) + (W[768] * x24);
        const int c5 = (W[257] * x8) + (W[769] * x24);
        const int c6 = (W[0] * x0) + (W[512] * x16);
        const int c7 = (W[1] * x0) + (W[513] * x16);
        const int c8 = c6 + c4;
        const int c9 = c7 + c5;
        const int c10 = c7 - c5;
        const int c11 = c6 - c4;

        const int d0 = c8 + c0;
        const int d1 = c9 + c1;
        const int d2 = c10 + c2;
        const int d3 = c11 + c3;
        const int d4 = c11 - c3;
        const int d5 = c10 - c2;
        const int d6 = c9 - c1;
        const int d7 = c8 - c0;

        const int e0 = d0 + b0;
        const int e1 = d1 + b1;
        const int e2 = d2 + b2;
        const int e3 = d3 + b3;
        const int e4 = d4 + b4;
        const int e5 = d5 + b5;
        const int e6 = d6 + b6;
        const int e7 = d7 + b7;
        const int e8 = d7 - b7;
        const int e9 = d6 - b6;
        const int e10 = d5 - b5;
        const int e11 = d4 - b4;
        const int e12 = d3 - b3;
        const int e13 = d2 - b2;
        const int e14 = d1 - b1;
        const int e15 = d0 - b0;

        const int r0 = (e0 + a0 + round) >> shift;
        const int r16 = (e15 - a15 + round) >> shift;
        const int r1 = (e1 + a1 + round) >> shift;
        const int r17 = (e14 - a14 + round) >> shift;
        const int r2 = (e2 + a2 + round) >> shift;
        const int r18 = (e13 - a13 + round) >> shift;
        const int r3 = (e3 + a3 + round) >> shift;
        const int r19 = (e12 - a12 + round) >> shift;
        const int r4 = (e4 + a4 + round) >> shift;
        const int r20 = (e11 - a11 + round) >> shift;
        const int r5 = (e5 + a5 + round) >> shift;
        const int r21 = (e10 - a10 + round) >> shift;
        const int r6 = (e6 + a6 + round) >> shift;
        const int r22 = (e9 - a9 + round) >> shift;
        const int r7 = (e7 + a7 + round) >> shift;
        const int r23 = (e8 - a8 + round) >> shift;
        const int r8 = (e8 + a8 + round) >> shift;
        const int r24 = (e7 - a7 + round) >> shift;
        const int r9 = (e9 + a9 + round) >> shift;
        const int r25 = (e6 - a6 + round) >> shift;
        const int r10 = (e10 + a10 + round) >> shift;
        const int r26 = (e5 - a5 + round) >> shift;
        const int r11 = (e11 + a11 + round) >> shift;
        const int r27 = (e4 - a4 + round) >> shift;
        const int r12 = (e12 + a12 + round) >> shift;
        const int r28 = (e3 - a3 + round) >> shift;
        const int r13 = (e13 + a13 + round) >> shift;
        const int r29 = (e2 - a2 + round) >> shift;
        const int r14 = (e14 + a14 + round) >> shift;
        const int r30 = (e1 - a1 + round) >> shift;
        const int r15 = (e15 + a15 + round) >> shift;
        const int r31 = (e0 - a0 + round) >> shift;

        const int di = i << 5;
        output[di] = (r0 > MAX_VAL) ? MAX_VAL : ((r0 <= MIN_VAL) ? MIN_VAL : r0);
        output[di + 1] = (r1 > MAX_VAL) ? MAX_VAL : ((r1 <= MIN_VAL) ? MIN_VAL : r1);
        output[di + 2] = (r2 > MAX_VAL) ? MAX_VAL : ((r2 <= MIN_VAL) ? MIN_VAL : r2);
        output[di + 3] = (r3 > MAX_VAL) ? MAX_VAL : ((r3 <= MIN_VAL) ? MIN_VAL : r3);
        output[di + 4] = (r4 > MAX_VAL) ? MAX_VAL : ((r4 <= MIN_VAL) ? MIN_VAL : r4);
        output[di + 5] = (r5 > MAX_VAL) ? MAX_VAL : ((r5 <= MIN_VAL) ? MIN_VAL : r5);
        output[di + 6] = (r6 > MAX_VAL) ? MAX_VAL : ((r6 <= MIN_VAL) ? MIN_VAL : r6);
        output[di + 7] = (r7 > MAX_VAL) ? MAX_VAL : ((r7 <= MIN_VAL) ? MIN_VAL : r7);
        output[di + 8] = (r8 > MAX_VAL) ? MAX_VAL : ((r8 <= MIN_VAL) ? MIN_VAL : r8);
        output[di + 9] = (r9 > MAX_VAL) ? MAX_VAL : ((r9 <= MIN_VAL) ? MIN_VAL : r9);
        output[di + 10] = (r10 > MAX_VAL) ? MAX_VAL : ((r10 <= MIN_VAL) ? MIN_VAL : r10);
        output[di + 11] = (r11 > MAX_VAL) ? MAX_VAL : ((r11 <= MIN_VAL) ? MIN_VAL : r11);
        output[di + 12] = (r12 > MAX_VAL) ? MAX_VAL : ((r12 <= MIN_VAL) ? MIN_VAL : r12);
        output[di + 13] = (r13 > MAX_VAL) ? MAX_VAL : ((r13 <= MIN_VAL) ? MIN_VAL : r13);
        output[di + 14] = (r14 > MAX_VAL) ? MAX_VAL : ((r14 <= MIN_VAL) ? MIN_VAL : r14);
        output[di + 15] = (r15 > MAX_VAL) ? MAX_VAL : ((r15 <= MIN_VAL) ? MIN_VAL : r15);
        output[di + 16] = (r16 > MAX_VAL) ? MAX_VAL : ((r16 <= MIN_VAL) ? MIN_VAL : r16);
        output[di + 17] = (r17 > MAX_VAL) ? MAX_VAL : ((r17 <= MIN_VAL) ? MIN_VAL : r17);
        output[di + 18] = (r18 > MAX_VAL) ? MAX_VAL : ((r18 <= MIN_VAL) ? MIN_VAL : r18);
        output[di + 19] = (r19 > MAX_VAL) ? MAX_VAL : ((r19 <= MIN_VAL) ? MIN_VAL : r19);
        output[di + 20] = (r20 > MAX_VAL) ? MAX_VAL : ((r20 <= MIN_VAL) ? MIN_VAL : r20);
        output[di + 21] = (r21 > MAX_VAL) ? MAX_VAL : ((r21 <= MIN_VAL) ? MIN_VAL : r21);
        output[di + 22] = (r22 > MAX_VAL) ? MAX_VAL : ((r22 <= MIN_VAL) ? MIN_VAL : r22);
        output[di + 23] = (r23 > MAX_VAL) ? MAX_VAL : ((r23 <= MIN_VAL) ? MIN_VAL : r23);
        output[di + 24] = (r24 > MAX_VAL) ? MAX_VAL : ((r24 <= MIN_VAL) ? MIN_VAL : r24);
        output[di + 25] = (r25 > MAX_VAL) ? MAX_VAL : ((r25 <= MIN_VAL) ? MIN_VAL : r25);
        output[di + 26] = (r26 > MAX_VAL) ? MAX_VAL : ((r26 <= MIN_VAL) ? MIN_VAL : r26);
        output[di + 27] = (r27 > MAX_VAL) ? MAX_VAL : ((r27 <= MIN_VAL) ? MIN_VAL : r27);
        output[di + 28] = (r28 > MAX_VAL) ? MAX_VAL : ((r28 <= MIN_VAL) ? MIN_VAL : r28);
        output[di + 29] = (r29 > MAX_VAL) ? MAX_VAL : ((r29 <= MIN_VAL) ? MIN_VAL : r29);
        output[di + 30] = (r30 > MAX_VAL) ? MAX_VAL : ((r30 <= MIN_VAL) ? MIN_VAL : r30);
        output[di + 31] = (r31 > MAX_VAL) ? MAX_VAL : ((r31 <= MIN_VAL) ? MIN_VAL : r31);
    }
}