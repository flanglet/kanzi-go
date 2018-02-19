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

import java.io.IOException;
import java.io.InputStream;
import kanzi.Memory;
import kanzi.BitStreamException;
import kanzi.InputBitStream;


public final class DefaultInputBitStream implements InputBitStream
{
   private final InputStream is;
   private final byte[] buffer;
   private int position;  // index of current byte (consumed if bitIndex == 63)
   private int bitIndex;  // index of current bit to read
   private long read;
   private boolean closed;
   private int maxPosition;
   private long current;


   public DefaultInputBitStream(InputStream is, int bufferSize)
   {
      if (is == null)
         throw new NullPointerException("Invalid null input stream parameter");

      if (bufferSize < 1024)
         throw new IllegalArgumentException("Invalid buffer size (must be at least 1024)");

      if (bufferSize > 1<<28)
         throw new IllegalArgumentException("Invalid buffer size (must be at most 268435456)");

      if ((bufferSize & 7) != 0)
         throw new IllegalArgumentException("Invalid buffer size (must be a multiple of 8)");

      this.is = is;
      this.buffer = new byte[bufferSize];
      this.bitIndex = 63;
      this.maxPosition = -1;
   }


   // Return 1 or 0. Trigger exception if stream is closed   
   @Override
   public int readBit() throws BitStreamException
   {
      if (this.bitIndex == 63)
         this.pullCurrent(); // Triggers an exception if stream is closed

      final int bit = (int) ((this.current >> this.bitIndex) & 1);
      this.bitIndex = (this.bitIndex - 1) & 63;
      return bit;
   }


   private int readFromInputStream(int count) throws BitStreamException
   {
      if (this.isClosed() == true)
         throw new BitStreamException("Stream closed", BitStreamException.STREAM_CLOSED);

      int size = -1;
      
      try
      {
         this.read += (((long) this.maxPosition+1) << 3);
         size = this.is.read(this.buffer, 0, count);

         if (size <= 0)
         {
            throw new BitStreamException("No more data to read in the bitstream",
                    BitStreamException.END_OF_STREAM);
         }

         return size;
      }
      catch (IOException e)
      {
         throw new BitStreamException(e.getMessage(), BitStreamException.INPUT_OUTPUT);
      }
      finally 
      {
         this.position = 0;
         this.maxPosition = (size <= 0) ? -1 : size - 1;
      }
   }


   // Return value of 'count' next bits as a long. Trigger exception if stream is closed   
   @Override
   public long readBits(int count) throws BitStreamException
   {
      if (((count-1) & -64) != 0)
         throw new IllegalArgumentException("Invalid length: "+count+" (must be in [1..64])");

      long res;
      int remaining = count - this.bitIndex - 1;

      if (remaining <= 0)
      {         
         // Enough spots available in 'current'     
         if (this.bitIndex == 63)
         {
            this.pullCurrent();
            remaining -= (this.bitIndex - 63); // adjust if bitIndex != 63 (end of stream)
         }
         
         res = (this.current >>> -remaining) & (-1L >>> -count);
         this.bitIndex = (this.bitIndex - count) & 63;
      }
      else
      {
         // Not enough spots available in 'current'
         res = this.current & (-1L >>> (63 - this.bitIndex));
         this.pullCurrent();
         res <<= remaining;
         this.bitIndex -= remaining;
         res |= (this.current >>> (this.bitIndex + 1));
      }

      return res;
   }

   
   // Pull 64 bits of current value from buffer.
   private void pullCurrent()
   {
       if (this.position > this.maxPosition)
          this.readFromInputStream(this.buffer.length);   
       
       long val;

       if (this.position + 7 > this.maxPosition) 
       {
          // End of stream: overshoot max position => adjust bit index
          int shift = (this.maxPosition - this.position) << 3;
          this.bitIndex = shift + 7;
          val = 0;
          
          while (this.position <= this.maxPosition)
          {
             val |= (((long) (this.buffer[this.position++] & 0xFF)) << shift);
             shift -= 8;
          }
       }
       else
       {
          // Regular processing, buffer length is multiple of 8
          val = Memory.BigEndian.readLong64(this.buffer, this.position);
          this.bitIndex = 63;
          this.position += 8;
       }

       this.current = val;
   }

   
   @Override
   public void close()
   {
      if (this.isClosed() == true)
         return;

      this.closed = true;
      this.read += 63;

      // Reset fields to force a readFromInputStream() and trigger an exception
      // on readBit() or readBits()
      this.bitIndex = 63;
      this.maxPosition = -1;
   }


   // Return number of bits read so far
   @Override
   public long read()
   {
      return this.read + (this.position << 3) - this.bitIndex;
   }


   @Override
   public boolean hasMoreToRead()
   {
      if (this.isClosed() == true)
         return false;

      if ((this.position < this.maxPosition) || (this.bitIndex != 63))
         return true;

      try
      {
         this.readFromInputStream(this.buffer.length);
      }
      catch (BitStreamException e)
      {
         return false;
      }

      return true;
   }
   
   
   public boolean isClosed()
   {
      return this.closed;
   }   
}
