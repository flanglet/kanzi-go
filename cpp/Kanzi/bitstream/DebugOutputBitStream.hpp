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

#ifndef _DebugOutputBitStream_
#define _DebugOutputBitStream_

#include <ostream>
#include "../IllegalArgumentException.hpp"
#include "../io/IOException.hpp"
#include "../OutputBitStream.hpp"
#include "../OutputStream.hpp"

using namespace std;

namespace kanzi 
{

   class DebugOutputBitStream : public OutputBitStream 
   {
   private:
       OutputBitStream& _delegate;
       OutputStream& _out;
       int _width;
       int _idx;
       bool _mark;
       bool _hexa;
       byte _current;

       void printByte(byte val);

   public:
       DebugOutputBitStream(OutputBitStream& obs) THROW;

       DebugOutputBitStream(OutputBitStream& obs, OutputStream& os) THROW;

       DebugOutputBitStream(OutputBitStream& obs, OutputStream& os, int width) THROW;

       ~DebugOutputBitStream();

       void writeBit(int bit) THROW;

       int writeBits(uint64 bits, uint length) THROW;

       // Return number of bits written so far
       uint64 written() const { return _delegate.written(); }

       void close() THROW { _delegate.close(); }

       inline void showByte(bool show) { _hexa = show; }

       inline bool showByte() const { return _hexa; }

       inline void setMark(bool mark) { _mark = mark; }

       inline bool mark() const { return _mark; }
   };

}
#endif
