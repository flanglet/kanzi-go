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

#ifndef _CompressedOutputStream_
#define _CompressedOutputStream_

#include <map>
#include <string>
#include <vector>
#include "../concurrent.hpp"
#include "../Listener.hpp"
#include "../OutputStream.hpp"
#include "../OutputBitStream.hpp"
#include "../SliceArray.hpp"
#include "../util/XXHash32.hpp"

namespace kanzi {

   class EncodingTaskResult {
   public:
       int _blockId;
       int _error; // 0 = OK
       string _msg;

       EncodingTaskResult()
           : _msg("")
       {
           _blockId = -1;
           _error = 0;
       }

       EncodingTaskResult(int blockId, int error, const string& msg)
           : _msg(msg)
       {
           _blockId = blockId;
           _error = error;
       }

       EncodingTaskResult(const EncodingTaskResult& result)
           : _msg(result._msg)
       {
           _blockId = result._blockId;
           _error = result._error;
       }

       ~EncodingTaskResult() {}
   };

   // A task used to encode a block
   // Several tasks may run in parallel. The transforms can be computed concurrently
   // but the entropy encoding is sequential since all tasks share the same bitstream.
   template <class T>
   class EncodingTask : public Task<T> {
   private:
       SliceArray<byte>* _data;
       SliceArray<byte>* _buffer;
       int _blockLength;
       uint64 _transformType;
       uint32 _entropyType;
       int _blockId;
       OutputBitStream* _obs;
       XXHash32* _hasher;
       atomic_int* _processedBlockId;
       vector<Listener*> _listeners;
       map<string, string> _ctx;

   public:
       EncodingTask(SliceArray<byte>* iBuffer, SliceArray<byte>* oBuffer, int length,
           uint64 transformType, uint32 entropyType, int blockId,
           OutputBitStream* obs, XXHash32* hasher,
           atomic_int* processedBlockId, vector<Listener*>& listeners,
           map<string, string>& ctx);

       ~EncodingTask(){};

       T call() THROW;
   };

   class CompressedOutputStream : public OutputStream {
       friend class EncodingTask<EncodingTaskResult>;

   private:
       static const int BITSTREAM_TYPE = 0x4B414E5A; // "KANZ"
       static const int BITSTREAM_FORMAT_VERSION = 6;
       static const int COPY_BLOCK_MASK = 0x80;
       static const int TRANSFORMS_MASK = 0x10;
       static const int MIN_BITSTREAM_BLOCK_SIZE = 1024;
       static const int MAX_BITSTREAM_BLOCK_SIZE = 1024 * 1024 * 1024;
       static const int SMALL_BLOCK_SIZE = 15;
       static const int MAX_CONCURRENCY = 64;

       int _blockSize;
       uint8 _nbInputBlocks;
       XXHash32* _hasher;
       SliceArray<byte>* _sa; // for all blocks
       SliceArray<byte>** _buffers; // input & output per block
       uint32 _entropyType;
       uint64 _transformType;
       OutputBitStream* _obs;
       OutputStream& _os;
       atomic_bool _initialized;
       atomic_bool _closed;
       atomic_int _blockId;
       int _jobs;
       vector<Listener*> _listeners;
       map<string, string> _ctx;

       void writeHeader() THROW;

       void processBlock(bool force) THROW;

       static void notifyListeners(vector<Listener*>& listeners, const Event& evt);

   public:
       CompressedOutputStream(OutputStream& os, map<string, string>& ctx);

       ~CompressedOutputStream();

       bool addListener(Listener& bl);

       bool removeListener(Listener& bl);

       ostream& write(const char* s, streamsize n) THROW;

       ostream& put(char c) THROW;

       ostream& flush();

       streampos tellp();

       ostream& seekp(streampos pos) THROW;

       void close() THROW;

       uint64 getWritten();
   };
}
#endif