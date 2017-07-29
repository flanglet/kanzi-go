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
#include <ctime>
#include <cstdio>
#include "BlockEvent.hpp"

using namespace kanzi;

BlockEvent::BlockEvent(BlockEvent::Type type, int id, int size)
    : _time(time(nullptr))
{
    _id = id;
    _size = size;
    _hash = 0;
    _hashing = false;
    _type = type;
}

BlockEvent::BlockEvent(BlockEvent::Type type, int id, int size, int hash)
    : _time(time(nullptr))
{
    _id = id;
    _size = size;
    _hash = hash;
    _hashing = true;
    _type = type;
}

BlockEvent::BlockEvent(BlockEvent::Type type, int id, int size, int hash, bool hashing)
    : _time(time(nullptr))
{
    _id = id;
    _size = size;
    _hash = hash;
    _hashing = hashing;
    _type = type;
}

string BlockEvent::toString() const
{
    std::stringstream ss;
    ss << "{ \"type\":\"" << getTypeAsString() << "\"";
    ss << ", \"id\":" << getId();
    ss << ", \"size\":" << getSize();
    ss << ", \"time\":" << getTime();

    if (_hashing == true) {
        char buf[32];
        sprintf(buf, "%08X", getHash());
        ss << ", \"hash\":" << buf;
    }

    ss << " }";
    return ss.str();
}

string BlockEvent::getTypeAsString() const
{
    switch (_type) {
    case BEFORE_TRANSFORM:
        return "BEFORE_TRANSFORM";

    case AFTER_TRANSFORM:
        return "AFTER_TRANSFORM";

    case BEFORE_ENTROPY:
        return "BEFORE_ENTROPY";

    case AFTER_ENTROPY:
        return "AFTER_ENTROPY";

    default:
        return "Unknown Type";
    }
}
