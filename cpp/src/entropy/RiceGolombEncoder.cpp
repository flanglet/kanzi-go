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

#include "RiceGolombEncoder.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

RiceGolombEncoder::RiceGolombEncoder(OutputBitStream& bitstream, uint logBase, bool sgn) THROW
    : _bitstream(bitstream)
{
    if ((logBase < 1) || (logBase > 12))
       throw IllegalArgumentException("Invalid logBase value (must be in [1..12])");

    _signed = sgn;
    _logBase = logBase;
    _base = 1 << _logBase;
}

int RiceGolombEncoder::encode(byte arr[], uint blkptr, uint len)
{
    const int end = blkptr + len;

    for (int i = blkptr; i < end; i++)
        encodeByte(arr[i]);

    return len;
}

inline void RiceGolombEncoder::encodeByte(byte val)
{
       if (val == 0)
       {
          _bitstream.writeBits(_base, _logBase+1);
          return;
       }

       int32 iVal = val;
       iVal = (iVal + (iVal >> 31)) ^ (iVal >> 31); // abs(val2)

        // quotient is unary encoded, remainder is binary encoded
       int emit = _base | (iVal & (_base-1));
       int n = (1 + (iVal >> _logBase)) + _logBase;

       if (_signed == true)
       {
          // Add 0 for positive and 1 for negative sign
          n++;
          emit = (emit << 1) | (uint32(val) >> 31);
       }

       _bitstream.writeBits(emit, n);
}
