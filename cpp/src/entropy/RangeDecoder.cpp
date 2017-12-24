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
#include "RangeDecoder.hpp"
#include "../IllegalArgumentException.hpp"
#include "EntropyUtils.hpp"

using namespace kanzi;

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block.
// The default chunk size is 65536 bytes.
RangeDecoder::RangeDecoder(InputBitStream& bitstream, int chunkSize) THROW : _bitstream(bitstream)
{
    if ((chunkSize != 0) && (chunkSize < 1024))
        throw IllegalArgumentException("The chunk size must be at least 1024");

    if (chunkSize > 1 << 30)
        throw IllegalArgumentException("The chunk size must be at most 2^30");

    _range = TOP_RANGE;
    _low = 0;
    _code = 0;
    _f2s_length = 0;
    _f2s = new short[_f2s_length];
    _chunkSize = chunkSize;
    _shift = 0;
}

int RangeDecoder::decodeHeader(uint frequencies[])
{
    int alphabetSize = EntropyUtils::decodeAlphabet(_bitstream, _alphabet);

    if (alphabetSize == 0)
        return 0;

    if (alphabetSize != 256) {
        memset(frequencies, 0, sizeof(uint) * 256);
    }

    const uint logRange = uint(8 + _bitstream.readBits(3));
    const int scale = 1 << logRange;
    _shift = logRange;
    int sum = 0;
    int inc = (alphabetSize > 64) ? 16 : 8;
    int llr = 3;

    while (uint(1 << llr) <= logRange)
        llr++;

    // Decode all frequencies (but the first one) by chunks of size 'inc'
    for (int i = 1; i < alphabetSize; i += inc) {
        const int logMax = int(1 + _bitstream.readBits(llr));
        const int endj = (i + inc < alphabetSize) ? i + inc : alphabetSize;

        // Read frequencies
        for (int j = i; j < endj; j++) {
            int val = int(_bitstream.readBits(logMax));

            if ((val <= 0) || (val >= scale)) {
                stringstream ss;
                ss << "Invalid bitstream: incorrect frequency " << val;
                ss << " for symbol '" << _alphabet[j] << "' in range decoder";
                throw BitStreamException(ss.str(),
                    BitStreamException::INVALID_STREAM);
            }

            frequencies[_alphabet[j]] = (uint)val;
            sum += val;
        }
    }

    // Infer first frequency
    if (scale <= sum) {
        stringstream ss;
        ss << "Invalid bitstream: incorrect frequency " << frequencies[_alphabet[0]];
        ss << " for symbol '" << _alphabet[0] << "' in range decoder";
        throw BitStreamException(ss.str(),
            BitStreamException::INVALID_STREAM);
    }

    frequencies[_alphabet[0]] = uint(scale - sum);
    _cumFreqs[0] = 0;

    if (_f2s_length < scale) {
        delete[] _f2s;
        _f2s_length = scale;
        _f2s = new short[_f2s_length];
    }

    // Create histogram of frequencies scaled to 'range' and reverse mapping
    for (int i = 0; i < 256; i++) {
        _cumFreqs[i + 1] = _cumFreqs[i] + frequencies[i];
        const int base = int(_cumFreqs[i]);

        for (int j = frequencies[i] - 1; j >= 0; j--)
            _f2s[base + j] = short(i);
    }

    return alphabetSize;
}

// Initialize once (if necessary) at the beginning, the use the faster decodeByte_()
// Reset frequency stats for each chunk of data in the block
int RangeDecoder::decode(byte block[], uint blkptr, uint len)
{
    if (len == 0)
        return 0;

    const int end = blkptr + len;
    const int sz = (_chunkSize == 0) ? len : _chunkSize;
    int startChunk = blkptr;

    while (startChunk < end) {
        if (decodeHeader(_freqs) == 0)
            return startChunk - blkptr;

        _range = TOP_RANGE;
        _low = 0;
        _code = _bitstream.readBits(60);
        const int endChunk = (startChunk + sz < end) ? startChunk + sz : end;

        for (int i = startChunk; i < endChunk; i++)
            block[i] = decodeByte();

        startChunk = endChunk;
    }

    return len;
}

byte RangeDecoder::decodeByte()
{
    // Compute next low and range
    _range >>= _shift;
    const int count = int((_code - _low) / _range);
    const int symbol = _f2s[count];
    const uint64 cumFreq = _cumFreqs[symbol];
    const uint64 freq = _cumFreqs[symbol + 1] - cumFreq;
    _low += (cumFreq * _range);
    _range *= freq;

    // If the left-most digits are the same throughout the range, read bits from bitstream
    while (true) {
        if (((_low ^ (_low + _range)) & RANGE_MASK) != 0) {
            if (_range > BOTTOM_RANGE)
                  break;

            // Normalize
            _range = ~(_low-1) & BOTTOM_RANGE;
        }

        _code = (_code << 28) | _bitstream.readBits(28);
        _range <<= 28;
        _low <<= 28;
    }

    return (byte)symbol;
}
