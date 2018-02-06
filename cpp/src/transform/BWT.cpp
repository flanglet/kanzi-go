
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
#include "BWT.hpp"
#include <cstdio>
using namespace kanzi;

bool BWT::setPrimaryIndex(int n, int primaryIndex)
{
    if ((primaryIndex < 0) || (n < 0) || (n >= 9))
        return false;

    _primaryIndexes[n] = primaryIndex;
    return true;
}

bool BWT::forward(SliceArray<byte>& input, SliceArray<byte>& output, int count)
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
    if ((_buffer3 == nullptr) || (_bufferSize < count)) {
        if (_buffer3 != nullptr)
           delete[] _buffer3;

        _bufferSize = count;
        _buffer3 = new int[_bufferSize];
    }

    int* sa = _buffer3;
    _saAlgo.computeSuffixArray(src, sa, 0, count);
    int n = 0;
    const int chunks = getBWTChunks(count);
    bool res = true;

    if (chunks == 1) {
        for (; n < count; n++) {
           if (sa[n] == 0) {
                res &= setPrimaryIndex(0, n);
                break;
            }

            dst[n] = src[sa[n] - 1];
        }

        dst[n] = src[count - 1];
        n++;

        for (; n < count; n++)
            dst[n] = src[sa[n] - 1];
    }
    else {
        const int step = count / chunks;

        for (; n < count; n++) {
            if ((sa[n] % step) == 0) {
                res &= setPrimaryIndex(sa[n] / step, n);

                if (sa[n] == 0)
                    break;
            }

            dst[n] = src[sa[n] - 1];
        }

        dst[n] = src[count - 1];
        n++;

        for (; n < count; n++) {
            if ((sa[n] % step) == 0)
                res &= setPrimaryIndex(sa[n] / step, n);

            dst[n] = src[sa[n] - 1];
        }
    }

    input._index += count;
    output._index += count;
    return res;
}

bool BWT::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int count)
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

    if (count >= 1 << 24)
        return inverseBigBlock(input, output, count);

    return inverseRegularBlock(input, output, count);
}

// When count < 1<<24
bool BWT::inverseRegularBlock(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];

    // Lazy dynamic memory allocation
    if ((_buffer1 == nullptr) || (_bufferSize < count)) {
        if (_buffer1 != nullptr)
            delete[] _buffer1;

        _bufferSize = count;
        _buffer1 = new uint32[_bufferSize];
    }

    // Aliasing
    uint32* buckets_ = _buckets;
    uint32* data = _buffer1;

    int chunks = getBWTChunks(count);

    // Initialize histogram
    memset(_buckets, 0, sizeof(_buckets));

    // Build array of packed index + value (assumes block size < 2^24)
    // Start with the primary index position
    int pIdx = getPrimaryIndex(0);
    const uint8 val0 = src[pIdx];
    data[pIdx] = val0;
    buckets_[val0]++;

    for (int i = 0; i < pIdx; i++) {
        const uint8 val = src[i];
        data[i] = (buckets_[val] << 8) | val;
        buckets_[val]++;
    }

    for (int i = pIdx + 1; i < count; i++) {
        const uint8 val = src[i];
        data[i] = (buckets_[val] << 8) | val;
        buckets_[val]++;
    }

    uint32 sum = 0;

    // Create cumulative histogram
    for (int i = 0; i < 256; i++) {
        sum += buckets_[i];
        buckets_[i] = sum - buckets_[i];
    }

    int idx = count - 1;

    // Build inverse
    if (chunks == 1) {
        uint32 ptr = data[pIdx];
        dst[idx--] = byte(ptr);

        for (; idx >= 0; idx--) {
            ptr = data[(ptr >> 8) + buckets_[ptr & 0xFF]];
            dst[idx] = byte(ptr);
        }
    }
    else {
        const int step = count / chunks;

        for (int i = chunks - 1; i >= 0; i--) {
            uint32 ptr = data[pIdx];
            dst[idx--] = byte(ptr);
            const int endChunk = i * step;

            for (; idx >= endChunk; idx--) {
                ptr = data[(ptr >> 8) + buckets_[ptr & 0xFF]];
                dst[idx] = byte(ptr);
            }

            pIdx = getPrimaryIndex(i);
            idx = endChunk - 1;
        }
    }

    input._index += count;
    output._index += count;
    return true;
}

// When count >= 1<<24
bool BWT::inverseBigBlock(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];

    // Lazy dynamic memory allocations
    if ((_buffer1 == nullptr) || (_bufferSize < count)) {
        if (_buffer1 != nullptr)
           delete[] _buffer1;

        if (_buffer2 != nullptr)
            delete[] _buffer2;

        _bufferSize = count;
        _buffer1 = new uint32[_bufferSize];
        _buffer2 = new byte[_bufferSize];
    }

    // Aliasing
    uint32* buckets_ = _buckets;
    uint32* data1 = _buffer1;
    byte* data2 = _buffer2;

    int chunks = getBWTChunks(count);

    // Initialize histogram
    memset(_buckets, 0, sizeof(_buckets));

    // Build arrays
    // Start with the primary index position
    int pIdx = getPrimaryIndex(0);
    const uint8 val0 = src[pIdx];
    data1[pIdx] = buckets_[val0];
    data2[pIdx] = val0;
    buckets_[val0]++;

    for (int i = 0; i < pIdx; i++) {
        const uint8 val = src[i];
        data1[i] = buckets_[val];
        data2[i] = val;
        buckets_[val]++;
    }

    for (int i = pIdx + 1; i < count; i++) {
        const uint8 val = src[i];
        data1[i] = buckets_[val];
        data2[i] = val;
        buckets_[val]++;
    }

    uint32 sum = 0;

    // Create cumulative histogram
    for (int i = 0; i < 256; i++) {
        sum += buckets_[i];
        buckets_[i] = sum - buckets_[i];
    }

    int idx = count - 1;

    // Build inverse
    if (chunks == 1) {
        uint32 val1 = data1[pIdx];
        uint8 val2 = data2[pIdx];
        dst[idx--] = val2;

        for (; idx >= 0; idx--) {
            const int n = val1 + buckets_[val2];
            val1 = data1[n];
            val2 = data2[n];
            dst[idx] = val2;
        }
    }
    else {
        const int step = count / chunks;

        for (int i = chunks - 1; i >= 0; i--) {
            uint32 val1 = data1[pIdx];
            uint8 val2 = data2[pIdx];
            dst[idx--] = val2;
            const int endChunk = i * step;

            for (; idx >= endChunk; idx--) {
                const int n = val1 + buckets_[val2];
                val1 = data1[n];
                val2 = data2[n];
                dst[idx] = val2;
            }

            pIdx = getPrimaryIndex(i);
            idx = endChunk - 1;
        }
    }

    input._index += count;
    output._index += count;
    return true;
}

int BWT::getBWTChunks(int)
{
    // For now, return 1 always !!!!
    return 1;
    //       int log = 0;
    //       size >>= 10;
    //
    //       while ((size>0) && (log<3))
    //       {
    //          size >>= 2;
    //          log++;
    //       }
    //
    //       return 1<<log;
}
