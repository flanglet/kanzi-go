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

#include <iostream>
#include "../bitstream/DebugInputBitStream.hpp"

using namespace kanzi;

DebugInputBitStream::DebugInputBitStream(InputBitStream& ibs) THROW : _delegate(ibs), _out(cout), _width(80)
{
    _idx = 0;
    _mark = false;
    _hexa = false;
    _current = 0;
}

DebugInputBitStream::DebugInputBitStream(InputBitStream& ibs, ostream& os) THROW : _delegate(ibs), _out(os), _width(80)
{
    _idx = 0;
    _mark = false;
    _hexa = false;
    _current = 0;
}

DebugInputBitStream::DebugInputBitStream(InputBitStream& ibs, ostream& os, int width) THROW : _delegate(ibs), _out(os)
{
    if ((width != -1) && (width < 8))
        width = 8;

    if (width != -1)
        width &= 0xFFFFFFF8;

    _width = width;
    _idx = 0;
    _mark = false;
    _hexa = false;
    _current = 0;
}

DebugInputBitStream::~DebugInputBitStream()
{
    close();
}

// Returns 1 or 0
int DebugInputBitStream::readBit() THROW
{
    int res = _delegate.readBit();
    _current <<= 1;
    _current |= res;
    _out << ((res & 1) == 1 ? "1" : "0");
    _idx++;

    if (_mark == true)
        _out << "r";

    if (_width != -1) {
        if ((_idx - 1) % _width == _width - 1) {
            if (showByte())
                printByte(_current);

            _out << endl;
            _idx = 0;
        }
        else if ((_idx & 7) == 0) {
            if (showByte())
                printByte(_current);
            else
                _out << " ";
        }
    }
    else if ((_idx & 7) == 0) {
        if (showByte())
            printByte(_current);
        else
            _out << " ";
    }

    return res;
}

uint64 DebugInputBitStream::readBits(uint count) THROW
{
    uint64 res = _delegate.readBits(count);

    for (uint i = 1; i <= count; i++) {
        int bit = (res >> (count - i)) & 1;
        _idx++;
        _current <<= 1;
        _current |= bit;
        _out << ((bit == 1) ? "1" : "0");

        if ((_mark == true) && (i == count))
            _out << "r";

        if (_width != -1) {
            if (_idx % _width == 0) {
                if (showByte())
                    printByte(_current);

                _out << endl;
                _idx = 0;
            }
            else if ((_idx & 7) == 0) {
                if (showByte())
                    printByte(_current);
                else
                    _out << " ";
            }
        }
        else if ((_idx & 7) == 0) {
            if (showByte())
                printByte(_current);
            else
                _out << " ";
        }
    }

    return res;
}

void DebugInputBitStream::printByte(byte b)
{
	int val = b & 0xFF;

    if (val < 10)
        _out << " [00" << val << "] ";
    else if (val < 100)
        _out << " [0" << val << "] ";
    else
        _out << " [" << val << "] ";
}
