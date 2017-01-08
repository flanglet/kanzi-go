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

#ifndef _DST4_
#define _DST4_

#include "../Transform.hpp"

namespace kanzi 
{

   // Implementation of Discrete Sine Transform of dimension 4
   class DST4 : public Transform<int> 
   {
   public:
       DST4();

       ~DST4() {}

       bool forward(SliceArray<int>& source, SliceArray<int>& destination, int length=16);

       bool inverse(SliceArray<int>& source, SliceArray<int>& destination, int length=16);

   private:
       // Weights
       static const int W29 = 29;
       static const int W74 = 74;
       static const int W55 = 55;

       static const int MAX_VAL = 1 << 16;
       static const int MIN_VAL = -(MAX_VAL + 1);

       int _fShift;
       int _iShift;
       int _data[16];

       void computeForward(int input[], int output[], const int shift);

       void computeInverse(int input[], int output[], const int shift);
   };

}
#endif
