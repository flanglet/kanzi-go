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

package kanzi.prediction;

import java.util.TreeSet;


// Class used to predict a block based on its neighbors in the frame
public class IntraPredictor
{
   public enum Mode
   {
      HORIZONTAL,  // Horizontal
      VERTICAL,    // Vertical (top)
      DC,          // DC
      AVERAGE_HV,  // Average horizontal-vertical
      MEDIAN,      // Median horizontal-vertical-dc
      DIAGONAL,    // Diagonal
      BILINEAR,    // Bilinear interpolation
      REFERENCE;   // Other block used as reference

      // 'direct' mode block can be encoded as reference prediction mode
      // with same frame and deltaX=deltaY=0
      
      public static Mode getMode(int val)
      {
         if (val == BILINEAR.ordinal())
            return BILINEAR;

         if (val == MEDIAN.ordinal())
            return MEDIAN;

         if (val == AVERAGE_HV.ordinal())
            return AVERAGE_HV;

         if (val == DC.ordinal())
            return DC;

         if (val == HORIZONTAL.ordinal())
            return HORIZONTAL;

         if (val == VERTICAL.ordinal())
            return VERTICAL;

         if (val == DIAGONAL.ordinal())
            return DIAGONAL;

         if (val == REFERENCE.ordinal())
            return REFERENCE;

         return null;
      }
   };

   private static final int ACTION_POPULATE  = 1;
   private static final int ACTION_GET_INDEX = 2;
   private static final int ACTION_GET_COORD = 3;   

   public static final int DIR_LEFT  = 1;
   public static final int DIR_RIGHT = 2;
   public static final int REFERENCE = 4;

   public static final int MAX_ERROR = 1 << 26; // Not Integer.Max to avoid add overflow
   private static final int DEFAULT_VALUE = 0;

   private final int width;
   private final int height;
   private final int stride;
   private final TreeSet<SearchBlockContext> set; // used during reference search
   private final int thresholdSAD; // used to trigger reference search
   private final int refSearchStepRatio;
   private final boolean isRGB;
   private final int maxBlockDim;
   private final Prediction refPrediction;


   public IntraPredictor(int width, int height, int maxBlockDim)
   {
      this(width, height, width, maxBlockDim, true);
   }


   public IntraPredictor(int width, int height, int maxBlockDim, int stride, boolean isRGB)
   {
      this(width, height, maxBlockDim, stride, isRGB, 5);
   }


   public IntraPredictor(int width, int height, int maxBlockDim, int stride,
           boolean isRGB, int errThreshold)
   {
      this(width, height, maxBlockDim, stride, isRGB, errThreshold, 4);
   }


   // errThreshold is the residue error per pixel that would trigger a spatial
   // search for neighbor blocks. It is checked at the end of the 1st step of
   // prediction to possibly trigger a 2nd step (if the residue error is too high).
   // a value of 0 means that the spatial search happens always (except if the
   // residue error per pixel is 0 at the end of step 1)
   // a value of 256 means that the spatial search never happens.
   // refSearchStepRatio can be 1/8,2/8,4/8 or 8/8. It indicates the reference
   // search step size compared to the block dimension (1/8 is excluded if block
   // dim is 4)
   public IntraPredictor(int width, int height, int maxBlockDim, int stride,
           boolean isRGB, int errThreshold, int refSearchStepRatio)
   {
     if (height < 8)
        throw new IllegalArgumentException("The height must be at least 8");

     if (width < 8)
        throw new IllegalArgumentException("The width must be at least 8");

     if (stride < 8)
        throw new IllegalArgumentException("The stride must be at least 8");

     if ((height & 3) != 0)
        throw new IllegalArgumentException("The height must be a multiple of 4");

     if ((width & 3) != 0)
        throw new IllegalArgumentException("The width must be a multiple of 4");

     if ((stride & 3) != 0)
        throw new IllegalArgumentException("The stride must be a multiple of 4");

     if ((maxBlockDim < 4) || (maxBlockDim > 64))
        throw new IllegalArgumentException("The maximum block dimension must be in the [4..64] range"); // for now

     if ((maxBlockDim & 3) != 0)
        throw new IllegalArgumentException("The maximum block dimension must be a multiple of 4");

     if ((errThreshold < 0) || (errThreshold > 256))
        throw new IllegalArgumentException("The residue error threshold per pixel must in [0..256]");

     if ((refSearchStepRatio != 1) && (refSearchStepRatio != 2) &&
             (refSearchStepRatio != 4) && (refSearchStepRatio != 8))
        throw new IllegalArgumentException("The reference search step ratio must "
                + "be in [1,1/2,1/4,1/8] of the block dimension");

     this.height = height;
     this.width = width;
     this.stride = stride;
     this.set = new TreeSet<SearchBlockContext>();
     this.thresholdSAD = errThreshold;
     this.maxBlockDim = maxBlockDim;
     this.isRGB = isRGB;
     this.refPrediction = new Prediction(maxBlockDim);
     this.refSearchStepRatio = refSearchStepRatio;
   }


   // Compute block prediction (from other blocks) using several different methods (modes)
   // Another block (spatial or temporal) can be provided optionally
   // The input arrays must be frame channels (R,G,B or Y,U,V)
   // input is a block in a frame at offset iy*stride+ix
   // output is the difference block (a.k.a residual block)
   // return index of best prediction
   public int computeResidues(int[] input, int ix, int iy,
           int[] other, int ox, int oy,
           Prediction[] predictions, int blockDim, int predictionType)
   {
      if ((ix < 0) || (ix >= this.width) || (iy < 0) || (iy >= this.height))
         return -1;

      // The block dimension must be a multiple of 4
      if ((blockDim & 3) != 0)
         return -1;

      // Check block dimension
      if (blockDim > this.maxBlockDim)
         return -1;

      // Reference cannot be empty
      if (((predictionType & DIR_RIGHT) == 0) && ((predictionType & DIR_LEFT) == 0)
              && ((predictionType & REFERENCE) == 0))
         return -1;

      // Both directions at the same time are not allowed
      if (((predictionType & DIR_RIGHT) != 0) && ((predictionType & DIR_LEFT) != 0))
         return -1;

      int minIdx = 0;

      // Intialize predictions
      for (Prediction p : predictions)
      {
         p.sad = MAX_ERROR;
         p.x = ix;
         p.y = iy;
         p.frame = input;
         p.blockDim = blockDim;
      }

      // Start with spatial/temporal reference (if any)
      if (other != null)
      {
         Prediction p = predictions[Mode.REFERENCE.ordinal()];
         p.frame = other;
         p.x = ox;
         p.y = oy;
         p.sad = this.computeReferenceDiff(input, iy*this.stride+ix,
                 other, oy*this.stride+ox, p.residue, blockDim);

         if (p.sad == 0)
           return Mode.REFERENCE.ordinal();

         minIdx = Mode.REFERENCE.ordinal();
      }

      // Compute block residues based on prediction modes
      this.computePredictionDiff(input, ox, oy, predictions, blockDim, predictionType);

      // Find best prediction
      for (int i=0; i<predictions.length; i++)
      {
         // Favor lower modes (less bits to encode than reference mode)
         if (predictions[minIdx].sad > predictions[i].sad)
            minIdx = i;
      }

      final int minSAD = predictions[minIdx].sad;

      if (minSAD == 0)
         return minIdx;

      // If the error of the best prediction is not low 'enough' and the
      // spatial reference is set, start a spatial search
      if (((predictionType & REFERENCE) != 0) && (minSAD >= blockDim * blockDim * this.thresholdSAD))
      {
         // Spatial search of best matching nearby block
         final Prediction newPrediction = this.refPrediction;
         newPrediction.frame = input;
         newPrediction.blockDim = blockDim;

         // Do the search and update prediction error, coordinates and result block
         this.computeReferenceSearch(input, ix, iy, minSAD, newPrediction, predictionType);
         final Prediction refPred = predictions[Mode.REFERENCE.ordinal()];

         // Is the new prediction an improvement ?
         if (newPrediction.sad < refPred.sad)
         {
            refPred.x = newPrediction.x;
            refPred.y = newPrediction.y;
            refPred.sad = newPrediction.sad;

            // Create residue block for reference mode
            this.computeReferenceDiff(input, iy*this.stride+ix, input,
                    newPrediction.y*this.stride+newPrediction.x,
                    newPrediction.residue, blockDim);

            System.arraycopy(newPrediction.residue, 0, refPred.residue, 0, newPrediction.residue.length);

            if (predictions[minIdx].sad > refPred.sad)
               minIdx = Mode.REFERENCE.ordinal();
         }
      }

      return minIdx;
   }


   // Compute residue against another (spatial/temporal) block
   // Return error of difference block
   private int computeReferenceDiff(int[] input, int iIdx, int[] other, int oIdx,
           int[] output, int blockDim)
   {
      final int st = this.stride;
      final int endj = iIdx + (st * blockDim);
      final int mask = (this.isRGB == true) ? 0xFF : -1;
      int k = 0;
      int sad = 0;

      if (other == null)
      {
          // Simple copy
          for (int j=iIdx; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i+=4)
             {
                final int val0 = input[i]   & mask;
                final int val1 = input[i+1] & mask;
                final int val2 = input[i+2] & mask;
                final int val3 = input[i+3] & mask;
                output[k]   = val0;
                output[k+1] = val1;
                output[k+2] = val2;
                output[k+3] = val3;
                k += 4;
                sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
             }
          }
      }
      else
      {
         // Block delta
         for (int j=iIdx; j<endj; j+=st)
         {
            final int endi = j + blockDim;

            for (int i=j; i<endi; i+=4)
            {
                final int val0 = (input[i]   & mask) - (other[oIdx]   & mask);
                final int val1 = (input[i+1] & mask) - (other[oIdx+1] & mask);
                final int val2 = (input[i+2] & mask) - (other[oIdx+2] & mask);
                final int val3 = (input[i+3] & mask) - (other[oIdx+3] & mask);
                output[k]   = val0;
                output[k+1] = val1;
                output[k+2] = val2;
                output[k+3] = val3;
                k += 4;
                oIdx += 4;
                sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
            }

            oIdx += (st - blockDim);
         }
      }

      return sad;
   }


   // Compute residue based on several prediction modes
   // Return predictions
   // Proceed line by line (for cache) and avoid branches when possible (for speed)
   // Example 8x8
   //   d   a0 a1 a2 a3 a4 a5 a6 a7   e
   //  b0   x0 x1 x2 x3 x4 x5 x6 x7   c0
   //  b1   x0 x1 x2 x3 x4 x5 x6 x7   c1
   //  b2   x0 x1 x2 x3 x4 x5 x6 x7   c2
   //  b3   x0 x1 x2 x3 x4 x5 x6 x7   c3
   //  b4   x0 x1 x2 x3 x4 x5 x6 x7   c4
   //  b5   x0 x1 x2 x3 x4 x5 x6 x7   c5
   //  b6   x0 x1 x2 x3 x4 x5 x6 x7   c6
   //  b7   x0 x1 x2 x3 x4 x5 x6 x7   c7
   private Prediction[] computePredictionDiff(int[] input, int x, int y,
           Prediction[] predictions, int blockDim, int direction)
   {
      if (((direction & DIR_LEFT) == 0) && ((direction & DIR_RIGHT) == 0))
         return predictions;

      final int xMax = this.width - blockDim;
      final int st = this.stride;
      final int start = y*st + x;
      final int endj = start + (st * blockDim);
      final int mask = (this.isRGB == true) ? 0xFF : -1;
      int line = 0;
      int idx_l = 0;
      int dc_l = 0;
      int dc_r = 0;
      int sum_l = 0;
      int sum_r = 0;
      int d_l = DEFAULT_VALUE;
      int d_r = DEFAULT_VALUE;      
      int aa = DEFAULT_VALUE;
      int bb = DEFAULT_VALUE;
      int xy = DEFAULT_VALUE;
      final int maxIndex = blockDim - 1;
      final int adjust = maxIndex >> 1;

      predictions[Mode.VERTICAL.ordinal()].sad = 0;
      predictions[Mode.HORIZONTAL.ordinal()].sad = 0;
      predictions[Mode.DC.ordinal()].sad = 0;
      predictions[Mode.DIAGONAL.ordinal()].sad = 0;
      predictions[Mode.MEDIAN.ordinal()].sad = 0;
      predictions[Mode.AVERAGE_HV.ordinal()].sad = 0;
      predictions[Mode.BILINEAR.ordinal()].sad = 0;

      // Initializations
      if ((direction & DIR_LEFT) != 0)
      {
         if (x > 0)
         {
            sum_l += blockDim;

            for (int j=start; j<endj; j+=st)
               dc_l += (input[j-1] & mask);

            // Pixel left of last row
            bb = input[start+(st*blockDim)-st-1] & mask;
         }
         
         // Last pixel above first row
         if (y > 0) 
            aa = input[start-st+blockDim-1] & mask;
         
         // Last pixel, last row
         xy = input[start+(st*blockDim)-st+blockDim-1] & mask;
         predictions[Mode.BILINEAR.ordinal()].anchor = xy;
      }

      if ((direction & DIR_RIGHT) != 0)
      {
         if (x < xMax)
         {
            sum_r += blockDim;

            for (int j=start; j<endj; j+=st)
               dc_r += (input[j+blockDim] & mask);

            // Pixel right of last row
            bb = input[start+(st*blockDim)-st+blockDim] & mask;
         }
         
         // First pixel above first row
         if (y > 0) 
            aa = input[start-st] & mask;
         
         // First pixel, last row
         xy = input[start+(st*blockDim)-st] & mask ;
         predictions[Mode.BILINEAR.ordinal()].anchor = xy;
      }

      if (y > 0)
      {
         final int above = start - st;

         if ((direction & DIR_LEFT) != 0)
         {
            sum_l += blockDim;

            for (int i=0; i<blockDim; i++)
               dc_l += (input[above+i] & mask);

            if (x > 0)
               d_l = input[above-1] & mask;
         }

         if ((direction & DIR_RIGHT) != 0)
         {
            sum_r += blockDim;

            for (int i=0; i<blockDim; i++)
               dc_r += (input[above+i] & mask);

            if (x < xMax)
               d_r = input[above+blockDim] & mask;
         }
      }

      if (sum_l != 0)
         dc_l = (dc_l + (sum_l >> 1)) / sum_l;
      else
         dc_l = DEFAULT_VALUE;

      if (sum_r != 0)
         dc_r = (dc_r + (sum_r >> 1)) / sum_r;
      else
         dc_r = DEFAULT_VALUE;
              
      // Main loop, line by line
      for (int j=0,offs=start; offs<endj; offs+=st, j++)
      {
         final int endi = offs + blockDim;

         if ((direction & DIR_LEFT) != 0)
         {
            // Pixel column to the left for current row
            final int b = (x > 0) ? input[offs-1] & mask : DEFAULT_VALUE;
            
            // Interpolated pixel at end of current row
            final int xv = ((xy*j) + (aa*(maxIndex-j)) + adjust) / maxIndex;
            
            // Scan from the left to the right
            for (int i=offs; i<endi; i+=4)
            {
               final int ii = i - offs;
               final int x0 = input[i]   & mask;
               final int x1 = input[i+1] & mask;
               final int x2 = input[i+2] & mask;
               final int x3 = input[i+3] & mask;
               int a0, a1, a2, a3;

               if (y > 0)
               {
                  final int blockAbove = ii + start - st;
                  a0 = input[blockAbove]   & mask;
                  a1 = input[blockAbove+1] & mask;
                  a2 = input[blockAbove+2] & mask;
                  a3 = input[blockAbove+3] & mask;
               }
               else
               {
                  a0 = DEFAULT_VALUE;
                  a1 = DEFAULT_VALUE;
                  a2 = DEFAULT_VALUE;
                  a3 = DEFAULT_VALUE;
               }

               {
                  // HORIZONTAL_L: xi-bi
                  final int val0 = x0 - b;
                  final int val1 = x1 - b;
                  final int val2 = x2 - b;
                  final int val3 = x3 - b;
                  final Prediction p = predictions[Mode.HORIZONTAL.ordinal()];
                  final int[] output = p.residue;
                  output[idx_l]   = val0;
                  output[idx_l+1] = val1;
                  output[idx_l+2] = val2;
                  output[idx_l+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               {
                  // AVERAGE_HV_L: (xi,yi)-avg(ai,bi)
                  int avg;
                  avg = (b + a0) >> 1;
                  final int val0 = x0 - avg;
                  avg = (b + a1) >> 1;
                  final int val1 = x1 - avg;
                  avg = (b + a2) >> 1;
                  final int val2 = x2 - avg;
                  avg = (b + a3) >> 1;
                  final int val3 = x3 - avg;
                  final Prediction p = predictions[Mode.AVERAGE_HV.ordinal()];
                  final int[] output = p.residue;
                  output[idx_l]   = val0;
                  output[idx_l+1] = val1;
                  output[idx_l+2] = val2;
                  output[idx_l+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               {
                  // MEDIAN_L: (xi,yi)-MEDIAN(ai, bi, (ai+bi)-d_l)
                  final int val = b - d_l;
                  int med;
                  med = val + a0;

                  if (a0 < b)
                     med = (med < a0) ? a0 : ((med < b) ? med : b);
                  else
                     med = (med < b) ? b : ((med < a0) ? med : a0);

                  final int val0 = x0 - med;
                  med = val + a1;

                  if (a1 < b)
                     med = (med < a1) ? a1 : ((med < b) ? med : b);
                  else
                     med = (med < b) ? b : ((med < a1) ? med : a1);

                  final int val1 = x1 - med;
                  med = val + a2;

                  if (a2 < b)
                     med = (med < a2) ? a2 : ((med < b) ? med : b);
                  else
                     med = (med < b) ? b : ((med < a2) ? med : a2);

                  final int val2 = x2 - med;
                  med = val + a3;

                  if (a3 < b)
                     med = (med < a3) ? a3 : ((med < b) ? med : b);
                  else
                     med = (med < b) ? b : ((med < a3) ? med : a3);

                  final int val3 = x3 - med;
                  final Prediction p = predictions[Mode.MEDIAN.ordinal()];
                  final int[] output = p.residue;
                  output[idx_l]   = val0;
                  output[idx_l+1] = val1;
                  output[idx_l+2] = val2;
                  output[idx_l+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               {
                  // BILINEAR_L
                  int xh;
                  xh = (xy*ii     + bb*(maxIndex-ii)     + adjust) / maxIndex;
                  final int val0 = x0 - ((xv + xh + 1) >> 1);
                  xh = (xy*(ii+1) + bb*(maxIndex-(ii+1)) + adjust) / maxIndex;
                  final int val1 = x1 - ((xv + xh + 1) >> 1);
                  xh = (xy*(ii+2) + bb*(maxIndex-(ii+2)) + adjust) / maxIndex;
                  final int val2 = x2 - ((xv + xh + 1) >> 1);
                  xh = (xy*(ii+3) + bb*(maxIndex-(ii+3)) + adjust) / maxIndex;
                  final int val3 = x3 - ((xv + xh + 1) >> 1);
                  final Prediction p = predictions[Mode.BILINEAR.ordinal()];
                  final int[] output = p.residue;
                  output[idx_l]   = val0;
                  output[idx_l+1] = val1;
                  output[idx_l+2] = val2;
                  output[idx_l+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }
               
               {
                  // DC_L: xi-dc_l
                  final int val0 = x0 - dc_l;
                  final int val1 = x1 - dc_l;
                  final int val2 = x2 - dc_l;
                  final int val3 = x3 - dc_l;
                  final Prediction p = predictions[Mode.DC.ordinal()];
                  final int[] output = p.residue;
                  output[idx_l]   = val0;
                  output[idx_l+1] = val1;
                  output[idx_l+2] = val2;
                  output[idx_l+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               {
                  // DIAGONAL_L
                  final int iV = ii - j - 1 + start - st;
                  final int iH = start + (j-ii-1)*st - 1;
                  final int y0 = (ii   >= j) ? ((y > 0) ? input[iV]   & mask : DEFAULT_VALUE)
                          : ((x > 0) ? input[iH]      & mask : DEFAULT_VALUE);
                  final int y1 = (ii+1 >= j) ? ((y > 0) ? input[iV+1] & mask : DEFAULT_VALUE)
                          : ((x > 0) ? input[iH-st]   & mask : DEFAULT_VALUE);
                  final int y2 = (ii+2 >= j) ? ((y > 0) ? input[iV+2] & mask : DEFAULT_VALUE)
                          : ((x > 0) ? input[iH-st*2] & mask : DEFAULT_VALUE);
                  final int y3 = (ii+3 >= j) ? ((y > 0) ? input[iV+3] & mask : DEFAULT_VALUE)
                          : ((x > 0) ? input[iH-st*3] & mask : DEFAULT_VALUE);
                  final int val0 = x0 - y0;
                  final int val1 = x1 - y1;
                  final int val2 = x2 - y2;
                  final int val3 = x3 - y3;
                  final Prediction p = predictions[Mode.DIAGONAL.ordinal()];
                  final int[] output = p.residue;
                  output[idx_l]   = val0;
                  output[idx_l+1] = val1;
                  output[idx_l+2] = val2;
                  output[idx_l+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               {
                  // VERTICAL: xi-ai
                  final int val0 = x0 - a0;
                  final int val1 = x1 - a1;
                  final int val2 = x2 - a2;
                  final int val3 = x3 - a3;
                  final Prediction p = predictions[Mode.VERTICAL.ordinal()];
                  final int[] output = p.residue;
                  output[idx_l]   = val0;
                  output[idx_l+1] = val1;
                  output[idx_l+2] = val2;
                  output[idx_l+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               idx_l += 4;
            }
         } // LEFT

         if ((direction & DIR_RIGHT) != 0)
         {
            final int c = (x < xMax) ? input[endi] & mask : DEFAULT_VALUE;

            // Interpolated pixel at start of current row
            final int xv = ((xy*j) + (aa*(maxIndex-j)) + adjust) / maxIndex;

            // Scan from right to left
            for (int i=endi-4; i>=offs; i-=4)
            {
               final int ii = i - offs;
               final int idx_r = line + ii;
               final int x0 = input[i]   & mask;
               final int x1 = input[i+1] & mask;
               final int x2 = input[i+2] & mask;
               final int x3 = input[i+3] & mask;
               int a0, a1, a2, a3;

               if (y > 0)
               {
                  final int blockAbove = ii + start - st;
                  a0 = input[blockAbove]   & mask;
                  a1 = input[blockAbove+1] & mask;
                  a2 = input[blockAbove+2] & mask;
                  a3 = input[blockAbove+3] & mask;
               }
               else
               {
                  a0 = DEFAULT_VALUE;
                  a1 = DEFAULT_VALUE;
                  a2 = DEFAULT_VALUE;
                  a3 = DEFAULT_VALUE;
               }

               {
                  // HORIZONTAL_R: xi-ci
                  final int val0 = x0 - c;
                  final int val1 = x1 - c;
                  final int val2 = x2 - c;
                  final int val3 = x3 - c;
                  final Prediction p = predictions[Mode.HORIZONTAL.ordinal()];
                  final int[] output = p.residue;
                  output[idx_r]   = val0;
                  output[idx_r+1] = val1;
                  output[idx_r+2] = val2;
                  output[idx_r+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               {
                  // AVERAGE_HV_R: (xi,yi)-avg(ai, ci)
                  int avg;
                  avg = (c + a0) >> 1;
                  final int val0 = x0 - avg;
                  avg = (c + a1) >> 1;
                  final int val1 = x1 - avg;
                  avg = (c + a2) >> 1;
                  final int val2 = x2 - avg;
                  avg = (c + a3) >> 1;
                  final int val3 = x3 - avg;
                  final Prediction p = predictions[Mode.AVERAGE_HV.ordinal()];
                  final int[] output = p.residue;
                  output[idx_r]   = val0;
                  output[idx_r+1] = val1;
                  output[idx_r+2] = val2;
                  output[idx_r+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               {
                  // MEDIAN_R: (xi,yi)-MEDIAN(ai, ci, (ai+ci)-d_r)
                  final int val = c - d_r;
                  int med;
                  med = val + a0;

                  if (a0 < c)
                     med = (med < a0) ? a0 : ((med < c) ? med : c);
                  else
                     med = (med < c) ? c : ((med < a0) ? med : a0);

                  final int val0 = x0 - med;
                  med = val + a1;

                  if (a1 < c)
                     med = (med < a1) ? a1 : ((med < c) ? med : c);
                  else
                     med = (med < c) ? c : ((med < a1) ? med : a1);

                  final int val1 = x1 - med;
                  med = val + a2;

                  if (a2 < c)
                     med = (med < a2) ? a2 : ((med < c) ? med : c);
                  else
                     med = (med < c) ? c : ((med < a2) ? med : a2);

                  final int val2 = x2 - med;
                  med = val + a3;

                  if (a3 < c)
                     med = (med < a3) ? a3 : ((med < c) ? med : c);
                  else
                     med = (med < c) ? c : ((med < a3) ? med : a3);

                  final int val3 = x3 - med;
                  final Prediction p = predictions[Mode.MEDIAN.ordinal()];
                  final int[] output = p.residue;
                  output[idx_r]   = val0;
                  output[idx_r+1] = val1;
                  output[idx_r+2] = val2;
                  output[idx_r+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }
               
               {
                  // BILINEAR_R
                  int xh;
                  xh = (bb*ii     + xy*(maxIndex-ii)     + adjust) / maxIndex;
                  final int val0 = x0 - ((xv + xh + 1) >> 1);
                  xh = (bb*(ii+1) + xy*(maxIndex-(ii+1)) + adjust) / maxIndex;
                  final int val1 = x1 - ((xv + xh + 1) >> 1);
                  xh = (bb*(ii+2) + xy*(maxIndex-(ii+2)) + adjust) / maxIndex;
                  final int val2 = x2 - ((xv + xh + 1) >> 1);
                  xh = (bb*(ii+3) + xy*(maxIndex-(ii+3)) + adjust) / maxIndex;
                  final int val3 = x3 - ((xv + xh + 1) >> 1);
                  final Prediction p = predictions[Mode.BILINEAR.ordinal()];
                  final int[] output = p.residue;
                  output[idx_r]   = val0;
                  output[idx_r+1] = val1;
                  output[idx_r+2] = val2;
                  output[idx_r+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               {
                  // DC_R: xi-dc_r
                  final int val0 = x0 - dc_r;
                  final int val1 = x1 - dc_r;
                  final int val2 = x2 - dc_r;
                  final int val3 = x3 - dc_r;
                  final Prediction p = predictions[Mode.DC.ordinal()];
                  final int[] output = p.residue;
                  output[idx_r]   = val0;
                  output[idx_r+1] = val1;
                  output[idx_r+2] = val2;
                  output[idx_r+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               {
                  // DIAGONAL_R
                  final int diag = endi - 1 - offs;
                  final int iV = ii + j + 1 + start - st;
                  final int iH = start + (ii+j-diag-1)*st + diag + 1;
                  final int y0 = (ii+j   <= diag) ? ((y > 0) ? input[iV]   & mask : DEFAULT_VALUE)
                          : ((x < xMax) ? input[iH]      & mask : DEFAULT_VALUE);
                  final int y1 = (ii+1+j <= diag) ? ((y > 0) ? input[iV+1] & mask : DEFAULT_VALUE)
                          : ((x < xMax) ? input[iH+st]   & mask : DEFAULT_VALUE);
                  final int y2 = (ii+2+j <= diag) ? ((y > 0) ? input[iV+2] & mask : DEFAULT_VALUE)
                          : ((x < xMax) ? input[iH+st*2] & mask : DEFAULT_VALUE);
                  final int y3 = (ii+3+j <= diag) ? ((y > 0) ? input[iV+3] & mask : DEFAULT_VALUE)
                          : ((x < xMax) ? input[iH+st*3] & mask : DEFAULT_VALUE);
                  final int val0 = x0 - y0;
                  final int val1 = x1 - y1;
                  final int val2 = x2 - y2;
                  final int val3 = x3 - y3;
                  final Prediction p = predictions[Mode.DIAGONAL.ordinal()];
                  final int[] output = p.residue;
                  output[idx_r]   = val0;
                  output[idx_r+1] = val1;
                  output[idx_r+2] = val2;
                  output[idx_r+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }

               if ((direction & DIR_LEFT) == 0) // if not already done
               {
                  // VERTICAL: xi-ai
                  final int val0 = x0 - a0;
                  final int val1 = x1 - a1;
                  final int val2 = x2 - a2;
                  final int val3 = x3 - a3;
                  final Prediction p = predictions[Mode.VERTICAL.ordinal()];
                  final int[] output = p.residue;
                  output[idx_r]   = val0;
                  output[idx_r+1] = val1;
                  output[idx_r+2] = val2;
                  output[idx_r+3] = val3;
                  p.sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                  p.sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                  p.sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                  p.sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
               }
            }

            line += blockDim;
         } // RIGHT
      } // y loop

      return predictions;
   }


   // Add residue to other block
   // Return output
   private int[] computeBlockReference(Prediction prediction, int[] output, int oIdx)
   {
      final int st = this.stride;
      final int blockDim = prediction.blockDim;
      final int endj = oIdx + (st * blockDim);
      final int[] residue = prediction.residue;
      final int[] input = prediction.frame;
      int iIdx = (prediction.y * st) + prediction.x;
      int k = 0;

      if (input == null)
      {
          // Simple copy
          for (int j=oIdx; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i+=4)
             {
                output[j]   = residue[k];
                output[j+1] = residue[k+1];
                output[j+2] = residue[k+2];
                output[j+3] = residue[k+3];
                k += 4;
             }
          }
      }
      else
      {
         final int mask = (this.isRGB == true) ? 0xFF : -1;

         for (int j=oIdx; j<endj; j+=st)
         {
            final int endi = j + blockDim;

            for (int i=j; i<endi; i+=4)
            {
                output[i]   = residue[k]   + (input[iIdx]   & mask);
                output[i+1] = residue[k+1] + (input[iIdx+1] & mask);
                output[i+2] = residue[k+2] + (input[iIdx+2] & mask);
                output[i+3] = residue[k+3] + (input[iIdx+3] & mask);
                iIdx += 4;
                k += 4;
            }

            iIdx += (st - blockDim);
         }
      }

      return output;
   }


   // Create block in output at x,y from prediction mode, residue and input
   // residue is a blockDim*blockDim size block
   // input and output are width*height size frames
   public int[] computeBlock(Prediction prediction, int[] output, int x, int y,
           Mode mode, int direction)
   {
      if (mode == Mode.REFERENCE)
         return this.computeBlockReference(prediction, output, (y*this.stride)+x);

      if (mode == Mode.BILINEAR)
         return this.computeBlockBilinear(prediction, output, x, y, direction);

      if (mode == Mode.VERTICAL)
         return this.computeBlockVertical(prediction, output, x, y, direction);

      if (mode == Mode.MEDIAN)
         return this.computeBlockMedian(prediction, output, x, y, direction);

      if (mode == Mode.AVERAGE_HV)
         return this.computeBlockAverageHV(prediction, output, x, y, direction);

      if (mode == Mode.HORIZONTAL)
         return this.computeBlockHorizontal(prediction, output, x, y, direction);

      if (mode == Mode.DC)
         return this.computeBlockDC(prediction, output, x, y, direction);

      if (mode == Mode.DIAGONAL)
         return this.computeBlockDiagonal(prediction, output, x, y, direction);

      return output;
   }


    private int[] computeBlockHorizontal(Prediction prediction, int[] output, int x, int y,
         int direction)
    {
       final int blockDim = prediction.blockDim;
       final int st = this.stride;
       final int start = (y * st) + x;
       final int endj = start + (st * blockDim);
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int mask = (this.isRGB == true) ? 0xFF : -1;
       final int xMax = this.width - blockDim;
       int k = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;
             final int b = (x > 0) ? input[j-1] & mask : DEFAULT_VALUE;

             for (int i=j; i<endi; i+=4)
             {
                // HORIZONTAL_L: xi+bi
                output[i]   = residue[k]   + b;
                output[i+1] = residue[k+1] + b;
                output[i+2] = residue[k+2] + b;
                output[i+3] = residue[k+3] + b;
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;
             final int c = (x < xMax) ? input[endi] & mask : DEFAULT_VALUE;

             for (int i=j; i<endi; i+=4)
             {
                // HORIZONTAL_R: xi+ci
                output[i]   = residue[k]   + c;
                output[i+1] = residue[k+1] + c;
                output[i+2] = residue[k+2] + c;
                output[i+3] = residue[k+3] + c;
                k += 4;
             }
          }
       }

       return output;
    }


    private int[] computeBlockVertical(Prediction prediction, int[] output, int x, int y,
         int direction)
    {
       final int blockDim = prediction.blockDim;
       final int st = this.stride;
       final int start = (y * st) + x;
       final int endj = start + (st * blockDim);
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int mask = (this.isRGB == true) ? 0xFF : -1;
       int k = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i+=4)
             {
                int a0, a1, a2, a3;

                // VERTICAL: xi+ai
                if (y > 0)
                {
                   final int blockAbove = i - j + start - st;
                   a0 = input[blockAbove]   & mask;
                   a1 = input[blockAbove+1] & mask;
                   a2 = input[blockAbove+2] & mask;
                   a3 = input[blockAbove+3] & mask;
                }
                else
                {
                   a0 = DEFAULT_VALUE;
                   a1 = DEFAULT_VALUE;
                   a2 = DEFAULT_VALUE;
                   a3 = DEFAULT_VALUE;
                }

                output[i]   = residue[k]   + a0;
                output[i+1] = residue[k+1] + a1;
                output[i+2] = residue[k+2] + a2;
                output[i+3] = residue[k+3] + a3;
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          int line = 0;

          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=endi-4; i>=j ; i-=4)
             {
                k = line + i - j;
                int a0, a1, a2, a3;

                // VERTICAL: xi+ai
                if (y > 0)
                {
                   final int blockAbove = i - j + start - st;
                   a0 = input[blockAbove]   & mask;
                   a1 = input[blockAbove+1] & mask;
                   a2 = input[blockAbove+2] & mask;
                   a3 = input[blockAbove+3] & mask;
                }
                else
                {
                   a0 = DEFAULT_VALUE;
                   a1 = DEFAULT_VALUE;
                   a2 = DEFAULT_VALUE;
                   a3 = DEFAULT_VALUE;
                }

                output[i]   = residue[k]   + a0;
                output[i+1] = residue[k+1] + a1;
                output[i+2] = residue[k+2] + a2;
                output[i+3] = residue[k+3] + a3;
             }

             line += blockDim;
          }
       }

       return output;
    }


    private int[] computeBlockAverageHV(Prediction prediction, int[] output, int x, int y,
         int direction)
    {
       final int blockDim = prediction.blockDim;
       final int xMax = this.width - blockDim;
       final int st = this.stride;
       final int start = (y * st) + x;
       final int endj = start + (st * blockDim);
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int mask = (this.isRGB == true) ? 0xFF : -1;
       int k = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i+=4)
             {
                final int b = (x > 0) ? input[j-1] & mask : DEFAULT_VALUE;
                int a0, a1, a2, a3;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask;
                   a1 = input[blockAbove+1] & mask;
                   a2 = input[blockAbove+2] & mask;
                   a3 = input[blockAbove+3] & mask;
                }
                else
                {
                   a0 = DEFAULT_VALUE;
                   a1 = DEFAULT_VALUE;
                   a2 = DEFAULT_VALUE;
                   a3 = DEFAULT_VALUE;
                }

                // AVERAGE_HV_L: (xi,yi)+avg(ai, bi)
                output[i]   = residue[k]   + ((b + a0) >> 1);
                output[i+1] = residue[k+1] + ((b + a1) >> 1);
                output[i+2] = residue[k+2] + ((b + a2) >> 1);
                output[i+3] = residue[k+3] + ((b + a3) >> 1);
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          int line = 0;

          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=endi-4; i>=j ; i-=4)
             {
                final int c = (x < xMax) ? input[endi] & mask : DEFAULT_VALUE;
                k = line + i - j;
                int a0, a1, a2, a3;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask;
                   a1 = input[blockAbove+1] & mask;
                   a2 = input[blockAbove+2] & mask;
                   a3 = input[blockAbove+3] & mask;
                }
                else
                {
                   a0 = DEFAULT_VALUE;
                   a1 = DEFAULT_VALUE;
                   a2 = DEFAULT_VALUE;
                   a3 = DEFAULT_VALUE;
                }

                // AVERAGE_HV_R: (xi,yi)+avg(bi, ci)
                output[i]   = residue[k]   + ((c + a0) >> 1);
                output[i+1] = residue[k+1] + ((c + a1) >> 1);
                output[i+2] = residue[k+2] + ((c + a2) >> 1);
                output[i+3] = residue[k+3] + ((c + a3) >> 1);
             }

             line += blockDim;
          }            
       }
     
       return output;
    }

    
    private int[] computeBlockBilinear(Prediction prediction, int[] output, int x, int y,
         int direction)
    {
       final int blockDim = prediction.blockDim;
       final int st = this.stride;
       final int start = (y * st) + x;
       final int endj = start + (st * blockDim);
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int mask = (this.isRGB == true) ? 0xFF : -1;
       final int xMax = this.width - blockDim;
       int k = 0;
       int aa = DEFAULT_VALUE;
       int bb = DEFAULT_VALUE;
       int xy;
       final int maxIndex = blockDim - 1;
       final int adjust = maxIndex >> 1;

       if ((direction & DIR_LEFT) != 0)
       {          
          // Last pixel above first row
          if (y > 0) 
             aa = input[start-st+blockDim-1] & mask;
         
          // Pixel left of last row
          if (x > 0) 
             bb = input[start+(st*blockDim)-st-1] & mask;

          // Last pixel, last row
          xy = prediction.anchor;
          
          for (int offs=start, j=0; offs<endj; offs+=st, j++)
          {
             final int endi = offs + blockDim;
            
             // Interpolated pixel at end of current row
             final int xv = ((xy*j) + (aa*(maxIndex-j)) + adjust) / maxIndex;

             for (int i=offs; i<endi; i+=4)
             {
                // BILINEAR_L
                final int ii = i - offs;
                int xh;
                xh = (xy*ii     + bb*(maxIndex-ii)     + adjust) / maxIndex;
                final int val0 = (xv + xh + 1) >> 1;
                xh = (xy*(ii+1) + bb*(maxIndex-(ii+1)) + adjust) / maxIndex;
                final int val1 = (xv + xh + 1) >> 1;
                xh = (xy*(ii+2) + bb*(maxIndex-(ii+2)) + adjust) / maxIndex;
                final int val2 = (xv + xh + 1) >> 1;
                xh = (xy*(ii+3) + bb*(maxIndex-(ii+3)) + adjust) / maxIndex;
                final int val3 = (xv + xh + 1) >> 1;
                output[i]   = residue[k]   + val0;
                output[i+1] = residue[k+1] + val1;
                output[i+2] = residue[k+2] + val2;
                output[i+3] = residue[k+3] + val3;
                k += 4;                
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          // First pixel above first row
          if (y > 0) 
             aa = input[start-st] & mask;
         
          // Pixel right of last row
          if (x < xMax) 
             bb = input[start+(st*blockDim)-st+blockDim] & mask;

          // First pixel, last row
          xy = prediction.anchor;
          
          int line = 0;
          
          for (int offs=start, j=0; offs<endj; offs+=st, j++)
          {
             final int endi = offs + blockDim;
         
             // Interpolated pixel at start of current row
             final int xv = ((xy*j) + (aa*(maxIndex-j)) + adjust) / maxIndex;
             
             for (int i=endi-4; i>=offs ; i-=4)
             {
                // BILINEAR_R
                final int ii = i - offs;
                k = line + ii;
                int xh;
                xh = (bb*ii     + xy*(maxIndex-ii)     + adjust) / maxIndex;
                final int val0 = (xv + xh + 1) >> 1;
                xh = (bb*(ii+1) + xy*(maxIndex-(ii+1)) + adjust) / maxIndex;
                final int val1 = (xv + xh + 1) >> 1;
                xh = (bb*(ii+2) + xy*(maxIndex-(ii+2)) + adjust) / maxIndex;
                final int val2 = (xv + xh + 1) >> 1;
                xh = (bb*(ii+3) + xy*(maxIndex-(ii+3)) + adjust) / maxIndex;
                final int val3 = (xv + xh + 1) >> 1;
                output[i]   = residue[k]   + val0;
                output[i+1] = residue[k+1] + val1;
                output[i+2] = residue[k+2] + val2;
                output[i+3] = residue[k+3] + val3;
             }

             line += blockDim;
          }
       }
       
       return output;
    }

               
    private int[] computeBlockMedian(Prediction prediction, int[] output, int x, int y,
         int direction)
    {
       final int blockDim = prediction.blockDim;
       final int xMax = this.width - blockDim;
       final int st = this.stride;
       final int start = (y * st) + x;
       final int endj = start + (st * blockDim);
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int mask = (this.isRGB == true) ? 0xFF : -1;
       int k = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          final int d_l = ((y > 0) && (x > 0)) ? input[start-st-1] & mask : DEFAULT_VALUE;
          
          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i+=4)
             {
                final int b = (x > 0) ? input[j-1] & mask : DEFAULT_VALUE;
                int a0, a1, a2, a3;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask;
                   a1 = input[blockAbove+1] & mask;
                   a2 = input[blockAbove+2] & mask;
                   a3 = input[blockAbove+3] & mask;
                }
                else
                {
                   a0 = DEFAULT_VALUE;
                   a1 = DEFAULT_VALUE;
                   a2 = DEFAULT_VALUE;
                   a3 = DEFAULT_VALUE;
                }

                // MEDIAN: (xi,yi)-MEDIAN(ai, bi, (ai+bi)-d)
                final int val = b - d_l;
                int med0 = val + a0;

                if (a0 < b)
                   med0 = (med0 < a0) ? a0 : ((med0 < b) ? med0 : b);
                else
                   med0 = (med0 < b) ? b : ((med0 < a0) ? med0 : a0);

                int med1 = val + a1;

                if (a1 < b)
                   med1 = (med1 < a1) ? a1 : ((med1 < b) ? med1 : b);
                else
                   med1 = (med1 < b) ? b : ((med1 < a1) ? med1 : a1);

                int med2 = val + a2;

                if (a2 < b)
                   med2 = (med2 < a2) ? a2 : ((med2 < b) ? med2 : b);
                else
                   med2 = (med2 < b) ? b : ((med2 < a2) ? med2 : a2);

                int med3 = val + a3;

                if (a3 < b)
                   med3 = (med3 < a3) ? a3 : ((med3 < b) ? med3 : b);
                else
                   med3 = (med3 < b) ? b : ((med3 < a3) ? med3 : a3);

                output[i]   = residue[k]   + med0;
                output[i+1] = residue[k+1] + med1;
                output[i+2] = residue[k+2] + med2;
                output[i+3] = residue[k+3] + med3;
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          final int d_r = ((y > 0) && (x < xMax)) ? input[start-st+blockDim] & mask : DEFAULT_VALUE;
          int line = 0;

          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=endi-4; i>=j ; i-=4)
             {
                final int c = (x < xMax) ? input[endi] & mask : DEFAULT_VALUE;
                k = line + i - j;
                int a0, a1, a2, a3;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask;
                   a1 = input[blockAbove+1] & mask;
                   a2 = input[blockAbove+2] & mask;
                   a3 = input[blockAbove+3] & mask;
                }
                else
                {
                   a0 = DEFAULT_VALUE;
                   a1 = DEFAULT_VALUE;
                   a2 = DEFAULT_VALUE;
                   a3 = DEFAULT_VALUE;
                }

                // MEDIAN: (xi,yi)-MEDIAN(ai, ci, (ai+ci)-e)
                final int val = c - d_r;
                int med0 = val + a0;

                if (a0 < c)
                   med0 = (med0 < a0) ? a0 : ((med0 < c) ? med0 : c);
                else
                   med0 = (med0 < c) ? c : ((med0 < a0) ? med0 : a0);

                int med1 = val + a1;

                if (a1 < c)
                   med1 = (med1 < a1) ? a1 : ((med1 < c) ? med1 : c);
                else
                   med1 = (med1 < c) ? c : ((med1 < a1) ? med1 : a1);

                int med2 = val + a2;

                if (a2 < c)
                   med2 = (med2 < a2) ? a2 : ((med2 < c) ? med2 : c);
                else
                   med2 = (med2 < c) ? c : ((med2 < a2) ? med2 : a2);

                int med3 = val + a3;

                if (a3 < c)
                   med3 = (med3 < a3) ? a3 : ((med3 < c) ? med3 : c);
                else
                   med3 = (med3 < c) ? c : ((med3 < a3) ? med3 : a3);
                
                output[i]   = residue[k]   + med0;
                output[i+1] = residue[k+1] + med1;
                output[i+2] = residue[k+2] + med2;
                output[i+3] = residue[k+3] + med3;
             }

             line += blockDim;
          }
       }

       return output;
    }


    private int[] computeBlockDiagonal(Prediction prediction, int[] output, int x, int y,
         int direction)
    {
       final int blockDim = prediction.blockDim;
       final int st = this.stride;
       final int start = (y * st) + x;
       final int endj = start + (st * blockDim);
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int mask = (this.isRGB == true) ? 0xFF : -1;
       final int xMax = this.width - blockDim;
       int k = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          for (int j=0, offs=start; offs<endj; offs+=st, j++)
          {
             final int endi = offs + blockDim;

             for (int i=offs; i<endi; i+=4)
             {
                // DIAGONAL_L
                final int ii = i - offs;
                final int iV = ii - j - 1 + start - st;
                final int iH = start + (j-ii-1)*st - 1;
                final int val0 = (ii   >= j) ? ((y > 0) ? input[iV]   & mask : DEFAULT_VALUE)
                        : ((x > 0) ? input[iH]      & mask : DEFAULT_VALUE);
                final int val1 = (ii+1 >= j) ? ((y > 0) ? input[iV+1] & mask : DEFAULT_VALUE)
                        : ((x > 0) ? input[iH-st]   & mask : DEFAULT_VALUE);
                final int val2 = (ii+2 >= j) ? ((y > 0) ? input[iV+2] & mask : DEFAULT_VALUE)
                        : ((x > 0) ? input[iH-st*2] & mask : DEFAULT_VALUE);
                final int val3 = (ii+3 >= j) ? ((y > 0) ? input[iV+3] & mask : DEFAULT_VALUE)
                        : ((x > 0) ? input[iH-st*3] & mask : DEFAULT_VALUE);
                output[i]   = residue[k]   + val0;
                output[i+1] = residue[k+1] + val1;
                output[i+2] = residue[k+2] + val2;
                output[i+3] = residue[k+3] + val3;
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          int line = 0;

          for (int j=0, offs=start; offs<endj; offs+=st, j++)
          {
             final int endi = offs + blockDim;

             for (int i=endi-4; i>=offs ; i-=4)
             {
                // DIAGONAL_R
                final int ii = i - offs;
                k = line + ii;
                final int diag = endi - 1 - offs;
                final int iV = ii + j + 1 + start - st;
                final int iH = start + (ii+j-diag-1)*st + diag + 1;
                final int val0 = (ii+j   <= diag) ? ((y > 0) ? input[iV]   & mask : DEFAULT_VALUE)
                        : ((x < xMax) ? input[iH]      & mask : DEFAULT_VALUE);
                final int val1 = (ii+1+j <= diag) ? ((y > 0) ? input[iV+1] & mask : DEFAULT_VALUE)
                        : ((x < xMax) ? input[iH+st]   & mask : DEFAULT_VALUE);
                final int val2 = (ii+2+j <= diag) ? ((y > 0) ? input[iV+2] & mask : DEFAULT_VALUE)
                        : ((x < xMax) ? input[iH+st*2] & mask : DEFAULT_VALUE);
                final int val3 = (ii+3+j <= diag) ? ((y > 0) ? input[iV+3] & mask : DEFAULT_VALUE)
                        : ((x < xMax) ? input[iH+st*3] & mask : DEFAULT_VALUE);
                output[i]   = residue[k]   + val0;
                output[i+1] = residue[k+1] + val1;
                output[i+2] = residue[k+2] + val2;
                output[i+3] = residue[k+3] + val3;
             }

             line += blockDim;
          }
       }

       return output;
    }


    private int[] computeBlockDC(Prediction prediction, int[] output, int x, int y,
         int direction)
    {
       final int blockDim = prediction.blockDim;
       final int st = this.stride;
       final int start = (y * st) + x;
       final int endj = start + (st * blockDim);
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int mask = (this.isRGB == true) ? 0xFF : -1;
       int k = 0;
       final int dc;
       final int above = start - st;

       if ((direction & DIR_LEFT) != 0)
       {
          // dc=ai+bi
          int dc_l = 0;
          int sum = 0;

          if (y > 0)
          {
             for (int i=0; i<blockDim; i++)
                dc_l += (input[above+i] & mask);

             sum += blockDim;
          }

          if (x > 0)
          {
            for (int j=start; j<endj; j+=st)
               dc_l += (input[j-1] & mask);

             sum += blockDim;
          }

          dc = (sum == 0) ? DEFAULT_VALUE : (dc_l + (sum >> 1)) / sum;
       }
       else
       {
          // dc=ai+ci
          int dc_r = 0;
          int sum = 0;

          if (y > 0)
          {
            for (int i=0; i<blockDim; i++)
               dc_r += (input[above+i] & mask);

            sum += blockDim;
          }

          if (x < this.width - blockDim)
          {
            for (int j=start; j<endj; j+=st)
               dc_r += (input[j+blockDim] & mask);

            sum += blockDim;
          }

          dc = (sum == 0) ? DEFAULT_VALUE : (dc_r + (sum >> 1)) / sum;
       }

       for (int j=start; j<endj; j+=st)
       {
          final int endi = j + blockDim;

          for (int i=j; i<endi; i+=4)
          {
             // DC_L: xi+dc_l
             // DC_R: xi+dc_r
             output[i]   = residue[k]   + dc;
             output[i+1] = residue[k+1] + dc;
             output[i+2] = residue[k+2] + dc;
             output[i+3] = residue[k+3] + dc;
             k += 4;
          }
       }

       return output;
   }


   // Search for a similar block that can be used as reference
   // Base prediction on difference with nearby blocks using 'winner update' strategy
   // Return error and update prediction argument
   private int computeReferenceSearch(int[] input, int x, int y,
           int maxSAD, Prediction prediction, int direction)
   {
      final int blockDim = prediction.blockDim;

      // Populate the set of neighboring candidate blocks
      this.getReferenceSearchBlocks(x, y, blockDim, 0, 0, direction,
              prediction.frame, ACTION_POPULATE, 0);

      SearchBlockContext ctx = null;
      final int mask = (this.isRGB == true) ? 0xFF : -1;
      final int st = this.stride;

      // Critical speed path
      while (this.set.size() > 0)
      {
         // Select partial winner (lowest error) to update
         ctx = this.set.pollFirst();

         // Full winner found ?
         if (ctx.line >= blockDim)
         {
            this.set.clear();
            break;
         }

         // Aliasing
         final int[] data = ctx.data;
         final int start = (y+ctx.line)*st + x;
         final int end = start + blockDim;
         int offs2 = (ctx.y+ctx.line)*st + ctx.x;
         int sad = ctx.sad;

         // Compute line difference
         for (int i=start; i<end; i+=4)
         {
             final int val0 = (input[i]   & mask) - (data[offs2]   & mask);
             final int val1 = (input[i+1] & mask) - (data[offs2+1] & mask);
             final int val2 = (input[i+2] & mask) - (data[offs2+2] & mask);
             final int val3 = (input[i+3] & mask) - (data[offs2+3] & mask);
             sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
             sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
             sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
             sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
             offs2 += 4;
         }

         ctx.sad = sad;
         ctx.line++;

         // Put back current block context into sorted set (likely new position)
         if (sad < maxSAD)
            this.set.add(ctx);
      }

      if (ctx == null)
         prediction.sad = MAX_ERROR;
      else
      {
         // Return best result
         prediction.x = ctx.x;
         prediction.y = ctx.y;
         prediction.sad = ctx.sad;
      }

      return prediction.sad;
   }


   public static int getReferenceCandidates(int refSearchStepRatio, int blockDim)
   {
      // for dim!=4, LEFT or RIGHT steps=>candidates: dim/1=>11, dim/2=>34, dim/4=>116, dim/8=>424
      // for dim=4,  LEFT or RIGHT steps=>candidates: dim/1=>11, dim/2=>34, dim/4=>116, dim/8=>116
      if (refSearchStepRatio == 8) // step=dim*8/8
         return 11;
      else if (refSearchStepRatio == 4) // step=dim*4/8
         return 34;
      else if ((refSearchStepRatio == 2) || (blockDim == 4)) // step=dim*2/8
         return 116;
      else if (refSearchStepRatio == 1) // step=dim*1/8
         return 424;

      return -1;
   }


   // (x,y) coordinates of current block
   // (xr, yr) coordinates of reference block
   public int getReferenceIndexFromPosition(int x, int y, int blockDim,
           int xr, int yr, int direction)
   {
      return this.getReferenceSearchBlocks(x, y, blockDim, xr, yr, direction,
              null, ACTION_GET_INDEX, 0);
   }


   // return (x << 16 | y)
   public int getReferencePositionFromIndex(int x, int y, int blockDim,
           int direction, int index)
   {
      return this.getReferenceSearchBlocks(x, y, blockDim, 0, 0, direction,
              null, ACTION_GET_COORD, index);
   }


   // if action == ACTION_POPULATE, populate the set of search blocks and return
   // the size of the set
   // if action == ACTION_GET_INDEX, return the index of the block based on the
   // provided coordinates
   // if action == ACTION_GET_COORD, return the coordinates of the block based on
   // the provided index
   private int getReferenceSearchBlocks(int x, int y, int blockDim, int xr, int yr,
           int direction, int[] referenceFrame, int action, int refIndex)
   {
      int step = (blockDim * this.refSearchStepRatio) >> 3;

      // Case where blockDim == 4 and refSearchStepRatio == 1/8
      if (step == 0)
         step = 1;

      // Populate set of block candidates
      // Add blocks to compare against (blocks must already have been encoded/decoded
      // to avoid circular dependencies). Check against upper neighbors (current block
      // is XX):
      //    LEFT+RIGHT        LEFT              RIGHT
      //    01 02 03 04 05    01 02 03 04 05    01 02 03 04 05
      //    06 07 08 09 10    06 07 08 09 10    06 07 08 09 10
      //       11 XX 12          11 XX                XX 11
      final int jstart = y - (blockDim << 1);
      int val = -1;

      for (int j=jstart; j<=y; j+=step)
      {
         if (j < 0)
            continue;

         final int istart = (j < y) ? x - (blockDim << 1) :
            (((direction & DIR_LEFT) != 0) ? x - blockDim : x + blockDim);
         final int iend = (j < y) ? x + (blockDim << 1) :
            (((direction & DIR_RIGHT) != 0) ? x + blockDim : x - blockDim);

         for (int i=istart; i<=iend; i+=step)
         {
            if ((i < 0) || (i + blockDim >= this.width))
               continue;

            // Block candidates are not allowed to intersect with current block
            if ((j + blockDim > y) && (i + blockDim > x) && (i < x + blockDim))
               continue;

            if (action == ACTION_POPULATE)
            {
               // Add to set sorted by residual error and coordinates
               this.set.add(SearchBlockContext.getContext(referenceFrame, i, j));
            }
            else if (action == ACTION_GET_INDEX)
            {
               val++;

               if ((i == xr) && (j == yr))
                  return val;
            }
            else if (action == ACTION_GET_COORD)
            {
               val++;

               if (refIndex == val)
                  return (i<<16) | j;
            }
         }
      }

      if (action == ACTION_POPULATE)
         val = this.set.size();

      return val;
   }



   private static class SearchBlockContext implements Comparable<SearchBlockContext>
   {
      int line;      // line to be processed
      int sad;       // sum of absolute differences so far
      int[] data;    // frame data
      int x;
      int y;

      private static final SearchBlockContext[] CACHE = init();
      private static int INDEX = 0;

      private static SearchBlockContext[] init()
      {
         SearchBlockContext[] res = new SearchBlockContext[425]; // max block candidates per call

         for (int i=0; i<res.length; i++)
            res[i] = new SearchBlockContext();

         return res;
      }


      public static SearchBlockContext getContext(int[] data, int x, int y)
      {
         SearchBlockContext res = CACHE[INDEX];

         if (++INDEX == CACHE.length)
            INDEX = 0;

         res.sad = 0;
         res.data = data;
         res.line = 0;
         res.x = x;
         res.y = y;
         return res;
      }


      @Override
      public int compareTo(SearchBlockContext c)
      {
         if (c == null)
            return 1;

         if (this.sad != c.sad)
            return this.sad - c.sad;

         if (this.y != c.y)
            return this.y - c.y;

         return this.x - c.x;
      }


      @Override
      public boolean equals(Object o)
      {
         try
         {
            if (o == this)
               return true;

            if (o == null)
               return false;

            SearchBlockContext c = (SearchBlockContext) o;

            if (this.sad != c.sad)
               return false;

            if (this.y != c.y)
               return false;

            return this.x == c.x;
         }
         catch (ClassCastException e)
         {
            return false;
         }
      }
      
      
      @Override
      public int hashCode()
      {
         return (this.y << 16) | (this.x & 0xFFFF);
      }
   }
}
