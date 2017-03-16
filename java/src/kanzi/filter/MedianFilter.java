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

import kanzi.SliceIntArray;
import kanzi.IntFilter;


// Implementation of a constant time median filter.
// See [Median Filtering in Constant Time] by Simon Perreault & Patrick Hebert
// https://nomis80.org/ctmf.pdf
// As an addition, an optional threshold can be provided to keep input pixels 
// that differ too much from their associated median. It gives the ability to keep
// edges and blur smooth areas.
public final class MedianFilter implements IntFilter
{
   // Type of input filter: use RGB for 3 channels, or 1 channel (use B for Y, or U or V)
   public static final int THREE_CHANNELS = 0x0310; // 3 channels with shifts 16,8,0
   public static final int R_CHANNEL = 0x0110;
   public static final int G_CHANNEL = 0x0108;
   public static final int B_CHANNEL = 0x0100;
   public static final int DEFAULT_RADIUS = 3;
   public static final int DEFAULT_THRESHOLD = 256;

   private final int width;
   private final int height;
   private final int stride;
   private final int channels;
   private final int radius;
   private final int medianThreshold;
   private int threshold;
   private final Histogram[] histos; // 1 histogram per column (+ 2*radius histograms)
   private final Histogram kernel; // histogram around current point


   public MedianFilter(int width, int height)
   {
      this(width, height, width, DEFAULT_RADIUS, DEFAULT_THRESHOLD);
   }


   public MedianFilter(int width, int height, int stride)
   {
      this(width, height, stride, DEFAULT_RADIUS, DEFAULT_THRESHOLD);
   }

   
   // Optional threshold in [2..256] (256 means disabled)
   // When the absolute difference between a pixel and its median is over the threshold
   // the pixels is kept, otherwise it is replaced with the median. The default 
   // behavior is to always replace (regular median filter).
   public MedianFilter(int width, int height, int stride, int radius, int threshold)
   {
      this(width, height, stride, radius, THREE_CHANNELS, threshold);
   }


   // Optional threshold in [2..256] (256 means disabled)
   // When the absolute difference between a pixel and its median is over the threshold
   // the pixels is kept, otherwise it is replaced with the median. The default 
   // behavior is to always replace (regular median filter).
   public MedianFilter(int width, int height, int stride, int radius, int channels, int threshold)
   {
      if (height < 8)
         throw new IllegalArgumentException("The height must be at least 8");

      if (width < 8)
         throw new IllegalArgumentException("The width must be at least 8");

      if (stride < 8)
         throw new IllegalArgumentException("The stride must be at least 8");

      if ((channels != THREE_CHANNELS) && (channels != R_CHANNEL)
         && (channels != G_CHANNEL) && (channels != B_CHANNEL))
         throw new IllegalArgumentException("Invalid input channel parameter (must be RGB or R or G or B)");

      if ((radius < 2) || (radius > 32))
         throw new IllegalArgumentException("The filter radius must be in [2..32]");

      if ((threshold < 2) || ((threshold > 255) && (threshold != DEFAULT_THRESHOLD)))
         throw new IllegalArgumentException("The threshold must be in [2..255]");

      this.height = height;
      this.width = width;
      this.stride = stride;
      this.channels = channels;
      this.radius = radius;
      this.threshold = threshold;
      this.medianThreshold = ((2*this.radius+1) * (2*this.radius+1)) / 2;
      this.kernel = new Histogram(); // contains (2*radius+1) * (2*radius+1) pixels
      this.histos = new Histogram[this.width+2*radius];

      for (int i=0; i<this.histos.length; i++)
         this.histos[i] = new Histogram();
   }

   
   public int getThreshold()
   {
      return this.threshold;
   }

   
   public boolean setThreshold(int threshold)
   {
      if ((threshold < 2) || (threshold > 255))
         return false;
      
      this.threshold = threshold;
      return true;
   }
  
   
   @Override
   public boolean apply(SliceIntArray input, SliceIntArray output)
   {
        if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
           return false;

        final int[] src = input.array;
        final int[] dst = output.array;
        final int nbChans = this.channels >> 8;
        final int maxShift = this.channels & 0xFF;
        final int h = this.height;
        final int w = this.width;
        final int st = this.stride;
        final int rd = this.radius;
        final int deltaOffs = st * rd;        
        final int endX = w - 1;
        final int endY = h - 1;
        final int th = this.threshold;

        // Process each channel
        for (int i=0; i<nbChans; i++)
        {
            final int shift = maxShift - 8*i;

            // Initialize columns histograms
            for (int x=0; x<this.histos.length; x++)
            {
               final Histogram hst = this.histos[x];
               hst.clear();
               int startX = input.index;
               final int xx = (x < rd) ? 0 : x - rd;

               for (int y=0; y<=2*rd; y++)
               {
                  final int val = (src[startX+xx]>>shift) & 0xFF;
                  hst.fine[val]++;
                  hst.coarse[val>>4]++;
                  startX += st;
               }
            }

            int srcStart = input.index;
            int dstStart = output.index;

            // Process each row
            for (int y=0; y<endY; y++)
            {
               // Initialize current histogram
               this.kernel.clear();

               for (int j=y; j<=y+2*rd; j++)
               {
                  final int yy = (j>=rd) ? ((j<endY) ? j-rd : endY) : 0;
                  final int startX = yy*st + input.index;

                  for (int r=-rd; r<0; r++)
                  {
                     final int val = (src[startX+0]>>shift) & 0xFF;
                     this.kernel.fine[val]++;
                     this.kernel.coarse[val>>4]++;
                  }

                  for (int r=0; r<=rd; r++)
                  {
                     final int val = (src[startX+r]>>shift) & 0xFF;
                     this.kernel.fine[val]++;
                     this.kernel.coarse[val>>4]++;
                  }
               }
       
               final int yy = (y>=rd) ? y-rd : 0;
               
               for (int rx=0; rx<=2*rd; rx++)
               {
                  final Histogram hst = this.histos[rx];
                  hst.clear();                  
                  int startX = yy*st + input.index;
                  final int xx = (rx>=rd) ? rx-rd : 0;

                  for (int ry=0; ry<=2*rd; ry++)
                  {
                     final int val = (src[startX+xx]>>shift) & 0xFF;
                     hst.fine[val]++;
                     hst.coarse[val>>4]++;
                     
                     if (yy+ry < endY)
                        startX += st;
                  }
               }

               // Process each column
               for (int x=0; x<endX; x++)
               {
                  // Find median of current histogram and update output
                  int med = this.getMedian();
                  
                  if (th < DEFAULT_THRESHOLD)
                  {
                     final int val = (src[srcStart+x]>>shift) & 0xFF;
                     int diff = val - med;
                     diff = (diff + (diff >> 31)) ^ (diff >> 31);
                     
                     // Keep current pixel is too different from median (EG. edges)
                     if (diff >= th)
                        med = val;
                  }
                  
                  dst[dstStart+x] &= ~(255<<shift);
                  dst[dstStart+x] |= (med<<shift);

                  if ((y > rd) && (y < endY-rd))
                  {
                     // Step 1: update histo for current column
                     // Remove old pixel from column, add new pixel from column
                     Histogram hst = this.histos[x+2*rd+1];
                     final int pos = srcStart + x + rd + 1;
                     final int outPix = (src[pos-deltaOffs-st] >> shift) & 0xFF;
                     hst.fine[outPix]--;
                     hst.coarse[outPix>>4]--;
                     final int inPix = (src[pos+deltaOffs] >> shift) & 0xFF;
                     hst.fine[inPix]++;
                     hst.coarse[inPix>>4]++;
                  }

                  // Step 2: update current histogram
                  // Remove old column histogram, add new column histogram
                  removeHistogram(this.kernel, this.histos[x]);
                  addHistogram(this.kernel, this.histos[x+2*rd+1]);
              }

              // Last column
              dst[dstStart+w-1] = src[srcStart+w-1];
              srcStart += st;
              dstStart += st;
           }
            
           // Last row
           System.arraycopy(src, input.index+st*(h-1), dst, output.index+st*(h-1), w);
       }

       return true;
    }


   private static void addHistogram(Histogram kernel, Histogram ch)
   {
      int idx = 0;

      for (; idx<16; idx++)
      {
         if (ch.coarse[idx] != 0)
            break;
      }

      final int[] hst1 = kernel.fine;
      final int[] hst2 = ch.fine;

      for (int i=idx; i<16; i++)
         kernel.coarse[i] += ch.coarse[i];

      for (int i=idx<<4; i<256; i+=8)
      {
         hst1[i]   += hst2[i];
         hst1[i+1] += hst2[i+1];
         hst1[i+2] += hst2[i+2];
         hst1[i+3] += hst2[i+3];
         hst1[i+4] += hst2[i+4];
         hst1[i+5] += hst2[i+5];
         hst1[i+6] += hst2[i+6];
         hst1[i+7] += hst2[i+7];
      }
   }


   private static void removeHistogram(Histogram kernel, Histogram ch)
   {
      int idx = 0;

      for (; idx<16; idx++)
      {
         if (ch.coarse[idx] != 0)
            break;
      }

      final int[] hst1 = kernel.fine;
      final int[] hst2 = ch.fine;

      for (int i=idx; i<16; i++)
         kernel.coarse[i] -= ch.coarse[i];

      for (int i=idx<<4; i<256; i+=8)
      {
         hst1[i]   -= hst2[i];
         hst1[i+1] -= hst2[i+1];
         hst1[i+2] -= hst2[i+2];
         hst1[i+3] -= hst2[i+3];
         hst1[i+4] -= hst2[i+4];
         hst1[i+5] -= hst2[i+5];
         hst1[i+6] -= hst2[i+6];
         hst1[i+7] -= hst2[i+7];
      }
   }


   private int getMedian()
   {
      int res = 0;
      int idx = 0;
      int n = 0;

      // Find segment where the median belongs
      for (; idx<16; idx++)
      {
         n += this.kernel.coarse[idx];

         if (n >= this.medianThreshold)
            break;
      }

      final int start = idx << 4;

      // Find the median in segment
      for (int i=start+15; i>=start; i-=4)
      {
         n -= this.kernel.fine[i];

         if (n < this.medianThreshold)
            return i;

         n -= this.kernel.fine[i-1];

         if (n < this.medianThreshold)
            return i - 1;

         n -= this.kernel.fine[i-2];

         if (n < this.medianThreshold)
            return i - 2;

         n -= this.kernel.fine[i-3];

         if (n < this.medianThreshold)
            return i - 3;
      }

      return res;
   }


   static class Histogram
   {
      public final int[] coarse;
      public final int[] fine;


      Histogram()
      {
         this.coarse = new int[16];
         this.fine = new int[256];
      }

      public void clear()
      {
         for (int i=0; i<16; i++)
            this.coarse[i] = 0;

         for (int i=0; i<256; i+=8)
         {
            this.fine[i]   = 0;
            this.fine[i+1] = 0;
            this.fine[i+2] = 0;
            this.fine[i+3] = 0;
            this.fine[i+4] = 0;
            this.fine[i+5] = 0;
            this.fine[i+6] = 0;
            this.fine[i+7] = 0;
         }
      }
   }
}
