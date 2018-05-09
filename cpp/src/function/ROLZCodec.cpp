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

#include <sstream>
#include "ROLZCodec.hpp"
#include "../IllegalArgumentException.hpp"
#include "../Memory.hpp"

using namespace kanzi;

ROLZCodec::ROLZCodec(uint logPosChecks)
    : _litPredictor(9)
    , _matchPredictor(logPosChecks)
{
    if ((logPosChecks < 2) || (logPosChecks > 8)) {
        stringstream ss;
        ss << "Invalid logPosChecks parameter: " << logPosChecks << " (must be in [2..8])";
        throw IllegalArgumentException(ss.str());
    }

    _logPosChecks = logPosChecks;
    _posChecks = 1 << logPosChecks;
    _maskChecks = _posChecks - 1;
    _matches = new int32[HASH_SIZE << logPosChecks];
}

// return position index (_logPosChecks bits) + length (8 bits) or -1
inline int ROLZCodec::findMatch(const byte buf[], const int pos, const int end)
{
    const uint32 key = getKey(&buf[pos - 2]);
    int32* matches = &_matches[key << _logPosChecks];
    const int32 hash32 = hash(&buf[pos]);
    const int32 counter = _counters[key];
    int bestLen = MIN_MATCH - 1;
    int bestIdx = -1;
    const byte* curBuf = &buf[pos];
    const int maxMatch = (end - pos >= MAX_MATCH) ? MAX_MATCH : end - pos;

    // Check all recorded positions
    for (int i = 0; i < _posChecks; i++) {
        int32 ref = matches[(counter - i) & _maskChecks];

        if (ref == 0)
            break;

        // Hash check may save a memory access ...
        if ((ref & HASH_MASK) != hash32)
            continue;

        ref &= ~HASH_MASK;

        if (buf[ref] != curBuf[0])
            continue;

        int n = 1;

        while ((n < maxMatch) && (buf[ref + n] == curBuf[n]))
            n++;

        if (n > bestLen) {
            bestIdx = i;
            bestLen = n;

            if (bestLen == maxMatch)
                break;
        }
    }

    // Register current position
    _counters[key]++;
    matches[(counter + 1) & _maskChecks] = hash32 | int32(pos);
    return (bestLen < MIN_MATCH) ? -1 : (bestIdx << 8) | (bestLen - MIN_MATCH);
}

bool ROLZCodec::forward(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
        return false;

    if (input._array == output._array)
        return false;

    if (output._length < getMaxEncodedLength(count))
        return false;

    if (count <= 16) {
        for (int i = 0; i < count; i++)
            output._array[output._index + i] = input._array[input._index + i];

        input._index += count;
        output._index += count;
        return true;
    }

    const int srcEnd = count - 4;
    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];
    int srcIdx = 0;
    int dstIdx = 0;
    BigEndian::writeInt32(&dst[dstIdx], count);
    dstIdx += 4;
    int sizeChunk = (count <= CHUNK_SIZE) ? count : CHUNK_SIZE;
    int startChunk = srcIdx;
    _litPredictor.reset();
    _matchPredictor.reset();
    Predictor* predictors[2] = { &_litPredictor, &_matchPredictor };
    ROLZEncoder re(predictors, &dst[0], dstIdx);
    memset(&_counters[0], 0, sizeof(int32) * 65536);

    while (startChunk < srcEnd) {
        memset(&_matches[0], 0, sizeof(int32) * (HASH_SIZE << _logPosChecks));
        const int endChunk = (startChunk + sizeChunk < srcEnd) ? startChunk + sizeChunk : srcEnd;
        sizeChunk = endChunk - startChunk;
        src = &input._array[startChunk];
        srcIdx = 2;
        _litPredictor.setContext(0);
        re.setContext(LITERAL_FLAG);
        re.encodeBit(LITERAL_FLAG);
        re.encodeByte(src[0]);

        if (startChunk + 1 < srcEnd) {
            re.encodeBit(LITERAL_FLAG);
            re.encodeByte(src[1]);
        }

        while (srcIdx < sizeChunk) {
            _litPredictor.setContext(src[srcIdx - 1]);
            re.setContext(LITERAL_FLAG);
            const int match = findMatch(src, srcIdx, sizeChunk);

            if (match == -1) {
                re.encodeBit(LITERAL_FLAG);
                re.encodeByte(src[srcIdx]);
                srcIdx++;
            }
            else {
                const int matchLen = match & 0xFF;
                re.encodeBit(MATCH_FLAG);
                re.encodeByte(byte(matchLen));
                const int matchIdx = match >> 8;
                _matchPredictor.setContext(src[srcIdx - 1]);
                re.setContext(MATCH_FLAG);

                for (int shift = _logPosChecks - 1; shift >= 0; shift--)
                    re.encodeBit((matchIdx >> shift) & 1);

                srcIdx += (matchLen + MIN_MATCH);
            }
        }

        startChunk = endChunk;
    }

    // Last literals
    re.setContext(LITERAL_FLAG);

    for (int i = 0; i < 4; i++, srcIdx++) {
        _litPredictor.setContext(src[srcIdx - 1]);
        re.encodeBit(LITERAL_FLAG);
        re.encodeByte(src[srcIdx]);
    }

    re.dispose();
    input._index = startChunk - sizeChunk + srcIdx;
    output._index = dstIdx;
    return input._index == count;
}

bool ROLZCodec::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
        return false;

    if (input._array == output._array)
        return false;

    if (count <= 16) {
        for (int i = 0; i < count; i++)
            output._array[output._index + i] = input._array[input._index + i];

        input._index += count;
        output._index += count;
        return true;
    }

    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];
    int srcIdx = 0;
    const int dstEnd = BigEndian::readInt32(&src[srcIdx]);
    srcIdx += 4;
    int sizeChunk = (dstEnd < CHUNK_SIZE) ? dstEnd : CHUNK_SIZE;
    int startChunk = 0;
    _litPredictor.reset();
    _matchPredictor.reset();
    Predictor* predictors[2] = { &_litPredictor, &_matchPredictor };
    ROLZDecoder rd(predictors, &src[0], srcIdx);
    memset(&_counters[0], 0, sizeof(int32) * 65536);

    while (startChunk < dstEnd) {
        memset(&_matches[0], 0, sizeof(int32) * (HASH_SIZE << _logPosChecks));
        const int endChunk = (startChunk + sizeChunk < dstEnd) ? startChunk + sizeChunk : dstEnd;
        sizeChunk = endChunk - startChunk;
        dst = &output._array[output._index];
        int dstIdx = 2;
        _litPredictor.setContext(0);
        rd.setContext(LITERAL_FLAG);
        int bit = rd.decodeBit();

        if (bit == LITERAL_FLAG) {
            dst[0] = rd.decodeByte();

            if (output._index + 1 < dstEnd) {
                bit = rd.decodeBit();

                if (bit == LITERAL_FLAG)
                    dst[1] = rd.decodeByte();
            }
        }

        // Sanity check
        if (bit == MATCH_FLAG) {
            output._index += dstIdx;
            break;
        }

        while (dstIdx < sizeChunk) {
            const int savedIdx = dstIdx;
            const uint32 key = getKey(&dst[dstIdx - 2]);
            int32* matches = &_matches[key << _logPosChecks];
            _litPredictor.setContext(dst[dstIdx - 1]);
            rd.setContext(LITERAL_FLAG);

            if (rd.decodeBit() == MATCH_FLAG) {
                // Match flag
                int matchLen = rd.decodeByte() & 0xFF;

                // Sanity check
                if (dstIdx + matchLen > dstEnd) {
                    output._index += dstIdx;
                    break;
                }

                _matchPredictor.setContext(dst[dstIdx - 1]);
                rd.setContext(MATCH_FLAG);
                int32 matchIdx = 0;

                for (int shift = _logPosChecks - 1; shift >= 0; shift--)
                    matchIdx |= (rd.decodeBit() << shift);

                int32 ref = matches[(_counters[key] - matchIdx) & _maskChecks];

                // Copy
                dst[dstIdx] = dst[ref];
                dst[dstIdx + 1] = dst[ref + 1];
                dst[dstIdx + 2] = dst[ref + 2];
                dstIdx += 3;
                ref += 3;

                //while (matchLen >= 4)
                // dst[dstIdx] = src[ref];

                while (matchLen != 0) {
                    dst[dstIdx++] = dst[ref++];
                    matchLen--;
                }
            }
            else {
                // Literal flag
                dst[dstIdx++] = rd.decodeByte();
            }

            // Update
            _counters[key]++;
            matches[_counters[key] & _maskChecks] = savedIdx;
        }

        startChunk = endChunk;
        output._index += dstIdx;
    }

    rd.dispose();
    input._index = srcIdx;
    return srcIdx == count;
}

inline ROLZPredictor::ROLZPredictor(uint logPosChecks)
{
    _logSize = logPosChecks;
    _size = 1 << logPosChecks;
    _p1 = new uint16[256 * _size];
    _p2 = new uint16[256 * _size];
    reset();
}

inline void ROLZPredictor::reset()
{
    _c1 = 1;
    _ctx = 0;

    for (int i = 0; i < 256 * _size; i++) {
        _p1[i] = 1 << 15;
        _p2[i] = 1 << 15;
    }
}

inline void ROLZPredictor::update(int bit)
{
    const int32 idx = _ctx + _c1;
    _p1[idx] -= (((_p1[idx] - (-bit & 0xFFFF)) >> 3) + bit);
    _p2[idx] -= (((_p2[idx] - (-bit & 0xFFFF)) >> 6) + bit);
    _c1 <<= 1;
    _c1 += bit;

    if (_c1 >= _size)
        _c1 = 1;
}

inline int ROLZPredictor::get()
{
    const int32 idx = _ctx + _c1;
    return (int(_p1[idx]) + int(_p2[idx])) >> 5;
}

ROLZEncoder::ROLZEncoder(Predictor* predictors[2], byte buf[], int& idx)
    : _idx(idx)
    , _low(0)
    , _high(TOP)
{
    _buf = buf;
    _predictors[0] = predictors[0];
    _predictors[1] = predictors[1];
    _predictor = _predictors[0];
}

inline void ROLZEncoder::encodeByte(byte val)
{
    encodeBit((val >> 7) & 1);
    encodeBit((val >> 6) & 1);
    encodeBit((val >> 5) & 1);
    encodeBit((val >> 4) & 1);
    encodeBit((val >> 3) & 1);
    encodeBit((val >> 2) & 1);
    encodeBit((val >> 1) & 1);
    encodeBit(val & 1);
}

inline void ROLZEncoder::encodeBit(int bit)
{
    // Calculate interval split
    const uint64 split = (((_high - _low) >> 4) * uint64(_predictor->get())) >> 8;

    // Update fields with new interval bounds
    _high -= (-bit & (_high - _low - split));
    _low += (~ - bit & (split + 1));

    // Update predictor
    _predictor->update(bit);

    // Emit unchanged first 32 bits
    while (((_low ^ _high) & MASK_24_56) == 0) {
        BigEndian::writeInt32(&_buf[_idx], int32(_high >> 32));
        _idx += 4;
        _low <<= 32;
        _high = (_high << 32) | MASK_0_32;
    }
}

inline void ROLZEncoder::dispose()
{
    for (int i = 0; i < 8; i++) {
        _buf[_idx + i] = byte(_low >> 56);
        _low <<= 8;
    }

    _idx += 8;
}

ROLZDecoder::ROLZDecoder(Predictor* predictors[2], byte buf[], int& idx)
    : _idx(idx)
    , _low(0)
    , _high(TOP)
    , _current(0)
{
    _buf = buf;

    for (int i = 0; i < 8; i++)
        _current = (_current << 8) | uint64(_buf[_idx + i] & 0xFF);

    _idx += 8;
    _predictors[0] = predictors[0];
    _predictors[1] = predictors[1];
    _predictor = _predictors[0];
}

inline byte ROLZDecoder::decodeByte()
{
    return byte((decodeBit() << 7) | (decodeBit() << 6) | (decodeBit() << 5) | (decodeBit() << 4) | (decodeBit() << 3) | (decodeBit() << 2) | (decodeBit() << 1) | decodeBit());
}

inline int ROLZDecoder::decodeBit()
{
    // Calculate interval split
    const uint64 mid = _low + ((((_high - _low) >> 4) * uint64(_predictor->get())) >> 8);
    int bit;

    if (mid >= _current) {
        bit = 1;
        _high = mid;
    }
    else {
        bit = 0;
        _low = mid + 1;
    }

    // Update predictor
    _predictor->update(bit);

    // Read 32 bits
    while (((_low ^ _high) & MASK_24_56) == 0) {
        _low = (_low << 32) & MASK_0_56;
        _high = ((_high << 32) | MASK_0_32) & MASK_0_56;
        const uint64 val = uint64(BigEndian::readInt32(&_buf[_idx])) & MASK_0_32;
        _current = ((_current << 32) | val) & MASK_0_56;
        _idx += 4;
    }

    return bit;
}
