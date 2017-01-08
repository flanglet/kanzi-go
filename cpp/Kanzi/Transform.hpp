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

#ifndef _Transform_
#define _Transform_

#include "types.hpp"
#include "SliceArray.hpp"

namespace kanzi 
{

   template <class T>
   class Transform 
   {
   public:
       virtual bool forward(SliceArray<T>& src, SliceArray<T>& dst, int length) = 0;

       virtual bool inverse(SliceArray<T>& src, SliceArray<T>& dst, int length) = 0;

       virtual ~Transform(){};
   };

}
#endif