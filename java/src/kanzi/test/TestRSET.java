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
import java.awt.GraphicsConfiguration;
import java.awt.GraphicsDevice;
import java.awt.GraphicsEnvironment;
import java.awt.Transparency;
import java.awt.image.BufferedImage;
import java.io.File;
import java.io.FileOutputStream;
import javax.imageio.ImageIO;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.ColorModelType;
import kanzi.IntTransform;
import kanzi.transform.RSET;
import kanzi.util.ImageQualityMonitor;
import kanzi.util.ImageSymmetries;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.ReversibleYUVColorModelConverter;


// Test RSET (bijective imafe transform) 
// Generate a file that can be entropy encoded for full lossless image compression
public class TestRSET
{
   public static void main(String[] args)
   {
      try
      {
         String fileName = (args.length > 0) ? args[0] : "c:\\temp\\lena.jpg";
         BufferedImage image = ImageIO.read(new File(fileName));
         int w = image.getWidth(null);
         int h = image.getHeight(null);

         if (image.getWidth(null) <= 0)
         {
            System.out.println("Cannot find file "+fileName);
            System.exit(1);
         }
//adjust (dim+15)&-16         
// header 
// 2 bits : header size bytes - 2
// 2 bits symmetry
// 1 bit progressive
// 3 bits steps-1 
// 12 bits * 2 dims/4  
        
                     
         GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
         GraphicsConfiguration gc = gs.getDefaultConfiguration();
         BufferedImage img1  = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         BufferedImage img3 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
         img1.getGraphics().drawImage(image, 0, 0, null);
         int[] orig = new int[w*h];
         int[][] src = new int[][] { new int[w*h], new int[w*h], new int[w*h] };
         int[][] transformed = new int[][] { new int[w*h], new int[w*h], new int[w*h] };
         int[][] dst = new int[][] { new int[w*h], new int[w*h], new int[w*h] };
         img1.getRaster().getDataElements(0, 0, w, h, orig);

         ImageSymmetries is = new ImageSymmetries(w, h);  
         boolean flip = true;
         boolean mirror = true;
         
         if (flip)
           orig = is.flip(orig);
             
         if (mirror)
           orig = is.mirror(orig);
             
         int steps = 5;
         System.out.println("File: " + fileName);
         System.out.println("Steps: " + steps);
         FileOutputStream fos = new FileOutputStream("r:\\data.bin");         
         
         ColorModelConverter cvt = new ReversibleYUVColorModelConverter(w, h);
         //cvt = new YCbCrColorModelConverter(w, h);
         cvt.convertRGBtoYUV(orig, src[0], src[1], src[2], ColorModelType.YUV444);
         
//         for (int i=0; i<w*h; i++)
//         {
//            src[0][i] =  orig[i] & 0xFF;
//            src[1][i] = (orig[i] >> 8) & 0xFF;
//            src[2][i] = (orig[i] >> 16) & 0xFF;
//         }
         
         byte[] buf;
         buf = processChannel(w, h, src[0], transformed[0], dst[0], 0, steps);
         fos.write(buf);
         buf = processChannel(w, h, src[1], transformed[1], dst[1], 1, steps);
         fos.write(buf);
         buf = processChannel(w, h, src[2], transformed[2], dst[2], 2, steps);
         fos.write(buf);
         fos.close(); 

//         cvt.convertYUVtoRGB(transformed[0], transformed[1], transformed[2], transformed[0], ColorModelType.YUV444);
         cvt.convertYUVtoRGB(dst[0], dst[1], dst[2], dst[0], ColorModelType.YUV444);

         for (int i=0; i<w*h; i++)
         {
            int val = ((transformed[0][i] & 0xFF) + (transformed[1][i] & 0xFF) + (transformed[2][i] & 0xFF)) / 3;
            transformed[0][i] = (val) | (val << 8) |
                (val << 16);
//            dst[0][i] = (dst[0][i] & 0xFF) | ((dst[1][i] & 0xFF) << 8) |
//                ((dst[2][i] & 0xFF) << 16);
         }

         img1.getRaster().setDataElements(0, 0, w, h, orig);
         img2.getRaster().setDataElements(0, 0, w, h, transformed[0]);
         img3.getRaster().setDataElements(0, 0, w, h, dst[0]); 
         
         int psnr1024 = new ImageQualityMonitor(w, h).computePSNR(orig, dst[0]);
         System.out.println("PSNR: " + (float) psnr1024/1024.f);

         JFrame frame3 = new JFrame("Inverse");
         frame3.setBounds(1180, 100, w, h);
         ImageIcon newIcon3 = new ImageIcon(img3);
         frame3.add(new JLabel(newIcon3));

         JFrame frame2 = new JFrame("Forward");
         frame2.setBounds(620, 100, w, h);
         ImageIcon newIcon2 = new ImageIcon(img2);
         frame2.add(new JLabel(newIcon2));

         JFrame frame1 = new JFrame("Original");
         frame1.setBounds(60, 100, w, h);
         ImageIcon newIcon1 = new ImageIcon(img1);
         frame1.add(new JLabel(newIcon1));

         frame1.setVisible(true);
         frame2.setVisible(true);
         frame3.setVisible(true);
      
         Thread.sleep(40000);
         System.exit(0);       
      }
      catch (Exception e)
      {
         e.printStackTrace();
      }
   }
   
   
   // apply RSET transform and inverse
   private static byte[] processChannel(int w, int h, int[] src, int[] transformed, int[] dst, int chanIdx, int steps)
   {  
      IndexedIntArray iia1 = new IndexedIntArray(src, 0);
      IndexedIntArray iia2 = new IndexedIntArray(transformed, 0);
      IndexedIntArray iia3 = new IndexedIntArray(dst, 0);

      IntTransform transform;
//      transform = new DWT_CDF_9_7(w, h, steps);
      transform = new RSET(w, h, steps);

      int dim0 = Math.min(w, h) >> steps;
      iia1.index = 0;
      iia2.index = 0;      
      transform.forward(iia1, iia2);   
      
//for (int j=0; j<h; j++)
//{
//   for (int i=0; i<w; i++)
//   {
//      if ((i>=16*dim0) || (j>=16*dim0))
//         transformed[j*w+i] = 0;
//   }
//}       

      transform.inverse(iia2, iia3);
      
      long sum =0, sumHH0 = 0;
      int dataIdx = 0;
      
      for (int j=0; j<h; j++)
      {
         for (int i=0; i<w; i++)
         {
            sum += Math.abs(transformed[j*w+i]);

            if ((i<dim0) && (j<dim0)) 
            {
               sumHH0 += Math.abs(transformed[j*w+i]);
            }
            else // for display
               transformed[j*w+i] = Math.min(Math.abs(transformed[j*w+i])*3, 255);
         }
      }
      
//      byte[] buf1 = new byte[w*h];
//      byte[] buf2 = new byte[w*h];
//      int[] indexes = new int[w*h];
//      WaveletBandScanner sc = new WaveletBandScanner(w, h, WaveletBandScanner.ALL_BANDS, steps);
//      sc.getIndexes(indexes);
//
//      for (int i=0; i<w*h; i++)
//         buf1[i] = (byte) transformed[indexes[i]];
 
      
      int[] tmp = new int[w*h];
      predict(transformed, tmp, w, h, dim0);
      System.arraycopy(tmp, 0, transformed, 0, w*h);

      int[] histo = new int[256];

      for (int i=0; i<w*h; i++)
         histo[transformed[i]&0xFF]++;      


//      for (int i=0; i<w*h; i++)  {
//         if (src[i] != dst[i])
//            System.out.println((dst[i]-src[i])+ " "+i);        
//      }

      String[] rgbs = new String[] { "Blue", "Green", "Red" };
      System.out.println("Channel: " + rgbs[chanIdx]);
      System.out.println("Energy: " + sum);
      System.out.println("Energy HH0: " + sumHH0);
      System.out.println("Energy HH0 / pixel : " + ((float) sumHH0)/(dim0*dim0));
      System.out.println("Energy non HH0 / pixel : " + ((float) sum-sumHH0)/(w*h-(dim0*dim0)));

//      for (int i=0; i<256; i++)
//         System.out.println(histo[i]+" ");
//
//      System.out.println();
      byte[] buf = new byte[w*h];
          
      for (int i=0; i<w*h; i++)
         buf[i] = (byte) transformed[i];

      return buf;            
   }

   
   private static void predict(int[] src, int[] dst, int w, int h, int dim0)
   {
      System.arraycopy(src, 0, dst, 0, w*h);
      
      for (int j=1; j<dim0-1; j++)
      {
         for (int i=1; i<dim0-1; i++)
         {      
            int a = src[j*w+i-1];
            int b = src[(j-1)*w+i];
            int c = src[(j-1)*w+i-1];
            int min = Math.min(a, b);
            int max = Math.max(a, b);
            int x = src[j*w+i];
            
            if (c >= max)
               dst[j*w+i] = min - x;
            else if (c <= min)
               dst[j*w+i] = max - x;
            else
               dst[j*w+i] = (a+b-c) - x;             
         }
      }
   }
}