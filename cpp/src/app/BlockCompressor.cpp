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
#include "../io/InfoPrinter.hpp"
#include "../io/Error.hpp"
#include "../io/FunctionFactory.hpp"
#include "../IllegalArgumentException.hpp"
#include "../io/IOException.hpp"
#include "../io/NullOutputStream.hpp"
#include "../SliceArray.hpp"

using namespace kanzi;

BlockCompressor::BlockCompressor(map<string, string>& args)
{
    map<string, string>::iterator it;
    it = args.find("level");
    _level = atoi(it->second.c_str());
    args.erase(it);
    it = args.find("verbose");
    _verbosity = atoi(it->second.c_str());
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
        strCodec = "HUFFMAN";
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
            strTransf = "BWT+MTFT+ZRLT";
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
    _jobs = atoi(it->second.c_str());
    args.erase(it);
    _cos = nullptr;
    _is = nullptr;

    if (_verbosity > 2)
        addListener(new InfoPrinter(_verbosity, InfoPrinter::ENCODING, cout));

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

    while (_listeners.size() > 0) {
        vector<Listener*>::iterator it = _listeners.begin();
        delete *it;
        _listeners.erase(it);
    }
}

// Close and flush streams. Do not deallocate resources. Idempotent.
void BlockCompressor::dispose()
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

int BlockCompressor::call()
{
    bool printFlag = _verbosity > 2;
    stringstream ss;
    ss << "Kanzi 1.1 (C) 2017,  Frederic Langlet";
    printOut(ss.str().c_str(), _verbosity >= 1);
    ss.str(string());
    ss << "Input file name set to '" << _inputName << "'";
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Output file name set to '" << _outputName << "'";
    printOut(ss.str().c_str(), printFlag);
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

    if (_jobs > 0) {
       ss << "Using " << _jobs << " job" << ((_jobs > 1) ? "s" : "");
       printOut(ss.str().c_str(), printFlag);
       ss.str(string());
    }

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
                return Error::ERR_CREATE_FILE;
            }

            struct stat buffer;

            if (stat(_outputName.c_str(), &buffer) == 0) {
                if ((buffer.st_mode & S_IFDIR) != 0) {
                    cerr << "The output file is a directory" << endl;
                    return Error::ERR_OUTPUT_IS_DIR;
                }

                if (_overwrite == false) {
                    cerr << "The output file exists and the 'force' command "
                         << "line option has not been provided" << endl;
                    return Error::ERR_OVERWRITE_FILE;
                }
            }

            os = new ofstream(_outputName.c_str(), ofstream::binary);

            if (!*os) {
                cerr << "Cannot open output file '" << _outputName + "' for writing: " << endl;
                return Error::ERR_CREATE_FILE;
            }
        }

        try {
            map<string, string> ctx;
            stringstream ss;
            ss << _blockSize;
            ctx["blockSize"] = ss.str();
            ss.str(string());
            ss << _checksum;
            ctx["checksum"] = ss.str();
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
            return Error::ERR_CREATE_COMPRESSOR;
        }
    }
    catch (exception& e) {
        cerr << "Cannot open output file '" << _outputName + "' for writing: " << e.what() << endl;
        return Error::ERR_CREATE_FILE;
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
            }

            _is = ifs;
        }
    }
    catch (exception& e) {
        cerr << "Cannot open input file '" << _inputName << "': " << e.what() << endl;
        return Error::ERR_OPEN_FILE;
    }

    // Encode
    printFlag = _verbosity > 1;
    printOut("Encoding ...", printFlag);
    int read = 0;
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
                len = (*_is) ? sa._length : (int)_is->gcount();
            }
            catch (exception& e) {
                cerr << "Failed to read block from file '" << _inputName << "': " << endl;
                cerr << e.what() << endl;
                return Error::ERR_READ_FILE;
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
        return ioe.error();
    }
    catch (exception& e) {
        delete[] buf;
        cerr << "An unexpected condition happened. Exiting ..." << endl;
        cerr << e.what() << endl;
        return Error::ERR_UNKNOWN;
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
        return WARN_EMPTY_INPUT;
    }

    clock.stop();
    double delta = clock.elapsed();
    printOut("", _verbosity >= 1);
    ss.str(string());
    ss << "Encoding:          " << uint(delta) << " ms";
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Input size:        " << read;
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Output size:       " << _cos->getWritten();
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Ratio:             " << float(_cos->getWritten()) / float(read);
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Encoding: " << read << " => " << _cos->getWritten();
    ss << " bytes in " << delta<< " ms";
    printOut(ss.str().c_str(), _verbosity == 1);

    if (delta > 0) {
        double b2KB = double(1000) / double(1024);
        ss.str(string());
        ss << "Throughput (KB/s): " << uint(read * b2KB / delta);
        printOut(ss.str().c_str(), printFlag);
    }

    printOut("", _verbosity >= 1);

    if (_listeners.size() > 0) {
        Event evt(Event::COMPRESSION_END, -1, int64(_cos->getWritten()));
        BlockCompressor::notifyListeners(_listeners, evt);
    }

    delete[] buf;
    return 0;
}

void BlockCompressor::printOut(const char* msg, bool print)
{
    if ((print == true) && (msg != nullptr))
        cout << msg << endl;
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
        tranformAndCodec[0] = "RLT+TEXT";
        tranformAndCodec[1] = "TPAQ";
        return;

    default:
        tranformAndCodec[0] = "Unknown";
        tranformAndCodec[1] = "Unknown";
        return;
    }
}