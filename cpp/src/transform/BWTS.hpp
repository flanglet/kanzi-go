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

#ifndef _BWTS_
#define _BWTS_

#include "../Transform.hpp"
#include "DivSufSort.hpp"

namespace kanzi 
{

   // Bijective version of the Burrows-Wheeler Transform
   // The main advantage over the regular BWT is that there is no need for a primary
   // index (hence the bijectivity). BWTS is about 10% slower than BWT.
   // Forward transform based on the code at https://code.google.com/p/mk-bwts/
   // by Neal Burns and DivSufSort (port of libDivSufSort by Yuta Mori)

   class BWTS : public Transform<byte> {

   private:
       static const int MAX_BLOCK_SIZE = 1024 * 1024 * 1024; // 1 GB (30 bits)

       int* _buffer1;
       int* _buffer2;
       int _bufferSize;
       int _buckets[256];
       DivSufSort _saAlgo;

       int moveLyndonWordHead(int sa[], int isa[], byte data[], int count, int start, int size, int rank);

   public:
       BWTS()
       {
           _buffer1 = new int[0];
           _buffer2 = new int[0];
           _bufferSize = 0;
           memset(_buckets, 0, sizeof(int) * 256);
       }

       ~BWTS() 
       { 
          delete[] _buffer1; 
          delete[] _buffer2; 
       }

       bool forward(SliceArray<byte>& input, SliceArray<byte>& output, int length);

       bool inverse(SliceArray<byte>& input, SliceArray<byte>& output, int length);

       static int maxBlockSize() { return MAX_BLOCK_SIZE; }
   };

}
#endif