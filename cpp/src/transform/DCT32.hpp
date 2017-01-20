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

#ifndef _DCT32_
#define _DCT32_

#include "../Transform.hpp"

namespace kanzi 
{

   // Implementation of Discrete Cosine Transform of dimension 32
   class DCT32 : public Transform<int>
   {
   public:
       DCT32();

       ~DCT32() {}

       bool forward(SliceArray<int>& source, SliceArray<int>& destination, int length=1024);

       bool inverse(SliceArray<int>& source, SliceArray<int>& destination, int length=1024);

   private:
       // Weights
       static const int W[1024];

       static const int MAX_VAL = 1 << 16;
       static const int MIN_VAL = -(MAX_VAL + 1);

       int _fShift;
       int _iShift;
       int _data[1024];

       void computeForward(int input[], int output[], const int shift);

       void computeInverse(int input[], int output[], const int shift);
   };

}
#endif
