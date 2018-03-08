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
#include "IllegalArgumentException.hpp"


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

       static int ten_log10(uint32 x) THROW;

       static int sin(int32 x);

       static int cos(int32 x);

       static int log2(uint32 x) THROW; // fast, integer rounded

       static int log2_1024(uint32 x) THROW; // slow, accurate

       static int sqrt(uint32 x);
       
       static void computeJobsPerTask(int jobsPerTask[], int jobs, int tasks);

   private:
       static const int INFINITE_VALUE;
       static const int PI_1024;
       static const int PI_1024_MULT2;
       static const int SMALL_RAD_ANGLE_1024; // arbitrarily set to 0.25 rad
       static const int CONST1; // 326 >> 12 === 1/(4*Math.PI)

       static const uint32 SQRT_THRESHOLD0;
       static const uint32 SQRT_THRESHOLD1;
       static const uint32 SQRT_THRESHOLD2;
       static const uint32 SQRT_THRESHOLD3;
       static const uint32 SQRT_THRESHOLD4;
       static const uint32 SQRT_THRESHOLD5;
       static const uint32 SQRT_THRESHOLD6;
       static const uint32 SQRT_THRESHOLD7;
       static const uint32 SQRT_THRESHOLD8;

       static const int* initStretch();
       static const int* initSquash();

       static int _log2(uint32 x);
   };


   // return p = 1/(1 + exp(-d)), d scaled by 8 bits, p scaled by 12 bits
   inline int Global::squash(int d)
   {
       if (d >= 2048)
           return 4095;

       if (d <= -2048)
           return 0;

       return SQUASH[d + 2047];
   }


   // Return 1024 * sin(1024*x) [x in radians]
   // Max error is less than 1.5%
   inline int Global::sin(int32 rad1024)
   {
       if ((rad1024 >= Global::PI_1024_MULT2) || (rad1024 <= -Global::PI_1024_MULT2))
           rad1024 %= Global::PI_1024_MULT2;

       // If x is small enough, return sin(x) === x
       if ((rad1024 < Global::SMALL_RAD_ANGLE_1024) && (-rad1024 < Global::SMALL_RAD_ANGLE_1024))
           return rad1024;

       const int x = (rad1024 + (rad1024 >> 31)) ^ (rad1024 >> 31); // abs(rad1024)

       if (x >= PI_1024)
           return -(((rad1024 >> 31) ^ Global::SIN_1024[((x - Global::PI_1024) * CONST1) >> 12]) - (rad1024 >> 31));

       return ((rad1024 >> 31) ^ Global::SIN_1024[(x * Global::CONST1) >> 12]) - (rad1024 >> 31);
   }


   // Return 1024 * cos(1024*x) [x in radians]
   // Max error is less than 1.5%
   inline int Global::cos(int32 rad1024)
   {
       if ((rad1024 >= Global::PI_1024_MULT2) || (rad1024 <= -Global::PI_1024_MULT2))
           rad1024 %= Global::PI_1024_MULT2;

       // If x is small enough, return cos(x) === 1 - (x*x)/2
       if ((rad1024 < Global::SMALL_RAD_ANGLE_1024) && (-rad1024 < Global::SMALL_RAD_ANGLE_1024))
           return 1024 - ((rad1024 * rad1024) >> 11);

       const int x = (rad1024 + (rad1024 >> 31)) ^ (rad1024 >> 31); // abs(rad1024)

       if (x >= Global::PI_1024)
           return -COS_1024[((x - Global::PI_1024) * Global::CONST1) >> 12];

       return COS_1024[(x * Global::CONST1) >> 12];
   }



   // Integer SQRT implementation based on algorithm at
   // http://guru.multimedia.cx/fast-integer-square-root/
   // Return 1024*sqrt(x) with a precision higher than 0.1%
   inline int Global::sqrt(uint32 x)
   {
       if (x <= 1)
           return x << 10;

       const int shift = (x < Global::SQRT_THRESHOLD5) ? ((x < Global::SQRT_THRESHOLD0) ? 16 : 10) : 0;
       x <<= shift; // scale up for better precision

       int val;

       if (x < Global::SQRT_THRESHOLD1) {
           if (x < Global::SQRT_THRESHOLD2) {
               val = Global::SQRT[(x + 3) >> 2] >> 3;
           }
           else {
               if (x < Global::SQRT_THRESHOLD3)
                   val = Global::SQRT[(x + 28) >> 6] >> 1;
               else
                   val = Global::SQRT[x >> 8];
           }
       }
       else {
           if (x < Global::SQRT_THRESHOLD4) {
               if (x < Global::SQRT_THRESHOLD5) {
                   val = Global::SQRT[x >> 12];
                   val = ((x / val) >> 3) + (val << 1);
               }
               else {
                   val = Global::SQRT[x >> 16];
                   val = ((x / val) >> 5) + (val << 3);
               }
           }
           else {
               if (x < Global::SQRT_THRESHOLD6) {
                   if (x < Global::SQRT_THRESHOLD7) {
                       val = Global::SQRT[x >> 18];
                       val = ((x / val) >> 6) + (val << 4);
                   }
                   else {
                       val = Global::SQRT[x >> 20];
                       val = ((x / val) >> 7) + (val << 5);
                   }
               }
               else {
                   if (x < Global::SQRT_THRESHOLD8) {
                       val = Global::SQRT[x >> 22];
                       val = ((x / val) >> 8) + (val << 6);
                   }
                   else {
                       val = Global::SQRT[x >> 24];
                       val = ((x / val) >> 9) + (val << 7);
                   }
               }
           }
       }

       // return 1024 * sqrt(x)
       return (val - ((x - (val * val)) >> 31)) << (10 - (shift >> 1));
   }

}
#endif
