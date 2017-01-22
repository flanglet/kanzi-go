
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

using namespace kanzi;

bool BWT::setPrimaryIndex(int primaryIndex)
{
    if (primaryIndex < 0)
        return false;

    _primaryIndex = primaryIndex;
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
    _saAlgo.reset();

    // Compute suffix array
    int* sa = _saAlgo.computeSuffixArray(src, 0, count);

    int i = 0;

    for (; i < count; i++) {
        // Found primary index
        if (sa[i] == 0)
            break;

        dst[i] = src[sa[i] - 1];
    }

    dst[i] = src[count - 1];
    setPrimaryIndex(i);

    for (i++; i < count; i++)
        dst[i] = src[sa[i] - 1];

    input._index += count;
    output._index += count;
    return true;
}

bool BWT::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
        return false;

    if ((count < 0) || (count + input._index > input._length))
        return false;

    if (count > maxBlockSize())
        return false;

    if (getPrimaryIndex() >= count)
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
    if (_bufferSize < count) {
        _bufferSize = count;
        delete[] _buffer1;
        _buffer1 = new int[_bufferSize];
    }

    // Aliasing
    int* buckets_ = _buckets;
    int* data = _buffer1;

    // Initialize histogram
    memset(_buckets, 0, sizeof(_buckets));

    // Build array of packed index + value (assumes block size < 2^24)
    // Start with the primary index position
    const int pIdx = getPrimaryIndex();
    const int val0 = src[pIdx] & 0xFF;
    data[pIdx] = val0;
    buckets_[val0]++;

    for (int i = 0; i < pIdx; i++) {
        const int val = src[i] & 0xFF;
        data[i] = (buckets_[val] << 8) | val;
        buckets_[val]++;
    }

    for (int i = pIdx + 1; i < count; i++) {
        const int val = src[i] & 0xFF;
        data[i] = (buckets_[val] << 8) | val;
        buckets_[val]++;
    }

    // Create cumulative histogram
    for (int i = 0, sum = 0; i < 256; i++) {
        const int tmp = buckets_[i];
        buckets_[i] = sum;
        sum += tmp;
    }

    uint ptr = data[pIdx];
    dst[count - 1] = byte(ptr);

    // Build inverse
    for (int i = count - 2; i >= 0; i--) {
        ptr = data[(ptr >> 8) + buckets_[ptr & 0xFF]];
        dst[i] = byte(ptr);
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
    if (_bufferSize < count) {
        _bufferSize = count;
        delete[] _buffer1;
        _buffer1 = new int[_bufferSize];
        delete[] _buffer2;
        _buffer2 = new byte[_bufferSize];
    }

    // Aliasing
    int* buckets_ = _buckets;
    int* data1 = _buffer1;
    byte* data2 = _buffer2;

    // Initialize histogram
    memset(_buckets, 0, sizeof(_buckets));

    // Build arrays
    // Start with the primary index position
    const int pIdx = getPrimaryIndex();
    const byte val0 = src[pIdx];
    data1[pIdx] = buckets_[val0 & 0xFF];
    data2[pIdx] = val0;
    buckets_[val0 & 0xFF]++;

    for (int i = 0; i < pIdx; i++) {
        const byte val = src[i];
        data1[i] = buckets_[val & 0xFF];
        data2[i] = val;
        buckets_[val & 0xFF]++;
    }

    for (int i = pIdx + 1; i < count; i++) {
        const byte val = src[i];
        data1[i] = buckets_[val & 0xFF];
        data2[i] = val;
        buckets_[val & 0xFF]++;
    }

    // Create cumulative histogram
    for (int i = 0, sum = 0; i < 256; i++) {
        const int tmp = buckets_[i];
        buckets_[i] = sum;
        sum += tmp;
    }

    int val1 = data1[pIdx];
    byte val2 = data2[pIdx];
    dst[count - 1] = val2;

    // Build inverse
    for (int i = count - 2; i >= 0; i--) {
        const int idx = val1 + buckets_[val2 & 0xFF];
        val1 = data1[idx];
        val2 = data2[idx];
        dst[i] = val2;
    }

    input._index += count;
    output._index += count;
    return true;
}
