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

#ifndef _CMPredictor_
#define _CMPredictor_

#include "../Predictor.hpp"

namespace kanzi 
{

   // Context model predictor based on BCM by Ilya Muravyov.
   // See https://github.com/encode84/bcm
   class CMPredictor : public Predictor 
   {
   private:
       static const int FAST_RATE = 2;
       static const int MEDIUM_RATE = 4;
       static const int SLOW_RATE = 6;

       int _c1;
       int _c2;
       int _ctx;
       int _run;
       int _idx;
       int _runMask;
       int _counter1[256][257];
       int _counter2[512][17];
       int* _pc1;
       int* _pc2;

   public:
       CMPredictor();

       ~CMPredictor(){};

       void update(int bit);

       int get();
   };

}
#endif