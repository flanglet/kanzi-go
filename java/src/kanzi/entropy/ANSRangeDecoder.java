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
import kanzi.EntropyDecoder;
import kanzi.InputBitStream;

// Implementation of Asymetric Numeral System decoder.
// See "Asymmetric Numeral System" by Jarek Duda at http://arxiv.org/abs/0902.0271
// For alternate C implementation examples, see https://github.com/Cyan4973/FiniteStateEntropy
// and https://github.com/rygorous/ryg_rans

public class ANSRangeDecoder implements EntropyDecoder
{
   private static final long TOP = 1L << 24;
   private static final int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default

   private final InputBitStream bitstream;
   private final int[] alphabet;
   private final int[] freqs;
   private final int[] cumFreqs;
   private short[] f2s; // mapping frequency -> symbol
   private final int chunkSize;
   private int logRange;


   public ANSRangeDecoder(InputBitStream bs)
   {
      this(bs, DEFAULT_CHUNK_SIZE);
   }


   public ANSRangeDecoder(InputBitStream bs, int chunkSize)
   {
      if (bs == null)
         throw new NullPointerException("Invalid null bitstream parameter");

      if ((chunkSize != 0) && (chunkSize < 1024))
         throw new IllegalArgumentException("The chunk size must be at least 1024");

      if (chunkSize > 1<<30)
         throw new IllegalArgumentException("The chunk size must be at most 2^30");

      this.bitstream = bs;
      this.alphabet = new int[256];
      this.freqs = new int[256];
      this.cumFreqs = new int[257];
      this.f2s = new short[0];
      this.chunkSize = chunkSize;
   }


   @Override
   public int decode(byte[] array, int blkptr, int len)
   {
      if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
         return -1;

      if (len == 0)
         return 0;

      final int end = blkptr + len;
      final int sz = (this.chunkSize == 0) ? len : this.chunkSize;
      int startChunk = blkptr;

      while (startChunk < end)
      {
         if (this.decodeHeader(this.freqs) == 0)
            return startChunk - blkptr;

         // logRange field set after decoding header !
         final long mask = (1L << this.logRange) - 1;
         final int endChunk = (startChunk + sz < end) ? startChunk + sz : end;

         // Read initial ANS state
         long st = this.bitstream.readBits(64);

         for (int i=startChunk; i<endChunk; i++)
         {
            final int idx = (int) (st & mask);
            final int symbol = this.f2s[idx];
            array[i] = (byte) symbol;

            // Compute next ANS state
            // D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
            st = (this.freqs[symbol] * (st >>> this.logRange)) + idx - this.cumFreqs[symbol];

            // Normalize
            while (st < TOP)
               st = (st << 32) | this.bitstream.readBits(32);
         }

         startChunk = endChunk;
      }

      return len;
   }


   protected int decodeHeader(int[] frequencies)
   {
      int alphabetSize = EntropyUtils.decodeAlphabet(this.bitstream, this.alphabet, 0);

      if (alphabetSize == 0)
         return 0;

      if (alphabetSize != 256)
      {
         for (int i=0; i<256; i++)
            frequencies[i] = 0;
      }

      this.logRange = (int) (8 + this.bitstream.readBits(3));
      final int scale = 1 << this.logRange;
      int sum = 0;
      int inc = (alphabetSize > 64) ? 16 : 8;
      int llr = 3;

      while (1<<llr <= this.logRange)
         llr++;

      // Decode all frequencies (but the first one) by chunks of size 'inc'
      for (int i=1; i<alphabetSize; i+=inc)
      {
         final int logMax = (int) (1 + this.bitstream.readBits(llr));
         final int endj = (i+inc < alphabetSize) ? i + inc : alphabetSize;

         // Read frequencies
         for (int j=i; j<endj; j++)
         {
            int val = (int) this.bitstream.readBits(logMax);

            if ((val <= 0) || (val >= scale))
            {
               throw new BitStreamException("Invalid bitstream: incorrect frequency " +
                       val + " for symbol '" + this.alphabet[j] + "' in ANS range decoder",
                       BitStreamException.INVALID_STREAM);
            }

            frequencies[this.alphabet[j]] = val;
            sum += val;
         }
      }

      // Infer first frequency
      if (scale <= sum)
      {
         throw new BitStreamException("Invalid bitstream: incorrect frequency " +
                 frequencies[this.alphabet[0]] + " for symbol '" + this.alphabet[0] +
                 "' in ANS range decoder", BitStreamException.INVALID_STREAM);
      }

      frequencies[this.alphabet[0]] = scale - sum;
      this.cumFreqs[0] = 0;

      if (this.f2s.length < scale)
         this.f2s = new short[scale];

      // Create histogram of frequencies scaled to 'range' and reverse mapping
      for (int i=0; i<256; i++)
      {
         this.cumFreqs[i+1] = this.cumFreqs[i] + frequencies[i];
         final int base = (int) this.cumFreqs[i];

         for (int j=frequencies[i]-1; j>=0; j--)
            this.f2s[base+j] = (short) i;
      }

      return alphabetSize;
   }


   @Override
   public InputBitStream getBitStream()
   {
      return this.bitstream;
   }


   @Override
   public void dispose() 
   {
   }
}