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

#ifndef _InputBitStream_
#define _InputBitStream_

#include "BitStreamException.hpp"
#include "types.hpp"

namespace kanzi 
{

   class InputBitStream 
   {
   public:
       // Returns 1 or 0
       virtual int readBit() THROW = 0;

       virtual uint64 readBits(uint length) THROW = 0;

       virtual void close() THROW = 0;

       // Number of bits read
       virtual uint64 read() const = 0;

       // Return false when the bitstream is closed or the End-Of-Stream has been reached
       virtual bool hasMoreToRead() = 0;

       InputBitStream(){};

       virtual ~InputBitStream(){};
   };

}
#endif
