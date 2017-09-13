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

#ifndef _AdaptiveProbMap_
#define _AdaptiveProbMap_

#include "../Global.hpp"

namespace kanzi 
{

   /////////////////////////////////////////////////////////////////
   // APM maps a probability and a context into a new probability
   // that bit y will next be 1.  After each guess it updates
   // its state to improve future guesses.  Methods:
   //
   // APM a(N) creates with N contexts, uses 66*N bytes memory.
   // a.get(y, pr, cx) returned adjusted probability in context cx (0 to
   //   N-1).  rate determines the learning rate (smaller = faster, default 8).
   //////////////////////////////////////////////////////////////////
   template <int RATE>
   class AdaptiveProbMap 
   {
   public:
       AdaptiveProbMap<RATE>(int n);

       ~AdaptiveProbMap<RATE>() { delete[] _data; }

       int get(int bit, int pr, int ctx);

   private:
       int _index; // last p, context
       int* _data; // [NbCtx][33]:  p, context -> p
   };

   template <int RATE>
   inline AdaptiveProbMap<RATE>::AdaptiveProbMap(int n)
   {
       _data = new int[33 * n];
       _index = 0;

       for (int i = 0, k = 0; i < n; i++, k += 33) {
           for (int j = 0; j < 33; j++)
               _data[k + j] = (i == 0) ? Global::squash((j - 16) << 7) << 4 : _data[j];
       }
   }

   template <int RATE>
   inline int AdaptiveProbMap<RATE>::get(int bit, int pr, int ctx)
   {
       // Update probability based on error and learning rate
       const int g = (bit << 16) + (bit << RATE) - (bit << 1);
       _data[_index] += ((g - _data[_index]) >> RATE);
       _data[_index + 1] += ((g - _data[_index + 1]) >> RATE);
       pr = Global::STRETCH[pr];

       // Find new context
       _index = ((pr + 2048) >> 7) + (ctx << 5) + ctx;

       // Return interpolated probabibility
       const int w = pr & 127;
       return (_data[_index] * (128 - w) + _data[_index + 1] * w) >> 11;
   }

}
#endif
