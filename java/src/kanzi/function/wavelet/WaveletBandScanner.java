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

package kanzi.function.wavelet;

// Works on images post wavelet transform
// Uses oriented raster scan (horizontal, then vertical, then diagonal)
// Sub-bands:
// LL HL (low)      (horizontal)
// LH HH (vertical) (diagonal)
// Example:
//  0  1  2  3
//  4  5  6  7
//  8  9 10 11
// 12 13 14 15
// scanner => 2 3 6 7 8 12 9 13 10 14 11 15
// Ignore L0 (0 1 4 5)
// Horizontal (HL) 2 3 6 7
// Vertical (LH) 8 12 9 13
// Diagonal (HH) 10 14 11 15
public class WaveletBandScanner
{
    public static final int HL_BAND = 1;
    public static final int LH_BAND = 2;
    public static final int HH_BAND = 4;
    public static final int ALL_BANDS = HL_BAND | HH_BAND | LH_BAND;
    
    private final int width;
    private final int height;
    private final int levels;
    private final int bandType;
    private final int size;
    
      
    // levels is used to limit the scanning to a subset of all bands
    public WaveletBandScanner(int width, int height, int bandType, int levels)
    {
        if (height < 2)
            throw new IllegalArgumentException("Invalid height parameter (must be at least 8)");
        
        if (width < 8)
            throw new IllegalArgumentException("Invalid width parameter (must be at least 8)");
         
        if (((bandType & HL_BAND) == 0) && ((bandType & LH_BAND) == 0) && ((bandType & HH_BAND) == 0))
            throw new IllegalArgumentException("Invalid bandType parameter");
        
        if (levels < 1)
            throw new IllegalArgumentException("Invalid levels parameter (must be at least 1)");

        this.width = width;
        this.height = height;
        this.bandType = bandType;
        this.levels = levels;
        int sz = 0;
        int subtreeSize = 0;
        int x0 = this.width >> this.levels;
        int y0 = this.height >> this.levels;
        int x = x0;
        int y = y0;
         
        for (int i=0; i<levels; i++)
        {
          subtreeSize += (x * y);
          x <<= 1;
          y <<= 1;
        }
        
        if ((this.bandType & HL_BAND) != 0)
            sz += subtreeSize;
        
        if ((this.bandType & LH_BAND) != 0)
            sz += subtreeSize;
        
        if ((this.bandType & HH_BAND) != 0)
            sz += subtreeSize;
        
        this.size = sz;
    }
    
    
    public int getSize()
    {
        return this.size;
    }
    
       
    // Read a chunk of the subtree of size length
    // Allows the use of an array much smaller than the tree
    // Return the number of integers put in the provided array
    public int getIndexes(int[] block, int length, int offset)
    {
        if (offset >= this.size)
           return 0;
        
        if (length > block.length)
           length = block.length;
        
        final int initialW = this.width >> this.levels;
        final int initialH = this.height >> this.levels;
        int w = initialW;
        int h = initialH;
        int offsetInBand = 0;
        int level = 0;
        
        // Find offset in band
        if (offset > 0)
        {
            int count = 0;
            int previousCount = 0;
            
            // Remove already scanned bands
            for (level=0; level<this.levels; level++)
            {
                if ((this.bandType & HL_BAND) != 0)
                   count += (w * h);
                
                if ((this.bandType & HH_BAND) != 0)
                   count += (w * h);
                
                if ((this.bandType & LH_BAND) != 0)
                   count += (w * h);
                
                if (count > offset)
                   break;
                
                w <<= 1;
                h <<= 1;
                previousCount = count;
            }
            
            offsetInBand = offset - previousCount;
        }
        
        int count = 0;
        
        while (level < this.levels)
        {
            // Scan sub-band by sub-band with increasing dimension
            count += this.getBandIndexes(block, w, h, count, offsetInBand);
            offsetInBand = 0;
            
            if (count >= length)
                break;
            
            w <<= 1;
            h <<= 1;
            level++;
        }
        
        return count;
    }
    
    
    // Read chunk of band of dimension 'dim' filtered by band type
    // Return the number of integers put in the provided array
    protected int getBandIndexes(int[] block, int w, int h, int blockIdx, int offsetInBand)
    {
        if ((w >= this.width) || (h >= this.height))
            return 0;
        
        int idx = blockIdx;
        int mult = h * this.width;
        int count = 0;
        
        // HL band: horizontal scan
        if (((this.bandType & HL_BAND) != 0) && (idx < block.length))
        {
            final int end = w + mult;
            
            for (int offs=w; offs<end; offs+=this.width)
            {   
                if (count + w < offsetInBand)
                {
                  count += w;
                  continue;
                }
                
                final int endStep = offs + w;
                
                for (int i=offs; i<endStep; i++, count++)
                {
                  if (count < offsetInBand)
                     continue;
                                    
                  if (idx == block.length)
                     return idx - blockIdx;
                    
                  block[idx++] = i;
                }
            }
        }
                
        // LH band: vertical scan
        if (((this.bandType & LH_BAND) != 0) && (idx < block.length))
        {
            final int end = w + mult;
            
            for (int offs=mult; offs<end; offs++)
            {
                if (count + h < offsetInBand)
                {
                  count += h;
                  continue;
                }

                final int endStep = offs + mult;

                for (int i=offs; i<endStep; i+=this.width, count++)
                {
                  if (count < offsetInBand)
                     continue;
                                  
                  if (idx == block.length)
                     return idx - blockIdx;

                  block[idx++] = i;
                }
            }
        }

        // HH band: diagonal scan (from lower left to higher right)
        if (((this.bandType & HH_BAND) != 0) && (idx < block.length))
        {
            final int min = (w < h) ? w : h;
            int offset = w + mult;
            
            for (int j=0; j<min; j++)
            {
                int offs = offset;
                
                for (int i=0; i<=j; i++, count++)
                {
                   if (count < offsetInBand)
                   {
                     offs -= this.width;
                     continue;
                   }
                                  
                   if (idx == block.length)
                     return idx - blockIdx;
                   
                   block[idx++] = offs + i;                 
                   offs -= this.width;
                }
                
                offset += this.width;
            }
            
            for (int j=min; j<h; j++)
            {
                int offs = offset;
                
                for (int i=0; i<w; i++, count++)
                {
                   if (count < offsetInBand)
                   {
                      offs -= this.width;
                      continue;
                   }
                    
                   if (idx == block.length)
                      return idx - blockIdx;
                      
                   block[idx++] = offs + i;
                   offs -= this.width;
                }
                
                offset += this.width;
            }
        
            offset = w + mult + mult - this.width + 1;
            
            for (int i=min; i<w; i++)                
            {
                int offs = offset;
                
                for (int j=0; j<h; j++, count++)
                {
                   if (count < offsetInBand)
                   {    
                      offs -= (this.width - 1);
                      continue;
                   }
                   
                   if (idx == block.length)
                     return idx - blockIdx;

                   block[idx++] = offs;
                   offs -= (this.width - 1);
                }
                
                offset++;
            }
            
            
            for (int i=1; i<min; i++)
            {
                int offs = offset;
                
                for (int j=0; j<min-i; j++, count++)
                {
                   if (count < offsetInBand)
                   {
                      offs -= (this.width - 1);
                      continue;                     
                   }
                     
                   if (idx == block.length)
                     return idx - blockIdx;
                     
                   block[idx++] = offs;
                   offs -= (this.width - 1);
                }
                
                offset++;
            }
        }
       
        return idx - blockIdx;
    }
    
    
    // Read whole tree (except top LL band) filtered by band type
    // Max speed compared to partial scan
    // Return the number of integers put in the provided array
    public int getIndexes(int[] block)
    {
        int w = this.width >> this.levels;
        int h = this.height >> this.levels;
        int count = 0;
        
        for (int i=0; i<this.levels; i++)
        {
             // Scan sub-band by sub-band with increasing dimension
            count += this.getBandIndexes(block, w, h, count, 0);
            
            if (count >= block.length)
               break;

            w <<= 1;
            h <<= 1;
        }
        
        return count;
    }
    
    
    // Read band of dimensions w & h filtered by band type
    // Return the number of integers put in the provided array
    public int getBandIndexes(int[] block, int w, int h, int blockIdx)
    {
       return this.getBandIndexes(block, w, h, blockIdx, 0);
    }
}
