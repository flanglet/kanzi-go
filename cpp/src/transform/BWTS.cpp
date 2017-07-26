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

#include <cstring>
#include "BWTS.hpp"

using namespace kanzi;

bool BWTS::forward(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if ((count < 0) || (count + input._index > input._length))
        return false;

    if (count > maxBlockSize())
        return false;

    if (count < 2) {
        if (count == 1)
            output._array[output._index++] = input._array[input._index++];

        return true;
    }

    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];

    // Lazy dynamic memory allocation
    if (_bufferSize < count) {
        _bufferSize = count;
        delete[] _buffer1;
        _buffer1 = new int[_bufferSize];
        delete[] _buffer2;
        _buffer2 = new int[_bufferSize];
    }

    // Aliasing
    int* sa = _buffer1;
    int* isa = _buffer2;

    _saAlgo.computeSuffixArray(src, sa, 0, count);

    for (int i = 0; i < count; i++)
        isa[sa[i]] = i;

    int min = isa[0];
    int idxMin = 0;

    for (int i = 1; ((i < count) && (min > 0)); i++) {
        if (isa[i] >= min)
            continue;

        int refRank = moveLyndonWordHead(sa, isa, src, count, idxMin, i - idxMin, min);

        for (int j = i - 1; j > idxMin; j--) {
            // Iterate through the new Lyndon word from end to start
            int testRank = isa[j];
            int startRank = testRank;

            while (testRank < count - 1) {
                int nextRankStart = sa[testRank + 1];

                if ((j > nextRankStart) || (src[j] != src[nextRankStart])
                    || (refRank < isa[nextRankStart + 1]))
                    break;

                sa[testRank] = nextRankStart;
                isa[nextRankStart] = testRank;
                testRank++;
            }

            sa[testRank] = j;
            isa[j] = testRank;
            refRank = testRank;

            if (startRank == testRank)
                break;
        }

        min = isa[i];
        idxMin = i;
    }

    min = count;

    for (int i = 0; i < count; i++) {
        if (isa[i] >= min) {
            dst[isa[i]] = src[i - 1];
            continue;
        }

        if (min < count)
            dst[min] = src[i - 1];

        min = isa[i];
    }

    dst[0] = src[count - 1];
    input._index += count;
    output._index += count;
    return true;
}

int BWTS::moveLyndonWordHead(int sa[], int isa[], byte data[], int count, int start, int size, int rank)
{
    const int end = start + size;

    while (rank + 1 < count) {
        const int nextStart0 = sa[rank + 1];

        if (nextStart0 <= end)
            break;

        int nextStart = nextStart0;
        int k = 0;

        while ((k < size) && (nextStart < count) && (data[start + k] == data[nextStart])) {
            k++;
            nextStart++;
        }

        if ((k == size) && (rank < isa[nextStart]))
            break;

        if ((k < size) && (nextStart < count) && ((data[start + k] & 0xFF) < (data[nextStart] & 0xFF)))
            break;

        sa[rank] = nextStart0;
        isa[nextStart0] = rank;
        rank++;
    }

    sa[rank] = start;
    isa[start] = rank;
    return rank;
}

bool BWTS::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if ((count < 0) || (count + input._index > input._length))
        return false;

    if (count > maxBlockSize())
        return false;

    if (count < 2) {
        if (count == 1)
            output._array[output._index++] = input._array[input._index++];

        return true;
    }

    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];

    // Lazy dynamic memory allocation
    if (_bufferSize < count) {
        _bufferSize = count;
        delete[] _buffer1;
        _buffer1 = new int[_bufferSize];
    }

    // Aliasing
    int* buckets_ = _buckets;
    int* lf = _buffer1;

    // Initialize histogram
    memset(_buckets, 0, sizeof(_buckets));

    for (int i = 0; i < count; i++)
        buckets_[src[i] & 0xFF]++;

    // Histogram
    for (int i = 0, sum = 0; i < 256; i++) {
        sum += buckets_[i];
        buckets_[i] = sum - buckets_[i];
    }

    for (int i = 0; i < count; i++)
        lf[i] = buckets_[src[i] & 0xFF]++;

    // Build inverse
    for (int i = 0, j = count - 1; j >= 0; i++) {
        if (lf[i] < 0)
            continue;

        int p = i;

        do {
            dst[j] = src[p];
            j--;
            const int t = lf[p];
            lf[p] = -1;
            p = t;
        } while (lf[p] >= 0);
    }

    input._index += count;
    output._index += count;
    return true;
}
