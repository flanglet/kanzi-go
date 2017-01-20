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

#ifndef _DCT16_
#define _DCT16_

#include "../Transform.hpp"

namespace kanzi 
{

   // Implementation of Discrete Cosine Transform of dimension 16
   class DCT16 : public Transform<int>
   {
   public:
       DCT16();

       ~DCT16() {}

       bool forward(SliceArray<int>& source, SliceArray<int>& destination, int length=256);

       bool inverse(SliceArray<int>& source, SliceArray<int>& destination, int length=256);

   private:
       // Weights
       static const int W0 = 64;
       static const int W1 = 64;
       static const int W16 = 90;
       static const int W17 = 87;
       static const int W18 = 80;
       static const int W19 = 70;
       static const int W20 = 57;
       static const int W21 = 43;
       static const int W22 = 25;
       static const int W23 = 9;
       static const int W32 = 89;
       static const int W33 = 75;
       static const int W34 = 50;
       static const int W35 = 18;
       static const int W48 = 87;
       static const int W49 = 57;
       static const int W50 = 9;
       static const int W51 = -43;
       static const int W52 = -80;
       static const int W53 = -90;
       static const int W54 = -70;
       static const int W55 = -25;
       static const int W64 = 83;
       static const int W65 = 36;
       static const int W80 = 80;
       static const int W81 = 9;
       static const int W82 = -70;
       static const int W83 = -87;
       static const int W84 = -25;
       static const int W85 = 57;
       static const int W86 = 90;
       static const int W87 = 43;
       static const int W96 = 75;
       static const int W97 = -18;
       static const int W98 = -89;
       static const int W99 = -50;
       static const int W112 = 70;
       static const int W113 = -43;
       static const int W114 = -87;
       static const int W115 = 9;
       static const int W116 = 90;
       static const int W117 = 25;
       static const int W118 = -80;
       static const int W119 = -57;
       static const int W128 = 64;
       static const int W129 = -64;
       static const int W144 = 57;
       static const int W145 = -80;
       static const int W146 = -25;
       static const int W147 = 90;
       static const int W148 = -9;
       static const int W149 = -87;
       static const int W150 = 43;
       static const int W151 = 70;
       static const int W160 = 50;
       static const int W161 = -89;
       static const int W162 = 18;
       static const int W163 = 75;
       static const int W176 = 43;
       static const int W177 = -90;
       static const int W178 = 57;
       static const int W179 = 25;
       static const int W180 = -87;
       static const int W181 = 70;
       static const int W182 = 9;
       static const int W183 = -80;
       static const int W192 = 36;
       static const int W193 = -83;
       static const int W208 = 25;
       static const int W209 = -70;
       static const int W210 = 90;
       static const int W211 = -80;
       static const int W212 = 43;
       static const int W213 = 9;
       static const int W214 = -57;
       static const int W215 = 87;
       static const int W224 = 18;
       static const int W225 = -50;
       static const int W226 = 75;
       static const int W227 = -89;
       static const int W240 = 9;
       static const int W241 = -25;
       static const int W242 = 43;
       static const int W243 = -57;
       static const int W244 = 70;
       static const int W245 = -80;
       static const int W246 = 87;
       static const int W247 = -90;

       static const int MAX_VAL = 1 << 16;
       static const int MIN_VAL = -(MAX_VAL + 1);

       int _fShift;
       int _iShift;
       int _data[256];

       void computeForward(int input[], int output[], const int shift);

       void computeInverse(int input[], int output[], const int shift);
   };

}
#endif
