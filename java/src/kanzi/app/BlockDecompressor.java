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
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import kanzi.Event;
import kanzi.Error;
import kanzi.SliceByteArray;
import kanzi.io.CompressedInputStream;
import kanzi.io.NullOutputStream;
import kanzi.Listener;



public class BlockDecompressor implements Runnable, Callable<Integer>
{
   private static final int DEFAULT_BUFFER_SIZE = 32768;
   private static final int DEFAULT_CONCURRENCY = 1;
   private static final int MAX_CONCURRENCY = 32;

   private int verbosity;
   private final boolean overwrite;
   private final String inputName;
   private final String outputName;
   private final int jobs;
   private final ExecutorService pool;
   private final List<Listener> listeners;


   public BlockDecompressor(Map<String, Object> map)
   {
      Boolean bForce = (Boolean) map.remove("overwrite");
      this.overwrite = (bForce == null) ? false : bForce;
      this.inputName = (String) map.remove("inputName");
      this.outputName = (String) map.remove("outputName");
      int concurrency = (Integer) map.remove("jobs");

      if (concurrency > MAX_CONCURRENCY)
      {
         System.err.println("Warning: the number of jobs is too high, defaulting to "+MAX_CONCURRENCY);
         concurrency = MAX_CONCURRENCY;
      }
                       
      this.jobs = (concurrency == 0) ? DEFAULT_CONCURRENCY : concurrency;
      this.pool = Executors.newFixedThreadPool(this.jobs);
      this.listeners = new ArrayList<>(10);
      this.verbosity = (Integer) map.remove("verbose");

      if ((this.verbosity > 0) && (map.size() > 0))
      {
         for (String k : map.keySet())
            printOut("Ignoring invalid option [" + k + "]", this.verbosity>0);
      }      
   }
   

   public void dispose()
   {
      if (this.pool != null)
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
      List<Path> files = new ArrayList<>();
      long read = 0;
      long before = System.nanoTime();
      
      try
      {
         Kanzi.createFileList(this.inputName, files);
      }
      catch (IOException e)
      {
         System.err.println(e.getMessage());
         return Error.ERR_OPEN_FILE;
      }
      
      if (files.isEmpty())
      {
         System.err.println("Cannot open input file '"+this.inputName+"'");
         return Error.ERR_OPEN_FILE;
      }
      
      Collections.sort(files);
      int nbFiles = files.size();
      
      boolean printFlag = this.verbosity > 2;
      String strFiles = (nbFiles > 1) ? " files" : " file";
      printOut(nbFiles+strFiles+" to decompress\n", this.verbosity > 0);
      printOut("Verbosity set to "+this.verbosity, printFlag);
      printOut("Overwrite set to "+this.overwrite, printFlag);
      printOut("Using " + this.jobs + " job" + ((this.jobs > 1) ? "s" : ""), printFlag);      
    
      if ((this.jobs>1) && ("STDOUT".equalsIgnoreCase(this.outputName)))
      {
         System.err.println("Cannot output to STDOUT with multiple jobs");
         return Error.ERR_CREATE_FILE;
      }   

      // Limit verbosity level when files are processed concurrently
      if ((this.jobs > 1) && (nbFiles > 1) && (this.verbosity > 1)) {
         printOut("Warning: limiting verbosity to 1 due to concurrent processing of input files.\n", true);
         this.verbosity = 1;;
      }
      
      if (this.verbosity > 2)
         this.addListener(new InfoPrinter(this.verbosity, InfoPrinter.Type.DECODING, System.out));
   
      int res = 0;

      try
      {
         boolean inputIsDir;
         String formattedOutName = this.outputName;
         String formattedInName = this.inputName;
         boolean specialOutput = ("NONE".equalsIgnoreCase(formattedOutName)) || 
            ("STDOUT".equalsIgnoreCase(formattedOutName));
         
         if (Files.isDirectory(Paths.get(this.inputName))) 
         {
            inputIsDir = true;

            if (formattedInName.endsWith(".") == true)
               formattedInName = formattedInName.substring(0, formattedInName.length()-1);

            if (formattedInName.endsWith(File.separator) == false)
               formattedInName += File.separator;
            
            if ((formattedOutName != null) && (specialOutput== false))          
            {
               if (Files.isDirectory(Paths.get(formattedOutName)) == false)
               {
                  System.err.println("Output must be an existing directory (or 'NONE')");
                  return Error.ERR_CREATE_FILE;
               }
               
               if (formattedOutName.endsWith(File.separator) == false)
                  formattedOutName += File.separator;
            }
         } 
         else
         {
            inputIsDir = false;
            
            if ((formattedOutName != null) && (specialOutput == false))          
            {
               if (Files.isDirectory(Paths.get(formattedOutName)) == true)
               {
                  System.err.println("Output must be a file (or 'NONE')");
                  return Error.ERR_CREATE_FILE;
               }
            }
         }

         // Run the task(s)
         if (nbFiles == 1)
         {
            String oName = this.outputName;
            String iName = files.get(0).toString();
            
            if (oName == null)
            {
               oName = iName + ".bak";
            }
            else if ((inputIsDir == true) && (specialOutput == false))
            {
               oName = formattedOutName + iName.substring(formattedInName.length()+1) + ".bak";
            }
            
            FileDecompressTask task = new FileDecompressTask(this.verbosity, this.overwrite, 
                     iName, oName, this.pool, this.jobs, this.listeners);

            FileDecompressResult fdr = task.call();
            res = fdr.code;
            read = fdr.read;
         }
         else
         {
            ArrayBlockingQueue<FileDecompressTask> queue = new ArrayBlockingQueue(nbFiles, true);
            int[] jobsPerTask = computeJobsPerTask(new int[nbFiles], this.jobs, nbFiles);
            int n = 0;
            
            // Create one task per file
            for (Path file : files)
            {
               String oName = formattedOutName;
               String iName = file.toString();

               if (oName == null)
               {
                  oName = iName + ".bak";
               }
               else if ((inputIsDir == true) && ("NONE".equalsIgnoreCase(oName) == false))
               {
                  oName = formattedOutName + iName.substring(formattedInName.length()) + ".knz";
               }
               
               FileDecompressTask task = new FileDecompressTask(this.verbosity, 
                  this.overwrite, iName, oName, this.pool, jobsPerTask[n++], this.listeners);
               queue.offer(task);               
            }

            List<FileDecompressWorker> workers = new ArrayList<>(this.jobs);
            
		  	   // Create one worker per job and run it. A worker calls several tasks sequentially.
            for (int i=0; i<this.jobs; i++)
               workers.add(new FileDecompressWorker(queue));
            
            // Invoke the tasks concurrently and wait for results
            // Using workers instead of tasks direclty, allows for early exit on failure
            for (Future<FileDecompressResult> result : this.pool.invokeAll(workers))
            {
               FileDecompressResult fdr = result.get();               
               read += fdr.read;

               if (fdr.code != 0)
               {
                  // Exit early by telling the workers that the queue is empty
                  queue.clear(); 
                  res = fdr.code;
               }              
            }           
         }
      }
      catch (Exception e)
      {
         System.err.println("An unexpected error occured: " + e.getMessage());
         res = Error.ERR_UNKNOWN;
      }
      
      long after = System.nanoTime();
      
      if (nbFiles > 1) 
      {
         long delta = (after - before) / 1000000L; // convert to ms
         printOut("", this.verbosity>0);
         printOut("Total encoding time: "+delta+" ms", this.verbosity > 0);
         printOut("Total input size: "+read+" byte"+((read>1)?"s":""), this.verbosity > 0);
	   }
      
      return res;
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
    
    
    private static int[] computeJobsPerTask(int[] jobsPerTask, int jobs, int tasks)
    {
       int q = (jobs <= tasks) ? 1 : jobs / tasks;
       int r = (jobs <= tasks) ? 0 : jobs - q*tasks;
       Arrays.fill(jobsPerTask, q);
       int n = 0;
      
       while (r != 0) 
       {
          jobsPerTask[n]++;
          r--;
          n++;
         
          if (n == tasks)
             n = 0;
       } 
       
       return jobsPerTask;
    }    
            
    
    
   static class FileDecompressResult
   {
       final int code;
       final long read; 


      public FileDecompressResult(int code, long read)
      {
         this.code = code;
         this.read = read;
      }  
   } 
   
   
   static class FileDecompressTask implements Callable<FileDecompressResult>
   {
      private final int verbosity;
      private final boolean overwrite;
      private final String inputName;
      private final String outputName;
      private CompressedInputStream cis;
      private OutputStream os;
      private final int jobs;
      private final ExecutorService pool;
      private final List<Listener> listeners;       


      public FileDecompressTask(int verbosity, boolean overwrite, String inputName, 
         String outputName, ExecutorService pool, int jobs, List<Listener> listeners)
      {
         this.verbosity = verbosity;
         this.overwrite = overwrite;
         this.inputName = inputName;
         this.outputName = outputName;
         this.jobs = jobs;
         this.pool = pool;
         this.listeners = listeners;
      }
      
      
      @Override
      public FileDecompressResult call() throws Exception
      {
         boolean printFlag = this.verbosity > 2;
         printOut("Input file name set to '" + this.inputName + "'", printFlag);
         printOut("Output file name set to '" + this.outputName + "'", printFlag);

         long read = 0;
         printFlag = this.verbosity > 1;
         printOut("\nDecoding "+this.inputName+" ...", printFlag);
         printOut("", this.verbosity>3);

         if (this.listeners.size() > 0)
         {
            Event evt = new Event(Event.Type.DECOMPRESSION_START, -1, 0);
            Listener[] array = this.listeners.toArray(new Listener[this.listeners.size()]);
            notifyListeners(array, evt);
         }

         if ("NONE".equalsIgnoreCase(this.outputName))
         {
            this.os = new NullOutputStream();
         }
         else if ("STDOUT".equalsIgnoreCase(this.outputName))
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
                     return new FileDecompressResult(Error.ERR_OUTPUT_IS_DIR, 0);
                  }

                  if (this.overwrite == false)
                  {
                     System.err.println("File '" + this.outputName + "' exists and " +
                        "the 'force' command line option has not been provided");
                     return new FileDecompressResult(Error.ERR_OVERWRITE_FILE, 0);
                  }

                  Path path1 = FileSystems.getDefault().getPath(this.inputName).toAbsolutePath();
                  Path path2 = FileSystems.getDefault().getPath(this.outputName).toAbsolutePath();

                  if (path1.equals(path2))
                  {
                     System.err.println("The input and output files must be different");
                     return new FileDecompressResult(Error.ERR_CREATE_FILE, 0);
                  }
               }
               
               try
               {
                  this.os = new FileOutputStream(output);
               }
               catch (IOException e1)
               {
                  if (this.overwrite == false)
                     throw e1;
                  
                  try 
                  {
                     // Attempt to create the full folder hierarchy to file
                     Files.createDirectories(FileSystems.getDefault().getPath(this.outputName).getParent());
                     this.os = new FileOutputStream(output);
                  } 
                  catch (IOException e2)
                  {
                     throw e1;
                  }
               }               
            }
            catch (Exception e)
            {
               System.err.println("Cannot open output file '"+ this.outputName+"' for writing: " + e.getMessage());
               return new FileDecompressResult(Error.ERR_CREATE_FILE, 0);
            }
         }

         InputStream is;

         try
         {
            is = ("STDIN").equalsIgnoreCase(this.inputName) ? System.in :
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
               return new FileDecompressResult(Error.ERR_CREATE_DECOMPRESSOR, 0);
            }
         }
         catch (Exception e)
         {
            System.err.println("Cannot open input file '"+ this.inputName+"': " + e.getMessage());
            return new FileDecompressResult(Error.ERR_OPEN_FILE, 0);
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
                  return new FileDecompressResult(Error.ERR_READ_FILE,  this.cis.getRead());
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
                  return new FileDecompressResult(Error.ERR_READ_FILE, this.cis.getRead());
               }
            }
            while (decoded == sa.array.length);
         }
         catch (kanzi.io.IOException e)
         {
            System.err.println(e.getMessage());
            return new FileDecompressResult(e.getErrorCode(), this.cis.getRead());
         }
         catch (Exception e)
         {
            System.err.println("An unexpected condition happened. Exiting ...");
            System.err.println(e.getMessage());
            return new FileDecompressResult(Error.ERR_UNKNOWN, this.cis.getRead());
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
         printOut("", this.verbosity>1);
         printOut("Decoding:          "+delta+" ms", printFlag);
         printOut("Input size:        "+this.cis.getRead(), printFlag);
         printOut("Output size:       "+read, printFlag);
         printOut("Decoding "+this.inputName+": "+this.cis.getRead()+" => "+read+
             " bytes in "+delta+" ms", this.verbosity==1);

         if (delta > 0)
            printOut("Throughput (KB/s): "+(((read * 1000L) >> 10) / delta), printFlag);

         printOut("", this.verbosity>1);
         return new FileDecompressResult(0, this.cis.getRead());
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
      }
   }
   
   
   
   static class FileDecompressWorker implements Callable<FileDecompressResult>
   {
      private final ArrayBlockingQueue<FileDecompressTask> queue;

      public FileDecompressWorker(ArrayBlockingQueue<FileDecompressTask> queue)
      {
         this.queue = queue;
      }
       
      @Override
      public FileDecompressResult call() throws Exception
      {
         int res = 0;
         long read = 0;
         
         while (res == 0)
         {
            FileDecompressTask task = this.queue.poll();
            
            if (task == null)
               break;

            FileDecompressResult result = task.call();
            res = result.code;
            read += result.read;
         }

         return new FileDecompressResult(res, read);
      }
   }   
}
