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
import kanzi.IndexedByteArray;
import kanzi.io.BlockListener;
import kanzi.io.CompressedOutputStream;
import kanzi.io.Error;
import kanzi.io.InfoPrinter;
import kanzi.io.NullOutputStream;


public class BlockCompressor implements Runnable, Callable<Integer>
{
   private static final int DEFAULT_BUFFER_SIZE = 32768;
   public static final int WARN_EMPTY_INPUT = -128;

   private boolean verbose;
   private boolean silent;
   private boolean overwrite;
   private boolean checksum;
   private String inputName;
   private String outputName;
   private String codec;
   private String transform;
   private int blockSize;
   private int jobs;
   private InputStream is;
   private CompressedOutputStream cos;
   private boolean ownPool;
   private final ExecutorService pool;
   private final List<BlockListener> listeners;

   
   public BlockCompressor(String[] args)
   {
      this(args, null, true);
   }
   
   
   public BlockCompressor(String[] args, ExecutorService threadPool)
   {
      this(args, threadPool, false);
   }
   
   
   protected BlockCompressor(String[] args, ExecutorService threadPool, boolean ownPool)
   {
      Map<String, Object> map = new HashMap<String, Object>();
      processCommandLine(args, map);
      this.verbose = (Boolean) map.get("verbose");
      this.silent = (Boolean) map.get("silent");
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
      this.ownPool = ownPool;
      this.listeners = new ArrayList<BlockListener>(10);
      
      if (this.verbose == true)
         this.addListener(new InfoPrinter(InfoPrinter.Type.ENCODING, System.out));
   }


   public static void main(String[] args)
   {
      BlockCompressor bc = null;

      try
      {
         bc = new BlockCompressor(args);
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
         /* ignore */
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
      printOut("Input file name set to '" + this.inputName + "'", this.verbose);
      printOut("Output file name set to '" + this.outputName + "'", this.verbose);
      printOut("Block size set to "+this.blockSize+ " bytes", this.verbose);
      printOut("Verbose set to "+this.verbose, this.verbose);
      printOut("Overwrite set to "+this.overwrite, this.verbose);
      printOut("Checksum set to "+this.checksum, this.verbose);
      String etransform = ("NONE".equals(this.transform)) ? "no" : this.transform;
      printOut("Using " + etransform + " transform (stage 1)", this.verbose);
      String ecodec = ("NONE".equals(this.codec)) ? "no" : this.codec;
      printOut("Using " + ecodec + " entropy codec (stage 2)", this.verbose);
      printOut("Using " + this.jobs + " job" + ((this.jobs > 1) ? "s" : ""), this.verbose);

      try
      {
         File output = null;
         
         if (!this.outputName.equalsIgnoreCase("NONE"))
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
         
         try
         {
            PrintStream ds = (this.verbose == true) ? System.out : null;
            OutputStream fos = (output == null) ? new NullOutputStream() : new FileOutputStream(output);
            this.cos = new CompressedOutputStream(this.codec, this.transform,
                 fos, this.blockSize, this.checksum,
                 ds, this.pool, this.jobs);
            
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
         System.err.println("Cannot open output file '"+ this.outputName+"' for writing: " + e.getMessage());
         return Error.ERR_CREATE_FILE;
      }

      try
      {
         File input = new File(this.inputName);
         this.is = new FileInputStream(input);
      }
      catch (Exception e)
      {
         System.err.println("Cannot open input file '"+ this.inputName+"': " + e.getMessage());
         return Error.ERR_OPEN_FILE;
      }

      // Encode
      printOut("Encoding ...", !this.silent);
      int read = 0;
      IndexedByteArray iba = new IndexedByteArray(new byte[DEFAULT_BUFFER_SIZE], 0);
      int len;
      long before = System.nanoTime();

      try
      {       
          while (true)
          {
             try
             {
                len = this.is.read(iba.array, 0, iba.array.length);
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
             this.cos.write(iba.array, 0, len);
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

       if (read == 0)
       {
          System.out.println("Empty input file ... nothing to do");
          return WARN_EMPTY_INPUT;
       }

       // Close streams to ensure all data are flushed
       this.dispose();

       long after = System.nanoTime();
       long delta = (after - before) / 1000000L; // convert to ms
       printOut("", !this.silent);
       printOut("Encoding:          "+delta+" ms", !this.silent);
       printOut("Input size:        "+read, !this.silent);
       printOut("Output size:       "+this.cos.getWritten(), !this.silent);
       printOut("Ratio:             "+this.cos.getWritten() / (float) read, !this.silent);

       if (delta > 0)
          printOut("Throughput (KB/s): "+(((read * 1000L) >> 10) / delta), !this.silent);

       printOut("", !this.silent);
       return 0;
    }


    private static void processCommandLine(String args[], Map<String, Object> map)
    {
        // Set default values
        int blockSize = 1024 * 1024; // 1 MB
        boolean verbose = false;
        boolean silent = false;
        boolean overwrite = false;
        boolean checksum = false;
        String inputName = null;
        String outputName = null;
        String codec = "HUFFMAN"; // default
        String transform = "BWT+MTF"; // default
        int tasks = 1;

        for (String arg : args)
        {
           arg = arg.trim();

           if (arg.equals("-help"))
           {
               printOut("-help                : display this message", true);
               printOut("-verbose             : display the block size at each stage (in bytes, floor rounding if fractional)", true);
               printOut("-silent              : silent mode, no output (except warnings and errors)", true);
               printOut("-overwrite           : overwrite the output file if it already exists", true);
               printOut("-input=<inputName>   : mandatory name of the input file to encode", true);
               printOut("-output=<outputName> : optional name of the output file (defaults to <input.knz>) or 'none' for dry-run", true);
               printOut("-block=<size>        : size of the input blocks, multiple of 8, max 512 MB (depends on transform), min 1KB, default 1MB", true);
               printOut("-entropy=<codec>     : entropy codec to use [None|Huffman*|ANS|Range|PAQ|FPAQ]", true);
               printOut("-transform=<codec>   : transform to use [None|BWT*|BWTS|Snappy|LZ4|RLT]", true);
               printOut("                       for BWT(S), an optional GST can be provided: [MTF|RANK|TIMESTAMP]", true);
               printOut("                       EG: BWT+RANK or BWTS+MTF (default is BWT+MTF)", true);
               printOut("-checksum            : enable block checksum", true);
               printOut("-jobs=<jobs>         : number of concurrent jobs", true);
               printOut("", true);
               printOut("EG. java -cp kanzi.jar kanzi.app.BlockCompressor -input=foo.txt -output=foo.knz -overwrite "
                       + "-transform=BWT+MTF -block=4m -entropy=FPAQ -verbose -jobs=4", true);
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
              int scale = 1;              

              try
              {
                 // Process K or M suffix
                 if ('K' == str.charAt(str.length()-1))
                 {
                    scale = 1024;
                    str = str.substring(0, str.length()-1);
                 }
                 else if ('M' == str.charAt(str.length()-1))
                 {
                    scale = 1024 * 1024;
                    str = str.substring(0, str.length()-1);
                 }
                 
                 blockSize = scale * Integer.parseInt(str);
              }
              catch (NumberFormatException e)
              {
                 System.err.println("Invalid block size provided on command line: "+arg);
                 System.exit(Error.ERR_BLOCK_SIZE);
              }
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

        if (outputName == null)
           outputName = inputName + ".knz";

        if ((silent == true) && (verbose == true))
        {
           printOut("Warning: both 'silent' and 'verbose' options were selected, ignoring 'verbose'", true);
           verbose = false;
        }

        map.put("blockSize", blockSize);
        map.put("verbose", verbose);
        map.put("silent", silent);
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
