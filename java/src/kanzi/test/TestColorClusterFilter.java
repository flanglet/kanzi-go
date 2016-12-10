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
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.SliceIntArray;
import kanzi.filter.ColorClusterFilter;
import kanzi.filter.FastBilateralFilter;
import kanzi.filter.SobelFilter;


public class TestColorClusterFilter
   {
    public static void main(String[] args)
   {
       try
        {
            String fileName = (args.length > 0) ? args[0] : "r:\\kodim24.png";
            ImageIcon icon = new ImageIcon(fileName);
            Image image = icon.getImage();
            int w = image.getWidth(null) & -8;
            int h = image.getHeight(null) & -8;
            System.out.println(w+"x"+h);
            GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
            GraphicsConfiguration gc = gs.getDefaultConfiguration();
            BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
            img.getGraphics().drawImage(image, 0, 0, null);
            BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
            SliceIntArray source = new SliceIntArray(new int[w*h], 0);
            SliceIntArray temp = new SliceIntArray(new int[w*h], 0);
            SliceIntArray dest = new SliceIntArray(new int[w*h], 0);
            boolean applySobel = false;
            boolean applyBilateral = false;

            // Do NOT use img.getRGB(): it is more than 10 times slower than
            // img.getRaster().getDataElements()
            img.getRaster().getDataElements(0, 0, w, h, source.array);

            ColorClusterFilter effect = new ColorClusterFilter(w, h, w, 200, 8);
            effect.setShowBorders(true);
            //System.arraycopy(dest, 0, source, 0, w*h);
            effect.apply(source, dest);
            final int[] dArray = dest.array;

            if (applySobel == true)
            {
               // Apply Sobel filter
               SobelFilter sb = new SobelFilter(w, h);
               sb.apply(dest, temp);

               for (int i=0; i<w*h; i++)
               {
                  int pix = temp.array[i] & 0xFF;

                  // Add a line
                  if (pix < 0x40)
                     continue;

                  pix >>= 1;
                  int r = (dArray[i] >> 16) & 0xFF;
                  int g = (dArray[i] >>  8) & 0xFF;
                  int b =  dArray[i] & 0xFF;

                  r += pix;
                  g += pix;
                  b += pix;

                  if (r > 255)
                     r = 255;
                  if (g > 255)
                     g = 255;
                  if (b > 255)
                     b = 255;

                 dArray[i] = (r<<16) | (g<<8) | b;
               }
            }

            // Smooth the results by adding bilateral filtering
            if (applyBilateral == true)
            {
               FastBilateralFilter fbl = new FastBilateralFilter(w, h, 40.0f, 0.03f);
               fbl.apply(dest, dest);
            }

            img2.getRaster().setDataElements(0, 0, w, h, dest.array);

            //icon = new ImageIcon(img);
            JFrame frame = new JFrame("Original");
            frame.setBounds(150, 100, w, h);
            frame.add(new JLabel(new ImageIcon(image)));
            frame.setVisible(true);
            JFrame frame2 = new JFrame("Filter");
            frame2.setBounds(700, 150, w, h);
            ImageIcon newIcon = new ImageIcon(img2);
            frame2.add(new JLabel(newIcon));
            frame2.setVisible(true);

            // Speed test
            {
                SliceIntArray tmp = new SliceIntArray(new int[w*h], 0);
                System.arraycopy(source.array, 0, tmp.array, 0, w * h);
                System.out.println("Speed test");
                int iters = 1000;
                long before = 0, after = 0, delta = 0;

                for (int ii=0; ii<iters; ii++)
                {
                   effect = new ColorClusterFilter(w, h, w, 30, 12);
                   before = System.nanoTime();
                   effect.apply(source, tmp);
                   after = System.nanoTime();
                   delta += (after - before);
                }

                System.out.println("Elapsed [ms]: "+ delta/1000000+" ("+iters+" iterations)");
            }

            try
            {
                Thread.sleep(55000);
            }
            catch (Exception e)
            {
            }
        }
        catch (Exception e)
        {
            e.printStackTrace();
        }

        System.exit(0);
    }
}

