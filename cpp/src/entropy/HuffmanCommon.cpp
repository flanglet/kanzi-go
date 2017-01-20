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

#include <algorithm>
#include <cstring>
#include <vector>
#include "HuffmanCommon.hpp"

using namespace kanzi;

class CodeLengthArrayComparator {
private:
    short* _sizes;

public:
    CodeLengthArrayComparator(short sizes[]) { _sizes = sizes; }

    CodeLengthArrayComparator() {}

    bool operator()(int i, int j);
};

bool CodeLengthArrayComparator::operator()(int lidx, int ridx)
{
    // Check size (natural order) as first key
    const int res = _sizes[lidx] - _sizes[ridx];

    // Check index (natural order) as second key
    return (res != 0) ? res < 0 : lidx < ridx;
}

// Return the number of codes generated
int HuffmanCommon::generateCanonicalCodes(short sizes[], uint codes[], uint ranks[], int count)
{
    // Sort by increasing size (first key) and increasing value (second key)
    if (count > 1) {
        vector<uint> v(ranks, ranks + count);
        CodeLengthArrayComparator comparator(sizes);
        sort(v.begin(), v.end(), comparator);
        uint* pv = &v[0];
        memcpy(ranks, pv, count*sizeof(uint));
    }

    int code = 0;
    int len = sizes[ranks[0]];

    for (int i = 0; i < count; i++) {
        const int r = ranks[i];

        if (sizes[r] > len) {
            code <<= (sizes[r] - len);
            len = sizes[r];

            // Max length reached
            if (len > 24)
                return -1;
        }

        codes[r] = code;
        code++;
    }

    return count;
}
