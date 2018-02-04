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
#include "SnappyCodec.hpp"
#include "../IllegalArgumentException.hpp"
#include "../Memory.hpp"

using namespace kanzi;

// emitLiteral writes a literal chunk and returns the number of bytes written.
int SnappyCodec::emitLiteral(SliceArray<byte>& input, SliceArray<byte>& output, int len)
{
    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];
    const int n = len - 1;
    int dstIdx = 0;

    if (n < 60) {
        dst[0] = (byte)((n << 2) | TAG_LITERAL);
        dstIdx = 1;

        if (len <= 16) {
            int i0 = 0;

            if (len >= 8) {
                memcpy(&dst[1], &src[0], 8);
                i0 = 8;
            }

            for (int i = i0; i < len; i++)
                dst[i + 1] = src[i];

            return len + 1;
        }
    }
    else if (n < 0x0100) {
        dst[0] = TAG_ENC_LEN1;
        dst[1] = byte(n);
        dstIdx = 2;
    }
    else if (n < 0x010000) {
        dst[0] = TAG_ENC_LEN2;
        dst[1] = byte(n);
        dst[2] = byte(n >> 8);
        dstIdx = 3;
    }
    else if (n < 0x01000000) {
        dst[0] = TAG_ENC_LEN3;
        dst[1] = byte(n);
        dst[2] = byte(n >> 8);
        dst[3] = byte(n >> 16);
        dstIdx = 4;
    }
    else {
        dst[0] = TAG_ENG_LEN4;
        dst[1] = byte(n);
        dst[2] = byte(n >> 8);
        dst[3] = byte(n >> 16);
        dst[4] = byte(n >> 24);
        dstIdx = 5;
    }

    memcpy(&dst[dstIdx], &src[0], len);
    return len + dstIdx;
}

// emitCopy writes a copy chunk and returns the number of bytes written.
int SnappyCodec::emitCopy(SliceArray<byte>& output, int offset, int len)
{
    byte* dst = &output._array[output._index];
    int idx = 0;
    const byte b1 = byte(offset);
    const byte b2 = byte(offset >> 8);

    while (len >= 64) {
        dst[idx] = B0;
        dst[idx + 1] = b1;
        dst[idx + 2] = b2;
        idx += 3;
        len -= 64;
    }

    if (len > 0) {
        if ((offset < 2048) && (len < 12) && (len >= 4)) {
            dst[idx] = byte(((b2 & 0x07) << 5) | ((len - 4) << 2) | TAG_COPY1);
            dst[idx + 1] = b1;
            idx += 2;
        }
        else {
            dst[idx] = byte(((len - 1) << 2) | TAG_COPY2);
            dst[idx + 1] = b1;
            dst[idx + 2] = b2;
            idx += 3;
        }
    }

    return idx;
}


bool SnappyCodec::forward(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if (input._array == output._array)
        return false;

    if (output._length - output._index < getMaxEncodedLength(count))
        return false;

    // The block starts with the varint-encoded length of the decompressed bytes.
    int dstIdx = output._index + putUvarint(&output._array[output._index], (uint64)count);

    // Return early if input is short
    if (count <= 4) {
        if (count > 0) {
            output._index = dstIdx;
            dstIdx += emitLiteral(input, output, count);
        }

        input._index += count;
        output._index = dstIdx;
        return true;
    }

    // The table size ranges from 1<<8 to 1<<14 inclusive.
    int shift = 24;
    int tableSize = 256;
    const int max = (count < MAX_TABLE_SIZE) ? count : MAX_TABLE_SIZE;

    while (tableSize < max) {
        shift--;
        tableSize <<= 1;
    }

    memset(_buffer, 0, sizeof(int) * tableSize);
    int* table = _buffer; // aliasing
    byte* src = &input._array[input._index];
    
    // The encoded block must start with a literal, as there are no previous
    // bytes to copy, so we start looking for hash matches at index 1
    int srcIdx = 1; 
    int lit = 0; // The start position of any pending literal bytes
    const int ends = count - 3;

    while (srcIdx < ends) {
        // Update the hash table
        const int h = (LittleEndian::readInt32(&src[srcIdx]) * HASH_SEED) >> shift;
        int t = table[h]; // The last position with the same hash as srcIdx
        table[h] = srcIdx;

        // If t is invalid or src[srcIdx:srcIdx+4] differs from src[t:t+4], accumulate a literal byte
        if ((t == 0) || (srcIdx - t >= MAX_OFFSET) || (differentInts(src, srcIdx, t))) {
            srcIdx++;
            continue;
        }

        // We have a match. First, emit any pending literal bytes
        if (lit != srcIdx) {
            input._index = lit;
            output._index = dstIdx;
            dstIdx += emitLiteral(input, output, srcIdx - lit);
        }

        // Extend the match to be as long as possible
        const int s0 = srcIdx;
        srcIdx += 4;
        t += 4;

        while ((srcIdx < count) && (src[srcIdx] == src[t])) {
            srcIdx++;
            t++;
        }

        // Emit the copied bytes
        output._index = dstIdx;
        dstIdx += emitCopy(output, srcIdx - t, srcIdx - s0);
        lit = srcIdx;
    }

    // Emit any const pending literal bytes and return
    if (lit != count) {
        input._index = lit;
        output._index = dstIdx;
        dstIdx += emitLiteral(input, output, count - lit);
    }

    input._index = count;
    output._index = dstIdx;
    return true;
}

inline int SnappyCodec::putUvarint(byte buf[], uint64 x)
{
    int idx = 0;

    for (; x >= 0x80; x >>= 7)
        buf[idx++] = byte(x | 0x80);

    buf[idx++] = byte(x);
    return idx;
}

// Uvarint decodes a long from the input array and returns that value.
// If an error occurred, an exception is raised.
// The index of the indexed byte array is incremented by the number of bytes read
uint64 SnappyCodec::getUvarint(SliceArray<byte>& iba) THROW
{
    byte* buf = &iba._array[iba._index];
    const int len = iba._length;
    uint64 res = 0;
    int s = 0;

    for (int i = 0; i < len; i++) {
        const uint64 b = buf[i] & 0xFF;

        if (s >= 63) {
           if (((s == 63) && (b > 1)) || (s > 63))
               throw IllegalArgumentException("Overflow: value is larger than 64 bits");
        }

        if ((b & 0x80) == 0) {
            iba._index += (i + 1);
            return res | (b << s);
        }

        res |= ((b & 0x7F) << s);
        s += 7;
    }

    throw IllegalArgumentException("Input buffer too small");
}


 // getDecodedLength returns the length of the decoded block or -1 if error
// The index of the indexed byte array is incremented by the number
// of bytes read
inline int SnappyCodec::getDecodedLength(SliceArray<byte>& input)
{
    try {
        uint64 v = getUvarint(input);
        return (v > 0x7FFFFFFF) ? -1 : (int)v;
    }
    catch (IllegalArgumentException&) {
        return -1;
    }
}

bool SnappyCodec::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if (input._array == output._array)
        return false;

    const int srcIdx = input._index;
    const int dstIdx = output._index;
    byte* src = input._array;
    byte* dst = output._array;

    // Get decoded length (modifies input index)
    const int dLen = getDecodedLength(input);

    if ((dLen < 0) || (output._length - dstIdx < dLen))
        return false;

    const int ends = srcIdx + count;
    int s = input._index;
    int d = dstIdx;
    int offset;
    int length;

    while (s < ends) {
        switch (src[s] & 0x03) {
        case TAG_LITERAL: {
            int x = src[s] & 0xFC;

            if (x < TAG_DEC_LEN1) {
                s++;
                x >>= 2;
            }
            else if (x == TAG_DEC_LEN1) {
                s += 2;
                x = src[s - 1] & 0xFF;
            }
            else if (x == TAG_DEC_LEN2) {
                s += 3;
                x = (src[s - 2] & 0xFF) | ((src[s - 1] & 0xFF) << 8);
            }
            else if (x == TAG_DEC_LEN3) {
                s += 4;
                x = (src[s - 3] & 0xFF) | ((src[s - 2] & 0xFF) << 8) | ((src[s - 1] & 0xFF) << 16);
            }
            else if (x == TAG_DEC_LEN4) {
                s += 5;
                x = (src[s - 4] & 0xFF) | ((src[s - 3] & 0xFF) << 8) | ((src[s - 2] & 0xFF) << 16) | ((src[s - 1] & 0xFF) << 24);
            }

            length = x + 1;

            if ((length <= 0) || (length > output._length - d) || (length > ends - s))
                return false;

            memcpy(&dst[d], &src[s], length);
            d += length;
            s += length;
            continue;
        }

        case TAG_COPY1: {
            s += 2;
            length = 4 + (((src[s - 2] & 0xFF) >> 2) & 0x07);
            offset = ((src[s - 2] & 0xE0) << 3) | (src[s - 1] & 0xFF);
            break;
        }

        case TAG_COPY2: {
            s += 3;
            length = 1 + ((src[s - 3] & 0xFF) >> 2);
            offset = (src[s - 2] & 0xFF) | ((src[s - 1] & 0xFF) << 8);
            break;
        }

        default:
            return false;
        }

        const int end = d + length;

        if ((offset > d) || (end > output._length))
            return false;

        for (; d < end; d++)
            dst[d] = dst[d - offset];
    }

    input._index = ends;
    output._index = d;
    return (d - dstIdx == dLen);
}

inline bool SnappyCodec::differentInts(byte block[], int srcIdx, int dstIdx)
{
    return *((int32*)&block[srcIdx]) != *((int32*)&block[dstIdx]);
}
