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

#ifndef _TPAQPredictor_
#define _TPAQPredictor_

#include "AdaptiveProbMap.hpp"
#include "../Global.hpp"
#include "Predictor.hpp"


namespace kanzi 
{

   // Tangelo PAQ predictor
   // Derived from a modified version of Tangelo 2.4 (by Jan Ondrus).
   // PAQ8 is written by Matt Mahoney.
   // See http://encode.ru/threads/1738-TANGELO-new-compressor-(derived-from-PAQ8-FP8)

   //////////////////////////// Mixer /////////////////////////////
   // Mixer combines models using 4096 neural networks with 8 inputs.
   // It is used as follows:
   // m.update(y) trains the network where the expected output is the last bit.
   // m.addInput(stretch(p)) inputs prediction from one of N models.  The
   //     prediction should be positive to predict a 1 bit, negative for 0,
   //     nominally -2K to 2K.
   // m.setContext(cxt) selects cxt (0..4095) as one of M neural networks to use.
   // m.get() returns the (squashed) output prediction that the next bit is 1.
   //  The normal sequence per prediction is:
   //
   // - m.addInput(x) called N times with input x=(-2047..2047)
   // - m.setContext(cxt) called once with cxt=(0..M-1)
   // - m.get() called once to predict the next bit, returns 0..4095
   // - m.update(y) called once for actual bit y=(0..1).
   class TPAQMixer
   {
       friend class TPAQPredictor;

   public:
       static const int* DATA;

       TPAQMixer(int size);

       ~TPAQMixer();

       int get();

   private:
       int _ctx;
       int _idx;
       int _pr;
       int* _buffer;

       void update(int bit);

       void setContext(int ctx) { _ctx = ctx << 4; }

       void addInput(int pred);
   };

   class TPAQPredictor : public Predictor
   {
   public:
       TPAQPredictor();

       ~TPAQPredictor();

       void update(int bit);

       // Return the split value representing the probability of 1 in the [0..4095] range.
       int get() { return _pr; }

   private:
       static const int MAX_LENGTH = 160;
       static const int MIXER_SIZE = 0x1000;
       static const int HASH_SIZE = 8 * 1024 * 1024;
       static const int MASK0 = MIXER_SIZE - 1;
       static const int MASK1 = HASH_SIZE - 1;
       static const int MASK2 = 8 * HASH_SIZE - 1;
       static const int MASK3 = 32 * HASH_SIZE - 1;
       static const int C1 = 0xcc9e2d51;
       static const int C2 = 0x1b873593;
       static const int C3 = 0xe6546b64;
       static const int C4 = 0x85ebca6b;
       static const int C5 = 0xc2b2ae35;
       static const int HASH1 = 200002979;
       static const int HASH2 = 30005491;
       static const int HASH3 = 50004239;

       int _pr; // next predicted value (0-4095)
       int _c0; // bitwise context: last 0-7 bits with a leading 1 (1-255)
       uint32 _c4; // last 4 whole bytes, last is in low 8 bits
       int _bpos; // number of bits in c0 (0-7)
       int _pos;
       int _matchLen;
       int _matchPos;
       int _hash;
       AdaptiveProbMap _apm;
       TPAQMixer _mixer;
       byte* _buffer;
       int* _hashes; // hash table(context, buffer position)
       byte* _states; // hash table(context, prediction)
       int _cp[8]; // context pointers
       int _ctx[8]; // contexts
       int _ctxId;

       static int hash(int x, int y);

       void addContext(int cx);

       void addMatchContext();

       void findMatch();
   };

}
#endif