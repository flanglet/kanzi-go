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


// Discrete Wavelet Transform Cohen-Daubechies-Feauveau 9/7 for 2D signals
// Fast integer based implementation using the lifting scheme.
public class DWT_CDF_9_7 implements IntTransform
{
    private static final int SHIFT_12  = 12;
    private static final int ADJUST_12 = 1 << (SHIFT_12 - 1);
    private static final int SHIFT_11  = SHIFT_12 - 1;
    private static final int ADJUST_11 = 1 << (SHIFT_11 - 1);

    private static final int PREDICT1 = 6497; // 1.586134342  * 1<<12
    private static final int UPDATE1  = 217;  // 0.0529801185 * 1<<12
    private static final int PREDICT2 = 3616; // 0.8829110762 * 1<<12
    private static final int UPDATE2  = 1817; // 0.4435068522 * 1<<12
    private static final int SCALING1 = 4709; // 1.149604398  * 1<<12
    private static final int SCALING2 = 3563; // 0.869864452  * 1<<12

    private final int[] data;
    private final int width;
    private final int height;
    private final int steps;


    // dim (dimension of the whole image)
    public DWT_CDF_9_7(int dim)
    {
       this(dim, dim, 5);
    }


    public DWT_CDF_9_7(int width, int height)
    {
       this(width, height, 5);
    }


    public DWT_CDF_9_7(int width, int height, int steps)
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

        if ((width >> steps) < 4)
            throw new IllegalArgumentException("Invalid width for band L0 (must"
                    + " be at least 4)");

        if ((height >> steps) < 4)
            throw new IllegalArgumentException("Invalid height for band L0 (must"
                    + " be at least 4)");

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
    @Override
    public boolean forward(IndexedIntArray src, IndexedIntArray dst)
    {
        if (src.array.length < this.width*this.height)
           return false;

        if (dst.array.length < this.width*this.height)
           return false;

        if ((src.array != dst.array) || (src.index != dst.index))
        {
           System.arraycopy(src.array, src.index, dst.array, dst.index, this.width*this.height);
        }

        for (int i=0; i<this.steps; i++)
        {
           // First, vertical transform
           this.forward(dst.array, dst.index, this.width, 1, this.width>>i, this.height>>i);

           // Then horizontal transform on the updated signal
           this.forward(dst.array, dst.index, 1, this.width, this.height>>i, this.width>>i);
        }

        src.index += (this.width*this.height);
        dst.index += (this.width*this.height);
        return true;
    }


    private void forward(int[] block, int blkptr, int stride, int inc, int dim1, int dim2)
    {
        final int stride2 = stride << 1;
        final int endOffs = blkptr + (dim1 * inc);
        final int half = stride * (dim2 >> 1);

        for (int offset=blkptr; offset<endOffs; offset+=inc)
        {
            final int end = offset + (dim2 - 2) * stride;
            int prev = block[offset];

            // First lifting stage : Predict 1
            for (int i=offset+stride; i<end; i+=stride2)
            {
                final int next = block[i+stride];
                block[i] -= ((PREDICT1 * (prev + next) + ADJUST_12) >> SHIFT_12);
                prev = next;
            }

            block[end+stride] -= ((PREDICT1 * block[end] + ADJUST_11) >> SHIFT_11);
            prev = block[offset+stride];

            // Second lifting stage : Update 1
            for (int i=offset+stride2; i<=end; i+=stride2)
            {
                final int next = block[i+stride];
                block[i] -= ((UPDATE1 * (prev + next)+ ADJUST_12) >> SHIFT_12);
                prev = next;
            }

            block[offset] -= ((UPDATE1 * block[offset+stride] + ADJUST_11) >> SHIFT_11);
            prev = block[offset];

            // Third lifting stage : Predict 2
            for (int i=offset+stride; i<end; i+=stride2)
            {
                final int next = block[i+stride];
                block[i] += ((PREDICT2 * (prev + next) + ADJUST_12) >> SHIFT_12);
                prev = next;
            }

            block[end+stride] += ((PREDICT2 * block[end] + ADJUST_11) >> SHIFT_11);
            prev = block[offset+stride];

            // Fourth lifting stage : Update 2
            for (int i=offset+stride2; i<=end; i+=stride2)
            {
                final int next = block[i+stride];
                block[i] += ((UPDATE2 * (prev + next) + ADJUST_12) >> SHIFT_12);
                prev = next;
            }

            block[offset] += ((UPDATE2 * block[offset+stride] + ADJUST_11) >> SHIFT_11);

            // Scale
            for (int i=offset; i<=end; i+=stride2)
            {
                block[i] = (block[i] * SCALING1 + ADJUST_12) >> SHIFT_12;
                block[i+stride] = (block[i+stride] * SCALING2 + ADJUST_12) >> SHIFT_12;
            }

            // De-interleave sub-bands
            final int endj = offset + half;

            for (int i=offset, j=offset; j<endj; i+=stride2, j+=stride)
            {
                this.data[j] = block[i];
                this.data[half+j] = block[i+stride];
            }

            block[end+stride] = this.data[end+stride];

            for (int i=offset; i<=end; i+=stride)
                block[i] = this.data[i];
        }
    }
    

    // Calculate the reverse discrete wavelet transform of the 2D input signal
    // Not thread safe because this.data is modified
    @Override
    public boolean inverse(IndexedIntArray src, IndexedIntArray dst)
    {
        if (src.array.length < this.width*this.height)
           return false;

        if (dst.array.length < this.width*this.height)
           return false;

        if ((src.array != dst.array) || (src.index != dst.index))
        {
           System.arraycopy(src.array, src.index, dst.array, dst.index, this.width*this.height);
        }

        for (int i=this.steps-1; i>=0; i--)
        {
           // First horizontal transform
           this.inverse(dst.array, dst.index, 1, this.width, this.height>>i, this.width>>i);

           // Then vertical transform on the updated signal
           this.inverse(dst.array, dst.index, this.width, 1, this.width>>i, this.height>>i);
        }

        src.index += (this.width*this.height);
        dst.index += (this.width*this.height);
        return true;
    }


    private void inverse(int[] block, int blkptr, int stride, int inc, int dim1, int dim2)
    {
        final int stride2 = stride << 1;
        final int endOffs = blkptr + (dim1 * inc);
        final int half = stride * (dim2 >> 1);

        for (int offset=blkptr; offset<endOffs; offset+=inc)
        {
            final int end = offset + (dim2 - 2) * stride;
            final int endj = offset + half;

            // De-interleave sub-bands
            for (int i=offset; i<=end; i+=stride)
                this.data[i] = block[i];

            this.data[end+stride] = block[end+stride];

            for (int i=offset, j=offset; j<endj; i+=stride2, j+=stride)
            {
                block[i] = this.data[j];
                block[i+stride] = this.data[half+j];
            }

            // Reverse scale
            for (int i=offset; i<=end; i+=stride2)
            {
                block[i] = (block[i] * SCALING2 + ADJUST_12) >> SHIFT_12;
                block[i+stride] = (block[i+stride] * SCALING1 + ADJUST_12) >> SHIFT_12;
            }

            // Reverse Update 2
            int prev = block[offset+stride];

            for (int i=offset+stride2; i<=end; i+=stride2)
            {
                final int next = block[i+stride];
                block[i] -= ((UPDATE2 * (prev + next) + ADJUST_12) >> SHIFT_12);
                prev = next;
            }

            block[offset] -= ((UPDATE2 * block[offset+stride] + ADJUST_11) >> SHIFT_11);
            prev = block[offset];

            // Reverse Predict 2
            for (int i=offset+stride; i<end; i+=stride2)
            {
                final int next = block[i+stride];
                block[i] -= ((PREDICT2 * (prev + next) + ADJUST_12) >> SHIFT_12);
                prev = next;
            }

            block[end+stride] -= ((PREDICT2 * block[end] + ADJUST_11) >> SHIFT_11);
            prev = block[offset+stride];

            // Reverse Update 1
            for (int i=offset+stride2; i<=end; i+=stride2)
            {
                final int next = block[i+stride];
                block[i] += ((UPDATE1 * (prev + next) + ADJUST_12) >> SHIFT_12);
                prev = next;
            }

            block[offset] += ((UPDATE1 * block[offset+stride] + ADJUST_11) >> SHIFT_11);
            prev = block[offset];

            // Reverse Predict 1
            for (int i=offset+stride; i<end; i+=stride2)
            {
                final int next = block[i+stride];
                block[i] += ((PREDICT1 * (prev + next) + ADJUST_12) >> SHIFT_12);
                prev = next;
            }

            block[end+stride] += ((PREDICT1 * block[end] + ADJUST_11) >> SHIFT_11);
        }
    }
}