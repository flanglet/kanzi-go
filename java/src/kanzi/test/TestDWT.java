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

import kanzi.IndexedIntArray;
import kanzi.transform.DWT_CDF_9_7;
import kanzi.function.wavelet.WaveletBandFilter;
import java.awt.GraphicsConfiguration;
import java.awt.GraphicsDevice;
import java.awt.GraphicsEnvironment;
import java.awt.Image;
import java.awt.Transparency;
import java.awt.image.BufferedImage;
import java.util.Arrays;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.ColorModelType;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.ImageQualityMonitor;
import kanzi.util.color.YSbSrColorModelConverter;


public class TestDWT
{
   public static void main(String[] args)
   {
      String fileName = (args.length > 0) ? args[0] : "c:\\temp\\lena.jpg";
      ImageIcon icon = new ImageIcon(fileName);
      Image image = icon.getImage();
      int ww = image.getWidth(null);
      int hh = image.getHeight(null);
      int w = 512;//image.getWidth(null);
      int h = 512;//image.getHeight(null);

      if (image.getWidth(null) <= 0)
      {
         System.out.println("Cannot find file "+fileName);
         System.exit(1);
      }

      GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
      GraphicsConfiguration gc = gs.getDefaultConfiguration();
      BufferedImage img  = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
      BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
      BufferedImage img3 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
      img.getGraphics().drawImage(image, 0, 0, null);
      int[] source = new int[ww * hh];
      int[] destination = new int[ww * hh];
      img.getRaster().getDataElements(0, 0, w, h, source);

      int dim = w;
      IndexedIntArray iia2 = new IndexedIntArray(source, 0);
      IndexedIntArray iia3 = new IndexedIntArray(destination, 0);
      ColorModelConverter cvt = new YSbSrColorModelConverter(w, h);
      process(dim, w, h, cvt, iia2, iia3, ColorModelType.YUV420);

      ImageQualityMonitor monitor = new ImageQualityMonitor(w, h);
      int psnr1024 = monitor.computePSNR(source, destination);
      System.out.println("PSNR: "+(float) psnr1024 /1024);
      int ssim1024 = monitor.computeSSIM(source, destination);
      System.out.println("SSIM: "+(float) ssim1024 / 1024);

      img2.getRaster().setDataElements(0, 0, w, h, destination);
      JFrame frame2 = new JFrame("Reverse");
      frame2.setBounds(580, 100, w, h);
      ImageIcon newIcon2 = new ImageIcon(img2);
      frame2.add(new JLabel(newIcon2));

      img.getRaster().setDataElements(0, 0, w, h, source);
      JFrame frame3 = new JFrame("Original");
      frame3.setBounds(30, 100, w, h);
      ImageIcon newIcon3 = new ImageIcon(img);
      frame3.add(new JLabel(newIcon3));

      // Calculate image difference
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
           int rr = Math.min(4*(Math.abs(r1 - r2) & 0xFF), 255) << 16;
           int gg = Math.min(4*(Math.abs(g1 - g2) & 0xFF), 255) << 8;
           int bb = Math.min(4*(Math.abs(b1 - b2) & 0xFF), 255);
           destination[j * w + i] = rr | gg | bb;
        }
     }

      img3.getRaster().setDataElements(0, 0, w, h, destination);
      JFrame frame4 = new JFrame("Diff");
      frame4.setBounds(1100, 100, w, h);
      ImageIcon newIcon4 = new ImageIcon(img3);
      frame4.add(new JLabel(newIcon4));

      frame3.setVisible(true);
      frame2.setVisible(true);
      frame4.setVisible(true);

      try
      {
         Thread.sleep(40000);
      }
      catch (Exception e)
      {
      }

      System.exit(0);
   }


   private static void process(int dim, int w, int h, ColorModelConverter cvt, 
           IndexedIntArray iia1, IndexedIntArray iia2, ColorModelType cmType)
   {
      int shift = (cmType == ColorModelType.YUV420) ? 1 : 0;
      int[] y = new int[w * h];
      int[] u = new int[(w * h) >> (shift + shift)];
      int[] v = new int[(w * h) >> (shift + shift)];
      long before = System.nanoTime();

      cvt.convertRGBtoYUV(iia1.array, y, u, v, cmType);
      
      DWT_CDF_9_7 yDWT = new DWT_CDF_9_7(w, h, 4);
      DWT_CDF_9_7 uvDWT = new DWT_CDF_9_7(w >> shift, h >> shift, 4);

      int log2 = 0;

      for (int val2=dim+1; val2>1; val2>>=1)
          log2++;

      iia1.array = y;
      iia1.index = 0;
      yDWT.forward(iia1, iia1);      

      // If Y444, we could also drop the highest frequency blocks for u & v
      // by dividing dim by 2 for these 2 components. That would be similar
      // to using y420 (but faster ... and higher quality)
      iia1.array = u;
      iia1.index = 0;
      uvDWT.forward(iia1, iia1);
      iia1.array = v;
      iia1.index = 0;
      uvDWT.forward(iia1, iia1);

      int levels = log2 - 4;

      // Quantization
      int[] quantizers = new int[levels+1];
      quantizers[0] = 55;
      quantizers[1] = 10;

      for (int i=2; i<quantizers.length; i++)
      {
          // Derive quantizer values for higher bands
          quantizers[i] = ((quantizers[i-1]) * 17 + 2) >> 4;
      }

      int[] quantizers2 = new int[levels+1];
      quantizers2[0] = 45;
      quantizers2[1] = 16;

      for (int i=2; i<quantizers2.length; i++)
      {
          // Derive quantizer values for higher bands
          quantizers2[i] = ((quantizers2[i-1]) * 17 + 2) >> 4;
      }

      int sizeAfter = 0;
      iia1.array = y;
      WaveletBandFilter yFilter = new WaveletBandFilter(w, h, levels, quantizers);
//      WaveletRateDistorsionFilter yFilter = new WaveletRateDistorsionFilter(dim, 8, levels, 100);
//      WaveletRingFilter ringFilter = new WaveletRingFilter(w, h, 3, 16);
//      ringFilter.forward(iia1, iia1);
      iia1.index = 0;
      yFilter.forward(iia1, iia2);
      sizeAfter += iia2.index;
      System.out.println("Y before: "+iia1.index+" coefficients");
      System.out.println("Y after : "+iia2.index+" coefficients");
      iia1.index = 0;
      iia2.index = 0;
      Arrays.fill(iia1.array, 0);
      yFilter.inverse(iia2, iia1);
      iia1.index = 0;
      iia2.index = 0;
      iia1.array = u;
      WaveletBandFilter uvFilter = new WaveletBandFilter(w >> shift, h >> shift, levels, quantizers2);
      uvFilter.forward(iia1, iia2);
      sizeAfter += iia2.index;
      System.out.println("U before: "+iia1.index+" coefficients");
      System.out.println("U after : "+iia2.index+" coefficients");
      iia1.index = 0;
      iia2.index = 0;
      //Arrays.fill(iia1.array, 0);
      uvFilter.inverse(iia2, iia1);
      iia1.index = 0;
      iia2.index = 0;
      iia1.array = v;
      sizeAfter += iia2.index;
      uvFilter.forward(iia1, iia2);
      System.out.println("V before: "+iia1.index+" coefficients");
      System.out.println("V after : "+iia2.index+" coefficients");
      iia1.index = 0;
      iia2.index = 0;
     // Arrays.fill(iia1.array, 0);
      uvFilter.inverse(iia2, iia1);

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

      cvt.convertYUVtoRGB(y, u, v, iia2.array, cmType);

      long after = System.nanoTime();

      System.out.println("Compression ratio: "+(float) sizeAfter/(3*w*h));
      System.out.println("Time elapsed [ms]: "+ (after-before)/1000000L);
   }
}