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


#ifndef _RangeDecoder_
#define _RangeDecoder_

#include "../EntropyDecoder.hpp"

using namespace std;

namespace kanzi
{

   // Based on Order 0 range coder by Dmitry Subbotin itself derived from the algorithm
   // described by G.N.N Martin in his seminal article in 1979.
   // [G.N.N. Martin on the Data Recording Conference, Southampton, 1979]
   // Optimized for speed.

   class RangeDecoder : public EntropyDecoder {
   public:
       static const int DECODING_BATCH_SIZE = 12; // in bits
       static const int DECODING_MASK = (1 << DECODING_BATCH_SIZE) - 1;

       RangeDecoder(InputBitStream& bitstream, int chunkSize=DEFAULT_CHUNK_SIZE) THROW;

       ~RangeDecoder() { delete[] _f2s; dispose(); }

       int decode(byte block[], uint blkptr, uint len);

       InputBitStream& getBitStream() const { return _bitstream; }

       void dispose(){};

   private:
       static const uint64 TOP_RANGE    = 0x0FFFFFFFFFFFFFFF;
       static const uint64 BOTTOM_RANGE = 0x000000000000FFFF;
       static const uint64 RANGE_MASK   = 0x0FFFFFFF00000000;
       static const int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
       static const int DEFAULT_LOG_RANGE = 13;

       uint64 _code;
       uint64 _low;
       uint64 _range;
       uint _alphabet[256];
       uint _freqs[256];
       uint64 _cumFreqs[257];
       short* _f2s;
       int _f2s_length;
       InputBitStream& _bitstream;
       uint _chunkSize;
       uint _shift;

       int decodeHeader(uint frequencies[]);

       byte decodeByte();
   };

}
#endif
