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
import kanzi.IndexedIntArray;
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
   public boolean apply(final IndexedIntArray source, final IndexedIntArray destination)
   {
      final int nbThreads = this.delegates.length;
      ArrayList<Callable<Boolean>> filterTasks = new ArrayList<Callable<Boolean>>(nbThreads);
      List<Future<Boolean>> results = new ArrayList<Future<Boolean>>(nbThreads);
      final int area = this.height * this.stride;
      boolean res = true;
      
      for (int i=0; i<this.delegates.length; i++)
      { 
         int srcIdx = (this.direction == HORIZONTAL) ? source.index + (area * i) / nbThreads
                 : source.index + (this.width * i) / nbThreads;
         IndexedIntArray src = new IndexedIntArray(source.array, srcIdx); 
         int dstIdx  = (this.direction == HORIZONTAL) ? destination.index + (area * i) / nbThreads
                 : destination.index + (this.width * i) / nbThreads;
         IndexedIntArray dst = new IndexedIntArray(destination.array, dstIdx); 
         filterTasks.add(new FilterTask(i, this.delegates[i], src, dst));
      } 
      
      try 
      {
         results = this.pool.invokeAll(filterTasks);
      
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
      final int id;
      final IndexedIntArray src;
      final IndexedIntArray dst;
      final IntFilter filter;
      
      
      public FilterTask(int id, IntFilter filter, IndexedIntArray src, IndexedIntArray dst) 
      {
         this.id = id;
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
