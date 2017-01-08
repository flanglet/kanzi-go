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

#ifndef _OutputBitStream_
#define _OutputBitStream_

#include "BitStreamException.hpp"
#include "types.hpp"

namespace kanzi 
{

   class OutputBitStream 
   {
   public:
       // Write the least significant bit of the input integer
       virtual void writeBit(int bit) THROW = 0;

       virtual int writeBits(uint64 bits, uint length) THROW = 0;

       virtual void close() THROW = 0;

       // Number of bits written
       virtual uint64 written() const = 0;

       OutputBitStream(){};

       virtual ~OutputBitStream(){};
   };

}
#endif
