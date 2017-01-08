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

#ifndef _DefaultInputBitStream_
#define _DefaultInputBitStream_

#include <istream>
#include "../InputBitStream.hpp"
#include "../InputStream.hpp"

using namespace std;

namespace kanzi 
{

   class DefaultInputBitStream : public InputBitStream 
   {
   private:
       InputStream& _is;
       byte* _buffer;
       int _position; // index of current byte (consumed if bitIndex == 63)
       uint _bitIndex; // index of current bit to read
       uint64 _read;
       uint64 _current;
       bool _closed;
       int _maxPosition;
       uint _bufferSize;

       int readFromInputStream(uint count) THROW;

       void pullCurrent();

   public:
       // Returns 1 or 0
       int readBit() THROW;

       uint64 readBits(uint length) THROW;

       void close() THROW;

       // Number of bits read
       uint64 read() const
       {
           return _read + ((uint64)_position << 3) - _bitIndex;
       }

       // Return false when the bitstream is closed or the End-Of-Stream has been reached
       bool hasMoreToRead();

       bool isClosed() const { return _closed; }

       DefaultInputBitStream(InputStream& is, uint bufferSize=65536) THROW;

       ~DefaultInputBitStream();
   };

}
#endif
