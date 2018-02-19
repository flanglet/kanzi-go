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
 limit
*/
package kanzi.test;

import java.util.Arrays;
import java.util.Random;
import kanzi.util.IntBTree;
import org.junit.Test;


public class TestIntBTree
{
   @Test
   public void testIntBTree()
   {
      System.out.println("Correctness Test");

      {
         Random random = new Random();
         IntBTree tree = new IntBTree();

         for (int ii=0; ii<5; ii++)
         {
            System.out.println("\nIteration "+ii);
            int max = 0;
            int min = Integer.MAX_VALUE;
            int[] array = new int[30];

            for (int i=0; i<array.length; i++)
               array[i] = 64+random.nextInt(5*i+20);

            for (int i=0; i<array.length; i++)
            {
               int val = array[i];
               min = (min < val) ? min : val;
               max = (max > val) ? max : val;
               tree.add(val);
               System.out.println("Add: "+val);
               System.out.print("Min/max: "+tree.min()+" "+tree.max());
               System.out.println(" Size: "+tree.size());

               if (tree.min() != min)
               {
                  System.err.println("Error: Found min="+tree.min()+", expected min="+min);
                  System.exit(1);
               }

               if (tree.max() != max)
               {
                  System.err.println("Error: Found max="+tree.max()+", expected max="+max);
                  System.exit(1);
               }
            }

     //       System.out.print("All nodes in reverse order: ");
     //       tree.scan(printNode, true);
            System.out.println("");
            int[] res = tree.toArray(new int[tree.size()]);
            System.out.print("All nodes in natural order: ");
            System.out.println(Arrays.toString(res));
            System.out.println("");

            for (int i=0; i<array.length/2; i++)
            {
               int r = tree.rank(array[i]);
               System.out.println("Rank "+array[i]+": "+r);
            }
            
            while (tree.size() > 0)
            {
               min = tree.min();
               max = tree.max();
               System.out.println("Remove: " + min + " " + max);
               tree.remove(min);
               tree.remove(max);
               res = tree.toArray(new int[tree.size()]);
               System.out.println(Arrays.toString(res));
               System.out.println(" Size: " +tree.size());

               if (tree.size() > 0)
               {
                  min = tree.min();
                  max = tree.max();
                  System.out.println("Min/max: " + min + " " + max);
               }
            }

            System.out.println("Success\n");
         }
      }

      System.out.println("Speed Test");

      try
      {
         int iter = 5000;
         int size = 10000;
         long before, after;
         long delta01 = 0;
         long delta02 = 0;
         long delta03 = 0;
         long delta04 = 0;
         int[] array = new int[size];

         Random random = new Random();

         for (int ii=0; ii<iter; ii++)
         {
            IntBTree tree = new IntBTree();
            array[0] = 100000;

            for (int i=1; i<size; i++)
              array[i] = random.nextInt(200000);

            tree.clear();
            before = System.nanoTime();

            for (int i=0; i<size; i++)
               tree.add(array[i]);

            after = System.nanoTime();
            delta01 += (after - before);
            before = System.nanoTime();

            for (int i=0; i<size; i++)
               tree.contains(array[size-1-i]);

            after = System.nanoTime();
            delta04 += (after - before);

            // Sanity check
            if (tree.size() != size)
            {
               System.err.println("Error: found size="+tree.size()+", expected size="+size);
               System.exit(1);
            }

            before = System.nanoTime();

            for (int i=0; i<size; i++)
            {
              if (tree.remove(array[i]) == false)
              {
                 System.err.println("Failed to remove: "+array[i]);
                 System.exit(1);
              }
            }

            after = System.nanoTime();
            delta02 += (after - before);

            // Sanity check
            if (tree.size() != 0)
            {
               System.err.println("Error: found size="+tree.size()+", expected size="+0);
               System.exit(1);
            }

            // Recreate a 'size' array tree
            for (int i=0; i<size; i++)
               tree.add(array[i]);

            for (int i=0; i<size; i++)
            {
               int val = random.nextInt(size);
               before = System.nanoTime();
               tree.add(val);
               tree.remove(val);
               after = System.nanoTime();
               delta03 += (after - before);
            }

            // Sanity check
            if (tree.size() != size)
            {
               System.err.println("Error: found size="+tree.size()+", expected size="+size);
               System.exit(1);
            }
         }

         System.out.println(size*iter+" iterations");
         System.out.println("Additions [ms]: "+(delta01/1000000L));
         System.out.println("Deletions [ms]: "+(delta02/1000000L));
         System.out.println("Contains  [ms]: "+(delta04/1000000L));
         System.out.println("Additions/Deletions at size="+size+" [ms]: "+(delta03/1000000L));
      }
      catch (Exception e)
      {
         e.printStackTrace();
      }
   }
}
