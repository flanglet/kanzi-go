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

#include "DefaultOutputBitStream.hpp"
#include "../IllegalArgumentException.hpp"
#include "../Memory.hpp"
#include "../io/IOException.hpp"
#include <fstream>

using namespace kanzi;

DefaultOutputBitStream::DefaultOutputBitStream(OutputStream& os, uint bufferSize) THROW : _os(os)
{
    if (bufferSize < 1024)
        throw IllegalArgumentException("Invalid buffer size (must be at least 1024)");

    if (bufferSize > 1 << 29)
        throw IllegalArgumentException("Invalid buffer size (must be at most 536870912)");

    if ((bufferSize & 7) != 0)
        throw IllegalArgumentException("Invalid buffer size (must be a multiple of 8)");

    _bitIndex = 63;
    _bufferSize = bufferSize;
    _buffer = new byte[_bufferSize];
    _position = 0;
    _current = 0;
    _written = 0;
    _closed = false;
}

// Write least significant bit of the input integer. Trigger exception if stream is closed
inline void DefaultOutputBitStream::writeBit(int bit) THROW
{
    if (_bitIndex <= 0) // bitIndex = -1 if stream is closed => force pushCurrent()
    {
        _current |= (bit & 1);
        pushCurrent();
    }
    else {
        _current |= (uint64(bit & 1) << _bitIndex);
        _bitIndex--;
    }
}

// Write 'count' (in [1..64]) bits. Trigger exception if stream is closed
int DefaultOutputBitStream::writeBits(uint64 value, uint count) THROW
{
    if (count == 0)
        return 0;

    if (count > 64)
        throw BitStreamException("Invalid count: " + to_string(count) + " (must be in [1..64])");

    value &= (uint64(-1) >> (64 - count));
    uint bi = _bitIndex + 1;

    if (count < bi) {
        // Enough spots available in 'current'
        uint remaining = bi - count;
        _current |= (value << remaining);
        _bitIndex -= count;
    }
    else {
        uint remaining = count - bi;
        _current |= (value >> remaining);
        pushCurrent();

        if (remaining != 0) {
            _current = value << (64 - remaining);
            _bitIndex -= remaining;
        }
    }

    return count;
}

void DefaultOutputBitStream::close() THROW
{
    if (isClosed() == true)
        return;

    int savedBitIndex = _bitIndex;
    uint savedPosition = _position;
    uint64 savedCurrent = _current;

    try {
        // Push last bytes (the very last byte may be incomplete)
        int size = ((63 - _bitIndex) + 7) >> 3;
        pushCurrent();
        _position -= (8 - size);
        flush();
    }
    catch (BitStreamException& e) {
        // Revert fields to allow subsequent attempts in case of transient failure
        _position = savedPosition;
        _bitIndex = savedBitIndex;
        _current = savedCurrent;
        throw e;
    }

    try {
        _os.flush();
	
		if (!_os.good())
			throw BitStreamException("Write to bitstream failed.", BitStreamException::INPUT_OUTPUT);
	}
    catch (ios_base::failure& e) {
        throw BitStreamException(e.what(), BitStreamException::INPUT_OUTPUT);
    }

    _closed = true;
    _position = 0;

    // Reset fields to force a flush() and trigger an exception
    // on writeBit() or writeBits()
    _bitIndex = -1;
    delete[] _buffer;
    _bufferSize = 8;
    _buffer = new byte[_bufferSize];
    _written -= 64; // adjust for method written()
}

// Push 64 bits of current value into buffer.
inline void DefaultOutputBitStream::pushCurrent() THROW
{
    BigEndian::writeLong64(&_buffer[_position], _current);   
    _bitIndex = 63;
    _current = 0;
    _position += 8;

    if (_position >= _bufferSize)
        flush();
}

// Write buffer to underlying stream
void DefaultOutputBitStream::flush() THROW
{
    if (isClosed() == true)
        throw BitStreamException("Stream closed", BitStreamException::STREAM_CLOSED);

    try {
        if (_position > 0) {
            _os.write(reinterpret_cast<char*>(_buffer), _position);
			
			   if (!_os.good())
				   throw BitStreamException("Write to bitstream failed", BitStreamException::INPUT_OUTPUT);

            _written += (uint64(_position) << 3);
            _position = 0;
        }	
    }
    catch (ios_base::failure& e) {
        throw BitStreamException(e.what(), BitStreamException::INPUT_OUTPUT);
    }
}

DefaultOutputBitStream::~DefaultOutputBitStream()
{
    close();
    delete[] _buffer;
}
