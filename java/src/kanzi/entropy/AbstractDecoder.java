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

package kanzi.entropy;

import kanzi.EntropyDecoder;
import kanzi.BitStreamException;
import kanzi.InputBitStream;


public abstract class AbstractDecoder implements EntropyDecoder
{
   @Override
   public abstract byte decodeByte();
   
   
   @Override
   public abstract InputBitStream getBitStream();

   
   // Default implementation: fallback to decodeByte
   @Override
   public int decode(byte[] array, int blkptr, int len)
   {
      if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
         return -1;

      int end = blkptr + len;
      int i = blkptr;

      try
      {
         while (i < end)
            array[i++] = this.decodeByte();
      }
      catch (BitStreamException e)
      {
         return i - blkptr;
      }

      return len;
   }


   @Override
   public void dispose()
   {
   }

}
