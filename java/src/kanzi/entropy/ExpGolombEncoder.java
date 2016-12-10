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

package kanzi.entropy;

import kanzi.EntropyEncoder;
import kanzi.OutputBitStream;


public final class ExpGolombEncoder implements EntropyEncoder
{
    private final boolean signed;
    private final OutputBitStream bitstream;
    
    
    public ExpGolombEncoder(OutputBitStream bitstream, boolean signed)
    {
       if (bitstream == null)
          throw new NullPointerException("Invalid null bitStream parameter");

       this.signed = signed;
       this.bitstream = bitstream;
    }
    
    
    public boolean isSigned()
    {
       return this.signed;
    }
       
    
    @Override
    public int encode(byte[] array, int blkptr, int len)
    {
      if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
         return -1;

      final int end = blkptr + len;

      for (int i = blkptr; i<end; i++)
         this.encodeByte(array[i]);

      return len;
    }
    

    public void encodeByte(byte val)
    {
       if (val == 0) 
       {
          // shortcut when input is 0
          this.bitstream.writeBit(1);
          return;
       }

       //  Take the abs() of 'val' add 1 to it
       int val2 = val;
       val2 = (val2 + (val2 >> 31)) ^ (val2 >> 31); // abs(val2)
       val2++;
       long emit = val2;
       int n;
       
       if (val2 <= 3) // shortcut when abs(input) = 1 or 2
       {
          n = 3;
       }
       else
       {
          // Count the bits (log2), subtract one, and write that number of zeros
          // preceding the previous bit string to get the encoded value
          int log2 = 2;
          
          for ( ; val2>=4; val2>>=1)
             log2++;

          //  val   val+1    exp-golomb
          //   0 =>  1    =>  1
          //   1 =>  10   =>  010
          //   2 =>  11   =>  011
          //   3 =>  100  =>  00100
          //   4 =>  101  =>  00101
          //   5 =>  110  =>  00110
          //   6 =>  111  =>  00111
          //   7 =>  1000 =>  0001000
          //   8 =>  1001 =>  0001001
          n = log2 + (log2 - 1);
       }

       if (this.signed == true)
       {
          // Add 0 for positive and 1 for negative sign
          n++;
          emit = (emit << 1) | (((int) val) >>> 31);
       }

       this.bitstream.writeBits(emit, n);
    }  


    @Override
    public OutputBitStream getBitStream()
    {
       return this.bitstream;
    }

    
    @Override
    public void dispose()
    {
    }
}
