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

#ifndef _EntropyCodecFactory_
#define _EntropyCodecFactory_

#include <algorithm>
#include "ANSRangeDecoder.hpp"
#include "ANSRangeEncoder.hpp"
#include "BinaryEntropyDecoder.hpp"
#include "BinaryEntropyEncoder.hpp"
#include "ExpGolombDecoder.hpp"
#include "ExpGolombEncoder.hpp"
#include "HuffmanDecoder.hpp"
#include "HuffmanEncoder.hpp"
#include "NullEntropyDecoder.hpp"
#include "NullEntropyEncoder.hpp"
#include "RangeDecoder.hpp"
#include "RangeEncoder.hpp"
#include "RiceGolombDecoder.hpp"
#include "RiceGolombEncoder.hpp"
#include "CMPredictor.hpp"
#include "FPAQPredictor.hpp"
#include "PAQPredictor.hpp"
#include "TPAQPredictor.hpp"
#include "../IllegalArgumentException.hpp"

namespace kanzi {

   class EntropyCodecFactory {
   public:
       static const byte NONE_TYPE = 0; // No compression
       static const byte HUFFMAN_TYPE = 1; // Huffman
       static const byte FPAQ_TYPE = 2; // Fast PAQ (order 0)
       static const byte PAQ_TYPE = 3; // PAQ (stripped from many models for speed)
       static const byte RANGE_TYPE = 4; // Range
       static const byte ANS0_TYPE = 5; // Asymmetric Numerical System order 0
       static const byte CM_TYPE = 6; // Context Model
       static const byte TPAQ_TYPE = 7; // Tangelo PAQ
       static const byte ANS1_TYPE = 8; // Asymmetric Numerical System order 1
       static const byte TPAQX_TYPE = 9; // Tangelo PAQ Extra

       static EntropyDecoder* newDecoder(InputBitStream& ibs, map<string, string>& ctx, short entropyType) THROW;

       static EntropyEncoder* newEncoder(OutputBitStream& obs, map<string, string>& ctx, short entropyType) THROW;

       static const char* getName(short entropyType) THROW;

       static short getType(const char* name) THROW;
   };

   inline EntropyDecoder* EntropyCodecFactory::newDecoder(InputBitStream& ibs, map<string, string>& ctx, short entropyType) THROW
   {
       switch (entropyType) {
       // Each block is decoded separately
       // Rebuild the entropy decoder to reset block statistics
       case HUFFMAN_TYPE:
           return new HuffmanDecoder(ibs);

       case ANS0_TYPE:
           return new ANSRangeDecoder(ibs, 0);

       case ANS1_TYPE:
           return new ANSRangeDecoder(ibs, 1);

       case RANGE_TYPE:
           return new RangeDecoder(ibs);

       case PAQ_TYPE:
           return new BinaryEntropyDecoder(ibs, new PAQPredictor());

       case FPAQ_TYPE:
           return new BinaryEntropyDecoder(ibs, new FPAQPredictor());

       case CM_TYPE:
           return new BinaryEntropyDecoder(ibs, new CMPredictor());

       case TPAQ_TYPE: 
           return new BinaryEntropyDecoder(ibs, new TPAQPredictor(&ctx));
       
       case TPAQX_TYPE: 
           ctx["extra"] = "true";
           return new BinaryEntropyDecoder(ibs, new TPAQPredictor(&ctx));

       case NONE_TYPE:
           return new NullEntropyDecoder(ibs);

       default:
           string msg = "Unknown entropy codec type: ";
           msg += char(entropyType);
           throw IllegalArgumentException(msg);
       }
   }

   inline EntropyEncoder* EntropyCodecFactory::newEncoder(OutputBitStream& obs, map<string, string>& ctx, short entropyType) THROW
   {
       switch (entropyType) {
       case HUFFMAN_TYPE:
           return new HuffmanEncoder(obs);

       case ANS0_TYPE:
           return new ANSRangeEncoder(obs, 0);

       case ANS1_TYPE:
           return new ANSRangeEncoder(obs, 1);

       case RANGE_TYPE:
           return new RangeEncoder(obs);

       case PAQ_TYPE:
           return new BinaryEntropyEncoder(obs, new PAQPredictor());

       case FPAQ_TYPE:
           return new BinaryEntropyEncoder(obs, new FPAQPredictor());

       case CM_TYPE:
           return new BinaryEntropyEncoder(obs, new CMPredictor());

       case TPAQ_TYPE: 
           return new BinaryEntropyEncoder(obs, new TPAQPredictor(&ctx));
       
       case TPAQX_TYPE: 
           ctx["extra"] = "true";
           return new BinaryEntropyEncoder(obs, new TPAQPredictor(&ctx));

       case NONE_TYPE:
           return new NullEntropyEncoder(obs);

       default:
           string msg = "Unknown entropy codec type: ";
           msg += char(entropyType);
           throw IllegalArgumentException(msg);
       }
   }

   inline const char* EntropyCodecFactory::getName(short entropyType) THROW
   {
       switch (entropyType) {
       case HUFFMAN_TYPE:
           return "HUFFMAN";

       case ANS0_TYPE:
           return "ANS0";

       case ANS1_TYPE:
           return "ANS1";

       case RANGE_TYPE:
           return "RANGE";

       case PAQ_TYPE:
           return "PAQ";

       case FPAQ_TYPE:
           return "FPAQ";

       case CM_TYPE:
           return "CM";

       case TPAQ_TYPE:
           return "TPAQ";

       case TPAQX_TYPE:
           return "TPAQX";

       case NONE_TYPE:
           return "NONE";

       default:
           string msg = "Unknown entropy codec type: ";
           msg += char(entropyType);
           throw IllegalArgumentException(msg);
       }
   }

   inline short EntropyCodecFactory::getType(const char* str) THROW
   {
       string name = str;
       transform(name.begin(), name.end(), name.begin(), ::toupper);

       if (name == "HUFFMAN")
           return HUFFMAN_TYPE;

       if (name == "ANS0")
           return ANS0_TYPE;

       if (name == "ANS1")
           return ANS1_TYPE;

       if (name == "FPAQ")
           return FPAQ_TYPE;

       if (name == "PAQ")
           return PAQ_TYPE;

       if (name == "RANGE")
           return RANGE_TYPE;

       if (name == "CM")
           return CM_TYPE;

       if (name == "TPAQ")
           return TPAQ_TYPE;

       if (name == "NONE")
           return NONE_TYPE;

       throw IllegalArgumentException("Unsupported entropy codec type: " + name);
   }
}
#endif
