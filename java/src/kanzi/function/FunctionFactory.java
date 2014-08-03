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
   public static final byte ZRLT_TYPE    = 90; // 'Z'
   public static final byte LZ4_TYPE     = 76; // 'L'
   public static final byte NONE_TYPE    = 78; // 'N'
   public static final byte BWT_TYPE     = 87; // 'W'


   public byte getType(String name)
   {
      switch (name.toUpperCase())
      {
         case "BLOCK":
            return BLOCK_TYPE; // BWT+GST+ZRLT
         case "SNAPPY":
            return SNAPPY_TYPE;
         case "LZ4":
            return LZ4_TYPE;
         case "RLT":
            return RLT_TYPE;
         case "ZRLT":
            return ZRLT_TYPE;
         case "BWT":
            return BWT_TYPE; // raw BWT
         case "NONE":
            return NONE_TYPE;
         default:
            throw new IllegalArgumentException("Unknown transform type: " + name);
      }
   }


   public ByteFunction newFunction(int size, byte type)
   {
      switch (type)
      {
         case BLOCK_TYPE:
            return new BlockCodec(BlockCodec.MODE_MTF, size); // BWT+GST+ZRLT
         case SNAPPY_TYPE:
            return new SnappyCodec(size);
         case LZ4_TYPE:
            return new LZ4Codec(size);
         case RLT_TYPE:
            return new RLT(size);
         case ZRLT_TYPE:
            return new ZRLT(size);
         case NONE_TYPE:
            return new NullFunction(size);
         case BWT_TYPE:
            return new BlockCodec(BlockCodec.MODE_RAW_BWT, size); // raw BWT
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
         case ZRLT_TYPE:
            return "ZRLT";
         case BWT_TYPE:
            return "BWT";
         case NONE_TYPE:
            return "NONE";
         default:
            throw new IllegalArgumentException("Unknown transform type: " + (char) type);
      }
   }
}
