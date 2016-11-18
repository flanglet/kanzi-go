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

import kanzi.SliceIntArray;
import kanzi.transform.DWT_CDF_9_7;
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
import kanzi.ColorModelType;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.image.ImageQualityMonitor;
import kanzi.util.color.YCbCrColorModelConverter;


public class TestDWT2
{
   public static void main(String[] args) throws Exception     
   {
      String fileName = (args.length > 0) ? args[0] : "c:\\temp\\lena.jpg";
      Image image = ImageIO.read(new File(fileName));

      if (image.getWidth(null) <= 0)
      {
         System.out.println("Cannot find file "+fileName);
         System.exit(1);
      }
      
      {
         // Square image
         int w = 512;//image.getWidth(null);
         int h = 512;//image.getHeight(null);
         GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
         GraphicsConfiguration gc = gs.getDefaultConfiguration();
         BufferedImage img  = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         BufferedImage img3 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         img.getGraphics().drawImage(image, 0, 0, null);
         int[] source = new int[w * h];
         int[] destination = new int[w * h];
         img.getRaster().getDataElements(0, 0, w, h, source);
         SliceIntArray iia1 = new SliceIntArray(source, 0);
         SliceIntArray iia2 = new SliceIntArray(destination, 0);
         long before = System.nanoTime();

         ColorModelConverter cvt = new YCbCrColorModelConverter(w, h);//, (y*w)+x, ww);
         process(1, w, h, cvt, iia1, iia2);
         long after = System.nanoTime();
         System.out.println("Time elapsed [ms]: "+ (after-before)/1000000);


         ImageQualityMonitor monitor = new ImageQualityMonitor(w, h);
         int psnr1024 = monitor.computePSNR(source, destination);
         System.out.println("PSNR: "+(float) psnr1024 /1024);
         int ssim1024 = monitor.computeSSIM(source, destination);
         System.out.println("SSIM: "+(float) ssim1024 / 1024);

         boolean imgDiff = false;

         if (imgDiff == true)
         {
            for (int j = 0; j < h; j++)
            {
               for (int i = 0; i < w; i++)
               {
                  int p1 = source[j * w + i];
                  int p2 = destination[j * w + i];
                  int r1 = (p1 >> 16) & 0xFF;
                  int g1 = (p1 >> 8) & 0xFF;
                  int b1 = p1 & 0xFF;
                  int r2 = (p2 >> 16) & 0xFF;
                  int g2 = (p2 >> 8) & 0xFF;
                  int b2 = p2 & 0xFF;
                  int rr = (Math.abs(r1 - r2) & 0xFF) << 16;
                  int gg = (Math.abs(g1 - g2) & 0xFF) << 8;
                  int bb =  Math.abs(b1 - b2) & 0xFF;
                  destination[j * w + i] = rr | gg | bb;
               }
            }
         }

         img2.getRaster().setDataElements(0, 0, w, h, destination);
         JFrame frame2 = new JFrame("Reverse");
         frame2.setBounds(720, 50, w, h);
         ImageIcon newIcon2 = new ImageIcon(img2);
         frame2.add(new JLabel(newIcon2));
         frame2.setVisible(true);

         img3.getRaster().setDataElements(0, 0, w, h, source);
         JFrame frame3 = new JFrame("Original");
         frame3.setBounds(100, 50, w, h);
         ImageIcon newIcon3 = new ImageIcon(img3);
         frame3.add(new JLabel(newIcon3));
         frame3.setVisible(true);
      }
      
      {
         // Non square image
         int w = 288;//image.getWidth(null);
         int h = 512;//image.getHeight(null);
         GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
         GraphicsConfiguration gc = gs.getDefaultConfiguration();
         BufferedImage img  = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         BufferedImage img3 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         img.getGraphics().drawImage(image, 0, 0, null);
         int[] source = new int[w * h];
         int[] destination = new int[w * h];
         img.getRaster().getDataElements(0, 0, w, h, source);
         SliceIntArray iia2 = new SliceIntArray(source, 0);
         SliceIntArray iia3 = new SliceIntArray(destination, 0);
         long before = System.nanoTime();

         ColorModelConverter cvt = new YCbCrColorModelConverter(w, h);//, (y*w)+x, ww);
         process(2, w, h, cvt, iia2, iia3);
         long after = System.nanoTime();
         System.out.println("Time elapsed [ms]: "+ (after-before)/1000000);


         ImageQualityMonitor monitor = new ImageQualityMonitor(w, h);
         int psnr1024 = monitor.computePSNR(source, destination);
         System.out.println("PSNR: "+(float) psnr1024 /1024);
         int ssim1024 = monitor.computeSSIM(source, destination);
         System.out.println("SSIM: "+(float) ssim1024 / 1024);

         boolean imgDiff = false;

         if (imgDiff == true)
         {
            for (int j = 0; j < h; j++)
            {
               for (int i = 0; i < w; i++)
               {
                  int p1 = source[j * w + i];
                  int p2 = destination[j * w + i];
                  int r1 = (p1 >> 16) & 0xFF;
                  int g1 = (p1 >> 8) & 0xFF;
                  int b1 = p1 & 0xFF;
                  int r2 = (p2 >> 16) & 0xFF;
                  int g2 = (p2 >> 8) & 0xFF;
                  int b2 = p2 & 0xFF;
                  int rr = (Math.abs(r1 - r2) & 0xFF) << 16;
                  int gg = (Math.abs(g1 - g2) & 0xFF) << 8;
                  int bb =  Math.abs(b1 - b2) & 0xFF;
                  destination[j * w + i] = rr | gg | bb;
               }
            }
         }

         img2.getRaster().setDataElements(0, 0, w, h, destination);
         JFrame frame2 = new JFrame("Reverse");
         frame2.setBounds(720, 600, w, h);
         ImageIcon newIcon2 = new ImageIcon(img2);
         frame2.add(new JLabel(newIcon2));
         frame2.setVisible(true);

         img3.getRaster().setDataElements(0, 0, w, h, source);
         JFrame frame3 = new JFrame("Original");
         frame3.setBounds(100, 600, w, h);
         ImageIcon newIcon3 = new ImageIcon(img3);
         frame3.add(new JLabel(newIcon3));
         frame3.setVisible(true);
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


   private static void process(int scale, int w, int h, ColorModelConverter cvt, SliceIntArray iia1, SliceIntArray iia2)
   {
      int[] y = new int[w * h];
      int[] u = new int[w * h];
      int[] v = new int[w * h];
      cvt.convertRGBtoYUV(iia1.array, y, u, v, ColorModelType.YUV444);
      DWT_CDF_9_7 yDWT = new DWT_CDF_9_7(w, h, 4);
      DWT_CDF_9_7 uvDWT = new DWT_CDF_9_7(w/scale, h/scale, 4);

      iia1.array = y;
      iia1.index = 0;
      yDWT.forward(iia1, iia1);
      iia1.array = u;
      iia1.index = 0;
      uvDWT.forward(iia1, iia1);
      iia1.array = v;
      iia1.index = 0;
      uvDWT.forward(iia1, iia1);

      // Inverse
      iia1.array = y;
      iia1.index = 0;
      yDWT.inverse(iia1, iia1);
      iia1.array = u;
      iia1.index = 0;
      uvDWT.inverse(iia1, iia1);
      iia1.array = v;
      iia1.index = 0;
      uvDWT.inverse(iia1, iia1);

      cvt.convertYUVtoRGB(y, u, v, iia2.array, ColorModelType.YUV444);
   }
}