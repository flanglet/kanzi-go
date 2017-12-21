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

#include <cstring>
#include "MTFT.hpp"

using namespace kanzi;

MTFT::MTFT()
{
   _anchor = nullptr;
   memset(_lengths, 0, sizeof(int) * 256);
   memset(_buckets, 0, sizeof(byte) * 256);
   memset(_heads, 0, sizeof(Payload*) * 16);
}

bool MTFT::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if (input._array == output._array)
        return false;

    if ((count < 0) || (count + input._index > input._length))
        return false;

    byte* indexes = _buckets;

    for (int i = 0; i<256; i++)
        indexes[i] = (byte)i;

    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];
    byte value = 0;

    for (int i = 0; i < count; i++) {
        if (src[i] == 0) {
            // Shortcut
            dst[i] = value;
            continue;
        }

        int idx = src[i] & 0xFF;
        value = indexes[idx];
        dst[i] = value;

        if (idx <= 16) {
            for (int j = idx - 1; j >= 0; j--)
                indexes[j + 1] = indexes[j];
        }
        else {
            memmove(&indexes[1], &indexes[0], idx);
        }

        indexes[0] = value;
    }

    input._index += count;
    output._index += count;
    return true;
}

// Initialize the linked lists: 1 item in bucket 0 and LIST_LENGTH in each other
// Used by forward() only
void MTFT::initLists()
{
    Payload* previous = &_payloads[0];
    previous->_value = 0;
    _heads[0] = previous;
    _lengths[0] = 1;
    _buckets[0] = 0;
    int listIdx = 0;

    for (int i = 1; i < 256; i++) {
        _payloads[i]._value = (byte)i;

        if ((i - 1) % LIST_LENGTH == 0) {
            listIdx++;
            _heads[listIdx] = &_payloads[i];
            _lengths[listIdx] = LIST_LENGTH;
        }

        _buckets[i] = (byte)listIdx;
        previous->_next = &_payloads[i];
        _payloads[i]._previous = previous;
        previous = &_payloads[i];
    }

    // Create a fake end payload so that every payload in every list has a successor
    _anchor = &_payloads[256];
    previous->_next = _anchor;
}

// Recreate one list with 1 item and 15 lists with LIST_LENGTH items
// Update lengths and buckets accordingly.
// Used by forward() only
void MTFT::balanceLists(bool resetValues)
{
    _lengths[0] = 1;
    Payload* p = _heads[0]->_next;
    byte val = 0;

    if (resetValues == true) {
        _heads[0]->_value = (byte)0;
        _buckets[0] = 0;
    }

    for (int listIdx = 1; listIdx < 16; listIdx++) {
        _heads[listIdx] = p;
        _lengths[listIdx] = LIST_LENGTH;

        for (int n = 0; n < LIST_LENGTH; n++) {
            if (resetValues == true)
                p->_value = ++val;

            _buckets[p->_value & 0xFF] = (byte)listIdx;
            p = p->_next;
        }
    }
}

bool MTFT::forward(SliceArray<byte>& input, SliceArray<byte>& output, int count)
{
    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
       return false;

    if (input._array == output._array)
        return false;

    if ((count < 0) || (count + input._index > input._length))
        return false;

    if (_anchor == nullptr)
        initLists();
    else
        balanceLists(true);

    byte* src = &input._array[input._index];
    byte* dst = &output._array[output._index];
    byte previous = _heads[0]->_value;

    for (int i = 0; i < count; i++) {
        const byte current = src[i];

        if (current == previous) {
            dst[i] = 0;
            continue;
        }

        // Find list index
        const int listIdx = _buckets[current & 0xFF];
        Payload* p = _heads[listIdx];
        int idx = 0;

        for (int ii = 0; ii < listIdx; ii++)
            idx += _lengths[ii];

        // Find index in list (less than RESET_THRESHOLD iterations)
        while (p->_value != current) {
            p = p->_next;
            idx++;
        }

        dst[i] = (byte)idx;

        // Unlink (the end anchor ensures p.next != nullptr)
        p->_previous->_next = p->_next;
        p->_next->_previous = p->_previous;

        // Add to head of first list
        p->_next = _heads[0];
        p->_next->_previous = p;
        _heads[0] = p;

        // Update list information
        if (listIdx != 0) {
            // Update head if needed
            if (p == _heads[listIdx])
                _heads[listIdx] = p->_previous->_next;

            _buckets[current & 0xFF] = 0;

            if ((_lengths[0] >= RESET_THRESHOLD)) {
                balanceLists(false);
            }
            else {
                _lengths[listIdx]--;
                _lengths[0]++;
            }
        }

        previous = current;
    }

    input._index += count;
    output._index += count;
    return true;
}
