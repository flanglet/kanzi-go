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

import java.util.PriorityQueue;
import kanzi.OutputBitStream;
import kanzi.BitStreamException;
import kanzi.entropy.HuffmanTree.Node;
import kanzi.io.CompressedOutputStream;


public class HuffmanEncoder extends AbstractEncoder
{
    private static final int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default

    private final OutputBitStream bitstream;
    private final int[] buffer;
    private final int[] codes;
    private final int[] ranks;
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
        this.buffer = new int[256];
        this.sizes = new short[256];
        this.ranks = new int[256];
        this.codes = new int[256];
        this.chunkSize = chunkSize;

        // Default frequencies, sizes and codes
        for (int i=0; i<256; i++)
        {
           this.buffer[i] = 1;
           this.sizes[i] = 8;
           this.codes[i] = i;
        }
    }


    // Rebuild Huffman tree
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
           // Create tree from frequencies
           createTreeFromFrequencies(frequencies, this.sizes, this.ranks, count);
        }
        catch (IllegalArgumentException e)
        {
           // Happens when a very rare symbol cannot be coded to due code length limit
           throw new BitStreamException(e.getMessage(), BitStreamException.INVALID_STREAM);
        }

        EntropyUtils.encodeAlphabet(this.bitstream, count, this.ranks);

        // Transmit code lengths only, frequencies and codes do not matter
        // Unary encode the length difference
        ExpGolombEncoder egenc = new ExpGolombEncoder(this.bitstream, true);
        int prevSize = 2;

        for (int i=0; i<count; i++)
        {
           final int currSize = this.sizes[this.ranks[i]];
           egenc.encodeByte((byte) (currSize - prevSize));
           prevSize = currSize;
        }

        // Create canonical codes (reorders ranks)
        if (HuffmanTree.generateCanonicalCodes(this.sizes, this.codes, this.ranks, count) < 0)
           return false;

        // Pack size and code (size <= 24 bits)
        for (int i=0; i<count; i++)
        {
           final int r = this.ranks[i];
           this.codes[r] = (this.sizes[r] << 24) | this.codes[r];
        }

        return true;
    }


    // Dynamically compute the frequencies for every chunk of data in the block
    @Override
    public int encode(byte[] array, int blkptr, int len)
    {
       if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
          return -1;

       if (len == 0)
          return 0;

       final int[] frequencies = this.buffer;
       final int end = blkptr + len;
       final int sz = (this.chunkSize == 0) ? len : this.chunkSize;
       int startChunk = blkptr;
       int sizeChunk = (startChunk + sz < end) ? sz : end - startChunk;
       int endChunk = startChunk + sizeChunk;

       while (startChunk < end)
       {
          for (int i=0; i<256; i++)
             frequencies[i] = 0;

          for (int i=startChunk; i<endChunk; i++)
             frequencies[array[i] & 0xFF]++;

          // Rebuild Huffman tree
          this.updateFrequencies(frequencies);

          for (int i=startChunk; i<endChunk; i++)
          {
             if (this.encodeByte(array[i]) == false)
                return i - blkptr;
          }

          startChunk = endChunk;
          sizeChunk = (startChunk + sz < end) ? sz : end - startChunk;
          endChunk = startChunk + sizeChunk;
       }

       return len;
    }


    // Frequencies of the data block must have been previously set
    @Override
    public boolean encodeByte(byte val)
    {
       final int len = this.codes[val&0xFF] >>> 24;
       return this.bitstream.writeBits(this.codes[val&0xFF], len) == len;
    }


    private static Node createTreeFromFrequencies(int[] frequencies, short[] sizes_, int[] ranks, int count)
    {
       // Create Huffman tree of (present) symbols
       PriorityQueue<Node> queue = new PriorityQueue<Node>();

       for (int i=0; i<count; i++)
       {
          queue.offer(new Node((byte) ranks[i], frequencies[ranks[i]]));
       }

       while (queue.size() > 1)
       {
           // Extract 2 minimum nodes, merge them and enqueue result
           queue.offer(new Node(queue.poll(), queue.poll()));
       }

       final Node rootNode = queue.poll();

       if (count == 1)
          sizes_[rootNode.symbol & 0xFF] = (short) 1;
       else
          fillSizes(rootNode, 0, sizes_);

       return rootNode;
    }


    // Recursively fill sizes
    private static void fillSizes(Node node, int depth, short[] sizes_)
    {
       if ((node.left == null) && (node.right == null))
       {
          if (depth > 24)
             throw new IllegalArgumentException("Cannot code symbol '" + (node.symbol & 0xFF) + "'");

          sizes_[node.symbol & 0xFF] = (short) depth;
          return;
       }

       if (node.left != null)
          fillSizes(node.left, depth+1, sizes_);

       if (node.right != null)
          fillSizes(node.right, depth+1, sizes_);
    }


    @Override
    public OutputBitStream getBitStream()
    {
       return this.bitstream;
    }
}
