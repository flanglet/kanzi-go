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

#ifndef _types_
#define _types_

#include <cstddef>

/*
MSVC++ 14.0 _MSC_VER == 1900 (Visual Studio 2015)
MSVC++ 12.0 _MSC_VER == 1800 (Visual Studio 2013)
MSVC++ 11.0 _MSC_VER == 1700 (Visual Studio 2012)
MSVC++ 10.0 _MSC_VER == 1600 (Visual Studio 2010)
MSVC++ 9.0  _MSC_VER == 1500 (Visual Studio 2008)
MSVC++ 8.0  _MSC_VER == 1400 (Visual Studio 2005)
MSVC++ 7.1  _MSC_VER == 1310 (Visual Studio 2003)
MSVC++ 7.0  _MSC_VER == 1300
MSVC++ 6.0  _MSC_VER == 1200
MSVC++ 5.0  _MSC_VER == 1100
*/

#ifndef _GLIBCXX_USE_NOEXCEPT
#define _GLIBCXX_USE_NOEXCEPT
//#define _GLIBCXX_USE_NOEXCEPT throw()
#endif

#ifndef THROW
   #ifdef __GNUG__
      #define THROW
   #else
      #define THROW throw(...)
   #endif
#endif

#if __cplusplus >= 201103L
   // C++ 11
   #include <cstdint>
   typedef uint8_t byte;
   typedef int32_t int32;
   typedef int64_t int64;
   typedef uint32_t uint;
   typedef uint32_t uint32;
   typedef uint64_t uint64;
#else
   #define nullptr NULL
   typedef char byte;
   typedef int int32;
   typedef unsigned int uint;
   typedef unsigned int uint32;
   typedef long long int64;
   typedef unsigned long long uint64;
#endif

#endif
