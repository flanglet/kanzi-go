/*
Copyright 2011-2013 Frederic Langlet
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

import kanzi.OutputBitStream;


public final class RiceGolombEncoder extends AbstractEncoder
{
    private final boolean signed;
    private final OutputBitStream bitstream;
    private final int logBase;
    private final int base;
    

    public RiceGolombEncoder(OutputBitStream bitstream, boolean signed, int logBase)
    {
        if (bitstream == null)
           throw new NullPointerException("Invalid null bitstream parameter");

        if ((logBase <= 0) || (logBase >= 8))
           throw new IllegalArgumentException("Invalid logBase value (must be in [1..7])");

        this.signed = signed;
        this.bitstream = bitstream;
        this.logBase = logBase;
        this.base = 1 << this.logBase;
    }


    public boolean isSigned()
    {
       return this.signed;
    }


    @Override
    public void encodeByte(byte val)
    {
       if (val == 0)
       {
          this.bitstream.writeBits(this.base, this.logBase+1);
          return;
       }

       int val2 = val;
       val2 = (val2 + (val2 >> 31)) ^ (val2 >> 31); // abs(val2)

        // quotient is unary encoded, remainder is binary encoded
       int emit = this.base | (val2 & (this.base-1));
       int n = (1 + (val2 >> this.logBase)) + this.logBase;

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
}
