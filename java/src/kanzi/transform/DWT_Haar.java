
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

package kanzi.transform;

import kanzi.SliceIntArray;
import kanzi.IntTransform;


// Discrete Wavelet Transform Haar for 2D signals
public class DWT_Haar implements IntTransform
{
    private final int[] data;
    private final int width;
    private final int height;
    private final int steps;
    private final boolean scale; // scale forward transform ?


    // dim (dimension of the whole image)
    // For perfect reconstruction, forward results are scaled by 2
    public DWT_Haar(int dim)
    {
       this(dim, dim, 5, true);
    }


    // For perfect reconstruction, forward results are scaled by 2
    public DWT_Haar(int width, int height)
    {
       this(width, height, 5, true);
    }


    // If scale is true, the forward transform is scaled by 2
    public DWT_Haar(int width, int height, int steps, boolean scale)
    {
        if (width < 8)
            throw new IllegalArgumentException("Invalid transform width (must"
                    + " be at least 8)");

        if (height < 8)
            throw new IllegalArgumentException("Invalid transform width (must"
                    + " be at least 8)");

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
        this.data = new int[width*height];
        this.scale = scale;
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


    // Calculate the forward discrete wavelet transform of the 2D input signal
    // Not thread safe because this.data is modified
    // Is scale is true, the results are scaled by 2
    @Override
    public boolean forward(SliceIntArray src, SliceIntArray dst)
    {
        if ((!SliceIntArray.isValid(src)) || (!SliceIntArray.isValid(dst)))
           return false;

        final int count = this.width * this.height;
        
        if (src.length != count)
           return false;
       
        if (dst.length < count)
           return false;
        
        if (dst.index + count > dst.array.length)
           return false;   
        
        if ((src.array != dst.array) || (src.index != dst.index))
           System.arraycopy(src.array, src.index, dst.array, dst.index, count);

        final int fScale = (this.scale == true) ? 0 : 1;
        
        for (int i=0; i<this.steps; i++)
        {
           // First, vertical transform
           this.forward(dst.array, dst.index, this.width, 1, this.width>>i, this.height>>i, 0);

           // Then horizontal transform on the updated signal
           this.forward(dst.array, dst.index, 1, this.width, this.height>>i, this.width>>i, fScale);
        }

        if (src.index + count > src.length)
           return false;
       
        if (dst.index + count > dst.length)
           return false;   
        
        src.index += count;
        dst.index += count;
        return true;
    }


    private void forward(int[] block, int blkptr, int stride, int inc, int dim1, int dim2, final int shift)
    {
        final int stride2 = stride << 1;
        final int endOffs = blkptr + (dim1 * inc);
        final int half = stride * (dim2 >> 1);

        for (int offset=blkptr; offset<endOffs; offset+=inc)
        {
            final int end = offset + (dim2 - 2) * stride;
            final int endj = offset + half;

            for (int i=offset, j=offset; j<endj; i+=stride2, j+=stride)
            {
               final int u = block[i]; 
               final int v = block[i+stride];
               this.data[j]      = (u+v) >> shift;
               this.data[half+j] = (u-v) >> shift;	
            }

            block[end+stride] = this.data[end+stride];
            
            for (int i=offset; i<=end; i+=stride)
                block[i] = this.data[i];
        }
    }


    // Calculate the reverse discrete wavelet transform of the 2D input signal
    // Not thread safe because this.data is modified
    @Override
    public boolean inverse(SliceIntArray src, SliceIntArray dst)
    {
        if ((!SliceIntArray.isValid(src)) || (!SliceIntArray.isValid(dst)))
           return false;

        final int count = this.width * this.height;      
        
        if (src.length != count)
           return false;
       
        if (dst.length < count)
           return false;
        
        if (dst.index + count > dst.array.length)
           return false;   
               
        if ((src.array != dst.array) || (src.index != dst.index))
           System.arraycopy(src.array, src.index, dst.array, dst.index, count);

        final int iScale = (this.scale == true) ? 1 : 0;

        for (int i=this.steps-1; i>=0; i--)
        {
           // First horizontal transform
           this.inverse(dst.array, dst.index, 1, this.width, this.height>>i, this.width>>i, iScale);

           // Then vertical transform on the updated signal
           this.inverse(dst.array, dst.index, this.width, 1, this.width>>i, this.height>>i, 1);
        }
        
        src.index += count;
        dst.index += count;
        return true;
    }


    private void inverse(int[] block, int blkptr, int stride, int inc, int dim1, int dim2, final int shift)
    {
        final int stride2 = stride << 1;
        final int endOffs = blkptr + (dim1 * inc);
        final int half = stride * (dim2 >> 1);

        for (int offset=blkptr; offset<endOffs; offset+=inc)
        {
            final int end = offset + (dim2 - 2) * stride;
            final int endj = offset + half;

            for (int i=offset; i<=end; i+=stride)
                this.data[i] = block[i];
             
            this.data[end+stride] = block[end+stride];
            
            for (int i=offset, j=offset; j<endj; i+=stride2, j+=stride)
            {
               final int u = this.data[j]; 
               final int v = this.data[half+j]; 
               block[i]        = (u+v) >> shift;
               block[i+stride] = (u-v) >> shift;	
            }
        }
    }
}