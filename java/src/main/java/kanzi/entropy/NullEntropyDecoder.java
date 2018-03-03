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

import kanzi.EntropyDecoder;
import kanzi.InputBitStream;
import kanzi.Memory;


// Null entropy encoder and decoder
// Pass through that writes the data directly to the bitstream
public final class NullEntropyDecoder implements EntropyDecoder
{
    private final InputBitStream bitstream;


    public NullEntropyDecoder(InputBitStream bitstream)
    {
       if (bitstream == null)
          throw new NullPointerException("Invalid null bitstream parameter");

       this.bitstream = bitstream;
    }
        
     
    @Override
    public int decode(byte[] block, int blkptr, int len)
    {
       if ((block == null) || (blkptr + len > block.length) || (blkptr < 0) || (len < 0))
          return -1;

       final int len8 = len & -8;
       final int end8 = blkptr + len8;
       int i = blkptr;

       while (i < end8)
       {
          Memory.BigEndian.writeLong64(block, i, this.bitstream.readBits(64));
          i += 8;
       }

       while (i < blkptr + len)
          block[i++] = (byte) this.bitstream.readBits(8);

       return len;
    }


    public  byte decodeByte()
    {
       return (byte) this.bitstream.readBits(8);
    }
    
    
    @Override
    public InputBitStream getBitStream()
    {
       return this.bitstream;
    }


    @Override
    public void dispose() 
    {
    }
}
