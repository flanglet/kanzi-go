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

    int _counters[256];
    byte _flags[32];

    for (int i = 0; i < 32; i++)
        _flags[i] = 0;

    for (int i = 0; i < 256; i++)
        _counters[i] = 0;

    byte* src = input._array;
    byte* dst = output._array;
    int srcIdx = input._index;
    int dstIdx = output._index;
    const int srcEnd = srcIdx + length;
    const int dstEnd = output._length;
    const int dstEnd4 = dstEnd - 4;
    bool res = true;
    int run = 0;
    const int threshold = _runThreshold;
    const int maxRun = MAX_RUN + _runThreshold;

    // Initialize with a value different from the first data
    byte prev = byte(~src[srcIdx]);

    // Step 1: create counters and set compression flags
    while (srcIdx < srcEnd) {
        const byte val = src[srcIdx++];

        if ((prev == val) && (run < MAX_RUN)) {
            run++;
            continue;
        }

        if (run >= threshold)
            _counters[prev & 0xFF] += (run - threshold - 1);

        prev = val;
        run = 1;
    }

    if (run >= threshold)
        _counters[prev & 0xFF] += (run - threshold - 1);

    for (int i = 0; i < 256; i++) {
        if (_counters[i] > 0)
            _flags[i >> 3] |= (1 << (7 - (i & 7)));
    }

    // Write flags to output
    for (int i = 0; i < 32; i++)
        dst[dstIdx++] = _flags[i];

    srcIdx = input._index;
    prev = byte(~src[srcIdx]);
    run = 0;

    // Step 2: output run lengths and literals
    // Note that it is possible to output runs over the threshold (for symbols
    // with an unset compression flag)
    while ((srcIdx < srcEnd) && (dstIdx < dstEnd)) {
        const byte val = src[srcIdx++];

        // Encode repetitions in the 'length' if the flag of the symbol is set.
        if ((prev == val) && (run < maxRun) && (_counters[prev & 0xFF] > 0)) {
            if (++run < threshold)
                dst[dstIdx++] = prev;

            continue;
        }

        if (run >= threshold) {
            run -= threshold;

            if (dstIdx >= dstEnd4) {
                if (run >= RUN_LEN_ENCODE2) {
                    break;
                }

                if ((run >= RUN_LEN_ENCODE1) && (dstIdx > dstEnd4)) {
                    break;
                }
            }

            dst[dstIdx++] = prev;

            // Encode run length
            if (run >= RUN_LEN_ENCODE1) {
                if (run < RUN_LEN_ENCODE2) {
                    run -= RUN_LEN_ENCODE1;
                    dst[dstIdx++] = byte(RUN_LEN_ENCODE1 + (run >> 8));
                }
                else {
                    run -= RUN_LEN_ENCODE2;
                    dst[dstIdx] = byte(0xFF);
                    dst[dstIdx + 1] = byte(run >> 8);
                    dstIdx += 2;
                }
            }

            dst[dstIdx++] = byte(run);
        }

        dst[dstIdx++] = val;
        prev = val;
        run = 1;
    }

    // Fill up the output array
    if (run >= threshold) {
        run -= threshold;

        if (dstIdx >= dstEnd4) {
            if (run >= RUN_LEN_ENCODE2)
                res = false;
            else if ((run >= RUN_LEN_ENCODE1) && (dstIdx > dstEnd4))
                res = false;
        }
        else {
            dst[dstIdx++] = prev;

            // Encode run length
            if (run >= RUN_LEN_ENCODE1) {
                if (run < RUN_LEN_ENCODE2) {
                    run -= RUN_LEN_ENCODE1;
                    dst[dstIdx++] = byte(RUN_LEN_ENCODE1 + (run >> 8));
                }
                else {
                    run -= RUN_LEN_ENCODE2;
                    dst[dstIdx] = byte(0xFF);
                    dst[dstIdx + 1] = byte(run >> 8);
                    dstIdx += 2;
                }
            }

            dst[dstIdx++] = byte(run);
        }
    }

    input._index = srcIdx;
    output._index = dstIdx;
    return res && (srcIdx == srcEnd);
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
    const int maxRun = MAX_RUN + _runThreshold;
    bool res = true;

    int _counters[256];

    // Read compression flags from input
    for (int i = 0, j = 0; i < 32; i++, j += 8) {
        const byte flag = src[srcIdx++];
        _counters[j] = (flag >> 7) & 1;
        _counters[j + 1] = (flag >> 6) & 1;
        _counters[j + 2] = (flag >> 5) & 1;
        _counters[j + 3] = (flag >> 4) & 1;
        _counters[j + 4] = (flag >> 3) & 1;
        _counters[j + 5] = (flag >> 2) & 1;
        _counters[j + 6] = (flag >> 1) & 1;
        _counters[j + 7] = flag & 1;
    }

    // Initialize with a value different from the first symbol
    byte prev = (byte)~src[srcIdx];

    while (srcIdx < srcEnd) {
        const byte val = src[srcIdx++];

        if ((prev == val) && (_counters[prev & 0xFF] > 0)) {
            run++;

            if (run >= threshold) {
                // Decode run length
                run = src[srcIdx++] & 0xFF;

                if (run == 0xFF) {
                    if (srcIdx + 1 >= srcEnd)
                        break;

                    run = ((src[srcIdx] & 0xFF) << 8) | (src[srcIdx + 1] & 0xFF);
                    srcIdx += 2;
                    run += RUN_LEN_ENCODE2;
                }
                else if (run >= RUN_LEN_ENCODE1) {
                    if (srcIdx >= srcEnd)
                        break;

                    run = ((run - RUN_LEN_ENCODE1) << 8) | (src[srcIdx++] & 0xFF);
                    run += RUN_LEN_ENCODE1;
                }

                if ((dstIdx >= dstEnd + run) || (run > maxRun)) {
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

        if (dstIdx >= dstEnd)
            break;

        dst[dstIdx++] = val;
    }

    input._index = srcIdx;
    output._index = dstIdx;
    return res & (srcIdx == srcEnd);
}
