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

package kanzi.filter;

import kanzi.SliceIntArray;
import kanzi.IntFilter;


// Based on algorithm from http://www.blackpawn.com/texts/blur/default.html

public final class BlurFilter implements IntFilter
{
    private final int width;
    private final int height;
    private final int stride;
    private final int radius;
    private final int iterations;
    private final int[] line;
 
    
    public BlurFilter(int width, int height, int radius)
    {
        this(width, height, width, radius, 4);
    }
    
    
    public BlurFilter(int width, int height, int stride, int radius)
    {
        this(width, height, stride, radius, 4);
    }
    
    
    public BlurFilter(int width, int height, int stride, int radius, int iterations)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");
        
        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");
        
        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");
        
        if ((radius < 1) || (radius > 32))
            throw new IllegalArgumentException("The radius must be int [1..32]");

        if (iterations < 1)
            throw new IllegalArgumentException("The iterations must be at least 1");
        
        if (iterations > 100)
            throw new IllegalArgumentException("The iterations must be at most 100");
        
        this.height = height;
        this.width = width;
        this.stride = stride;
        this.radius = radius;
        this.iterations = iterations;
        int size = (this.width > this.height) ? this.width : this.height;
        this.line = new int[size];
    }
    

    @Override
    public boolean apply(SliceIntArray input, SliceIntArray output)
    {
        if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
            return false;
      
        this.blurHorizontal(input, output);
        this.blurVertical(output, output);
        
        for (int i=1; i<this.iterations; i++)
        {
           this.blurHorizontal(output, output);
           this.blurVertical(output, output);
        }
        
        return true;
    }
    
    
    // Implementation using a sliding box to reduce the number of operations
    private boolean blurHorizontal(SliceIntArray source, SliceIntArray destination)
    {
        final int[] src = source.array;
        final int[] dst = destination.array;
        final int srcIdx = source.index;
        final int dstIdx = destination.index;
        final int rd = this.radius;
        final int w = this.width;
        final int h = this.height;
        final int st = this.stride;
        
        final int boxSize = (2 * rd) + 1;
        final int invBoxSize = (1<<16) / boxSize;
        int srcStart = srcIdx;
        int dstStart = dstIdx;
        
        for (int j=0; j<h; j++)
        {
            // First pixel of each line: calculate the sum over the whole box
            int pixel = src[srcStart];
            
            // Pixel 0: sum 'negative' x pixels ('radius' times)
            int totalR = rd * ((pixel >> 16) & 0xFF);
            int totalG = rd * ((pixel >>  8) & 0xFF);
            int totalB = rd * ( pixel & 0xFF);
            
            for (int i=0, n=0; i<=rd; i++)
            {
                pixel = src[srcStart+n];
                totalR += ((pixel >> 16) & 0xFF);
                totalG += ((pixel >>  8) & 0xFF);
                totalB +=  (pixel & 0xFF);
                
                if (n < w - 1)
                    n++;
            }
            
            // Subsequent pixels: update the sum by sliding the whole box
            for (int i=0; i<w; i++)
            {
                int val;
                val  = ((totalR*invBoxSize) >>> 16) << 16;
                val |= ((totalG*invBoxSize) >>> 16) << 8;
                val |= (totalB*invBoxSize) >>> 16;
                this.line[i] = val;
                
                // Limit lastIdx to positive or null values
                int lastIdx = i - rd;
                lastIdx = lastIdx & (-lastIdx >> 31);
                
                // Limit newIdx to values less than width
                int newIdx = i + rd + 1;
                final int mask = (newIdx - w) >>> 31;
                newIdx = (newIdx & -mask) | ((w - 1) & (mask - 1));
                
                final int enteringPixel = src[srcStart+newIdx];
                final int leavingPixel  = src[srcStart+lastIdx];
                
                // Update sums of sliding window
                totalR += ((enteringPixel >> 16) & 0xFF);
                totalG += ((enteringPixel >>  8) & 0xFF);
                totalB +=  (enteringPixel & 0xFF);
                totalR -= ((leavingPixel >> 16) & 0xFF);
                totalG -= ((leavingPixel >>  8) & 0xFF);
                totalB -=  (leavingPixel & 0xFF);
            }
            
            for (int i=0, n=dstStart; i<w; i++, n++)
                dst[n] = this.line[i];
            
            srcStart += st;
            dstStart += st;
        }
        
        return true;
    }
    
    
    
    // Implementation using a sliding box to reduce the number of operations
    private boolean blurVertical(SliceIntArray source, SliceIntArray destination)
    {
        final int[] src = source.array;
        final int[] dst = destination.array;
        final int srcIdx = source.index;
        final int dstIdx = destination.index;
        final int rd = this.radius;
        final int w = this.width;
        final int h = this.height;
        final int st = this.stride;
        
        int len = st * h;
        int boxSize = (2 * rd) + 1;
        int srcStart = srcIdx;
        int dstStart = dstIdx;
        
        for (int j=0; j<w; j++)
        {
            // First pixel of each line: calculate the sum over the whole box
            int pixel = src[srcStart];
            
            // Pixel 0: sum 'negative' x pixels ('radius' times)
            int totalR = rd * ((pixel >> 16) & 0xFF);
            int totalG = rd * ((pixel >>  8) & 0xFF);
            int totalB = rd * (pixel & 0xFF);
            
            for (int i=0, n=0; i<=rd; i++)
            {
                pixel = src[srcStart+n];
                totalR += ((pixel >> 16) & 0xFF);
                totalG += ((pixel >>  8) & 0xFF);
                totalB +=  (pixel & 0xFF);
                
                if (n + st < len)
                    n += st;
            }
            
            // Subsequent pixels: update the sum by sliding the window
            for (int i=0; i<h; i++)
            {
                int val;
                val  = (totalR / boxSize) << 16;
                val |= (totalG / boxSize) <<  8;
                val |= (totalB / boxSize);
                this.line[i] = val;
                
                // Limit lastIdx to positive or null values
                int lastIdx = i - rd;
                lastIdx = lastIdx & (-lastIdx >> 31);
                lastIdx *= st;
                
                // Limit newIdx to values less than height
                int newIdx  = i + rd + 1;
                final int mask = (newIdx - h) >>> 31;
                newIdx = (newIdx & -mask) | ((h - 1) & (mask - 1));
                newIdx *= st;
                
                final int enteringPixel = src[srcStart+newIdx];
                final int leavingPixel  = src[srcStart+lastIdx];
                
                // Update sums of sliding window
                totalR += ((enteringPixel >> 16) & 0xFF);
                totalG += ((enteringPixel >> 8)  & 0xFF);
                totalB +=  (enteringPixel & 0xFF);
                totalR -= ((leavingPixel >> 16) & 0xFF);
                totalG -= ((leavingPixel >> 8)  & 0xFF);
                totalB -=  (leavingPixel & 0xFF);
            }
            
            for (int i=0, n=dstStart; i<h; i++, n+=st)
                dst[n] = this.line[i];
            
            srcStart++;
            dstStart++;
        }
        
        return true;
    }
       
}