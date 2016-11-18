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
import javax.imageio.ImageIO;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.SliceIntArray;
import kanzi.IntFilter;
import kanzi.filter.BilateralFilter;
import kanzi.filter.FastBilateralFilter;


public class TestBilateralFilter
{

    public static void main(String[] args) throws Exception
    {
        String fileName = (args.length > 0) ? args[0] : "C:\\temp\\lena.jpg";
        Image image = ImageIO.read(new File(fileName));
        int w = image.getWidth(null);
        int h = image.getHeight(null);
        System.out.println(w+"x"+h);
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        img.getGraphics().drawImage(image, 0, 0, null);
        BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        BufferedImage img3 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        SliceIntArray src = new SliceIntArray(new int[w*h], 0);
        SliceIntArray dst1 = new SliceIntArray(new int[w*h], 0);
        SliceIntArray dst2 = new SliceIntArray(new int[w*h], 0);

        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img.getRaster().getDataElements(0, 0, w, h, src.array);

        float sigmaR = 30.0f;
        float sigmaD = 0.03f;
        IntFilter fbf = new FastBilateralFilter(w, h, w, sigmaR, sigmaD);
        fbf.apply(src, dst1);
        img2.getRaster().setDataElements(0, 0, w, h, dst1.array);
        IntFilter bf = new BilateralFilter(w, h, w, 3, 8);
        bf.apply(src, dst2);
        img3.getRaster().setDataElements(0, 0, w, h, dst2.array);
        JFrame frame = new JFrame("Original");
        frame.setBounds(200, 100, w, h);
        frame.add(new JLabel(new ImageIcon(image)));
        frame.setVisible(true);
        JFrame frame2 = new JFrame("Fast Bilateral Filter");
        frame2.setBounds(400, 200, w, h);
        ImageIcon newIcon = new ImageIcon(img2);
        frame2.add(new JLabel(newIcon));
        frame2.setVisible(true);
        JFrame frame3 = new JFrame("Full Bilateral Filter");
        frame3.setBounds(700, 300, w, h);
        ImageIcon newIcon3 = new ImageIcon(img3);
        frame3.add(new JLabel(newIcon3));
        frame3.setVisible(true);

        {
           int iters = 1500;
           System.out.println("Fast Bilateral: speed test ("+iters+" iterations)");
           long before = System.nanoTime();

           for (int ii=0; ii<iters; ii++)
           {
               fbf.apply(src, dst1);
           }

           long after = System.nanoTime();
           System.out.println("Elapsed [ms]: "+(after-before)/1000000L);
           System.out.println("");
        }
        
        {
           int iters = 10;
           System.out.println("Full Bilateral: speed test ("+iters+" iterations)");
           long before = System.nanoTime();

           for (int ii=0; ii<iters; ii++)
           {
               bf.apply(src, dst2);
           }

           long after = System.nanoTime();
           System.out.println("Elapsed [ms]: "+(after-before)/1000000L);
           System.out.println("");
        }

        try
        {
            Thread.sleep(40000);
        }
        catch (Exception e)
        {
           e.printStackTrace();
        }

        System.exit(0);
    }  
}
