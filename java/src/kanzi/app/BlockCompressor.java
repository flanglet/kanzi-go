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

package kanzi.app;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.file.FileSystems;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import kanzi.SliceByteArray;
import kanzi.io.BlockListener;
import kanzi.io.ByteFunctionFactory;
import kanzi.io.CompressedOutputStream;
import kanzi.io.Error;
import kanzi.io.InfoPrinter;
import kanzi.io.NullOutputStream;


public class BlockCompressor implements Runnable, Callable<Integer>
{
   private static final int DEFAULT_BUFFER_SIZE = 32768;
   public static final int WARN_EMPTY_INPUT = -128;
   
   private final int verbosity;
   private final boolean overwrite;
   private final boolean checksum;
   private final String inputName;
   private final String outputName;
   private final String codec;
   private final String transform;
   private final int blockSize;
   private final int jobs;
   private InputStream is;
   private CompressedOutputStream cos;
   private final boolean ownPool;
   private final ExecutorService pool;
   private final List<BlockListener> listeners;

   
   public BlockCompressor(Map<String, Object> map, ExecutorService threadPool)
   {
      this.verbosity = (Integer) map.remove("verbose");
      Boolean bForce = (Boolean) map.remove("overwrite");
      this.overwrite = (bForce == null) ? false : bForce;
      this.inputName = (String) map.remove("inputName");
      this.outputName = (String) map.remove("outputName");
      String strCodec = (String) map.remove("entropy");
      this.codec = (strCodec == null) ? "HUFFMAN" : strCodec;
      Integer iBlockSize = (Integer) map.remove("block");
      this.blockSize = (iBlockSize == null) ? 1024*1024 : iBlockSize;
      // Extract transform names. Curate input (EG. NONE+NONE+xxxx => xxxx)          
      String tName = (String) map.remove("transform");
      ByteFunctionFactory bff = new ByteFunctionFactory();      
      this.transform = (tName == null) ? "BWT+MTFT+ZRLT" : bff.getName(bff.getType(tName));
      Boolean bChecksum = (Boolean) map.remove("checksum");
      this.checksum = (bChecksum == null) ? false : bChecksum;
      this.jobs = (Integer) map.remove("jobs");
      this.pool = (this.jobs == 1) ? null : 
              ((threadPool == null) ? Executors.newCachedThreadPool() : threadPool);
      this.ownPool = (threadPool == null) && (this.pool != null);
      this.listeners = new ArrayList<>(10);
      
      if (this.verbosity > 1)
         this.addListener(new InfoPrinter(this.verbosity, InfoPrinter.Type.ENCODING, System.out));
      
      if ((this.verbosity > 0) && (map.size() > 0))
      {
         for (String k : map.keySet())
            printOut("Ignoring invalid option [" + k + "]", verbosity>0);
      }  
   }
 

   public void dispose()
   {
      try
      {
         if (this.is != null)
            this.is.close();
      }
      catch (IOException ioe)
      {
         /* ignore */
      }

      try
      {
         if (this.cos != null)
            this.cos.close();
      }
      catch (IOException ioe)
      {
         System.err.println("Compression failure: " + ioe.getMessage());
         System.exit(Error.ERR_WRITE_FILE);
      }
      
      if ((this.pool != null) && (this.ownPool == true))
         this.pool.shutdown();
      
      this.listeners.clear();
   }


   @Override
   public void run()
   {
      this.call();
   }


   // Return status (success = 0, error < 0)
   @Override
   public Integer call()
   { 
      boolean printFlag = this.verbosity > 1;
      printOut("Kanzi 1.1 (C) 2017,  Frederic Langlet", this.verbosity >= 1);
      printOut("Input file name set to '" + this.inputName + "'", printFlag);
      printOut("Output file name set to '" + this.outputName + "'", printFlag);
      printOut("Block size set to " + this.blockSize + " bytes", printFlag);
      printOut("Verbosity set to " + this.verbosity, printFlag);
      printOut("Overwrite set to " + this.overwrite, printFlag);
      printOut("Checksum set to "+  this.checksum, printFlag);
      String etransform = ("NONE".equals(this.transform)) ? "no" : this.transform;
      printOut("Using " + etransform + " transform (stage 1)", printFlag);
      String ecodec = ("NONE".equals(this.codec)) ? "no" : this.codec;
      printOut("Using " + ecodec + " entropy codec (stage 2)", printFlag);
      printOut("Using " + this.jobs + " job" + ((this.jobs > 1) ? "s" : ""), printFlag);

      OutputStream os;
      
      try
      {  
         if (this.outputName.equalsIgnoreCase("NONE"))
         {
            os = new NullOutputStream(); 
         }
         else if (this.outputName.equalsIgnoreCase("STDOUT"))
         {
            os = System.out;
         }
         else
         {
            File output = new File(this.outputName);
         
            if (output.exists())
            {
               if (output.isDirectory())
               {
                  System.err.println("The output file is a directory");
                  return Error.ERR_OUTPUT_IS_DIR;
               }

               if (this.overwrite == false)
               {
                  System.err.println("The output file exists and the 'overwrite' command "
                          + "line option has not been provided");
                  return Error.ERR_OVERWRITE_FILE;
               }
               
               Path path1 = FileSystems.getDefault().getPath(this.inputName).toAbsolutePath();
               Path path2 = FileSystems.getDefault().getPath(this.outputName).toAbsolutePath();
               
               if (path1.equals(path2))
               {
                  System.err.println("The input and output files must be different");
                  return Error.ERR_CREATE_FILE; 
               }
            }
            
            os = new FileOutputStream(output);
         }
         
         try
         {
            this.cos = new CompressedOutputStream(this.codec, this.transform,
                 os, this.blockSize, this.checksum,
                 this.pool, this.jobs);
            
            for (BlockListener bl : this.listeners)
               this.cos.addListener(bl);
         }
         catch (Exception e)
         {
            System.err.println("Cannot create compressed stream: "+e.getMessage());
            return Error.ERR_CREATE_COMPRESSOR;
         }
      }
      catch (Exception e)
      {
         System.err.println("Cannot open output file '"+this.outputName+"' for writing: " + e.getMessage());
         return Error.ERR_CREATE_FILE;
      }

      try
      {
         this.is = (this.inputName.equalsIgnoreCase("STDIN")) ? System.in : new FileInputStream(this.inputName);
      }
      catch (Exception e)
      {
         System.err.println("Cannot open input file '"+this.inputName+"': " + e.getMessage());
         return Error.ERR_OPEN_FILE;
      }

      // Encode
      boolean silent = this.verbosity < 1;
      printOut("Encoding ...", !silent);
      int read = 0;
      SliceByteArray sa = new SliceByteArray(new byte[DEFAULT_BUFFER_SIZE], 0);
      int len;
      long before = System.nanoTime();

      try
      {       
          while (true)
          {
             try
             {
                len = this.is.read(sa.array, 0, sa.length);
             }
             catch (Exception e)
             {
                System.err.print("Failed to read block from file '"+this.inputName+"': ");
                System.err.println(e.getMessage());                
                return Error.ERR_READ_FILE;
             }
             
             if (len <= 0)
                break;
             
             // Just write block to the compressed output stream !
             read += len;
             this.cos.write(sa.array, 0, len);
          }
       }
       catch (kanzi.io.IOException e)
       {
          System.err.println(e.getMessage());
          return e.getErrorCode();
       }
       catch (Exception e)
       {
          System.err.println("An unexpected condition happened. Exiting ...");
          System.err.println(e.getMessage());
          return Error.ERR_UNKNOWN;
       }
       finally
       {
          // Close streams to ensure all data are flushed
          this.dispose(); 
         
          try 
          {
             os.close();
          } 
          catch (IOException e)
          {
             // Ignore
          }            
       }
      
       if (read == 0)
       {
          System.out.println("Empty input file ... nothing to do");
          return WARN_EMPTY_INPUT;
       }

       long after = System.nanoTime();
       long delta = (after - before) / 1000000L; // convert to ms
       printOut("", !silent);
       printOut("Encoding:          "+delta+" ms", !silent);
       printOut("Input size:        "+read, !silent);
       printOut("Output size:       "+this.cos.getWritten(), !silent);
       float f = this.cos.getWritten() / (float) read;
       printOut("Ratio:             "+String.format("%1$.6f", f), !silent);

       if (delta > 0)
          printOut("Throughput (KB/s): "+(((read * 1000L) >> 10) / delta), !silent);

       printOut("", !silent);
       return 0;
    }


    private static void printOut(String msg, boolean print)
    {
       if ((print == true) && (msg != null))
          System.out.println(msg);
    }
    

    public final boolean addListener(BlockListener bl)
    {
       return (bl != null) ? this.listeners.add(bl) : false;
    }

   
    public final boolean removeListener(BlockListener bl)
    {
       return (bl != null) ? this.listeners.remove(bl) : false;
    }
}
