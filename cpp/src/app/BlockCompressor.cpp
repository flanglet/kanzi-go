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

#include <algorithm>
#include <cstdlib>
#include <stdio.h>
#include <stdlib.h>
#include <fstream>
#include <iostream>
#include <string>
#include <sys/stat.h>
#include "BlockCompressor.hpp"
#include "InfoPrinter.hpp"
#include "../io/Error.hpp"
#include "../io/FunctionFactory.hpp"
#include "../IllegalArgumentException.hpp"
#include "../io/IOException.hpp"
#include "../io/NullOutputStream.hpp"
#include "../SliceArray.hpp"
#ifdef CONCURRENCY_ENABLED
#include <future>
#endif

using namespace kanzi;

BlockCompressor::BlockCompressor(map<string, string>& args)
{
    map<string, string>::iterator it;
    it = args.find("level");
    _level = atoi(it->second.c_str());
    args.erase(it);
    it = args.find("overwrite");

    if (it == args.end()) {
        _overwrite = false;
    }
    else {
        string str = it->second;
        transform(str.begin(), str.end(), str.begin(), ::toupper);
        _overwrite = str == "TRUE";
        args.erase(it);
    }

    it = args.find("inputName");
    _inputName = it->second;
    args.erase(it);
    it = args.find("outputName");
    _outputName = it->second;
    args.erase(it);
    string strTransf;
    string strCodec;

    it = args.find("entropy");

    if (it == args.end()) {
        strCodec = "ANS0";
    }
    else {
        strCodec = it->second;
        args.erase(it);
    }

    if (_level >= 0) {
        string tranformAndCodec[2];
        getTransformAndCodec(_level, tranformAndCodec);
        strTransf = tranformAndCodec[0];
        strCodec = tranformAndCodec[1];
    }

    _codec = strCodec;
    it = args.find("block");

    if (it == args.end()) {
        _blockSize = DEFAULT_BLOCK_SIZE;
    }
    else {
        _blockSize = atoi(it->second.c_str());
        args.erase(it);
    }

    it = args.find("transform");

    if (it == args.end()) {
        if (strTransf.length() == 0)
            strTransf = "BWT+RANK+ZRLT";
    }
    else {
        if (strTransf.length() == 0)
            strTransf = it->second;

        args.erase(it);
    }

    // Extract transform names. Curate input (EG. NONE+NONE+xxxx => xxxx)
    FunctionFactory<byte> bff;
    _transform = bff.getName(bff.getType(strTransf.c_str()));
    it = args.find("checksum");

    if (it == args.end()) {
        _checksum = false;
    }
    else {
        string str = it->second;
        transform(str.begin(), str.end(), str.begin(), ::toupper);
        _checksum = str == "TRUE";
        args.erase(it);
    }

    it = args.find("jobs");
    int concurrency = atoi(it->second.c_str());
    _jobs = (concurrency == 0) ? DEFAULT_CONCURRENCY : concurrency;
    args.erase(it);

#ifndef CONCURRENCY_ENABLED
    if (_jobs > 1)
        throw IllegalArgumentException("The number of jobs is limited to 1 in this version");
#endif

    it = args.find("verbose");
    _verbosity = atoi(it->second.c_str());
    args.erase(it);

    if ((_verbosity > 0) && (args.size() > 0)) {
        for (it = args.begin(); it != args.end(); it++) {
            stringstream ss;
            ss << "Ignoring invalid option [" << it->first << "]";
            printOut(ss.str().c_str(), _verbosity > 0);
        }
    }
}

BlockCompressor::~BlockCompressor()
{
    dispose();

    while (_listeners.size() > 0) {
        vector<Listener*>::iterator it = _listeners.begin();
        delete *it;
        _listeners.erase(it);
    }
}

void BlockCompressor::dispose()
{
}

int BlockCompressor::call()
{
    vector<string> files;
    Clock clock;

    try {
        createFileList(_inputName, files);
    }
    catch (exception& e) {
        cerr << e.what() << endl;
        return Error::ERR_OPEN_FILE;
    }

    if (files.size() == 0) {
        cerr << "Cannot access input file '" << _inputName << "'" << endl;
        return Error::ERR_OPEN_FILE;
    }

    // Sort files by name to ensure same order each time
    sort(files.begin(), files.end());
    int nbFiles = int(files.size());

    // Limit verbosity level when files are processed concurrently
    if ((_jobs > 1) && (nbFiles > 1) && (_verbosity > 1)) {
        printOut("Warning: limiting verbosity to 1 due to concurrent processing of input files.\n", _verbosity > 1);
        _verbosity = 1;
    }

    if (_verbosity > 2)
        addListener(new InfoPrinter(_verbosity, InfoPrinter::ENCODING, cout));

    bool printFlag = _verbosity > 2;
    printOut("\n", printFlag);

    stringstream ss;
    string strFiles = (nbFiles > 1) ? " files" : " file";
    ss << nbFiles << strFiles << " to compress\n";
    printOut(ss.str().c_str(), _verbosity > 0);
    ss.str(string());
    ss << "Block size set to " << _blockSize << " bytes";
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Verbosity set to " << _verbosity;
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Overwrite set to " << (_overwrite ? "true" : "false");
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Checksum set to " << (_checksum ? "true" : "false");
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());

    if (_level < 0) {
        string etransform = _transform;
        transform(etransform.begin(), etransform.end(), etransform.begin(), ::toupper);
        ss << "Using " << ((etransform == "NONE") ? "no" : _transform) << " transform (stage 1)";
        printOut(ss.str().c_str(), printFlag);
        ss.str(string());
        string ecodec = _codec;
        transform(ecodec.begin(), ecodec.end(), ecodec.begin(), ::toupper);
        ss << "Using " << ((ecodec == "NONE") ? "no" : _codec) << " entropy codec (stage 2)";
        printOut(ss.str().c_str(), printFlag);
        ss.str(string());
    }
    else {
        ss << "Compression level set to " << _level;
        printOut(ss.str().c_str(), printFlag);
        ss.str(string());
    }

    ss << "Using " << _jobs << " job" << ((_jobs > 1) ? "s" : "");
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());

    string outputName = _outputName;
    transform(outputName.begin(), outputName.end(), outputName.begin(), ::toupper);

    if ((_jobs > 1) && (outputName.compare("STDOUT") == 0)) {
        cerr << "Cannot output to STDOUT with multiple jobs" << endl;
        return Error::ERR_CREATE_FILE;
    }

    int res = 0;
    uint64 read = 0;
    uint64 written = 0;

    // Run the task(s)
    if (nbFiles == 1) {
        string iName = files[0];
        string oName = _outputName;

        if (oName.size() == 0)
            oName = iName + ".knz";

        FileCompressTask<FileCompressResult> task(_verbosity,
            _overwrite, _checksum, iName, oName, _codec,
            _transform, _blockSize, 1, _listeners);

        FileCompressResult fcr = task.call();
        res = fcr._code;
        read = fcr._read;
        written = fcr._written;
    }
    else {
        if ((_outputName.length() > 0) && (outputName.compare("NONE") != 0)) {
            cerr << "Output file cannot be provided when input is a directory (except 'NONE')" << endl;
            return Error::ERR_CREATE_FILE;
        }

        vector<FileCompressTask<FileCompressResult>*> tasks;

        // Create one task per file
        for (int i = 0; i < nbFiles; i++) {
            string iName = files[i];
            string oName = (_outputName.length() > 0) ? "NONE" : iName + ".knz";
            FileCompressTask<FileCompressResult>* task = new FileCompressTask<FileCompressResult>(_verbosity, _overwrite, _checksum,
                iName, oName, _codec, _transform, _blockSize, 1, _listeners);
            tasks.push_back(task);
        }

        bool doConcurrent = _jobs > 1;

#ifdef CONCURRENCY_ENABLED
        if (doConcurrent) {
            vector<FileCompressWorker<FileCompressTask<FileCompressResult>*, FileCompressResult>*> workers;
            vector<future<FileCompressResult> > results;
            BoundedConcurrentQueue<FileCompressTask<FileCompressResult>*, FileCompressResult> queue(nbFiles, &tasks[0]);

            // Create one worker per job and run it. A worker calls several tasks sequentially.
            for (int i = 0; i < _jobs; i++) {
                workers.push_back(new FileCompressWorker<FileCompressTask<FileCompressResult>*, FileCompressResult>(&queue));
                results.push_back(async(launch::async, &FileCompressWorker<FileCompressTask<FileCompressResult>*, FileCompressResult>::call, workers[i]));
            }

            // Wait for results
            for (int i = 0; i < _jobs; i++) {
                FileCompressResult fcr = results[i].get();
                res = fcr._code;
                read += fcr._read;
                written += fcr._written;

                if (res != 0) {
                    // Exit early by telling the workers that the queue is empty
                    queue.clear();
                    break;
                }
            }

            for (int i = 0; i < _jobs; i++)
                delete workers[i];
        }
#endif

        if (!doConcurrent) {
            for (uint i = 0; i < tasks.size(); i++) {
                FileCompressResult fcr = tasks[i]->call();
                res = fcr._code;
                read += fcr._read;
                written += fcr._written;

                if (res != 0)
                    break;
            }
        }

        for (int i = 0; i < nbFiles; i++)
            delete tasks[i];
    }

    clock.stop();

    if (nbFiles > 1) {
        double delta = clock.elapsed();
        printOut("", _verbosity > 0);
        ss << "Total encoding time: " << uint64(delta) << " ms";
        printOut(ss.str().c_str(), _verbosity > 0);
        ss.str(string());
        ss << "Total output size: " << written << " byte" << ((written > 1) ? "s" : "");
        printOut(ss.str().c_str(), _verbosity > 0);
        ss.str(string());

        if (read > 0) {
            ss << "Compression ratio: " << float(written) / float(read);
            printOut(ss.str().c_str(), _verbosity > 0);
            ss.str(string());
        }
    }

    return res;
}

bool BlockCompressor::addListener(Listener* bl)
{
    if (bl == nullptr)
        return false;

    _listeners.push_back(bl);
    return true;
}

bool BlockCompressor::removeListener(Listener* bl)
{
    std::vector<Listener*>::iterator it = find(_listeners.begin(), _listeners.end(), bl);

    if (it == _listeners.end())
        return false;

    _listeners.erase(it);
    return true;
}

void BlockCompressor::notifyListeners(vector<Listener*>& listeners, const Event& evt)
{
    vector<Listener*>::iterator it;

    for (it = listeners.begin(); it != listeners.end(); it++)
        (*it)->processEvent(evt);
}

void BlockCompressor::getTransformAndCodec(int level, string tranformAndCodec[2])
{
    switch (level) {
    case 0:
        tranformAndCodec[0] = "NONE";
        tranformAndCodec[1] = "NONE";
        return;

    case 1:
        tranformAndCodec[0] = "TEXT+LZ4";
        tranformAndCodec[1] = "HUFFMAN";
        return;

    case 2:
        tranformAndCodec[0] = "BWT+RANK+ZRLT";
        tranformAndCodec[1] = "ANS0";
        return;

    case 3:
        tranformAndCodec[0] = "BWT+RANK+ZRLT";
        tranformAndCodec[1] = "FPAQ";
        return;

    case 4:
        tranformAndCodec[0] = "BWT";
        tranformAndCodec[1] = "CM";
        return;

    case 5:
        tranformAndCodec[0] = "X86+RLT+TEXT";
        tranformAndCodec[1] = "TPAQ";
        return;

    default:
        tranformAndCodec[0] = "Unknown";
        tranformAndCodec[1] = "Unknown";
        return;
    }
}

template <class T>
FileCompressTask<T>::FileCompressTask(int verbosity, bool overwrite, bool checksum,
    const string& inputName, const string& outputName, const string& codec,
    const string& transform, int blockSize, int jobs, vector<Listener*> listeners)
{
    _verbosity = verbosity;
    _overwrite = overwrite;
    _checksum = checksum;
    _inputName = inputName;
    _outputName = outputName;
    _codec = codec;
    _transform = transform;
    _blockSize = blockSize;
    _jobs = jobs;
    _listeners = listeners;
}

template <class T>
T FileCompressTask<T>::call()
{
    bool printFlag = _verbosity > 2;
    stringstream ss;
    ss << "Input file name set to '" << _inputName << "'";
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Output file name set to '" << _outputName << "'";
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());

    OutputStream* os = nullptr;

    try {
        string str = _outputName;
        transform(str.begin(), str.end(), str.begin(), ::toupper);

        if (str.compare(0, 4, "NONE") == 0) {
            os = new NullOutputStream();
        }
        else if (str.compare(0, 6, "STDOUT") == 0) {
            os = &cout;
        }
        else {
            if (samePaths(_inputName, _outputName)) {
                cerr << "The input and output files must be different" << endl;
                return T(Error::ERR_CREATE_FILE, 0, 0);
            }

            struct stat buffer;

            if (stat(_outputName.c_str(), &buffer) == 0) {
                if ((buffer.st_mode & S_IFDIR) != 0) {
                    cerr << "The output file is a directory" << endl;
                    return T(Error::ERR_OUTPUT_IS_DIR, 0, 0);
                }

                if (_overwrite == false) {
                    cerr << "The output file exists and the 'force' command "
                         << "line option has not been provided" << endl;
                    return T(Error::ERR_OVERWRITE_FILE, 0, 0);
                }
            }

            os = new ofstream(_outputName.c_str(), ofstream::binary);

            if (!*os) {
                cerr << "Cannot open output file '" << _outputName + "' for writing: " << endl;
                return T(Error::ERR_CREATE_FILE, 0, 0);
            }
        }

        try {
            map<string, string> ctx;
            stringstream ss;
            ss << _blockSize;
            ctx["blockSize"] = ss.str();
            ctx["checksum"] = (_checksum == true) ? "TRUE" : "FALSE";
            ss.str(string());
            ss << _jobs;
            ctx["jobs"] = ss.str();
            ctx["codec"] = _codec;
            ctx["transform"] = _transform;
            _cos = new CompressedOutputStream(*os, ctx);

            for (uint i = 0; i < _listeners.size(); i++)
                _cos->addListener(*_listeners[i]);
        }
        catch (IllegalArgumentException& e) {
            cerr << "Cannot create compressed stream: " << e.what() << endl;
            return T(Error::ERR_CREATE_COMPRESSOR, 0, 0);
        }
    }
    catch (exception& e) {
        cerr << "Cannot open output file '" << _outputName + "' for writing: " << e.what() << endl;
        return T(Error::ERR_CREATE_FILE, 0, 0);
    }

    try {
        string str = _inputName;
        transform(str.begin(), str.end(), str.begin(), ::toupper);

        if (str.compare(0, 5, "STDIN") == 0) {
            _is = &cin;
        }
        else {
            ifstream* ifs = new ifstream(_inputName.c_str(), ifstream::binary);

            if (!*ifs) {
                cerr << "Cannot open input file '" << _inputName << "'" << endl;
                return T(Error::ERR_OPEN_FILE, 0, 0);
            }

            _is = ifs;
        }
    }
    catch (exception& e) {
        cerr << "Cannot open input file '" << _inputName << "': " << e.what() << endl;
        return T(Error::ERR_OPEN_FILE, 0, 0);
    }

    // Encode
    printFlag = _verbosity > 1;
    ss << "\nEncoding " << _inputName << " ...";
    printOut(ss.str().c_str(), printFlag);
    printOut("\n", _verbosity > 3);
    int64 read = 0;
    byte* buf = new byte[DEFAULT_BUFFER_SIZE];
    SliceArray<byte> sa(buf, DEFAULT_BUFFER_SIZE, 0);
    int len;

    if (_listeners.size() > 0) {
        Event evt(Event::COMPRESSION_START, -1, int64(0));
        BlockCompressor::notifyListeners(_listeners, evt);
    }

    Clock clock;

    try {
        while (true) {
            try {
                _is->read((char*)&sa._array[0], sa._length);
                len = (*_is) ? sa._length : int(_is->gcount());
            }
            catch (exception& e) {
                cerr << "Failed to read block from file '" << _inputName << "': " << endl;
                cerr << e.what() << endl;
                return T(Error::ERR_READ_FILE, read, _cos->getWritten());
            }

            if (len <= 0)
                break;

            // Just write block to the compressed output stream !
            read += len;
            _cos->write((const char*)&sa._array[0], len);
        }
    }
    catch (IOException ioe) {
        delete[] buf;
        cerr << ioe.what() << endl;
        return T(ioe.error(), _cos->getWritten());
    }
    catch (exception& e) {
        delete[] buf;
        cerr << "An unexpected condition happened. Exiting ..." << endl;
        cerr << e.what() << endl;
        return T(Error::ERR_UNKNOWN, read, _cos->getWritten());
    }

    // Close streams to ensure all data are flushed
    dispose();

    if (os != &cout) {
        ofstream* ofs = dynamic_cast<ofstream*>(os);

        if (ofs) {
            try {
                ofs->close();
            }
            catch (exception&) {
                // Ignore
            }
        }

        if (os != nullptr)
            delete os;
    }

    if (read == 0) {
        delete[] buf;
        cout << "Empty input file ... nothing to do" << endl;
        return T(WARN_EMPTY_INPUT, read, _cos->getWritten());
    }

    clock.stop();
    double delta = clock.elapsed();
    printOut("", _verbosity > 1);
    ss.str(string());
    ss << "Encoding:          " << uint64(delta) << " ms";
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Input size:        " << read;
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Output size:       " << _cos->getWritten();
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Compression ratio: " << float(_cos->getWritten()) / float(read);
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Encoding " << _inputName << ": " << read << " => " << _cos->getWritten();
    ss << " bytes in " << delta << " ms";
    printOut(ss.str().c_str(), _verbosity == 1);

    if (delta > 0) {
        double b2KB = double(1000) / double(1024);
        ss.str(string());
        ss << "Throughput (KB/s): " << uint(read * b2KB / delta);
        printOut(ss.str().c_str(), printFlag);
    }

    printOut("", _verbosity > 1);

    if (_listeners.size() > 0) {
        Event evt(Event::COMPRESSION_END, -1, int64(_cos->getWritten()));
        BlockCompressor::notifyListeners(_listeners, evt);
    }

    delete[] buf;
    return T(0, read, _cos->getWritten());
}

template <class T>
FileCompressTask<T>::~FileCompressTask()
{
    dispose();

    if (_cos != nullptr) {
        delete _cos;
        _cos = nullptr;
    }

    try {
        if ((_is != nullptr) && (_is != &cin)) {
            delete _is;
        }

        _is = nullptr;
    }
    catch (exception ioe) {
    }
}

// Close and flush streams. Do not deallocate resources. Idempotent.
template <class T>
void FileCompressTask<T>::dispose()
{
    try {
        if (_cos != nullptr) {
            _cos->close();
        }
    }
    catch (exception& e) {
        cerr << "Compression failure: " << e.what() << endl;
        exit(Error::ERR_WRITE_FILE);
    }

    if (_is != &cin) {
        ifstream* ifs = dynamic_cast<ifstream*>(_is);

        if (ifs) {
            try {
                ifs->close();
            }
            catch (exception&) {
                // Ignore
            }
        }
    }
}
#include <typeinfo>

#ifdef CONCURRENCY_ENABLED
template <class T, class R>
R FileCompressWorker<T, R>::call()
{
    int res = 0;
    uint64 read = 0;
    uint64 written = 0;

    while (res == 0) {
        T* task = _queue->get();

        if (task == nullptr)
            break;

        R result = (*task)->call();
        res = result._code;
        read += result._read;
        written += result._written;
    }

    return R(res, read, written);
}
#endif