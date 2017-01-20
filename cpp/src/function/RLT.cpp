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

#include "RLT.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

RLT::RLT(int runThreshold)
{
    if (runThreshold < 2)
        throw IllegalArgumentException("Invalid run threshold parameter (must be at least 2)");

    _runThreshold = runThreshold;
}

bool RLT::forward(SliceArray<byte>& input, SliceArray<byte>& output, int length)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if (output._length - output._index < getMaxEncodedLength(length))
         return false;

    byte* src = input._array;
    byte* dst = output._array;
    int srcIdx = input._index;
    int dstIdx = output._index;
    const int srcEnd = srcIdx + length;
    const int dstEnd = output._length;
    const int dstEnd3 = length - 3;
    bool res = true;
    int run = 0;
    const int threshold = _runThreshold;
    const int maxThreshold = MAX_RUN_VALUE + _runThreshold;

    // Initialize with a value different from the first data
    byte prev = (byte)~src[srcIdx];

    while ((srcIdx < srcEnd) && (dstIdx < dstEnd)) {
        const byte val = src[srcIdx++];

        // Encode up to 0x7FFF repetitions in the 'length' information
        if ((prev == val) && (run < maxThreshold)) {
            if (++run < threshold)
                dst[dstIdx++] = prev;

            continue;
        }

        if (run >= threshold) {
            if (dstIdx >= dstEnd3) {
                res = false;
                break;
            }

            dst[dstIdx++] = prev;
            run -= threshold;

            // Force MSB to indicate a 2 byte encoding of the length
            if (run >= TWO_BYTE_RLE_MASK1)
                dst[dstIdx++] = (byte)((run >> 8) | TWO_BYTE_RLE_MASK1);

            dst[dstIdx++] = (byte)run;
            run = 1;
        }

        dst[dstIdx++] = val;

        if (prev != val) {
            prev = val;
            run = 1;
        }
    }

    // Fill up the output array
    if (run >= threshold) {
        if (dstIdx >= dstEnd3) {
            res = false;
        }
        else {
            dst[dstIdx++] = prev;
            run -= threshold;

            // Force MSB to indicate a 2 byte encoding of the length
            if (run >= TWO_BYTE_RLE_MASK1)
                dst[dstIdx++] = (byte)((run >> 8) | TWO_BYTE_RLE_MASK1);

            dst[dstIdx++] = (byte)run;
        }
    }

    res &= (srcIdx == length);
    input._index = srcIdx;
    output._index = dstIdx;
    return res;
}

bool RLT::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int length)
{
   if ((input._array == nullptr) || (output._array == nullptr) || (input._array == output._array))
        return false;

    int srcIdx = input._index;
    int dstIdx = output._index;
    byte* src = input._array;
    byte* dst = output._array;
    const int srcEnd = srcIdx + length;
    const int dstEnd = output._length;
    int run = 0;
    const int threshold = _runThreshold;
    bool res = true;

    // Initialize with a value different from the first data
    byte prev = (byte)~src[srcIdx];

    while ((srcIdx < srcEnd) && (dstIdx < dstEnd)) {
        const byte val = src[srcIdx++];

        if (prev == val) {
            if (++run >= threshold) {
                // Read the length
                run = src[srcIdx++] & 0xFF;

                // If the length is encoded in 2 bytes, process next byte
                if ((run & TWO_BYTE_RLE_MASK1) != 0) {
                    run = ((run & TWO_BYTE_RLE_MASK2) << 8) | (src[srcIdx++] & 0xFF);
                }

                if (dstIdx >= dstEnd + run) {
                    res = false;
                    break;
                }

                // Emit length times the previous byte
                while (--run >= 0)
                    dst[dstIdx++] = prev;

                run = 0;
            }
        }
        else {
            prev = val;
            run = 1;
        }

        dst[dstIdx++] = val;
    }

    res &= (srcIdx == srcEnd);
    input._index = srcIdx;
    output._index = dstIdx;
    return res;
}
