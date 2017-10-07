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

#ifndef _X86Codec_
#define _X86Codec_

#include "../Function.hpp"

namespace kanzi 
{
   // Adapted from MCM: https://github.com/mathieuchartier/mcm/blob/master/X86Binary.hpp
   class X86Codec : public Function<byte> {
   public:
       X86Codec() { }

       ~X86Codec() {}

       bool forward(SliceArray<byte>& source, SliceArray<byte>& destination, int length);

       bool inverse(SliceArray<byte>& source, SliceArray<byte>& destination, int length);

       int getMaxEncodedLength(int inputLen) const { return (inputLen * 5) >> 2; }

   private:

      static const int INSTRUCTION_MASK = 0xFE;
      static const int INSTRUCTION_JUMP = 0xE8;
      static const int ADDRESS_MASK = 0xD5; 
      static const int ESCAPE = 0x02;
   };

}
#endif
