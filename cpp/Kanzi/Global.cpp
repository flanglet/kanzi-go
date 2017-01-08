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

#include "Global.hpp"

using namespace kanzi;

const int Global::INV_EXP[] = {
    0, 24, 41, 70, 118, 200, 338, 570,
    958, 1606, 2673, 4400, 7116, 11203, 16955, 24339,
    32768, 41197, 48581, 54333, 58420, 61136, 62863, 63930,
    64578, 64966, 65198, 65336, 65418, 65466, 65495, 65512,
    65522
};

const int* Global::STRETCH = Global::initStretch();

const int* Global::initStretch()
{
    int* res = new int[4096];
    int pi = 0;

    for (int x = -2047; (x <= 2047) && (pi < 4096); x++) {
        int i = squash(x);

        while (pi <= i)
            res[pi++] = x;
    }

    res[4095] = 2047;
    return res;
}

