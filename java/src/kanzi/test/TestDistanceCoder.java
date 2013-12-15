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

import java.util.Arrays;
import java.util.Random;
import kanzi.IndexedByteArray;
import kanzi.function.DistanceCodec;


public class TestDistanceCoder
{

    public static void main(String[] args)
    {
       System.out.println("TestDistanceCoder");
       testCorrectness();
       testSpeed();
    }
    
    
    public static void testCorrectness()
    {
        System.out.println("Correctness test");
        byte[] input;
        byte[] values = new byte[4096];
        Random rnd =  new Random();
        values[0] = (byte) 64;
        
        for (int i=1; i<values.length; i++)
           values[i] = (byte) (values[i-1] + rnd.nextInt(5) - 3);
        
        for (int ii=0; ii<3; ii++)
        {
            if (ii == 1)
              input = "rccrsaaaa".getBytes();
            else if (ii == 2)
              input = "abdbcrraaaa".getBytes();
            else
               input = values;

            int length = input.length;
            IndexedByteArray src = new IndexedByteArray(new byte[length], 0);
            IndexedByteArray dst = new IndexedByteArray(new byte[Math.max(length, 256)], 0); // can be bigger than original !
            IndexedByteArray inv = new IndexedByteArray(new byte[length], 0);
            System.arraycopy(input, 0, src.array, 0, length);
            Arrays.fill(dst.array, (byte) 0xAA);
            System.out.println("\nSource");

            for (int i=0; i<length; i++)
                System.out.print((src.array[i] & 0xFF)+" ("+i+") ");

            System.out.println();
            DistanceCodec codec = new DistanceCodec(length);

            if (codec.forward(src, dst) == false)
            {
                System.err.println("Failure in forward() call !");
                System.exit(1);
            }

            System.out.println("\nDestination");

            System.out.println("header:");
            System.out.print("[ ");
            int alphabetSize = dst.array[0] & 0xFF;
            int headerSize = 1 + alphabetSize;

            for (int i=0; i<headerSize; i++)
                System.out.print((dst.array[i] & 0xFF)+" ("+i+") ");

            System.out.println("]");
            System.out.println("body:");
            System.out.print("[ ");

            for (int i=headerSize; i<dst.index; i++)
                System.out.print((dst.array[i] & 0xFF)+" ("+i+") ");

            System.out.println("]");
            System.out.println();
            System.out.println("Source size: "+src.index);
            System.out.println("Destination size: "+dst.index);

            codec.setSize(dst.index);
            src.index = 0;
            dst.index = 0;

            if (codec.inverse(dst, inv) == false)
            {
                System.err.println("Failure in inverse() call !");
                System.exit(1);
            }

            System.out.println("\nInverse");

            for (int i=0; i<length; i++)
            {
                System.out.print((inv.array[i] & 0xFF)+" ("+i+") ");

                if (inv.array[i] != src.array[i])
                {
                    System.err.println("Error at index "+i+" : "+src.array[i]+" <=> "+inv.array[i]);
                    System.exit(1);
                }
            }

            System.out.println("\nIdentical");
        }
    }
    
    
    public static void testSpeed()
    {
        // Speed test  
        System.out.println("\n\nSpeed test\n");
        final int size = 10000;
        byte[] input = new byte[size];
        byte[] values = new byte[size];

        Random rnd =  new Random();
        values[0] = (byte) 64;
        
        for (int i=1; i<values.length; i++)
           values[i] = (byte) (values[i-1] + rnd.nextInt(5) - 3);
         
        IndexedByteArray src = new IndexedByteArray(new byte[size], 0);
        IndexedByteArray dst = new IndexedByteArray(new byte[size], 0);
        IndexedByteArray inv = new IndexedByteArray(new byte[size], 0);
        System.arraycopy(input, 0, src.array, 0, size);
        DistanceCodec codec = new DistanceCodec(size);
        long before, after;
        int iter = 20000;
        long delta1 = 0;
        long delta2 = 0;

        for (int ii=0; ii<iter; ii++)
        {
            before = System.nanoTime();
            src.index = 0;
            dst.index = 0;
            codec.setSize(size);
            
            if (codec.forward(src, dst) == false)
            {
               System.out.println("Encoding error");
               System.exit(1);
            }

            after = System.nanoTime();
            delta1 += (after-before);
            before = System.nanoTime();
            codec.setSize(dst.index);
            dst.index = 0;
            inv.index = 0;
            
            if (codec.inverse(dst, inv) == false)
            {
               System.out.println("Decoding error");
               System.exit(1);
            }

            after = System.nanoTime();
            delta2 += (after-before);
        }
        

        final long prod = (long) iter * (long) size;
        System.out.println("Encode [ms]       : " + delta1/1000000L);
        System.out.println("Throughput [KB/s] : " + prod * 1000000L / delta1 * 1000L / 1024L);
        System.out.println("Decode [ms]       : " + delta2/1000000L);
        System.out.println("Throughput [KB/s] : " + prod * 1000000L / delta2 * 1000L / 1024L);
    }
}
