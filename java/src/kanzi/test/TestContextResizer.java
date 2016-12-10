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
import java.awt.Rectangle;
import java.awt.Transparency;
import java.awt.image.BufferedImage;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.SliceIntArray;
import kanzi.filter.seam.ContextResizer;


public class TestContextResizer
{
    public static void main(String[] args)
    {
        try
        {
            String fileName = "c:\\temp\\lena.jpg";
            boolean debug = false;
            boolean vertical = false;
            boolean horizontal = false;
            int tests = 0;
            int effectPerMil = 100;
            boolean fileProvided = false;
            
            for (String arg : args)
            {
               arg = arg.trim();
               
               if (arg.equals("-help"))
               {
                   System.out.println("-help               : display this message");
                   System.out.println("-debug              : display the computed geodesics");
                   System.out.println("-file=<filename>    : load image file with provided name");
                   System.out.println("-strength=<percent> : number of geodesics to create (in percent of dimension)");
                   System.out.println("-vertical           : process vertical geodesics");
                   System.out.println("-horizontal         : process horizontal geodesics");
                   System.out.println("-speedtest=<steps>  : run an extra speed test");
                   System.exit(0);
               }
               else if (arg.equals("-debug"))
               {
                   debug = true;
                   System.out.println("Debug set to true");
               }
               else if (arg.startsWith("-file="))
               {
                  fileName = arg.substring(6);
                  fileProvided = true;
               }
               else if (arg.equals("-vertical"))
               {
                   vertical = true;
                   System.out.println("Vertical set to true");
               }
               else if (arg.equals("-horizontal"))
               {
                   horizontal = true;
                   System.out.println("Horizontal set to true");
               }
               else if (arg.startsWith("-strength="))
               {
                  arg = arg.substring(10);
                  
                  try
                  {
                     effectPerMil = 10*Integer.parseInt(arg);                   
                  }
                  catch (NumberFormatException e)
                  {
                     System.err.println("Invalid effect strength (percentage) provided on command line: "+arg);
                  }
               }
               else if (arg.startsWith("-speedtest="))
               {
                  arg = arg.substring(11);
                  
                  try
                  {
                     tests = Integer.parseInt(arg);                   
                  }
                  catch (NumberFormatException e)
                  {
                     System.err.println("Invalid number of speed tests provided on command line: "+arg);
                  }
                  
                  System.out.println("Speed test set to " + tests);
               }
               else
               {
                   System.out.println("Warning: unknown option: ["+ arg + "]");
               }
            }

            if ((vertical == false) && (horizontal == false))
            {
               System.out.println("Warning: no direction has been selected, selecting both");
               vertical = true;
               horizontal = true;
            }

            if (fileProvided == false)
                System.out.println("No image file name provided on command line, using default value");

            System.out.println("File name set to '" + fileName + "'");
            System.out.println("Strength set to "+(effectPerMil/10)+"%");
            ImageIcon icon = new ImageIcon(fileName);
            Image image = icon.getImage();
            int w = image.getWidth(null);
            int h = image.getHeight(null);
            
            if ((w<0) || (h<0))
            {
                System.err.println("Cannot read or decode input file '"+fileName+"'");
                System.exit(1);
            }
            
            System.out.println("Image dimensions: "+w+"x"+h);
            GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
            GraphicsConfiguration gc = gs.getDefaultConfiguration();
            BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
            img.getGraphics().drawImage(image, 0, 0, null);
            JFrame frame = new JFrame("Original");
            frame.setBounds(50, 50, w, h);
            frame.add(new JLabel(icon));
            frame.setVisible(true);
            SliceIntArray src = new SliceIntArray(new int[w*h], 0);
            SliceIntArray tmp = new SliceIntArray(new int[w*h], 0);
            SliceIntArray dst = new SliceIntArray(new int[w*h], 0);

            // Do NOT use img.getRGB(): it is more than 10 times slower than
            // img.getRaster().getDataElements()
            img.getRaster().getDataElements(0, 0, w, h, src.array);
            ContextResizer effect;

            int dir = 0;
            
            if (vertical == true) 
                dir |= ContextResizer.VERTICAL;
            
            if (horizontal == true)
                dir |= ContextResizer.HORIZONTAL;

            effect = new ContextResizer(w, h,  w, dir, -effectPerMil, false, debug, null);            
            effect.apply(src, tmp);

            Rectangle bounds = gs.getDefaultConfiguration().getBounds();
            BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
            img2.getRaster().setDataElements(0, 0, w, h, tmp.array);
            JFrame frame2 = new JFrame("Filter");
            frame2.setBounds(Math.max(10, Math.min(w+50,bounds.width-5*w/4)), 80, w, h);
            ImageIcon icon2 = new ImageIcon(img2);
            frame2.add(new JLabel(icon2));
            frame2.setVisible(true);

            // Speed test
            if (tests > 0)
            {
                ExecutorService pool = Executors.newFixedThreadPool(4);
                System.out.println("Speed test");
                long sum = 0;
                int iter = tests;
                System.out.print("Accurate mode");
                effect = new ContextResizer(w, h, w, dir, -effectPerMil, false, false, pool);

                for (int ii=0; ii<iter; ii++)
                {
                   if ((ii % (iter/10)) == (iter/10) - 1)
                      System.out.print(".");
                   
                   img.getRaster().getDataElements(0, 0, w, h, src);
                   long before = System.nanoTime();
                   effect.apply(src, tmp);
                   long after = System.nanoTime();
                   sum += (after - before);
                }

                System.out.println("\nElapsed [ms]: "+ sum/1000000+" ("+iter+" iterations)"); 
                System.out.println(1000000000*(long)iter/sum+" FPS");
                System.out.println("Fast mode");
                sum = 0;
                effect = new ContextResizer(w, h, w, dir, -effectPerMil, true, false, pool);

                for (int ii=0; ii<iter; ii++)
                {
                   img.getRaster().getDataElements(0, 0, w, h, src);
                   long before = System.nanoTime();
                   effect.apply(src, tmp);
                   long after = System.nanoTime();
                   sum += (after - before);
                }

                System.out.println("Elapsed [ms]: "+ sum/1000000+" ("+iter+" iterations)");
                System.out.println(1000000000*(long)iter/sum+" FPS");
            }

            Thread.sleep(4000);
                        
            JFrame frame3 = new JFrame("Animation");
            frame3.setBounds(700, 150, w, h);
            frame3.setResizable(true);
            frame3.setUndecorated(true);
            ImageIcon newIcon = new ImageIcon(img2);
            frame3.add(new JLabel(newIcon));
//            frame3.setVisible(true);
//            BufferedImage img3 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
//            final int w0 = w;
//
//            // Add delay to make sure that the frame is visible before creating back buffer
//            Thread.sleep(10);
//            frame3.createBufferStrategy(1);
//            ExecutorService pool = Executors.newFixedThreadPool(4);
//            System.arraycopy(src.array, 0, dst.array, 0, src.array.length);
//            int iters = 1;
//            BufferStrategy bufferStrategy = frame3.getBufferStrategy();
//            Graphics2D g = (Graphics2D) bufferStrategy.getDrawGraphics();
//            
//            for (int ii=0; ii<iters; ii++)
//            {
//               while (w >= 3*w0/4)
//               {   
//                  int scaling = (-1000/w) < -10 ? -1000/w : -10; // in per mil
//                  effect = new ContextResizer(w, h, w0, ContextResizer.VERTICAL,
//                          scaling, true, true, pool);
//                  effect.apply(src, dst);
//                  img3.getRaster().setDataElements(0, 0, w0, h, dst.array);
//                  g.drawImage(img3, 0, 0, null);
//                  img3.getRaster().setDataElements(0, 0, w0, h, dst.array);
//                  //frame3.setBounds(Math.max(10, Math.min(w0+w0+50,bounds.width-5*w0/4)), 100, w0, h);
//                  bufferStrategy.show();
//                  int offset = 0;
//
//                  for (int j=h; j>0; j--, offset+=w0)
//                     src.array[offset+w-1] = 0;
//                  
//                  Thread.sleep(150);
//                  effect.setDebug(false);
//                  effect.apply(src, dst);
//                  img3.getRaster().setDataElements(0, 0, w0, h, dst.array);
//                  g.drawImage(img3, 0, 0, w0, h, null);
//                  frame3.setBounds(Math.max(10, Math.min(w0+w0+50,bounds.width-5*w0/4)), 100, w0, h);
//                  bufferStrategy.show();
//                  System.arraycopy(dst.array, 0, src.array, 0, dst.array.length);
//                  offset = 0;
//
//                  for (int j=h; j>0; j--, offset+=w0)
//                     src.array[offset+w-1] = 0;
//
//                  Thread.sleep(150);
//                  w--;
//               }
//
//               w = w0;
//               img.getRaster().getDataElements(0, 0, w0, h, src.array);             
//               Thread.sleep(15000);
//            }
            
            Thread.sleep(99000);
        }
        catch (Exception e)
        {
            e.printStackTrace();
        }

        System.exit(0);
    }
}
