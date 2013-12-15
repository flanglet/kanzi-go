/*
Copyright 2011-2013 Frederic Langlet
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

package kanzi.function;

import kanzi.ByteFunction;


public class FunctionFactory
{
   public static final byte BLOCK_TYPE   = 66; // 'B'
   public static final byte RLT_TYPE     = 82; // 'R'
   public static final byte SNAPPY_TYPE  = 83; // 'S'
   public static final byte ZLT_TYPE     = 90; // 'Z'
   public static final byte LZ4_TYPE     = 76; // 'L'
   public static final byte NONE_TYPE    = 78; // 'N'


   public ByteFunction newFunction(int size, byte type)
   {
      switch (type)
      {
         case BLOCK_TYPE:
            return new BlockCodec(size);
         case SNAPPY_TYPE:
            return new SnappyCodec(size);
         case LZ4_TYPE:
            return new LZ4Codec(size);
         case RLT_TYPE:
            return new RLT(size);
         case ZLT_TYPE:
            return new ZLT(size);
         case NONE_TYPE:
            return new NullFunction(size);
         default:
            throw new IllegalArgumentException("Unknown transform type: " + (char) type);
      }
   }


   public String getName(byte type)
   {
      switch (type)
      {
         case BLOCK_TYPE:
            return "BLOCK";
         case SNAPPY_TYPE:
            return "SNAPPY";
         case LZ4_TYPE:
            return "LZ4";
         case RLT_TYPE:
            return "RLT";
         case ZLT_TYPE:
            return "ZLT";
         case NONE_TYPE:
            return "NONE";
         default:
            throw new IllegalArgumentException("Unknown transform type: " + (char) type);
      }
   }
}
