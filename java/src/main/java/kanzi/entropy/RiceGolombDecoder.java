/*
Copyright 2011-2017 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
you may obtain a copy of the License at

                http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either Riceress or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kanzi.entropy;

import kanzi.EntropyDecoder;
import kanzi.InputBitStream;


// Rice-Golomb Coder
public final class RiceGolombDecoder implements EntropyDecoder
{
    private final boolean signed;
    private final InputBitStream bitstream;
    private final int logBase;

    
    public RiceGolombDecoder(InputBitStream bitstream, boolean signed, int logBase)
    {
        if (bitstream == null)
           throw new NullPointerException("Invalid null bitstream parameter");

        if ((logBase < 1) || (logBase > 12))
           throw new IllegalArgumentException("Invalid logBase value (must be in [1..12])");

        this.signed = signed;
        this.bitstream = bitstream;
        this.logBase = logBase;
    }


    public boolean isSigned()
    {
        return this.signed;
    }


    public byte decodeByte()
    {
       long q = 0;

       // quotient is unary encoded
       while (this.bitstream.readBit() == 0)
          q++;

       // remainder is binary encoded
       final long res = (q << this.logBase) | this.bitstream.readBits(this.logBase);

       if ((res != 0) && (this.signed == true))
       {
          if (this.bitstream.readBit() == 1)
             return (byte) -res;
       }

       return (byte) res;
    }


    @Override
    public InputBitStream getBitStream()
    {
       return this.bitstream;
    }

    
    @Override
    public int decode(byte[] array, int blkptr, int len) 
    {
      if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
         return -1;

      final int end = blkptr + len;

      for (int i=blkptr; i<end; i++)
         array[i] = this.decodeByte();

      return len;
    }


    @Override
    public void dispose() 
    {
    }
}
