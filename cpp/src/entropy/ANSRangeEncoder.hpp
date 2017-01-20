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

#ifndef _ANSRangeEncoder_
#define _ANSRangeEncoder_

#include "../EntropyEncoder.hpp"
#include "EntropyUtils.hpp"

using namespace std;

namespace kanzi 
{

   // Implementation of an Asymmetric Numeral System encoder.
   // See "Asymmetric Numeral System" by Jarek Duda at http://arxiv.org/abs/0902.0271
   // For alternate C implementation examples, see https://github.com/Cyan4973/FiniteStateEntropy
   // and https://github.com/rygorous/ryg_rans

   class ANSRangeEncoder : public EntropyEncoder
   {
   public:
	   ANSRangeEncoder(OutputBitStream& bitstream, int chunkSize = DEFAULT_CHUNK_SIZE, int logRange = DEFAULT_LOG_RANGE) THROW;

	   ~ANSRangeEncoder() { delete[] _buffer;  dispose(); };

	   int updateFrequencies(uint frequencies[], int size, int lr);

	   int encode(byte block[], uint blkptr, uint len);

	   OutputBitStream& getBitStream() const { return _bitstream; }

	   void dispose() {};

   private:
	   static const uint64 ANS_TOP = 1 << 24;
	   static const int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
	   static const int DEFAULT_LOG_RANGE = 13;


	   uint _alphabet[256];
	   uint _freqs[256];
	   uint _cumFreqs[257];
	   int* _buffer;
	   uint _bufferSize;
	   EntropyUtils _eu;
	   OutputBitStream& _bitstream;
	   uint _chunkSize;
	   uint _logRange;


	   int rebuildStatistics(byte block[], int start, int end, int lr);

	   void encodeChunk(byte block[], int start, int end, int lr);

	   bool encodeHeader(int alphabetSize, uint alphabet[], uint frequencies[], int lr);
   };

}
#endif
