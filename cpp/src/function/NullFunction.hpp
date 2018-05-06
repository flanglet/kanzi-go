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

#ifndef _NullFunction_
#define _NullFunction_

#include "../Function.hpp"

namespace kanzi
{

   template <class T>
   class NullFunction : public Function<T> {
   public:
       NullFunction() {}

       ~NullFunction() {}

       bool forward(SliceArray<T>& input, SliceArray<T>& output, int length) { return doCopy(input, output, length); }

       bool inverse(SliceArray<T>& input, SliceArray<T>& output, int length) { return doCopy(input, output, length); }

       // Required encoding output buffer size
       int getMaxEncodedLength(int inputLen) const { return inputLen; }

   private:
       static bool doCopy(SliceArray<T>& input, SliceArray<T>& output, int length);

   };

   template <class T>
   bool NullFunction<T>::doCopy(SliceArray<T>& input, SliceArray<T>& output, int length)
   {
       if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
          return false;

       if (input._index + length > input._length)
           return false;

       if (output._index + length > output._length)
           return false;

       if ((&input._array[0] != &output._array[0]) || (input._index != output._index))
           memmove(&output._array[output._index], &input._array[input._index], length);

       input._index += length;
       output._index += length;
       return true;
   }

}
#endif