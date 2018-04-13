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

#include "BinaryEntropyEncoder.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

BinaryEntropyEncoder::BinaryEntropyEncoder(OutputBitStream& bitstream, Predictor* predictor, bool deallocate) THROW
: _bitstream(bitstream)
{
    if (predictor == nullptr)
       throw IllegalArgumentException("Invalid null predictor parameter");

    _predictor = predictor;
    _low = 0;
    _high = TOP;
    _disposed = false;
    _deallocate = deallocate;
}

BinaryEntropyEncoder::~BinaryEntropyEncoder()
{
    dispose();

    if (_deallocate)
       delete _predictor;
}

int BinaryEntropyEncoder::encode(byte block[], uint blkptr, uint len)
{
    const int end = blkptr + len;

    for (int i = blkptr; i < end; i++)
        encodeByte(block[i]);

    return len;
}

inline void BinaryEntropyEncoder::encodeByte(byte val)
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

inline void BinaryEntropyEncoder::encodeBit(int bit)
{
    // Calculate interval split
    // Written in a way to maximize accuracy of multiplication/division
    const uint64 split = (((_high - _low) >> 4) * uint64(_predictor->get())) >> 8;

    // Update fields with new interval bounds
    _high -= (-bit & (_high - _low - split));
    _low += (~-bit & (split + 1));

    // Update predictor
    _predictor->update(bit);

    // Write unchanged first 32 bits to bitstream
    while (((_low ^ _high) & MASK_24_56) == 0)
        flush();
}

inline void BinaryEntropyEncoder::flush()
{
    _bitstream.writeBits(_high >> 24, 32);
    _low <<= 32;
    _high = (_high << 32) | MASK_0_32;
}

void BinaryEntropyEncoder::dispose()
{
    if (_disposed == true)
        return;

    _disposed = true;
    _bitstream.writeBits(_low | MASK_0_24, 56);
}