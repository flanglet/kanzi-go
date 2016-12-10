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

import kanzi.IntFilter;
import kanzi.SliceIntArray;


// Directional deringer implementation based on Daala deringing filter.
// See [The Daala Directional Deringing Filter] by Jean-Marc Valin at 
// https://arxiv.org/pdf/1602.05975.pdf
// Also see https://people.xiph.org/~jm/daala/deringing_demo/
public class DeringingFilter implements IntFilter
{
   private final static int DEFAULT_STEP = 8;
   private final static int DEFAULT_STRENGTH = 3;
   private static final int[] THRESHOLDS = { 0, 2, 4, 8, 12, 16 };
   // for each direction, deltaX,deltaY for even,odd positions
   private static final int[] DIR_INC_XY = { 1,0,1,0,  1,0,1,-1,  1,-1,1,-1,  1,0,1,-1,  0,1,0,1,  0,1,1,1,  1,1,1,1,  1,0,1,1 };
      
   private enum Direction
   {
      HORIZONTAL, // Horizontal
      ANGLE_30,   // Directional 30 degrees
      ANGLE_45,   // Diagonal 45 degrees
      ANGLE_60,   // Directional 60 degrees
      VERTICAL,   // Vertical
      ANGLE_120,  // Directional 120 degrees
      ANGLE_135,  // Diagonal 135 degrees
      ANGLE_150   // Directional 150 degrees
   }
    
 
   private final int width;
   private final int height;
   private final int stride;
   private final int step;


   public DeringingFilter(int width, int height, int stride)
   {
      this(width, height, stride, DEFAULT_STEP);
   }
   
   
   public DeringingFilter(int width, int height, int stride, int step)
   {
      this(width, height, stride, DEFAULT_STEP, DEFAULT_STRENGTH);
   }
   
   
   public DeringingFilter(int width, int height, int stride, int step, int strength)
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

      if (((step & (step-1)) != 0) || (step < 4) || (step > 64))
         throw new IllegalArgumentException("The step must be in [4, 8, 16, 32, 64]");
       
      this.width = width;
      this.height = height;
      this.stride = stride;
      this.step = step;
   }


   // Implementation of IntFilter. Apply filter to step x step squares
   // Image must be unpacked.
   @Override
   public boolean apply(SliceIntArray input, SliceIntArray output)
   {
      if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
         return false;
       
      if (input.array != output.array)
         System.arraycopy(input.array, input.index, output.array, output.index, this.stride*this.height);
      
      final int x0 = output.index % this.stride;
      final int y0 = output.index / this.stride;

      for (int y=0; y<this.height; y+=this.step) 
      {         
         for (int x=0; x<this.width; x+=this.step) 
            this.apply(output.array, x+x0, y+y0, this.step, DEFAULT_STRENGTH);
      }
      
      return true;
   }
    
   
   // strength is in [0..5] where 0 means disabled.
   public boolean apply(int[] frame, int x, int y, int blockDim, int strength)
   {
      if ((strength < 0) || (strength >= THRESHOLDS.length))
         return false;
    
      if (((blockDim & (blockDim-1)) != 0) || (blockDim < 4) || (blockDim > 64))
         return false;

      if (strength == 0)
         return true;

      Direction dir = this.computeDirection(frame, x, y, blockDim);  
      ;
      this.applyDirectional(frame, x, y, blockDim, dir, THRESHOLDS[strength]);
      this.applyAntiDirectional(frame, x, y, blockDim, dir, THRESHOLDS[strength]);
      return true;
   }
 
   
   private Direction computeDirection(int[] input, int x, int y, int blockDim)
   {
      // Skip borders
      if ((x < 3) || (x+blockDim+3 >= this.width)) 
         return Direction.VERTICAL;

      if ((y < 3) || (y+blockDim+3 >= this.height))
         return Direction.HORIZONTAL;

      final int st = this.stride;
      final int start = (y*st) + x;
      final int endj = start + (st*blockDim);
      Direction res = Direction.HORIZONTAL;
      int minSAD = Integer.MAX_VALUE;
     
      for (Direction dir : Direction.values())
      {
         // Compute Sum of Absolute Differences
         int sad = 0;
         final int dXEven = DIR_INC_XY[dir.ordinal()<<2];
         final int dXOdd  = DIR_INC_XY[(dir.ordinal()<<2)+2];
         final int dYEven = DIR_INC_XY[(dir.ordinal()<<2)+1] * st;
         final int dYOdd  = DIR_INC_XY[(dir.ordinal()<<2)+3] * st;

         for (int j=start; j<endj; j+=st)
         {
            for (int i=0; i<blockDim; )
            { 
               // Find which pixel is the preceding one in the scanning order 
               // defined by the direction 
               {
                  final int offs = j + i;
                  final int val = (input[offs] & 0xFF) - (input[offs-dXEven-dYEven] & 0xFF);
                  sad += ((val + (val >> 31)) ^ (val >> 31)); //abs                                   
                  i++;
               } 
               
               {
                  final int offs = j + i;
                  final int val = (input[offs] & 0xFF) - (input[offs-dXOdd-dYOdd] & 0xFF);
                  sad += ((val + (val >> 31)) ^ (val >> 31)); //abs
                  i++;
               }              
            }         
         }
        
         if (sad < minSAD)
         {
            minSAD = sad;
            res = dir;
            
            if (minSAD == 0)
               break;
         }
      }
     
      return res;
   }

   
   // Apply filter along the detected main direction 
   // The filter is a Conditional Replacement Filter: a 7-tap median like filter
   // The conditional replacement filter (CRF) operates by excluding from the averaging
   // the pixel values that are too different from the filtered pixel x(n) to be
   // just ringing. It uses a threshold T to decide whether a pixel value is close 
   // enough. Any value that differs by more than T is replaced (in the filter 
   // computation only) by the value of the center pixel.   
   private void applyDirectional(int[] frame, int x, int y, int blockDim, Direction dir, int threshold)
   {
      // Skip borders
      if ((x < 3) || (x+blockDim+3 >= this.width) || (y < 3) || (y+blockDim+3 >= this.height))
         return;
      
      final int st = this.stride;
      final int dXEven = DIR_INC_XY[dir.ordinal()<<2];
      final int dXOdd  = DIR_INC_XY[(dir.ordinal()<<2)+2];
      final int dYEven = DIR_INC_XY[(dir.ordinal()<<2)+1] * st;
      final int dYOdd  = DIR_INC_XY[(dir.ordinal()<<2)+3] * st;
      final int delta03 = dXOdd + 2*dXEven + dYOdd + 2*dYEven;
      final int delta02 = dXOdd + dXEven + dYOdd + dYEven;
      final int delta01 = dXEven + dYEven;
      final int delta13 = dXEven + 2*dXOdd + dYEven + 2*dYOdd;
      final int delta12 = dXEven + dXOdd + dYEven + dYOdd;
      final int delta11 = dXOdd + dYOdd;
      final int start = y*st + x;
      final int endj = start + st*blockDim;
      final int endi = blockDim;

      for (int j=start; j<endj; j+=st)
      {
         for (int i=0; i<endi; )
         {
            int p0, p1;
        
            {
               // Odd offset
               int sum = 0;
               final int offs = j + i;
               final int pix = frame[offs] & 0xFF;  
               p0 = (frame[offs+delta03] & 0xFF) - pix;
               p1 = (frame[offs-delta03] & 0xFF) - pix;
               if ((p0 < threshold) && (-p0 < threshold)) sum += 4*p0;
               if ((p1 < threshold) && (-p1 < threshold)) sum += 4*p1;            
               p0 = (frame[offs+delta02] & 0xFF) - pix;
               p1 = (frame[offs-delta02] & 0xFF) - pix;
               if ((p0 < threshold) && (-p0 < threshold)) sum += 5*p0;
               if ((p1 < threshold) && (-p1 < threshold)) sum += 5*p1;            
               p0 = (frame[offs+delta01] & 0xFF) - pix;
               p1 = (frame[offs-delta01] & 0xFF) - pix;
               if ((p0 < threshold) && (-p0 < threshold)) sum += 7*p0;
               if ((p1 < threshold) && (-p1 < threshold)) sum += 7*p1;                       
               frame[offs] = pix + ((sum + 0) >> 5);
               i++;
            }
            
            if (i < endi)               
            {
               // Even offset
               int sum = 0;
               final int offs = j + i;
               final int pix = frame[offs] & 0xFF;  
               p0 = (frame[offs+delta13] & 0xFF) - pix;
               p1 = (frame[offs-delta13] & 0xFF) - pix;
               if ((p0 < threshold) && (-p0 < threshold)) sum += 4*p0;
               if ((p1 < threshold) && (-p1 < threshold)) sum += 4*p1;            
               p0 = (frame[offs+delta12] & 0xFF) - pix;
               p1 = (frame[offs-delta12] & 0xFF) - pix;
               if ((p0 < threshold) && (-p0 < threshold)) sum += 5*p0;
               if ((p1 < threshold) && (-p1 < threshold)) sum += 5*p1;            
               p0 = (frame[offs+delta11] & 0xFF) - pix;
               p1 = (frame[offs-delta11] & 0xFF) - pix;
               if ((p0 < threshold) && (-p0 < threshold)) sum += 7*p0;
               if ((p1 < threshold) && (-p1 < threshold)) sum += 7*p1;                       
               frame[offs] = pix + ((sum + 0) >> 5);
               i++;
            }
         }        
      }
   }
   
   
   // Apply filter (roughly) orthogonally to the detected direction using a 5-tap CRF
   private void applyAntiDirectional(int[] frame, int x, int y, int blockDim, Direction dir, int threshold)
   {
      if ((x < 2) || (x+blockDim+2 >= this.width) || (y < 2) || (y+blockDim+2 >= this.height))
         return;
            
      final int st = this.stride;
      final int start = y*st + x;
      final int endj = start + st*blockDim;
      final int endi = blockDim;
      threshold = (threshold * 3) >> 2; // reduce threshold
      final int delta = (dir.ordinal() < Direction.VERTICAL.ordinal()) ? st : 1;

      for (int j=start; j<endj; j+=st)
      {
         for (int i=0; i<endi; i++)
         {
            int p0, p1;
            int sum = 0;
            final int offs = j + i;
            final int pix = frame[offs] & 0xFF; 
            p0 = (frame[offs+delta+delta] & 0xFF) - pix;
            p1 = (frame[offs-delta-delta] & 0xFF) - pix;
            if ((p0 < threshold) && (-p0 < threshold)) sum += p0;
            if ((p1 < threshold) && (-p1 < threshold)) sum += p1;               
            p0 = (frame[offs+delta] & 0xFF) - pix;
            p1 = (frame[offs-delta] & 0xFF) - pix;
            if ((p0 < threshold) && (-p0 < threshold)) sum += p0;
            if ((p1 < threshold) && (-p1 < threshold)) sum += p1;
            frame[offs] = pix + ((sum + 0) >> 2);
         }
      }
   }
}
