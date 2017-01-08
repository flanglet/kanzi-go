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

#ifndef _SBRT_
#define _SBRT_

#include "../Transform.hpp"

namespace kanzi 
{
   // Sort by Rank Transform is a family of transforms typically used after
   // a BWT to reduce the variance of the data prior to entropy coding.
   // SBR(alpha) is defined by sbr(x, alpha) = (1-alpha)*(t-w1(x,t)) + alpha*(t-w2(x,t))
   // where x is an item in the data list, t is the current access time and wk(x,t) is
   // the k-th access time to x at time t (with 0 <= alpha <= 1).
   // See [Two new families of list update algorihtms] by Frank Schulz for details.
   // SBR(0)= Move to Front Transform
   // SBR(1)= Time Stamp Transform
   // This code implements SBR(0), SBR(1/2) and SBR(1). Code derived from openBWT
   class SBRT : public Transform<byte>
   {
   public:
       static const int MODE_MTF = 1; // alpha = 0
       static const int MODE_RANK = 2; // alpha = 1/2
       static const int MODE_TIMESTAMP = 3; // alpha = 1

       SBRT(int mode);

       ~SBRT() {}

       bool forward(SliceArray<byte>& input, SliceArray<byte>& output, int length);

       bool inverse(SliceArray<byte>& input, SliceArray<byte>& output, int length);

   private:

       int _mode;
       int _prev[256];
       int _curr[256];
       int _symbols[256];
       int _ranks[256];

       void computeForward(int* pSrc, int* pDst, int shift);

       void computeInverse(int* pSrc, int* pDst, int shift);
   };

}
#endif
