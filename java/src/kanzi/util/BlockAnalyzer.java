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

package kanzi.util;


// Utility class to analyze a block in a frame: calculate energy and 'most important'
// pixels
public class BlockAnalyzer
{
    private final int[] block;
    private final int[] scanTable;
    private final int blockDim;
    private final int logBlockDim;
    private final int stride;
    private final int[] bins;
    private final int logWidth;

    
    public BlockAnalyzer(int blockDim, int width)
    {
        if (width < 8)
            throw new IllegalArgumentException("The width parameter must be at least 8");

        if ((blockDim != 8) && (blockDim != 16))
            throw new IllegalArgumentException("The block dimension must be either 8 or 16");

        this.blockDim = blockDim;
        this.logBlockDim = (blockDim == 8) ? 3 : 4;
        this.scanTable = new HilbertCurveGenerator(blockDim).generate(new int[blockDim*blockDim]);
        this.stride = width;
        this.block = new int[blockDim * blockDim];
        this.bins = new int[256];
        int log = -1;

        // Is width a power of 2 ? If so, calculate and save log
        if  ((width & (width - 1)) == 0)
        {
          log = 0;

          for (int y=width+1; y>1; y>>=1)
            log++;
        }

        this.logWidth = log;
    }


    // Compute the energy of a block as a sum of square pixel value differences
    public int computeEnergy(int[] input, int offset)
    {
       int energy = 0;
       final int length = this.block.length;
       final int[] scanOrder = this.scanTable; // use Peano-Hilbert ordering
       final int mask = this.blockDim - 1;
       int pos = scanOrder[0];
       int idx = (pos & mask) + (pos >> this.logBlockDim) * this.stride;
       int val = input[offset+idx];
       final int lgbd = this.logBlockDim;

       // If the width is a power of 2, use shift to calculate index
       if (this.logWidth >= 0)
       {
          final int lgw = this.logWidth;
          
          for (int i=1; i<length; i++)
          {
             pos = scanOrder[i];
             idx = (pos & mask) + ((pos >> lgbd) << lgw);
             final int next = input[offset+idx];
             final int diff = next - val;
             energy += (diff * diff);
             val = next;
         }
      }
      else // use multiplication to calculate index
      {
         final int w = this.stride;
  
         for (int i=1; i<length; i++)
         {
             pos = scanOrder[i];
             idx = (pos & mask) + ((pos >> lgbd) * w);
             final int next = input[offset+idx];
             final int diff = next - val;
             energy += (diff * diff);
             val = next;
          }
       }

       return energy;
    }


    // Extract the hotspots (most significant pixels) from the block and put their
    // indexes in the output array. The hotspots are defined as the pixels in the
    // block with the highest difference in value vs. their neighbors
    // The output array (indexes) may not be sorted
    // input contains integers in the [0..255] range
    // Return an estimate of the energy in the block
    // Not thread safe
    public int findHotspots(int[] input, int offset, int[] output, int nbHotspots)
    {
        if ((nbHotspots < 1) || (input == null) || (output == null))
           return -1;

        int energy = 0;
        final int[] block_ = this.block;
        final  int length = (nbHotspots > block_.length) ? block_.length : nbHotspots;

        // If all the pixels are requested, no need to search
        if (length == block_.length)
        {
           for (int i=block_.length-1; i>=0; i--)
             this.block[i] = i;

           return this.computeEnergy(input, offset);
        }

        // Turn the block into a curve using the Peano-Hilbert scanning
        // Then we just need to calculate the pixel value difference with its predecessor
        final int[] bins_ = this.bins;
        final int[] scanTable_ = this.scanTable;
        final int mask = this.blockDim - 1;
        final int w = this.stride;
        final int lgw = this.logWidth;
        final int lgbd = this.logBlockDim;
        int pos = scanTable_[0];
        int idx = (pos & mask) + (pos >> this.logBlockDim) * this.stride;
        int val = input[offset+idx];
        final int len = block_.length;

        // Save the absolute difference for each pixel vs. its predecessor
        // (the first one is always 0)

        // If the width is a power of 2, use shift to calculate index
        if (lgw >= 0)
        {
           for (int i=1; i<len; i++)
           {
               pos = scanTable_[i];
               idx = (pos & mask) + ((pos >> lgbd) << lgw);
               final int next = input[offset+idx];
               int diff = next - val;
               energy += (diff * diff);
               diff = (diff + (diff >> 31)) ^ (diff >> 31); // abs
               block_[i] = diff;
               bins_[diff]++;
               val = next;
           }
        }
        else // use multiplication to calculate index
        {
           for (int i=1; i<len; i++)
           {
               pos = scanTable_[i];
               idx = (pos & mask) + ((pos >> lgbd) *w);
               int next = input[offset+idx];
               int diff = next - val;
               diff = (diff + (diff >> 31)) ^ (diff >> 31); // abs
               energy += (diff * diff);
               block_[i] = diff;
               bins_[diff]++;
               val = next;
           }
        }

        val = 0;
        int n = 255;
        int valThreshold = 0;

        // Find index of bin corresponding to 'length' hotspots and clear bins
        for (; ((n >= 0) && (val < length)); n--)
        {
           valThreshold = bins_[n];
           val += valThreshold;
           bins_[n] = 0;
        }

        final int overshoot = val - length;
        final int diffThreshold = n + 1;
        int partialCount = valThreshold - overshoot;
        idx = 0;

        // Complete clearing of bins
        while (n >= 0)
           bins_[n--] = 0;

        // Now extract the points with the highest block values (hotspots)
        // Any block value with a diff higher than diffThreshold is added to output
        for (n=len-1; n>=1; n--)
        {
           int diff = block_[n] - diffThreshold;

           if (diff < 0)
              continue;

           if (diff == 0)
           {
              // Add partially the values in bins_[diffThreshold]
              partialCount--;

              if (partialCount < 0)
                 break;
           }

           output[idx++] = scanTable_[n];

           if (idx == length)
              break;
        }

        if (idx < length)
        {
           // Faster loop with no partial bin
           for (n--; n>=1; n--)
           {
              if (block_[n] <= diffThreshold)
                 continue;

              output[idx++] = scanTable_[n];

              if (idx == length)
                 break;
           }
        }

        return energy;
    }
    
}


// --- Example: find hot spots ---

// Initial block
// 121 121 120 121 120 121 119 117
// 114 113 113 111 111 109 109 107
// 90  88  87  86  85  84  83  83
// 87  87  84  84  84  83  83  83
// 86  86  87  87  88  89  89  89
// 89  89  90  91  94  95  96  96
// 90  91  91  91  92  93  93  93
// 89  88  88  89  89  88  87  87
//
// public static final int[] PEANO_HILBERT_SCAN_TABLE_8x8 =
// {  0,  1,  9,  8, 16, 24, 25, 17,
//   18, 26, 27, 19, 11, 10,  2,  3,
//    4, 12, 13,  5,  6,  7, 15, 14,
//   22, 23, 31, 30, 29, 21, 20, 28,
//   36, 44, 45, 37, 38, 39, 47, 46,
//   54, 55, 63, 62, 61, 53, 52, 60,
//   59, 58, 50, 51, 43, 35, 34, 42,
//   41, 33, 32, 40, 48, 49, 57, 56
// };
//
// Initial block with transitions:
// 121-121 120-121-120 121-119-117
//      |   |       |   |       |
// 114-113 113-111 111-109 109-107
//  |           |           |
// 090 088-087 086 085-084 083-083
//  |   |   |   |   |   |       |
// 087-087 084-084 084 083-083-083
//                 |
// 086-086 087-087 088 089-089-089
//  |   |   |   |   |   |       |
// 089 089-090 091 094-095 096-096
//  |           |           |
// 090-091 091-091 092-093 093-093
//      |   |       |   |       |
// 089-088 088-089-089 088-087-087
//
//
// Reordered diff
// 0, 0, 8, 1, 24, 3, 0, 1,
// 1, 3, 0, 2, 25, 2, 7, 1,
// 1, 9, 2, 12, 2, 2, 10, 2,
// ...
//
// 16 hotspots:
// 57, 35, 53, 63, 47, 37, 44, 36, 22, 15, 05, 12, 02, 11, 16, 09