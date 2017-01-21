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

BlockCompressor::BlockCompressor(map<string, string>& map)
{
    _verbosity = atoi(map["verbose"].c_str());
    string str = map["overwrite"];
    transform(str.begin(), str.end(), str.begin(), ::toupper);
    _overwrite = str == "TRUE";
    _inputName = map["inputName"];
    _outputName = map["outputName"];
    _jobs = atoi(map["jobs"].c_str());
    _cos = nullptr;
    _is = nullptr;

    if (_verbosity > 1)
        addListener(new InfoPrinter(_verbosity, InfoPrinter::ENCODING, cout));
}

BlockCompressor::BlockCompressor(int argc, const char* argv[])
{
    map<string, string> map;
    processCommandLine(argc, argv, map);
    _verbosity = atoi(map["verbose"].c_str());
    string str = map["overwrite"];
    transform(str.begin(), str.end(), str.begin(), ::toupper);
    _overwrite = str == "TRUE";
    _inputName = map["inputName"];
    _outputName = map["outputName"];
    _codec = map["codec"];
    _blockSize = atoi(map["blockSize"].c_str());
    // Extract transform names. Curate input (EG. NONE+NONE+xxxx => xxxx)
    str = map["transform"];
    FunctionFactory<byte> bff;
    _transform = bff.getName(bff.getType(str.c_str()));
    str = map["checksum"];
    transform(str.begin(), str.end(), str.begin(), ::toupper);
    _checksum = str == "TRUE";
    _jobs = atoi(map["jobs"].c_str());
    _cos = nullptr;
    _is = nullptr;

    if (_verbosity > 1)
        addListener(new InfoPrinter(_verbosity, InfoPrinter::ENCODING, cout));
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
        vector<BlockListener*>::iterator it = _listeners.begin();
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

int BlockCompressor::main(int argc, const char* argv[])
{
    try {
        BlockCompressor bc(argc, argv);
        int code = bc.call();
        return code;
    }
    catch (exception& e) {
        cerr << "Could not create the block codec: " << e.what() << endl;
        exit(Error::ERR_CREATE_COMPRESSOR);
    }
}

int BlockCompressor::call()
{
    bool printFlag = _verbosity > 1;
    stringstream ss;
    ss << "Kanzi 1.0 (C) 2017,  Frederic Langlet";
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
    ss << "Using " << _jobs << " job" << ((_jobs > 1) ? "s" : "");
    printOut(ss.str().c_str(), printFlag);

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
                    cerr << "The output file exists and the 'overwrite' command "
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
            _cos = new CompressedOutputStream(_codec, _transform,
                *os, _blockSize, _checksum, _jobs);

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
    bool silent = _verbosity < 1;
    printOut("Encoding ...", !silent);
    int read = 0;
    byte* buf = new byte[DEFAULT_BUFFER_SIZE];
    SliceArray<byte> sa(buf, DEFAULT_BUFFER_SIZE, 0);
    int len;
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
    printOut("", !silent);
    ss.str(string());
    ss << "Encoding:          " << uint(delta) << " ms";
    printOut(ss.str().c_str(), !silent);
    ss.str(string());
    ss << "Input size:        " << read;
    printOut(ss.str().c_str(), !silent);
    ss.str(string());
    ss << "Output size:       " << _cos->getWritten();
    printOut(ss.str().c_str(), !silent);
    ss.str(string());
    ss << "Ratio:             " << float(_cos->getWritten()) / float(read);
    printOut(ss.str().c_str(), !silent);

    if (delta > 0) {
        double b2KB = double(1000) / double(1024);
        ss.str(string());
        ss << "Throughput (KB/s): " << uint(read * b2KB / delta);
        printOut(ss.str().c_str(), !silent);
    }

    printOut("", !silent);
    delete[] buf;
    return 0;
}

void BlockCompressor::processCommandLine(int argc, const char* argv[], map<string, string>& map)
{
    string inputName;
    string outputName;
    string strVerbose = "1";
    string strTasks = "1";
    string strBlockSize = "1048576";
    string strOverwrite = "false";
    string strChecksum = "false";
    string codec = "HUFFMAN"; // default
    string transf = "BWT+MTFT+ZRLT"; // default
    int verbose = 1;

    for (int i = 1; i < argc; i++) {
        string arg = argv[i];
        arg = ltrim(rtrim(arg));

        // Extract verbosity and output first
        if (arg.compare(0, 9, "-verbose=") == 0) {
            strVerbose = arg.substr(9);
            int verbose = atoi(strVerbose.c_str());

            if (verbose < 0) {
                cerr << "Invalid verbosity level provided on command line: " << arg << endl;
                exit(Error::ERR_INVALID_PARAM);
            }
        }
        else if (arg.compare(0, 8, "-output=") == 0) {
            arg = arg.substr(8);
            outputName = ltrim(rtrim(arg));
        }
    }

    // Overwrite verbosity if the output goes to stdout
    if (outputName.length() != 0) {
        string str = outputName;
        transform(str.begin(), str.end(), str.begin(), ::toupper);

        if (str == "STDOUT") {
            verbose = 0;
            strVerbose = "0";
        }
    }

    for (int i = 1; i < argc; i++) {
        string arg = argv[i];
        arg = ltrim(rtrim(arg));

        if (arg == "-help") {
            printOut("-help                : display this message", true);
            printOut("-verbose=<level>     : set the verbosity level [1..4]", true);
            printOut("                       0=silent, 1=default, 2=display block size (byte rounded)", true);
            printOut("                       3=display timings, 4=display extra information", true);
            printOut("-overwrite           : overwrite the output file if it already exists", true);
            printOut("-input=<inputName>   : mandatory name of the input file to encode or 'stdin'", true);
            printOut("-output=<outputName> : optional name of the output file (defaults to <input.knz>) or 'none' or 'stdout'", true);
            printOut("-block=<size>        : size of the input blocks, multiple of 16, max 1 GB (transform dependent), min 1 KB, default 1 MB", true);
            printOut("-entropy=<codec>     : entropy codec to use [None|Huffman*|ANS|Range|PAQ|FPAQ|TPAQ|CM]", true);
            printOut("-transform=<codec>   : transform to use [None|BWT*|BWTS|Snappy|LZ4|RLT|ZRLT|MTFT|RANK|TIMESTAMP]", true);
            printOut("                       EG: BWT+RANK or BWTS+MTFT (default is BWT+MTFT+ZRLT)", true);
            printOut("-checksum            : enable block checksum", true);
            printOut("-jobs=<jobs>         : number of concurrent jobs", true);
            printOut("", true);
            stringstream ss;
            ss << "EG. BlockCompressor -input=foo.txt -output=foo.knz -overwrite ";
            ss << "-transform=BWT+MTFT+ZRLT -block=4m -entropy=FPAQ -verbose=3 -jobs=4";
            printOut(ss.str().c_str(), true);
            exit(0);
        }
        else if (arg == "-overwrite") {
            strOverwrite = "true";
        }
        else if (arg == "-checksum") {
            strChecksum = "true";
        }
        else if (arg.compare(0, 7, "-input=") == 0) {
            arg = arg.substr(7);
            inputName = ltrim(rtrim(arg));
        }
        else if (arg.compare(0, 8, "-output=") == 0) {
            arg = arg.substr(8);
            outputName = ltrim(rtrim(arg));
        }
        else if (arg.compare(0, 9, "-entropy=") == 0) {
            arg = arg.substr(9);
            codec = ltrim(rtrim(arg));
            transform(codec.begin(), codec.end(), codec.begin(), ::toupper);
        }
        else if (arg.compare(0, 11, "-transform=") == 0) {
            arg = arg.substr(11);
            transf = ltrim(rtrim(arg));
            transform(transf.begin(), transf.end(), transf.begin(), ::toupper);
        }
        else if (arg.compare(0, 7, "-block=") == 0) {
            arg = arg.substr(7);
            string str = ltrim(rtrim(arg));
            transform(str.begin(), str.end(), str.begin(), ::toupper);
            char lastChar = str[str.length() - 1];
            int scale = 1;

            // Process K or M or G suffix
            if ('K' == lastChar) {
                scale = 1024;
                str = str.substr(0, str.length() - 1);
            }
            else if ('M' == lastChar) {
                scale = 1024 * 1024;
                str = str.substr(0, str.length() - 1);
            }
            else if ('G' == lastChar) {
                scale = 1024 * 1024 * 1024;
                str = str.substr(0, str.length() - 1);
            }

            int bk = atoi(str.c_str());

            if (bk <= 0) {
                cerr << "Invalid block size provided on command line: " << arg << endl;
                exit(Error::ERR_INVALID_PARAM);
            }

            stringstream ss;
            ss << scale * bk;
            strBlockSize = ss.str();
        }
        else if (arg.compare(0, 6, "-jobs=") == 0) {
            strTasks = arg.substr(6);
            int tasks = atoi(strTasks.c_str());

            if (tasks < 1) {
                cerr << "Invalid number of jobs provided on command line: " << arg << endl;
                exit(Error::ERR_INVALID_PARAM);
            }
        }
        else if ((arg.compare(0, 9, "-verbose=") != 0) && (arg.compare(0, 8, "-output=") != 0)) {
            stringstream ss;
            ss << "Warning: ignoring unknown option [" << arg << "]";
            printOut(ss.str().c_str(), verbose > 0);
        }
    }

    if (inputName.length() == 0) {
        cerr << "Missing input file name, exiting ..." << endl;
        exit(Error::ERR_MISSING_PARAM);
    }

    if (outputName.length() == 0) {
        outputName = inputName + ".knz";
    }

    map["blockSize"] = strBlockSize;
    map["verbose"] = strVerbose;
    map["overwrite"] = strOverwrite;
    map["inputName"] = inputName;
    map["outputName"] = outputName;
    map["codec"] = codec;
    map["transform"] = transf;
    map["checksum"] = strChecksum;
    map["jobs"] = strTasks;
}

void BlockCompressor::printOut(const char* msg, bool print)
{
    if ((print == true) && (msg != nullptr))
        cout << msg << endl;
}

bool BlockCompressor::addListener(BlockListener* bl)
{
    if (bl == nullptr)
        return false;

    _listeners.push_back(bl);
    return true;
}

bool BlockCompressor::removeListener(BlockListener* bl)
{
    std::vector<BlockListener*>::iterator it = find(_listeners.begin(), _listeners.end(), bl);

    if (it == _listeners.end())
        return false;

    _listeners.erase(it);
    return true;
}
