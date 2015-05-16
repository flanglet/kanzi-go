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
import kanzi.transform.DCT8;
import kanzi.IntTransform;
import kanzi.transform.DCT16;
import kanzi.transform.DCT32;
import kanzi.transform.DCT4;
import kanzi.transform.DWT_CDF_9_7;
import kanzi.transform.WHT16;
import kanzi.transform.WHT32;
import kanzi.transform.WHT4;
import kanzi.transform.WHT8;
import kanzi.util.ImageQualityMonitor;

public class TestImageTransform
{
  public static void main(String[] args)
  {
        String filename = (args.length > 0) ? args[0] : "C:\\temp\\lena.jpg";
        javax.swing.ImageIcon icon = new javax.swing.ImageIcon(filename);
        java.awt.Image image = icon.getImage();
        int w = image.getWidth(null) & 0xFFF0;
        int h = image.getHeight(null) & 0xFFF0;
        java.awt.GraphicsDevice gs = java.awt.GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        java.awt.GraphicsConfiguration gc = gs.getDefaultConfiguration();
        java.awt.image.BufferedImage img = gc.createCompatibleImage(w, h, java.awt.Transparency.OPAQUE);
        img.getGraphics().drawImage(image, 0, 0, null);

        int[] rgb = new int[w*h];
        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img.getRaster().getDataElements(0, 0, w, h, rgb);

        for (int i=0; i<rgb.length; i++)
        {
            final int grey = ((rgb[i] & 0xFF) + ((rgb[i] >> 8) & 0xFF) + 
                    ((rgb[i] >> 16) & 0xFF)) / 3;
            rgb[i] = (grey << 16) | (grey << 8) | grey;
        }
        
        img.getRaster().setDataElements(0, 0, w, h, rgb);

        javax.swing.JFrame frame = new javax.swing.JFrame("Original");
        frame.setBounds(100, 100, w, h);
        frame.add(new javax.swing.JLabel(new javax.swing.ImageIcon(img)));
        frame.setVisible(true);

        DCT4 dct4 = new DCT4();
        transform(dct4, w, h, rgb, 4, "Discrete Cosine Transform 4x4", 150, 150);

        DCT8 dct8 = new DCT8();
        transform(dct8, w, h, rgb, 8, "Discrete Cosine Transform 8x8", 200, 200);

        DCT16 dct16 = new DCT16();
        transform(dct16, w, h, rgb, 16, "Discrete Cosine Transform 16x16", 250, 250);

        DCT32 dct32 = new DCT32();
        transform(dct32, w, h, rgb, 32, "Discrete Cosine Transform 32x32", 300, 300);

        WHT4 wht4 = new WHT4();
        transform(wht4, w, h, rgb, 4, "Walsh-Hadamard Transform 4x4", 350, 350);
        
        WHT8 wht8 = new WHT8();
        transform(wht8, w, h, rgb, 8, "Walsh-Hadamard Transform 8x8", 400, 400);

        WHT16 wht16 = new WHT16();
        transform(wht16, w, h, rgb, 16, "Walsh-Hadamard Transform 16x16", 450, 450);

        WHT32 wht32 = new WHT32();
        transform(wht32, w, h, rgb, 32, "Walsh-Hadamard Transform 32x32", 500, 500);

        DWT_CDF_9_7 dwt1 = new DWT_CDF_9_7(w, h, 4);
        transform(dwt1, w, h, rgb, Math.min(w, h), "Wavelet Transform CDF 9/7 "+w+"x"+h, 550, 550);

        DWT_CDF_9_7 dwt2 = new DWT_CDF_9_7(32, 32, 2);
        transform(dwt2,  w, h, rgb, 32, "Wavelet Transform CDF 9/7 32x32", 600, 600);

        try
        {
           Thread.sleep(25000);
        }
        catch (InterruptedException e)
        {           
        }
        
        System.exit(0);
  }


  private static int[] transform(IntTransform transform, int w, int h, int[] rgb, 
          int dim, String title, int xx, int yy)
  {
    int len = w * h;
    int[] rgb2 = new int[len];
    int[] data = new int[w*h];
    long sum = 0L;
    int iter = 1000;
    IndexedIntArray iia = new IndexedIntArray(data, 0);

    for (int ii=0; ii<iter; ii++)
    {
       for (int y=0; y<h; y+=dim)
       {
           for (int x=0; x<w; x+=dim)
           {
              int idx = 0;

              for (int j=y; j<y+dim; j++)
              {
                 int offs = j * w;

                 for (int i=x; i<x+dim; i++)
                     data[idx++] = rgb[offs+i] & 0xFF;
              }

              long before = System.nanoTime();
              iia.index = 0;
              transform.forward(iia, iia);
              iia.index = 0;
              transform.inverse(iia, iia);
              long after = System.nanoTime();
              sum += (after - before);
              
              idx = 0;

              for (int j=y; j<y+dim; j++)
              {
                 int offs = j * w;

                 for (int i=x; i<x+dim; i++)
                 {
                     rgb2[offs+i] = (data[idx] << 16) | (data[idx] << 8) | (data[idx] & 0xFF);
                     idx++;
                 }
              }
           }
       }
    }

    int psnr1024 = new ImageQualityMonitor(w, h).computePSNR(rgb, rgb2);
    int ssim1024 = new ImageQualityMonitor(w, h).computeSSIM(rgb, rgb2);
    //System.out.println("PSNR: "+(float) psnr256 / 256);
    title += " - PSNR: ";
    title += (psnr1024 < 1024) ? "Infinite" : ((float) psnr1024 / 1024);
    title += " - SSIM: ";
    title += ((float) ssim1024 / 1024);
    System.out.println(title);
    System.out.println("Elapsed time for "+iter+" iterations [ms]: "+sum/1000000L);

    java.awt.GraphicsDevice gs = java.awt.GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
    java.awt.GraphicsConfiguration gc = gs.getDefaultConfiguration();
    java.awt.image.BufferedImage img = gc.createCompatibleImage(w, h, java.awt.Transparency.OPAQUE);
    img.getRaster().setDataElements(0, 0, w, h, rgb2);
    javax.swing.ImageIcon icon = new javax.swing.ImageIcon(img);
    javax.swing.JFrame frame = new javax.swing.JFrame(title);
    frame.setBounds(xx, yy, w, h);
    frame.add(new javax.swing.JLabel(icon));
    frame.setVisible(true);

    return rgb;
  }

}
