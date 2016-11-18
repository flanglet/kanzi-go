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

package kanzi.function.wavelet;

import kanzi.SliceIntArray;
import kanzi.IntTransform;


// Contrast Sensitivity Function (CSF) used to model the Human Visual System (HVS)
// applied in the wavelet domain
// Other model available (for Y channel) in [Wavelet-based Image Compression
// Using Human Visual System Models] by Andrew Beegan
public class WaveletCSFFilter implements IntTransform
{
   public static final int  Y_CHANNEL = 0;
   public static final int Cb_CHANNEL = 1;
   public static final int Cr_CHANNEL = 2;

    // ----- JPEG 2000 model -----
    //   Recommended color frequency weighting for Y CbCr in JPEG2000
    //   Level5 <-> LL0, Level1 <-> HH4, LH4, HL4
    //
    //   Level Y-channel(LH HL HH)
    //   5 1 1 1
    //   4 1 1 1
    //   3 0.999994 0.999994 0.999988
    //   2 0.837755 0.837755 0.701837
    //   1 0.275783 0.275783 0.090078
    //
    //   Level Cb-channel(LH HL HH)
    //   5 0.812612 0.812612 0.737656
    //   4 0.679829 0.679829 0.567414
    //   3 0.488887 0.488887 0.348719
    //   2 0.267216 0.267216 0.141965
    //   1 0.089950 0.089950 0.027441
    //
    //   Level Cr-channel(LH HL HH)
    //   5 0.856065 0.856065 0.796593
    //   4 0.749805 0.749805 0.655884
    //   3 0.587213 0.587213 0.457826
    //   2 0.375176 0.375176 0.236030
    //   1 0.166647 0.166647 0.070185

    // Weights (times 32) per band level (vertical/horizontal then diagonal band)
    // Only the values for 5 levels are accurate as per tables above.
    // Values for other number of levels are approximates.

    private static final CSF[][] Y_BAND_CSF_WEIGHTS_32 =
    {
       { new CSF(32, 32) },
       { new CSF(32, 32) },
       { new CSF(32, 32), new CSF(32, 32) },
       { new CSF(32, 32), new CSF(32, 32), new CSF(32, 32) },
       { new CSF(32, 32), new CSF(32, 32), new CSF(32, 32), new CSF(27, 22) },
       { new CSF(32, 32), new CSF(32, 32), new CSF(32, 32), new CSF(27, 22), new CSF(9, 3) },
       { new CSF(32, 32), new CSF(32, 32), new CSF(32, 32), new CSF(27, 22), new CSF(9, 3),   new CSF(3, 1) },
       { new CSF(32, 32), new CSF(32, 32), new CSF(32, 32), new CSF(32, 32), new CSF(27, 22), new CSF(9, 3), new CSF(3, 1) }
    };

    private static final CSF[][] Cr_BAND_CSF_WEIGHTS_32 =
    {
       { new CSF(26, 24) },
       { new CSF(26, 24) },
       { new CSF(26, 24), new CSF(26, 24) },
       { new CSF(26, 24), new CSF(26, 24), new CSF(22, 18) },
       { new CSF(26, 24), new CSF(26, 24), new CSF(22, 18), new CSF(16, 11) },
       { new CSF(26, 24), new CSF(22, 18), new CSF(16, 11), new CSF( 9,  5), new CSF(3, 1) },
       { new CSF(26, 24), new CSF(26, 24), new CSF(22, 18), new CSF(16, 11), new CSF( 9,  5), new CSF(3, 1) },
       { new CSF(26, 24), new CSF(26, 24), new CSF(26, 24), new CSF(22, 18), new CSF(16, 11), new CSF(9, 5), new CSF(3, 1) }
    };

    private static final CSF[][] Cb_BAND_CSF_WEIGHTS_32 =
    {
       { new CSF(27, 25) },
       { new CSF(27, 25) },
       { new CSF(27, 25), new CSF(27, 25) },
       { new CSF(27, 25), new CSF(27, 25), new CSF(24, 21) },
       { new CSF(27, 25), new CSF(24, 21), new CSF(19, 15), new CSF(12,  8) },
       { new CSF(27, 25), new CSF(24, 21), new CSF(19, 15), new CSF(12,  8), new CSF(5, 2) },
       { new CSF(27, 25), new CSF(27, 25), new CSF(24, 21), new CSF(19, 15), new CSF(12,  8), new CSF(5,  2) },
       { new CSF(27, 25), new CSF(27, 25), new CSF(27, 25), new CSF(24, 21), new CSF(19, 15), new CSF(12, 8), new CSF(5, 2) }
    };

//    // ----- Other model -----
//    // Highest value adjusted to scaling of 1 (x32)
//    // No difference between HL_LH and HH bands
//    private static final CSF[][] Y2_BAND_CSF_WEIGHTS_32 =
//    {
//       { new CSF(15, 15) },
//       { new CSF(32, 32) },
//       { new CSF(32, 32), new CSF(10, 10) },
//       { new CSF(31, 31), new CSF(26, 26), new CSF(10, 10) },
//       { new CSF(25, 25), new CSF(32, 32), new CSF(26, 26), new CSF(10, 10) },
//       { new CSF(24, 24), new CSF(29, 29), new CSF(32, 32), new CSF(26, 26), new CSF(10, 10) },
//       { new CSF(20, 20), new CSF(24, 24), new CSF(29, 29), new CSF(32, 32), new CSF(26, 26), new CSF(10, 10) },
//       { new CSF(16, 16), new CSF(20, 20), new CSF(24, 24), new CSF(29, 29), new CSF(32, 32), new CSF(26, 26), new CSF(10, 10) }
//    };

    private final int width;
    private final int height;
    private final int levels;
    private final int[] buffer;
    private int channelType;


    public WaveletCSFFilter(int width, int height, int levels, int channelType)
    {
        if (width < 8)
            throw new IllegalArgumentException("The width of the image must be at least 8");

        if (height < 8)
            throw new IllegalArgumentException("The height of the image must be at least 8");

        if (levels < 2)
            throw new IllegalArgumentException("The number of wavelet sub-band levels must be at least 2");

        if (levels >= 8)
           throw new IllegalArgumentException("The number of wavelet sub-band levels must be at most 7");

       if ((channelType != Y_CHANNEL) && (channelType != Cb_CHANNEL)
               && (channelType != Cr_CHANNEL))
           throw new IllegalArgumentException("The channel must be of type Y or Cr or Cb");

        this.width = width;
        this.height = height;
        this.levels = levels;
        this.buffer = new int[(Math.max(width, height) <= 512) ? 16384 : 32768];
        this.channelType = channelType;
    }


    // Not thread safe
    public boolean setChannelType(int channelType)
    {
       if ((channelType != Y_CHANNEL) && (channelType != Cb_CHANNEL)
               && (channelType != Cr_CHANNEL))
          return false;

       this.channelType = channelType;
       return true;
    }


    public int getChannelType()
    {
       return this.channelType;
    }


    // Apply Contrast Sensitivy Function weights to wavelet coefficients
    @Override
    public boolean forward(SliceIntArray source, SliceIntArray destination)
    {
        final int srcIdx = source.index;
        final int dstIdx = destination.index;
        final int[] src = source.array;
        final int[] dst = destination.array;

        WaveletBandScanner sc = new WaveletBandScanner(this.width,
                 this.height, WaveletBandScanner.ALL_BANDS, this.levels);

        final int channel = this.channelType; // protection from concurrent access
        int level = 0;
        int levelSize = 3 * this.width * this.height;
        int startHHBand = (levelSize + levelSize) / 3;
        int read = 0;
        int length = levelSize;
        CSF[] csfWeights;
        
        if (channel == Y_CHANNEL)
           csfWeights = Y_BAND_CSF_WEIGHTS_32[this.levels];
        else if (channel == Cr_CHANNEL)
           csfWeights = Cr_BAND_CSF_WEIGHTS_32[this.levels];
        else
           csfWeights = Cb_BAND_CSF_WEIGHTS_32[this.levels];

        // Process LL0 band
        int csfWeight = csfWeights[level].HL_LH_bands;
        int offset = 0;

        for (int j=0; j<this.height; j++)
        {
           for (int i=0; i<this.width; i++)
           {
              int idx = offset + i;
              int val = src[srcIdx+idx];
              dst[dstIdx+idx] = ((csfWeight * val) + 16); // value multiplied by 32S
           }

           offset += this.width;
        }

        level++;
        csfWeight = csfWeights[level].HL_LH_bands;
        int processed = 0;

        // Process sub-bands
        while (processed < sc.getSize())
        {
           final int l = sc.getIndexes(this.buffer, length, read);
           processed += l;
           read += l;
           int n;
           int part1 = (startHHBand < length) ? startHHBand : length;

           // Process LH, HL and HH bands
           for (n=0; n<part1; n++)
           {
               final int idx = this.buffer[n];
               final int val = src[srcIdx+idx];
               dst[dstIdx+idx] = ((csfWeight * val) + 16) >> 2; // range x8 to x1/4
           }

           if (n == startHHBand)
           {
              csfWeight = csfWeights[level].HH_band;
           }

           for ( ; n<length; n++)
           {
              final int idx = this.buffer[n];
              final int val = src[srcIdx+idx];
              dst[dstIdx+idx] = ((csfWeight * val) + 16) >> 2; // range x8 to x1/4
           }

           if (read == levelSize)
           {
              levelSize <<= 2;
              read = 0;
              level++;
              startHHBand = (levelSize + levelSize) / 3;

              if (level < this.levels)
                 csfWeight = csfWeights[level].HL_LH_bands;
          }

           length = levelSize - read;

           if (length > this.buffer.length)
               length = this.buffer.length;
        }

        source.index = this.width * this.height;
        destination.index =  this.width * this.height;
        return true;
    }


    // Revert Contrast Sensitivy Function weights on wavelet coefficients
    @Override
    public boolean inverse(SliceIntArray source, SliceIntArray destination)
    {
        final int srcIdx = source.index;
        final int dstIdx = destination.index;
        final int[] src = source.array;
        final int[] dst = destination.array;

        WaveletBandScanner sc = new WaveletBandScanner(this.width,
                 this.height, WaveletBandScanner.ALL_BANDS, this.levels);

        final int channel = this.channelType; // protection from concurrent access
        int level = 0;
        int levelSize = 3 * this.width * this.height;
        int startHHBand = (levelSize + levelSize) / 3;
        int read = 0;
        int length = levelSize;
        CSF[] csfWeights;

        if (channel == Y_CHANNEL)
           csfWeights = Y_BAND_CSF_WEIGHTS_32[this.levels];
        else if (channel == Cr_CHANNEL)
           csfWeights = Cr_BAND_CSF_WEIGHTS_32[this.levels];
        else
           csfWeights = Cb_BAND_CSF_WEIGHTS_32[this.levels];

        // Process LL0 band
        int csfWeight = csfWeights[level].HL_LH_bands;
        int offset = 0;

        for (int j=0; j<this.height; j++)
        {
           for (int i=0; i<this.width; i++)
           {
              int idx = this.buffer[offset+i];
              int val = src[srcIdx+idx];
              dst[dstIdx+idx] = val / csfWeight;
           }

           offset += this.width;
        }

        level++;
        csfWeight = csfWeights[level].HL_LH_bands;
        int processed = 0;

        // Process sub-bands
        while (processed < sc.getSize())
        {
           final int l = sc.getIndexes(this.buffer, length, read);
           processed += l;
           read += l;
           int n;
           int part1 = (startHHBand < length) ? startHHBand : length;

           // Process LH, HL and HH bands
           for (n=0; n<part1; n++)
           {
              final int idx = this.buffer[n];
              final int val = src[srcIdx+idx];
              dst[dstIdx+idx] = (val << 2) / csfWeight;
           }

           if (n == startHHBand)
           {
              csfWeight = csfWeights[level].HH_band;
           }

           for ( ; n<length; n++)
           {
              final int idx = this.buffer[n];
              final int val = src[srcIdx+idx];
              dst[dstIdx+idx] = (val << 2) / csfWeight;
           }

           if (read == levelSize)
           {
              levelSize <<= 2;
              read = 0;
              level++;
              csfWeight = csfWeights[level].HL_LH_bands;
              startHHBand = (levelSize + levelSize) / 3;
          }

           length = levelSize - read;

           if (length > this.buffer.length)
               length = this.buffer.length;
        }

        source.index = this.width * this.height;
        destination.index = this.width * this.height;
        return true;
    }


    private static class CSF
    {
       final int HL_LH_bands;
       final int HH_band;

       CSF(int HL_LH_bands, int HH_band)
       {
          this.HL_LH_bands = HL_LH_bands;
          this.HH_band = HH_band;
       }
    }
}