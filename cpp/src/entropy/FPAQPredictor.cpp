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

#include "FPAQPredictor.hpp"

using namespace kanzi;

FPAQPredictor::FPAQPredictor()
{
    _ctxIdx = 1;

    for (int i = 0; i < 256; i++)
        _probs[i] = PSCALE >> 1;
}

// Update the probability model
// bit == 1 -> prob += ((PSCALE-prob) >> 6);
// bit == 0 -> prob -= (prob >> 6);
inline void FPAQPredictor::update(int bit)
{
    _probs[_ctxIdx] -= (((_probs[_ctxIdx] - (-bit & PSCALE)) >> 6) + bit);

    // Update context by registering the current bit (or wrapping after 8 bits)
    _ctxIdx = (_ctxIdx < 128) ? (_ctxIdx << 1) + bit : 1;
}
