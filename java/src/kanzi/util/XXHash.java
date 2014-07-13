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

package kanzi.util;

// XXHash is an extremely fast hash algorithm. It was written by Yann Collet.
// Port to Java of the original source code: https://code.google.com/p/xxhash/

public class XXHash
{
  private static final int PRIME1 = -1640531535;
  private static final int PRIME2 = -2048144777;
  private static final int PRIME3 = -1028477379;
  private static final int PRIME4 = 668265263;
  private static final int PRIME5 = 374761393;

  
  private int seed;

  
  public XXHash()
  {
     this((int) (System.nanoTime()));
  }


  public XXHash(int seed)
  {
     this.seed = seed;
  }

  
  public void setSeed(int seed)
  {
     this.seed = seed;
  }
  
  
  public int hash(byte[] data)
  {
     return this.hash(data, 0, data.length);
  }
  
  
  public int hash(byte[] data, int offset, int len)
  { 
     final int end = offset + len;
     int h32;
     int idx = offset;
 
     if (len >= 16) 
     {
        final int limit = end - 16;
        int v1 = this.seed + PRIME1 + PRIME2;
        int v2 = this.seed + PRIME2;
        int v3 = this.seed;
        int v4 = this.seed - PRIME1;
      
        do
        {
           v1 = pack(v1, data, idx);
           v2 = pack(v2, data, idx+4);
           v3 = pack(v3, data, idx+8);
           v4 = pack(v4, data, idx+12);
           idx += 16;
        } 
        while (idx <= limit);

        h32  = ((v1 << 1)  | (v1 >>> 31));
        h32 += ((v2 << 7)  | (v2 >>> 25));
        h32 += ((v3 << 12) | (v3 >>> 20));
        h32 += ((v4 << 18) | (v4 >>> 14));
      } 
      else 
      {
         h32 = this.seed + PRIME5;
      }

      h32 += len;

      while (idx <= end - 4) 
      {
         h32 += (((data[idx] & 0xFF) | ((data[idx+1] & 0xFF) << 8) | ((data[idx+2] & 0xFF) << 16) | 
                   ((data[idx+3] & 0xFF) << 24)) * PRIME3);
         h32 = ((h32 << 17) | (h32 >>> 15)) * PRIME4;
         idx += 4;
      }

      while (idx < end) 
      {
         h32 += ((data[idx] & 0xFF) * PRIME5);
         h32 = ((h32 << 11) | (h32 >>> 21)) * PRIME1;
         idx++;
      }

      h32 ^= (h32 >>> 15);
      h32 *= PRIME2;
      h32 ^= (h32 >>> 13);
      h32 *= PRIME3;
      return h32 ^ (h32 >>> 16);
   }
  
  
  private static int pack(int v, byte[] data, int idx)
  {
     v += ((data[idx] & 0xFF) | ((data[idx+1] & 0xFF) << 8) | ((data[idx+2] & 0xFF) << 16) | 
                  ((data[idx+3] & 0xFF) << 24) * PRIME2);
     return ((v << 13) | (v >>> 19)) * PRIME1;
  }
}