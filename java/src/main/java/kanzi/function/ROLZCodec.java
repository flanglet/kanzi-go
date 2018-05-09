
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

package kanzi.function;

import kanzi.ByteFunction;
import kanzi.Memory;
import kanzi.Predictor;
import kanzi.SliceByteArray;

// Implementation of a Reduced Offset Lempel Ziv transform
// Code based on 'balz' by Ilya Muravyov
// More information about ROLZ at http://ezcodesample.com/rolz/rolz_article.html

public final class ROLZCodec implements ByteFunction
{
   private static final int HASH_SIZE = 1 << 16;
   private static final int MIN_MATCH = 3;
   private static final int MAX_MATCH = MIN_MATCH + 255;
   private static final int LOG_POS_CHECKS = 5;
   private static final int CHUNK_SIZE = 1 << 26;
   private static final int LITERAL_FLAG = 0;
   private static final int MATCH_FLAG = 1;
   private static final int HASH = 200002979;
   private static final int HASH_MASK = ~(CHUNK_SIZE - 1);

   private final int logPosChecks;
   private final int maskChecks;
   private final int posChecks;
   private final int[] matches;
   private final int[] counters;
   private final ROLZPredictor litPredictor;
   private final ROLZPredictor matchPredictor;


   public ROLZCodec()
   {
      this(LOG_POS_CHECKS);
   }


   public ROLZCodec(int logPosChecks)
   {
      if ((logPosChecks < 2) || (logPosChecks > 8))
         throw new IllegalArgumentException("Invalid logPosChecks parameter " +
            "(must be in [2..8])");

      this.logPosChecks = logPosChecks;
      this.posChecks = 1 << logPosChecks;
      this.maskChecks = this.posChecks - 1;
      this.counters = new int[1<<16];
      this.matches = new int[HASH_SIZE<<this.logPosChecks];
      this.litPredictor = new ROLZPredictor(9);
      this.matchPredictor = new ROLZPredictor(LOG_POS_CHECKS);
   }


   private static int getKey(final byte[] buf, final int idx)
   {
      return Memory.LittleEndian.readInt16(buf, idx) & 0x7FFFFFFF;
   }


   private static int hash(final byte[] buf, final int idx)
   {
      return ((Memory.LittleEndian.readInt32(buf, idx)&0x00FFFFFF) * HASH) & HASH_MASK;
   }


   // return position index (LOG_POS_CHECKS bits) + length (8 bits) or -1
   private int findMatch(final SliceByteArray sba, final int pos)
   {
      final byte[] buf = sba.array;
      final int key = getKey(buf, pos-2);
      final int base = key << this.logPosChecks;
      final int hash32 = hash(buf, pos);
      final int counter = this.counters[key];
      int bestLen = MIN_MATCH - 1;
      int bestIdx = -1;
      byte first = buf[pos];
      final int maxMatch = (sba.length-pos >= MAX_MATCH) ? MAX_MATCH : sba.length-pos;

      // Check all recorded positions
      for (int i=0; i<this.posChecks; i++)
      {
         int ref = this.matches[base+((counter-i)&this.maskChecks)];

         if (ref == 0)
            break;

         // Hash check may save a memory access ...
         if ((ref & HASH_MASK) != hash32)
            continue;

         ref = (ref & ~HASH_MASK) + sba.index;

         if (buf[ref] != first)
            continue;

         int n = 1;

         while ((n < maxMatch) && (buf[ref+n] == buf[pos+n]))
            n++;

         if (n > bestLen)
         {
            bestIdx = i;
            bestLen = n;

            if (bestLen == maxMatch)
               break;
         }
      }

      // Register current position
      this.counters[key]++;
      this.matches[base+(this.counters[key]&this.maskChecks)] = hash32 | (pos-sba.index);
      return (bestLen < MIN_MATCH) ? -1 : (bestIdx<<8) | (bestLen-MIN_MATCH);
   }



   @Override
   public boolean forward(SliceByteArray input, SliceByteArray output)
   {
      if ((!SliceByteArray.isValid(input)) || (!SliceByteArray.isValid(output)))
         return false;

      if (input.array == output.array)
         return false;

      final int count = input.length;

      if (output.length - output.index < this.getMaxEncodedLength(count))
         return false;

      if (count <= 16)
      {
         for (int i=0; i<count; i++)
            output.array[output.index+i] = input.array[input.index+i];

         input.index += count;
         output.index += count;
         return true;
      }

      int srcIdx = input.index;
      int dstIdx = output.index;
      final byte[] src = input.array;
      final byte[] dst = output.array;
      final int srcEnd = srcIdx + count - 4;
      Memory.BigEndian.writeInt32(dst, dstIdx, count);
      dstIdx += 4;
      int sizeChunk = (count <= CHUNK_SIZE) ? count : CHUNK_SIZE;
      int startChunk = 0;
      SliceByteArray sba1 = new SliceByteArray(dst, dstIdx);
      this.litPredictor.reset();
      this.matchPredictor.reset();
      final Predictor[] predictors = new Predictor[] { this.litPredictor, this.matchPredictor };
      ROLZEncoder re = new ROLZEncoder(predictors, sba1);

      for (int i=0; i<this.counters.length; i++)
         this.counters[i] = 0;

      while (startChunk < srcEnd)
      {
         for (int i=0; i<this.matches.length; i++)
            this.matches[i] = 0;

         final int endChunk = (startChunk+sizeChunk < srcEnd) ? startChunk+sizeChunk : srcEnd;
         final SliceByteArray sba2 = new SliceByteArray(src, endChunk, startChunk);
         srcIdx = startChunk + 2;
         this.litPredictor.setContext((byte) 0);
         re.setContext(LITERAL_FLAG);
         re.encodeBit(LITERAL_FLAG);
         re.encodeByte(src[startChunk]);

         if (startChunk+1 < srcEnd)
         {
            re.encodeBit(LITERAL_FLAG);
            re.encodeByte(src[startChunk+1]);
         }

         while (srcIdx < endChunk)
         {
            this.litPredictor.setContext(src[srcIdx-1]);
            re.setContext(LITERAL_FLAG);
            final int match = findMatch(sba2, srcIdx);

            if (match == -1)
            {
               re.encodeBit(LITERAL_FLAG);
               re.encodeByte(src[srcIdx]);
               srcIdx++;
            }
            else
            {
               final int matchLen = match & 0xFF;
               re.encodeBit(MATCH_FLAG);
               re.encodeByte((byte) matchLen);
               final int matchIdx = match >> 8;
               this.matchPredictor.setContext(src[srcIdx-1]);
               re.setContext(MATCH_FLAG);
                             
               for (int shift=this.logPosChecks-1; shift>=0; shift--)
                  re.encodeBit((matchIdx>>shift) & 1);

               srcIdx += (matchLen + MIN_MATCH);
            }
         }

         startChunk = endChunk;
      }

      // Last literals
      re.setContext(LITERAL_FLAG);
      
      for (int i=0; i<4; i++, srcIdx++)
      {
         this.litPredictor.setContext(src[srcIdx-1]);
         re.encodeBit(LITERAL_FLAG);
         re.encodeByte(src[srcIdx]);
      }

      re.dispose();
      input.index = srcIdx; 
      output.index = sba1.index;      
      return srcIdx == srcEnd + 4;
   }


   @Override
   public boolean inverse(SliceByteArray input, SliceByteArray output)
   {
      if ((!SliceByteArray.isValid(input)) || (!SliceByteArray.isValid(output)))
         return false;

      if (input.array == output.array)
         return false;

      final int count = input.length;

      if (count <= 16)
      {
         for (int i=0; i<count; i++)
            output.array[output.index+i] = input.array[input.index+i];

         input.index += count;
         output.index += count;
         return true;
      }

      final byte[] src = input.array;
      final byte[] dst = output.array;
      int srcIdx = input.index;
      final int srcEnd = srcIdx + count - 4;
      final int dstEnd = Memory.BigEndian.readInt32(src, srcIdx);
      srcIdx += 4;
      int sizeChunk = (dstEnd < CHUNK_SIZE) ? dstEnd : CHUNK_SIZE;
      int startChunk = output.index;
      SliceByteArray sba = new SliceByteArray(src, srcIdx);
      this.litPredictor.reset();
      this.matchPredictor.reset();
      final Predictor[] predictors = new Predictor[] { this.litPredictor, this.matchPredictor };
      ROLZDecoder rd = new ROLZDecoder(predictors, sba);

      for (int i=0; i<this.counters.length; i++)
         this.counters[i] = 0;

      while (startChunk < dstEnd)
      {
         for (int i=0; i<this.matches.length; i++)
            this.matches[i] = 0;

         final int endChunk = (startChunk+sizeChunk < dstEnd) ? startChunk+sizeChunk : dstEnd;
         int dstIdx = output.index;
         this.litPredictor.setContext((byte) 0);
         rd.setContext(LITERAL_FLAG);
         int bit = rd.decodeBit();

         if (bit == LITERAL_FLAG)
         {
            dst[dstIdx++] = rd.decodeByte();

            if (output.index+1 < dstEnd)
            {
               bit = rd.decodeBit();

               if (bit == LITERAL_FLAG)
                  dst[dstIdx++] = rd.decodeByte();
            }
         }

         // Sanity check
         if (bit == MATCH_FLAG)
         {
            output.index = dstIdx;
            break;
         }

         while (dstIdx < endChunk)
         {
            final int savedIdx = dstIdx;
            final int key = getKey(dst, dstIdx-2);
            final int base = key << this.logPosChecks;
            this.litPredictor.setContext(dst[dstIdx-1]);
            rd.setContext(LITERAL_FLAG);

            if (rd.decodeBit() == MATCH_FLAG)
            {
               // Match flag
               int matchLen = rd.decodeByte() & 0xFF;

               // Sanity check
               if (dstIdx + matchLen > dstEnd)
               {
                  output.index = dstIdx;
                  break;
               }

               this.matchPredictor.setContext(dst[dstIdx-1]);
               rd.setContext(MATCH_FLAG);
               int matchIdx = 0;

               for (int shift=this.logPosChecks-1; shift>=0; shift--)
                  matchIdx |= (rd.decodeBit()<<shift);

               int ref = output.index + this.matches[base+((this.counters[key]-matchIdx)&this.maskChecks)];

               // Copy
               dst[dstIdx] = dst[ref];
               dst[dstIdx+1] = dst[ref+1];
               dst[dstIdx+2] = dst[ref+2];
               dstIdx += 3;
               ref += 3;

               while (matchLen != 0)
               {
                  dst[dstIdx++] = dst[ref++];
                  matchLen--;
               }
            }
            else
            {
               // Literal flag
               dst[dstIdx++] = rd.decodeByte();
            }

            // Update
            this.counters[key]++;
            this.matches[base+(this.counters[key]&this.maskChecks)] = savedIdx - output.index;
         }

         startChunk = endChunk;
         output.index = dstIdx;
      }

      rd.dispose();   
      input.index = sba.index;
      return input.index == srcEnd + 4;
   }


   @Override
   public int getMaxEncodedLength(int srcLength)
   {
      return srcLength * 5 / 4;
   }



   static class ROLZEncoder
   {
      private static final long TOP        = 0x00FFFFFFFFFFFFFFL;
      private static final long MASK_24_56 = 0x00FFFFFFFF000000L;
      private static final long MASK_0_32  = 0x00000000FFFFFFFFL;

      private final Predictor[] predictors;
      private final SliceByteArray sba;
      private Predictor predictor;
      private long low;
      private long high;


      public ROLZEncoder(Predictor[] predictors, SliceByteArray sba)
      {
         this.low = 0L;
         this.high = TOP;
         this.sba = sba;
         this.predictors = predictors;
         this.predictor = this.predictors[0];
      }

      public void setContext(int n)
      {
         this.predictor = this.predictors[n];
      }

      public final void encodeByte(byte val)
      {
         this.encodeBit((val >> 7) & 1);
         this.encodeBit((val >> 6) & 1);
         this.encodeBit((val >> 5) & 1);
         this.encodeBit((val >> 4) & 1);
         this.encodeBit((val >> 3) & 1);
         this.encodeBit((val >> 2) & 1);
         this.encodeBit((val >> 1) & 1);
         this.encodeBit(val & 1);
      }

      public void encodeBit(int bit)
      {
         // Calculate interval split
         final long split = (((this.high-this.low) >>> 4) * this.predictor.get()) >>> 8;

         // Update fields with new interval bounds
         this.high -= (-bit & (this.high - this.low - split));
         this.low += (~-bit & -~split);

         // Update predictor
         this.predictor.update(bit);

         // Write unchanged first 32 bits to bitstream
         while (((this.low ^ this.high) & MASK_24_56) == 0)
         {
            Memory.BigEndian.writeInt32(this.sba.array, this.sba.index, (int) (this.high>>>32));
            this.sba.index += 4;
            this.low <<= 32;
            this.high = (this.high << 32) | MASK_0_32;
         }
      }

      public void dispose()
      {
         for (int i=0; i<8; i++)
         {
            this.sba.array[this.sba.index+i] = (byte) (this.low>>56);
            this.low <<= 8;
         }

         this.sba.index += 8;
      }
   }


   static class ROLZDecoder
   {
      private static final long TOP        = 0x00FFFFFFFFFFFFFFL;
      private static final long MASK_24_56 = 0x00FFFFFFFF000000L;
      private static final long MASK_0_56  = 0x00FFFFFFFFFFFFFFL;
      private static final long MASK_0_32  = 0x00000000FFFFFFFFL;

      private final Predictor[] predictors;
      private final SliceByteArray sba;
      private Predictor predictor;
      private long low;
      private long high;
      private long current;


      public ROLZDecoder(Predictor[] predictors, SliceByteArray sba)
      {
         this.low = 0L;
         this.high = TOP;
         this.sba = sba;
         this.current  = 0;

         for (int i=0; i<8; i++)
            this.current = (this.current << 8) | (long) (this.sba.array[this.sba.index+i] &0xFF);

         this.sba.index += 8;
         this.predictors = predictors;
         this.predictor = this.predictors[0];
      }

      public void setContext(int n)
      {
         this.predictor = this.predictors[n];
      }

      public byte decodeByte()
      {
         return (byte) ((this.decodeBit() << 7)
               | (this.decodeBit() << 6)
               | (this.decodeBit() << 5)
               | (this.decodeBit() << 4)
               | (this.decodeBit() << 3)
               | (this.decodeBit() << 2)
               | (this.decodeBit() << 1)
               |  this.decodeBit());
      }

      public int decodeBit()
      {
         // Calculate interval split
         final long mid = this.low + ((((this.high-this.low) >>> 4) * this.predictor.get()) >>> 8);
         int bit;

         if (mid >= this.current)
         {
            bit = 1;
            this.high = mid;
         }
         else
         {
            bit = 0;
            this.low = -~mid;
         }

          // Update predictor
         this.predictor.update(bit);

         // Read 32 bits from bitstream
         while (((this.low ^ this.high) & MASK_24_56) == 0)
         {
            this.low = (this.low << 32) & MASK_0_56;
            this.high = ((this.high << 32) | MASK_0_32) & MASK_0_56;
            final long val = Memory.BigEndian.readInt32(this.sba.array, this.sba.index) & MASK_0_32;
            this.current = ((this.current << 32) | val) & MASK_0_56;
            this.sba.index += 4;
         }

         return bit;
      }

      public void dispose()
      {
      }
   }


   static class ROLZPredictor implements Predictor
   {
      private final int[] p1;
      private final int[] p2;
      private final int size;
      private final int logSize;
      private int c1;
      private int ctx;
      
      ROLZPredictor(int logPosChecks)
      {
         this.logSize = logPosChecks;
         this.size = 1 << logPosChecks;
         this.p1 = new int[256*this.size];
         this.p2 = new int[256*this.size];
         this.reset();
      }

      private void reset()
      {
         this.c1 = 1;
         this.ctx = 0;

         for (int i=0; i<this.p1.length; i++)
         {
            this.p1[i] = 1 << 15;
            this.p2[i] = 1 << 15;
         }
      }

      void setContext(byte ctx)
      {
         this.ctx = (ctx & 0xFF) << this.logSize;
      }

      @Override
      public  void update(int bit)
      {
         final int idx = this.ctx + this.c1;
         this.p1[idx] -= (((this.p1[idx] - (-bit&0xFFFF)) >> 3) + bit);
         this.p2[idx] -= (((this.p2[idx] - (-bit&0xFFFF)) >> 6) + bit);
         this.c1 <<= 1;
         this.c1 += bit;

         if (this.c1 >= this.size)
            this.c1 = 1;
      }

      @Override
      public int get()
      {
         final int idx = this.ctx + this.c1;
         return (this.p1[idx] + this.p2[idx]) >>> 5;
      }
   }
}
