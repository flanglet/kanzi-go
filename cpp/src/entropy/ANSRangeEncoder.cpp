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
#include "ANSRangeEncoder.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block.
// The default chunk size is 65536 bytes.
ANSRangeEncoder::ANSRangeEncoder(OutputBitStream& bitstream, int chunkSize, int logRange) THROW : _bitstream(bitstream)
{
	if ((chunkSize != 0) && (chunkSize < 1024))
		throw IllegalArgumentException("The chunk size must be at least 1024");

	if (chunkSize > 1 << 30)
		throw IllegalArgumentException("The chunk size must be at most 2^30");

	if ((logRange < 8) || (logRange > 16)) {
		stringstream ss;
		ss << "Invalid range parameter: " << logRange << " (must be in [8..16])";
		throw IllegalArgumentException(ss.str());
	}

	_logRange = logRange;
	_chunkSize = chunkSize;
	_buffer = new int[0];
	_bufferSize = 0;
}

int ANSRangeEncoder::updateFrequencies(uint frequencies[], int size, int lr)
{
	int alphabetSize = _eu.normalizeFrequencies(frequencies, _alphabet, 256, size, 1 << lr);

	if (alphabetSize > 0) {
		_cumFreqs[0] = 0;

		// Create histogram of frequencies scaled to 'range'
		for (int i = 0; i < 256; i++)
			_cumFreqs[i + 1] = _cumFreqs[i] + frequencies[i];
	}

	encodeHeader(alphabetSize, _alphabet, frequencies, lr);
	return alphabetSize;
}

bool ANSRangeEncoder::encodeHeader(int alphabetSize, uint alphabet[], uint frequencies[], int lr)
{
	EntropyUtils::encodeAlphabet(_bitstream, alphabet, 256, alphabetSize);

	if (alphabetSize == 0)
		return true;

	_bitstream.writeBits(lr - 8, 3); // logRange
	const int inc = (alphabetSize > 64) ? 16 : 8;
	int llr = 3;

	while (1 << llr <= lr)
		llr++;

	// Encode all frequencies (but the first one) by chunks of size 'inc'
	for (int i = 1; i < alphabetSize; i += inc) {
		uint max = 0;
		uint logMax = 1;
		int endj = (i + inc < alphabetSize) ? i + inc : alphabetSize;

		// Search for max frequency log size in next chunk
		for (int j = i; j < endj; j++) {
			if (frequencies[alphabet[j]] > max)
				max = frequencies[alphabet[j]];
		}

		while (uint(1 << logMax) <= max)
			logMax++;

		_bitstream.writeBits(logMax - 1, llr);

		// Write frequencies
		for (int j = i; j < endj; j++)
			_bitstream.writeBits(frequencies[alphabet[j]], logMax);
	}

	return true;
}

// Reset frequency stats for each chunk of data in the block
int ANSRangeEncoder::encode(byte block[], uint blkptr, uint len)
{
	if (len == 0)
		return 0;

	const int end = blkptr + len;
	const int sz = (_chunkSize == 0) ? len : _chunkSize;
	int startChunk = blkptr;

	if (4 * _bufferSize < uint(sz + 3)) {
		delete[] _buffer;
		_bufferSize = (sz + 3) >> 2;
		_buffer = new int[_bufferSize];
	}

	while (startChunk < end) {
		const int endChunk = (startChunk + sz < end) ? startChunk + sz : end;
		int lr = _logRange;

		// Lower log range if the size of the data chunk is small
		while ((lr > 8) && (1 << lr > endChunk - startChunk))
			lr--;

		if (rebuildStatistics(block, startChunk, endChunk, lr) < 0)
			return startChunk;

		encodeChunk(block, startChunk, endChunk, lr);
		startChunk = endChunk;
	}

	return len;
}

void ANSRangeEncoder::encodeChunk(byte block[], int start, int end, int lr)
{
	const uint64 top = (ANS_TOP >> lr) << 32;
	uint64 st = ANS_TOP;
	int n = 0;

	// Encoding works in reverse
	for (int i = end - 1; i >= start; i--) {
		const int symbol = block[i] & 0xFF;
		const int freq = _freqs[symbol];

		// Normalize
		if (st >= top * freq) {
			_buffer[n++] = (int)st;
			st >>= 32;
		}

		// Compute next ANS state
		// C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
		st = (uint64(st / freq) << lr) + (st % freq) + _cumFreqs[symbol];
	}

	// Write final ANS state
	_bitstream.writeBits(st, 64);

	// Write encoded data to bitstream
	for (n--; n >= 0; n--)
		_bitstream.writeBits(_buffer[n], 32);
}

// Compute chunk frequencies, cumulated frequencies and encode chunk header
int ANSRangeEncoder::rebuildStatistics(byte block[], int start, int end, int lr)
{
	memset(_freqs, 0, sizeof(_freqs));

	for (int i = start; i < end; i++)
		_freqs[block[i] & 0xFF]++;

	// Rebuild statistics
	return updateFrequencies(_freqs, end - start, lr);
}