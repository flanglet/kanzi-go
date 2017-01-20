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

#ifndef _DCT8_
#define _DCT8_

#include "../Transform.hpp"

namespace kanzi 
{

   // Implementation of Discrete Cosine Transform of dimension 8
   class DCT8 : public Transform<int> 
   {
   public:
       DCT8();

       ~DCT8() {}

       bool forward(SliceArray<int>& source, SliceArray<int>& destination, int length=64);

       bool inverse(SliceArray<int>& source, SliceArray<int>& destination, int length=64);

   private:
       // Weights
       static const int W0 = 64;
       static const int W1 = 64;
       static const int W8 = 89;
       static const int W9 = 75;
       static const int W10 = 50;
       static const int W11 = 18;
       static const int W16 = 83;
       static const int W17 = 36;
       static const int W24 = 75;
       static const int W25 = -18;
       static const int W26 = -89;
       static const int W27 = -50;
       static const int W32 = 64;
       static const int W33 = -64;
       static const int W40 = 50;
       static const int W41 = -89;
       static const int W42 = 18;
       static const int W43 = 75;
       static const int W48 = 36;
       static const int W49 = -83;
       static const int W56 = 18;
       static const int W57 = -50;
       static const int W58 = 75;
       static const int W59 = -89;

       static const int MAX_VAL = 1 << 16;
       static const int MIN_VAL = -(MAX_VAL + 1);

       int _fShift;
       int _iShift;
       int _data[64];

       void computeForward(int input[], int output[], const int shift);

       void computeInverse(int input[], int output[], const int shift);
   };

}
#endif
