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

#ifndef _Memory_
#define _Memory_

#include <cstring>
#include <emmintrin.h>
#include "types.hpp"

namespace kanzi {

   static inline uint32 bswap32(uint32 x) {
   #if defined(HAVE_BUILTIN_BSWAP32)
	   return __builtin_bswap32(x);
   #elif defined(_MSC_VER)
		return uint32(_byteswap_ulong(x));
   #elif defined(__i386__) || defined(__x86_64__)
		uint32 swapped_bytes;
		__asm__ volatile("bswap %0" : "=r"(swapped_bytes) : "0"(x));
		return swapped_bytes;
   #else // fallback
		return (x >> 24) | ((x >> 8) & 0xFF00) | ((x << 8) & 0xFF0000) | (x << 24);
   #endif
	}


   static inline uint16 bswap16(uint16 x) {
   #if defined(HAVE_BUILTIN_BSWAP16)
		return __builtin_bswap16(x);
   #elif defined(_MSC_VER)
		return _byteswap_ushort(x);
   #else // fallback
		return (x >> 8) | ((x & 0xFF) << 8);
   #endif
	}


   static inline uint64 bswap64(uint64 x) {
   #if defined(HAVE_BUILTIN_BSWAP64)
		return __builtin_bswap64(x);
   #elif defined(_MSC_VER)
		return uint64(_byteswap_uint64(x));
   #elif defined(__x86_64__)
		uint64 swapped_bytes;
		__asm__ volatile("bswapq %0" : "=r"(swapped_bytes) : "0"(x));
		return swapped_bytes;
   #else // fallback
		x = ((x & 0xFFFFFFFF00000000ull) >> 32) | ((x & 0x00000000FFFFFFFFull) << 32);
		x = ((x & 0xFFFF0000FFFF0000ull) >> 16) | ((x & 0x0000FFFF0000FFFFull) << 16);
		x = ((x & 0xFF00FF00FF00FF00ull) >> 8) | ((x & 0x00FF00FF00FF00FFull) << 8);
		return x;
   #endif
	}

   
   inline bool isBigEndian() {
		const union { uint32 u; byte c[4]; } one = { 1 };
		return one.c[3] == 1;
	}

	static const bool KANZI_BIG_ENDIAN = isBigEndian();

	#ifndef IS_BIG_ENDIAN
		#if defined(__BYTE_ORDER) && __BYTE_ORDER == __BIG_ENDIAN || \
			   defined(__BIG_ENDIAN__) || \
			   defined(__ARMEB__) || \
			   defined(__THUMBEB__) || \
			   defined(__AARCH64EB__) || \
			   defined(_MIBSEB) || defined(__MIBSEB) || defined(__MIBSEB__)
			#define IS_BIG_ENDIAN true
		#else
			//#define IS_BIG_ENDIAN (((const union { uint32 x; byte c[4]; }) {1}).c[3] != 0)
            #define IS_BIG_ENDIAN KANZI_BIG_ENDIAN
		#endif
	#endif

	class BigEndian {
	public:
		static int64 readLong64(const byte* p);
		static int32 readInt32(const byte* p);
		static int16 readInt16(const byte* p);

		static void writeLong64(byte* p, int64 val);
		static void writeInt32(byte* p, int32 val);
		static void writeInt16(byte* p, int16 val);
	};

	class LittleEndian {
	public:
		static int64 readLong64(const byte* p);
		static int32 readInt32(const byte* p);
		static int16 readInt16(const byte* p);

		static void writeLong64(byte* p, int64 val);
		static void writeInt32(byte* p, int32 val);
		static void writeInt16(byte* p, int16 val);
	};

	inline int64 BigEndian::readLong64(const byte* p)
	{
      int64 val;

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		val = *(const int64*)p;
   #else
		memcpy(&val, p, 8);
   #endif

   #if (!IS_BIG_ENDIAN)
      val = bswap64(val);
   #endif 
      return val;
	}


	inline int32 BigEndian::readInt32(const byte* p)
	{
      int32 val;

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		val = *(const int32*)p;
   #else
		memcpy(&val, p, 4);
   #endif

   #if (!IS_BIG_ENDIAN)
      val = bswap32(val);
   #endif 
      return val;
	}


	inline int16 BigEndian::readInt16(const byte* p)
	{
      int16 val;

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		val = *(const int16*)p;
   #else
		memcpy(&val, p, 2);
   #endif

   #if (!IS_BIG_ENDIAN)
      val = bswap16(val);
   #endif 
      return val;
	}


	inline void BigEndian::writeLong64(byte* p, int64 val)
	{
   #if (!IS_BIG_ENDIAN)
      val = bswap64(val);
   #endif 

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		*(int64*)p = val;
   #else
		memcpy(p, &val, 8);
   #endif
	}


	inline void BigEndian::writeInt32(byte* p, int32 val)
	{
  #if (!IS_BIG_ENDIAN)
      val = bswap32(val);
   #endif 

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		*(int32*)p = val;
   #else
		memcpy(p, &val, 4);
   #endif
	}


	inline void BigEndian::writeInt16(byte* p, int16 val)
	{
  #if (!IS_BIG_ENDIAN)
      val = bswap16  (val);
   #endif 

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		*(int16*)p = val;
   #else
		memcpy(p, &val, 2);
   #endif
	}


	inline int64 LittleEndian::readLong64(const byte* p)
	{
      int64 val;

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		val = *(const int64*)p;
   #else
		memcpy(&val, p, 8);
   #endif

   #if (IS_BIG_ENDIAN)
      val = bswap64(val);
   #endif 
      return val;
	}


	inline int32 LittleEndian::readInt32(const byte* p)
	{
      int32 val;

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		val = *(const int32*)p;
   #else
		memcpy(&val, p, 4);
   #endif

   #if (IS_BIG_ENDIAN)
      val = bswap32(val);
   #endif 
      return val;
	}


	inline int16 LittleEndian::readInt16(const byte* p)
	{
      int16 val;

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		val = *(const int16*)p;
   #else
		memcpy(&val, p, 2);
   #endif

   #if (IS_BIG_ENDIAN)
      val = bswap16(val);
   #endif 
      return val;
	}


	inline void LittleEndian::writeLong64(byte* p, int64 val)
	{
   #if (IS_BIG_ENDIAN)
      val = bswap64(val);
   #endif 

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		*(int64*)p = val;
   #else
		memcpy(p, &val, 8);
   #endif
	}


	inline void LittleEndian::writeInt32(byte* p, int32 val)
	{
   #if (IS_BIG_ENDIAN)
      val = bswap32(val);
   #endif 

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		*(int32*)p = val;
   #else
		memcpy(p, &val, 4);
   #endif
	}


	inline void LittleEndian::writeInt16(byte* p, int16 val)
	{
   #if (IS_BIG_ENDIAN)
      val = bswap16(val);
   #endif 

   #ifdef AGGRESSIVE_OPTIMIZATION
      // !!! unaligned data
		*(int16*)p = val;
   #else
		memcpy(p, &val, 2);
   #endif
	}
}
#endif
