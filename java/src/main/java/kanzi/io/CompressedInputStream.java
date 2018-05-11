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

import kanzi.function.ByteFunctionFactory;
import kanzi.Error;
import kanzi.Event;
import java.io.IOException;
import java.io.InputStream;
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
import kanzi.EntropyDecoder;
import kanzi.Global;
import kanzi.SliceByteArray;
import kanzi.InputBitStream;
import kanzi.bitstream.DefaultInputBitStream;
import kanzi.entropy.EntropyCodecFactory;
import kanzi.function.ByteTransformSequence;
import kanzi.util.hash.XXHash32;
import kanzi.Listener;


// Implementation of a java.io.InputStream that can decode a stream
// compressed with CompressedOutputStream
public class CompressedInputStream extends InputStream
{
   private static final int BITSTREAM_TYPE           = 0x4B414E5A; // "KANZ"
   private static final int BITSTREAM_FORMAT_VERSION = 6;
   private static final int DEFAULT_BUFFER_SIZE      = 1024*1024;
   private static final int EXTRA_BUFFER_SIZE        = 256;
   private static final int COPY_BLOCK_MASK          = 0x80;
   private static final int TRANSFORMS_MASK          = 0x10;
   private static final int MIN_BITSTREAM_BLOCK_SIZE = 1024;
   private static final int MAX_BITSTREAM_BLOCK_SIZE = 1024*1024*1024;
   private static final byte[] EMPTY_BYTE_ARRAY      = new byte[0];
   private static final int CANCEL_TASKS_ID          = -1;
   private static final int MAX_CONCURRENCY          = 64;

   private int blockSize;
   private int nbInputBlocks;
   private XXHash32 hasher;
   private final SliceByteArray sa; // for all blocks
   private final SliceByteArray[] buffers; // input & output per block
   private int entropyType;
   private long transformType;
   private final InputBitStream ibs;
   private final AtomicBoolean initialized;
   private final AtomicBoolean closed;
   private int maxIdx;
   private final AtomicInteger blockId;
   private int jobs;
   private final ExecutorService pool;
   private final List<Listener> listeners;
   private final Map<String, Object> ctx;


   // debug print stream is optional (may be null)
   public CompressedInputStream(InputStream is, Map<String, Object> ctx)
   {
      if (is == null)
         throw new NullPointerException("Invalid null input stream parameter");
            
      if (ctx == null)
         throw new NullPointerException("Invalid null context parameter");
            
      final int tasks = (Integer) ctx.get("jobs");
 
      if ((tasks <= 0) || (tasks > MAX_CONCURRENCY)) 
         throw new IllegalArgumentException("The number of jobs must be in [1.." + MAX_CONCURRENCY+ "]");

      ExecutorService threadPool = (ExecutorService) ctx.get("pool");
      
      if ((tasks > 1) && (threadPool == null))
         throw new IllegalArgumentException("The thread pool cannot be null when the number of jobs is "+tasks);

      this.ibs = new DefaultInputBitStream(is, DEFAULT_BUFFER_SIZE);
      this.sa = new SliceByteArray();
      this.jobs = tasks;
      this.pool = threadPool;
      this.buffers = new SliceByteArray[2*this.jobs];
      this.closed = new AtomicBoolean(false);
      this.initialized = new AtomicBoolean(false);

      for (int i=0; i<this.buffers.length; i++)
         this.buffers[i] = new SliceByteArray(EMPTY_BYTE_ARRAY, 0);

      this.blockId = new AtomicInteger(0);
      this.listeners = new ArrayList<>(10);
      this.ctx = ctx;
      this.blockSize = 0;
      this.entropyType = EntropyCodecFactory.NONE_TYPE;
      this.transformType = ByteFunctionFactory.NONE_TYPE;
   }


   protected void readHeader() throws IOException
   {
      // Read stream type
      final int type = (int) this.ibs.readBits(32);

      // Sanity check
      if (type != BITSTREAM_TYPE)
         throw new kanzi.io.IOException("Invalid stream type", Error.ERR_INVALID_FILE);

      // Read stream version
      final int version = (int) this.ibs.readBits(5);

      // Sanity check
      if (version != BITSTREAM_FORMAT_VERSION)
         throw new kanzi.io.IOException("Invalid bitstream, cannot read this version of the stream: " + version,
                 Error.ERR_STREAM_VERSION);

      // Read block checksum
      if (this.ibs.readBit() == 1)
         this.hasher = new XXHash32(BITSTREAM_TYPE);

      // Read entropy codec
      this.entropyType = (int) this.ibs.readBits(5);

      // Read transforms: 8*6 bits
      this.transformType = this.ibs.readBits(48);

      // Read block size
      this.blockSize = (int) this.ibs.readBits(26) << 4;
      this.ctx.put("blockSize", this.blockSize);

      if ((this.blockSize < MIN_BITSTREAM_BLOCK_SIZE) || (this.blockSize > MAX_BITSTREAM_BLOCK_SIZE))
         throw new kanzi.io.IOException("Invalid bitstream, incorrect block size: " + this.blockSize,
                 Error.ERR_BLOCK_SIZE);

      if (((long) this.blockSize) * ((long) this.jobs) >= (long) Integer.MAX_VALUE)
         this.jobs = Integer.MAX_VALUE / this.blockSize;

      // Read number of blocks in input. 0 means 'unknown' and 63 means 63 or more.
      this.nbInputBlocks = (int) this.ibs.readBits(6);
      
      // Read reserved bits
      this.ibs.readBits(5);   

      if (this.listeners.size() > 0)
      {
         StringBuilder sb = new StringBuilder(200);
         sb.append("Checksum set to ").append(this.hasher != null).append("\n");
         sb.append("Block size set to ").append(this.blockSize).append(" bytes").append("\n");

         try
         {
            String w1 = new ByteFunctionFactory().getName(this.transformType);

            if ("NONE".equals(w1))
               w1 = "no";

            sb.append("Using ").append(w1).append(" transform (stage 1)").append("\n");
         }
         catch (IllegalArgumentException e)
         {
            throw new kanzi.io.IOException("Invalid bitstream, unknown transform type: "+
                    this.transformType, Error.ERR_INVALID_CODEC);
         }

        try
         {
            String w2 = EntropyCodecFactory.getName(this.entropyType);

            if ("NONE".equals(w2))
               w2 = "no";

            sb.append("Using ").append(w2).append(" entropy codec (stage 2)");
         }
         catch (IllegalArgumentException e)
         {
            throw new kanzi.io.IOException("Invalid bitstream, unknown entropy codec type: "+
                    this.entropyType , Error.ERR_INVALID_CODEC);
         }
        
         // Protect against future concurrent modification of the list of block listeners
         Listener[] blockListeners = this.listeners.toArray(new Listener[this.listeners.size()]);
         Event evt = new Event(Event.Type.AFTER_HEADER_DECODING, 0, sb.toString());
         notifyListeners(blockListeners, evt);        
      }
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

		   // Protect against future concurrent modification of the list of block listeners
         Listener[] blockListeners = this.listeners.toArray(new Listener[this.listeners.size()]);
         int decoded = 0;
         this.sa.index = 0;
         List<Callable<Status>> tasks = new ArrayList<>(this.jobs);
         final int firstBlockId = this.blockId.get();
         int nbJobs = this.jobs;
         int[] jobsPerTask;
         
         // Assign optimal number of tasks and jobs per task 
         if (nbJobs > 1)
         {
            // If the number of input blocks is available, use it to optimize 
            // memory usage
            if (this.nbInputBlocks != 0)
            {
               // Limit the number of jobs if there are fewer blocks that this.jobs
               // It allows more jobs per task and reduces memory usage.
               nbJobs = Math.min(nbJobs, this.nbInputBlocks);
            }
  
            jobsPerTask = Global.computeJobsPerTask(new int[nbJobs], this.jobs, nbJobs);           
         }
         else
         {
            jobsPerTask = new int[] { this.jobs };
         }

         // Create as many tasks as required
         for (int jobId=0; jobId<nbJobs; jobId++)
         {
            this.buffers[2*jobId].index = 0;
            this.buffers[2*jobId+1].index = 0;
            
            if (this.buffers[2*jobId].array.length < blkSize)
            {
               // Lazy instantiation of input buffers this.buffers[2*jobId]
               // Output buffers this.buffers[2*jobId+1] are lazily instantiated
               // by the decoding tasks.
               this.buffers[2*jobId].array = new byte[blkSize];    
               this.buffers[2*jobId].length = blkSize;
            }
                         
            Map<String, Object> map = new HashMap<>(this.ctx);
            map.put("jobs", jobsPerTask[jobId]);
            Callable<Status> task = new DecodingTask(this.buffers[2*jobId],
                    this.buffers[2*jobId+1], blkSize, this.transformType,
                    this.entropyType, firstBlockId+jobId+1,
                    this.ibs, this.hasher, this.blockId,
                    blockListeners, map);
            tasks.add(task);            
         }

         List<Status> results = new ArrayList<>(tasks.size());

         if (tasks.size() == 1)
         {
            // Synchronous call
            Status status = tasks.get(0).call();
            results.add(status);
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
               results.add(status);
               decoded += status.decoded;

               if (status.error != 0)
                  throw new kanzi.io.IOException(status.msg, status.error);
            }
         }

         final int size = this.sa.index + decoded;
            
         if (size > nbJobs*this.blockSize)
            throw new kanzi.io.IOException("Invalid data", Error.ERR_PROCESS_BLOCK);
         
         this.sa.length = size;

         if (this.sa.array.length < this.sa.length)
             this.sa.array = new byte[this.sa.length];
         
         for (Status res : results)
         {                           
            System.arraycopy(res.data, 0, this.sa.array, this.sa.index, res.decoded);
            this.sa.index += res.decoded;
                       
            if (blockListeners.length > 0)
            {
               // Notify after transform ... in block order !
               Event evt = new Event(Event.Type.AFTER_TRANSFORM, res.blockId,
                       res.decoded, res.checksum, this.hasher != null, res.completionTime);

               notifyListeners(blockListeners, evt);
            }
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

      for (int i=0; i<this.buffers.length; i++)
         this.buffers[i] = new SliceByteArray(EMPTY_BYTE_ARRAY, 0);
   }


   // Return the number of bytes read so far
   public long getRead()
   {
      return (this.ibs.read() + 7) >> 3;
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


   // A task used to decode a block
   // Several tasks may run in parallel. The transforms can be computed concurrently
   // but the entropy decoding is sequential since all tasks share the same bitstream.
   static class DecodingTask implements Callable<Status>
   {
      private final SliceByteArray data;
      private final SliceByteArray buffer;
      private final int blockSize;
      private final long transformType;
      private final int entropyType;
      private final int blockId;
      private final InputBitStream ibs;
      private final XXHash32 hasher;
      private final AtomicInteger processedBlockId;
      private final Listener[] listeners;
      private final Map<String, Object> ctx;


      DecodingTask(SliceByteArray iBuffer, SliceByteArray oBuffer, int blockSize,
              long transformType, int entropyType, int blockId,
              InputBitStream ibs, XXHash32 hasher,
              AtomicInteger processedBlockId, Listener[] listeners,
              Map<String, Object> ctx)
      {
         this.data = iBuffer;
         this.buffer = oBuffer;
         this.blockSize = blockSize;
         this.transformType = transformType;
         this.entropyType = entropyType;
         this.blockId = blockId;
         this.ibs = ibs;
         this.hasher = hasher;
         this.processedBlockId = processedBlockId;
         this.listeners = listeners;
         this.ctx = ctx;
      }


      @Override
      public Status call() throws Exception
      {
         return this.decodeBlock(this.data, this.buffer,
                 this.transformType, this.entropyType, this.blockId);
      }


      // Decode mode + transformed entropy coded data
      // mode | 0b10000000 => copy block
      //      | 0b0yy00000 => size(size(block))-1
      //      | 0b000y0000 => 1 if more than 4 transforms
      //  case 4 transforms or less
      //      | 0b0000yyyy => transform sequence skip flags (1 means skip)
      //  case more than 4 transforms
      //      | 0b00000000
      //      then 0byyyyyyyy => transform sequence skip flags (1 means skip)
      private Status decodeBlock(SliceByteArray data, SliceByteArray buffer,
         long blockTransformType, int blockEntropyType, int currentBlockId)
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
            return new Status(data, currentBlockId, 0, 0, 0, null);

         int checksum1 = 0;
         EntropyDecoder ed = null;

         try
         {
            // Extract block header directly from bitstream
            final long read = this.ibs.read();
            byte mode = (byte) this.ibs.readBits(8);
            byte skipFlags = 0;

            if ((mode & COPY_BLOCK_MASK) != 0)
            {
               blockTransformType = ByteFunctionFactory.NONE_TYPE;
               blockEntropyType = EntropyCodecFactory.NONE_TYPE;
            }
            else
            {
               if ((mode & TRANSFORMS_MASK) != 0)
                  skipFlags = (byte) this.ibs.readBits(8);
               else
                  skipFlags = (byte) ((mode<<4) | 0x0F);
            }
            
            final int dataSize = 1 + ((mode>>5)&0x03);
            final int length = dataSize << 3;
            final long mask = (1L<<length) - 1;
            int preTransformLength = (int) (this.ibs.readBits(length) & mask);

            if (preTransformLength == 0)
            {
               // Last block is empty, return success and cancel pending tasks
               this.processedBlockId.set(CANCEL_TASKS_ID);
               return new Status(data, currentBlockId, 0, checksum1, 0, null);
            }

            if ((preTransformLength < 0) || (preTransformLength > MAX_BITSTREAM_BLOCK_SIZE))
            {
               // Error => cancel concurrent decoding tasks
               this.processedBlockId.set(CANCEL_TASKS_ID);
               return new Status(data, currentBlockId, 0, checksum1, Error.ERR_READ_FILE,
                    "Invalid compressed block length: " + preTransformLength);
            }

            // Extract checksum from bit stream (if any)
            if (this.hasher != null)
               checksum1 = (int) this.ibs.readBits(32);

            if (this.listeners.length > 0)
            {
               // Notify before entropy (block size in bitstream is unknown)
               Event evt = new Event(Event.Type.BEFORE_ENTROPY, currentBlockId,
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
            this.ctx.put("size", preTransformLength);

            // Each block is decoded separately
            // Rebuild the entropy decoder to reset block statistics
            ed = new EntropyCodecFactory().newDecoder(this.ibs, this.ctx, blockEntropyType);

            // Block entropy decode
            if (ed.decode(buffer.array, 0, preTransformLength) != preTransformLength)
            {
               // Error => cancel concurrent decoding tasks
               this.processedBlockId.set(CANCEL_TASKS_ID);
               return new Status(data, currentBlockId, 0, checksum1, Error.ERR_PROCESS_BLOCK,
                  "Entropy decoding failed");
            }

            if (this.listeners.length > 0)
            {
               // Notify after entropy (block size set to size in bitstream)
               Event evt = new Event(Event.Type.AFTER_ENTROPY, currentBlockId,
                       (int) ((this.ibs.read()-read)/8L), checksum1, this.hasher != null);

               notifyListeners(this.listeners, evt);
            }

            // After completion of the entropy decoding, increment the block id.
            // It unfreezes the task processing the next block (if any)
            this.processedBlockId.incrementAndGet();

            if (this.listeners.length > 0)
            {
               // Notify before transform (block size after entropy decoding)
               Event evt = new Event(Event.Type.BEFORE_TRANSFORM, currentBlockId,
                       preTransformLength, checksum1, this.hasher != null);

               notifyListeners(this.listeners, evt);
            }

            ByteTransformSequence transform = new ByteFunctionFactory().newFunction(this.ctx,
                     blockTransformType);
            transform.setSkipFlags(skipFlags);
            buffer.index = 0;

            // Inverse transform
            buffer.length = preTransformLength;

            if (transform.inverse(buffer, data) == false)
               return new Status(data, currentBlockId, 0, checksum1, Error.ERR_PROCESS_BLOCK,
                  "Transform inverse failed");

            final int decoded = data.index - savedIdx;

            // Verify checksum
            if (this.hasher != null)
            {
               final int checksum2 = this.hasher.hash(data.array, savedIdx, decoded);

               if (checksum2 != checksum1)
                  return new Status(data, currentBlockId, decoded, checksum1, Error.ERR_CRC_CHECK,
                          "Corrupted bitstream: expected checksum " + Integer.toHexString(checksum1) +
                          ", found " + Integer.toHexString(checksum2));
            }

            return new Status(data, currentBlockId, decoded, checksum1, 0, null);
         }
         catch (Exception e)
         {
            return new Status(data, currentBlockId, 0, checksum1, Error.ERR_PROCESS_BLOCK, e.getMessage());
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
      final byte[] data;
      final int error; // 0 = OK
      final String msg;
      final int checksum;
      final long completionTime;

      Status(SliceByteArray data, int blockId, int decoded, int checksum, int error, String msg)
      {
         this.data = data.array;
         this.blockId = blockId;
         this.decoded = decoded;
         this.checksum = checksum;
         this.error = error;
         this.msg = msg;
         this.completionTime = System.nanoTime();
      }
   }
}
