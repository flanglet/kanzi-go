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
#include "IOException.hpp"
#include "../Error.hpp"
#include "../IllegalArgumentException.hpp"
#include "../bitstream/DefaultOutputBitStream.hpp"
#include "../entropy/EntropyCodecFactory.hpp"
#include "../function/FunctionFactory.hpp"

#ifdef CONCURRENCY_ENABLED
#include <future>
#endif

using namespace kanzi;

CompressedOutputStream::CompressedOutputStream(OutputStream& os, map<string, string>& ctx)
    : OutputStream(os.rdbuf())
    , _os(os)
    , _ctx(ctx)
{
    map<string, string>::iterator it;
    it = ctx.find("jobs");
    int tasks = atoi(it->second.c_str());

#ifdef CONCURRENCY_ENABLED
    if ((tasks <= 0) || (tasks > MAX_CONCURRENCY)) {
        stringstream ss;
        ss << "The number of jobs must be in [1.." << MAX_CONCURRENCY << "]";
        throw IllegalArgumentException(ss.str());
    }
#else
    if ((tasks <= 0) || (tasks > 1))
        throw IllegalArgumentException("The number of jobs is limited to 1 in this version");
#endif

    it = ctx.find("codec");
    string entropyCodec = it->second.c_str();
    it = ctx.find("transform");
    string transform = it->second.c_str();
    it = ctx.find("blockSize");
    int bSize = atoi(it->second.c_str());

    if (bSize > MAX_BITSTREAM_BLOCK_SIZE) {
        std::stringstream ss;
        ss << "The block size must be at most " << (MAX_BITSTREAM_BLOCK_SIZE >> 20) << " MB";
        throw IllegalArgumentException(ss.str());
    }

    if (bSize < MIN_BITSTREAM_BLOCK_SIZE) {
        std::stringstream ss;
        ss << "The block size must be at least " << MIN_BITSTREAM_BLOCK_SIZE;
        throw IllegalArgumentException(ss.str());
    }

    if ((bSize & -16) != bSize)
        throw IllegalArgumentException("The block size must be a multiple of 16");

#ifdef CONCURRENCY_ENABLED
    if (uint64(bSize) * uint64(tasks) >= uint64(1 << 31))
        tasks = (1 << 31) / bSize;
#endif

    _blockId = 0;
    _blockSize = bSize;

    // If input size has been provided, calculate the number of blocks
    // in the input data else use 0. A value of 63 means '63 or more blocks'.
    // This value is written to the bitstream header to let the decoder make
    // better decisions about memory usage and job allocation in concurrent
    // decompression scenario.
    it = ctx.find("fileSize");
    int64 fileSize = (it == ctx.end()) ? 0 : atoi(it->second.c_str());
    int nbBlocks = int(fileSize + (bSize - 1)) / bSize;
    _nbInputBlocks = (nbBlocks > 63) ? 63 : nbBlocks;

    _initialized = false;
    _closed = false;
    const int bufferSize = (bSize <= 65536) ? bSize : 65536;
    _obs = new DefaultOutputBitStream(os, bufferSize);
    _entropyType = EntropyCodecFactory::getType(entropyCodec.c_str());
    _transformType = FunctionFactory<byte>::getType(transform.c_str());
    it = ctx.find("checksum");
    string str = it->second;
    bool checksum = str == "TRUE";
    _hasher = (checksum == true) ? new XXHash32(BITSTREAM_TYPE) : nullptr;
    _jobs = tasks;
    _sa = new SliceArray<byte>(new byte[_blockSize], _blockSize, 0); // initially 1 blockSize
    _buffers = new SliceArray<byte>*[2 * _jobs];

    for (int i = 0; i < 2 * _jobs; i++)
        _buffers[i] = new SliceArray<byte>(new byte[0], 0, 0);
}

CompressedOutputStream::~CompressedOutputStream()
{
    try {
        close();
    }
    catch (exception&) {
        // Ignore and continue
    }

    for (int i = 0; i < 2 * _jobs; i++)
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

    if (_obs->writeBits(BITSTREAM_FORMAT_VERSION, 5) != 5)
        throw IOException("Cannot write bitstream version to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits((_hasher != nullptr) ? 1 : 0, 1) != 1)
        throw IOException("Cannot write checksum to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(_entropyType, 5) != 5)
        throw IOException("Cannot write entropy type to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(_transformType, 48) != 48)
        throw IOException("Cannot write transform types to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(_blockSize >> 4, 26) != 26)
        throw IOException("Cannot write block size to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(_nbInputBlocks, 6) != 6)
        throw IOException("Cannot write  number of blocks to header", Error::ERR_WRITE_FILE);

    if (_obs->writeBits(0, 5) != 5)
        throw IOException("Cannot write reserved bits to header", Error::ERR_WRITE_FILE);
}

bool CompressedOutputStream::addListener(Listener& bl)
{
    _listeners.push_back(&bl);
    return true;
}

bool CompressedOutputStream::removeListener(Listener& bl)
{
    std::vector<Listener*>::iterator it = find(_listeners.begin(), _listeners.end(), &bl);

    if (it == _listeners.end())
        return false;

    _listeners.erase(it);
    return true;
}

ostream& CompressedOutputStream::write(const char* data, streamsize length) THROW
{
    int remaining = int(length);

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
            processBlock(false);

        _sa->_array[_sa->_index++] = (byte)c;
        return *this;
    }
    catch (exception& e) {
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
        processBlock(true);

    try {
        // Write end block of size 0
        _obs->writeBits(COPY_BLOCK_MASK, 8);
        _obs->writeBits(0, 8);
        _obs->close();
    }
    catch (exception& e) {
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

    for (int i = 0; i < 2 * _jobs; i++) {
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

void CompressedOutputStream::processBlock(bool force) THROW
{
    if (_sa->_index == 0)
        return;

    if (!force && (_sa->_length < _jobs * _blockSize)) {
        // Grow byte array until max allowed
        byte* buf = new byte[_sa->_length + _blockSize];
        memcpy(buf, _sa->_array, _sa->_length);
        delete[] _sa->_array;
        _sa->_array = buf;
        _sa->_length = _sa->_length + _blockSize;
        return;
    }

    if (!_initialized.exchange(true, memory_order_acquire))
        writeHeader();

    vector<EncodingTask<EncodingTaskResult>*> tasks;

    try {

        // Protect against future concurrent modification of the list of block listeners
        vector<Listener*> blockListeners(_listeners);
        const int dataLength = _sa->_index;
        _sa->_index = 0;
        int firstBlockId = _blockId.load();

        // Create as many tasks as required
        for (int jobId = 0; jobId < _jobs; jobId++) {
            const int sz = (_sa->_index + _blockSize > dataLength) ? dataLength - _sa->_index : _blockSize;

            if (sz == 0)
                break;

            map<string, string> copyCtx(_ctx);
            _buffers[2 * jobId]->_index = 0;
            _buffers[2 * jobId + 1]->_index = 0;

            if (_buffers[2 * jobId]->_length < sz) {
                delete[] _buffers[2 * jobId]->_array;
                _buffers[2 * jobId]->_array = new byte[sz];
                _buffers[2 * jobId]->_length = sz;
            }

            memcpy(&_buffers[2 * jobId]->_array[0], &_sa->_array[_sa->_index], sz);

            EncodingTask<EncodingTaskResult>* task = new EncodingTask<EncodingTaskResult>(_buffers[2 * jobId],
                _buffers[2 * jobId + 1], sz, _transformType,
                _entropyType, firstBlockId + jobId + 1,
                _obs, _hasher, &_blockId,
                blockListeners, copyCtx);
            tasks.push_back(task);
            _sa->_index += sz;
        }

        if (tasks.size() == 1) {
            // Synchronous call
            EncodingTask<EncodingTaskResult>* task = tasks.back();
            tasks.pop_back();
            EncodingTaskResult res = task->call();

            if (res._error != 0)
                throw IOException(res._msg, res._error); // deallocate in catch block
        }
#ifdef CONCURRENCY_ENABLED
        else {
            vector<future<EncodingTaskResult> > futures;

            // Register task futures and launch tasks in parallel
            for (uint i = 0; i < tasks.size(); i++) {
                futures.push_back(async(launch::async, &EncodingTask<EncodingTaskResult>::call, tasks[i]));
            }

            // Wait for tasks completion and check results
            for (uint i = 0; i < tasks.size(); i++) {
                EncodingTaskResult status = futures[i].get();

                if (status._error != 0)
                    throw IOException(status._msg, status._error); // deallocate in catch block
            }
        }
#endif
        for (vector<EncodingTask<EncodingTaskResult>*>::iterator it = tasks.begin(); it != tasks.end(); it++)
            delete *it;

        tasks.clear();
        _sa->_index = 0;
    }
    catch (BitStreamException& e) {
        for (vector<EncodingTask<EncodingTaskResult>*>::iterator it = tasks.begin(); it != tasks.end(); it++)
            delete *it;

        tasks.clear();
        throw IOException(e.what(), e.error());
    }
    catch (IOException& e) {
        for (vector<EncodingTask<EncodingTaskResult>*>::iterator it = tasks.begin(); it != tasks.end(); it++)
            delete *it;

        tasks.clear();
        throw e;
    }
    catch (exception& e) {
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

void CompressedOutputStream::notifyListeners(vector<Listener*>& listeners, const Event& evt)
{
    vector<Listener*>::iterator it;

    for (it = listeners.begin(); it != listeners.end(); it++)
        (*it)->processEvent(evt);
}

template <class T>
EncodingTask<T>::EncodingTask(SliceArray<byte>* iBuffer, SliceArray<byte>* oBuffer, int length,
    uint64 transformType, uint32 entropyType, int blockId,
    OutputBitStream* obs, XXHash32* hasher,
    atomic_int* processedBlockId, vector<Listener*>& listeners,
    map<string, string>& ctx)
    : _ctx(ctx)
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
    _processedBlockId = processedBlockId;
}

// Encode mode + transformed entropy coded data
// mode | 0b10000000 => copy block
//      | 0b0yy00000 => size(size(block))-1
//      | 0b000y0000 => 1 if more than 4 transforms
//  case 4 transforms or less
//      | 0b0000yyyy => transform sequence skip flags (1 means skip)
//  case more than 4 transforms
//      | 0b00000000
//      then 0byyyyyyyy => transform sequence skip flags (1 means skip)
template <class T>
T EncodingTask<T>::call() THROW
{
    EntropyEncoder* ee = nullptr;

    try {
        byte mode = 0;
        int postTransformLength = _blockLength;
        int checksum = 0;

        // Compute block checksum
        if (_hasher != nullptr)
            checksum = _hasher->hash(&_data->_array[_data->_index], _blockLength);

        if (_listeners.size() > 0) {
            // Notify before transform
            Event evt(Event::BEFORE_TRANSFORM, _blockId,
                int64(_blockLength), checksum, _hasher != nullptr, clock());

            CompressedOutputStream::notifyListeners(_listeners, evt);
        }

        if (_blockLength <= CompressedOutputStream::SMALL_BLOCK_SIZE) {
            _transformType = FunctionFactory<byte>::NONE_TYPE;
            _entropyType = EntropyCodecFactory::NONE_TYPE;
            mode |= CompressedOutputStream::COPY_BLOCK_MASK;
        }
        else {
            bool skipHighEntropyBlocks = false;
            map<string, string>::iterator it = _ctx.find("skipBlocks");

            if (it != _ctx.end()) {
                string str = it->second;
                transform(str.begin(), str.end(), str.begin(), ::toupper);
                skipHighEntropyBlocks = str == "TRUE";
            }

            if (skipHighEntropyBlocks == true) {
                int histo[256];
                const int entropy = EntropyUtils::computeFirstOrderEntropy1024(&_data->_array[_data->_index], _blockLength, histo);
                //_ctx["histo0"] = toString(histo, 256);

                if (entropy >= EntropyUtils::INCOMPRESSIBLE_THRESHOLD) {
                    _transformType = FunctionFactory<byte>::NONE_TYPE;
                    _entropyType = EntropyCodecFactory::NONE_TYPE;
                    mode |= CompressedOutputStream::COPY_BLOCK_MASK;
                }
            }
        }

        stringstream ss;
        ss << _blockLength;
        _ctx["size"] = ss.str();
        TransformSequence<byte>* transform = FunctionFactory<byte>::newFunction(_ctx, _transformType);
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
        postTransformLength = _buffer->_index;

        if (postTransformLength < 0)
            return T(_blockId, Error::ERR_WRITE_FILE, "Invalid transform size");

        ss.str(string());
        ss << postTransformLength;
        _ctx["size"] = ss.str();
        int dataSize = 0;

        for (uint64 n = 0xFF; n < uint64(postTransformLength); n <<= 8)
            dataSize++;

        if (dataSize > 3)
            return T(_blockId, Error::ERR_WRITE_FILE, "Invalid block data length");

        // Record size of 'block size' - 1 in bytes
        mode |= ((dataSize & 0x03) << 5);
        dataSize++;

        if (_listeners.size() > 0) {
            // Notify after transform
            Event evt(Event::AFTER_TRANSFORM, _blockId,
                int64(postTransformLength), checksum, _hasher != nullptr, clock());

            CompressedOutputStream::notifyListeners(_listeners, evt);
        }

        // Lock free synchronization
        while (_processedBlockId->load() != _blockId - 1) {
            // Busy loop
        }

        // Write block 'header' (mode + compressed length);
        uint64 written = _obs->written();

        if (((mode & CompressedOutputStream::COPY_BLOCK_MASK) != 0) || (transform->getNbFunctions() <= 4)) {
            mode |= (uint8(transform->getSkipFlags()) >> 4);
            _obs->writeBits(mode, 8);
        }
        else {
            mode |= CompressedOutputStream::TRANSFORMS_MASK;
            _obs->writeBits(mode, 8);
            _obs->writeBits(transform->getSkipFlags(), 8);
        }

        delete transform;
        _obs->writeBits(postTransformLength, 8 * dataSize);

        // Write checksum
        if (_hasher != nullptr)
            _obs->writeBits(checksum, 32);

        if (_listeners.size() > 0) {
            // Notify before entropy
            Event evt(Event::BEFORE_ENTROPY, _blockId,
                int64(postTransformLength), checksum, _hasher != nullptr, clock());

            CompressedOutputStream::notifyListeners(_listeners, evt);
        }

        // Each block is encoded separately
        // Rebuild the entropy encoder to reset block statistics
        ee = EntropyCodecFactory::newEncoder(*_obs, _ctx, _entropyType);

        // Entropy encode block
        if (ee->encode(_buffer->_array, 0, postTransformLength) != postTransformLength)
            return T(_blockId, Error::ERR_PROCESS_BLOCK, "Entropy coding failed");

        // Dispose before processing statistics. Dispose may write to the bitstream
        delete ee;
        ee = nullptr;

        // After completion of the entropy coding, increment the block id.
        // It unfreezes the task processing the next block (if any)
        (*_processedBlockId)++;

        if (_listeners.size() > 0) {
            // Notify after entropy
            const int w = int((_obs->written() - written) / 8);

            Event evt(Event::AFTER_ENTROPY,
                int64(_blockId), w, checksum, _hasher != nullptr, clock());

            CompressedOutputStream::notifyListeners(_listeners, evt);
        }

        return T(_blockId, 0, "Success");
    }
    catch (exception& e) {
        // Make sure to unfreeze next block
        if (_processedBlockId->load() == _blockId - 1)
            (*_processedBlockId)++;

        if (ee != nullptr)
            delete ee;

        return T(_blockId, Error::ERR_PROCESS_BLOCK, e.what());
    }
}