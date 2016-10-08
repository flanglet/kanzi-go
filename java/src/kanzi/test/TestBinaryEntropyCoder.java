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

import kanzi.BitStreamException;
import kanzi.entropy.BinaryEntropyDecoder;
import kanzi.entropy.BinaryEntropyEncoder;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.util.Random;
import kanzi.InputBitStream;
import kanzi.OutputBitStream;
import kanzi.bitstream.DebugOutputBitStream;
import kanzi.bitstream.DefaultInputBitStream;
import kanzi.bitstream.DefaultOutputBitStream;
import kanzi.entropy.CMPredictor;
import kanzi.entropy.FPAQPredictor;
import kanzi.entropy.PAQPredictor;
import kanzi.entropy.Predictor;
import kanzi.entropy.TPAQPredictor;


public class TestBinaryEntropyCoder
{
    public static void main(String[] args)
    {
       if (args.length == 0)
       {
          args = new String[] { "-TYPE=ALL" };
       }

       String type = args[0].toUpperCase();

       if (type.startsWith("-TYPE=")) 
       {
           type = type.substring(6);

           if (type.equals("ALL"))
           {
              System.out.println("\n\nTestFPAQEntropyCoder");
              testCorrectness("FPAQ");
              testSpeed("FPAQ");
              System.out.println("\n\nTestCMEntropyCoder");
              testCorrectness("CM");
              testSpeed("CM");
              System.out.println("\n\nTestPAQEntropyCoder");
              testCorrectness("PAQ");
              testSpeed("PAQ");
              System.out.println("\n\nTestTPAQEntropyCoder");
              testCorrectness("TPAQ");
              testSpeed("TPAQ");
           }
           else
           {
              System.out.println("Test" + type + "EntropyCoder");
              testCorrectness(type);
              testSpeed(type);
           }
       }        
    }
    
    
    private static Predictor getPredictor(String type)
    {
       if (type.equals("PAQ"))
          return new PAQPredictor();
       
       if (type.equals("TPAQ"))
          return new TPAQPredictor();
       
       if (type.equals("FPAQ"))
          return new FPAQPredictor();
       
       if (type.equals("CM"))
          return new CMPredictor();
       
       return null;
    }
    
    
    public static void testCorrectness(String name)
    {
        // Test behavior
        System.out.println("Correctness test for " + name);

        for (int ii=1; ii<20; ii++)
        {
            System.out.println("\n\nTest "+ii);

            try
            {
                byte[] values;
                Random random = new Random();

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
                          values[i] = (byte) 2; // all identical
                }
                else if (ii == 5)
                {
                     values = new byte[32];

                     for (int i=0; i<values.length; i++)
                          values[i] = (byte) (2 + (i & 1)); // 2 symbols
                }
                else
                {
                     values = new byte[32];

                     for (int i=0; i<values.length; i++)
                          values[i] = (byte) (64 + 3*ii + random.nextInt(ii+1));
                }

                System.out.println("Original:");

                for (int i=0; i<values.length; i++)
                    System.out.print((values[i]&0xFF)+" ");

                System.out.println();
                System.out.println("\nEncoded:");
                ByteArrayOutputStream os = new ByteArrayOutputStream(16384);
                OutputBitStream obs = new DefaultOutputBitStream(os, 16384);
                DebugOutputBitStream dbgbs = new DebugOutputBitStream(obs, System.out);
                dbgbs.showByte(true);
                BinaryEntropyEncoder bec = new BinaryEntropyEncoder(dbgbs, getPredictor(name));
                bec.encode(values, 0, values.length);
                
                bec.dispose();
                dbgbs.close();
                byte[] buf = os.toByteArray();
                InputBitStream ibs = new DefaultInputBitStream(new ByteArrayInputStream(buf), 1024);
                BinaryEntropyDecoder bed = new BinaryEntropyDecoder(ibs, getPredictor(name));
                System.out.println();
                System.out.println("\nDecoded:");
                boolean ok = true;
                byte[] values2 = new byte[values.length];
                bed.decode(values2, 0, values2.length);
                bed.dispose();
                ibs.close();

                try
                {
                   for (int j=0; j<values2.length; j++)
                   {
                        if (values[j] != values2[j])
                           ok = false;

                        System.out.print((values2[j]&0xFF)+" ");
                    }
                }
                catch (BitStreamException e)
                {
                   e.printStackTrace();
                   break;
                }

                System.out.println("\n"+((ok == true) ? "Identical" : "Different"));
            }
            catch (Exception e)
            {
                e.printStackTrace();
            }
        }
     }

    
     public static void testSpeed(String name)
     {
        // Test speed
        System.out.println("\n\nSpeed test for " + name);
        int[] repeats = { 3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3 };
        final int size = 500000;
        final int iter = 100;
        Random random = new Random();

        for (int jj=0; jj<3; jj++)
        {
            System.out.println("\nTest "+(jj+1));
            byte[] values1 = new byte[size];
            byte[] values2 = new byte[size];
            long delta1 = 0, delta2 = 0;

            for (int ii=0; ii<iter; ii++)
            {
                int idx = 0;

                for (int i=0; i<size; i++)
                {
                    int i0 = i;
                    int len = repeats[idx];
                    idx = (idx + 1) & 0x0F;
                    byte b = (byte) random.nextInt(256);

                    if (i0+len >= size)
                        len = size-i0-1;

                    for (int j=i0; j<i0+len; j++)
                    {
                       values1[j] = b;
                       i++;
                    }
                }

                // Encode
                ByteArrayOutputStream os = new ByteArrayOutputStream(size*2);
                OutputBitStream bs = new DefaultOutputBitStream(os, size);
                BinaryEntropyEncoder bec = new BinaryEntropyEncoder(bs, getPredictor(name));
                long before1 = System.nanoTime();
                
                if (bec.encode(values1, 0, values1.length) < 0)
                {
                   System.out.println("Encoding error");
                   System.exit(1);
                }

                bec.dispose();
                long after1 = System.nanoTime();
                delta1 += (after1 - before1);
                bs.close();

                // Decode
                byte[] buf = os.toByteArray();
                InputBitStream bs2 = new DefaultInputBitStream(new ByteArrayInputStream(buf), size);
                BinaryEntropyDecoder bed = new BinaryEntropyDecoder(bs2, getPredictor(name));
                long before2 = System.nanoTime();
                
                if (bed.decode(values2, 0, size) < 0)
                {
                   System.out.println("Decoding error");
                   System.exit(1);
                }

                bed.dispose();
                long after2 = System.nanoTime();
                delta2 += (after2 - before2);
                bs2.close();

                // Sanity check
                for (int i=0; i<size; i++)
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
