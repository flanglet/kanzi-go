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

#ifndef _BlockEvent_
#define _BlockEvent_

#include <time.h>
#include "../types.hpp"
#include "../concurrent.hpp"

using namespace std;

namespace kanzi 
{

   class BlockEvent
   {
   public:
       enum Type 
       {
           BEFORE_TRANSFORM,
           AFTER_TRANSFORM,
           BEFORE_ENTROPY,
           AFTER_ENTROPY
       };

       BlockEvent(BlockEvent::Type type, int id, int size);

       BlockEvent(BlockEvent::Type type, int id, int size, int hash);

       BlockEvent(BlockEvent::Type type, int id, int size, int hash, bool hashing);

       ~BlockEvent() {}

       int getId() const { return _id; }

       int getSize() const { return _size; }

       BlockEvent::Type getType() const { return _type; }

       string getTypeAsString() const;

       time_t getTime() const { return _time; }

       double getElapsed() const { return _clock.elapsed(); }

       int getHash() const { return (_hashing) ? _hash : 0; }

       string toString() const;

   private:
       int _id;
       int _size;
       int _hash;
       BlockEvent::Type _type;
       bool _hashing;
       time_t _time;
       Clock _clock;
   };

}
#endif