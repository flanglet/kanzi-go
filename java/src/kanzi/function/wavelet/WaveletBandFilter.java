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

import kanzi.SliceIntArray;
import kanzi.IntFunction;


// Quantize, reorder and comb the wavelet sub-band coefficients before entropy coding.
// This filter being the lossy part of the compression pipeline, its implementation
// has a big impact on compression ratio and image quality.
// This implementation does not try to optimize bit rate/distortion.
// It is very basic compared to patented algorithms such as SPIHT or EBCOT but
// still yields good coding speed/quality/compression rate balance.
// The coefficients are reordered per sub-band (resolution scalability) but the
// resulting bitstream (after entropy coding) is not embedded.
// Not thread safe
public class WaveletBandFilter implements IntFunction
{
    private static final int IS_LEAF = 0x7F;
    private static final int MIN_NB_COEFFS_FOR_CLUSTER = 2;

    private final int width;
    private final int height;
    private final int levels;
    private final int minClusterSize;
    private boolean setQuantizers;
    private final int[] quantizers;
    private final int[] buffer;


    // image dimension, dimension of LL band, number of wavelet subband levels
    // and quantization value
    public WaveletBandFilter(int width, int height, int levels)
    {
        this(width, height, levels, null);
    }


    // Provide array of quantizers per level (if null, no quantization)
    public WaveletBandFilter(int width, int height, int levels, int[] quantizers)
    {
       this(width, height, levels, quantizers, MIN_NB_COEFFS_FOR_CLUSTER);
    }

    
    // Provide array of quantizers per level (if null, no quantization)
    public WaveletBandFilter(int width, int height, int levels,
            int[] quantizers, int minClusterSize)
    {
        if (width < 8)
            throw new IllegalArgumentException("The width of the image must be at least 8");

        if (height < 8)
            throw new IllegalArgumentException("The height of the image must be at least 8");

        if (levels < 2)
            throw new IllegalArgumentException("The number of wavelet sub-band levels must be at least 2");

        if ((quantizers != null) && (quantizers.length < levels+1))
            throw new IllegalArgumentException("Some sub-band levels have no quantizer value");

        if (minClusterSize < 0)
            throw new IllegalArgumentException("The minimum size of a coefficients cluster must be positive or null");

        if (minClusterSize > 8)
            throw new IllegalArgumentException("The minimum size of a coefficients cluster must be 8 at most (8 direct neighbors)");

        this.width = width;
        this.height = height;
        this.levels = levels;
        this.minClusterSize = minClusterSize;

        if (quantizers == null)
        {
            this.quantizers = new int[this.levels+1];
            this.setQuantizers = true;
        }
        else
        {
            this.quantizers = new int [quantizers.length];
            System.arraycopy(quantizers, 0, this.quantizers, 0, quantizers.length);
            this.setQuantizers = false;
        }

        this.buffer = new int[16384];
    }


    public boolean getQuantizers(int[] quantizers_)
    {
        if (quantizers_ == null)
            return false;

        if (quantizers_.length != this.quantizers.length)
            return false;

        System.arraycopy(this.quantizers, 0, quantizers_, 0, this.quantizers.length);
        return true;
    }


    public boolean setQuantizers(int[] quantizers_)
    {
        if (quantizers_ == null)
            return false;

        if (quantizers_.length != this.quantizers.length)
            return false;

        System.arraycopy(quantizers_, 0, this.quantizers, 0, this.quantizers.length);
        return true;
    }


    @Override
    public boolean forward(SliceIntArray source, SliceIntArray destination)
    {
        int srcIdx = source.index;
        int dstIdx = destination.index;
        final int[] src = source.array;
        final int[] dst = destination.array;
        final int w0 = this.width >> this.levels;
        final int h0 = this.height >> this.levels;
        
        this.quantize(src, srcIdx, this.quantizers);

        if (this.minClusterSize > 0)
            this.filter(src, srcIdx);

        // Process LL band
        for (int j=0, offset=srcIdx; j<h0; j++)
        {
            for (int i=0; i<w0; i++)
                dst[dstIdx++] = src[offset+i];

            offset += this.width;
        }

        // Reorder the coefficients and remove those under leaves
        int start = srcIdx;
        WaveletBandScanner sc = new WaveletBandScanner(this.width,
                this.height, WaveletBandScanner.ALL_BANDS, this.levels);
        
        final int w = this.width;
        int count = 0;

        // Process sub-bands
        while (count < sc.getSize())
        {
            final int len = sc.getIndexes(this.buffer, this.buffer.length, count);
            count += len;

            for (int i=0; i<len; i++, srcIdx++)
            {
                final int idx = start + this.buffer[i];

                if (src[idx] != IS_LEAF)
                {
                    dst[dstIdx++] = src[idx];
                    continue;
                }

                // Check if the parent coefficient is a leaf, if so skip
                final int x = (idx % w) >> 1;
                final int y = (idx / w) >> 1;

                if ((x < w0) && (y < h0))
                {
                    dst[dstIdx++] = IS_LEAF;
                }
                else if (src[start+(y*w)+x] != IS_LEAF)
                {
                    dst[dstIdx++] = IS_LEAF;
                }
            }
        }

        source.index = srcIdx;
        destination.index = dstIdx;
        return true;
    }


    protected void quantize(int[] block, int srcIdx, int[] qt)
    {
       WaveletBandScanner sc = new WaveletBandScanner(this.width,
                this.height, WaveletBandScanner.ALL_BANDS, this.levels);

       final int w = this.width;
       final int w0 = w >> this.levels;
       final int h0 = this.height >> this.levels;
       int levelSize = 3 * w0 * h0;
       
       if (this.setQuantizers == true)
       {
          // Find max in LL band
          int max = 0;
          final int end = srcIdx + (w * h0);

          for (int offset=srcIdx; offset<end; offset+=w)
          {
             for (int j=0; j<w0; j++)
             {
                int val = block[offset+j];
                val = (val + (val >> 31)) ^ (val >> 31); // abs

                if (val > max)
                   max = val;
             }
          }

          int q0 = max >> 9;
          int q1 = q0 + (q0 >> 3);
          int minErr = Integer.MAX_VALUE;
          int bestQ = q0;

          for (int q=q0; q<=q1; q++)
          {
             int err = 0;
             int adjust = q >> 1;

             for (int offset=srcIdx; offset<end; offset+=w)
             {
                for (int j=0; j<w0; j++)
                {
                   int val = block[offset+j];
                   int scaled = (q == 0) ? val : (val + adjust) / q;

                   if (scaled > 127)
                      scaled = 127;

                   int diff = val - (q * scaled);
                   err += (diff >= 0) ? diff : -diff;

                   if (err > minErr)
                      break;
                }
             }
             
             if (err < minErr)
             {
                minErr = err;
                bestQ = q;
             }
          }

          this.quantizers[0] = bestQ;
          int[] indexes = new int[levelSize];
          final int len = sc.getBandIndexes(indexes, w0, h0, 0);
          max = 0;

          for (int i=0; i<len; i++)
          {
             int val = block[srcIdx+indexes[i]];
             val = (val + (val >> 31)) ^ (val >> 31); // abs

             if (val > max)
                max = val;
          }

          this.quantizers[1] = ((max >> 8) + 1 < 18) ? (max >> 8) + 1 : 18;

          for (int i=2; i<this.quantizers.length; i++)
          {
             // Derive quantizer values for higher bands
             this.quantizers[i] = ((this.quantizers[i-1] * 17) + 2) >> 4;
          }

          this.setQuantizers = false;
       }

       int level = 0;
       int quant = qt[level];
       int adjust = quant >> 1;

       // Quantize LL band
       for (int j=0, offset=srcIdx; j<h0; j++, offset+=w)
       {
          for (int i=0; i<w0; i++)
          {
             final int val = block[offset+i];

             if (val > 0)
                block[offset+i] = (val + adjust) / quant;
             else if (val < 0)
                block[offset+i] = (val - adjust) / quant;
          }
       }

       int buffSize = levelSize;
       int startHHBand = (levelSize + levelSize) / 3;
       int levelWritten = 0;
       level++;
       quant = qt[level];
       int count = 0;

       // Process sub-bands
       while (count < sc.getSize())
       {
          // Quantize per level (3 sub-bands)
          final int processed = sc.getIndexes(this.buffer, buffSize, count);
          count += processed;
          adjust = quant >> 1;

          // Process LH, HL and HH bands
          for (int n=0; n<processed; n++)
          {
             final int idx = srcIdx + this.buffer[n];
             int val = block[idx];

             if (val > 0)
                val = (val + adjust) / quant;
             else if (val < 0)
                val = (val - adjust) / quant;

             // Avoid 'key' value (will be used to encode 'no descendant')
             // Introduces a very small error
             block[idx] = (val != IS_LEAF) ? val : val - 1;
 
             // Use bigger quantizer value for HH sub-band
             if (levelWritten + n == startHHBand)
             {
                quant = (quant * 9) >> 3;
                adjust = quant >> 1;
             }
          }
          
          levelWritten += processed;

          if (levelWritten == levelSize)
          {
             levelSize <<= 2;
             levelWritten = 0;
             level++;
             startHHBand = (levelSize + levelSize) / 3;

             if (level < qt.length)
                quant = qt[level];
          }

          buffSize = levelSize - levelWritten;

          if (buffSize > this.buffer.length)
              buffSize = this.buffer.length;
       }
    }


    protected void filter(int[] block, int srcIdx)
    {
        // Keep only clusters of coefficients, remove individual coefficients
        final int endi = this.width - 1;
        final int endj = this.height - 1;
        final int w0 = this.width >> this.levels;
        final int h0 = this.height >> this.levels;
        final int w1 = w0 << 1;
        final int h1 = h0 << 1;
        int offset = 0;

        for (int j=1; j<endj; j++)
        {
            offset += this.width;
            int parentOffset = (j >> 1) * this.width;

            for (int i=1; i<endi; i++)
            {
                // Ignore LL band
                if ((j < h0) && (i < w0))
                    continue;

                // Check neighbors, ignore inter-band correlation
                int val = 0;
                int idx = offset - this.width + i;

                if (block[idx-1] != 0)
                   val++;

                if (block[idx] != 0)
                   val++;

                if (block[idx+1] != 0)
                   val++;

                idx += this.width;

                // Check parent (super-band correlation)
                if ((j >= h1) && (i >= w1))
                {
                    if (block[parentOffset+(i>>1)] == 0)
                        val -= 2;
                }

                if (block[idx-1] != 0)
                   val++;

                if (block[idx+1] != 0)
                   val++;

                idx += this.width;

                if (block[idx-1] != 0)
                   val++;

                if (block[idx] != 0)
                   val++;

                if (block[idx+1] != 0)
                   val++;

                // Cut the pixels with few neighbors/parents
                if (val <= MIN_NB_COEFFS_FOR_CLUSTER)
                   block[offset+i] = 0;
            }
        }

        // Scan LL band and find descendants to each coefficient (one pass)
        final int widthLLBand = this.width >> this.levels;
        final int heightLLBand = this.height >> this.levels;
        final int halfW = widthLLBand >> 1;
        final int halfH = heightLLBand >> 1;
        Context ctx = new Context(block, srcIdx, this.width, this.height);

        for (int j=0; j<heightLLBand; j++)
        {
            for (int i=0; i<widthLLBand; i++)
            {
               if ((i < halfW) && (j < halfH))
                  continue;

               ctx.x = i + i;
               ctx.y = j + j;
               findDescendants(ctx);
            }
        }
    }


    // This recursive search for descendants is a computational bottleneck
    // (called for each pixel except those in the LL band)
    private static boolean findDescendants(Context ctx)
    {
        int x = ctx.x;
        int y = ctx.y;
        int[] block = ctx.block;
        int offset = ctx.srcIdx + (y * ctx.width) + x;
        int leaves = 0;
        x <<= 1;
        y <<= 1;

        for (int j=y; j<=y+2; j+=2)
        {
            for (int i=x, k=0; i<=x+2; i+=2, k++)
            {
                if ((i >= ctx.width) || (j >= ctx.height))
                {
                    if (block[offset+k] == 0)
                    {
                        // Substitute value for LEAF symbol
                        block[offset+k] = IS_LEAF;
                        leaves++;
                    }
                }
                else
                {
                    // Inline this method to avoid one level of recursion
                    // By avoiding the last level of recursion, the number of
                    // calls to this method is reduced by 4.
                    int offset2 = ctx.srcIdx + (j * ctx.width) + i;
                    int leaves2 = 0;
                    final int i2 = i << 1;
                    final int j2 = j << 1;

                    for (int jj=j2; jj<=j2+2; jj+=2)
                    {
                        for (int ii=i2, kk=0; ii<=i2+2; ii+=2, kk++)
                        {
                            ctx.x = ii;
                            ctx.y = jj;

                            if (((ii >= ctx.width) || (jj >= ctx.height)
                                    || (findDescendants(ctx) == false))
                                    && (block[offset2+kk] == 0))
                            {
                                // Substitute value for LEAF symbol
                                block[offset2+kk] = IS_LEAF;
                                leaves2++;
                            }
                        }

                        offset2 += ctx.width;
                    }

                    if ((leaves2 == 4) && (block[offset+k] == 0))
                    {
                        // Substitute value for LEAF symbol
                        block[offset+k] = IS_LEAF;
                        leaves++;
                    }
                }
            }

            offset += ctx.width;
        }

        return (leaves != 4);
    }


    // The filter MUST know the levels and quantizers !!!
    @Override
    public boolean inverse(SliceIntArray source, SliceIntArray destination)
    {
        int srcIdx = source.index;
        int dstIdx = destination.index;
        int[] src = source.array;
        int[] dst = destination.array;
        int end8 = destination.index & 0xFFFFFFF8;

        for (int i=0; i<end8; i+=8)
        {
            dst[i]   = 0;
            dst[i+1] = 0;
            dst[i+2] = 0;
            dst[i+3] = 0;
            dst[i+4] = 0;
            dst[i+5] = 0;
            dst[i+6] = 0;
            dst[i+7] = 0;
        }

        for (int i=end8; i<dst.length; i++)
            dst[i] = 0;

        int quant = this.quantizers[0];
        final int w = this.width;
        final int w0 = w >> this.levels;
        final int h = this.height;
        final int h0 = h >> this.levels;
        final int end = dstIdx + (w * h0);

        // Process LL band
        for (int offset=dstIdx; offset<end; offset+=w)
        {
            for (int i=0; i<w0; i++, srcIdx++)
                dst[offset+i] = src[srcIdx] * quant;
        }

        // Process sub-bands: insert leaves under coefficients tagged as LEAF
        final int start = dstIdx;
        int levelSize = 3 * w0 * h0;
        int buffSize = levelSize;
        int startHHBand = (levelSize + levelSize) / 3;
        int levelRead = 0;
        int level = 1;
        quant = this.quantizers[level];

        // Use a it to reorder the coefficients from the source array
        // Do NOT provide the number of levels to the it: ALL the bands
        // must be scanned (even the ones not in the source) to fully process
        // the leaves (need to insert 0s in missing sub-bands).
        WaveletBandScanner sc = new WaveletBandScanner(this.width,
                this.height, WaveletBandScanner.ALL_BANDS, this.levels);
        int count = 0;

        while (count < sc.getSize())
        {
            // Retrieve indexes level by level
            final int processed = sc.getIndexes(this.buffer, buffSize, count);
            count += processed;
            for (int i=0; i<processed; i++, dstIdx++)
            {
                int idx = start + this.buffer[i];

                if ((dst[idx] == IS_LEAF) || (src[srcIdx] == IS_LEAF))
                {
                    if (dst[idx] != IS_LEAF)
                        srcIdx++;

                    dst[idx] = 0;

                    // Mark children as leaves
                    final int x = (idx % w) << 1;
                    final int y = (idx / w) << 1;

                    if ((x < w) && (y < h))
                    {
                        idx <<= 1;
                        dst[idx] = IS_LEAF;
                        dst[idx+1] = IS_LEAF;
                        idx += w;
                        dst[idx] = IS_LEAF;
                        dst[idx+1] = IS_LEAF;
                    }

                    continue;
                }

                // Restore bigger quantizer value for HH band
                if (levelRead + i == startHHBand)
                    quant = (quant * 9) >> 3;

                // Reverse quantization (approximate)
                dst[idx] = src[srcIdx] * quant;
                srcIdx++;
            }
            
            levelRead += processed;

            if (levelRead == levelSize)
            {
               levelSize <<= 2;
               levelRead = 0;
               level++;
               startHHBand = (levelSize + levelSize) / 3;

               if (level < this.quantizers.length)
                  quant = this.quantizers[level];
            }

            buffSize = levelSize - levelRead;

            if (buffSize > this.buffer.length)
               buffSize = this.buffer.length;
        }

        source.index = srcIdx;
        destination.index = dstIdx;
        return true;
    }


    private static class Context
    {
       final int[] block;
       final int width;
       final int height;
       int srcIdx;
       int x;
       int y;

       Context(int[] block, int srcIdx, int width, int height)
       {
          this.block = block;
          this.srcIdx = srcIdx;
          this.width = width;
          this.height = height;
       }
    }
    
      

   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      return srcLen;
   }
}