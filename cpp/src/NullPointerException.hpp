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

#ifndef _NullPointerException_
#define _NullPointerException_

#include <string>
#include <stdexcept>
#include "Kanzi.hpp"

using namespace std;

namespace kanzi 
{

   class NullPointerException : public runtime_error
   {
      public:

        NullPointerException(const string& msg) : runtime_error(msg) {}

        virtual ~NullPointerException() _GLIBCXX_USE_NOEXCEPT {};
   };

}
#endif
