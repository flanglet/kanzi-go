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

import kanzi.SliceIntArray;
import kanzi.IntFilter;


// Implementation of a bilateral filter using integer gaussian filters.
// It is basically a convolution filter that smoothes an image but preserves
// the edges. It is based on convolution of a gaussian filter based on pixel
// position and a gaussian filter based on pixel intensity.
// This implementation is decently fast but does not scale well when the filter
// radius increases: O(n^2) because the filter is non-linear.
// A fast implementation is described in [Constant Time O(1) Bilateral Filtering]
// by Fatih Porikli at www.merl.com. However it requires to create several integral 
// histogram pictures (big memory consumption). The gaussian filter for distance
// is approximated by a linear combination of (power) images and the convolution
// is calculated using the histogram pictures.
public final class BilateralFilter implements IntFilter
{ 
    private final int width;
    private final int height;
    private final int stride;
    private final int radius;
    private final int[] kernel;
    private final int[] intensities;
    
    
    // Table with 1024 values: 2048 * exp(-x) with x in [0, 8[ (step = 8/1024)
    private static final int[] EXP_MINUS_X_2048 =
    {
      2048, 2032, 2016, 2000, 1984, 1969, 1954, 1939, 1923, 1908, 1894, 1879, 1864, 1850, 1835, 1821,
      1807, 1793, 1779, 1765, 1751, 1738, 1724, 1711, 1697, 1684, 1671, 1658, 1645, 1632, 1620, 1607,
      1594, 1582, 1570, 1558, 1545, 1533, 1521, 1510, 1498, 1486, 1475, 1463, 1452, 1440, 1429, 1418,
      1407, 1396, 1385, 1374, 1364, 1353, 1343, 1332, 1322, 1311, 1301, 1291, 1281, 1271, 1261, 1251,
      1242, 1232, 1222, 1213, 1203, 1194, 1185, 1176, 1166, 1157, 1148, 1139, 1131, 1122, 1113, 1104,
      1096, 1087, 1079, 1070, 1062, 1054, 1046, 1037, 1029, 1021, 1013, 1005,  998,  990,  982,  974,
       967,  959,  952,  944,  937,  930,  923,  915,  908,  901,  894,  887,  880,  873,  867,  860,
       853,  847,  840,  833,  827,  821,  814,  808,  802,  795,  789,  783,  777,  771,  765,  759,
       753,  747,  741,  735,  730,  724,  718,  713,  707,  702,  696,  691,  685,  680,  675,  670,
       664,  659,  654,  649,  644,  639,  634,  629,  624,  619,  614,  610,  605,  600,  596,  591,
       586,  582,  577,  573,  568,  564,  559,  555,  551,  546,  542,  538,  534,  530,  525,  521,
       517,  513,  509,  505,  501,  497,  494,  490,  486,  482,  478,  475,  471,  467,  464,  460,
       456,  453,  449,  446,  442,  439,  436,  432,  429,  425,  422,  419,  416,  412,  409,  406,
       403,  400,  397,  393,  390,  387,  384,  381,  378,  375,  372,  370,  367,  364,  361,  358,
       355,  353,  350,  347,  344,  342,  339,  336,  334,  331,  329,  326,  324,  321,  319,  316,
       314,  311,  309,  306,  304,  302,  299,  297,  295,  292,  290,  288,  285,  283,  281,  279,
       277,  275,  272,  270,  268,  266,  264,  262,  260,  258,  256,  254,  252,  250,  248,  246,
       244,  242,  240,  238,  237,  235,  233,  231,  229,  227,  226,  224,  222,  220,  219,  217,
       215,  214,  212,  210,  209,  207,  205,  204,  202,  201,  199,  198,  196,  195,  193,  191,
       190,  189,  187,  186,  184,  183,  181,  180,  178,  177,  176,  174,  173,  172,  170,  169,
       168,  166,  165,  164,  162,  161,  160,  159,  157,  156,  155,  154,  153,  151,  150,  149,
       148,  147,  146,  144,  143,  142,  141,  140,  139,  138,  137,  136,  135,  134,  132,  131,
       130,  129,  128,  127,  126,  125,  124,  123,  122,  122,  121,  120,  119,  118,  117,  116,
       115,  114,  113,  112,  111,  111,  110,  109,  108,  107,  106,  106,  105,  104,  103,  102,
       101,  101,  100,   99,   98,   98,   97,   96,   95,   95,   94,   93,   92,   92,   91,   90,
        89,   89,   88,   87,   87,   86,   85,   85,   84,   83,   83,   82,   81,   81,   80,   80,
        79,   78,   78,   77,   76,   76,   75,   75,   74,   74,   73,   72,   72,   71,   71,   70,
        70,   69,   68,   68,   67,   67,   66,   66,   65,   65,   64,   64,   63,   63,   62,   62,
        61,   61,   60,   60,   59,   59,   59,   58,   58,   57,   57,   56,   56,   55,   55,   55,
        54,   54,   53,   53,   52,   52,   52,   51,   51,   50,   50,   50,   49,   49,   48,   48,
        48,   47,   47,   47,   46,   46,   45,   45,   45,   44,   44,   44,   43,   43,   43,   42,
        42,   42,   41,   41,   41,   40,   40,   40,   39,   39,   39,   39,   38,   38,   38,   37,
        37,   37,   36,   36,   36,   36,   35,   35,   35,   34,   34,   34,   34,   33,   33,   33,
        33,   32,   32,   32,   32,   31,   31,   31,   31,   30,   30,   30,   30,   29,   29,   29,
        29,   28,   28,   28,   28,   28,   27,   27,   27,   27,   27,   26,   26,   26,   26,   25,
        25,   25,   25,   25,   24,   24,   24,   24,   24,   24,   23,   23,   23,   23,   23,   22,
        22,   22,   22,   22,   22,   21,   21,   21,   21,   21,   21,   20,   20,   20,   20,   20,
        20,   19,   19,   19,   19,   19,   19,   19,   18,   18,   18,   18,   18,   18,   17,   17,
        17,   17,   17,   17,   17,   17,   16,   16,   16,   16,   16,   16,   16,   16,   15,   15,
        15,   15,   15,   15,   15,   15,   14,   14,   14,   14,   14,   14,   14,   14,   14,   13,
        13,   13,   13,   13,   13,   13,   13,   13,   12,   12,   12,   12,   12,   12,   12,   12,
        12,   12,   11,   11,   11,   11,   11,   11,   11,   11,   11,   11,   11,   11,   10,   10,
        10,   10,   10,   10,   10,   10,   10,   10,   10,   10,    9,    9,    9,    9,    9,    9,
         9,    9,    9,    9,    9,    9,    9,    8,    8,    8,    8,    8,    8,    8,    8,    8,
         8,    8,    8,    8,    8,    8,    7,    7,    7,    7,    7,    7,    7,    7,    7,    7,
         7,    7,    7,    7,    7,    7,    7,    6,    6,    6,    6,    6,    6,    6,    6,    6,
         6,    6,    6,    6,    6,    6,    6,    6,    6,    6,    6,    5,    5,    5,    5,    5,
         5,    5,    5,    5,    5,    5,    5,    5,    5,    5,    5,    5,    5,    5,    5,    5,
         5,    5,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,
         4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    4,    3,
         3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,
         3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,    3,
         3,    3,    3,    3,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,
         2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,
         2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,    2,
         2,    2,    2,    2,    2,    2,    2,    2,    1,    1,    1,    1,    1,    1,    1,    1,
         1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,
         1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,
         1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,
         1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,
         1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,    1,
         0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,
         0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,
         0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0,    0
    };

    // sigmaR = sigma Range (for pixel intensities)
    // sigmaD = sigma Distance (for pixel locations)
    public BilateralFilter(int width, int height, int stride,
            int sigmaR, int sigmaD)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");
        
        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");
        
        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");
        
        if ((sigmaR < 1) && (sigmaR > 32))
            throw new IllegalArgumentException("The range sigma must be in [1..32]");
        
        if ((sigmaD < 1) && (sigmaD > 32))
            throw new IllegalArgumentException("The distance sigma must be in [1..32]");

        this.height = height;
        this.width = width;
        this.stride = stride;
        int sigma = (sigmaR > sigmaD) ? sigmaR : sigmaD;
        this.radius = sigma + sigma;
        int size = this.radius + this.radius + 1;
        this.kernel = new int[size*size];
        this.intensities = new int[256];
        int offs = 0;
        int twoSigmaD2 = 2 * sigmaD * sigmaD;
        int twoSigmaR2 = 2 * sigmaR * sigmaR;
        
        // Create gaussian kernel table for pixel positions
        for (int i=0; i<size; i++, offs+=size)
        {
            int x = i - this.radius;
            int dx2 = x * x;
            
            for (int j=0; j<size; j++)
            {
                int y = j - this.radius;
                int dy2 = y * y;                
                
                // Step in the array is 1 / (1 << 7)
                int idx = ((dx2 + dy2) << 7) / twoSigmaD2;
                
                if (idx < EXP_MINUS_X_2048.length)
                    this.kernel[offs+j] = EXP_MINUS_X_2048[idx];
            }
        }
        
        // Create gaussian table for pixel intensities
        for (int i=0; i<256; i++)
        {
            // Step in the array is 1 / (1 << 7)
            int idx = (i << 7) / twoSigmaR2;
            
            if (idx >= EXP_MINUS_X_2048.length)
               break;
            
            this.intensities[i] = EXP_MINUS_X_2048[idx];
        }
    }

    
    // Works on RGB or YUV 'packed' images
    // Ideally, should be applied to YUV because intensity depends on the color
    // plan (no conversion is made from RGB to YUV and back). However, RGB works
    // fine (with a small distorsion).
    @Override
   public boolean apply(SliceIntArray input, SliceIntArray output)
   {
      if ((!SliceIntArray.isValid(input)) || (!SliceIntArray.isValid(output)))
         return false;
      
        // Aliasing
        final int[] src = input.array;
        final int[] dst = output.array;
        int srcIdx = input.index;
        int dstIdx = output.index;
        final int r = this.radius;
        final int w = this.width;
        final int h = this.height;
        final int[] intens = this.intensities;
        final int[] k = this.kernel;

        final int mult = (r << 1) + 1;

        for (int j=0; j<h; j++)
        {
            // For now, exclude first and last rows (within radius of border)
            int startY = (j >= r) ? j - r : 0;
            int endY = (j + r < h) ? j + r : h;

            for (int i=0; i<w; i++)
            {
                final int val1 = src[srcIdx+i];
                int r1 = (val1 >> 16) & 0xFF;
                int g1 = (val1 >>  8) & 0xFF;
                int b1 =  val1 & 0xFF;

                // For now, exclude first and last columns (within radius of border)
                final int startX = (i >= r) ? i - r : 0;
                final int endX = (i + r < w) ? i + r : w;
                int offs = input.index + (startY * this.stride);
                int kIdx = 0;

                long sumR = 0;
                long sumG = 0;
                long sumB = 0;
                long totalWeightR = 0;
                long totalWeightG = 0;
                long totalWeightB = 0;

                // Apply convolution (intensities * location) to the window
                // of width and height 'radius'
                for (int y=startY; y<endY; y++) 
                {                        
                    for (int x=startX, ii=kIdx; x<endX; x++, ii++)
                    {
                        int dist, weight;
                        final int val2 = src[offs+x];
                        final int val3 = k[ii];
                        final int r2 = (val2 >> 16) & 0xFF;
                        final int g2 = (val2 >>  8) & 0xFF;
                        final int b2 =  val2 & 0xFF;
                        dist = (r1 - r2 + ((r1 - r2) >> 31)) ^ ((r1 - r2) >> 31); 
                        weight = (val3 * intens[dist]) >> 11;
                        totalWeightR += weight;
                        sumR += (weight * r2);
                        dist = (g1 - g2 + ((g1 - g2) >> 31)) ^ ((g1 - g2) >> 31); 
                        weight = (val3 * intens[dist]) >> 11;
                        totalWeightG += weight;
                        sumG += (weight * g2);
                        dist = (b1 - b2 + ((b1 - b2) >> 31)) ^ ((b1 - b2) >> 31);
                        weight = (val3 * intens[dist]) >> 11;
                        totalWeightB += weight;
                        sumB += (weight * b2);
                    }

                    offs += this.stride;
                    kIdx += mult;
                }

                // Set the destination pixel to the average value of the 
                // weighted convoluted pixel values in the window 
                r1 = (int) ((sumR << 5) / ((totalWeightR << 5) + 1));
                g1 = (int) ((sumG << 5) / ((totalWeightG << 5) + 1));
                b1 = (int) ((sumB << 5) / ((totalWeightB << 5) + 1));
                dst[dstIdx+i] = (r1 << 16) | (g1 << 8) | b1;
            }

            srcIdx += this.stride;
            dstIdx += this.stride;
        }
        
        return true;
    }
}