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

package kanzi.filter;

import java.util.LinkedList;
import java.util.Random;
import kanzi.IndexedIntArray;
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
    private boolean chooseCentroids;
    private int subSample;

    
    public ColorClusterFilter(int width, int height, int nbClusters)
    {
       this(width, height, width, nbClusters, 16, null, 0);
    }


    public ColorClusterFilter(int width, int height, int stride, int nbClusters, int iterations)
    {
       this(width, height, stride, nbClusters, iterations, null, 0);
    }


    // centroidXY is an optional array of packed (16 bits + 16 bits) centroid coordinates
    public ColorClusterFilter(int width, int height, int stride, int nbClusters, 
            int iterations, int[] centroidsXY, int downSampling)
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

      if ((nbClusters < 2) || (nbClusters > 256))
         throw new IllegalArgumentException("The number of clusters must be in [2..256]");

      if ((downSampling < 0) || (downSampling > 2))
         throw new IllegalArgumentException("The down sampling factor must be in [0..2]");

      if ((iterations < 2) || (iterations > 256))
         throw new IllegalArgumentException("The maximum number of iterations must be in [2..256]");

      if ((centroidsXY != null) && (centroidsXY.length < nbClusters))
         throw new IllegalArgumentException("The number of centroid coordinates "
                 + "is less than the number of clusters");

      this.width = width;
      this.height = height;
      this.stride = stride;
      this.maxIterations = iterations;
      this.chooseCentroids = (centroidsXY == null) ? true : false;
      this.clusters = new Cluster[nbClusters];
      this.subSample = downSampling + 1; // down scale by 2 at least
      this.buffer = new int[(width>>1)*(height>>1)]; // always 1 because of the automatic upsampling in apply()

      for (int i=0; i<nbClusters; i++)
      {
         this.clusters[i] = new Cluster();
         
         if (centroidsXY != null)
         {
            // The work image is downscaled by 2 at the beginning of the process
            // Rescale the coordinates
            this.clusters[i].centroidX = ((centroidsXY[i] >> 16) & 0x0000FFFF) >> this.subSample;
            this.clusters[i].centroidY =  (centroidsXY[i] & 0x0000FFFF) >> this.subSample;
         }
      }
    }


    // Use K-Means algorithm to create clusters of pixels with similar colors
    @Override
    public boolean apply(IndexedIntArray src, IndexedIntArray dst)
    {
       final int[] buf = this.buffer;
       int scale = this.subSample;
       final int adjust = (1 << scale) - 1;
       int scaledW = (this.width + adjust) >> scale;
       int scaledH = (this.height + adjust) >> scale;
       final Cluster[] cl = this.clusters;
       final int nbClusters = cl.length;
       final int rescaleThreshold = (this.maxIterations * 2 / 3);
       int iterations = 0;

       // Create a down sampled copy of the source 
       this.createWorkImage(src.array, src.index, scale);

       // Choose centers
       if (this.chooseCentroids == true)
          this.chooseCentroids(this.clusters, buf, scaledW, scaledH);

       // Main loop, associate points to clusters and re-calculate centroids
       while (iterations < this.maxIterations)
       {
         int offs = 0;
         int moves = 0;

         // Associate a pixel to the nearest cluster
         for (int j=0; j<scaledH; j++, offs+=scaledW)
         {
            int kfound = -1;

            for (int i=0; i<scaledW; i++)
            {
               final int pixel = buf[offs+i];
               final int r = (pixel >> 16) & 0xFF;
               final int g = (pixel >>  8) & 0xFF;
               final int b =  pixel & 0xFF;
               int nearest;

               if (kfound >= 0)
               {
                  // Reuse previous cluster as 'best initial guess' which yield
                  // a small value for 'nearest' most of the time
                  final Cluster cluster = cl[kfound];
                  final int dx = i - cluster.centroidX;
                  final int dy = j - cluster.centroidY;
                  final int dr = r - cluster.centroidR;
                  final int dg = g - cluster.centroidG;
                  final int db = b - cluster.centroidB;

                  // Distance is based on 3 color and 2 position coordinates
                  nearest = 2 * (dx*dx + dy*dy) + (dr*dr + dg*dg + db*db);
               }
               else
               {
                  nearest = Integer.MAX_VALUE;
               }

               // Iterate over clusters, calculating pixel distance to centroid
               for (int k=0; k<nbClusters; k++)
               {
                  final Cluster cluster = cl[k];
                  final int dx = i - cluster.centroidX;
                  final int dy = j - cluster.centroidY;

                  // Distance is based on 3 color and 2 position coordinates
                  int sq_dist = (dx*dx + dy*dy) << 1;

                  if (sq_dist >= nearest) // early exit
                     continue;

                  final int dr = r - cluster.centroidR;
                  final int dg = g - cluster.centroidG;
                  final int db = b - cluster.centroidB;

                  // Distance is based on 3 color and 2 position coordinates
                  sq_dist += (dr*dr + dg*dg + db*db);

                  if (sq_dist < nearest)
                  {
                     nearest = sq_dist;
                     kfound = k;
                  }
               }

               final Cluster cluster = cl[kfound];
               buf[offs+i] &= 0x00FFFFFF;
               buf[offs+i] |= ((kfound + 1) << 24); // update pixel's cluster index (top byte)
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
            final Cluster cluster = cl[j];

            if (cluster.items == 0)
               continue;

            if (cluster.computeCentroid() == true)
               moves++;
         }

         iterations++;

         if ((scale > 1) && ((iterations == rescaleThreshold) || (moves == 0)))
         {
            // Upscale by 2 in each dimension, now that centroids are somewhat stable
            scale >>= 1;
            scaledW <<= 1;
            scaledH <<= 1;
            this.createWorkImage(src.array, src.index, scale);

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

      this.subSample = scale;
      return this.createFinalImage(src, dst);
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
             buf[dstIdx++] = ((r << 16) | (g << 8) | b) & 0x00FFFFFF;
          }

          srcIdx += scaledStride;
       }

       return buf;
   }


   // Up-sample and set all points in the cluster to the color of the centroid pixel
   private boolean createFinalImage(IndexedIntArray source, IndexedIntArray destination)
   {
      final int[] buf = this.buffer;
      final int[] src = source.array;
      final int[] dst = destination.array;
      final int srcStart = source.index;
      final int dstStart = destination.index;
      final Cluster[] cl = this.clusters;
      final int scale = this.subSample; 
      final int scaledW = this.width >> scale;
      final int scaledY = this.height >> scale;
      final int st = this.stride;
      int offs = (scaledY - 1) * scaledW;
      int nlOffs = offs;
           
      for (int j=this.height-2; j>=0; j-=2)
      {
         Cluster c1 = cl[(buf[offs+scaledW-1]>>>24)-1]; // pixel p1 to the right of current p0
         Cluster c3 = cl[(buf[nlOffs+scaledW-1]>>>24)-1]; // pixel p3 to the right of p2
         final int srcIdx = srcStart + j * st;
         final int dstIdx = dstStart + j * st;

         for (int i=this.width-2; i>=0; i-=2)
         {
            int iOffs = srcIdx + i;
            int oOffs = dstIdx + i;
            final int cluster0Idx = (buf[offs+(i>>scale)] >>> 24) - 1;
            final Cluster c0 = cl[cluster0Idx];
            final int cluster2Idx = (buf[nlOffs+(i>>scale)] >>> 24) - 1;
            final Cluster c2 = cl[cluster2Idx]; // pixel p2 below current p0
            final int pixel0 = c0.centroidValue;
            dst[oOffs] = pixel0;

            if (c0 == c3)
            {
               // Inside cluster
               dst[oOffs+st+1] = pixel0;
            }
            else
            {
               // Diagonal cluster border
               final int pixel = src[iOffs+st+1];
               final int r = (pixel >> 16) & 0xFF;
               final int g = (pixel >>  8) & 0xFF;
               final int b =  pixel & 0xFF;
               final int d0 = ((r-c0.centroidR)*(r-c0.centroidR)
                             + (g-c0.centroidG)*(g-c0.centroidG)
                             + (b-c0.centroidB)*(b-c0.centroidB));
               final int d3 = ((r-c3.centroidR)*(r-c3.centroidR)
                             + (g-c3.centroidG)*(g-c3.centroidG)
                             + (b-c3.centroidB)*(b-c3.centroidB));
               dst[oOffs+st+1] = (d0 < d3) ? pixel0 : c3.centroidValue;
            }

            if (c0 == c2)
            {
               // Inside cluster
               dst[oOffs+st] = pixel0;
            }
            else
            {
               // Vertical cluster border
               final int pixel = src[iOffs+st];
               final int r = (pixel >> 16) & 0xFF;
               final int g = (pixel >>  8) & 0xFF;
               final int b =  pixel & 0xFF;
               final int d0 = ((r-c0.centroidR)*(r-c0.centroidR)
                             + (g-c0.centroidG)*(g-c0.centroidG)
                             + (b-c0.centroidB)*(b-c0.centroidB));
               final int d2 = ((r-c2.centroidR)*(r-c2.centroidR)
                             + (g-c2.centroidG)*(g-c2.centroidG)
                             + (b-c2.centroidB)*(b-c2.centroidB));
               dst[oOffs+st] = (d0 < d2) ? pixel0 : c2.centroidValue;
            }

            if (c0 == c1)
            {
               // Inside cluster
               dst[oOffs+1] = pixel0;
            }
            else
            {
               // Horizontal cluster border
               final int pixel = src[iOffs+1];
               final int r = (pixel >> 16) & 0xFF;
               final int g = (pixel >>  8) & 0xFF;
               final int b =  pixel & 0xFF;
               final int d0 = ((r-c0.centroidR)*(r-c0.centroidR)
                             + (g-c0.centroidG)*(g-c0.centroidG)
                             + (b-c0.centroidB)*(b-c0.centroidB));
               final int d1 = ((r-c1.centroidR)*(r-c1.centroidR)
                             + (g-c1.centroidG)*(g-c1.centroidG)
                             + (b-c1.centroidB)*(b-c1.centroidB));
               dst[oOffs+1] = (d0 < d1) ? pixel0 : c1.centroidValue;
            }

            c1 = c0;
            c3 = c2;
         }
           
         nlOffs = offs;
         offs -= scaledW;
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
      final LinkedList<QuadTreeGenerator.Node> nodes = new LinkedList<QuadTreeGenerator.Node>();
      final QuadTreeGenerator qtg = new QuadTreeGenerator(ww & -3, hh & -3, 8);
      qtg.decomposeNodes(nodes, buffer, 0, clusters.length);
      int n = clusters.length-1;

      while ((n >= 0) && (nodes.size() > 0))
      {
         final QuadTreeGenerator.Node next = nodes.removeFirst();
         final Cluster c = clusters[n];
         c.centroidX = next.x + (next.w >> 1);
         c.centroidY = next.y + (next.h >> 1);
         final int centroidValue = buffer[(c.centroidY * ww) + c.centroidX];
         c.centroidR = (centroidValue >> 16) & 0xFF;
         c.centroidG = (centroidValue >>  8) & 0xFF;
         c.centroidB =  centroidValue & 0xFF;
         n--;
     }

     if (n > 0)
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



   private static class Cluster
   {
      int items;
      int centroidR;
      int centroidG;
      int centroidB;
      int centroidX;
      int centroidY;
      int centroidValue;
      int sumR;
      int sumG;
      int sumB;
      int sumX;
      int sumY;

      // Requires items != 0
      boolean computeCentroid()
      {
         final int r = (this.sumR / this.items);
         final int g = (this.sumG / this.items);
         final int b = (this.sumB / this.items);
         final int newCentroidX = (this.sumX / this.items);
         final int newCentroidY = (this.sumY / this.items);
         this.items = 0;
         this.sumR = 0;
         this.sumG = 0;
         this.sumB = 0;
         this.sumX = 0;
         this.sumY = 0;

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
   }
}
