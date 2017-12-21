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

#ifndef _SnappyCodec_
#define _SnappyCodec_

#include "../Function.hpp"

namespace kanzi 
{

   // Snappy is a fast compression codec aiming for very high speed and
   // reasonable compression ratios.
   // This implementation is a port of the Go source at https://github.com/golang/snappy

   class SnappyCodec : public Function<byte>
   {
   public:
       SnappyCodec() { memset(_buffer, 0, sizeof(int)*MAX_TABLE_SIZE); }

       ~SnappyCodec( ){}

       bool forward(SliceArray<byte>& src, SliceArray<byte>& dst, int length);

       bool inverse(SliceArray<byte>& src, SliceArray<byte>& dst, int length);

       // Required encoding output buffer size
       int getMaxEncodedLength(int srcLen) const;


   private:
      static const int MAX_OFFSET     = 32768;
      static const int MAX_TABLE_SIZE = 16384;
      static const int TAG_LITERAL    = 0x00;
      static const int TAG_COPY1      = 0x01;
      static const int TAG_COPY2      = 0x02;
      static const int TAG_DEC_LEN1   = 0xF0;
      static const int TAG_DEC_LEN2   = 0xF4;
      static const int TAG_DEC_LEN3   = 0xF8;
      static const int TAG_DEC_LEN4   = 0xFC;
      static const byte TAG_ENC_LEN1  = byte(TAG_DEC_LEN1 | TAG_LITERAL);
      static const byte TAG_ENC_LEN2  = byte(TAG_DEC_LEN2 | TAG_LITERAL);
      static const byte TAG_ENC_LEN3  = byte(TAG_DEC_LEN3 | TAG_LITERAL);
      static const byte TAG_ENG_LEN4  = byte(TAG_DEC_LEN4 | TAG_LITERAL);
      static const byte B0            = byte(TAG_DEC_LEN4 | TAG_COPY2);
      static const uint HASH_SEED     = 0x1E35A7BD;

      int _buffer[MAX_TABLE_SIZE];

      static int emitLiteral(SliceArray<byte>& source, SliceArray<byte>& destination, int len);

      static int emitCopy(SliceArray<byte>& destination, int offset, int len);

      static int putUvarint(byte buf[], uint64 x);

      static uint64 getUvarint(SliceArray<byte>& iba) THROW; 
       
      static int getDecodedLength(SliceArray<byte>& source);

      static bool differentInts(byte block[], int srcIdx, int dstIdx);
   };


   // getMaxEncodedLength returns the maximum length of a snappy block, given its
   // uncompressed length.
   //
   // Compressed data can be defined as:
   //    compressed := item* literal*
   //    item       := literal* copy
   //
   // The trailing literal sequence has a space blowup of at most 62/60
   // since a literal of length 60 needs one tag byte + one extra byte
   // for length information.
   //
   // Item blowup is trickier to measure. Suppose the "copy" op copies
   // 4 bytes of data. Because of a special check in the encoding code,
   // we produce a 4-byte copy only if the offset is < 65536. Therefore
   // the copy op takes 3 bytes to encode, and this type of item leads
   // to at most the 62/60 blowup for representing literals.
   //
   // Suppose the "copy" op copies 5 bytes of data. If the offset is big
   // enough, it will take 5 bytes to encode the copy op. Therefore the
   // worst case here is a one-byte literal followed by a five-byte copy.
   // That is, 6 bytes of input turn into 7 bytes of "compressed" data.
   //
   // This last factor dominates the blowup, so the const estimate is:
   inline int SnappyCodec::getMaxEncodedLength(int srcLen) const
   {
       return 32 + srcLen + srcLen / 6;
   }

}
#endif