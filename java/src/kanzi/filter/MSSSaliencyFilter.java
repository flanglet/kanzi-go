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

package kanzi.filter;

import kanzi.ColorModelType;
import kanzi.SliceIntArray;
import kanzi.IntFilter;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.YCbCrColorModelConverter;


// Maximum Symmetric Surround Saliency Filter
// See [SALIENCY DETECTION USING MAXIMUM SYMMETRIC SURROUND]
// by Radhakrishna Achanta and Sabine Susstrunk.
// Proceedings of IEEE International Conference on Image Processing (ICIP), 2010.
// Fast integer based approximation using YUV rather than the slow LAB color model.
public final class MSSSaliencyFilter implements IntFilter
{    
    // Type of filter output (3 'pixel' output channels or 1 'cost' output channel)
    public static final int IMAGE = 0;
    public static final int COST = 1;
    
    private final int width;
    private final int height;
    private final int stride;
    private final boolean doColorTransform;
    private final IntFilter integralFilter;
    private final int mask;
    private int[] chanL1;
    private int[] chanA1;
    private int[] chanB1;
    private int[] chanL2;
    private int[] chanA2;
    private int[] chanB2;
    private int[] buf;
    

    public MSSSaliencyFilter(int width, int height)
    {
       this(width, height, width, true, IMAGE);
    }


    public MSSSaliencyFilter(int width, int height, int stride)
    {
       this(width, height, stride, true, IMAGE);
    }


    public MSSSaliencyFilter(int width, int height, int stride, boolean doColorTransform,
                             int filterType)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");
        
        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

        if ((filterType != COST) && (filterType != IMAGE))
            throw new IllegalArgumentException("Invalid filter type parameter (must be IMAGE or COST)");

        this.width = width;
        this.height = height;
        this.stride = stride;
        this.doColorTransform = doColorTransform;
        this.mask = (filterType == COST) ? 0xFF : -1;
        this.integralFilter = new IntegralImageFilter(width, height, stride);
        this.chanL1 = new int[0];
        this.chanA1 = new int[0];
        this.chanB1 = new int[0];
        this.buf = new int[0];
    }

   
   @Override
   public boolean apply(SliceIntArray input, SliceIntArray output)
   {
      if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
         return false;
      
      final int count = this.stride * this.height;
      
      // Lazy instantiation
      if (this.buf.length < count)
      {
         this.buf    = new int[count];
         this.chanL1 = new int[count];
         this.chanA1 = new int[count];
         this.chanB1 = new int[count];
         this.chanL2 = new int[count];
         this.chanA2 = new int[count];
         this.chanB2 = new int[count];
      }
      
      final int[] src = input.array;
      final int[] dst = output.array;
      int srcIdx = input.index;
      int dstIdx = output.index;
      final int h = this.height;
      final int w = this.width;
      
      SliceIntArray saL1 = new SliceIntArray(this.chanL1, 0);
      SliceIntArray saA1 = new SliceIntArray(this.chanA1, 0);
      SliceIntArray saB1 = new SliceIntArray(this.chanB1, 0);
      SliceIntArray saL2 = new SliceIntArray(this.chanL2, 0);
      SliceIntArray saA2 = new SliceIntArray(this.chanA2, 0);
      SliceIntArray saB2 = new SliceIntArray(this.chanB2, 0);
      SliceIntArray sa   = new SliceIntArray(this.buf,    0);

      // Create Gaussian and Integral images for 1 or 3 channels.
      if (this.doColorTransform == true)
      {
         ColorModelConverter cvt = new YCbCrColorModelConverter(this.width, this.height, srcIdx, this.stride);

         if (cvt.convertRGBtoYUV(src, this.chanL1, this.chanA1, this.chanB1, ColorModelType.YUV444) == false)
            return false;
         
         copyImage(this.chanA1, this.buf, w, h, 0, this.width, this.stride);

         if (this.integralFilter.apply(sa, saA2) == false)
            return false;

         if (this.gaussianSmooth(sa, saA1) == false)
            return false;

         copyImage(this.chanB1, this.buf, w, h, 0, this.width, this.stride);

         if (this.integralFilter.apply(sa, saB2) == false)
            return false;

         if (this.gaussianSmooth(sa, saB1) == false)
            return false;
      }
      else
      {
         // No color transform, use L channel as input data
         for (int i=0; srcIdx+i<count; i++)
         {
            this.chanL1[i] = src[srcIdx+i];
            this.chanA1[i] = 0;
            this.chanB1[i] = 0;
         }
      }
          
      copyImage(this.chanL1, this.buf, w, h, 0, this.width, this.stride);

      if (this.integralFilter.apply(sa, saL2) == false)
         return false;

      if (this.gaussianSmooth(sa, saL1) == false)
         return false;

      final int st = this.stride;
      int minVal = Integer.MAX_VALUE;
      int maxVal = 0;
      srcIdx = 0;

      // Compute distance of differences 
      for (int y=0; y<h; y++)
      {
         final int yoff	= Math.min(y, h-y);
         final int y1	= y - yoff;
         final int y2	= Math.min(y+yoff, h-1);
         
         for (int x=0; x<w; x++)
         {
            final int xoff	= Math.min(x, w-x);
            final int x1	= x - xoff;
            final int x2	= Math.min(x+xoff, w-1);
            final int area = (x2-x1+1) * (y2-y1+1);     
            final int offset1 = (y1-1) * st;
            final int offset2 = y2 * st;
            final int valL = getIntegralSum(this.chanL2, x1, offset1, x2, offset2) / area;
            final int valA = getIntegralSum(this.chanA2, x1, offset1, x2, offset2) / area;
            final int valB = getIntegralSum(this.chanB2, x1, offset1, x2, offset2) / area;
            final int idx = srcIdx + x;
            final int val1 = valL - this.chanL1[idx];
            final int val2 = valA - this.chanA1[idx];
            final int val3 = valB - this.chanB1[idx];
            final int val = (val1*val1) + (val2*val2) + (val3*val3); // non linearity (dist. square)            
            dst[dstIdx+x] = val; 
            
            if (val < minVal)
               minVal = val; 
           
            if (val > maxVal) 
               maxVal = (int) val; 
         }

         srcIdx += st;
         dstIdx += st;
      } 
      
      final int range = maxVal - minVal;
      dstIdx = output.index;
      
      if ((maxVal - minVal > 1) || (this.mask != -1))
      {
         final int scale = (255<<16) / range;
         
         for (int y=0; y<h; y++)
         {
            for (int x=0; x<w; x++)
            {
               final int val = (scale * (dst[dstIdx+x]-minVal)) >>> 16;
               dst[dstIdx+x] = ((val<<16) | (val<<8) | val) & this.mask;
            }

            dstIdx += st;
         }         
      }

      return true;
   }

   
   private static void copyImage(int[] input, int[] output, int w, int h, int offs, int stride1, int stride2)
   {
      if (input == output)
         return;
      
      if (stride1 == stride2)
      {
         // Copy full buffer
         System.arraycopy(input, offs, output, 0, input.length-offs);
         return;
      }
      
      int srcIdx = offs;
      int dstIdx = 0;
      
      for (int j=h-1; j>=0; j--)
      {         
         // Copy line by line to respect different strides
         System.arraycopy(input, srcIdx, output, dstIdx, w);
         srcIdx += stride1;
         dstIdx += stride2;
      }
   }
   
   
   private static int getIntegralSum(int[] data, int x1, int offset1, int x2, int offset2)
   {
      if (x1 <= 0)
      {
         if (offset1 <= 0)
            return data[offset2+x2];
         
         return data[offset2+x2] - data[offset1+x2];
      }

		if (offset1 <= 0)
			return data[offset2+x2] - data[offset2+x1-1];

      return data[offset2+x2] + data[offset1+x1-1] - data[offset1+x2] - data[offset2+x1-1];     
   }
   
   
   private boolean gaussianSmooth(SliceIntArray input, SliceIntArray output)
   {
      // Use a very small and inaccurate kernel (1, 2, 1)
      final int[] src = input.array;
      final int[] dst = output.array;
      final int w = this.width;
      final int h = this.height;
      final int st = this.stride;

      // Horizontal blur
      {
         int idx = output.index;
         int srcIdx = input.index;

         for (int j=0; j<h; j++)
         {
            int prv = src[srcIdx];
            int cur = src[srcIdx+1];
            int nxt;
            this.buf[idx] = prv;

            for (int i=1; i<w-1; i++)
            {
               nxt = src[srcIdx+i+1];
               this.buf[idx+i] = (prv + cur + cur + nxt + 2) >>> 2 ; 
               prv = cur;
               cur = nxt;
            }
            
            this.buf[idx+w-1] = cur;
            srcIdx += st;
            idx += st;            
         }
      }

      // Vertical blur
      {
         int idx = 0;
         int dstIdx = output.index;
         System.arraycopy(this.buf, idx, dst, dstIdx, w);
         idx += st;
         dstIdx += st;

         for (int j=1; j<h-1; j++)
         {
            for (int i=0; i<w; i++)
            {
               final int prv = this.buf[idx+i-st];
               final int cur = this.buf[idx+i];
               final int nxt = this.buf[idx+i+st];
               dst[dstIdx+i] = (prv + cur + cur + nxt + 2) >>> 2;
            }
            
            dstIdx += st;
            idx += st;
         }
         
         System.arraycopy(this.buf, idx, dst, dstIdx, w);
      }

      return true;
   } 
    
}
