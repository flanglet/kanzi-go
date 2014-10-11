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

import kanzi.ArrayComparator;
import kanzi.util.sort.QuickSort;


// Tree utility class for a canonical implementation of Huffman codec
public final class HuffmanTree
{
    // Return the number of codes generated
    public static int generateCanonicalCodes(short[] sizes, int[] codes, int[] ranks, int count)
    {
       // Sort by increasing size (first key) and increasing value (second key)
       if (count > 1)
       {
          QuickSort sorter = new QuickSort(new HuffmanArrayComparator(sizes));
          sorter.sort(ranks, 0, count);
       }

       int code = 0;
       int len = sizes[ranks[0]];

       for (int i=0; i<count; i++)
       {
          final int r = ranks[i];

          if (sizes[r] > len)
          {
             code <<= (sizes[r] - len);
             len = sizes[r];

             // Max length reached
             if (len > 24)
                return -1;
          }

          codes[r] = code;
          code++;
       }

       return count;
    }


    // Huffman node
    public static class Node implements Comparable<Node>
    {
       protected final int weight;
       protected final byte symbol;
       protected Node left;
       protected Node right;


       // Leaf
       Node(byte symbol, int frequency)
       {
          this.weight = frequency;
          this.symbol = symbol;
       }


       // Not leaf
       Node(Node left, Node right)
       {
          this.weight = left.weight + right.weight;
          this.symbol = left.symbol; // Critical to resolve ties during node sorting !
          this.left = left;
          this.right = right;
       }


       @Override
       public boolean equals(Object o)
       {
           if (o == null)
               return false;
     
           if (o == this)
               return true;
     
           return this.symbol == ((Node) o).symbol; 
       }

       
       @Override
       public int hashCode()
       {
          return this.symbol;
       }
       
       
       @Override
       public int compareTo(Node o)
       {
          if (o == null)
             return 1;

          if (o == this)
             return 0;
   
          if (this.weight != o.weight) 
             return this.weight - o.weight;
          
          if (this.left == null) 
          {
             if (o.left != null)
                return -1;
          } 
          else if (o.left == null)
             return 1;
                     
          return (this.symbol & 0xFF) - (o.symbol & 0xFF);
       }
    }


    // Array comparator used to sort keys and values to generate canonical codes
    private static class HuffmanArrayComparator implements ArrayComparator
    {
        private final short[] array;


        public HuffmanArrayComparator(short[] sizes)
        {
            if (sizes == null)
                throw new NullPointerException("Invalid null array parameter");

            this.array = sizes;
        }


        @Override
        public int compare(int lidx, int ridx)
        {
            // Check size (natural order) as first key
            final int res = this.array[lidx] - this.array[ridx];

            // Check index (natural order) as second key
            return (res != 0) ? res : lidx - ridx;
        }
    }
}