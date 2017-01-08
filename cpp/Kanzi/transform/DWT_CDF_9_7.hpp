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

#ifndef _DWT_CDF_9_7_
#define _DWT_CDF_9_7_

#include "../Transform.hpp"

namespace kanzi 
{

   // Discrete Wavelet Transform Cohen-Daubechies-Feauveau 9/7 for 2D signals
   // Fast integer based implementation using the lifting scheme.
   class DWT_CDF_9_7 : public Transform<byte>
   {
   public:
       DWT_CDF_9_7(int width, int height, int steps);

       ~DWT_CDF_9_7();

       bool forward(SliceArray<byte>& source, SliceArray<byte>& destination, int length);

       bool inverse(SliceArray<byte>& source, SliceArray<byte>& destination, int length);

       int getWidth() const { return _width; }

       int getHeight() const { return _height; }

       int getLevels() const { return _steps; }

   private:
       static const int SHIFT1 = 12;
       static const int ADJUST1 = 1 << (SHIFT1 - 1);
       static const int SHIFT2 = SHIFT1 - 1;
       static const int ADJUST2 = 1 << (SHIFT2 - 1);

       static const int PREDICT1 = 6497; // 1.586134342  * 1<<SHIFT1
       static const int UPDATE1 = 217; // 0.0529801185 * 1<<SHIFT1
       static const int PREDICT2 = 3616; // 0.8829110762 * 1<<SHIFT1
       static const int UPDATE2 = 1817; // 0.4435068522 * 1<<SHIFT1
       static const int SCALING1 = 4709; // 1.149604398  * 1<<SHIFT1
       static const int SCALING2 = 3563; // 0.869864452  * 1<<SHIFT1

       int* _data;
       int _width;
       int _height;
       int _steps;

       void forward(byte* block, int stride, int inc, int dim1, int dim2);

       void inverse(byte* block, int stride, int inc, int dim1, int dim2);
   };

}
#endif
