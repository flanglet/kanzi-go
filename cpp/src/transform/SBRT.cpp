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
#include "SBRT.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

SBRT::SBRT(int mode)
{
    if ((mode != MODE_MTF) && (mode != MODE_RANK) && (mode != MODE_TIMESTAMP))
        throw IllegalArgumentException("Invalid mode parameter");

    _mode = mode;
    memset(_prev, 0, sizeof(int) * 256);
    memset(_curr, 0, sizeof(int) * 256);
    memset(_symbols, 0, sizeof(int) * 256);
    memset(_ranks, 0, sizeof(int) * 256);
}

bool SBRT::forward(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if (input._array == output._array)
        return false;

    if ((count < 0) || (count+input._index > input._length))
      return false;

    // Aliasing
    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];
    int* p = _prev;
    int* q = _curr;
    int* s2r = _symbols;
    int* r2s = _ranks;

    const int mask1 = (_mode == MODE_TIMESTAMP) ? 0 : -1;
    const int mask2 = (_mode == MODE_MTF) ? 0 : -1;
    const int shift = (_mode == MODE_RANK) ? 1 : 0;

    for (int i = 0; i < 256; i++) {
        p[i] = 0;
        q[i] = 0;
        s2r[i] = i;
        r2s[i] = i;
    }

    for (int i = 0; i < count; i++) {
        int c = src[i] & 0xFF;
        int r = s2r[c];
        dst[i] = (byte)r;
        q[c] = ((i & mask1) + (p[c] & mask2)) >> shift;
        p[c] = i;
        int curVal = q[c];

        // Move up symbol to correct rank
        while ((r > 0) && (q[r2s[r - 1]] <= curVal)) {
            r2s[r] = r2s[r - 1];
            s2r[r2s[r]] = r;
            r--;
        }

        r2s[r] = c;
        s2r[c] = r;
    }

    input._index += count;
    output._index += count;
    return true;
}

bool SBRT::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if (input._array == output._array)
        return false;

    if ((count < 0) || (count+input._index > input._length))
      return false;

    // Aliasing
    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];
    int* p = _prev;
    int* q = _curr;
    int* r2s = _ranks;

    const int mask1 = (_mode == MODE_TIMESTAMP) ? 0 : -1;
    const int mask2 = (_mode == MODE_MTF) ? 0 : -1;
    const int shift = (_mode == MODE_RANK) ? 1 : 0;

    for (int i = 0; i < 256; i++) {
        p[i] = 0;
        q[i] = 0;
        r2s[i] = i;
    }

    for (int i = 0; i < count; i++) {
        int r = src[i] & 0xFF;
        int c = r2s[r];
        dst[i] = (byte)c;
        q[c] = ((i & mask1) + (p[c] & mask2)) >> shift;
        p[c] = i;
        int curVal = q[c];

        // Move up symbol to correct rank
        while ((r > 0) && (q[r2s[r - 1]] <= curVal)) {
            r2s[r] = r2s[r - 1];
            r--;
        }

        r2s[r] = c;
    }

    input._index += count;
    output._index += count;
    return true;
}
