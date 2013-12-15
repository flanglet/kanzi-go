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
 limit
*/
package kanzi.test;

import java.util.Random;
import kanzi.util.IntBTree;
import kanzi.util.IntBTree.IntBTNode;


public class TestIntBTree 
{
   public static void main(String[] args)
   {
      System.out.println("Correctness Test");
      
      {
         Random random = new Random();
         IntBTree tree = new IntBTree();

         IntBTree.Callback printNode = new IntBTree.Callback()
         {
            @Override
            public void call(IntBTNode node) 
            {
               System.out.print(node.value()+", ");
            }        
         };

         for (int ii=0; ii<5; ii++)
         {
            System.out.println("\nIteration "+ii);
            int max = 0;
            int min = Integer.MAX_VALUE;

            for (int i=0; i<30; i++)
            {
               int val = 64+random.nextInt(5*i+20);
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

            System.out.print("All nodes in reverse order: ");
            tree.scan(printNode, true);            System.out.println("");


            while (tree.size() > 0)
            {
               min = tree.min();
               max = tree.max();
               tree.remove(min);
               tree.remove(max);
               System.out.print("Remove: " + min + " " + max);
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
      
      {
         int iter = 10000;
         int size = 10000;
         long before1, after1;
         long before2, after2;
         long before3, after3;
         long delta1 = 0;
         long delta2 = 0;
         long delta3 = 0;
         int[] array = new int[size];
         
         Random random = new Random();
         
         for (int ii=0; ii<iter; ii++)
         {
            IntBTree tree = new IntBTree();
            
            for (int i=0; i<size; i++)
              array[i] = random.nextInt(size/2);
             
            before1 = System.nanoTime();

            for (int i=0; i<size; i++)
               tree.add(array[i]);
            
            after1 = System.nanoTime();
            delta1 += (after1 - before1);
            
            // Sanity check
            if (tree.size() != size)
            {
               System.err.println("Error: found size="+tree.size()+", expected size="+size);
               System.exit(1);
            }
            
            before2 = System.nanoTime();

            for (int i=0; i<size; i++)
               tree.remove(array[i]);
            
            after2 = System.nanoTime();
            delta2 += (after2 - before2);

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
               int val = random.nextInt(size/2);
               before3 = System.nanoTime();
               tree.add(val);
               tree.remove(val);
               after3 = System.nanoTime();
               delta3 += (after3 - before3);
            }

            // Sanity check
            if (tree.size() != size)
            {
               System.err.println("Error: found size="+tree.size()+", expected size="+size);
               System.exit(1);
            }
         }

         System.out.println(size*iter+" iterations");
         System.out.println("Additions [ms]: "+(delta1/1000000L));
         System.out.println("Deletions [ms]: "+(delta2/1000000L));
         System.out.println("Additions/Deletions at size="+size+" [ms]: "+(delta3/1000000L));
      }      
   }   
}
