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

package kanzi.function;

import kanzi.ByteFunction;
import kanzi.IndexedByteArray;


// Distance coder / decoder
// Can be used to replace the Move-To-Front + Run Length Encoding post BWT
//
// The algorithm is explained in the following example
// Example: input = "caracaras", BTW(input) = "rccrsaaaa"
//
// The symbols to be processed will be displayed with upper case
// The ones already processed will be displayed with lower case
// The symbol being processed will be displayed within brackets
//   alphabet | BWT data  | EOF
//   acrs     | rccrsaaaa | # (13 bytes + EOF)
//
// Forward:
// For each symbol, find the next ocurrence (not in a run) and store the distance
// (ignoring already processed symbols).
// STEP 1 : Processing a:  [a]crs RCCRS[a]AAA    #  d=6 (number of upper case symbols + 1)
// STEP 2 : Processing c:  a[c]rs R[c]CRSaAAA    #  d=2
// STEP 3 : Processing r:  ac[r]s [r]cCRSaAAA    #  d=1
// STEP 4 : Processing s:  acr[s] rcCR[s]aAAA    #  d=3
// STEP 5 : Processing r:  acrs   [r]cC[r]saAAA  #  d=2
// STEP 6 : Processing c:  acrs   r[c][c]rsaAAA  #  skip (it is a run)
// STEP 7 : Processing c:  acrs   rc[c]rsaAAA   [#] d=0 (no more 'c')
// STEP 8 : Processing r:  acrs   rcc[r]saAAA   [#] d=0 (no more 'r')
// STEP 9 : Processing s:  acrs   rccr[s]aAAA   [#] d=0 (no more 's')
// STEP 10: Processing a:  acrs   rccrs[a][a]AA  #  skip (it is a run)
// STEP 11: Processing a:  acrs   rccrsa[a][a]A  #  skip (it is a run)
// STEP 12: Processing a:  acrs   rccrsaa[a][a]  #  skip (it is a run)
//
// Result: DC(BTW(input)) = 62132000
//
// Inverse:
//
// STEP 1 : Processing a, d=6 => [a]crs ?????[a]???  (count unprocessed symbols)
// STEP 2 : Processing c, d=2 => a[c]rs ?[c]???a???
// STEP 3 : Processing r, d=1 => ac[r]s [r]c???a???
// STEP 4 : Processing s, d=3 => acr[s] rc??[s]a???
// STEP 5 : Processing r, d=2 => acrs   [r]c?[r]sa???
// STEP 6 : Processing c, d=0 => acrs   r[c]?rsa???   (unchanged, no more 'c')
// STEP 7 : Processing ?,     => acrs   rc[?]rsa???   unknown symbol, repeat previous symbol)
// STEP 8 : Processing r, d=0 => acrs   rcc[r]sa???   (unchanged, no more 'r')
// STEP 9 : Processing s, d=0 => acrs   rccr[s]a???   (unchanged, no more 's')
// STEP 10: Processing ?,     => acrs   rccrsa[?]??   unknown byte, repeat previous byte)
// STEP 11: Processing ?,     => acrs   rccrsaa[?]?   unknown byte, repeat previous byte)
// STEP 12: Processing ?,     => acrs   rccrsaaa[?]   unknown byte, repeat previous byte)
//
// Result: invDC(DC(BTW(input))) = "rccrsaaaa" = BWT(input)

public class DistanceCodec implements ByteFunction
{
    private static int DEFAULT_DISTANCE_THRESHOLD = 0x80;

    private int size;
    private byte[] data;
    private final int[] buffer;
    private final int logDistThreshold;


    public DistanceCodec()
    {
       this(0);
    }


    public DistanceCodec(int size)
    {
       this(size, DEFAULT_DISTANCE_THRESHOLD);
    }


    public DistanceCodec(int size, int distanceThreshold)
    {
       if (distanceThreshold < 4)
           throw new IllegalArgumentException("The distance threshold cannot be less than 4");

       if (distanceThreshold > 0x80)
           throw new IllegalArgumentException("The distance threshold cannot more than 128");

       if ((distanceThreshold  & (distanceThreshold - 1)) != 0)
           throw new IllegalArgumentException("The distance threshold must be a multiple of 2");

       this.size = size;
       this.buffer = new int[256];
       this.data = new byte[0];
       int log2 = 0;
       distanceThreshold++;

        for ( ; distanceThreshold>1; distanceThreshold>>=1)
            log2++;

        this.logDistThreshold = log2;
    }


    // Determine the alphabet and encode it in the destination array
    // Encode the distance for each symbol in the alphabet (of bytes)
    // The header is either:
    // 0 + 256 * encoded distance for each character
    // or (if aplhabet size >= 32)
    // alphabet size (byte) + 32 bytes (bit encoded presence of symbol) +
    // n (<256) * encoded distance for each symbol
    // else
    // alphabet size (byte) + m (<32) alphabet symbols +
    // n (<256) * encoded distance for each symbol
    // The distance is encoded as 1 byte if less than 'distance threshold' or
    // else several bytes (with a mask to indicate continuation)
    // Return success or failure
    private boolean encodeHeader(IndexedByteArray src, IndexedByteArray dst, byte[] significanceFlags)
    {
       try
       {
          final byte[] srcArray = src.array;
          final byte[] dstArray = dst.array;
          int srcIdx = src.index;
          final int[] positions = this.buffer;
          final int inLength = (this.size == 0) ? srcArray.length - srcIdx : this.size;
          final int eof = src.index + inLength + 1;

          // Set all the positions to 'unknown'
          for (int i=0; i<positions.length; i++)
            positions[i] = eof;

          byte current = (byte) ~srcArray[srcIdx];
          int alphabetSize = 0;

          // Record the position of the first occurence of each symbol
          while ((alphabetSize < 256) && (srcIdx < inLength))
          {
            // Skip run
            while ((srcIdx < inLength) && (srcArray[srcIdx] == current))
               srcIdx++;

            // Fill distances array by finding first occurence of each symbol
            if (srcIdx < inLength)
            {
               current = srcArray[srcIdx];
               int idx = current & 0xFF;

               if (positions[idx] == eof)
               {
                  // distance = alphabet size + index
                  positions[idx] = srcIdx - src.index;
                  alphabetSize++;
               }

               srcIdx++;
            }
          }

          // Check if alphabet is complete (256 symbols), if so encode 0
          if (alphabetSize == 256)
             dstArray[dst.index++] = 0;
          else
          {
             // Encode size, then encode the alphabet
             dstArray[dst.index++] = (byte) alphabetSize;

             if (alphabetSize >= 32)
             {
                // Big alphabet, encode symbol presence bit by bit
                for (int i=0; i<256; i+=8)
                {
                   byte val = 0;

                   for (int j=0, mask=1; j<8; j++, mask<<=1)
                   {
                      if (positions[i+j] != eof)
                        val |= mask;
                   }

                   dstArray[dst.index++] = val;
                }
             }
             else // small alphabet, spell each symbol
             {
                int previous = 0;

                for (int i=0; i<256; i++)
                {
                   if (positions[i] != eof)
                   {
                      // Encode sumbol as delta
                      dstArray[dst.index++] = (byte) (i - previous);
                      previous = i;
                   }
                }
             }
          }

          final int distThreshold = 1 << this.logDistThreshold;
          final int distMask = distThreshold - 1;

          // For each symbol in the alphabet, encode distance
          for (int i=0; i<256; i++)
          {
             int position = positions[i];

             if (position != eof)
             {
               int distance = 1;

               // Calculate distance
               for (int j=0; j<position; j++)
                  distance += significanceFlags[j];

               // Mark symbol as already used
               significanceFlags[position] = 0;

               // Encode distance over one or several bytes with mask distThreshold
               // to indicate a continuation
               while (distance >= distThreshold)
               {
                  dstArray[dst.index++] = (byte) (distThreshold | (distance & distMask));
                  distance >>= this.logDistThreshold;
               }

               dstArray[dst.index++] = (byte) distance;
             }
          }
       }
       catch (ArrayIndexOutOfBoundsException e)
       {
          return false;
       }

       return true;
    }


    @Override
    public boolean forward(IndexedByteArray source, IndexedByteArray destination)
    {
      if ((source == null) || (destination == null) || (source.array == destination.array))
         return false;

       final byte[] src = source.array;
       final byte[] dst = destination.array;
       final int inLength = (this.size == 0) ? src.length - source.index : this.size;

       if (this.data.length < inLength)
           this.data = new byte[inLength];

       final byte[] unprocessedFlags = this.data; // aliasing

       for (int i=0; i<unprocessedFlags.length; i++)
          unprocessedFlags[i] = 1;

       // Encode header, update src.index and dst.index
       if (this.encodeHeader(source, destination, unprocessedFlags) == false)
          return false;

       // Encode body (corresponding to input data only)
       int srcIdx = source.index;
       int dstIdx = destination.index;
       boolean res;

       try
       {
           final int distThreshold = 1 << this.logDistThreshold;
           final int distMask = distThreshold - 1;

           while (srcIdx < inLength)
           {
              final byte first = src[srcIdx];
              byte current = first;
              int distance = 1;

              // Skip initial run
              while ((srcIdx < inLength) && (src[srcIdx] == current))
                 srcIdx++;

              // Save index of next (different) symbol
              final int nextIdx = srcIdx;

              while (srcIdx < inLength)
              {
                 current = src[srcIdx];

                 // Next occurence of first symbol found => exit
                 if (current == first)
                    break;

                 // Already processed symbols are ignored (flag = 0)
                 distance += unprocessedFlags[srcIdx-source.index];
                 srcIdx++;
              }

              if (srcIdx == inLength)
              {
                 // The symbol has not been found, encode 0
                 if (current != first)
                    dst[dstIdx++] = 0;
              }
              else
              {
                 // Mark symbol as already used (first of run only)
                 unprocessedFlags[srcIdx] = 0;

                 // Encode distance over one or several bytes with mask distThreshold
                 // to indicate a continuation
                 while (distance >= distThreshold)
                 {
                    dst[dstIdx++] = (byte) (distThreshold | (distance & distMask));
                    distance >>= this.logDistThreshold;
                 }

                 dst[dstIdx++] = (byte) distance;
              }

              // Move to next symbol
              srcIdx = nextIdx;
           }

           res = ((srcIdx - source.index) == inLength) ? true : false;
       }
       catch (ArrayIndexOutOfBoundsException e)
       {
          res = false;
       }

       source.index = srcIdx;
       destination.index = dstIdx;
       return res;
    }


    private boolean decodeHeader(IndexedByteArray src, IndexedByteArray dst, byte[] unprocessedFlags)
    {
       try
       {
          final byte[] srcArray = src.array;
          final byte[] dstArray = dst.array;
          final int[] alphabet = this.buffer; // aliasing

          final int alphabetSize = srcArray[src.index++] & 0xFF;
          final boolean completeAlphabet = (alphabetSize == 0) ? true : false;

          if (completeAlphabet == false)
          {
             // Reset list of present symbols
             for (int i=0; i<alphabet.length; i++)
                alphabet[i] = 0;

             if (alphabetSize >= 32)
             {
                // Big alphabet, decode symbol presence mask
                for (int i=0; i<256; i+=8)
                {
                   final int val = srcArray[src.index++] & 0xFF;

                   for (int j=0, mask=1; j<8; j++, mask<<=1)
                     alphabet[i+j] = val & mask;
                }
             }
             else // small alphabet, list all present symbols
             {
                int previous = 0;

                for (int i=0; i<alphabetSize; i++)
                {
                   final int delta = srcArray[src.index++] & 0xFF;
                   alphabet[previous + delta] = 1;
                   previous += delta;
                }
             }
          }

          final int distThreshold = 1 << this.logDistThreshold;
          final int distMask = distThreshold - 1;

          // Process alphabet (find first occurence of each symbol)
          for (int i=0; i<256; i++)
          {
            if ((completeAlphabet == true) || (alphabet[i] != 0))
            {
              int val = srcArray[src.index++] & 0xFF;
              int distance = 0;
              int shift = 0;

              // Decode distance
              while (val >= distThreshold)
              {
                 distance |= ((val & distMask) << shift);
                 shift += this.logDistThreshold;
                 val = srcArray[src.index++] & 0xFF;
              }

              // Distance cannot be 0 since the symbol is present in the alphabet
              distance |= (val << shift);
              int idx = -1;

              while (distance > 0)
                distance -= unprocessedFlags[++idx];

              // Output next occurence
              dstArray[idx] = (byte) i;
              unprocessedFlags[idx] = 0;
           }
         }
       }
       catch (ArrayIndexOutOfBoundsException e)
       {
          return false;
       }

       return true;
    }


    @Override
    public boolean inverse(IndexedByteArray source, IndexedByteArray destination)
    {
      if ((source == null) || (destination == null) || (source.array == destination.array))
         return false;

       final byte[] src = source.array;
       final byte[] dst = destination.array;
       final int end = (this.size == 0) ? src.length : source.index + this.size;

       if (this.data.length < dst.length - destination.index)
           this.data = new byte[dst.length-destination.index];

       final byte[] unprocessedFlags = this.data; // aliasing

       for (int i=0; i<unprocessedFlags.length; i++)
          unprocessedFlags[i] = 1;

       // Decode header, update src.index and dst.index
       if (this.decodeHeader(source, destination, unprocessedFlags) == false)
          return false;

       int srcIdx = source.index;
       int dstIdx = destination.index;
       boolean res;

       try
       {
           byte current = dst[dstIdx];
           final int distThreshold = 1 << this.logDistThreshold;
           final int distMask = distThreshold - 1;

           // Decode body
           while (true)
           {
              // If the current symbol is unknown, duplicate previous
              if (unprocessedFlags[dstIdx] != 0)
              {
                 dst[dstIdx++] = current;
                 continue;
              }

              // Get current symbol
              current = dst[dstIdx++];

              if (srcIdx >= end)
                 break;

              // For the current symbol, get distance to the next occurence
              int distance = src[srcIdx++] & 0xFF;

              // Last occurence of current symbol
              if (distance == 0)
                continue;

              if (distance >= distThreshold)
              {
                 int val = distance;
                 distance = 0;
                 int shift = 0;

                 // Decode distance
                 while (val >= distThreshold)
                 {
                    distance |= ((val & distMask) << shift);
                    shift += this.logDistThreshold;
                    val = src[srcIdx++] & 0xFF;
                 }

                 distance |= (val << shift);
              }

              // Skip run
              while (unprocessedFlags[dstIdx] != 0)
                 dst[dstIdx++] = current;

              int idx = dstIdx;

              // Compute index
              while (distance > 0)
                 distance -= unprocessedFlags[++idx];

              // Output next occurence
              dst[idx] = current;
              unprocessedFlags[idx] = 0;
           }

           // Repeat last symbol if needed
           while (dstIdx < dst.length)
               dst[dstIdx++] = current;

           res = (srcIdx == end) ? true : false;
       }
       catch (ArrayIndexOutOfBoundsException e)
       {
          res = false;
       }

       source.index = srcIdx;
       destination.index = dstIdx;
       return res;
    }


    public boolean setSize(int size)
    {
        if (size < 0) // 0 is valid
            return false;

        this.size = size;
        return true;
    }


    // Not thread safe
    public int size()
    {
       return this.size;
    }
    
  
    // Required encoding output buffer size unknown
    @Override
    public int getMaxEncodedLength(int srcLen)
    {
       return -1;
    }
}

