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

#ifndef _BinaryEntropyDecoder_
#define _BinaryEntropyDecoder_

#include "../EntropyDecoder.hpp"
#include "../Predictor.hpp"

namespace kanzi 
{

   // This class is a generic implementation of a bool entropy decoder
   class BinaryEntropyDecoder : public EntropyDecoder 
   {
   private:
       static const uint64 TOP = 0x00FFFFFFFFFFFFFF;
       static const uint64 MASK_24_56 = 0x00FFFFFFFF000000;
       static const uint64 MASK_0_56 = 0x00FFFFFFFFFFFFFF;
       static const uint64 MASK_0_32 = 0x00000000FFFFFFFF;

       Predictor* _predictor;
       uint64 _low;
       uint64 _high;
       uint64 _current;
       InputBitStream& _bitstream;
       bool _initialized;
       bool _deallocate;

   protected:
       virtual void initialize();

       void read();

   public:
       BinaryEntropyDecoder(InputBitStream& bitstream, Predictor* predictor, bool deallocate=true) THROW;

       virtual ~BinaryEntropyDecoder();

       int decode(byte array[], uint blkptr, uint len);

       InputBitStream& getBitStream() const { return _bitstream; };

       bool isInitialized() const { return _initialized; };

       virtual void dispose();

       byte decodeByte();

       virtual int decodeBit();
   };

}
#endif