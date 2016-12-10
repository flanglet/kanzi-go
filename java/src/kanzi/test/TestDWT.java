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
import java.awt.GraphicsConfiguration;
import java.awt.GraphicsDevice;
import java.awt.GraphicsEnvironment;
import java.awt.Image;
import java.awt.Transparency;
import java.awt.image.BufferedImage;
import java.io.FileInputStream;
import java.util.Arrays;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.ColorModelType;
import kanzi.IntTransform;
import kanzi.transform.DWT_CDF_9_7;
import kanzi.transform.DWT_Haar;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.image.ImageQualityMonitor;
import kanzi.util.color.RGBColorModelConverter;
import kanzi.util.image.ImageUtils;


public class TestDWT
{
   public static void main(String[] args)
   {
      try
      {
         String fileName = (args.length > 0) ? args[0] : "r:\\lena.jpg";         

         // Load image (PPM/PGM supported)
         String type = fileName.substring(fileName.lastIndexOf(".")+1);
         ImageUtils.ImageInfo ii = ImageUtils.loadImage(new FileInputStream(fileName), type);

         final int w = ii.width;
         final int h = ii.height;
         GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
         GraphicsConfiguration gc = gs.getDefaultConfiguration();
         BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         img.getRaster().setDataElements(0, 0, w, h, ii.data);
         JFrame frame = new JFrame("Original");
         frame.setBounds(30, 100, w, h);
         ImageIcon newIcon = new ImageIcon(img);
         frame.add(new JLabel(newIcon));
         frame.setVisible(true);
         
         IntTransform yDWT;
         IntTransform uvDWT;
         ColorModelType cmType = ColorModelType.YUV444;
         int shift = (cmType == ColorModelType.YUV420) ? 1 : 0;
         
         yDWT = new DWT_Haar(w, h, 4, true);
         uvDWT = new DWT_Haar(w >> shift, h >> shift, 4, true);         
         process("Haar", img, w, h, yDWT, uvDWT, 565, 100);
         
         yDWT = new DWT_CDF_9_7(w, h, 6);
         uvDWT = new DWT_CDF_9_7(w >> shift, h >> shift, 6);
         process("Daubechies", img, w, h, yDWT, uvDWT, 1100, 100);

         Thread.sleep(40000);
      }
      catch (Exception e)
      {
         e.printStackTrace();
      }

      System.exit(0);
   }


   private static void process(String title, Image image, int w, int h, IntTransform yDWT, IntTransform uvDWT, int xpos, int ypos)
   {
         GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
         GraphicsConfiguration gc = gs.getDefaultConfiguration();
         BufferedImage imgs  = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         BufferedImage imgd = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         imgs.getGraphics().drawImage(image, 0, 0, null);
         int[] src = new int[w*h];
         int[] dest = new int[w*h];
         imgs.getRaster().getDataElements(0, 0, w, h, src);
         SliceIntArray iias  = new SliceIntArray(src, 0);
         SliceIntArray iiad = new SliceIntArray(dest, 0);

         ColorModelConverter cvt;
//         cvt = new YCbCrColorModelConverter(w, h);
//         cvt = new YSbSrColorModelConverter(w, h);
//         cvt = new YCoCgColorModelConverter(w, h);
         cvt = new RGBColorModelConverter(w, h);

         ColorModelType cmType = ColorModelType.RGB;
         int shift = (cmType == ColorModelType.YUV420) ? 1 : 0;
         ImageQualityMonitor monitor = new ImageQualityMonitor(w, h);
         
         System.out.println();
         process(w, h, cvt, iias, iiad, cmType, yDWT, uvDWT);
         int psnr1024 = monitor.computePSNR(src, dest);
         System.out.println(title);
         System.out.println("PSNR: "+((psnr1024 == 0) ? "Infinite" : (float) psnr1024 /1024));
         imgd.getRaster().setDataElements(0, 0, w, h, dest);
         JFrame frame = new JFrame(title);
         frame.setBounds(xpos, ypos, w, h);
         ImageIcon newIcon = new ImageIcon(imgd);
         frame.add(new JLabel(newIcon));
         frame.setVisible(true);
   }
   
   
   private static void process(int w, int h, ColorModelConverter cvt, 
           SliceIntArray iia1, SliceIntArray iia2, ColorModelType cmType,
           IntTransform yDWT, IntTransform uvDWT)
   {
      int shift = (cmType == ColorModelType.YUV420) ? 1 : 0;
      int[] y1 = new int[w * h];
      int[] u1 = new int[(w * h) >> (shift + shift)];
      int[] v1 = new int[(w * h) >> (shift + shift)];
      int[] y2 = new int[w * h];
      int[] u2 = new int[(w * h) >> (shift + shift)];
      int[] v2 = new int[(w * h) >> (shift + shift)];
      int[] dest = iia2.array;

      long before = System.nanoTime();
      
      // Forward color transform
      cvt.convertRGBtoYUV(iia1.array, y1, u1, v1, cmType);      

      // Forward DWT
      iia1.array = y1;
      iia2.array = y2;
      iia1.index = 0;
      iia2.index = 0;
      yDWT.forward(iia1, iia2);      
      iia1.array = u1;
      iia2.array = u2;
      iia1.index = 0;
      iia2.index = 0;
      uvDWT.forward(iia1, iia2);
      iia1.array = v1;
      iia2.array = v2;
      iia1.index = 0;
      iia2.index = 0;
      uvDWT.forward(iia1, iia2);
   
      // Clear data
      Arrays.fill(y1, 0);
      Arrays.fill(u1, 0);
      Arrays.fill(v1, 0);

      // Inverse DWT
      iia1.array = y1;
      iia2.array = y2;
      iia1.index = 0;
      iia2.index = 0;
      yDWT.inverse(iia2, iia1);
      iia1.array = u1;
      iia2.array = u2;
      iia1.index = 0;
      iia2.index = 0;
      uvDWT.inverse(iia2, iia1);
      iia1.array = v1;
      iia2.array = v2;
      iia1.index = 0;
      iia2.index = 0;
      uvDWT.inverse(iia2, iia1);

      // Inverse color transform
      cvt.convertYUVtoRGB(y1, u1, v1, dest, cmType);

      long after = System.nanoTime();
      System.out.println("Time elapsed [ms]: "+ (after-before)/1000000L);
   }
}
