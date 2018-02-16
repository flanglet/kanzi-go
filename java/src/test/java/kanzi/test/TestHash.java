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

import java.io.File;
import java.io.FileInputStream;
import kanzi.util.hash.MurMurHash3;
import kanzi.util.hash.SipHash_2_4;
import kanzi.util.hash.XXHash32;
import kanzi.util.hash.XXHash64;


public class TestHash
{
  public static void main(String[] args)
  {    
      try
      {
         String fileName = (args.length > 0) ? args[0] : "c:\\temp\\rt.jar";
         int iter = 500;
         System.out.println("Processing "+fileName);
         System.out.println(iter+" iterations");

         {
            System.out.println("XXHash32 speed test");
            File input = new File(fileName);
            FileInputStream fis = new FileInputStream(input);
            byte[] array = new byte[16384];
            XXHash32 hash = new XXHash32(0);
            int len;
            long size = 0;
            int res = 0;
            long sum = 0;

            while ((len = fis.read(array, 0, array.length)) > 0)
            { 
               long before = System.nanoTime();
               
               for (int i=0; i<iter; i++)
               {
                  hash.setSeed(0);
                  res += hash.hash(array, 0, len);
               }
               
               long after = System.nanoTime();
               sum += (after - before);
               size += (len * iter);
            }
           
            fis.close();
            System.out.println("XXHash32 res="+Integer.toHexString(res));
            System.out.println("Elapsed [ms]: " +sum/1000000L);
            System.out.println("Throughput [MB/s]: " +(size/1024*1000/1024)/(sum/1000000L));
         }
         
         System.out.println();
         
         {
            System.out.println("XXHash64 speed test");
            File input = new File(fileName);
            FileInputStream fis = new FileInputStream(input);
            byte[] array = new byte[16384];
            XXHash64 hash = new XXHash64(0);
            int len;
            long size = 0;
            long res = 0;
            long sum = 0;

            while ((len = fis.read(array, 0, array.length)) > 0)
            { 
               long before = System.nanoTime();
               
               for (int i=0; i<iter; i++)
               {
                  hash.setSeed(0);
                  res += hash.hash(array, 0, len);
               }
               
               long after = System.nanoTime();
               sum += (after - before);
               size += (len * iter);
            }
           
            fis.close();
            System.out.println("XXHash64 res="+Long.toHexString(res));
            System.out.println("Elapsed [ms]: " +sum/1000000L);
            System.out.println("Throughput [MB/s]: " +(size/1024*1000/1024)/(sum/1000000L));
         }
         
         System.out.println();
         
         {
            System.out.println("MurmurHash3 speed test");
            File input = new File(fileName);
            FileInputStream fis = new FileInputStream(input);
            byte[] array = new byte[16384];
            MurMurHash3 hash = new MurMurHash3(0);
            int len;
            long size = 0;
            int res = 0;
            long sum = 0;

            while ((len = fis.read(array, 0, array.length)) > 0)
            { 
               long before = System.nanoTime();

               for (int i=0; i<iter; i++)
               {
                  hash.setSeed(0);
                  res += hash.hash(array, 0, len);     
               }               

               long after = System.nanoTime();
               sum += (after - before);
               size += (len * iter);
            }            
            
            fis.close();
            System.out.println("MurmurHash res="+Integer.toHexString(res));
            System.out.println("Elapsed [ms]: " +sum/1000000L);
            System.out.println("Throughput [MB/s]: " +(size/1024*1000/1024)/(sum/1000000L));
         }

         
         System.out.println();
         
         {
            System.out.println("SipHash_2_4 speed test");
            File input = new File(fileName);
            FileInputStream fis = new FileInputStream(input);
            byte[] array = new byte[16384];
            SipHash_2_4 hash = new SipHash_2_4();
            int len;
            long size = 0;
            long res = 0;
            long sum = 0;

            while ((len = fis.read(array, 0, array.length)) > 0)
            { 
               long before = System.nanoTime();
   
               for (int i=0; i<iter; i++)
               {
                  hash.setSeed(0, 0);
                  res += hash.hash(array, 0, len);     
               }
               
               long after = System.nanoTime();
               sum += (after - before);
               size += (len * iter);
            }            
            
            fis.close();
            System.out.println("SipHash_2_4 res="+Long.toHexString(res));
            System.out.println("Elapsed [ms]: " +sum/1000000L);
            System.out.println("Throughput [MB/s]: " +(size/1024*1000/1024)/(sum/1000000L));
         }
      }
      catch (Exception e)
      {
         e.printStackTrace();
      }
  }  
}