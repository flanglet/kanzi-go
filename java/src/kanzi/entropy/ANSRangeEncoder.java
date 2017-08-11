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

package kanzi.entropy;

import kanzi.EntropyEncoder;
import kanzi.OutputBitStream;

// Implementation of an Asymmetric Numeral System encoder.
// See "Asymmetric Numeral System" by Jarek Duda at http://arxiv.org/abs/0902.0271
// Some code has been ported from https://github.com/rygorous/ryg_rans
// For an alternate C implementation example, see https://github.com/Cyan4973/FiniteStateEntropy

public class ANSRangeEncoder implements EntropyEncoder
{
   private static final int ANS_TOP = 1 << 23;
   private static final int DEFAULT_ANS0_CHUNK_SIZE = 1 << 15; // 32 KB by default
   private static final int DEFAULT_LOG_RANGE = 13; // max possible for ANS_TOP=1<23

   private final OutputBitStream bitstream;
   private final int[][] alphabet;
   private final int[][] freqs;
   private final Symbol[][] symbols;
   private byte[] buffer;
   private final EntropyUtils eu;
   private final int chunkSize;
   private final int order;
   private int logRange;


   public ANSRangeEncoder(OutputBitStream bs)
   {
      this(bs, 0, DEFAULT_ANS0_CHUNK_SIZE, DEFAULT_LOG_RANGE);
   }


   public ANSRangeEncoder(OutputBitStream bs, int order)
   {
      this(bs, order, DEFAULT_ANS0_CHUNK_SIZE<<(8*(order&1)), DEFAULT_LOG_RANGE);
   }


   // chunkSize = 0 means 'use input buffer length' during decoding
   public ANSRangeEncoder(OutputBitStream bs, int order, int chunkSize, int logRange)
   {
      if (bs == null)
         throw new NullPointerException("Invalid null bitstream parameter");

      if ((order != 0) && (order != 1))
         throw new IllegalArgumentException("The order must be 0 or 1");

      if ((chunkSize != 0) && (chunkSize < 1024))
         throw new IllegalArgumentException("The chunk size must be at least 1024");

      if (chunkSize > 1<<30)
         throw new IllegalArgumentException("The chunk size must be at most 2^30");

      if ((logRange < 8) || (logRange > 16))
         throw new IllegalArgumentException("Invalid range: "+logRange+" (must be in [8..16])");

      this.bitstream = bs;
      this.order = order;
      final int dim = 255*order + 1;
      this.alphabet = new int[dim][256];
      this.freqs = new int[dim][257]; // freqs[x][256] = total(freqs[x][0..255])
      this.symbols = new Symbol[dim][256];
      this.buffer = new byte[0];
      this.logRange = logRange;
      this.chunkSize = chunkSize;
      this.eu = new EntropyUtils();

      for (int i=0; i<dim; i++)
      {
         this.alphabet[i] = new int[256];
         this.freqs[i] = new int[257];
         this.symbols[i] = new Symbol[256];
      }
   }


   // Compute cumulated frequencies and encode header
   private int updateFrequencies(int[][] frequencies, int lr)
   {
      int res = 0;
      final int endk = 255*this.order + 1;
      this.bitstream.writeBits(lr-8, 3); // logRange

      for (int k=0; k<endk; k++)
      {
         final int[] f = frequencies[k];
         final Symbol[] symb = this.symbols[k];
         final int[] alphabet_ = this.alphabet[k];
         int alphabetSize = this.eu.normalizeFrequencies(f, alphabet_, f[256], 1<<lr);

         if (alphabetSize > 0)
         {
            int sum = 0;

            for (int i=0; i<256; i++)
            {
               if (f[i] == 0)
                  continue;

               symb[i].reset(sum, f[i], lr);
               sum += f[i];
            }
         }

         this.encodeHeader(alphabetSize, alphabet_, f, lr);
         res += alphabetSize;
      }

      return res;
   }


   // Encode alphabet and frequencies
   protected boolean encodeHeader(int alphabetSize, int[] alphabet, int[] frequencies, int lr)
   {
      EntropyUtils.encodeAlphabet(this.bitstream, alphabet, alphabetSize);
 
      if (alphabetSize == 0)
         return true;

      final int chkSize = (alphabetSize > 64) ? 16 : 8;
      int llr = 3;

      while (1<<llr <= lr)
         llr++;

      // Encode all frequencies (but the first one) by chunks
      for (int i=1; i<alphabetSize; i+=chkSize)
      {
         int max = 0;
         int logMax = 1;
         final int endj = (i+chkSize < alphabetSize) ? i + chkSize : alphabetSize;

         // Search for max frequency log size in next chunk
         for (int j=i; j<endj; j++)
         {
            if (frequencies[alphabet[j]] > max)
               max = frequencies[alphabet[j]];
         }

         while (1<<logMax <= max)
            logMax++;

         this.bitstream.writeBits(logMax-1, llr);

         // Write frequencies
         for (int j=i; j<endj; j++)
            this.bitstream.writeBits(frequencies[alphabet[j]], logMax);
      }

      return true;
   }


   // Dynamically compute the frequencies for every chunk of data in the block
   @Override
   public int encode(byte[] block, int blkptr, int len)
   {
      if ((block == null) || (blkptr+len > block.length) || (blkptr < 0) || (len < 0))
         return -1;

      if (len == 0)
         return 0;

      final int end = blkptr + len;
      final int sz = (this.chunkSize == 0) ? len : this.chunkSize;
      int startChunk = blkptr;
      final int endk = 255*this.order + 1;

      for (int k=0; k<endk; k++)
      {
         Symbol[] syms = this.symbols[k];

         for (int i=0; i<256; i++)
            syms[i] = new Symbol();
      }

      if (this.buffer.length < 2*sz)
         this.buffer = new byte[2*sz];

      while (startChunk < end)
      {
         final int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
         int lr = this.logRange;

         // Lower log range if the size of the data chunk is small
         while ((lr > 8) && (1<<lr > endChunk-startChunk))
            lr--;

         this.rebuildStatistics(block, startChunk, endChunk, lr);
         this.encodeChunk(block, startChunk, endChunk);
         startChunk = endChunk;
      }

      return len;
   }


   private void encodeChunk(byte[] block, int start, int end)
   {
      int st = ANS_TOP;
      int n = 0;

      if (this.order == 0)
      {
         final Symbol[] symb = this.symbols[0];

         for (int i=end-1; i>=start; i--)
         {
            final int cur = block[i] & 0xFF;
            final Symbol sym = symb[cur];

            while (st >= sym.xMax)
            {
               this.buffer[n++] = (byte) st;
               st >>>= 8;
            }

            // Compute next ANS state
            // C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
            // st = ((st / freq) << lr) + (st % freq) + cumFreq[prv];
            final long q = (st*sym.invFreq) >>> sym.invShift;
            st = (int) (st + sym.bias + q*sym.cmplFreq);
         }
      }
      else // order 1
      {
         int prv = block[end-1] & 0xFF;

         for (int i=end-2; i>=start; i--)
         {
            final int cur = block[i] & 0xFF;
            final Symbol sym = this.symbols[cur][prv];

            while (st >= sym.xMax)
            {
               this.buffer[n++] = (byte) st;
               st >>>= 8;
            }

            // Compute next ANS state
            // C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
            // st = ((st / freq) << lr) + (st % freq) + cumFreq[cur][prv];
            final long q = (st*sym.invFreq) >>> sym.invShift;            
            st = (int) (st + sym.bias + q*sym.cmplFreq);
            prv = cur;
         }

         // Last symbol
         final Symbol sym = this.symbols[0][prv];

         while (st >= sym.xMax)
         {
            this.buffer[n++] = (byte) st;
            st >>>= 8;
         }

         final long q = (st*sym.invFreq) >>> sym.invShift;
         st = (int) (st + sym.bias + q*sym.cmplFreq);
      }

      // Write final ANS state
      this.bitstream.writeBits(st, 32);

      // Write encoded data to bitstream
      for (n--; n>=0; n--)
         this.bitstream.writeBits(this.buffer[n], 8);
   }


   // Compute chunk frequencies, cumulated frequencies and encode chunk header
   private void rebuildStatistics(byte[] block, int start, int end, int lr)
   {
      final int endj = 255*this.order + 1;

      for (int j=0; j<endj; j++)
      {
         final int[] f = this.freqs[j];

         for (int i=f.length-1; i>=0; i--)
            f[i] = 0;
      }

      if (this.order == 0)
      {
         final int[] f = this.freqs[0];
         f[256] = end - start;

         for (int i=start; i<end; i++)
            f[block[i]&0xFF]++;
      }
      else
      {
         int prv = 0;

         for (int i=start; i<end; i++)
         {
            final int cur = block[i] & 0xFF;
            this.freqs[prv][cur]++;
            this.freqs[prv][256]++;
            prv = cur;
         }
      }

      this.updateFrequencies(this.freqs, lr);
   }


   @Override
   public OutputBitStream getBitStream()
   {
      return this.bitstream;
   }


   @Override
   public void dispose()
   {
   }



   static class Symbol
   {
      int xMax; // (Exclusive) upper bound of pre-normalization interval
      int bias; // Bias
      int cmplFreq; // Complement of frequency: (1 << scale_bits) - freq
      int invShift; // Reciprocal shift
      long invFreq; // Fixed-point reciprocal frequency
      int freq;


      public void reset(int cumFreq, int freq, int logRange)
      {
         // Make sure xMax is a positive int32
         if (freq >= 1<<logRange)
            freq = (1<<logRange) - 1;
                  
         this.freq = freq;
         this.xMax = ((ANS_TOP>>>logRange) << 8) * freq;
         this.cmplFreq = (1<<logRange) - freq;

         if (freq < 2)
         {
            this.invFreq = 0xFFFFFFFFL;
            this.invShift = 32;
            this.bias = cumFreq + (1<<logRange) - 1;
         }
         else
         {
            int shift = 0;

            while (freq > (1<<shift))
                shift++;

            // Alverson, "Integer Division using reciprocals"
            this.invFreq = (((1L<<(shift+31))+freq-1) / freq) & 0xFFFFFFFFL;
            this.invShift = 32 + shift - 1;
            this.bias = cumFreq;
         }
      }
   }
}