/*
 * To change this license header, choose License Headers in Project Properties.
 * To change this template file, choose Tools | Templates
 * and open the template in the editor.
 */
package kanzi.test;

import java.awt.GraphicsConfiguration;
import java.awt.GraphicsDevice;
import java.awt.GraphicsEnvironment;
import java.awt.Image;
import java.awt.Transparency;
import java.awt.image.BufferedImage;
import java.io.File;
import javax.imageio.ImageIO;
import javax.swing.ImageIcon;
import javax.swing.JFrame;
import javax.swing.JLabel;
import kanzi.ColorModelType;
import kanzi.util.DeblockingFilter;
import kanzi.prediction.LossyIntraPredictor;
import kanzi.util.ImageUtils;
import kanzi.util.color.ColorModelConverter;
import kanzi.util.color.ReversibleYUVColorModelConverter;
import kanzi.util.color.YCbCrColorModelConverter;


/**
 *
 * @author fred
 */
public class TestDeblockingFilter
{
  public static void main(String[] args)
  {
      try
      {
        String fileName = (args.length > 0) ? args[0] : "r:\\lena_blocky.jpg";
        Image image1 = ImageIO.read(new File(fileName));        
        int iw = image1.getWidth(null);
        int ih = image1.getHeight(null);
        ImageUtils iu = new ImageUtils(iw, ih);
        int w = (iw + 31) & -32;
        int h = (ih + 31) & -32;
        GraphicsDevice gs = GraphicsEnvironment.getLocalGraphicsEnvironment().getScreenDevices()[0];
        GraphicsConfiguration gc = gs.getDefaultConfiguration();
        BufferedImage img1 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
        BufferedImage img2 = gc.createCompatibleImage(w, h, Transparency.OPAQUE);
       
        img1.getGraphics().drawImage(image1, 0, 0, null);
        System.out.println(w+"x"+h);
        int[] rgb1 = new int[w*h];
        int[] rgb2 = new int[w*h];

        boolean lossless = false; //!!!
        int chromaShift;
        ColorModelConverter cvt;
        ColorModelType cmt;
        
        if (lossless == true)
        {
           cvt = new ReversibleYUVColorModelConverter(w, h);
           cmt = ColorModelType.YUV444;
           chromaShift = 0;
        }
        else
        {
           cvt = new YCbCrColorModelConverter(w, h);   
           cmt = ColorModelType.YUV420;
           chromaShift = 1;
        }
        
        int[] y1 = new int[w*h];
        int[] u1 = new int[(w*h)>>(chromaShift+chromaShift)];
        int[] v1 = new int[(w*h)>>(chromaShift+chromaShift)];
        int[] y2 = new int[w*h];
        int[] u2 = new int[(w*h)>>(chromaShift+chromaShift)];
        int[] v2 = new int[(w*h)>>(chromaShift+chromaShift)];

        // Do NOT use img.getRGB(): it is more than 10 times slower than
        // img.getRaster().getDataElements()
        img1.getRaster().getDataElements(0, 0, w, h, rgb1);        
        iu.pad(rgb1, w, h);
        System.arraycopy(rgb1, 0, rgb2, 0, w*h);
                        
        cvt.convertRGBtoYUV(rgb1, y1, u1, v1, cmt);
        cvt.convertRGBtoYUV(rgb2, y2, u2, v2, cmt);
        DeblockingFilter dfY = new DeblockingFilter(w, h, w);
        DeblockingFilter dfUV = new DeblockingFilter(w>>chromaShift, h>>chromaShift, w>>chromaShift);
        
        for (int y=0; y<h; y+=8)
           for (int x=0; x<w; x+=8) {
              dfY.apply(y2, x, y, 8, LossyIntraPredictor.DIR_LEFT, 0, true);
           }
        
        for (int y=0; y<h>>chromaShift; y+=(8>>chromaShift))
           for (int x=0; x<w>>chromaShift; x+=(8>>chromaShift)) {
              dfUV.apply(u2, x, y, 8>>chromaShift, LossyIntraPredictor.DIR_LEFT, 0, false);
              dfUV.apply(v2, x, y, 8>>chromaShift, LossyIntraPredictor.DIR_LEFT, 0, false);
           }
        
        cvt.convertYUVtoRGB(y1, u1, v1, rgb1, cmt);
        cvt.convertYUVtoRGB(y2, u2, v2, rgb2, cmt);

        img1.getRaster().setDataElements(0, 0, w, h, rgb1);
        img2.getRaster().setDataElements(0, 0, w, h, rgb2);

        ImageIcon icon1 = new ImageIcon(img1);
        ImageIcon icon2 = new ImageIcon(img2);
        
        final JFrame frame1 = new JFrame("Source");
        frame1.setBounds(50, 30, w, h);
        frame1.add(new JLabel(icon1));
        frame1.setVisible(true);
        final JFrame frame2 = new JFrame("Result");
        frame2.setBounds(650, 30, w, h);
        frame2.add(new JLabel(icon2));      
        frame2.setVisible(true);
        Thread.sleep(75000);
        System.exit(0);
      }
      catch (Exception e)
      {
         e.printStackTrace();
      }

      System.exit(0);
   }     
}
