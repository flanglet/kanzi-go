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


import kanzi.OutputBitStream;


// This class is a generic implementation of a boolean entropy encoder
public class BinaryEntropyEncoder extends AbstractEncoder
{
   private static final long TOP        = 0x00FFFFFFFFFFFFFFL;
   private static final long MASK_24_56 = 0x00FFFFFFFF000000L;
   private static final long MASK_0_24  = 0x0000000000FFFFFFL;
   private static final long MASK_0_32  = 0x00000000FFFFFFFFL;
   
   private final Predictor predictor;
   private long low;
   private long high;
   private final OutputBitStream bitstream;
   private boolean disposed;


   public BinaryEntropyEncoder(OutputBitStream bitstream, Predictor predictor)
   {
      if (bitstream == null)
         throw new NullPointerException("Invalid null bitstream parameter");

      if (predictor == null)
         throw new NullPointerException("Invalid null predictor parameter");

      this.low = 0L;
      this.high = TOP;
      this.bitstream = bitstream;
      this.predictor = predictor;
   }


   @Override
   public void encodeByte(byte val)
   {
      this.encodeBit_((val >> 7) & 1);
      this.encodeBit_((val >> 6) & 1);
      this.encodeBit_((val >> 5) & 1);
      this.encodeBit_((val >> 4) & 1);
      this.encodeBit_((val >> 3) & 1);
      this.encodeBit_((val >> 2) & 1);
      this.encodeBit_((val >> 1) & 1);
      this.encodeBit_(val & 1);          
   }
   

   public boolean encodeBit(int bit)
   {
      return this.encodeBit_(bit & 1);
   }
   
   
   private boolean encodeBit_(int bit)
   {
      // Compute prediction
      final int prediction = this.predictor.get();

      // Calculate interval split
      final long xmid = this.low + ((this.high - this.low) >> 12) * prediction;

      // Update fields with new interval bounds
      final int bitmask = bit - 1;
      this.low = (bitmask & (xmid + 1)) | (~bitmask & this.low);
      this.high = (bitmask & this.high) | (~bitmask & xmid);

      // Update predictor
      this.predictor.update(bit);

      // Write unchanged first 32 bits to bitstream
      while (((this.low ^ this.high) & MASK_24_56) == 0)
         this.flush();

      return true;
   }


   protected void flush()
   {
      this.bitstream.writeBits(this.high >>> 24, 32);
      this.low <<= 32;
      this.high = (this.high << 32) | MASK_0_32;
   }


   @Override
   public OutputBitStream getBitStream()
   {
      return this.bitstream;
   }


   @Override
   public void dispose()
   {
      if (this.disposed == true)
         return;

      this.disposed = true;
      this.bitstream.writeBits(this.low | MASK_0_24, 56);
   }
}
