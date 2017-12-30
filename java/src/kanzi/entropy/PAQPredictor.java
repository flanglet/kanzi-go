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
package kanzi.entropy;

// http://code.google.com/p/dcs-bwt-compressor/(itself based on PAQ coders)

//// It was originally written by Matt Mahoney as
//// bbb.cpp - big block BWT compressor version 1, Aug. 31, 2006.
//// http://cs.fit.edu/~mmahoney/compression/bbb.cpp
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

public class PAQPredictor implements Predictor
{          
   ///////////////////////// state table ////////////////////////
   // STATE_TABLE[state,0] = next state if bit is 0, 0 <= state < 256
   // STATE_TABLE[state,1] = next state if bit is 1
   // STATE_TABLE[state,2] = number of zeros in bit history represented by state
   // STATE_TABLE[state,3] = number of ones represented
   // States represent a bit history within some context.
   // State 0 is the starting state (no bits seen).
   // States 1-30 represent all possible sequences of 1-4 bits.
   // States 31-252 represent a pair of counts, (n0,n1), the number
   //   of 0 and 1 bits respectively.  If n0+n1 < 16 then there are
   //   two states for each pair, depending on if a 0 or 1 was the last
   //   bit seen.
   // If n0 and n1 are too large, then there is no state to represent this
   // pair, so another state with about the same ratio of n0/n1 is substituted.
   // Also, when a bit is observed and the count of the opposite bit is large,
   // then part of this count is discarded to favor newer data over old.
   private static final int[] STATE_TABLE__ =
   {
         1,  2, 0, 0,   3,  5, 1, 0,   4,  6, 0, 1,   7, 10, 2, 0, // 0-3
         8, 12, 1, 1,   9, 13, 1, 1,  11, 14, 0, 2,  15, 19, 3, 0, // 4-7
        16, 23, 2, 1,  17, 24, 2, 1,  18, 25, 2, 1,  20, 27, 1, 2, // 8-11
        21, 28, 1, 2,  22, 29, 1, 2,  26, 30, 0, 3,  31, 33, 4, 0, // 12-15
        32, 35, 3, 1,  32, 35, 3, 1,  32, 35, 3, 1,  32, 35, 3, 1, // 16-19
        34, 37, 2, 2,  34, 37, 2, 2,  34, 37, 2, 2,  34, 37, 2, 2, // 20-23
        34, 37, 2, 2,  34, 37, 2, 2,  36, 39, 1, 3,  36, 39, 1, 3, // 24-27
        36, 39, 1, 3,  36, 39, 1, 3,  38, 40, 0, 4,  41, 43, 5, 0, // 28-31
        42, 45, 4, 1,  42, 45, 4, 1,  44, 47, 3, 2,  44, 47, 3, 2, // 32-35
        46, 49, 2, 3,  46, 49, 2, 3,  48, 51, 1, 4,  48, 51, 1, 4, // 36-39
        50, 52, 0, 5,  53, 43, 6, 0,  54, 57, 5, 1,  54, 57, 5, 1, // 40-43
        56, 59, 4, 2,  56, 59, 4, 2,  58, 61, 3, 3,  58, 61, 3, 3, // 44-47
        60, 63, 2, 4,  60, 63, 2, 4,  62, 65, 1, 5,  62, 65, 1, 5, // 48-51
        50, 66, 0, 6,  67, 55, 7, 0,  68, 57, 6, 1,  68, 57, 6, 1, // 52-55
        70, 73, 5, 2,  70, 73, 5, 2,  72, 75, 4, 3,  72, 75, 4, 3, // 56-59
        74, 77, 3, 4,  74, 77, 3, 4,  76, 79, 2, 5,  76, 79, 2, 5, // 60-63
        62, 81, 1, 6,  62, 81, 1, 6,  64, 82, 0, 7,  83, 69, 8, 0, // 64-67
        84, 71, 7, 1,  84, 71, 7, 1,  86, 73, 6, 2,  86, 73, 6, 2, // 68-71
        44, 59, 5, 3,  44, 59, 5, 3,  58, 61, 4, 4,  58, 61, 4, 4, // 72-75
        60, 49, 3, 5,  60, 49, 3, 5,  76, 89, 2, 6,  76, 89, 2, 6, // 76-79
        78, 91, 1, 7,  78, 91, 1, 7,  80, 92, 0, 8,  93, 69, 9, 0, // 80-83
        94, 87, 8, 1,  94, 87, 8, 1,  96, 45, 7, 2,  96, 45, 7, 2, // 84-87
        48, 99, 2, 7,  48, 99, 2, 7,  88,101, 1, 8,  88,101, 1, 8, // 88-91
        80,102, 0, 9, 103, 69,10, 0, 104, 87, 9, 1, 104, 87, 9, 1, // 92-95
       106, 57, 8, 2, 106, 57, 8, 2,  62,109, 2, 8,  62,109, 2, 8, // 96-99
        88,111, 1, 9,  88,111, 1, 9,  80,112, 0,10, 113, 85,11, 0, // 100-103
       114, 87,10, 1, 114, 87,10, 1, 116, 57, 9, 2, 116, 57, 9, 2, // 104-107
        62,119, 2, 9,  62,119, 2, 9,  88,121, 1,10,  88,121, 1,10, // 108-111
        90,122, 0,11, 123, 85,12, 0, 124, 97,11, 1, 124, 97,11, 1, // 112-115
       126, 57,10, 2, 126, 57,10, 2,  62,129, 2,10,  62,129, 2,10, // 116-119
        98,131, 1,11,  98,131, 1,11,  90,132, 0,12, 133, 85,13, 0, // 120-123
       134, 97,12, 1, 134, 97,12, 1, 136, 57,11, 2, 136, 57,11, 2, // 124-127
        62,139, 2,11,  62,139, 2,11,  98,141, 1,12,  98,141, 1,12, // 128-131
        90,142, 0,13, 143, 95,14, 0, 144, 97,13, 1, 144, 97,13, 1, // 132-135
        68, 57,12, 2,  68, 57,12, 2,  62, 81, 2,12,  62, 81, 2,12, // 136-139
        98,147, 1,13,  98,147, 1,13, 100,148, 0,14, 149, 95,15, 0, // 140-143
       150,107,14, 1, 150,107,14, 1, 108,151, 1,14, 108,151, 1,14, // 144-147
       100,152, 0,15, 153, 95,16, 0, 154,107,15, 1, 108,155, 1,15, // 148-151
       100,156, 0,16, 157, 95,17, 0, 158,107,16, 1, 108,159, 1,16, // 152-155
       100,160, 0,17, 161,105,18, 0, 162,107,17, 1, 108,163, 1,17, // 156-159
       110,164, 0,18, 165,105,19, 0, 166,117,18, 1, 118,167, 1,18, // 160-163
       110,168, 0,19, 169,105,20, 0, 170,117,19, 1, 118,171, 1,19, // 164-167
       110,172, 0,20, 173,105,21, 0, 174,117,20, 1, 118,175, 1,20, // 168-171
       110,176, 0,21, 177,105,22, 0, 178,117,21, 1, 118,179, 1,21, // 172-175
       110,180, 0,22, 181,115,23, 0, 182,117,22, 1, 118,183, 1,22, // 176-179
       120,184, 0,23, 185,115,24, 0, 186,127,23, 1, 128,187, 1,23, // 180-183
       120,188, 0,24, 189,115,25, 0, 190,127,24, 1, 128,191, 1,24, // 184-187
       120,192, 0,25, 193,115,26, 0, 194,127,25, 1, 128,195, 1,25, // 188-191
       120,196, 0,26, 197,115,27, 0, 198,127,26, 1, 128,199, 1,26, // 192-195
       120,200, 0,27, 201,115,28, 0, 202,127,27, 1, 128,203, 1,27, // 196-199
       120,204, 0,28, 205,115,29, 0, 206,127,28, 1, 128,207, 1,28, // 200-203
       120,208, 0,29, 209,125,30, 0, 210,127,29, 1, 128,211, 1,29, // 204-207
       130,212, 0,30, 213,125,31, 0, 214,137,30, 1, 138,215, 1,30, // 208-211
       130,216, 0,31, 217,125,32, 0, 218,137,31, 1, 138,219, 1,31, // 212-215
       130,220, 0,32, 221,125,33, 0, 222,137,32, 1, 138,223, 1,32, // 216-219
       130,224, 0,33, 225,125,34, 0, 226,137,33, 1, 138,227, 1,33, // 220-223
       130,228, 0,34, 229,125,35, 0, 230,137,34, 1, 138,231, 1,34, // 224-227
       130,232, 0,35, 233,125,36, 0, 234,137,35, 1, 138,235, 1,35, // 228-231
       130,236, 0,36, 237,125,37, 0, 238,137,36, 1, 138,239, 1,36, // 232-235
       130,240, 0,37, 241,125,38, 0, 242,137,37, 1, 138,243, 1,37, // 236-239
       130,244, 0,38, 245,135,39, 0, 246,137,38, 1, 138,247, 1,38, // 240-243
       140,248, 0,39, 249,135,40, 0, 250, 69,39, 1,  80,251, 1,39, // 244-247
       140,252, 0,40, 249,135,41, 0, 250, 69,40, 1,  80,251, 1,40, // 248-251
       140,252, 0,41,   0,  0, 0, 0,   0,  0, 0, 0,   0,  0, 0, 0  // 253-255 are reserved
   };

static int[] Probs = new int[] {

  2048,  1843,  1659,  1493,  1344,  1210,  1089,   980,   882,   794,
   715,   644,   580,   522,   470,   423,   381,   343,   309,   278,
   250,   225,   203,   183,   165,   149,   134,   121,   109,    98,
    88,    79,    71,    64,    58,    52,    47,    42,    38,    34,
    31,    28,    25,    23,    21,    19,    17,    15,    14,    13,
    12,    11,    10,     9,     8,     7,     6,     5,     5,     5,
     5,     5,     5,     5,
};

static int[] MPS = new int[] {

     1,     2,     3,     4,     5,     6,     7,     8,     9,    10,
    11,    12,    13,    14,    15,    16,    17,    18,    19,    20,
    21,    22,    23,    24,    25,    26,    27,    28,    29,    30,
    31,    32,    33,    34,    35,    36,    37,    38,    39,    40,
    41,    42,    43,    44,    45,    46,    47,    48,    49,    50,
    51,    52,    53,    54,    55,    56,    57,    58,    59,    60,
    61,    62,    63,    63,
};

static int[] LPS = new int[] {

     1,     0,     1,     1,     2,     3,     3,     4,     5,     5,
     6,     6,     7,     8,     8,     9,    10,    10,    11,    11,
    12,    13,    13,    14,    15,    15,    16,    16,    17,    18,
    18,    19,    20,    20,    21,    21,    22,    23,    23,    24,
    25,    25,    26,    26,    27,    28,    28,    29,    30,    30,
    31,    31,    32,    33,    33,    34,    35,    35,    36,    36,
    37,    38,    38,    39
};


   private static final byte[][] STATE_TABLE =
   {
      // Bit 0
      { 
            1,     3,  -113,     4,     5,     6,     7,     8,     9,    10,
           11,    12,    13,    14,    15,    16,    17,    18,    19,    20,
           21,    22,    23,    24,    25,    26,    27,    28,    29,    30,
           31,    32,    33,    34,    35,    36,    37,    38,    39,    40,
           41,    42,    43,    44,    45,    46,    47,    48,    49,    50,
           51,    52,    47,    54,    55,    56,    57,    58,    59,    60,
           61,    62,    63,    64,    65,    66,    67,    68,    69,     6,
           71,    71,    71,    61,    75,    56,    77,    78,    77,    80,
           81,    82,    83,    84,    85,    86,    87,    88,    77,    90,
           91,    92,    80,    94,    95,    96,    97,    98,    99,    90,
          101,    94,   103,   101,   102,   104,   107,   104,   105,   108,
          111,   112,   113,   114,   115,   116,    92,   118,    94,   103,
          119,   122,   123,    94,   113,   126,   113,  -128,  -127,   114,
         -125,  -124,   112,  -122,   111,  -122,   110,  -122,  -122,  -128,
         -128,  -114,  -113,   115,   113,  -114,  -128,  -108,  -107,    79,
         -108,  -114,  -108,  -106,  -101,  -107,   -99,  -107,   -97,  -107,
         -125,   101,    98,   115,   114,    91,    79,    58,     1,   -86,
         -127,  -128,   110,   -82,  -128,   -80,  -127,   -82,   -77,   -82,
          -80,  -115,   -99,   -77,   -71,   -99,   -69,   -68,   -88,  -105,
          -65,   -64,   -68,   -69,   -84,   -81,   -86,  -104,   -71,   -86,
          -80,   -86,   -53,  -108,   -71,   -53,   -71,   -64,   -47,   -68,
          -45,   -64,   -43,   -42,   -68,   -40,   -88,    84,    54,    54,
          -35,    54,    55,    85,    69,    63,    56,    86,    58,   -26,
          -25,    57,   -27,    56,   -32,    54,    54,    66,    58,    54,
           61,    57,   -34,    78,    85,    82,     0,     0,     0,     0,
            0,     0,     0,     0,     0,     0
      },
      // Bit 1
      {
            2,   -93,   -87,   -93,   -91,    89,   -11,   -39,   -11,   -11,
          -23,   -12,   -29,    74,   -35,   -35,   -38,   -30,   -13,   -38,
          -18,   -14,    74,   -18,   -15,   -16,   -17,   -32,   -31,   -35,
          -24,    72,   -32,   -28,   -33,   -31,   -18,    73,   -89,    76,
          -19,   -22,   -25,    72,    31,    63,   -31,   -19,   -20,   -21,
           53,   -22,    53,   -22,   -27,   -37,   -27,   -23,   -24,   -28,
          -30,    72,    74,   -34,    75,   -36,   -89,    57,   -38,    70,
          -88,    72,    73,    74,   -39,    76,   -89,    79,    79,   -90,
          -94,   -94,   -94,   -94,   -91,    89,    89,   -91,    89,   -94,
           93,    93,    93,   -95,   100,    93,    93,    93,    93,    93,
          -95,   102,   120,   104,   105,   106,   108,   106,   109,   110,
          -96,  -122,   108,   108,   126,   117,   117,   121,   119,   120,
          107,   124,   117,   117,   125,   127,   124,  -117,  -126,   124,
         -123,   109,   110,  -121,   110,  -120,  -119,  -118,   127,  -116,
         -115,  -111,  -112,   124,   125,  -110,  -109,  -105,   125,  -106,
          127,  -104,  -103,  -102,  -100,  -117,   -98,  -117,  -100,  -117,
         -126,   117,   -93,   -92,  -115,   -93,  -109,     2,     2,   -57,
          -85,   -84,   -83,   -79,   -81,   -85,   -85,   -78,   -76,   -84,
          -75,   -74,   -73,   -72,   -70,   -78,   -67,   -75,   -75,   -66,
          -63,   -74,   -74,   -62,   -61,   -60,   -59,   -58,   -87,   -56,
          -55,   -54,   -52,   -76,   -51,   -50,   -49,   -48,   -46,   -62,
          -44,   -72,   -41,   -63,   -72,   -48,   -63,   -93,   -37,   -88,
           94,   -39,   -33,   -32,   -31,    76,   -29,   -39,   -27,   -37,
           79,    86,   -91,   -39,   -42,   -31,   -40,   -40,   -22,    75,
          -42,   -19,    74,    74,   -93,   -39,     0,     0,     0,     0,
            0,     0,     0,     0,     0,     0        
      }
   };
   
          
   // Removed apm11, apm12 and apm5 from original
   private int pr;                   // next predicted value (0-4095)
   private int c0;                   // bitwise context: last 0-7 bits with a leading 1 (1-255)
   private int c4;                   // last 4 whole bytes, last is in low 8 bits
   private int bpos;                 // bit in c0 (0-7)
   private final int[] states;       // context -> state
   private final StateMap sm;        // state -> pr
   private int run;                  // count of consecutive identical bytes (0-65535)
   private int runCtx;               // (0-3) if run is 0, 1, 2-3, 4+
   private final LogisticAdaptiveProbMap apm2;
   private final LogisticAdaptiveProbMap apm3;
   private final LogisticAdaptiveProbMap apm4;
   int lps;
   int ctx;
   
   
   public PAQPredictor()
   {
     this.pr = 2048;
     this.c0 = 1;
     this.states = new int[256];
     this.apm2 = new LogisticAdaptiveProbMap(1024, 6);
     this.apm3 = new LogisticAdaptiveProbMap(1024, 7);
     this.apm4 = new LogisticAdaptiveProbMap(65536, 8);
     this.sm = new StateMap();
     this.bpos = 8;
   } 

   
   // Update the probability model
   @Override
   public void update(int bit)
   {
     this.states[this.c0] = STATE_TABLE__[(this.states[this.c0]<<2)+bit];
//this.states[this.c0] = STATE_TABLE[bit][this.states[this.c0]] & 0xFF;
     // update context
     this.c0 = (this.c0 << 1) | bit;

     if (this.c0 > 255)
     {
        if ((this.c0 & 0xFF) == (this.c4 & 0xFF))
        {
           if ((this.run < 4) && (this.run != 2))
              this.runCtx += 256;

           this.run++;
        }
        else
        {
           this.run = 0;
           this.runCtx = 0;
        }

        this.bpos = 8;
        this.c4 = (this.c4 << 8) | (this.c0 & 0xFF);
        this.c0 = 1;
     }
     
     int c1d = ((((this.c4 & 0xFF) | 256) >> this.bpos) == this.c0) ? 2 : 0;
     this.bpos--;
     c1d += ((this.c4 >> this.bpos) & 1);
     
     // Get prediction from state map
     int p = this.sm.get(bit, this.states[this.c0]);
     
//     if (bit == lps)
//     {
//        ctx = LPS[ctx];
//     }
//     else
//     {
//        ctx = MPS[ctx];
//     }
//     p = LPS[ctx];
//     
//     if (lps != 1)
//        p = 4095 - p;
//     
//     if (ctx == 0)
//        lps = 1 - lps;

     // SSE (Secondary Symbol Estimation)     
     p = this.apm2.get(bit, p, this.c0 | (c1d<<8));
     p = (3*this.apm3.get(bit, p, (this.c4&0xFF) | this.runCtx) + p + 2) >> 2;    
     p = (3*this.apm4.get(bit, p, this.c0 | (this.c4&0xFF00)) + p + 2) >> 2;
     this.pr = p + ((p-2048) >>> 31);   
   }


   // Return the split value representing the probability of 1 in the [0..4095] range.
   @Override
   public int get()
   {
      return this.pr;
   }

   private static final int[] PROBS = new int[] // ((n1+1) << 16) / (n0+n1+3)
   {
      21845, 16384, 32768, 13107, 26214, 26214, 39321, 10922, 21845, 21845,
      21845, 32768, 32768, 32768, 43690,  9362, 18724, 18724, 18724, 18724,
      28086, 28086, 28086, 28086, 28086, 28086, 37449, 37449, 37449, 37449,
      46811,  8192, 16384, 16384, 24576, 24576, 32768, 32768, 40960, 40960,
      49152,  7281, 14563, 14563, 21845, 21845, 29127, 29127, 36408, 36408,
      43690, 43690, 50972,  6553, 13107, 13107, 19660, 19660, 26214, 26214,
      32768, 32768, 39321, 39321, 45875, 45875, 52428,  5957, 11915, 11915,
      17873, 17873, 23831, 23831, 29789, 29789, 35746, 35746, 41704, 41704,
      47662, 47662, 53620,  5461, 10922, 10922, 16384, 16384, 43690, 43690,
      49152, 49152, 54613,  5041, 10082, 10082, 15123, 15123, 45371, 45371,
      50412, 50412, 55453,  4681,  9362,  9362, 14043, 14043, 46811, 46811,
      51492, 51492, 56173,  4369,  8738,  8738, 13107, 13107, 48059, 48059,
      52428, 52428, 56797,  4096,  8192,  8192, 12288, 12288, 49152, 49152,
      53248, 53248, 57344,  3855,  7710,  7710, 11565, 11565, 50115, 50115,
      53970, 53970, 57825,  3640,  7281,  7281, 54613, 54613, 58254,  3449,
       6898, 55188, 58637,  3276,  6553, 55705, 58982,  3120,  6241, 56173,
      59294,  2978,  5957, 56599, 59578,  2849,  5698, 56987, 59837,  2730,
       5461, 57344, 60074,  2621,  5242, 57671, 60293,  2520,  5041, 57974,
      60494,  2427,  4854, 58254, 60681,  2340,  4681, 58514, 60854,  2259,
       4519, 58756, 61016,  2184,  4369, 58982, 61166,  2114,  4228, 59193,
      61307,  2048,  4096, 59392, 61440,  1985,  3971, 59578, 61564,  1927,
       3855, 59753, 61680,  1872,  3744, 59918, 61791,  1820,  3640, 60074,
      61895,  1771,  3542, 60222, 61993,  1724,  3449, 60362, 62086,  1680,
       3360, 60494, 62175,  1638,  3276, 60620, 62259,  1598,  3196, 60740,
      62339,  1560,  3120, 60854, 62415,  1524,  3048, 60963, 62487,  1489,
       2978, 61067, 62557, 21845, 21845, 21845
   };
   
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
   static class StateMap
   {
//      private static final int[] PROBS = initStateMapData();
//
//      private static int[] initStateMapData()
//      {
//         int[] array = new int[256];
//
//         for (int i=0; i<256; i++)
//         {
//            int n0 = STATE_TABLE[0][i] & 0xFF;
//            int n1 = STATE_TABLE[1][i] & 0xFF;
//            array[i] = ((n1+1) << 16) / (n0+n1+3);
//         }
//    
//         return array;
//      }


      private int ctx;
      private final int[] data;


      StateMap()
      {
         this.data = new int[256];
         System.arraycopy(PROBS, 0, this.data, 0, 256);
      }


      int get(int bit, int nctx)
      {
         this.data[this.ctx] += (((bit<<16) - this.data[this.ctx] + 256) >> 9);
         this.ctx = nctx;
         return this.data[nctx] >>> 4;
      }
   }
}
