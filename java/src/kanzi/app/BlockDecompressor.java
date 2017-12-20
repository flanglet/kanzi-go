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
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import kanzi.Event;
import kanzi.io.Error;
import kanzi.SliceByteArray;
import kanzi.io.CompressedInputStream;
import kanzi.io.NullOutputStream;
import kanzi.Listener;



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
   private final List<Listener> listeners;


   public BlockDecompressor(Map<String, Object> map, ExecutorService threadPool)
   {
      this.verbosity = (Integer) map.remove("verbose");
      Boolean bForce = (Boolean) map.remove("overwrite");
      this.overwrite = (bForce == null) ? false : bForce;
      this.inputName = (String) map.remove("inputName");
      this.outputName = (String) map.remove("outputName");
      this.jobs = (Integer) map.remove("jobs");
      this.pool = (this.jobs < 2) ? null :
              ((threadPool == null) ? Executors.newCachedThreadPool() : threadPool);
      this.ownPool = (threadPool == null) && (this.pool != null);
      this.listeners = new ArrayList<>(10);

      if (this.verbosity > 2)
         this.addListener(new InfoPrinter(this.verbosity, InfoPrinter.Type.DECODING, System.out));

      if ((this.verbosity > 0) && (map.size() > 0))
      {
         for (String k : map.keySet())
            printOut("Ignoring invalid option [" + k + "]", this.verbosity>0);
      }      
   }
   

   public void dispose()
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
   }
   

   @Override
   public void run()
   {
      this.call();
   }


   @Override
   public Integer call()
   {
      boolean printFlag = this.verbosity > 2;
      printOut("Input file name set to '" + this.inputName + "'", printFlag);
      printOut("Output file name set to '" + this.outputName + "'", printFlag);
      printOut("Verbosity set to "+this.verbosity, printFlag);
      printOut("Overwrite set to "+this.overwrite, printFlag);
      
      if (this.jobs > 0)
         printOut("Using " + this.jobs + " job" + ((this.jobs > 1) ? "s" : ""), printFlag);

      long read = 0;
      printFlag = this.verbosity > 1;
      printOut("Decoding ...", printFlag);
      
      if (this.listeners.size() > 0)
      {
         Event evt = new Event(Event.Type.DECOMPRESSION_START, -1, 0);
         Listener[] array = this.listeners.toArray(new Listener[this.listeners.size()]);
         notifyListeners(array, evt);
      }
      
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
                  System.err.println("The output file exists and the 'force' command "
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
            Map<String, Object> ctx = new HashMap<>();
            ctx.put("pool", this.pool);
            ctx.put("jobs", this.jobs);
            this.cis = new CompressedInputStream(is, ctx);

            for (Listener bl : this.listeners)
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

         if (this.listeners.size() > 0)
         {
            Event evt = new Event(Event.Type.DECOMPRESSION_END, -1, this.cis.getRead());
            Listener[] array = this.listeners.toArray(new Listener[this.listeners.size()]);
            notifyListeners(array, evt);
         }          
      }

      long after = System.nanoTime();
      long delta = (after - before) / 1000000L; // convert to ms
      printOut("", this.verbosity>=1);
      printOut("Decoding:          "+delta+" ms", printFlag);
      printOut("Input size:        "+this.cis.getRead(), printFlag);
      printOut("Output size:       "+read, printFlag);
      printOut("Decoding "+this.inputName+": "+this.cis.getRead()+" => "+read+
          " bytes in "+delta+" ms", this.verbosity==1);

      if (delta > 0)
         printOut("Throughput (KB/s): "+(((read * 1000L) >> 10) / delta), printFlag);

      printOut("", this.verbosity>=1);
      return 0;
   }


    private static void printOut(String msg, boolean print)
    {
       if ((print == true) && (msg != null))
          System.out.println(msg);
    }


    public final boolean addListener(Listener bl)
    {
       return (bl != null) ? this.listeners.add(bl) : false;
    }


    public final boolean removeListener(Listener bl)
    {
       return (bl != null) ? this.listeners.remove(bl) : false;
    }
    
    
    static void notifyListeners(Listener[] listeners, Event evt)
    {
       for (Listener bl : listeners)
       {
          try 
          {
             bl.processEvent(evt);
          }
          catch (Exception e)
          {
            // Ignore exceptions in listeners
          }
       }
    }    
}
