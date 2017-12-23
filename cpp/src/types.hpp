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
	#else
	   #if (_MSC_VER < 1300)
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



	#if !defined(WORDS_BIGENDIAN) && \
		(defined(__BIG_ENDIAN__) || defined(_M_PPC) || \
		 (defined(__BYTE_ORDER__) && (__BYTE_ORDER__ == __ORDER_BIG_ENDIAN__)))
		#define WORDS_BIGENDIAN
	#endif

	static inline uint32 bswap32(uint32 x) {
	#if defined(HAVE_BUILTIN_BSWAP32)
		return __builtin_bswap32(x);
    #elif defined(_MSC_VER)
		return (uint32) _byteswap_ulong(x);
    #elif defined(__i386__) || defined(__x86_64__)
		uint32_t swapped_bytes;
		__asm__ volatile("bswap %0" : "=r"(swapped_bytes) : "0"(x));
		return swapped_bytes;
	#else
		return (x >> 24) | ((x >> 8) & 0xFF00) | ((x << 8) & 0xFF0000) | (x << 24);
	#endif 
	}

	static inline uint16 bswap16(uint16 x) {
	#if defined(HAVE_BUILTIN_BSWAP16)
		return __builtin_bswap16(x);
    #elif defined(_MSC_VER)
		return _byteswap_ushort(x);
	#else
		return (x >> 8) | ((x & 0xFF) << 8); 
	#endif
	}

	static inline uint64 bswap64(uint64 x) {
	#if defined(HAVE_BUILTIN_BSWAP64)
			return __builtin_bswap64(x);
	#elif defined(__x86_64__)
			uint64 swapped_bytes;
			__asm__ volatile("bswapq %0" : "=r"(swapped_bytes) : "0"(x));
			return swapped_bytes;
	#elif defined(_MSC_VER)
			return (uint64) _byteswap_uint64(x);
	#else 
			x = ((x & 0xffffffff00000000ull) >> 32) | ((x & 0x00000000ffffffffull) << 32);
			x = ((x & 0xffff0000ffff0000ull) >> 16) | ((x & 0x0000ffff0000ffffull) << 16);
			x = ((x & 0xff00ff00ff00ff00ull) >> 8) | ((x & 0x00ff00ff00ff00ffull) << 8);
			return x;
	#endif 
	}

#endif