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
#include "../io/InfoPrinter.hpp"
#include "../io/Error.hpp"
#include "../io/IOException.hpp"
#include "../IllegalArgumentException.hpp"
#include "../io/NullOutputStream.hpp"
#include "../SliceArray.hpp"

using namespace kanzi;

BlockDecompressor::BlockDecompressor(map<string, string>& map)
{
    _blockSize = 0;
    _verbosity = atoi(map["verbose"].c_str());
    string str = map["overwrite"];
    transform(str.begin(), str.end(), str.begin(), ::toupper);
    _overwrite = str == "TRUE";
    _inputName = map["inputName"];
    _outputName = map["outputName"];
    _jobs = atoi(map["jobs"].c_str());
    _cis = nullptr;
    _os = nullptr;

    if (_verbosity > 1)
        addListener(new InfoPrinter(_verbosity, InfoPrinter::DECODING, cout));
}

BlockDecompressor::BlockDecompressor(int argc, const char* argv[])
{
    _blockSize = 0;
    map<string, string> map;
    processCommandLine(argc, argv, map);
    _verbosity = atoi(map["verbose"].c_str());
    string str = map["overwrite"];
    transform(str.begin(), str.end(), str.begin(), ::toupper);
    _overwrite = str == "TRUE";
    _inputName = map["inputName"];
    _outputName = map["outputName"];
    _jobs = atoi(map["jobs"].c_str());
    _cis = nullptr;
    _os = nullptr;

    if (_verbosity > 1)
        addListener(new InfoPrinter(_verbosity, InfoPrinter::DECODING, cout));
}

BlockDecompressor::~BlockDecompressor()
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

    while (_listeners.size() > 0) {
        vector<BlockListener*>::iterator it = _listeners.begin();
        delete *it;
        _listeners.erase(it);
    }
}

// Close and flush streams. Do not deallocate resources. Idempotent.
void BlockDecompressor::dispose()
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

int BlockDecompressor::main(int argc, const char* argv[])
{
    try {
        BlockDecompressor bd(argc, argv);
        int code = bd.call();
        return code;
    }
    catch (exception& e) {
        cerr << "Could not create the block codec: " << e.what() << endl;
        exit(Error::ERR_CREATE_COMPRESSOR);
    }
}

int BlockDecompressor::call()
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
    ss << "Verbosity set to " << _verbosity;
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Overwrite set to " << (_overwrite ? "true" : "false");
    printOut(ss.str().c_str(), printFlag);
    ss.str(string());
    ss << "Using " << _jobs << " job" << ((_jobs > 1) ? "s" : "");
    printOut(ss.str().c_str(), printFlag);

    uint64 read = 0;
    bool silent = _verbosity < 1;
    printOut("Decoding ...", !silent);
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

            _os = new ofstream(_outputName.c_str(), ofstream::binary);

            if (!*_os) {
                cerr << "Cannot open output file '" << _outputName + "' for writing: " << endl;
                return Error::ERR_CREATE_FILE;
            }
        }
        catch (exception& e) {
            cerr << "Cannot open output file '" << _outputName << "' for writing: " << e.what() << endl;
            return Error::ERR_CREATE_FILE;
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
                return Error::ERR_OPEN_FILE;
            }

            is = ifs;
        }

        try {
            OutputStream* ds = (printFlag == true) ? &cout : nullptr;
            _cis = new CompressedInputStream(*is, ds, _jobs);

            for (uint i = 0; i < _listeners.size(); i++)
                _cis->addListener(*_listeners[i]);
        }
        catch (IllegalArgumentException& e) {
            cerr << "Cannot create compressed stream: " << e.what() << endl;
            return Error::ERR_CREATE_DECOMPRESSOR;
        }
    }
    catch (exception& e) {
        cerr << "Cannot open input file '" << _inputName << "': " << e.what() << endl;
        return Error::ERR_OPEN_FILE;
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
                return Error::ERR_READ_FILE;
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
                return Error::ERR_READ_FILE;
            }
        } while (decoded == sa._length);
    }
    catch (IOException& e) {
        // Close streams to ensure all data are flushed
        dispose();
        delete[] buf;

        if (_cis->eof()) {
            cerr << "Reached end of stream" << endl;
            return Error::ERR_READ_FILE;
        }

        cerr << e.what() << endl;
        return e.error();
    }
    catch (exception& e) {
        // Close streams to ensure all data are flushed
        dispose();
        delete[] buf;

        if (_cis->eof()) {
            cerr << "Reached end of stream" << endl;
            return Error::ERR_READ_FILE;
        }

        cerr << "An unexpected condition happened. Exiting ..." << endl
             << e.what() << endl;
        return Error::ERR_UNKNOWN;
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
    printOut("", !silent);
    ss.str(string());
    ss << "Decoding:          " << uint(delta) << " ms";
    printOut(ss.str().c_str(), !silent);
    ss.str(string());
    ss << "Input size:        " << _cis->getRead();
    printOut(ss.str().c_str(), !silent);
    ss.str(string());
    ss << "Output size:       " << read;
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

void BlockDecompressor::processCommandLine(int argc, const char* argv[], map<string, string>& map)
{
    string inputName;
    string outputName;
    int verbose = 1;
    string strVerbose = "1";
    string strTasks = "1";
    string strOverwrite = "false";

    for (int i = 1; i < argc; i++) {
        string arg = argv[i];
        arg = ltrim(rtrim(arg));

        // Extract verbosity and output first
        if (arg.compare(0, 9, "-verbose=") == 0) {
            strVerbose = arg.substr(9);
            verbose = atoi(strVerbose.c_str());

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
            printOut("-verbose=<level>     : set the verbosity level [0..4]", true);
            printOut("                       0=silent, 1=default, 2=display block size (byte rounded)", true);
            printOut("                       3=display timings, 4=display extra information", true);
            printOut("-overwrite           : overwrite the output file if it already exists", true);
            printOut("-input=<inputName>   : mandatory name of the input file to decode or 'stdin'", true);
            printOut("-output=<outputName> : optional name of the output file or 'none' or 'stdout'", true);
            printOut("-jobs=<jobs>         : number of concurrent jobs", true);
            printOut("", true);
            printOut("EG. BlockDecompressor -input=foo.knz -overwrite -verbose=2 -jobs=2", true);
            exit(0);
        }
        else if (arg == "-overwrite") {
            strOverwrite = "true";
        }
        else if (arg.compare(0, 7, "-input=") == 0) {
            arg = arg.substr(7);
            inputName = ltrim(rtrim(arg));
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

    string str = inputName;
    transform(str.begin(), str.end(), str.begin(), ::toupper);

    if ((str != "STDIN") && (str.compare(str.length() - 4, 4, ".KNZ") != 0))
        printOut("Warning: the input file name does not end with the .KNZ extension", verbose > 0);

    if (outputName.length() == 0) {
        outputName = (str.compare(str.length() - 4, 4, ".KNZ") == 0) ? inputName.substr(0, inputName.length() - 4)
                                                                     : inputName + ".tmp";
    }

    map["verbose"] = strVerbose;
    map["overwrite"] = strOverwrite;
    map["outputName"] = outputName;
    map["inputName"] = inputName;
    map["jobs"] = strTasks;
}

void BlockDecompressor::printOut(const char* msg, bool print)
{
    if ((print == true) && (msg != nullptr))
        cout << msg << endl;
}

bool BlockDecompressor::addListener(BlockListener* bl)
{
    if (bl == nullptr)
        return false;

    _listeners.push_back(bl);
    return true;
}

bool BlockDecompressor::removeListener(BlockListener* bl)
{
    std::vector<BlockListener*>::iterator it = find(_listeners.begin(), _listeners.end(), bl);

    if (it == _listeners.end())
        return false;

    _listeners.erase(it);
    return true;
}
