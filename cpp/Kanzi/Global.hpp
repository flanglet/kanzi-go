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

#ifndef _Kanzi_
#define _Kanzi_

#include <cstring>
#include "types.hpp"
#include "util.hpp"

namespace kanzi 
{

   class Global
   {
   public:
       //  1<<16* 1/(1 + exp(-alpha*x)) with alpha = 0.52631
       static const int INV_EXP[];

       // Inverse of squash. d = ln(p/(1-p)), d scaled by 8 bits, p by 12 bits.
       // d has range -2047 to 2047 representing -8 to 8.  p has range 0 to 4095.
       static const int* STRETCH;

       static int squash(int d);

       static int readInt32(const byte* p);

       static int readUInt32(const byte* p);

   private:
       static const int* initStretch();
   };

   inline int Global::readInt32(const byte* p)
   {
      int val; 
      memcpy(&val, p, 4); 
      return val;
   }

   inline int Global::readUInt32(const byte* p)
   {
      uint val; 
      memcpy(&val, p, 4); 
      return val;
   }

   // return p = 1/(1 + exp(-d)), d scaled by 8 bits, p scaled by 12 bits
   inline int Global::squash(int d)
   {
       if (d > 2047)
           return 4095;

       if (d < -2047)
           return 0;

       int w = d & 127;
       d = (d >> 7) + 16;
       return (INV_EXP[d] * (128 - w) + INV_EXP[d + 1] * w) >> 11;
   }

}
#endif
