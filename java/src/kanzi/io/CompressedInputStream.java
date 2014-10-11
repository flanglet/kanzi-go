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

package kanzi.io;

import java.io.IOException;
import java.io.InputStream;
import java.io.PrintStream;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Future;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.locks.LockSupport;
import kanzi.BitStreamException;
import kanzi.ByteFunction;
import kanzi.EntropyDecoder;
import kanzi.IndexedByteArray;
import kanzi.InputBitStream;
import kanzi.bitstream.DefaultInputBitStream;
import kanzi.entropy.EntropyCodecFactory;
import kanzi.function.FunctionFactory;
import kanzi.util.XXHash;


// Implementation of a java.io.InputStream that can decode a stream
// compressed with CompressedOutputStream
public class CompressedInputStream extends InputStream
{
   private static final int BITSTREAM_TYPE           = 0x4B414E5A; // "KANZ"
   private static final int BITSTREAM_FORMAT_VERSION = 9;
   private static final int DEFAULT_BUFFER_SIZE      = 1024*1024;
   private static final int COPY_LENGTH_MASK         = 0x0F;
   private static final int SMALL_BLOCK_MASK         = 0x80;
   private static final int SKIP_FUNCTION_MASK       = 0x40;
   private static final int MIN_BLOCK_SIZE           = 1024;
   private static final int MAX_BLOCK_SIZE           = (64*1024*1024) - 4;
   private static final byte[] EMPTY_BYTE_ARRAY      = new byte[0];
   private static final int CANCEL_TASKS_ID          = -1;

   private int blockSize;
   private XXHash hasher;
   private final IndexedByteArray iba;
   private final IndexedByteArray[] buffers;
   private byte entropyType;
   private byte transformType;
   private final InputBitStream ibs;
   private final PrintStream ds;
   private boolean initialized;
   private boolean closed;
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
      this.iba = new IndexedByteArray(EMPTY_BYTE_ARRAY, 0);
      this.jobs = jobs;
      this.pool = pool;
      this.buffers = new IndexedByteArray[this.jobs];

      for (int i=0; i<this.jobs; i++)
         this.buffers[i] = new IndexedByteArray(EMPTY_BYTE_ARRAY, 0);

      this.ds = debug;
      this.blockId = new AtomicInteger(0);
      this.listeners = new ArrayList<BlockListener>(10);
   }


   protected void readHeader() throws IOException
   {
      if (this.initialized == true)
         return;

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
         this.hasher = new XXHash(BITSTREAM_TYPE);

      // Read entropy codec
      this.entropyType = (byte) this.ibs.readBits(7);

      // Read transform
      this.transformType = (byte) this.ibs.readBits(7);

      // Read block size
      this.blockSize = (int) this.ibs.readBits(26);

      if ((this.blockSize < MIN_BLOCK_SIZE) || (this.blockSize > MAX_BLOCK_SIZE))
         throw new kanzi.io.IOException("Invalid bitstream, incorrect block size: " + this.blockSize,
                 Error.ERR_BLOCK_SIZE);

      if (this.ds != null)
      {
         this.ds.println("Checksum set to " + (this.hasher != null));
         this.ds.println("Block size set to " + this.blockSize + " bytes");

         try
         {
            String w1 = new FunctionFactory().getName((byte) this.transformType);

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
            String w2 = new EntropyCodecFactory().getName((byte) this.entropyType);

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
         if (this.iba.index >= this.maxIdx)
         {
            this.maxIdx = this.processBlock();

            if (this.maxIdx == 0) // Reached end of stream
               return -1;
         }

         return this.iba.array[this.iba.index++] & 0xFF;
      }
      catch (BitStreamException e)
      {
         if (e.getErrorCode() == BitStreamException.END_OF_STREAM)
            return -1;

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
     * @param      array   the buffer into which the data is read.
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
   public int read(byte[] array, int off, int len) throws IOException
   {
      if ((off < 0) || (len < 0) || (len + off > array.length))
         throw new IndexOutOfBoundsException();

      if (this.closed == true)
         throw new kanzi.io.IOException("Stream closed", Error.ERR_READ_FILE);

      int remaining = len;

      while (remaining > 0)
      {
         // Limit to number of available bytes in buffer
         final int lenChunk = (this.iba.index + remaining < this.maxIdx) ? remaining :
                 this.maxIdx - this.iba.index;

         if (lenChunk > 0)
         {
            // Process a chunk of in-buffer data. No access to bitstream required
            System.arraycopy(this.iba.array, this.iba.index, array, off, lenChunk);
            this.iba.index += lenChunk;
            off += lenChunk;
            remaining -= lenChunk;

            if (remaining == 0)
               break;
         }

         // Buffer empty, time to decode
         int c2 = this.read();

         if (c2 == -1)
         {
            // EOF and we did not read any bytes in this call
            if (len == remaining)
               return -1;

            break;
         }

         array[off++] = (byte) c2;
         remaining--;
      }

      return len - remaining;
   }


   private int processBlock() throws IOException
   {
      if (this.initialized == false)
      {
         this.readHeader();
         this.initialized = true;
      }

      try
      {
         if (this.iba.array.length < this.jobs*this.blockSize)
            this.iba.array = new byte[this.jobs*this.blockSize];

         int decoded = 0;
         this.iba.index = 0;
         List<Callable<Integer>> tasks = new ArrayList<Callable<Integer>>(this.jobs);
         int blockNumber = this.blockId.get();

         // Create as many tasks as required
         for (int jobId=0; jobId<this.jobs; jobId++)
         {
            blockNumber++;
            Callable<Integer> task = new DecodingTask(this.iba.array, this.iba.index,
                    this.buffers[jobId].array, this.blockSize, (byte) this.transformType,
                    (byte) this.entropyType, blockNumber,
                    this.ibs, this.hasher, this.blockId,
                    this.listeners.toArray(new BlockListener[this.listeners.size()]));
            tasks.add(task);
            this.iba.index += this.blockSize;
         }

         if (this.jobs == 1)
         {
            // Synchronous call
            decoded = tasks.get(0).call();

            if (decoded < 0)
               throw new kanzi.io.IOException("Error in transform inverse()", Error.ERR_PROCESS_BLOCK);
         }
         else
         {
            // Invoke the tasks concurrently and validate the results
            for (Future<Integer> result : this.pool.invokeAll(tasks))
            {
               // Wait for completion of next task and validate result
               final int res = result.get();

               if (res < 0)
                  throw new kanzi.io.IOException("Error in transform inverse()", Error.ERR_PROCESS_BLOCK);

               decoded += res;
            }
         }

         this.iba.index = 0;
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
   public synchronized void close() throws IOException
   {
      if (this.closed == true)
         return;

      try
      {
         this.ibs.close();
      }
      catch (BitStreamException e)
      {
         throw new kanzi.io.IOException(e.getMessage(), ((BitStreamException) e).getErrorCode());
      }

      this.closed = true;


      // Release resources
      // Force error on any subsequent write attempt
      this.maxIdx = 0;
      this.iba.array = EMPTY_BYTE_ARRAY;
      this.iba.index = -1;

      for (int i=0; i<this.jobs; i++)
         this.buffers[i] = new IndexedByteArray(EMPTY_BYTE_ARRAY, -1);
   }


   // Return the number of bytes read so far
   public long getRead()
   {
      return (this.ibs.read() + 7) >> 3;
   }



   // A task used to decode a block
   // Several tasks may run in parallel. The transforms can be computed concurrently
   // but the entropy decoding is sequential since all tasks share the same bitstream.
   static class DecodingTask implements Callable<Integer>
   {
      private final IndexedByteArray data;
      private final IndexedByteArray buffer;
      private final int blockSize;
      private final byte transformType;
      private final byte entropyType;
      private final int blockId;
      private final InputBitStream ibs;
      private final XXHash hasher;
      private final AtomicInteger processedBlockId;
      private final BlockListener[] listeners;


      DecodingTask(byte[] data, int offset, byte[] buffer, int blockSize,
              byte transformType, byte entropyType, int blockId,
              InputBitStream ibs, XXHash hasher,
              AtomicInteger processedBlockId, BlockListener[] listeners)
      {
         this.data = new IndexedByteArray(data, offset);
         this.buffer = new IndexedByteArray(buffer, 0);
         this.blockSize = blockSize;
         this.transformType = transformType;
         this.entropyType = entropyType;
         this.blockId = blockId;
         this.ibs = ibs;
         this.hasher = hasher;
         this.processedBlockId = processedBlockId;
         this.listeners = ((listeners != null) && (listeners.length > 0)) ? listeners : null;
      }


      @Override
      public Integer call() throws Exception
      {
         return this.decodeBlock(this.data, this.buffer,
                 this.transformType, this.entropyType, this.blockId);
      }


      // Return -1 if error, otherwise the number of bytes read from the encoder
      private int decodeBlock(IndexedByteArray data, IndexedByteArray buffer, byte typeOfTransform,
              byte typeOfEntropy, int currentBlockId)
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

         // Skip, either all data have been processed or an error occured
         if (taskId == CANCEL_TASKS_ID)
            return 0;

         // Each block is decoded separately
         // Rebuild the entropy decoder to reset block statistics
         EntropyDecoder ed = new EntropyCodecFactory().newDecoder(this.ibs, typeOfEntropy);

         try
         {
            // Extract header directly from bitstream
            final long read = this.ibs.read();
            byte mode = (byte) this.ibs.readBits(8);
            int preTransformLength;
            int checksum1 = 0;

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
               return 0;
            }

            if ((preTransformLength < 0) || (preTransformLength > MAX_BLOCK_SIZE))
            {
               // Error => cancel concurrent decoding tasks
               this.processedBlockId.set(CANCEL_TASKS_ID);
               return -1;
            }

            // Extract checksum from bit stream (if any)
            if (this.hasher != null)
               checksum1 = (int) this.ibs.readBits(32);

            if (this.listeners != null)
            {
               // Notify before entropy (block size in bitstream is unknown)
               BlockEvent evt = new BlockEvent(BlockEvent.Type.BEFORE_ENTROPY, currentBlockId,
                       -1, checksum1, this.hasher != null);

               for (BlockListener bl : this.listeners)
                  bl.processEvent(evt);
            }

            if (typeOfTransform == FunctionFactory.NULL_TRANSFORM_TYPE)
               buffer.array = data.array; // share buffers if no transform
            else
            {
               int bufferSize = (this.blockSize >= preTransformLength) ? this.blockSize : preTransformLength;

               if (buffer.array.length < bufferSize)
                  buffer.array = new byte[bufferSize];
            }

            final int savedIdx = data.index;

            // Block entropy decode
            if (ed.decode(buffer.array, 0, preTransformLength) != preTransformLength)
            {
               // Error => cancel concurrent decoding tasks
               this.processedBlockId.set(CANCEL_TASKS_ID);
               return -1;
            }

            if (this.listeners != null)
            {
               // Notify after entropy (block size set to size in bitstream)
               BlockEvent evt = new BlockEvent(BlockEvent.Type.AFTER_ENTROPY, currentBlockId,
                       (int) ((this.ibs.read()-read)/8L), checksum1, this.hasher != null);

               for (BlockListener bl : this.listeners)
                  bl.processEvent(evt);
            }

            // After completion of the entropy decoding, increment the block id.
            // It unfreezes the task processing the next block (if any)
            this.processedBlockId.incrementAndGet();

            if (this.listeners != null)
            {
               // Notify before transform (block size after entropy decoding)
               BlockEvent evt = new BlockEvent(BlockEvent.Type.BEFORE_TRANSFORM, currentBlockId,
                       preTransformLength, checksum1, this.hasher != null);

               for (BlockListener bl : this.listeners)
                  bl.processEvent(evt);
            }

            if (((mode & SMALL_BLOCK_MASK) != 0) || ((mode & SKIP_FUNCTION_MASK) != 0))
            {
               if (buffer.array != data.array)
                  System.arraycopy(buffer.array, 0, data.array, savedIdx, preTransformLength);

               buffer.index = preTransformLength;
               data.index = savedIdx + preTransformLength;
            }
            else
            {
               // Each block is decoded separately
               // Rebuild the entropy decoder to reset block statistics
               ByteFunction transform = new FunctionFactory().newFunction(preTransformLength,
                       (byte) this.transformType);

               buffer.index = 0;

               // Inverse transform
               if (transform.inverse(buffer, data) == false)
                  return -1;
            }

            final int decoded = data.index - savedIdx;

            if (this.listeners != null)
            {
               // Notify after transform
               BlockEvent evt = new BlockEvent(BlockEvent.Type.AFTER_TRANSFORM, currentBlockId,
                       decoded, checksum1, this.hasher != null);

               for (BlockListener bl : this.listeners)
                  bl.processEvent(evt);
            }

            // Verify checksum
            if (this.hasher != null)
            {
               final int checksum2 = this.hasher.hash(data.array, savedIdx, decoded);

               if (checksum2 != checksum1)
                  throw new IllegalStateException("Corrupted bitstream: expected checksum " +
                          Integer.toHexString(checksum1) + ", found " + Integer.toHexString(checksum2));
            }

            return decoded;
         }
         finally
         {
            // Reset buffer in case another block uses a different transform
            if (typeOfTransform == FunctionFactory.NULL_TRANSFORM_TYPE)
               buffer.array = EMPTY_BYTE_ARRAY;

            if (ed != null)
               ed.dispose();
         }
      }
   }
}
