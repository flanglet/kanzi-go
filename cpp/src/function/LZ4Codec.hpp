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

#ifndef _LZ4Codec_
#define _LZ4Codec_

#include "../Function.hpp"

namespace kanzi 
{

   // LZ4 is a very fast lossless compression algorithm created by Yann Collet.
   // See original code here: https://code.google.com/p/lz4/
   // More details on the algorithm are available here:
   // http://fastcompression.blogspot.com/2011/05/lz4-explained.html

   class LZ4Codec : public Function<byte>
   {
   public:
       LZ4Codec();

       ~LZ4Codec() { delete[] _buffer; }

       bool forward(SliceArray<byte>& src, SliceArray<byte>& dst, int length);

       bool inverse(SliceArray<byte>& src, SliceArray<byte>& dst, int length);

       // Required encoding output buffer size
       int getMaxEncodedLength(int srcLen) const { return srcLen + (srcLen / 255) + 16; }

   private:
      static const uint LZ4_HASH_SEED     = 0x9E3779B1;
      static const uint HASH_LOG          = 12;
      static const uint HASH_LOG_64K      = 13;
      static const int MAX_DISTANCE       = (1 << 16) - 1;
      static const int SKIP_STRENGTH      = 6;
      static const int LAST_LITERALS      = 5;
      static const int MIN_MATCH          = 4;
      static const int MF_LIMIT           = 12;
      static const int LZ4_64K_LIMIT      = MAX_DISTANCE + MF_LIMIT;
      static const int ML_BITS            = 4;
      static const int ML_MASK            = (1 << ML_BITS) - 1;
      static const int RUN_BITS           = 8 - ML_BITS;
      static const int RUN_MASK           = (1 << RUN_BITS) - 1;
      static const int COPY_LENGTH        = 8;
      static const int MIN_LENGTH         = 14;
      static const int MAX_LENGTH         = (32*1024*1024) - 4 - MIN_MATCH;
      static const int ACCELERATION       = 1;
      static const int SKIP_TRIGGER       = 6;
      static const int SEARCH_MATCH_NB    = ACCELERATION << SKIP_TRIGGER;

      int* _buffer;

      static int writeLength(byte block[], int len);

      static int writeLastLiterals(byte src[], byte dst[], int runLength);

      static bool differentInts(byte block[], int srcIdx, int dstIdx);

      static void customArrayCopy(byte src[], byte dst[], int len);
   };

}
#endif
