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

#ifndef _RLT_
#define _RLT_

#include "../Function.hpp"

namespace kanzi 
{

   // Implementation of Mespotine RLE
   // See [An overhead-reduced and improved Run-Length-Encoding Method] by Meo Mespotine
   // Length is transmitted as 1 to 3 bytes. The run threshold can be provided.
   // EG. runThreshold = 2 and RUN_LEN_ENCODE1 = 239 => RUN_LEN_ENCODE2 = 4096
   // 2    <= runLen < 239+2      -> 1 byte
   // 241  <= runLen < 4096+2     -> 2 bytes
   // 4098 <= runLen < 65536+4098 -> 3 bytes

   class RLT : public Function<byte> 
   {
   public:
       RLT(int runThreshold=2);

       ~RLT() {}

       bool forward(SliceArray<byte>& pSrc, SliceArray<byte>& pDst, int length);

       bool inverse(SliceArray<byte>& pSrc, SliceArray<byte>& pDst, int length);

       int getRunThreshold() const { return _runThreshold; }

       // Required encoding output buffer size unknown => guess
       int getMaxEncodedLength(int srcLen) const { return srcLen + 32; }

   private:
       static const int RUN_LEN_ENCODE1 = 224; // used to encode run length
       static const int RUN_LEN_ENCODE2 = (256-1-RUN_LEN_ENCODE1) << 8; // used to encode run length
       static const int MAX_RUN = 0xFFFF + RUN_LEN_ENCODE2; 

       int _runThreshold;
   };

}
#endif
