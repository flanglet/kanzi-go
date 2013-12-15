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
    public static int generateCanonicalCodes(short[] sizes, int[] codes)
    {
       final int[] array = new int[sizes.length];
       int n = 0;

       for (int i=0; i<array.length; i++)
       {
          codes[i] = 0;

          if (sizes[i] > 0)
             array[n++] = i;
       }
       
       // Sort by decreasing size (first key) and increasing value (second key)
       QuickSort sorter = new QuickSort(new HuffmanArrayComparator(sizes));
       sorter.sort(array, 0, n);
       int code = 0;
       int len = sizes[array[0]];

       for (int i=0; i<n; i++)
       {
          final int idx = array[i];
          final int currentSize = sizes[idx];

          if (len > currentSize)
          {
             code >>= (len - currentSize);
             len = currentSize;
          }

          codes[idx] = code;
          code++;
       }
       
       return n;
    }
    

    // Huffman node
    public static class Node
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
       Node(int frequency, Node node1, Node node2)
       {
          this.weight = frequency;
          this.symbol = 0;
          this.left  = node1;
          this.right = node2;
       }
    }


    // Array comparator used to sort keys and values to generate canonical codes
    private static class HuffmanArrayComparator implements ArrayComparator
    {
        private final short[] array;
        

        public HuffmanArrayComparator(short[] array)
        {
            if (array == null)
                throw new NullPointerException("Invalid null array parameter");

            this.array = array;
        }


        @Override
        public int compare(int lidx, int ridx)
        {
            // Check sizes (reverse order) as first key
            final int res = this.array[ridx] - this.array[lidx];
            
            if (res != 0)
               return res;

            // Check value (natural order) as second key
            return lidx - ridx;
        }
    }  
}