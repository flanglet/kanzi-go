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

#include <iostream>
#include <fstream>
#include <ctime>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <algorithm>
#include "../types.hpp"
#include "../transform/MTFT.hpp"

using namespace std;
using namespace kanzi;

void testMTFTCorrectness()
{
    // Test behavior
    cout << "MTFT Correctness test" << endl;
    srand(uint(time(nullptr)));

    for (int ii = 0; ii < 20; ii++) {
        byte val[32];
        int size = 32;

        if (ii == 0) {
            byte val2[] = { 5, 2, 4, 7, 0, 0, 7, 1, 7 };
            size = 32;
            memset(&val[0], 0, size);
            memcpy(&val[0], &val2[0], 9);
        }
        else {
            for (int i = 0; i < 32; i++)
                val[i] = byte(65 + (rand() % (5 * ii)));
        }

        MTFT mtft;
        byte* input = &val[0];
        byte* transform = new byte[size + 20];
        byte* reverse = new byte[size];
        cout << endl
             << "Test " << (ii + 1);
        cout << endl
            << "Input     : ";

        for (int i = 0; i < size; i++)
            cout << (input[i] & 0xFF) << " ";

        int start = (ii & 1) * ii;
        SliceArray<byte> ia1(input, size, 0);
        SliceArray<byte> ia2(transform, size + 20, start);
        mtft.forward(ia1, ia2, size);

        cout << endl
             << "Transform : ";

        for (int i = start; i < start + size; i++)
            cout << (transform[i] & 0xFF) << " ";

        SliceArray<byte> ia3(reverse, size, 0);
        ia2._index = start;
        mtft.inverse(ia2, ia3, size);

        bool ok = true;
        cout << endl
           << "Reverse   : ";

        for (int i = 0; i < size; i++)
            cout << (reverse[i] & 0xFF) << " ";

        for (int j = 0; j < size; j++) {
            if (input[j] != reverse[j]) {
                ok = false;
                break;
            }
        }

        cout << endl;
        cout << ((ok) ? "Identical" : "Different") << endl;
        delete[] transform;
        delete[] reverse;
    }
}

int testMTFTSpeed()
{
    // Test speed
    int iter = 20000;
    int size = 10000;
    cout << endl
         << endl
         << "MTFT Speed test" << endl;
    cout << "Iterations: " << iter << endl;
    srand(uint(time(nullptr)));

    for (int jj = 0; jj < 4; jj++) {
        byte input[20000];
        byte output[20000];
        byte reverse[20000];
        MTFT mtft;
        double delta1 = 0, delta2 = 0;

        if (jj == 0) {
            cout << endl
                 << endl
                 << "Purely random input" << endl;
        }

        if (jj == 2) {
            cout << endl
                 << endl
                 << "Semi random input" << endl;
        }

        for (int ii = 0; ii < iter; ii++) {
            for (int i = 0; i < size; i++) {
                int n = 128;

                if (jj < 2) {
                    // Pure random
                    input[i] = byte(rand() % 256);
                }
                else {
                    // Semi random (a bit more realistic input)
                    int rng = 5;

                    if ((i & 7) == 0) {
                        rng = 128;
                    }

                    int p = ((rand() % rng) - rng / 2 + n) & 0xFF;
                    input[i] = (byte)p;
                    n = p;
                }
            }

            SliceArray<byte> ia1(input, size, 0);
            SliceArray<byte> ia2(output, size, 0);
            SliceArray<byte> ia3(reverse, size, 0);
            clock_t before1 = clock();
            mtft.forward(ia1, ia2, size);
            clock_t after1 = clock();
            delta1 += (after1 - before1);
            clock_t before2 = clock();
            ia2._index = 0;
            mtft.inverse(ia2, ia3, size);
            clock_t after2 = clock();
            delta2 += (after2 - before2);

            // Sanity check
            int idx = -1;

            for (int i = 0; i < size; i++) {
                if (input[i] != reverse[i]) {
                    idx = i;
                    break;
                }

                if (idx >= 0) {
                    cout << "Failure at index " << i << " (" << (int)input[i] << "<->" << (int)reverse[i] << ")" << endl;
                    break;
                }
            }
        }

        double prod = double(iter) * double(size);
        double b2KB = double(1) / double(1024);
        double d1_sec = double(delta1) / CLOCKS_PER_SEC;
        double d2_sec = double(delta2) / CLOCKS_PER_SEC;
        cout << "MTFT Forward transform [ms]: " << int(d1_sec * 1000) << endl;
        cout << "Throughput [KB/s]          : " << int(prod * b2KB / d1_sec) << endl;
        cout << "MTFT Reverse transform [ms]: " << int(d2_sec * 1000) << endl;
        cout << "Throughput [KB/s]          : " << int(prod * b2KB / d2_sec) << endl;
        cout << endl;
    }

    return 0;
}

#ifdef __GNUG__
int main(int, const char**)
#else
int TestMTFT_main(int, const char**)
#endif
{
    testMTFTCorrectness();
    testMTFTSpeed();
    return 0;
}
