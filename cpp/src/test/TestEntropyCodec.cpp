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
#include <algorithm>
#include "../types.hpp"
#include "../entropy/HuffmanEncoder.hpp"
#include "../entropy/RangeEncoder.hpp"
#include "../entropy/ANSRangeEncoder.hpp"
#include "../entropy/BinaryEntropyEncoder.hpp"
#include "../entropy/ExpGolombEncoder.hpp"
#include "../entropy/RiceGolombEncoder.hpp"
#include "../bitstream/DefaultOutputBitStream.hpp"
#include "../bitstream/DefaultInputBitStream.hpp"
#include "../bitstream/DebugOutputBitStream.hpp"
#include "../bitstream/DebugInputBitStream.hpp"
#include "../entropy/HuffmanDecoder.hpp"
#include "../entropy/RangeDecoder.hpp"
#include "../entropy/ANSRangeDecoder.hpp"
#include "../entropy/BinaryEntropyDecoder.hpp"
#include "../entropy/ExpGolombDecoder.hpp"
#include "../entropy/RiceGolombDecoder.hpp"
#include "../entropy/FPAQPredictor.hpp"
#include "../entropy/CMPredictor.hpp"
#include "../entropy/PAQPredictor.hpp"
#include "../entropy/TPAQPredictor.hpp"

using namespace kanzi;

static Predictor* getPredictor(string type)
{
    if (type.compare("PAQ") == 0)
        return new PAQPredictor();

    if (type.compare("TPAQ") == 0)
        return new TPAQPredictor();

    if (type.compare("FPAQ") == 0)
        return new FPAQPredictor();

    if (type.compare("CM") == 0)
        return new CMPredictor();

    return nullptr;
}

static EntropyEncoder* getEncoder(string name, OutputBitStream& obs, Predictor* predictor)
{
    if (name.compare("HUFFMAN") == 0)
        return new HuffmanEncoder(obs);

    if (name.compare("ANS0") == 0)
        return new ANSRangeEncoder(obs, 0);

    if (name.compare("ANS1") == 0)
        return new ANSRangeEncoder(obs, 1);

    if (name.compare("RANGE") == 0)
        return new RangeEncoder(obs);

    if (name.compare("EXPGOLOMB") == 0)
        return new ExpGolombEncoder(obs);

    if (name.compare("RICEGOLOMB") == 0)
        return new RiceGolombEncoder(obs, 4);

    if (predictor != nullptr) {
       if (name.compare("PAQ") == 0)
           return new BinaryEntropyEncoder(obs, predictor, false);

       if (name.compare("TPAQ") == 0)
           return new BinaryEntropyEncoder(obs, predictor, false);

       if (name.compare("FPAQ") == 0)
           return new BinaryEntropyEncoder(obs, predictor, false);

       if (name.compare("CM") == 0)
           return new BinaryEntropyEncoder(obs, predictor, false);
    }

    cout << "No such entropy encoder: " << name << endl;
    return nullptr;
}

static EntropyDecoder* getDecoder(string name, InputBitStream& ibs, Predictor* predictor)
{
    if (name.compare("HUFFMAN") == 0)
        return new HuffmanDecoder(ibs);

    if (name.compare("ANS0") == 0)
        return new ANSRangeDecoder(ibs, 0);

    if (name.compare("ANS1") == 0)
        return new ANSRangeDecoder(ibs, 1);

    if (name.compare("RANGE") == 0)
        return new RangeDecoder(ibs);

    if (name.compare("PAQ") == 0)
        return new BinaryEntropyDecoder(ibs, predictor, false);

    if (name.compare("TPAQ") == 0)
        return new BinaryEntropyDecoder(ibs, predictor, false);

    if (name.compare("FPAQ") == 0)
        return new BinaryEntropyDecoder(ibs, predictor, false);

    if (name.compare("CM") == 0)
        return new BinaryEntropyDecoder(ibs, predictor, false);

    if (name.compare("EXPGOLOMB") == 0)
        return new ExpGolombDecoder(ibs);

    if (name.compare("RICEGOLOMB") == 0)
        return new RiceGolombDecoder(ibs, 4);

    cout << "No such entropy decoder: " << name << endl;
    return nullptr;
}

void testEntropyCodecCorrectness(const string& name)
{
    // Test behavior
    cout << "Correctness test for " << name << endl;
    srand((uint)time(nullptr));

    for (int ii = 1; ii < 20; ii++) {
        cout << endl
             << endl
             << "Test " << ii << endl;
        byte val[32];
        int size = 32;

        if (ii == 3) {
            byte val2[] = { 0, 0, 32, 15, (byte)-4, 16, 0, 16, 0, 7, (byte)-1, (byte)-4, (byte)-32, 0, 31, (byte)-1 };
            size = 16;
            memcpy(val, &val2[0], size);
        }
        else if (ii == 2) {
            byte val2[] = { 0x3d, 0x4d, 0x54, 0x47, 0x5a, 0x36, 0x39, 0x26, 0x72, 0x6f, 0x6c, 0x65, 0x3d, 0x70, 0x72, 0x65 };
            size = 16;
            memcpy(val, &val2[0], size);
        }
        else if (ii == 4) {
            byte val2[] = { 65, 71, 74, 66, 76, 65, 69, 77, 74, 79, 68, 75, 73, 72, 77, 68, 78, 65, 79, 79, 78, 66, 77, 71, 64, 70, 74, 77, 64, 67, 71, 64 };
            memcpy(val, &val2[0], size);
        }
        else if (ii == 1) {
            for (int i = 0; i < 32; i++)
                val[i] = (byte)2; // all identical
        }
        else if (ii == 5) {
            for (int i = 0; i < 32; i++)
                val[i] = (byte)(2 + (i & 1)); // 2 symbols
        }
        else {
            for (int i = 0; i < 32; i++)
                val[i] = (byte)(64 + 3 * ii + (rand() % (ii + 1)));
        }

        byte* values = &val[0];
        cout << "Original:" << endl;

        for (int i = 0; i < size; i++)
            cout << (int)values[i] << " ";

        cout << endl
             << endl
             << "Encoded:" << endl;
        stringbuf buffer;
        iostream ios(&buffer);
        DefaultOutputBitStream obs(ios);
        DebugOutputBitStream dbgobs(obs);
        dbgobs.showByte(true);

        EntropyEncoder* ec = getEncoder(name, dbgobs, getPredictor(name));

        if (ec == nullptr)
           exit(1);

        ec->encode(values, 0, size);
        ec->dispose();
        delete ec;
        dbgobs.close();
        ios.rdbuf()->pubseekpos(0);

        DefaultInputBitStream ibs(ios);
        EntropyDecoder* ed = getDecoder(name, ibs, getPredictor(name));
        
        if (ec == nullptr)
           exit(1);

        cout << endl
             << endl
             << "Decoded:" << endl;
        bool ok = true;
        byte* values2 = new byte[size];
        ed->decode(values2, 0, size);
        ed->dispose();
        delete ed;
        ibs.close();

        for (int j = 0; j < size; j++) {
            if (values[j] != values2[j])
                ok = false;

            cout << (int)values2[j] << " ";
        }

        cout << endl;
        cout << ((ok) ? "Identical" : "Different") << endl;
        delete[] values2;
    }
}

int testEntropyCodecSpeed(const string& name)
{
    // Test speed
    cout << endl
         << endl
         << "Speed test for " << name << endl;
    int repeats[] = { 3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3 };
    int size = 500000;
    int iter = 100;
    srand((uint)time(nullptr));
    Predictor* predictor;
    byte values1[500000];
    byte values2[500000];

    for (int jj = 0; jj < 3; jj++) {
        cout << endl
             << "Test " << (jj + 1) << endl;
        double delta1 = 0, delta2 = 0;

        for (int ii = 0; ii < iter; ii++) {
            int idx = 0;

            for (int i = 0; i < size; i++) {
                int i0 = i;
                int len = repeats[idx];
                idx = (idx + 1) & 0x0F;
                byte b = (byte)(rand() % 255);

                if (i0 + len >= size)
                    len = size - i0 - 1;

                for (int j = i0; j < i0 + len; j++) {
                    values1[j] = b;
                    i++;
                }
            }

            // Encode
            stringbuf buffer;
            iostream ios(&buffer);
            DefaultOutputBitStream obs(ios, 16384);
            predictor = getPredictor(name);
            EntropyEncoder* ec = getEncoder(name, obs, predictor);
            
            if (ec == nullptr)
                 exit(1);

            clock_t before1 = clock();

            if (ec->encode(values1, 0, size) < 0) {
                cout << "Encoding error" << endl;
                delete ec;
                return 1;
            }

            ec->dispose();
            clock_t after1 = clock();
            delta1 += (after1 - before1);
            delete ec;
            obs.close();

            if (predictor)
                delete predictor;

            // Decode
            ios.rdbuf()->pubseekpos(0);
            DefaultInputBitStream ibs(ios, 16384);
            predictor = getPredictor(name);
            EntropyDecoder* ed = getDecoder(name, ibs, predictor);
            
            if (ed == nullptr)
                 exit(1);

            clock_t before2 = clock();

            if (ed->decode(values2, 0, size) < 0) {
                cout << "Decoding error" << endl;
                delete ed;
                return 1;
            }

            ed->dispose();
            clock_t after2 = clock();
            delta2 += (after2 - before2);
            delete ed;
            ibs.close();

            if (predictor)
                delete predictor;

            // Sanity check
            for (int i = 0; i < size; i++) {
                if (values1[i] != values2[i]) {
                    cout << "Error at index " << i << " (" << (int)values1[i] << "<->" << (int)values2[i] << ")" << endl;
                    break;
                }
            }
        }

        double prod = (double)iter * (double)size;
        double b2KB = (double)1 / (double)1024;
        double d1_sec = (double)delta1 / CLOCKS_PER_SEC;
        double d2_sec = (double)delta2 / CLOCKS_PER_SEC;
        cout << "Encode [ms]       : " << (int)(d1_sec * 1000) << endl;
        cout << "Throughput [KB/s] : " << (int)(prod * b2KB / d1_sec) << endl;
        cout << "Decode [ms]       : " << (int)(d2_sec * 1000) << endl;
        cout << "Throughput [KB/s] : " << (int)(prod * b2KB / d2_sec) << endl;
    }

    return 0;
}

#ifdef __GNUG__
int main(int argc, const char* argv[])
#else
int TestEntropyCodec_main(int argc, const char* argv[])
#endif
{
    try {
        string str;

        if (argc == 1) {
            str = "-TYPE=ALL";
        }
        else {
            str = argv[1];
        }

        transform(str.begin(), str.end(), str.begin(), ::toupper);

        if (str.compare(0, 6, "-TYPE=") == 0) {
            str = str.substr(6);

            if (str.compare("ALL") == 0) {
                cout << endl
                     << endl
                     << "TestHuffmanCodec" << endl;
                testEntropyCodecCorrectness("HUFFMAN");
                testEntropyCodecSpeed("HUFFMAN");
                cout << endl
                     << endl
                     << "TestANS0Codec" << endl;
                testEntropyCodecCorrectness("ANS0");
                testEntropyCodecSpeed("ANS0");
                cout << endl
                     << endl
                     << "TestANS1Codec" << endl;
                testEntropyCodecCorrectness("ANS1");
                testEntropyCodecSpeed("ANS1");
                cout << endl
                     << endl
                     << "TestRangCodec" << endl;
                testEntropyCodecCorrectness("RANGE");
                testEntropyCodecSpeed("RANGE");
                cout << endl
                     << endl
                     << "TestFPAQCodec" << endl;
                testEntropyCodecCorrectness("FPAQ");
                testEntropyCodecSpeed("FPAQ");
                cout << endl
                     << endl
                     << "TestCMCodec" << endl;
                testEntropyCodecCorrectness("CM");
                testEntropyCodecSpeed("CM");
                cout << endl
                     << endl
                     << "TestPAQCodec" << endl;
                testEntropyCodecCorrectness("PAQ");
                testEntropyCodecSpeed("PAQ");
                cout << endl
                     << endl
                     << "TestTPAQCodec" << endl;
                testEntropyCodecCorrectness("TPAQ");
                testEntropyCodecSpeed("TPAQ");
                cout << endl
                     << endl
                     << "TestExpGolombCodec" << endl;
                testEntropyCodecCorrectness("EXPGOLOMB");
                testEntropyCodecSpeed("EXPGOLOMB");
                cout << endl
                     << endl
                     << "TestRiceGolombCodec" << endl;
                testEntropyCodecCorrectness("RICEGOLOMB");
                testEntropyCodecSpeed("RICEGOLOMB");
            }
            else {
                cout << endl
                     << endl
                     << "Test" << str << "EntropyCodec" << endl;
                testEntropyCodecCorrectness(str);
                testEntropyCodecSpeed(str);
            }
        }
    }
    catch (exception& e) {
        cout << e.what() << endl;
    }
    return 0;
}
