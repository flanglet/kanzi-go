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

import java.util.PriorityQueue;
import kanzi.BitStreamException;
import kanzi.InputBitStream;
import kanzi.OutputBitStream;


public class EntropyUtils
{
   private static final int FULL_ALPHABET = 0;
   private static final int PARTIAL_ALPHABET = 1;
   private static final int ALPHABET_256 = 0;
   private static final int ALPHABET_NOT_256 = 1;
   private static final int DELTA_ENCODED_ALPHABET = 0;
   private static final int BIT_ENCODED_ALPHABET_256 = 1;
   private static final int PRESENT_SYMBOLS_MASK = 0;
   private static final int ABSENT_SYMBOLS_MASK = 1;


   private int[] ranks;
   private int[] errors;


   public EntropyUtils()
   {
      this.ranks = new int[0];
      this.errors = new int[0];
   }


   // alphabet must be sorted in increasing order
   // alphabet length must be a power of 2
   public static int encodeAlphabet(OutputBitStream obs, int[] alphabet, int count)
   {
      // Alphabet length must be a power of 2
      if ((alphabet.length & (alphabet.length-1)) != 0)
         return -1;
      
      if (count > alphabet.length)
         return -1;

      // First, push alphabet encoding mode
      if ((alphabet.length > 0) && (count == alphabet.length))
      {
         // Full alphabet
         obs.writeBit(FULL_ALPHABET);
         
         if (count == 256)
           obs.writeBit(ALPHABET_256); // shortcut
         else
         {
            int log = 1;

            while (1<<log <= count)
               log++;            

            // Write alphabet size
            obs.writeBit(ALPHABET_NOT_256);
            obs.writeBits(log-1, 5);
            obs.writeBits(count, log);
         }
         
         return count;
      }

      obs.writeBit(PARTIAL_ALPHABET);

      if ((alphabet.length == 256) && (count >= 32) && (count <= 224))
      {
         // Regular alphabet of symbols less than 256
         obs.writeBit(BIT_ENCODED_ALPHABET_256);
         long[] masks = new long[4];

         for (int i=0; i<count; i++)
            masks[alphabet[i]>>6] |= (1L << (alphabet[i] & 63));
         
         for (int i=0; i<masks.length; i++)
            obs.writeBits(masks[i], 64);
         
         return count;
      }      

      obs.writeBit(DELTA_ENCODED_ALPHABET);
      final int[] diffs = new int[count];

      if (alphabet.length - count < count)
      {
         // Encode all missing symbols
         count = alphabet.length - count;
         int log = 1;

         while (1<<log <= count)
            log++;

         // Write length
         obs.writeBits(log-1, 4);
         obs.writeBits(count, log);

         if (count == 0)
            return 0;
         
         obs.writeBit(ABSENT_SYMBOLS_MASK);
         log = 1;

         while (1<<log <= alphabet.length)
            log++;
         
         // Write log(alphabet size)
         obs.writeBits(log-1, 5);         
         int symbol = 0;
         int previous = 0;

         // Create deltas of missing symbols
         for (int n=0, i=0; n<count; )
         {
            if (symbol == alphabet[i])
            {
               if (i < alphabet.length-1-count)
                  i++;

               symbol++;
               continue;
            }

            diffs[n] = symbol - previous;
            symbol++;
            previous = symbol;             
            n++;
         }
      }
      else
      {
         // Encode all present symbols
         int log = 1;

         while (1<<log <= count)
            log++;

         // Write length
         obs.writeBits(log-1, 4);
         obs.writeBits(count, log);

         if (count == 0)
            return 0;
            
         obs.writeBit(PRESENT_SYMBOLS_MASK);
         int previous = 0;

         // Create deltas of present symbols
         for (int i=0; i<count; i++)
         {
            diffs[i] = alphabet[i] - previous;
            previous = alphabet[i] + 1;                  
         }         
      }

      final int ckSize = (count <= 64) ? 8 : 16;

      // Encode all deltas by chunks 
      for (int i=0; i<count; i+=ckSize)
      {
         int max = 0;

         // Find log(max(deltas)) for this chunk
         for (int j=i; (j<count) && (j<i+ckSize); j++)
         {
            if (max < diffs[j])
               max = diffs[j];
         }
          
         int log = 1;

         while (1<<log <= max)
               log++;

         obs.writeBits(log-1, 4);

         // Write deltas for this chunk
         for (int j=i; (j<count) && (j<i+ckSize); j++)
            encodeSize(obs, log, diffs[j]);
      } 
      
      return count;
   }
   
   
   private static void encodeSize(OutputBitStream obs, int log, int val)
   {
      obs.writeBits(val, log);
   }
      
      
   private static long decodeSize(InputBitStream ibs, int log)
   {
      return ibs.readBits(log);            
   }
   
   
   public static int decodeAlphabet(InputBitStream ibs, int[] alphabet) throws BitStreamException
   {
      // Read encoding mode from bitstream
      final int aphabetType = ibs.readBit();

      if (aphabetType == FULL_ALPHABET)
      {
         int alphabetSize;
         
         if (ibs.readBit() == ALPHABET_256) 
            alphabetSize = 256;
         else
         {
            int log = 1 + (int) ibs.readBits(5);
            alphabetSize = (int) ibs.readBits(log);
         }

         // Full alphabet
         for (int i=0; i<alphabetSize; i++)
            alphabet[i] = i;

         return alphabetSize;
      }

      int count = 0;
      final int mode = ibs.readBit();

      if (mode == BIT_ENCODED_ALPHABET_256)
      {
         // Decode presence flags
         for (int i=0; i<256; i+=64)
         {
            final long val = ibs.readBits(64);

            for (int j=0; j<64; j++)
            {
               if ((val & (1L << j)) != 0)
               {
                  alphabet[count] = i + j;
                  count++;
               }
            }
         }
         
         return count;
      }
      
      // DELTA_ENCODED_ALPHABET
      int log = 1 + (int) ibs.readBits(4); 
      count = (int) ibs.readBits(log); 
      
      if (count == 0)
         return 0;
      
      final int ckSize = (count <= 64) ? 8 : 16;
      int n = 0;
      int symbol = 0;

      if (ibs.readBit() == ABSENT_SYMBOLS_MASK)
      {
         int alphabetSize = 1 << (int) ibs.readBits(5);
         
         // Read missing symbols
         for (int i=0; i<count; i+=ckSize)
         {
            log = 1 + (int) ibs.readBits(4);
      
            // Read deltas for this chunk
            for (int j=i; (j<count) && (j<i+ckSize); j++)
            {
               final int next = symbol + (int) decodeSize(ibs, log);

               while ((symbol < next) && (n < alphabetSize))
               {
                  alphabet[n] = symbol++;
                  n++;
               }

               symbol++;
            }
         }

         count = alphabetSize - count;

         while (n < count)
            alphabet[n++] = symbol++;
      }
      else
      {
         // Read present symbols
         for (int i=0; i<count; i+=ckSize)
         {
            log = 1 + (int) ibs.readBits(4);

            // Read deltas for this chunk
            for (int j=i; (j<count) && (j<i+ckSize); j++)
            {
               symbol += (int) decodeSize(ibs, log);
               alphabet[j] = symbol;
               symbol++;
            }
         }          
      }

      return count;
   }


   // Not thread safe
   // Returns the size of the alphabet
   // The alphabet and freqs parameters are updated
   public int normalizeFrequencies(int[] freqs, int[] alphabet, int count, int scale)
   {
      if (count == 0)
         return 0;

      if ((scale < 1<<8) || (scale > 1<<16))
         throw new IllegalArgumentException("Invalid scale parameter: "+ scale +
                 " (must be in [256..65536])");

      int alphabetSize = 0;

      // range == count shortcut
      if (count == scale)
      {
         for (int i=0; i<freqs.length; i++)
         {
            if (freqs[i] != 0)
               alphabet[alphabetSize++] = i;
         }

         return alphabetSize;
      }

      if (this.ranks.length < alphabet.length)
         this.ranks = new int[alphabet.length];

      if (this.errors.length < alphabet.length)
         this.errors = new int[alphabet.length];

      int sum = -scale;

      // Scale frequencies by stretching distribution over complete range
      for (int i=0; i<alphabet.length; i++)
      {
         alphabet[i] = 0;
         this.errors[i] = -1;
         this.ranks[i] = i;

         if (freqs[i] == 0)
            continue;

         long sf = (long) freqs[i] * scale;
         int scaledFreq = (int) (sf / count);

         if (scaledFreq == 0)
         {
            // Quantum of frequency
            scaledFreq = 1;
         }
         else
         {
            // Find best frequency rounding value
            long errCeiling = ((scaledFreq+1) * (long) count) - sf;
            long errFloor = sf - (scaledFreq * (long) count);

            if (errCeiling < errFloor)
            {
               scaledFreq++;
               this.errors[i] = (int) errCeiling;
            }
            else
            {
               this.errors[i] = (int) errFloor;
            }
         }

         alphabet[alphabetSize++] = i;
         sum += scaledFreq;
         freqs[i] = scaledFreq;
      }

      if (alphabetSize == 0)
         return 0;

      if (alphabetSize == 1)
      {
         freqs[alphabet[0]] = scale;
         return 1;
      }

      if (sum != 0)
      {
         // Need to normalize frequency sum to range
         final int inc = (sum > 0) ? -1 : 1;
         PriorityQueue<FreqSortData> queue = new PriorityQueue<FreqSortData>();

         // Create sorted queue of present symbols (except those with 'quantum frequency')
         for (int i=0; i<alphabetSize; i++)
         {
            if (this.errors[alphabet[i]] >= 0)
               queue.add(new FreqSortData(this.errors, freqs, alphabet[i]));
         }

         while ((sum != 0) && (queue.size() > 0))
         {
             // Remove symbol with highest error
             FreqSortData fsd = queue.poll();

             // Do not zero out any frequency
             if (freqs[fsd.symbol] == -inc)
                continue;

             // Distort frequency and error
             freqs[fsd.symbol] += inc;
             this.errors[fsd.symbol] -= scale;
             sum += inc;
             queue.add(fsd);
         }
      }

      return alphabetSize;
   }


   private static class FreqSortData implements Comparable<FreqSortData>
   {
      final int symbol;
      final int[] errors;
      final int[] frequencies;


      public FreqSortData(int[] errors, int[] frequencies, int symbol)
      {
         this.errors = errors;
         this.frequencies = frequencies;
         this.symbol = symbol & 0xFFFF;
      }


      @Override
      public boolean equals(Object o)
      {
         if (o == null)
            return false;
         
         if (this == o)
            return true;
         
         return ((FreqSortData) o).symbol == this.symbol;
      }

      
      @Override
      public int hashCode() 
      {
         return this.symbol;
      }
      
      
      @Override
      public int compareTo(FreqSortData sd)
      {
         // Decreasing error
         int res = sd.errors[sd.symbol] - this.errors[this.symbol];

         // Decreasing frequency
         if (res == 0) 
         {
            res = sd.frequencies[sd.symbol] - this.frequencies[this.symbol];
         
            // Decreasing symbol
            if (res == 0)
               return sd.symbol - this.symbol;
         }

         return res;
      }
   }  
}
