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

import kanzi.OutputBitStream;
import kanzi.BitStreamException;
import kanzi.EntropyEncoder;
import kanzi.entropy.HuffmanCommon.FrequencyArrayComparator;
import kanzi.util.sort.QuickSort;


// Implementation of a static Huffman encoder.
// Uses in place generation of canonical codes instead of a tree
public class HuffmanEncoder implements EntropyEncoder
{
    private static final int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
    private static final int MAX_SYMBOL_SIZE = 24;

    private final OutputBitStream bitstream;
    private final int[] freqs;
    private final int[] codes;
    private final int[] ranks;
    private final int[] sranks;  // sorted ranks
    private final int[] buffer;  // temporary data
    private final short[] sizes; // Cache for speed purpose
    private final int chunkSize;


    public HuffmanEncoder(OutputBitStream bitstream) throws BitStreamException
    {
       this(bitstream, DEFAULT_CHUNK_SIZE);
    }


    // The chunk size indicates how many bytes are encoded (per block) before
    // resetting the frequency stats. 0 means that frequencies calculated at the
    // beginning of the block apply to the whole block.
    // The default chunk size is 65536 bytes.
    public HuffmanEncoder(OutputBitStream bitstream, int chunkSize) throws BitStreamException
    {
        if (bitstream == null)
           throw new NullPointerException("Invalid null bitstream parameter");

        if ((chunkSize != 0) && (chunkSize < 1024))
           throw new IllegalArgumentException("The chunk size must be at least 1024");

        if (chunkSize > 1<<30)
           throw new IllegalArgumentException("The chunk size must be at most 2^30");

        this.bitstream = bitstream;
        this.freqs = new int[256];
        this.sizes = new short[256];
        this.ranks = new int[256];
        this.sranks = new int[256];
        this.buffer = new int[256];
        this.codes = new int[256];
        this.chunkSize = chunkSize;

        // Default frequencies, sizes and codes
        for (int i=0; i<256; i++)
        {
           this.freqs[i] = 1;
           this.sizes[i] = 8;
           this.codes[i] = i;
        }
    }

    
    // Rebuild Huffman codes
    public boolean updateFrequencies(int[] frequencies) throws BitStreamException
    {
        if ((frequencies == null) || (frequencies.length != 256))
           return false;

        int count = 0;

        for (int i=0; i<256; i++)
        {
           this.sizes[i] = 0;
           this.codes[i] = 0;

           if (frequencies[i] > 0)
              this.ranks[count++] = i;
        }

        try
        {
           if (count == 1)
           {
              this.sranks[0] = this.ranks[0];
              this.sizes[this.ranks[0]] = 1;
           }
           else   
           {
              this.computeCodeLengths(frequencies, count);
           }
        }
        catch (IllegalArgumentException e)
        {
           // Happens when a very rare symbol cannot be coded to due code length limit
           throw new BitStreamException(e.getMessage(), BitStreamException.INVALID_STREAM);
        }

        EntropyUtils.encodeAlphabet(this.bitstream, this.ranks, count);

        // Transmit code lengths only, frequencies and codes do not matter
        // Unary encode the length difference
        ExpGolombEncoder egenc = new ExpGolombEncoder(this.bitstream, true);
        short prevSize = 2;

        for (int i=0; i<count; i++)
        {
           final short currSize = this.sizes[this.ranks[i]];
           egenc.encodeByte((byte) (currSize - prevSize));
           prevSize = currSize;
        }

        // Create canonical codes 
        if (HuffmanCommon.generateCanonicalCodes(this.sizes, this.codes, this.sranks, count) < 0)
           throw new BitStreamException("Could not generate codes: max code length (24 bits) exceeded",
                                        BitStreamException.INVALID_STREAM);

        // Pack size and code (size <= MAX_SYMBOL_SIZE bits)
        for (int i=0; i<count; i++)
        {
           final int r = this.ranks[i];
           this.codes[r] |= (this.sizes[r] << 24);           
        }

        return true;
    }

    
    // See [In-Place Calculation of Minimum-Redundancy Codes]
    // by Alistair Moffat & Jyrki Katajainen
    // count > 1 by design
    private void computeCodeLengths(int[] frequencies, int count) 
    {  
      // Sort ranks by increasing frequency
      System.arraycopy(this.ranks, 0, this.sranks, 0, count);
      
      // Sort by increasing frequencies (first key) and increasing value (second key)
      new QuickSort(new FrequencyArrayComparator(frequencies)).sort(this.sranks, 0, count);
    
      for (int i=0; i<count; i++)               
         this.buffer[i] = frequencies[this.sranks[i]];
      
      computeInPlaceSizesPhase1(this.buffer, count);
      computeInPlaceSizesPhase2(this.buffer, count);
      
      for (int i=0; i<count; i++) 
      {
         short codeLen = (short) this.buffer[i];
         
         if ((codeLen <= 0) || (codeLen > MAX_SYMBOL_SIZE))
            throw new IllegalArgumentException("Could not generate codes: max code " +
               "length (" + MAX_SYMBOL_SIZE + " bits) exceeded");
         
         this.sizes[this.sranks[i]] = codeLen;
      }
    }
    
    
    static void computeInPlaceSizesPhase1(int[] data, int n) 
    {
      for (int s=0, r=0, t=0; t<n-1; t++) 
      {
         int sum = 0;

         for (int i=0; i<2; i++) 
         {
            if ((s>=n) || ((r<t) && (data[r]<data[s]))) 
            {
               sum += data[r];
               data[r] = t;
               r++;
            }
            else 
            {
               sum += data[s];

               if (s > t) 
                  data[s] = 0;

               s++;
            }
         }

         data[t] = sum;
      }
    }

    
    static void computeInPlaceSizesPhase2(int[] data, int n) 
    {
        int level_top = n - 2; //root
        int depth = 1;
        int i = n;
        int total_nodes_at_level = 2;

        while (i > 0) 
        {
           int k = level_top;

           while ((k>0) && (data[k-1]>=level_top))
              k--;

           final int internal_nodes_at_level = level_top - k;
           final int leaves_at_level = total_nodes_at_level - internal_nodes_at_level;

           for (int j=0; j<leaves_at_level; j++)
              data[--i] = depth;

           total_nodes_at_level = internal_nodes_at_level << 1;
           level_top = k;
           depth++;
        }
    }
    
    
    // Dynamically compute the frequencies for every chunk of data in the block   
    @Override
    public int encode(byte[] array, int blkptr, int len)
    {
       if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
          return -1;

       if (len == 0)
          return 0;

       final int[] frequencies = this.freqs;
       final int end = blkptr + len;
       final int sz = (this.chunkSize == 0) ? len : this.chunkSize;
       int startChunk = blkptr;
       
       while (startChunk < end)
       {
          final int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
          final int endChunk8 = ((endChunk - startChunk) & -8) + startChunk;

          for (int i=0; i<256; i++)
             frequencies[i] = 0;

          for (int i=startChunk; i<endChunk8; i+=8)
          {
             frequencies[array[i]   & 0xFF]++;
             frequencies[array[i+1] & 0xFF]++;
             frequencies[array[i+2] & 0xFF]++;
             frequencies[array[i+3] & 0xFF]++;
             frequencies[array[i+4] & 0xFF]++;
             frequencies[array[i+5] & 0xFF]++;
             frequencies[array[i+6] & 0xFF]++;
             frequencies[array[i+7] & 0xFF]++;
          }
          
          for (int i=endChunk8; i<endChunk; i++)
             frequencies[array[i] & 0xFF]++;

          // Rebuild Huffman codes
          this.updateFrequencies(frequencies);
 
          for (int i=startChunk; i<endChunk8; i+=8)
          {
             int val;
             val = this.codes[array[i]&0xFF];
             this.bitstream.writeBits(val, val >>> 24);
             val = this.codes[array[i+1]&0xFF];
             this.bitstream.writeBits(val, val >>> 24);
             val = this.codes[array[i+2]&0xFF];
             this.bitstream.writeBits(val, val >>> 24);
             val = this.codes[array[i+3]&0xFF];
             this.bitstream.writeBits(val, val >>> 24);
             val = this.codes[array[i+4]&0xFF];
             this.bitstream.writeBits(val, val >>> 24);
             val = this.codes[array[i+5]&0xFF];
             this.bitstream.writeBits(val, val >>> 24);
             val = this.codes[array[i+6]&0xFF];
             this.bitstream.writeBits(val, val >>> 24);
             val = this.codes[array[i+7]&0xFF];
             this.bitstream.writeBits(val, val >>> 24);
          }
          
          for (int i=endChunk8; i<endChunk; i++)
          {
             final int val = this.codes[array[i]&0xFF];
             this.bitstream.writeBits(val, val >>> 24);
          }
          
          startChunk = endChunk;
       }

       return len;
    }
    

    @Override
    public OutputBitStream getBitStream()
    {
       return this.bitstream;
    }

 
    @Override
    public void dispose() 
    {
    }
}
