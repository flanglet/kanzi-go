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

package kanzi.io;

import kanzi.Event;
import java.io.IOException;
import java.io.OutputStream;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Future;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.locks.LockSupport;
import kanzi.BitStreamException;
import kanzi.EntropyEncoder;
import kanzi.SliceByteArray;
import kanzi.OutputBitStream;
import kanzi.bitstream.DefaultOutputBitStream;
import kanzi.entropy.EntropyCodecFactory;
import kanzi.function.ByteTransformSequence;
import kanzi.util.hash.XXHash32;
import kanzi.Listener;



// Implementation of a java.io.OutputStream that encodes a stream
// using a 2 step process:
// - step 1: a ByteFunction is used to reduce the size of the input data (bytes input & output)
// - step 2: an EntropyEncoder is used to entropy code the results of step 1 (bytes input, bits output)
public class CompressedOutputStream extends OutputStream
{
   private static final int BITSTREAM_TYPE           = 0x4B414E5A; // "KANZ"
   private static final int BITSTREAM_FORMAT_VERSION = 4;
   private static final int COPY_LENGTH_MASK         = 0x0F;
   private static final int SMALL_BLOCK_MASK         = 0x80;
   private static final int MIN_BITSTREAM_BLOCK_SIZE = 1024;
   private static final int MAX_BITSTREAM_BLOCK_SIZE = 1024*1024*1024;
   private static final int SMALL_BLOCK_SIZE         = 15;
   private static final byte[] EMPTY_BYTE_ARRAY      = new byte[0];

   private final int blockSize;
   private final XXHash32 hasher;
   private final SliceByteArray sa; // for all blocks
   private final SliceByteArray[] buffers; // input & output per block
   private final short entropyType;
   private final short transformType;
   private final OutputBitStream obs;
   private final AtomicBoolean initialized;
   private final AtomicBoolean closed;
   private final AtomicInteger blockId;
   private final int jobs;
   private final ExecutorService pool;
   private final List<Listener> listeners;
   private final Map<String, Object> ctx;


   public CompressedOutputStream(OutputStream os, Map<String, Object> ctx)
   {
      if (os == null)
         throw new NullPointerException("Invalid null output stream parameter");
            
      if (ctx == null)
         throw new NullPointerException("Invalid null context parameter");
            
      String entropyCodec = (String) ctx.get("codec");
      
      if (entropyCodec == null)
         throw new NullPointerException("Invalid null entropy encoder type parameter");

      String transform = (String) ctx.get("transform");

      if (transform == null)
         throw new NullPointerException("Invalid null transform type parameter");

      final int bSize = (Integer) ctx.get("blockSize");   
      
      if (bSize > MAX_BITSTREAM_BLOCK_SIZE)
         throw new IllegalArgumentException("The block size must be at most "+(MAX_BITSTREAM_BLOCK_SIZE>>20)+ " MB");

      if (bSize < MIN_BITSTREAM_BLOCK_SIZE)
         throw new IllegalArgumentException("The block size must be at least "+MIN_BITSTREAM_BLOCK_SIZE);

      if ((bSize & -16) != bSize)
         throw new IllegalArgumentException("The block size must be a multiple of 16");

      final int tasks = (Integer) ctx.get("jobs");
 
      if ((tasks < 0) || (tasks > 16)) // 0 indicates no user choice
         throw new IllegalArgumentException("The number of jobs must be in [1..16]");

      ExecutorService threadPool = (ExecutorService) ctx.get("pool");

      if ((tasks > 1) && (threadPool == null))
         throw new IllegalArgumentException("The thread pool cannot be null when the number of jobs is "+tasks);

      final int bufferSize = (bSize <= 65536) ? bSize : 65536;
      this.obs = new DefaultOutputBitStream(os, bufferSize);
      this.entropyType = new EntropyCodecFactory().getType(entropyCodec);
      this.transformType = new ByteFunctionFactory().getType(transform);
      this.blockSize = bSize;
      final boolean checksum = (Boolean) ctx.get("checksum");
      this.hasher = (checksum == true) ? new XXHash32(BITSTREAM_TYPE) : null;
      this.jobs = (tasks == 0) ? 1: tasks;
      this.pool = threadPool;
      this.sa = new SliceByteArray(new byte[this.blockSize*this.jobs], 0);
      this.buffers = new SliceByteArray[2*this.jobs];
      this.closed = new AtomicBoolean(false);
      this.initialized = new AtomicBoolean(false);

      for (int i=0; i<this.buffers.length; i++)
         this.buffers[i] = new SliceByteArray(EMPTY_BYTE_ARRAY, 0);

      this.blockId = new AtomicInteger(0);
      this.listeners = new ArrayList<>(10);
      this.ctx = ctx;
   }


   protected void writeHeader() throws IOException
   {
      if (this.obs.writeBits(BITSTREAM_TYPE, 32) != 32)
         throw new kanzi.io.IOException("Cannot write bitstream type to header", Error.ERR_WRITE_FILE);

      if (this.obs.writeBits(BITSTREAM_FORMAT_VERSION, 7) != 7)
         throw new kanzi.io.IOException("Cannot write bitstream version to header", Error.ERR_WRITE_FILE);

      if (this.obs.writeBits((this.hasher != null) ? 1 : 0, 1) != 1)
         throw new kanzi.io.IOException("Cannot write checksum to header", Error.ERR_WRITE_FILE);
      
      if (this.obs.writeBits(this.entropyType, 5) != 5)
         throw new kanzi.io.IOException("Cannot write entropy type to header", Error.ERR_WRITE_FILE);

      if (this.obs.writeBits(this.transformType, 16) != 16)
         throw new kanzi.io.IOException("Cannot write transform types to header", Error.ERR_WRITE_FILE);

      if (this.obs.writeBits(this.blockSize >>> 4, 26) != 26)
         throw new kanzi.io.IOException("Cannot write block size to header", Error.ERR_WRITE_FILE);

      if (this.obs.writeBits(0L, 9) != 9)
         throw new kanzi.io.IOException("Cannot write reserved bits to header", Error.ERR_WRITE_FILE);
   }


    public boolean addListener(Listener bl)
    {
       return (bl != null) ? this.listeners.add(bl) : false;
    }

   
    public boolean removeListener(Listener bl)
    {
       return (bl != null) ? this.listeners.remove(bl) : false;
    }
    

    /**
     * Writes <code>len</code> bytes from the specified byte array
     * starting at offset <code>off</code> to this output stream.
     * The general contract for <code>write(array, off, len)</code> is that
     * some of the bytes in the array <code>array</code> are written to the
     * output stream in order; element <code>array[off]</code> is the first
     * byte written and <code>array[off+len-1]</code> is the last byte written
     * by this operation.
     * <p>
     * The <code>write</code> method of <code>OutputStream</code> calls
     * the write method of one argument on each of the bytes to be
     * written out. Subclasses are encouraged to override this method and
     * provide a more efficient implementation.
     * <p>
     * If <code>array</code> is <code>null</code>, a
     * <code>NullPointerException</code> is thrown.
     * <p>
     * If <code>off</code> is negative, or <code>len</code> is negative, or
     * <code>off+len</code> is greater than the length of the array
     * <code>array</code>, then an <tt>IndexOutOfBoundsException</tt> is thrown.
     *
     * @param      data the data.
     * @param      off   the start offset in the data.
     * @param      len   the number of bytes to write.
     * @exception  IOException  if an I/O error occurs. In particular,
     *             an <code>IOException</code> is thrown if the output
     *             stream is closed.
     */
    @Override
    public void write(byte[] data, int off, int len) throws IOException
    {
      if ((off < 0) || (len < 0) || (len + off > data.length))
         throw new IndexOutOfBoundsException();

      if (this.closed.get() == true)
         throw new kanzi.io.IOException("Stream closed", Error.ERR_WRITE_FILE);

      int remaining = len;

      while (remaining > 0)
      {
         // Limit to number of available bytes in buffer
         final int lenChunk = (this.sa.index + remaining < this.sa.length) ? remaining :
                 this.sa.length - this.sa.index;

         if (lenChunk > 0)
         {
            // Process a chunk of in-buffer data. No access to bitstream required
            System.arraycopy(data, off, this.sa.array, this.sa.index, lenChunk);
            this.sa.index += lenChunk;
            off += lenChunk;
            remaining -= lenChunk;

            if (remaining == 0)
               break;
         }

         // Buffer full, time to encode
         this.write(data[off]);
         off++;
         remaining--;
      }
   }



   /**
    * Writes the specified byte to this output stream. The general
    * contract for <code>write</code> is that one byte is written
    * to the output stream. The byte to be written is the eight
    * low-order bits of the argument <code>b</code>. The 24
    * high-order bits of <code>b</code> are ignored.
    * <p>
    * Subclasses of <code>OutputStream</code> must provide an
    * implementation for this method.
    *
    * @param      b   the <code>byte</code>..
    * @throws java.io.IOException
    */
   @Override
   public void write(int b) throws IOException
   {
      try
      {
         // If the buffer is full, time to encode
         if (this.sa.index >= this.sa.length)
            this.processBlock();

         this.sa.array[this.sa.index++] = (byte) b;
      }
      catch (BitStreamException e)
      {
         throw new kanzi.io.IOException(e.getMessage(), Error.ERR_READ_FILE);
      }
      catch (kanzi.io.IOException e)
      {
         throw e;
      }
      catch (ArrayIndexOutOfBoundsException e)
      {
         // Happens only if the stream is closed
         throw new kanzi.io.IOException("Stream closed", Error.ERR_READ_FILE);
      }
      catch (Exception e)
      {
         throw new kanzi.io.IOException(e.getMessage(), Error.ERR_UNKNOWN);
      }
   }


   /**
    * Flushes this output stream and forces any buffered output bytes
    * to be written out. The general contract of <code>flush</code> is
    * that calling it is an indication that, if any bytes previously
    * written have been buffered by the implementation of the output
    * stream, such bytes should immediately be written to their
    * intended destination.
    * <p>
    * If the intended destination of this stream is an abstraction provided by
    * the underlying operating system, for example a file, then flushing the
    * stream guarantees only that bytes previously written to the stream are
    * passed to the operating system for writing; it does not guarantee that
    * they are actually written to a physical device such as a disk drive.
    * <p>
    * The <code>flush</code> method of <code>OutputStream</code> does nothing.
    *
    */
   @Override
   public void flush()
   {
      // Let the bitstream of the entropy encoder flush itself when needed
   }


   /**
    * Closes this output stream and releases any system resources
    * associated with this stream. The general contract of <code>close</code>
    * is that it closes the output stream. A closed stream cannot perform
    * output operations and cannot be reopened.
    * <p>
    *
    * @exception  IOException  if an I/O error occurs.
    */
   @Override
   public void close() throws IOException
   {
      if (this.closed.getAndSet(true) == true)
         return;

      if (this.sa.index > 0)
         this.processBlock();

      try
      {
         // Write end block of size 0
         this.obs.writeBits(SMALL_BLOCK_MASK, 8);
         this.obs.close();
      }
      catch (BitStreamException e)
      {
         throw new kanzi.io.IOException(e.getMessage(), e.getErrorCode());
      }

      this.listeners.clear();

      // Release resources
      // Force error on any subsequent write attempt
      this.sa.array = EMPTY_BYTE_ARRAY;
      this.sa.length = 0;
      this.sa.index = -1;

      for (int i=0; i<this.buffers.length; i++)
         this.buffers[i] = new SliceByteArray(EMPTY_BYTE_ARRAY, 0);
   }

   
   private void processBlock() throws IOException
   {
      if (this.sa.index == 0)
         return;

      if (this.initialized.getAndSet(true) == false)
         this.writeHeader();

      try
      {
         // Protect against future concurrent modification of the list of block listeners         
         Listener[] blockListeners = this.listeners.toArray(new Listener[this.listeners.size()]);
         final int dataLength = this.sa.index;
         this.sa.index = 0;
         List<Callable<Status>> tasks = new ArrayList<>(this.jobs);
         int firstBlockId = this.blockId.get();

         // Create as many tasks as required
         for (int jobId=0; jobId<this.jobs; jobId++)
         {
            final int sz = (this.sa.index + this.blockSize > dataLength) ?
                    dataLength - this.sa.index : this.blockSize;
            
            if (sz == 0)
               break;
            
            this.buffers[2*jobId].index = 0;
            this.buffers[2*jobId+1].index = 0;
            
            if (this.buffers[2*jobId].array.length < sz)
            {
               this.buffers[2*jobId].array = new byte[sz];
               this.buffers[2*jobId].length = sz;
            }
            
            System.arraycopy(this.sa.array, this.sa.index, this.buffers[2*jobId].array, 0, sz);
            
            Callable<Status> task = new EncodingTask(this.buffers[2*jobId],
                    this.buffers[2*jobId+1], sz, this.transformType,
                    this.entropyType, firstBlockId+jobId+1,
                    this.obs, this.hasher, this.blockId,
                    blockListeners, new HashMap<>(this.ctx));
            tasks.add(task);
            this.sa.index += sz;
         }

         if (this.jobs == 1)
         {
            // Synchronous call
            Status status = tasks.get(0).call();
            
            if (status.error != 0)
               throw new kanzi.io.IOException(status.msg, status.error);
         }
         else
         {
            // Invoke the tasks concurrently and validate the results
            for (Future<Status> result : this.pool.invokeAll(tasks))
            {
               // Wait for completion of next task and validate result
               Status status = result.get();

               if (status.error != 0)
                  throw new kanzi.io.IOException(status.msg, status.error);
            }
         }

         this.sa.index = 0;
      }
      catch (kanzi.io.IOException e)
      {
         throw e;
      }
      catch (Exception e)
      {
         int errorCode = (e instanceof BitStreamException) ? ((BitStreamException) e).getErrorCode() :
                 Error.ERR_UNKNOWN;
         throw new kanzi.io.IOException(e.getMessage(), errorCode);
      }
   }


   // Return the number of bytes written so far
   public long getWritten()
   {
      return (this.obs.written() + 7) >> 3;
   }

   
   static void notifyListeners(Listener[] listeners, Event evt)
   {
      for (Listener bl : listeners)
      {
         try 
         {
            bl.processEvent(evt);
         }
         catch (Exception e)
         {
            // Ignore exceptions in block listeners
         }
      }
   }
      
   
   // A task used to encode a block
   // Several tasks may run in parallel. The transforms can be computed concurrently
   // but the entropy encoding is sequential since all tasks share the same bitstream.
   static class EncodingTask implements Callable<Status>
   {
      private final SliceByteArray data;
      private final SliceByteArray buffer;
      private final int length;
      private final short transformType;
      private final short entropyType;
      private final int blockId;
      private final OutputBitStream obs;
      private final XXHash32 hasher;
      private final AtomicInteger processedBlockId;
      private final Listener[] listeners;
      private final Map<String, Object> ctx;


      EncodingTask(SliceByteArray iBuffer, SliceByteArray oBuffer, int length,
              short transformType, short entropyType, int blockId,
              OutputBitStream obs, XXHash32 hasher,
              AtomicInteger processedBlockId, Listener[] listeners,
              Map<String, Object> ctx)
      {
         this.data = iBuffer;
         this.buffer = oBuffer;
         this.length = length;
         this.transformType = transformType;
         this.entropyType = entropyType;
         this.blockId = blockId;
         this.obs = obs;
         this.hasher = hasher;
         this.processedBlockId = processedBlockId;
         this.listeners = listeners;
         this.ctx = ctx;
      }


      @Override
      public Status call() throws Exception
      {
         return this.encodeBlock(this.data, this.buffer, this.length,
                 this.transformType, this.entropyType, this.blockId);
      }


      // Encode mode + transformed entropy coded data
      // mode: 0b1000xxxx => small block (written as is) + 4 LSB for block size (0-15)
      //       0x00xxxx00 => transform sequence skip flags (1 means skip)
      //       0x000000xx => size(size(block))-1
      private Status encodeBlock(SliceByteArray data, SliceByteArray buffer,
           int blockLength, short typeOfTransform,
           short typeOfEntropy, int currentBlockId)
      {
         EntropyEncoder ee = null;

         try
         {
            byte mode = 0;
            int dataSize = 0;
            int postTransformLength = blockLength;
            int checksum = 0;

            // Compute block checksum
            if (this.hasher != null)
               checksum = this.hasher.hash(data.array, data.index, blockLength);

            if (this.listeners.length > 0)
            {
               // Notify before transform               
               Event evt = new Event(Event.Type.BEFORE_TRANSFORM, currentBlockId,
                       blockLength, checksum, this.hasher != null);
               
               notifyListeners(this.listeners, evt);
            }
            
            if (blockLength <= SMALL_BLOCK_SIZE)
            {
               // Just copy
               if (data.array != buffer.array)
               {
                  if (buffer.length < blockLength)
                  {
                     buffer.length = blockLength;
                     
                     if (buffer.array.length < buffer.length)
                        buffer.array = new byte[buffer.length];
                  }
               
                  System.arraycopy(data.array, data.index, buffer.array, 0, blockLength);
               }
               
               data.index += blockLength;
               buffer.index = blockLength;
               mode = (byte) (SMALL_BLOCK_MASK | (blockLength & COPY_LENGTH_MASK));
            }
            else
            {
               this.ctx.put("size", blockLength);
               ByteTransformSequence transform = new ByteFunctionFactory().newFunction(this.ctx, typeOfTransform);               
               int requiredSize = transform.getMaxEncodedLength(blockLength);

               if (buffer.length < requiredSize)
               {
                  buffer.length = requiredSize;
                  
                  if (buffer.array.length < buffer.length)
                      buffer.array = new byte[buffer.length];
               }
               
               // Forward transform (ignore error, encode skipFlags)
               buffer.index = 0;
               data.length = blockLength;
               transform.forward(data, buffer);
               mode |= ((transform.getSkipFlags() & ByteTransformSequence.SKIP_MASK) << 2);                                   
               postTransformLength = buffer.index;

               if (postTransformLength < 0)
                  return new Status(currentBlockId, Error.ERR_WRITE_FILE, "Invalid transform size");

               for (long n=0xFF; n<postTransformLength; n<<=8)
                  dataSize++;

               if (dataSize > 3) 
                  return new Status(currentBlockId, Error.ERR_WRITE_FILE, "Invalid block data length");
               
               // Record size of 'block size' - 1 in bytes
               mode |= (dataSize & 0x03);               
               dataSize++;
            }

            if (this.listeners.length > 0)
            {
               // Notify after transform
               Event evt = new Event(Event.Type.AFTER_TRANSFORM, currentBlockId,
                       postTransformLength, checksum, this.hasher != null);
               
               notifyListeners(this.listeners, evt);
            }

            // Lock free synchronization
            while (this.processedBlockId.get() != currentBlockId-1)
            {
               // Wait for the concurrent task processing the previous block to complete
               // entropy encoding. Entropy encoding must happen sequentially (and
               // in the correct block order) in the bitstream.
               // Backoff improves performance in heavy contention scenarios
               LockSupport.parkNanos(10);
            }

            // Write block 'header' (mode + compressed length);
            final long written = this.obs.written();
            this.obs.writeBits(mode, 8);

            if (dataSize > 0)
               this.obs.writeBits(postTransformLength, 8*dataSize);

            // Write checksum
            if (this.hasher != null)
               this.obs.writeBits(checksum, 32);

            if (this.listeners.length > 0)
            {
               // Notify before entropy
               Event evt = new Event(Event.Type.BEFORE_ENTROPY, currentBlockId,
                       postTransformLength, checksum, this.hasher != null);
               
               notifyListeners(this.listeners, evt);
            }
   
            // Each block is encoded separately
            // Rebuild the entropy encoder to reset block statistics
            ee = new EntropyCodecFactory().newEncoder(this.obs, this.ctx, typeOfEntropy);

            // Entropy encode block
            if (ee.encode(buffer.array, 0, postTransformLength) != postTransformLength)
               return new Status(currentBlockId, Error.ERR_PROCESS_BLOCK, "Entropy coding failed");

            // Dispose before displaying statistics. Dispose may write to the bitstream
            ee.dispose();

            // Force ee to null to avoid double dispose (in the finally section)
            ee = null;

            final int w = (int) ((this.obs.written() - written) / 8L);
            
            // After completion of the entropy coding, increment the block id.
            // It unfreezes the task processing the next block (if any)
            this.processedBlockId.incrementAndGet();

            if (this.listeners.length > 0)
            {
               // Notify after entropy
               Event evt = new Event(Event.Type.AFTER_ENTROPY, 
                       currentBlockId, w, checksum, this.hasher != null);
               
               notifyListeners(this.listeners, evt);
            }

            return new Status(currentBlockId, 0, "Success");
         }
         catch (Exception e)
         {
            return new Status(currentBlockId, Error.ERR_PROCESS_BLOCK, e.getMessage());
         }
         finally
         {
            // Make sure to unfreeze next block
            if (this.processedBlockId.get() == this.blockId-1)
               this.processedBlockId.incrementAndGet();
            
            if (ee != null)
              ee.dispose();
         }
      }     
   }

   
   static class Status
   {
      final int blockId;
      final int error; // 0 = OK
      final String msg;
      
      Status(int blockId, int error, String msg)
      {
         this.blockId = blockId;
         this.error = error;
         this.msg = msg;
      }
   }
}
