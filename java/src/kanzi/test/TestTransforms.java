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
import kanzi.SliceIntArray;
import kanzi.IntTransform;
import kanzi.transform.DCT16;
import kanzi.transform.DCT32;
import kanzi.transform.DCT4;
import kanzi.transform.DCT8;
import kanzi.transform.DST4;
import kanzi.transform.DWT_CDF_9_7;
import kanzi.transform.DWT_DCT;
import kanzi.transform.WHT16;
import kanzi.transform.WHT32;
import kanzi.transform.WHT4;
import kanzi.transform.WHT8;

public class TestTransforms
{

  public static void main(String[] args)
  {    
        Runnable r2 = new Runnable()
        {
            int[] block = new int[] {
               3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5, 8, 9, 7, 9, 3,
               2, 3, 8, 4, 6, 2, 6, 4, 3, 3, 8, 3, 2, 7, 9, 5,
               0, 2, 8, 8, 4, 1, 9, 7, 1, 6, 9, 3, 9, 9, 3, 7,
               5, 1, 0, 5, 8, 2, 0, 9, 7, 4, 9, 4, 4, 5, 9, 2,
               3, 0, 7, 8, 1, 6, 4, 0, 6, 2, 8, 6, 2, 0, 8, 9,
               9, 8, 6, 2, 8, 0, 3, 4, 8, 2, 5, 3, 4, 2, 1, 1,
               7, 0, 6, 7, 9, 8, 2, 1, 4, 8, 0, 8, 6, 5, 1, 3,
               2, 8, 2, 3, 0, 6, 6, 4, 7, 0, 9, 3, 8, 4, 4, 6,
               0, 9, 5, 5, 0, 5, 8, 2, 2, 3, 1, 7, 2, 5, 3, 5,
               9, 4, 0, 8, 1, 2, 8, 4, 8, 1, 1, 1, 7, 4, 5, 0,
               2, 8, 4, 1, 0, 2, 7, 0, 1, 9, 3, 8, 5, 2, 1, 1,
               0, 5, 5, 5, 9, 6, 4, 4, 6, 2, 2, 9, 4, 8, 9, 5,
               4, 9, 3, 0, 3, 8, 1, 9, 6, 4, 4, 2, 8, 8, 1, 0,
               9, 7, 5, 6, 6, 5, 9, 3, 3, 4, 4, 6, 1, 2, 8, 4,
               7, 5, 6, 4, 8, 2, 3, 3, 7, 8, 6, 7, 8, 3, 1, 6,
               5, 2, 7, 1, 2, 0, 1, 9, 0, 9, 1, 4, 5, 6, 4, 8,
               5, 6, 6, 9, 2, 3, 4, 6, 0, 3, 4, 8, 6, 1, 0, 4,
               5, 4, 3, 2, 6, 6, 4, 8, 2, 1, 3, 3, 9, 3, 6, 0,
               7, 2, 6, 0, 2, 4, 9, 1, 4, 1, 2, 7, 3, 7, 2, 4,
               5, 8, 7, 0, 0, 6, 6, 0, 6, 3, 1, 5, 5, 8, 8, 1,
               7, 4, 8, 8, 1, 5, 2, 0, 9, 2, 0, 9, 6, 2, 8, 2,
               9, 2, 5, 4, 0, 9, 1, 7, 1, 5, 3, 6, 4, 3, 6, 7,
               8, 9, 2, 5, 9, 0, 3, 6, 0, 0, 1, 1, 3, 3, 0, 5,
               3, 0, 5, 4, 8, 8, 2, 0, 4, 6, 6, 5, 2, 1, 3, 8,
               4, 1, 4, 6, 9, 5, 1, 9, 4, 1, 5, 1, 1, 6, 0, 9,
               4, 3, 3, 0, 5, 7, 2, 7, 0, 3, 6, 5, 7, 5, 9, 5,
               9, 1, 9, 5, 3, 0, 9, 2, 1, 8, 6, 1, 1, 7, 3, 8,
               1, 9, 3, 2, 6, 1, 1, 7, 9, 3, 1, 0, 5, 1, 1, 8,
               5, 4, 8, 0, 7, 4, 4, 6, 2, 3, 7, 9, 9, 6, 2, 7,
               4, 9, 5, 6, 7, 3, 5, 1, 8, 8, 5, 7, 5, 2, 7, 2,
               4, 8, 9, 1, 2, 2, 7, 9, 3, 8, 1, 8, 3, 0, 1, 1,
               9, 4, 9, 1, 2, 9, 8, 3, 3, 6, 7, 3, 3, 6, 2, 4,
               4, 0, 6, 5, 6, 6, 4, 3, 0, 8, 6, 0, 2, 1, 3, 9,
               4, 9, 4, 6, 3, 9, 5, 2, 2, 4, 7, 3, 7, 1, 9, 0,
               7, 0, 2, 1, 7, 9, 8, 6, 0, 9, 4, 3, 7, 0, 2, 7,
               7, 0, 5, 3, 9, 2, 1, 7, 1, 7, 6, 2, 9, 3, 1, 7,
               6, 7, 5, 2, 3, 8, 4, 6, 7, 4, 8, 1, 8, 4, 6, 7,
               6, 6, 9, 4, 0, 5, 1, 3, 2, 0, 0, 0, 5, 6, 8, 1,
               2, 7, 1, 4, 5, 2, 6, 3, 5, 6, 0, 8, 2, 7, 7, 8,
               5, 7, 7, 1, 3, 4, 2, 7, 5, 7, 7, 8, 9, 6, 0, 9,
               1, 7, 3, 6, 3, 7, 1, 7, 8, 7, 2, 1, 4, 6, 8, 4,
               4, 0, 9, 0, 1, 2, 2, 4, 9, 5, 3, 4, 3, 0, 1, 4,
               6, 5, 4, 9, 5, 8, 5, 3, 7, 1, 0, 5, 0, 7, 9, 2,
               2, 7, 9, 6, 8, 9, 2, 5, 8, 9, 2, 3, 5, 4, 2, 0,
               1, 9, 9, 5, 6, 1, 1, 2, 1, 2, 9, 0, 2, 1, 9, 6,
               0, 8, 6, 4, 0, 3, 4, 4, 1, 8, 1, 5, 9, 8, 1, 3,
               6, 2, 9, 7, 7, 4, 7, 7, 1, 3, 0, 9, 9, 6, 0, 5,
               1, 8, 7, 0, 7, 2, 1, 1, 3, 4, 9, 9, 9, 9, 9, 9,
               8, 3, 7, 2, 9, 7, 8, 0, 4, 9, 9, 5, 1, 0, 5, 9,
               7, 3, 1, 7, 3, 2, 8, 1, 6, 0, 9, 6, 3, 1, 8, 5,
               9, 5, 0, 2, 4, 4, 5, 9, 4, 5, 5, 3, 4, 6, 9, 0,
               8, 3, 0, 2, 6, 4, 2, 5, 2, 2, 3, 0, 8, 2, 5, 3,
               3, 4, 4, 6, 8, 5, 0, 3, 5, 2, 6, 1, 9, 3, 1, 1,
               8, 8, 1, 7, 1, 0, 1, 0, 0, 0, 3, 1, 3, 7, 8, 3,
               8, 7, 5, 2, 8, 8, 6, 5, 8, 7, 5, 3, 3, 2, 0, 8,
               3, 8, 1, 4, 2, 0, 6, 1, 7, 1, 7, 7, 6, 6, 9, 1,
               4, 7, 3, 0, 3, 5, 9, 8, 2, 5, 3, 4, 9, 0, 4, 2,
               8, 7, 5, 5, 4, 6, 8, 7, 3, 1, 1, 5, 9, 5, 6, 2,
               8, 6, 3, 8, 8, 2, 3, 5, 3, 7, 8, 7, 5, 9, 3, 7,
               5, 1, 9, 5, 7, 7, 8, 1, 8, 5, 7, 7, 8, 0, 5, 3,
               2, 1, 7, 1, 2, 2, 6, 8, 0, 6, 6, 1, 3, 0, 0, 1,
               9, 2, 7, 8, 7, 6, 6, 1, 1, 1, 9, 5, 9, 0, 9, 2,
               1, 6, 4, 2, 0, 1, 9, 8, 9, 3, 8, 0, 9, 5, 2, 5,
               7, 2, 0, 1, 0, 6, 5, 4, 8, 5, 8, 6, 3, 2, 7, 8
            };
            
            @Override
            public void run()
            {            
              IntTransform[] transforms = new IntTransform[] 
                      { new DCT4(), new DCT8(), new DCT16(), new DCT32(), 
                        new DWT_DCT(8), new DWT_DCT(16), new DWT_DCT(32), 
                        new DWT_CDF_9_7(8, 8, 1), new DWT_CDF_9_7(16, 16, 1), new DWT_CDF_9_7(32, 32, 1),
                        new DST4(), new WHT4(), new WHT8(), new WHT16(), new WHT32() };
              int[] dims = new int[] { 4, 8, 16, 32, 8, 16, 32, 8, 16, 32, 4, 4, 8, 16, 32 };
              String[] names = new String[] {"DCT", "DCT", "DCT", "DCT", "DWT_DCT", 
                 "DWT_DCT", "DWT_DCT", "DWT", "DWT", "DWT", "DST",
              "WHT", "WHT", "WHT", "WHT"};
              
              for (int idx=0; idx<transforms.length; idx++)
              {   
                  int dim = dims[idx];
                  String transform = names[idx] + " " + dim;
                  System.out.println("\n"+transform+" correctness\n");
                  final int blockSize = dim * dim;
                  int[] data1 = new int[blockSize+20]; // source
                  int[] data2 = new int[blockSize+20]; // destination
                  int[] data3 = new int[blockSize+20]; // source copy
                  int[] data;
                  SliceIntArray iia1 = new SliceIntArray(data1, 0);
                  SliceIntArray iia2 = new SliceIntArray(data2, 0);
                  Random rnd = new Random();

                  for (int nn=0; nn<20; nn++)
                  {
                     System.out.println(transform + " - input "+nn+":");

                     for (int i=0; i<blockSize; i++)
                     {
                        if (nn == 0)
                          data1[i] = this.block[i];
                        else
                          data1[i] = rnd.nextInt(nn*10);
                        
                        data3[i] = data1[i];
                        System.out.print(data1[i]);
                        System.out.print("  ");
                     }

                     int start = 0;//(nn & 1) * nn;

                     if (nn <= 10) 
                        data = data1;
                     else 
                        data = data2;
                     
                     iia1.array = data1;
                     iia1.index = 0;
                     iia2.array = data;
                     iia2.index = start;
                     iia1.length = dim*dim;
                     transforms[idx].forward(iia1, iia2);
                     System.out.println();
                     System.out.println("Output:");

                     for (int i=0; i<blockSize; i++)
                     {
                        System.out.print(data[i]);
                        System.out.print(" ");
                     }

                     iia1.array = data2;
                     iia1.index = 0;
                     iia2.index = start;
                     iia2.length = dim*dim;
                     transforms[idx].inverse(iia2, iia1);
                     System.out.println();
                     System.out.println("Result:");
                     int sad = 0;

                     for (int i=0; i<blockSize; i++)
                     {
                        System.out.print(data2[i]);
                        sad += Math.abs(data2[i] - data3[i]);
                        System.out.print((data3[i] != data2[i]) ? "! " : "= ");
                     }

                     System.out.println("\nSAD: " + sad + "\n");
                   }
               }
            }
        };
                
        r2.run();


        Runnable r1 = new Runnable()
        {
            @Override
            public void run()
            {
                long delta1 = 0;
                long delta2 = 0;
                int iter = 500000;
                Random rnd = new Random();
                
                for (int times=0; times<100; times++)
                {
                    int[][] data = new int[1000][];
                    DCT8 dct = new DCT8();

                    for (int i=0; i<1000; i++)
                    {
                        data[i] = new int[64];

                        for (int j=0; j<64; j++)
                            data[i][j] = rnd.nextInt(10+i+j*10);
                    }

                    SliceIntArray iia = new SliceIntArray(data[0], 0);
                    long before, after;

                    for (int i=0; i<iter; i++)
                    {
                       iia.array = data[i%100];
                       before = System.nanoTime();
                       iia.index = 0;                       
                       dct.forward(iia, iia);
                       after = System.nanoTime();
                       delta1 += (after-before);
                       before = System.nanoTime();
                       iia.index = 0;
                       dct.inverse(iia, iia);
                       after = System.nanoTime();
                       delta2 += (after-before);
                    }
                }
                
                System.out.println("Iterations: "+iter*100);
                System.out.println("Forward [ms]: "+delta1/1000000);
                System.out.println("Inverse [ms]: "+delta2/1000000);                
            }
        };

        System.out.println("\nDCT8 speed");

        // Speed test
        r1.run();        
    }  
}