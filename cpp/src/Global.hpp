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

namespace kanzi {

   class Global {
   public:
       static const int INV_EXP[]; //  1<<16* 1/(1 + exp(-alpha*x)) with alpha = 0.52631
       static const int COS_1024[]; // array with 256 elements: 1024*Math.cos(x) x in [0..Math.PI[
       static const int SIN_1024[]; // array with 256 elements: 1024*Math.sin(x) x in [0..Math.PI[
       static const int TEN_LOG10_100[]; // array with 10 elements: 10 * (4096*Math.log10(x))
       static const int LOG2_4096[]; // array with 256 elements: 4096*Math.log2(x)
       static const int LOG2[]; // array with 256 elements: int(Math.log2(x-1))
       static const int SQRT[]; 

       // Inverse of squash. d = ln(p/(1-p)), d scaled by 8 bits, p by 12 bits.
       // d has range -2047 to 2047 representing -8 to 8.  p has range 0 to 4095.
       static const int* STRETCH;

       static const int* SQUASH;

       static int squash(int d);

       static int64 readLong64(const byte* p);

       static int32 readInt32(const byte* p);

       static int16 readInt16(const byte* p);

       static int ten_log10(int32 x) THROW;

       static int sin(int32 x);

       static int cos(int32 x);

       static int log2(int32 x) THROW;

       static int log2_1024(int32 x) THROW;

       static int len32(int32 x);

       static int sqrt(int32 x) THROW;

   private:
       static const int INFINITE_VALUE;
       static const int PI_1024;
       static const int PI_1024_MULT2;
       static const int SMALL_RAD_ANGLE_1024; // arbitrarily set to 0.25 rad
       static const int CONST1; // 326 >> 12 === 1/(4*Math.PI)

       static const int SQRT_THRESHOLD0;
       static const int SQRT_THRESHOLD1;
       static const int SQRT_THRESHOLD2;
       static const int SQRT_THRESHOLD3;
       static const int SQRT_THRESHOLD4;
       static const int SQRT_THRESHOLD5;
       static const int SQRT_THRESHOLD6;
       static const int SQRT_THRESHOLD7;
       static const int SQRT_THRESHOLD8;

       static const int* initStretch();
       static const int* initSquash();
   };

   inline int64 Global::readLong64(const byte* p)
   {
       int64 val;
       memcpy(&val, p, 8);
       return val;
   }

   inline int32 Global::readInt32(const byte* p)
   {
       int32 val;
       memcpy(&val, p, 4);
       return val;
   }

   inline int16 Global::readInt16(const byte* p)
   {
       int16 val;
       memcpy(&val, p, 2);
       return val;
   }

   // return p = 1/(1 + exp(-d)), d scaled by 8 bits, p scaled by 12 bits
   inline int Global::squash(int d)
   {
       if (d >= 2048)
           return 4095;

       if (d <= -2048)
           return 0;

       return SQUASH[d+2047];
   }
}
#endif
