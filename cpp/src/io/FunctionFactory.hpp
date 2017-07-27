#ifndef _FunctionFactory_
#define _FunctionFactory_

#include <iostream>
#include <fstream>
#include <algorithm>
#include <cstring>
#include "../types.hpp"
#include "../function/TransformSequence.hpp"
#include "../IllegalArgumentException.hpp"
#include "../function/BWTBlockCodec.hpp"
#include "../transform/BWT.hpp"
#include "../transform/BWTS.hpp"
#include "../function/SnappyCodec.hpp"
#include "../function/LZ4Codec.hpp"
#include "../transform/MTFT.hpp"
#include "../function/ZRLT.hpp"
#include "../function/RLT.hpp"
#include "../transform/SBRT.hpp"
#include "../function/NullFunction.hpp"
#include "../function/TextCodec.hpp"

namespace kanzi {

template <class T>
   class FunctionFactory {
   public:
       // Up to 15 transforms can be declared (4 bit index)
       static const short NULL_TRANSFORM_TYPE = 0; // copy
       static const short BWT_TYPE = 1; // Burrows Wheeler
       static const short BWTS_TYPE = 2; // Burrows Wheeler Scott
       static const short LZ4_TYPE = 3; // LZ4
       static const short SNAPPY_TYPE = 4; // Snappy
       static const short RLT_TYPE = 5; // Run Length
       static const short ZRLT_TYPE = 6; // Zero Run Length
       static const short MTFT_TYPE = 7; // Move To Front
       static const short RANK_TYPE = 8; // Rank
       static const short TIMESTAMP_TYPE = 9; // TimeStamp
       static const short TEXTCODEC_TYPE = 10; // Text codec

       FunctionFactory() {}

       ~FunctionFactory() {}

       short getType(const char* name) const THROW;

       short getTypeToken(const char* name) const THROW;

       string getName(short functionType) const THROW;

       static TransformSequence<T>* newFunction(int size, short functionType) THROW;

   private:
       static Transform<T>* newFunctionToken(int size, short functionType) THROW;

       static const char* getNameToken(int functionType) THROW;
   };

   // The returned type contains 4 (nibble based) transform values
   template <class T>
   short FunctionFactory<T>::getType(const char* cname) const THROW
   {
       string name(cname);

       if (name.find("+") == string::npos)
           return (short)(getTypeToken(name.c_str()) << 12);

       char buf[64];
       int length = (name.length() < 63) ? int(name.length()) : 63;
       memcpy(buf, name.c_str(), length);
       buf[length] = 0;
       const char* token = strtok(buf, "+");

       if (token == NULL) {
           stringstream ss;
           ss << "Unknown transform type: " << name;
           throw IllegalArgumentException(ss.str());
       }

       int res = 0;
       int shift = 12;
       int n = 0;

       while (token != NULL) {
           short typeTk = getTypeToken(token);
           n++;

           if (n > 4) {
               stringstream ss;
               ss << "Only 4 transforms allowed: " << name;
               throw IllegalArgumentException(ss.str());
           }

           // Skip null transform
           if (typeTk != NULL_TRANSFORM_TYPE) {
               res |= (typeTk << shift);
               shift -= 4;
           }

           token = strtok(nullptr, "+");
       }

       return (short)res;
   }

   template <class T>
   short FunctionFactory<T>::getTypeToken(const char* cname) const THROW
   {
       string name(cname);
       transform(name.begin(), name.end(), name.begin(), ::toupper);

       if (name.compare("BWT") == 0)
           return BWT_TYPE;

       if (name.compare("BWTS") == 0)
           return BWTS_TYPE;

       if (name.compare("SNAPPY") == 0)
           return SNAPPY_TYPE;

       if (name.compare("LZ4") == 0)
           return LZ4_TYPE;

       if (name.compare("MTFT") == 0)
           return MTFT_TYPE;

       if (name.compare("ZRLT") == 0)
           return ZRLT_TYPE;

       if (name.compare("RLT") == 0)
           return RLT_TYPE;

       if (name.compare("RANK") == 0)
           return RANK_TYPE;

       if (name.compare("TIMESTAMP") == 0)
           return TIMESTAMP_TYPE;

       if (name.compare("TEXT") == 0)
           return TEXTCODEC_TYPE;

       if (name.compare("NONE") == 0)
           return NULL_TRANSFORM_TYPE;

       stringstream ss;
       ss << "Unknown transform type: " << name;
       throw IllegalArgumentException(ss.str());
   }

   template <class T>
   TransformSequence<T>* FunctionFactory<T>::newFunction(int size, short functionType) THROW
   {
       int nbtr = 0;

       // Several transforms
       for (int i = 0; i < 4; i++) {
           if (((functionType >> (12 - 4 * i)) & 0x0F) != NULL_TRANSFORM_TYPE)
               nbtr++;
       }

       // Only null transforms ? Keep first.
       if (nbtr == 0)
           nbtr = 1;

       Transform<T>* transforms[4];
       nbtr = 0;

       for (int i = 0; i < 4; i++) {
           transforms[i] = nullptr;
           int t = (functionType >> (12 - 4 * i)) & 0x0F;

           if ((t != NULL_TRANSFORM_TYPE) || (i == 0))
               transforms[nbtr++] = newFunctionToken(size, short(t));
       }

       return new TransformSequence<T>(transforms, true);
   }

   template <class T>
   Transform<T>* FunctionFactory<T>::newFunctionToken(int size, short functionType) THROW
   {
       switch (functionType & 0x0F) {
       case SNAPPY_TYPE:
           return new SnappyCodec();

       case LZ4_TYPE:
           return new LZ4Codec();

       case BWT_TYPE:
           return new BWTBlockCodec();

       case BWTS_TYPE:
           return new BWTS();

       case MTFT_TYPE:
           return new MTFT();

       case ZRLT_TYPE:
           return new ZRLT();

       case RLT_TYPE:
           return new RLT();

       case RANK_TYPE:
           return new SBRT(SBRT::MODE_RANK);

       case TEXTCODEC_TYPE:
           {
              // Select an appropriate initial dictionary size
              int dictSize = 1<<12;
            
              for (int i=14; i<=24; i+=2)
              {
                 if (size >= 1<<i)
                    dictSize <<= 1;
              }
 
              return new TextCodec(dictSize); 
           }

       case TIMESTAMP_TYPE:
           return new SBRT(SBRT::MODE_TIMESTAMP);

       case NULL_TRANSFORM_TYPE:
           return new NullFunction<byte>();

       default:
           stringstream ss;
           ss << "Unknown transform type: " << functionType;
           throw IllegalArgumentException(ss.str());
       }
   }

   template <class T>
   string FunctionFactory<T>::getName(short functionType) const THROW
   {
       stringstream ss;

       for (int i = 0; i < 4; i++) {
           int t = functionType >> (12 - 4 * i);

           if ((t & 0x0F) == NULL_TRANSFORM_TYPE)
               continue;

           string name = getNameToken(t);

           if (ss.str().length() != 0)
               ss << "+";

           ss << name;
       }

       if (ss.str().length() == 0) {
           ss << getNameToken(NULL_TRANSFORM_TYPE);
       }

       return ss.str();
   }

   template <class T>
   const char* FunctionFactory<T>::getNameToken(int functionType) THROW
   {
       switch (functionType & 0x0F) {
       case LZ4_TYPE:
           return "LZ4";

       case BWT_TYPE:
           return "BWT";

       case BWTS_TYPE:
           return "BWTS";

       case SNAPPY_TYPE:
           return "SNAPPY";

       case MTFT_TYPE:
           return "MTFT";

       case ZRLT_TYPE:
           return "ZRLT";

       case RLT_TYPE:
           return "RLT";

       case RANK_TYPE:
           return "RANK";

       case TIMESTAMP_TYPE:
           return "TIMESTAMP";

       case TEXTCODEC_TYPE:
           return "TEXT";

       case NULL_TRANSFORM_TYPE:
           return "NONE";

       default:
           stringstream ss;
           ss << "Unknown transform type: " << functionType;
           throw IllegalArgumentException(ss.str());
       }
   }
}
#endif