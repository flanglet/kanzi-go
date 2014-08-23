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

package kanzi.app;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.OutputStream;
import java.io.PrintStream;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import kanzi.io.Error;
import kanzi.IndexedByteArray;
import kanzi.io.BlockListener;
import kanzi.io.CompressedInputStream;
import kanzi.io.InfoPrinter;



public class BlockDecompressor implements Runnable, Callable<Integer>
{
   private static final int DEFAULT_BUFFER_SIZE = 32768;

   private final boolean verbose;
   private final boolean silent;
   private final boolean overwrite;
   private final String inputName;
   private final String outputName;
   private CompressedInputStream cis;
   private OutputStream fos;
   private int jobs;
   private boolean ownPool;
   private final ExecutorService pool;
   private final List<BlockListener> listeners;
   
   
   public BlockDecompressor(String[] args)
   {
      this(args, null, true);
   }
   
   
   public BlockDecompressor(String[] args, ExecutorService threadPool)
   {
      this(args, threadPool, false);
   }
   
   
   protected BlockDecompressor(String[] args, ExecutorService threadPool, boolean ownPool)
   {
      Map<String, Object> map = new HashMap<String, Object>();
      processCommandLine(args, map);
      this.verbose = (Boolean) map.get("verbose");
      this.silent = (Boolean) map.get("silent");
      this.overwrite = (Boolean) map.get("overwrite");
      this.inputName = (String) map.get("inputName");
      this.outputName = (String) map.get("outputName");
      this.jobs = (Integer) map.get("jobs");
      this.pool = (this.jobs == 1) ? null : 
              ((threadPool == null) ? Executors.newCachedThreadPool() : threadPool);
      this.ownPool = ownPool;
      this.listeners = new ArrayList<BlockListener>(10);      
      
      if (this.verbose == true)
         this.addListener(new InfoPrinter(InfoPrinter.Type.DECODING, System.out));   
   }


   protected void dispose()
   {
      try
      {
         if (this.cis != null)
            this.cis.close();
      }
      catch (IOException ioe)
      {
         /* ignore */
      }

      try
      {
         if (this.fos != null)
            this.fos.close();
      }
      catch (IOException ioe)
      {
         /* ignore */
      }
      
      if ((this.pool != null) && (this.ownPool == true))
         this.pool.shutdown();
      
      this.listeners.clear();      
   }


   public static void main(String[] args)
   {
      BlockDecompressor bd = null;

      try
      {
         bd = new BlockDecompressor(args);
      }
      catch (Exception e)
      {
         System.err.println("Could not create the block codec: "+e.getMessage());
         System.exit(Error.ERR_CREATE_COMPRESSOR);
      }

      final int code = bd.call();

      if (code != 0)
         bd.dispose();

      System.exit(code);
   }


   @Override
   public void run()
   {
      this.call();
   }


   @Override
   public Integer call()
   {
      printOut("Input file name set to '" + this.inputName + "'", this.verbose);
      printOut("Output file name set to '" + this.outputName + "'", this.verbose);
      printOut("Verbose set to "+this.verbose, this.verbose);
      printOut("Overwrite set to "+this.overwrite, this.verbose);
      printOut("Using " + this.jobs + " job" + ((this.jobs > 1) ? "s" : ""), this.verbose);

      long read = 0;
      printOut("Decoding ...", !this.silent);
      File output;

      try
      {
         output = new File(this.outputName);

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
      }
      catch (Exception e)
      {
         System.err.println("Cannot open output file '"+ this.outputName+"' for writing: " + e.getMessage());
         return Error.ERR_CREATE_FILE;
      }

      try
      {
         // Create output steam (note: it creates the file yielding file.exists()
         // to return true so it must be called after the check).
         this.fos = new FileOutputStream(output);
      }
      catch (IOException e)
      {
         System.err.println("Cannot open output file '"+ this.outputName+"' for writing: " + e.getMessage());
         return Error.ERR_CREATE_FILE;
      }

      try
      {
         File input = new File(this.inputName);
         
         try
         {
            PrintStream ds = (this.verbose == true) ? System.out : null; 
            this.cis = new CompressedInputStream(new FileInputStream(input),
                 ds, this.pool, this.jobs);
            
            for (BlockListener bl : this.listeners)
               this.cis.addListener(bl);            
         }
         catch (Exception e)
         {
            System.err.println("Cannot create compressed stream: "+e.getMessage());
            return Error.ERR_CREATE_DECOMPRESSOR;    
         }
      }
      catch (Exception e)
      {
         System.err.println("Cannot open input file '"+ this.inputName+"': " + e.getMessage());
         return Error.ERR_OPEN_FILE;
      }

      long before = System.nanoTime();

      try
      {
         IndexedByteArray iba = new IndexedByteArray(new byte[DEFAULT_BUFFER_SIZE], 0);
         int decoded;

         // Decode next block
         do
         {
            decoded = this.cis.read(iba.array, 0, iba.array.length);

            if (decoded < 0)
            {
               System.err.println("Reached end of stream");
               return Error.ERR_READ_FILE;
            }
            
            try
            {
               if (decoded > 0)
               {
                  this.fos.write(iba.array, 0, decoded);
                  read += decoded;
               }
            }
            catch (Exception e)
            {
               System.err.print("Failed to write decompressed block to file '"+this.outputName+"': ");
               System.err.println(e.getMessage());
               return Error.ERR_READ_FILE;
            }
         }
         while (decoded == iba.array.length);
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

      // Close streams to ensure all data are flushed
      this.dispose();

      long after = System.nanoTime();
      long delta = (after - before) / 1000000L; // convert to ms
      printOut("", !this.silent);
      printOut("Decoding:          "+delta+" ms", !this.silent);
      printOut("Input size:        "+this.cis.getRead(), !this.silent);
      printOut("Output size:       "+read, !this.silent);
      
      if (delta > 0)
         printOut("Throughput (KB/s): "+(((read * 1000L) >> 10) / delta), !this.silent);
      
      printOut("", !this.silent);
      return 0;
   }


    private static void processCommandLine(String args[], Map<String, Object> map)
    {
        boolean verbose = false;
        boolean silent = false;
        boolean overwrite = false;
        String inputName = null;
        String outputName = null;
        int tasks = 1;

        for (String arg : args)
        {
           arg = arg.trim();

           if (arg.equals("-help"))
           {
              printOut("-help                : display this message", true);
              printOut("-verbose             : display the block size at each stage (in bytes, floor rounding if fractional)", true);
              printOut("-overwrite           : overwrite the output file if it already exists", true);
              printOut("-silent              : silent mode, no output (except warnings and errors)", true);
              printOut("-input=<inputName>   : mandatory name of the input file to decode", true);
              printOut("-output=<outputName> : optional name of the output file", true);
              printOut("-jobs=<jobs>         : number of parallel jobs", true);
              System.exit(0);
           }
           else if (arg.equals("-verbose"))
           {
              verbose = true;
           }
           else if (arg.equals("-silent"))
           {
              silent = true;
           }
           else if (arg.equals("-overwrite"))
           {
              overwrite = true;
           }
           else if (arg.startsWith("-input="))
           {
              inputName = arg.substring(7).trim();
           }
           else if (arg.startsWith("-output="))
           {
              outputName = arg.substring(8).trim();
           }
           else if (arg.startsWith("-jobs="))
           {
              arg = arg.substring(6).trim();
              String str = arg.toUpperCase();
              
              try
              {
                 tasks = Integer.parseInt(str);
              }
              catch (NumberFormatException e)
              {
                 System.err.println("Invalid number of jobs provided on command line: "+arg);
                 System.exit(Error.ERR_BLOCK_SIZE);
              }
           }
           else
           {
              printOut("Warning: ignoring unknown option ["+ arg + "]", true);
           }
        }

        if (inputName == null)
        {
           System.err.println("Missing input file name, exiting ...");
           System.exit(Error.ERR_MISSING_FILENAME);
        }

        if (inputName.endsWith(".knz") == false)
           printOut("Warning: the input file name does not end with the .KNZ extension", true);

        if (outputName == null)
        {
           outputName = (inputName.endsWith(".knz")) ? inputName.substring(0, inputName.length()-4)
                   : inputName + ".tmp";
        }

        if ((silent == true) && (verbose == true))
        {
           printOut("Warning: both 'silent' and 'verbose' options were selected, ignoring 'verbose'", true);
           verbose = false;
        }

        map.put("verbose", verbose);
        map.put("overwrite", overwrite);
        map.put("silent", silent);
        map.put("outputName", outputName);
        map.put("inputName", inputName);
        map.put("jobs", tasks);
    }


    private static void printOut(String msg, boolean print)
    {
       if ((print == true) && (msg != null))
          System.out.println(msg);
    }
    

    public boolean addListener(BlockListener bl)
    {
       return (bl != null) ? this.listeners.add(bl) : false;
    }

   
    public boolean removeListener(BlockListener bl)
    {
       return (bl != null) ? this.listeners.remove(bl) : false;
    }
}
