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
import kanzi.IndexedIntArray;
import kanzi.transform.DWT_CDF_9_7;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.YCbCrColorModelConverter;
import kanzi.util.ImageQualityMonitor;
import kanzi.util.color.ReversibleYUVColorModelConverter;
import kanzi.util.color.YSbSrColorModelConverter;
import kanzi.util.sampling.BilinearUpSampler;
import kanzi.util.sampling.DecimateDownSampler;
import kanzi.util.sampling.FourTapUpSampler;
import kanzi.util.sampling.DownSampler;
import kanzi.util.sampling.UpSampler;


public class TestColorModel
{
    public static void main(String[] args)
    {
        String fileName = (args.length > 0) ? args[0] : "c:\\temp\\lena.jpg";
        ImageIcon icon = new ImageIcon(fileName);
        Image image = icon.getImage();
        int w = image.getWidth(null);
        int h = image.getHeight(null);
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);

        img.getGraphics().drawImage(image, 0, 0, null);
        int[] rgb = new int[w*h];
        int[] rgb2 = new int[w*h];
        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img.getRaster().getDataElements(0, 0, w, h, rgb);

        System.out.println(w + "x" + h);

        UpSampler uFourTap1 = new FourTapUpSampler(w/2, h/2, 2, true);
        UpSampler uFourTap2 = new FourTapUpSampler(w/2, h/2, 2, false);
        UpSampler uBilinear = new BilinearUpSampler(w/2, h/2, 2);
        DownSampler dFourTap = new DecimateDownSampler(w, h, 2); // For now !!!
        DownSampler dBilinear = new DecimateDownSampler(w, h, 2);//BilinearDownSampler(w, h, 2);
        DownSampler dDWT = new DWTDownSampler(w, h);
        UpSampler uDWT = new DWTUpSampler(w, h);

        ColorModelConverter[] cvts = new ColorModelConverter[]
        {
           new YCbCrColorModelConverter(w, h, dFourTap, uFourTap1),
           new YCbCrColorModelConverter(w, h, dBilinear, uBilinear),
           new YCbCrColorModelConverter(w, h, dDWT, uDWT),
           new YCbCrColorModelConverter(w, h),
           new YCbCrColorModelConverter(w, h),
           new YSbSrColorModelConverter(w, h, dFourTap, uFourTap2),
           new YSbSrColorModelConverter(w, h, dBilinear, uBilinear),
           new YSbSrColorModelConverter(w, h, dDWT, uDWT),
           new YSbSrColorModelConverter(w, h),
           new YSbSrColorModelConverter(w, h),
           new ReversibleYUVColorModelConverter(w, h)
        };

        boolean[] is420 = new boolean[]
        {
            true, true, true, true, false,
            true, true, true, true, false,
            false
        };

        String[] names = { "YCbCr - four taps",
                           "YCbCr - bilinear",
                           "YCbCr - DWT",
                           "YCbCr - built-in (bilinear)",
                           "YCbCr",
                           "YSbSr - four taps",
                           "YSbSr - bilinear",
                           "YSbSr - DWT",
                           "YSbSr - built-in (bilinear)",
                           "YSbSr",
                           "Reversible YUV"
                         };

        icon = new ImageIcon(img);
        JFrame frame = new JFrame("Original");
        frame.setBounds(20, 30, w, h);
        frame.add(new JLabel(icon));
        frame.setVisible(true);
        System.out.println("================ Test round trip RGB -> YXX -> RGB ================");

        for (int i=0; i<cvts.length; i++)
        {
           test(names[i], cvts[i], rgb, rgb2, w, h, i+1, is420[i]);
        }

        try
        {
           Thread.sleep(35000);
        }
        catch (Exception e)
        {
        }

        System.exit(0);
    }


    private static void test(String name, ColorModelConverter cvt, int[] rgb1, int[] rgb2,
            int w, int h, int iter, boolean y420)
    {
        long sum = 0;
        int nn = 1000;
        int[] y1 = new int[rgb1.length];
        int[] u1 = new int[rgb1.length];
        int[] v1 = new int[rgb1.length];
        Arrays.fill(rgb2, 0x0A0A0A0A);

        if (y420)
        {
          cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.YUV420);
          cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.YUV420);
        }
        else
        {
          cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.YUV444);
          cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.YUV444);
        }

        // Compute PSNR
        // Computing the SSIM makes little sense since y,u and v are shared
        // by both images => SSIM = 1.0
        name += ((y420) ? " - 420 " : " - 444 ");
        System.out.println("\n"+name);
        ImageQualityMonitor iqm = new ImageQualityMonitor(w, h);
        int psnr1024 = iqm.computePSNR(rgb1, rgb2);
        System.out.println("PSNR : "+ ((psnr1024 == 0) ? "Infinite" : ((float) psnr1024 / 1024)));
        String title = name + "- PSNR: "+ ((psnr1024 == 0) ? "Infinite" : ((float) psnr1024 / 1024));
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        img2.getRaster().setDataElements(0, 0, w, h, rgb2);
        JFrame frame2 = new JFrame(title);
        frame2.setBounds(20+(iter)*100, 30+(iter*30), w, h);
        ImageIcon newIcon = new ImageIcon(img2);
        frame2.add(new JLabel(newIcon));
        frame2.setVisible(true);
        System.out.println("Speed test");

        if (y420)
        {
           for (int i=0; i<nn; i++)
           {
               Arrays.fill(rgb2, 0);
               long before = System.nanoTime();
               cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.YUV420);
               cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.YUV420);
               long after = System.nanoTime();
               sum += (after - before);
           }
        }
        else
        {
          for (int i=0; i<nn; i++)
           {
               Arrays.fill(rgb2, 0);
               long before = System.nanoTime();
               cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.YUV444);
               cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.YUV444);
               long after = System.nanoTime();
               sum += (after - before);
           }
        }

        System.out.println("Elapsed [ms] ("+nn+" iterations): "+sum/1000000);
    }
    
    
    static class DWTDownSampler implements DownSampler
    {
      private final int w;
      private final int h;
      private final DWT_CDF_9_7 dwt;
      
      public DWTDownSampler(int w, int h)
      {
         this.w = w;
         this.h = h;
         this.dwt = new DWT_CDF_9_7(this.w, this.h, 1);
      }
            
      @Override
      public void subSampleHorizontal(int[] input, int[] output) 
      {
         throw new UnsupportedOperationException("Not supported yet."); 
      }

      @Override
      public void subSampleVertical(int[] input, int[] output)
      {
         throw new UnsupportedOperationException("Not supported yet.");
      }

      @Override
      public void subSample(int[] input, int[] output) 
      {
         IndexedIntArray src = new IndexedIntArray(input, 0);
         IndexedIntArray dst = new IndexedIntArray(output, 0);
         this.dwt.forward(src, dst);
         
         for (int j=this.h/2-1; j>=0; j--)
         {
            final int offset = j * this.w + this.w/2;
            
            // Remove high bands coefficients to down sample
            for (int i=offset+this.w/2-1; i>=offset; i--)
               output[i] = 0;
         }

         for (int j=this.h/2; j<this.h; j++)
         {
            final int offset = j * this.w;
            
            // Remove high bands coefficients to down sample
            for (int i=offset+this.w-1; i>=offset; i--)
               output[i] = 0;
         }
      }

      @Override
      public boolean supportsScalingFactor(int factor) 
      {
         return (factor == 2);
      }
       
    }
    
    
    static class DWTUpSampler implements UpSampler
    {
      private final int w;
      private final int h;
      private final DWT_CDF_9_7 dwt;
      
      public DWTUpSampler(int w, int h)
      {
         this.w = w;
         this.h = h;
         this.dwt = new DWT_CDF_9_7(this.w, this.h, 1);
      }

      @Override
      public void superSampleHorizontal(int[] input, int[] output) 
      {
         throw new UnsupportedOperationException("Not supported yet.");       
      }

      @Override
      public void superSampleVertical(int[] input, int[] output) 
      {
         throw new UnsupportedOperationException("Not supported yet."); 
      }

      @Override
      public void superSample(int[] input, int[] output) 
      {
         IndexedIntArray src = new IndexedIntArray(input, 0);
         IndexedIntArray dst = new IndexedIntArray(output, 0);
         dwt.inverse(src, dst);
      }

      @Override
      public boolean supportsScalingFactor(int factor) 
      {
         return (factor == 2);
      }
    }
    
}
