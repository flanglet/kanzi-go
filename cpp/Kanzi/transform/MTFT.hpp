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

#ifndef _MTFT_
#define _MTFT_

#include "../Transform.hpp"

namespace kanzi 
{

   // The Move-To-Front Transform is a simple reversible transform based on
   // permutation of the data in the original message to reduce the entropy.
   // See http://en.wikipedia.org/wiki/Move-to-front_transform
   // Fast implementation using double linked lists to minimize the number of lookups

   class Payload {
   public:
       Payload* _previous;
       Payload* _next;
       byte _value;

       Payload()
       {
           _value = 0;
           _previous = 0;
           _next = 0;
       }

       Payload(byte value)
       {
           _value = value;
           _previous = 0;
           _next = 0;
       }

       ~Payload() {}
   };

   class MTFT : public Transform<byte> {
   public:
       MTFT();

       ~MTFT() {}

       bool forward(SliceArray<byte>& source, SliceArray<byte>& destination, int length);

       bool inverse(SliceArray<byte>& source, SliceArray<byte>& destination, int length);

   private:
       static const int RESET_THRESHOLD = 64;
       static const int LIST_LENGTH = 17;

       Payload* _heads[16]; // linked lists
       int _lengths[256]; // length of linked lists
       byte _buckets[256]; // index of list
       Payload* _anchor;
       Payload _payloads[257];

       void balanceLists(bool resetValues);

       void initLists();
   };

}
#endif
