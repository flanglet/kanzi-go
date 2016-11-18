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

import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import kanzi.app.BlockCompressor;
import kanzi.app.BlockDecompressor;
import kanzi.io.BlockEvent;
import kanzi.io.BlockListener;


public class TestBlockCoder
{
    public static void main(String[] args)
    {
       ExecutorService pool = Executors.newCachedThreadPool();
       boolean addListeners = true;
       String output = null;
       String input = null;
       
       for (String arg : args)
       {
          if (arg.equals("-noListeners"))
             addListeners = false;
          else if (arg.startsWith("-output="))
             output = arg.substring("-output=".length());
          else if (arg.startsWith("-input="))
             input = arg.substring("-input=".length());
       }

       BlockCompressor enc = new BlockCompressor(args, pool);
       
       if (addListeners == true)
       {
          // Trivial example of a block listener
          enc.addListener(new BlockListener() 
          {
             @Override
             public void processEvent(BlockEvent evt) 
             {
               System.out.println(">>> "+evt);
             }
          });

          // enc.addListener(new InfoPrinter(InfoPrinter.Type.ENCODING, System.out));
       }
       
       int status = enc.call();
       
       if (status < 0)
       {
          System.out.println("Compression failed with status " + status + ", skipping decompression");
          System.exit(status);
       }

       if ("none".equalsIgnoreCase(output))
          System.exit(0);
       
       Set<String> set = new HashSet<String>();
       set.add("-overwrite");

       for (String arg : args)
       {
          if (arg.startsWith("-verbose="))
             set.add(arg);
          else if (arg.equals("-silent"))
             set.add(arg);
          else if (arg.startsWith("-jobs="))
             set.add(arg);
       }

       if (output != null)
          set.add("-input="+output);
       else
          set.add("-input="+input+".knz");
       
       args = set.toArray(new String[set.size()]);
       BlockDecompressor dec = new BlockDecompressor(args, pool);

       if (addListeners == true)
       {  
          // Trivial example of a block listener
          dec.addListener(new BlockListener() 
          {
             @Override
             public void processEvent(BlockEvent evt) 
             {
                System.out.println("<<< "+evt);
             }
          });
       }
       
       // dec.addListener(new InfoPrinter(InfoPrinter.Type.DECODING, System.out));
       status = dec.call();

       if (status < 0)
       {
          System.out.println("Decompression failed with status " + status);
          System.exit(status);
       }
       
       pool.shutdown();
    }
}