
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
#include <iomanip>
#include "CompressedInputStream.hpp"
#include "../bitstream/DefaultInputBitStream.hpp"
#include "IOException.hpp"
#include "../IllegalArgumentException.hpp"
#include "Error.hpp"
#include "../entropy/EntropyCodecFactory.hpp"
#include "FunctionFactory.hpp"
#ifdef CONCURRENCY_ENABLED
#include <future>
#endif

using namespace kanzi;

CompressedInputStream::CompressedInputStream(InputStream& is, OutputStream* debug, int jobs)
    : InputStream(is.rdbuf())
    , _is(is)
{
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
    _maxIdx = 0;
    _ds = debug;
    _gcount = 0;
    _ibs = new DefaultInputBitStream(is, DEFAULT_BUFFER_SIZE);
    _jobs = jobs;
    _sa = new SliceArray<byte>(new byte[0], 0, 0);
    _hasher = nullptr;
    _buffers = new SliceArray<byte>*[2 * _jobs];

    for (int i = 0; i < 2 * _jobs; i++)
        _buffers[i] = new SliceArray<byte>(new byte[0], 0, 0);
}

CompressedInputStream::~CompressedInputStream()
{
    try {
        close();
    }
    catch (exception) {
        // Ignore and continue
    }

    for (int i = 0; i < 2 * _jobs; i++)
        delete[] _buffers[i]->_array;

    delete[] _buffers;
    delete _ibs;
    delete[] _sa->_array;
    delete _sa;

    if (_hasher != nullptr) {
        delete _hasher;
        _hasher = nullptr;
    }
}

void CompressedInputStream::readHeader() THROW
{
    // Read stream type
    const int type = int(_ibs->readBits(32));

    // Sanity check
    if (type != BITSTREAM_TYPE) {
        stringstream ss;
        throw IOException("Invalid stream type", Error::ERR_INVALID_FILE);
    }

    // Read stream version
    int version = (int)_ibs->readBits(7);

    // Sanity check
    if (version != BITSTREAM_FORMAT_VERSION) {
        stringstream ss;
        ss << "Invalid bitstream, cannot read this version of the stream: " << version;
        throw IOException(ss.str(), Error::ERR_STREAM_VERSION);
    }

    // Read block checksum
    if (_ibs->readBit() == 1)
        _hasher = new XXHash32(BITSTREAM_TYPE);

    // Read entropy codec
    _entropyType = short(_ibs->readBits(5));

    // Read transform
    _transformType = short(_ibs->readBits(16));

    // Read block size
    _blockSize = int(_ibs->readBits(26)) << 4;

    if ((_blockSize < MIN_BITSTREAM_BLOCK_SIZE) || (_blockSize > MAX_BITSTREAM_BLOCK_SIZE)) {
        stringstream ss;
        ss << "Invalid bitstream, incorrect block size: " << _blockSize;
        throw IOException(ss.str(), Error::ERR_BLOCK_SIZE);
    }

    // Read reserved bits
    _ibs->readBits(9);

    if (_ds != nullptr) {
        *_ds << "Checksum set to " << (_hasher != nullptr ? "true" : "false") << endl;
        *_ds << "Block size set to " << _blockSize << " bytes" << endl;

        try {
            FunctionFactory<byte> ff;
            string w1 = ff.getName(_transformType);

            if (w1 == "NONE")
                w1 = "no";

            *_ds << "Using " << w1 << " transform (stage 1)" << endl;
        }
        catch (IllegalArgumentException&) {
            stringstream ss;
            ss << "Invalid bitstream, unknown transform type: " << _transformType;
            throw IOException(ss.str(), Error::ERR_INVALID_CODEC);
        }

        try {
            string w2 = EntropyCodecFactory::getName(_entropyType);

            if (w2 == "NONE")
                w2 = "no";

            *_ds << "Using " << w2 << " entropy codec (stage 2)" << endl;
        }
        catch (IllegalArgumentException&) {
            stringstream ss;
            ss << "Invalid bitstream, unknown entropy codec type: " << _entropyType;
            throw IOException(ss.str(), Error::ERR_INVALID_CODEC);
        }
    }
}

bool CompressedInputStream::addListener(BlockListener& bl)
{
    _listeners.push_back(&bl);
    return true;
}

bool CompressedInputStream::removeListener(BlockListener& bl)
{
    std::vector<BlockListener*>::iterator it = find(_listeners.begin(), _listeners.end(), &bl);

    if (it == _listeners.end())
        return false;

    _listeners.erase(it);
    return true;
}

int CompressedInputStream::peek() THROW
{
    try {
        if (_sa->_index >= _maxIdx) {
            _maxIdx = processBlock();

            if (_maxIdx == 0) {
                // Reached end of stream
                setstate(ios::eofbit);
                return EOF;
            }
        }

        return _sa->_array[_sa->_index] & 0xFF;
    }
    catch (IOException& e) {
        setstate(ios::badbit);
        throw e;
    }
    catch (exception& e) {
        setstate(ios::badbit);
        throw e;
    }
}

int CompressedInputStream::get() THROW
{
    _gcount = 0;
    int res = peek();

    if (res != EOF) {
        _sa->_index++;
        _gcount++;
    }

    return res;
}

int CompressedInputStream::_get() THROW
{
    int res = peek();

    if (res != EOF) {
        _sa->_index++;
    }

    return res;
}

istream& CompressedInputStream::read(char* data, streamsize length) THROW
{
    int remaining = (int)length;

    if (remaining < 0)
        throw ios_base::failure("Invalid buffer size");

    if (_closed.load() == true) {
        setstate(ios::badbit);
        throw ios_base::failure("Stream closed");
    }

    int off = 0;
    _gcount = 0;

    while (remaining > 0) {
        // Limit to number of available bytes in buffer
        const int lenChunk = (_sa->_index + remaining < _maxIdx) ? remaining : _maxIdx - _sa->_index;

        if (lenChunk > 0) {
            // Process a chunk of in-buffer data. No access to bitstream required
            memcpy(&data[off], &_sa->_array[_sa->_index], lenChunk);
            _sa->_index += lenChunk;
            off += lenChunk;
            remaining -= lenChunk;
            _gcount += lenChunk;

            if (remaining == 0)
                break;
        }

        // Buffer empty, time to decode
        int c2 = _get();

        // EOF ?
        if (c2 == EOF)
            break;

        data[off++] = (byte)c2;
        _gcount++;
        remaining--;
    }

    return *this;
}

streampos CompressedInputStream::tellg()
{
    return _is.tellg();
}

istream& CompressedInputStream::seekp(streampos) THROW
{
    setstate(ios::badbit);
    throw ios_base::failure("Not supported");
}

int CompressedInputStream::processBlock() THROW
{
    vector<DecodingTask<DecodingTaskResult>*> tasks;

    if (!_initialized.exchange(true, memory_order_acquire))
        readHeader();

    try {
        // Add a padding area to manage any block with header (of size <= EXTRA_BUFFER_SIZE)
        const int blkSize = _blockSize + EXTRA_BUFFER_SIZE;

        if (_sa->_length < _jobs * blkSize) {
            _sa->_length = _jobs * blkSize;
            delete[] _sa->_array;
            _sa->_array = new byte[_sa->_length];
        }

        // Protect against future concurrent modification of the list of block listeners
        vector<BlockListener*> blockListeners(_listeners);
        int decoded = 0;
        _sa->_index = 0;
        int firstBlockId = _blockId.load();

        // Create as many tasks as required
        for (int jobId = 0; jobId < _jobs; jobId++) {
            _buffers[2 * jobId]->_index = 0;
            _buffers[2 * jobId + 1]->_index = 0;

            if (_buffers[2 * jobId]->_length < blkSize) {
                delete[] _buffers[2 * jobId]->_array;
                _buffers[2 * jobId]->_array = new byte[blkSize];
                _buffers[2 * jobId]->_length = blkSize;
            }

            DecodingTask<DecodingTaskResult>* task = new DecodingTask<DecodingTaskResult>(_buffers[2 * jobId],
                _buffers[2 * jobId + 1], blkSize, _transformType,
                _entropyType, firstBlockId + jobId + 1, _ibs, _hasher, &_blockId,
                blockListeners);
            tasks.push_back(task);
        }

        if (_jobs == 1) {
            // Synchronous call
            DecodingTask<DecodingTaskResult>* task = tasks.back();
            tasks.pop_back();
            DecodingTaskResult res = task->call();
            int err = res._error;
            string msg = res._msg;

            if (err != 0)
                throw IOException(msg, err);

            memcpy(&_sa->_array[_sa->_index], &res._data[0], res._decoded);
            _sa->_index += res._decoded;
            decoded += res._decoded;

            // Notify after transform ... in block order
            BlockEvent evt(BlockEvent::AFTER_TRANSFORM, res._blockId,
                res._decoded, res._checksum, _hasher != nullptr);

            CompressedInputStream::notifyListeners(blockListeners, evt);
        }
#ifdef CONCURRENCY_ENABLED
        else {
            vector<DecodingTask<DecodingTaskResult>*>::iterator it;
            vector<future<DecodingTaskResult> > results;

            // Register task futures and launch tasks in parallel
            for (it = tasks.begin(); it != tasks.end(); it++) {
                results.push_back(async(&DecodingTask<DecodingTaskResult>::call, *it));
            }

            // Wait for tasks completion and check results
            for (uint i = 0; i < tasks.size(); i++) {
                DecodingTaskResult res = results[i].get();
                int err = res._error;
                string msg = res._msg;

                if (err != 0)
                    throw IOException(msg, err);

                memcpy(&_sa->_array[_sa->_index], &res._data[0], res._decoded);
                _sa->_index += res._decoded;
                decoded += res._decoded;

                // Notify after transform ... in block order
                BlockEvent evt(BlockEvent::AFTER_TRANSFORM, res._blockId,
                    res._decoded, res._checksum, _hasher != nullptr);

                CompressedInputStream::notifyListeners(blockListeners, evt);
            }
        }
#endif

        for (vector<DecodingTask<DecodingTaskResult>*>::iterator it = tasks.begin(); it != tasks.end(); it++)
            delete *it;

        tasks.clear();
        _sa->_index = 0;
        return decoded;
    }
    catch (IOException& e) {
        for (vector<DecodingTask<DecodingTaskResult>*>::iterator it = tasks.begin(); it != tasks.end(); it++)
            delete *it;

        tasks.clear();
        throw e;
    }
    catch (exception& e) {
        for (vector<DecodingTask<DecodingTaskResult>*>::iterator it = tasks.begin(); it != tasks.end(); it++)
            delete *it;

        tasks.clear();
        throw IOException(e.what(), Error::ERR_UNKNOWN);
    }
}

void CompressedInputStream::close() THROW
{
    if (_closed.exchange(true, memory_order_acquire))
        return;

    try {
        _ibs->close();
    }
    catch (BitStreamException& e) {
        throw IOException(e.what(), e.error());
    }

    // Release resources
    // Force error on any subsequent write attempt
    delete[] _sa->_array;
    _sa->_array = new byte[0];
    _sa->_length = 0;
    _sa->_index = -1;

    for (int i = 0; i < _jobs; i++) {
        delete[] _buffers[i]->_array;
        _buffers[i]->_array = new byte[0];
        _buffers[i]->_length = 0;
    }
}

// Return the number of bytes read so far
// Return the number of bytes written so far
uint64 CompressedInputStream::getRead()
{
    return (_ibs->read() + 7) >> 3;
}

void CompressedInputStream::notifyListeners(vector<BlockListener*>& listeners, const BlockEvent& evt)
{
    vector<BlockListener*>::iterator it;

    for (it = listeners.begin(); it != listeners.end(); it++)
        (*it)->processEvent(evt);
}

template <class T>
DecodingTask<T>::DecodingTask(SliceArray<byte>* iBuffer, SliceArray<byte>* oBuffer, int blockSize,
    short transformType, short entropyType, int blockId,
    InputBitStream* ibs, XXHash32* hasher,
    atomic_int* processedBlockId, vector<BlockListener*>& listeners)
{
    _blockLength = blockSize;
    _data = iBuffer;
    _buffer = oBuffer;
    _transformType = transformType;
    _entropyType = entropyType;
    _blockId = blockId;
    _ibs = ibs;
    _hasher = hasher;
    _listeners = listeners;
    _processedBlockId = processedBlockId;
}

// Decode mode + transformed entropy coded data
// mode: 0b1000xxxx => small block (written as is) + 4 LSB for block size (0-15)
//       0x00xxxx00 => transform sequence skip flags (1 means skip)
//       0x000000xx => size(size(block))-1
// Return -1 if error, otherwise the number of bytes read from the encoder
template <class T>
T DecodingTask<T>::call() THROW
{
    int taskId = _processedBlockId->load();

    // Lock free synchronization
    while ((taskId != CompressedInputStream::CANCEL_TASKS_ID) && (taskId != _blockId - 1)) {
        taskId = _processedBlockId->load();
    }

    int checksum1 = 0;

    // Skip, either all data have been processed or an error occured
    if (taskId == CompressedInputStream::CANCEL_TASKS_ID) {
        return DecodingTaskResult(*_data, _blockId, checksum1, 0, 0, "");
    }

    EntropyDecoder* ed = nullptr;

    try {
        // Extract block header directly from bitstream
        uint64 read = _ibs->read();
        byte mode = byte(_ibs->readBits(8));
        int preTransformLength;

        if ((mode & CompressedInputStream::SMALL_BLOCK_MASK) != 0) {
            preTransformLength = mode & CompressedInputStream::COPY_LENGTH_MASK;
        }
        else {
            int dataSize = 1 + (mode & 0x03);
            int length = dataSize << 3;
            uint64 mask = (uint64(1) << length) - 1;
            preTransformLength = int(_ibs->readBits(length) & mask);
        }

        if (preTransformLength == 0) {
            // Last block is empty, return success and cancel pending tasks
            _processedBlockId->store(CompressedInputStream::CANCEL_TASKS_ID);
            return DecodingTaskResult(*_data, _blockId, 0, checksum1, 0, "");
        }

        if ((preTransformLength < 0) || (preTransformLength > CompressedInputStream::MAX_BITSTREAM_BLOCK_SIZE)) {
            // Error => cancel concurrent decoding tasks
            _processedBlockId->store(CompressedInputStream::CANCEL_TASKS_ID);
            stringstream ss;
            ss << "Invalid compressed block length: " << preTransformLength;
            return DecodingTaskResult(*_data, _blockId, 0, checksum1, Error::ERR_READ_FILE, ss.str());
        }

        // Extract checksum from bit stream (if any)
        if (_hasher != nullptr)
            checksum1 = (int)_ibs->readBits(32);

        if (_listeners.size() > 0) {
            // Notify before entropy (block size in bitstream is unknown)
            BlockEvent evt(BlockEvent::BEFORE_ENTROPY, _blockId, -1, checksum1, _hasher != nullptr);
            CompressedInputStream::notifyListeners(_listeners, evt);
        }

        const int bufferSize = (_blockLength >= preTransformLength + CompressedInputStream::EXTRA_BUFFER_SIZE) ? _blockLength : preTransformLength + CompressedInputStream::EXTRA_BUFFER_SIZE;

        if (_buffer->_length < bufferSize) {
            _buffer->_length = bufferSize;
            delete[] _buffer->_array;
            _buffer->_array = new byte[_buffer->_length];
        }

        const int savedIdx = _data->_index;

        // Each block is decoded separately
        // Rebuild the entropy decoder to reset block statistics
        ed = EntropyCodecFactory::newDecoder(*_ibs, _entropyType);

        // Block entropy decode
        if (ed->decode(_buffer->_array, 0, preTransformLength) != preTransformLength) {
            // Error => cancel concurrent decoding tasks
            _processedBlockId->store(CompressedInputStream::CANCEL_TASKS_ID);
            return DecodingTaskResult(*_data, _blockId, 0, checksum1, Error::ERR_PROCESS_BLOCK,
                "Entropy decoding failed");
        }

        delete ed;
        ed = nullptr;

        if (_listeners.size() > 0) {
            // Notify after entropy (block size set to size in bitstream)
            BlockEvent evt(BlockEvent::AFTER_ENTROPY, _blockId,
                (int)((_ibs->read() - read) / 8), checksum1, _hasher != nullptr);

            CompressedInputStream::notifyListeners(_listeners, evt);
        }

        // After completion of the entropy decoding, increment the block id.
        // It unfreezes the task processing the next block (if any)
        (*_processedBlockId)++;

        if (_listeners.size() > 0) {
            // Notify before transform (block size after entropy decoding)
            BlockEvent evt(BlockEvent::BEFORE_TRANSFORM, _blockId,
                preTransformLength, checksum1, _hasher != nullptr);

            CompressedInputStream::notifyListeners(_listeners, evt);
        }

        if ((mode & CompressedInputStream::SMALL_BLOCK_MASK) != 0) {
            if (_buffer->_array != _data->_array)
                memcpy(&_data->_array[savedIdx], &_buffer->_array[0], preTransformLength);

            _buffer->_index = preTransformLength;
            _data->_index = savedIdx + preTransformLength;
        }
        else {
            TransformSequence<byte>* transform = FunctionFactory<byte>::newFunction(_transformType);
            transform->setSkipFlags((byte)((mode >> 2) & TransformSequence<byte>::SKIP_MASK));
            _buffer->_index = 0;

            // Inverse transform
            _buffer->_length = preTransformLength;
            bool res = transform->inverse(*_buffer, *_data, _buffer->_length);
            delete transform;

            if (res == false) {
                return DecodingTaskResult(*_data, _blockId, 0, checksum1, Error::ERR_PROCESS_BLOCK,
                    "Transform inverse failed");
            }
        }

        const int decoded = _data->_index - savedIdx;

        // Verify checksum
        if (_hasher != nullptr) {
            const int checksum2 = _hasher->hash(&_data->_array[savedIdx], decoded);

            if (checksum2 != checksum1) {
                stringstream ss;
                ss << "Corrupted bitstream: expected checksum " << hex << checksum1 << ", found " << hex << checksum2;
                return DecodingTaskResult(*_data, _blockId, decoded, checksum1, Error::ERR_PROCESS_BLOCK, ss.str());
            }
        }

        return DecodingTaskResult(*_data, _blockId, decoded, checksum1, 0, "");
    }
    catch (exception& e) {
        // Make sure to unfreeze next block
        if (_processedBlockId->load() == _blockId - 1)
            (*_processedBlockId)++;

        if (ed != nullptr)
            delete ed;

        return DecodingTaskResult(*_data, _blockId, 0, checksum1, Error::ERR_PROCESS_BLOCK, e.what());
    }
}
