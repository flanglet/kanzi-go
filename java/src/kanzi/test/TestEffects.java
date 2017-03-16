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


import java.awt.GraphicsConfiguration;
import java.awt.GraphicsDevice;
import java.awt.GraphicsEnvironment;
import java.awt.Image;
import java.awt.Transparency;
import java.awt.image.BufferedImage;
import java.io.FileInputStream;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.Global;
import kanzi.SliceIntArray;
import kanzi.IntFilter;
import kanzi.filter.BilateralFilter;
import kanzi.filter.BlurFilter;
import kanzi.filter.ColorClusterFilter;
import kanzi.filter.ContrastFilter;
import kanzi.filter.FastBilateralFilter;
import kanzi.filter.GaussianFilter;
import kanzi.filter.LightingEffect;
import kanzi.filter.MSSSaliencyFilter;
import kanzi.filter.MedianFilter;
import kanzi.filter.SharpenFilter;
import kanzi.filter.SobelFilter;
import kanzi.filter.seam.ContextResizer;
import kanzi.util.image.ImageUtils;



public class TestEffects
{
    public static void main(String[] args)
    {     
        try
        {
           if (args.length == 0)
           {
              System.out.println("Invalid command: type TestEffects -help");
              System.exit(1);
           }
              
            String fileName = "r:\\lena.jpg";
            String filterName = "";
            Integer param1 = null;
            Integer param2 = null;
            
            for (String arg : args)
            {
               arg = arg.trim();
              
               if (arg.equals("-help"))
               {
                   System.out.println("-help                : display this message");
                   System.out.println("-file=<filename>     : load image file with provided name");
                   System.out.println("-filter=<filtername> : apply named filter ");
                   System.out.println("                       [Bilateral|Blur|Contrast|ColorCluster|FastBilateral|Median|");
                   System.out.println("                        Gaussian|Lighting|Sobel|Saliency|ContextResizer|Sharpen]");
                   System.out.println("-arg1=<param>        : parameter used by the filter (EG. contract in percent)");
                   System.out.println("-arg2=<param>        : parameter used by the filter (EG. contract in percent)");
                   System.exit(0);
               }
               else if (arg.startsWith("-file="))
               {
                  fileName = arg.substring(6);
               }
               else if (arg.startsWith("-arg1="))
               {
                  param1 = Integer.parseInt(arg.substring(6));
               }
               else if (arg.startsWith("-arg2="))
               {
                  param2 = Integer.parseInt(arg.substring(6));
               }
               else if (arg.startsWith("-filter="))
               {
                   filterName = arg.substring(8).toUpperCase();
               }            
               else
               {
                   System.out.println("Warning: unknown option: ["+ arg + "]");
               }
            }
            
            // Load image (PPM/PGM supported)
            String type = fileName.substring(fileName.lastIndexOf(".")+1);
            ImageUtils.ImageInfo ii = ImageUtils.loadImage(new FileInputStream(fileName), type);

            final int w = ii.width & -16;
            final int h = ii.height & -16;
            
            if ((w < 0) || (h < 0))
            {
               System.err.println("Cannot find or read: "+fileName);
               System.exit(1);
            }

            if (filterName.isEmpty())
               filterName = "Filter";
            
            GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
            GraphicsConfiguration gc = gs.getDefaultConfiguration();
            BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
            img.getRaster().setDataElements(0, 0, w, h, ii.data);
            ImageIcon icon = new ImageIcon(img);
            System.out.println(fileName);
            System.out.println(filterName);
            System.out.println(w+"x"+h);
            JFrame frame = new JFrame("Original");
            frame.setBounds(100, 50, w, h);
            frame.add(new JLabel(icon));
            IntFilter effect;
            int adjust = 100 * 512 * 512 / (w * h); // adjust number of tests based on size
           
            switch (filterName)
            {               
               case "CONTEXTRESIZER" :
               {
                  // Context Resizer
                  frame.setVisible(true);            
                  int vertical = ContextResizer.VERTICAL;
                  int horizontal = ContextResizer.HORIZONTAL;
                  boolean debug = (param1 == null) ? false : (param1 == 1);                  
                  System.out.println("Debug: " + debug);
                  int scaling = 50000/w;  // unit is .01%
                  boolean fastMode = false;
                  effect = new ContextResizer(w/2, h, w, vertical, -scaling, fastMode, debug, null);
                  test(effect, img, filterName + " - left half - "+scaling+" seams", 0, 200, 150, 0, 0);
                  effect = new ContextResizer(w/2, h, w, vertical, -scaling, fastMode, debug, null);
                  test(effect, img, filterName + " - right half - "+scaling+" seams", w/2, 300, 250, 0, 0);
                  effect = new ContextResizer(w, h/2, w, horizontal, -scaling, fastMode, debug, null);
                  test(effect, img, filterName + " - upper half - "+scaling+" seams", 0, 400, 350, 0, 0);
                  effect = new ContextResizer(w, h/2, w, horizontal, -scaling, fastMode, debug, null);
                  test(effect, img, filterName + " - lower half - "+scaling+" seams", h*w/2, 500, 450, 0, 0);
                  effect = new ContextResizer(w/2, h/2, w, vertical|horizontal, -scaling, fastMode, debug, null);
                  test(effect, img, filterName + " - one quarter - "+scaling+" seams", h*w/4+w/4, 600, 550, 0, 0);
                  scaling = 50000/w; // unit is .01%
                  effect = new ContextResizer(w, h, w, vertical|horizontal, -scaling, fastMode, debug, null);
                  test(effect, img, filterName + " - full - "+scaling+" seams", 0, 700, 650, 2000*adjust/100, 30000);
                  break;  
               }                           
 
               case "COLORCLUSTER" :
               {
                  // Color Cluster
                  frame.setVisible(true);            
                  int clusters = (param1 == null) ? 20 : param1;
                  int iterations = (param2 == null) ? 5 : param1;
                  System.out.println("Clusters: " + clusters);
                  System.out.println("Iterations: " + iterations);
                  effect = new ColorClusterFilter(w/2, h, w, clusters, iterations);
                  test(effect, img, filterName + " - left half - "+clusters+" clusters", 0, 200, 150, 0, 0);
                  effect = new ColorClusterFilter(w/2, h, w, clusters, iterations);
                  test(effect, img, filterName + " - right half - "+clusters+" clusters", w/2, 300, 250, 0, 0);
                  effect = new ColorClusterFilter(w, h/2, w, clusters, iterations);
                  test(effect, img, filterName + " - upper half - "+clusters+" clusters", 0, 400, 350, 0, 0);
                  effect = new ColorClusterFilter(w, h/2, w, clusters, iterations);
                  test(effect, img, filterName + " - lower half - "+clusters+" clusters", h*w/2, 500, 450, 0, 0);
                  effect = new ColorClusterFilter(w/2, h/2, w, clusters, iterations);
                  clusters = 10;
                  test(effect, img, filterName + " - one quarter - "+clusters+" clusters", h*w/4+w/4, 600, 550, 0, 0);
                  clusters = 5 + (Global.log2(w*h) >> 10);
                  effect = new ColorClusterFilter(w, h, w, clusters, iterations);
                  test(effect, img, filterName + " - full - "+clusters+" clusters", 0, 700, 650, 1000*adjust/100, 30000);
                  break;  
               }

               case "LIGHTING" :
               {
                  // Lighting
                  frame.setVisible(true);            
                  int radius = Math.min(w, h) / 4;
                  int power = (param1 == null) ? 120 : param1;
                  System.out.println("Power: " + power + "%");
                  boolean bumpMapping = (param2 == null) ? false : param2 == 1;
                  effect = new LightingEffect(w/2, h, w, w/4, h/2, radius, power, bumpMapping);
                  test(effect, img, filterName + " - left half - radius "+radius, 0, 200, 150, 0, 0);
                  effect = new LightingEffect(w/2, h, w, w/4, h/2, radius, power, bumpMapping);
                  test(effect, img, filterName + " - right half - radius "+radius, w/2, 300, 250, 0, 0);
                  effect = new LightingEffect(w, h/2, w, w/2, h/4, radius, power, bumpMapping);
                  test(effect, img, filterName + " - upper half - radius "+radius, 0, 400, 350, 0, 0);
                  effect = new LightingEffect(w, h/2, w, w/2, h/4, radius, power, bumpMapping);
                  test(effect, img, filterName + " - lower half - radius "+radius, h*w/2, 500, 450, 0, 0);
                  effect = new LightingEffect(w/2, h/2, w, w/4, h/4, radius, power, bumpMapping);
                  test(effect, img, filterName + " - one quarter - radius "+radius, h*w/4+w/4, 600, 550, 0, 0);
                  radius *= 2;
                  effect = new LightingEffect(w, h, w, w/2, h/2, radius, power, bumpMapping);
                  test(effect, img, filterName + " - full - radius "+radius, 0, 700, 650, 10000*adjust/100, 30000);
                  break;  
               }
              
               case "BLUR" :
               {
                  // Blur
                  frame.setVisible(true);            
                  int radius = (param1 == null) ? 8 : param1;
                  System.out.println("Radius: " + radius);
                  effect = new BlurFilter(w/2, h, w, radius);
                  test(effect, img, filterName + " - left half", 0, 200, 150, 0, 0);
                  effect = new BlurFilter(w/2, h, w, radius);
                  test(effect, img, filterName + " - right half", w/2, 300, 250, 0, 0);
                  effect = new BlurFilter(w, h/2, w, radius);
                  test(effect, img, filterName + " - upper half", 0, 400, 350, 0, 0);
                  effect = new BlurFilter(w, h/2, w, radius);
                  test(effect, img, filterName + " - lower half", h*w/2, 500, 450, 0, 0);
                  effect = new BlurFilter(w/2, h/2, w, radius);
                  test(effect, img, filterName + " - one quarter", h*w/4+w/4, 600, 550, 0, 0);
                  effect = new BlurFilter(w, h, w, radius);
                  test(effect, img, filterName + " - full", 0, 700, 650, 1000*adjust/100, 30000);
                  break;
               }
           
               case "FASTBILATERAL" :
               {
                  // Fast Bilateral
                  frame.setVisible(true);            
                  float sigmaR = (param1 == null) ? 20.0f : (float) param1;
                  float sigmaD = (param2 == null) ? 0.03f : (float) param2;
                  System.out.println("SigmaR: " + sigmaR);
                  System.out.println("SigmaD: " + sigmaD);
                  effect = new FastBilateralFilter(w/2, h, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - left half", 0, 200, 150, 0, 0);
                  effect = new FastBilateralFilter(w/2, h, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - right half", w/2, 300, 250, 0, 0);
                  effect = new FastBilateralFilter(w, h/2, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - upper half", 0, 400, 350, 0, 0);
                  effect = new FastBilateralFilter(w, h/2, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - lower half", h*w/2, 500, 450, 0, 0);
                  effect = new FastBilateralFilter(w/2, h/2, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - one quarter", h*w/4+w/4, 600, 550, 0, 0);
                  effect = new FastBilateralFilter(w, h, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - full", 0, 700, 650, 1000*adjust/100, 30000);
                  break;
               }

               case "BILATERAL" :
               {
                  // Bilateral
                  frame.setVisible(true);            
                  int sigmaR = (param1 == null) ? 4 : param1;
                  int sigmaD = (param2 == null) ? 10 : param2;
                  System.out.println("SigmaR: " + sigmaR);
                  System.out.println("SigmaD: " + sigmaD);
                  effect = new BilateralFilter(w/2, h, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - left half", 0, 200, 150, 0, 0);
                  effect = new BilateralFilter(w/2, h, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - right half", w/2, 300, 250, 0, 0);
                  effect = new BilateralFilter(w, h/2, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - upper half", 0, 400, 350, 0, 0);
                  effect = new BilateralFilter(w, h/2, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - lower half", h*w/2, 500, 450, 0, 0);
                  effect = new BilateralFilter(w/2, h/2, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - one quarter", h*w/4+w/4, 600, 550, 0, 0);
                  effect = new BilateralFilter(w, h, w, sigmaR, sigmaD);
                  test(effect, img, filterName + " - full", 0, 700, 650, 25*adjust/100, 30000);
                  break;
               }

               case "GAUSSIAN" :
               {
                  // Gaussian
                  frame.setVisible(true);            
                  int channels = (param1 == null) ? 3 : param1;
                  int sigma16 = (param2 == null) ? 192 : param2;
                  System.out.println("Channels: " + channels);
                  System.out.println("Sigma16: " + sigma16);
                  effect = new GaussianFilter(w/2, h, w, sigma16, channels);
                  test(effect, img, filterName + " - left half", 0, 200, 150, 0, 0);
                  effect = new GaussianFilter(w/2, h, w, sigma16, channels);
                  test(effect, img, filterName + " - right half", w/2, 300, 250, 0, 0);
                  effect = new GaussianFilter(w, h/2, w, sigma16, channels);
                  test(effect, img, filterName + " - upper half", 0, 400, 350, 0, 0);
                  effect = new GaussianFilter(w, h/2, w, sigma16, channels);
                  test(effect, img, filterName + " - lower half", h*w/2, 500, 450, 0, 0);
                  effect = new GaussianFilter(w/2, h/2, w, sigma16, channels);
                  test(effect, img, filterName + " - one quarter", h*w/4+w/4, 600, 550, 0, 0);
                  effect = new GaussianFilter(w, h, w, sigma16, channels);
                  test(effect, img, filterName + " - full", 0, 700, 650, 2000*adjust/100, 30000);
                  break;
               }
                  
               case "CONTRAST" :
               {
                  // Contrast
                  frame.setVisible(true);            
                  int contrast = (param1 == null) ? 75 : param1; //per cent
                  System.out.println("Contrast: " + contrast + "%");
                  effect = new ContrastFilter(w/2, h, w, contrast);
                  test(effect, img, filterName + " - left half", 0, 200, 150, 0, 0);
                  effect = new ContrastFilter(w/2, h, w, contrast);
                  test(effect, img, filterName + " - right half", w/2, 300, 250, 0, 0);
                  effect = new ContrastFilter(w, h/2, w, contrast);
                  test(effect, img, filterName + " - upper half", 0, 400, 350, 0, 0);
                  effect = new ContrastFilter(w, h/2, w, contrast);
                  test(effect, img, filterName + " - lower half", h*w/2, 500, 450, 0, 0);
                  effect = new ContrastFilter(w/2, h/2, w, contrast);
                  test(effect, img, filterName + " - one quarter", h*w/4+w/4, 600, 550, 0, 0);
                  effect = new ContrastFilter(w, h, w, contrast);
                  test(effect, img, filterName + " - full", 0, 700, 650, 10000*adjust/100, 30000);
                  break;
               }

               case "SOBEL" :
               {
                  // Sobel
                  frame.setVisible(true);            
                  effect = new SobelFilter(w/2, h, w);
                  test(effect, img, filterName + " - left half", 0, 200, 150, 0, 0);
                  effect = new SobelFilter(w/2, h, w);
                  test(effect, img, filterName + " - right half", w/2, 300, 250, 0, 0);
                  effect = new SobelFilter(w, h/2, w);
                  test(effect, img, filterName + " - upper half", 0, 400, 350, 0, 0);
                  effect = new SobelFilter(w, h/2, w);
                  test(effect, img, filterName + " - lower half", h*w/2, 500, 450, 0, 0);
                  effect = new SobelFilter(w/2, h/2, w);
                  test(effect, img, filterName + " - one quarter", h*w/4+w/4, 600, 550, 0, 0);
                  effect = new SobelFilter(w, h, w);
                  test(effect, img, filterName + " - full", 0, 700, 650, 4000*adjust/100, 30000);
                  break;
               }
    
               case "SHARPEN" :
               {
                  // Sharpen
                  frame.setVisible(true);            
                  effect = new SharpenFilter(w/2, h, w);
                  test(effect, img, filterName + " - left half", 0, 200, 150, 0, 0);
                  effect = new SharpenFilter(w/2, h, w);
                  test(effect, img, filterName + " - right half", w/2, 300, 250, 0, 0);
                  effect = new SharpenFilter(w, h/2, w);
                  test(effect, img, filterName + " - upper half", 0, 400, 350, 0, 0);
                  effect = new SharpenFilter(w, h/2, w);
                  test(effect, img, filterName + " - lower half", h*w/2, 500, 450, 0, 0);
                  effect = new SharpenFilter(w/2, h/2, w);
                  test(effect, img, filterName + " - one quarter", h*w/4+w/4, 600, 550, 0, 0);
                  effect = new SharpenFilter(w, h, w);
                  test(effect, img, filterName + " - full", 0, 700, 650, 4000*adjust/100, 30000);
                  break;
               }
               
               case "MEDIAN" :
               {
                  // Median
                  frame.setVisible(true);    
                  int radius = (param1 == null) ? MedianFilter.DEFAULT_RADIUS : param1;
                  int threshold = (param2 == null) ? MedianFilter.DEFAULT_THRESHOLD : param1;
                  effect = new MedianFilter(w/2, h, w, radius, threshold);
                  test(effect, img, filterName + " - left half", 0, 200, 150, 0, 0);
                  effect = new MedianFilter(w/2, h, w, radius, threshold);
                  test(effect, img, filterName + " - right half", w/2, 300, 250, 0, 0);
                  effect = new MedianFilter(w, h/2, w, radius, threshold);
                  test(effect, img, filterName + " - upper half", 0, 400, 350, 0, 0);
                  effect = new MedianFilter(w, h/2, w, radius, threshold);
                  test(effect, img, filterName + " - lower half", h*w/2, 500, 450, 0, 0);
                  effect = new MedianFilter(w/2, h/2, w, radius, threshold);
                  test(effect, img, filterName + " - one quarter", h*w/4+w/4, 600, 550, 0, 0);
                  effect = new MedianFilter(w, h, w, radius, threshold);
                  test(effect, img, filterName + " - full", 0, 700, 650, 400*adjust/100, 30000);
                  break;
               }
                              
               case "SALIENCY" :
               {
                  // Saliency
                  frame.setVisible(true);            
                  effect = new MSSSaliencyFilter(w/2, h, w);
                  test(effect, img, filterName + " - left half", 0, 200, 150, 0, 0);
                  effect = new MSSSaliencyFilter(w/2, h, w);
                  test(effect, img, filterName + " - right half", w/2, 300, 250, 0, 0);
                  effect = new MSSSaliencyFilter(w, h/2, w);
                  test(effect, img, filterName + " - upper half", 0, 400, 350, 0, 0);
                  effect = new MSSSaliencyFilter(w, h/2, w);
                  test(effect, img, filterName + " - lower half", h*w/2, 500, 450, 0, 0);
                  effect = new MSSSaliencyFilter(w/2, h/2, w);
                  test(effect, img, filterName + " - one quarter", h*w/4+w/4, 600, 550, 0, 0);
                  effect = new MSSSaliencyFilter(w, h, w);
                  test(effect, img, filterName + " - full", 0, 700, 650, 1000*adjust/100, 30000);                 
                  break;
               }
               
               default:
               {
                  System.out.println("Unknown filter: '"+filterName+"'");
                  System.out.println("Supported filters: [Bilateral|Blur|Contrast|ColorCluster|FastBilateral|" +
                                     "Gaussian|Lighting|Sobel|Saliency|ContextResizer|Sharpen]");
                  System.exit(1);  
               }
            }
        }
        catch (Exception e)
        {
            e.printStackTrace();
        }

        System.exit(0);
    }

    
    public static void test(IntFilter effect, Image image, String title, 
            int offset, int xx, int yy, int iters, long sleep)
    {
         int w = image.getWidth(null);
         int h = image.getHeight(null);
         GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
         GraphicsConfiguration gc = gs.getDefaultConfiguration();
         BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         img.getGraphics().drawImage(image, 0, 0, null);
         BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         SliceIntArray source = new SliceIntArray(new int[w*h], w*h-offset, offset);
         SliceIntArray dest = new SliceIntArray(new int[w*h], w*h-offset, offset);
  
         // Do NOT use img.getRGB(): it is more than 10 times slower than
         // img.getRaster().getDataElements()
         img.getRaster().getDataElements(0, 0, w, h, source.array);
         System.out.println("Running test '" + title + "'");
         
         if (effect.apply(source, dest) == false)
         {
            System.out.println("Test failed");   
            return;
         }
         
         img2.getRaster().setDataElements(0, 0, w, h, dest.array);

         JFrame frame2 = new JFrame(title);
         frame2.setBounds(xx, yy, w, h);
         ImageIcon newIcon = new ImageIcon(img2);
         frame2.add(new JLabel(newIcon));
         frame2.setVisible(true);

         // Speed test
         if (iters > 0)
         {
             System.out.println("Speed test");
             long before = System.nanoTime();

             for (int ii=0; ii<iters; ii++)
                effect.apply(source, dest);

             long after = System.nanoTime();
             float mpixsec = (float)(w*h)*(float)(iters)/(float)(1024*1024)/((float)(after-before)/1000000000.f);
             System.out.println("Elapsed [ms]: "+ (after-before)/1000000+" ("+iters+" iterations)");
             System.out.println(String.format("%1.2f FPS", 1000000000*(float)iters/(after-before)));
             System.out.println(String.format("%1.2f MPix/s", mpixsec));
         }

         try
         {
             Thread.sleep(sleep);
         }
         catch (Exception e)
         {
         }
    }
}
