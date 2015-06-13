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
import java.io.FileInputStream;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.ColorModelType;
import kanzi.IntTransform;
import kanzi.transform.RSET;
import kanzi.util.ImageQualityMonitor;
import kanzi.util.ImageUtils;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.ReversibleYUVColorModelConverter;


// Test RSET (bijective image transform) 
// Generate a file that can be entropy encoded for full lossless image compression
public class TestRSET
{
   static final int DIM0 = 16;
   
   public static void main(String[] args) 
   {
      try
      {
         String fileName = (args.length > 0) ? args[0] : "r:\\lena.jpg";
         int channels = 3;

         // Load image (PPM/PGM supported)
         String type = fileName.substring(fileName.lastIndexOf(".")+1);
         ImageUtils.ImageInfo ii = ImageUtils.loadImage(new FileInputStream(fileName), type);
         roundTrip(ii, fileName, channels);
         Thread.sleep(40000);
         System.exit(0);       
      }
      catch (Exception e)
      {
         e.printStackTrace();
      }   
   }

   
   private static int[][] roundTrip(ImageUtils.ImageInfo ii, String fileName, int channels) throws Exception
   {
      if (ii == null)
      {
         System.out.println("Cannot find file "+fileName);
         System.exit(1);
      }

      int ow = ii.width;
      int oh = ii.height;

      int dim = Math.min(ow, oh);
      int steps = 9;

      while ((dim >> steps) < DIM0)
         steps--;

      int mask = 1 << steps;
      int w = (ow + mask - 1) & -mask;
      int h = (oh + mask - 1) & -mask;

      GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
      GraphicsConfiguration gc = gs.getDefaultConfiguration();
      BufferedImage img1 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
      BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
      BufferedImage img3 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
      int[][] src = new int[channels][w*h];
      int[][] transf = new int[channels][w*h];
      int[][] dst = new int[channels][w*h];
      
      for (int i=0; i<channels; i++)
      {
         src[i] = new int[w*h];
         transf[i] = new int[w*h];
         dst[i] = new int[w*h];
      }
  
      int[] rgb1 = new ImageUtils(ow, oh).pad(ii.data, w, h);
      int[] rgb2 = new int[rgb1.length];
      int[] rgb3 = new int[rgb1.length];
 
      if (channels == 1)
      {
         for (int i=0; i<w*h; i++)
            src[0][i] = rgb1[i] & 0xFF;
      }
      else
      {
         ColorModelConverter cvt = new ReversibleYUVColorModelConverter(w, h);
         cvt.convertRGBtoYUV(rgb1, src[0], src[1], src[2], ColorModelType.YUV444);
      }
      
      System.out.println("File: " + fileName);
      System.out.println(ow+"x"+oh+" => "+w+"x"+h);
      System.out.println("Channels: " + channels);
      System.out.println("Steps: " + steps);
    
      for (int i=0; i<channels; i++)
      {
         compressChannel(w, h, src[i], transf[i], steps);
         decompressChannel(w, h, transf[i], dst[i], steps);
      }

      if (channels == 1)
      {
         for (int i=0; i<w*h; i++)
         {
            rgb2[i] =
               ((transf[0][i] & 0xFF) << 16) | 
               ((transf[0][i] & 0xFF) << 8) | 
                (transf[0][i] & 0xFF);
            rgb3[i] =
               ((dst[0][i] & 0xFF) << 16) | 
               ((dst[0][i] & 0xFF) << 8) | 
                (dst[0][i] & 0xFF);
         }
      }
      else
      {
         ColorModelConverter cvt = new ReversibleYUVColorModelConverter(w, h);
         cvt.convertYUVtoRGB(transf[0], transf[1], transf[2], rgb2, ColorModelType.YUV444);
         cvt.convertYUVtoRGB(dst[0], dst[1], dst[2], rgb3, ColorModelType.YUV444);
      }

      img1.getRaster().setDataElements(0, 0, w, h, rgb1);
      img2.getRaster().setDataElements(0, 0, w, h, rgb2);
      img3.getRaster().setDataElements(0, 0, w, h, rgb3);
      int psnr1024 = new ImageQualityMonitor(w, h).computePSNR(rgb1, rgb3);
      System.out.println("PSNR: "+ ((psnr1024 == 0) ? "Infinite" : (float) psnr1024/1024.0f));
      JFrame frame1 = new JFrame("Original");
      frame1.setBounds(50, 100, w, h);
      ImageIcon newIcon1 = new ImageIcon(img1);
      frame1.add(new JLabel(newIcon1));
      frame1.setVisible(true);      
      JFrame frame2 = new JFrame("Forward Transform");
      frame2.setBounds(400, 150, w, h);
      ImageIcon newIcon2 = new ImageIcon(img2);
      frame2.add(new JLabel(newIcon2));
      frame2.setVisible(true);
      JFrame frame3 = new JFrame("Reverse Transform");
      frame3.setBounds(750, 200, w, h);
      ImageIcon newIcon3 = new ImageIcon(img3);
      frame3.add(new JLabel(newIcon3));
      frame3.setVisible(true);
      return transf;
   }
   
   
   // apply forward transform
   private static int[] compressChannel(int w, int h, int[] src, int[] transformed, int steps)
   {  
      IndexedIntArray iia1 = new IndexedIntArray(src, 0);
      IndexedIntArray iia2 = new IndexedIntArray(transformed, 0);
      IntTransform transform = new RSET(w, h, w, steps); 
      transform.forward(iia1, iia2);   
      return transformed;            
   }


   // apply reverse transform
   private static int[] decompressChannel(int w, int h, int[] src, int[] dst, int steps)
   {      
      IndexedIntArray iia1 = new IndexedIntArray(src, 0);
      IndexedIntArray iia2 = new IndexedIntArray(dst, 0);
      IntTransform transform = new RSET(w, h, w, steps);   
      transform.inverse(iia1, iia2);    
      return dst;            
   }

}