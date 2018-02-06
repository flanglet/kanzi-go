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
#include "../transform/BWT.hpp"
#include "../transform/BWTS.hpp"

using namespace std;
using namespace kanzi;

void testBWTCorrectness(bool isBWT)
{
    // Test behavior
    cout << endl
         << endl
         << (isBWT ? "BWT" : "BWTS") << " Correctness test" << endl;
    srand((uint)time(nullptr));

    for (int ii = 1; ii <= 20; ii++) {
        byte buf1[128];
        int size = 128;

        if (ii == 1) {
            string str("mississippi");
            const char* val2 = str.c_str();
            cout << val2 << endl;
            size = int(str.length());
            memcpy(buf1, &val2[0], size);
        }
        else if (ii == 2) {
            string str("3.14159265358979323846264338327950288419716939937510");
            const char* val2 = str.c_str();
            size = int(str.length());
            memcpy(buf1, &val2[0], size);
        }
        else if (ii == 3) {
            string str("SIX.MIXED.PIXIES.SIFT.SIXTY.PIXIE.DUST.BOXES");
            const char* val2 = str.c_str();
            size = int(str.length());
            memcpy(buf1, &val2[0], size);
        }
        else {
            for (int i = 0; i < size; i++)
                buf1[i] = byte(65 + (rand() % (4 * ii)));
        }

        Transform<byte>* bwt;

        if (isBWT) {
            bwt = new BWT();
        }
        else {
            bwt = new BWTS();
        }

        byte* input = &buf1[0];
        byte* transform = new byte[size];
        byte* reverse = new byte[size];
        cout << endl
             << "Test " << ii;
        cout << endl
             << "Input   : ";

        for (int i = 0; i < size; i++)
            cout << input[i];

        SliceArray<byte> ia1(input, size, 0);
        SliceArray<byte> ia2(transform, size, 0);
        bwt->forward(ia1, ia2, size);

        cout << endl
             << "Encoded : ";

        for (int i = 0; i < size; i++)
            cout << transform[i];

        if (isBWT) {
            int primaryIndex = ((BWT*)bwt)->getPrimaryIndex(0);
            cout << "  (Primary index=" << primaryIndex << ")" << endl;
        }
        else {
            cout << endl;
        }

        SliceArray<byte> ia3(reverse, size, 0);
        ia2._index = 0;
        bwt->inverse(ia2, ia3, size);

        bool ok = true;
        cout << "Reverse : ";

        for (int i = 0; i < size; i++)
            cout << reverse[i];

        cout << endl;

        for (int j = 0; j < size; j++) {
            if (input[j] != reverse[j]) {
                ok = false;
                break;
            }
        }

        cout << endl;
        cout << ((ok) ? "Identical" : "Different") << endl;
        delete bwt;
        delete[] transform;
        delete[] reverse;
    }
}

int testBWTSpeed(bool isBWT)
{
    // Test speed
    int iter = 2000;
    int size = 256 * 1024;
    cout << endl
         << endl
         << (isBWT ? "BWT" : "BWTS") << " Speed test" << endl;    
    cout << "Iterations: " << iter << endl;
    cout << "Transform size: " << size << endl;
    srand(uint(time(nullptr)));

    for (int jj = 0; jj < 3; jj++) {
        byte input[256 * 1024];
        byte output[256 * 1024];
        byte reverse[256 * 1024];
        SliceArray<byte> ia1(input, size, 0);
        SliceArray<byte> ia2(output, size, 0);
        SliceArray<byte> ia3(reverse, size, 0);
        double delta1 = 0, delta2 = 0;
        Transform<byte>* bwt;

        if (isBWT) {
            bwt = new BWT();
        }
        else {
            bwt = new BWTS();
        }

        for (int ii = 0; ii < iter; ii++) {
            for (int i = 0; i < size; i++) {
                input[i] = 1 + byte(rand() % 255);
            }

            clock_t before1 = clock();
            ia1._index = 0;
            ia2._index = 0;
            bwt->forward(ia1, ia2, size);
            clock_t after1 = clock();
            delta1 += (after1 - before1);
            clock_t before2 = clock();
            ia2._index = 0;
            ia3._index = 0;
            bwt->inverse(ia2, ia3, size);
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
                    cout << "Failure at index " << i << " (" << int(input[i]) << "<->" << int(reverse[i]) << ")" << endl;
                    break;
                }
            }
        }

        delete bwt;

        double prod = double(iter) * double(size);
        double b2KB = double(1) / double(1024);
        double d1_sec = double(delta1) / CLOCKS_PER_SEC;
        double d2_sec = double(delta2) / CLOCKS_PER_SEC;
        cout << "Forward transform [ms] : " << int(d1_sec * 1000) << endl;
        cout << "Throughput [KB/s]      : " << int(prod * b2KB / d1_sec) << endl;
        cout << "Reverse transform [ms] : " << int(d2_sec * 1000) << endl;
        cout << "Throughput [KB/s]      : " << int(prod * b2KB / d2_sec) << endl;
        cout << endl;
    }

    return 0;
}

#ifdef __GNUG__
int main()
#else
int TestBWT_main()
#endif
{
    testBWTCorrectness(true);
    testBWTCorrectness(false);
    testBWTSpeed(true);
    testBWTSpeed(false);
    return 0;
}
