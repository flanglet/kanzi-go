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

import java.util.Random;
import kanzi.IntSorter;


public class TestAbstractSort 
{  
    public static void testCorrectness(String sortName, IntSorter sorter, int iters)
    {
        System.out.println("\n\nTest" + sortName);

        // Test behavior
        for (int ii=1; ii<=iters; ii++)
        {
            System.out.println("\n\nCorrectness test "+ii);
            final int[] array = new int[64];
            Random random = new Random();

            for (int i=ii; i<array.length; i++)
                array[i] = 64 + (random.nextInt(ii*8));

            byte[] b = new byte[array.length];

            for (int i=ii; i<b.length; i++)
                b[i] = (byte) array[i];

            System.out.println(new String(b));

            for (int i=ii; i<array.length; i++)
                System.out.print(array[i]+" ");

            System.out.println();
            sorter.sort(array, ii, array.length-ii);

            for (int i=ii; i<b.length; i++)
                b[i] = (byte) (array[i] & 255);

            System.out.println(new String(b));

            for (int i=ii; i<array.length; i++)
            {
                System.out.print(array[i]+" ");
                
                if ((i > 0) && (array[i] < array[i-1]))
                {
                   System.err.println("Error at position "+(i-ii));
                   System.exit(1);
                }
            }
        }
    }

  
    public static void testSpeed(String sortName, IntSorter sorter, int iters)
    {
       testSpeed(sortName, sorter, iters, -1);
    }
    
    
    public static void testSpeed(String sortName, IntSorter sorter, int iters, int mask)
    {
        System.out.println("\n\nSpeed test");
        System.out.println(iters+" iterations");
        int[] array = new int[10000];
        int[] array2 = new int[10000];
        Random random = new Random();
        long before, after;
        int[] vals = { 0xFF, 0xFFFF, 0xFFFFFF, 0x7FFFFFFF, Integer.MAX_VALUE };
        String empty = "                   ";
        int idx2 = sortName.length() - "arrays.sort".length();
        String adjust2 = (idx2 > 0 ) ? empty.substring(0, idx2) : "";
        int idx1 = -idx2;
        String adjust1 = (idx1 > 0 ) ? empty.substring(0, idx1) : "";

        for (int k=0; k<5; k++)
        {
            long sum = 0;
            long sum2 = 0;
            final int val = vals[k] & mask;

            for (int ii=0; ii<iters; ii++)
            {
                for (int i=0; i<array.length; i++)
                    array[i] = random.nextInt(val);

                System.arraycopy(array, 0, array2, 0, array.length);

                before = System.nanoTime();
                sorter.sort(array, 0, array.length);
                after = System.nanoTime();
                sum += (after - before);
                before = System.nanoTime();
                java.util.Arrays.sort(array2);
                after = System.nanoTime();
                sum2 += (after - before);
             }

             System.out.println("Elapsed "+sortName+adjust1+" [ms]: "+sum/1000000);
             System.out.println("Elapsed arrays.sort"+adjust2+" [ms]: "+sum2/1000000);
         }
    }
   
}
