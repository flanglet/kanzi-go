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


public class BilinearDownSampler implements DownSampler
{
    private final int width;
    private final int height;
    private final int stride;
    private final int offset;
    private final int factor;


    public BilinearDownSampler(int width, int height)
    {
        this(width, height, width, 0, 2);
    }


    public BilinearDownSampler(int width, int height, int factor)
    {
        this(width, height, width, 0, factor);
    }


    public BilinearDownSampler(int width, int height, int stride, int offset, int factor)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");

        if (offset < 0)
            throw new IllegalArgumentException("The offset must be at least 0");

        if (stride < width)
            throw new IllegalArgumentException("The stride must be at least as big as the width");

        if ((height & 7) != 0)
            throw new IllegalArgumentException("The height must be a multiple of 8");

        if ((width & 7) != 0)
            throw new IllegalArgumentException("The width must be a multiple of 8");

        if ((factor != 2) && (factor != 4))
            throw new IllegalArgumentException("This implementation only supports "+
                    "a scaling factor equal to 2 or 4");

        this.height = height;
        this.width = width;
        this.stride = stride;
        this.offset = offset;
        this.factor = factor;
    }


    @Override
    public void subSampleHorizontal(int[] input, int[] output)
    {
        int iOffs = this.offset;
        int oOffs = 0;
        final int w = this.width;
        final int st = this.stride;
        
        if (this.factor == 2)
        {
            for (int j=this.height; j>0; j--)
            {
                final int end = iOffs + w;

                for (int i=iOffs; i<end; i+=8)
                {
                    output[oOffs++] = (input[i]   + input[i+1]) >> 1;
                    output[oOffs++] = (input[i+2] + input[i+3]) >> 1;
                    output[oOffs++] = (input[i+4] + input[i+5]) >> 1;
                    output[oOffs++] = (input[i+6] + input[i+7]) >> 1;
                }

                iOffs += st;
            }
        }
        else // factor == 4
        {
            for (int j=this.height; j>0; j--)
            {
                final int end = iOffs + w;

                for (int i=iOffs; i<end; i+=8)
                {
                    output[oOffs++] = (input[i]   + input[i+1] + input[i+2] + input[i+3] + 2) >> 2;
                    output[oOffs++] = (input[i+4] + input[i+5] + input[i+6] + input[i+7] + 2) >> 2;
                }

                iOffs += st;
            }
        }
    }


    @Override
    public void subSampleVertical(int[] input, int[] output)
    {
        final int w = this.width;
        final int st = this.stride;
        final int st2 = st  + st;
        int iOffs = this.offset;
        int oOffs = 0;

        if (this.factor == 2)
        {
            for (int j=this.height; j>0; j-=2)
            {
                final int end = iOffs + w;

                for (int i=iOffs; i<end; )
                {
                    output[oOffs++] = (input[i] + input[i+st]) >> 1;
                    i++;
                    output[oOffs++] = (input[i] + input[i+st]) >> 1;
                    i++;
                    output[oOffs++] = (input[i] + input[i+st]) >> 1;
                    i++;
                    output[oOffs++] = (input[i] + input[i+st]) >> 1;
                    i++;
                }

                iOffs += st2;
            }
        }
        else // factor = 4
        {
            final int st3 = st2 + st;
            final int st4 = st3 + st;

            for (int j=this.height; j>0; j-=4)
            {
                final int end = iOffs + w;

                for (int i=iOffs; i<end; )
                {
                    output[oOffs++] = (input[i] + input[i+st] + input[i+st2] + input[i+st3] + 2) >> 2;
                    i++;
                    output[oOffs++] = (input[i] + input[i+st] + input[i+st2] + input[i+st3] + 2) >> 2;
                    i++;
                    output[oOffs++] = (input[i] + input[i+st] + input[i+st2] + input[i+st3] + 2) >> 2;
                    i++;
                    output[oOffs++] = (input[i] + input[i+st] + input[i+st2] + input[i+st3] + 2) >> 2;
                    i++;
                }

                iOffs += st4;
            }
        }
    }


    @Override
    public void subSample(int[] input, int[] output)
    {
        int line0 = this.offset;
        final int w = this.width;
        final int st = this.stride;
        int oOffs = 0;      

        if (this.factor == 2)
        {
            for (int j=this.height; j>0; j-=2)
            {
                final int line1 = line0 + st;

                for (int i=0; i<w; i+=2)
                {
                    final int val = input[line0+i]  + input[line0+i+1] +
                                    input[line1+i]  + input[line1+i+1];
                    output[oOffs++] = (val + 2) >> 2;
                }

                line0 = line1 + st;
            }
        }
        else
        {
            for (int j=this.height; j>0; j-=4)
            {
                final int line1 = line0 + st;
                final int line2 = line1 + st;
                final int line3 = line2 + st;

                for (int i=0; i<w; i+=4)
                {
                    final int val = input[line0+i]   + input[line0+i+1] +
                                       input[line0+i+2] + input[line0+i+3] +
                                       input[line1+i]   + input[line1+i+1] +
                                       input[line1+i+2] + input[line1+i+3] +
                                       input[line2+i]   + input[line2+i+1] +
                                       input[line2+i+2] + input[line2+i+3] +
                                       input[line3+i]   + input[line3+i+1] +
                                    input[line3+i+2] + input[line3+i+3];
                    output[oOffs++] = (val + 8) >> 4;
                }

                line0 = line3 + st;
            }
        }
    }


    @Override
    public boolean supportsScalingFactor(int factor)
    {
        return ((factor == 2) || (factor == 4));
    }
}