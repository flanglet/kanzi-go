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

package kanzi.bitstream;

import kanzi.BitStreamException;
import java.io.IOException;
import java.io.OutputStream;
import kanzi.Memory;
import kanzi.OutputBitStream;


public final class DefaultOutputBitStream implements OutputBitStream
{
   private final OutputStream os;
   private byte[] buffer;
   private boolean closed;
   private int position;  // index of current byte in buffer
   private int bitIndex;  // index of current bit to write in current
   private long written;
   private long current;  // cached bits


   public DefaultOutputBitStream(OutputStream os, int bufferSize)
   {
      if (os == null)
         throw new NullPointerException("Invalid null output stream parameter");

      if (bufferSize < 1024)
         throw new IllegalArgumentException("Invalid buffer size (must be at least 1024)");

      if (bufferSize > 1<<28)
         throw new IllegalArgumentException("Invalid buffer size (must be at most 268435456)");

      if ((bufferSize & 7) != 0)
         throw new IllegalArgumentException("Invalid buffer size (must be a multiple of 8)");

      this.os = os;
      this.buffer = new byte[bufferSize];
      this.bitIndex = 63;
   }


   // Write least significant bit of the input integer. Trigger exception if stream is closed
   @Override
   public void writeBit(int bit)
   {
      if (this.bitIndex <= 0) // bitIndex = -1 if stream is closed => force pushCurrent()
      {
         this.current |= (bit & 1);
         this.pushCurrent();
      }
      else
      {
         this.current |= ((long) (bit & 1) << this.bitIndex);
         this.bitIndex--;
      }
   }

   
   // Write 'count' (in [1..64]) bits. Trigger exception if stream is closed
   @Override
   public int writeBits(long value, int count)
   {
      if (count == 0)
         return 0;
      
      if (count > 64)
         throw new IllegalArgumentException("Invalid length: "+count+" (must be in [1..64])");
      
      value &= (-1L >>> -count);
      final int remaining = this.bitIndex + 1 - count;

      // Pad the current position in buffer
      if (remaining > 0)
      {
         // Enough spots available in 'current'
         this.current |= (value << remaining);
         this.bitIndex -= count;
      }
      else
      {
         // Not enough spots available in 'current'
         this.current |= (value >>> -remaining);
         this.pushCurrent();
         
         if (remaining != 0)
         {
            this.current = (value << remaining);
            this.bitIndex += remaining; 
         }
      }

      return count;
   }


   // Push 64 bits of current value into buffer.
   private void pushCurrent()
   {
      Memory.BigEndian.writeLong64(this.buffer, this.position, this.current);
      this.bitIndex = 63;
      this.current = 0;
      this.position += 8;

      if (this.position >= this.buffer.length)
         this.flush();
   }


   // Write buffer to underlying stream
   private void flush() throws BitStreamException
   {
      if (this.isClosed() == true)
         throw new BitStreamException("Stream closed", BitStreamException.STREAM_CLOSED);

      try
      {
         if (this.position > 0)
         {
            this.os.write(this.buffer, 0, this.position);
            this.written += (this.position << 3);
            this.position = 0;
         }
      }
      catch (IOException e)
      {
         throw new BitStreamException(e.getMessage(), BitStreamException.INPUT_OUTPUT);
      }
   }


   @Override
   public void close()
   {
      if (this.isClosed() == true)
         return;

      final int savedBitIndex = this.bitIndex;
      final int savedPosition = this.position;
      final long savedCurrent = this.current;

      try
      {
         // Push last bytes (the very last byte may be incomplete)
         final int size = ((63 - this.bitIndex) + 7) >> 3;
         this.pushCurrent();
         this.position -= (8 - size);
         this.flush();
      }
      catch (BitStreamException e)
      {
         // Revert fields to allow subsequent attempts in case of transient failure
         this.position = savedPosition;
         this.bitIndex = savedBitIndex;
         this.current = savedCurrent;
         throw e;
      }

      try
      {
         this.os.flush();
      }
      catch (IOException e)
      {
         throw new BitStreamException(e, BitStreamException.INPUT_OUTPUT);
      }

      this.closed = true;
      this.position = 0;

      // Reset fields to force a flush() and trigger an exception
      // on writeBit() or writeBits()
      this.bitIndex = -1;
      this.buffer = new byte[8];
      this.written -= 64; // adjust for method written()
   }


   // Return number of bits written so far
   @Override
   public long written()
   {
      // Number of bits flushed + bytes written in memory + bits written in memory
      return this.written + (this.position << 3) + (63 - this.bitIndex);
   }


   public boolean isClosed()
   {
      return this.closed;
   }
}