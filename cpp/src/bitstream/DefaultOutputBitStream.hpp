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

#ifndef _DefaultOutputBitStream_
#define _DefaultOutputBitStream_

#include "../OutputStream.hpp"
#include "../OutputBitStream.hpp"

using namespace std;

namespace kanzi 
{

   class DefaultOutputBitStream : public OutputBitStream
   {
   private:
       OutputStream&_os;
       byte* _buffer;
       bool _closed;
       uint _bufferSize;
       uint _position; // index of current byte in buffer
       int _bitIndex; // index of current bit to write in current
       uint64 _written;
       uint64 _current; // cached bits

       void pushCurrent() THROW;

       void flush() THROW;

   public:
       DefaultOutputBitStream(OutputStream& os, uint bufferSize=65536) THROW;

       ~DefaultOutputBitStream();

       void writeBit(int bit) THROW;

       int writeBits(uint64 bits, uint length) THROW;

       void close() THROW;

       // Return number of bits written so far
       uint64 written() const
       {
           // Number of bits flushed + bytes written in memory + bits written in memory
           return _written + ((uint64)_position << 3) + (63 - _bitIndex);
       }

       bool isClosed() const { return _closed; }
   };

}
#endif
