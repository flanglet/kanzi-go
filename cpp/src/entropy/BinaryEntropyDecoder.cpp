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

#include "BinaryEntropyDecoder.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

BinaryEntropyDecoder::BinaryEntropyDecoder(InputBitStream& bitstream, Predictor* predictor, bool deallocate) THROW
: _bitstream(bitstream)
{
    if (predictor == nullptr)
       throw IllegalArgumentException("Invalid null predictor parameter");

    _predictor = predictor;
    _low = 0;
    _high = TOP;
    _current = 0;
    _initialized = false;
    _deallocate = deallocate;
}

BinaryEntropyDecoder::~BinaryEntropyDecoder()
{
    dispose();

    if (_deallocate)
       delete _predictor;
}

int BinaryEntropyDecoder::decode(byte block[], uint blkptr, uint len)
{
    if (isInitialized() == false)
        initialize();

    const int end = blkptr + len;

    for (int i = blkptr; i < end; i++)
        block[i] = decodeByte();

    return len;
}

inline byte BinaryEntropyDecoder::decodeByte()
{
    return byte((decodeBit() << 7)
        | (decodeBit() << 6)
        | (decodeBit() << 5)
        | (decodeBit() << 4)
        | (decodeBit() << 3)
        | (decodeBit() << 2)
        | (decodeBit() << 1)
        | decodeBit());
}

void BinaryEntropyDecoder::initialize()
{
    if (_initialized == true)
        return;

    _current = _bitstream.readBits(56);
    _initialized = true;
}

inline int BinaryEntropyDecoder::decodeBit()
{
    // Calculate interval split
    // Written in a way to maximize accuracy of multiplication/division
    const uint64 split = ((((_high - _low) >> 4) * uint64(_predictor->get())) >> 8) + _low;
    int bit;

    if (split >= _current) {
        bit = 1;
        _high = split;
    } else {
        bit = 0;
        _low = split + 1;
    }

    // Update predictor
    _predictor->update(bit);

    // Read 32 bits from bitstream
    while (((_low ^ _high) & MASK_24_56) == 0)
        read();

    return bit;
}

inline void BinaryEntropyDecoder::read()
{
    _low = (_low << 32) & MASK_0_56;
    _high = ((_high << 32) | MASK_0_32) & MASK_0_56;
    _current = ((_current << 32) | _bitstream.readBits(32)) & MASK_0_56;
}

void BinaryEntropyDecoder::dispose()
{
}