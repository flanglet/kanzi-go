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

package kanzi.entropy;

import kanzi.EntropyDecoder;
import kanzi.EntropyEncoder;
import kanzi.InputBitStream;
import kanzi.OutputBitStream;


public class EntropyCodecFactory 
{
   public static final byte NONE_TYPE    = 0;
   public static final byte HUFFMAN_TYPE = 1;
   public static final byte FPAQ_TYPE    = 2;
   public static final byte PAQ_TYPE     = 3;
   public static final byte RANGE_TYPE   = 4;
   public static final byte ANS_TYPE     = 5;
   
   
   public EntropyDecoder newDecoder(InputBitStream ibs, byte entropyType)
   {
      if (ibs == null)
         throw new NullPointerException("Invalid null input bitstream parameter");

      switch (entropyType)
      {
         // Each block is decoded separately
         // Rebuild the entropy decoder to reset block statistics
         case HUFFMAN_TYPE:
            return new HuffmanDecoder(ibs);
         case ANS_TYPE:
            return new ANSRangeDecoder(ibs);
         case RANGE_TYPE:
            return new RangeDecoder(ibs);
         case PAQ_TYPE:
            return new BinaryEntropyDecoder(ibs, new PAQPredictor());
         case FPAQ_TYPE:
            return new BinaryEntropyDecoder(ibs, new FPAQPredictor());
         case NONE_TYPE:
            return new NullEntropyDecoder(ibs);
         default:
            throw new IllegalArgumentException("Unsupported entropy codec type: " + (char) entropyType);
      }
   } 
   
   
   public EntropyEncoder newEncoder(OutputBitStream obs, byte entropyType)
   {
      if (obs == null)
         throw new NullPointerException("Invalid null output bitstream parameter");

      switch (entropyType)
      {
         case HUFFMAN_TYPE:
            return new HuffmanEncoder(obs);
         case ANS_TYPE:
            return new ANSRangeEncoder(obs);
         case RANGE_TYPE:
            return new RangeEncoder(obs);
         case PAQ_TYPE:
            return new BinaryEntropyEncoder(obs, new PAQPredictor());
         case FPAQ_TYPE:
            return new BinaryEntropyEncoder(obs, new FPAQPredictor());
         case NONE_TYPE:
            return new NullEntropyEncoder(obs);
         default :
            throw new IllegalArgumentException("Unknown entropy codec type: " + (char) entropyType);
      }
   }
   
   
   public String getName(byte entropyType)
   {
      switch (entropyType)
      {
         case HUFFMAN_TYPE:
            return "HUFFMAN";
         case ANS_TYPE:
            return "ANS";
         case RANGE_TYPE:
            return "RANGE";
         case PAQ_TYPE:
            return "PAQ";
         case FPAQ_TYPE:
            return "FPAQ";
         case NONE_TYPE:
            return "NONE";
         default :
            throw new IllegalArgumentException("Unknown entropy codec type: " + (char) entropyType);
      }
   }
   
  
   public byte getType(String name)
   {
      if (name.equalsIgnoreCase("HUFFMAN"))
         return HUFFMAN_TYPE; 
      
      if (name.equalsIgnoreCase("ANS"))
         return ANS_TYPE; 
      
      if (name.equalsIgnoreCase("FPAQ"))
         return FPAQ_TYPE;
      
      if (name.equalsIgnoreCase("PAQ"))
         return PAQ_TYPE;
      
      if (name.equalsIgnoreCase("RANGE"))
         return RANGE_TYPE; 
      
      if (name.equalsIgnoreCase("NONE"))
         return NONE_TYPE;

      throw new IllegalArgumentException("Unsupported entropy codec type: " + name); 
   } 
   
}

