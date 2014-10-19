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
   private static final int SEVEN_BIT_ALPHABET = 0;
   private static final int EIGHT_BIT_ALPHABET = 1;
   private static final int DELTA_ENCODED_ALPHABET = 0;
   private static final int BIT_ENCODED_ALPHABET = 1;
   private static final int PRESENT_SYMBOLS_MASK = 0;
   private static final int ABSENT_SYMBOLS_MASK = 1;


   private int[] buf1;
   private int[] buf2;


   public EntropyUtils()
   {
      this.buf1 = new int[0];
      this.buf2 = new int[0];
   }


   // alphabet must be sorted in increasing order
   public static int encodeAlphabet(OutputBitStream obs, int alphabetSize, int[] alphabet)
   {
      // First, push alphabet encoding mode
      if (alphabetSize == 256)
      {
         // Full alphabet
         obs.writeBit(FULL_ALPHABET);
         obs.writeBit(EIGHT_BIT_ALPHABET);
         return 256;
      }

      if (alphabetSize == 128)
      {
         boolean flag = true;

         for (int i=0; ((flag) && (i<128)); i++)
            flag &= (alphabet[i] == i);

         if (flag == true)
         {
            obs.writeBit(FULL_ALPHABET);
            obs.writeBit(SEVEN_BIT_ALPHABET);
            return 128;
         }
      }

      obs.writeBit(PARTIAL_ALPHABET);

      final int[] diffs = new int[32];
      int maxSymbolDiff = 1;

      if ((alphabetSize != 0) && ((alphabetSize < 32) || (alphabetSize > 224)))
      {
         obs.writeBit(DELTA_ENCODED_ALPHABET);

         if (alphabetSize >= 224)
         {
            // Big alphabet, encode all missing symbols
            alphabetSize = 256 - alphabetSize;
            obs.writeBits(alphabetSize, 5);
            obs.writeBit(ABSENT_SYMBOLS_MASK);
            int symbol = 0;
            int previous = 0;

            for (int n=0, i=0; n<alphabetSize; )
            {
               if (symbol == alphabet[i])
               {
                  if (i < alphabet.length-1)
                     i++;

                  symbol++;
                  continue;
               }

               diffs[n] = symbol - previous;
               symbol++;
               previous = symbol;

               if (diffs[n] > maxSymbolDiff)
                  maxSymbolDiff = diffs[n];

               n++;
            }
         }
         else
         {
            // Small alphabet, encode all present symbols
            obs.writeBits(alphabetSize, 5);
            obs.writeBit(PRESENT_SYMBOLS_MASK);
            int previous = 0;

            for (int i=0; i<alphabetSize; i++)
            {
               diffs[i] = alphabet[i] - previous;
               previous = alphabet[i] + 1;

               if (diffs[i] > maxSymbolDiff)
                  maxSymbolDiff = diffs[i];
            }
         }

         // Write log(max(diff)) to bitstream
         int log = 1;

         while (1<<log <= maxSymbolDiff)
            log++;

         obs.writeBits(log-1, 3); // delta size

         // Write all symbols with delta encoding
         for (int i=0; i<alphabetSize; i++)
            obs.writeBits(diffs[i], log);
      }
      else
      {
         // Regular (or empty) alphabet
         obs.writeBit(BIT_ENCODED_ALPHABET);
         long[] masks = new long[4];

         for (int i=0; i<alphabetSize; i++)
            masks[alphabet[i]>>6] |= (1L << (alphabet[i] & 63));

         for (int i=0; i<masks.length; i++)
            obs.writeBits(masks[i], 64);
      }

      return alphabetSize;
   }


   public static int decodeAlphabet(InputBitStream ibs, int[] alphabet) throws BitStreamException
   {
      // Read encoding mode from bitstream
      final int aphabetType = ibs.readBit();

      if (aphabetType == FULL_ALPHABET)
      {
         int alphabetSize = (ibs.readBit() == EIGHT_BIT_ALPHABET) ? 256 : 128;

         // Full alphabet
         for (int i=0; i<alphabetSize; i++)
            alphabet[i] = i;

         return alphabetSize;
      }

      int alphabetSize = 0;
      final int mode = ibs.readBit();

      if (mode == BIT_ENCODED_ALPHABET)
      {
         // Decode presence flags
         for (int i=0; i<256; i+=64)
         {
            final long val = ibs.readBits(64);

            for (int j=0; j<64; j++)
            {
               if ((val & (1L << j)) != 0)
                  alphabet[alphabetSize++] = i + j;
            }
         }
      }
      else // DELTA_ENCODED_ALPHABET
      {
         final int val = (int) ibs.readBits(6);
         final int log = (int) (1 + ibs.readBits(3)); // log(max(diff))
         alphabetSize = val >> 1;
         int n = 0;
         int symbol = 0;

         if ((val & 1) == ABSENT_SYMBOLS_MASK)
         {
            for (int i=0; i<alphabetSize; i++)
            {
               final int next = symbol + (int) ibs.readBits(log);

               while (symbol < next)
                  alphabet[n++] = symbol++;

               symbol++;
            }

            alphabetSize = 256 - alphabetSize;

            while (n < alphabetSize)
               alphabet[n++] = symbol++;
         }
         else
         {
            for (int i=0; i<alphabetSize; i++)
            {
               symbol += (int) ibs.readBits(log);
               alphabet[i] = symbol;
               symbol++;
            }
         }
      }

      return alphabetSize;
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
         for (int i=0; i<256; i++)
         {
            if (freqs[i] != 0)
               alphabet[alphabetSize++] = i;
         }

         return alphabetSize;
      }

      if (this.buf1.length < 256)
         this.buf1 = new int[256];

      if (this.buf2.length < 256)
         this.buf2 = new int[256];

      final int[] ranks = this.buf1;
      final int[] errors = this.buf2;
      int sum = -scale;

      // Scale frequencies by stretching distribution over complete range
      for (int i=0; i<256; i++)
      {
         alphabet[i] = 0;
         errors[i] = -1;
         ranks[i] = i;

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
               errors[i] = (int) errCeiling;
            }
            else
            {
               errors[i] = (int) errFloor;
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
            if (errors[alphabet[i]] >= 0)
               queue.add(new FreqSortData(errors, freqs, alphabet[i]));
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
             errors[fsd.symbol] -= scale;
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
         this.symbol = symbol & 0xFF;
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
