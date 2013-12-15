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

import java.util.Map;
import java.util.TreeMap;
import kanzi.BitStreamException;
import kanzi.InputBitStream;
import kanzi.entropy.HuffmanTree.Node;



public class HuffmanDecoder extends AbstractDecoder
{
    public static final int DECODING_BATCH_SIZE = 10; // in bits
    private static final int DEFAULT_CHUNK_SIZE = 1 << 16; // 64 KB by default
    private static final Key ZERO_KEY = new Key((short) 0, 0);

    private final InputBitStream bitstream;
    private final int[] codes;
    private final short[] sizes;
    private Node root;
    private CacheData[] decodingCache;
    private CacheData current;
    private final int chunkSize;


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
        this.codes = new int[256];
        this.chunkSize = chunkSize;

        // Default lengths & canonical codes
        for (int i=0; i<256; i++)
        {
           this.sizes[i] = 8;
           this.codes[i] = i;
        }

       // Create tree from code sizes
       this.root = this.createTreeFromSizes(8);
       this.decodingCache = buildDecodingCache(this.root, new CacheData[1<<DECODING_BATCH_SIZE]);
       this.current = this.decodingCache[0]; // point to root
    }


    private static CacheData[] buildDecodingCache(Node rootNode, CacheData[] cache)
    {
       final int end = 1 << DECODING_BATCH_SIZE;
       CacheData previousData = (cache[0] == null) ? new CacheData(rootNode) : cache[0];

       // Create an array storing a list of tree nodes (shortcuts) for each input value
       for (int val=0; val<end; val++)
       {
          int shift = DECODING_BATCH_SIZE - 1;
          boolean firstAdded = false;

          while (shift >= 0)
          {
             // Start from root
             Node currentNode = rootNode;

             // Process next bit
             while ((currentNode.left != null) || (currentNode.right != null))
             {
                currentNode = (((val >> shift) & 1) == 0) ? currentNode.left : currentNode.right;

                if (--shift < 0)
                   break;
             }

             // Reuse cache data objects when recreating the cache
             if (previousData.next == null)
                previousData.next = new CacheData(currentNode);
             else
                previousData.next.value = currentNode;

             // The cache is made of linked nodes
             previousData = previousData.next;

             if (firstAdded == false)
             {
                // Add first node of list to array (whether it is a leaf or not)
                cache[val] = previousData;
                firstAdded = true;
             }
          }

          // Reuse cache data objects when recreating the cache
          if (previousData.next == null)
             previousData.next = new CacheData(rootNode);
          else
             previousData.next.value = rootNode;

          previousData = previousData.next;
       }

       return cache;
    }


    private Node createTreeFromSizes(int maxSize)
    {
       TreeMap<Key, Node> codeMap = new TreeMap<Key, Node>();
       final int sum = 1 << maxSize;
       codeMap.put(ZERO_KEY, new Node((byte) 0, sum));

       // Create node for each (present) symbol and add to map
       for (int i=this.sizes.length-1; i>=0; i--)
       {
          final short size = this.sizes[i];

          if (size <= 0)
             continue;

          final Key key = new Key(size, this.codes[i]);
          final Node value = new Node((byte) i, sum >> size);
          codeMap.put(key, value);
       }

       // Process each element of the map except the root node
       while (codeMap.size() > 1)
       {
          // Remove last entry and reuse key for upNode
          final Map.Entry<Key, Node> last = codeMap.pollLastEntry();
          Key key = last.getKey();
          final int code = key.code;
          key.length--;
          key.code = code >> 1;
          Node upNode = codeMap.get(key);

          // Create superior node if it does not exist (length gap > 1)
          if (upNode == null)
          {
             upNode = new Node((byte) 0, sum >> key.length);
             codeMap.put(key, upNode);
          }

          // Add the current node to its parent at the correct place
          if ((code & 1) == 1)
             upNode.right = last.getValue();
          else
             upNode.left = last.getValue();
       }

       // Return the last element of the map (root node)
       return codeMap.firstEntry().getValue();
    }


    public boolean readLengths() throws BitStreamException
    {
        final short[] buf = this.sizes;
        ExpGolombDecoder egdec = new ExpGolombDecoder(this.bitstream, true);
        int currSize = 2 + egdec.decodeByte();

        if (currSize < 0)
        {
           throw new BitStreamException("Invalid bitstream: incorrect size "+currSize+
                   " for Huffman symbol 0", BitStreamException.INVALID_STREAM);
        }

        int maxSize = currSize;
        int prevSize = currSize;
        buf[0] = (short) currSize;
        int zeros = 0;

        // Read lengths
        for (int i=1; i<256; i++)
        {
           currSize = prevSize + egdec.decodeByte();

           if (currSize < 0)
           {
              throw new BitStreamException("Invalid bitstream: incorrect size "+currSize+
                      " for Huffman symbol "+i, BitStreamException.INVALID_STREAM);
           }

           buf[i] = (short) currSize;
           zeros = (currSize == 0) ? zeros+1 : 0;

           if (maxSize < currSize)
              maxSize = currSize;

           // If there is one zero size symbol, save a few bits by avoiding the
           // encoding of a big size difference twice
           // EG: 13 13 0 13 14 ... encoded as 0 -13 0 +1 instead of 0 -13 +13 0 +1
           // If there are several zero size symbols in a row, use regular decoding
           if (zeros != 1)
              prevSize = currSize;
        }

        // Create canonical codes
        HuffmanTree.generateCanonicalCodes(buf, this.codes);

        // Create tree from code sizes
        this.root = this.createTreeFromSizes(maxSize);
        buildDecodingCache(this.root, this.decodingCache);
        this.current = new CacheData(this.root); // point to root
        return true;
    }


    // Rebuild the Huffman tree for each chunk of data in the block
    // Use fastDecodeByte until the near end of chunk or block.
    @Override
    public int decode(byte[] array, int blkptr, int len)
    {
       if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
         return -1;

       final int sz = (this.chunkSize == 0) ? len : this.chunkSize;
       int startChunk = blkptr;
       final int end = blkptr + len;
       int sizeChunk = (startChunk + sz < end) ? sz : end - startChunk;
       int endChunk = startChunk + sizeChunk;

       while (startChunk < end)
       {
          // Reinitialize the Huffman tree
          this.readLengths();
          final int endChunk1 = endChunk - DECODING_BATCH_SIZE;
          int i = startChunk;

          try
          {
             // Fast decoding by reading several bits at a time from the bitstream
             for ( ; i<endChunk1; i++)
                array[i] = this.fastDecodeByte();

             // Regular decoding by reading one bit at a time from the bitstream
             for ( ; i<endChunk; i++)
                array[i] = this.decodeByte();
          }
          catch (BitStreamException e)
          {
             return i - blkptr;
          }

          startChunk = endChunk;
          sizeChunk = (startChunk + sz < end) ? sz : end - startChunk;
          endChunk = startChunk + sizeChunk;
       }

       return len;
    }


    // The data block header must have been read before
    @Override
    public byte decodeByte()
    {
       // Empty cache
       Node currNode = this.current.value;

       if (currNode != this.root)
          this.current = this.current.next;

       while ((currNode.left != null) || (currNode.right != null))
       {
          currNode = (this.bitstream.readBit() == 0) ? currNode.left : currNode.right;
       }

       return currNode.symbol;
    }


    // DECODING_BATCH_SIZE bits must be available in the bitstream
    protected final byte fastDecodeByte()
    {
       Node currNode = this.current.value;

       // Use the cache to find a good starting point in the tree
       if (currNode == this.root)
       {
          // Read more bits from the bitstream and fetch starting point from cache
          final int idx = (int) this.bitstream.readBits(DECODING_BATCH_SIZE);
          this.current = this.decodingCache[idx];
          currNode = this.current.value;
       }

       // The node symbol is 0 only if the node is not a leaf or it codes the value 0.
       // We need to check if it is a leaf only if the symbol is 0.
       if (currNode.symbol == 0)
       {
          while ((currNode.left != null) || (currNode.right != null))
          {
             currNode = (this.bitstream.readBit() == 0) ? currNode.left : currNode.right;
          }
       }

       // Move to next starting point in cache
       this.current = this.current.next;
       return currNode.symbol;
    }


    @Override
    public InputBitStream getBitStream()
    {
       return this.bitstream;
    }


    private static class Key implements Comparable<Key>
    {
       int code;
       short length;

       Key(short length, int code)
       {
          this.code = code;
          this.length = length;
       }

       @Override
       public int compareTo(Key o)
       {
          if (o == this)
             return 0;

          if (o == null)
             return 1;

          final Key k = (Key) o;
          final int len = this.length - k.length;

          if (len != 0)
              return len;

          return this.code - k.code;
       }
    }


    private static class CacheData
    {
       Node value;
       CacheData next;

       CacheData(Node value)
       {
          this.value = value;
       }
    }
}
