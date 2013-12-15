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

package kanzi.quantization;

import java.util.Arrays;

public class IntraTables
{
   public static final int FLAT = 0;
   public static final int BALANCED = 1;
   public static final int STEEP = 2;

   private IntraTables() { }

   
   public static final IntraTables SINGLETON = new IntraTables();

   
   private static final int[] QSCALE = new int[]
   {
      3623, 3385, 3164, 2958, 2767, 2589, 2424, 2270,
      2127, 1994, 1870, 1754, 1646, 1545, 1451, 1363,
      1281, 1205, 1133, 1067, 1004,  946,  891,  840,
       792,  747,  705,  666,  629,  594,  561,  531,
       502,  475,  450,  426,  404,  383,  363,  344,
       327,  310,  295,  280,  266,  253,  241,  229,
       218,  208,  198,  189,  180,  172,  164,  156,
       149,  143,  136,  130,  125,  119,  114,  109,
       105,  100,   96,   92,   89,   85,   82,   79,
       76,    73,   70,   67,   65,   62,   60,   58,
       56,    54,   52,   50,   48,   47,   45,   44,
       42,    41,   40,   38,   37,   36,   35,   34,
       33,    32,   24,   16           
   }; // step in 1/32th


   private static int[][][] init(int dim)
   {
      if (dim == 4)
      {
         return new int[][][]
         {
            // LUMA
            new int[][] { FLAT_4x4, BALANCED_4x4, STEEP_4x4 },
            // CHROMA U
            new int[][] { BALANCED_4x4, STEEP_4x4, STEEP_4x4 },
            // CHROMA V
            new int[][] { BALANCED_4x4, STEEP_4x4, STEEP_4x4 }
         };
      }

      if (dim == 8)
      {
         return new int[][][]
         {
            // LUMA
            new int[][] { Y_FLAT_8x8, Y_BALANCED_8x8, Y_STEEP_8x8 },
            // CHROMA U
            new int[][] { UV_FLAT_8x8, UV_BALANCED_8x8, UV_STEEP_8x8 },
            // CHROMA V
            new int[][] { UV_FLAT_8x8, UV_BALANCED_8x8, UV_STEEP_8x8 }
         };
      }

      if (dim == 16)
      {
         final int[] y_flat      = extend(new int[dim*dim], TABLES_8x8[0][FLAT], dim);
         final int[] y_balanced  = extend(new int[dim*dim], TABLES_8x8[0][BALANCED], dim);
         final int[] y_steep     = extend(new int[dim*dim], TABLES_8x8[0][STEEP], dim);
         final int[] uv_flat     = extend(new int[dim*dim], TABLES_8x8[1][FLAT], dim);
         final int[] uv_balanced = extend(new int[dim*dim], TABLES_8x8[1][BALANCED], dim);
         final int[] uv_steep    = extend(new int[dim*dim], TABLES_8x8[1][STEEP], dim);

         return new int[][][]
         {
            // LUMA
            new int[][] { y_flat, y_balanced, y_steep },
            // CHROMA U
            new int[][] { uv_flat, uv_balanced, uv_steep },
            // CHROMA V
            new int[][] { uv_flat, uv_balanced, uv_steep }
         };
      }

      if (dim == 32)
      {
         final int[] y_flat      = extend(new int[dim*dim], TABLES_16x16[0][FLAT], dim);
         final int[] y_balanced  = extend(new int[dim*dim], TABLES_16x16[0][BALANCED], dim);
         final int[] y_steep     = extend(new int[dim*dim], TABLES_16x16[0][STEEP], dim);
         final int[] uv_flat     = extend(new int[dim*dim], TABLES_16x16[1][FLAT], dim);
         final int[] uv_balanced = extend(new int[dim*dim], TABLES_16x16[1][BALANCED], dim);
         final int[] uv_steep    = extend(new int[dim*dim], TABLES_16x16[1][STEEP], dim);

         return new int[][][]
         {
            // LUMA
            new int[][] { y_flat, y_balanced, y_steep },
            // CHROMA U
            new int[][] { uv_flat, uv_balanced, uv_steep },
            // CHROMA V
            new int[][] { uv_flat, uv_balanced, uv_steep }
         };
      }

      return null;
   }


   private static int[] extend(int[] output, int input[], int dim)
   {
      Arrays.fill(output, 0, dim*dim, input[(dim/2)*(dim/2)-1]);

      for (int j=0; j<(dim/2); j++)
      {
         for (int i=0; i<(dim/2); i++)
         {
            final int offs = j * (dim/2) + i;
            output[offs] = input[offs];
         }
      }

      return output;
   }


   private static final int[] FLAT_4x4 = new int[]
   {
      32, 36, 32, 36,
      36, 40, 36, 40,
      32, 36, 32, 36,
      36, 40, 36, 40
   };


   private static final int[] BALANCED_4x4 = new int[]
   {
      32, 40, 32, 40,
      40, 50, 40, 50,
      32, 40, 32, 40,
      40, 50, 40, 50
   };


   private static final int[] STEEP_4x4 = new int[]
   {
      32, 60, 40, 60,
      60, 75, 60, 75,
      40, 60, 40, 60,
      60, 75, 60, 75
   };


   private static final int[] Y_FLAT_8x8 =  new int[]
   {
      32,  33,  34,  35,  36,  37,  38,  40,
      33,  34,  35,  36,  37,  38,  40,  41,
      34,  35,  36,  37,  38,  40,  41,  42,
      35,  36,  37,  38,  40,  41,  42,  43,
      36,  37,  38,  40,  41,  42,  43,  45,
      37,  38,  40,  41,  42,  43,  45,  47,
      38,  40,  41,  42,  43,  45,  47,  49,
      40,  41,  42,  43,  45,  47,  49,  52
   };


   private static final int[] Y_BALANCED_8x8 =  new int[]
   {
      32,  28,  30,  32,  40,  48,  56,  64,
      28,  28,  30,  35,  44,  53,  64,  80,
      30,  30,  32,  40,  50,  64,  80,  96,
      32,  35,  40,  50,  60,  80,  96, 112,
      40,  44,  50,  60,  80,  96, 112, 128,
      48,  53,  64,  80,  96, 112, 128, 144,
      56,  64,  80,  96, 112, 128, 144, 160,
      64,  80,  96, 112, 128, 144, 160, 176
   };


   private static final int[] Y_STEEP_8x8 =  new int[]
   {
      32,  30,  34,  40,  55,  92, 112, 165,
      30,  30,  34,  45,  59, 128, 147, 180,
      34,  34,  36,  55,  92, 131, 180, 195,
      40,  45,  50,  66, 117, 180, 195, 210,
      55,  59,  85, 133, 180, 195, 210, 226,
      92, 128, 126, 180, 195, 210, 226, 245,
     112, 147, 180, 195, 210, 226, 245, 265,
     165, 180, 195, 210, 226, 245, 265, 290
   };


   private static final int[] UV_FLAT_8x8 = new int[]
   {
       32,  36,  48,  79, 160, 160, 160, 160,
       36,  40,  52, 110, 160, 160, 160, 160,
       48,  52,  90, 160, 160, 160, 160, 160,
       79, 110, 160, 160, 160, 160, 160, 160,
      160, 160, 160, 160, 160, 160, 160, 160,
      160, 160, 160, 160, 160, 160, 160, 160,
      160, 160, 160, 160, 160, 160, 160, 160,
      160, 160, 160, 160, 160, 160, 160, 160
   };


   private static final int[] UV_BALANCED_8x8 = new int[]
   {
      32,  37,  50,  97, 200, 200, 200, 200,
      37,  43,  54, 138, 200, 200, 200, 200,
      50,  54, 117, 200, 200, 200, 200, 200,
      97, 138, 200, 200, 200, 200, 200, 200,
     200, 200, 200, 200, 200, 200, 200, 200,
     200, 200, 200, 200, 200, 200, 200, 200,
     200, 200, 200, 200, 200, 200, 200, 200,
     200, 200, 200, 200, 200, 200, 200, 200
   };


   private static final int[] UV_STEEP_8x8 = new int[]
   {
      36,  43,  62, 108, 230, 230, 230, 230,
      43,  53,  67, 166, 230, 230, 230, 230,
      62,  67, 142, 230, 230, 230, 230, 230,
     108, 166, 230, 230, 230, 230, 230, 230,
     230, 230, 230, 230, 230, 230, 230, 230,
     230, 230, 230, 230, 230, 230, 230, 230,
     230, 230, 230, 230, 230, 230, 230, 230,
     230, 230, 230, 230, 230, 230, 230, 230
   };


   private static final int[][][] TABLES_4x4   = init(4);
   private static final int[][][] TABLES_8x8   = init(8);
   private static final int[][][] TABLES_16x16 = init(16);
   private static final int[][][] TABLES_32x32 = init(32);


   // First index is channel
   // Second index is type (FLAT, BALANCED, STEEP)
   public int[][][] getTables(int dim)
   {
      switch (dim)
      {
         case 4:
            return TABLES_4x4;

         case 8:
            return TABLES_8x8;

         case 16:
            return TABLES_16x16;

         case 32:
            return TABLES_32x32;

         default:
            return null;
      }
   }


   public int getMaxQuantizerIndex()
   {
      return QSCALE.length - 1;
   }


   public int getQuantizer(int qidx)
   {
      if (qidx < 0)
         return QSCALE[0];

      if (qidx >= QSCALE.length)
         return QSCALE[QSCALE.length-1];

      return QSCALE[qidx];
   }
}
