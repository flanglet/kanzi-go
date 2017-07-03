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
#include "RangeEncoder.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block.
// The default chunk size is 65536 bytes.
RangeEncoder::RangeEncoder(OutputBitStream& bitstream, int chunkSize, int logRange) THROW : _bitstream(bitstream)
{
    if ((chunkSize != 0) && (chunkSize < 1024))
        throw IllegalArgumentException("The chunk size must be at least 1024");

    if (chunkSize > 1 << 30)
        throw IllegalArgumentException("The chunk size must be at most 2^30");

    if ((logRange < 8) || (logRange > 16)) {
        stringstream ss;
        ss << "Invalid range parameter: " << logRange << " (must be in [8..16])";
        throw IllegalArgumentException(ss.str());
    }

    _logRange = logRange;
    _chunkSize = chunkSize;
    _shift = 0;
}

int RangeEncoder::updateFrequencies(uint frequencies[], int size, int lr)
{
    int alphabetSize = _eu.normalizeFrequencies(frequencies, _alphabet, 256, size, 1 << lr);

    if (alphabetSize > 0) {
        _cumFreqs[0] = 0;

        // Create histogram of frequencies scaled to 'range'
        for (int i = 0; i < 256; i++)
            _cumFreqs[i + 1] = _cumFreqs[i] + frequencies[i];
    }

    encodeHeader(alphabetSize, _alphabet, frequencies, lr);
    return alphabetSize;
}

bool RangeEncoder::encodeHeader(int alphabetSize, uint alphabet[], uint frequencies[], int lr)
{
    EntropyUtils::encodeAlphabet(_bitstream, alphabet, 256, alphabetSize);

    if (alphabetSize == 0)
        return true;

    _bitstream.writeBits(lr - 8, 3); // logRange
    int inc = (alphabetSize > 64) ? 16 : 8;
    int llr = 3;

    while (1 << llr <= lr)
        llr++;

    // Encode all frequencies (but the first one) by chunks of size 'inc'
    for (int i = 1; i < alphabetSize; i += inc) {
        uint max = 0;
        uint logMax = 1;
        int endj = (i + inc < alphabetSize) ? i + inc : alphabetSize;

        // Search for max frequency log size in next chunk
        for (int j = i; j < endj; j++) {
            if (frequencies[alphabet[j]] > max)
                max = frequencies[alphabet[j]];
        }

        while (uint(1 << logMax) <= max)
            logMax++;

        _bitstream.writeBits(logMax - 1, llr);

        // Write frequencies
        for (int j = i; j < endj; j++)
            _bitstream.writeBits(frequencies[alphabet[j]], logMax);
    }

    return true;
}

// Reset frequency stats for each chunk of data in the block
int RangeEncoder::encode(byte block[], uint blkptr, uint len)
{
    if (len == 0)
        return 0;

    const int end = blkptr + len;
    const int sz = (_chunkSize == 0) ? len : _chunkSize;
    int startChunk = blkptr;

    while (startChunk < end) {
        const int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
        _range = TOP_RANGE;
        _low = 0;
        int lr = _logRange;

        // Lower log range if the size of the data chunk is small
        while ((lr > 8) && (1 << lr > endChunk - startChunk))
            lr--;

        if (rebuildStatistics(block, startChunk, endChunk, lr) < 0)
            return startChunk;

        _shift = lr;

        for (int i = startChunk; i < endChunk; i++)
            encodeByte(block[i]);

        // Flush 'low'
        _bitstream.writeBits(_low, 60);
        startChunk = endChunk;
    }

    return len;
}

inline void RangeEncoder::encodeByte(byte b)
{
    // Compute next low and range
    const int symbol = b & 0xFF;
    const uint64 cumFreq = _cumFreqs[symbol];
    const uint64 freq = _cumFreqs[symbol + 1] - cumFreq;
    _range >>= _shift;
    _low += (cumFreq * _range);
    _range *= freq;

    // If the left-most digits are the same throughout the range, write bits to bitstream
    while (true) {
        if (((_low ^ (_low + _range)) & RANGE_MASK) != 0) {
            if (_range > BOTTOM_RANGE)
                  break;

            // Normalize
            _range = ~(_low-1) & BOTTOM_RANGE;
        }

        _bitstream.writeBits(_low >> 32, 28);
        _range <<= 28;
        _low <<= 28;
    }
}

// Compute chunk frequencies, cumulated frequencies and encode chunk header
int RangeEncoder::rebuildStatistics(byte block[], int start, int end, int lr)
{
    memset(_freqs, 0, sizeof(_freqs));

    for (int i = start; i < end; i++)
        _freqs[block[i] & 0xFF]++;

    // Rebuild statistics
    return updateFrequencies(_freqs, end - start, lr);
}