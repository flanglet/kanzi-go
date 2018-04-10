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

#ifndef _PAQPredictor_
#define _PAQPredictor_

#include "../Global.hpp"
#include "../Predictor.hpp"
#include "AdaptiveProbMap.hpp"

namespace kanzi 
{

   // This class is a port from the code of the dcs-bwt-compressor project
   // http://code.google.com/p/dcs-bwt-compressor/(itself based on PAQ coders)

   // It was originally written by Matt Mahoney as
   // bbb.cpp - big block BWT compressor version 1, Aug. 31, 2006.
   // http://cs.fit.edu/~mmahoney/compression/bbb.cpp

   //////////////////////////////////////////////////////////////////
   // A StateMap maps a nonstationary counter state to a probability.
   // After each mapping, the mapping is adjusted to improve future
   // predictions.  Methods:
   //
   //  get(y, cx) converts state cx (0-255) to a probability (0-4095),
   //  and trains by updating the previous prediction with y (0-1).
   //
   // Counter state -> probability * 256
   //////////////////////////////////////////////////////////////////
   class StateMap 
   {
   public:
       static const int* DATA;

       StateMap();

       ~StateMap() {}

       int get(int bit, int nctx);

   private:
       int _ctx;
       int _data[256];

       static const int* initStateMapData();
   };

   ////
   //// ENTROPY CODING
   ////
   //// BWT data is best coded with an order 0 model.  The transformed text tends
   //// to have long runs of identical bytes (e.g. "nnbaaa").  The BWT data is
   //// modeled with a modified PAQ with just one context (no mixing) followed
   //// by a 5 stage SSE (APM) and bitwise arithmetic coding.  Modeling typically
   //// takes about as much time as sorting and unsorting in slow mode.
   //// The model uses about 5 MB memory.
   //// [ Now reduced to about 256KB of memory. ]
   ////
   //// The order 0 model consists of a mapping:
   ////
   ////             order 1, 2, 3 contexts ----------+
   ////                                              V
   ////  order 0 context -> bit history -> p -> APM chain -> arithmetic coder
   ////                  t1             sm
   ////
   //// Bits are coded one at a time.  The arithmetic coder maintains a range
   //// [lo, hi), initially [0, 1) and repeatedly subdivides the range in proportion
   //// to p(0), p(1), the next bit probabilites predicted by the model.  The final
   //// output is the intest base 256 number x such that lo <= x < hi.  As the
   //// leading bytes of x become known, they are output.  To decompress, the model
   //// predictions are repeated as during compression, then the actual bit is
   //// determined by which half of the subrange contains x.
   ////
   //// The model inputs a bytewise order 0 context consisting of the last 0 to 7
   //// bits of the current byte, plus the number of bits.  There are a total of
   //// 255 possible bitwise contexts.  For each context, a table (t1) maintains
   //// an 8 bit state representing the history of 0 and 1 bits previously seen.
   //// This history is mapped by another table (a StateMap sm) to a probability,
   //// p, that the next bit will be 1. This table is adaptive: after each
   //// prediction, the mapping (state -> p) is adjusted to improve the last
   //// prediction.
   ////
   //// The output of the StateMap is passed through a series of 6 more adaptive
   //// tables, (Adaptive Probability Maps, or APM) each of which maps a context
   //// and the input probability to an output probability.  The input probability
   //// is interpolated between 33 bins on a nonlinear scale with smaller bins
   //// near 0 and 1.  After each prediction, the corresponding table entries
   //// on both sides of p are adjusted to improve the last prediction.
   ////  The APM chain is like this:
   ////
   ////      + A11 ->+            +--->---+ +--->---+
   ////      |       |            |       | |       |
   ////  p ->+       +-> A2 -> A3 +-> A4 -+-+-> A5 -+-> Encoder
   ////      |       |
   ////      + A12 ->+
   ////
   //// [ The APM chain has been modified into:
   ////
   ////  p --> A2 -> A3 --> A4 --> Encoder
   ////
   //// ]
   ////
   //// A11 and A12 both take c0 (the preceding bits of the current byte) as
   //// additional context, but one is fast adapting and the other is slow
   //// adapting.  Their outputs are averaged.
   ////
   //// A2 is an order 1 context (previous byte and current partial byte).
   //// [ A2 has been modified so that it uses only two bits of information
   //// from the previous byte: what is the bit in the current bit position
   //// and whether the preceding bits are same or different from c0. ]
   ////
   //// A3 takes the previous (but not current) byte as context, plus 2 bits
   //// that depend on the current run length (0, 1, 2-3, or 4+), the number
   //// of times the last byte was repeated.
   //// [ A3 now only takes the two bits on run length. ]
   ////
   //// A4 takes the current byte and the low 5 bits of the second byte back.
   //// The output is averaged with 3/4 weight to the A3 output with 1/4 weight.
   //// [ A4 has been moved after A5, it takes only the current byte (not the
   //// 5 additional bits), and the averaging weights are 1/2 and 1/2. ]
   ////
   //// A5 takes a 14 bit hash of an order 3 context (last 3 bytes plus
   //// current partial byte) and is averaged with 1/2 weight to the A4 output.
   //// [ A5 takes now 11 bit hash of an order 4 context. ]
   ////
   //// The StateMap, state table, APM, Encoder, and associated code (Array,
   //// squash(), stretch()) are taken from PAQ8 with minor non-functional
   //// changes (e.g. removing global context).

   class PAQPredictor : public Predictor 
   {
   public:
       PAQPredictor();

       ~PAQPredictor(){};

       void update(int bit);

       // Return the split value representing the probability of 1 in the [0..4095] range.
       int get() { return _pr; }

   private:
       int _pr; // next predicted value (0-4095)
       int32 _c0; // bitwise context: last 0-7 bits with a leading 1 (1-255)
       int32 _c4; // last 4 whole bytes, last is in low 8 bits
       int _bpos; // bit in c0 (0-7)
       short _states[256]; // context -> state
       StateMap _sm; // state -> pr
       int _run; // count of consecutive identical bytes (0-65535)
       int _runCtx; // (0-3) if run is 0, 1, 2-3, 4+
       LogisticAdaptiveProbMap<6> _apm2;
       LogisticAdaptiveProbMap<7> _apm3;
       LogisticAdaptiveProbMap<8> _apm4;
   };

}
#endif