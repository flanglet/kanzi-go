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
import kanzi.transform.MTFT;


public class TestMTFT
{
    public static void main(String[] args)
    {
        System.out.println("TestMTFT");

        // Behavior Test
        {
            System.out.println("\nCorrectness test");

            for (int ii=1; ii<=20; ii++)
            {
                byte[] input;
                
                if (ii == 1)
                {
                    input = new byte[] { 5, 2, 4, 7, 0, 0, 7, 1, 7 };
                }
                else
                {
                    input = new byte[32];
                    Random rnd = new Random();

                    for (int i=0; i<input.length; i++)
                    {
                        input[i] = (byte) (65 + rnd.nextInt(5*ii));
                    }
                }

                int size = input.length;
                MTFT mtft = new MTFT(size);
                byte[] transform = new byte[size+20];
                byte[] reverse = new byte[size];
                IndexedByteArray iba1 = new IndexedByteArray(input, 0);
                IndexedByteArray iba2 = new IndexedByteArray(transform, 0);
                IndexedByteArray iba3 = new IndexedByteArray(reverse, 0);

                System.out.println("\nTest "+ii);
                System.out.print("Input     : ");

                for (int i=0; i<size; i++)
                {
                    System.out.print((input[i] & 0xFF) + " ");
                }

                int start = (ii & 1) * ii;
                iba2.index = start;
                mtft.forward(iba1, iba2);
                System.out.println();
                System.out.print("Transform : ");

                for (int i=start; i<start+size; i++)
                {
                    System.out.print((transform[i] & 0xFF) + " ");
                }

                iba2.index = start;
                mtft.inverse(iba2, iba3);
                System.out.println();
                System.out.print("Reverse   : ");

                for (int i = 0; i < size; i++)
                {
                    System.out.print((reverse[i] & 0xFF) + " ");
                }

                System.out.println();
                boolean ok = true;

                for (int i=0; i<size; i++)
                {
                    if (reverse[i] != input[i])
                    {
                        ok = false;
                        break;
                    }
                }

                System.out.println((ok == true) ? "Identical" : "Different");

                if (ok == false)
                  System.exit(1);
            }
        }

      // Speed Test
      final int iter = 20000;
      final int size = 10000;
      System.out.println("\n\nSpeed test");
      System.out.println("Iterations: "+iter);
     
      for (int jj=0; jj<4; jj++)
      {     
            byte[] input = new byte[size];
            byte[] transform = new byte[size];
            byte[] reverse = new byte[size];
            IndexedByteArray iba1 = new IndexedByteArray(input, 0);
            IndexedByteArray iba2 = new IndexedByteArray(transform, 0);
            IndexedByteArray iba3 = new IndexedByteArray(reverse, 0);
            MTFT mtft = new MTFT();
            long delta1 = 0, delta2 = 0;
            long before, after;

            if (jj == 0)
               System.out.println("\nPurely random input");
            
            if (jj == 2)
               System.out.println("\nSemi random input");
               
            for (int ii = 0; ii < iter; ii++)
            {
                Random rnd = new Random();
                int n = 128;
                
                for (int i = 0; i < input.length; i++)
                {
                    if (jj < 2)
                    {
                       // Pure random
                       input[i] = (byte) (rnd.nextInt(256));
                    }
                    else
                    {
                       // Semi random (a bit more realistic input)
                       int rng = ((i & 7) == 0) ? 256 : 5;
                       int p = (rnd.nextInt(rng) - rng/2 + n) & 0xFF;
                       input[i] = (byte) p;
                       n = p;
                    }
                }

                before = System.nanoTime();
                iba1.index = 0;
                iba2.index = 0;
                mtft.forward(iba1, iba2);
                after = System.nanoTime();
                delta1 += (after - before);
                before = System.nanoTime();
                iba2.index = 0;
                iba3.index = 0;
                mtft.inverse(iba2, iba3);
                after = System.nanoTime();
                delta2 += (after - before);

                int idx = -1;

                // Sanity check
                for (int i=0; i<size; i++)
                {
                   if (input[i] != reverse[i])
                   {
                      idx = i;
                      break;
                   } 
                }

                if (idx >= 0)
                   System.out.println("Failure at index "+idx+" ("+input[idx]+"<->"+reverse[idx]+")");
            }

            final long prod = (long) iter * (long) size;
            System.out.println("MTFT Forward transform [ms]: " + delta1 / 1000000);
            System.out.println("Throughput [KB/s]          : " + prod * 1000000L / delta1 * 1000L / 1024);
            System.out.println("MTFT Reverse transform [ms]: " + delta2 / 1000000);
            System.out.println("Throughput [KB/s]          : " + prod * 1000000L / delta2 * 1000L / 1024);        
            System.out.println();
        }
    }
} 
