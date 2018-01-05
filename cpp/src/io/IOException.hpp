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

#ifndef _IOException_
#define _IOException_

#include <string>
#include <stdexcept>
#include "../util.hpp"
#include "../Error.hpp"
#include "../Global.hpp"

using namespace std;

namespace kanzi 
{

   class IOException : public runtime_error 
   {
   private:
       int _code;

   public:
       IOException(const string& msg) : runtime_error(msg+". Error code: "+to_string(Error::ERR_UNKNOWN))
       {
           _code = Error::ERR_UNKNOWN;
       }

       IOException(const string& msg, int error) : runtime_error(msg+". Error code: "+to_string(error))
       {
           _code = error;
       }
        
       int error() const { return _code; }

       virtual ~IOException() _GLIBCXX_USE_NOEXCEPT{};
   };

}
#endif
