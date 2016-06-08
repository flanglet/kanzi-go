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
   public boolean forward(IndexedByteArray source, IndexedByteArray destination, int length)
   {
      return doCopy(source, destination, length);
   }
  

   @Override
   public boolean inverse(IndexedByteArray source, IndexedByteArray destination, int length)
   {    
      return doCopy(source, destination, length);
   }

   
   private static boolean doCopy(IndexedByteArray source, IndexedByteArray destination, int sz)
   {      
      final int len = (sz == 0) ? source.array.length : sz;

      if (source.index + len > source.array.length)
         return false;
      
      if (destination.index + len > destination.array.length)
         return false;

      if (source.array == destination.array)
      {
         if (source.index == destination.index)
         {
            source.index += len;
            destination.index += len;
            return true; 
         }
      }
      
      System.arraycopy(source.array, source.index, destination.array, destination.index, len);     
      source.index += len;
      destination.index += len;
      return true;
   }
   
   
   @Override
   public int getMaxEncodedLength(int srcLen)
   {
      return srcLen;
   }   
}