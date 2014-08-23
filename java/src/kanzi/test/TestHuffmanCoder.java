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

import kanzi.bitstream.DebugInputBitStream;
import kanzi.entropy.HuffmanDecoder;
import kanzi.entropy.HuffmanEncoder;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import kanzi.InputBitStream;
import kanzi.OutputBitStream;
import kanzi.bitstream.DebugOutputBitStream;
import kanzi.bitstream.DefaultInputBitStream;
import kanzi.bitstream.DefaultOutputBitStream;


public class TestHuffmanCoder
{
    public static void main(String[] args)
    {
        System.out.println("TestHuffmanCoder");
        testCorrectness();
        testSpeed();
    }
    
    
    public static void testCorrectness()
    {
        // Test behavior
        System.out.println("Correctness test");

        for (int ii=1; ii<20; ii++)
        {
            System.out.println("\n\nTest "+ii);
            
            try
            {
                byte[] values;
                java.util.Random random = new java.util.Random();

                if (ii == 3)
                     values = new byte[] { 0, 0, 32, 15, -4, 16, 0, 16, 0, 7, -1, -4, -32, 0, 31, -1 };
                else if (ii == 2)
                     values = new byte[] { 0x3d, 0x4d, 0x54, 0x47, 0x5a, 0x36, 0x39, 0x26, 0x72, 0x6f, 0x6c, 0x65, 0x3d, 0x70, 0x72, 0x65 };
                else if (ii == 4)
                     values = new byte[] { 65, 71, 74, 66, 76, 65, 69, 77, 74, 79, 68, 75, 73, 72, 77, 68, 78, 65, 79, 79, 78, 66, 77, 71, 64, 70, 74, 77, 64, 67, 71, 64 };
                else if (ii == 1)
                {
                     values = new byte[32];

                     for (int i=0; i<values.length; i++)
                          values[i] = 2; // all identical
                }
                else if (ii == 5)
                {
                     values = new byte[70];

                     for (int i=0; i<values.length; i++)
                          values[i] = (byte) (2 + (i&1)); // 2 symbols
                }
                else
                {
                     values = new byte[70];

                     for (int i=0; i<values.length; i++)
                          values[i] = (byte) (random.nextInt() & 31);
                }

                System.out.println("Original:");

                for (int i=0; i<values.length; i++)
                    System.out.print(values[i]+" ");

                System.out.println("\nEncoded:");
                ByteArrayOutputStream os = new ByteArrayOutputStream(16384);
                OutputBitStream bs = new DefaultOutputBitStream(os, 16384);
                DebugOutputBitStream dbgbs = new DebugOutputBitStream(bs, System.out);
                dbgbs.showByte(true);
                dbgbs.setMark(true);
                int[] freq = new int[256];

                for (int i=0; i<values.length; i++)
                    freq[values[i] & 255]++;

                HuffmanEncoder rc = new HuffmanEncoder(dbgbs);
                rc.encode(values, 0, values.length);
                rc.dispose();
                dbgbs.close();
                System.out.println();
                byte[] buf = os.toByteArray();
                InputBitStream bs2 = new DefaultInputBitStream(new ByteArrayInputStream(buf), 16384);
                DebugInputBitStream dbgbs2 = new DebugInputBitStream(bs2, System.out);
                dbgbs2.setMark(true);
                HuffmanDecoder rd = new HuffmanDecoder(dbgbs2);
                System.out.println("\nDecoded:");
                int len = values.length; // buf.length >> 3;
                boolean ok = true;
                byte[] values2 = new byte[values.length];
                rd.decode(values2, 0, values.length);

                for (int j=0; j<len; j++)
                {
                   if (values2[j] != values[j])
                   {
                      ok = false;
                      break;
                   }
                }

                System.out.println();

                for (int i=0; i<values2.length; i++)
                    System.out.print(values2[i]+" ");

                System.out.println("\n"+((ok == true) ? "Identical" : "! *** Different *** !"));
                rd.dispose();
            }
            catch (Exception e)
            {
                e.printStackTrace();
            }
        }
     }
    

     public static void testSpeed()
     {
        // Test speed
        System.out.println("\n\nSpeed Test");
        int[] repeats = { 3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3 };
        final int iter = 4000;
        final int size = 50000;
        
        for (int jj=0; jj<3; jj++)
        {
            System.out.println("\nTest "+(jj+1));
            byte[] values1 = new byte[size];
            byte[] values2 = new byte[size];
            long delta1 = 0, delta2 = 0;

            for (int ii=0; ii<iter; ii++)
            {
                int idx = 0;

                for (int i=0; i<values1.length; i++)
                {
                    int i0 = i;
                    int len = repeats[idx];
                    idx = (idx + 1) & 0x0F;

                    if (i0+len >= values1.length)
                        len = 1;

                    for (int j=i0; j<i0+len; j++)
                    {
                       values1[j] = (byte) (i0 & 255);
                       i++;
                    }
                }

                // Encode
                ByteArrayOutputStream os = new ByteArrayOutputStream(size*2);
                OutputBitStream bs = new DefaultOutputBitStream(os, size);
                HuffmanEncoder rc = new HuffmanEncoder(bs);
                long before1 = System.nanoTime();
                
                if (rc.encode(values1, 0, values1.length) < 0)
                {
                   System.out.println("Encoding error");
                   System.exit(1);
                }

                long after1 = System.nanoTime();
                delta1 += (after1 - before1);
                rc.dispose();
                bs.close();

                // Decode
                byte[] buf = os.toByteArray();
                InputBitStream bs2 = new DefaultInputBitStream(new ByteArrayInputStream(buf), size);
                HuffmanDecoder rd = new HuffmanDecoder(bs2);
                long before2 = System.nanoTime();
                
                if (rd.decode(values2, 0, values2.length) < 0)
                {
                   System.out.println("Decoding error");
                   System.exit(1);
                }
                   
                long after2 = System.nanoTime();
                delta2 += (after2 - before2);
                rd.dispose();

                // Sanity check
                for (int i=0; i<values1.length; i++)
                {
                   if (values1[i] != values2[i])
                   {
                      System.out.println("Error at index "+i+" ("+values1[i]
                              +"<->"+values2[i]+")");
                      break;
                   }
                }
            }

            final long prod = (long) iter * (long) size;
            System.out.println("Encode [ms]       : " + delta1/1000000);
            System.out.println("Throughput [KB/s] : " + prod * 1000000L / delta1 * 1000L / 1024L);
            System.out.println("Decode [ms]       : " + delta2/1000000);
            System.out.println("Throughput [KB/s] : " + prod * 1000000L / delta2 * 1000L / 1024L);
        }
    }
}
