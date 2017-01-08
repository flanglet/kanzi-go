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

#include <sstream>
#include "ANSRangeDecoder.hpp"
#include "../IllegalArgumentException.hpp"
#include "EntropyUtils.hpp"

using namespace kanzi;


// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block.
// The default chunk size is 65536 bytes.
ANSRangeDecoder::ANSRangeDecoder(InputBitStream& bitstream, int chunkSize) THROW : _bitstream(bitstream)
{
	if ((chunkSize != 0) && (chunkSize < 1024))
		throw IllegalArgumentException("The chunk size must be at least 1024");

	if (chunkSize > 1 << 30)
		throw IllegalArgumentException("The chunk size must be at most 2^30");

	_chunkSize = chunkSize;
	_f2sSize = 0;
	_f2s = new short[_f2sSize];
}

int ANSRangeDecoder::decodeHeader(uint frequencies[])
{
	int alphabetSize = EntropyUtils::decodeAlphabet(_bitstream, _alphabet);

	if (alphabetSize == 0)
		return 0;

	if (alphabetSize != 256) {
		memset(frequencies, 0, sizeof(uint) * 256);
	}

	_logRange = (uint)(8 + _bitstream.readBits(3));
	const int scale = 1 << _logRange;
	int sum = 0;
	const int inc = (alphabetSize > 64) ? 16 : 8;
	int llr = 3;

	while (uint(1 << llr) <= _logRange)
		llr++;

	// Decode all frequencies (but the first one) by chunks of size 'inc'
	for (int i = 1; i < alphabetSize; i += inc) {
		const int logMax = (int)(1 + _bitstream.readBits(llr));
		const int endj = (i + inc < alphabetSize) ? i + inc : alphabetSize;

		// Read frequencies
		for (int j = i; j < endj; j++) {
			int val = (int)_bitstream.readBits(logMax);

			if ((val <= 0) || (val >= scale)) {
				stringstream ss;
				ss << "Invalid bitstream: incorrect frequency " << val;
				ss << " for symbol '" << _alphabet[j] << "' in range decoder";
				throw BitStreamException(ss.str(),
					BitStreamException::INVALID_STREAM);
			}

			frequencies[_alphabet[j]] = (uint)val;
			sum += val;
		}
	}

	// Infer first frequency
	if (scale <= sum) {
		stringstream ss;
		ss << "Invalid bitstream: incorrect frequency " << frequencies[_alphabet[0]];
		ss << " for symbol '" << _alphabet[0] << "' in range decoder";
		throw BitStreamException(ss.str(),
			BitStreamException::INVALID_STREAM);
	}

	frequencies[_alphabet[0]] = uint(scale - sum);
	_cumFreqs[0] = 0;

	if (_f2sSize < scale) {
		delete[] _f2s;
		_f2sSize = scale;
		_f2s = new short[_f2sSize];
	}

	// Create histogram of frequencies scaled to 'range' and reverse mapping
	for (int i = 0; i < 256; i++) {
		_cumFreqs[i + 1] = _cumFreqs[i] + frequencies[i];
		const int base = (int)_cumFreqs[i];

		for (int j = frequencies[i] - 1; j >= 0; j--)
			_f2s[base + j] = (short)i;
	}

	return alphabetSize;
}

int ANSRangeDecoder::decode(byte block[], uint blkptr, uint len)
{
	if (len == 0)
		return 0;

	const int end = blkptr + len;
	const int sz = (_chunkSize == 0) ? len : _chunkSize;
	int startChunk = blkptr;

	while (startChunk < end) {
		if (decodeHeader(_freqs) == 0)
			return startChunk - blkptr;

		const int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
		decodeChunk(block, startChunk, endChunk);
		startChunk = endChunk;
	}

	return len;
}

void ANSRangeDecoder::decodeChunk(byte block[], int start, int end)
{
	// Read initial ANS state
	uint64 st = _bitstream.readBits(64);
	const uint64 mask = (1 << _logRange) - 1;

	for (int i = start; i<end; i++)
	{
		const int idx = (int)(st & mask);
		const int symbol = _f2s[idx];
		block[i] = (byte)symbol;

		// Compute next ANS state
		// D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
		st = (_freqs[symbol] * (st >> _logRange)) + idx - _cumFreqs[symbol];

		// Normalize
		while (st < ANS_TOP)
			st = (st << 32) | _bitstream.readBits(32);
	}
}