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

import kanzi.Memory;

// MurmurHash3 was written by Austin Appleby, and is placed in the public
// domain. The author hereby disclaims copyright to this source code.
// Original source code: https://github.com/aappleby/smhasher

public class MurMurHash3
{
  private static final int C1 = 0xcc9e2d51;
  private static final int C2 = 0x1b873593;
  private static final int C3 = 0xe6546b64;
  private static final int C4 = 0x85ebca6b;
  private static final int C5 = 0xc2b2ae35;
  
  private int seed;

  
  public MurMurHash3()
  {
     this((int) System.nanoTime());
  }


  public MurMurHash3(int seed)
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
  
  
  public int hash(byte[] data, int offset, int length)
  {
     int h1 = this.seed; // aliasing
     int n = offset;

     // Body
     if (length >= 4)
     {
         final int end = offset + length - 4;
         
         for ( ; n<end; n+=4)
         {
            int k1 = Memory.LittleEndian.readInt32(data, n);
            k1 *= C1;
            k1 = (k1 << 15) | (k1 >>> 17);
            k1 *= C2; 
            h1 ^= k1;
            h1 = (h1 << 13) | (h1 >>> 19); 
            h1 = (h1*5) + C3;
         }
     }

     // Tail
     int k1 = 0;

     switch(length & 3)
     {
        case 3: 
           k1 ^= ((data[n+2] & 0xFF) << 16);
           // Fallthrough

        case 2: 
           k1 ^= ((data[n+1] & 0xFF) << 8);
           // Fallthrough

        case 1: 
           k1 ^= (data[n] & 0xFF);
           k1 *= C1;
           k1 = (k1 << 15) | (k1 >>> 17);
           k1 *= C2;
           h1 ^= k1;
           // Fallthrough
           
        default:
           // Fallthrough
      }

      // Finalization
      h1 ^= length;
      h1 ^= (h1 >>> 16);
      h1 *= C4;
      h1 ^= (h1 >>> 13);
      h1 *= C5;
      return h1 ^ (h1 >>> 16);
   }
}