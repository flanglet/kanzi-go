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

import java.awt.Image;
import java.io.File;
import javax.imageio.ImageIO;
import kanzi.IndexedIntArray;
import kanzi.transform.DCT8;
import kanzi.IntTransform;
import kanzi.transform.DCT16;
import kanzi.transform.DCT32;
import kanzi.transform.DCT4;
import kanzi.transform.DWT_CDF_9_7;
import kanzi.transform.DWT_DCT;
import kanzi.transform.WHT16;
import kanzi.transform.WHT32;
import kanzi.transform.WHT4;
import kanzi.transform.WHT8;
import kanzi.util.image.ImageQualityMonitor;

public class TestImageTransform
{
  public static void main(String[] args) throws Exception
  {
        String fileName = (args.length > 0) ? args[0] : "C:\\temp\\lena.jpg";
        Image image = ImageIO.read(new File(fileName));        
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
        frame.setBounds(50, 50, w, h);
        frame.add(new javax.swing.JLabel(new javax.swing.ImageIcon(img)));
        frame.setVisible(true);

        // No alteration
        {
            System.out.println("No distortion");
            DWT_DCT dwtdct8 = new DWT_DCT(8);
            transform(dwtdct8, w, h, rgb, 8, "Wavelet Transform DWT/DCT 8x8", 80, 80, 1000, false);

            DWT_DCT dwtdct16 = new DWT_DCT(16);
            transform(dwtdct16, w, h, rgb, 16, "Wavelet Transform DWT/DCT 16x16", 110, 110, 1000, false);

            DWT_DCT dwtdct32 = new DWT_DCT(32);
            transform(dwtdct32, w, h, rgb, 32, "Wavelet Transform DWT/DCT 32x32", 140, 140, 1000, false);

            DWT_DCT dwtdct64 = new DWT_DCT(64);
            transform(dwtdct64, w, h, rgb, 64, "Wavelet Transform DWT/DCT 64x64", 170, 170, 1000, false);

            DCT4 dct4 = new DCT4();
            transform(dct4, w, h, rgb, 4, "Discrete Cosine Transform 4x4", 200, 200, 1000, false);

            DCT8 dct8 = new DCT8();
            transform(dct8, w, h, rgb, 8, "Discrete Cosine Transform 8x8", 230, 230, 1000, false);

            DCT16 dct16 = new DCT16();
            transform(dct16, w, h, rgb, 16, "Discrete Cosine Transform 16x16", 260, 260, 1000, false);

            DCT32 dct32 = new DCT32();
            transform(dct32, w, h, rgb, 32, "Discrete Cosine Transform 32x32", 290, 290, 1000, false);

            WHT4 wht4 = new WHT4();
            transform(wht4, w, h, rgb, 4, "Walsh-Hadamard Transform 4x4", 320, 320, 1000, false);

            WHT8 wht8 = new WHT8();
            transform(wht8, w, h, rgb, 8, "Walsh-Hadamard Transform 8x8", 350, 350, 1000, false);

            WHT16 wht16 = new WHT16();
            transform(wht16, w, h, rgb, 16, "Walsh-Hadamard Transform 16x16", 380, 380, 1000, false);

            WHT32 wht32 = new WHT32();
            transform(wht32, w, h, rgb, 32, "Walsh-Hadamard Transform 32x32", 410, 410, 1000, false);

            DWT_CDF_9_7 dwt8 = new DWT_CDF_9_7(8, 8, 1);
            transform(dwt8, w, h, rgb, 8, "Wavelet Transform CDF 9/7 8x8", 440, 440, 1000, false);

            DWT_CDF_9_7 dwt16 = new DWT_CDF_9_7(16, 16, 1);
            transform(dwt16, w, h, rgb, 16, "Wavelet Transform CDF 9/7 16x16", 470, 470, 1000, false);

            DWT_CDF_9_7 dwt32 = new DWT_CDF_9_7(32, 32, 1);
            transform(dwt32, w, h, rgb, 32, "Wavelet Transform CDF 9/7 32x32", 500, 500, 1000, false);

            DWT_CDF_9_7 dwt64 = new DWT_CDF_9_7(64, 64, 1);
            transform(dwt64, w, h, rgb, 64, "Wavelet Transform CDF 9/7 64x64", 530, 530, 1000, false);

            DWT_CDF_9_7 dwt = new DWT_CDF_9_7(w, h, 1);
            transform(dwt, w, h, rgb, Math.min(w, h), "Wavelet Transform CDF 9/7 "+w+"x"+h, 560, 560, 1000, false);
        }

        System.out.println("");
        
        // Drop 3/4 of coefficients (HL, LH and HH sub bands)
        {
            System.out.println("Drop 3/4 coefficients");
            DWT_DCT dwtdct8 = new DWT_DCT(8);
            transform(dwtdct8, w, h, rgb, 8, "Wavelet Transform DWT/DCT 8x8", 380, 80, 1, true);

            DWT_DCT dwtdct16 = new DWT_DCT(16);
            transform(dwtdct16, w, h, rgb, 16, "Wavelet Transform DWT/DCT 16x16", 410, 110, 1, true);

            DWT_DCT dwtdct32 = new DWT_DCT(32);
            transform(dwtdct32, w, h, rgb, 32, "Wavelet Transform DWT/DCT 32x32", 440, 140, 1, true);

            DWT_DCT dwtdct64 = new DWT_DCT(64);
            transform(dwtdct64, w, h, rgb, 64, "Wavelet Transform DWT/DCT 64x64", 470, 170, 1, true);

            DCT4 dct4 = new DCT4();
            transform(dct4, w, h, rgb, 4, "Discrete Cosine Transform 4x4", 500, 200, 1, true);

            DCT8 dct8 = new DCT8();
            transform(dct8, w, h, rgb, 8, "Discrete Cosine Transform 8x8", 530, 230, 1, true);

            DCT16 dct16 = new DCT16();
            transform(dct16, w, h, rgb, 16, "Discrete Cosine Transform 16x16", 560, 260, 1, true);

            DCT32 dct32 = new DCT32();
            transform(dct32, w, h, rgb, 32, "Discrete Cosine Transform 32x32", 590, 290, 1, true);

            WHT4 wht4 = new WHT4();
            transform(wht4, w, h, rgb, 4, "Walsh-Hadamard Transform 4x4", 620, 620, 1, true);

            WHT8 wht8 = new WHT8();
            transform(wht8, w, h, rgb, 8, "Walsh-Hadamard Transform 8x8", 650, 650, 1, true);

            WHT16 wht16 = new WHT16();
            transform(wht16, w, h, rgb, 16, "Walsh-Hadamard Transform 16x16", 680, 380, 1, true);

            WHT32 wht32 = new WHT32();
            transform(wht32, w, h, rgb, 32, "Walsh-Hadamard Transform 32x32", 710, 410, 1, true);

            DWT_CDF_9_7 dwt8 = new DWT_CDF_9_7(8, 8, 1);
            transform(dwt8, w, h, rgb, 8, "Wavelet Transform CDF 9/7 8x8", 740, 440, 1, true);

            DWT_CDF_9_7 dwt16 = new DWT_CDF_9_7(16, 16, 1);
            transform(dwt16, w, h, rgb, 16, "Wavelet Transform CDF 9/7 16x16", 770, 470, 1, true);

            DWT_CDF_9_7 dwt32 = new DWT_CDF_9_7(32, 32, 1);
            transform(dwt32, w, h, rgb, 32, "Wavelet Transform CDF 9/7 32x32", 800, 500, 1, true);

            DWT_CDF_9_7 dwt64 = new DWT_CDF_9_7(64, 64, 1);
            transform(dwt64, w, h, rgb, 64, "Wavelet Transform CDF 9/7 64x64", 830, 530, 1, true);

            DWT_CDF_9_7 dwt = new DWT_CDF_9_7(w, h, 1);
            transform(dwt, w, h, rgb, Math.min(w, h), "Wavelet Transform CDF 9/7 "+w+"x"+h, 860, 560, 1, true);
        }
  
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
          int dim, String title, int xx, int yy, int iter, boolean dropSubBands)
  {
    int len = w * h;
    int[] rgb2 = new int[len];
    int[] data = new int[dim*dim];
    long sum = 0L;
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
              
              if (dropSubBands) 
              {
              for (int j=0; j<dim; j++)
                 for (int i=0; i<dim; i++)
                    if ((i>=dim/2) || (j>=dim/2))
                       iia.array[j*dim+i] = 0;
              }
              
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
                     int val = data[idx] & 0xFF;
                     rgb2[offs+i] = (val << 16) | (val << 8) | val;
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
    
    if (iter > 1)
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
