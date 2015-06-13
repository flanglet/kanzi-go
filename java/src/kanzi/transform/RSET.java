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

package kanzi.transform;

import kanzi.IndexedIntArray;
import kanzi.IntTransform;


// Recursive SubSampling Estimate Transform
// A pyramid transform based on linear subsampling estimates.
// At each steps, the image is subsampled by 2 horizontally, vertically and
// diagonally by decimation. The low pass band (LL) gets the subsamples, the 3
// high pass band (HL, LH, HH) receive the difference between the linear estimate
// from 2 successives pixels in LL and the actual value of the pixels in the 
// original image. 
// Advantages: reversible, separable, simple, pure integer math, good energy compaction
// EG.
// original:               =>  LL        HL  
// 133 134 140 143 144 ...     133 140    -2   1  
// 133 136 137 139 143 ...     133 138     3   3
// 133 138 138 139 145 ...     LH        HH    
// 135 140 145 149 150 ...       0  -2     1  -3
// ...                         ... ...
// LL take 1 sample out of 2 in each direction
// HL (horizontal): 134-(140+133)/2 = -2,  143-(144+140)/2 =  1, etc...
// LH (vertical)  : 133-(133+133)/2 =  0,  137-(138+140)/2 = -2, etc...

public class RSET implements IntTransform
{
    private final int[] data;
    private final int width;
    private final int height;
    private final int steps;
    private final int stride;


    // dim (dimension of the whole image) 
    public RSET(int dim)
    {
       this(dim, dim, 5);
    }


    public RSET(int width, int height)
    {
       this(width, height, 5);
    }


    public RSET(int width, int height, int steps)
    {
       this(width, height, width, steps);
    }
    
    
    public RSET(int width, int height, int stride, int steps)
    {
        if (width < 8)
            throw new IllegalArgumentException("Invalid transform width (must"
                    + " be at least 8)");

        if (height < 8)
            throw new IllegalArgumentException("Invalid transform width (must"
                    + " be at least 8)");

        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

        if (steps < 1)
            throw new IllegalArgumentException("Invalid number of iterations "
                    + "(must be at least 1)");

        if ((width >> steps) < 8)
            throw new IllegalArgumentException("Invalid width for band L0 (must"
                    + " be at least 8)");

        if ((height >> steps) < 8)
            throw new IllegalArgumentException("Invalid height for band L0 (must"
                    + " be at least 8)");

        if (((width >> steps) << steps) != width)
            throw new IllegalArgumentException("Invalid parameters: change width or number of steps ("
                    + width + " divided by 2^" + steps + " is not an integer value)");

        if (((height >> steps) << steps) != height)
            throw new IllegalArgumentException("Invalid parameters: change height or number of steps ("
                    + height + " divided by 2^" + steps + " is not an integer value)");

        this.width = width;
        this.height = height;
        this.steps = steps;
        this.data = new int[width];
        this.stride = stride;
    }


    public int getWidth()
    {
        return this.width;
    }


    public int getHeight()
    {
        return this.height;
    }


    public int getLevels()
    {
        return this.steps;
    }


    @Override
    public boolean forward(IndexedIntArray src, IndexedIntArray dst)
    {
        if (src.array.length < this.width*this.height)
           return false;

        if (dst.array.length < this.width*this.height)
           return false;

        if ((src.array != dst.array) || (src.index != dst.index))
        {
           if (this.stride == this.width) 
           {
              System.arraycopy(src.array, src.index, dst.array, dst.index, this.width*this.height);
           }
           else
           {
              int iOffs = src.index;
              int oOffs = dst.index;

              for (int j=this.height-1; j>=0; j--)
              {
                 System.arraycopy(src.array, iOffs, dst.array, oOffs, this.stride);
                 iOffs += this.stride;
                 oOffs += this.stride;
              }
           }
        }

        for (int i=0; i<this.steps; i++)
        {
           // First, vertical transform
           this.forward(dst.array, dst.index, 1, this.stride, this.width>>i, this.height>>i);

           // Then horizontal transform on the updated signal
           this.forward(dst.array, dst.index, this.stride, 1, this.height>>i, this.width>>i);
        }

        src.index += (this.width*this.height);
        dst.index += (this.width*this.height);
        return true;
    }


    private void forward(int[] block, int blkptr, int stride, int inc, int dim1, int dim2)
    {
       final int[] delta = this.data;
       int offs = blkptr;
       final int end1 = dim1;
       final int end2 = (dim2-1) * inc;
       final int half = (dim2 * inc) >> 1;

       for (int j=0; j<end1; j++)
       {
          int prev = block[offs];
          int n = 0;

          for (int i=inc; i<end2; i+=inc)
          {
             final int mid = block[offs+i];
             i += inc;
             final int cur = block[offs+i];
             delta[n++] = mid - ((cur + prev) >> 1);
             block[offs+(i>>1)] = cur;
             prev = cur;
          }

          delta[n] = block[offs+end2] - prev;
          n = 0;
          
          for (int i=0; i<half; i+=inc)
             block[offs+half+i] = delta[n++];

          offs += stride;
       }
    }

    
    @Override
    public boolean inverse(IndexedIntArray src, IndexedIntArray dst)
    {
        if (src.array.length < this.width*this.height)
           return false;

        if (dst.array.length < this.width*this.height)
           return false;

        if ((src.array != dst.array) || (src.index != dst.index))
        {
           if (this.stride == this.width) 
           {
              System.arraycopy(src.array, src.index, dst.array, dst.index, this.width*this.height);
           }
           else
           {
              int iOffs = src.index;
              int oOffs = dst.index;

              for (int j=this.height-1; j>=0; j--)
              {
                 System.arraycopy(src.array, iOffs, dst.array, oOffs, this.stride);
                 iOffs += this.stride;
                 oOffs += this.stride;
              }
           }
        }

        for (int i=this.steps-1; i>=0; i--)
        {
           // First horizontal transform
           this.inverse(dst.array, dst.index, this.stride, 1, this.height>>i, this.width>>i);

           // Then vertical transform on the updated signal
           this.inverse(dst.array, dst.index, 1, this.stride, this.width>>i, this.height>>i);
        }

        src.index += (this.width*this.height);
        dst.index += (this.width*this.height);
        return true;
    }


    private void inverse(int[] block, int blkptr, int stride, int inc, int dim1, int dim2)
    {
       final int[] delta = this.data;
       final int end1 = dim1;
       final int end2 = (dim2 - 1) * inc;
       final int half = (dim2 * inc) >> 1;
       int offs = blkptr + (dim1 - 1) * stride;

       for (int j=0; j<end1; j++)
       {
          int n = 0;
          
          for (int i=0; i<half; i+=inc) 
             delta[n++] = block[offs+half+i];         
             
          int prev = block[offs+half-inc];
          
          for (int i=end2; i>0; i-=inc)
          {           
             i -= inc;
             final int cur = block[offs+(i>>1)];
             block[offs+i] = cur;
             block[offs+i+inc] = ((cur + prev) >> 1) + delta[--n];
             prev = cur;
          }
       
          offs -= stride;
       }
    }
}