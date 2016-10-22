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

package kanzi.function;

import kanzi.ByteFunction;
import kanzi.IndexedByteArray;


public class NullFunction implements ByteFunction
{
   public NullFunction()
   {
   }

   
   @Override
   public boolean forward(IndexedByteArray input, IndexedByteArray output, int length)
   {
      return doCopy(input, output, length);
   }
  

   @Override
   public boolean inverse(IndexedByteArray input, IndexedByteArray output, int length)
   {    
      return doCopy(input, output, length);
   }

   
   private static boolean doCopy(IndexedByteArray input, IndexedByteArray output, final int len)
   {      
      if (input.index + len > input.array.length)
         return false;
      
      if (output.index + len > output.array.length)
         return false;

      if ((input.array != output.array) || (input.index != output.index))
         System.arraycopy(input.array, input.index, output.array, output.index, len);     
      
      input.index += len;
      output.index += len;
      return true;
   }
   
   
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      return srcLen;
   }   
}