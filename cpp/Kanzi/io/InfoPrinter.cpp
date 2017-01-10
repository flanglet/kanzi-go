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
{
    _level = infoLevel;
    _type = type;

    if (type == InfoPrinter::ENCODING) {
        _thresholds[0] = BlockEvent::BEFORE_TRANSFORM;
        _thresholds[1] = BlockEvent::AFTER_TRANSFORM;
        _thresholds[2] = BlockEvent::BEFORE_ENTROPY;
        _thresholds[3] = BlockEvent::AFTER_ENTROPY;
    }
    else {
        _thresholds[0] = BlockEvent::BEFORE_ENTROPY;
        _thresholds[1] = BlockEvent::AFTER_ENTROPY;
        _thresholds[2] = BlockEvent::BEFORE_TRANSFORM;
        _thresholds[3] = BlockEvent::AFTER_TRANSFORM;
    }
}

void InfoPrinter::processEvent(const BlockEvent& evt)
{
    int currentBlockId = evt.getId();

    if (evt.getType() == _thresholds[0]) {
        // Register initial block size
        BlockInfo* bi = new BlockInfo();
        bi->time0 = evt.getTime();

        if (_type == InfoPrinter::ENCODING)
            bi->stage0Size = evt.getSize();

        _map.insert(pair<int, BlockInfo*>(currentBlockId, bi));

        if (_level >= 4) {
            _os << evt.toString() << endl;
        }
    }
    else if (evt.getType() == _thresholds[1]) {
        map<int, BlockInfo*>::iterator it = _map.find(currentBlockId);

        if (it == _map.end())
            return;

        BlockInfo* bi = it->second;
        bi->time1 = evt.getTime();

        if (_type == InfoPrinter::DECODING)
            bi->stage0Size = evt.getSize();

        if (_level >= 4) {
            int duration_ms = int(1000 * (bi->time1 - bi->time0) / CLOCKS_PER_SEC);
            stringstream ss;
            ss << evt.toString() << " [" << duration_ms << " ms]";
            _os << ss.str() << endl;
        }
    }
    else if (evt.getType() == _thresholds[2]) {
        map<int, BlockInfo*>::iterator it = _map.find(currentBlockId);

        if (it == _map.end())
            return;

        BlockInfo* bi = it->second;
        bi->time2 = evt.getTime();
        bi->stage1Size = evt.getSize();

        if (_level >= 4) {
            int duration_ms = int(1000 * (bi->time2 - bi->time1) / CLOCKS_PER_SEC);
            stringstream ss;
            ss << evt.toString() << " [" << duration_ms << " ms]";
            _os << ss.str() << endl;
        }
    }
    else if (evt.getType() == _thresholds[3]) {
        int stage2Size = evt.getSize();
        map<int, BlockInfo*>::iterator it = _map.find(currentBlockId);

        if (it == _map.end())
            return;

        if (_level < 2) {
            delete it->second;
            _map.erase(it);
            return;
        }

        BlockInfo* bi = it->second;
        bi->time3 = evt.getTime();
        int duration1_ms = int(1000 * (bi->time1 - bi->time0) / CLOCKS_PER_SEC);
        int duration2_ms = int(1000 * (bi->time3 - bi->time2) / CLOCKS_PER_SEC);
        stringstream ss;

        if (_level >= 4) {
            ss << evt.toString() << " [" << duration2_ms << " ms]" << endl;
        }

        // Display block info
        if (_level >= 3) {
            ss << "Block " << currentBlockId << ": " << bi->stage0Size << " => ";
            ss << bi->stage1Size << " [" << duration1_ms << " ms] => " << stage2Size;
            ss << " [" << duration2_ms << " ms]";
        }
        else {
            ss << "Block " << currentBlockId << ": " << bi->stage0Size << " => ";
            ss << bi->stage1Size << " => " << stage2Size;
        }

        // Add compression ratio for encoding
        if (_type == InfoPrinter::ENCODING) {
            if (bi->stage0Size != 0) {
                char buf[32];
                sprintf(buf, " (%d%%)", (int)(stage2Size * (double)100 / (double)bi->stage0Size));
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
        delete bi;
        _map.erase(it);
    }
}
