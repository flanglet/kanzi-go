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

#ifndef _ROLZCodec_
#define _ROLZCodec_

#include "../Function.hpp"
#include "../Memory.hpp"
#include "../Predictor.hpp"

// Implementation of a Reduced Offset Lempel Ziv transform
// Code based on 'balz' by Ilya Muravyov
// More information about ROLZ at http://ezcodesample.com/rolz/rolz_article.html

namespace kanzi
{
	class ROLZPredictor : public Predictor
	{
	private:
		uint16* _p1;
		uint16* _p2;
		uint _logSize;
		int32 _size;
		int32 _c1;
		int32 _ctx;

	public:
		ROLZPredictor(uint logMaxSymbolSize);

		~ROLZPredictor() {
			delete[] _p1;
			delete[] _p2;
		};

		void reset();

		void update(int bit);

		int get();

		void setContext(byte ctx) { _ctx = uint8(ctx) << _logSize; }
	};


	class ROLZEncoder
	{
	private:
		static const uint64 TOP = 0x00FFFFFFFFFFFFFF;
		static const uint64 MASK_24_56 = 0x00FFFFFFFF000000;
		static const uint64 MASK_0_24 = 0x0000000000FFFFFF;
		static const uint64 MASK_0_32 = 0x00000000FFFFFFFF;

		Predictor* _predictors[2];
		Predictor* _predictor;
		byte* _buf;
		int& _idx;
		uint64 _low;
		uint64 _high;

	public:
		ROLZEncoder(Predictor* predictors[2], byte buf[], int& idx);

		~ROLZEncoder() {}

		void encodeByte(byte val);

		void encodeBit(int bit);

		void dispose();

      void setContext(int n) { _predictor = _predictors[n]; }
	};

	class ROLZDecoder
	{
	private:
		static const uint64 TOP = 0x00FFFFFFFFFFFFFF;
		static const uint64 MASK_24_56 = 0x00FFFFFFFF000000;
		static const uint64 MASK_0_56 = 0x00FFFFFFFFFFFFFF;
		static const uint64 MASK_0_32 = 0x00000000FFFFFFFF;

		Predictor* _predictors[2];
		Predictor* _predictor;
		byte* _buf;
		int& _idx;
		uint64 _low;
		uint64 _high;
		uint64 _current;


	public:
		ROLZDecoder(Predictor* predictors[2], byte buf[], int& idx);

		~ROLZDecoder() {}

		byte decodeByte();

		int decodeBit();

		void dispose() {}

        void setContext(int n) { _predictor = _predictors[n]; }
	};


	class ROLZCodec : public Function<byte>
	{
	public:
		ROLZCodec(uint logPosChecks = LOG_POS_CHECKS) THROW;

		~ROLZCodec() { delete[] _matches; }

		bool forward(SliceArray<byte>& src, SliceArray<byte>& dst, int length);

		bool inverse(SliceArray<byte>& src, SliceArray<byte>& dst, int length);

		// Required encoding output buffer size
		int getMaxEncodedLength(int srcLen) const { return srcLen * 5 / 4; }

	private:
		static const int HASH_SIZE = 1 << 16;
		static const int MIN_MATCH = 3;
		static const int MAX_MATCH = MIN_MATCH + 255;
		static const int LOG_POS_CHECKS = 5;
		static const int CHUNK_SIZE = 1 << 26;
		static const int LITERAL_FLAG = 0;
		static const int MATCH_FLAG = 1;
		static const int32 HASH = 200002979;
		static const int32 HASH_MASK = ~(CHUNK_SIZE - 1);

		int32* _matches;
		int32 _counters[65536];
		int _logPosChecks;
		int _maskChecks;
		int _posChecks;
		ROLZPredictor _litPredictor;
		ROLZPredictor _matchPredictor;


		int findMatch(const byte buf[], const int pos, const int end);

		uint16 getKey(const byte* p) {
			return uint16(LittleEndian::readInt16(p));
		}

		int32 hash(const byte* p) {
			return ((LittleEndian::readInt32(p) & 0x00FFFFFF) * HASH) & HASH_MASK;
		}
	};

}

#endif
