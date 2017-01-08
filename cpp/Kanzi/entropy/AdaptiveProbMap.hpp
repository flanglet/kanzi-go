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
   class AdaptiveProbMap 
   {
   public:
       AdaptiveProbMap(int n, int rate=7);

       ~AdaptiveProbMap() { delete[] _data; }

       int get(int bit, int pr, int ctx);

   private:
       int _index; // last p, context
       int _rate; // update rate
       int* _data; // [NbCtx][33]:  p, context -> p
   };

   inline int AdaptiveProbMap::get(int bit, int pr, int ctx)
   {
       // Update probability based on error and learning rate
       const int g = (bit << 16) + (bit << _rate) - (bit << 1);
       _data[_index] += ((g - _data[_index]) >> _rate);
       _data[_index + 1] += ((g - _data[_index + 1]) >> _rate);
       pr = Global::STRETCH[pr];

       // Find new context
       _index = ((pr + 2048) >> 7) + (ctx << 5) + ctx;

       // Return interpolated probabibility
       const int w = pr & 127;
       return (_data[_index] * (128 - w) + _data[_index + 1] * w) >> 11;
   }

}
#endif
