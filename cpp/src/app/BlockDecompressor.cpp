
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
    it = args.find("jobs");
    int concurrency = atoi(it->second.c_str());
    _jobs = (concurrency == 0) ? DEFAULT_CONCURRENCY : concurrency;
    args.erase(it);

#ifndef CONCURRENCY_ENABLED
    if (_jobs > 1)
        throw IllegalArgumentException("The number of jobs is limited to 1 in this version");
#endif

    _cis = nullptr;
    _os = nullptr;

    it = args.find("verbose");
    _verbosity = atoi(it->second.c_str());
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
    vector<string> files;
    uint64 read = 0;
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

        if ((formattedInName.size() != 0) && (formattedInName[formattedInName.size() - 1] != PATH_SEPARATOR)) {
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

    // Run the task(s)
    if (nbFiles == 1) {
        string oName = formattedOutName;
        string iName = files[0];

        if (oName.length() == 0) {
            oName = iName + ".bak";
        }
        else if ((inputIsDir == true) && (specialOutput == false)) {
            oName = formattedOutName + iName.substr(formattedInName.size()) + ".bak";
        }

        FileDecompressTask<FileDecompressResult> task(_verbosity, _overwrite, iName, oName, _jobs, _listeners);
        FileDecompressResult fdr = task.call();
        res = fdr._code;
        read = fdr._read;
    }
    else {
        vector<FileDecompressTask<FileDecompressResult>*> tasks;
        int* jobsPerTask = new int[nbFiles];
        computeJobsPerTask(jobsPerTask, _jobs, nbFiles);
        int n = 0;

        //  Create one task per file
        for (int i = 0; i < nbFiles; i++) {
            string oName = formattedOutName;
            string iName = files[i];

            if (oName.length() == 0) {
                oName = iName + ".bak";
            }
            else if ((inputIsDir == true) && (specialOutput == false)) {
                oName = formattedOutName + iName.substr(formattedInName.size()) + ".bak";
            }
            FileDecompressTask<FileDecompressResult>* task = new FileDecompressTask<FileDecompressResult>(_verbosity, _overwrite, iName, oName, jobsPerTask[n++], _listeners);
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
                FileDecompressResult fcr = results[i].get();
                res = fcr._code;
                read += fcr._read;

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
                FileDecompressResult fcr = tasks[i]->call();
                res = fcr._code;
                read += fcr._read;

                if (res != 0)
                    break;
            }
        }

        delete[] jobsPerTask;

        for (int i = 0; i < nbFiles; i++)
            delete tasks[i];
    }

    clock.stop();

    if (nbFiles > 1) {
        Printer log(&cout);
        double delta = clock.elapsed();
        log.println("", _verbosity > 0);
        ss << "Total decoding time: " << uint64(delta) << " ms";
        log.println(ss.str().c_str(), _verbosity > 0);
        ss.str(string());
        ss << "Total input size: " << read << " byte" << ((read > 1) ? "s" : "");
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

void BlockDecompressor::computeJobsPerTask(int jobsPerTask[], int jobs, int tasks)
{
    if ((jobs <= 0) || (tasks <= 0))
        return;

    int q = (jobs <= tasks) ? 1 : jobs / tasks;
    int r = (jobs <= tasks) ? 0 : jobs - q * tasks;

    for (int i = 0; i < tasks; i++)
        jobsPerTask[i] = q;

    int n = 0;

    while (r != 0) {
        jobsPerTask[n]++;
        r--;
        n++;

        if (n == tasks)
            n = 0;
    }
}

template <class T>
FileDecompressTask<T>::FileDecompressTask(int verbosity, bool overwrite,
    const string& inputName, const string& outputName,
    int jobs, vector<Listener*> listeners)
{
    _verbosity = verbosity;
    _overwrite = overwrite;
    _inputName = inputName;
    _outputName = outputName;
    _jobs = jobs;
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
    bool printFlag = _verbosity > 2;
    stringstream ss;
    ss << "Input file name set to '" << _inputName << "'";
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Output file name set to '" << _outputName << "'";
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());

    int64 read = 0;
    printFlag = _verbosity > 1;
    ss << "\nDecoding " << _inputName << " ...";
    log.println(ss.str().c_str(), printFlag);
    log.println("\n", _verbosity > 3);

    if (_listeners.size() > 0) {
        Event evt(Event::DECOMPRESSION_START, -1, int64(0));
        BlockDecompressor::notifyListeners(_listeners, evt);
    }

    string str = _outputName;
    transform(str.begin(), str.end(), str.begin(), ::toupper);

    if (str.compare(0, 4, "NONE") == 0) {
        _os = new NullOutputStream();
    }
    else if (str.compare(0, 6, "STDOUT") == 0) {
        _os = &cout;
    }
    else {
        try {
            if (samePaths(_inputName, _outputName)) {
                cerr << "The input and output files must be different" << endl;
                return T(Error::ERR_CREATE_FILE, 0);
            }

            struct stat buffer;

            if (stat(_outputName.c_str(), &buffer) == 0) {
                if ((buffer.st_mode & S_IFDIR) != 0) {
                    cerr << "The output file is a directory" << endl;
                    return T(Error::ERR_OUTPUT_IS_DIR, 0);
                }

                if (_overwrite == false) {
                    cerr << "File '" << _outputName << "' exists and the 'force' command "
                         << "line option has not been provided" << endl;
                    return T(Error::ERR_OVERWRITE_FILE, 0);
                }
            }

            _os = new ofstream(_outputName.c_str(), ofstream::binary);

            if (!*_os) {
                if (_overwrite == true) {
                    // Attempt to create the full folder hierarchy to file
                    string parentDir = _outputName;
                    size_t idx = _outputName.find_last_of(PATH_SEPARATOR);

                    if (idx != string::npos) {
                        parentDir = parentDir.substr(0, idx);
                    }

                    if (mkdirAll(parentDir) == 0) {
                        _os = new ofstream(_outputName.c_str(), ofstream::binary);
                    }
                }

                if (!*_os) {
                    cerr << "Cannot open output file '" << _outputName << "' for writing." << endl;
                    return T(Error::ERR_CREATE_FILE, 0);
                }
            }
        }
        catch (exception& e) {
            cerr << "Cannot open output file '" << _outputName << "' for writing: " << e.what() << endl;
            return T(Error::ERR_CREATE_FILE, 0);
        }
    }

    InputStream* is;

    try {
        str = _inputName;
        transform(str.begin(), str.end(), str.begin(), ::toupper);

        if (str.compare(0, 5, "STDIN") == 0) {
            is = &cin;
        }
        else {
            ifstream* ifs = new ifstream(_inputName.c_str(), ifstream::binary);

            if (!*ifs) {
                cerr << "Cannot open input file '" << _inputName << "'" << endl;
                return T(Error::ERR_OPEN_FILE, 0);
            }

            is = ifs;
        }

        try {
            map<string, string> ctx;
            stringstream ss;
            ss << _jobs;
            ctx["jobs"] = ss.str();
            _cis = new CompressedInputStream(*is, ctx);

            for (uint i = 0; i < _listeners.size(); i++)
                _cis->addListener(*_listeners[i]);
        }
        catch (IllegalArgumentException& e) {
            cerr << "Cannot create compressed stream: " << e.what() << endl;
            return T(Error::ERR_CREATE_DECOMPRESSOR, 0);
        }
    }
    catch (exception& e) {
        cerr << "Cannot open input file '" << _inputName << "': " << e.what() << endl;
        return T(Error::ERR_OPEN_FILE, _cis->getRead());
    }

    Clock clock;
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
                cerr << "Reached end of stream" << endl;
                return T(Error::ERR_READ_FILE, _cis->getRead());
            }

            try {
                if (decoded > 0) {
                    _os->write((const char*)&sa._array[0], decoded);
                    read += decoded;
                }
            }
            catch (exception& e) {
                delete[] buf;
                cerr << "Failed to write decompressed block to file '" << _outputName << "': ";
                cerr << e.what() << endl;
                return T(Error::ERR_READ_FILE, _cis->getRead());
            }
        } while (decoded == sa._length);
    }
    catch (IOException& e) {
        // Close streams to ensure all data are flushed
        dispose();
        delete[] buf;

        if (_cis->eof()) {
            cerr << "Reached end of stream" << endl;
            return T(Error::ERR_READ_FILE, _cis->getRead());
        }

        cerr << e.what() << endl;
        return T(e.error(), _cis->getRead());
    }
    catch (exception& e) {
        // Close streams to ensure all data are flushed
        dispose();
        delete[] buf;

        if (_cis->eof()) {
            cerr << "Reached end of stream" << endl;
            return T(Error::ERR_READ_FILE, _cis->getRead());
        }

        cerr << "An unexpected condition happened. Exiting ..." << endl
             << e.what() << endl;
        return T(Error::ERR_UNKNOWN, _cis->getRead());
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

    clock.stop();
    double delta = clock.elapsed();
    log.println("", _verbosity > 1);
    ss.str(string());
    ss << "Decoding:          " << uint64(delta) << " ms";
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Input size:        " << _cis->getRead();
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Output size:       " << read;
    log.println(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Decoding " << _inputName << ": " << _cis->getRead() << " => " << read;
    ss << " bytes in " << delta << " ms";
    log.println(ss.str().c_str(), _verbosity == 1);

    if (delta > 0) {
        double b2KB = double(1000) / double(1024);
        ss.str(string());
        ss << "Throughput (KB/s): " << uint(read * b2KB / delta);
        log.println(ss.str().c_str(), printFlag);
    }

    log.println("", _verbosity > 1);

    if (_listeners.size() > 0) {
        Event evt(Event::DECOMPRESSION_END, -1, int64(_cis->getRead()));
        BlockDecompressor::notifyListeners(_listeners, evt);
    }

    delete[] buf;
    return T(0, read);
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

    while (res == 0) {
        T* task = _queue->get();

        if (task == nullptr)
            break;

        R result = (*task)->call();
        res = result._code;
        read += result._read;
    }

    return R(res, read);
}
#endif
