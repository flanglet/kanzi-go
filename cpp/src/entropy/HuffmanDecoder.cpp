
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
#include <sstream>
#include <algorithm>
#include <vector>
#include "HuffmanDecoder.hpp"
#include "HuffmanCommon.hpp"
#include "EntropyUtils.hpp"
#include "ExpGolombDecoder.hpp"
#include "../BitStreamException.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block.
// The default chunk size is 65536 bytes.
HuffmanDecoder::HuffmanDecoder(InputBitStream& bitstream, int chunkSize) THROW : _bitstream(bitstream)
{
    if ((chunkSize != 0) && (chunkSize < 1024))
        throw IllegalArgumentException("The chunk size must be at least 1024");

    if (chunkSize > 1 << 30)
        throw IllegalArgumentException("The chunk size must be at most 2^30");

    _chunkSize = chunkSize;
    _minCodeLen = 8;
    _state = 0;
    _bits = 0;

    // Default lengths & canonical codes
    for (int i = 0; i < 256; i++) {
        _codes[i] = i;
        _sizes[i] = 8;
    }

    memset(_ranks, 0, sizeof(_ranks));
}

int HuffmanDecoder::readLengths() THROW
{
    int count = EntropyUtils::decodeAlphabet(_bitstream, _ranks);
    ExpGolombDecoder egdec(_bitstream, true);
    int currSize;
    _minCodeLen = MAX_SYMBOL_SIZE; // max code length
    int prevSize = 2;

    // Read lengths
    for (int i = 0; i < count; i++) {
        const uint r = _ranks[i];

        if (r >= 256) {
            string msg = "Invalid bitstream: incorrect Huffman symbol ";
            msg += to_string(r);
            throw BitStreamException(msg, BitStreamException::INVALID_STREAM);
        }

        _codes[r] = 0;
        currSize = prevSize + egdec.decodeByte();

        if (currSize <= 0) {
            stringstream ss;
            ss << "Invalid bitstream: incorrect size " << currSize;
            ss << " for Huffman symbol " << r;
            throw BitStreamException(ss.str(), BitStreamException::INVALID_STREAM);
        }

        if (currSize > MAX_SYMBOL_SIZE) {
            stringstream ss;
            ss << "Invalid bitstream: incorrect max size " << currSize;
            ss << " for Huffman symbol " << r;
            throw BitStreamException(ss.str(), BitStreamException::INVALID_STREAM);
        }

        if (_minCodeLen > currSize)
            _minCodeLen = currSize;

        _sizes[r] = short(currSize);
        prevSize = currSize;
    }

    if (count == 0)
        return 0;

    // Create canonical codes
    if (HuffmanCommon::generateCanonicalCodes(_sizes, _codes, _ranks, count) < 0) {
        stringstream ss;
        ss << "Could not generate codes: max code length (" << MAX_SYMBOL_SIZE;
        ss << " bits) exceeded";
        throw BitStreamException(ss.str(), BitStreamException::INVALID_STREAM);
    }

    // Build decoding tables
    buildDecodingTables(count);
    return count;
}

// Build decoding tables
// The slow decoding table contains the codes in natural order.
// The fast decoding table contains all the prefixes with DECODING_BATCH_SIZE bits.
void HuffmanDecoder::buildDecodingTables(int count)
{
    memset(_fdTable, 0, sizeof(_fdTable));
    memset(_sdTable, 0, sizeof(_sdTable));

    for (int i = 0; i <= MAX_SYMBOL_SIZE; i++)
        _sdtIndexes[i] = SYMBOL_ABSENT;

    int len = 0;

    for (int i = 0; i < count; i++) {
        const int r = _ranks[i];
        const int code = _codes[r];

        if (_sizes[r] > len) {
            len = _sizes[r];
            _sdtIndexes[len] = i - code;
        }

        // Fill slow decoding table
        const int val = (_sizes[r] << 8) | r;
        _sdTable[i] = val;
        int idx, end;

        // Fill fast decoding table
        // Find location index in table
        if (len < DECODING_BATCH_SIZE) {
            idx = code << (DECODING_BATCH_SIZE - len);
            end = idx + (1 << (DECODING_BATCH_SIZE - len));
        }
        else {
            idx = code >> (len - DECODING_BATCH_SIZE);
            end = idx + 1;
        }

        // All DECODING_BATCH_SIZE bit values read from the bit stream and
        // starting with the same prefix point to symbol r
        while (idx < end)
            _fdTable[idx++] = val;
    }
}

// Use fastDecodeByte until the near end of chunk or block.
int HuffmanDecoder::decode(byte block[], uint blkptr, uint len)
{
    if (len == 0)
        return 0;

    if (_minCodeLen == 0)
        return -1;

    const int sz = (_chunkSize == 0) ? len : _chunkSize;
    int startChunk = blkptr;
    const int end = blkptr + len;

    while (startChunk < end) {
        // Reinitialize the Huffman tables
        if (readLengths() <= 0)
            return startChunk - blkptr;

        // Compute minimum number of bits required in bitstream for fast decoding
        int endPaddingSize = 64 / _minCodeLen;

        if (_minCodeLen * endPaddingSize != 64)
            endPaddingSize++;

        const int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
        const int endChunk1 = (endChunk - endPaddingSize) & -8;
        int i = startChunk;

        // Fast decoding (read DECODING_BATCH_SIZE bits at a time)
        for (; i < endChunk1; i+=8)
        {
            block[i]   = fastDecodeByte();
            block[i+1] = fastDecodeByte();
            block[i+2] = fastDecodeByte();
            block[i+3] = fastDecodeByte();
            block[i+4] = fastDecodeByte();
            block[i+5] = fastDecodeByte();
            block[i+6] = fastDecodeByte();
            block[i+7] = fastDecodeByte();
        }

        // Fallback to regular decoding (read one bit at a time)
        for (; i < endChunk; i++)
            block[i] = slowDecodeByte(0, 0);

        startChunk = endChunk;
    }

    return len;
}

byte HuffmanDecoder::slowDecodeByte(int code, int codeLen) THROW
{
    while (codeLen < MAX_SYMBOL_SIZE) {
        codeLen++;
        code <<= 1;

        if (_bits == 0)
            code |= _bitstream.readBit();
        else {
            // Consume remaining bits in 'state'
            _bits--;
            code |= ((_state >> _bits) & 1);
        }

        const int idx = _sdtIndexes[codeLen];

        if (idx == SYMBOL_ABSENT) // No code with this length ?
            continue;

        if ((_sdTable[idx + code] >> 8) == uint(codeLen))
            return byte(_sdTable[idx + code]);
    }

    throw BitStreamException("Invalid bitstream: incorrect Huffman code",
        BitStreamException::INVALID_STREAM);
}

// 64 bits must be available in the bitstream
byte HuffmanDecoder::fastDecodeByte()
{
    if (_bits < DECODING_BATCH_SIZE) {
        // Fetch more bits from bitstream
        const uint64 read = _bitstream.readBits(64 - _bits);
        const uint64 mask = (1 << _bits) - 1;
        _state = ((_state & mask) << (64 - _bits)) | read;
        _bits = 64;
    }

    // Retrieve symbol from fast decoding table
    const int idx = int(_state >> (_bits - DECODING_BATCH_SIZE)) & DECODING_MASK;
    const uint val = _fdTable[idx];

    if (val > MAX_DECODING_INDEX) {
        _bits -= DECODING_BATCH_SIZE;
        return slowDecodeByte(idx, DECODING_BATCH_SIZE);
    }

    _bits -= (val >> 8);
    return byte(val);
}
