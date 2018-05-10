
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
#include <time.h>
#include <sys/stat.h>
#include "BlockCompressor.hpp"
#include "InfoPrinter.hpp"
#include "../util.hpp"
#include "../SliceArray.hpp"
#include "../Error.hpp"
#include "../IllegalArgumentException.hpp"
#include "../function/FunctionFactory.hpp"
#include "../io/IOException.hpp"
#include "../io/IOUtil.hpp"
#include "../io/NullOutputStream.hpp"
#include "../io/NullOutputStream.hpp"

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

    it = args.find("skipBlocks");

    if (it == args.end()) {
        _skipBlocks = false;
    }
    else {
        string str = it->second;
        transform(str.begin(), str.end(), str.begin(), ::toupper);
        _skipBlocks = str == "TRUE";
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
    _transform = FunctionFactory<byte>::getName(FunctionFactory<byte>::getType(strTransf.c_str()));
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

    it = args.find("verbose");
    _verbosity = atoi(it->second.c_str());
    args.erase(it);
    it = args.find("jobs");
    int concurrency = atoi(it->second.c_str());

#ifndef CONCURRENCY_ENABLED
    if (concurrency > 1)
        throw IllegalArgumentException("The number of jobs is limited to 1 in this version");
#else
    if (concurrency > MAX_CONCURRENCY) {
        stringstream ss;
        ss << "Warning: the number of jobs is too high, defaulting to " << MAX_CONCURRENCY << endl;
        Printer log(&cerr);
        log.println(ss.str().c_str(), _verbosity > 0);
        concurrency = MAX_CONCURRENCY;
    }
#endif

    _jobs = (concurrency == 0) ? DEFAULT_CONCURRENCY : concurrency;
    args.erase(it);

    if ((_verbosity > 0) && (args.size() > 0)) {
        Printer log(&cout);

        for (it = args.begin(); it != args.end(); it++) {
            stringstream ss;
            ss << "Ignoring invalid option [" << it->first << "]";
            log.println(ss.str().c_str(), _verbosity > 0);
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
    vector<FileData> files;
    Clock stopClock;

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

    int nbFiles = int(files.size());
    Printer log(&cout);
    bool printFlag = _verbosity > 2;
    stringstream ss;
    string strFiles = (nbFiles > 1) ? " files" : " file";
    ss << nbFiles << strFiles << " to compress\n";
    log.println(ss.str().c_str(), _verbosity > 0);
    ss.str(string());
    ss << "Block size set to " << _blockSize << " bytes";
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Verbosity set to " << _verbosity;
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Overwrite set to " << (_overwrite ? "true" : "false");
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Checksum set to " << (_checksum ? "true" : "false");
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());

    if (_level < 0) {
        string etransform = _transform;
        transform(etransform.begin(), etransform.end(), etransform.begin(), ::toupper);
        ss << "Using " << ((etransform == "NONE") ? "no" : _transform) << " transform (stage 1)";
        log.println(ss.str().c_str(), printFlag);
        ss.str(string());
        string ecodec = _codec;
        transform(ecodec.begin(), ecodec.end(), ecodec.begin(), ::toupper);
        ss << "Using " << ((ecodec == "NONE") ? "no" : _codec) << " entropy codec (stage 2)";
        log.println(ss.str().c_str(), printFlag);
        ss.str(string());
    }
    else {
        ss << "Compression level set to " << _level;
        log.println(ss.str().c_str(), printFlag);
        ss.str(string());
    }

    ss << "Using " << _jobs << " job" << ((_jobs > 1) ? "s" : "");
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());

    string outputName = _outputName;
    transform(outputName.begin(), outputName.end(), outputName.begin(), ::toupper);

    if ((_jobs > 1) && (outputName.compare("STDOUT") == 0)) {
        cerr << "Cannot output to STDOUT with multiple jobs" << endl;
        return Error::ERR_CREATE_FILE;
    }

    // Limit verbosity level when files are processed concurrently
    if ((_jobs > 1) && (nbFiles > 1) && (_verbosity > 1)) {
        log.println("Warning: limiting verbosity to 1 due to concurrent processing of input files.\n", _verbosity > 1);
        _verbosity = 1;
    }

    if (_verbosity > 2)
        addListener(new InfoPrinter(_verbosity, InfoPrinter::ENCODING, cout));

    int res = 0;
    uint64 read = 0;
    uint64 written = 0;

    bool inputIsDir;
    string formattedOutName = _outputName;
    string formattedInName = _inputName;
    string upperOutputName = _outputName;
    transform(upperOutputName.begin(), upperOutputName.end(), upperOutputName.begin(), ::toupper);
    bool specialOutput = (upperOutputName.compare(0, 4, "NONE") == 0) || (upperOutputName.compare(0, 6, "STDOUT") == 0);
    struct stat buffer;

    // Need to strip path separator at the end to make 'stat()' happy
    if ((formattedInName.size() != 0) && (formattedInName[formattedInName.size() - 1] == PATH_SEPARATOR)) {
        formattedInName = formattedInName.substr(0, formattedInName.size() - 1);
    }

    if ((formattedOutName.size() != 0) && (formattedOutName[formattedOutName.size() - 1] == PATH_SEPARATOR)) {
        formattedOutName = formattedOutName.substr(0, formattedOutName.size() - 1);
    }

    if (stat(formattedInName.c_str(), &buffer) != 0) {
        cerr << "Cannot access input file '" << formattedInName << "'" << endl;
        return Error::ERR_OPEN_FILE;
    }

    if ((buffer.st_mode & S_IFDIR) != 0) {
        inputIsDir = true;

        if (formattedInName[formattedInName.size() - 1] == '.') {
            formattedInName = formattedInName.substr(0, formattedInName.size() - 1);
        }

        if ((formattedInName.size() != 0) && (formattedInName[formattedInName.size() - 1] != PATH_SEPARATOR)) {
            formattedInName += PATH_SEPARATOR;
        }

        if ((formattedOutName.size() != 0) && (specialOutput == false)) {
            if (stat(formattedOutName.c_str(), &buffer) != 0) {
                cerr << "Output must be an existing directory (or 'NONE')" << endl;
                return Error::ERR_OPEN_FILE;
            }

            if ((buffer.st_mode & S_IFDIR) == 0) {
                cerr << "Output must be a directory (or 'NONE')" << endl;
                return Error::ERR_CREATE_FILE;
            }

            formattedOutName += PATH_SEPARATOR;
        }
    }
    else {
        inputIsDir = false;

        if ((formattedOutName.size() != 0) && (specialOutput == false)) {
            if ((stat(formattedOutName.c_str(), &buffer) != 0) && ((buffer.st_mode & S_IFDIR) != 0)) {
                cerr << "Output must be a file (or 'NONE')" << endl;
                return Error::ERR_CREATE_FILE;
            }
        }
    }

    map<string, string> ctx;
    ss.str(string());
    ss << _verbosity;
    ctx["verbosity"] = ss.str();
    ctx["overwrite"] = (_overwrite == true) ? "TRUE" : "FALSE";
    ss.str(string());
    ss << _blockSize;
    ctx["blockSize"] = ss.str();
    ctx["skipBlocks"] = (_skipBlocks == true) ? "TRUE" : "FALSE";
    ctx["checksum"] = (_checksum == true) ? "TRUE" : "FALSE";
    ctx["codec"] = _codec;
    ctx["transform"] = _transform;

    // Run the task(s)
    if (nbFiles == 1) {
        string oName = formattedOutName;
        string iName = files[0]._path;

        if (oName.length() == 0) {
            oName = iName + ".knz";
        }
        else if ((inputIsDir == true) && (specialOutput == false)) {
            oName = formattedOutName + iName.substr(formattedInName.size()) + ".knz";
        }

        ss.str(string());
        ss << files[0]._size;
        ctx["fileSize"] = ss.str();
        ctx["inputName"] = iName;
        ctx["outputName"] = oName;
        ss.str(string());
        ss << _jobs;
        ctx["jobs"] = ss.str();
        FileCompressTask<FileCompressResult> task(ctx, _listeners);
        FileCompressResult fcr = task.call();
        res = fcr._code;
        read = fcr._read;
        written = fcr._written;

        if (res != 0) {
            cerr << fcr._errMsg << endl;
        }
    }
    else {
        vector<FileCompressTask<FileCompressResult>*> tasks;
        int* jobsPerTask = new int[nbFiles];
        Global::computeJobsPerTask(jobsPerTask, _jobs, nbFiles);
        int n = 0;
        sort(files.begin(), files.end());

        // Create one task per file
        for (int i = 0; i < nbFiles; i++) {
            string oName = formattedOutName;
            string iName = files[i]._path;

            if (oName.length() == 0) {
                oName = iName + ".knz";
            }
            else if ((inputIsDir == true) && (specialOutput == false)) {
                oName = formattedOutName + iName.substr(formattedInName.size()) + ".knz";
            }

            map<string, string> taskCtx(ctx);
            ss.str(string());
            ss << files[i]._size;
            taskCtx["fileSize"] = ss.str();
            taskCtx["inputName"] = iName;
            taskCtx["outputName"] = oName;
            ss.str(string());
            ss << jobsPerTask[n++];
            taskCtx["jobs"] = ss.str();
            ss.str(string());
            FileCompressTask<FileCompressResult>* task = new FileCompressTask<FileCompressResult>(taskCtx, _listeners);
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
                    cerr << fcr._errMsg << endl;
                    // Exit early by telling the workers that the queue is empty
                    queue.clear();
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

                if (res != 0) {
                    cerr << fcr._errMsg << endl;
                    break;
                }
            }
        }

        delete[] jobsPerTask;

        for (int i = 0; i < nbFiles; i++)
            delete tasks[i];
    }

    stopClock.stop();

    if (nbFiles > 1) {
        double delta = stopClock.elapsed();
        log.println("", _verbosity > 0);
        ss << "Total encoding time: " << uint64(delta) << " ms";
        log.println(ss.str().c_str(), _verbosity > 0);
        ss.str(string());
        ss << "Total output size: " << written << " byte" << ((written > 1) ? "s" : "");
        log.println(ss.str().c_str(), _verbosity > 0);
        ss.str(string());

        if (read > 0) {
            ss << "Compression ratio: " << float(written) / float(read);
            log.println(ss.str().c_str(), _verbosity > 0);
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
        tranformAndCodec[0] = "TEXT+ROLZ";
        tranformAndCodec[1] = "NONE";
        return;

    case 3:
        tranformAndCodec[0] = "BWT+RANK+ZRLT";
        tranformAndCodec[1] = "ANS0";
        return;

    case 4:
        tranformAndCodec[0] = "BWT+RANK+ZRLT";
        tranformAndCodec[1] = "FPAQ";
        return;

    case 5:
        tranformAndCodec[0] = "BWT";
        tranformAndCodec[1] = "CM";
        return;

    case 6:
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
FileCompressTask<T>::FileCompressTask(map<string, string>& ctx, vector<Listener*>& listeners)
    : _ctx(ctx)
{
    _listeners = listeners;
    _is = nullptr;
    _cos = nullptr;
}

template <class T>
T FileCompressTask<T>::call()
{
    Printer log(&cout);
    map<string, string>::iterator it;
    it = _ctx.find("verbosity");
    int verbosity = atoi(it->second.c_str());
    it = _ctx.find("inputName");
    string inputName = it->second.c_str();
    it = _ctx.find("outputName");
    string outputName = it->second.c_str();
    bool printFlag = verbosity > 2;
    stringstream ss;
    ss << "Input file name set to '" << inputName << "'";
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Output file name set to '" << outputName << "'";
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    it = _ctx.find("overwrite");
    string strOverwrite = it->second.c_str();
    bool overwrite = strOverwrite.compare(0, 4, "TRUE") == 0;

    OutputStream* os = nullptr;

    try {
        string str = outputName;
        transform(str.begin(), str.end(), str.begin(), ::toupper);

        if (str.compare(0, 4, "NONE") == 0) {
            os = new NullOutputStream();
        }
        else if (str.compare(0, 6, "STDOUT") == 0) {
            os = &cout;
        }
        else {
            if (samePaths(inputName, outputName)) {
                stringstream sserr;
                sserr << "The input and output files must be different" << endl;
                return T(Error::ERR_CREATE_FILE, 0, 0, sserr.str().c_str());
            }

            struct stat buffer;
            string path = outputName;
            replace(path.begin(), path.end(), '\\', '/');

            if (stat(outputName.c_str(), &buffer) == 0) {
                if ((buffer.st_mode & S_IFDIR) != 0) {
                    return T(Error::ERR_OUTPUT_IS_DIR, 0, 0, "The output file is a directory");
                }

                if (overwrite == false) {
                    stringstream sserr;
                    sserr << "File '" << outputName << "' exists and the 'force' command "
                          << "line option has not been provided";
                    return T(Error::ERR_OVERWRITE_FILE, 0, 0, sserr.str().c_str());
                }
            }

            os = new ofstream(outputName.c_str(), ofstream::out | ofstream::binary);

            if (!*os) {
                if (overwrite == true) {
                    // Attempt to create the full folder hierarchy to file
                    string parentDir = outputName;
                    size_t idx = outputName.find_last_of(PATH_SEPARATOR);

                    if (idx != string::npos) {
                        parentDir = parentDir.substr(0, idx);
                    }

                    if (mkdirAll(parentDir) == 0) {
                        os = new ofstream(outputName.c_str(), ofstream::binary);
                    }
                }

                if (!*os) {
                    stringstream sserr;
                    sserr << "Cannot open output file '" << outputName << "' for writing";
                    return T(Error::ERR_CREATE_FILE, 0, 0, sserr.str().c_str());
                }
            }
        }

        try {
            _cos = new CompressedOutputStream(*os, _ctx);

            for (uint i = 0; i < _listeners.size(); i++)
                _cos->addListener(*_listeners[i]);
        }
        catch (IllegalArgumentException& e) {
            stringstream sserr;
            sserr << "Cannot create compressed stream: " << e.what();
            return T(Error::ERR_CREATE_COMPRESSOR, 0, 0, sserr.str().c_str());
        }
    }
    catch (exception& e) {
        stringstream sserr;
        sserr << "Cannot open output file '" << outputName << "' for writing: " << e.what();
        return T(Error::ERR_CREATE_FILE, 0, 0, sserr.str().c_str());
    }

    try {
        string str = inputName;
        transform(str.begin(), str.end(), str.begin(), ::toupper);

        if (str.compare(0, 5, "STDIN") == 0) {
            _is = &cin;
        }
        else {
            ifstream* ifs = new ifstream(inputName.c_str(), ifstream::in | ifstream::binary);

            if (!*ifs) {
                stringstream sserr;
                sserr << "Cannot open input file '" << inputName << "'";
                return T(Error::ERR_OPEN_FILE, 0, 0, sserr.str().c_str());
            }

            _is = ifs;
        }
    }
    catch (exception& e) {
        stringstream sserr;
        sserr << "Cannot open input file '" << inputName << "': " << e.what();
        return T(Error::ERR_OPEN_FILE, 0, 0, sserr.str().c_str());
    }

    // Encode
    printFlag = verbosity > 1;
    ss << "\nEncoding " << inputName << " ...";
    log.println(ss.str().c_str(), printFlag);
    log.println("\n", verbosity > 3);
    int64 read = 0;
    byte* buf = new byte[DEFAULT_BUFFER_SIZE];
    SliceArray<byte> sa(buf, DEFAULT_BUFFER_SIZE, 0);
    int len;

    if (_listeners.size() > 0) {
        Event evt(Event::COMPRESSION_START, -1, int64(0), clock());
        BlockCompressor::notifyListeners(_listeners, evt);
    }

    Clock stopClock;

    try {
        while (true) {
            try {
                _is->read((char*)&sa._array[0], sa._length);
                len = (*_is) ? sa._length : int(_is->gcount());
            }
            catch (exception& e) {
                stringstream sserr;
                sserr << "Failed to read block from file '" << inputName << "': ";
                sserr << e.what() << endl;
                return T(Error::ERR_READ_FILE, read, _cos->getWritten(), sserr.str().c_str());
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
        return T(ioe.error(), read, _cos->getWritten(), ioe.what());
    }
    catch (exception& e) {
        delete[] buf;
        stringstream sserr;
        sserr << "An unexpected condition happened. Exiting ..." << endl
              << e.what();
        return T(Error::ERR_UNKNOWN, read, _cos->getWritten(), sserr.str().c_str());
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
        stringstream sserr;
        sserr << "Input file " << inputName << " is empty ... nothing to do";
        log.println(ss.str().c_str(), verbosity > 0);
        return T(0, read, _cos->getWritten(), sserr.str().c_str());
    }

    stopClock.stop();
    double delta = stopClock.elapsed();
    log.println("", verbosity > 1);
    ss.str(string());
    char buffer[32];

    if (delta >= 1e5) {
        sprintf(buffer, "%.1f s", delta / 1000);
        ss << "Encoding:          " << buffer;
    }
    else {
        sprintf(buffer, "%.0f ms", delta);
        ss << "Encoding:          " << buffer;
    }

    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Input size:        " << read;
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Output size:       " << _cos->getWritten();
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Compression ratio: " << float(_cos->getWritten()) / float(read);
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Encoding " << inputName << ": " << read << " => " << _cos->getWritten();

    if (delta >= 1e5) {
        sprintf(buffer, "%.1f s", delta / 1000);
        ss << " bytes in " << buffer;
    }
    else {
        sprintf(buffer, "%.0f ms", delta);
        ss << " bytes in " << buffer;
    }

    log.println(ss.str().c_str(), verbosity == 1);

    if (delta > 0) {
        double b2KB = double(1000) / double(1024);
        ss.str(string());
        ss << "Throughput (KB/s): " << uint(read * b2KB / delta);
        log.println(ss.str().c_str(), printFlag);
    }

    log.println("", verbosity > 1);

    if (_listeners.size() > 0) {
        Event evt(Event::COMPRESSION_END, -1, int64(_cos->getWritten()), clock());
        BlockCompressor::notifyListeners(_listeners, evt);
    }

    delete[] buf;
    return T(0, read, _cos->getWritten(), "");
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
    string errMsg;

    while (res == 0) {
        T* task = _queue->get();

        if (task == nullptr)
            break;

        R result = (*task)->call();
        res = result._code;
        read += result._read;
        written += result._written;

        if (res != 0) {
            errMsg += result._errMsg;
        }
    }

    return R(res, read, written, errMsg);
}
#endif