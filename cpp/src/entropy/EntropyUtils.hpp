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

#ifndef _EntropyUtils_
#define _EntropyUtils_

#include <cstring>
#include "../InputBitStream.hpp"
#include "../OutputBitStream.hpp"

namespace kanzi 
{

   class EntropyUtils
   {
   private:
       static const int FULL_ALPHABET = 0;
       static const int PARTIAL_ALPHABET = 1;
       static const int ALPHABET_256 = 0;
       static const int ALPHABET_NOT_256 = 1;
       static const int DELTA_ENCODED_ALPHABET = 0;
       static const int BIT_ENCODED_ALPHABET_256 = 1;
       static const int PRESENT_SYMBOLS_MASK = 0;
       static const int ABSENT_SYMBOLS_MASK = 1;

       int _buffer[65536];

       static void encodeSize(OutputBitStream& obs, int log, int val);

       static uint64 decodeSize(InputBitStream& ibs, int log);

   public:
       static const int INCOMPRESSIBLE_THRESHOLD = 973; // 0.95*1024

       EntropyUtils() { memset(_buffer, 0, sizeof(int) * 65536); }

       ~EntropyUtils() {}

       static int encodeAlphabet(OutputBitStream& obs, uint alphabet[], int length, int count);

       static int decodeAlphabet(InputBitStream& ibs, uint alphabet[]) THROW;

       int normalizeFrequencies(uint freqs[], uint alphabet[], int length, uint totalFreq, uint scale) THROW;

       static int computeFirstOrderEntropy1024(byte block[], int length, int histo[]);
   };

}
#endif