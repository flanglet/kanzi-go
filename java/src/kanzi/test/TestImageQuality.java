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
import java.awt.image.BufferedImage;
import java.io.File;
import java.util.Arrays;
import java.util.Random;
import javax.imageio.ImageIO;
import kanzi.ColorModelType;
import kanzi.util.image.ImageQualityMonitor;


public class TestImageQuality
{
   public static void main(String[] args) throws Exception
   {
        String fileName1 = (args.length > 0) ? args[0] : "r:\\bees.png";
        String fileName2 = (args.length > 1) ? args[1] : "r:\\bees-webp.png";
        Image image1 = ImageIO.read(new File(fileName1));
        Image image2 = ImageIO.read(new File(fileName2));
        int w = image1.getWidth(null) & -8;
        int h = image1.getHeight(null) & -8;
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img1 = gc.createCompatibleImage(w, h, java.awt.Transparency.OPAQUE);
        BufferedImage img2 = gc.createCompatibleImage(w, h, java.awt.Transparency.OPAQUE);

        img1.getGraphics().drawImage(image1, 0, 0, null);
        img2.getGraphics().drawImage(image2, 0, 0, null);
        int[] rgb1 = new int[w*h];
        int[] rgb2 = new int[w*h];
        ImageQualityMonitor monitor;
        int psnr, ssim;
        Random rnd = new Random();

        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img1.getRaster().getDataElements(0, 0, w, h, rgb1);
        img2.getRaster().getDataElements(0, 0, w, h, rgb2);

        {
           img2.getRaster().setDataElements(0, 0, w, h, rgb2);
           System.out.println("\nNo distortion");
           monitor = new ImageQualityMonitor(w, h);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR: ", psnr, "SSIM: ", ssim);
        }


        {
           for (int i=0; i<((w*h+500)/1000); i++)
              rgb2[Math.abs(rnd.nextInt())/(w*h)] = rnd.nextInt();

           img2.getRaster().setDataElements(0, 0, w, h, rgb2);
           System.out.println("\nRandom noise (0.1% samples)");
           monitor = new ImageQualityMonitor(w, h, w);
           psnr = monitor.computePSNR(rgb1, rgb2, ColorModelType.RGB);
           ssim = monitor.computeSSIM(rgb1, rgb2, ColorModelType.RGB);
           printResults("PSNR: ", psnr, "SSIM: ", ssim);
           monitor = new ImageQualityMonitor(w&-16, h&-16, w&-16, 1);
           psnr = monitor.computePSNR(rgb1, rgb2, ColorModelType.RGB);
           ssim = monitor.computeSSIM(rgb1, rgb2, ColorModelType.RGB);
           printResults("PSNR (subsampled by 2x2): ", psnr, "SSIM (subsampled by 2x2): ", ssim);
           monitor = new ImageQualityMonitor(w&-32, h&-32, w&-32, 2);
           psnr = monitor.computePSNR(rgb1, rgb2, ColorModelType.RGB);
           ssim = monitor.computeSSIM(rgb1, rgb2, ColorModelType.RGB);
           printResults("PSNR (subsampled by 4x4): ", psnr, "SSIM (subsampled by 4x4): ", ssim);
        }


        {
           for (int i=0; i<((w*h+50)/100); i++)
              rgb2[Math.abs(rnd.nextInt())/(w*h)] = rnd.nextInt();

           img2.getRaster().setDataElements(0, 0, w, h, rgb2);
           System.out.println("\nRandom noise (1% samples)");
           monitor = new ImageQualityMonitor(w, h, w);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR: ", psnr, "SSIM: ", ssim);
           monitor = new ImageQualityMonitor(w&-16, h&-16, w&-16, 1);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR (subsampled by 2x2): ", psnr, "SSIM (subsampled by 2x2): ", ssim);
           monitor = new ImageQualityMonitor(w&-32, h&-32, w&-32, 2);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR (subsampled by 4x4): ", psnr, "SSIM (subsampled by 4x4): ", ssim);
        }


        {
           for (int i=0; i<((w*h+5)/10); i++)
              rgb2[Math.abs(rnd.nextInt())/(w*h)] = rnd.nextInt();

           img2.getRaster().setDataElements(0, 0, w, h, rgb2);
           System.out.println("\nRandom noise (10% samples)");
           monitor = new ImageQualityMonitor(w, h, w);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR: ", psnr, "SSIM: ", ssim);
           monitor = new ImageQualityMonitor(w&-16, h&-16, w&-16, 1);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR (subsampled by 2x2): ", psnr, "SSIM (subsampled by 2x2): ", ssim);
           monitor = new ImageQualityMonitor(w&-32, h&-32, w&-32, 2);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR (subsampled by 4x4): ", psnr, "SSIM (subsampled by 4x4): ", ssim);
        }


        {
           Arrays.fill(rgb2, 0);
           System.arraycopy(rgb1, 0, rgb2, 0, w*h/2);
           img2.getRaster().setDataElements(0, 0, w, h, rgb2);
           System.out.println("\nSecond image: half empty + half initial image");
           monitor = new ImageQualityMonitor(w, h, w);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR: ", psnr, "SSIM: ", ssim);
           monitor = new ImageQualityMonitor(w&-16, h&-16, w&-16, 1);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR (subsampled by 2x2): ", psnr, "SSIM (subsampled by 2x2): ", ssim);
           monitor = new ImageQualityMonitor(w&-32, h&-32, w&-32, 2);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR (subsampled by 4x4): ", psnr, "SSIM (subsampled by 4x4): ", ssim);
       }

       {
           Arrays.fill(rgb2, 0);
           img2.getRaster().setDataElements(0, 0, w, h, rgb2);
           System.out.println("\nSecond image: empty");
           monitor = new ImageQualityMonitor(w, h, w);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR: ", psnr, "SSIM: ", ssim);
           monitor = new ImageQualityMonitor(w&-16, h&-16, w&-16, 1);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR (subsampled by 2x2): ", psnr, "SSIM (subsampled by 2x2): ", ssim);
           monitor = new ImageQualityMonitor(w&-32, h&-32, w&-32, 2);
           psnr = monitor.computePSNR(rgb1, rgb2);
           ssim = monitor.computeSSIM(rgb1, rgb2);
           printResults("PSNR (subsampled by 4x4): ", psnr, "SSIM (subsampled by 4x4): ", ssim);
       }

//        javax.swing.JFrame frame = new javax.swing.JFrame("Image1");
//        frame.setBounds(50, 30, w, h);
//        frame.add(new javax.swing.JLabel(icon1));
//        frame.setVisible(true);
//        javax.swing.JFrame frame2 = new javax.swing.JFrame("Image2");
//        frame2.setBounds(600, 30, w, h);
//        frame2.add(new javax.swing.JLabel(icon2));
//        frame2.setVisible(true);

//        try
//        {
//            Thread.sleep(35000);
//        }
//        catch (Exception e)
//        {
//        }

        System.exit(0);
   }


   private static void printResults(String titlePSNR, int psnr, String titleSSIM, int ssim)
   {
      if (psnr != 0)
         System.out.println(titlePSNR+(float) psnr/1024);
      else
         System.out.println(titlePSNR+"Infinite");

      System.out.println(titleSSIM+(float) ssim/1024);
   }
}
