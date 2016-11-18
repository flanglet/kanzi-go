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
import java.io.File;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import javax.imageio.ImageIO;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.SliceIntArray;
import kanzi.IntFilter;
import kanzi.filter.ParallelFilter;
import kanzi.filter.SobelFilter;


public class TestSobelFilter
{
    public static void main(String[] args)
    {
        try
        {
            String fileName = (args.length > 0) ? args[0] : "c:\\temp\\lena.jpg";
            Image image = ImageIO.read(new File(fileName));
            int w = image.getWidth(null);
            int h = image.getHeight(null);
            
            if ((w < 0) || (h < 0))
            {
               System.err.println("Cannot find or read: "+fileName);
               System.exit(1);
            }

            ImageIcon icon = new ImageIcon(image);
            int adjust = 100 * 512 * 512 / (w * h); // adjust number of tests based on size
            System.out.println(fileName);
            System.out.println(w+"x"+h);
            JFrame frame = new JFrame("Original");
            frame.setBounds(100, 50, w, h);
            frame.add(new JLabel(icon));
            frame.setVisible(true);  
            IntFilter effect;
            ExecutorService pool = Executors.newFixedThreadPool(4);
            
            // One thread
            effect = new SobelFilter(w, h, w);
            test(effect, icon, "Sobel - 1 thread", 0, 200, 150, 5000*adjust/100, 0);
            IntFilter[] effects = new IntFilter[4];
            int dir = SobelFilter.HORIZONTAL | SobelFilter.VERTICAL;
                        
            // 4 threads, vertical split 
            // Overlap by increasing dim and do not process boundaries to avoid artefacts
            // Setting 'process boundaries' to true means that the first and last
            // column/row of each filter is going to be duplicated (since the Sobel
            // kernel cannot be applied at the boundary), creating visible artefacts.
            // Overlapping filters and no boundary processing avoids the artefacts.
            for (int i=0; i<effects.length; i++)
               effects[i] = new SobelFilter(w/effects.length+2, h+2, w, dir, false);
            
            effect = new ParallelFilter(w, h, w, pool, effects, ParallelFilter.VERTICAL);            
            test(effect, icon, "Sobel - 4 threads - vertical split", 0, 300, 350, 10000*adjust/100, 0);
            
            // 4 threads, horizontal split
            // Overlap filters and do not process boundaries to avoid artefacts
            for (int i=0; i<effects.length; i++)
               effects[i] = new SobelFilter(w+2, h/effects.length+2, w, dir, false);
            
            effect = new ParallelFilter(w, h, w, pool, effects, ParallelFilter.HORIZONTAL);            
            test(effect, icon, "Sobel - 4 threads - horizontal split", 0, 400, 550, 10000*adjust/100, 30000);
        }
        catch (Exception e)
        {
            e.printStackTrace();
        }

        System.exit(0);
    }

    
    public static void test(IntFilter effect, ImageIcon icon, String title, 
            int offset, int xx, int yy, int iters, long sleep)
    {
         Image image = icon.getImage();
         int w = image.getWidth(null);
         int h = image.getHeight(null);
         GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
         GraphicsConfiguration gc = gs.getDefaultConfiguration();
         BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         img.getGraphics().drawImage(image, 0, 0, null);
         BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         SliceIntArray source = new SliceIntArray(new int[w*h], offset);
         SliceIntArray dest = new SliceIntArray(new int[w*h], offset);

         // Do NOT use img.getRGB(): it is more than 10 times slower than
         // img.getRaster().getDataElements()
         img.getRaster().getDataElements(0, 0, w, h, source.array);
         effect.apply(source, dest);
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
             System.out.println(title);
             long before = System.nanoTime();

             for (int ii=0; ii<iters; ii++)
                effect.apply(source, dest);

             long after = System.nanoTime();
             System.out.println("Elapsed [ms]: "+ (after-before)/1000000+" ("+iters+" iterations)");
             System.out.println(1000000000*(long)iters/(after-before)+" FPS");
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
