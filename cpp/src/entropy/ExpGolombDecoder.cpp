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

#include "ExpGolombDecoder.hpp"

using namespace kanzi;

ExpGolombDecoder::ExpGolombDecoder(InputBitStream& bitstream, bool sgn)
    : _bitstream(bitstream)
{
    _signed = sgn;
}


int ExpGolombDecoder::decode(byte block[], uint blkptr, uint len)
{
    const int end = blkptr + len;

    for (int i = blkptr; i < end; i++)
        block[i] = decodeByte();

    return len;
}

byte ExpGolombDecoder::decodeByte()
{
    if (_bitstream.readBit() == 1)
        return 0;

    int log2 = 1;

    while (_bitstream.readBit() == 0)
        log2++;

    if (_signed == true) {
        // Decode signed: read value + sign
        byte res = byte(_bitstream.readBits(log2 + 1));
        byte sgn = res & 1;
        res = (res >> 1) + (1 << log2) - 1;
        return byte((res - sgn) ^ -sgn); // res or -res
    }

    // Decode unsigned
    return byte((1 << log2) - 1 + _bitstream.readBits(log2));
}