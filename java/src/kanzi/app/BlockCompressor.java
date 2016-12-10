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
import java.util.ArrayList;
import java.util.HashMap;
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
      this.verbosity = (Integer) map.get("verbose");
      this.overwrite = (Boolean) map.get("overwrite");
      this.inputName = (String) map.get("inputName");
      this.outputName = (String) map.get("outputName");
      this.codec = (String) map.get("codec");
      this.blockSize = (Integer) map.get("blockSize");
      this.transform = (String) map.get("transform");
      this.checksum = (Boolean) map.get("checksum");
      this.jobs = (Integer) map.get("jobs");
      this.pool = (this.jobs == 1) ? null : 
              ((threadPool == null) ? Executors.newCachedThreadPool() : threadPool);
      this.ownPool = (threadPool == null) && (this.pool != null);
      this.listeners = new ArrayList<BlockListener>(10);
      
      if (this.verbosity > 1)
         this.addListener(new InfoPrinter(this.verbosity, InfoPrinter.Type.ENCODING, System.out));
   }
    
   
   public BlockCompressor(String[] args, ExecutorService threadPool)
   {
      Map<String, Object> map = new HashMap<String, Object>();
      processCommandLine(args, map);
      
      String tName = (String) map.get("transform");
      ByteFunctionFactory bff = new ByteFunctionFactory();      
      // Extract transform names. Curate input (EG. NONE+NONE+xxxx => xxxx)      
      this.transform = bff.getName(bff.getType(tName));
      this.verbosity = (Integer) map.get("verbose");
      this.overwrite = (Boolean) map.get("overwrite");
      this.inputName = (String) map.get("inputName");
      this.outputName = (String) map.get("outputName");
      this.codec = (String) map.get("codec");
      this.blockSize = (Integer) map.get("blockSize");      
      this.checksum = (Boolean) map.get("checksum");
      this.jobs = (Integer) map.get("jobs");
      this.pool = (this.jobs == 1) ? null : 
              ((threadPool == null) ? Executors.newCachedThreadPool() : threadPool);
      this.ownPool = (threadPool == null) && (this.pool != null);
      this.listeners = new ArrayList<BlockListener>(10);
      
      if (this.verbosity > 1)
         this.addListener(new InfoPrinter(this.verbosity, InfoPrinter.Type.ENCODING, System.out));
   }


   public static void main(String[] args)
   {
      BlockCompressor bc = null;

      try
      {
         bc = new BlockCompressor(args, null);
      }
      catch (Exception e)
      {
         System.err.println("Could not create the block codec: "+e.getMessage());
         System.exit(Error.ERR_CREATE_COMPRESSOR);
      }

      final int code = bc.call();

      if (code != 0)
         bc.dispose();

      System.exit(code);
   }


   protected void dispose()
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
       printOut("Ratio:             "+this.cos.getWritten() / (float) read, !silent);

       if (delta > 0)
          printOut("Throughput (KB/s): "+(((read * 1000L) >> 10) / delta), !silent);

       printOut("", !silent);
       return 0;
    }


    private static void processCommandLine(String args[], Map<String, Object> map)
    {
        // Set default values
        int blockSize = 1024 * 1024; // 1 MB
        int verbose = 1;
        boolean overwrite = false;
        boolean checksum = false;
        String inputName = null;
        String outputName = null;
        String codec = "HUFFMAN"; // default
        String transform = "BWT+MTFT+ZRLT"; // default
        int tasks = 1;

        for (String arg : args)
        {
           arg = arg.trim();

           // Extract verbosity and output first
           if (arg.startsWith("-verbose="))
           {
               String verboseLevel = arg.substring(9).trim();
               
               try
               {
                   verbose = Integer.parseInt(verboseLevel);
                   
                   if (verbose < 0)
                      throw new NumberFormatException();
               }
               catch (NumberFormatException e)
               {
                  System.err.println("Invalid verbosity level provided on command line: "+arg);
                  System.exit(Error.ERR_INVALID_PARAM);
               }    
           }
           else if (arg.startsWith("-output="))
           {
              outputName = arg.substring(8).trim();
           }             
        }

        // Overwrite verbosity if the output goes to stdout
        if ("STDOUT".equalsIgnoreCase(outputName))
           verbose = 0;      

        for (String arg : args)
        {
           arg = arg.trim();
           
           if (arg.equals("-help"))
           {
               printOut("-help                : display this message", true);
               printOut("-verbose=<level>     : set the verbosity level [1..4]", true);
               printOut("                       0=silent, 1=default, 2=display block size (byte rounded)", true);
               printOut("                       3=display timings, 4=display extra information", true);
               printOut("-overwrite           : overwrite the output file if it already exists", true);
               printOut("-input=<inputName>   : mandatory name of the input file to encode or 'stdin'", true);
               printOut("-output=<outputName> : optional name of the output file (defaults to <input.knz>) or 'none' or 'stdout'", true);
               printOut("-block=<size>        : size of the input blocks, multiple of 16, max 1 GB (transform dependent), min 1 KB, default 1 MB", true);
               printOut("-entropy=<codec>     : entropy codec to use [None|Huffman*|ANS|Range|PAQ|FPAQ|TPAQ|CM]", true);
               printOut("-transform=<codec>   : transform to use [None|BWT*|BWTS|Snappy|LZ4|RLT|ZRLT|MTFT|RANK|TIMESTAMP]", true);
               printOut("                       EG: BWT+RANK or BWTS+MTFT (default is BWT+MTFT+ZRLT)", true);
               printOut("-checksum            : enable block checksum", true);
               printOut("-jobs=<jobs>         : number of concurrent jobs", true);
               printOut("", true);
               printOut("EG. java -cp kanzi.jar kanzi.app.BlockCompressor -input=foo.txt -output=foo.knz -overwrite "
                       + "-transform=BWT+MTFT+ZRLT -block=4m -entropy=FPAQ -verbose=3 -jobs=4", true);
               System.exit(0);
           }
           else if (arg.equals("-overwrite"))
           {
               overwrite = true;
           }
           else if (arg.equals("-checksum"))
           {
               checksum = true;
           }
           else if (arg.startsWith("-input="))
           {
              inputName = arg.substring(7).trim();
           }
           else if (arg.startsWith("-output="))
           {
              outputName = arg.substring(8).trim();
           }
           else if (arg.startsWith("-entropy="))
           {
              codec = arg.substring(9).trim().toUpperCase();
           }
           else if (arg.startsWith("-transform="))
           {
              transform = arg.substring(11).trim().toUpperCase();
           }
           else if (arg.startsWith("-block="))
           {
              arg = arg.substring(7).trim();
              String str = arg.toUpperCase();
              char lastChar = str.charAt(str.length()-1);
              int scale = 1;              

              try
              {
                 // Process K or M or G suffix
                 if ('K' == lastChar)
                 {
                    scale = 1024;
                    str = str.substring(0, str.length()-1);
                 }
                 else if ('M' == lastChar)
                 {
                    scale = 1024 * 1024;
                    str = str.substring(0, str.length()-1);
                 }
                 else if ('G' == lastChar)
                 {
                    scale = 1024 * 1024 * 1024;
                    str = str.substring(0, str.length()-1);
                 }
                 
                 blockSize = scale * Integer.parseInt(str);
              }
              catch (NumberFormatException e)
              {
                 System.err.println("Invalid block size provided on command line: "+arg);
                 System.exit(Error.ERR_INVALID_PARAM);
              }
           }
           else if (arg.startsWith("-jobs="))
           {
              arg = arg.substring(6).trim();
              
              try
              {
                 tasks = Integer.parseInt(arg);
                   
                 if (tasks < 1)
                    throw new NumberFormatException();
              }
              catch (NumberFormatException e)
              {
                 System.err.println("Invalid number of jobs provided on command line: "+arg);
                 System.exit(Error.ERR_INVALID_PARAM);
              }
           }
           else if ((!arg.startsWith("-verbose=")) && (!arg.startsWith("-output=")))
           {
              printOut("Warning: ignoring unknown option ["+ arg + "]", verbose>0);
           }
        }

        if (inputName == null)
        {
           System.err.println("Missing input file name, exiting ...");
           System.exit(Error.ERR_MISSING_PARAM);
        }

        if (outputName == null)
           outputName = inputName + ".knz";
        
        map.put("blockSize", blockSize);
        map.put("verbose", verbose);
        map.put("overwrite", overwrite);
        map.put("inputName", inputName);
        map.put("outputName", outputName);
        map.put("codec", codec);
        map.put("transform", transform);
        map.put("checksum", checksum);
        map.put("jobs", tasks);
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
