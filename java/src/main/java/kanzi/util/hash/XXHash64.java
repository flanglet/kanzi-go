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

package kanzi.util.hash;

// XXHash is an extremely fast hash algorithm. It was written by Yann Collet.
// Port to Java of the original source code: https://github.com/Cyan4973/xxHash

import kanzi.Memory;


public class XXHash64
{
  private final static long PRIME64_1 = 0x9E3779B185EBCA87L;
  private final static long PRIME64_2 = 0xC2B2AE3D27D4EB4FL;
  private final static long PRIME64_3 = 0x165667B19E3779F9L;
  private final static long PRIME64_4 = 0x85EBCA77C2b2AE63L;
  private final static long PRIME64_5 = 0x27D4EB2F165667C5L;


  private long seed;


  public XXHash64()
  {
     this(System.nanoTime());
  }


  public XXHash64(long seed)
  {
     this.seed = seed;
  }


  public void setSeed(long seed)
  {
     this.seed = seed;
  }


  public long hash(byte[] data)
  {
     return this.hash(data, 0, data.length);
  }


  public long hash(byte[] data, int offset, int length)
  {
     final int end = offset + length;
     long h64;
     int idx = offset;

     if (length >= 32)
     {
        final int end32 = end - 32;
        long v1 = this.seed + PRIME64_1 + PRIME64_2;
        long v2 = this.seed + PRIME64_2;
        long v3 = this.seed;
        long v4 = this.seed - PRIME64_1;

        do
        {
           v1 = round(v1, Memory.LittleEndian.readLong64(data, idx));
           v2 = round(v2, Memory.LittleEndian.readLong64(data, idx+8));
           v3 = round(v3, Memory.LittleEndian.readLong64(data, idx+16));
           v4 = round(v4, Memory.LittleEndian.readLong64(data, idx+24));
           idx += 32;
        }
        while (idx <= end32);

        h64  = ((v1 << 1)  | (v1 >>> 31)) + ((v2 << 7)  | (v2 >>> 25)) +
               ((v3 << 12) | (v3 >>> 20)) + ((v4 << 18) | (v4 >>> 14));

        h64 = mergeRound(h64, v1);
        h64 = mergeRound(h64, v2);
        h64 = mergeRound(h64, v3);
        h64 = mergeRound(h64, v4);
      }
      else
      {
         h64 = this.seed + PRIME64_5;
      }

      h64 += length;

      while (idx+8 <= end)
      {
         h64 ^= round(0, Memory.LittleEndian.readLong64(data, idx));
         h64 = ((h64 << 27) | (h64 >>> 37)) * PRIME64_1 + PRIME64_4;
         idx += 8;
      }

      while (idx+4 <= end)
      {
         h64 ^= (Memory.LittleEndian.readInt32(data, idx) * PRIME64_1);
         h64 = ((h64 << 23) | (h64 >>> 41)) * PRIME64_2 + PRIME64_3;
         idx += 4;
      }

      while (idx < end)
      {
         h64 ^= ((data[idx] & 0xFF) * PRIME64_5);
         h64 = ((h64 << 11) | (h64 >>> 53)) * PRIME64_1;
         idx++;
      }

      // Finalize
      h64 ^= (h64 >>> 33);
      h64 *= PRIME64_2;
      h64 ^= (h64 >>> 29);
      h64 *= PRIME64_3;
      return h64 ^ (h64 >>> 32);
   }

  
   private static long round(long acc, long val)
   {
      acc += (val*PRIME64_2);
      return ((acc << 31) | (acc >>> 33)) * PRIME64_1;
   }


   private static long mergeRound(long acc, long val)
   {
      acc ^= round(0, val);
      return acc*PRIME64_1 + PRIME64_4;
   }
}