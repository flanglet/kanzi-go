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
#include "ANSRangeDecoder.hpp"
#include "../IllegalArgumentException.hpp"
#include "EntropyUtils.hpp"

using namespace kanzi;

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block.
// The default chunk size is 65536 bytes.
ANSRangeDecoder::ANSRangeDecoder(InputBitStream& bitstream, int order, int chunkSize) THROW : _bitstream(bitstream)
{
    if ((order != 0) && (order != 1))
        throw IllegalArgumentException("The order must be 0 or 1");

    if ((chunkSize != 0) && (chunkSize != -1) && (chunkSize < 1024))
        throw IllegalArgumentException("The chunk size must be at least 1024");

    if (chunkSize > 1 << 30)
        throw IllegalArgumentException("The chunk size must be at most 2^30");

    if (chunkSize == -1)
    	chunkSize = DEFAULT_ANS0_CHUNK_SIZE << (8*order);

    _chunkSize = chunkSize;
    _order = order;
    const int dim = 255 * order + 1;
    _alphabet = new uint[dim * 256];
    _freqs = new uint[dim * 256];
    _symbols = new ANSDecSymbol[dim * 256];
    _f2s = new byte[0];
    _f2sSize = 0;
}

ANSRangeDecoder::~ANSRangeDecoder()
{
    dispose();
    delete[] _symbols;
    delete[] _f2s;
    delete[] _freqs;
    delete[] _alphabet;
}

int ANSRangeDecoder::decodeHeader(uint frequencies[])
{
    int res = 0;
    const int dim = 255 * _order + 1;
    _logRange = int(8 + _bitstream.readBits(3));
    const int scale = 1 << _logRange;

    if (_f2sSize < dim * scale) {
        delete[] _f2s;
        _f2sSize = dim * scale;
        _f2s = new byte[_f2sSize];
    }

    for (int k = 0; k < dim; k++) {
        uint* f = &frequencies[k << 8];
        uint* curAlphabet = &_alphabet[k << 8];
        int alphabetSize = EntropyUtils::decodeAlphabet(_bitstream, curAlphabet);

        if (alphabetSize == 0)
            continue;

        if (alphabetSize != 256)
            memset(f, 0, sizeof(uint) * 256);

        const int chkSize = (alphabetSize > 64) ? 16 : 8;
        int sum = 0;
        int llr = 3;

        while (uint(1 << llr) <= _logRange)
            llr++;

        // Decode all frequencies (but the first one) by chunks
        for (int i = 1; i < alphabetSize; i += chkSize) {
            // Read frequencies size for current chunk
            const int logMax = int(1 + _bitstream.readBits(llr));

            if (1 << logMax > scale) {
                stringstream ss;
                ss << "Invalid bitstream: incorrect frequency size ";
                ss << logMax << " in ANS range decoder";
                throw BitStreamException(ss.str(), BitStreamException::INVALID_STREAM);
            }

            const int endj = (i + chkSize < alphabetSize) ? i + chkSize : alphabetSize;

            // Read frequencies
            for (int j = i; j < endj; j++) {
                const int freq = int(_bitstream.readBits(logMax));

                if ((freq < 0) || (freq >= scale)) {
                    stringstream ss;
                    ss << "Invalid bitstream: incorrect frequency " << freq;
                    ss << " for symbol '" << curAlphabet[j] << "' in ANS range decoder";
                    throw BitStreamException(ss.str(), BitStreamException::INVALID_STREAM);
                }

                f[curAlphabet[j]] = uint(freq);
                sum += freq;
            }
        }

        // Infer first frequency
        if (scale <= sum) {
            stringstream ss;
            ss << "Invalid bitstream: incorrect frequency " << frequencies[_alphabet[0]];
            ss << " for symbol '" << _alphabet[0] << "' in ANS range decoder";
            throw BitStreamException(ss.str(),
                BitStreamException::INVALID_STREAM);
        }

        f[curAlphabet[0]] = uint(scale - sum);
        sum = 0;
        ANSDecSymbol* symb = &_symbols[k << 8];
        byte* freq2sym = &_f2s[k << _logRange];

        // Create reverse mapping
        for (int i = 0; i < 256; i++) {
            if (f[i] == 0)
                continue;

            for (int j = f[i] - 1; j >= 0; j--)
                freq2sym[sum + j] = byte(i);

            symb[i].reset(sum, f[i], _logRange);
            sum += f[i];
        }

        res += alphabetSize;
    }

    return res;
}

int ANSRangeDecoder::decode(byte block[], uint blkptr, uint len)
{
    if (len == 0)
        return 0;

    const int end = blkptr + len;
    const int sz = (_chunkSize == 0) ? len : _chunkSize;
    int startChunk = blkptr;

    while (startChunk < end) {
        if (decodeHeader(_freqs) == 0)
            return startChunk - blkptr;

        const int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
        decodeChunk(block, startChunk, endChunk);
        startChunk = endChunk;
    }

    return len;
}

void ANSRangeDecoder::decodeChunk(byte block[], int start, int end)
{
    // Read initial ANS state
    int st = int(_bitstream.readBits(32));
    const uint mask = (1 << _logRange) - 1;

    if (_order == 0) {
        byte* freq2sym = &_f2s[0];
        ANSDecSymbol* symb = &_symbols[0];

        for (int i = start; i < end; i++) {
            byte cur = freq2sym[st & mask];
            block[i] = cur;
            const ANSDecSymbol sym = symb[cur & 0xFF];

            // Compute next ANS state
            // D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
            st = int(sym._freq * (st >> _logRange) + (st & mask) - sym._cumFreq);

            // Normalize
            while (st < ANS_TOP)
                st = (st << 8) | int(_bitstream.readBits(8));
        }
    }
    else {
        int prv = 0;

        for (int i = start; i < end; i++) {
            byte cur = _f2s[(prv << _logRange) + (st & mask)];
            block[i] = cur;
            const ANSDecSymbol sym = _symbols[(prv << 8) + (cur & 0xFF)];

            // Compute next ANS state
            // D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
            st = int(sym._freq * (st >> _logRange) + (st & mask) - sym._cumFreq);

            // Normalize
            while (st < ANS_TOP)
                st = (st << 8) | int(_bitstream.readBits(8));

            prv = cur & 0xFF;
        }
    }
}

void ANSDecSymbol::reset(int cumFreq, int freq, int logRange)
{
    // Mirror encoder
    if (freq >= 1<<logRange)
        freq = (1<<logRange) - 1;
    
    _cumFreq = cumFreq;
    _freq = freq;
}