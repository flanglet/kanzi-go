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
import kanzi.transform.BWT;
import kanzi.transform.BWTS;


public class FunctionFactory
{
   // Transform: 4 lsb
   public static final byte NULL_TRANSFORM_TYPE = 0; 
   public static final byte BWT_TYPE            = 1; 
   public static final byte BWTS_TYPE           = 2; 
   public static final byte LZ4_TYPE            = 3; 
   public static final byte SNAPPY_TYPE         = 4; 
   public static final byte RLT_TYPE            = 5; 
 
   // GST: 3 msb
 

   public byte getType(String name)
   {
      String args = "";
      name = name.toUpperCase();
      
      if (name.startsWith("BWT"))
      {
         int idx = name.indexOf('+');

         if (idx >= 0)
         {
            args = name.substring(idx+1);
            name = name.substring(0, idx);
         }
      }
      
      if (name.equalsIgnoreCase("SNAPPY"))
            return SNAPPY_TYPE;
      
      if (name.equalsIgnoreCase("LZ4"))
            return LZ4_TYPE;
      
      if (name.equalsIgnoreCase("RLT"))
            return RLT_TYPE;
      
      if (name.equalsIgnoreCase("BWT")) 
            return (byte) ((getGSTType(args) << 4) | BWT_TYPE);
      
      if (name.equalsIgnoreCase("BWTS"))
            return (byte) ((getGSTType(args) << 4) | BWTS_TYPE);
      
      if (name.equalsIgnoreCase("NONE"))
            return NULL_TRANSFORM_TYPE;
 
      throw new IllegalArgumentException("Unknown transform type: " + name);
   }


   private static byte getGSTType(String args)
   {
      if (args == null)
         throw new IllegalArgumentException("Missing GST type");
      
      if ((args.length() == 0) || (args.equalsIgnoreCase("NONE")))
         return BWTBlockCodec.MODE_RAW;
      
      if (args.equalsIgnoreCase("MTF"))
         return BWTBlockCodec.MODE_MTF;
      
      if (args.equalsIgnoreCase("RANK"))        
         return BWTBlockCodec.MODE_RANK;
      
      if (args.equalsIgnoreCase("TIMESTAMP"))
         return BWTBlockCodec.MODE_TIMESTAMP;
      
       throw new IllegalArgumentException("Unknown GST type: " + args);
   }
   
   
   public ByteFunction newFunction(int size, byte type)
   {
      switch (type & 0x0F)
      {
         case SNAPPY_TYPE:
            return new SnappyCodec(size);
            
         case LZ4_TYPE:
            return new LZ4Codec(size);
            
         case RLT_TYPE:
            return new RLT(size);
            
         case NULL_TRANSFORM_TYPE:
            return new NullFunction(size);
            
         case BWT_TYPE:
            return new BWTBlockCodec(new BWT(), type >>> 4, size); 
            
         case BWTS_TYPE:
            return new BWTBlockCodec(new BWTS(), type >>> 4, size); 
          
         default:
            throw new IllegalArgumentException("Unknown transform type: " + (char) type);
      }
   }

      
   private static String getGSTName(int type)
   {
       switch (type)
      {
         case BWTBlockCodec.MODE_MTF:
            return "MTF";
            
         case BWTBlockCodec.MODE_RANK:
            return "RANK";
            
         case BWTBlockCodec.MODE_TIMESTAMP:
            return "TIMESTAMP";
            
         case BWTBlockCodec.MODE_RAW:
            return "";
            
         default:
            throw new IllegalArgumentException("Unknown GST type: " + type);
      }
   }

   
   public String getName(byte type)
   {
      switch (type & 0x0F)
      {
         case LZ4_TYPE:
            return "LZ4";
            
         case BWT_TYPE:
         case BWTS_TYPE:
            String gstName = getGSTName(type >>> 4);
            
            if ((type & 0x0F) == BWT_TYPE)
               return (gstName.length() == 0) ? "BWT" : "BWT+" + gstName;         
               
            return (gstName.length() == 0) ? "BWTS" : "BWTS+" + gstName;
            
         case SNAPPY_TYPE:
            return "SNAPPY";
            
         case RLT_TYPE:
            return "RLT";
            
         case NULL_TRANSFORM_TYPE:
            return "NONE";
            
         default:
            throw new IllegalArgumentException("Unknown transform type: " + (char) type);
      }
   }
}
