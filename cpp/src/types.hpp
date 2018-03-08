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
#include <stdlib.h>
#include <emmintrin.h>


	#ifdef _MSC_VER
		#if !defined(__x86_64__)
			#define __x86_64__  _M_X64
		#endif
		#if !defined(__i386__)
			#define __i386__  _M_IX86 
		#endif
	#endif

   /*
   MSVC++ 14.1 _MSC_VER == 1912 (Visual Studio 2017)
   MSVC++ 14.1 _MSC_VER == 1911 (Visual Studio 2017)
   MSVC++ 14.1 _MSC_VER == 1910 (Visual Studio 2017)
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
	#else
	   #if defined(_MSC_VER) && _MSC_VER < 1300
	      typedef signed char int8_t;
	      typedef signed short int16_t;
	      typedef signed int int32_t;
	      typedef unsigned char uint8_t;
	      typedef unsigned short uint16_t;
	      typedef unsigned int uint32_t;
	   #else
	      typedef signed __int8 int8_t;
	      typedef signed __int16 int16_t;
	      typedef signed __int32 int32_t;
	      typedef unsigned __int8 uint8_t;
	      typedef unsigned __int16 uint16_t;
	      typedef unsigned __int32 uint32_t;
	   #endif

	   typedef signed __int64 int64_t;
	   typedef unsigned __int64 uint64_t;
	   #define nullptr NULL
	#endif

	typedef int8_t byte;
	typedef uint8_t uint8;
	typedef int16_t int16;
	typedef int32_t int32;
	typedef int64_t int64;
	typedef uint16_t uint16;
	typedef uint32_t uint;
	typedef uint32_t uint32;
	typedef uint64_t uint64;


   #if defined(WIN32) || defined(_WIN32) 
      #define PATH_SEPARATOR '\\' 
   #else 
      #define PATH_SEPARATOR '/' 
   #endif


   #if defined(_MSC_VER)
      #define ALIGNED_(x) __declspec(align(x))
   #else
      #if defined(__GNUC__)
         #define ALIGNED_(x) __attribute__ ((aligned(x)))
      #endif
   #endif

#endif