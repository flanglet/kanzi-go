package kanzi.app;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.Callable;
import kanzi.ColorModelType;
import kanzi.IndexedIntArray;
import kanzi.IntTransform;
import kanzi.transform.RSET;
import kanzi.util.ImageUtils;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.ReversibleYUVColorModelConverter;


public class ImageCompressor implements Runnable, Callable<Integer>
{
   public static final int WARN_EMPTY_INPUT = -128;

   private final int verbosity;
   private final boolean overwrite;
   private final String inputName;
   private final String outputName;
   private final Map<String, Object> argMap;
   private InputStream is;
   private OutputStream os;

   
   public ImageCompressor(String[] args)
   {
      this.argMap = new HashMap<String, Object>();
      processCommandLine(args, this.argMap);
      this.verbosity = (Integer) this.argMap.get("verbose");
      this.overwrite = (Boolean) this.argMap.get("overwrite");
      this.inputName = (String) this.argMap.get("inputName");
      this.outputName = (String) this.argMap.get("outputName");
   }


   public static void main(String[] args)
   {
      ImageCompressor ic = null;

      try
      {
         ic = new ImageCompressor(args);
      }
      catch (Exception e)
      {
         System.err.println("Could not create the codec: "+e.getMessage());
         System.exit(kanzi.io.Error.ERR_CREATE_COMPRESSOR);
      }

      final int code = ic.call();
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
         if (this.os != null)
            this.os.close();
      }
      catch (IOException ioe)
      {
         /* ignore */
      }
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
      printOut("Verbosity set to " + this.verbosity, printFlag);
      printOut("Overwrite set to " + this.overwrite, printFlag);
      long before = System.nanoTime();
      BlockCompressor bc = null;

      try
      {
         try
         {
            byte[] buf = processImage();
            ByteArrayInputStream bais = new ByteArrayInputStream(buf);
            this.argMap.put("inputStream", bais);
            bc = new BlockCompressor(this.argMap, null);
            int blockSize = (Integer) this.argMap.get("blockSize");

            if (blockSize < 0)
            {
               // Set block size to image data size
               blockSize = buf.length; 
               blockSize = (blockSize + 7) & -8;
               this.argMap.put("blockSize", blockSize);
            }
         }
         catch (IOException e)
         {
            System.err.println("Cannot compress file: "+e.getMessage());
            return kanzi.io.Error.ERR_OPEN_FILE;
         }
         
         try
         {
            this.argMap.put("verbose", 1);
            bc = new BlockCompressor(this.argMap, null);
         }
         catch (Exception e)
         {
            System.err.println("Cannot create block compressor: "+e.getMessage());
            return kanzi.io.Error.ERR_CREATE_COMPRESSOR;
         }         

         try
         {
            bc.run();
         }
         catch (Exception e)
         {
            System.err.println("Cannot compress file: "+e.getMessage());

            if (e instanceof kanzi.io.IOException)
               return ((kanzi.io.IOException) e).getErrorCode();

            return kanzi.io.Error.ERR_UNKNOWN;
         }
      }
      finally
      {
         // Close streams to ensure all data are flushed
         if (bc != null)
            bc.dispose();

         this.dispose();         
      }
      
      return 0;
    }


    private static void processCommandLine(String args[], Map<String, Object> map)
    {
        // Set default values
        int blockSize = -1;
        int verbose = 1;
        boolean overwrite = false;
        boolean checksum = false;
        String inputName = null;
        String outputName = null;
        String codec = "CM"; // default
        String transform = "NONE"; // default
        int tasks = 1;

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
               printOut("-input=<inputName>   : mandatory name of the input file to encode", true);
               printOut("-output=<outputName> : optional name of the output file (defaults to <input.knz>) or 'none' for dry-run", true);
               printOut("", true);
               printOut("EG. java -cp kanzi.jar kanzi.app.ImageCompressor -input=foo.txt -output=foo.knz -overwrite "
                       + "-entropy=CM -verbose=3", true);
               System.exit(0);
           }
           else if (arg.startsWith("-verbose="))
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
                  System.exit(kanzi.io.Error.ERR_INVALID_PARAM);
               }               
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
                 System.exit(kanzi.io.Error.ERR_INVALID_PARAM);
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
           System.exit(kanzi.io.Error.ERR_MISSING_PARAM);
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
    
    
    private byte[] processImage() throws IOException
    {
       FileInputStream fis = null;
       
       try
       {
         // Load image (supports PNM)
         String type = this.inputName.substring(this.inputName.lastIndexOf(".")+1);
         fis = new FileInputStream(this.inputName);
         ImageUtils.ImageInfo ii = ImageUtils.loadImage(fis, type);

         if (ii == null)
            throw new IOException("Cannot load image file "+this.inputName);
         
         // Adjust image width and height to ensure 16 <= dim0 < 32
         int dim = Math.min(ii.width, ii.height);
         int steps = 9;
         
         while ((dim >> steps) < 16)
            steps--;
  
         int mask = 1 << steps;
         final int w = (ii.width + mask - 1) & -mask;
         final int h = (ii.height + mask - 1) & -mask; 
         
         int[] data = ii.data;         
         ImageUtils iu = new ImageUtils(w, h); 
         data = iu.pad(data, w, h);
         
         // Run symmetries 
         boolean flip = false;
         boolean mirror = false;
         
         if (flip)
           data = iu.flip(data);
             
         if (mirror)
           data = iu.mirror(data);

         // Convert RGB to reversible YUV
         final int[] y = new int[w*h];
         final int[] u = new int[w*h];
         final int[] v = new int[w*h];        
         ColorModelConverter cvt = new ReversibleYUVColorModelConverter(w, h);
         cvt.convertRGBtoYUV(data, y, u, v, ColorModelType.YUV444);
         
         // Release memory
         data = null;
         ii = null;
     
         ByteArrayOutputStream baos = new ByteArrayOutputStream(w*h);
         processChannel(w, h, y, baos, steps);
         processChannel(w, h, u, baos, steps);
         processChannel(w, h, v, baos, steps);
         return baos.toByteArray(); 
       }
       finally
       {
          try
          {
            if (fis != null)
               fis.close();
          }
          catch (IOException ioe)
          {
            /* ignore */
          }          
       }
    }
    
    
   private static void processChannel(int w, int h, int[] buf,
      OutputStream os, int steps) throws IOException
   {  
      // Run RSE Transform to create a pyramid of images
      IndexedIntArray iia = new IndexedIntArray(buf, 0);
      IntTransform transform = new RSET(w, h, steps);   
      transform.forward(iia, iia);
      
      // Run Paeth filter on LL0 image to reduce entropy
      final int end0 = (Math.min(w, h) >> steps) - 1;
      int offs = w;
      
      for (int j=1; j<end0; j++)
      {         
         for (int i=1; i<end0; i++)
         {      
            final int a = buf[offs+i-1];
            final int b = buf[offs-w+i];
            final int c = buf[offs-w+i-1];
            final int p = a + b - c; 
            final int pa = Math.abs(p-a);
            final int pb = Math.abs(p-b);
            final int pc = Math.abs(p-c); 
            
            if ((pa <= pb) && (pa <= pc))
               buf[offs+i] = a;
            else if (pb <= pc)
               buf[offs+i] = b;
            else 
               buf[offs+i] = c;
         }
         
         offs += w;
      }          
  
      // Write image data to output stream
      for (int i=0; i<w*h; i++)
         os.write(buf[i]);           
   }
    
}
