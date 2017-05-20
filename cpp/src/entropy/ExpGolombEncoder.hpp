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

#ifndef _ExpGolombEncoder_
#define _ExpGolombEncoder_

#include "../EntropyEncoder.hpp"

namespace kanzi 
{

   class ExpGolombEncoder : public EntropyEncoder 
   {
   private:
       OutputBitStream& _bitstream;
       int _signed;

   public:
       ExpGolombEncoder(OutputBitStream& bitstream, bool sign=true);

       ~ExpGolombEncoder() { dispose(); }

       int encode(byte block[], uint blkptr, uint len);

       OutputBitStream& getBitStream() const { return _bitstream; };

       void encodeByte(byte val);

       void dispose() {}

       bool isSigned() const { return _signed == 1; }
   };

}
#endif
