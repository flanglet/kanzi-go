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

package kanzi.filter;

import java.util.LinkedList;
import java.util.Random;
import kanzi.SliceIntArray;
import kanzi.IntFilter;
import kanzi.util.QuadTreeGenerator;


// A filter that splits the image into patches of similar colors using k-means
// clustering.
// This implementation contains several speed optimizations:
// a) The initial clusters are derived from a variance based quad-tree decomposition.
//    It also yields repeatable results (EG. when filtering the same image several
//    times) which is a requirement to apply the filter on image sequences.
// b) The main algorithm is applied to an image initially down scaled by 4 in each
//    direction, then (once sensible clusters have been identified), the sall image
//    is upscaled by 2. Only the creation of the final image applies on a full
//    scale image (by filling in the blanks for the pixels that were never processed)
// c) For each pixel, the initial value for the cluster is the one of the previous
//    pixel. As a result the initial value for the distance is small (high likelyhood
//    that adjacent pixels belong to the same cluster), meaning that the early exit
//    in the loop (no computation of 'color' distance) is used frequently.

public class ColorClusterFilter implements IntFilter
{
    private final int width;
    private final int height;
    private final int stride;
    private final int maxIterations;
    private final Cluster[] clusters;
    private final int[] buffer;
    private final short[] labels;
    private boolean chooseCentroids;
    private boolean showBorders;

    
    public ColorClusterFilter(int width, int height, int nbClusters)
    {
       this(width, height, width, nbClusters, 16, null);
    }


    public ColorClusterFilter(int width, int height, int stride, int nbClusters, int iterations)
    {
       this(width, height, stride, nbClusters, iterations, null);
    }


    // centroidXY is an optional array of packed (16 bits + 16 bits) centroid coordinates
    public ColorClusterFilter(int width, int height, int stride, int nbClusters, 
            int iterations, int[] centroidsXY)
    {
      if (height < 8)
         throw new IllegalArgumentException("The height must be at least 8");

      if (width < 8)
         throw new IllegalArgumentException("The width must be at least 8");

      if ((height & 3) != 0)
         throw new IllegalArgumentException("The height must be a multiple of 4");

      if ((width & 3) != 0)
         throw new IllegalArgumentException("The width must be a multiple of 4");

      if (stride < 8)
         throw new IllegalArgumentException("The stride must be at least 8");

      if ((nbClusters < 2) || (nbClusters >= 65536))
         throw new IllegalArgumentException("The number of clusters must be in [2..65535]");

      if (nbClusters > width*height)
         throw new IllegalArgumentException("The number of clusters must be less than the number of pixels]");

      if ((iterations < 2) || (iterations >= 256))
         throw new IllegalArgumentException("The maximum number of iterations must be in [2..255]");

      if ((centroidsXY != null) && (centroidsXY.length < nbClusters))
         throw new IllegalArgumentException("The number of centroid coordinates "
                 + "is less than the number of clusters");

      this.width = width;
      this.height = height;
      this.stride = stride;
      this.maxIterations = iterations;
      this.chooseCentroids = (centroidsXY == null);
      this.clusters = new Cluster[nbClusters];
      this.buffer = new int[width*height/4];
      this.labels = new short[width*height];

      for (int i=0; i<nbClusters; i++)
      {
         this.clusters[i] = new Cluster();
         
         if (centroidsXY != null)
         {
            // The work image is downscaled by 2 at the beginning of the process
            // Rescale the coordinates
            this.clusters[i].centroidX = ((centroidsXY[i] >> 16) & 0x0000FFFF) >> 1;
            this.clusters[i].centroidY =  (centroidsXY[i] & 0x0000FFFF) >> 1;
         }
      }
    }


    // Use K-Means algorithm to create clusters of pixels with similar colors
    @Override
    public boolean apply(SliceIntArray input, SliceIntArray output)
    {
      if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
         return false;
      
       int scale = 1; // for now
       int scaledW = this.width >> scale;
       int scaledH = this.height >> scale;
       int scaledSt = this.stride >> scale;
       final Cluster[] cl = this.clusters;
       final int nbClusters = cl.length;
       final int rescaleThreshold = (this.maxIterations * 2 / 3);
       int iterations = 0;

       // Create a down sampled copy of the source 
       int[] buf = this.createWorkImage(input.array, input.index, scale);

       // Choose centers
       if (this.chooseCentroids == true)
          this.chooseCentroids(this.clusters, buf, scaledW, scaledH);

       // Main loop, associate points to clusters and re-calculate centroids
       while (iterations < this.maxIterations)
       {
         int offs = 0;
         int moves = 0;

         // Associate a pixel to the nearest cluster
         for (int j=0; j<scaledH; j++, offs+=scaledSt)
         {
            int kfound = 0;

            for (int i=0; i<scaledW; i++)
            {
               final int pixel = buf[offs+i];
               final int r = (pixel >> 16) & 0xFF;
               final int g = (pixel >>  8) & 0xFF;
               final int b =  pixel & 0xFF;
               int minSqDist;

               {
                  // Reuse previous cluster as 'best initial guess' which yield
                  // a small value for 'nearest' most of the time
                  final Cluster refCluster = cl[kfound];
                  final int dx = i - refCluster.centroidX;
                  final int dy = j - refCluster.centroidY;
                  final int dr = r - refCluster.centroidR;
                  final int dg = g - refCluster.centroidG;
                  final int db = b - refCluster.centroidB;
               
                  // Distance is based on 3 color and 2 position coordinates
                  minSqDist = 16 * (dx*dx + dy*dy) + (dr*dr + dg*dg + db*db);
               }

               // Iterate over clusters, calculating pixel distance to centroid
               for (int k=0; k<nbClusters; k++)
               {
                  final Cluster cluster = cl[k];
                  final int dx = i - cluster.centroidX;
                  final int dy = j - cluster.centroidY;

                  // Distance is based on 3 color and 2 position coordinates
                  int sqDist = 16 *(dx*dx + dy*dy);

                  if (sqDist >= minSqDist) // early exit
                     continue;

                  final int dr = r - cluster.centroidR;
                  final int dg = g - cluster.centroidG;
                  final int db = b - cluster.centroidB;

                  // Distance is based on 3 color and 2 position coordinates
                  sqDist += (dr*dr + dg*dg + db*db);

                  if (sqDist >= minSqDist)
                     continue;

                  minSqDist = sqDist;
                  kfound = k;
               }

               final Cluster cluster = cl[kfound];
               this.labels[offs+i] = (short) kfound;
               cluster.sumR += r;
               cluster.sumG += g;
               cluster.sumB += b;
               cluster.sumX += i;
               cluster.sumY += j;
               cluster.items++;
            }
         }

         // Compute new centroid for each cluster
         for (int j=0; j<nbClusters; j++)
         {
            if (cl[j].computeCentroid() == true)
               moves++;
         }

         iterations++;

         if ((scale > 1) && ((iterations == rescaleThreshold) || (moves == 0)))
         {
            // Upscale by 2 in each dimension, now that centroids are somewhat stable
            scale--;
            scaledW <<= 1;
            scaledH <<= 1;
            scaledSt <<= 1;
            buf = this.createWorkImage(input.array, input.index, scale);

            for (int j=0; j<nbClusters; j++)
            {
               cl[j].centroidX <<= 1;
               cl[j].centroidY <<= 1;
            }
         }

         if (moves == 0)
            break;
      }

      for (int j=0; j<nbClusters; j++)
      {
         final Cluster c = cl[j];
         c.centroidValue = (c.centroidR << 16) | (c.centroidG << 8) | c.centroidB;
         c.centroidX <<= 1;
         c.centroidY <<= 1;
      }

      return this.createFinalImage(input, output);
   }


   // Create a down sampled copy of the source
   private int[] createWorkImage(int[] src, int srcStart, int scale)
   {
       final int[] buf = this.buffer;
       final int scaledW = this.width >> scale;
       final int scaledH = this.height >> scale;
       final int st = this.stride;
       final int scaledStride = st << scale;
       final int inc = 1 << scale;
       final int scale2 = scale + scale;
       final int adjust = 1 << (scale2 - 1);
       int srcIdx = srcStart;
       int dstIdx = 0;      

       for (int j=0; j<scaledH; j++)
       {
          for (int i=0; i<scaledW; i++)
          {
             int idx = srcIdx + (i << scale);
             int r = 0, g = 0, b = 0;

             // Take mean value of each pixel
             for (int jj=0; jj<inc; jj++)
             {
                for (int ii=0; ii<inc; ii++)
                {
                   final int pixel = src[idx+ii];
                   r += ((pixel >> 16) & 0xFF);
                   g += ((pixel >>  8) & 0xFF);
                   b +=  (pixel & 0xFF);
                }

                idx += st;
             }

             r = (r + adjust) >> scale2;
             g = (g + adjust) >> scale2;
             b = (b + adjust) >> scale2;
             buf[dstIdx++] = (r << 16) | (g << 8) | b;
          }

          srcIdx += scaledStride;
       }

       return buf;
   }


   // Up-sample and set all points in the cluster to the color of the centroid pixel
   private boolean createFinalImage(SliceIntArray source, SliceIntArray destination)
   {
      final int[] src = source.array;
      final int[] dst = destination.array;
      final int srcStart = source.index;
      final int dstStart = destination.index;
      final Cluster[] cl = this.clusters; 
      final short[] labels_ = this.labels; 
      final int scaledW = this.width >> 1;
      final int scaledH = this.height >> 1;
      final int st = this.stride;
      final int scaledSt = st >> 1;
      int offs = (scaledH - 1) * scaledSt;
      int nlOffs = offs;

      for (int j=this.height-2; j>=0; j-=2)
      {        
         Cluster c1 = cl[labels_[offs+scaledW-1]]; // pixel p1 to the right of current p0
         Cluster c3 = cl[labels_[nlOffs+scaledW-1]]; // pixel p3 to the right of p2
         final int srcIdx = srcStart + offs;
         final int dstIdx = dstStart + j * st;

         for (int i=this.width-2; i>=0; i-=2)
         {
            final int iOffs = srcIdx + (i>>1);
            final int oOffs = dstIdx + i;
            final int cluster0Idx = labels_[offs+(i>>1)];
            final Cluster c0 = cl[cluster0Idx];
            final int pixel0 = c0.centroidValue;
            final int c0r = c0.centroidR;
            final int c0g = c0.centroidG;
            final int c0b = c0.centroidB;
            final int c0x = c0.centroidX;
            final int c0y = c0.centroidY;
            final int cluster2Idx = labels_[nlOffs+(i>>1)];
            final Cluster c2 = cl[cluster2Idx]; // pixel p2 below current p0
            //dst[oOffs] = pixel0;
            dst[oOffs] = src[oOffs];

            if (c0 == c3)
            {
               // Inside cluster
               //dst[oOffs+st+1] = pixel0;
               dst[oOffs+st+1] = src[oOffs+st+1];
            }
            else if (this.showBorders == true)
            {
               dst[oOffs+st+1] = 0xFFFFFFFF;
            }
            else
            {
               // Diagonal cluster border
               final int pixel = src[iOffs+st+1];
               final int r = (pixel >> 16) & 0xFF;
               final int g = (pixel >>  8) & 0xFF;
               final int b =  pixel & 0xFF;
               final int d0 = 16 * ((i+1-c0x)*(i+1-c0x)+(j+1-c0y)*(j+1-c0y)) 
                            + (r-c0r)*(r-c0r)
                            + (g-c0g)*(g-c0g)
                            + (b-c0b)*(b-c0b);
               final int d3 = 16 * ((i+1-c3.centroidX)*(i+1-c3.centroidX)+(j+1-c3.centroidY)*(j+1-c3.centroidY))
                             + (r-c3.centroidR)*(r-c3.centroidR)
                             + (g-c3.centroidG)*(g-c3.centroidG)
                             + (b-c3.centroidB)*(b-c3.centroidB);
               dst[oOffs+st+1] = (d0 < d3) ? pixel0 : c3.centroidValue;
            }

            if (c0 == c2)
            {
               // Inside cluster
               //dst[oOffs+st] = pixel0;
               dst[oOffs+st] = src[oOffs+st];
            }
            else if (this.showBorders == true)
            {
               dst[oOffs+st] = 0xFFFFFFFF;
            }
            else
            {
               // Vertical cluster border
               final int pixel = src[iOffs+st];
               final int r = (pixel >> 16) & 0xFF;
               final int g = (pixel >>  8) & 0xFF;
               final int b =  pixel & 0xFF;
               final int d0 = 16 * ((i-c0x)*(i-c0x)+(j+1-c0y)*(j+1-c0y)) 
                            + (r-c0r)*(r-c0r)
                            + (g-c0g)*(g-c0g)
                            + (b-c0b)*(b-c0b);
               final int d2 = 16 * ((i-c2.centroidX)*(i-c2.centroidX)+(j+1-c2.centroidY)*(j+1-c2.centroidY))
                             + (r-c2.centroidR)*(r-c2.centroidR)
                             + (g-c2.centroidG)*(g-c2.centroidG)
                             + (b-c2.centroidB)*(b-c2.centroidB);
               dst[oOffs+st] = (d0 < d2) ? pixel0 : c2.centroidValue;
            }           

            if (c0 == c1)
            {
               // Inside cluster
             //  dst[oOffs+1] = pixel0;
               dst[oOffs+1] = src[oOffs+1];
            }
            else if (this.showBorders == true)
            {
               dst[oOffs+1] = 0xFFFFFFFF;
            }
            else
            {
               // Horizontal cluster border
               final int pixel = src[iOffs+1];
               final int r = (pixel >> 16) & 0xFF;
               final int g = (pixel >>  8) & 0xFF;
               final int b =  pixel & 0xFF;
               final int d0 = ((i+1-c0x)*(i+1-c0x)+(j-c0y)*(j-c0y)) 
                            + (r-c0r)*(r-c0r)
                            + (g-c0g)*(g-c0g)
                            + (b-c0b)*(b-c0b);
               final int d1 = 16 * ((i+1-c1.centroidX)*(i+1-c1.centroidX)+(j-c1.centroidY)*(j-c1.centroidY))
                             + ((r-c1.centroidR)*(r-c1.centroidR)
                             + (g-c1.centroidG)*(g-c1.centroidG)
                             + (b-c1.centroidB)*(b-c1.centroidB));
               dst[oOffs+1] = (d0 < d1) ? pixel0 : c1.centroidValue;
            }

            c1 = c0;
            c3 = c2;
         }
           
         nlOffs = offs;
         offs -= scaledSt;
      }

      return true;
   }


   // Quad-tree decomposition of the input image based on variance of each node
   // The decomposition stops when enough nodes have been computed.
   // The centroid of each cluster is initialized at the center of the rectangle
   // pointed to by the nodes in the tree. It should provide a good initial
   // value for the centroids and help converge faster.
   private void chooseCentroids(Cluster[] clusters, int[] buffer, int ww, int hh)
   {
      // Create quad tree decomposition of the image
      LinkedList<QuadTreeGenerator.Node> nodes = new LinkedList<QuadTreeGenerator.Node>();
      QuadTreeGenerator qtg = new QuadTreeGenerator(8, true);
      QuadTreeGenerator.Node node = QuadTreeGenerator.getNode(null, 0, 0, ww, hh, true);
      node.computeVariance(buffer, ww);
      nodes.add(node);  
      qtg.decomposeNodes(nodes, buffer, clusters.length, ww & -4);
      int n = clusters.length-1;

      while ((n >= 0) && (nodes.size() > 0))
      {
         QuadTreeGenerator.Node next = nodes.removeFirst();
         final Cluster c = clusters[n];
         c.centroidX = next.x + (next.w >> 1);
         c.centroidY = next.y + (next.h >> 1);
         final int centroidValue = buffer[(c.centroidY * ww) + c.centroidX];
         c.centroidR = (centroidValue >> 16) & 0xFF;
         c.centroidG = (centroidValue >>  8) & 0xFF;
         c.centroidB =  centroidValue & 0xFF;
         n--;
     }

     if (n >= 0)
     {
       // If needed, other centroids are set to random values
       Random rnd = new Random();

       while (n >= 0)
       {
          final Cluster c = clusters[n];
          c.centroidX = rnd.nextInt(ww);
          c.centroidY = rnd.nextInt(hh);
          final int centroidValue = buffer[(c.centroidY * ww) + c.centroidX];
          c.centroidR = (centroidValue >> 16) & 0xFF;
          c.centroidG = (centroidValue >>  8) & 0xFF;
          c.centroidB =  centroidValue & 0xFF;
          n--;
       }
     }
   }

   
   public int getCentroids(int[] coordinates)
   {
      if (coordinates == null)
         return -1;
      
      final int len = (coordinates.length < this.clusters.length) ? coordinates.length 
              : this.clusters.length;
      
      for (int i=0; i<len; i++)
         coordinates[i] = (this.clusters[i].centroidX << 16) | this.clusters[i].centroidY;
      
      return len;
   }


   public boolean getChooseCentroids()
   {
      return this.chooseCentroids;
   }


   public void setChooseCentroids(boolean choose)
   {
      this.chooseCentroids = choose;
   }


   public boolean showBorders()
   {
      return this.showBorders;
   }


   public void setShowBorders(boolean showBorders)
   {
      this.showBorders = showBorders;
   }



   private static class Cluster
   {
      int items;
      int centroidR;
      int centroidG;
      int centroidB;
      int centroidX;
      int centroidY;
      int centroidValue; // only used in final step
      int sumR;
      int sumG;
      int sumB;
      int sumX;
      int sumY;

      boolean computeCentroid()
      {
         if (this.items == 0)
            return false;
         
         final int r = (this.sumR / this.items);
         final int g = (this.sumG / this.items);
         final int b = (this.sumB / this.items);
         final int newCentroidX = (this.sumX / this.items);
         final int newCentroidY = (this.sumY / this.items);
         this.reset();

         if ((r != this.centroidR) || (g != this.centroidG)
                 || (b != this.centroidB) || (newCentroidX != this.centroidX)
                 || (newCentroidY != this.centroidY))
         {
           this.centroidR = r;
           this.centroidG = g;
           this.centroidB = b;
           this.centroidX = newCentroidX;
           this.centroidY = newCentroidY;
           return true;
        }

         return false;
      }
      
      void reset()
      {
         this.items = 0;
         this.sumR = 0;
         this.sumG = 0;
         this.sumB = 0;
         this.sumX = 0;
         this.sumY = 0;
      }
   }
}
