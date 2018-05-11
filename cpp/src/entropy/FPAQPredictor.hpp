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

#ifndef _FPAQPredictor_
#define _FPAQPredictor_

#include "../Predictor.hpp"
#include "../types.hpp"

namespace kanzi
{

   // Derived from fpaq0r by Matt Mahoney & Alexander Ratushnyak.
   // See http://mattmahoney.net/dc/#fpaq0.
   // Simple (and fast) adaptive order 0 entropy coder predictor
   class FPAQPredictor : public Predictor
   {
   private:
       static const int PSCALE = 16 * 4096;

       uint16 _probs[256]; // probability of bit=1
       int _ctxIdx; // previous bits

   public:
       FPAQPredictor();

       ~FPAQPredictor() {}

       void update(int bit);

       int get() { return int(_probs[_ctxIdx] >> 4); }
   };

}
#endif