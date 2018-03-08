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

import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.util.Random;
import kanzi.BitStreamException;
import kanzi.InputBitStream;
import kanzi.OutputBitStream;
import kanzi.bitstream.DebugOutputBitStream;
import kanzi.bitstream.DefaultInputBitStream;
import kanzi.bitstream.DefaultOutputBitStream;
import org.junit.Assert;
import org.junit.Test;


public class TestDefaultBitStream
{
   public static void main(String[] args)
   {
      testCorrectnessAligned();
      testCorrectnessMisaligned();
      testSpeed(args); // Writes big output.bin file to local dir (or specified file name) !!!
   }
    
    
   @Test
   public void testDefaultBitStream()
   {
      Assert.assertTrue(testCorrectnessAligned());
      Assert.assertTrue(testCorrectnessMisaligned());
   }
    
    
   public static boolean testCorrectnessAligned()
   {    
      // Test correctness (byte aligned)
      System.out.println("Correctness Test - byte aligned");
      int[] values = new int[100];
      Random rnd = new Random();
      System.out.println("\nInitial");

      try
      {
         for (int test=0; test<10; test++)
         {
            ByteArrayOutputStream baos = new ByteArrayOutputStream(4*values.length);
            OutputStream os = new BufferedOutputStream(baos);
            OutputBitStream obs = new DefaultOutputBitStream(os, 16384);
            DebugOutputBitStream dbs = new DebugOutputBitStream(obs, System.out);
            dbs.showByte(true);       

            for (int i=0; i<values.length; i++)
            {
               values[i] = (test<5) ? rnd.nextInt(test*1000+100) : rnd.nextInt();
               System.out.print(values[i]+" ");

                if ((i % 50) == 49)
                   System.out.println();                                     
            }

            System.out.println();
            System.out.println();

            for (int i=0; i<values.length; i++)
            {                   
                dbs.writeBits(values[i], 32);
            }

            // Close first to force flush()
            dbs.close();
            byte[] output = baos.toByteArray();
            ByteArrayInputStream bais = new ByteArrayInputStream(output);
            InputStream is = new BufferedInputStream(bais);
            InputBitStream ibs = new DefaultInputBitStream(is, 16384);
            System.out.println("Read:");
            boolean ok = true;

            for (int i=0; i<values.length; i++)
            {                
                int x = (int) ibs.readBits(32);
                System.out.print(x);
                System.out.print((x == values[i]) ? " ": "* ");
                ok &= (x == values[i]);

                if ((i % 50) == 49)
                   System.out.println();                                      
            }

            ibs.close();
            System.out.println("\n");
            System.out.println("Bits written: "+dbs.written());
            System.out.println("Bits read: "+ibs.read());
            System.out.println("\n"+((ok)?"Success":"Failure"));
            System.out.println();
            System.out.println();
         }
      }
      catch (Exception e)
      {
        e.printStackTrace();
        return false;
      }
      
      return true;
   }


   public static boolean testCorrectnessMisaligned()
   {    
      // Test correctness (not byte aligned)
      System.out.println("Correctness Test - not byte aligned");
      int[] values = new int[100];
      Random rnd = new Random();

      try
      {
         for (int test=0; test<10; test++)
         {
            ByteArrayOutputStream baos = new ByteArrayOutputStream(4*values.length);
            OutputStream os = new BufferedOutputStream(baos);
            OutputBitStream obs = new DefaultOutputBitStream(os, 16384);
            DebugOutputBitStream dbs = new DebugOutputBitStream(obs, System.out);
            dbs.showByte(true);       

            for (int i=0; i<values.length; i++)
            {
               values[i] = (test<5) ? rnd.nextInt(test*1000+100) : rnd.nextInt();
               final int mask = (1 << (1 + (i & 63))) - 1;
               values[i] &= mask;
               System.out.print(values[i]+" ");

               if ((i % 50) == 49)
                  System.out.println();                   
            }

            System.out.println();
            System.out.println();

            for (int i=0; i<values.length; i++)
            {
               dbs.writeBits(values[i], (1 + (i & 63)));
            }

            // Close first to force flush()
            dbs.close();

            System.out.println();
            System.out.println("Trying to write to closed stream");

            try
            {
               dbs.writeBit(1);
            }
            catch (BitStreamException e)
            {
               System.out.println("Exception: " + e.getMessage());
            }

            byte[] output = baos.toByteArray();
            ByteArrayInputStream bais = new ByteArrayInputStream(output);
            InputStream is = new BufferedInputStream(bais);
            InputBitStream ibs = new DefaultInputBitStream(is, 16384);
            System.out.println();
            System.out.println("Read: ");
            boolean ok = true;

            for (int i=0; i<values.length; i++)
            {
               int x = (int) ibs.readBits((1 + (i & 63)));
               System.out.print(x);
               System.out.print((x == values[i]) ? " ": "* ");
               ok &= (x == values[i]);

               if ((i % 50) == 49)
                  System.out.println();                   
            }

            ibs.close();
            System.out.println("\n");
            System.out.println("Bits written: "+dbs.written());
            System.out.println("Bits read: "+ibs.read());
            System.out.println("\n"+((ok)?"Success":"Failure"));
            System.out.println();
            System.out.println();
            System.out.println("Trying to read from closed stream");

            try
            {
               ibs.readBits(1);
            }
            catch (BitStreamException e)
            {
                  System.out.println("Exception: " + e.getMessage());
            }
         }
      }
      catch (Exception e)
      {
         e.printStackTrace();
         return false;
      }
      
      return true;
   }


   public static boolean testSpeed(String[] args)
   {    
      // Test speed
      System.out.println("\nSpeed Test");
      String fileName = (args.length > 0) ? args[0] : "r:\\output.bin";
      File file = new File(fileName);
      file.deleteOnExit();
      int[] values = new int[] { 3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3,
                                 31, 14, 41, 15, 59, 92, 26, 65, 53, 35, 58, 89, 97, 79, 93, 32 };

      try
      {
         final int iter = 150;
         long written = 0;
         long read = 0;
         long before, after;
         long delta1 = 0, delta2 = 0;
         int nn = 100000 * values.length;           

         for (int test=0; test<iter; test++)
         {
            FileOutputStream os = new FileOutputStream(file);
            OutputBitStream obs = new DefaultOutputBitStream(os, 1024*1024);
            before = System.nanoTime();

            for (int i=0; i<nn; i++)
            {
               obs.writeBits(values[i%values.length], 1+(i&63));
            }

            // Close first to force flush()
            obs.close();
            after = System.nanoTime();
            delta1 += (after-before);
            written += obs.written();

            FileInputStream is = new FileInputStream(new File(fileName));
            InputBitStream ibs = new DefaultInputBitStream(is, 1024*1024);
            before = System.nanoTime();

            for (int i=0; i<nn; i++)
            {
               ibs.readBits(1+(i&63));
            }

            ibs.close();
            after = System.nanoTime();
            delta2 += (after-before);
            read += ibs.read();
         }

         System.out.println(written+ " bits written ("+(written/1024/1024/8)+" MB)");
         System.out.println(read+ " bits read ("+(read/1024/1024/8)+" MB)");
         System.out.println();
         System.out.println("Write [ms]        : "+(delta1/1000000L));
         System.out.println("Throughput [MB/s] : "+((written/1024*1000/8192)/(delta1/1000000L)));
         System.out.println("Read [ms]         : "+(delta2/1000000L));
         System.out.println("Throughput [MB/s] : "+((read/1024*1000/8192)/(delta2/1000000L)));
      }
      catch (Exception e)
      {
         e.printStackTrace();
         return false;
      }
      
      return true;
   }
}
