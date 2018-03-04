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
#include <cstdio>
#include "InfoPrinter.hpp"

using namespace kanzi;

InfoPrinter::InfoPrinter(int infoLevel, InfoPrinter::Type type, OutputStream& os)
    : _os(os)
    , _type(type)
{
    _level = infoLevel;

    if (type == InfoPrinter::ENCODING) {
        _thresholds[0] = Event::COMPRESSION_START;
        _thresholds[1] = Event::BEFORE_TRANSFORM;
        _thresholds[2] = Event::AFTER_TRANSFORM;
        _thresholds[3] = Event::BEFORE_ENTROPY;
        _thresholds[4] = Event::AFTER_ENTROPY;
        _thresholds[5] = Event::COMPRESSION_END;
    }
    else {
        _thresholds[0] = Event::DECOMPRESSION_START;
        _thresholds[1] = Event::BEFORE_ENTROPY;
        _thresholds[2] = Event::AFTER_ENTROPY;
        _thresholds[3] = Event::BEFORE_TRANSFORM;
        _thresholds[4] = Event::AFTER_TRANSFORM;
        _thresholds[5] = Event::DECOMPRESSION_END;
    }
}

void InfoPrinter::processEvent(const Event& evt)
{
    int currentBlockId = evt.getId();

    if (evt.getType() == _thresholds[1]) {

        // Register initial block size
        BlockInfo* bi = new BlockInfo();
        bi->_time0 = evt.getTime();

        if (_type == InfoPrinter::ENCODING)
            bi->_stage0Size = evt.getSize();

        {
#ifdef CONCURRENCY_ENABLED
            unique_lock<mutex> lock(_mutex);
#endif
            _map.insert(pair<int, BlockInfo*>(currentBlockId, bi));
        }

        if (_level >= 5) {
            _os << evt.toString() << endl;
        }
    }
    else if (evt.getType() == _thresholds[2]) {
        BlockInfo* bi = nullptr;

        {
#ifdef CONCURRENCY_ENABLED
            unique_lock<mutex> lock(_mutex);
#endif
            map<int, BlockInfo*>::iterator it = _map.find(currentBlockId);

            if (it == _map.end())
                return;

            bi = it->second;
        }

        if (_type == InfoPrinter::DECODING)
            bi->_stage0Size = evt.getSize();

        bi->_time1 = evt.getTime();

        if (_level >= 5) {
            stringstream ss;
            ss << evt.toString() << " [" << uint(double(bi->_time1 - bi->_time0) / CLOCKS_PER_SEC * 1000.0) << " ms]";
            _os << ss.str() << endl;
        }
    }
    else if (evt.getType() == _thresholds[3]) {
        BlockInfo* bi = nullptr;

        {
#ifdef CONCURRENCY_ENABLED
            unique_lock<mutex> lock(_mutex);
#endif
            map<int, BlockInfo*>::iterator it = _map.find(currentBlockId);

            if (it == _map.end())
                return;

            bi = it->second;
        }

        bi->_time2 = evt.getTime();
        bi->_stage1Size = evt.getSize();

        if (_level >= 5) {
            stringstream ss;
            ss << evt.toString() << " [" << uint(double(bi->_time2 - bi->_time1) / CLOCKS_PER_SEC * 1000.0) << " ms]";
            _os << ss.str() << endl;
        }
    }
    else if (evt.getType() == _thresholds[4]) {
        BlockInfo* bi = nullptr;
        map<int, BlockInfo*>::iterator it;

        {
#ifdef CONCURRENCY_ENABLED
            unique_lock<mutex> lock(_mutex);
#endif
            it = _map.find(currentBlockId);

            if (it == _map.end())
                return;

            if (_level < 3) {
                delete it->second;
                _map.erase(it);
                return;
            }

            bi = it->second;
        }

        int64 stage2Size = evt.getSize();
        bi->_time3 = evt.getTime();
        stringstream ss;

        if (_level >= 5) {
            ss << evt.toString() << " [" << uint(double(bi->_time3 - bi->_time2) / CLOCKS_PER_SEC * 1000.0) << " ms]" << endl;
        }

        // Display block info
        if (_level >= 4) {
            ss << "Block " << currentBlockId << ": " << bi->_stage0Size << " => ";
            ss << bi->_stage1Size << " [" << uint(double(bi->_time1 - bi->_time0) / CLOCKS_PER_SEC * 1000.0) << " ms] => " << stage2Size;
            ss << " [" << uint(double(bi->_time3 - bi->_time2) / CLOCKS_PER_SEC * 1000.0) << " ms]";

            // Add compression ratio for encoding
            if (_type == InfoPrinter::ENCODING) {
                if (bi->_stage0Size != 0) {
                    char buf[32];
                    sprintf(buf, " (%d%%)", uint(stage2Size * double(100) / double(bi->_stage0Size)));
                    ss << buf;
                }
            }

            // Optionally add hash
            if (evt.getHash() != 0) {
                char buf[32];
                sprintf(buf, " [%08X]", evt.getHash());
                ss << buf;
            }

            _os << ss.str() << endl;
        }

        delete bi;

        {
#ifdef CONCURRENCY_ENABLED
            unique_lock<mutex> lock(_mutex);
#endif
            _map.erase(it);
        }
    }
    else if ((evt.getType() == Event::AFTER_HEADER_DECODING) && (_level >= 3)) {
        _os << evt.toString() << endl;
    }
    else if (_level >= 5) {
        _os << evt.toString() << endl;
    }
}
