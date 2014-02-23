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
   public static final byte HUFFMAN_TYPE = 72;
   public static final byte NONE_TYPE    = 78;
   public static final byte FPAQ_TYPE    = 70;
   public static final byte PAQ_TYPE     = 80;
   public static final byte RANGE_TYPE   = 82;
   
   
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
      switch (name.toUpperCase())
      {
         case "HUFFMAN":
            return HUFFMAN_TYPE; 
         case "FPAQ":
            return FPAQ_TYPE;
         case "PAQ":
            return PAQ_TYPE;
         case "RANGE":
            return RANGE_TYPE; 
         case "NONE":
            return NONE_TYPE;
         default:
            throw new IllegalArgumentException("Unsupported entropy codec type: " + name);
      }
   } 
   
}
