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

package kanzi.util;


public class ImageSymmetries
{
   private int[] iBuf;
   private byte[] bBuf;
   private final int width;
   private final int height;
   private final int stride;

   
   public ImageSymmetries(int width, int height)
   {
      this(width, height, width);
   }
   
   
   public ImageSymmetries(int width, int height, int stride)
   {
       if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

       if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");

       if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");
       
      this.iBuf = new int[0];
      this.bBuf = new byte[0];
      this.width = width;
      this.height = height;
      this.stride = stride;
   }
   
   
   public int[] flip(int[] data)
   {
      final int w = this.width;
      final int h = this.height;
      final int st = this.stride;
      
      if (this.iBuf.length < w)
         this.iBuf = new int[w];
      
      final int h2 = h >> 1;
      int offs1 = 0;
      int offs2 = (h-1) * st;
      
      for (int j=0; j<h2; j++)
      {
         System.arraycopy(data, offs2, this.iBuf, 0, w);
         System.arraycopy(data, offs1, data, offs2, w);
         System.arraycopy(this.iBuf, 0, data, offs1, w);
         offs1 += st;
         offs2 -= st;
      }
      
      return data;
   }
   
   
   public int[] mirror(int[] data)
   {
      final int w = this.width - 1;
      final int h = this.height;
      final int st = this.stride;
      final int w2 = this.width >> 1;
      int offs = 0;
      
      for (int j=0; j<h; j++)
      {
         for (int i=0; i<w2; i++)
         {
            final int tmp = data[offs+w-i];
            data[offs+w-i] = data[offs+i];
            data[offs+i] = tmp;
         }
         
         offs += st; 
      }
      
      return data;
   }  
   
   
   public byte[] flip(byte[] data)
   {
      final int w = this.width;
      final int h = this.height;
      final int st = this.stride;
      
      if (this.bBuf.length < w)
         this.bBuf = new byte[w];
      
      final int h2 = h >> 1;
      int offs1 = 0;
      int offs2 = (h-1) * st;
      
      for (int j=0; j<h2; j++)
      {
         System.arraycopy(data, offs2, this.bBuf, 0, w);
         System.arraycopy(data, offs1, data, offs2, w);
         System.arraycopy(this.bBuf, 0, data, offs1, w);
         offs1 += st;
         offs2 -= st;
      }
      
      return data;
   }

   
   public byte[] mirror(byte[] data)
   {
      final int w = this.width - 1;
      final int h = this.height;
      final int st = this.stride;
      final int w2 = this.width >> 1;
      int offs = 0;
      
      for (int j=0; j<h; j++)
      {
         for (int i=0; i<w2; i++)
         {
            final byte tmp = data[offs+w-i];
            data[offs+w-i] = data[offs+i];
            data[offs+i] = tmp;
         }
         
         offs += st; 
      }
      
      return data;
   }     
}
