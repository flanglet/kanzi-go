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

package kanzi.test;

import java.util.Random;
import kanzi.IndexedByteArray;
import kanzi.transform.BWT;


public class TestBWT
{
    public static void main(String[] args)
    {
        System.out.println("TestBWT");
        testCorrectness();
        testSpeed();
    }
    
    
    public static void testCorrectness()
    {
        System.out.println("\nCorrectness test");

        // Test behavior
        for (int ii=1; ii<=20; ii++)
        {
            System.out.println("\nTest "+ii);
            int start = 0;
            int size = 0;
            byte[] buf1;
            Random rnd = new Random();

            if (ii == 1)
            {
               size = 0;
               buf1 = "mississippi".getBytes();
            }
            else if (ii == 2)
            {
               buf1 = "3.14159265358979323846264338327950288419716939937510".getBytes();
            }
            else
            {
               size = 128;
               buf1 = new byte[size];

               for (int i=0; i<buf1.length; i++)
               {
                   buf1[i] = (byte) (65 + rnd.nextInt(4*ii));
               }

               buf1[buf1.length-1] = (byte) 0;
            }

            byte[] buf2 = new byte[buf1.length];
            byte[] buf3 = new byte[buf1.length];
            IndexedByteArray iba1 = new IndexedByteArray(buf1, 0);
            IndexedByteArray iba2 = new IndexedByteArray(buf2, 0);
            IndexedByteArray iba3 = new IndexedByteArray(buf3, 0);
            BWT bwt = new BWT(size);
            String str1 = new String(buf1, start, buf1.length-start);
            System.out.println("Input:   "+str1);
            iba1.index = start;
            iba2.index = 0;
            bwt.forward(iba1, iba2);
            int primaryIndex = bwt.getPrimaryIndex();
            String str2 = new String(buf2);
            System.out.print("Encoded: "+str2);
            System.out.println("  (Primary index="+primaryIndex+")");
            bwt.setPrimaryIndex(primaryIndex);
            iba2.index = 0;
            iba3.index = start;
            bwt.inverse(iba2, iba3);
            String str3 = new String(buf3, start, buf3.length-start);
            System.out.println("Output:  "+str3);

            if (str1.equals(str3) == true)
               System.out.println("Identical");
            else
            {
               System.out.println("Different");
               System.exit(1);
            }
        }
    }
    
    
    public static void testSpeed()
    {
         System.out.println("\nSpeed test");
         int iter = 2000;
         int size = 256*1024;
         byte[] buf1 = new byte[size];
         byte[] buf2 = new byte[size];
         byte[] buf3 = new byte[size];
         IndexedByteArray iba1 = new IndexedByteArray(buf1, 0);
         IndexedByteArray iba2 = new IndexedByteArray(buf2, 0);
         IndexedByteArray iba3 = new IndexedByteArray(buf3, 0);
         System.out.println("Iterations: "+iter);
         System.out.println("Transform size: "+size);

         for (int jj = 0; jj < 3; jj++)
         {
             long delta1 = 0;
             long delta2 = 0;
             BWT bwt = new BWT(size);
             java.util.Random random = new java.util.Random();
             long before, after;

             for (int ii = 0; ii < iter; ii++)
             {
                 for (int i = 0; i < size; i++)
                     buf1[i] = (byte) (random.nextInt(255) + 1);

                 buf1[size-1] = 0;
                 before = System.nanoTime();
                 iba1.index = 0;
                 iba2.index = 0;
                 bwt.forward(iba1, iba2);
                 after = System.nanoTime();
                 delta1 += (after - before);
                 before = System.nanoTime();
                 iba2.index = 0;
                 iba3.index = 0;
                 bwt.inverse(iba2, iba3);
                 after = System.nanoTime();
                 delta2 += (after - before);
             
               int idx = -1;

               // Sanity check
               for (int i=0; i<size; i++)
               {
                  if (buf1[i] != buf3[i])
                  {
                     idx = i;
                     break;
                  }
               }

               if (idx >= 0)
                  System.out.println("Failure at index "+idx+" ("+buf1[idx]+"<->"+buf3[idx]+")");             
             }

            final long prod = (long) iter * (long) size;
            System.out.println("Forward transform [ms] : " + delta1 / 1000000);
            System.out.println("Throughput [KB/s]      : " + prod * 1000000L / delta1 * 1000L / 1024);
            System.out.println("Inverse transform [ms] : " + delta2 / 1000000);
            System.out.println("Throughput [KB/s]      : " + prod * 1000000L / delta2 * 1000L / 1024);
         }
    }
}
