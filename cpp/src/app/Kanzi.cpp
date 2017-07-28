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

#include <iostream>
#include <algorithm>
#include "BlockCompressor.hpp"
#include "BlockDecompressor.hpp"
#include "../io/Error.hpp"

using namespace kanzi;

static const string CMD_LINE_ARGS[13] = {
    "-c", "-d", "-i", "-o", "-b", "-t", "-e", "-j", "-v", "-l", "-x", "-f", "-h"
};

static const int ARG_IDX_COMPRESS = 0;
static const int ARG_IDX_DECOMPRESS = 1;
static const int ARG_IDX_INPUT = 2;
static const int ARG_IDX_OUTPUT = 3;
static const int ARG_IDX_BLOCK = 4;
static const int ARG_IDX_TRANSFORM = 5;
static const int ARG_IDX_ENTROPY = 6;
static const int ARG_IDX_JOBS = 7;
static const int ARG_IDX_VERBOSE = 8;
static const int ARG_IDX_LEVEL = 9;

void printOut(const char* msg, bool print)
{
    if ((print == true) && (msg != nullptr))
        cout << msg << endl;
}

void processCommandLine(int argc, const char* argv[], map<string, string>& map)
{
    string inputName;
    string outputName;
    string strLevel = "";
    string strVerbose = "1";
    string strTasks = "1";
    string strBlockSize = "";
    string strOverwrite = "false";
    string strChecksum = "false";
    string codec = "";
    string transf = "";
    int verbose = 1;
    int ctx = -1;
    int level = -1;
    string mode = " ";

    for (int i = 1; i < argc; i++) {
        string arg = ltrim(rtrim(argv[i]));

        if (arg.compare(0, 2, "-o") == 0) {
            ctx = ARG_IDX_OUTPUT;
            continue;
        }

        if (arg.compare(0, 2, "-v") == 0) {
            ctx = ARG_IDX_VERBOSE;
            continue;
        }

        // Extract verbosity, output and mode first
        if ((arg.compare(0, 10, "--compress") == 0) || (arg.compare(0, 2, "-c") == 0)) {
            if (mode == "d") {
                cerr << "Both compression and decompression options were provided." << endl;
                exit(Error::ERR_INVALID_PARAM);
            }

            mode = "c";
            continue;
        }

        if ((arg.compare(0, 12, "--decompress") == 0) || (arg.compare(0, 2, "-d") == 0)) {
            if (mode == "c") {
                cerr << "Both compression and decompression options were provided." << endl;
                exit(Error::ERR_INVALID_PARAM);
            }

            mode = "d";
            continue;
        }

        if ((arg.compare(0, 10, "--verbose=") == 0) || (ctx == ARG_IDX_VERBOSE)) {
            strVerbose = (arg.compare(0, 10, "--verbose=") == 0) ? arg.substr(10) : arg;
            int verbose = atoi(strVerbose.c_str());

            if ((verbose < 0) || (verbose > 4)) {
                cerr << "Invalid verbosity level provided on command line: " << arg << endl;
                exit(Error::ERR_INVALID_PARAM);
            }
        }
        else if ((arg.compare(0, 9, "--output=") == 0) || (ctx == ARG_IDX_OUTPUT)) {
            arg = (arg.compare(0, 9, "--output=") == 0) ? arg.substr(9) : arg;
            outputName = ltrim(rtrim(arg));
        }

        ctx = -1;
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

    ctx = -1;

    for (int i = 1; i < argc; i++) {
        string arg = ltrim(rtrim(argv[i]));

        if ((arg == "--help") || (arg == "-h")) {
            printOut("", true);
            printOut("   -h, --help", true);
            printOut("        display this message\n", true);
            printOut("   -v, --verbose=<level>", true);
            printOut("        set the verbosity level [0..4]", true);
            printOut("        0=silent, 1=default, 2=display block size (byte rounded)", true);
            printOut("        3=display timings, 4=display extra information\n", true);
            printOut("   -f, --force", true);
            printOut("        overwrite the output file if it already exists\n", true);
            printOut("   -i, --input=<inputName>", true);
            printOut("        mandatory name of the input file or 'stdin'\n", true);
            printOut("   -o, --output=<outputName>", true);
            printOut("        optional name of the output file (defaults to <input.knz>) or 'none'", true);
            printOut("        or 'stdout'\n", true);

            if (mode.compare(0, 1, "d") != 0) {
                printOut("   -b, --block=<size>", true);
                printOut("        size of blocks, multiple of 16, max 1 GB, min 1 KB, default 1 MB\n", true);
                printOut("   -l, --level=<compression>", true);
                printOut("        set the compression level [0..5]", true);
                printOut("        Providing this option forces entropy and transform.", true);
                printOut("        0=None&None (store), 1=TEXT+LZ4&HUFFMAN, 2=BWT+RANK+ZRLT&RANGE", true);
                printOut("        3=BWT+RANK+ZRLT&FPAQ, 4=BWT&CM, 5=RLT+TEXT&TPAQ\n", true);
                printOut("   -e, --entropy=<codec>", true);
                printOut("        entropy codec [None|Huffman|ANS|Range|PAQ|FPAQ|TPAQ|CM]", true);
                printOut("       (default is Huffman)\n", true);
                printOut("   -t, --transform=<codec>", true);
                printOut("        transform [None|BWT|BWTS|SNAPPY|LZ4|RLT|ZRLT|MTFT|RANK|TEXT|TIMESTAMP]", true);
                printOut("        EG: BWT+RANK or BWTS+MTFT (default is BWT+MTFT+ZRLT)\n", true);
                printOut("   -x, --checksum", true);
                printOut("        enable block checksum\n", true);
            }

            printOut("   -j, --jobs=<jobs>", true);
            printOut("        number of concurrent jobs\n", true);
            printOut("", true);
            stringstream ss;

            if (mode.compare(0, 1, "d") != 0) {
                printOut("EG. Kanzi -c -i foo.txt -o none -b 4m -l 4 -v 3\n", true);
                printOut("EG. Kanzi -c -i foo.txt -o foo.knz -f -t BWT+MTFT+ZRLT -b 4m -e FPAQ -v 3 -j 4\n", true);
                printOut("EG. Kanzi --compress --input=foo.txt --output=foo.knz --force", true);
                printOut("          --transform=BWT+MTFT+ZRLT --block=4m --entropy=FPAQ --verbose=3 --jobs=4\n", true);
            }

            if (mode.compare(0, 1, "c") != 0) {
                printOut("EG. Kanzi -d -i foo.knz -f -v 2 -j 2\n", true);
                printOut("EG. Kanzi --decompress --input=foo.knz --force --verbose=2 --jobs=2\n", true);
            }

            exit(0);
        }

        if ((arg == "--compress") || (arg == "-c") || (arg == "--decompress") || (arg == "-d")) {
            if (ctx != -1) {
                stringstream ss;
                ss << "Warning: ignoring option [" << CMD_LINE_ARGS[ctx] << "] with no value.";
                printOut(ss.str().c_str(), verbose > 0);
            }

            ctx = -1;
            continue;
        }

        if ((arg == "--force") || (arg == "-f")) {
            if (ctx != -1) {
                stringstream ss;
                ss << "Warning: ignoring option [" << CMD_LINE_ARGS[ctx] << "] with no value.";
                printOut(ss.str().c_str(), verbose > 0);
            }

            strOverwrite = "true";
            ctx = -1;
            continue;
        }

        if ((arg == "--checksum") || (arg == "-x")) {
            if (ctx != -1) {
                stringstream ss;
                ss << "Warning: ignoring option [" << CMD_LINE_ARGS[ctx] << "] with no value.";
                printOut(ss.str().c_str(), verbose > 0);
            }

            strChecksum = "true";
            ctx = -1;
            continue;
        }

        if (ctx == -1) {
            int idx = -1;

            for (int i = 0; i < 10; i++) {
                if (arg == CMD_LINE_ARGS[i]) {
                    idx = i;
                    break;
                }
            }

            if (idx != -1) {
                ctx = idx;
                continue;
            }
        }

        if ((arg.compare(0, 8, "--input=") == 0) | (ctx == ARG_IDX_INPUT)) {
            inputName = (arg.compare(0, 8, "--input=") == 0) ? arg.substr(8) : arg;
            inputName = ltrim(rtrim(inputName));
            ctx = -1;
            continue;
        }

        if ((arg.compare(0, 10, "--entropy=") == 0) || (ctx == ARG_IDX_ENTROPY)) {
            codec = (arg.compare(0, 10, "--entropy=") == 0) ? arg.substr(10) : arg;
            codec = ltrim(rtrim(codec));
            transform(codec.begin(), codec.end(), codec.begin(), ::toupper);
            ctx = -1;
            continue;
        }

        if ((arg.compare(0, 12, "--transform=") == 0) || (ctx == ARG_IDX_TRANSFORM)) {
            transf = (arg.compare(0, 12, "--transform=") == 0) ? arg.substr(12) : arg;
            transf = ltrim(rtrim(transf));
            transform(transf.begin(), transf.end(), transf.begin(), ::toupper);
            ctx = -1;
            continue;
        }

        if ((arg.compare(0, 10, "--level=") == 0) || (ctx == ARG_IDX_LEVEL)) {
            strLevel = (arg.compare(0, 8, "--level=") == 0) ? arg.substr(8) : arg;
            level = atoi(strLevel.c_str());

            if ((level < 0) || (level > 5)) {
                cerr << "Invalid compression level provided on command line: " << arg << endl;
                exit(Error::ERR_INVALID_PARAM);
            }

            ctx = -1;
            continue;
        }

        if ((arg.compare(0, 8, "--block=") == 0) || (ctx == ARG_IDX_BLOCK)) {
            string str = (arg.compare(0, 8, "--block=") == 0) ? arg.substr(8) : arg;
            str = ltrim(rtrim(str));
            transform(str.begin(), str.end(), str.begin(), ::toupper);
            char lastChar = (str.length() == 0) ? ' ' : str[str.length() - 1];
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
            ctx = -1;
            continue;
        }

        if ((arg.compare(0, 7, "--jobs=") == 0) || (ctx == ARG_IDX_JOBS)) {
            strTasks = (arg.compare(0, 7, "--jobs=") == 0) ? arg.substr(7) : arg;
            int tasks = atoi(strTasks.c_str());

            if (tasks < 1) {
                cerr << "Invalid number of jobs provided on command line: " << arg << endl;
                exit(Error::ERR_INVALID_PARAM);
            }

            ctx = -1;
            continue;
        }

        if ((arg.compare(0, 10, "--verbose=") != 0) && (ctx == -1) && (arg.compare(0, 9, "--output=") != 0)) {
            stringstream ss;
            ss << "Warning: ignoring unknown option [" << arg << "]";
            printOut(ss.str().c_str(), verbose > 0);
        }

        ctx = -1;
    }

    if (inputName.length() == 0) {
        cerr << "Missing input file name, exiting ..." << endl;
        exit(Error::ERR_MISSING_PARAM);
    }

    if (outputName.length() == 0) {
        outputName = inputName + ".knz";
    }

    if (ctx != -1) {
        stringstream ss;
        ss << "Warning: ignoring option with missing value [" << CMD_LINE_ARGS[ctx] << "]";
        printOut(ss.str().c_str(), verbose > 0);
    }

    if (level >= 0) {
        if (codec.length() > 0) {
            stringstream ss;
            ss << "Warning: providing the 'level' option forces the entropy codec. Ignoring [" << codec << "]";
            printOut(ss.str().c_str(), verbose > 0);
        }

        if (transf.length() > 0) {
            stringstream ss;
            ss << "Warning: providing the 'level' option forces the transform. Ignoring [" << transf << "]";
            printOut(ss.str().c_str(), verbose > 0);
        }
    }

    if (strBlockSize.length() > 0)
        map["block"] = strBlockSize;

    map["verbose"] = strVerbose;
    map["mode"] = mode;
    map["level"] = strLevel;

    if (strOverwrite == "true")
        map["overwrite"] = strOverwrite;

    map["inputName"] = inputName;
    map["outputName"] = outputName;

    if (codec.length() > 0)
        map["entropy"] = codec;

    if (transf.length() > 0)
        map["transform"] = transf;

    if (strChecksum == "true")
        map["checksum"] = strChecksum;

    map["jobs"] = strTasks;
}

int main(int argc, const char* argv[])
{
    map<string, string> args;
    processCommandLine(argc, argv, args);
    map<string, string>::iterator it = args.find("mode");
    string mode = it->second;
    args.erase(it);

    if (mode == "c") {
        try {
            BlockCompressor bc(args);
            int code = bc.call();
            exit(code);
        }
        catch (exception& e) {
            cerr << "Could not create the compressor: " << e.what() << endl;
            exit(Error::ERR_CREATE_COMPRESSOR);
        }
    }

    if (mode == "d") {
        try {
            BlockDecompressor bd(args);
            int code = bd.call();
            exit(code);
        }
        catch (exception& e) {
            cerr << "Could not create the decompressor: " << e.what() << endl;
            exit(Error::ERR_CREATE_DECOMPRESSOR);
        }
    }

    it = args.find("help");

    if (it != args.end()) {
        cout << "Kanzi --compress | --decompress | --help" << endl;
        return 1;
    }

    cout << "Missing arguments: try --help or -h" << endl;
    return 1;
}