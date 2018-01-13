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

#include "DefaultInputBitStream.hpp"
#include "../IllegalArgumentException.hpp"
#include "../Memory.hpp"
#include "../io/IOException.hpp"

using namespace kanzi;

DefaultInputBitStream::DefaultInputBitStream(InputStream& is, uint bufferSize) THROW : _is(is)
{
    if (bufferSize < 1024)
        throw IllegalArgumentException("Invalid buffer size (must be at least 1024)");

    if (bufferSize > 1 << 29)
        throw IllegalArgumentException("Invalid buffer size (must be at most 536870912)");

    if ((bufferSize & 7) != 0)
        throw IllegalArgumentException("Invalid buffer size (must be a multiple of 8)");

    _bufferSize = bufferSize;
    _buffer = new byte[_bufferSize];
    _bitIndex = 63;
    _maxPosition = -1;
    _position = 0;
    _current = 0;
    _read = 0;
    _closed = false;
}

DefaultInputBitStream::~DefaultInputBitStream()
{
    close();
    delete[] _buffer;
}

// Returns 1 or 0
inline int DefaultInputBitStream::readBit() THROW
{
    if (_bitIndex == 63)
        pullCurrent(); // Triggers an exception if stream is closed

    int bit = (int)((_current >> _bitIndex) & 1);
    _bitIndex = (_bitIndex + 63) & 63;
    return bit;
}

uint64 DefaultInputBitStream::readBits(uint count) THROW
{
    if ((count == 0) || (count > 64))
        throw BitStreamException("Invalid count: " + to_string(count) + " (must be in [1..64])");

    uint64 res;

    if (count <= _bitIndex + 1) {
        // Enough spots available in 'current'
        uint shift = _bitIndex + 1 - count;

        if (_bitIndex == 63) {
            pullCurrent();
            shift += (_bitIndex - 63); // adjust if bitIndex != 63 (end of stream)
        }

        res = (_current >> shift) & ((uint64(-1)) >> (64 - count));
        _bitIndex = (_bitIndex - count) & 63;
    }
    else {
        // Not enough spots available in 'current'
        uint remaining = count - _bitIndex - 1;
        res = _current & (uint64(-1) >> (63 - _bitIndex));
        pullCurrent();
        res <<= remaining;
        _bitIndex -= remaining;
        res |= (_current >> (_bitIndex + 1));
    }

    return res;
}

void DefaultInputBitStream::close() THROW
{
    if (isClosed() == true)
        return;

    _closed = true;
    _read += 63;

    // Reset fields to force a readFromInputStream() and trigger an exception
    // on readBit() or readBits()
    _bitIndex = 63;
    _maxPosition = -1;
}

int DefaultInputBitStream::readFromInputStream(uint count) THROW
{
    if (isClosed() == true)
        throw BitStreamException("Stream closed", BitStreamException::STREAM_CLOSED);

    int size = -1;

    try {
        _read += (uint64(_maxPosition + 1) << 3);
        _is.read(reinterpret_cast<char*>(_buffer), count);  

        if (_is.good()) {
            size = count;
        }
        else {
            size = int(_is.gcount());

            if (!_is.eof()) {
               _position = 0;
               _maxPosition = (size <= 0) ? -1 : size - 1;
               throw BitStreamException("No more data to read in the bitstream",
                   BitStreamException::END_OF_STREAM);
            }
        }
    }
    catch (IOException& e) {
        _position = 0;
        _maxPosition = (size <= 0) ? -1 : size - 1;
        throw BitStreamException(e.what(), BitStreamException::INPUT_OUTPUT);
    }

    _position = 0;
    _maxPosition = (size <= 0) ? -1 : size - 1;
    return size;
}

// Return false when the bitstream is closed or the End-Of-Stream has been reached
bool DefaultInputBitStream::hasMoreToRead()
{
    if (isClosed() == true)
        return false;

    if ((_position < _maxPosition) || (_bitIndex != 63))
        return true;

    try {
        readFromInputStream(_bufferSize);
    }
    catch (BitStreamException&) {
        return false;
    }

    return true;
}

// Pull 64 bits of current value from buffer.
inline void DefaultInputBitStream::pullCurrent()
{
    if (_position > _maxPosition)
        readFromInputStream(_bufferSize);

    uint64 val;

    if (_position + 7 > _maxPosition) {
        // End of stream: overshoot max position => adjust bit index
        uint shift = (_maxPosition - _position) << 3;
        _bitIndex = shift + 7;
        val = 0;

        while (_position <= _maxPosition) {
            val |= ((uint64(_buffer[_position++] & 0xFF)) << shift);
            shift -= 8;
        }
    }
    else {
        // Regular processing, buffer length is multiple of 8
        val = BigEndian::readLong64(&_buffer[_position]);
        _bitIndex = 63;
        _position += 8;
    }

    _current = val;
}
