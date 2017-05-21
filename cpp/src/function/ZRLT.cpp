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

#include <stddef.h>
#include "ZRLT.hpp"

using namespace kanzi;

bool ZRLT::forward(SliceArray<byte>& input, SliceArray<byte>& output, int length)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if (input._array == output._array)
        return false;

    byte* src = input._array;
    byte* dst = output._array;
    
    if (output._length - output._index < getMaxEncodedLength(length))
         return false;
     
    int srcIdx = input._index;
    int dstIdx = output._index;
    const int srcEnd = srcIdx + length;
    const int dstEnd = output._length;
    const int dstEnd2 = length - 2;
    int runLength = 1;

    if (dstIdx < dstEnd) {
       while (srcIdx < srcEnd) {
           int val = src[srcIdx];

           if (val == 0) {
               runLength++;
               srcIdx++;

               if ((srcIdx < length) && (runLength < ZRLT_MAX_RUN))
                   continue;
           }

           if (runLength > 1) {
               // Encode length
               int log2 = 1;

               for (int val2 = runLength >> 1; val2 > 1; val2 >>= 1)
                   log2++;

               if (dstIdx >= length - log2)
                   break;

               // Write every bit as a byte except the most significant one
               while (log2 > 0) {
                   log2--;
                   dst[dstIdx++] = byte((runLength >> log2) & 1);
               }

               runLength = 1;
               continue;
           }

           val &= 0xFF;

           if (val >= 0xFE) {
               if (dstIdx >= dstEnd2)
                   break;

               dst[dstIdx] = byte(0xFF);
               dstIdx++;
               dst[dstIdx] = byte(val - 0xFE);
           }
           else {
               if (dstIdx >= dstEnd)
                   break;

               dst[dstIdx] = byte(val + 1);
           }

           srcIdx++;
           dstIdx++;

           if (dstIdx >= dstEnd)
               break; 
       }
    }

    input._index = srcIdx;
    output._index = dstIdx;
    return (srcIdx == length) && (runLength == 1);
}

bool ZRLT::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int length)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if (input._array == output._array)
        return false;

    byte* src = input._array;
    byte* dst = output._array;     
    int srcIdx = input._index;
    int dstIdx = output._index;
    const int srcEnd = srcIdx + length;
    const int dstEnd = output._length;
    int runLength = 1;

    if (srcIdx < srcEnd) {
       while (dstIdx < dstEnd) {
           if (runLength > 1) {
               runLength--;
               dst[dstIdx++] = 0;
               continue;
           }

           if (srcIdx >= length)
               break;

           int val = src[srcIdx] & 0xFF;

           if (val <= 1) {
               // Generate the run length bit by bit (but force MSB)
               runLength = 1;

               do {
                   runLength = (runLength << 1) | val;
                   srcIdx++;

                   if (srcIdx >= length)
                       break;

                   val = src[srcIdx] & 0xFF;
               } while (val <= 1);

               continue;
           }
           
           // Regular data processing
           if (val == 0xFF) {
               srcIdx++;

               if (srcIdx >= length)
                   break;

               dst[dstIdx] = byte(0xFE + src[srcIdx]);
           } else {
               dst[dstIdx] = byte(val - 1);
           }

           srcIdx++;
           dstIdx++;
       }
    }

    // If runLength is not 1, add trailing 0s
    const int end = dstIdx + runLength - 1;
    input._index= srcIdx;
    output._index= dstIdx;

    if (end > dstEnd)
        return false;

    while (dstIdx < end)
        dst[dstIdx++] = 0;

    output._index = dstIdx;
    return srcIdx == srcEnd;
}
