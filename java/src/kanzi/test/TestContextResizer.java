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

package kanzi.test;

import java.awt.Graphics2D;
import java.awt.GraphicsConfiguration;
import java.awt.GraphicsDevice;
import java.awt.GraphicsEnvironment;
import java.awt.Image;
import java.awt.Rectangle;
import java.awt.Transparency;
import java.awt.image.BufferStrategy;
import java.awt.image.BufferedImage;
import java.util.Arrays;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.IndexedIntArray;
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
            boolean speed = false;
            int effectPct = 10;
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
                   System.out.println("-speedtest          : run an extra speed test");
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
                     int pct = Integer.parseInt(arg);

                     if (pct < 1)
                     {
                         System.err.println("The minimum strength is 1%, the provided value is "+arg);
                         System.exit(1);
                     }
                     else if (pct > 90)
                     {
                         System.err.println("The maximum strength is 90%, the provided value is  "+arg);
                         System.exit(1);
                     }
                     else
                         effectPct = pct;                     
                  }
                  catch (NumberFormatException e)
                  {
                     System.err.println("Invalid effect strength (percentage) provided on command line: "+arg);
                  }
               }
               else if (arg.equals("-speedtest"))
               {
                   speed = true;
                   System.out.println("Speed test set to true");
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
            System.out.println("Strength set to "+effectPct+"%");
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
            IndexedIntArray src = new IndexedIntArray(new int[w*h], 0);
            IndexedIntArray tmp = new IndexedIntArray(new int[w*h], 0);
            IndexedIntArray dst = new IndexedIntArray(new int[w*h], 0);

            // Do NOT use img.getRGB(): it is more than 10 times slower than
            // img.getRaster().getDataElements()
            img.getRaster().getDataElements(0, 0, w, h, src.array);
            ContextResizer effect;

            Arrays.fill(tmp.array, 0);
            int dir = 0;
            int min = Integer.MAX_VALUE;
            
            if (vertical == true) 
            {
                dir |= ContextResizer.VERTICAL;
                min = Math.min(min, w);
            }
            
            if (horizontal == true)
            {
                dir |= ContextResizer.HORIZONTAL;
                min = Math.min(min, h);
            }

            effect = new ContextResizer(w, h,  w, dir,
                    ContextResizer.SHRINK, min * effectPct / 100, false, debug, null);            
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
            if (speed == true)
            {
                ExecutorService pool = Executors.newFixedThreadPool(4);
                System.out.println("Speed test");
                long sum = 0;
                int iter = 1000;
                System.out.println("Accurate mode");
                effect = new ContextResizer(w, h, w, dir,
                       ContextResizer.SHRINK, min * effectPct/100, false, false, pool);

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
                System.out.println("Fast mode");
                sum = 0;
                effect = new ContextResizer(w, h, w, dir,
                       ContextResizer.SHRINK, min * effectPct/100, true, false, pool);

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
            frame3.setVisible(true);
            BufferedImage img3 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
            final int w0 = w;

            // Add delay to make sure that the frame is visible before creating back buffer
            Thread.sleep(10);
            frame3.createBufferStrategy(1);
            ExecutorService pool = Executors.newFixedThreadPool(4);
            System.arraycopy(src.array, 0, dst.array, 0, src.array.length);
            int iters = 1;
            
            for (int ii=0; ii<iters; ii++)
            {
               while (w>= 3*w0/4)
               {
                  effect = new ContextResizer(w, h, w0, ContextResizer.VERTICAL,
                          ContextResizer.SHRINK, 1, true, true, pool);
                  effect.apply(src, dst);
                  img3.getRaster().setDataElements(0, 0, w0, h, dst.array);
                  BufferStrategy bufferStrategy = frame3.getBufferStrategy();
                  Graphics2D g = (Graphics2D) bufferStrategy.getDrawGraphics();
                  g.drawImage(img3, 0, 0, w0, h, null);
                  img3.getRaster().setDataElements(0, 0, w0, h, dst.array);
                  frame3.setBounds(Math.max(10, Math.min(w0+w0+50,bounds.width-5*w0/4)), 100, w0, h);
                  bufferStrategy.show();
                  int offset = 0;

                  for (int j=h; j>0; j--, offset+=w0)
                     src.array[offset+w-1] = 0;

                  Thread.sleep(150);
                  effect.setDebug(false);
                  effect.apply(src, dst);
                  img3.getRaster().setDataElements(0, 0, w0, h, dst.array);
                  g.drawImage(img3, 0, 0, w0, h, null);
                  frame3.setBounds(Math.max(10, Math.min(w0+w0+50,bounds.width-5*w0/4)), 100, w0, h);
                  bufferStrategy.show();
                  System.arraycopy(dst.array, 0, src.array, 0, dst.array.length);
                  offset = 0;

                  for (int j=h; j>0; j--, offset+=w0)
                     src.array[offset+w-1] = 0;

                  Thread.sleep(150);
                  w--;
               }

               w = w0;
               img.getRaster().getDataElements(0, 0, w0, h, src.array);             
               Thread.sleep(51000);
            }
            
            Thread.sleep(4000);
        }
        catch (Exception e)
        {
            e.printStackTrace();
        }

        System.exit(0);
    }
}
