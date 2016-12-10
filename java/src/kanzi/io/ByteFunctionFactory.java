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

package kanzi.io;

import kanzi.ByteTransform;
import kanzi.function.BWTBlockCodec;
import kanzi.function.ByteTransformSequence;
import kanzi.function.LZ4Codec;
import kanzi.function.NullFunction;
import kanzi.function.RLT;
import kanzi.function.SnappyCodec;
import kanzi.function.ZRLT;
import kanzi.transform.BWTS;
import kanzi.transform.MTFT;
import kanzi.transform.SBRT;


public class ByteFunctionFactory
{
   // Up to 15 transforms can be declared (4 bit index)
   public static final short NULL_TRANSFORM_TYPE = 0;  // copy
   public static final short BWT_TYPE            = 1;  // Burrows Wheeler
   public static final short BWTS_TYPE           = 2;  // Burrows Wheeler Scott
   public static final short LZ4_TYPE            = 3;  // LZ4
   public static final short SNAPPY_TYPE         = 4;  // Snappy
   public static final short RLT_TYPE            = 5;  // Run Length
   public static final short ZRLT_TYPE           = 6;  // Zero Run Length
   public static final short MTFT_TYPE           = 7;  // Move To Front
   public static final short RANK_TYPE           = 8;  // Rank
   public static final short TIMESTAMP_TYPE      = 9;  // TimeStamp
 

   // The returned type contains 4 (nibble based) transform values
   public short getType(String name)
   {
      if (name.indexOf('+') <  0)
         return (short) (this.getTypeToken(name) << 12);
      
      String[] tokens = name.split("\\+");
      
      if (tokens.length == 0)
         throw new IllegalArgumentException("Unknown transform type: " + name);

      if (tokens.length > 4)
         throw new IllegalArgumentException("Only 4 transforms allowed: " + name);

      int res = 0;
      int shift = 12;
      
      for (String token: tokens)
      {
         final int typeTk = this.getTypeToken(token);
         
         // Skip null transform
         if (typeTk != NULL_TRANSFORM_TYPE)
         {
            res |= (typeTk << shift);
            shift -= 4;
         }
      }
      
      return (short) res;
   }
   
   
   private short getTypeToken(String name)
   {
      // Strings in switch not supported in JDK 6
      name = name.toUpperCase();
      
      switch (name)
      {
         case "BWT":
            return BWT_TYPE;

         case "BWTS":
            return BWTS_TYPE;

         case "SNAPPY":
            return SNAPPY_TYPE;

         case "LZ4":
            return LZ4_TYPE;

         case "MTFT":
            return MTFT_TYPE;

         case "ZRLT":
            return ZRLT_TYPE;

         case "RLT":
            return RLT_TYPE;

         case "RANK":
            return RANK_TYPE;

         case "TIMESTAMP":
            return TIMESTAMP_TYPE;

         case "NONE":
            return NULL_TRANSFORM_TYPE;

         default:
            throw new IllegalArgumentException("Unknown transform type: " + name);
      }
   }
   
   
   public ByteTransformSequence newFunction(int size, short functionType)
   {      
      int nbtr = 0;
      
      // Several transforms
      for (int i=0; i<4; i++)
      {
          if (((functionType >>> (12-4*i)) & 0x0F) != NULL_TRANSFORM_TYPE)
             nbtr++;
      }
    
      // Only null transforms ? Keep first.
      if (nbtr == 0)
         nbtr = 1;
      
      ByteTransform[] transforms = new ByteTransform[nbtr];
      nbtr = 0;
      
      for (int i=0; i<transforms.length; i++)
      {
          int t = (functionType >>> (12-4*i)) & 0x0F;
          
          if ((t != NULL_TRANSFORM_TYPE) || (i == 0))
             transforms[nbtr++] = newFunctionToken(size, (short) t);
      }
    
      return new ByteTransformSequence(transforms);
   }
   
   
   private static ByteTransform newFunctionToken(int size, short functionType)
   {
      switch (functionType & 0x0F)
      {
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
            return new SBRT(SBRT.MODE_RANK);
            
         case TIMESTAMP_TYPE:
            return new SBRT(SBRT.MODE_TIMESTAMP);
            
         case NULL_TRANSFORM_TYPE:
            return new NullFunction();
            
         default:
            throw new IllegalArgumentException("Unknown transform type: " + functionType);
      }
   }

   
   public String getName(short functionType)
   {              
       StringBuilder sb = new StringBuilder();

       for (int i=0; i<4; i++)
       {
          int t = functionType >>> (12-4*i);
          
          if ((t & 0x0F) == NULL_TRANSFORM_TYPE)
             continue;
          
          String name = getNameToken(t);
          
          if (sb.length() != 0)
             sb.append('+');
             
         sb.append(name);
       }
       
       if (sb.length() == 0)
          sb.append(getNameToken(NULL_TRANSFORM_TYPE));
       
       return sb.toString();
   }
   
   
   private static String getNameToken(int functionType)
   {
      switch (functionType & 0x0F)
      {
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
            
         case NULL_TRANSFORM_TYPE:
            return "NONE";
            
         default:
            throw new IllegalArgumentException("Unknown transform type: " + functionType);
      }
   }
}
