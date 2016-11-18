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

package kanzi.util.image;


public class DeblockingFilter
{
   public static final int DIR_LEFT  = 1;
   public static final int DIR_RIGHT = 2;
   private final static int DEFAULT_STEP = 8;
   private final static int DEFAULT_STRENGTH = 12;
   
   private final int width;
   private final int height;
   private final int stride;
   private final int step;


   public DeblockingFilter(int width, int height, int stride)
   {
      this(width, height, stride, DEFAULT_STEP);
   }
   
   
   public DeblockingFilter(int width, int height, int stride, int step)
   {
      this(width, height, stride, DEFAULT_STEP, DEFAULT_STRENGTH);
   }
   
   
   public DeblockingFilter(int width, int height, int stride, int step, int strength)
   {
      if (height < 8)
         throw new IllegalArgumentException("The height must be at least 8");

      if (width < 8)
         throw new IllegalArgumentException("The width must be at least 8");

      if (stride < 8)
         throw new IllegalArgumentException("The stride must be at least 8");

      if ((height & 7) != 0)
         throw new IllegalArgumentException("The height must be a multiple of 8");

      if ((width & 7) != 0)
         throw new IllegalArgumentException("The width must be a multiple of 8");

      if ((stride & 7) != 0)
         throw new IllegalArgumentException("The stride must be a multiple of 8");

      if ((step != 4) && (step != 8) && (step != 16) && (step != 32))
         throw new IllegalArgumentException("The step must be in [2, 4, 8, 16, 32]");
       
      this.width = width;
      this.height = height;
      this.stride = stride;
      this.step = step;
   }


   public void apply(int[] frame, int x, int y, int blockDim,
           int predictionType, int q, boolean strong)
   {
      this.applyVertical(frame, x, y, blockDim, predictionType, q, strong);
      this.applyHorizontal(frame, x, y, blockDim, predictionType, q, strong);
   }
   
   
   public void applyVertical(int[] frame, int x, int y, int blockDim,
           int predictionType, int q, boolean strong)
   {
      if (frame == null)
         return;

      final int st = this.stride;
      final int w = this.width;
      final int h = this.height;
      final int inc = this.step;
      final int threshold = this.getFilterStrength(q, strong);
      int line = y*st + x;
      final int endj = line + blockDim*st;
      final int start = (y >= inc) ? line : line + inc*st;
      final int end = (y < h-inc) ? endj : endj - inc*st;

      for (int j=start; j<end; j+=st)
      {
         if (((predictionType & DIR_LEFT) != 0) && (x >= inc))
         {
            final int q0 = frame[j];
            final int p0 = frame[j-1];
            int deltaPQ = q0 - p0;
            deltaPQ = (deltaPQ + (deltaPQ >> 31)) ^ (deltaPQ >> 31); // abs

            if (deltaPQ >= threshold)
               continue;

            final int q1 = frame[j+1];
            int deltaQQ = (q0 - q1) << 2;
            deltaQQ = (deltaQQ + (deltaQQ >> 31)) ^ (deltaQQ >> 31); // abs

            if (deltaQQ >= threshold)
               continue;

            final int p1 = frame[j-2];
            int deltaPP = (p0 - p1) << 2;
            deltaPP = (deltaPP + (deltaPP >> 31)) ^ (deltaPP >> 31); // abs

            if (deltaPP >= threshold)
               continue;

            if ((deltaPQ <= 1) && (deltaPP <= 1) && (deltaQQ <= 1))
               continue;
            
            if (strong == true)
            {
               final int q2 = frame[j+2];
               final int p2 = frame[j-3];
               final int q3 = frame[j+3];
               final int p3 = frame[j-4];
               frame[j]   = (q2 + (q1<<1) + (q0<<1) + (p0<<1) + p1 + 4) >> 3;
               frame[j-1] = (p2 + (p1<<1) + (p0<<1) + (q0<<1) + q1 + 4) >> 3;
               frame[j+1] = (q2 + q1 + q0 + p0 + 2) >> 2;
               frame[j-2] = (p2 + p1 + p0 + q0 + 2) >> 2;
               frame[j+2] = ((q3<<1) + (q2*3) + q1 + q0 + p0 + 4) >> 3;
               frame[j-3] = ((p3<<1) + (p2*3) + p1 + p0 + q0 + 4) >> 3;
            }
            else
            {
               frame[j]   = ((q1<<1) + q0 + p1 + 2) >> 2;
               frame[j-1] = ((p1<<1) + p0 + q1 + 2) >> 2;
            }
         }

         if (((predictionType & DIR_RIGHT) != 0) && (x < w-blockDim-inc))
         {
            final int k = j + blockDim;
            final int q0 = frame[k];
            final int p0 = frame[k+1];
            int deltaPQ = q0 - p0;
            deltaPQ = (deltaPQ + (deltaPQ >> 31)) ^ (deltaPQ >> 31); // abs

            if (deltaPQ >= threshold)
               continue;

            final int q1 = frame[k-1];
            int deltaQQ = (q0 - q1) << 2;
            deltaQQ = (deltaQQ + (deltaQQ >> 31)) ^ (deltaQQ >> 31); // abs

            if (deltaQQ >= threshold)
               continue;

            final int p1 = frame[k+2];
            int deltaPP = (p0 - p1) << 2;
            deltaPP = (deltaPP + (deltaPP >> 31)) ^ (deltaPP >> 31); // abs

            if (deltaPP >= threshold)
               continue;

            if (strong == true)
            {
               final int q2 = frame[k-2];
               final int p2 = frame[k+3];
               final int q3 = frame[k-3];
               final int p3 = frame[k+4];
               frame[k]   = (q2 + (q1<<1) + (q0<<1) + (p0<<1) + p1 + 4) >> 3;
               frame[k+1] = (p2 + (p1<<1) + (p0<<1) + (q0<<1) + q1 + 4) >> 3;
               frame[k-1] = (q2 + q1 + q0 + p0 + 2) >> 2;
               frame[k+2] = (p2 + p1 + p0 + q0 + 2) >> 2;
               frame[k-2] = ((q3<<1) + (q2*3) + q1 + q0 + p0 + 4) >> 3;
               frame[k+3] = ((p3<<1) + (p2*3) + p1 + p0 + q0 + 4) >> 3;
            }
            else
            {
               frame[k]   = ((q1<<1) + q0 + p1 + 2) >> 2;
               frame[k+1] = ((p1<<1) + p0 + q1 + 2) >> 2;
            }
         }

         line += st;
      } // vertical loop
   }

   
   public void applyHorizontal(int[] frame, int x, int y, int blockDim,
           int predictionType, int q, boolean strong)
   {
      if (frame == null)
         return;

      final int st = this.stride;
      final int w = this.width;
      final int h = this.height;
      final int inc = this.step;
      
      if ((y < inc) || (y >= h-inc))
         return;

      final int threshold = this.getFilterStrength(q, strong);
      int line = y*st + x;
      final int start = (x >= inc) ? line : line + inc;
      final int endi = start + blockDim;
      final int end = (x < w-inc) ? endi : endi - inc;
      final int st2 = st + st;
      final int st3 = st2 + st;
      final int st4 = st3 + st;

      // HORIZONTAL
      for (int i=start; i<end; i++)
      {
         final int q0 = frame[i];
         final int p0 = frame[i-st];
         int deltaPQ = q0 - p0;
         deltaPQ = (deltaPQ + (deltaPQ >> 31)) ^ (deltaPQ >> 31); // abs

         if (deltaPQ >= threshold)
            continue;

         final int q1 = frame[i+st];
         int deltaQQ = (q0 - q1) << 2;
         deltaQQ = (deltaQQ + (deltaQQ >> 31)) ^ (deltaQQ >> 31); // abs

         if (deltaQQ >= threshold)
            continue;

         final int p1 = frame[i-st2];
         int deltaPP = (p0 - p1) << 2;
         deltaPP = (deltaPP + (deltaPP >> 31)) ^ (deltaPP >> 31); // abs

         if (deltaPP >= threshold)
            continue;

         if (strong == true)
         {
            final int q2 = frame[i+st2];
            final int p2 = frame[i-st3];
            final int q3 = frame[i+st3];
            final int p3 = frame[i-st4];
            frame[i]     = (q2 + (q1<<1) + (q0<<1) + (p0<<1) + p1 + 4) >> 3;
            frame[i-st]  = (p2 + (p1<<1) + (p0<<1) + (q0<<1) + q1 + 4) >> 3;
            frame[i+st]  = (q2 + q1 + q0 + p0 + 2) >> 2;
            frame[i-st2] = (p2 + p1 + p0 + q0 + 2) >> 2;
            frame[i+st2] = ((q3<<1) + (q2*3) + q1 + q0 + p0 + 4) >> 3;
            frame[i-st3] = ((p3<<1) + (p2*3) + p1 + p0 + q0 + 4) >> 3;
         }
         else
         {
            frame[i]    = ((q1<<1) + q0 + p1 + 2) >> 2;
            frame[i-st] = ((p1<<1) + p0 + q1 + 2) >> 2;
         }
      }
   }

   
   private int getFilterStrength(int q, boolean strong)
   {
      return DEFAULT_STRENGTH;
   }
   
   
}
