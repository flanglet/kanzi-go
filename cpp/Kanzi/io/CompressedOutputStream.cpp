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

#include <sstream>
#include "CompressedOutputStream.hpp"
#include "../bitstream/DefaultOutputBitStream.hpp"
#include "Error.hpp"
#include "../entropy/EntropyCodecFactory.hpp"
#include "FunctionFactory.hpp"
#include "../io/IOException.hpp"
#include "../IllegalArgumentException.hpp"

using namespace kanzi;

CompressedOutputStream::CompressedOutputStream(const string& entropyCodec, const string& transform,
    OutputStream& os, int blockSize, bool checksum, ThreadPool<EncodingTaskResult>& pool,
    int jobs)
    : OutputStream(os.rdbuf())
    , _os(os)
    , _pool(pool)
{
    if (blockSize > MAX_BITSTREAM_BLOCK_SIZE) {
        std::stringstream ss;
        ss << "The block size must be at most " << (MAX_BITSTREAM_BLOCK_SIZE >> 20) << " MB";
        throw IllegalArgumentException(ss.str());
    }

    if (blockSize < MIN_BITSTREAM_BLOCK_SIZE) {
        std::stringstream ss;
        ss << "The block size must be at least " << MIN_BITSTREAM_BLOCK_SIZE;
        throw IllegalArgumentException(ss.str());
    }

    if ((blockSize & -16) != blockSize)
        throw IllegalArgumentException("The block size must be a multiple of 16");

#ifndef CONCURRENCY_ENABLED
    if (jobs != 1)
        throw IllegalArgumentException("The number of jobs is limited to 1 in this version");
#else
    if ((jobs < 1) || (jobs > 16))
        throw IllegalArgumentException("The number of jobs must be in [1..16]");
#endif

    _blockId = 0;
    _initialized = false;
    _closed = false;
    const int bufferSize = (blockSize <= 65536) ? blockSize : 65536;
    _obs = new DefaultOutputBitStream(os, bufferSize);
    _entropyType = EntropyCodecFactory::getType(entropyCodec.c_str());
    FunctionFactory<byte> ff;
    _transformType = ff.getType(transform.c_str());
    _blockSize = blockSize;
    _hasher = (checksum == true) ? new XXHash32(BITSTREAM_TYPE) : nullptr;
    _jobs = jobs;
    _sa = new SliceArray<byte>(new byte[blockSize * _jobs], blockSize * _jobs, 0);
    _buffers = new SliceArray<byte>*[2*_jobs];

    for (int i = 0; i < 2*_jobs; i++)
        _buffers[i] = new SliceArray<byte>(new byte[0], 0, 0);
}

CompressedOutputStream::~CompressedOutputStream()
{
    try {
        close();
    }
    catch (exception) {
        // Ignore and continue
    }

    for (int i = 0; i < 2*_jobs; i++)
        delete[] _buffers[i]->_array;

    delete[] _buffers;
    delete _obs;
    delete[] _sa->_array;
    delete _sa;

    if (_hasher != nullptr) {
        delete _hasher;
        _hasher = nullptr;
    }
}

void CompressedOutputStream::writeHeader() THROW
{
    if (_obs->writeBits(BITSTREAM_TYPE, 32) != 32)
        throw IOException("Cannot write bitstream type to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(BITSTREAM_FORMAT_VERSION, 7) != 7)
        throw IOException("Cannot write bitstream version to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits((_hasher != nullptr) ? 1 : 0, 1) != 1)
        throw IOException("Cannot write checksum to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(_entropyType, 5) != 5)
        throw IOException("Cannot write entropy type to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(_transformType, 16) != 16)
        throw IOException("Cannot write transform types to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(_blockSize >> 4, 26) != 26)
        throw IOException("Cannot write block size to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(0L, 9) != 9)
        throw IOException("Cannot write reserved bits to header", Error::ERR_WRITE_FILE);
}

bool CompressedOutputStream::addListener(BlockListener& bl)
{
    _listeners.push_back(&bl);
    return true;
}

bool CompressedOutputStream::removeListener(BlockListener& bl)
{
    std::vector<BlockListener*>::iterator it = find(_listeners.begin(), _listeners.end(), &bl);

    if (it == _listeners.end())
        return false;

    _listeners.erase(it);
    return true;
}

ostream& CompressedOutputStream::write(const char* data, streamsize length) THROW
{
    int remaining = (int)length;

    if (remaining < 0)
        throw IOException("Invalid buffer size");

    if (_closed.load() == true)
        throw ios_base::failure("Stream closed");

    int off = 0;

    while (remaining > 0) {
        // Limit to number of available bytes in buffer
        const int lenChunk = (_sa->_index + remaining < _sa->_length) ? remaining : _sa->_length - _sa->_index;

        if (lenChunk > 0) {
            // Process a chunk of in-buffer data. No access to bitstream required
            memcpy(&_sa->_array[_sa->_index], &data[off], lenChunk);
            _sa->_index += lenChunk;
            off += lenChunk;
            remaining -= lenChunk;

            if (remaining == 0)
                break;
        }

        // Buffer full, time to encode
        put(data[off]);
        off++;
        remaining--;
    }

    return *this;
}

ostream& CompressedOutputStream::put(char c) THROW
{
    try {
        // If the buffer is full, time to encode
        if (_sa->_index >= _sa->_length)
            processBlock();

        _sa->_array[_sa->_index++] = (byte)c;
        return *this;
    }
    catch (exception e) {
        setstate(ios::badbit);
        throw ios_base::failure(e.what());
    }
}

ostream& CompressedOutputStream::flush()
{
    // Let the bitstream of the entropy encoder flush itself when needed
    return *this;
}

void CompressedOutputStream::close() THROW
{
    if (_closed.exchange(true, memory_order_acquire))
        return;

    if (_sa->_index > 0)
        processBlock();

    try {
        // Write end block of size 0
        _obs->writeBits(SMALL_BLOCK_MASK, 8);
        _obs->close();
    }
    catch (exception e) {
        setstate(ios::badbit);
        throw ios_base::failure(e.what());
    }

    setstate(ios::eofbit);

    // Release resources
    // Force error on any subsequent write attempt
    delete[] _sa->_array;
    _sa->_array = new byte[0];
    _sa->_length = 0;
    _sa->_index = -1;

    for (int i = 0; i < 2*_jobs; i++) {
        delete[] _buffers[i]->_array;
        _buffers[i]->_array = new byte[0];
        _buffers[i]->_length = 0;
    }
}

streampos CompressedOutputStream::tellp()
{
    return _os.tellp();
}

ostream& CompressedOutputStream::seekp(streampos) THROW
{
    setstate(ios::badbit);
    throw ios_base::failure("Not supported");
}

void CompressedOutputStream::processBlock() THROW
{
    if (_sa->_index == 0)
        return;

    if (!_initialized.exchange(true, memory_order_acquire))
        writeHeader();

    vector<EncodingTask<EncodingTaskResult>*> tasks(_jobs);

    try {

        // Protect against future concurrent modification of the list of block listeners
        vector<BlockListener*> blockListeners(_listeners);
        const int dataLength = _sa->_index;
        _sa->_index = 0;
        int firstBlockId = _blockId.load();

        // Create as many tasks as required
        for (int jobId = 0; jobId < _jobs; jobId++) {
            const int sz = (_sa->_index + _blockSize > dataLength) ? dataLength - _sa->_index : _blockSize;

            if (sz == 0)
                break;

            _buffers[2*jobId]->_index = 0;
            _buffers[2*jobId+1]->_index = 0;
              		              
            if (_buffers[2*jobId]->_length < sz)
            {
                delete[] _buffers[2*jobId]->_array;
                _buffers[2*jobId]->_array = new byte[sz];
                _buffers[2*jobId]->_length = sz;
            }
 
            memcpy(&_buffers[2*jobId]->_array[0], &_sa->_array[_sa->_index], sz);

            EncodingTask<EncodingTaskResult>* task = new EncodingTask<EncodingTaskResult>(_buffers[2*jobId],
                _buffers[2*jobId+1], sz, _transformType,
                _entropyType, firstBlockId + jobId + 1,
                _obs, _hasher, &_blockId,
                blockListeners);
            tasks.push_back(task);
            _sa->_index += sz;
        }

        if (_jobs == 1) {
            // Synchronous call
            EncodingTask<EncodingTaskResult>* task = tasks.back();
            tasks.pop_back();
            EncodingTaskResult status = task->call();
            delete task;

            if (status._error != 0)
                throw IOException(status._msg, status._error);
        }
#ifdef CONCURRENCY_ENABLED
        else {
            vector<EncodingTask<EncodingTaskResult>*>::iterator it;

            // Add tasks to thread pool
            for (it = tasks.begin(); it != tasks.end(); it++) {
                _pool.add(*it);
            }

            // Wait for completion of all tasks
            while (_pool.active_tasks() != 0) {
            }

            // Check results
            while (tasks.size() > 0) {
                it = tasks.begin();
                tasks.erase(it);
                EncodingTask<EncodingTaskResult>* task = *it;
                EncodingTaskResult status = task->result();
                delete task;

                if (status._error != 0)
                    throw IOException(status._msg, status._error);
            }
        }
#endif

        _sa->_index = 0;
    }
    catch (BitStreamException e) {
        for (vector<EncodingTask<EncodingTaskResult>*>::iterator it = tasks.begin(); it != tasks.end(); it++)
            delete *it;

        tasks.clear();
        throw IOException(e.what(), e.error());
    }
    catch (exception e) {
        for (vector<EncodingTask<EncodingTaskResult>*>::iterator it = tasks.begin(); it != tasks.end(); it++)
            delete *it;

        tasks.clear();
        throw IOException(e.what(), Error::ERR_UNKNOWN);
    }
}

// Return the number of bytes written so far
uint64 CompressedOutputStream::getWritten()
{
    return (_obs->written() + 7) >> 3;
}

void CompressedOutputStream::notifyListeners(vector<BlockListener*>& listeners, const BlockEvent& evt)
{
    vector<BlockListener*>::iterator it;

    for (it = listeners.begin(); it != listeners.end(); it++)
        (*it)->processEvent(evt);
}

template <class T>
EncodingTask<T>::EncodingTask(SliceArray<byte>* iBuffer, SliceArray<byte>* oBuffer, int length,
    short transformType, short entropyType, int blockId,
    OutputBitStream* obs, XXHash32* hasher,
    atomic_int* processedBlockId, vector<BlockListener*>& listeners)
{
    _data = iBuffer;
    _buffer = oBuffer;
    _blockLength = length;
    _transformType = transformType;
    _entropyType = entropyType;
    _blockId = blockId;
    _obs = obs;
    _hasher = hasher;
    _listeners = listeners;
    _result = nullptr;
    _processedBlockId = processedBlockId;
}

template <class T>
EncodingTask<T>::~EncodingTask()
{
    if (_result != nullptr)
        delete _result;

    _result = nullptr;
}

template <class T>
T EncodingTask<T>::result()
{
    if (_result == nullptr)
        throw ios_base::failure("No result available");

    return *_result;
}

// Encode mode + transformed entropy coded data
// mode: 0b1000xxxx => small block (written as is) + 4 LSB for block size (0-15)
//       0x00xxxx00 => transform sequence skip flags (1 means skip)
//       0x000000xx => size(size(block))-1
template <class T>
T EncodingTask<T>::call() THROW
{
    EntropyEncoder* ee = nullptr;

    try {
        byte mode = 0;
        int dataSize = 0;
        int postTransformLength = _blockLength;
        int checksum = 0;

        // Compute block checksum
        if (_hasher != nullptr)
            checksum = _hasher->hash(&_data->_array[_data->_index], _blockLength);

        if (_listeners.size() > 0) {
            // Notify before transform
            BlockEvent evt(BlockEvent::BEFORE_TRANSFORM, _blockId,
                _blockLength, checksum, _hasher != nullptr);

            CompressedOutputStream::notifyListeners(_listeners, evt);
        }

        if (_blockLength <= CompressedOutputStream::SMALL_BLOCK_SIZE) {
            // Just copy
            if (_data->_array != _buffer->_array) {
                if (_buffer->_length < _blockLength) {
                    _buffer->_length = _blockLength;
                    delete[] _buffer->_array;
                    _buffer->_array = new byte[_buffer->_length];
                }

                memcpy(&_buffer->_array[0], &_data->_array[_data->_index], _blockLength);
            }

            _data->_index += _blockLength;
            _buffer->_index = _blockLength;
            mode = (byte)(CompressedOutputStream::SMALL_BLOCK_MASK | (_blockLength & CompressedOutputStream::COPY_LENGTH_MASK));
        }
        else {
            TransformSequence<byte>* transform = FunctionFactory<byte>::newFunction(_transformType);
            int requiredSize = transform->getMaxEncodedLength(_blockLength);

            if (_buffer->_length < requiredSize) {
                _buffer->_length = requiredSize;
                delete[] _buffer->_array;
                _buffer->_array = new byte[_buffer->_length];
            }

            // Forward transform (ignore error, encode skipFlags)
            _buffer->_index = 0;
            _data->_length = _blockLength;
            transform->forward(*_data, *_buffer, _data->_length);
            mode |= ((transform->getSkipFlags() & TransformSequence<byte>::SKIP_MASK) << 2);
            postTransformLength = _buffer->_index;
            delete transform;

            if (postTransformLength < 0)
                return EncodingTaskResult(_blockId, Error::ERR_WRITE_FILE, "Invalid transform size");

            for (long n = 0xFF; n < postTransformLength; n <<= 8)
                dataSize++;

            if (dataSize > 3)
                return EncodingTaskResult(_blockId, Error::ERR_WRITE_FILE, "Invalid block data length");

            // Record size of 'block size' - 1 in bytes
            mode |= (dataSize & 0x03);
            dataSize++;
        }

        if (_listeners.size() > 0) {
            // Notify after transform
            BlockEvent evt(BlockEvent::AFTER_TRANSFORM, _blockId,
                postTransformLength, checksum, _hasher != nullptr);

            CompressedOutputStream::notifyListeners(_listeners, evt);
        }

        // Lock free synchronization
        while (_processedBlockId->load() != _blockId - 1) {
            // Busy loop
        }

        // Write block 'header' (mode + compressed length);
        uint64 written = _obs->written();
        _obs->writeBits(mode, 8);

        if (dataSize > 0)
            _obs->writeBits(postTransformLength, 8 * dataSize);

        // Write checksum
        if (_hasher != nullptr)
            _obs->writeBits(checksum, 32);

        if (_listeners.size() > 0) {
            // Notify before entropy
            BlockEvent evt(BlockEvent::BEFORE_ENTROPY, _blockId,
                postTransformLength, checksum, _hasher != nullptr);

            CompressedOutputStream::notifyListeners(_listeners, evt);
        }

        // Each block is encoded separately
        // Rebuild the entropy encoder to reset block statistics
        ee = EntropyCodecFactory::newEncoder(*_obs, _entropyType);

        // Entropy encode block
        if (ee->encode(_buffer->_array, 0, postTransformLength) != postTransformLength)
            return EncodingTaskResult(_blockId, Error::ERR_PROCESS_BLOCK, "Entropy coding failed");

        // Dispose before processing statistics. Dispose may write to the bitstream
        delete ee;
        ee = nullptr;

        const int w = (int)((_obs->written() - written) / 8);

        // After completion of the entropy coding, increment the block id.
        // It unfreezes the task processing the next block (if any)
        (*_processedBlockId)++;

        if (_listeners.size() > 0) {
            // Notify after entropy
            BlockEvent evt(BlockEvent::AFTER_ENTROPY,
                _blockId, w, checksum, _hasher != nullptr);

            CompressedOutputStream::notifyListeners(_listeners, evt);
        }

        _result = new EncodingTaskResult(_blockId, 0, "Success");
        return result();
    }
    catch (exception e) {
        // Make sure to unfreeze next block
        if (_processedBlockId->load() == _blockId - 1)
            (*_processedBlockId)++;

        if (ee != nullptr)
            delete ee;

        _result = new EncodingTaskResult(_blockId, Error::ERR_PROCESS_BLOCK, e.what());
        return result();
    }
}
