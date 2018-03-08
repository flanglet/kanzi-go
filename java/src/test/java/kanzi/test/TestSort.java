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

package kanzi.test;

import org.junit.Test;

import java.util.Arrays;
import java.util.concurrent.ForkJoinPool;
import kanzi.util.sort.BucketSort;
import kanzi.util.sort.FlashSort;
import kanzi.util.sort.ForkJoinParallelSort;
import kanzi.util.sort.HeapSort;
import kanzi.util.sort.MergeSort;
import kanzi.util.sort.QuickSort;
import kanzi.util.sort.RadixSort;
import kanzi.util.sort.SpreadSort;
import org.junit.Assert;


public class TestSort
{
   @Test
   public void testSorting()
   {
      Assert.assertTrue(doSorting(1000000, 10));
   }
   
   
   public static void main(String args)
   {
      doSorting(1000000, 400);
   }
   
   
   private static boolean doSorting(int len, int max)
   {
      int[] array = new int[len];
      int[] rnd = new int[len];
      int[] copy = new int[len];
      java.util.Random random = new java.util.Random();
      long before, after;
      BucketSort bucketSort = new BucketSort(8);
      HeapSort heapSort = new HeapSort();
//       InsertionSort insertionSort = new InsertionSort();
      RadixSort radix4Sort = new RadixSort(4, 8); //radix 4
      RadixSort radix8Sort = new RadixSort(8, 8); //radix 8
      QuickSort quickSort = new QuickSort();
      FlashSort flashSort = new FlashSort();
      MergeSort mergeSort = new MergeSort();
      SpreadSort spreadSort = new SpreadSort();
      ForkJoinPool pool = new ForkJoinPool();
      ForkJoinParallelSort fjSort = new ForkJoinParallelSort(pool);

      try
      {
         long sum0  = 0;
         long sum1  = 0;
         long sum2  = 0;
         long sum3  = 0;
         long sum4  = 0;
         long sum5  = 0;
         long sum6  = 0;
         long sum7  = 0;
         long sum8  = 0;
         long sum9  = 0;
         long sum10 = 0;
         int iter = 1;

          for (int k=0; k<max; k++)
          {
              if (k % 50 == 0)
                 System.out.println("Iteration "+k+" of "+max);

              for (int i=0; i<rnd.length; i++)
                  rnd[i] = random.nextInt() & 0xFF;

              System.arraycopy(rnd, 0, copy, 0, rnd.length);

              for (int ii=0; ii<iter; ii++)
              {
                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  bucketSort.sort(array, 0, array.length);

                  if (ii == 0)
                     check("Bucket Sort", array);

                  after = System.nanoTime();
                  sum0 += (after - before);
                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  heapSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum1 += (after - before);

                  if (ii == 0)
                    check("Heap Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  radix4Sort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum3 += (after - before);

                  if (ii == 0)
                    check("Radix4 Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  radix8Sort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum4 += (after - before);

                  if (ii == 0)
                    check("Radix8 Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  quickSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum5 += (after - before);

                  if (ii == 0)
                    check("Quick Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  Arrays.sort(array);
                  after = System.nanoTime();
                  sum6 += (after - before);

                  if (ii == 0)
                    check("Arrays Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  flashSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum7 += (after - before);

                  if (ii == 0)
                    check("Flash Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  fjSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum8 += (after - before);

                  if (ii == 0)
                    check("Parallel Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  mergeSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum9 += (after - before);

                  if (ii == 0)
                    check("Merge Sort", array);  

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  spreadSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum10 += (after - before);

                  if (ii == 0)
                    check("Spread Sort", array);                
              }
          }

          System.out.println("\n\n -------------------------------------- \n");
          System.out.println("Speed test - byte values");
          System.out.println((iter*max)+" iterations\n");
          System.out.println("BucketSort      Elapsed [ms]: " + (sum0  / 1000000));
          System.out.println("HeapSort        Elapsed [ms]: " + (sum1  / 1000000));
          System.out.println("InsertionSort   Elapsed [ms]: " + "too slow, skipped");//(sum2  / 1000000));
          System.out.println("Radix4Sort      Elapsed [ms]: " + (sum3  / 1000000));
          System.out.println("Radix8Sort      Elapsed [ms]: " + (sum4  / 1000000));
          System.out.println("QuickSort       Elapsed [ms]: " + (sum5  / 1000000));
          System.out.println("Arrays.sort     Elapsed [ms]: " + (sum6  / 1000000));
          System.out.println("FlashSort       Elapsed [ms]: " + (sum7  / 1000000));
          System.out.println("MergeSort       Elapsed [ms]: " + (sum9  / 1000000));
          System.out.println("SpreadSort      Elapsed [ms]: " + (sum10 / 1000000));
          System.out.println("ParallelSort    Elapsed [ms]: " + (sum8  / 1000000));
          System.out.println("");

          sum2 = 0;
          sum3 = 0;
          sum5 = 0;
          sum6 = 0;
          sum7 = 0;
          sum8 = 0;
          sum9 = 0;

          RadixSort radixSort = new RadixSort(8); // radix 8

          for (int k=0; k<max; k++)
          {
              if (k % 50 == 0)
                 System.out.println("Iteration "+k+" of "+max);

               for (int i=0; i<rnd.length; i++)
                   rnd[i] = random.nextInt(Integer.MAX_VALUE) & -2;

               System.arraycopy(rnd, 0, copy, 0, rnd.length);

               // Validation test
               System.arraycopy(copy, 0, array, 0, array.length);
               radixSort.sort(array, 0, array.length);
               check("Radix8 Sort", array);
               System.arraycopy(copy, 0, array, 0, array.length);
               quickSort.sort(array, 0, array.length);
               check("Quick Sort", array);
               System.arraycopy(copy, 0, array, 0, array.length);
               flashSort.sort(array, 0, array.length);
               check("Flash Sort", array);
               System.arraycopy(copy, 0, array, 0, array.length);
               mergeSort.sort(array, 0, array.length);
               check("Merge Sort", array);
               System.arraycopy(copy, 0, array, 0, array.length);
               fjSort.sort(array, 0, array.length);
               check("Parallel Sort", array);
               System.arraycopy(copy, 0, array, 0, array.length);
               spreadSort.sort(array, 0, array.length);
               check("Spread Sort", array);

               for (int ii=0; ii<iter; ii++)
               {
                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  radixSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum3 += (after - before);

                  if (ii == 0)
                     check("Radix8 Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  quickSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum5 += (after - before);

                  if (ii == 0)
                     check("Quick Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  Arrays.sort(array);
                  after = System.nanoTime();
                  sum6 += (after - before);

                  if (ii == 0)
                     check("Arrays Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  flashSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum7 += (after - before);

                  if (ii == 0)
                     check("Flash Sort", array);                   

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  fjSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum8 += (after - before);

                  if (ii == 0)
                     check("Parallel Sort", array);

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  mergeSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum9 += (after - before);

                  if (ii == 0)
                     check("Merge Sort", array);  

                  System.arraycopy(copy, 0, array, 0, array.length);
                  before = System.nanoTime();
                  spreadSort.sort(array, 0, array.length);
                  after = System.nanoTime();
                  sum2 += (after - before);

                  if (ii == 0)
                     check("Spread Sort", array);   
              }
         }

        System.out.println("\n\n -------------------------------------- \n");
        System.out.println("Speed test - integer values\n");
        System.out.println((iter*max)+" iterations\n");
        System.out.println("Radix8Sort      Elapsed [ms]: " + (sum3  / 1000000));
        System.out.println("QuickSort       Elapsed [ms]: " + (sum5  / 1000000));
        System.out.println("Arrays.sort     Elapsed [ms]: " + (sum6  / 1000000));
        System.out.println("FlashSort       Elapsed [ms]: " + (sum7  / 1000000));
        System.out.println("MergeSort       Elapsed [ms]: " + (sum9  / 1000000));
        System.out.println("SpreadSort      Elapsed [ms]: " + (sum2  / 1000000));
        System.out.println("ParallelSort    Elapsed [ms]: " + (sum8  / 1000000));
      
      }
      catch (Exception e)
      {
         e.printStackTrace();
         return false;
      }
      finally 
      {
         pool.shutdown();      
      }

      return true;
   }


   private static void check(String name, int[] array) 
   {
      for (int i=1; i<array.length; i++) 
      {
         if (array[i] < array[i-1]) 
         {
            for (int j=0; j<=i; j++)
                System.out.print(array[j] + " ");

            System.out.println("");
            throw new RuntimeException("Check for '" + name + "' failed at index " + i);
         }
      }
   }
}
