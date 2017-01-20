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

   // Simple implementation of a Run Length Codec
   // Length is transmitted as 1 or 2 bytes (minus 1 bit for the mask that indicates
   // whether a second byte is used). The run threshold can be provided.
   // For a run threshold of 2:
   // EG input: 0x10 0x11 0x11 0x17 0x13 0x13 0x13 0x13 0x13 0x13 0x12 (160 times) 0x14
   //   output: 0x10 0x11 0x11 0x17 0x13 0x13 0x13 0x05 0x12 0x12 0x80 0xA0 0x14
   class RLT : public Function<byte> 
   {
   public:
       RLT(int runThreshold=3);

       ~RLT() {}

       bool forward(SliceArray<byte>& pSrc, SliceArray<byte>& pDst, int length);

       bool inverse(SliceArray<byte>& pSrc, SliceArray<byte>& pDst, int length);

       int getRunThreshold() const { return _runThreshold; }

       // Required encoding output buffer size unknown => guess
       int getMaxEncodedLength(int srcLen) const { return srcLen; }

   private:
       static const int TWO_BYTE_RLE_MASK1 = 0x80;
       static const int TWO_BYTE_RLE_MASK2 = 0x7F;
       static const int MAX_RUN_VALUE = 0x7FFF;

       int _runThreshold;
   };

}
#endif
