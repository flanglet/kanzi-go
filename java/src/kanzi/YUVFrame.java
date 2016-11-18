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

package kanzi;


public final class YUVFrame
{
   public static final int Y_IDX  = 0;
   public static final int U_IDX  = 1;
   public static final int V_IDX  = 2;

   public final int width;
   public final int height;
   public final int stride;
   public final int offset;
   public final int[] y;
   public final int[] u;
   public final int[] v;
   public int[][] channels;
   public final ColorModelType cmType;


   public YUVFrame(int width, int height, int[] y, int[] u, int[] v)
   {
      this(width, height, width, 0, y, u, v, ColorModelType.YUV420);
   }


   public YUVFrame(int width, int height, int[] y, int[] u, int[] v, ColorModelType cmType)
   {
      this(width, height, width, 0, y, u, v, cmType);
   }


   public YUVFrame(int width, int height, int stride, int offset,
           int[] y, int[] u, int[] v, ColorModelType cmType)
   {
      if (y == null)
         throw new NullPointerException("Invalid null Y channel parameter");

      if (u == null)
         throw new NullPointerException("Invalid null U channel parameter");

      if (v == null)
         throw new NullPointerException("Invalid null V channel parameter");

      if (cmType == null)
         throw new NullPointerException("Invalid null color model parameter");

      this.width = width;
      this.height = height;
      this.stride = stride;
      this.offset = offset;
      this.y = y;
      this.u = u;
      this.v = v;
      this.channels = new int[][] { y, u, v };
      this.cmType = cmType;
   }

   
   @Override
   public String toString()
   {
      StringBuilder sb = new StringBuilder(200);
      sb.append("{ \"width\":").append(this.width);
      sb.append(", \"height\":").append(this.height);
      sb.append(", \"stride\":").append(this.stride);
      sb.append(", \"offset\":").append(this.offset);
      sb.append(", \"color model\":\"").append(this.cmType.name()).append("\"");
      sb.append(" }");
      return sb.toString();
   }     
}
