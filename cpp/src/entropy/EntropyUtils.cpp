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

#include <algorithm>
#include <vector>
#include <sstream>
#include "../IllegalArgumentException.hpp"
#include "EntropyUtils.hpp"

using namespace kanzi;

class FreqSortData {
public:
    uint _symbol;
    int* _errors;
    uint* _frequencies;

    FreqSortData(int errors[], uint frequencies[], int symbol)
    {
        _errors = errors;
        _frequencies = frequencies;
        _symbol = symbol;
    }
};

struct FreqDataComparator {
    bool operator()(FreqSortData* fd1, FreqSortData* fd2) const
    {
        int res = fd1->_errors[fd1->_symbol] - fd2->_errors[fd2->_symbol];

        // Increasing frequency
        if (res == 0) {
            res = fd1->_frequencies[fd1->_symbol] - fd2->_frequencies[fd2->_symbol];

            // Increasing symbol
            if (res == 0)
                return fd1->_symbol < fd2->_symbol;
        }

        return res < 0;
    }
};

// alphabet must be sorted in increasing order
// length = alphabet array length
int EntropyUtils::encodeAlphabet(OutputBitStream& obs, uint alphabet[], int length, int count)
{
    // Alphabet length must be a power of 2
    if ((length & (length - 1)) != 0)
        return -1;

    if (count > length)
        return -1;

    // First, push alphabet encoding mode
    if ((length > 0) && (count == length)) {
        // uint64 alphabet
        obs.writeBit(FULL_ALPHABET);

        if (count == 256)
            obs.writeBit(ALPHABET_256); // shortcut
        else {
            int log = 1;

            while (1 << log <= count)
                log++;

            // Write alphabet size
            obs.writeBit(ALPHABET_NOT_256);
            obs.writeBits(log - 1, 5);
            obs.writeBits(count, log);
        }

        return count;
    }

    obs.writeBit(PARTIAL_ALPHABET);

    if ((length == 256) && (count >= 32) && (count <= 224)) {
        // Regular alphabet of symbols less than 256
        obs.writeBit(BIT_ENCODED_ALPHABET_256);
        uint64 masks[4] = { 0, 0, 0, 0 };

        for (int i = 0; i < count; i++)
            masks[alphabet[i] >> 6] |= ((uint64(1)) << (alphabet[i] & 63));

        for (int i = 0; i < 4; i++)
            obs.writeBits(masks[i], 64);

        return count;
    }

    obs.writeBit(DELTA_ENCODED_ALPHABET);
    int* diffs = new int[count];

    if (length - count < count) {
        // Encode all missing symbols
        count = length - count;
        int log = 1;

        while (1 << log <= count)
            log++;

        // Write length
        obs.writeBits(log - 1, 4);
        obs.writeBits(count, log);

        if (count == 0) {
            delete[] diffs;
            return 0;
        }

        obs.writeBit(ABSENT_SYMBOLS_MASK);
        log = 1;

        while (1 << log <= length)
            log++;

        // Write log(alphabet size)
        obs.writeBits(log - 1, 5);
        uint symbol = 0;
        int previous = 0;

        // Create deltas of missing symbols
        for (int n = 0, i = 0; n < count;) {
            if (symbol == alphabet[i]) {
                if (i < length - 1 - count)
                    i++;

                symbol++;
                continue;
            }

            diffs[n] = symbol - previous;
            symbol++;
            previous = symbol;
            n++;
        }
    }
    else {
        // Encode all present symbols
        int log = 1;

        while (1 << log <= count)
            log++;

        // Write length
        obs.writeBits(log - 1, 4);
        obs.writeBits(count, log);

        if (count == 0) {
            delete[] diffs;
            return 0;
        }

        obs.writeBit(PRESENT_SYMBOLS_MASK);
        int previous = 0;

        // Create deltas of present symbols
        for (int i = 0; i < count; i++) {
            diffs[i] = alphabet[i] - previous;
            previous = alphabet[i] + 1;
        }
    }

    const int ckSize = (count <= 64) ? 8 : 16;

    // Encode all deltas by chunks
    for (int i = 0; i < count; i += ckSize) {
        int max = 0;

        // Find log(max(deltas)) for this chunk
        for (int j = i; (j < count) && (j < i + ckSize); j++) {
            if (max < diffs[j])
                max = diffs[j];
        }

        int log = 1;

        while (1 << log <= max)
            log++;

        obs.writeBits(log - 1, 4);

        // Write deltas for this chunk
        for (int j = i; (j < count) && (j < i + ckSize); j++)
            encodeSize(obs, log, diffs[j]);
    }

    delete[] diffs;
    return count;
}

void EntropyUtils::encodeSize(OutputBitStream& obs, int log, int val)
{
    obs.writeBits(val, log);
}

uint64 EntropyUtils::decodeSize(InputBitStream& ibs, int log)
{
    return ibs.readBits(log);
}

int EntropyUtils::decodeAlphabet(InputBitStream& ibs, uint alphabet[]) THROW
{
    // Read encoding mode from bitstream
    const int alphabetType = ibs.readBit();

    if (alphabetType == FULL_ALPHABET) {
        int alphabetSize;

        if (ibs.readBit() == ALPHABET_256)
            alphabetSize = 256;
        else {
            int log = 1 + int(ibs.readBits(5));
            alphabetSize = int(ibs.readBits(log));
        }

        if (alphabetSize > 256) {
            stringstream ss;
            ss << "Invalid bitstream: incorrect alphabet size: " << alphabetSize;
            throw BitStreamException(ss.str(), BitStreamException::INVALID_STREAM);
        }

        // Full alphabet
        for (int i = 0; i < alphabetSize; i++)
            alphabet[i] = i;

        return alphabetSize;
    }

    int count = 0;
    const int mode = ibs.readBit();

    if (mode == BIT_ENCODED_ALPHABET_256) {
        // Decode presence flags
        for (int i = 0; i < 256; i += 64) {
            const uint64 val = ibs.readBits(64);

            for (int j = 0; j < 64; j++) {
                if ((val & ((uint64(1)) << j)) != 0) {
                    alphabet[count] = i + j;
                    count++;
                }
            }
        }

        return count;
    }

    // DELTA_ENCODED_ALPHABET
    int log = 1 + int(ibs.readBits(4));
    count = int(ibs.readBits(log));

    if (count == 0)
        return 0;

    const int ckSize = (count <= 64) ? 8 : 16;
    int n = 0;
    int symbol = 0;

    if (ibs.readBit() == ABSENT_SYMBOLS_MASK) {
        int alphabetSize = 1 << int(ibs.readBits(5));

        // Read missing symbols
        for (int i = 0; i < count; i += ckSize) {
            log = 1 + int(ibs.readBits(4));

            // Read deltas for this chunk
            for (int j = i; (j < count) && (j < i + ckSize); j++) {
                const int next = symbol + int(decodeSize(ibs, log));

                while ((symbol < next) && (n < alphabetSize)) {
                    alphabet[n] = symbol++;
                    n++;
                }

                symbol++;
            }
        }

        count = alphabetSize - count;

        while (n < count)
            alphabet[n++] = symbol++;
    }
    else {
        // Read present symbols
        for (int i = 0; i < count; i += ckSize) {
            log = 1 + int(ibs.readBits(4));

            // Read deltas for this chunk
            for (int j = i; (j < count) && (j < i + ckSize); j++) {
                symbol += int(decodeSize(ibs, log));
                alphabet[j] = symbol;
                symbol++;
            }
        }
    }

    return count;
}

// Return the first order entropy in the [0..1024] range
// Fills in the histogram with order 0 frequencies. Incoming array size must be 256
int EntropyUtils::computeFirstOrderEntropy1024(byte block[], int length, int histo[])
{
    if (length == 0)
        return 0;

    for (int i = 0; i < 256; i++) {
        histo[i] = 0;
    }

    const int end8 = length & -8;

    for (int i = 0; i < end8; i += 8) {
        histo[block[i] & 0xFF]++;
        histo[block[i + 1] & 0xFF]++;
        histo[block[i + 2] & 0xFF]++;
        histo[block[i + 3] & 0xFF]++;
        histo[block[i + 4] & 0xFF]++;
        histo[block[i + 5] & 0xFF]++;
        histo[block[i + 6] & 0xFF]++;
        histo[block[i + 7] & 0xFF]++;
    }

    for (int i = end8; i < length; i++)
        histo[block[i] & 0xFF]++;

    int64 sum = 0;
    const int logLength1024 = Global::log2_1024(length);

    for (int i = 0; i < 256; i++) {
        if (histo[i] == 0)
            continue;

        sum += int64((histo[i] * (logLength1024 - Global::log2_1024(histo[i]))) >> 3);
    }

    return int(sum / int64(length));
}

// Returns the size of the alphabet
// length is the length of the alphabet array
// 'totalFreq 'is the sum of frequencies.
// 'scale' is the target new total of frequencies
// The alphabet and freqs parameters are updated
int EntropyUtils::normalizeFrequencies(uint freqs[], uint alphabet[], int length, uint totalFreq, uint scale) THROW
{
    if (length > 256) {
        stringstream ss;
        ss << "Invalid alphabet size parameter: " << scale << " (must be less than or equal to 256)";
        throw IllegalArgumentException(ss.str());
    }

    if ((scale < 1 << 8) || (scale > 1 << 16)) {
        stringstream ss;
        ss << "Invalid scale parameter: " << scale << " (must be in [256..65536])";
        throw IllegalArgumentException(ss.str());
    }

    if ((length == 0) || (totalFreq == 0))
        return 0;

    // Number of present symbols
    int alphabetSize = 0;

    // shortcut
    if (totalFreq == scale) {
        for (int i = 0; i < 256; i++) {
            if (freqs[i] != 0)
                alphabet[alphabetSize++] = i;
        }

        return alphabetSize;
    }

    uint sumScaledFreq = 0;
    uint sumFreq = 0;
    uint freqMax = 0;
    int idxMax = -1;
    int* errors = &_buffer[0];

    // Scale frequencies by stretching distribution over complete range
    for (int i = 0; (i < length) && (sumFreq < totalFreq); i++) {
        alphabet[i] = 0;
        errors[i] = 0;
        const uint f = freqs[i];

        if (f == 0)
            continue;

        if (f > freqMax) {
            freqMax = f;
            idxMax = i;
        }

        sumFreq += f;
        int64 sf = int64(f) * scale;
        uint scaledFreq;

        if (sf <= int64(totalFreq)) {
            // Quantum of frequency
            scaledFreq = 1;
        }
        else {
            // Find best frequency rounding value
            scaledFreq = uint(sf / totalFreq);
            int64 errCeiling = (scaledFreq + 1) * int64(totalFreq) - sf;
            int64 errFloor = sf - scaledFreq * int64(totalFreq);

            if (errCeiling < errFloor) {
                scaledFreq++;
                errors[i] = int(errCeiling);
            }
            else {
                errors[i] = int(errFloor);
            }
        }

        alphabet[alphabetSize++] = i;
        sumScaledFreq += scaledFreq;
        freqs[i] = scaledFreq;
    }

    if (alphabetSize == 0)
        return 0;

    if (alphabetSize == 1) {
        freqs[alphabet[0]] = scale;
        return 1;
    }

    if (sumScaledFreq != scale) {
        if (int(freqs[idxMax]) > int(sumScaledFreq - scale)) {
            // Fast path: just adjust the max frequency
            freqs[idxMax] += int(scale - sumScaledFreq);
        }
        else {
            // Slow path: spread error across frequencies
            const int inc = (sumScaledFreq > scale) ? -1 : 1;
            vector<FreqSortData*> queue;

            // Create sorted queue of present symbols (except those with 'quantum frequency')
            for (int i = 0; i < alphabetSize; i++) {
                if ((errors[alphabet[i]] > 0) && (int(freqs[alphabet[i]]) != -inc)) {
                    queue.push_back(new FreqSortData(errors, freqs, alphabet[i]));
                }
            }

            make_heap(queue.begin(), queue.end(), FreqDataComparator());

            while ((sumScaledFreq != scale) && (queue.size() > 0)) {
                // Remove symbol with highest error
                pop_heap(queue.begin(), queue.end(), FreqDataComparator());
                FreqSortData* fsd = queue.back();
                queue.pop_back();

                // Do not zero out any frequency
                if (int(freqs[fsd->_symbol]) == -inc) {
                    delete fsd;
                    continue;
                }

                // Distort frequency and error
                freqs[fsd->_symbol] += inc;
                errors[fsd->_symbol] -= scale;
                sumScaledFreq += inc;
                queue.push_back(fsd);
                push_heap(queue.begin(), queue.end(), FreqDataComparator());
            }

            while (queue.size() > 0) {
                FreqSortData* fsd = queue.back();
                queue.pop_back();
                delete fsd;
            }
        }
    }

    return alphabetSize;
}
