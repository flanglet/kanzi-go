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


public class IntegralImageFilter implements IntFilter
{
    private final int width;
    private final int height;
    private final int stride;


    public IntegralImageFilter(int width, int height)
    {
       this(width, height, width);
    }


    public IntegralImageFilter(int width, int height, int stride)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");
        
        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

        if (width*height >= 1<<23)
            throw new IllegalArgumentException("The image area is limited to 8388608 pixels");

        this.width = width;
        this.height = height;
        this.stride = stride;
    }


   @Override
   public boolean apply(SliceIntArray input, SliceIntArray output)
   {
      if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
         return false;

      if (input.array == output.array)
         return false;
      
      final int[] src = input.array;
      final int[] dst = output.array;
      int srcIdx = input.index;
      int dstIdx = output.index;
      final int w = this.width;
      final int h = this.height;
      final int st = this.stride;           
		int sumRow = 0;
         
      // First row
      for (int x=0; x<w; x++)
      {
         sumRow += src[srcIdx+x];
         dst[dstIdx+x]= sumRow;
      }

      srcIdx += st;
      dstIdx += st;
         
      // Other rows
      for (int y=1; y<h; y++)
      {
         sumRow = 0;

         for (int x=0; x<w; x++)
         {
            sumRow += src[srcIdx+x];
            dst[dstIdx+x] = dst[dstIdx+x-st] + sumRow;
         }

         srcIdx += st;
         dstIdx += st;
      }   
      
      return true;
   }   
}
