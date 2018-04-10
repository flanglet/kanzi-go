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

// Implementation of an Asymmetric Numeral System decoder.
// See "Asymmetric Numeral System" by Jarek Duda at http://arxiv.org/abs/0902.0271
// Some code has been ported from https://github.com/rygorous/ryg_rans
// For an alternate C implementation example, see https://github.com/Cyan4973/FiniteStateEntropy

namespace kanzi 
{
   class ANSDecSymbol 
   {
   public:
      ANSDecSymbol() { _freq = 0; _cumFreq = 0; }
      ~ANSDecSymbol() { }
      void reset(int cumFreq, int freq, int logRange);

      int _cumFreq;
      int _freq;
   };


   class ANSRangeDecoder : public EntropyDecoder {
   public:
	   static const int ANS_TOP = 1 << 23;

      ANSRangeDecoder(InputBitStream& bitstream, int order = 0, int chunkSize = -1) THROW;

	   ~ANSRangeDecoder();

	   int decode(byte block[], uint blkptr, uint len);

	   InputBitStream& getBitStream() const { return _bitstream; }

	   void dispose() {};

   private:
	   static const int DEFAULT_ANS0_CHUNK_SIZE = 1 << 15; // 32 KB by default
	   static const int DEFAULT_LOG_RANGE = 13;

	   InputBitStream& _bitstream;
	   uint* _alphabet;
	   uint* _freqs;
	   byte* _f2s;
	   int _f2sSize;
	   ANSDecSymbol* _symbols;
	   uint _chunkSize;
	   uint _order;
	   uint _logRange;

	   void decodeChunk(byte block[], int start, int end);

	   int decodeHeader(uint frequencies[]);
   };

}
#endif
