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
#include "DebugOutputBitStream.hpp"

using namespace kanzi;

DebugOutputBitStream::DebugOutputBitStream(OutputBitStream& obs) THROW : _delegate(obs), _out(cout), _width(80)
{
    _mark = false;
    _hexa = false;
    _current = 0;
    _idx = 0;
}

DebugOutputBitStream::DebugOutputBitStream(OutputBitStream& obs, OutputStream& os) THROW : _delegate(obs), _out(os), _width(80)
{
    _mark = false;
    _hexa = false;
    _current = 0;
    _idx = 0;
}

DebugOutputBitStream::DebugOutputBitStream(OutputBitStream& obs, OutputStream& os, int width) THROW : _delegate(obs), _out(os)
{
    if ((width != -1) && (width < 8))
        width = 8;

    if (width != -1)
        width &= 0xFFFFFFF8;

    _width = width;
    _mark = false;
    _hexa = false;
    _current = 0;
    _idx = 0;
}

DebugOutputBitStream::~DebugOutputBitStream()
{
    close();
}

void DebugOutputBitStream::writeBit(int bit) THROW
{
    bit &= 1;
    _out << ((bit == 1) ? "1" : "0");
    _current <<= 1;
    _current |= bit;
    _idx++;

    if (_mark == true)
        _out << "w";

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

    _delegate.writeBit(bit);
}

int DebugOutputBitStream::writeBits(uint64 bits, uint count) THROW
{
    int res = _delegate.writeBits(bits, count);

    for (int i = 1; i <= res; i++) {
        uint64 bit = (bits >> (res - i)) & 1;
        _current <<= 1;
        _current |= bit;
        _idx++;
        _out << ((bit == 1) ? "1" : "0");

        if ((_mark == true) && (i == res))
            _out << "w";

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

void DebugOutputBitStream::printByte(byte b)
{
    int val = b & 0xFF;

    if (val < 10)
        _out << " [00" << val << "] ";
    else if (val < 100)
        _out << " [0" << val << "] ";
    else
        _out << " [" << val << "] ";
}
