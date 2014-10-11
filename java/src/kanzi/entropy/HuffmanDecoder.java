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

import kanzi.BitStreamException;
import kanzi.InputBitStream;



public class HuffmanDecoder extends AbstractDecoder
{
    public static final int DECODING_BATCH_SIZE = 10; // in bits
    public static final int DECODING_MASK = (1 << DECODING_BATCH_SIZE) - 1;
    private static final int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
    private static final int ABSENT = Integer.MAX_VALUE;

    private final InputBitStream bitstream;
    private final int[] codes;
    private final int[] ranks;
    private final short[] sizes;
    private final int[] fdTable; // Fast decoding table
    private final int[] sdTable; // Slow decoding table
    private final int[] sdtIndexes; // Indexes for slow decoding table
    private final int chunkSize;
    private long state; // holds bits read from bitstream
    private int bits; // hold number of unused bits in 'state'
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
        this.sdtIndexes = new int[24];
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
        this.minCodeLen = 24; // max code length
        int prevSize = 2;
        
        // Read lengths
        for (int i=0; i<count; i++)
        {
           final int r = this.ranks[i];
           this.codes[r] = 0;
           int delta = egdec.decodeByte();
           currSize = prevSize + delta;

           if (currSize < 0)
           {
              throw new BitStreamException("Invalid bitstream: incorrect size " + currSize +
                      " for Huffman symbol " + r, BitStreamException.INVALID_STREAM);
           }
           
           if (currSize != 0)
           {
              if (currSize > 24)
              {
                 throw new BitStreamException("Invalid bitstream: incorrect max size " + currSize +
                    " for Huffman symbol " + r, BitStreamException.INVALID_STREAM);
              }

              if (this.minCodeLen > currSize)
                 this.minCodeLen = currSize;
           }
           
           this.sizes[r] = (short) currSize;
           prevSize = currSize;
        }
 
        if (count == 0)
           return 0;

        // Create canonical codes
        HuffmanTree.generateCanonicalCodes(this.sizes, this.codes, this.ranks, count);

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
           this.sdtIndexes[i] = ABSENT;

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
           final int val = (r << 8) | this.sizes[r];
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
              idx = code >> (len - DECODING_BATCH_SIZE);
              end = idx + 1;
           }

           // All DECODING_BATCH_SIZE bit values read from the bit stream and
           // starting with the same prefix point to symbol r
           while (idx < end)
              this.fdTable[idx++] = val;
        }
    }


    // Rebuild the Huffman tree for each chunk of data in the block
    // Use fastDecodeByte until the near end of chunk or block.
    @Override
    public int decode(byte[] array, int blkptr, int len)
    {
       if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
         return -1;

       if (len == 0)
          return 0;

       final int sz = (this.chunkSize == 0) ? len : this.chunkSize;
       int startChunk = blkptr;
       final int end = blkptr + len;       

       while (startChunk < end)
       { 
          // Reinitialize the Huffman tables
          if (this.readLengths() == 0)
             return startChunk - blkptr;

          // Compute minimum number of bits requires in bitstream for fast decoding
          int endPaddingSize = 64 / this.minCodeLen;

          if (this.minCodeLen * endPaddingSize != 64)
             endPaddingSize++;

          final int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
          final int endChunk1 = endChunk - endPaddingSize;
          int i = startChunk;

          // Fast decoding (read DECODING_BATCH_SIZE bits at a time)
          for ( ; i<endChunk1; i++)
             array[i] = this.fastDecodeByte();

          // Fallback to regular decoding (read one bit at a time)
          for ( ; i<endChunk; i++)
             array[i] = this.decodeByte();

          startChunk = endChunk;
       }

       return len;
    }


    // The data block header must have been read before (call to updateFrequencies())
    @Override
    public byte decodeByte()
    {
       return this.slowDecodeByte(0, 0);
    }


    private byte slowDecodeByte(int code, int codeLen)
    { 
       while (codeLen < 23)
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

          int idx = this.sdtIndexes[codeLen];

          if (idx == ABSENT) // No code with this length ?
             continue;

          if ((this.sdTable[idx+code] & 0xFF) == codeLen)
             return (byte) (this.sdTable[idx+code] >>> 8);
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
          long read = this.bitstream.readBits(64-this.bits);
          final long mask = (1 << this.bits) - 1;
          this.state = ((this.state & mask) << (64-this.bits)) | read;
          this.bits = 64;
       }

       // Retrieve symbol from fast decoding table
       int idx = (int) (this.state >>> (this.bits-DECODING_BATCH_SIZE)) & DECODING_MASK;
       int val = this.fdTable[idx];

       if ((val & 0xFF) > DECODING_BATCH_SIZE)
       {
          this.bits -= DECODING_BATCH_SIZE;
          return this.slowDecodeByte(idx, DECODING_BATCH_SIZE);
       }

       this.bits -= (val & 0xFF);
       return (byte) (val >>> 8);      
    }


    @Override
    public InputBitStream getBitStream()
    {
       return this.bitstream;
    }
}
