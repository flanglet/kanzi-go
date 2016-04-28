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

import kanzi.EntropyEncoder;
import kanzi.OutputBitStream;

// Implementation of Asymetric Numeral System encoder.
// See "Asymmetric Numeral System" by Jarek Duda at http://arxiv.org/abs/0902.0271
// For alternate C implementation examples, see https://github.com/Cyan4973/FiniteStateEntropy
// and https://github.com/rygorous/ryg_rans

public class ANSRangeEncoder implements EntropyEncoder
{
   private static final long TOP = 1L << 24;
   private static final int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
   private static final int DEFAULT_LOG_RANGE = 13;

   private final OutputBitStream bitstream;
   private final int[] alphabet;
   private final int[] freqs;
   private final int[] cumFreqs;
   private int[] buffer;
   private final EntropyUtils eu;
   private final int chunkSize;
   private int logRange;


   public ANSRangeEncoder(OutputBitStream bs)
   {
      this(bs, DEFAULT_CHUNK_SIZE, DEFAULT_LOG_RANGE);
   }


   public ANSRangeEncoder(OutputBitStream bs, int chunkSize, int logRange)
   {
      if (bs == null)
         throw new NullPointerException("Invalid null bitstream parameter");

      if ((chunkSize != 0) && (chunkSize < 1024))
         throw new IllegalArgumentException("The chunk size must be at least 1024");

      if (chunkSize > 1<<30)
         throw new IllegalArgumentException("The chunk size must be at most 2^30");

      if ((logRange < 8) || (logRange > 16))
         throw new IllegalArgumentException("Invalid range parameter: "+logRange+" (must be in [8..16])");

      this.bitstream = bs;
      this.alphabet = new int[256];
      this.freqs = new int[256];
      this.cumFreqs = new int[257];
      this.buffer = new int[0];
      this.logRange = logRange;
      this.chunkSize = chunkSize;
      this.eu = new EntropyUtils();
   }


   protected int updateFrequencies(int[] frequencies, int size, int lr)
   {
      if ((frequencies == null) || (frequencies.length != 256))
         return -1;

      int alphabetSize = this.eu.normalizeFrequencies(frequencies, this.alphabet, size, 1<<lr);
      
      if (alphabetSize > 0)
      {
         this.cumFreqs[0] = 0;

         // Create histogram of frequencies scaled to 'range'
         for (int i=0; i<256; i++)
            this.cumFreqs[i+1] = this.cumFreqs[i] + frequencies[i];
      }
      
      this.encodeHeader(alphabetSize, this.alphabet, frequencies, lr);
      return alphabetSize;
   }


   protected boolean encodeHeader(int alphabetSize, int[] alphabet, int[] frequencies, int lr)
   {
      EntropyUtils.encodeAlphabet(this.bitstream, alphabet, 0, alphabetSize);

      if (alphabetSize == 0)
         return true;

      this.bitstream.writeBits(lr-8, 3); // logRange
      int inc = (alphabetSize > 64) ? 16 : 8;
      int llr = 3;

      while (1<<llr <= lr)
         llr++;

      // Encode all frequencies (but the first one) by chunks of size 'inc'
      for (int i=1; i<alphabetSize; i+=inc)
      {
         int max = 0;
         int logMax = 1;
         final int endj = (i+inc < alphabetSize) ? i + inc : alphabetSize;

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
   public int encode(byte[] array, int blkptr, int len)
   {
      if ((array == null) || (blkptr+len > array.length) || (blkptr < 0) || (len < 0))
         return -1;

      if (len == 0)
         return 0;

      final int[] frequencies = this.freqs;
      final int end = blkptr + len;
      final int sz = (this.chunkSize == 0) ? len : this.chunkSize;
      int startChunk = blkptr;

      if (this.buffer.length < sz)
         this.buffer = new int[sz];

      while (startChunk < end)
      {
         long st = TOP;
         final int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
         int lr = this.logRange;

         // Lower log range if the size of the data chunk is small
         while ((lr > 8) && (1<<lr > endChunk-startChunk))
            lr--;

         for (int i=0; i<256; i++)
            frequencies[i] = 0;

         for (int i=startChunk; i<endChunk; i++)
            frequencies[array[i] & 0xFF]++;

         // Rebuild statistics
         this.updateFrequencies(frequencies, endChunk-startChunk, lr);

         final long top = (TOP >> lr) << 32;
         int n = 0;
         
         // Encoding works in reverse
         for (int i=endChunk-1; i>=startChunk; i--)
         {
            final int symbol = array[i] & 0xFF;
            final int freq = frequencies[symbol];

            // Normalize
            if (st >= top*freq)
            {            
               this.buffer[n++] = (int) st;
               st >>>= 32;
            }

            // Compute next ANS state
            // C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
            st = ((st / freq) << lr) + (st % freq) + this.cumFreqs[symbol];
         }

         startChunk = endChunk;

         // Write final ANS state
         this.bitstream.writeBits(st, 64);

         // Write encoded data to bitstream
         for (n--; n>=0; n--)
            this.bitstream.writeBits(this.buffer[n], 32);
      }

      return len;
   }


   // Not thread safe
   @Override
   public OutputBitStream getBitStream()
   {
      return this.bitstream;
   }
   
   
   @Override
   public void dispose()
   {
   }   
}