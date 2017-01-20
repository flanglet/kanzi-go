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


#ifndef _ANSRangeDecoder_
#define _ANSRangeDecoder_

#include "../EntropyDecoder.hpp"

using namespace std;

namespace kanzi 
{

   // Implementation of an Asymmetric Numeral System decoder.
   // See "Asymmetric Numeral System" by Jarek Duda at http://arxiv.org/abs/0902.0271
   // For alternate C implementation examples, see https://github.com/Cyan4973/FiniteStateEntropy
   // and https://github.com/rygorous/ryg_rans

   class ANSRangeDecoder : public EntropyDecoder {
   public:
	   ANSRangeDecoder(InputBitStream& bitstream, int chunkSize = DEFAULT_CHUNK_SIZE) THROW;

	   ~ANSRangeDecoder() { delete[] _f2s; dispose(); }

	   int decode(byte block[], uint blkptr, uint len);

	   InputBitStream& getBitStream() const { return _bitstream; }

	   void dispose() {};

   private:
	   static const uint64 ANS_TOP = 1 << 24;
	   static const int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
	   static const int DEFAULT_LOG_RANGE = 13;

	   uint _alphabet[256];
	   uint _freqs[256];
	   uint _cumFreqs[257];
	   short* _f2s;
	   int _f2sSize;
	   InputBitStream& _bitstream;
	   uint _chunkSize;
	   uint _logRange;

	   void decodeChunk(byte block[], int start, int end);

	   int decodeHeader(uint frequencies[]);
   };

}
#endif
