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
#include "../bitstream/DebugInputBitStream.hpp"
#include "../bitstream/DebugOutputBitStream.hpp"
#include "../bitstream/DefaultInputBitStream.hpp"
#include "../bitstream/DefaultOutputBitStream.hpp"

using namespace std;
using namespace kanzi;

void testBitStreamCorrectnessAligned()
{
    // Test correctness (byte aligned)
    cout << "Correctness Test - byte aligned" << endl;
    const int length = 100;
    int* values = new int[length];
    srand((uint)time(nullptr));
    cout << "\nInitial" << endl;

    for (int test = 0; test < 10; test++) {
        stringbuf buffer;
        iostream ios(&buffer);
        DefaultOutputBitStream obs(ios, 16384);
        DebugOutputBitStream dbs(obs, cout);
        dbs.showByte(true);

        for (int i = 0; i < length; i++) {
            values[i] = rand();
            cout << (int)values[i] << " ";

            if ((i % 50) == 49)
                cout << endl;
        }

        cout << endl
             << endl;

        for (int i = 0; i < length; i++) {
            dbs.writeBits(values[i], 32);
        }

        // Close first to force flush()
        dbs.close();
        ios.rdbuf()->pubseekpos(0);
        istringstream is;
        char* cvalues = new char[4 * length];

        for (int i = 0; i < length; i++) {
            cvalues[4 * i] = (values[i] >> 24) & 0xFF;
            cvalues[4 * i + 1] = (values[i] >> 16) & 0xFF;
            cvalues[4 * i + 2] = (values[i] >> 8) & 0xFF;
            cvalues[4 * i + 3] = (values[i] >> 0) & 0xFF;
        }

        is.read(cvalues, length);

        DefaultInputBitStream ibs(ios, 16384);
        cout << endl
             << endl
             << "Read:" << endl;
        bool ok = true;

        for (int i = 0; i < length; i++) {
            int x = (int)ibs.readBits(32);
            cout << x;
            cout << ((x == values[i]) ? " " : "* ");
            ok &= (x == values[i]);

            if ((i % 50) == 49)
                cout << endl;
        }

        delete[] cvalues;
        ibs.close();
        cout << endl;
        cout << endl
             << "Bits written: " << dbs.written() << endl;
        cout << endl
             << "Bits read: " << ibs.read() << endl;
        cout << endl
             << "\n" << ((ok) ? "Success" : "Failure") << endl;
        cout << endl;
        cout << endl;
    }

    delete[] values;
}

void testBitStreamSpeed(const string& fileName)
{
    // Test speed
    cout << "\nSpeed Test" << endl;

    int values[] = { 3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3,
        31, 14, 41, 15, 59, 92, 26, 65, 53, 35, 58, 89, 97, 79, 93, 32 };

    int iter = 150;
    uint64 written = 0;
    uint64 read = 0;
    double delta1 = 0, delta2 = 0;
    int nn = 100000 * 32;

    for (int test = 1; test <= iter; test++) {
        ofstream os(fileName.c_str(), std::ofstream::binary);
        DefaultOutputBitStream obs(os, 1024 * 1024);
        clock_t before = clock();

        for (int i = 0; i < nn; i++) {
            obs.writeBits((uint64)values[i % 32], 1 + (i & 63));
        }

        // Close first to force flush()
        obs.close();
        os.close();
        clock_t after = clock();
        delta1 += (after - before);
        written += obs.written();

        ifstream is(fileName.c_str(), std::ifstream::binary);
        DefaultInputBitStream ibs(is, 1024 * 1024);
        before = clock();

        for (int i = 0; i < nn; i++) {
            ibs.readBits(1 + (i & 63));
        }

        ibs.close();
        is.close();
        after = clock();
        delta2 += (after - before);
        read += ibs.read();
    }

    double d = 1024.0 * 8192.0;
    //cout << delta1 << " " << delta2 << endl;
    cout << written << " bits written (" << (written / 1024 / 1024 / 8) << " MB)" << endl;
    cout << read << " bits read (" << (read / 1024 / 1024 / 8) << " MB)" << endl;
    cout << endl;
    cout << "Write [ms]        : " << (int)(delta1 / CLOCKS_PER_SEC * 1000) << endl;
    cout << "Throughput [MB/s] : " << (int)((double)written / d / (delta1 / CLOCKS_PER_SEC)) << endl;
    cout << "Read [ms]         : " << (int)(delta2 / CLOCKS_PER_SEC * 1000) << endl;
    cout << "Throughput [MB/s] : " << (int)((double)read / d / (delta2 / CLOCKS_PER_SEC)) << endl;
}

#ifdef __GNUG__
int main(int argc, const char* argv[])
#else
int TestDefaultBitStream_main(int argc, const char* argv[])
#endif
{
    testBitStreamCorrectnessAligned();

    string fileName;
    fileName = (argc > 1) ? argv[1] :  "r:\\output.bin";
    testBitStreamSpeed(fileName);
    return 0;
}
