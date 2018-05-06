
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
#include <ctime>
#include <stdio.h>
#include <stdlib.h>
#include <fstream>
#include <iostream>
#include <string>
#include <time.h>
#include <sys/stat.h>
#include "BlockDecompressor.hpp"
#include "InfoPrinter.hpp"
#include "../IllegalArgumentException.hpp"
#include "../SliceArray.hpp"
#include "../util.hpp"
#include "../Error.hpp"
#include "../io/IOException.hpp"
#include "../io/IOUtil.hpp"
#include "../io/NullOutputStream.hpp"
#include "../io/NullOutputStream.hpp"

#ifdef CONCURRENCY_ENABLED
#include <future>
#endif

using namespace kanzi;

BlockDecompressor::BlockDecompressor(map<string, string>& args)
{
    _blockSize = 0;
    map<string, string>::iterator it;
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
    }
#endif

    _jobs = (concurrency == 0) ? DEFAULT_CONCURRENCY : concurrency;
    args.erase(it);
    _cis = nullptr;
    _os = nullptr;

    if ((_verbosity > 0) && (args.size() > 0)) {
        Printer log(&cout);

        for (it = args.begin(); it != args.end(); it++) {
            stringstream ss;
            ss << "Ignoring invalid option [" << it->first << "]";
            log.println(ss.str().c_str(), _verbosity > 0);
        }
    }
}

BlockDecompressor::~BlockDecompressor()
{
    dispose();

    while (_listeners.size() > 0) {
        vector<Listener*>::iterator it = _listeners.begin();
        delete *it;
        _listeners.erase(it);
    }
}

void BlockDecompressor::dispose()
{
}

int BlockDecompressor::call()
{
    vector<FileData> files;
    uint64 read = 0;
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
    ss << nbFiles << strFiles << " to decompress\n";
    log.println(ss.str().c_str(), _verbosity > 0);
    ss.str(string());
    ss << "Verbosity set to " << _verbosity;
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Overwrite set to " << (_overwrite ? "true" : "false");
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
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
        addListener(new InfoPrinter(_verbosity, InfoPrinter::DECODING, cout));

    int res = 0;

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

    // Run the task(s)
    if (nbFiles == 1) {
        string oName = formattedOutName;
        string iName = files[0]._path;

        if (oName.length() == 0) {
            oName = iName + ".bak";
        }
        else if ((inputIsDir == true) && (specialOutput == false)) {
            oName = formattedOutName + iName.substr(formattedInName.size()) + ".bak";
        }

        ss.str(string());
        ss << files[0]._size;
        ctx["fileSize"] = ss.str();
        ctx["inputName"] = iName;
        ctx["outputName"] = oName;
        ss.str(string());
        ss << _jobs;
        ctx["jobs"] = ss.str();
        FileDecompressTask<FileDecompressResult> task(ctx, _listeners);
        FileDecompressResult fdr = task.call();
        res = fdr._code;
        read = fdr._read;

        if (res != 0) {
            cerr << fdr._errMsg << endl;
        }
    }
    else {
        vector<FileDecompressTask<FileDecompressResult>*> tasks;
        int* jobsPerTask = new int[nbFiles];
        Global::computeJobsPerTask(jobsPerTask, _jobs, nbFiles);
        int n = 0;
        sort(files.begin(), files.end());

        //  Create one task per file
        for (int i = 0; i < nbFiles; i++) {
            string oName = formattedOutName;
            string iName = files[i]._path;

            if (oName.length() == 0) {
                oName = iName + ".bak";
            }
            else if ((inputIsDir == true) && (specialOutput == false)) {
                oName = formattedOutName + iName.substr(formattedInName.size()) + ".bak";
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
            FileDecompressTask<FileDecompressResult>* task = new FileDecompressTask<FileDecompressResult>(taskCtx, _listeners);
            tasks.push_back(task);
        }

        bool doConcurrent = _jobs > 1;

#ifdef CONCURRENCY_ENABLED
        if (doConcurrent) {
            vector<FileDecompressWorker<FileDecompressTask<FileDecompressResult>*, FileDecompressResult>*> workers;
            vector<future<FileDecompressResult> > results;
            BoundedConcurrentQueue<FileDecompressTask<FileDecompressResult>*, FileDecompressResult> queue(nbFiles, &tasks[0]);

            // Create one worker per job and run it. A worker calls several tasks sequentially.
            for (int i = 0; i < _jobs; i++) {
                workers.push_back(new FileDecompressWorker<FileDecompressTask<FileDecompressResult>*, FileDecompressResult>(&queue));
                results.push_back(async(launch::async, &FileDecompressWorker<FileDecompressTask<FileDecompressResult>*, FileDecompressResult>::call, workers[i]));
            }

            // Wait for results
            for (int i = 0; i < _jobs; i++) {
                FileDecompressResult fdr = results[i].get();
                res = fdr._code;
                read += fdr._read;

                if (res != 0) {
                    cerr << fdr._errMsg << endl;
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
                FileDecompressResult fdr = tasks[i]->call();
                res = fdr._code;
                read += fdr._read;

                if (res != 0) {
                    cerr << fdr._errMsg << endl;
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
        Printer log(&cout);
        double delta = stopClock.elapsed();
        log.println("", _verbosity > 0);
        ss << "Total decoding time: " << uint64(delta) << " ms";
        log.println(ss.str().c_str(), _verbosity > 0);
        ss.str(string());
        ss << "Total output size: " << read << " byte" << ((read > 1) ? "s" : "");
        log.println(ss.str().c_str(), _verbosity > 0);
        ss.str(string());
    }

    return res;
}

bool BlockDecompressor::addListener(Listener* bl)
{
    if (bl == nullptr)
        return false;

    _listeners.push_back(bl);
    return true;
}

bool BlockDecompressor::removeListener(Listener* bl)
{
    std::vector<Listener*>::iterator it = find(_listeners.begin(), _listeners.end(), bl);

    if (it == _listeners.end())
        return false;

    _listeners.erase(it);
    return true;
}

void BlockDecompressor::notifyListeners(vector<Listener*>& listeners, const Event& evt)
{
    vector<Listener*>::iterator it;

    for (it = listeners.begin(); it != listeners.end(); it++)
        (*it)->processEvent(evt);
}

template <class T>
FileDecompressTask<T>::FileDecompressTask(map<string, string>& ctx, vector<Listener*>& listeners)
    : _ctx(ctx)
{
    _listeners = listeners;
    _os = nullptr;
    _cis = nullptr;
}

template <class T>
FileDecompressTask<T>::~FileDecompressTask()
{
    dispose();

    if (_cis != nullptr) {
        delete _cis;
        _cis = nullptr;
    }

    try {
        if ((_os != nullptr) && (_os != &cout)) {
            delete _os;
        }

        _os = nullptr;
    }
    catch (exception ioe) {
    }
}

template <class T>
T FileDecompressTask<T>::call()
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

    int64 read = 0;
    printFlag = verbosity > 1;
    ss << "\nDecoding " << inputName << " ...";
    log.println(ss.str().c_str(), printFlag);
    log.println("\n", verbosity > 3);

    if (_listeners.size() > 0) {
        Event evt(Event::DECOMPRESSION_START, -1, int64(0), clock());
        BlockDecompressor::notifyListeners(_listeners, evt);
    }

    string str = outputName;
    transform(str.begin(), str.end(), str.begin(), ::toupper);

    if (str.compare(0, 4, "NONE") == 0) {
        _os = new NullOutputStream();
    }
    else if (str.compare(0, 6, "STDOUT") == 0) {
        _os = &cout;
    }
    else {
        try {
            if (samePaths(inputName, outputName)) {
                stringstream sserr;
                sserr << "The input and output files must be different";
                return T(Error::ERR_CREATE_FILE, 0, sserr.str().c_str());
            }

            struct stat buffer;

            if (stat(outputName.c_str(), &buffer) == 0) {
                if ((buffer.st_mode & S_IFDIR) != 0) {
                    stringstream sserr;
                    sserr << "The output file is a directory";
                    return T(Error::ERR_OUTPUT_IS_DIR, 0, sserr.str().c_str());
                }

                if (overwrite == false) {
                    stringstream sserr;
                    sserr << "File '" << outputName << "' exists and the 'force' command "
                          << "line option has not been provided";
                    return T(Error::ERR_OVERWRITE_FILE, 0, sserr.str().c_str());
                }
            }

            _os = new ofstream(outputName.c_str(), ofstream::out | ofstream::binary);

            if (!*_os) {
                if (overwrite == true) {
                    // Attempt to create the full folder hierarchy to file
                    string parentDir = outputName;
                    size_t idx = outputName.find_last_of(PATH_SEPARATOR);

                    if (idx != string::npos) {
                        parentDir = parentDir.substr(0, idx);
                    }

                    if (mkdirAll(parentDir) == 0) {
                        _os = new ofstream(outputName.c_str(), ofstream::binary);
                    }
                }

                if (!*_os) {
                    stringstream sserr;
                    sserr << "Cannot open output file '" << outputName << "' for writing";
                    return T(Error::ERR_CREATE_FILE, 0, sserr.str().c_str());
                }
            }
        }
        catch (exception& e) {
            stringstream sserr;
            sserr << "Cannot open output file '" << outputName << "' for writing: " << e.what();
            return T(Error::ERR_CREATE_FILE, 0, sserr.str().c_str());
        }
    }

    InputStream* is;

    try {
        str = inputName;
        transform(str.begin(), str.end(), str.begin(), ::toupper);

        if (str.compare(0, 5, "STDIN") == 0) {
            is = &cin;
        }
        else {
            ifstream* ifs = new ifstream(inputName.c_str(), ifstream::in | ifstream::binary);

            if (!*ifs) {
                stringstream sserr;
                sserr << "Cannot open input file '" << inputName << "'";
                return T(Error::ERR_OPEN_FILE, 0, sserr.str().c_str());
            }

            is = ifs;
        }

        try {
            _cis = new CompressedInputStream(*is, _ctx);

            for (uint i = 0; i < _listeners.size(); i++)
                _cis->addListener(*_listeners[i]);
        }
        catch (IllegalArgumentException& e) {
            stringstream sserr;
            sserr << "Cannot create compressed stream: " << e.what();
            return T(Error::ERR_CREATE_DECOMPRESSOR, 0, sserr.str().c_str());
        }
    }
    catch (exception& e) {
        stringstream sserr;
        sserr << "Cannot open input file '" << inputName << "': " << e.what();
        return T(Error::ERR_OPEN_FILE, _cis->getRead(), sserr.str().c_str());
    }

    Clock stopClock;
    byte* buf = new byte[DEFAULT_BUFFER_SIZE];

    try {
        SliceArray<byte> sa(buf, DEFAULT_BUFFER_SIZE, 0);
        int decoded = 0;

        // Decode next block
        do {
            _cis->read((char*)&sa._array[0], sa._length);
            decoded = (int)_cis->gcount();

            if (decoded < 0) {
                delete[] buf;
                stringstream sserr;
                sserr << "Reached end of stream";
                return T(Error::ERR_READ_FILE, _cis->getRead(), sserr.str().c_str());
            }

            try {
                if (decoded > 0) {
                    _os->write((const char*)&sa._array[0], decoded);
                    read += decoded;
                }
            }
            catch (exception& e) {
                delete[] buf;
                stringstream sserr;
                sserr << "Failed to write decompressed block to file '" << outputName << "': " << e.what();
                return T(Error::ERR_READ_FILE, _cis->getRead(), sserr.str().c_str());
            }
        } while (decoded == sa._length);
    }
    catch (IOException& e) {
        // Close streams to ensure all data are flushed
        dispose();
        delete[] buf;
        stringstream sserr;

        if (_cis->eof()) {
            sserr << "Reached end of stream";
            return T(Error::ERR_READ_FILE, _cis->getRead(), sserr.str().c_str());
        }

        sserr << e.what();
        return T(e.error(), _cis->getRead(), sserr.str().c_str());
    }
    catch (exception& e) {
        // Close streams to ensure all data are flushed
        dispose();
        delete[] buf;
        stringstream sserr;

        if (_cis->eof()) {
            sserr << "Reached end of stream";
            return T(Error::ERR_READ_FILE, _cis->getRead(), sserr.str().c_str());
        }

        sserr << "An unexpected condition happened. Exiting ..." << endl
              << e.what();
        return T(Error::ERR_UNKNOWN, _cis->getRead(), sserr.str().c_str());
    }

    // Close streams to ensure all data are flushed
    dispose();

    if (is != &cin) {
        ifstream* ifs = dynamic_cast<ifstream*>(is);

        if (ifs) {
            try {
                ifs->close();
            }
            catch (exception&) {
                // Ignore
            }
        }

        delete is;
    }

    stopClock.stop();
    double delta = stopClock.elapsed();
    log.println("", verbosity > 1);
    ss.str(string());
    char buffer[32];

    if (delta >= 1e5) {
        sprintf(buffer, "%.1f s", delta / 1000);
        ss << "Decoding:          " << buffer;
    }
    else {
        sprintf(buffer, "%.0f ms", delta);
        ss << "Decoding:          " << buffer;
    }

    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Input size:        " << _cis->getRead();
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Output size:       " << read;
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Decoding " << inputName << ": " << _cis->getRead() << " => " << read;

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
        Event evt(Event::DECOMPRESSION_END, -1, int64(_cis->getRead()), clock());
        BlockDecompressor::notifyListeners(_listeners, evt);
    }

    delete[] buf;
    return T(0, read, "");
}

// Close and flush streams. Do not deallocate resources. Idempotent.
template <class T>
void FileDecompressTask<T>::dispose()
{
    try {
        if (_cis != nullptr) {
            _cis->close();
        }
    }
    catch (exception& e) {
        cerr << "Decompression failure: " << e.what() << endl;
        exit(Error::ERR_WRITE_FILE);
    }

    if (_os != &cout) {
        ofstream* ofs = dynamic_cast<ofstream*>(_os);

        if (ofs) {
            try {
                ofs->close();
            }
            catch (exception&) {
                // Ignore
            }
        }
    }
}

#ifdef CONCURRENCY_ENABLED
template <class T, class R>
R FileDecompressWorker<T, R>::call()
{
    int res = 0;
    uint64 read = 0;
    string errMsg;

    while (res == 0) {
        T* task = _queue->get();

        if (task == nullptr)
            break;

        R result = (*task)->call();
        res = result._code;
        read += result._read;

        if (res != 0) {
            errMsg += result._errMsg;
        }
    }

    return R(res, read, errMsg);
}
#endif
