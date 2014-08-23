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

import kanzi.BitStreamException;
import kanzi.InputBitStream;
import kanzi.OutputBitStream;
import kanzi.util.sort.DefaultArrayComparator;
import kanzi.util.sort.QuickSort;


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
   private int[] scaledFreqs;
   private final int logRange;


   public EntropyUtils(int logRange)
   {
      if ((logRange < 8) || (logRange > 16))
         throw new IllegalArgumentException("Invalid range parameter: "+ logRange +
                 " (must be in [8..16])");

      this.buf1 = new int[0];
      this.buf2 = new int[0];
      this.scaledFreqs = new int[0];
      this.logRange = logRange;
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
   public int normalizeFrequencies(int[] freqs, int[] cumFreqs, int count) throws BitStreamException
   {
      if (count == 0)
         return 0;

      if (this.buf1.length < 256)
         this.buf1 = new int[256];

      if (this.buf2.length < 256)
         this.buf2 = new int[256];

      if (this.scaledFreqs.length < 256)
         this.scaledFreqs = new int[256];

      final int[] alphabet = this.buf1;
      final int[] errors = this.buf2;
      int alphabetSize = 0;
      int sum = 0;
      final int range = 1 << this.logRange;

      for (int i=0; i<256; i++)
         alphabet[i] = 0;

      // Scale frequencies by stretching distribution over complete range
      for (int i=0; i<256; i++)
      {
         if (freqs[i] == 0)
            continue;

         int scaledFreq = (range * freqs[i]) / count;

         if (scaledFreq == 0)
         {
            // Smallest non zero frequency numerator
            // Pretend that this is a perfect fit (to avoid messing with this frequency below)
            scaledFreq = 1;
            errors[i] = 0;
         }
         else
         {
            int errCeiling = ((scaledFreq+1) * count) / range - freqs[i];
            int errFloor = freqs[i] - (scaledFreq * count) / range;

            if (errCeiling < errFloor)
            {
               scaledFreq++;
               errors[i] = errCeiling;
            }
            else
            {
               errors[i] = errFloor;
            }
         }

         alphabet[alphabetSize++] = i;
         sum += scaledFreq;
         this.scaledFreqs[i] = scaledFreq;
      }

      if (alphabetSize == 0)
         return 0;

      cumFreqs[0] = 0;

      if (sum != range)
      {
         // Need to normalize frequency sum to range
         int[] ranks = new int[256];

         for (int i=0; i<256; i++)
            ranks[i] = i;

         // Adjust rounding of fractional scaled frequencies so that sum == range
         sum -= range;
         int prevSum = ~sum;

         while (sum != 0)
         {
            // If we cannot converge, exit
            if (prevSum == sum)
               break;

            // Sort array by increasing rounding error
            QuickSort sorter = new QuickSort(new DefaultArrayComparator(errors));
            sorter.sort(ranks, 0, alphabetSize);
            prevSum = sum;
            int inc = (sum > 0) ? -1 : 1;
            int idx = alphabetSize - 1;

            // Remove from frequencies with largest floor rounding error
            while ((idx >= 0) && (sum != 0))
            {
               if (errors[ranks[idx]] == 0)
                  break;

               this.scaledFreqs[alphabet[ranks[idx]]] += inc;
               errors[alphabet[ranks[idx]]] += inc;
               sum += inc;
               idx--;
            }
         }
      }

      // Create histogram of frequencies scaled to 'range'
      for (int i=0; i<256; i++)
         cumFreqs[i+1] = cumFreqs[i] + this.scaledFreqs[i];

      return alphabetSize;
   }

}
