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
#include "../types.hpp"
#include "../IllegalArgumentException.hpp"
#include "TPAQPredictor.hpp"

using namespace kanzi;

///////////////////////// state table ////////////////////////
// States represent a bit history within some context.
// State 0 is the starting state (no bits seen).
// States 1-30 represent all possible sequences of 1-4 bits.
// States 31-252 represent a pair of counts, (n0,n1), the number
//   of 0 and 1 bits respectively.  If n0+n1 < 16 then there are
//   two states for each pair, depending on if a 0 or 1 was the last
//   bit seen.
// If n0 and n1 are too large, then there is no state to represent this
// pair, so another state with about the same ratio of n0/n1 is substituted.
// Also, when a bit is observed and the count of the opposite bit is large,
// then part of this count is discarded to favor newer data over old.
const uint8 STATE_TRANSITIONS[2][256] = {
    // Bit 0
    { 1, 3, 143, 4, 5, 6, 7, 8, 9, 10,
        11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
        21, 22, 23, 24, 25, 26, 27, 28, 29, 30,
        31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
        41, 42, 43, 44, 45, 46, 47, 48, 49, 50,
        51, 52, 47, 54, 55, 56, 57, 58, 59, 60,
        61, 62, 63, 64, 65, 66, 67, 68, 69, 6,
        71, 71, 71, 61, 75, 56, 77, 78, 77, 80,
        81, 82, 83, 84, 85, 86, 87, 88, 77, 90,
        91, 92, 80, 94, 95, 96, 97, 98, 99, 90,
        101, 94, 103, 101, 102, 104, 107, 104, 105, 108,
        111, 112, 113, 114, 115, 116, 92, 118, 94, 103,
        119, 122, 123, 94, 113, 126, 113, 128, 129, 114,
        131, 132, 112, 134, 111, 134, 110, 134, 134, 128,
        128, 142, 143, 115, 113, 142, 128, 148, 149, 79,
        148, 142, 148, 150, 155, 149, 157, 149, 159, 149,
        131, 101, 98, 115, 114, 91, 79, 58, 1, 170,
        129, 128, 110, 174, 128, 176, 129, 174, 179, 174,
        176, 141, 157, 179, 185, 157, 187, 188, 168, 151,
        191, 192, 188, 187, 172, 175, 170, 152, 185, 170,
        176, 170, 203, 148, 185, 203, 185, 192, 209, 188,
        211, 192, 213, 214, 188, 216, 168, 84, 54, 54,
        221, 54, 55, 85, 69, 63, 56, 86, 58, 230,
        231, 57, 229, 56, 224, 54, 54, 66, 58, 54,
        61, 57, 222, 78, 85, 82, 0, 0, 0, 0,
        0, 0, 0, 0, 0, 0 },
    // Bit 1
    { 2, 163, 169, 163, 165, 89, 245, 217, 245, 245,
        233, 244, 227, 74, 221, 221, 218, 226, 243, 218,
        238, 242, 74, 238, 241, 240, 239, 224, 225, 221,
        232, 72, 224, 228, 223, 225, 238, 73, 167, 76,
        237, 234, 231, 72, 31, 63, 225, 237, 236, 235,
        53, 234, 53, 234, 229, 219, 229, 233, 232, 228,
        226, 72, 74, 222, 75, 220, 167, 57, 218, 70,
        168, 72, 73, 74, 217, 76, 167, 79, 79, 166,
        162, 162, 162, 162, 165, 89, 89, 165, 89, 162,
        93, 93, 93, 161, 100, 93, 93, 93, 93, 93,
        161, 102, 120, 104, 105, 106, 108, 106, 109, 110,
        160, 134, 108, 108, 126, 117, 117, 121, 119, 120,
        107, 124, 117, 117, 125, 127, 124, 139, 130, 124,
        133, 109, 110, 135, 110, 136, 137, 138, 127, 140,
        141, 145, 144, 124, 125, 146, 147, 151, 125, 150,
        127, 152, 153, 154, 156, 139, 158, 139, 156, 139,
        130, 117, 163, 164, 141, 163, 147, 2, 2, 199,
        171, 172, 173, 177, 175, 171, 171, 178, 180, 172,
        181, 182, 183, 184, 186, 178, 189, 181, 181, 190,
        193, 182, 182, 194, 195, 196, 197, 198, 169, 200,
        201, 202, 204, 180, 205, 206, 207, 208, 210, 194,
        212, 184, 215, 193, 184, 208, 193, 163, 219, 168,
        94, 217, 223, 224, 225, 76, 227, 217, 229, 219,
        79, 86, 165, 217, 214, 225, 216, 216, 234, 75,
        214, 237, 74, 74, 163, 217, 0, 0, 0, 0,
        0, 0, 0, 0, 0, 0 }
};

const int32 STATE_MAP[] = {
    -10, -436, 401, -521, -623, -689, -736, -812, -812, -900,
    -865, -891, -1006, -965, -981, -916, -946, -976, -1072, -1014,
    -1058, -1090, -1044, -1030, -1044, -1104, -1009, -1418, -1131, -1131,
    -1269, -1332, -1191, -1169, -1108, -1378, -1367, -1126, -1297, -1085,
    -1355, -1344, -1169, -1269, -1440, -1262, -1332, -2047, -2047, -1984,
    -2047, -2047, -2047, -225, -402, -556, -502, -746, -609, -647,
    -625, -718, -700, -805, -748, -935, -838, -1053, -787, -806,
    -269, -1006, -278, -212, -41, -399, 137, -984, -998, -219,
    -455, -524, -556, -564, -577, -592, -610, -690, -650, -140,
    -396, -471, -450, -168, -215, -301, -325, -364, -315, -401,
    -96, -174, -102, -146, -61, -9, 54, 81, 116, 140,
    192, 115, -41, -93, -183, -277, -365, 104, -134, 37,
    -80, 181, -111, -184, 194, 317, 63, 394, 105, -92,
    299, 166, -17, 333, 131, 386, 403, 450, 499, 480,
    493, 504, 89, -119, 333, 558, 568, 501, -7, -151,
    203, 557, 595, 603, 650, 104, 960, 204, 933, 239,
    247, -12, -105, 94, 222, -139, 40, 168, -203, 566,
    -53, 243, 344, 542, 42, 208, 14, 474, 529, 82,
    513, 504, 570, 616, 644, 92, 669, 91, -179, 677,
    720, 157, -10, 687, 672, 750, 686, 830, 787, 683,
    723, 780, 783, 9, 842, 816, 885, 901, 1368, 188,
    1356, 178, 1419, 173, -22, 1256, 240, 167, 1, -31,
    -165, 70, -493, -45, -354, -25, -142, 98, -17, -158,
    -355, -448, -142, -67, -76, -310, -324, -225, -96, 0,
    46, -72, 0, -439, 14, -55, 1, 1, 1, 1,
    1, 1, 1, 1, 1, 1,
};

inline int32 TPAQPredictor::hash(int32 x, int32 y)
{
    const int32 h = x * HASH ^ y * HASH;
    return (h >> 1) ^ (h >> 9) ^ (x >> 2) ^ (y >> 3) ^ HASH;
}

TPAQPredictor::TPAQPredictor(map<string, string>* ctx)
    : _sse0(256)
    , _sse1(65536)
{
    int statesSize = 1 << 28;
    int mixersSize = 1 << 12;
    int hashSize = HASH_SIZE;
    _extra = false;
    uint extraMem = 0;

    if (ctx != nullptr) {
        if (ctx->find("extra") != ctx->end()) {
            string strExtra = (*ctx)["extra"];
            _extra = strExtra.compare(0, 5, "true") == 0;
        }

        extraMem = (_extra == true) ? 1 : 0;

        // Block size requested by the user
        // The user can request a big block size to force more states
        string strRBSZ = (*ctx)["blockSize"];
        const int rbsz = atoi(strRBSZ.c_str());

        if (rbsz >= 64 * 1024 * 1024)
            statesSize = 1 << 29;
        else if (rbsz >= 16 * 1024 * 1024)
            statesSize = 1 << 28;
        else
            statesSize = (rbsz >= 1024 * 1024) ? 1 << 27 : 1 << 26;


        // Actual size of the current block
        // Too many mixers hurts compression for small blocks.
        // Too few mixers hurts compression for big blocks.
        string strABSZ = (*ctx)["size"];
        const int absz = atoi(strABSZ.c_str());

        if (absz >= 8 * 1024 * 1024)
            mixersSize = 1 << 15;
        else if (absz >= 4 * 1024 * 1024)
            mixersSize = 1 << 12;
        else
            mixersSize = (absz >= 1 * 1024 * 1024) ? 1 << 10 : 1 << 9;
    }

    statesSize <<= extraMem;
    hashSize <<= (2 * extraMem);
    _pr = 2048;
    _c0 = 1;
    _c4 = 0;
    _c8 = 0;
    _pos = 0;
    _binCount = 0;
    _matchLen = 0;
    _matchPos = 0;
    _hash = 0;
    _mixers = new TPAQMixer[mixersSize];
    _mixer = &_mixers[0];
    _bigStatesMap = new uint8[statesSize];
    memset(_bigStatesMap, 0, statesSize);
    _smallStatesMap0 = new uint8[1 << 16];
    memset(_smallStatesMap0, 0, 1 << 16);
    _smallStatesMap1 = new uint8[1 << 24];
    memset(_smallStatesMap1, 0, 1 << 24);
    _hashes = new int32[hashSize];
    memset(_hashes, 0, sizeof(int32) * hashSize);
    _buffer = new byte[BUFFER_SIZE];
    memset(_buffer, 0, BUFFER_SIZE);
    _statesMask = statesSize - 1;
    _mixersMask = mixersSize - 1;
    _hashMask = hashSize - 1;
    _bpos = 0;
    _cp0 = &_smallStatesMap0[0];
    _cp1 = &_smallStatesMap1[0];
    _cp2 = &_bigStatesMap[0];
    _cp3 = &_bigStatesMap[0];
    _cp4 = &_bigStatesMap[0];
    _cp5 = &_bigStatesMap[0];
    _cp6 = &_bigStatesMap[0];
    _ctx0 = _ctx1 = _ctx2 = _ctx3 = 0;
    _ctx4 = _ctx5 = _ctx6 = 0;
}

TPAQPredictor::~TPAQPredictor()
{
    delete[] _bigStatesMap;
    delete[] _smallStatesMap0;
    delete[] _smallStatesMap1;
    delete[] _hashes;
    delete[] _buffer;
    delete[] _mixers;
}

// Update the probability model
void TPAQPredictor::update(int bit)
{
    _mixer->update(bit);
    _bpos++;
    _c0 = (_c0 << 1) | bit;

    if (_c0 > 255) {
        _buffer[_pos & MASK_BUFFER] = byte(_c0);
        _pos++;
        _c8 = (_c8 << 8) | ((_c4 >> 24) & 0xFF);
        _c4 = (_c4 << 8) | (_c0 & 0xFF);
        _hash = (((_hash * 43707) << 4) + _c4) & _hashMask;
        _c0 = 1;
        _bpos = 0;
        _binCount += ((_c4 >> 7) & 1);

        // Select Neural Net
        _mixer = &_mixers[_c4 & _mixersMask];

        // Add contexts to NN
        _ctx0 = (_c4 & 0xFF) << 8;
        _ctx1 = (_c4 & 0xFFFF) << 8;
        _ctx2 = createContext(2, _c4 & 0x00FFFFFF);
        _ctx3 = createContext(3, _c4);

        if (_binCount < _pos >> 2) {
            // Mostly text or mixed
            const int32 h1 = ((_c4 & MASK_80808080) == 0) ? _c4 : _c4 & MASK_80808080;
            const int32 h2 = ((_c8 & MASK_80808080) == 0) ? _c8 : _c8 & MASK_80808080;
            _ctx4 = createContext(4, _c4 ^ (_c8 & 0xFFFF));
            _ctx5 = hash(h1, h2);
            _ctx6 = hash(_c8 & MASK_F0F0F0F0, _c4 & MASK_F0F0F0F0);
        }
        else {
            // Mostly binary
            _ctx4 = createContext(4, _c4 ^ (_c4 & 0xFFFF));
            _ctx5 = hash(_c4 >> 16, _c8 >> 16);
            _ctx6 = ((_c4 & 0xFF) << 8) | ((_c8 & 0xFFFF) << 16);
        }

        // Find match
        findMatch();

        // Keep track current position
        _hashes[_hash] = _pos;
    }

    prefetchRead(&_bigStatesMap[(_ctx2 + _c0) & _statesMask]);
    prefetchRead(&_bigStatesMap[(_ctx3 + _c0) & _statesMask]);
    prefetchRead(&_bigStatesMap[(_ctx4 + _c0) & _statesMask]);
    prefetchRead(&_bigStatesMap[(_ctx5 + _c0) & _statesMask]);
    prefetchRead(&_bigStatesMap[(_ctx6 + _c0) & _statesMask]);

    // Get initial predictions
    const uint8* table = STATE_TRANSITIONS[bit];
    *_cp0 = table[*_cp0];
    _cp0 = &_smallStatesMap0[_ctx0 + _c0];
    const int p0 = STATE_MAP[*_cp0];
    *_cp1 = table[*_cp1];
    _cp1 = &_smallStatesMap1[_ctx1 + _c0];
    const int p1 = STATE_MAP[*_cp1];
    *_cp2 = table[*_cp2];
    _cp2 = &_bigStatesMap[(_ctx2 + _c0) & _statesMask];
    const int p2 = STATE_MAP[*_cp2];
    *_cp3 = table[*_cp3];
    _cp3 = &_bigStatesMap[(_ctx3 + _c0) & _statesMask];
    const int p3 = STATE_MAP[*_cp3];
    *_cp4 = table[*_cp4];
    _cp4 = &_bigStatesMap[(_ctx4 + _c0) & _statesMask];
    const int p4 = STATE_MAP[*_cp4];
    *_cp5 = table[*_cp5];
    _cp5 = &_bigStatesMap[(_ctx5 + _c0) & _statesMask];
    const int p5 = STATE_MAP[*_cp5];
    *_cp6 = table[*_cp6];
    _cp6 = &_bigStatesMap[(_ctx6 + _c0) & _statesMask];
    const int p6 = STATE_MAP[*_cp6];

    const int p7 = getMatchContextPred();

    // Mix predictions using NN
    int p = _mixer->get(p0, p1, p2, p3, p4, p5, p6, p7);

    // SSE (Secondary Symbol Estimation)
    if ((_extra == false) || (_binCount < (_pos >> 2))) {
        p = _sse1.get(bit, p, _c0 | (_c4 & 0xFF00));
    }
    else {
        p = _sse0.get(bit, p, _c0);
        p = (3 * _sse1.get(bit, p, _c0 | (_c4 & 0xFF00)) + p + 2) >> 2;
    }

    _pr = p + (uint32(p - 2048) >> 31);
}

inline void TPAQPredictor::findMatch()
{
    // Update ongoing sequence match or detect match in the buffer (LZ like)
    if (_matchLen > 0) {
        if (_matchLen < MAX_LENGTH)
            _matchLen++;

        _matchPos++;
    }
    else {
        // Retrieve match position
        _matchPos = _hashes[_hash];

        // Detect match
        if ((_matchPos != 0) && (_pos - _matchPos <= MASK_BUFFER)) {
            int r = _matchLen + 1;

            while ((r <= MAX_LENGTH) && (_buffer[(_pos - r) & MASK_BUFFER] == _buffer[(_matchPos - r) & MASK_BUFFER]))
                r++;

            _matchLen = r - 1;
        }
    }
}

// Get a prediction from the match model in [-2047..2048]
inline int TPAQPredictor::getMatchContextPred()
{
    int p = 0;

    if (_matchLen > 0) {
        if (_c0 == ((_buffer[_matchPos & MASK_BUFFER] & 0xFF) | 256) >> (8 - _bpos)) {
            // Add match length to NN inputs. Compute input based on run length
            p = (_matchLen <= 24) ? _matchLen : 24 + ((_matchLen - 24) >> 3);

            if (((_buffer[_matchPos & MASK_BUFFER] >> (7 - _bpos)) & 1) == 0)
                p = -p;

            p <<= 6;
        }
        else
            _matchLen = 0;
    }

    return p;
}

inline int32 TPAQPredictor::createContext(uint32 ctxId, uint32 cx)
{
    cx = cx * 987654323 + ctxId;
    cx = (cx << 16) | (cx >> 16);
    return cx * 123456791 + ctxId;
}

inline TPAQMixer::TPAQMixer()
{
    _pr = 2048;
    _skew = 0;
    _w0 = _w1 = _w2 = _w3 = _w4 = _w5 = _w6 = _w7 = 2048;
    _p0 = _p1 = _p2 = _p3 = _p4 = _p5 = _p6 = _p7 = 0;
    _learnRate = BEGIN_LEARN_RATE;
}

// Adjust weights to minimize coding cost of last prediction
inline void TPAQMixer::update(int bit)
{
    int32 err = (bit << 12) - _pr;

    if (err == 0)
        return;

    // Quickly decaying learn rate
    err = (err * _learnRate) >> 7;
    _learnRate += ((END_LEARN_RATE - _learnRate) >> 31);
    _skew += err;

    // Train Neural Network: update weights
    _w0 += ((_p0 * err + 0) >> 15);
    _w1 += ((_p1 * err + 0) >> 15);
    _w2 += ((_p2 * err + 0) >> 15);
    _w3 += ((_p3 * err + 0) >> 15);
    _w4 += ((_p4 * err + 0) >> 15);
    _w5 += ((_p5 * err + 0) >> 15);
    _w6 += ((_p6 * err + 0) >> 15);
    _w7 += ((_p7 * err + 0) >> 15);
}

inline int TPAQMixer::get(int32 p0, int32 p1, int32 p2, int32 p3, int32 p4, int32 p5, int32 p6, int32 p7)
{
    _p0 = p0;
    _p1 = p1;
    _p2 = p2;
    _p3 = p3;
    _p4 = p4;
    _p5 = p5;
    _p6 = p6;
    _p7 = p7;

    // Neural Network dot product (sum weights*inputs)
    const int32 p = (p0 * _w0) + (p1 * _w1) + (p2 * _w2) + (p3 * _w3) +
                    (p4 * _w4) + (p5 * _w5) + (p6 * _w6) + (p7 * _w7) + _skew;

    _pr = Global::squash((p + 65536) >> 17);
    return _pr;
}