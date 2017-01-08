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

#ifndef _BlockCompressor_
#define _BlockCompressor_

#include <map>
#include <vector>
#include "../concurrent.hpp"
#include "../InputStream.hpp"
#include "../io/BlockListener.hpp"
#include "../io/CompressedOutputStream.hpp"

namespace kanzi 
{

   class BlockCompressor
   {
   public:
      static const int WARN_EMPTY_INPUT = -128;

      BlockCompressor(map<string, string>& m, ThreadPool<EncodingTaskResult>* threadPool, bool ownPool);
      
      BlockCompressor(int argc, const char* argv[], ThreadPool<EncodingTaskResult>* threadPool, bool ownPool);

      ~BlockCompressor();

      static int main(int argc, const char* argv[]);

      int call();

      bool addListener(BlockListener* bl);

      bool removeListener(BlockListener* bl);


   private:
      static const int DEFAULT_BUFFER_SIZE = 32768;

      int _verbosity;
      bool _overwrite;
      bool _checksum;
      string _inputName;
      string _outputName;
      string _codec;
      string _transform;
      int _blockSize;
      int _jobs;
      InputStream* _is;
      CompressedOutputStream* _cos;
      bool _ownPool;
      ThreadPool<EncodingTaskResult>* _pool;
      vector<BlockListener*> _listeners;

      void dispose();

      static void processCommandLine(int argc, const char* argv[], map<string, string>& map);

      static void printOut(const char* msg, bool print);
   };

}
#endif