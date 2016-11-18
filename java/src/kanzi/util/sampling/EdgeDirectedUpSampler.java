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

package kanzi.util.sampling;


// Edge Oriented Interpolation based upsampler.
// Original code by David Schleef: gstediupsample.c
// See http://schleef.org/ds/cgak-demo-1 for explanation of the algo and
// http://schleef.org/ds/cgak-demo-1.png for visual examples.
public class EdgeDirectedUpSampler implements UpSampler
{
   private final int width;
   private final int height;
   private final int stride;


   public EdgeDirectedUpSampler(int width, int height)
   {
      this(width, height, width);
   }
   
   
   public EdgeDirectedUpSampler(int width, int height, int stride)
   {
      if (height < 8)
         throw new IllegalArgumentException("The height must be at least 8");

      if (width < 8)
         throw new IllegalArgumentException("The width must be at least 8");

      if (stride < width)
         throw new IllegalArgumentException("The stride must be at least as big as the width");

      if ((height & 7) != 0)
         throw new IllegalArgumentException("The height must be a multiple of 8");

      if ((width & 7) != 0)
         throw new IllegalArgumentException("The width must be a multiple of 8");

      this.height = height;
      this.width = width;
      this.stride = stride;
   }


   @Override
   public void superSampleVertical(int[] src, int[] dst)
   { 
      final int st = this.stride;
      final int sh = this.height;
      final int sw = this.width;
      final int dw = sw << 1;
      final int dw2 = dw << 1;
      int srcOffs = 0;
      int dstOffs = 0;
  /*    
      // Vertical 
      for (int j=0; j<sh-1; j++)
      {
         final int srcOffs1 = srcOffs;
         final int srcOffs2 = srcOffs + sw;
         final int srcOffs3 = srcOffs + sw + sw;

         for (int i=0; i<dw; i++)
         {
            if ((i>=3) && (i<dw-4))
            {
               int v;
               int dx  = (-src[srcOffs1+i-1] - src[srcOffs3+i-1] + src[srcOffs1+i+1] + dst[dstOffs3+i+1]) << 1;
               int dx2 = -src[srcOffs1+i-1] + 2*src[srcOffs1+i] - src[srcOffs1+i+1] - dst[dstOffs3+i-1] + 2*dst[dstOffs3+i] - dst[dstOffs3+i+1];

               if (Math.abs(dx) <= 4*Math.abs(dx2))
               {
                 v = (src[srcOffs1+i] + src[srcOffs3+i]) >> 1;
               }
               else 
               {
                  int dy  = -src[srcOffs1+i-1] - 2*src[srcOffs1+i] - src[srcOffs1+i+1] + dst[dstOffs3+i-1] + 2*dst[dstOffs3+i] + dst[dstOffs3+i+1];

                  if (dy < 0)
                  {
                     dy = -dy;
                     dx = -dx;
                  }

                  if (dx < 0)
                  {
                    if (dx < -2*dy)
                      v = reconstructH(src, srcOffs1+i, srcOffs3+i, 0, 0, 0, 16);
                    else if (dx < -dy)
                      v = reconstructH(src, srcOffs1+i, srcOffs3+i, 0, 0, 8, 8);
                    else if (2*dx < -dy)
                      v = reconstructH(src, srcOffs1+i, srcOffs3+i, 0, 4, 8, 4);
                    else if (3*dx < -dy)
                      v = reconstructH(src, srcOffs1+i, srcOffs3+i, 1, 7, 7, 1);
                    else
                      v = reconstructH(src, srcOffs1+i, srcOffs3+i, 4, 8, 4, 0);
                  }
                  else
                  {
                    if (dx > 2*dy)
                      v = reconstructH(src, srcOffs3+i, srcOffs1+i, 0, 0, 0, 16);
                    else if (dx > dy)
                      v = reconstructH(src, srcOffs3+i, srcOffs1+i, 0, 0, 8, 8);
                    else if (2*dx > dy)
                      v = reconstructH(src, srcOffs3+i, srcOffs1+i, 0, 4, 8, 4);
                    else if (3*dx > dy)
                      v = reconstructH(src, srcOffs3+i, srcOffs1+i, 1, 7, 7, 1);
                    else
                      v = reconstructH(src, srcOffs3+i, srcOffs1+i, 4, 8, 4, 0);
                  }
               }

               //dst[dstOffs2+i] = Math.min(Math.max(v, 0), 255);
               dst[dstOffs2+i] = v;
            }
            else
            {
               dst[dstOffs2+i] = (src[srcOffs1+i] + src[srcOffs3+i]) >> 1;
            }
	 }
         
         dstOffs += dw2;
         srcOffs += st;
      }
   
      int prev = src[srcOffs];
      dst[dstOffs+1] = prev;
      dst[dstOffs+dw] = prev;
      dst[dstOffs+dw+1] = prev;
      
      // Bilinear upsampling for first and last lines
      for (int i=2; i<sw; i++) 
      {
         final int dstOffs1 = dstOffs + i + i;
         final int dstOffs2 = dstOffs + dw + i + i;
         final int cur = src[srcOffs1];
         dst[dstOffs1-1] = (cur + prev) >> 1;
         dst[dstOffs2] = cur;
         dst[dstOffs2-1] = (cur + prev) >> 1;
         prev = cur;
      }      
     */
   }


   @Override
   public void superSampleHorizontal(int[] src, int[] dst)
   {
      final int st = this.stride;
      final int sh = this.height;
      final int sw = this.width;
      final int dw = sw << 1;
      int srcOffs = 0;
      int dstOffs = 0;

      // Horizontal
      for (int j=0; j<sh; j++)
      {
         if ((j >= 3) && (j < sh-3))
         {
            for (int i=0; i<sw-1; i++)
            {
               int v;
               int dx  = (-src[srcOffs-st+i] - src[srcOffs-st+i+1] + src[srcOffs+st+i] + src[srcOffs+st+i+1]) << 1;
               int dx2 = -src[srcOffs-st+i] + 2*src[srcOffs+i] - src[srcOffs+st+i] - src[srcOffs-st+i+1] + 2*src[srcOffs+i+1] - src[srcOffs+st+i+1];

               if (Math.abs(dx) <= 4*Math.abs(dx2))
               {
                  v = (src[srcOffs+i] + src[srcOffs+i+1]) >> 1;
               }
               else
               {
                  int dy  = -src[srcOffs-st+i] - 2*src[srcOffs+i] - src[srcOffs+st+i] + src[srcOffs-st+i+1] + 2*src[srcOffs+i+1] + src[srcOffs+st+i+1];

                  if (dy < 0)
                  {
                     dy = -dy;
                     dx = -dx;
                  }
                  
                  if (dx < 0)
                  {
                     if (dx < -2*dy)
                       v = reconstructV(src, srcOffs+i, st, 0, 0, 0, 16);
                     else if (dx < -dy)
                       v = reconstructV(src, srcOffs+i, st, 0, 0, 8, 8);
                     else if (2*dx < -dy)
                       v = reconstructV(src, srcOffs+i, st, 0, 4, 8, 4);
                     else if (3*dx < -dy)
                       v = reconstructV(src, srcOffs+i, st, 1, 7, 7, 1);
                     else
                       v = reconstructV(src, srcOffs+i, st, 4, 8, 4, 0);
                  }
                  else
                  {
                    if (dx > 2*dy)
                      v = reconstructV(src, srcOffs+i, -st, 0, 0, 0, 16);
                    else if (dx > dy)
                      v = reconstructV(src, srcOffs+i, -st, 0, 0, 8, 8);
                    else if (2*dx > dy)
                      v = reconstructV(src, srcOffs+i, -st, 0, 4, 8, 4);
                    else if (3*dx > dy)
                      v = reconstructV(src, srcOffs+i, -st, 1, 7, 7, 1);
                    else
                      v = reconstructV(src, srcOffs+i, -st, 4, 8, 4, 0);
                  }
               }

               dst[dstOffs+i+i] = src[srcOffs+i];
               //dst[dstOffs+i+i+1] = Math.min(Math.max(v, 0), 255);
               dst[dstOffs+i+i+1] = v;
            }

            dst[dstOffs+dw-2] = src[srcOffs+sw-1];
            dst[dstOffs+dw-1] = src[srcOffs+sw-1];
         }
         else
         {
            // Bilinear upsampling for first and last lines
            final int dstOffs1 = dstOffs;
            final int dstOffs2 = dstOffs + dw;
            final int srcOffs1 = srcOffs;
            int prev = src[srcOffs1];
            dst[dstOffs1] = prev;
            dst[dstOffs2] = prev;
            
            for (int i=1; i<sw; i++)
            {
               final int cur = src[srcOffs1+i];
               dst[dstOffs1+i+i] = cur;
               dst[dstOffs1+i+i-1] = (cur + prev) >> 1;
               dst[dstOffs2+i+i] = cur;
               dst[dstOffs2+i+i-1] = (cur + prev) >> 1;
               prev = cur;
            }

            dst[dstOffs1+dw-1] = prev;
            dst[dstOffs2+dw-1] = prev;
         }
         
         srcOffs += st;
         dstOffs += dw;
      }
   }


   private static int reconstructV(int[] buf, int offs, int stride, int a, int b, int c, int d)
   {
      int x;
      x  = (buf[offs-3*stride]+buf[offs+1+3*stride]) * a;
      x += (buf[offs-2*stride]+buf[offs+1+2*stride]) * b;
      x += (buf[offs-1*stride]+buf[offs+1+1*stride]) * c;
      x += (buf[offs-0*stride]+buf[offs+1+0*stride]) * d;
      return (x + 16) >> 5;
   }

   
   private static int reconstructH(int[] buf, int offs1, int offs2, int a, int b, int c, int d)
   {
      int x;
      x  = (buf[offs1-3]+buf[offs2+3]) * a;
      x += (buf[offs1-2]+buf[offs2+2]) * b;
      x += (buf[offs1-1]+buf[offs2+1]) * c;
      x += (buf[offs1-0]+buf[offs2+0]) * d;
      return (x + 16) >> 5;
   }

   
   @Override
   public void superSample(int[] src, int[] dst)
   {
      final int st = this.stride;
      final int sh = this.height;
      final int sw = this.width;
      final int dw = sw << 1;
      final int dw2 = dw + dw;
      int srcOffs = 0;
      int dstOffs = 0;

      // Horizontal
      for (int j=0; j<sh; j++)
      {
         if ((j >= 3) && (j < sh-3))
         {
            for (int i=0; i<sw-1; i++)
            {
               int v;
               int dx  = (-src[srcOffs-st+i] - src[srcOffs-st+i+1] + src[srcOffs+st+i] + src[srcOffs+st+i+1]) << 1;
               int dx2 = -src[srcOffs-st+i] + 2*src[srcOffs+i] - src[srcOffs+st+i] - src[srcOffs-st+i+1] + 2*src[srcOffs+i+1] - src[srcOffs+st+i+1];

               if (Math.abs(dx) <= 4*Math.abs(dx2))
               {
                  v = (src[srcOffs+i] + src[srcOffs+i+1]) >> 1;
               }
               else
               {
                  int dy  = -src[srcOffs-st+i] - 2*src[srcOffs+i] - src[srcOffs+st+i] + src[srcOffs-st+i+1] + 2*src[srcOffs+i+1] + src[srcOffs+st+i+1];

                  if (dy < 0)
                  {
                     dy = -dy;
                     dx = -dx;
                  }
                  
                  if (dx < 0)
                  {
                     if (dx < -2*dy)
                       v = reconstructV(src, srcOffs+i, st, 0, 0, 0, 16);
                     else if (dx < -dy)
                       v = reconstructV(src, srcOffs+i, st, 0, 0, 8, 8);
                     else if (2*dx < -dy)
                       v = reconstructV(src, srcOffs+i, st, 0, 4, 8, 4);
                     else if (3*dx < -dy)
                       v = reconstructV(src, srcOffs+i, st, 1, 7, 7, 1);
                     else
                       v = reconstructV(src, srcOffs+i, st, 4, 8, 4, 0);
                  }
                  else
                  {
                    if (dx > 2*dy)
                      v = reconstructV(src, srcOffs+i, -st, 0, 0, 0, 16);
                    else if (dx > dy)
                      v = reconstructV(src, srcOffs+i, -st, 0, 0, 8, 8);
                    else if (2*dx > dy)
                      v = reconstructV(src, srcOffs+i, -st, 0, 4, 8, 4);
                    else if (3*dx > dy)
                      v = reconstructV(src, srcOffs+i, -st, 1, 7, 7, 1);
                    else
                      v = reconstructV(src, srcOffs+i, -st, 4, 8, 4, 0);
                  }
               }

               dst[dstOffs+i+i] = src[srcOffs+i];
               //dst[dstOffs+i+i+1] = Math.min(Math.max(v, 0), 255);
               dst[dstOffs+i+i+1] = v;
            }

            dst[dstOffs+dw-2] = src[srcOffs+sw-1];
            dst[dstOffs+dw-1] = src[srcOffs+sw-1];
         }
         else
         {
            // Bilinear upsampling for first and last lines
            final int dstOffs1 = dstOffs;
            final int dstOffs2 = dstOffs + dw;
            final int srcOffs1 = srcOffs;
            int prev = src[srcOffs1];
            dst[dstOffs1] = prev;
            dst[dstOffs2] = prev;
            
            for (int i=1; i<sw; i++)
            {
               final int cur = src[srcOffs1+i];
               dst[dstOffs1+i+i] = cur;
               dst[dstOffs1+i+i-1] = (cur + prev) >> 1;
               dst[dstOffs2+i+i] = cur;
               dst[dstOffs2+i+i-1] = (cur + prev) >> 1;
               prev = cur;
            }

            dst[dstOffs1+dw-1] = prev;
            dst[dstOffs2+dw-1] = prev;
         }
         
         srcOffs += st;
         dstOffs += dw2;
      }

      dstOffs = 0;
      
      // Vertical 
      for (int j=0; j<sh-1; j++)
      {
         final int dstOffs1 = dstOffs;
         final int dstOffs2 = dstOffs + dw;
         final int dstOffs3 = dstOffs + dw2;

         for (int i=0; i<dw; i++)
         {
            if ((i>=3) && (i<dw-4))
            {
               int v;
               int dx  = (-dst[dstOffs1+i-1] - dst[dstOffs3+i-1] + dst[dstOffs1+i+1] + dst[dstOffs3+i+1]) << 1;
               int dx2 = -dst[dstOffs1+i-1] + 2*dst[dstOffs1+i] - dst[dstOffs1+i+1] - dst[dstOffs3+i-1] + 2*dst[dstOffs3+i] - dst[dstOffs3+i+1];

               if (Math.abs(dx) <= 4*Math.abs(dx2))
               {
                 v = (dst[dstOffs1+i] + dst[dstOffs3+i]) >> 1;
               }
               else 
               {
                  int dy  = -dst[dstOffs1+i-1] - 2*dst[dstOffs1+i] - dst[dstOffs1+i+1] + dst[dstOffs3+i-1] + 2*dst[dstOffs3+i] + dst[dstOffs3+i+1];

                  if (dy < 0)
                  {
                     dy = -dy;
                     dx = -dx;
                  }

                  if (dx < 0)
                  {
                    if (dx < -2*dy)
                      v = reconstructH(dst, dstOffs1+i, dstOffs3+i, 0, 0, 0, 16);
                    else if (dx < -dy)
                      v = reconstructH(dst, dstOffs1+i, dstOffs3+i, 0, 0, 8, 8);
                    else if (2*dx < -dy)
                      v = reconstructH(dst, dstOffs1+i, dstOffs3+i, 0, 4, 8, 4);
                    else if (3*dx < -dy)
                      v = reconstructH(dst, dstOffs1+i, dstOffs3+i, 1, 7, 7, 1);
                    else
                      v = reconstructH(dst, dstOffs1+i, dstOffs3+i, 4, 8, 4, 0);
                  }
                  else
                  {
                    if (dx > 2*dy)
                      v = reconstructH(dst, dstOffs3+i, dstOffs1+i, 0, 0, 0, 16);
                    else if (dx > dy)
                      v = reconstructH(dst, dstOffs3+i, dstOffs1+i, 0, 0, 8, 8);
                    else if (2*dx > dy)
                      v = reconstructH(dst, dstOffs3+i, dstOffs1+i, 0, 4, 8, 4);
                    else if (3*dx > dy)
                      v = reconstructH(dst, dstOffs3+i, dstOffs1+i, 1, 7, 7, 1);
                    else
                      v = reconstructH(dst, dstOffs3+i, dstOffs1+i, 4, 8, 4, 0);
                  }
               }

               //dst[dstOffs2+i] = Math.min(Math.max(v, 0), 255);
               dst[dstOffs2+i] = v;
            }
            else
            {
               dst[dstOffs2+i] = (dst[dstOffs1+i] + dst[dstOffs3+i]) >> 1;
            }
	 }
         
         dstOffs += dw2;         
      }
   
      int prev = dst[dstOffs];
      dst[dstOffs+1] = prev;
      dst[dstOffs+dw] = prev;
      dst[dstOffs+dw+1] = prev;
      
      // Bilinear upsampling for first and last lines
      for (int i=2; i<sw; i++) 
      {
         final int dstOffs1 = dstOffs + i + i;
         final int dstOffs2 = dstOffs + dw + i + i;
         final int cur = dst[dstOffs1];
         dst[dstOffs1-1] = (cur + prev) >> 1;
         dst[dstOffs2] = cur;
         dst[dstOffs2-1] = (cur + prev) >> 1;
         prev = cur;
      }      
   }
   
   
   @Override
   public boolean supportsScalingFactor(int factor)
   {
      return (factor == 2);
   }   
}