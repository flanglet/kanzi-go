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

#ifndef _DebugInputBitStream_
#define _DebugInputBitStream_

#include <ostream>
#include "../IllegalArgumentException.hpp"
#include "../io/IOException.hpp"
#include "../InputBitStream.hpp"
#include "../OutputStream.hpp"

using namespace std;

namespace kanzi 
{

   class DebugInputBitStream : public InputBitStream 
   {
   private:
       InputBitStream& _delegate;
       OutputStream& _out;
       int _width;
       int _idx;
       bool _mark;
       bool _hexa;
       byte _current;

       void printByte(byte val);

   public:
       DebugInputBitStream(InputBitStream& ibs) THROW;

       DebugInputBitStream(InputBitStream& ibs, OutputStream& os) THROW;

       DebugInputBitStream(InputBitStream& ibs, OutputStream& os, int width) THROW;

       ~DebugInputBitStream();

       // Returns 1 or 0
       int readBit() THROW;

       uint64 readBits(uint length) THROW;

       // Number of bits read
       uint64 read() const { return _delegate.read(); }

       // Return false when the bitstream is closed or the End-Of-Stream has been reached
       bool hasMoreToRead() { return _delegate.hasMoreToRead(); }

       void close() THROW { _delegate.close(); }

       inline void showByte(bool show) { _hexa = show; }

       inline bool showByte() const { return _hexa; }

       inline void setMark(bool mark) { _mark = mark; }

       inline bool mark() const { return _mark; }
   };

}
#endif
