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
#include <sstream>
#include <fstream>
#include <ctime>
#include <cstdio>
#include <cstdlib>
#include "../Global.hpp"
#include "../transform/DCT4.hpp"
#include "../transform/DCT8.hpp"
#include "../transform/DCT16.hpp"
#include "../transform/DCT32.hpp"
#include "../transform/DST4.hpp"

using namespace std;
using namespace kanzi;

void testTransformsCorrectness()
{
    int block[] = {
        3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3,
        2, 3, 8, 4, 6, 2, 6, 4, 3, 3, 8, 3, 2, 7, 9, 5,
        0, 2, 8, 8, 4, 1, 9, 7, 1, 6, 9, 3, 9, 9, 3, 7,
        5, 1, 0, 5, 8, 2, 0, 9, 7, 4, 9, 4, 4, 5, 9, 2,
        3, 0, 7, 8, 1, 6, 4, 0, 6, 2, 8, 6, 2, 0, 8, 9,
        9, 8, 6, 2, 8, 0, 3, 4, 8, 2, 5, 3, 4, 2, 1, 1,
        7, 0, 6, 7, 9, 8, 2, 1, 4, 8, 0, 8, 6, 5, 1, 3,
        2, 8, 2, 3, 0, 6, 6, 4, 7, 0, 9, 3, 8, 4, 4, 6,
        0, 9, 5, 5, 0, 5, 8, 2, 2, 3, 1, 7, 2, 5, 3, 5,
        9, 4, 0, 8, 1, 2, 8, 4, 8, 1, 1, 1, 7, 4, 5, 0,
        2, 8, 4, 1, 0, 2, 7, 0, 1, 9, 3, 8, 5, 2, 1, 1,
        0, 5, 5, 5, 9, 6, 4, 4, 6, 2, 2, 9, 4, 8, 9, 5,
        4, 9, 3, 0, 3, 8, 1, 9, 6, 4, 4, 2, 8, 8, 1, 0,
        9, 7, 5, 6, 6, 5, 9, 3, 3, 4, 4, 6, 1, 2, 8, 4,
        7, 5, 6, 4, 8, 2, 3, 3, 7, 8, 6, 7, 8, 3, 1, 6,
        5, 2, 7, 1, 2, 0, 1, 9, 0, 9, 1, 4, 5, 6, 4, 8,
        5, 6, 6, 9, 2, 3, 4, 6, 0, 3, 4, 8, 6, 1, 0, 4,
        5, 4, 3, 2, 6, 6, 4, 8, 2, 1, 3, 3, 9, 3, 6, 0,
        7, 2, 6, 0, 2, 4, 9, 1, 4, 1, 2, 7, 3, 7, 2, 4,
        5, 8, 7, 0, 0, 6, 6, 0, 6, 3, 1, 5, 5, 8, 8, 1,
        7, 4, 8, 8, 1, 5, 2, 0, 9, 2, 0, 9, 6, 2, 8, 2,
        9, 2, 5, 4, 0, 9, 1, 7, 1, 5, 3, 6, 4, 3, 6, 7,
        8, 9, 2, 5, 9, 0, 3, 6, 0, 0, 1, 1, 3, 3, 0, 5,
        3, 0, 5, 4, 8, 8, 2, 0, 4, 6, 6, 5, 2, 1, 3, 8,
        4, 1, 4, 6, 9, 5, 1, 9, 4, 1, 5, 1, 1, 6, 0, 9,
        4, 3, 3, 0, 5, 7, 2, 7, 0, 3, 6, 5, 7, 5, 9, 5,
        9, 1, 9, 5, 3, 0, 9, 2, 1, 8, 6, 1, 1, 7, 3, 8,
        1, 9, 3, 2, 6, 1, 1, 7, 9, 3, 1, 0, 5, 1, 1, 8,
        5, 4, 8, 0, 7, 4, 4, 6, 2, 3, 7, 9, 9, 6, 2, 7,
        4, 9, 5, 6, 7, 3, 5, 1, 8, 8, 5, 7, 5, 2, 7, 2,
        4, 8, 9, 1, 2, 2, 7, 9, 3, 8, 1, 8, 3, 0, 1, 1,
        9, 4, 9, 1, 2, 9, 8, 3, 3, 6, 7, 3, 3, 6, 2, 4,
        4, 0, 6, 5, 6, 6, 4, 3, 0, 8, 6, 0, 2, 1, 3, 9,
        4, 9, 4, 6, 3, 9, 5, 2, 2, 4, 7, 3, 7, 1, 9, 0,
        7, 0, 2, 1, 7, 9, 8, 6, 0, 9, 4, 3, 7, 0, 2, 7,
        7, 0, 5, 3, 9, 2, 1, 7, 1, 7, 6, 2, 9, 3, 1, 7,
        6, 7, 5, 2, 3, 8, 4, 6, 7, 4, 8, 1, 8, 4, 6, 7,
        6, 6, 9, 4, 0, 5, 1, 3, 2, 0, 0, 0, 5, 6, 8, 1,
        2, 7, 1, 4, 5, 2, 6, 3, 5, 6, 0, 8, 2, 7, 7, 8,
        5, 7, 7, 1, 3, 4, 2, 7, 5, 7, 7, 8, 9, 6, 0, 9,
        1, 7, 3, 6, 3, 7, 1, 7, 8, 7, 2, 1, 4, 6, 8, 4,
        4, 0, 9, 0, 1, 2, 2, 4, 9, 5, 3, 4, 3, 0, 1, 4,
        6, 5, 4, 9, 5, 8, 5, 3, 7, 1, 0, 5, 0, 7, 9, 2,
        2, 7, 9, 6, 8, 9, 2, 5, 8, 9, 2, 3, 5, 4, 2, 0,
        1, 9, 9, 5, 6, 1, 1, 2, 1, 2, 9, 0, 2, 1, 9, 6,
        0, 8, 6, 4, 0, 3, 4, 4, 1, 8, 1, 5, 9, 8, 1, 3,
        6, 2, 9, 7, 7, 4, 7, 7, 1, 3, 0, 9, 9, 6, 0, 5,
        1, 8, 7, 0, 7, 2, 1, 1, 3, 4, 9, 9, 9, 9, 9, 9,
        8, 3, 7, 2, 9, 7, 8, 0, 4, 9, 9, 5, 1, 0, 5, 9,
        7, 3, 1, 7, 3, 2, 8, 1, 6, 0, 9, 6, 3, 1, 8, 5,
        9, 5, 0, 2, 4, 4, 5, 9, 4, 5, 5, 3, 4, 6, 9, 0,
        8, 3, 0, 2, 6, 4, 2, 5, 2, 2, 3, 0, 8, 2, 5, 3,
        3, 4, 4, 6, 8, 5, 0, 3, 5, 2, 6, 1, 9, 3, 1, 1,
        8, 8, 1, 7, 1, 0, 1, 0, 0, 0, 3, 1, 3, 7, 8, 3,
        8, 7, 5, 2, 8, 8, 6, 5, 8, 7, 5, 3, 3, 2, 0, 8,
        3, 8, 1, 4, 2, 0, 6, 1, 7, 1, 7, 7, 6, 6, 9, 1,
        4, 7, 3, 0, 3, 5, 9, 8, 2, 5, 3, 4, 9, 0, 4, 2,
        8, 7, 5, 5, 4, 6, 8, 7, 3, 1, 1, 5, 9, 5, 6, 2,
        8, 6, 3, 8, 8, 2, 3, 5, 3, 7, 8, 7, 5, 9, 3, 7,
        5, 1, 9, 5, 7, 7, 8, 1, 8, 5, 7, 7, 8, 0, 5, 3,
        2, 1, 7, 1, 2, 2, 6, 8, 0, 6, 6, 1, 3, 0, 0, 1,
        9, 2, 7, 8, 7, 6, 6, 1, 1, 1, 9, 5, 9, 0, 9, 2,
        1, 6, 4, 2, 0, 1, 9, 8, 9, 3, 8, 0, 9, 5, 2, 5,
        7, 2, 0, 1, 0, 6, 5, 4, 8, 5, 8, 6, 3, 2, 7, 8
    };

    // Test correctness (byte aligned)

    Transform<int>* pTransforms[] = { new DCT4(), new DCT8(), new DCT16(), new DCT32(), new DST4() };
    int dims[] = { 4, 8, 16, 32, 4 };
    string names[] = { "DCT", "DCT", "DCT", "DCT", "DST" };
    srand((uint)time(nullptr));

    for (int idx = 0; idx < 5; idx++) {
        int dim = dims[idx];
        cout << endl
             << names[idx] << dim << " correctness" << endl;
        int blockSize = dim * dim;
        int* data1 = new int[blockSize + 20];
        int* data2 = new int[blockSize + 20];
        int* data3 = new int[blockSize + 20];

        for (int nn = 0; nn < 20; nn++) {
            cout << names[idx] << dim << " - input " << nn << ":" << endl;

            for (int i = 0; i < blockSize; i++) {
                if (nn == 0)
                    data1[i] = block[i];
                else
                    data1[i] = rand() % (nn * 10);

                cout << data1[i] << " ";
            }

            int start = (nn & 1) * nn;

            SliceArray<int> ia1(data1, blockSize + 20, 0);
            SliceArray<int> ia2(data2, blockSize + 20, start);
            pTransforms[idx]->forward(ia1, ia2, dim * dim);
            cout << endl
                 << "Output:"
                 << endl;

            for (int i = start; i < start+blockSize; i++) {
                cout << data2[i] << " ";
            }

            ia2._index = start;
            SliceArray<int> ia3(data3, blockSize + 20, 0);
            pTransforms[idx]->inverse(ia2, ia3, dim * dim);
            cout << endl
                 << "Result:"
                 << endl;
            int sad = 0;

            for (int i = 0; i < blockSize; i++) {
                cout << data3[i];
                sad += abs(data1[i] - data3[i]);
                cout << ((data3[i] != data1[i]) ? "! " : "= ");
            }

            cout << endl
                 << "SAD: " << sad << endl;
        }

        delete[] data1;
        delete[] data2;
        delete[] data3;
        delete pTransforms[idx];
    }
}

void testTransformsSpeed()
{
    // Test speed
    cout << "\nDCT8 speed" << endl;
    double delta1 = 0;
    double delta2 = 0;
    int iter = 500000;
    srand((uint)time(nullptr));

    for (int times = 0; times < 100; times++) {
        int data[1000][64];
        DCT8 dct;

        for (int i = 0; i < 1000; i++) {
            for (int j = 0; j < 64; j++)
                data[i][j] = rand() % (10 + i + j * 10);
        }

        clock_t before, after;

        for (int i = 0; i < iter; i++) {
            int* pData = reinterpret_cast<int*>(&data[i % 100]);
            SliceArray<int> ia1(pData, 64, 0);
            before = clock();
            dct.forward(ia1, ia1, 64);
            after = clock();
            delta1 += (after - before);
            SliceArray<int> ia3(pData, 64, 0);
            pData = reinterpret_cast<int*>(&data[i % 100]);
            before = clock();
            dct.inverse(ia3, ia3, 64);
            after = clock();
            delta2 += (after - before);
        }
    }

    cout << "Iterations: " << iter * 100 << endl;
    cout << "Forward [ms]: " << (int)(delta1 / CLOCKS_PER_SEC * 1000) << endl;
    cout << "Inverse [ms]: " << (int)(delta2 / CLOCKS_PER_SEC * 1000) << endl;
}

#ifdef __GNUG__
int main()
#else
int TestTransforms_main()
#endif
{
    testTransformsCorrectness();
    testTransformsSpeed();
    return 0;
}
