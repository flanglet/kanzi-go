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

package kanzi.function;

import kanzi.ByteFunction;
import kanzi.SliceByteArray;


public class NullFunction implements ByteFunction
{
   public NullFunction()
   {
   }

   
   @Override
   public boolean forward(SliceByteArray input, SliceByteArray output)
   {
      return doCopy(input, output);
   }
  

   @Override
   public boolean inverse(SliceByteArray input, SliceByteArray output)
   {    
      return doCopy(input, output);
   }

   
   private static boolean doCopy(SliceByteArray input, SliceByteArray output)
   {      
      if ((!SliceByteArray.isValid(input)) || (!SliceByteArray.isValid(output)))
         return false;
 
      final int count = input.length;
      
      if (output.length - output.index < count)
         return false;

      if ((input.array != output.array) || (input.index != output.index))
         System.arraycopy(input.array, input.index, output.array, output.index, count);     
      
      input.index += count;
      output.index += count;
      return true;
   }
   
   
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      return srcLen;
   }   
}