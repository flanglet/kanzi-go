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

import kanzi.BitStreamException;
import kanzi.EntropyEncoder;
import kanzi.OutputBitStream;


// Null entropy encoder and decoder
// Pass through that writes the data directly to the bitstream
// Helpful to debug
public final class NullEntropyEncoder implements EntropyEncoder
{
    private final OutputBitStream bitstream;


    public NullEntropyEncoder(OutputBitStream bitstream)
    {
       if (bitstream == null)
          throw new NullPointerException("Invalid null bitstream parameter");

       this.bitstream = bitstream;
    }


    @Override
    public int encode(byte[] array, int blkptr, int len)
    {
        if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
           return -1;

        final int len8 = len & -8;
        final int end8 = blkptr + len8;
        int i = blkptr;

        try
        {
           while (i < end8)
           {            
              if (this.encodeLong(array, i) == false)
                 return i;

              i += 8;
           }
           
           while (i < blkptr + len)
           {            
              this.bitstream.writeBits(array[i], 8);
              i++;
           }
        }
        catch (BitStreamException e)
        {
           return i - blkptr;
        }

        return len;
    }

    
    private boolean encodeLong(byte[] array, int offset)
    {
        long val;
        val  =  (long) (array[offset]   & 0xFF) << 56;
        val |= ((long) (array[offset+1] & 0xFF) << 48);
        val |= ((long) (array[offset+2] & 0xFF) << 40);
        val |= ((long) (array[offset+3] & 0xFF) << 32);
        val |= ((long) (array[offset+4] & 0xFF) << 24);
        val |= ((long) (array[offset+5] & 0xFF) << 16);
        val |= ((long) (array[offset+6] & 0xFF) << 8);
        val |=  (long) (array[offset+7] & 0xFF);
        return (this.bitstream.writeBits(val, 64) == 64);
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
