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
import java.util.Arrays;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.ColorModelType;
import kanzi.prediction.LossyIntraPredictor;
import kanzi.prediction.Prediction;
import kanzi.util.image.ImageQualityMonitor;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.YCbCrColorModelConverter;


public class TestIntraPredictor
{
   public static void main(String[] args)
   {
      try
      {
        String fileName = (args.length > 0) ? args[0] : "c:\\temp\\lena.jpg";
        Image image1 = new ImageIcon(fileName).getImage();
        final int w = image1.getWidth(null) & -8;
        final int h = image1.getHeight(null) & -8;
        System.out.println(w+"x"+h);
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img1 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);

        img1.getGraphics().drawImage(image1, 0, 0, null);
        int[] rgb1 = new int[w*h];
        int[] rgb2 = new int[w*h];
        int[] y1 = new int[w*h];
        int[] u1 = new int[w*h];
        int[] v1 = new int[w*h];
        int[] y2 = new int[w*h];
        int[] u2 = new int[w*h];
        int[] v2 = new int[w*h];

        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img1.getRaster().getDataElements(0, 0, w, h, rgb1);
        ColorModelConverter cvt = new YCbCrColorModelConverter(w, h);
        cvt.convertRGBtoYUV(rgb1, y1, u1, v1, ColorModelType.YUV420);
        cvt.convertRGBtoYUV(rgb1, y2, u2, v2, ColorModelType.YUV420);

        // Wipe out y2
        Arrays.fill(y2, 0);

        {
          int predictionType = LossyIntraPredictor.DIR_RIGHT | LossyIntraPredictor.REFERENCE;
          System.out.println("Direction: Right");
          testRoundTrip(y1, y2, w, h, 4, predictionType);
          testRoundTrip(y1, y2, w, h, 8, predictionType);
          testRoundTrip(y1, y2, w, h, 16, predictionType);
          testRoundTrip(y1, y2, w, h, 32, predictionType);
        }
        
        System.out.println();
        
        {
          int predictionType = LossyIntraPredictor.DIR_LEFT | LossyIntraPredictor.REFERENCE; 
          System.out.println("Direction: Left");
          testRoundTrip(y1, y2, w, h, 4, predictionType);
          testRoundTrip(y1, y2, w, h, 8, predictionType);
          testRoundTrip(y1, y2, w, h, 16, predictionType);
          testRoundTrip(y1, y2, w, h, 32, predictionType);
        }
        
        cvt.convertYUVtoRGB(y2, u2, v2, rgb2, ColorModelType.YUV420);
        img2.getRaster().setDataElements(0, 0, w, h, rgb2);

        int psnr1024 = new ImageQualityMonitor(w, h).computePSNR(y1, y2, ColorModelType.YUV444);
        System.out.println("PSNR: "+((psnr1024 == 0) ? "Infinite" : ((float) psnr1024/1024.0f)));

        ImageIcon icon1 = new ImageIcon(img1);
        ImageIcon icon2 = new ImageIcon(img2);
        final JFrame frame1 = new JFrame("Source");
        frame1.setBounds(50, 30, w, h);
        frame1.add(new JLabel(icon1));
        frame1.setVisible(true);
        final JFrame frame2 = new JFrame("Result");
        frame2.setBounds(900, 30, w, h);
        frame2.add(new JLabel(icon2));
        frame2.setVisible(true);
        Thread.sleep(30000);
      }
      catch (Exception e)
      {
         e.printStackTrace();
      }

      System.exit(0);
   }


   private static void testRoundTrip(int[] frame1, int[] frame2, int w, int h, int dim, int predictionType)
   {
      LossyIntraPredictor predictor = new LossyIntraPredictor(w, h, 32, w, false, 5, 2);
      Prediction[] results = new Prediction[10];
      System.out.println();

      for (int i=0; i<results.length; i++)
         results[i] = new Prediction(32);

      for (int j=0; j<h; j+=dim)
      {
         for (int i=0; i<w; i+=dim)
         {            
             System.out.println("Processing "+i+"@"+j+" dim="+dim);
             predictor.computeResidues(frame1, i, j, null, i, j, results, dim, predictionType, true);

             for (int nn=0; nn<results.length; nn++)
             {
                Prediction pred = results[nn];

                if (pred.sad != LossyIntraPredictor.MAX_ERROR)
                {
                   for (int jj=j; jj<j+dim; jj++)
                      for (int ii=i; ii<i+dim; ii++)
                         frame2[jj*w+ii] = 0;
                   
                   System.out.println("Prediction "+LossyIntraPredictor.Mode.getMode(nn));
                   predictor.computeBlock(pred, frame2, i, j, LossyIntraPredictor.Mode.getMode(nn), predictionType);
                   long error = computeError(frame1, frame2, i, j, w, dim);
                   System.out.println("Error average: "+(float) (error)/(dim*dim));

                   if (error != 0)
                   {
                      for (int k=0; k<dim*dim; k++)
                         System.out.print(pred.residue[k]+" ");

                      System.out.println();
                      System.out.println("Errors:");

                      for (int jj=j; jj<j+dim; jj++)
                      {
                        for (int ii=i; ii<i+dim; ii++)
                           System.out.print("("+frame2[jj*w+ii]+","+frame1[jj*w+ii]+") ");
                      }

                      System.out.println("");
//                      predictor.computeResidues(frame1, i, j, null, i, j, results, dim,
//                           IntraPredictor.DIR_LEFT | IntraPredictor.DIR_RIGHT | IntraPredictor.REFERENCE);
//                      predictor.computeBlock(pred, frame2, i, j, dim, IntraPredictor.Mode.getMode(nn));
                      System.exit(1);
                   }
                }
             }
         }
      }
   }


   private static long computeError(int[] frame1, int[] frame2, int x, int y, int w, int dim)
   {
      long error = 0;

      for (int j=y; j<y+dim; j++)
      {
         for (int i=x; i<x+dim; i++)
         {
            int diff = frame2[j*w+i] - frame1[j*w+i];
            error += (diff*diff);
         }
      }

      return error;
   }
}
