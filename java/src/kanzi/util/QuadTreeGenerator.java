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

package kanzi.util;

import java.util.Collection;
import java.util.TreeSet;


public class QuadTreeGenerator
{
    private final int minNodeDim;
    private final boolean isRGB;


    public QuadTreeGenerator(int minNodeDim, boolean isRGB)
    {
      this.minNodeDim = minNodeDim;
      this.isRGB = isRGB;
   }


   // Quad-tree decomposition of the input image based on variance of each node
   // The decomposition stops when enough nodes have been computed or the minimum
   // node dimension has been reached.
   // Input nodes are reused and new nodes are added to the input collection (if needed).
   public Collection<Node> decomposeNodes(Collection<Node> list, int[] input, int nbNodes, int stride)
   {
      if (nbNodes < list.size())
         throw new IllegalArgumentException("The target number of nodes must be at least list.size()");

      if (nbNodes == list.size())
         return list;
      
      return this.decompose(list, input, nbNodes, -1, stride);
   }


   // Quad-tree decomposition of the input image based on variance of each node
   // The decomposition stops when all the nodes in the tree have a variance lower
   // than or equal to the target variance or the minimum node dimension has been
   // reached.
   // Input nodes are reused and new nodes are added to the input collection (if needed).
   public Collection<Node> decomposeVariance(Collection<Node> list, int[] input, int variance, int stride)
   {
      if (variance < 0)
         throw new IllegalArgumentException("The target variance of nodes must be at least 0");

      return this.decompose(list, input, -1, variance, stride);
   }


   protected Collection<Node> decompose(Collection<Node> list, int[] input,
           int nbNodes, int variance, int stride)
   {
      if (list == null)
         return null;
      
      final TreeSet<Node> processed = new TreeSet<Node>();
      final TreeSet<Node> nodes = new TreeSet<Node>();

      for (Node node : list)
      {
         if ((node.w <= this.minNodeDim) || (node.h <= this.minNodeDim))
            processed.add(node);
         else
            nodes.add(node);
      }      

      while ((nodes.size() > 0) && ((nbNodes < 0) || (processed.size() + nodes.size() < nbNodes)))
      {
         Node parent = nodes.pollFirst();

         if ((parent.w <= this.minNodeDim) || (parent.h <= this.minNodeDim))
         {
            processed.add(parent);
            continue;
         }

         if ((variance >= 0) && (parent.variance <= variance))
         {
            processed.add(parent);
            continue;
         }

         // Create 4 children, taking into account odd dimensions
         final int pw = parent.w;
         final int ph = parent.h;
         final int px = parent.x;
         final int py = parent.y;
         final int cw = (pw + 1) >> 1;
         final int ch = (ph + 1) >> 1;
         
         Node node1 = getNode(parent, px, py, cw, ch, this.isRGB);
         Node node2 = getNode(parent, px+pw-cw, py, cw, ch, this.isRGB);
         Node node3 = getNode(parent, px, py+ph-ch, cw, ch, this.isRGB);
         Node node4 = getNode(parent, px+pw-cw, py+ph-ch, cw, ch, this.isRGB);

         node1.computeVariance(input, stride);
         node2.computeVariance(input, stride);
         node3.computeVariance(input, stride);
         node4.computeVariance(input, stride);

         // Add to set of nodes sorted by decreasing variance
         nodes.add(node1);
         nodes.add(node2);
         nodes.add(node3);
         nodes.add(node4);
      }
      
      nodes.addAll(processed);
      list.clear();    
      list.addAll(nodes);
      return list;
   }


   public static Node getNode(Node parent, int x, int y, int w, int h, boolean isRGB)
   {
      // TODO: optimize allocation
      return new Node(parent, x, y, w, h, isRGB);
   }


   public static class Node implements Comparable<Node>
   {
      public final Node parent;
      public int x;
      public int y;
      public int w;
      public int h;
      public int variance;
      public final boolean isRGB;

      private Node(Node parent, int x, int y, int w, int h, boolean isRGB)
      {
         this.parent = parent;
         this.x = x;
         this.y = y;
         this.w = w;
         this.h = h;
         this.isRGB = isRGB;
      }


      @Override
      public int compareTo(Node o)
      {
         if (o == null)
            return -1;
         
         if (o == this)
            return 0;
         
         // compare by decreasing variance
         final int val = o.variance - this.variance;

         if (val != 0)
            return val;

         // In case of equal variance values, order does not matter
         return o.hashCode() - this.hashCode();
      }


      @Override
      public boolean equals(Object o)
      {
         try
         {
            if (o == null)
               return false;
            
           if (o == this)
              return true;

           Node n = (Node) o;

           if (this.x != n.x)
              return false;

           if (this.y != n.y)
              return false;

           if (this.w != n.w)
              return false;

           return (this.h == n.h);
         }
         catch (ClassCastException e)
         {
            return false;
         }
      }


      @Override
      public int hashCode()
      {
         int hash = 3;
         hash = 79 * hash + this.x;
         hash = 79 * hash + this.y;
         hash = 79 * hash + this.w;
         hash = 79 * hash + this.h;
         //hash = 79 * hash + this.variance;
        
         return hash;
      }


      public int computeVariance(int[] buffer, int stride)
      {
         return (this.isRGB == true) ? this.computeVarianceRGB(buffer, stride) :
             this.computeVarianceY(buffer, stride);
      }


      private int computeVarianceRGB(int[] rgb, int stride)
      {
         final int iend = this.x + this.w;
         final int jend = this.y + this.h;
         final int len = this.w * this.h;
         long sq_sumR = 0, sq_sumB = 0, sq_sumG = 0;
         long sumR = 0, sumG = 0, sumB = 0;
         int offs = this.y * stride;

         for (int j=this.y; j<jend; j++)
         {
            for (int i=this.x; i<iend; i++)
            {
               final int pixel = rgb[offs+i];
               final int r = (pixel >> 16) & 0xFF;
               final int g = (pixel >>  8) & 0xFF;
               final int b =  pixel & 0xFF;
               sumR += r;
               sumG += g;
               sumB += b;
               sq_sumR += (r*r);
               sq_sumG += (g*g);
               sq_sumB += (b*b);
            }

            offs += stride;
         }

         final long vR = (sq_sumR - ((sumR * sumR) / len));
         final long vG = (sq_sumG - ((sumG * sumG) / len));
         final long vB = (sq_sumB - ((sumB * sumB) / len));
         this.variance = (int) ((vR + vG + vB) / (3 * len));

         return this.variance;
      }


      private int computeVarianceY(int[] yBuffer, int stride)
      {
         final int iend = this.x + this.w;
         final int jend = this.y + this.h;
         final int len = this.w * this.h;
         long sq_sum = 0;
         long sum = 0;
         int offs = this.y * stride;
         
         for (int j=this.y; j<jend; j++)
         {
            for (int i=this.x; i<iend; i++)
            {
               final int pixel = yBuffer[offs+i];
               sum += pixel;
               sq_sum += (pixel*pixel);
            }

            offs += stride;
         }

         this.variance = (int) ((sq_sum - ((sum * sum) / len)) / len);
         return this.variance;
      }


      @Override
      public String toString()
      {
         StringBuilder builder = new StringBuilder(200);
         builder.append('[');
         builder.append("x=");
         builder.append(this.x);
         builder.append(", y=");
         builder.append(this.y);
         builder.append(", w=");
         builder.append(this.w);
         builder.append(", h=");
         builder.append(this.h);
         builder.append(", variance=");
         builder.append(this.variance);
         builder.append(']');
         return builder.toString();
      }
   }
}
