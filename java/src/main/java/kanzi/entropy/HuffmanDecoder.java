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

package kanzi.entropy;

import kanzi.BitStreamException;
import kanzi.EntropyDecoder;
import kanzi.InputBitStream;


// Uses tables to decode symbols instead of a tree
public class HuffmanDecoder implements EntropyDecoder
{
    public static final int DECODING_BATCH_SIZE = 12; // in bits
    public static final int DECODING_MASK = (1 << DECODING_BATCH_SIZE) - 1;
    private static final int MAX_DECODING_INDEX = (DECODING_BATCH_SIZE << 8) | 0xFF;
    private static final int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
    private static final int SYMBOL_ABSENT = Integer.MAX_VALUE;
    private static final int MAX_SYMBOL_SIZE = 24;

    private final InputBitStream bitstream;
    private final int[] codes;
    private final int[] ranks;
    private final short[] sizes;
    private final int[] fdTable; // Fast decoding table
    private final int[] sdTable; // Slow decoding table
    private final int[] sdtIndexes; // Indexes for slow decoding table
    private final int chunkSize;
    private long state; // holds bits read from bitstream
    private int bits; // holds number of unused bits in 'state'
    private int minCodeLen;


    public HuffmanDecoder(InputBitStream bitstream) throws BitStreamException
    {
       this(bitstream, DEFAULT_CHUNK_SIZE);
    }


    // The chunk size indicates how many bytes are encoded (per block) before
    // resetting the frequency stats. 0 means that frequencies calculated at the
    // beginning of the block apply to the whole block.
    // The default chunk size is 65536 bytes.
    public HuffmanDecoder(InputBitStream bitstream, int chunkSize) throws BitStreamException
    {
        if (bitstream == null)
            throw new NullPointerException("Invalid null bitstream parameter");

        if ((chunkSize != 0) && (chunkSize < 1024))
           throw new IllegalArgumentException("The chunk size must be at least 1024");

        if (chunkSize > 1<<30)
           throw new IllegalArgumentException("The chunk size must be at most 2^30");

        this.bitstream = bitstream;
        this.sizes = new short[256];
        this.ranks = new int[256];
        this.codes = new int[256];
        this.fdTable = new int[1<<DECODING_BATCH_SIZE];
        this.sdTable = new int[256];
        this.sdtIndexes = new int[MAX_SYMBOL_SIZE+1];
        this.chunkSize = chunkSize;
        this.minCodeLen = 8;

        // Default lengths & canonical codes
        for (int i=0; i<256; i++)
        {
           this.sizes[i] = 8;
           this.codes[i] = i;
        }
    }


    public int readLengths() throws BitStreamException
    {
        int count = EntropyUtils.decodeAlphabet(this.bitstream, this.ranks);
        ExpGolombDecoder egdec = new ExpGolombDecoder(this.bitstream, true);
        int currSize ;
        this.minCodeLen = MAX_SYMBOL_SIZE; // max code length
        int prevSize = 2;
        
        // Read lengths
        for (int i=0; i<count; i++)
        {
           final int r = this.ranks[i];
           
           if ((r < 0) || (r >= this.codes.length))
           {
              throw new BitStreamException("Invalid bitstream: incorrect Huffman symbol " + r, 
                 BitStreamException.INVALID_STREAM);
           }
           
           this.codes[r] = 0;
           currSize = prevSize + egdec.decodeByte();

           if (currSize <= 0)
           {
              throw new BitStreamException("Invalid bitstream: incorrect size " + currSize +
                      " for Huffman symbol " + r, BitStreamException.INVALID_STREAM);
           }
           
           if (currSize > MAX_SYMBOL_SIZE)
           {
              throw new BitStreamException("Invalid bitstream: incorrect max size " + currSize +
                 " for Huffman symbol " + r, BitStreamException.INVALID_STREAM);
           }

           if (this.minCodeLen > currSize)
              this.minCodeLen = currSize;
           
           this.sizes[r] = (short) currSize;
           prevSize = currSize;
        }
 
        if (count == 0)
           return 0;

        // Create canonical codes
        if (HuffmanCommon.generateCanonicalCodes(this.sizes, this.codes, this.ranks, count) < 0)
        {
           throw new BitStreamException("Could not generate codes: max code length " +
                "(" + MAX_SYMBOL_SIZE + " bits) exceeded", BitStreamException.INVALID_STREAM);
        }

        // Build decoding tables
        this.buildDecodingTables(count);
        return count;
    }


    // Build decoding tables
    // The slow decoding table contains the codes in natural order.
    // The fast decoding table contains all the prefixes with DECODING_BATCH_SIZE bits.
    private void buildDecodingTables(int count)
    {
        for (int i=this.fdTable.length-1; i>=0; i--)
           this.fdTable[i] = 0;

        for (int i=this.sdTable.length-1; i>=0; i--)
           this.sdTable[i] = 0;

        for (int i=this.sdtIndexes.length-1; i>=0; i--)
           this.sdtIndexes[i] = SYMBOL_ABSENT;

        int len = 0;

        for (int i=0; i<count; i++)
        {
           final int r = this.ranks[i];
           final int code = this.codes[r];

           if (this.sizes[r] > len)
           {
              len = this.sizes[r];
              this.sdtIndexes[len] = i - code;
           }
    
           // Fill slow decoding table
           final int val = (this.sizes[r] << 8) | r;
           this.sdTable[i] = val;
           int idx, end;

           // Fill fast decoding table
           // Find location index in table
           if (len < DECODING_BATCH_SIZE)
           {
              idx = code << (DECODING_BATCH_SIZE - len);
              end = idx + (1 << (DECODING_BATCH_SIZE - len));
           }
           else
           {
              idx = code >>> (len - DECODING_BATCH_SIZE);
              end = idx + 1;
           }

           // All DECODING_BATCH_SIZE bit values read from the bit stream and
           // starting with the same prefix point to symbol r
           while (idx < end)
              this.fdTable[idx++] = val;
        }
    }


    // Use fastDecodeByte until the near end of chunk or block.
    @Override
    public int decode(byte[] array, int blkptr, int len)
    {
       if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
         return -1;

       if (len == 0)
          return 0;

       if (this.minCodeLen == 0)
          return -1;

       final int sz = (this.chunkSize == 0) ? len : this.chunkSize;
       int startChunk = blkptr;
       final int end = blkptr + len;       

       while (startChunk < end)
       { 
          // Reinitialize the Huffman tables
          if (this.readLengths() <= 0)
             return startChunk - blkptr;

          // Compute minimum number of bits required in bitstream for fast decoding
          int endPaddingSize = 64 / this.minCodeLen;

          if (this.minCodeLen * endPaddingSize != 64)
             endPaddingSize++;

          final int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
          final int endChunk1 = (endChunk - endPaddingSize) & -8;
          int i = startChunk;

          // Fast decoding (read DECODING_BATCH_SIZE bits at a time)
          for ( ; i<endChunk1; i+=8)
          {
             array[i]   = this.fastDecodeByte();
             array[i+1] = this.fastDecodeByte();
             array[i+2] = this.fastDecodeByte();
             array[i+3] = this.fastDecodeByte();
             array[i+4] = this.fastDecodeByte();
             array[i+5] = this.fastDecodeByte();
             array[i+6] = this.fastDecodeByte();
             array[i+7] = this.fastDecodeByte();
          }
          
          // Fallback to regular decoding (read one bit at a time)
          for ( ; i<endChunk; i++)
             array[i] = this.slowDecodeByte(0, 0);

          startChunk = endChunk;
       }

       return len;
    }


    private byte slowDecodeByte(int code, int codeLen)
    { 
       while (codeLen < MAX_SYMBOL_SIZE)
       {
          codeLen++;
          code <<= 1;

          if (this.bits == 0)
             code |= this.bitstream.readBit();
          else
          {
             // Consume remaining bits in 'state'
             this.bits--;
             code |= ((this.state >>> this.bits) & 1);
          }

          final int idx = this.sdtIndexes[codeLen];

          if (idx == SYMBOL_ABSENT) // No code with this length ?
             continue;

          if ((this.sdTable[idx+code] >>> 8) == codeLen)
             return (byte) this.sdTable[idx+code];
       }

       throw new BitStreamException("Invalid bitstream: incorrect Huffman code",
          BitStreamException.INVALID_STREAM);
    }


    // 64 bits must be available in the bitstream
    private byte fastDecodeByte()
    { 
       if (this.bits < DECODING_BATCH_SIZE)
       {
          // Fetch more bits from bitstream
          final long read = this.bitstream.readBits(64-this.bits);
          // Mask: 0 if this.bits == 0
          final long mask = (1 << this.bits) - 1;
          this.state = ((this.state & mask) << -this.bits) | read;
          this.bits = 64;
       }

       // Retrieve symbol from fast decoding table
       final int idx = (int) (this.state >>> (this.bits-DECODING_BATCH_SIZE)) & DECODING_MASK;
       final int val = this.fdTable[idx];

       if (val > MAX_DECODING_INDEX)
       {
          this.bits -= DECODING_BATCH_SIZE;
          return this.slowDecodeByte(idx, DECODING_BATCH_SIZE);
       }

       this.bits -= (val >>> 8);
       return (byte) (val);      
    }


    @Override
    public InputBitStream getBitStream()
    {
       return this.bitstream;
    }

  
    @Override
    public void dispose() 
    {
    }
}
