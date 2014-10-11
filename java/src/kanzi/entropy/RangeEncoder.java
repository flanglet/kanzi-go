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


// Based on Order 0 range coder by Dmitry Subbotin itself derived from the algorithm
// described by G.N.N Martin in his seminal article in 1979.
// [G.N.N. Martin on the Data Recording Conference, Southampton, 1979]
// Optimized for speed.

// Not thread safe
public final class RangeEncoder extends AbstractEncoder
{
    private static final long TOP_RANGE    = 0x00FFFFFFFFFFFFFFL;
    private static final long BOTTOM_RANGE = 0x000000FFFFFFFFFFL;
    private static final long MASK         = 0x00FF000000000000L;
    private static final long MAX_RANGE = BOTTOM_RANGE + 1;
    
    private static final int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
    private static final int NB_SYMBOLS = 257; //256 + EOF
    private static final int BASE_LEN = NB_SYMBOLS >> 4;

    private long low;
    private long range;
    private boolean disposed;
    private final int[] baseFreq;
    private final int[] deltaFreq;
    private final OutputBitStream bitstream;
    private final int chunkSize;


    public RangeEncoder(OutputBitStream bitstream)
    {
       this(bitstream, DEFAULT_CHUNK_SIZE);
    }
    
    
    // The chunk size indicates how many bytes are encoded (per block) before 
    // resetting the frequency stats. 0 means that frequencies calculated at the
    // beginning of the block apply to the whole block.
    // The default chunk size is 65536 bytes.
    public RangeEncoder(OutputBitStream bitstream, int chunkSize)
    {
        if (bitstream == null)
            throw new NullPointerException("Invalid null bitstream parameter");

        if ((chunkSize != 0) && (chunkSize < 1024))
           throw new IllegalArgumentException("The chunk size must be at least 1024");

        if (chunkSize > 1<<30)
           throw new IllegalArgumentException("The chunk size must be at most 2^30");

        this.range = TOP_RANGE;
        this.bitstream = bitstream;
        this.chunkSize = chunkSize;

        // Since the frequency update after each byte encoded is the bottleneck,
        // split the frequency table into an array of absolute frequencies (with
        // indexes multiple of 16) and delta frequencies (relative to the previous
        // absolute frequency) with indexes in the [0..15] range
        this.deltaFreq = new int[NB_SYMBOLS+1];
        this.baseFreq = new int[BASE_LEN+1];
        this.resetFrequencies();
    }

    
    public final void resetFrequencies()
    {
       for (int i=0; i<=NB_SYMBOLS; i++)
          this.deltaFreq[i] = i & 15; // DELTA

       for (int i=0; i<=BASE_LEN; i++)
          this.baseFreq[i] = i << 4; // BASE  
    }

    
    // Reset frequency stats for each chunk of data in the block
    @Override
    public int encode(byte[] array, int blkptr, int len)
    {
       if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
          return -1;
        
       if (len == 0)
          return 0;
      
       final int end = blkptr + len;
       final int sz = (this.chunkSize == 0) ? len : this.chunkSize;
       int startChunk = blkptr;
       int sizeChunk = (startChunk + sz < end) ? sz : end - startChunk;
       int endChunk = startChunk + sizeChunk;

       while (startChunk < end)
       {         
          this.resetFrequencies(); 
          
          for (int i=startChunk; i<endChunk; i++)
             this.encodeByte(array[i]);
         
          startChunk = endChunk;
          sizeChunk = (startChunk + sz < end) ? sz : end - startChunk;
          endChunk = startChunk + sizeChunk;
       }
       
       return len;
    }
    
    
    // This method is on the speed critical path (called for each byte)
    // The speed optimization is focused on reducing the frequency table update
    @Override
    public void encodeByte(byte b)
    {
        final int value = b & 0xFF;
        final int symbolLow = this.baseFreq[value>>4] + this.deltaFreq[value];
        final int symbolHigh = this.baseFreq[(value+1)>>4] + this.deltaFreq[value+1];
        this.range /= (this.baseFreq[BASE_LEN] + this.deltaFreq[NB_SYMBOLS]);

        // Encode symbol
        this.low += (symbolLow * this.range);
        this.range *= (symbolHigh - symbolLow);

        // If the left-most digits are the same throughout the range, write bits to bitstream
        while (true)
        {                       
            if (((this.low ^ (this.low + this.range)) & MASK) != 0)
            {
               if (this.range >= MAX_RANGE)
                  break;
               else // Normalize
                  this.range = -this.low & BOTTOM_RANGE;
            }

            this.bitstream.writeBits(this.low >> 48, 8);
            this.range <<= 8;
            this.low <<= 8;
        }

        this.updateFrequencies(value+1);
    }


    private void updateFrequencies(int value)
    {
       final int start = (value + 15) >> 4;

       // Update absolute frequencies
       for (int j=start; j<=BASE_LEN; j++)
          this.baseFreq[j]++;

       // Update relative frequencies (in the 'right' segment only)
       for (int j=value; j<(start<<4); j++)
          this.deltaFreq[j]++;
    }


    @Override
    public void dispose()
    {
       if (this.disposed == true)
          return;
        
       this.disposed = true;
       this.bitstream.writeBits(this.low, 56);
    }


    @Override
    public OutputBitStream getBitStream()
    {
       return this.bitstream;
    }
}