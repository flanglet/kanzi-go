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

#ifndef _NullEntropyEncoder_
#define _NullEntropyEncoder_

namespace kanzi 
{

   // Null entropy encoder
   // Pass through that writes the data directly to the bitstream
   class NullEntropyEncoder : public EntropyEncoder 
   {
   private:
       OutputBitStream& _bitstream;

   public:
       NullEntropyEncoder(OutputBitStream& bitstream);

       ~NullEntropyEncoder() { dispose(); }

       int encode(byte arr[], uint blkptr, uint len);

       OutputBitStream& getBitStream() const { return _bitstream; };

       void dispose() {}
   };

   inline NullEntropyEncoder::NullEntropyEncoder(OutputBitStream& bitstream)
       : _bitstream(bitstream)
   {
   }

   inline int NullEntropyEncoder::encode(byte block[], uint blkptr, uint len)
   {
       const uint len8 = len & -8;
       const uint end8 = blkptr + len8;
       uint i = blkptr;

       try {
           while (i < end8) {
               uint64 val;
               val = ((uint64)(block[blkptr] & 0xFF)) << 56;
               val |= ((uint64)(block[blkptr + 1] & 0xFF) << 48);
               val |= ((uint64)(block[blkptr + 2] & 0xFF) << 40);
               val |= ((uint64)(block[blkptr + 3] & 0xFF) << 32);
               val |= ((uint64)(block[blkptr + 4] & 0xFF) << 24);
               val |= ((uint64)(block[blkptr + 5] & 0xFF) << 16);
               val |= ((uint64)(block[blkptr + 6] & 0xFF) << 8);
               val |= (uint64)(block[blkptr + 7] & 0xFF);

               if (_bitstream.writeBits(val, 64) != 64)
                   return i;

               i += 8;
           }

           while (i < blkptr + len) {
               _bitstream.writeBits(block[i], 8);
               i++;
           }
       }
       catch (BitStreamException e) {
           return i - blkptr;
       }

       return len;
   }

}
#endif
