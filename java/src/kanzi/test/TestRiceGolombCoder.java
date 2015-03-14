/*
Copyright 2011-2013 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
you may obtain a copy of the License at

                http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either Riceress or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kanzi.test;

import kanzi.bitstream.DebugInputBitStream;
import kanzi.entropy.RiceGolombDecoder;
import kanzi.entropy.RiceGolombEncoder;
import java.io.BufferedInputStream;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.util.Random;
import kanzi.InputBitStream;
import kanzi.OutputBitStream;
import kanzi.bitstream.DebugOutputBitStream;
import kanzi.bitstream.DefaultInputBitStream;
import kanzi.bitstream.DefaultOutputBitStream;


public class TestRiceGolombCoder
{

    public static void main(String[] args)
    {
        System.out.println("TestRiceGolombCoder");

        // Test behavior
        for (int ii=1; ii<20; ii++)
        {
            try
            {
                byte[] values;
                Random rnd = new Random();
                System.out.println("\nTest "+ii);

                if (ii == 1)
                   values = new byte[] { -13, -3, -15, -11, 12, -14, -11, 15, 7, 9, 5, -7, 4, 3, 15, -12  };
                else
                {
                   values = new byte[32];

                   for (int i=0; i<values.length; i++)
                      values[i] = (byte) (rnd.nextInt(32) - 16*(ii&1));
                }

                ByteArrayOutputStream os = new ByteArrayOutputStream(16384);
                OutputBitStream bs = new DefaultOutputBitStream(os, 16384);
                DebugOutputBitStream dbgbs = new DebugOutputBitStream(bs, System.out, -1);

                // Alternate signed / unsigned coding
                RiceGolombEncoder gc = new RiceGolombEncoder(dbgbs, (ii&1)==1, 1+ii%6);

                for (int i=0; i<values.length; i++)
                {
                    System.out.print(values[i]+" ");
                }

                System.out.println();
                gc.encode(values, 0, values.length);    
                gc.dispose();
                bs.close();
                byte[] array = os.toByteArray();
                BufferedInputStream is = new BufferedInputStream(new ByteArrayInputStream(array));
                InputBitStream bs2 = new DefaultInputBitStream(is, 16384);
                DebugInputBitStream dbgbs2 = new DebugInputBitStream(bs2, System.out, -1);
                dbgbs2.setMark(true);
                RiceGolombDecoder gd = new RiceGolombDecoder(dbgbs2, (ii&1)==1, 1+ii%6);
                byte[] values2 = new byte[values.length];
                System.out.println("\nDecoded:");
                gd.decode(values2, 0, values2.length);
                System.out.println();
                gd.dispose();
                dbgbs2.close();
                boolean ok = true;

                for (int i=0; i<values.length; i++)
                {
                    System.out.print(values2[i]+" ");

                    if (values2[i] != values[i])
                    {
                       ok = false;
                       break;
                    }
                }

                System.out.println();
                System.out.println((ok) ? "Identical" : "Different");
                
                if (ok == false)
                   System.exit(1);
            }
            catch (Exception e)
            {
                e.printStackTrace();
            }
        }

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
                ByteArrayOutputStream baos = new ByteArrayOutputStream(size*2);
                OutputBitStream os = new DefaultOutputBitStream(baos, size);
                RiceGolombEncoder gc = new RiceGolombEncoder(os, true, 3);
                long before1 = System.nanoTime();
                
                if (gc.encode(values1, 0, values1.length) < 0)
                {
                   System.out.println("Encoding error");
                   System.exit(1);
                }
                   
                long after1 = System.nanoTime();
                delta1 += (after1 - before1);
                gc.dispose();
                os.close();

                // Decode
                byte[] buf = baos.toByteArray();
                InputBitStream is = new DefaultInputBitStream(new ByteArrayInputStream(buf), size);
                RiceGolombDecoder gd = new RiceGolombDecoder(is, true, 3);
                long before2 = System.nanoTime();
                
                if (gd.decode(values2, 0, values2.length) < 0)
                {
                  System.out.println("Decoding error");
                  System.exit(1);
                }

                long after2 = System.nanoTime();
                delta2 += (after2 - before2);
                gd.dispose();
                is.close();

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
            System.out.println("Encode [ms]       : " +delta1/1000000);
            System.out.println("Throughput [KB/s] : " + prod * 1000000L / delta1 * 1000L / 1024L);
            System.out.println("Decode [ms]       : " +delta2/1000000);
            System.out.println("Throughput [KB/s] : " + prod * 1000000L / delta2 * 1000L / 1024L);
        }
    }
}
