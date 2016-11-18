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
import java.util.Arrays;
import javax.imageio.ImageIO;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.ColorModelType;
import kanzi.util.image.ImageQualityMonitor;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.YCbCrColorModelConverter;
import kanzi.util.sampling.BicubicUpSampler;
import kanzi.util.sampling.BilinearUpSampler;
import kanzi.util.sampling.DCTDownSampler;
import kanzi.util.sampling.DCTUpSampler;
import kanzi.util.sampling.DWTDownSampler;
import kanzi.util.sampling.DWTUpSampler;
import kanzi.util.sampling.DecimateDownSampler;
import kanzi.util.sampling.DownSampler;
import kanzi.util.sampling.EdgeDirectedUpSampler;
import kanzi.util.sampling.GuidedBilinearUpSampler;
import kanzi.util.sampling.UpSampler;


public class TestResampler
{
   public static void main(String[] args)
   {
        try
        {
           String fileName = (args.length > 0) ? args[0] : "r:\\salon.png";
           Image image1 = ImageIO.read(new File(fileName));
           int w = image1.getWidth(null) & -32;
           int h = image1.getHeight(null) & -32;
           GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
           GraphicsConfiguration gc = gs.getDefaultConfiguration();
           BufferedImage bi = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
           bi.getGraphics().drawImage(image1, 0, 0, null);
           int iters = (w*h) < (1500*1500) ? 100 : 10;
           roundtrip(image1, w, h, iters);
           boolean upscale = false;
           
           if (upscale == true)
           {
               Image image = ImageIO.read(new File("c:\\temp\\lena256.jpg"));
               upscale(image, bi, 100, true, 2);
               image = ImageIO.read(new File("c:\\temp\\lena128.jpg"));
               image = upscale(image, null, 100, false, 2);
               upscale(image, bi, 100, true, 2);
               image = ImageIO.read(new File("c:\\temp\\lena128.jpg"));
               upscale(image, bi, 100, true, 4);
           }
           
           Thread.sleep(60000);
        }
        catch (Exception e)
        {
           e.printStackTrace();
        }

        System.exit(0);

   }


    public static void roundtrip(Image image, int w, int h, int iter) throws Exception 
    {
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        System.out.println("\n\n========= Testing round trip: original -> downsample by " + 2 + " -> upsample by " + 2);

        img.getGraphics().drawImage(image, 0, 0, null);
        int[] rgb = new int[w*h];
        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img.getRaster().getDataElements(0, 0, w, h, rgb);

        int[] y = new int[rgb.length];
        int[] u = new int[rgb.length];
        int[] v = new int[rgb.length];
        int[] y0 = new int[rgb.length];
        int[] u0 = new int[rgb.length];
        int[] v0 = new int[rgb.length];
        int[] input = rgb;
        int[] output = new int[rgb.length];

        JFrame frame = new JFrame("Original");
        frame.setBounds(20, 20, w, h);
        frame.add(new JLabel(new ImageIcon(image)));
        frame.setVisible(true);

        UpSampler uBilinear = new BilinearUpSampler(w/2, h/2, 2);
        DownSampler dDecimate = new DecimateDownSampler(w, h, 2);
        UpSampler oriented = new EdgeDirectedUpSampler(w/2, h/2);
        UpSampler guided = new GuidedBilinearUpSampler(w/2, h/2);
        DownSampler dDWT = new DWTDownSampler(w, h, w, 1);
        UpSampler uDWT = new DWTUpSampler(w/2, h/2, w, 1);
        DownSampler dDCT = new DCTDownSampler(w, h, w, 0, 32);
        UpSampler uDCT = new DCTUpSampler(w/2, h/2, w/2, 0, 32);
        UpSampler uBicubic = new BicubicUpSampler(w/2, h/2, w/2, w, 0);
        DownSampler[] subSamplers = new DownSampler[]  { dDecimate, dDecimate, dDecimate, dDecimate, dDCT, dDWT };
        UpSampler[] superSamplers = new UpSampler[]  { uBilinear, uBicubic, oriented, guided, uDCT, uDWT };
        String[] titles = new String[] { "Bilinear", "Bicubic", "Oriented", "Guided", "DCT", "DWT" };
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
           System.arraycopy(y, 0, y0, 0, y.length);
           System.arraycopy(u, 0, u0, 0, u.length);
           System.arraycopy(v, 0, v0, 0, v.length);

           if (superSamplers[s] instanceof GuidedBilinearUpSampler)
           {
              int[] buf = new int[y.length];
              System.arraycopy(y, 0, buf, 0, buf.length);
              ((GuidedBilinearUpSampler) superSamplers[s]).setGuide(buf);
           }

//           if (superSamplers[s] instanceof DCTUpSampler)
//           {
//              int[] buf = new int[y.length];
//              System.arraycopy(y, 0, buf, 0, buf.length);
//              ((DCTUpSampler) superSamplers[s]).setGuide(buf);
//           }

           for (int ii=0; ii<iter; ii++)
           {
               System.arraycopy(y0, 0, y, 0, y.length);
               System.arraycopy(u0, 0, u, 0, u.length);
               System.arraycopy(v0, 0, v, 0, v.length);
               long before = System.nanoTime();                              
               subSamplers[s].subSample(y, output);
               Arrays.fill(y, 0);
               superSamplers[s].superSample(output, y);
               subSamplers[s].subSample(u, output);
               Arrays.fill(u, 0);
               superSamplers[s].superSample(output, u);
               subSamplers[s].subSample(v, output);
               Arrays.fill(v, 0);
               superSamplers[s].superSample(output, v);
               long after = System.nanoTime();
               delta += (after - before);
           }

           System.out.println("Elapsed [ms] ("+iter+" iterations): "+delta/1000000);
           cvt.convertYUVtoRGB(y, u, v, output, ColorModelType.YUV444);
           int psnr1024, ssim1024;
           psnr1024 = new ImageQualityMonitor(w, h).computePSNR(input, output);
           String res = "PSNR: "+(float) psnr1024 /1024;
           ssim1024 = new ImageQualityMonitor(w, h).computeSSIM(input, output);
           res += " SSIM: "+(float) ssim1024 /1024;
           System.out.println(res);
           title += " x" + 2 + " round trip - " + res;
           BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
           img2.getRaster().setDataElements(0, 0, w, h, output);
           JFrame frame2 = new JFrame(title);
           frame2.setBounds(50+100*s, 50+100*s, w, h);
           ImageIcon icon2 = new ImageIcon(img2);
           frame2.add(new JLabel(icon2));
           frame2.setVisible(true);
        }
    }



    public static Image upscale(Image image, BufferedImage refImage, 
       int iter, boolean displayResult, int scale)
    {
        int w = image.getWidth(null);
        int h = image.getHeight(null);
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        System.out.println("\n\n========= Testing upsampling by " + scale);

        img.getGraphics().drawImage(image, 0, 0, null);
        int[] rgb = new int[w*h];
        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img.getRaster().getDataElements(0, 0, w, h, rgb);

        int[] r = new int[rgb.length];
        int[] g = new int[rgb.length];
        int[] b = new int[rgb.length];
        int[] ro = new int[rgb.length*scale*scale];
        int[] go = new int[rgb.length*scale*scale];
        int[] bo = new int[rgb.length*scale*scale];
        int[] ref = new int[rgb.length*scale*scale];
        int[] output = new int[rgb.length*scale*scale];

        JFrame frame = new JFrame("Original");
        frame.setBounds(20, 20, w, h);
        frame.add(new JLabel(new ImageIcon(image)));
        frame.setVisible(true);

        UpSampler uBilinear = new BilinearUpSampler(w, h, scale);
        UpSampler uBicubic = (scale == 2) ? new BicubicUpSampler(w, h, w, 2*w, 0) : null;
        UpSampler edi = (scale == 2) ? new EdgeDirectedUpSampler(w, h) : null;
        w *= scale;
        h *= scale;
        UpSampler[] superSamplers = new UpSampler[] { uBilinear, uBicubic, edi };
        String[] titles = new String[] { "Bilinear", "Bicubic", "Edge Oriented" };
        System.out.println(w + "x" + h);
        System.out.println();
        long delta = 0;
        Image res = null;

        // Round trip down / up
        for (int s=0; s<superSamplers.length; s++)
        {
           if (superSamplers[s] == null)
              continue;
           
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
           
           if (refImage != null)
              refImage.getRaster().getDataElements(0, 0, w, h, ref);

           for (int i=0; i<output.length; i++)
              output[i] = (ro[i] << 16) | (go[i] << 8) | bo[i];

           if (refImage != null)
           {
             System.out.println("Elapsed [ms] ("+iter+" iterations): "+delta/1000000);
             int psnr1024 = new ImageQualityMonitor(w, h).computePSNR(output, ref);
             System.out.println("PSNR: "+ (float) psnr1024 /1024);
           }

           title += " scale by " + scale;
           BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
           img2.getRaster().setDataElements(0, 0, w, h, output);

           if (displayResult == true)
           {
              JFrame frame2 = new JFrame(title);
              frame2.setBounds(50+100*s, 50+100*s, w, h);
              ImageIcon icon2 = new ImageIcon(img2);
              frame2.add(new JLabel(icon2));
              frame2.setVisible(true);
           }
           
           res = img2;
        }
        
        return res;
    }
    
}