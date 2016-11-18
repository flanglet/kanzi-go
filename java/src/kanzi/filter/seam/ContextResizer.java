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

package kanzi.filter.seam;


import java.util.concurrent.ExecutorService;
import kanzi.SliceIntArray;
import kanzi.IntSorter;
import kanzi.IntFilter;
import kanzi.filter.ParallelFilter;
import kanzi.filter.SobelFilter;
import kanzi.util.sort.BucketSort;


// Based on algorithm by Shai Avidan, Ariel Shamir
// Described in [Seam Carving for Content-Aware Image Resizing]
//
// This implementation is focused on speed and is indeed very fast (but it does
// only calculate an approximation of the energy minimizing paths because finding
// the absolute best paths takes too much time)
// It is also possible to calculate the seams on a subset of the image which is
// useful to iterate over the same (shrinking) image.
//
// Note: the name seam carving is a bit unfortunate, what the algo achieves
// is detection and removal of the paths of least resistance (energy wise) in
// the image. These paths really are geodesics.
public class ContextResizer implements IntFilter
{
    // Directions
    public static final int HORIZONTAL = 1;
    public static final int VERTICAL = 2;

    // Actions
    public static final int SHRINK = 1;
    public static final int EXPAND = 2;

    private static final int USED_MASK = 0x80000000;
    private static final int VALUE_MASK = 0x7FFFFFFF;
    private static final int DEFAULT_BEST_COST = 0x0FFFFFFF;
    private static final int DEFAULT_MAX_COST_PER_PIXEL = 256;
    private static final int RED_COLOR = 0xFFFF0000;
    private static final int BLUE_COLOR = 0xFF0000FF;

    private int width;
    private int height;
    private final int stride;
    private final int direction;
    private final int maxSearches;
    private final int maxAvgGeoPixCost;
    private final int[] costs;
    private int scalingFactor;
    private boolean debug;
    private final IntSorter sorter;
    private SliceIntArray buffer;
    private final boolean fastMode;
    private final ExecutorService pool;


    public ContextResizer(int width, int height, int direction)
    {
        this(width, height, width, direction, 1, false, false, null);
    }


    public ContextResizer(int width, int height, int stride, int direction,
            int scalingFactor)
    {
        this(width, height, stride, direction, scalingFactor, false, false, null);
    }


    // width, height, offset and stride allow to apply the filter on a subset of an image
    // For packed RGB images, use 3 channels mode for more accurate results (fastMode=false)
    // and one channel mode (fastMode=true) for faster results.
    // For unpacked images, use one channel mode (fastMode=true).
    public ContextResizer(int width, int height, int stride,
            int direction, int scalingFactor,
            boolean fastMode, boolean debug, ExecutorService pool)
    {
        this(width, height, stride, direction, scalingFactor,
                Math.max(width, height), fastMode, debug, pool, DEFAULT_MAX_COST_PER_PIXEL);
    }


    // width, height, offset and stride allow to apply the filter on a subset of an image
    // maxAvgGeoPixCost allows to limit the cost of geodesics: only those with an
    // average cost per pixel less than maxAvgGeoPixCost are allowed (it may be
    // less than nbGeodesics).
    // For packed RGB images, use 3 channels mode for more accurate results (fastMode=false)
    // and one channel mode (fastMode=true) for faster results.
    // For unpacked images, use one channel mode (fastMode=true).
    // Scaling factor (unit is 0.1% or per mil), negative for shrink and positive for expand.
    public ContextResizer(int width, int height, int stride, int direction,
            int scalingFactor, int maxSearches, boolean fastMode,
            boolean debug, ExecutorService pool, int maxAvgGeoPixCost)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");

        if (height > 4096)
            throw new IllegalArgumentException("The height must be at most 4096");

        if (width > 4096)
            throw new IllegalArgumentException("The width must be at most 4096");

        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

        if (maxAvgGeoPixCost < 1)
            throw new IllegalArgumentException("The max average pixel cost in a geodesic must be at least 1");

        if ((scalingFactor <= -1000) || (scalingFactor == 0) || (scalingFactor >= 1000))
            throw new IllegalArgumentException("The scaling factor unit is 0.1% (valid range "
                    + "]-1000..0[ or ]0..1000[) : "+scalingFactor);

        if (((direction & HORIZONTAL) == 0) && ((direction & VERTICAL) == 0))
            throw new IllegalArgumentException("Invalid direction parameter (must be VERTICAL or HORIZONTAL)");

        int absScalingFactor = Math.abs(scalingFactor);

        if ((direction & VERTICAL) != 0)
        {
           if ((absScalingFactor * width / 1000) == 0)
              throw new IllegalArgumentException("The number of vertical geodesics is 0, increase scaling factor");

           if ((absScalingFactor * width / 1000) == width)
              throw new IllegalArgumentException("The number of vertical geodesics is " +
                      width + ", decrease scaling factor");
        }

        if ((direction & HORIZONTAL) != 0)
        {
           if ((absScalingFactor * height / 1000) == 0)
              throw new IllegalArgumentException("The number of horizontal geodesics is 0, increase scaling factor");

           if ((absScalingFactor * height / 1000) == height)
              throw new IllegalArgumentException("The number of horizontal geodesics is " +
                      height + ", decrease scaling factor");
        }

        this.height = height;
        this.width = width;
        this.stride = stride;
        this.direction = direction;
        this.maxSearches = maxSearches;
        this.costs = new int[stride*height];
        this.scalingFactor = scalingFactor;
        this.maxAvgGeoPixCost = maxAvgGeoPixCost;
        this.buffer = new SliceIntArray();
        this.fastMode = fastMode;
        this.debug = debug;
        this.pool = pool;
        int dim = (height >= width) ? height : width;
        int log = 3;

        while (1<<log < dim)
           log++;

        // Used to sort coordinates of geodesics
        this.sorter = new BucketSort(log);
    }


    public int getWidth()
    {
        return this.width;
    }


    public int getHeight()
    {
        return this.height;
    }


    public boolean getDebug()
    {
        return this.debug;
    }


    // Not thread safe
    public boolean setDebug(boolean debug)
    {
        this.debug = debug;
        return true;
    }


    public int getAction()
    {
        return (this.scalingFactor > 0) ? EXPAND : SHRINK;
    }


    // Not thread safe
    public boolean setAction(int action)
    {
        if ((action != SHRINK) && (action != EXPAND))
            return false;

        this.scalingFactor = (action == SHRINK) ? -Math.abs(this.scalingFactor)
                : Math.abs(this.scalingFactor);
        return true;
    }


    public boolean shrink(SliceIntArray src, SliceIntArray dst)
    {
        this.setAction(SHRINK);
        return this.shrink_(src, dst);
    }


    public boolean expand(SliceIntArray src, SliceIntArray dst)
    {
        this.setAction(EXPAND);
        return this.expand_(src, dst);
    }


    // Modifies the width and/or height attributes
    // The src image is modified if both directions are selected
    @Override
    public boolean apply(SliceIntArray src, SliceIntArray dst)
    {
       return (this.scalingFactor < 0) ? this.shrink_(src, dst) :
               this.expand_(src, dst);
    }


    // Increases the width and/or height attributes. Result must fit in width*height
    private boolean expand_(SliceIntArray src, SliceIntArray dst)
    {
        int processed = 0;
        SliceIntArray input = src;
        SliceIntArray output = dst;

        if ((this.direction & VERTICAL) != 0)
        {
            if ((this.direction & HORIZONTAL) != 0)
            {
               // Lazy dynamic memory allocation
               if (this.buffer.array.length < this.stride * this.height)
               {
                  this.buffer.length = this.stride * this.height;

                  if (this.buffer.array.length < this.buffer.length)
                     this.buffer.array = new int[this.buffer.length];
               }

               output = this.buffer;
            }

            Geodesic[] geodesics = this.computeGeodesics(input, VERTICAL);

            if (geodesics.length > 0)
            {
                processed += geodesics.length;
                this.addGeodesics(geodesics, input, output, VERTICAL);
            }

            if ((this.direction & HORIZONTAL) != 0)
            {
               input = this.buffer;
               output = dst;
            }
        }

        if ((this.direction & HORIZONTAL) != 0)
        {
            Geodesic[] geodesics = this.computeGeodesics(input, HORIZONTAL);

            if (geodesics.length > 0)
            {
                processed += geodesics.length;
                this.addGeodesics(geodesics, input, output, HORIZONTAL);
            }
        }

        if ((processed == 0) && (src.array != dst.array))
        {
           System.arraycopy(src.array, src.index, dst.array, dst.index, this.height*this.stride);
        }

        return true;
    }


    // Decreases the width and/or height attributes
    private boolean shrink_(SliceIntArray src, SliceIntArray dst)
    {
        int processed = 0;
        SliceIntArray input = src;
        SliceIntArray output = dst;

        if ((this.direction & VERTICAL) != 0)
        {
            if ((this.direction & HORIZONTAL) != 0)
            {
               // Lazy dynamic memory allocation
               if (this.buffer.array.length < this.stride * this.height)
               {
                  this.buffer.length = this.stride * this.height;

                  if (this.buffer.array.length < this.buffer.length)
                     this.buffer.array = new int[this.buffer.length];
               }

               output = this.buffer;
            }

            Geodesic[] geodesics = this.computeGeodesics(input, VERTICAL);

            if (geodesics.length > 0)
            {
               processed += geodesics.length;
               this.removeGeodesics(geodesics, input, output, VERTICAL);
            }

            if ((this.direction & HORIZONTAL) != 0)
            {
               input = this.buffer;
               output = dst;
            }
        }

        if ((this.direction & HORIZONTAL) != 0)
        {
            Geodesic[] geodesics = this.computeGeodesics(input, HORIZONTAL);

            if (geodesics.length > 0)
            {
               processed += geodesics.length;
               this.removeGeodesics(geodesics, input, output, HORIZONTAL);
            }
        }

        if ((processed == 0) && (src.array != dst.array))
        {
           System.arraycopy(src.array, src.index, dst.array, dst.index, this.height*this.stride);
        }

        return true;
    }


    // dir must be either VERTICAL or HORIZONTAL
    public boolean addGeodesics(Geodesic[] geodesics, SliceIntArray source,
            SliceIntArray destination, int dir)
    {
        if (((dir & VERTICAL) == 0) && ((dir & HORIZONTAL) == 0))
           return false;

        if (((dir & VERTICAL) != 0) && ((dir & HORIZONTAL) != 0))
           return false;

        if (geodesics.length == 0)
           return true ;

        final int[] src = source.array;
        final int[] dst = destination.array;
        int srcStart = source.index;
        int dstStart = destination.index;
        final int[] linePositions = new int[geodesics.length];
        final int endj;
        final int endi;
        final int incStart;
        final int incIdx;
        final int color;

        if (dir == HORIZONTAL)
        {
            endj = this.width;
            endi = this.height;
            incStart = 1;
            incIdx = this.stride;
            color = BLUE_COLOR;
        }
        else
        {
            endj = this.height;
            endi = this.width;
            incStart = this.stride;
            incIdx = 1;
            color = RED_COLOR;
        }

        for (int j=endj-1; j>=0; j--)
        {
            final int endPosIdx = linePositions.length;

            // Find all the pixels belonging to geodesics in this line
            for (int k=0; k<endPosIdx; k++)
                linePositions[k] = geodesics[k].positions[j];

            // Sort the pixels by increasing position
            if (endPosIdx > 1)
                this.sorter.sort(linePositions, 0, endPosIdx);

            int posIdx = 0;
            int srcIdx = srcStart;
            int dstIdx = dstStart;
            int pos = 0;

            while (posIdx < endPosIdx)
            {
                final int newPos = linePositions[posIdx];

                if (newPos > pos)
                {
                   final int len = newPos - pos;

                   if ((dir == VERTICAL) && (len >= 32))
                   {
                       // Speed up copy
                       System.arraycopy(src, srcIdx, dst, dstIdx, len);
                       srcIdx += len;
                       dstIdx += len;
                    }
                    else
                    {
                       copy(src, srcIdx, dst, dstIdx, len, incIdx);
                       srcIdx += (len * incIdx);
                       dstIdx += (len * incIdx);
                    }

                    pos = newPos;
                }

                // Insert new pixel into the destination
                if (this.debug == true)
                {
                   dst[dstIdx] = color;
                }
                else
                {
                   dst[dstIdx] = src[srcIdx];
                }

                pos++;
                dstIdx += incIdx;
                posIdx++;
            }

            final int len = endi - pos;

            // Finish the line, no more test for geodesic pixels required
            if ((dir == VERTICAL) && (len >= 32))
            {
               // Speed up copy
               System.arraycopy(src, srcIdx, dst, dstIdx, len);
               srcIdx += len;
               dstIdx += len;
            }
            else
            {
               // Either incIdx != 1 or not enough pixels for arraycopy to be worth it
               copy(src, srcIdx, dst, dstIdx, len, incIdx);
               srcIdx += (len * incIdx);
               dstIdx += (len * incIdx);
            }

            srcStart += incStart;
            dstStart += incStart;
        }

        return true;
    }


    // dir must be either VERTICAL or HORIZONTAL
    public boolean removeGeodesics(Geodesic[] geodesics, SliceIntArray source,
            SliceIntArray destination, int dir)
    {
        if (((dir & VERTICAL) == 0) && ((dir & HORIZONTAL) == 0))
           return false;

        if (((dir & VERTICAL) != 0) && ((dir & HORIZONTAL) != 0))
           return false;

        if (geodesics.length == 0)
           return true ;

        final int[] src = source.array;
        final int[] dst = destination.array;
        int srcStart = source.index;
        int dstStart = destination.index;

        final int[] linePositions = new int[geodesics.length];
        final int endj;
        final int endLine;
        final int incIdx;
        final int incStart;
        final int color;

        if (dir == HORIZONTAL)
        {
            endj = this.width;
            endLine = this.height;
            incIdx = this.stride;
            incStart = 1;
            color = BLUE_COLOR;
        }
        else
        {
            endj = this.height;
            endLine = this.width;
            incIdx = 1;
            incStart = this.stride;
            color = RED_COLOR;
        }

        for (int j=0; j<endj; j++)
        {
            final int endPosIdx = linePositions.length;

            // Find all the pixels belonging to geodesics in this line
            for (int k=0; k<endPosIdx; k++)
                linePositions[k] = geodesics[k].positions[j];

            // Sort the pixels by increasing position
            if (endPosIdx > 1)
                this.sorter.sort(linePositions, 0, endPosIdx);

            int srcIdx = srcStart;
            int dstIdx = dstStart;
            int posIdx = 0;
            int pos = 0;

            while (posIdx < endPosIdx)
            {
                final int newPos = linePositions[posIdx];

                // Copy pixels not belonging to a geodesic
                if (newPos > pos)
                {
                    final int len = newPos - pos;

                    if ((dir == VERTICAL) && (len >= 32))
                    {
                       // Speed up copy
                       System.arraycopy(src, srcIdx, dst, dstIdx, len);
                       srcIdx += len;
                       dstIdx += len;
                    }
                    else
                    {
                       // Either incIdx != 1 or not enough pixels for arraycopy to be worth it
                       copy(src, srcIdx, dst, dstIdx, len, incIdx);
                       srcIdx += (len * incIdx);
                       dstIdx += (len * incIdx);
                    }

                    pos = newPos;
                }

                // Mark or remove pixel belonging to a geodesic
                if (this.debug == true)
                {
                    dst[dstIdx] = color;
                    dstIdx += incIdx;
                }

                pos++;
                srcIdx += incIdx;
                posIdx++;
            }

            final int len = endLine - pos;

            // Finish the line, no more test for geodesic pixels required
            if ((dir == VERTICAL) && (len >= 32))
            {
               // Speed up copy
               System.arraycopy(src, srcIdx, dst, dstIdx, len);
               srcIdx += len;
               dstIdx += len;
            }
            else
            {
               // Either incIdx != 1 or not enough pixels for arraycopy to be worth it
               copy(src, srcIdx, dst, dstIdx, len, incIdx);
               srcIdx += (len * incIdx);
               dstIdx += (len * incIdx);
            }

            srcStart += incStart;
            dstStart += incStart;
        }

        return true;
    }


    // dir must be either VERTICAL or HORIZONTAL
    public Geodesic[] computeGeodesics(SliceIntArray source, int dir)
    {
        if (((dir & VERTICAL) == 0) && ((dir & HORIZONTAL) == 0))
           return new Geodesic[0];

        if (((dir & VERTICAL) != 0) && ((dir & HORIZONTAL) != 0))
           return new Geodesic[0];

        final int dim = (dir == HORIZONTAL) ? this.height : this.width;
        final int searches = Math.min(dim, this.maxSearches);
        int[] firstPositions = new int[searches];
        int n = 0;

        // Spread the first position along 'direction' for better uniformity
        // Should improve speed by detecting faster low cost paths and reduce
        // geodesic crossing management.
        // It will improve quality by spreading the search over the whole image
        // if maxSearches is small.
        for (int i=0; ((n<searches) && (i<24)); i+=3)
        {
            // i & 7 shuffles the start position : 0, 3, 6, 1, 4, 7, 2, 5
            for (int j=(i & 7); ((n<searches) && (j<dim)); j+=8)
                firstPositions[n++] = j;
        }

        return this.computeGeodesics_(source, dir, firstPositions, searches);
    }


    // Compute the geodesics but give a constraint on where to start from
    // All first position values must be different
    // dir must be either VERTICAL or HORIZONTAL
    public Geodesic[] computeGeodesics(SliceIntArray source, int dir, int[] firstPositions)
    {
        if (((dir & VERTICAL) == 0) && ((dir & HORIZONTAL) == 0))
           return new Geodesic[0];

        if (((dir & VERTICAL) != 0) && ((dir & HORIZONTAL) != 0))
           return new Geodesic[0];

        final int dim = (dir == HORIZONTAL) ? this.height : this.width;
        final int searches = Math.min(dim, this.maxSearches);
        return this.computeGeodesics_(source, dir, firstPositions, searches);
    }


    private Geodesic[] computeGeodesics_(SliceIntArray source, int dir, int[] firstPositions, int searches)
    {
        if ((searches == 0) || (source == null) || (source.array == null) || (firstPositions == null))
            return new Geodesic[0];

        // Limit searches if there are not enough starting positions
        if (searches > firstPositions.length)
            searches = firstPositions.length;

        final int geoLength;
        final int inc;
        final int incLine;
        final int dim;

        if (dir == HORIZONTAL)
        {
            geoLength = this.width;
            dim = this.height;
            inc = this.stride;
            incLine = 1;
        }
        else
        {
            geoLength = this.height;
            dim = this.width;
            inc = 1;
            incLine = this.stride;
        }

        // Calculate cost at each pixel
        this.calculateCosts(source, this.costs);
        final int nbGeodesics = dim * Math.abs(this.scalingFactor) / 1000;
        final int maxGeo = (nbGeodesics > searches) ? searches : nbGeodesics;

        // Queue of geodesics sorted by cost
        // The queue size could be less than firstPositions.length
        final GeodesicSortedQueue queue = new GeodesicSortedQueue(maxGeo);
        Geodesic geodesic = null;
        Geodesic last = null; // last in queue
        int maxCost = geoLength * this.maxAvgGeoPixCost;
        final int[] costs_ = this.costs; // aliasing

        // Calculate path and cost for each geodesic
        for (int i=0; i<searches; i++)
        {
            if (geodesic == null)
                geodesic = new Geodesic(dir, geoLength);

            int bestLinePos = firstPositions[i];
            int costIdx = inc * bestLinePos;
            geodesic.positions[0] = bestLinePos;
            geodesic.cost = costs_[costIdx];

            // Process each row/column
            for (int pos=1; pos<geoLength; pos++)
            {
                costIdx += incLine;
                final int startCostIdx = costIdx;
                int startBestLinePos = bestLinePos;
                int bestCost = ((costs_[startCostIdx] & USED_MASK) == 0) ? costs_[startCostIdx]
                        : DEFAULT_BEST_COST;

                if (bestCost > 0)
                {
                    // Check left/upper pixel, skip already used pixels
                    int idx = startCostIdx - inc;

                    for (int linePos=startBestLinePos-1; linePos>=0; idx-=inc, linePos--)
                    {
                        final int cost = costs_[idx];

                        // Skip pixels in use
                        if ((cost & USED_MASK) != 0)
                           continue;

                        if (cost < bestCost)
                        {
                            bestCost = cost;
                            bestLinePos = linePos;
                            costIdx = idx;
                        }

                        break;
                    }
                }

                if (bestCost > 0)
                {
                    // Check right/lower pixel, skip already used pixels
                    int idx = startCostIdx + inc;

                    for (int linePos=startBestLinePos+1; linePos<dim; idx+=inc, linePos++)
                    {
                        final int cost = costs_[idx];

                        if ((cost & USED_MASK) != 0)
                           continue;

                         if (cost < bestCost)
                         {
                             bestCost = cost;
                             bestLinePos = linePos;
                             costIdx = idx;
                         }

                         break;
                    }

                    geodesic.cost += bestCost;

                    // Skip, this path is already too expensive
                    if (geodesic.cost >= maxCost)
                       break;
                }

                geodesic.positions[pos] = bestLinePos;
            }

            if (geodesic.cost < maxCost)
            {
                 // Add geodesic (in increasing cost order).
                 Geodesic newLast = queue.add(geodesic);

                 // Prevent geodesics from sharing pixels by marking the used pixels
                 // Only the pixels of the geodesics in the queue are marked as used
                 if (nbGeodesics > 1)
                 {
                     // If the previous last element has been expelled from the queue,
                     // the corresponding pixels can be reused by other geodesics
                     final int geoLength4 = geoLength & -4;
                     int startLine = 0;
                     final int[] gp = geodesic.positions;

                     if (last != null)
                     {
                        final int[] lp = last.positions;

                        // Tag old pixels as 'free' and new pixels as 'used'
                        for (int k=0; k<geoLength4; k+=4)
                        {
                            costs_[startLine+(inc*gp[k])]   |= USED_MASK;
                            costs_[startLine+(inc*lp[k])]   &= VALUE_MASK;
                            startLine += incLine;
                            costs_[startLine+(inc*gp[k+1])] |= USED_MASK;
                            costs_[startLine+(inc*lp[k+1])] &= VALUE_MASK;
                            startLine += incLine;
                            costs_[startLine+(inc*gp[k+2])] |= USED_MASK;
                            costs_[startLine+(inc*lp[k+2])] &= VALUE_MASK;
                            startLine += incLine;
                            costs_[startLine+(inc*gp[k+3])] |= USED_MASK;
                            costs_[startLine+(inc*lp[k+3])] &= VALUE_MASK;
                            startLine += incLine;
                        }

                        for (int k=geoLength4; k<geoLength; k++)
                        {
                            costs_[startLine+(inc*gp[k])] |= USED_MASK;
                            costs_[startLine+(inc*lp[k])] &= VALUE_MASK;
                            startLine += incLine;
                        }
                     }
                     else
                     {
                        for (int k=0; k<geoLength4; k+=4)
                        {
                            costs_[startLine+(inc*gp[k])]   |= USED_MASK;
                            startLine += incLine;
                            costs_[startLine+(inc*gp[k+1])] |= USED_MASK;
                            startLine += incLine;
                            costs_[startLine+(inc*gp[k+2])] |= USED_MASK;
                            startLine += incLine;
                            costs_[startLine+(inc*gp[k+3])] |= USED_MASK;
                            startLine += incLine;
                        }

                        for (int k=geoLength4; k<geoLength; k++)
                        {
                            costs_[startLine+(inc*gp[k])] |= USED_MASK;
                            startLine += incLine;
                        }
                     }
                 }

                 // Be green, recycle
                 geodesic = last;

                 // Update maxCost
                 if (queue.isFull())
                 {
                    last = newLast;
                    maxCost = newLast.cost;
                 }
            }

            // All requested geodesics have been found with a cost of 0 => done !
            if ((maxCost == 0) && (queue.isFull() == true))
                break;
        }

        return queue.toArray(new Geodesic[queue.size()]);
    }


    private static void copy(int[] src, int srcIdx, int[] dst, int dstIdx, int len, int inc1)
    {
        final int len4 = len & -4;
        final int inc2 = inc1 + inc1;
        final int inc3 = inc2 + inc1;
        final int inc4 = inc3 + inc1;

        for (int i=0; i<len4; i+=4)
        {
           dst[dstIdx]      = src[srcIdx];
           dst[dstIdx+inc1] = src[srcIdx+inc1];
           dst[dstIdx+inc2] = src[srcIdx+inc2];
           dst[dstIdx+inc3] = src[srcIdx+inc3];
           dstIdx += inc4;
           srcIdx += inc4;
        }

        for (int i=len4; i<len; i++)
        {
           dst[dstIdx] = src[srcIdx];
           dstIdx += inc1;
           srcIdx += inc1;
        }
    }


    private int[] calculateCosts(SliceIntArray source, int[] costs_)
    {
        // For packed RGB images, use 3 channels mode for more accurate results and
        // one channel mode (green) for faster results.
        // For unpacked images, use one channel mode (Y for YUV or any for RGB).
        int sobelMode = (this.fastMode == true) ? SobelFilter.G_CHANNEL : SobelFilter.THREE_CHANNELS;

        if (this.pool != null)
        {
           // Use 4 threads
           SobelFilter[] gradientFilters = new SobelFilter[4];

           for (int i=0; i<gradientFilters.length; i++)
           {
              // Do not process the boundaries and overlap the filters (dim + 2)
              // to avoid boundary artefacts.
              gradientFilters[i] = new SobelFilter(this.width+2, this.height/gradientFilters.length+2,
                   this.stride, SobelFilter.HORIZONTAL | SobelFilter.VERTICAL,
                   sobelMode, SobelFilter.COST, false);
           }

           // Apply the parallel filter
           IntFilter pf = new ParallelFilter(this.width, this.height, this.stride,
                   this.pool, gradientFilters, ParallelFilter.HORIZONTAL);
           pf.apply(source, new SliceIntArray(costs_, 0));

           // Fix missing first and last rows of costs (due to non boundary processing filters)
           System.arraycopy(costs_, this.width, costs_, 0, this.width);
           System.arraycopy(costs_, (this.height-2)*this.stride,
                            costs_, (this.height-1)*this.stride, this.width);

           // Fix missing first column of costs
           final int area = this.stride * this.height;

           for (int j=0; j<area; j+=this.stride)
              costs_[j] = costs_[j+1];
        }
        else
        {
           // Mono threaded
           SobelFilter gradientFilter = new SobelFilter(this.width, this.height,
                            this.stride, SobelFilter.HORIZONTAL | SobelFilter.VERTICAL,
                            sobelMode, SobelFilter.COST, true);

           gradientFilter.apply(source, new SliceIntArray(costs_, 0));
        }

        // Add a quadratic contribution to the cost to favor straight lines
        // if costs of neighbors are all low
        for (int i=0; i<costs_.length; i++)
        {
           final int c = costs_[i];
           costs_[i] = (c < 5) ? 0 :  c + ((c * c) >> 8);
        }

        return costs_;
    }

}
