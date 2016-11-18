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
import java.io.PrintStream;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import kanzi.io.Error;
import kanzi.SliceByteArray;
import kanzi.io.BlockListener;
import kanzi.io.CompressedInputStream;
import kanzi.io.InfoPrinter;
import kanzi.io.NullOutputStream;



public class BlockDecompressor implements Runnable, Callable<Integer>
{
   private static final int DEFAULT_BUFFER_SIZE = 32768;

   private final int verbosity;
   private final boolean overwrite;
   private final String inputName;
   private final String outputName;   
   private CompressedInputStream cis;
   private OutputStream os;
   private final int jobs;
   private final boolean ownPool;
   private final ExecutorService pool;
   private final List<BlockListener> listeners;
   
    
   public BlockDecompressor(Map<String, Object> map, ExecutorService threadPool, boolean ownPool)
   {
      this.verbosity = (Integer) map.get("verbose");
      this.overwrite = (Boolean) map.get("overwrite");
      this.inputName = (String) map.get("inputName");
      this.outputName = (String) map.get("outputName");
      this.jobs = (Integer) map.get("jobs");
      this.pool = (this.jobs == 1) ? null : 
              ((threadPool == null) ? Executors.newCachedThreadPool() : threadPool);
      this.ownPool = (threadPool == null) && (this.pool != null);
      this.listeners = new ArrayList<BlockListener>(10);      
      
      if (this.verbosity > 1)
         this.addListener(new InfoPrinter(this.verbosity, InfoPrinter.Type.DECODING, System.out));   
   }
   
   
   public BlockDecompressor(String[] args, ExecutorService threadPool)
   {
      Map<String, Object> map = new HashMap<String, Object>();
      processCommandLine(args, map);
      this.verbosity = (Integer) map.get("verbose");
      this.overwrite = (Boolean) map.get("overwrite");
      this.inputName = (String) map.get("inputName");
      this.outputName = (String) map.get("outputName");
      this.jobs = (Integer) map.get("jobs");
      this.pool = (this.jobs == 1) ? null : 
              ((threadPool == null) ? Executors.newCachedThreadPool() : threadPool);
      this.ownPool = (threadPool == null) && (this.pool != null);
      this.listeners = new ArrayList<BlockListener>(10);      
      
      if (this.verbosity > 1)
         this.addListener(new InfoPrinter(this.verbosity, InfoPrinter.Type.DECODING, System.out));   
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
         System.err.println("Decompression failure: " + ioe.getMessage());
         System.exit(Error.ERR_WRITE_FILE);
      }

      try
      {
         if (this.os != null)
            this.os.close();
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
         bd = new BlockDecompressor(args, null);
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
      boolean printFlag = this.verbosity > 1;
      printOut("Input file name set to '" + this.inputName + "'", printFlag);
      printOut("Output file name set to '" + this.outputName + "'", printFlag);
      printOut("Verbosity set to "+this.verbosity, printFlag);
      printOut("Overwrite set to "+this.overwrite, printFlag);
      printOut("Using " + this.jobs + " job" + ((this.jobs > 1) ? "s" : ""), printFlag);

      long read = 0;
      boolean silent = this.verbosity < 1;
      printOut("Decoding ...", !silent);

      if (this.outputName.equalsIgnoreCase("NONE"))
      {
         this.os = new NullOutputStream();
      }
      else if (this.outputName.equalsIgnoreCase("STDOUT"))
      {
         this.os = System.out;
      }
      else
      {
         try
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
         }
         catch (Exception e)
         {
            System.err.println("Cannot open output file '"+ this.outputName+"' for writing: " + e.getMessage());
            return Error.ERR_CREATE_FILE;
         }

         try
         {
            // Create output stream (note: it creates the file yielding file.exists()
            // to return true so it must be called after the check).
            this.os = new FileOutputStream(this.outputName);
         }
         catch (IOException e)
         {
            System.err.println("Cannot open output file '"+ this.outputName+"' for writing: " + e.getMessage());
            return Error.ERR_CREATE_FILE;
         }
      } 

      InputStream is;
      
      try
      {
         is = (this.inputName.equalsIgnoreCase("STDIN")) ? System.in :
            new FileInputStream(new File(this.inputName));
         
         try
         {
            PrintStream ds = (printFlag == true) ? System.out : null; 
            this.cis = new CompressedInputStream(is, ds, this.pool, this.jobs);
            
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
         SliceByteArray sa = new SliceByteArray(new byte[DEFAULT_BUFFER_SIZE], 0);
         int decoded;

         // Decode next block
         do
         {
            decoded = this.cis.read(sa.array, 0, sa.length);

            if (decoded < 0)
            {
               System.err.println("Reached end of stream");
               return Error.ERR_READ_FILE;
            }
            
            try
            {
               if (decoded > 0)
               {
                  this.os.write(sa.array, 0, decoded);
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
         while (decoded == sa.array.length);
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
            is.close();
         } 
         catch (IOException e)
         {
            // Ignore
         }         
      }
      
      long after = System.nanoTime();
      long delta = (after - before) / 1000000L; // convert to ms
      printOut("", !silent);
      printOut("Decoding:          "+delta+" ms", !silent);
      printOut("Input size:        "+this.cis.getRead(), !silent);
      printOut("Output size:       "+read, !silent);
      
      if (delta > 0)
         printOut("Throughput (KB/s): "+(((read * 1000L) >> 10) / delta), !silent);
      
      printOut("", !silent);
      return 0;
   }


    private static void processCommandLine(String args[], Map<String, Object> map)
    {
        int verbose = 1;
        boolean overwrite = false;
        String inputName = null;
        String outputName = null;
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
              printOut("-verbose=<level>     : set the verbosity level [0..4]", true);
              printOut("                       0=silent, 1=default, 2=display block size (byte rounded)", true);
              printOut("                       3=display timings, 4=display extra information", true);
              printOut("-overwrite           : overwrite the output file if it already exists", true);
              printOut("-input=<inputName>   : mandatory name of the input file to decode or 'stdin'", true);
              printOut("-output=<outputName> : optional name of the output file or 'none' or 'stdout'", true);
              printOut("-jobs=<jobs>         : number of concurrent jobs", true);
              printOut("", true);
              printOut("EG. java -cp kanzi.jar kanzi.app.BlockDecompressor -input=foo.knz -overwrite -verbose=2 -jobs=2", true);
              System.exit(0);
           }
           else if (arg.equals("-overwrite"))
           {
              overwrite = true;
           }
           else if (arg.startsWith("-input="))
           {
              inputName = arg.substring(7).trim();
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

        if (("STDIN".equalsIgnoreCase(inputName) == false) && (inputName.toLowerCase().endsWith(".knz") == false))
           printOut("Warning: the input file name does not end with the .KNZ extension", verbose>0);

        if (outputName == null)
        {
           outputName = (inputName.toLowerCase().endsWith(".knz")) ? inputName.substring(0, inputName.length()-4)
                   : inputName + ".tmp";
        }
        
        map.put("verbose", verbose);
        map.put("overwrite", overwrite);
        map.put("outputName", outputName);
        map.put("inputName", inputName);
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
