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

import kanzi.util.color.YCbCrColorModelConverter;
import kanzi.util.color.ColorModelConverter;
import kanzi.Global;
import kanzi.ColorModelType;
import kanzi.util.sampling.DecimateDownSampler;


// PSNR: peak signal noise ratio
// SSIM: structural similarity metric
// See [Image quality assessment: From error measurement to structural similarity] by Zhou Wang
// See http://www.cns.nyu.edu/~lcv/ssim/
// This implemenation is an approximation of SSIM and PSNR using (long) integers only

public final class ImageQualityMonitor
{
   // SSIM constants (fudge factors used to avoid diverging divisions)
   private static final int C1 = 104;    // 16 * ((0.01 * 255) ^ 2)
   private static final int C2 = 9 * C1; // 16 * ((0.03 * 255) ^ 2)

   // gaussian: 32 * exp(-k*x*x);
   public static final int[] DEFAULT_GAUSSIAN_KERNEL = new int [] { 2, 9, 23, 32, 23, 9 , 2 };

   private final int width;
   private final int height;
   private final int stride;
   private final int[] kernel32;
   private final int downSampling;
   private int[] buffer;
   private int[] y1;
   private int[] y2;
   private int[] u1;
   private int[] u2;
   private int[] v1;
   private int[] v2;


   public ImageQualityMonitor(int width, int height)
   {
      this(width, height, width, 0, DEFAULT_GAUSSIAN_KERNEL);
   }


   public ImageQualityMonitor(int width, int height, int stride)
   {
      this(width, height, stride, 0, DEFAULT_GAUSSIAN_KERNEL);
   }


   public ImageQualityMonitor(int width, int height, int stride, int downSampling)
   {
      this(width, height, stride, downSampling, DEFAULT_GAUSSIAN_KERNEL);
   }


   // gaussian kernel is used exclusively for SSIM, can be null
   public ImageQualityMonitor(int width, int height, int stride, int downSampling, int[] ssim_gauss_kernel)
   {
       if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

       if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");

       if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

       if ((downSampling < 0) || (downSampling > 8))
            throw new IllegalArgumentException("The down sampling value must in the [0..8] range");

       if ((ssim_gauss_kernel != null) && ((ssim_gauss_kernel.length <= 2) || (ssim_gauss_kernel.length >= 16)))
            throw new IllegalArgumentException("The kernel length is invalid (must be 3,5,7,9,11,13 or 15)");

       if ((ssim_gauss_kernel != null) && ((ssim_gauss_kernel.length & 1) == 0))
            throw new IllegalArgumentException("The kernel length is invalid (must be 3,5,7,9,11,13 or 15)");

       this.width = width;
       this.height = height;
       this.stride = stride;
       this.kernel32 = (ssim_gauss_kernel == null) ? DEFAULT_GAUSSIAN_KERNEL : ssim_gauss_kernel;
       this.downSampling = downSampling;
       this.buffer = new int[0];
       this.y1 = new int[0];
       this.y2 = new int[0];
       this.u1 = new int[0];
       this.u2 = new int[0];
       this.v1 = new int[0];
       this.v2 = new int[0];
   }


   public int computePSNR(int[] img1_chan1, int[] img1_chan2, int[] img1_chan3,
                          int[] img2_chan1, int[] img2_chan2, int[] img2_chan3,
                          ColorModelType type)
   {
      return this.computePSNR(img1_chan1, img1_chan2, img1_chan3,
                              img2_chan1, img2_chan2, img2_chan3,
                              0, 0, this.width, this.height, type);
   }


   // return psnr * 1024 or INFINITE_PSNR (=0)
   // channels 1,2,3 can be RGB or YUV in each image
   public int computePSNR(int[] img1_chan1, int[] img1_chan2, int[] img1_chan3,
                          int[] img2_chan1, int[] img2_chan2, int[] img2_chan3,
                          int x, int y, int w, int h, ColorModelType type)
   {
       if (x < 0)
          x = 0;

       if (y < 0)
          y = 0;

       if (x >= w)
          x = w - 1;

       if (y >= h)
          y = h - 1;

       if (x + w > this.width)
          w = this.width - x;

       if (y + h > this.height)
          h = this.height - y;

       final int iterations1 = ((w - x) >> this.downSampling) * ((h - y) >> this.downSampling);

       // Rescale to avoid overflow
       final long lsum1 = this.computeDeltaSum(img1_chan1, img2_chan1, x, y, w, h, type);
       final int isum1 = (int) ((lsum1 + 50) / 100);

       if ((type == ColorModelType.YUV420) || (type == ColorModelType.YUV422))
           w >>= 1;

       if (type == ColorModelType.YUV420)
           h >>= 1;

       final int iterations2 = ((w - x) >> this.downSampling) * ((h - y) >> this.downSampling);

       // Rescale to avoid overflow
       final long lsum2 = this.computeDeltaSum(img1_chan2, img2_chan2, x, y, w, h, type);
       final int isum2 = (int) ((lsum2 + 50) / 100);
       final long lsum3 = this.computeDeltaSum(img1_chan3, img2_chan3, x, y, w, h, type);
       final int isum3 = (int) ((lsum3 + 50) / 100);

       if (isum1 + isum2 + isum3 == 0)
          return Global.INFINITE_VALUE;

       // Formula:  double mse = (double) (sum) / size
       //           double psnr = 10 * Math.log10(255d*255d/mse);
       // or        double psnr = 10 * (Math.log10(65025) + (Math.log10(size) - Math.log10(sum))
       // Calculate PSNR << 10 with 1024 * 10 * (log10(65025L) = 49286
       // 1024*10*log10(100) = 20480
       final int psnr1024_chan1 = 49286 + Global.ten_log10(iterations1) - Global.ten_log10(isum1) - 20480;
       final int psnr1024_chan2 = 49286 + Global.ten_log10(iterations2) - Global.ten_log10(isum2) - 20480;
       final int pnsr1024_chan3 = 49286 + Global.ten_log10(iterations2) - Global.ten_log10(isum3) - 20480;

       if (type == ColorModelType.RGB) // RGB => weight 1/3 for R, G & B (?)
          return (psnr1024_chan1 + psnr1024_chan2 + pnsr1024_chan3) / 3;
       else // YUV => weight 0.8 for Y and 0.1 for U & V
          return ((102*psnr1024_chan1) + (13*psnr1024_chan2) + (13*pnsr1024_chan3)) >> 7;
   }


   // return psnr * 1024 or INFINITE_PSNR (=0)
   public int computePSNR(int[] data1, int[] data2)
   {
      return this.computePSNR(data1, data2, 0, 0, this.width, this.height, ColorModelType.RGB);
   }


   // return psnr * 1024 or INFINITE_PSNR (=0)
   public int computePSNR(int[] data1, int[] data2, ColorModelType type)
   {
      return this.computePSNR(data1, data2, 0, 0, this.width, this.height, type);
   }


   // return psnr * 1024 or INFINITE_PSNR (=0)
   public int computePSNR(int[] data1, int[] data2, int x, int y, int w, int h, ColorModelType type)
   {
      if (x < 0)
         x = 0;

      if (y < 0)
         y = 0;

      if (x + w > this.width)
         w = this.width - x;

      if (y + h > this.height)
         h = this.height - y;

      final long lsum = this.computeDeltaSum(data1, data2, x, y, w, h, type);

      // Rescale to avoid overflow
      final int isum = (int) ((lsum + 50) / 100);

      if (isum <= 0)
         return Global.INFINITE_VALUE;

      // Formula:  double mse = (double) (sum) / size
      //           double psnr = 10 * Math.log10(255d*255d/mse);
      // or        double psnr = 10 * (Math.log10(65025) + (Math.log10(size) - Math.log10(sum))
      // Calculate PSNR << 10 with 1024 * 10 * (log10(65025L) = 49286
      // 1024*10*log10(100) = 20480
      final int iterations = ((w - x) >> this.downSampling) * ((h - y) >> this.downSampling);
      return 49286 + (Global.ten_log10(iterations) - Global.ten_log10(isum)) - 20480;
   }


   // return sum of squared differences between data
   private long computeDeltaSum(int[] data1, int[] data2, int x, int y, int w, int h, ColorModelType type)
   {
      if (data1 == data2)
         return 0;

      long sum = 0, sum1 = 0, sum2 = 0, sum3 = 0;
      final int st = this.stride << this.downSampling;
      final int inc = 1 << this.downSampling;
      int startOffs = (y * this.stride) + x;

      // Check for packed rgb type
      if (type == ColorModelType.RGB)
      {
         for (int j=0; j<h; j+=inc)
         {
            for (int i=0; i<w; i+=inc)
            {
               final int idx = startOffs + i;
               final int p1 = data1[idx];
               final int p2 = data2[idx];
               final int r1 = (p1 >> 16) & 0xFF;
               final int r2 = (p2 >> 16) & 0xFF;
               final int g1 = (p1 >>  8) & 0xFF;
               final int g2 = (p2 >>  8) & 0xFF;
               final int b1 =  p1 & 0xFF;
               final int b2 =  p2 & 0xFF;
               sum1 += ((r1-r2)*(r1-r2));
               sum2 += ((g1-g2)*(g1-g2));
               sum3 += ((b1-b2)*(b1-b2));
            }

            startOffs += st;
         }

         sum = (sum1 + sum2 + sum3) / 3;
      }
      else
      {
         for (int j=0; j<h; j+=inc)
         {
           for (int i=0; i<w; i+=inc)
           {
              final int idx = startOffs + i;
              final int p1 = data1[idx];
              final int p2 = data2[idx];
              sum += ((p1-p2)*(p1-p2));
           }

           startOffs += st;
         }
      }

      return sum;
   }


   // return SSIM * 1024
   public int computeSSIM(int[] img1_chan1, int[] img1_chan2, int[] img1_chan3,
                          int[] img2_chan1, int[] img2_chan2, int[] img2_chan3,
                          ColorModelType type)
   {
      return this.computeSSIM(img1_chan1, img1_chan2, img1_chan3,
                              img2_chan1, img2_chan2, img2_chan3,
                              0, 0, this.width, this.height, type, this.downSampling);
   }


   // Calculate SSIM for the subimages at x,y of width w and height h
   // return SSIM * 1024
   public int computeSSIM(int[] img1_chan1, int[] img1_chan2, int[] img1_chan3,
                          int[] img2_chan1, int[] img2_chan2, int[] img2_chan3,
                          int x, int y, int w, int h, ColorModelType type)
   {
      return this.computeSSIM(img1_chan1, img1_chan2, img1_chan3,
                              img2_chan1, img2_chan2, img2_chan3,
                              x, y, w, h, type, this.downSampling);
   }


   public int computeSSIM(int[] img1_chan1, int[] img1_chan2, int[] img1_chan3,
                          int[] img2_chan1, int[] img2_chan2, int[] img2_chan3,
                          int x, int y, int w, int h, ColorModelType type, int ds)
   {
      if ((type != ColorModelType.YUV444) && (type != ColorModelType.YUV422) &&
        (type != ColorModelType.YUV420))
         return -1;

      final int ssim1024_chan1 = this.computeOneChannelSSIM(img1_chan1, img2_chan1, x, y, w, h, ds);

      if ((type == ColorModelType.YUV420) || (type == ColorModelType.YUV422))
         w >>= 1;

      if (type == ColorModelType.YUV420)
         h >>= 1;

      final int ssim1024_chan2 = this.computeOneChannelSSIM(img1_chan2, img2_chan2, x, y, w, h, ds);
      final int ssim1024_chan3 = this.computeOneChannelSSIM(img1_chan3, img2_chan3, x, y, w, h, ds);

      // YUV => weight 0.8 for Y and 0.1 for U & V
      return (int) ((102*ssim1024_chan1) + (13*ssim1024_chan2) + (13*ssim1024_chan3)) >> 7;
   }


   // Calculate SSIM for RGB (packed) images
   // Return ssim * 1024
   public int computeSSIM(int[] data1, int[] data2)
   {
      return this.computeSSIM(data1, data2, 0, 0, this.width, this.height);
   }


   public int computeSSIM(int[] data1, int[] data2, int x, int y, int w, int h)
   {
      if ((w > this.width) || (h > this.height))
         return -1;

      x >>= this.downSampling;
      y >>= this.downSampling;
      w >>= this.downSampling;
      h >>= this.downSampling;

      // Turn RGB_PACKED data into Y, U, V data
      final int size = w * h;

      if (this.y1.length < size)
         this.y1 = new int[size];

      if (this.y2.length < size)
         this.y2 = new int[size];

      if (this.u1.length < size)
         this.u1 = new int[size];

      if (this.u2.length < size)
         this.u2 = new int[size];

      if (this.v1.length < size)
         this.v1 = new int[size];

      if (this.v2.length < size)
         this.v2 = new int[size];

      final int offset = (y * this.width) + x;
      ColorModelConverter cvt = new YCbCrColorModelConverter(w, h, offset, this.width >> this.downSampling);
      ColorModelType colorModel;

      if (this.downSampling > 0)
      {
         if (this.buffer.length < size)
            this.buffer = new int[size];

         // Downsample before color conversion
         colorModel = ColorModelType.YUV444;
         DecimateDownSampler ds = new DecimateDownSampler(w<<this.downSampling, h<<this.downSampling, 1<<this.downSampling);
         ds.subSample(data1, this.buffer);
         cvt.convertRGBtoYUV(this.buffer, this.y1, this.u1, this.v1, colorModel);
         ds.subSample(data2, this.buffer);
         cvt.convertRGBtoYUV(this.buffer, this.y2, this.u2, this.v2, colorModel);
      }
      else
      {
         colorModel = ColorModelType.YUV420;
         cvt.convertRGBtoYUV(data1, this.y1, this.u1, this.v1, colorModel);
         cvt.convertRGBtoYUV(data2, this.y2, this.u2, this.v2, colorModel);
      }

      return this.computeSSIM(this.y1, this.u1, this.v1, this.y2, this.u2, this.v2,
              x, y, w, h, colorModel, 0);
   }


   // Calculate SSIM for the subimages at x,y of width w and height h (one channel)
   // Return ssim * 1024
   private int computeOneChannelSSIM(int[] data1, int[] data2, int x, int y, int w, int h, int ds)
   {
       if (data1 == data2)
          return 1024;

       if (x < 0)
          x = 0;

       if (y < 0)
          y = 0;

       if (x + w > this.width)
          w = this.width - x;

       if (y + h > this.height)
          h = this.height - y;

       final Context ctx = new Context(data1, data2, x, y, w, h, ds, this.kernel32);
       final int inc = 1 << ds;
       final int endi = x + w;
       final int endj = y + h;
       int iterations = 0;

       for (int j=y; j<endj; j+=inc)
       {
          ctx.y = j;

          for (int i=x; i<endi; i+=inc)
          {
             ctx.x = i;
 //System.out.println(i+","+j+" "+data1[j*w+i]+" "+data2[j*w+i]+" "+ctx.sumSSIM/256);
             computeBlockSSIM(ctx);
             iterations++;
          }
       }

       return (int) (ctx.sumSSIM + (iterations >> 1)) / iterations;
   }


   private static void computeBlockSSIM(Context ctx)
   {
     final int x0 = ctx.x;
     final int y0 = ctx.y;
     final int kOffset = ctx.kernel.length >> 1;
     final int scale = ctx.ds;
     final int st = ctx.w << scale;
     final int inc = 1 << scale;
     final int xMin = (x0-kOffset < 0) ? 0 : x0 - kOffset;
     final int xMax = (x0+kOffset >= ctx.w) ? ctx.w-1 : x0 + kOffset;
     final int yMin = (y0-kOffset < 0) ? 0 : y0 - kOffset;
     final int yMax = (y0+kOffset >= ctx.h) ? ctx.h-1 : y0 + kOffset;
     final int[] data1 = ctx.data1;
     final int[] data2 = ctx.data2;
     final int[] kernel = ctx.kernel;
     int offset = yMin * ctx.w;
     long sumWeights = 0, sumX = 0, sumY = 0, sumXY = 0, sumXX = 0, sumYY = 0;

     for (int y=yMin; y<=yMax; y+=inc)
     {
         final int weightY = kernel[kOffset+((y-y0)>>scale)];

         for (int x=xMin; x<=xMax; x+=inc)
         {
             final int weightXY = weightY * kernel[kOffset+((x-x0)>>scale)];
             final int idx = offset + ((x - xMin) << scale);
             final int xVal = data1[idx];
             final int yVal = data2[idx];
             final int wxVal = weightXY * xVal;
             final int wyVal = weightXY * yVal;
             sumX += wxVal;
             sumY += wyVal;
             sumXX += (wxVal * xVal);
             sumYY += (wyVal * yVal);
             sumXY += (wxVal * yVal);
             sumWeights += weightXY;
         }

         offset += st;
      }

      final long adjust = sumWeights >> 1;
      final long sumWeights_sq = sumWeights * sumWeights;
      final long adjust2 = sumWeights_sq >> 1;

      // Calculations scaled by a factor of 16 (especially important for sigma accuracy)
      final long muXX = ((sumXX << 4) + adjust) / sumWeights;
      final long muYY = ((sumYY << 4) + adjust) / sumWeights;
      final long muXY = ((sumXY << 4) + adjust) / sumWeights;
      final long muXmuX = (((sumX * sumX) << 4) + adjust2) / sumWeights_sq;
      final long muYmuY = (((sumY * sumY) << 4) + adjust2) / sumWeights_sq;
      final long muXmuY = (((sumX * sumY) << 4) + adjust2) / sumWeights_sq;

      long sigmaXX = muXX - muXmuX;
      sigmaXX &= (~(sigmaXX >> 31));
      long sigmaYY = muYY - muYmuY;
      sigmaYY &= (~(sigmaYY >> 31));
      long sigmaXY = muXY - muXmuY;

      // l(x,y) = (2*muX*muY + A1) / (muX*muX + muY*muY + A1)
      // c(x,y) = (2*sigmaX*sigmaY + A2) / ((sigmaX*sigmaX) + (sigmaY*sigmaY) + A2)
      // s(x,y) = (sigmaXY + A3) / ((sigmaX * sigmaY) + A3)
      // ssim(x,y) = l(x,y) * c(x,y) * s(x,y)
      // ssim(x,y) = (2*muX*muY+A1)*(2*sigmaXY+A2)/((muX*mux+muY*muY+A1) * (sigmaX*sigmaX+sigmaY*sigmaY+A2))
      // C1, C2 and C3 are scaled compared to the reference values A1, A2, A3 and C3 is omitted
      final long num = ((muXmuY + muXmuY) + C1) * ((sigmaXY + sigmaXY) + C2);
      final long den = ((muXmuX + muYmuY) + C1) * ((sigmaXX + sigmaYY) + C2);
      final long ssim1024 = ((num << 7) + (den >> 4)) / (den >> 3);

      // Fix rounding errors
      if (ssim1024 > 1024)
         ctx.sumSSIM += 1024;
      else
         ctx.sumSSIM += ssim1024;
   }


   static class Context
   {
      Context(int[] data1, int[] data2, int x, int y, int w, int h, int ds, int[] kernel)
      {
         this.data1 = data1;
         this.data2 = data2;
         this.x = x;
         this.y = y;
         this.w = w;
         this.h = h;
         this.ds = ds;
         this.kernel = kernel;
      }

      final int[] data1;
      final int[] data2;
      final int w;
      final int h;
      final int ds; // downsampling
      final int[] kernel;
      int x;
      int y;
      long sumSSIM;
   }

}