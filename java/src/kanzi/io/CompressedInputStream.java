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

import java.io.IOException;
import java.io.InputStream;
import java.io.PrintStream;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Future;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.locks.LockSupport;
import kanzi.BitStreamException;
import kanzi.EntropyDecoder;
import kanzi.SliceByteArray;
import kanzi.InputBitStream;
import kanzi.bitstream.DefaultInputBitStream;
import kanzi.entropy.EntropyCodecFactory;
import kanzi.function.ByteTransformSequence;
import kanzi.util.hash.XXHash32;


// Implementation of a java.io.InputStream that can decode a stream
// compressed with CompressedOutputStream
public class CompressedInputStream extends InputStream
{
   private static final int BITSTREAM_TYPE           = 0x4B414E5A; // "KANZ"
   private static final int BITSTREAM_FORMAT_VERSION = 3;
   private static final int DEFAULT_BUFFER_SIZE      = 1024*1024;
   private static final int EXTRA_BUFFER_SIZE        = 256;
   private static final int COPY_LENGTH_MASK         = 0x0F;
   private static final int SMALL_BLOCK_MASK         = 0x80;
   private static final int MIN_BITSTREAM_BLOCK_SIZE = 1024;
   private static final int MAX_BITSTREAM_BLOCK_SIZE = 1024*1024*1024;
   private static final byte[] EMPTY_BYTE_ARRAY      = new byte[0];
   private static final int CANCEL_TASKS_ID          = -1;

   private int blockSize;
   private XXHash32 hasher;
   private final SliceByteArray sa; // for all blocks
   private final SliceByteArray[] buffers; // per block
   private short entropyType;
   private short transformType;
   private final InputBitStream ibs;
   private final PrintStream ds;
   private final AtomicBoolean initialized;
   private final AtomicBoolean closed;
   private int maxIdx;
   private final AtomicInteger blockId;
   private final int jobs;
   private final ExecutorService pool;
   private final List<BlockListener> listeners;


   public CompressedInputStream(InputStream is)
   {
      this(is, null);
   }


   // debug print stream is optional (may be null)
   public CompressedInputStream(InputStream is, PrintStream debug)
   {
      this(is, debug, null, 1);
   }


   // debug print stream is optional (may be null)
   public CompressedInputStream(InputStream is, PrintStream debug,
               ExecutorService pool, int jobs)
   {
      if (is == null)
         throw new NullPointerException("Invalid null input stream parameter");

      if ((jobs < 1) || (jobs > 16))
         throw new IllegalArgumentException("The number of jobs must be in [1..16]");

      if ((jobs != 1) && (pool == null))
         throw new IllegalArgumentException("The thread pool cannot be null when the number of jobs is "+jobs);

      this.ibs = new DefaultInputBitStream(is, DEFAULT_BUFFER_SIZE);
      this.sa = new SliceByteArray();
      this.jobs = jobs;
      this.pool = pool;
      this.buffers = new SliceByteArray[this.jobs];
      this.closed = new AtomicBoolean(false);
      this.initialized = new AtomicBoolean(false);

      for (int i=0; i<this.jobs; i++)
         this.buffers[i] = new SliceByteArray(EMPTY_BYTE_ARRAY, 0);

      this.ds = debug;
      this.blockId = new AtomicInteger(0);
      this.listeners = new ArrayList<BlockListener>(10);
   }


   protected void readHeader() throws IOException
   {
      // Read stream type
      final int type = (int) this.ibs.readBits(32);

      // Sanity check
      if (type != BITSTREAM_TYPE)
         throw new kanzi.io.IOException("Invalid stream type: expected "
                 + Integer.toHexString(BITSTREAM_TYPE) + ", got "
                 + Integer.toHexString(type), Error.ERR_INVALID_FILE);

      // Read stream version
      final int version = (int) this.ibs.readBits(7);

      // Sanity check
      if (version != BITSTREAM_FORMAT_VERSION)
         throw new kanzi.io.IOException("Invalid bitstream, cannot read this version of the stream: " + version,
                 Error.ERR_STREAM_VERSION);

      // Read block checksum
      if (this.ibs.readBit() == 1)
         this.hasher = new XXHash32(BITSTREAM_TYPE);

      // Read entropy codec
      this.entropyType = (short) this.ibs.readBits(5);

      // Read transform
      this.transformType = (short) this.ibs.readBits(16);

      // Read block size
      this.blockSize = (int) this.ibs.readBits(26) << 4;

      if ((this.blockSize < MIN_BITSTREAM_BLOCK_SIZE) || (this.blockSize > MAX_BITSTREAM_BLOCK_SIZE))
         throw new kanzi.io.IOException("Invalid bitstream, incorrect block size: " + this.blockSize,
                 Error.ERR_BLOCK_SIZE);

      // Read reserved bits
      this.ibs.readBits(9);

      if (this.ds != null)
      {
         this.ds.println("Checksum set to " + (this.hasher != null));
         this.ds.println("Block size set to " + this.blockSize + " bytes");

         try
         {
            String w1 = new ByteFunctionFactory().getName(this.transformType);

            if ("NONE".equals(w1))
               w1 = "no";

            this.ds.println("Using " + w1 + " transform (stage 1)");
         }
         catch (IllegalArgumentException e)
         {
            throw new kanzi.io.IOException("Invalid bitstream, unknown transform type: "+
                    this.transformType, Error.ERR_INVALID_CODEC);
         }

        try
         {
            String w2 = new EntropyCodecFactory().getName(this.entropyType);

            if ("NONE".equals(w2))
               w2 = "no";

            this.ds.println("Using " + w2 + " entropy codec (stage 2)");
         }
         catch (IllegalArgumentException e)
         {
            throw new kanzi.io.IOException("Invalid bitstream, unknown entropy codec type: "+
                    this.entropyType , Error.ERR_INVALID_CODEC);
         }
      }
   }


   public boolean addListener(BlockListener bl)
   {
      return (bl != null) ? this.listeners.add(bl) : false;
   }


   public boolean removeListener(BlockListener bl)
   {
      return (bl != null) ? this.listeners.remove(bl) : false;
   }


   /**
    * Reads the next byte of data from the input stream. The value byte is
    * returned as an <code>int</code> in the range <code>0</code> to
    * <code>255</code>. If no byte is available because the end of the stream
    * has been reached, the value <code>-1</code> is returned. This method
    * blocks until input data is available, the end of the stream is detected,
    * or an exception is thrown.
    *
    * @return     the next byte of data, or <code>-1</code> if the end of the
    *             stream is reached.
    * @exception  IOException  if an I/O error occurs.
    */
   @Override
   public int read() throws IOException
   {
      try
      {
         if (this.sa.index >= this.maxIdx)
         {
            this.maxIdx = this.processBlock();

            if (this.maxIdx == 0) // Reached end of stream
               return -1;
         }

         return this.sa.array[this.sa.index++] & 0xFF;
      }
      catch (kanzi.io.IOException e)
      {
         throw e;
      }
      catch (BitStreamException e)
      {
         throw new kanzi.io.IOException(e.getMessage(), Error.ERR_READ_FILE);
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
     * Reads some number of bytes from the input stream and stores them into
     * the buffer array <code>array</code>. The number of bytes actually read is
     * returned as an integer.  This method blocks until input data is
     * available, end of file is detected, or an exception is thrown.
     *
     * <p> If the length of <code>array</code> is zero, then no bytes are read and
     * <code>0</code> is returned; otherwise, there is an attempt to read at
     * least one byte. If no byte is available because the stream is at the
     * end of the file, the value <code>-1</code> is returned; otherwise, at
     * least one byte is read and stored into <code>array</code>.
     *
     * <p> The first byte read is stored into element <code>array[0]</code>, the
     * next one into <code>array[1]</code>, and so on. The number of bytes read is,
     * at most, equal to the length of <code>array</code>. Let <i>k</i> be the
     * number of bytes actually read; these bytes will be stored in elements
     * <code>array[0]</code> through <code>array[</code><i>k</i><code>-1]</code>,
     * leaving elements <code>array[</code><i>k</i><code>]</code> through
     * <code>array[array.length-1]</code> unaffected.
     *
     * <p> The <code>read(array)</code> method for class <code>InputStream</code>
     * has the same effect as: <pre><code> read(b, 0, array.length) </code></pre>
     *
     * @param      data   the buffer into which the data is read.
     * @return     the total number of bytes read into the buffer, or
     *             <code>-1</code> if there is no more data because the end of
     *             the stream has been reached.
     * @exception  IOException  If the first byte cannot be read for any reason
     * other than the end of the file, if the input stream has been closed, or
     * if some other I/O error occurs.
     * @exception  NullPointerException  if <code>array</code> is <code>null</code>.
     * @see        java.io.InputStream#read(byte[], int, int)
     */
   @Override
   public int read(byte[] data, int off, int len) throws IOException
   {
      if ((off < 0) || (len < 0) || (len + off > data.length))
         throw new IndexOutOfBoundsException();

      if (this.closed.get() == true)
         throw new kanzi.io.IOException("Stream closed", Error.ERR_READ_FILE);

      int remaining = len;

      while (remaining > 0)
      {
         // Limit to number of available bytes in buffer
         final int lenChunk = (this.sa.index + remaining < this.maxIdx) ? remaining :
                 this.maxIdx - this.sa.index;

         if (lenChunk > 0)
         {
            // Process a chunk of in-buffer data. No access to bitstream required
            System.arraycopy(this.sa.array, this.sa.index, data, off, lenChunk);
            this.sa.index += lenChunk;
            off += lenChunk;
            remaining -= lenChunk;

            if (remaining == 0)
               break;
         }

         // Buffer empty, time to decode
         int c2 = this.read();

         // EOF ?
         if (c2 == -1)
            break;

         data[off++] = (byte) c2;
         remaining--;
      }

      return len - remaining;
   }


   private int processBlock() throws IOException
   {
      if (this.initialized.getAndSet(true)== false)
         this.readHeader();

      try
      {
         // Add a padding area to manage any block with header (of size <= EXTRA_BUFFER_SIZE)
         final int blkSize = this.blockSize + EXTRA_BUFFER_SIZE;

         if (this.sa.length < this.jobs*blkSize)
         {
			 this.sa.length = this.jobs * blkSize;

			 if (this.sa.array.length < this.sa.length)
                this.sa.array = new byte[this.sa.length];
         }

		 // Protect against future concurrent modification of the list of block listeners
         BlockListener[] blockListeners = this.listeners.toArray(new BlockListener[this.listeners.size()]);
         int decoded = 0;
         this.sa.index = 0;
         List<Callable<Status>> tasks = new ArrayList<Callable<Status>>(this.jobs);
         int firstBlockId = this.blockId.get();

         // Create as many tasks as required
         for (int jobId=0; jobId<this.jobs; jobId++)
         {
            this.buffers[jobId].index = 0;
            
            Callable<Status> task = new DecodingTask(this.sa.array, this.sa.index,
                    this.buffers[jobId], blkSize, this.transformType,
                    this.entropyType, firstBlockId+jobId+1,
                    this.ibs, this.hasher, this.blockId,
                    blockListeners);
            tasks.add(task);
            this.sa.index += blkSize;
         }

         Status[] results = new Status[this.jobs];

         if (this.jobs == 1)
         {
            // Synchronous call
            Status status = tasks.get(0).call();
            results[status.blockId-firstBlockId-1] = status;
            decoded += status.decoded;

            if (status.error != 0)
               throw new kanzi.io.IOException(status.msg, status.error);
         }
         else
         {
            // Invoke the tasks concurrently and validate the results
            for (Future<Status> result : this.pool.invokeAll(tasks))
            {
               Status status = result.get();
               results[status.blockId-firstBlockId-1] = status;
               decoded += status.decoded;

               if (status.error != 0)
                  throw new kanzi.io.IOException(status.msg, status.error);
            }
         }

         for (Status res : results)
         {
            // Notify after transform ... in block order
            BlockEvent evt = new BlockEvent(BlockEvent.Type.AFTER_TRANSFORM, res.blockId,
                    res.decoded, res.checksum, this.hasher != null);

            notifyListeners(blockListeners, evt);
         }

         this.sa.index = 0;
         return decoded;
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


   /**
    * Closes this input stream and releases any system resources associated
    * with the stream.
    *
    * @exception  IOException  if an I/O error occurs.
    */
   @Override
   public void close() throws IOException
   {
      if (this.closed.getAndSet(true)== true)
         return;

      try
      {
         this.ibs.close();
      }
      catch (BitStreamException e)
      {
         throw new kanzi.io.IOException(e.getMessage(), e.getErrorCode());
      }

      // Release resources
      // Force error on any subsequent write attempt
      this.maxIdx = 0;
      this.sa.array = EMPTY_BYTE_ARRAY;
      this.sa.length = 0;
      this.sa.index = -1;

      for (int i=0; i<this.jobs; i++)
         this.buffers[i] = new SliceByteArray(EMPTY_BYTE_ARRAY, 0);
   }


   // Return the number of bytes read so far
   public long getRead()
   {
      return (this.ibs.read() + 7) >> 3;
   }


   static void notifyListeners(BlockListener[] listeners, BlockEvent evt)
   {
      for (BlockListener bl : listeners)
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


   // A task used to decode a block
   // Several tasks may run in parallel. The transforms can be computed concurrently
   // but the entropy decoding is sequential since all tasks share the same bitstream.
   static class DecodingTask implements Callable<Status>
   {
      private final SliceByteArray data;
      private final SliceByteArray buffer;
      private final int blockSize;
      private final short transformType;
      private final short entropyType;
      private final int blockId;
      private final InputBitStream ibs;
      private final XXHash32 hasher;
      private final AtomicInteger processedBlockId;
      private final BlockListener[] listeners;


      DecodingTask(byte[] data, int offset, SliceByteArray buffer, int blockSize,
              short transformType, short entropyType, int blockId,
              InputBitStream ibs, XXHash32 hasher,
              AtomicInteger processedBlockId, BlockListener[] listeners)
      {
         this.data = new SliceByteArray(data, offset);
         this.buffer = buffer;
         this.blockSize = blockSize;
         this.transformType = transformType;
         this.entropyType = entropyType;
         this.blockId = blockId;
         this.ibs = ibs;
         this.hasher = hasher;
         this.processedBlockId = processedBlockId;
         this.listeners = listeners;
      }


      @Override
      public Status call() throws Exception
      {
         return this.decodeBlock(this.data, this.buffer,
                 this.transformType, this.entropyType, this.blockId);
      }


      // Decode mode + transformed entropy coded data
      // mode: 0b1000xxxx => small block (written as is) + 4 LSB for block size (0-15)
      //       0x00xxxx00 => transform sequence skip flags (1 means skip)
      //       0x000000xx => size(size(block))-1
      // Return -1 if error, otherwise the number of bytes read from the encoder
      private Status decodeBlock(SliceByteArray data, SliceByteArray buffer,
         short typeOfTransform, short typeOfEntropy, int currentBlockId)
      {
         int taskId = this.processedBlockId.get();

         // Lock free synchronization
         while ((taskId != CANCEL_TASKS_ID) && (taskId != currentBlockId-1))
         {
            // Wait for the concurrent task processing the previous block to complete
            // entropy decoding. Entropy decoding must happen sequentially (and
            // in the correct block order) in the bitstream.
            // Backoff improves performance in heavy contention scenarios
            LockSupport.parkNanos(10);
            taskId = this.processedBlockId.get();
         }

         int checksum1 = 0;

         // Skip, either all data have been processed or an error occured
         if (taskId == CANCEL_TASKS_ID)
            return new Status(currentBlockId,checksum1, 0, 0, null);

         EntropyDecoder ed = null;

         try
         {
            // Extract block header directly from bitstream
            final long read = this.ibs.read();
            byte mode = (byte) this.ibs.readBits(8);
            int preTransformLength;

            if ((mode & SMALL_BLOCK_MASK) != 0)
            {
               preTransformLength = mode & COPY_LENGTH_MASK;
            }
            else
            {
               final int dataSize = 1 + (mode & 0x03);
               final int length = dataSize << 3;
               final long mask = (1L << length) - 1;
               preTransformLength = (int) (this.ibs.readBits(length) & mask);
            }

            if (preTransformLength == 0)
            {
               // Last block is empty, return success and cancel pending tasks
               this.processedBlockId.set(CANCEL_TASKS_ID);
               return new Status(currentBlockId, 0, checksum1, 0, null);
            }

            if ((preTransformLength < 0) || (preTransformLength > MAX_BITSTREAM_BLOCK_SIZE))
            {
               // Error => cancel concurrent decoding tasks
               this.processedBlockId.set(CANCEL_TASKS_ID);
               return new Status(currentBlockId, 0, checksum1, Error.ERR_READ_FILE,
                    "Invalid compressed block length: " + preTransformLength);
            }

            // Extract checksum from bit stream (if any)
            if (this.hasher != null)
               checksum1 = (int) this.ibs.readBits(32);

            if (this.listeners.length > 0)
            {
               // Notify before entropy (block size in bitstream is unknown)
               BlockEvent evt = new BlockEvent(BlockEvent.Type.BEFORE_ENTROPY, currentBlockId,
                       -1, checksum1, this.hasher != null);

               notifyListeners(this.listeners, evt);
            }

            final int bufferSize = (this.blockSize >= preTransformLength + EXTRA_BUFFER_SIZE) ?
               this.blockSize : preTransformLength + EXTRA_BUFFER_SIZE;

            if (buffer.length < bufferSize)
            {
               buffer.length = bufferSize;
               
               if (buffer.array.length < buffer.length)
                  buffer.array = new byte[buffer.length];
            }
            
            final int savedIdx = data.index;

            // Each block is decoded separately
            // Rebuild the entropy decoder to reset block statistics
            ed = new EntropyCodecFactory().newDecoder(this.ibs, typeOfEntropy);

            // Block entropy decode
            if (ed.decode(buffer.array, 0, preTransformLength) != preTransformLength)
            {
               // Error => cancel concurrent decoding tasks
               this.processedBlockId.set(CANCEL_TASKS_ID);
               return new Status(currentBlockId, 0, checksum1, Error.ERR_PROCESS_BLOCK,
                  "Entropy decoding failed");
            }

            if (this.listeners.length > 0)
            {
               // Notify after entropy (block size set to size in bitstream)
               BlockEvent evt = new BlockEvent(BlockEvent.Type.AFTER_ENTROPY, currentBlockId,
                       (int) ((this.ibs.read()-read)/8L), checksum1, this.hasher != null);

               notifyListeners(this.listeners, evt);
            }

            // After completion of the entropy decoding, increment the block id.
            // It unfreezes the task processing the next block (if any)
            this.processedBlockId.incrementAndGet();

            if (this.listeners.length > 0)
            {
               // Notify before transform (block size after entropy decoding)
               BlockEvent evt = new BlockEvent(BlockEvent.Type.BEFORE_TRANSFORM, currentBlockId,
                       preTransformLength, checksum1, this.hasher != null);

               notifyListeners(this.listeners, evt);
            }

            if ((mode & SMALL_BLOCK_MASK) != 0)
            {
               if (buffer.array != data.array)
                  System.arraycopy(buffer.array, 0, data.array, savedIdx, preTransformLength);

               buffer.index = preTransformLength;
               data.index = savedIdx + preTransformLength;
            }
            else
            {
               ByteTransformSequence transform = new ByteFunctionFactory().newFunction(preTransformLength,
                       typeOfTransform);
               transform.setSkipFlags((byte) ((mode>>2) & ByteTransformSequence.SKIP_MASK));
               buffer.index = 0;

               // Inverse transform
               buffer.length = preTransformLength;

               if (transform.inverse(buffer, data) == false)
                  return new Status(currentBlockId, 0, checksum1, Error.ERR_PROCESS_BLOCK,
                     "Transform inverse failed");
            }

            final int decoded = data.index - savedIdx;

            // Verify checksum
            if (this.hasher != null)
            {
               final int checksum2 = this.hasher.hash(data.array, savedIdx, decoded);

               if (checksum2 != checksum1)
                  return new Status(currentBlockId, decoded, checksum1, Error.ERR_PROCESS_BLOCK,
                          "Corrupted bitstream: expected checksum " + Integer.toHexString(checksum1) +
                          ", found " + Integer.toHexString(checksum2));
            }

            return new Status(currentBlockId, decoded, checksum1, 0, null);
         }
         catch (Exception e)
         {
            return new Status(currentBlockId, 0, checksum1, Error.ERR_PROCESS_BLOCK, e.getMessage());
         }
         finally
         {
            // Make sure to unfreeze next block
            if (this.processedBlockId.get() == this.blockId-1)
               this.processedBlockId.incrementAndGet();

            if (ed != null)
               ed.dispose();
         }
      }
   }


   static class Status
   {
      final int blockId;
      final int decoded;
      final int error; // 0 = OK
      final String msg;
      final int checksum;

      Status(int blockId, int decoded, int checksum, int error, String msg)
      {
         this.blockId = blockId;
         this.decoded = decoded;
         this.checksum = checksum;
         this.error = error;
         this.msg = msg;
      }
   }
}
