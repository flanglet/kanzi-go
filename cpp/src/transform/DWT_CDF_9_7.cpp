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
#include "../IllegalArgumentException.hpp"
#include "DWT_CDF_9_7.hpp"

using namespace kanzi;

DWT_CDF_9_7::DWT_CDF_9_7(int width, int height, int steps) {
    if (width < 8)
        throw IllegalArgumentException("Invalid transform width (must be at least 8)");

    if (height < 8)
        throw IllegalArgumentException("Invalid transform width (must be at least 8)");

    if (steps < 1)
        throw IllegalArgumentException("Invalid number of iterations (must be at least 1)");

    if ((width >> steps) < 4)
        throw IllegalArgumentException("Invalid width for band L0 (must be at least 4)");

    if ((height >> steps) < 4)
        throw IllegalArgumentException("Invalid height for band L0 (must be at least 4)");

    if (((width >> steps) << steps) != width) {
        std::ostringstream errMsg;
        errMsg << "Invalid parameters: change width or number of steps (";
        errMsg << width << " divided by 2^" << steps << " is not an integer value)";
        throw IllegalArgumentException(errMsg.str());
    }

    if (((height >> steps) << steps) != height) {
        std::ostringstream errMsg;
        errMsg << "Invalid parameters: change height or number of steps (";
        errMsg << height << " divided by 2^" << steps << " is not an integer value)";
        throw IllegalArgumentException(errMsg.str());
    }

    _width = width;
    _height = height;
    _steps = steps;
    _data = new int[width * height];
}

DWT_CDF_9_7::~DWT_CDF_9_7() {
    delete[] _data;
}

// Calculate the forward discrete wavelet transform of the 2D input signal
bool DWT_CDF_9_7::forward(SliceArray<byte>& input, SliceArray<byte>& output, int length) {
    if (length != _width * _height)
        return false;

    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
        return false;

    if (input._index + input._length < length)
        return false;

    if (output._index + output._length < length)
        return false;

    if ((input._array != output._array) || (input._index != output._index)) {
        memmove( & output._array[output._index], & input._array[input._index], _width * _height);
    }

    for (int i = 0; i < _steps; i++) {
        // First, vertical transform
        forward( & output._array[output._index], _width, 1, _width >> i, _height >> i);

        // Then horizontal transform on the updated signal
        forward( & output._array[output._index], 1, _width, _height >> i, _width >> i);
    }

    input._index += (_width * _height);
    output._index += (_width * _height);
    return true;
}

void DWT_CDF_9_7::forward(byte * block, int stride, int inc, int dim1, int dim2) {
    const int stride2 = stride << 1;
    const int endOffs = dim1 * inc;
    const int half = stride * (dim2 >> 1);

    for (int offset = 0; offset < endOffs; offset += inc) {
        const int end = offset + (dim2 - 2) * stride;
        int prev = block[offset];

        // First lifting stage : Predict 1
        for (int i = offset + stride; i < end; i += stride2) {
            const int next = block[i + stride];
            block[i] -= ((PREDICT1 * (prev + next) + ADJUST1) >> SHIFT1);
            prev = next;
        }

        block[end + stride] -= ((PREDICT1 * block[end] + ADJUST2) >> SHIFT2);
        prev = block[offset + stride];

        // Second lifting stage : Update 1
        for (int i = offset + stride2; i <= end; i += stride2) {
            const int next = block[i + stride];
            block[i] -= ((UPDATE1 * (prev + next) + ADJUST1) >> SHIFT1);
            prev = next;
        }

        block[offset] -= ((UPDATE1 * block[offset + stride] + ADJUST2) >> SHIFT2);
        prev = block[offset];

        // Third lifting stage : Predict 2
        for (int i = offset + stride; i < end; i += stride2) {
            const int next = block[i + stride];
            block[i] += ((PREDICT2 * (prev + next) + ADJUST1) >> SHIFT1);
            prev = next;
        }

        block[end + stride] += ((PREDICT2 * block[end] + ADJUST2) >> SHIFT2);
        prev = block[offset + stride];

        // Fourth lifting stage : Update 2
        for (int i = offset + stride2; i <= end; i += stride2) {
            const int next = block[i + stride];
            block[i] += ((UPDATE2 * (prev + next) + ADJUST1) >> SHIFT1);
            prev = next;
        }

        block[offset] += ((UPDATE2 * block[offset + stride] + ADJUST2) >> SHIFT2);

        // Scale
        for (int i = offset; i <= end; i += stride2) {
            block[i] = (block[i] * SCALING1 + ADJUST1) >> SHIFT1;
            block[i + stride] = (block[i + stride] * SCALING2 + ADJUST1) >> SHIFT1;
        }

        // De-interleave sub-bands
        const int endj = offset + half;

        for (int i = offset, j = offset; j < endj; i += stride2, j += stride) {
            _data[j] = block[i];
            _data[half + j] = block[i + stride];
        }

        block[end + stride] = _data[end + stride];

        for (int i = offset; i <= end; i += stride)
            block[i] = _data[i];
    }
}

// Calculate the reverse discrete wavelet transform of the 2D input signal
bool DWT_CDF_9_7::inverse(SliceArray<byte>& input, SliceArray<byte>& output, int length) {
    if (length != _width * _height)
        return false;

    if ((!SliceArray<byte>::isValid(input)) || (!SliceArray<byte>::isValid(output)))
        return false;

    if (input._index + input._length < length)
        return false;

    if (output._index + output._length < length)
        return false;

    if ((input._array != output._array) || (input._index != output._index)) {
        memmove( & output._array[output._index], & input._array[input._index], _width * _height);
    }

    for (int i = _steps - 1; i >= 0; i--) {
        // First horizontal transform
        inverse( & output._array[output._index], 1, _width, _height >> i, _width >> i);

        // Then vertical transform on the updated signal
        inverse( & output._array[output._index], _width, 1, _width >> i, _height >> i);
    }

    input._index += (_width * _height);
    output._index += (_width * _height);
    return true;
}

void DWT_CDF_9_7::inverse(byte * block, int stride, int inc, int dim1, int dim2) {
    const int stride2 = stride << 1;
    const int endOffs = dim1 * inc;
    const int half = stride * (dim2 >> 1);

    for (int offset = 0; offset < endOffs; offset += inc) {
        const int end = offset + (dim2 - 2) * stride;
        const int endj = offset + half;

        // De-interleave sub-bands
        for (int i = offset; i <= end; i += stride)
            _data[i] = block[i];

        _data[end + stride] = block[end + stride];

        for (int i = offset, j = offset; j < endj; i += stride2, j += stride) {
            block[i] = _data[j];
            block[i + stride] = _data[half + j];
        }

        // Reverse scale
        for (int i = offset; i <= end; i += stride2) {
            block[i] = (block[i] * SCALING2 + ADJUST1) >> SHIFT1;
            block[i + stride] = (block[i + stride] * SCALING1 + ADJUST1) >> SHIFT1;
        }

        // Reverse Update 2
        int prev = block[offset + stride];

        for (int i = offset + stride2; i <= end; i += stride2) {
            const int next = block[i + stride];
            block[i] -= ((UPDATE2 * (prev + next) + ADJUST1) >> SHIFT1);
            prev = next;
        }

        block[offset] -= ((UPDATE2 * block[offset + stride] + ADJUST2) >> SHIFT2);
        prev = block[offset];

        // Reverse Predict 2
        for (int i = offset + stride; i < end; i += stride2) {
            const int next = block[i + stride];
            block[i] -= ((PREDICT2 * (prev + next) + ADJUST1) >> SHIFT1);
            prev = next;
        }

        block[end + stride] -= ((PREDICT2 * block[end] + ADJUST2) >> SHIFT2);
        prev = block[offset + stride];

        // Reverse Update 1
        for (int i = offset + stride2; i <= end; i += stride2) {
            const int next = block[i + stride];
            block[i] += ((UPDATE1 * (prev + next) + ADJUST1) >> SHIFT1);
            prev = next;
        }

        block[offset] += ((UPDATE1 * block[offset + stride] + ADJUST2) >> SHIFT2);
        prev = block[offset];

        // Reverse Predict 1
        for (int i = offset + stride; i < end; i += stride2) {
            const int next = block[i + stride];
            block[i] += ((PREDICT1 * (prev + next) + ADJUST1) >> SHIFT1);
            prev = next;
        }

        block[end + stride] += ((PREDICT1 * block[end] + ADJUST2) >> SHIFT2);
    }
}