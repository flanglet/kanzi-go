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

package kanzi.prediction;

import java.util.TreeSet;
import kanzi.Global;


// Class used to predict a block based on its neighbors in the current frame or 
// in a reference frame
public class LossyIntraPredictor
{
   // 4096 / n
   private static final int[] IDIV  = initInverse();
   
   private static int[] initInverse()
   {
      final int[] res = new int[32*32+1];
      
      for (int i=1; i<res.length; i++)
         res[i] = (1<<16) / i;
         
      return res;
   }
   
   public enum Mode
   {
      DC,              // DC
      HORIZONTAL,      // Horizontal
      DIRECTIONAL_30,  // Directional 30 degrees
      DIRECTIONAL_45,  // Diagonal
      DIRECTIONAL_60,  // Directional 60 degrees
      VERTICAL,        // Vertical
      MEDIAN,          // Median horizontal-vertical-dc
      BILINEAR_HV,     // Bilinear interpolation row/column
      REFERENCE_INTER, // Other block used as reference in another frame
      REFERENCE_INTRA; // Other block used as reference in current frame

      // 'direct' encoding mode can be encoded as reference prediction mode
      // with same frame (or empty frame?) and deltaX=deltaY=0
      
      public static Mode getMode(int val)
      {
         if (val == BILINEAR_HV.ordinal())
            return BILINEAR_HV;

         if (val == MEDIAN.ordinal())
            return MEDIAN;

         if (val == DC.ordinal())
            return DC;

         if (val == HORIZONTAL.ordinal())
            return HORIZONTAL;

         if (val == VERTICAL.ordinal())
            return VERTICAL;

         if (val == DIRECTIONAL_30.ordinal())
            return DIRECTIONAL_30;

         if (val == DIRECTIONAL_45.ordinal())
            return DIRECTIONAL_45;

         if (val == DIRECTIONAL_60.ordinal())
            return DIRECTIONAL_60;

         if (val == REFERENCE_INTER.ordinal())
            return REFERENCE_INTER;

         if (val == REFERENCE_INTRA.ordinal())
            return REFERENCE_INTRA;

         return null;
      }
   }

   private static final int ACTION_POPULATE  = 1;
   private static final int ACTION_GET_INDEX = 2;
   private static final int ACTION_GET_COORD = 3;   
   private static final int ANGLE_30 = 30;    // Approximate (deltaX=2, deltaY=1)
   private static final int ANGLE_45 = 45;    // Exact (deltaX=1, deltaY=1)
   private static final int ANGLE_60 = 60;    // Approximate (deltaX=1, deltaY=2)

   public static final int DIR_LEFT  = 1;
   public static final int DIR_RIGHT = 2;
   public static final int REFERENCE = 4;

   public static final int MAX_ERROR = 1 << 26; // Not Integer.Max to avoid add overflow

   private final int width;
   private final int height;
   private final int stride;
   private final TreeSet<SearchBlockContext> searchSet; // used during reference search
   private final int thresholdSAD; // used to trigger reference search
   private final int refSearchStepRatio;
   private final boolean isRGB;
   private final int maxBlockDim;
   private final Prediction refPrediction;
   private final int mask;
   private final int defaultPixVal;


   public LossyIntraPredictor(int width, int height, int maxBlockDim)
   {
      this(width, height, width, maxBlockDim, true);
   }


   public LossyIntraPredictor(int width, int height, int maxBlockDim, int stride, boolean isRGB)
   {
      this(width, height, maxBlockDim, stride, isRGB, 5);
   }


   public LossyIntraPredictor(int width, int height, int maxBlockDim, int stride,
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
   public LossyIntraPredictor(int width, int height, int maxBlockDim, int stride,
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
     this.searchSet = new TreeSet<SearchBlockContext>();
     this.thresholdSAD = errThreshold;
     this.maxBlockDim = maxBlockDim;
     this.isRGB = isRGB;
     this.refPrediction = new Prediction(maxBlockDim);
     this.refSearchStepRatio = refSearchStepRatio;
     this.mask = (this.isRGB == true) ? 0xFF : -1;
     this.defaultPixVal = 128;
   }


   public int getWidth()
   {
      return this.width;
   }


   public int getHeight()
   {
      return this.height;
   }


   public int getStride()
   {
      return this.stride;
   }

   
   // Compute block prediction (from other blocks) using several different methods (modes)
   // Another block (spatial or temporal) can be provided optionally
   // The input arrays must be frame channels (R,G,B or Y,U,V)
   // input is a block in a frame at offset iy*stride+ix
   // output is the difference block (a.k.a residual block)
   // return index of best prediction
   public int computeResidues(int[] input, int ix, int iy,
           int[] other, int ox, int oy,
           Prediction[] predictions, int blockDim, int predictionType,
           boolean exhaustive)
   {
      if ((ix < 0) || (ix >= this.width) || (iy < 0) || (iy >= this.height))
         return -1;

      // The block dimension must be a multiple of 4
      if ((blockDim & 3) != 0)
         return -1;

      // Check block dimension
      if (blockDim > this.maxBlockDim)
         return -1;

      // Check coordinates
      if ((blockDim+ix > this.width) || (blockDim+iy > this.height))
         return -1;

      // Prediction type must be set
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

      // Look for matching block at same position in reference frame (if any)
      if ((other != null) && ((predictionType & REFERENCE) != 0))
      {
         Prediction p = predictions[Mode.REFERENCE_INTER.ordinal()];
         p.frame = other;
         p.x = ox;
         p.y = oy;
         this.computePredictionReference(predictions[Mode.REFERENCE_INTER.ordinal()], 
            input, iy*this.stride+ix, other, oy*this.stride+ox,blockDim);

         // Set best prediction index
         minIdx = Mode.REFERENCE_INTER.ordinal();
      }
      
      // Found perfect match ?
      if ((predictions[minIdx].sad == 0) && (exhaustive == false))
        return Mode.REFERENCE_INTER.ordinal();
      
      if (((predictionType & DIR_RIGHT) != 0) || (predictionType & DIR_LEFT) != 0)
      {
         // Compute block residues based on prediction modes
         // EG. block 8x8: xi
         //   d   a0 a1 a2 a3 a4 a5 a6 a7   e
         //  b0   x0 x1 x2 x3 x4 x5 x6 x7   c0
         //  b1   x0 x1 x2 x3 x4 x5 x6 x7   c1
         //  b2   x0 x1 x2 x3 x4 x5 x6 x7   c2
         //  b3   x0 x1 x2 x3 x4 x5 x6 x7   c3
         //  b4   x0 x1 x2 x3 x4 x5 x6 x7   c4
         //  b5   x0 x1 x2 x3 x4 x5 x6 x7   c5
         //  b6   x0 x1 x2 x3 x4 x5 x6 x7   c6
         //  b7   x0 x1 x2 x3 x4 x5 x6 x7   c7    
         int sad;
         sad = this.computePredictionHorizontal(predictions[Mode.HORIZONTAL.ordinal()], input, ox, oy, predictionType);
         
         if ((sad == 0) && (exhaustive == false))
            return Mode.HORIZONTAL.ordinal();

         sad = this.computePredictionVertical(predictions[Mode.VERTICAL.ordinal()], input, ox, oy, predictionType);
                  
         if ((sad == 0) && (exhaustive == false))
            return Mode.VERTICAL.ordinal();
         
         sad = this.computePredictionDC(predictions[Mode.DC.ordinal()], input, ox, oy, predictionType);
         
         if ((sad == 0) && (exhaustive == false))
            return Mode.DC.ordinal();
                  
         sad = this.computePredictionMedian(predictions[Mode.MEDIAN.ordinal()], input, ox, oy, predictionType);
         
         if ((sad == 0) && (exhaustive == false))
            return Mode.MEDIAN.ordinal();
                  
         sad = this.computePredictionBilinearHV(predictions[Mode.BILINEAR_HV.ordinal()], input, ox, oy, predictionType);
         
         if ((sad == 0) && (exhaustive == false))
            return Mode.BILINEAR_HV.ordinal();

         sad = this.computePredictionDirectional(predictions[Mode.DIRECTIONAL_30.ordinal()], input, ox, oy, ANGLE_30, predictionType);
         
         if ((sad == 0) && (exhaustive == false))
            return Mode.DIRECTIONAL_30.ordinal();
         
         sad = this.computePredictionDirectional(predictions[Mode.DIRECTIONAL_45.ordinal()], input, ox, oy, ANGLE_45, predictionType);
         
         if ((sad == 0) && (exhaustive == false))
            return Mode.DIRECTIONAL_45.ordinal();
         
         sad = this.computePredictionDirectional(predictions[Mode.DIRECTIONAL_60.ordinal()], input, ox, oy, ANGLE_60, predictionType);
         
         if ((sad == 0) && (exhaustive == false))
            return Mode.DIRECTIONAL_60.ordinal();         
      }
      
      // Find best prediction
      for (int i=0; i<predictions.length; i++)
      {
         if (predictions[i].sad < predictions[minIdx].sad)
            minIdx = i;
      }

      // If the error of the best prediction is not low 'enough' and the
      // spatial reference is set, start a spatial search
      if (((predictionType & REFERENCE) != 0) && (predictions[minIdx].sad >= blockDim*blockDim*this.thresholdSAD))
      {
         // Spatial search of best matching nearby block
         final Prediction newPrediction = this.refPrediction;
         newPrediction.frame = input;
         newPrediction.blockDim = blockDim;

         // Do the search and update prediction error, coordinates and result block
         this.computeReferenceSearch(input, ix, iy, predictions[minIdx].sad, newPrediction, predictionType);
         Prediction refPred = predictions[Mode.REFERENCE_INTRA.ordinal()];

         // Is the new prediction an improvement ?
         if (newPrediction.sad < refPred.sad)
         {
            refPred.x = newPrediction.x;
            refPred.y = newPrediction.y;
            refPred.sad = newPrediction.sad;

            // Create residue block for reference mode
            this.computePredictionReference(newPrediction, input, iy*this.stride+ix, input,
                    newPrediction.y*this.stride+newPrediction.x, blockDim);

            System.arraycopy(newPrediction.residue, 0, refPred.residue, 0, newPrediction.residue.length);

            if (refPred.sad < predictions[minIdx].sad)
               minIdx = Mode.REFERENCE_INTRA.ordinal();
         }
      }

      return minIdx;
   }


   // Compute residue against another (spatial/temporal) block
   // Return error of difference block
   private int computePredictionReference(Prediction prediction, int[] input, int iIdx, 
      final int[] other, int oIdx, int blockDim)
   {
      final int st = this.stride;
      final int mask_ = this.mask;
      final int endj = iIdx + (st*blockDim);
      final int[] output = prediction.residue;
      int k = 0;
      int sad = 0;
      int ref0 = 0;
      int ref1 = 0;
      int ref2 = 0;
      int ref3 = 0;

      for (int j=iIdx; j<endj; j+=st)
      {
         final int endi = j + blockDim;

         for (int i=j; i<endi; i+=4)
         {
             if (other != null)
             {
                ref0 = other[oIdx]   & mask_;
                ref1 = other[oIdx+1] & mask_;
                ref2 = other[oIdx+2] & mask_;
                ref3 = other[oIdx+3] & mask_;
             }
             
             final int val0 = (input[i]   & mask_) - ref0;
             final int val1 = (input[i+1] & mask_) - ref1;
             final int val2 = (input[i+2] & mask_) - ref2;
             final int val3 = (input[i+3] & mask_) - ref3;
             sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
             sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
             sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
             sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
             output[k]   = val0;
             output[k+1] = val1;
             output[k+2] = val2;
             output[k+3] = val3;
             k += 4;
             oIdx += 4;
         }

         oIdx += (st - blockDim);
      }

      prediction.sad = sad;
      return prediction.sad;
   }


   // Add residue to other block located at prediction.x, prediction.y
   // Return output
   private int[] computeBlockReference(Prediction prediction, int[] output, int x, int y,
         int direction)
   {
      final int st = this.stride;
      final int blockDim = prediction.blockDim;
      final int start = (y*st) + x;
      final int endj = start + (st*blockDim);
      final int[] residue = prediction.residue;
      final int[] input = prediction.frame;
      int ref = (prediction.y*st) + prediction.x;
      int k = 0;

      if (input == null)
      {
          // Simple copy
          for (int j=start; j<endj; j+=st)
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
         final int mask_ = this.mask;

         for (int j=start; j<endj; j+=st)
         {
            final int endi = j + blockDim;

            for (int i=j; i<endi; i+=4)
            {
                output[i]   = residue[k]   + (input[ref]   & mask_);
                output[i+1] = residue[k+1] + (input[ref+1] & mask_);
                output[i+2] = residue[k+2] + (input[ref+2] & mask_);
                output[i+3] = residue[k+3] + (input[ref+3] & mask_);
                ref += 4;
                k += 4;
            }

            ref += (st - blockDim);
         }
      }

      return output;
   }


   // Create block in output at x,y from prediction mode, residue and input.
   // residue is a blockDim*blockDim size block
   // output is a width*height size frame
   public int[] computeBlock(Prediction prediction, int[] output, final int x, final int y,
           Mode mode, int direction)
   {
      if ((mode == Mode.REFERENCE_INTER) || (mode == Mode.REFERENCE_INTRA))
         return this.computeBlockReference(prediction, output, x, y, direction);

      if (mode == Mode.BILINEAR_HV)
         return this.computeBlockBilinearHV(prediction, output, x, y, direction);

      if (mode == Mode.VERTICAL)
         return this.computeBlockVertical(prediction, output, x, y, direction);

      if (mode == Mode.MEDIAN)
         return this.computeBlockMedian(prediction, output, x, y, direction);

      if (mode == Mode.HORIZONTAL)
         return this.computeBlockHorizontal(prediction, output, x, y, direction);

      if (mode == Mode.DC)
         return this.computeBlockDC(prediction, output, x, y, direction);

      if (mode == Mode.DIRECTIONAL_30)
         return this.computeBlockDirectional(prediction, output, x, y, ANGLE_30, direction);

      if (mode == Mode.DIRECTIONAL_45)
         return this.computeBlockDirectional(prediction, output, x, y, ANGLE_45, direction);

      if (mode == Mode.DIRECTIONAL_60)
         return this.computeBlockDirectional(prediction, output, x, y, ANGLE_60, direction);

      return output;
   }


    private int[] computeBlockHorizontal(Prediction prediction, int[] output, int x, int y,
         int direction)
    {
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int endj = start + (st*blockDim);
       int k = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;
             final int b = (x > 0) ? input[j-1] & mask_ : this.defaultPixVal;

             for (int i=j; i<endi; i+=4)
             {
                // HORIZONTAL_L: xi+bi
                output[i]   = Global.clip0_255(residue[k]   + b);
                output[i+1] = Global.clip0_255(residue[k+1] + b);
                output[i+2] = Global.clip0_255(residue[k+2] + b);
                output[i+3] = Global.clip0_255(residue[k+3] + b);
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          for (int j=start; j<endj; j+=st)
          {
             final int xMax = this.width - blockDim;             
             final int endi = j + blockDim;
             final int c = (x < xMax) ? input[endi] & mask_ : this.defaultPixVal;

             for (int i=j; i<endi; i+=4)
             {
                // HORIZONTAL_R: xi+ci
                output[i]   = Global.clip0_255(residue[k]   + c);
                output[i+1] = Global.clip0_255(residue[k+1] + c);
                output[i+2] = Global.clip0_255(residue[k+2] + c);
                output[i+3] = Global.clip0_255(residue[k+3] + c);
                k += 4;
             }
          }
       }

       return output;
    }


    private int[] computeBlockVertical(Prediction prediction, int[] output, 
       final int x, final int y, int direction)
    {
       final int blockDim = prediction.blockDim;
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int endj = start + (st*blockDim);
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       int k = 0;
       int a0 = this.defaultPixVal;
       int a1 = this.defaultPixVal;
       int a2 = this.defaultPixVal;
       int a3 = this.defaultPixVal;

       for (int j=start; j<endj; j+=st)
       {
         final int endi = j + blockDim;

          for (int i=j; i<endi; i+=4)
          {
             // VERTICAL: xi+ai
             if (y > 0)
             {
                final int blockAbove = i - j + start - st;
                a0 = input[blockAbove]   & mask_;
                a1 = input[blockAbove+1] & mask_;
                a2 = input[blockAbove+2] & mask_;
                a3 = input[blockAbove+3] & mask_;
             }

             output[i]   = Global.clip0_255(residue[k]   + a0);
             output[i+1] = Global.clip0_255(residue[k+1] + a1);
             output[i+2] = Global.clip0_255(residue[k+2] + a2);
             output[i+3] = Global.clip0_255(residue[k+3] + a3);
             k += 4;
          }
       }

       return output;
    }


    private int[] computeBlockBilinearHV(Prediction prediction, int[] output, 
       final int x, final int y, int direction)
    {
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int endj = start + (st*blockDim);
       int k = 0;
       int a0 = this.defaultPixVal;
       int a1 = this.defaultPixVal;
       int a2 = this.defaultPixVal;
       int a3 = this.defaultPixVal;
       
       if ((direction & DIR_LEFT) != 0)
       {
          for (int j=start, jj=1; j<endj; j+=st, jj++)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i+=4)
             {
                final int b = (x > 0) ? input[j-1] & mask_ : this.defaultPixVal;
                final int ii = i - j + 1;
                final int jjb = jj * b;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask_;
                   a1 = input[blockAbove+1] & mask_;
                   a2 = input[blockAbove+2] & mask_;
                   a3 = input[blockAbove+3] & mask_;
                }

                // BILINEAR_HV_L: (xi,yi)+(dist(xi,ai)*bi+dist(xi,bi)*ai))/(dist(xi,a1)+dist(xi,bi))
                output[i]   = Global.clip0_255(residue[k]   + ((((jjb +     ii*a0) * IDIV[ii+jj])   + 32768) >> 16));
                output[i+1] = Global.clip0_255(residue[k+1] + ((((jjb + (ii+1)*a1) * IDIV[ii+jj+1]) + 32768) >> 16));
                output[i+2] = Global.clip0_255(residue[k+2] + ((((jjb + (ii+2)*a2) * IDIV[ii+jj+2]) + 32768) >> 16));
                output[i+3] = Global.clip0_255(residue[k+3] + ((((jjb + (ii+3)*a3) * IDIV[ii+jj+3]) + 32768) >> 16));
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          int line = 0;
          final int xMax = this.width - blockDim;

          for (int j=start, jj=1; j<endj; j+=st, jj++)
          {
             final int endi = j + blockDim;

             for (int i=endi-4; i>=j ; i-=4)
             {
                final int c = (x < xMax) ? input[endi] & mask_ : this.defaultPixVal;
                k = line + i - j;
                final int ii = i - j + 1;
                final int jjc = jj * c;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask_;
                   a1 = input[blockAbove+1] & mask_;
                   a2 = input[blockAbove+2] & mask_;
                   a3 = input[blockAbove+3] & mask_;
                }

                // BILINEAR_HV_R: (xi,yi)+(dist(xi,a1)*ci+dist(xi,ci)*ai))/(dist(xi,ai)+dist(xi,ci))
                output[i]   = Global.clip0_255(residue[k]   + ((((jjc +     ii*a0) * IDIV[ii+jj])   + 32768) >> 16));
                output[i+1] = Global.clip0_255(residue[k+1] + ((((jjc + (ii+1)*a1) * IDIV[ii+jj+1]) + 32768) >> 16));
                output[i+2] = Global.clip0_255(residue[k+2] + ((((jjc + (ii+2)*a2) * IDIV[ii+jj+2]) + 32768) >> 16));
                output[i+3] = Global.clip0_255(residue[k+3] + ((((jjc + (ii+3)*a3) * IDIV[ii+jj+3]) + 32768) >> 16));
             }

             line += blockDim;
          }            
       }
     
       return output;
    }

                   
    private int[] computeBlockMedian(Prediction prediction, int[] output, 
       final int x, final int y, int direction)
    {
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int endj = start + (st*blockDim);
       int k = 0;
       int a0 = this.defaultPixVal;
       int a1 = this.defaultPixVal;
       int a2 = this.defaultPixVal;
       int a3 = this.defaultPixVal;
       
       if ((direction & DIR_LEFT) != 0)
       {
          final int d = ((y > 0) && (x > 0)) ? input[start-st-1] & mask_ : this.defaultPixVal;
          
          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i+=4)
             {
                final int b = (x > 0) ? input[j-1] & mask_ : this.defaultPixVal;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask_;
                   a1 = input[blockAbove+1] & mask_;
                   a2 = input[blockAbove+2] & mask_;
                   a3 = input[blockAbove+3] & mask_;
                }

                // MEDIAN: (xi,yi)+MEDIAN(ai, bi, (ai+bi)-d)
                output[i]   = Global.clip0_255(residue[k]   + median(a0, b, a0+b-d));
                output[i+1] = Global.clip0_255(residue[k+1] + median(a1, b, a1+b-d));
                output[i+2] = Global.clip0_255(residue[k+2] + median(a2, b, a2+b-d));
                output[i+3] = Global.clip0_255(residue[k+3] + median(a3, b, a3+b-d));
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          final int xMax = this.width - blockDim;          
          final int e = ((y > 0) && (x < xMax)) ? input[start-st+blockDim] & mask_ : this.defaultPixVal;
          int line = 0;

          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=endi-4; i>=j ; i-=4)
             {
                final int c = (x < xMax) ? input[endi] & mask_ : this.defaultPixVal;
                k = line + i - j;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask_;
                   a1 = input[blockAbove+1] & mask_;
                   a2 = input[blockAbove+2] & mask_;
                   a3 = input[blockAbove+3] & mask_;
                }

                // MEDIAN: (xi,yi)+MEDIAN(ai, ci, (ai+ci)-e)  
                output[i]   = Global.clip0_255(residue[k]   + median(a0, c, a0+c-e));
                output[i+1] = Global.clip0_255(residue[k+1] + median(a1, c, a1+c-e));
                output[i+2] = Global.clip0_255(residue[k+2] + median(a2, c, a2+c-e));
                output[i+3] = Global.clip0_255(residue[k+3] + median(a3, c, a3+c-e));
             }

             line += blockDim;
          }
       }

       return output;
    }


    private static int median(int x, int y, int z)
    {     
       if (y < z)
          return (x < y) ? y : ((x < z) ? x : z);
       else
          return (x < z) ? z : ((x < y) ? x : y);       
    }
    
    
    private int[] computeBlockDirectional(Prediction prediction, int[] output, 
       final int x, final int y, int angle, int direction)
    {       
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int endj = start + (st*blockDim);
       final int shiftX = (angle == ANGLE_60) ? 1 : 0;
       final int shiftY = (angle == ANGLE_30) ? 1 : 0;
       int k = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          // DIRECTIONAL_L          
          for (int j=start, jj=1; j<endj; j+=st, jj++)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i++)
             {
                int offset = -1;
                final int ii = i - j + 1;
                
                if ((ii<<shiftX) >= (jj<<shiftY))
                {
                  // Above 'diagonal' (including it) : reference is ai (or d)
                  if (y > 0)
                     offset = (jj - ((ii<<shiftX)>>shiftY)) + start - st - 1;
               }
               else
               {
                 // Below 'diagonal' : reference is bi
                 if (x > 0)
                    offset = (jj - ((ii<<shiftX)>>shiftY))*st + start - st - 1;              
               }  

               if (offset >= 0)
                  output[i] = Global.clip0_255(residue[k] + (input[offset] & mask_));
               else
                  output[i] = Global.clip0_255(residue[k] + this.defaultPixVal);

               k++;
            }         
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          // DIRECTIONAL_R
          final int xMax = this.width - blockDim;
          int line = 0;

          for (int j=start, jj=1; j<endj; j+=st, jj++)
          {
             final int endi = j + blockDim;

             for (int i=endi-1; i>=j; i--)
             {
                int offset = -1;
                final int ii = blockDim - i + j - 1;
                k = line + i - j;                
                
                if ((ii<<shiftX) >= (jj<<shiftY))
                {
                   // Above 'diagonal' (including it) : reference is ai
                   if (y > 0)
                      offset = (jj - ((ii<<shiftX)>>shiftY)) + start - st - 1;   
                }
                else
                {
                   // Below 'diagonal' : reference is ci
                   if (x < xMax)
                      offset = (jj - ((ii<<shiftX)>>shiftY))*st + start - st - 1;                                  
                }

                if (offset >= 0)
                   output[i] = Global.clip0_255(residue[k] + (input[offset] & mask_));
                else
                   output[i] = Global.clip0_255(residue[k] + this.defaultPixVal);
             } 
             
             line += blockDim; 
          }
       }

       return output;
    }


    private int[] computeBlockDC(Prediction prediction, int[] output, 
       final int x, final int y, int direction)
    {
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int[] input = prediction.frame;
       final int endj = start + (st*blockDim);
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
                dc_l += (input[above+i] & mask_);

             sum += blockDim;
          }

          if (x > 0)
          {
            for (int j=start; j<endj; j+=st)
               dc_l += (input[j-1] & mask_);

             sum += blockDim;
          }

          dc = (sum == 0) ? this.defaultPixVal : (dc_l + (sum >> 1)) / sum;
       }
       else
       {
          // dc=ai+ci
          int dc_r = 0;
          int sum = 0;

          if (y > 0)
          {
            for (int i=0; i<blockDim; i++)
               dc_r += (input[above+i] & mask_);

            sum += blockDim;
          }

          if (x < this.width - blockDim)
          {
            for (int j=start; j<endj; j+=st)
               dc_r += (input[j+blockDim] & mask_);

            sum += blockDim;
          }

          dc = (sum == 0) ? this.defaultPixVal : (dc_r + (sum >> 1)) / sum;
       }

       for (int j=start; j<endj; j+=st)
       {
          final int endi = j + blockDim;

          for (int i=j; i<endi; i+=4)
          {
             // DC_L: xi+dc_l
             // DC_R: xi+dc_r
             output[i]   = Global.clip0_255(residue[k]   + dc);
             output[i+1] = Global.clip0_255(residue[k+1] + dc);
             output[i+2] = Global.clip0_255(residue[k+2] + dc);
             output[i+3] = Global.clip0_255(residue[k+3] + dc);
             k += 4;
          }
       }

       return output;
   }

    private int computePredictionHorizontal(Prediction prediction, int[] input, int x, int y,
         int direction)
    {
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int endj = start + (st*blockDim);
       int k = 0;
       int sad = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;
             final int b = (x > 0) ? input[j-1] & mask_ : this.defaultPixVal;

             for (int i=j; i<endi; i+=4)
             {
                // HORIZONTAL_L: xi-bi
                final int val0 = (input[i]   & mask_) - b;
                final int val1 = (input[i+1] & mask_) - b;
                final int val2 = (input[i+2] & mask_) - b;
                final int val3 = (input[i+3] & mask_) - b;
                sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
                residue[k]   = val0;
                residue[k+1] = val1;
                residue[k+2] = val2;
                residue[k+3] = val3;
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          for (int j=start; j<endj; j+=st)
          {
             final int xMax = this.width - blockDim;             
             final int endi = j + blockDim;
             final int c = (x < xMax) ? input[endi] & mask_ : this.defaultPixVal;

             for (int i=j; i<endi; i+=4)
             {
                // HORIZONTAL_R: xi-ci
                final int val0 = (input[i]   & mask_) - c;
                final int val1 = (input[i+1] & mask_) - c;
                final int val2 = (input[i+2] & mask_) - c;
                final int val3 = (input[i+3] & mask_) - c;
                sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
                residue[k]   = val0;
                residue[k+1] = val1;
                residue[k+2] = val2;
                residue[k+3] = val3;
                k += 4;
             }         
          }
       }

       prediction.sad = sad;
       return prediction.sad;
    }


    private int computePredictionVertical(Prediction prediction, int[] input, 
       final int x, final int y, int direction)
    {
       final int blockDim = prediction.blockDim;
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int endj = start + (st*blockDim);
       final int[] residue = prediction.residue;
       int k = 0;
       int a0 = this.defaultPixVal;
       int a1 = this.defaultPixVal;
       int a2 = this.defaultPixVal;
       int a3 = this.defaultPixVal;
       int sad = 0;

       for (int j=start; j<endj; j+=st)
       {
          final int endi = j + blockDim;

          for (int i=j; i<endi; i+=4)
          {
             // VERTICAL: xi-ai
             if (y > 0)
             {
                final int blockAbove = i - j + start - st;
                a0 = input[blockAbove]   & mask_;
                a1 = input[blockAbove+1] & mask_;
                a2 = input[blockAbove+2] & mask_;
                a3 = input[blockAbove+3] & mask_;
             }

             final int val0 = (input[i]   & mask_) - a0;
             final int val1 = (input[i+1] & mask_) - a1;
             final int val2 = (input[i+2] & mask_) - a2;
             final int val3 = (input[i+3] & mask_) - a3;
             sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
             sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
             sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
             sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
             residue[k]   = val0;
             residue[k+1] = val1;
             residue[k+2] = val2;
             residue[k+3] = val3;
             k += 4;
          }
       }

       prediction.sad = sad;
       return prediction.sad;
    }


    private int computePredictionBilinearHV(Prediction prediction, int[] input, 
       final int x, final int y, int direction)
    {
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int endj = start + (st*blockDim);
       int k = 0;
       int a0 = this.defaultPixVal;
       int a1 = this.defaultPixVal;
       int a2 = this.defaultPixVal;
       int a3 = this.defaultPixVal;
       int sad = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          for (int j=start, jj=1; j<endj; j+=st, jj++)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i+=4)
             {
                final int b = (x > 0) ? input[j-1] & mask_ : this.defaultPixVal;
                final int ii = i - j + 1;
                final int jjb = jj * b;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask_;
                   a1 = input[blockAbove+1] & mask_;
                   a2 = input[blockAbove+2] & mask_;
                   a3 = input[blockAbove+3] & mask_;
                }

                // BILINEAR_HV_L: (xi,yi)-(dist(xi,ai)*bi+dist(xi,bi)*ai))/(dist(xi,a1)+dist(xi,bi))
                final int val0 = (input[i]   & mask_) - ((((jjb +     ii*a0) * IDIV[ii+jj])   + 32768) >> 16);
                final int val1 = (input[i+1] & mask_) - ((((jjb + (ii+1)*a1) * IDIV[ii+jj+1]) + 32768) >> 16);
                final int val2 = (input[i+2] & mask_) - ((((jjb + (ii+2)*a2) * IDIV[ii+jj+2]) + 32768) >> 16);
                final int val3 = (input[i+3] & mask_) - ((((jjb + (ii+3)*a3) * IDIV[ii+jj+3]) + 32768) >> 16);
                sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
                residue[k]   = val0;
                residue[k+1] = val1;
                residue[k+2] = val2;
                residue[k+3] = val3;
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          int line = 0;
          final int xMax = this.width - blockDim;

          for (int j=start, jj=1; j<endj; j+=st, jj++)
          {
             final int endi = j + blockDim;

             for (int i=endi-4; i>=j ; i-=4)
             {
                final int c = (x < xMax) ? input[endi] & mask_ : this.defaultPixVal;
                k = line + i - j;
                final int ii = i - j + 1;
                final int jjc = jj * c;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask_;
                   a1 = input[blockAbove+1] & mask_;
                   a2 = input[blockAbove+2] & mask_;
                   a3 = input[blockAbove+3] & mask_;
                }

                // BILINEAR_HV_R: (xi,yi)-(dist(xi,ai)*ci+dist(xi,ci)*ai))/(dist(xi,a1)+dist(xi,ci))
                final int val0 = (input[i]   & mask_) - ((((jjc +     ii*a0) * IDIV[ii+jj])   + 32768) >> 16);
                final int val1 = (input[i+1] & mask_) - ((((jjc + (ii+1)*a1) * IDIV[ii+jj+1]) + 32768) >> 16);
                final int val2 = (input[i+2] & mask_) - ((((jjc + (ii+2)*a2) * IDIV[ii+jj+2]) + 32768) >> 16);
                final int val3 = (input[i+3] & mask_) - ((((jjc + (ii+3)*a3) * IDIV[ii+jj+3]) + 32768) >> 16);
                sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
                residue[k]   = val0;
                residue[k+1] = val1;
                residue[k+2] = val2;
                residue[k+3] = val3;
             }

             line += blockDim;
          }            
       }

       prediction.sad = sad;
       return prediction.sad;     
    }

               
    private int computePredictionMedian(Prediction prediction, int[] input, 
       final int x, final int y, int direction)
    {
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int endj = start + (st*blockDim);
       int k = 0;
       int a0 = this.defaultPixVal;
       int a1 = this.defaultPixVal;
       int a2 = this.defaultPixVal;
       int a3 = this.defaultPixVal;
       int sad = 0;
       
       if ((direction & DIR_LEFT) != 0)
       {
          final int d = ((y > 0) && (x > 0)) ? input[start-st-1] & mask_ : this.defaultPixVal;
          
          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i+=4)
             {
                final int b = (x > 0) ? input[j-1] & mask_ : this.defaultPixVal;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask_;
                   a1 = input[blockAbove+1] & mask_;
                   a2 = input[blockAbove+2] & mask_;
                   a3 = input[blockAbove+3] & mask_;
                }

                // MEDIAN: (xi,yi)-MEDIAN(ai, bi, (ai+bi)-d)
                final int val0 = input[i]   - median(a0, b, a0+b-d);
                final int val1 = input[i+1] - median(a1, b, a1+b-d);
                final int val2 = input[i+2] - median(a2, b, a2+b-d);
                final int val3 = input[i+3] - median(a3, b, a3+b-d);
                sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
                residue[k]   = val0;
                residue[k+1] = val1;
                residue[k+2] = val2;
                residue[k+3] = val3;
                k += 4;
             }
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          final int xMax = this.width - blockDim;          
          final int e = ((y > 0) && (x < xMax)) ? input[start-st+blockDim] & mask_ : this.defaultPixVal;
          int line = 0;

          for (int j=start; j<endj; j+=st)
          {
             final int endi = j + blockDim;

             for (int i=endi-4; i>=j ; i-=4)
             {
                final int c = (x < xMax) ? input[endi] & mask_ : this.defaultPixVal;
                k = line + i - j;

                if (y > 0)
                {
                   final int blockAbove = start - st + i - j;
                   a0 = input[blockAbove]   & mask_;
                   a1 = input[blockAbove+1] & mask_;
                   a2 = input[blockAbove+2] & mask_;
                   a3 = input[blockAbove+3] & mask_;
                }

                // MEDIAN: (xi,yi)-MEDIAN(ai, ci, (ai+ci)-e)
                final int val0 = input[i]   - median(a0, c, a0+c-e);
                final int val1 = input[i+1] - median(a1, c, a1+c-e);
                final int val2 = input[i+2] - median(a2, c, a2+c-e);
                final int val3 = input[i+3] - median(a3, c, a3+c-e);                sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
                sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
                sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
                sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
                residue[k]   = val0;
                residue[k+1] = val1;
                residue[k+2] = val2;
                residue[k+3] = val3;
             }

             line += blockDim;
          }
       }

       prediction.sad = sad;
       return prediction.sad;
    }


    private int computePredictionDirectional(Prediction prediction, int[] input, 
       final int x, final int y, int angle, int direction)
    {       
       final int st = this.stride;
       final int start = (y*st) + x;
       final int mask_ = this.mask;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int endj = start + (st*blockDim);
       final int shiftX = (angle == ANGLE_60) ? 1 : 0;
       final int shiftY = (angle == ANGLE_30) ? 1 : 0;
       int k = 0;
       int val;
       int sad = 0;

       if ((direction & DIR_LEFT) != 0)
       {
          // DIRECTIONAL_L          
          for (int j=start, jj=1; j<endj; j+=st, jj++)
          {
             final int endi = j + blockDim;

             for (int i=j; i<endi; i++)
             {
                // (y<<shiftY) = (x<<shiftX) + const(offset)
                final int ii = i - j + 1;
                int offset = -1;
                
                if ((ii<<shiftX) >= (jj<<shiftY))
                {
                  // Above 'diagonal' (including it) : reference is ai (or d)
                  if (y > 0)
                     offset = (jj - ((ii<<shiftX)>>shiftY)) + start - st - 1;
               }
               else
               {
                 // Below 'diagonal' : reference is bi
                 if (x > 0)
                    offset = (jj - ((ii<<shiftX)>>shiftY))*st + start - st - 1;              
              } 

              if (offset >= 0)
                 val = (input[i] & mask_) - (input[offset] & mask_);
              else
                 val = (input[i] & mask_) - this.defaultPixVal;

              sad += ((val + (val >> 31)) ^ (val >> 31)); //abs
              residue[k] = val;
              k++;
            }         
          }
       }
       else if ((direction & DIR_RIGHT) != 0)
       {
          // DIRECTIONAL_R
          final int xMax = this.width - blockDim;
          int line = 0;

          for (int j=start, jj=1; j<endj; j+=st, jj++)
          {
             final int endi = j + blockDim;

             for (int i=endi-1; i>=j ; i--)
             {
                int offset = -1;
                final int ii = blockDim - i + j - 1;
                k = line + i - j;                

                if ((ii<<shiftX) >= (jj<<shiftY))
                {
                   // Above 'diagonal' (including it) : reference is ai
                   if (y > 0)
                      offset = (jj - ((ii<<shiftX)>>shiftY)) + start - st - 1;   
                }
                else
                {
                   // Below 'diagonal' : reference is ci
                   if (x < xMax)
                      offset = (jj - ((ii<<shiftX)>>shiftY))*st + start - st - 1;                                  
                }

                if (offset >= 0)
                   val = (input[i] & mask_) - (input[offset] & mask_);
                else
                   val = (input[i] & mask_) - this.defaultPixVal;
             
                sad += ((val + (val >> 31)) ^ (val >> 31)); //abs
                residue[k] = val;
             } 
             
             line += blockDim; 
          }
       }

       prediction.sad = sad;
       return prediction.sad;
    }


    private int computePredictionDC(Prediction prediction, int[] input, 
       final int x, final int y, int direction)
    {
       final int st = this.stride;
       final int mask_ = this.mask;
       final int start = (y*st) + x;
       final int blockDim = prediction.blockDim;
       final int[] residue = prediction.residue;
       final int endj = start + (st*blockDim);
       int sum = 0;
       int dc = 0;

       if (y > 0)
       {
         sum += blockDim;
         final int above = start - st;
  
         for (int i=0; i<blockDim; i++)
           dc += (input[above+i] & mask_);
       }
          
       if ((direction & DIR_LEFT) != 0)
       {
          // dc=ai+bi
          if (x > 0)
          {
            for (int j=start; j<endj; j+=st)
               dc += (input[j-1] & mask_);

             sum += blockDim;
          }
       }
       else
       {
          // dc=ai+ci
          if (x < this.width - blockDim)
          {
            for (int j=start; j<endj; j+=st)
               dc += (input[j+blockDim] & mask_);

            sum += blockDim;
          }
       }

       dc = (sum == 0) ? this.defaultPixVal : (dc + (sum>>1)) / sum;
       int k = 0;
       int sad = 0;
          
       for (int j=start; j<endj; j+=st)
       {
          final int endi = j + blockDim;

          for (int i=j; i<endi; i+=4)
          {
             // DC_L: xi-dc_l
             // DC_R: xi-dc_r
             final int val0 = (input[i]   & mask_) - dc;
             final int val1 = (input[i+1] & mask_) - dc;
             final int val2 = (input[i+2] & mask_) - dc;
             final int val3 = (input[i+3] & mask_) - dc;
             sad += ((val0 + (val0 >> 31)) ^ (val0 >> 31)); //abs
             sad += ((val1 + (val1 >> 31)) ^ (val1 >> 31)); //abs
             sad += ((val2 + (val2 >> 31)) ^ (val2 >> 31)); //abs
             sad += ((val3 + (val3 >> 31)) ^ (val3 >> 31)); //abs
             residue[k]   = val0;
             residue[k+1] = val1;
             residue[k+2] = val2;
             residue[k+3] = val3;
             k += 4;
          }
       }

       prediction.sad = sad;
       return prediction.sad;
   }


   // Search for a similar block that can be used as reference
   // Base prediction on difference with nearby blocks using 'winner update' strategy
   // Return error and update prediction argument with matching block coord.
   private int computeReferenceSearch(int[] input, final int x, final int y,
           int maxSAD, Prediction prediction, int direction)
   {
      final int blockDim = prediction.blockDim;

      // Populate the set of neighboring candidate blocks
      this.getReferenceSearchBlocks(x, y, blockDim, 0, 0, direction,
              prediction.frame, ACTION_POPULATE, 0);

      SearchBlockContext ctx = null;
      final int mask_ = this.mask;
      final int st = this.stride;
      prediction.sad = MAX_ERROR;

      // Critical speed path
      while (this.searchSet.size() > 0)
      {
         // Select partial winner (lowest error) to update
         ctx = this.searchSet.pollFirst();

         // Full winner found ?
         if (ctx.line >= blockDim)
         {
            this.searchSet.clear();
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
             final int val0 = (input[i]   & mask_) - (data[offs2]   & mask_);
             final int val1 = (input[i+1] & mask_) - (data[offs2+1] & mask_);
             final int val2 = (input[i+2] & mask_) - (data[offs2+2] & mask_);
             final int val3 = (input[i+3] & mask_) - (data[offs2+3] & mask_);
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
            this.searchSet.add(ctx);
      }

      if (ctx != null)
      {
         // Return best result
         prediction.x = ctx.x;
         prediction.y = ctx.y;
         prediction.sad = ctx.sad;
      }

      return prediction.sad;
   }


   // Return the number of block candidates to search for
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
   // Return the index of the match (see getReferenceCandidates for number of matches)
   public int getReferenceIndexFromPosition(int x, int y, int blockDim,
           int xr, int yr, int direction)
   {
      return this.getReferenceSearchBlocks(x, y, blockDim, xr, yr, direction,
              null, ACTION_GET_INDEX, 0);
   }


   // return the coordinates of the match as (x<<16) | y
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
               this.searchSet.add(SearchBlockContext.getContext(referenceFrame, i, j));
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
         val = this.searchSet.size();

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

            return (this.y == c.y) && (this.x == c.x);
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
