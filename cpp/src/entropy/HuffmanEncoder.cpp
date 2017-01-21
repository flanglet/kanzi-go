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
#include "HuffmanEncoder.hpp"
#include "HuffmanCommon.hpp"
#include "EntropyUtils.hpp"
#include "ExpGolombEncoder.hpp"
#include "../BitStreamException.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

class FrequencyArrayComparator {
private:
    uint* _freqs;

public:
    FrequencyArrayComparator(uint freqs[]) { _freqs = freqs; }

    FrequencyArrayComparator() {}

    bool operator()(int i, int j);
};

bool FrequencyArrayComparator::operator()(int lidx, int ridx)
{
    // Check size (natural order) as first key
    const int res = _freqs[lidx] - _freqs[ridx];

    // Check index (natural order) as second key
    return (res != 0) ? res < 0 : lidx < ridx;
}


// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block.
// The default chunk size is 65536 bytes.
HuffmanEncoder::HuffmanEncoder(OutputBitStream& bitstream, int chunkSize) THROW : _bitstream(bitstream)
{
    if ((chunkSize != 0) && (chunkSize < 1024))
        throw IllegalArgumentException("The chunk size must be at least 1024");

    if (chunkSize > 1 << 30)
        throw IllegalArgumentException("The chunk size must be at most 2^30");

    _chunkSize = chunkSize;

    // Default frequencies, sizes and codes
    for (int i = 0; i < 256; i++) {
        _freqs[i] = 1;
        _sizes[i] = 8;
        _codes[i] = i;
    }

    memset(_ranks, 0, sizeof(_ranks));
    memset(_sranks, 0, sizeof(_sranks));
    memset(_buffer, 0, sizeof(_buffer));
}

// Rebuild Huffman codes
bool HuffmanEncoder::updateFrequencies(uint frequencies[]) THROW
{
    int count = 0;

    for (int i = 0; i < 256; i++) {
        _sizes[i] = 0;
        _codes[i] = 0;

        if (frequencies[i] > 0)
            _ranks[count++] = i;
    }

    try {
        if (count == 1) {
            _sranks[0] = _ranks[0];
            _sizes[_ranks[0]] = 1;
        }
        else {
            computeCodeLengths(frequencies, count);
        }
    }
    catch (IllegalArgumentException& e) {
        // Happens when a very rare symbol cannot be coded to due code length limit
        throw BitStreamException(e.what(), BitStreamException::INVALID_STREAM);
    }

    EntropyUtils::encodeAlphabet(_bitstream, _ranks, 256, count);

    // Transmit code lengths only, frequencies and codes do not matter
    // Unary encode the length difference
    ExpGolombEncoder egenc(_bitstream, true);
    short prevSize = 2;

    for (int i = 0; i < count; i++) {
        const short currSize = _sizes[_ranks[i]];
        egenc.encodeByte((byte)(currSize - prevSize));
        prevSize = currSize;
    }

    // Create canonical codes
    if (HuffmanCommon::generateCanonicalCodes(_sizes, _codes, _sranks, count) < 0) {
        throw BitStreamException("Could not generate codes: max code length (24 bits) exceeded",
            BitStreamException::INVALID_STREAM);
    }

    // Pack size and code (size <= MAX_SYMBOL_SIZE bits)
    for (int i = 0; i < count; i++) {
        const int r = _ranks[i];
        _codes[r] |= (_sizes[r] << 24);
    }

    return true;
}

// See [In-Place Calculation of Minimum-Redundancy Codes]
// by Alistair Moffat & Jyrki Katajainen
// count > 1 by design
void HuffmanEncoder::computeCodeLengths(uint frequencies[], int count) THROW
{
    // Sort by increasing frequencies (first key) and increasing value (second key)
    vector<uint> v(_ranks, _ranks + count);
    FrequencyArrayComparator comparator(frequencies);
    sort(v.begin(), v.end(), comparator);
    memcpy(_sranks, &v[0], count*sizeof(uint));

    for (int i = 0; i < count; i++)
        _buffer[i] = frequencies[_sranks[i]];

    computeInPlaceSizesPhase1(_buffer, count);
    computeInPlaceSizesPhase2(_buffer, count);

    for (int i = 0; i < count; i++) {
        short codeLen = (short)_buffer[i];

        if ((codeLen <= 0) || (codeLen > MAX_SYMBOL_SIZE)) {
            stringstream ss;
            ss << "Could not generate codes: max code length (" << MAX_SYMBOL_SIZE;
            ss << " bits) exceeded";
            throw IllegalArgumentException(ss.str());
        }

        _sizes[_sranks[i]] = codeLen;
    }
}

void HuffmanEncoder::computeInPlaceSizesPhase1(uint data[], int n)
{
    for (int s = 0, r = 0, t = 0; t < n - 1; t++) {
        int sum = 0;

        for (int i = 0; i < 2; i++) {
            if ((s >= n) || ((r < t) && (data[r] < data[s]))) {
                sum += data[r];
                data[r] = t;
                r++;
            }
            else {
                sum += data[s];

                if (s > t)
                    data[s] = 0;

                s++;
            }
        }

        data[t] = sum;
    }
}

void HuffmanEncoder::computeInPlaceSizesPhase2(uint data[], int n)
{
    uint level_top = n - 2; //root
    int depth = 1;
    int i = n;
    int total_nodes_at_level = 2;

    while (i > 0) {
        int k = level_top;

        while ((k > 0) && (data[k - 1] >= level_top))
            k--;

        const int internal_nodes_at_level = level_top - k;
        const int leaves_at_level = total_nodes_at_level - internal_nodes_at_level;

        for (int j = 0; j < leaves_at_level; j++)
            data[--i] = depth;

        total_nodes_at_level = internal_nodes_at_level << 1;
        level_top = k;
        depth++;
    }
}

// Dynamically compute the frequencies for every chunk of data in the block
int HuffmanEncoder::encode(byte block[], uint blkptr, uint len)
{
    if (len == 0)
        return 0;

    uint* frequencies = _freqs;
    const int end = blkptr + len;
    const int sz = (_chunkSize == 0) ? len : _chunkSize;
    int startChunk = blkptr;

    while (startChunk < end) {
        const int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
        const int endChunk8 = ((endChunk - startChunk) & -8) + startChunk;
        memset(frequencies, 0, sizeof(_freqs));

        for (int i = startChunk; i < endChunk8; i += 8) {
            frequencies[block[i] & 0xFF]++;
            frequencies[block[i + 1] & 0xFF]++;
            frequencies[block[i + 2] & 0xFF]++;
            frequencies[block[i + 3] & 0xFF]++;
            frequencies[block[i + 4] & 0xFF]++;
            frequencies[block[i + 5] & 0xFF]++;
            frequencies[block[i + 6] & 0xFF]++;
            frequencies[block[i + 7] & 0xFF]++;
        }

        for (int i = endChunk8; i < endChunk; i++)
            frequencies[block[i] & 0xFF]++;

        // Rebuild Huffman codes
        updateFrequencies(frequencies);

        for (int i = startChunk; i < endChunk8; i += 8) {
            uint val;
            val = _codes[block[i] & 0xFF];
            _bitstream.writeBits(val, val >> 24);
            val = _codes[block[i + 1] & 0xFF];
            _bitstream.writeBits(val, val >> 24);
            val = _codes[block[i + 2] & 0xFF];
            _bitstream.writeBits(val, val >> 24);
            val = _codes[block[i + 3] & 0xFF];
            _bitstream.writeBits(val, val >> 24);
            val = _codes[block[i + 4] & 0xFF];
            _bitstream.writeBits(val, val >> 24);
            val = _codes[block[i + 5] & 0xFF];
            _bitstream.writeBits(val, val >> 24);
            val = _codes[block[i + 6] & 0xFF];
            _bitstream.writeBits(val, val >> 24);
            val = _codes[block[i + 7] & 0xFF];
            _bitstream.writeBits(val, val >> 24);
        }

        for (int i = endChunk8; i < endChunk; i++) {
            uint val = _codes[block[i] & 0xFF];
            _bitstream.writeBits(val, val >> 24);
        }

        startChunk = endChunk;
    }

    return len;
}
