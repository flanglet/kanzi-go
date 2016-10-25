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

package kanzi.filter;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Future;
import kanzi.SliceIntArray;
import kanzi.IntFilter;


// A filter that runs several filter delegates in parallel
public class ParallelFilter implements IntFilter
{
   // Possible directions
   public static final int HORIZONTAL = 1;
   public static final int VERTICAL = 2;

   private final int width;
   private final int height;
   private final int stride;
   private final int direction;
   private final IntFilter[] delegates;
   private final ExecutorService  pool;
   
   
   public ParallelFilter(int width, int height, int stride, 
           ExecutorService pool, IntFilter[] delegates)
   {
      this(width, height, stride, pool, delegates, HORIZONTAL);
   }
   
   
   public ParallelFilter(int width, int height, int stride, ExecutorService pool, 
           IntFilter[] delegates, int direction)
   {
      if (height < 8)
          throw new IllegalArgumentException("The height must be at least 8");

      if (width < 8)
          throw new IllegalArgumentException("The width must be at least 8");

      if (stride < 8)
          throw new IllegalArgumentException("The stride must be at least 8");
        
      if (pool == null)
         throw new NullPointerException("Invalid null pool parameter");
      
      if (delegates == null)
         throw new NullPointerException("Invalid null delegates parameter");
      
      if (((direction & HORIZONTAL) == 0) && ((direction & VERTICAL) == 0))
          throw new IllegalArgumentException("Invalid direction parameter (must be VERTICAL or HORIZONTAL)");
        
      if (((direction & HORIZONTAL) != 0) && ((direction & VERTICAL) != 0))
          throw new IllegalArgumentException("Invalid direction parameter (must be VERTICAL or HORIZONTAL)");
        
      if (delegates.length == 0)
         throw new IllegalArgumentException("Invalid empty delegates parameter");
      
      this.width  = width;
      this.height = height;
      this.stride = stride;   
      this.direction = direction;
      this.delegates = new IntFilter[delegates.length];
      System.arraycopy(delegates, 0, this.delegates, 0, delegates.length);
      this.pool = pool;
   }

   
   @Override
   public boolean apply(final SliceIntArray input, final SliceIntArray output)
   {
      if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
         return false;
      
      final int nbThreads = this.delegates.length;
      ArrayList<Callable<Boolean>> filterTasks = new ArrayList<Callable<Boolean>>(nbThreads);
      final int area = this.height * this.stride;
      boolean res = true;
      
      for (int i=0; i<this.delegates.length; i++)
      { 
         int srcIdx = (this.direction == HORIZONTAL) ? input.index + (area * i) / nbThreads
                 : input.index + (this.width * i) / nbThreads;
         SliceIntArray src = new SliceIntArray(input.array, srcIdx); 
         int dstIdx  = (this.direction == HORIZONTAL) ? output.index + (area * i) / nbThreads
                 : output.index + (this.width * i) / nbThreads;
         SliceIntArray dst = new SliceIntArray(output.array, dstIdx); 
         filterTasks.add(new FilterTask(this.delegates[i], src, dst));
      } 
      
      try 
      {
         List<Future<Boolean>> results = this.pool.invokeAll(filterTasks);
      
         for (Future<Boolean> fr : results)
            res &= fr.get();      
      }
      catch (InterruptedException e)
      {
         // Ignore
      }
      catch (ExecutionException e)
      {
         return false;
      }
      
      return res;      
   }
   
   
   static class FilterTask implements Callable<Boolean>
   {
      final SliceIntArray src;
      final SliceIntArray dst;
      final IntFilter filter;
      
      
      public FilterTask(IntFilter filter, SliceIntArray src, SliceIntArray dst) 
      {
         this.filter = filter;
         this.src = src;
         this.dst = dst;
      }
      
      
      @Override
      public Boolean call() 
      {
         return this.filter.apply(this.src, this.dst);
      }
      
   }
}
