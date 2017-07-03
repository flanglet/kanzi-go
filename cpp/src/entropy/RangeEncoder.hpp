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

#ifndef _RangeEncoder_
#define _RangeEncoder_

#include "../EntropyEncoder.hpp"
#include "EntropyUtils.hpp"

using namespace std;

namespace kanzi
{

   // Based on Order 0 range coder by Dmitry Subbotin itself derived from the algorithm
   // described by G.N.N Martin in his seminal article in 1979.
   // [G.N.N. Martin on the Data Recording Conference, Southampton, 1979]
   // Optimized for speed.

   class RangeEncoder : public EntropyEncoder
   {
   public:
       RangeEncoder(OutputBitStream& bitstream, int chunkSize=DEFAULT_CHUNK_SIZE, int logRange=DEFAULT_LOG_RANGE) THROW;

       ~RangeEncoder() { dispose(); };

       int updateFrequencies(uint frequencies[], int size, int lr);

       int encode(byte block[], uint blkptr, uint len);

       OutputBitStream& getBitStream() const { return _bitstream; }

       void dispose(){};

   private:
       static const uint64 TOP_RANGE    = 0x0FFFFFFFFFFFFFFF;
       static const uint64 BOTTOM_RANGE = 0x000000000000FFFF;
       static const uint64 RANGE_MASK   = 0x0FFFFFFF00000000;
       static const int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
       static const int DEFAULT_LOG_RANGE = 13;

       uint64 _low;
       uint64 _range;
       uint _alphabet[256];
       uint _freqs[256];
       uint64 _cumFreqs[257];
       EntropyUtils _eu;
       OutputBitStream& _bitstream;
       uint _chunkSize;
       uint _logRange;
       uint _shift;

       int rebuildStatistics(byte block[], int start, int end, int lr);

       void encodeByte(byte b);

       bool encodeHeader(int alphabetSize, uint alphabet[], uint frequencies[], int lr);
   };

}
#endif
