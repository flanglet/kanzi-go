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

#ifndef _BitStreamException_
#define _BitStreamException_

#include <string>
#include <stdexcept>
#include "types.hpp"
#include "util.hpp"

using namespace std;

namespace kanzi 
{

   class BitStreamException : public runtime_error 
   {
   private:
       int _code;

   public:
       static const int UNDEFINED = 0;
       static const int INPUT_OUTPUT = 1;
       static const int END_OF_STREAM = 2;
       static const int INVALID_STREAM = 3;
       static const int STREAM_CLOSED = 4;

       BitStreamException(const string& msg) : runtime_error(msg+". Error code: "+to_string(UNDEFINED))
       {
           _code = UNDEFINED;
       }

       BitStreamException(const string& msg, int code) : runtime_error(string(msg)+". Error code: "+to_string(code))
       {
           _code = code;
       }

       int error() const { return _code; }

       virtual ~BitStreamException() _GLIBCXX_USE_NOEXCEPT {};
   };

}
#endif
