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

#ifndef _NullEntropyDecoder_
#define _NullEntropyDecoder_


namespace kanzi 
{

   // Null entropy decoder
   // Pass through that writes the data directly to the bitstream
   class NullEntropyDecoder : public EntropyDecoder
   {
      private:

         InputBitStream& _bitstream;

      public :

        NullEntropyDecoder(InputBitStream& bitstream);

	     ~NullEntropyDecoder() { }

        int decode(byte block[], uint blkptr, uint len);

	     InputBitStream& getBitStream() const { return _bitstream; };

	     void dispose() { }
   };


   inline NullEntropyDecoder::NullEntropyDecoder(InputBitStream& bitstream) : _bitstream(bitstream)
   {
   }

   inline int NullEntropyDecoder::decode(byte block[], uint blkptr, uint len)
   {
          const uint len8 = len & -8;
          const uint end8 = blkptr + len8;
          uint i = blkptr;

          while (i < end8)
          {
		      uint64 val = _bitstream.readBits(64);
		      block[blkptr]   = (byte) (val >> 56);
		      block[blkptr+1] = (byte) (val >> 48);
		      block[blkptr+2] = (byte) (val >> 40);
		      block[blkptr+3] = (byte) (val >> 32);
		      block[blkptr+4] = (byte) (val >> 24);
		      block[blkptr+5] = (byte) (val >> 16);
		      block[blkptr+6] = (byte) (val >> 8);
		      block[blkptr+7] = (byte)  val;          
		      i += 8;
          }

          while (i < blkptr + len)
             block[i++] = (byte) _bitstream.readBits(8);

          return len;
   }

}
#endif
