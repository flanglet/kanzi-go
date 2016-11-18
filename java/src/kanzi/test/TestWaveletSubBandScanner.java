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

import kanzi.function.wavelet.WaveletBandScanner;


public class TestWaveletSubBandScanner
{
    
    public static void main(String[] args)
    {
        int h = 256;
        int w = 512;
//        int[] buffer = new int[size*size];
        int[] output = new int[w*h];
        
//        for (int i=0; i<buffer.length; i++)
//            buffer[i] = i;
        
        System.out.println("\nOne shot Scan");
        WaveletBandScanner scanner = new WaveletBandScanner(w, h, WaveletBandScanner.ALL_BANDS, 5);
        int maxLoop = 1;
        int n = 0;
        long before = System.nanoTime();
        
        for (int ii=0; ii<maxLoop; ii++)
          n = scanner.getIndexes(output);       
        
        long after = System.nanoTime();
        
        if (maxLoop > 1)
            System.out.println("Elapsed [ms]: "+(after-before)/1000000);
        
        System.out.println(scanner.getSize()+" coefficients");
        System.out.println(n+" read coefficients");
        
        for (int i=0; i<n; i+=100)
        {
            for (int j=i; j<i+100; j++)
                System.out.print(output[j]+" ");
            
            System.out.println();
        }
        
        System.out.println("\nPartial Scan");
        output = new int[100];
        int count = 0;
        n = 0;
        
        while (count < scanner.getSize())
        {
            if (count == 130900)
                System.out.println("");
            
            int processed = scanner.getIndexes(output, output.length, count);
            count += processed;
            System.out.println(processed+" coefficients (total="+count+")");
            n++;
            
            for (int i=0; i<processed; i++)
                System.out.print(output[i]+" ");
            
            System.out.println();
        }
        
    }
}
