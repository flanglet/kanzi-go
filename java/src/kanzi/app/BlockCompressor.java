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
import kanzi.SliceByteArray;
import kanzi.io.ByteFunctionFactory;
import kanzi.io.CompressedOutputStream;
import kanzi.Error;
import kanzi.io.NullOutputStream;
import kanzi.Listener;


public class BlockCompressor implements Runnable, Callable<Integer>
{
   private static final int DEFAULT_BUFFER_SIZE = 32768;
   private static final int DEFAULT_BLOCK_SIZE  = 1024*1024; 
   private static final int DEFAULT_CONCURRENCY = 1;
   private static final int MAX_CONCURRENCY = 32;   
   public static final int WARN_EMPTY_INPUT = -128;
   
   private int verbosity;
   private final boolean overwrite;
   private final boolean checksum;
   private final String inputName;
   private final String outputName;
   private final String codec;
   private final String transform;
   private final int blockSize;
   private final int level; // command line compression level
   private final int jobs;
   private final List<Listener> listeners;
   private final ExecutorService pool;

   
   public BlockCompressor(Map<String, Object> map)
   {
      this.level = (Integer) map.remove("level");
      Boolean bForce = (Boolean) map.remove("overwrite");
      this.overwrite = (bForce == null) ? false : bForce;
      this.inputName = (String) map.remove("inputName");
      this.outputName = (String) map.remove("outputName");
      String strTransf;
      String strCodec;
      
      if (this.level >= 0)
      {
         String tranformAndCodec = getTransformAndCodec(this.level);
         String[] tokens = tranformAndCodec.split("&");
         strTransf = tokens[0];
         strCodec = tokens[1];
      } 
      else 
      {
         strTransf = (String) map.remove("transform");
         strCodec = (String) map.remove("entropy");
      }

      this.codec = (strCodec == null) ? "ANS0" : strCodec;
      Integer iBlockSize = (Integer) map.remove("block");
      this.blockSize = (iBlockSize == null) ? DEFAULT_BLOCK_SIZE : iBlockSize;
      
      // Extract transform names. Curate input (EG. NONE+NONE+xxxx => xxxx)          
      ByteFunctionFactory bff = new ByteFunctionFactory();      
      this.transform = (strTransf == null) ? "BWT+RANK+ZRLT" : bff.getName(bff.getType(strTransf));
      Boolean bChecksum = (Boolean) map.remove("checksum");
      this.checksum = (bChecksum == null) ? false : bChecksum;
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


   // Return status (success = 0, error < 0)
   @Override
   public Integer call()
   { 
      List<Path> files = new ArrayList<>();
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
         System.err.println("Cannot access input file '"+this.inputName+"'");
         return Error.ERR_OPEN_FILE;
      }

      Collections.sort(files);
      int nbFiles = files.size();
      
      boolean printFlag = this.verbosity > 2;
      String strFiles = (nbFiles > 1) ? " files" : " file";
      printOut(nbFiles+strFiles+" to compress\n", this.verbosity > 0);
      printOut("Block size set to " + this.blockSize + " bytes", printFlag);
      printOut("Verbosity set to " + this.verbosity, printFlag);
      printOut("Overwrite set to " + this.overwrite, printFlag);
      printOut("Checksum set to " +  this.checksum, printFlag);

      if (this.level < 0)
      {
         String etransform = ("NONE".equals(this.transform)) ? "no" : this.transform;
         printOut("Using " + etransform + " transform (stage 1)", printFlag);
         String ecodec = ("NONE".equals(this.codec)) ? "no" : this.codec;
         printOut("Using " + ecodec + " entropy codec (stage 2)", printFlag);
      }
      else
      {
         printOut("Compression level set to " +  this.level, printFlag);
      }

      printOut("Using " + this.jobs + " job" + ((this.jobs > 1) ? "s" : ""), printFlag);      

      if ((this.jobs>1) && ("STDOUT".equalsIgnoreCase(this.outputName)))
      {
         System.err.println("Cannot output to STDOUT with multiple jobs");
         return Error.ERR_CREATE_FILE;
      }      
            
      // Limit verbosity level when files are processed concurrently
      if ((this.jobs > 1) && (nbFiles > 1) && (this.verbosity > 1)) {
         printOut("Warning: limiting verbosity to 1 due to concurrent processing of input files.\n", true);
         this.verbosity = 1;
      }
      
      if (this.verbosity > 2)
         this.addListener(new InfoPrinter(this.verbosity, InfoPrinter.Type.ENCODING, System.out));
         
      int res = 0;
      long read = 0;
      long written = 0;            

      try
      {
         boolean inputIsDir;
         String formattedOutName = this.outputName;
         String formattedInName = this.inputName;
         boolean specialOutput = ("NONE".equalsIgnoreCase(formattedOutName)) || 
            ("STDOUT".equalsIgnoreCase(formattedOutName));
         
         if (Files.isDirectory(Paths.get(formattedInName))) 
         {
            inputIsDir = true;

            if (formattedInName.endsWith(".") == true)
               formattedInName = formattedInName.substring(0, formattedInName.length()-1);

            if (formattedInName.endsWith(File.separator) == false)
               formattedInName += File.separator;
            
            if ((formattedOutName != null) && (specialOutput == false))          
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
            String oName = formattedOutName;
            String iName = files.get(0).toString();
            
            if (oName == null)
            {
               oName = iName + ".knz";
            }
            else if ((inputIsDir == true) && (specialOutput == false))
            {
               oName = formattedOutName + iName.substring(formattedInName.length()+1) + ".knz";
            }
            
            FileCompressTask task = new FileCompressTask(this.verbosity, this.overwrite, this.checksum, 
                     iName, oName, this.codec, this.transform, this.blockSize, 
                     this.pool, this.jobs, this.listeners);

            FileCompressResult fcr = task.call();
            res = fcr.code;
            read = fcr.read;
            written = fcr.written;
         }
         else
         {
            ArrayBlockingQueue<FileCompressTask> queue = new ArrayBlockingQueue(nbFiles, true);
            int[] jobsPerTask = computeJobsPerTask(new int[nbFiles], this.jobs, nbFiles);
            int n = 0;

            // Create one task per file
            for (Path file : files)
            {
               String oName = formattedOutName;
               String iName = file.toString();

               if (oName == null)
               {
                  oName = iName + ".knz";
               }
               else if ((inputIsDir == true) && (specialOutput == false))
               {
                  oName = formattedOutName + iName.substring(formattedInName.length()) + ".knz";
               }
            
               FileCompressTask task = new FileCompressTask(this.verbosity, this.overwrite, this.checksum, 
                  iName, oName, this.codec, this.transform, this.blockSize, 
                  this.pool, jobsPerTask[n++], this.listeners);
               queue.offer(task);               
            }
       
            List<FileCompressWorker> workers = new ArrayList<>(this.jobs);
            
		  	   // Create one worker per job and run it. A worker calls several tasks sequentially.
            for (int i=0; i<this.jobs; i++)
               workers.add(new FileCompressWorker(queue));
            
            // Invoke the tasks concurrently and wait for results
            // Using workers instead of tasks direclty, allows for early exit on failure
            for (Future<FileCompressResult> result : this.pool.invokeAll(workers))
            {
               FileCompressResult fcr = result.get();   
               read += fcr.read;
               written += fcr.written;

               if (fcr.code != 0)
               {
                  // Exit early by telling the workers that the queue is empty
                  queue.clear();
                  res = fcr.code;
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
         printOut("Total output size: "+written+" byte"+((written>1)?"s":""), this.verbosity > 0);
         
         if (read > 0)
         {
            float f = written / (float) read;
            printOut("Compression ratio: "+String.format("%1$.6f", f), this.verbosity > 0);
         }
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
   
    
    private static String getTransformAndCodec(int level)
    {
       switch (level)
       {
          case 0 :
             return "NONE&NONE";
             
          case 1 :
             return "TEXT+LZ4&HUFFMAN";
             
          case 2 :
             return "BWT+RANK+ZRLT&ANS0";
             
          case 3 :
             return "BWT+RANK+ZRLT&FPAQ";
             
          case 4 :
             return "BWT&CM";
             
          case 5 :
             return "X86+RLT+TEXT&TPAQ";
             
          default :
             return "Unknown&Unknown";             
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
    

    static class FileCompressResult
    {
       final int code;
       final long read; 
       final long written; 


       public FileCompressResult(int code, long read, long written)
       {
          this.code = code;
          this.read = read;
          this.written = written;
       }  
    }
    
  
    static class FileCompressTask implements Callable<FileCompressResult>
    {
      private final int verbosity;
      private final boolean overwrite;
      private final boolean checksum;
      private final String inputName;
      private final String outputName;
      private final String codec;
      private final String transform;
      private final int blockSize;
      private final ExecutorService pool;
      private final int jobs;
      private InputStream is;
      private CompressedOutputStream cos;
      private final List<Listener> listeners;


      public FileCompressTask(int verbosity, boolean overwrite, boolean checksum, 
         String inputName, String outputName, String codec, String transform, 
         int blockSize, ExecutorService pool, int jobs, List<Listener> listeners)
      {
         this.verbosity = verbosity;
         this.overwrite = overwrite;
         this.checksum = checksum;
         this.inputName = inputName;
         this.outputName = outputName;
         this.codec = codec;
         this.transform = transform;
         this.blockSize = blockSize;
         this.pool = pool;
         this.jobs = jobs;
         this.listeners = listeners;
      }
      
       
      @Override
      public FileCompressResult call() throws Exception
      {
         boolean printFlag = this.verbosity > 2;
         printOut("Input file name set to '" + this.inputName + "'", printFlag);
         printOut("Output file name set to '" + this.outputName + "'", printFlag);

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
                     return new FileCompressResult(Error.ERR_OUTPUT_IS_DIR, 0, 0);
                  }

                  if (this.overwrite == false)
                  {
                     System.err.println("File '" + this.outputName + "' exists and " +
                        "the 'force' command line option has not been provided");
                     return new FileCompressResult(Error.ERR_OVERWRITE_FILE, 0, 0);
                  }

                  Path path1 = FileSystems.getDefault().getPath(this.inputName).toAbsolutePath();
                  Path path2 = FileSystems.getDefault().getPath(this.outputName).toAbsolutePath();

                  if (path1.equals(path2))
                  {
                     System.err.println("The input and output files must be different");
                     return new FileCompressResult(Error.ERR_CREATE_FILE, 0, 0); 
                  }
               }
             
               try
               {
                  os = new FileOutputStream(output);
               }
               catch (IOException e1)
               {
                  if (this.overwrite == false)
                     throw e1;
                  
                  try 
                  {
                     // Attempt to create the full folder hierarchy to file
                     Files.createDirectories(FileSystems.getDefault().getPath(this.outputName).getParent());
                     os = new FileOutputStream(output);
                  } 
                  catch (IOException e2)
                  {
                     throw e1;
                  }
               }
            }

            try
            {
               Map<String, Object> ctx = new HashMap<>();
               ctx.put("blockSize", this.blockSize);
               ctx.put("checksum", this.checksum);
               ctx.put("pool", this.pool);
               ctx.put("jobs", this.jobs);
               ctx.put("codec", this.codec);
               ctx.put("transform", this.transform);
               this.cos = new CompressedOutputStream(os, ctx);

               for (Listener bl : this.listeners)
                  this.cos.addListener(bl);
            }
            catch (Exception e)
            {
               System.err.println("Cannot create compressed stream: "+e.getMessage());
               return new FileCompressResult(Error.ERR_CREATE_COMPRESSOR, 0, 0);
            }
         }
         catch (Exception e)
         {
            System.err.println("Cannot open output file '"+this.outputName+"' for writing: " + e.getMessage());
            return new FileCompressResult(Error.ERR_CREATE_FILE, 0, 0);
         }

         try
         {
            this.is = (this.inputName.equalsIgnoreCase("STDIN")) ? System.in : new FileInputStream(this.inputName);
         }
         catch (Exception e)
         {
            System.err.println("Cannot open input file '"+this.inputName+"': " + e.getMessage());
            return new FileCompressResult(Error.ERR_OPEN_FILE, 0, 0);
         }

         // Encode
         printFlag = this.verbosity > 1;
         printOut("\nEncoding "+this.inputName+" ...", printFlag);
         printOut("", this.verbosity>3);
         long read = 0;
         SliceByteArray sa = new SliceByteArray(new byte[DEFAULT_BUFFER_SIZE], 0);
         int len;

         if (this.listeners.size() > 0)
         {
            Event evt = new Event(Event.Type.COMPRESSION_START, -1, 0);
            Listener[] array = this.listeners.toArray(new Listener[this.listeners.size()]);
            notifyListeners(array, evt);
         }

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
                  return new FileCompressResult(Error.ERR_READ_FILE, read, this.cos.getWritten());
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
            return new FileCompressResult(e.getErrorCode(), read, this.cos.getWritten());
         }
         catch (Exception e)
         {
            System.err.println("An unexpected condition happened. Exiting ...");
            System.err.println(e.getMessage());
            return new FileCompressResult(Error.ERR_UNKNOWN, read, this.cos.getWritten());
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
            printOut("Input file " + this.inputName + " is empty... nothing to do", this.verbosity > 0);
            return new FileCompressResult(0, read, this.cos.getWritten());
         }

         long after = System.nanoTime();
         long delta = (after - before) / 1000000L; // convert to ms
         printOut("", this.verbosity>1);
         printOut("Encoding:          "+delta+" ms", printFlag);
         printOut("Input size:        "+read, printFlag);
         printOut("Output size:       "+this.cos.getWritten(), printFlag);
         float f = this.cos.getWritten() / (float) read;
         printOut("Compression ratio: "+String.format("%1$.6f", f), printFlag);
         printOut("Encoding "+this.inputName+": "+read+" => "+this.cos.getWritten()+
            " bytes in "+delta+" ms", this.verbosity==1);

         if (delta > 0)
            printOut("Throughput (KB/s): "+(((read * 1000L) >> 10) / delta), printFlag);

         printOut("", this.verbosity>1);

         if (this.listeners.size() > 0)
         {
            Event evt = new Event(Event.Type.COMPRESSION_END, -1, this.cos.getWritten());
            Listener[] array = this.listeners.toArray(new Listener[this.listeners.size()]);
            notifyListeners(array, evt);
         }          

         return new FileCompressResult(0, read, this.cos.getWritten());         
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
            System.err.println("Compression failure for '" + this.inputName+"' : " + ioe.getMessage());
            System.exit(Error.ERR_WRITE_FILE);
         }
      }      
   }
    
   
   static class FileCompressWorker implements Callable<FileCompressResult>
   {
      private final ArrayBlockingQueue<FileCompressTask> queue;


      public FileCompressWorker(ArrayBlockingQueue<FileCompressTask> queue)
      {
         this.queue = queue;
      }
       
      @Override
      public FileCompressResult call() throws Exception
      {
         int res = 0;
         long read = 0;         
         long written = 0;
         
         while (res == 0)
         {
            FileCompressTask task = this.queue.poll();
            
            if (task == null)
               break;

            FileCompressResult result = task.call();
            res = result.code;
            read += result.read;
            written += result.written;
         }

         return new FileCompressResult(res, read, written);
      }
   }

}
