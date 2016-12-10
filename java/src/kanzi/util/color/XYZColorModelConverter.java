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

package kanzi.util.color;

import kanzi.ColorModelType;


// Fat color model converter RGB <-> CIE 1931 XYZ
public final class XYZColorModelConverter implements ColorModelConverter
{
    private final int height;
    private final int width;
    private final int rgbOffset;
    private final int stride;
    private final int shift;

    // 4096 * { ((x+0.055)/1.055) ^ 2.4 if (x>0.04045) else x/12.92 }
    // Range 256, Increment 1 => 256 values
    private static final int[] RGB_XYZ = new int[] 
    {
         0,    1,    2,    4,    5,    6,    7,    9,   10,   11, 
        12,   14,   15,   16,   18,   20,   21,   23,   25,   27, 
        29,   31,   33,   35,   37,   40,   42,   45,   48,   50, 
        53,   56,   59,   62,   66,   69,   72,   76,   79,   83, 
        87,   91,   95,   99,  103,  107,  112,  116,  121,  126, 
       131,  136,  141,  146,  151,  156,  162,  168,  173,  179, 
       185,  191,  197,  204,  210,  217,  223,  230,  237,  244, 
       251,  258,  265,  273,  280,  288,  296,  304,  312,  320, 
       329,  337,  346,  354,  363,  372,  381,  390,  400,  409, 
       419,  429,  438,  448,  458,  469,  479,  490,  500,  511, 
       522,  533,  544,  556,  567,  579,  590,  602,  614,  626, 
       639,  651,  664,  676,  689,  702,  715,  729,  742,  756, 
       769,  783,  797,  811,  826,  840,  855,  869,  884,  899, 
       914,  930,  945,  961,  976,  992, 1008, 1025, 1041, 1058, 
      1074, 1091, 1108, 1125, 1142, 1160, 1177, 1195, 1213, 1231, 
      1249, 1268, 1286, 1305, 1324, 1343, 1362, 1381, 1400, 1420, 
      1440, 1460, 1480, 1500, 1521, 1541, 1562, 1583, 1604, 1625, 
      1647, 1668, 1690, 1712, 1734, 1756, 1778, 1801, 1824, 1846, 
      1869, 1893, 1916, 1940, 1963, 1987, 2011, 2035, 2060, 2084, 
      2109, 2134, 2159, 2184, 2210, 2235, 2261, 2287, 2313, 2339, 
      2366, 2392, 2419, 2446, 2473, 2501, 2528, 2556, 2584, 2612, 
      2640, 2668, 2697, 2725, 2754, 2783, 2813, 2842, 2872, 2902, 
      2931, 2962, 2992, 3022, 3053, 3084, 3115, 3146, 3178, 3209, 
      3241, 3273, 3305, 3338, 3370, 3403, 3436, 3469, 3502, 3535, 
      3569, 3603, 3637, 3671, 3705, 3740, 3775, 3810, 3845, 3880, 
      3916, 3951, 3987, 4023, 4060, 4096, 4096
    };


    // 255 * 1.055f * ((x/4096 ^ 1/2.4) - 0.055f) if (x/4096>0.0031308) else x*12.92 }
    // Range 4096, Increment 8 => 512 values
    private static final int[] XYZ_RGB = new int[] 
    {
         0,    6,   12,   17,   21,   24,   27,   30,   33,   35, 
        37,   40,   42,   43,   45,   47,   49,   50,   52,   53, 
        55,   56,   58,   59,   60,   62,   63,   64,   65,   67, 
        68,   69,   70,   71,   72,   73,   74,   75,   76,   77, 
        78,   79,   80,   81,   82,   83,   84,   85,   86,   86, 
        87,   88,   89,   90,   91,   91,   92,   93,   94,   95, 
        95,   96,   97,   98,   98,   99,  100,  100,  101,  102, 
       103,  103,  104,  105,  105,  106,  107,  107,  108,  109, 
       109,  110,  111,  111,  112,  113,  113,  114,  114,  115, 
       116,  116,  117,  117,  118,  119,  119,  120,  120,  121, 
       121,  122,  123,  123,  124,  124,  125,  125,  126,  126, 
       127,  127,  128,  129,  129,  130,  130,  131,  131,  132, 
       132,  133,  133,  134,  134,  135,  135,  136,  136,  137, 
       137,  138,  138,  139,  139,  140,  140,  141,  141,  141, 
       142,  142,  143,  143,  144,  144,  145,  145,  146,  146, 
       147,  147,  147,  148,  148,  149,  149,  150,  150,  150, 
       151,  151,  152,  152,  153,  153,  153,  154,  154,  155, 
       155,  156,  156,  156,  157,  157,  158,  158,  158,  159, 
       159,  160,  160,  160,  161,  161,  162,  162,  162,  163, 
       163,  164,  164,  164,  165,  165,  166,  166,  166,  167, 
       167,  167,  168,  168,  169,  169,  169,  170,  170,  170, 
       171,  171,  172,  172,  172,  173,  173,  173,  174,  174, 
       174,  175,  175,  175,  176,  176,  177,  177,  177,  178, 
       178,  178,  179,  179,  179,  180,  180,  180,  181,  181, 
       181,  182,  182,  182,  183,  183,  183,  184,  184,  184, 
       185,  185,  185,  186,  186,  186,  187,  187,  187,  188, 
       188,  188,  189,  189,  189,  190,  190,  190,  191,  191, 
       191,  192,  192,  192,  193,  193,  193,  193,  194,  194, 
       194,  195,  195,  195,  196,  196,  196,  197,  197,  197, 
       197,  198,  198,  198,  199,  199,  199,  200,  200,  200, 
       201,  201,  201,  201,  202,  202,  202,  203,  203,  203, 
       203,  204,  204,  204,  205,  205,  205,  206,  206,  206, 
       206,  207,  207,  207,  208,  208,  208,  208,  209,  209, 
       209,  210,  210,  210,  210,  211,  211,  211,  211,  212, 
       212,  212,  213,  213,  213,  213,  214,  214,  214,  215, 
       215,  215,  215,  216,  216,  216,  216,  217,  217,  217, 
       218,  218,  218,  218,  219,  219,  219,  219,  220,  220, 
       220,  220,  221,  221,  221,  221,  222,  222,  222,  223, 
       223,  223,  223,  224,  224,  224,  224,  225,  225,  225, 
       225,  226,  226,  226,  226,  227,  227,  227,  227,  228, 
       228,  228,  228,  229,  229,  229,  229,  230,  230,  230, 
       230,  231,  231,  231,  231,  232,  232,  232,  232,  233, 
       233,  233,  233,  234,  234,  234,  234,  235,  235,  235, 
       235,  236,  236,  236,  236,  237,  237,  237,  237,  238, 
       238,  238,  238,  238,  239,  239,  239,  239,  240,  240, 
       240,  240,  241,  241,  241,  241,  242,  242,  242,  242, 
       242,  243,  243,  243,  243,  244,  244,  244,  244,  245, 
       245,  245,  245,  245,  246,  246,  246,  246,  247,  247, 
       247,  247,  248,  248,  248,  248,  248,  249,  249,  249, 
       249,  250,  250,  250,  250,  250,  251,  251,  251,  251, 
       252,  252,  252,  252,  252,  253,  253,  253,  253,  254, 
       254,  254,  254,  254
    };
    
    
    // default YUV precision: [0..4096]
    public XYZColorModelConverter(int width, int height)
    {
        this(width, height, 0, width);
    }


    // shift is used to scale the YUV values: 0 -> [0..4096], 4 -> [0..256], 12 -> [0..1]
    public XYZColorModelConverter(int width, int height, int shift)
    {
        this(width, height, 0, width, shift);
    }


    // default YUV precision: [0..256]
    public XYZColorModelConverter(int width, int height, int rgbOffset, int stride)
    {
       this(width, height, rgbOffset, stride, 4);
    }
    

    // rgbOffset is the offset in the RGB frame while stride is the width of the RGB frame
    // width and height are the dimension of the XYZ frame
    // shift is used to scale the YUV values: 0 -> [0..4096], 4 -> [0..256], 12 -> [0..1]
    public XYZColorModelConverter(int width, int height, int rgbOffset, int stride, int shift)
    {
        if (height < 8)
            throw new IllegalArgumentException("The height must be at least 8");

        if (width < 8)
            throw new IllegalArgumentException("The width must be at least 8");

        if (stride < 8)
            throw new IllegalArgumentException("The stride must be at least 8");

        if ((height & 7) != 0)
            throw new IllegalArgumentException("The height must be a multiple of 8");

        if ((width & 7) != 0)
            throw new IllegalArgumentException("The width must be a multiple of 8");

        if ((stride & 7) != 0)
            throw new IllegalArgumentException("The stride must be a multiple of 8");

        if ((shift < 0) || (shift > 12))
            throw new IllegalArgumentException("The shift parameter must be in [0..12]");

        this.height = height;
        this.width = width;
        this.rgbOffset = rgbOffset;
        this.stride = stride;
        this.shift = shift;
    }


    // conversion matrix 
    // 0.4124564  0.3575761  0.1804375
    // 0.2126729  0.7151522  0.0721750
    // 0.0193339  0.1191920  0.9503041
    @Override
    public boolean convertRGBtoYUV(int[] rgb, int[] xval, int[] yval, int[] zval, ColorModelType type)
    {
        if (type != ColorModelType.XYZ)
           return false;
       
        int startLine  = this.rgbOffset;
        int startLine2 = 0;
        final int shift_ = this.shift + 12;
        final int adjust_ = 1 << (shift_ - 1);        

        for (int j=0; j<this.height; j++)
        {
            final int end = startLine + this.width;

            for (int k=startLine, i=startLine2; k<end; i++)
            {
                // ------- fromRGB 'Macro'
                final int rgbVal = rgb[k++];
                int r = (rgbVal >> 16) & 0xFF;
                int g = (rgbVal >> 8)  & 0xFF;
                int b =  rgbVal & 0xFF;

                // Scaled by 4096
                r = (RGB_XYZ[r] + RGB_XYZ[r+1]) >> 1;
                g = (RGB_XYZ[g] + RGB_XYZ[g+1]) >> 1;               
                b = (RGB_XYZ[b] + RGB_XYZ[b+1]) >> 1;               

                xval[i] = (1689*r + 1465*g +  739*b + adjust_) >> shift_;
                yval[i] = ( 871*r + 2929*g +  296*b + adjust_) >> shift_;
                zval[i] = (  79*r +  488*g + 3892*b + adjust_) >> shift_;
                // ------- fromRGB 'Macro'  END
            }

            startLine2 += this.width;
            startLine  += this.stride;
        }

        return true;
    }


    // conversion matrix 
    //  3.2404542 -1.5371385 -0.4985314
    // -0.9692660  1.8760108  0.0415560
    //  0.0556434 -0.2040259  1.0572252
    @Override
    public boolean convertYUVtoRGB(int[] xval, int[] yval, int[] zval, int[] rgb, ColorModelType type)
    {
        if (type != ColorModelType.XYZ)
           return false;

        int startLine = 0;
        int startLine2 = this.rgbOffset;
        final int shift_ = this.shift;

        for (int j=0; j<this.height; j++)
        {
            final int end = startLine + this.width;

            for (int i=startLine, k=startLine2; i<end; i++)
            {
                // ------- toRGB 'Macro'
                // Scaled by 4096
                final int xVal = xval[i] << shift_;
                final int yVal = yval[i] << shift_;
                final int zVal = zval[i] << shift_;

                int r = (13273*xVal - 6296*yVal - 2042*zVal + 2048) >> 12;
                int g = (-3970*xVal + 7684*yVal +  170*zVal + 2048) >> 12;
                int b =   (228*xVal -  836*yVal + 4330*zVal + 2048) >> 12;
                
                if (r >= 4096)
                   r = 0xFF0000;
                else
                {
                   r &= ~(r >> 31);
                   r = (XYZ_RGB[r>>3] + XYZ_RGB[(r>>3)+1]) >> 1;
                   r <<= 16;
                }
                
                if (g >= 4096)
                   g = 0x00FF00;
                else
                {
                   g &= ~(g >> 31);
                   g = (XYZ_RGB[g>>3] + XYZ_RGB[(g>>3)+1]) >> 1;
                   g <<= 8;
                }

                if (b >= 4096)
                   b = 0x0000FF;
                else
                {
                   b &= ~(b >> 31);
                   b = (XYZ_RGB[b>>3] + XYZ_RGB[(b>>3)+1]) >> 1;
                }

                rgb[k++] = r | g | b;
            }

            startLine  += this.width;
            startLine2 += this.stride;
        }

        return true;
    }
    
    
    @Override
    public String toString() 
    {
       return "XYZ";
    }
}