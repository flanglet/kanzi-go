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
import kanzi.util.ImageQualityMonitor;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.YCbCrColorModelConverter;
import kanzi.util.sampling.BilinearUpSampler;
import kanzi.util.sampling.DecimateDownSampler;
import kanzi.util.sampling.FourTapUpSampler;
import kanzi.util.sampling.SixTapUpSampler;
import kanzi.util.sampling.DownSampler;
import kanzi.util.sampling.UpSampler;


public class TestResampler
{
   public static void main(String[] args)
   {
        String fileName = (args.length > 0) ? args[0] : "c:\\temp\\lena.jpg";
        roundtrip(fileName, 2, 1);
        upscale("c:\\temp\\lena256.jpg", 2, 100);

        try
        {
            Thread.sleep(60000);
        }
        catch (Exception e)
        {
        }

        System.exit(0);

   }


    public static void roundtrip(String fileName, int factor, int iter)
    {
       //for (int i=-0; i<=300; i++)
       //   System.out.println(i+"  "+((((255-i) >> 31) & 255) | (i & 255)));

        ImageIcon icon = new ImageIcon(fileName);
        Image image = icon.getImage();
        int w = image.getWidth(null);
        int h = image.getHeight(null);
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        System.out.println("\n\n========= Testing round trip: original -> downsample by " + factor + " -> upsample by " + factor);

        img.getGraphics().drawImage(image, 0, 0, null);
        int[] rgb = new int[w*h];
        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img.getRaster().getDataElements(0, 0, w, h, rgb);

        int[] y = new int[rgb.length];
        int[] u = new int[rgb.length];
        int[] v = new int[rgb.length];
        int[] input = rgb;
        //int[] tmp = new int[rgb.length/factor];
        int[] output = new int[rgb.length];

        JFrame frame = new JFrame("Original");
        frame.setBounds(20, 20, w, h);
        frame.add(new JLabel(icon));
        frame.setVisible(true);

        UpSampler uBilinear = new BilinearUpSampler(w/factor, h/factor, factor);
        DownSampler dBilinear = new DecimateDownSampler(w, h, factor);
        UpSampler uFourtap = new FourTapUpSampler(w/factor, h/factor, factor);
        //SubSampler dFourtap = new FourTapDownSampler(w, h, factor);
        UpSampler uSixtap = new SixTapUpSampler(w/factor, h/factor, factor);
        //SubSampler dSixtap = new SixTapDownSampler(w, h, factor);
        DownSampler[] subSamplers = new DownSampler[]  { dBilinear, dBilinear, dBilinear };
        UpSampler[] superSamplers = new UpSampler[]  { uBilinear, uFourtap, uSixtap };
        String[] titles = new String[] { "Bilinear", "Four taps", "Six taps" };
        System.out.println(w + "x" + h);
        System.out.println();

        // Round trip down / up
        for (int s=0; s<subSamplers.length; s++)
        {
           String title = titles[s];
           Arrays.fill(output, 0);
           System.out.println(title);
           long delta = 0;
           ColorModelConverter cvt = new YCbCrColorModelConverter(w, h);
           cvt.convertRGBtoYUV(rgb, y, u, v, ColorModelType.YUV444);

           for (int ii=0; ii<iter; ii++)
           {
               long before = System.nanoTime();
               subSamplers[s].subSample(y, output);
               superSamplers[s].superSample(output, y);
               subSamplers[s].subSample(u, output);
               superSamplers[s].superSample(output, u);
               subSamplers[s].subSample(v, output);
               superSamplers[s].superSample(output, v);
               long after = System.nanoTime();
               delta += (after - before);
           }

           System.out.println("Elapsed [ms] ("+iter+" iterations): "+delta/1000000);
           System.out.println();

           cvt.convertYUVtoRGB(y, u, v, output, ColorModelType.YUV444);

           int psnr1024, ssim1024;
           psnr1024 = new ImageQualityMonitor(w, h).computePSNR(input, output);
           String res =  "PSNR: "+(float) psnr1024 /1024;
           ssim1024 = new ImageQualityMonitor(w, h).computeSSIM(input, output);
           res += " SSIM: "+(float) ssim1024 /1024;
           System.out.println(res);
           title += " x" + factor + " round trip - " + res;
           BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
           img2.getRaster().setDataElements(0, 0, w, h, output);
           JFrame frame2 = new JFrame(title);
           frame2.setBounds(50+100*s, 50+100*s, w, h);
           ImageIcon icon2 = new ImageIcon(img2);
           frame2.add(new JLabel(icon2));
           frame2.setVisible(true);
        }

        // Test up sample of sub image by 2
        // TODO

        // Test up sample of sub image by 4
        // TODO
    }



    public static void upscale(String fileName, int factor, int iter)
    {
       //for (int i=-0; i<=300; i++)
       //   System.out.println(i+"  "+((((255-i) >> 31) & 255) | (i & 255)));

        ImageIcon icon = new ImageIcon(fileName);
        Image image = icon.getImage();
        int w = image.getWidth(null);
        int h = image.getHeight(null);
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        System.out.println("========= Testing round trip: original -> downsample by 2 -> upsample by 2");

        img.getGraphics().drawImage(image, 0, 0, null);
        int[] rgb = new int[w*h];
        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img.getRaster().getDataElements(0, 0, w, h, rgb);

        int[] r = new int[rgb.length];
        int[] g = new int[rgb.length];
        int[] b = new int[rgb.length];
        int[] ro = new int[rgb.length*factor*factor];
        int[] go = new int[rgb.length*factor*factor];
        int[] bo = new int[rgb.length*factor*factor];
        int[] input = rgb;
        //int[] tmp = new int[rgb.length/factor];
        int[] output = new int[rgb.length*factor*factor];

        JFrame frame = new JFrame("Original");
        frame.setBounds(20, 20, w, h);
        frame.add(new JLabel(icon));
        frame.setVisible(true);

        UpSampler uBilinear = new BilinearUpSampler(w, h, factor);
        UpSampler uFourtap = new FourTapUpSampler(w, h, factor);
        UpSampler uSixtap = new SixTapUpSampler(w, h, factor);
        w *= factor;
        h *= factor;
        UpSampler[] superSamplers = new UpSampler[]  { uBilinear, uFourtap, uSixtap };
        String[] titles = new String[] { "Bilinear", "Four taps", "Six taps" };
        System.out.println(w + "x" + h);
        System.out.println();
        long delta = 0;

        // Round trip down / up
        for (int s=0; s<superSamplers.length; s++)
        {
           String title = titles[s];
           Arrays.fill(output, 0);
           System.out.println(title);

           for (int ii=0; ii<iter; ii++)
           {
               for (int i=0; i<rgb.length; i++)
               {
                  r[i] = (rgb[i] >> 16) & 0xFF;
                  g[i] = (rgb[i] >> 8) & 0xFF;
                  b[i] = rgb[i] & 0xFF;
               }

               long before = System.nanoTime();
               superSamplers[s].superSample(r, ro);
               superSamplers[s].superSample(g, go);
               superSamplers[s].superSample(b, bo);
               long after = System.nanoTime();
               delta += (after - before);
           }

           System.out.println("Elapsed [ms] ("+iter+" iterations): "+delta/1000000);
           System.out.println();

           for (int i=0; i<output.length; i++)
              output[i] = (ro[i] << 16) | (go[i] << 8) | bo[i];

            title += " scale by " + factor;
           BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
           img2.getRaster().setDataElements(0, 0, w, h, output);
           JFrame frame2 = new JFrame(title);
           frame2.setBounds(50+100*s, 50+100*s, w, h);
           ImageIcon icon2 = new ImageIcon(img2);
           frame2.add(new JLabel(icon2));
           frame2.setVisible(true);
        }
    }
}
