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
import java.awt.Transparency;
import java.awt.image.BufferedImage;
import java.io.FileInputStream;
import java.util.Arrays;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.ColorModelType;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.RGBColorModelConverter;
import kanzi.util.color.YCbCrColorModelConverter;
import kanzi.util.image.ImageQualityMonitor;
import kanzi.util.color.ReversibleYUVColorModelConverter;
import kanzi.util.color.XYZColorModelConverter;
import kanzi.util.color.YCoCgColorModelConverter;
import kanzi.util.color.YIQColorModelConverter;
import kanzi.util.color.YSbSrColorModelConverter;
import kanzi.util.image.ImageUtils;
import kanzi.util.image.ImageUtils.ImageInfo;
import kanzi.util.sampling.BicubicUpSampler;
import kanzi.util.sampling.BilinearUpSampler;
import kanzi.util.sampling.DWTDownSampler;
import kanzi.util.sampling.DWTUpSampler;
import kanzi.util.sampling.DecimateDownSampler;
import kanzi.util.sampling.DownSampler;
import kanzi.util.sampling.GuidedBilinearUpSampler;
import kanzi.util.sampling.UpSampler;


public class TestColorModel
{
    public static void main(String[] args)
    {
       try
       {
         String fileName = (args.length > 0) ? args[0] : "r:\\kodim24.png";
         String type = fileName.substring(fileName.lastIndexOf(".")+1);
         ImageInfo ii = ImageUtils.loadImage(new FileInputStream(fileName), type);
         int w = ii.width & -16;
         int h = ii.height & -16;
         int[] rgb = ii.data;
         int[] rgb2 = new int[rgb.length];
         
         System.out.println(w + "x" + h);

         UpSampler uBicubic = new BicubicUpSampler(w/2, h/2, w/2, w, 0, false);
         UpSampler uBilinear = new BilinearUpSampler(w/2, h/2, 2);
         GuidedBilinearUpSampler ugBilinear = new GuidedBilinearUpSampler(w/2, h/2);
         DownSampler downSampler = new DecimateDownSampler(w, h, 2);
         DownSampler dDCT = new DWTDownSampler(w, h, w, 1);
         UpSampler uDCT = new DWTUpSampler(w/2, h/2, w, 1);

         ColorModelConverter[] cvts = new ColorModelConverter[]
         {
            new RGBColorModelConverter(w, h),
            new ReversibleYUVColorModelConverter(w, h),
            new YCoCgColorModelConverter(w, h),
            new XYZColorModelConverter(w, h),
            new YCbCrColorModelConverter(w, h),
            new YSbSrColorModelConverter(w, h),
            new YIQColorModelConverter(w, h),
            new YCbCrColorModelConverter(w, h, downSampler, uBicubic),
            new YSbSrColorModelConverter(w, h, downSampler, uBicubic, true),
            new YIQColorModelConverter(w, h, downSampler, uBicubic),
            new YCbCrColorModelConverter(w, h),
            new YSbSrColorModelConverter(w, h, true),
            new YIQColorModelConverter(w, h),
            new YCbCrColorModelConverter(w, h, downSampler, uBilinear),
            new YSbSrColorModelConverter(w, h, downSampler, uBilinear, true),
            new YIQColorModelConverter(w, h, downSampler, uBilinear),
            new YCbCrColorModelConverter(w, h, downSampler, ugBilinear),
            new YSbSrColorModelConverter(w, h, downSampler, ugBilinear, true),
            new YIQColorModelConverter(w, h, downSampler, ugBilinear),
            new YCbCrColorModelConverter(w, h, dDCT, uDCT),
            new YSbSrColorModelConverter(w, h,dDCT, uDCT, true),
            new YIQColorModelConverter(w, h, dDCT, uDCT),         
         };
         
         ModelInfo[] models = new ModelInfo[]
         {
            new ModelInfo("RGB", false, null),
            new ModelInfo("Reversible YUV", false, null),
            new ModelInfo("YCoCg", false, null),
            new ModelInfo("XYZ", false, null),
            new ModelInfo("YCbCr", false, null),
            new ModelInfo("YSbSr", false, null),
            new ModelInfo("YIQ", false, null),
            new ModelInfo("YCbCr - bicubic", true, uBicubic),
            new ModelInfo("YSbSr - bicubic", true, uBicubic),
            new ModelInfo("YIQ - bicubic", true, uBicubic),
            new ModelInfo("YCbCr - built-in (bilinear)", true, null),
            new ModelInfo("YSbSr - built-in (bilinear)", true, null),
            new ModelInfo("YIQ - built-in (bilinear)", true, null),
            new ModelInfo("YCbCr - bilinear", true, uBilinear),
            new ModelInfo("YSbSr - bilinear", true, uBilinear),
            new ModelInfo("YIQ - bilinear", true, uBilinear),
            new ModelInfo("YCbCr - guided bilinear", true, ugBilinear),
            new ModelInfo("YSbSr - guided bilinear", true, ugBilinear),
            new ModelInfo("YIQ - guided bilinear", true, ugBilinear),
            new ModelInfo("YCbCr - DCT", true, uDCT),
            new ModelInfo("YSbSr - DCT", true, uDCT),
            new ModelInfo("YIQ - DCT", true, uDCT),         };

         GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
         GraphicsConfiguration gc = gs.getDefaultConfiguration();
         BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE); 
         img.getRaster().setDataElements(0, 0, w, h, rgb);
         JFrame frame = new JFrame("Original");
         frame.setBounds(20, 30, w, h);
         frame.add(new JLabel(new ImageIcon(img)));
         frame.setVisible(true);
         System.out.println("================ Test round trip RGB -> YXX -> RGB ================");
         boolean display = true;

         for (int i=0; i<cvts.length; i++)
         {
            test(models[i], cvts[i], rgb, rgb2, w, h, i+1, display);
         }

           Thread.sleep(35000);
        }
        catch (Exception e)
        {
           e.printStackTrace();
        }

        System.exit(0);
    }


    private static void test(ModelInfo model, ColorModelConverter cvt, int[] rgb1, int[] rgb2,
            int w, int h, int testIdx, boolean display)
    {
        long sum = 0;
        int iters = 10;
        int[] y1 = new int[rgb1.length];
        int[] u1 = new int[rgb1.length];
        int[] v1 = new int[rgb1.length];
        Arrays.fill(rgb2, 0x0A0A0A0A);

        if (model.is420)
        {          
          cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.YUV420);
          
          if (model.us instanceof GuidedBilinearUpSampler)
             ((GuidedBilinearUpSampler) model.us).setGuide(y1);

          cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.YUV420);
        }
        else
        {
           if (cvt instanceof XYZColorModelConverter) 
           {
              cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.XYZ);
              cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.XYZ);
           } 
           else if (cvt instanceof RGBColorModelConverter) 
           {
              cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.RGB);
              cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.RGB);
           }  
           else
           {
              cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.YUV444);
              cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.YUV444);
           }
        }

        // Compute PSNR
        // Computing the SSIM makes little sense since y, u and v are shared
        // by both images => SSIM = 1.0
        String name = model.name + ((model.is420) ? " - 420 " : " - 444 ");
        System.out.println("\n"+name);
        ImageQualityMonitor iqm = new ImageQualityMonitor(w, h);
        int psnr1024 = iqm.computePSNR(rgb1, rgb2);
        System.out.println("PSNR : "+ ((psnr1024 == 0) ? "Infinite" : ((float) psnr1024 / 1024)));
        
        if (display == true) 
        {
            String title = name + "- PSNR: "+ ((psnr1024 == 0) ? "Infinite" : ((float) psnr1024 / 1024));      
            GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
            GraphicsConfiguration gc = gs.getDefaultConfiguration();
            BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
            img2.getRaster().setDataElements(0, 0, w, h, rgb2);
            JFrame frame2 = new JFrame(title);
            frame2.setBounds(20+(testIdx)*60, 30+(testIdx*30), w, h);
            ImageIcon newIcon = new ImageIcon(img2);
            frame2.add(new JLabel(newIcon));
            frame2.setVisible(true);
        }
        
        System.out.println("Speed test");

        if (model.is420)
        {
           if (model.us instanceof GuidedBilinearUpSampler)
              ((GuidedBilinearUpSampler) model.us).setGuide(y1);
           
           for (int i=0; i<iters; i++)
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
          for (int i=0; i<iters; i++)
           {
               Arrays.fill(rgb2, 0);
               long before = System.nanoTime();

               if (cvt instanceof XYZColorModelConverter) 
               {
                  cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.XYZ);
                  cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.XYZ);
               } 
               else if (cvt instanceof RGBColorModelConverter) 
               {
                  cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.RGB);
                  cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.RGB);
               } 
               else
               {
                  cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.YUV444);
                  cvt.convertYUVtoRGB(y1, u1, v1, rgb2, ColorModelType.YUV444);
               }
               
               long after = System.nanoTime();
               sum += (after - before);
           }
        }

        System.out.println("Elapsed [ms] ("+iters+" iterations): "+sum/1000000);
    }
    
    
    static class ModelInfo
    {


      public ModelInfo(String name, boolean is420, UpSampler us)
      {
         this.name = name;
         this.is420 = is420;
         this.us = us;
      }
       
       final String name;
       final boolean is420;
       final UpSampler us;
    }
}
