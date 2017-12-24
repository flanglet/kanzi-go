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

#include "RiceGolombDecoder.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

RiceGolombDecoder::RiceGolombDecoder(InputBitStream& bitstream, uint logBase, bool sgn) THROW
    : _bitstream(bitstream)
{
    if ((logBase < 1) || (logBase > 12))
       throw IllegalArgumentException("Invalid logBase value (must be in [1..12])");

    _signed = sgn;
    _logBase = logBase;
}


int RiceGolombDecoder::decode(byte block[], uint blkptr, uint len)
{
    const int end = blkptr + len;

    for (int i = blkptr; i < end; i++)
        block[i] = decodeByte();

    return len;
}

inline byte RiceGolombDecoder::decodeByte()
{
       uint64 q = 0;

       // quotient is unary encoded
       while (_bitstream.readBit() == 0)
          q++;

       // remainder is binary encoded
       const uint64 res = (q << _logBase) | _bitstream.readBits(_logBase);

       if ((res != 0) && (_signed == true))
       {
          if (_bitstream.readBit() == 1)
             return byte(~res+1);
       }

       return byte(res);
}