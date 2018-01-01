Build Kanzi
===========

The C++ code can be built on Windows with Visual Studio and Linux with g++.
Porting to other operating systems should be straightforward.

### Visual Studio 2008
This version of VS uses C++03 (AFAIK). Unzip the file "Kanzi_VS2008.zip" in place.
The project generates a Windows 32 binary. Multithreading is not supported with this version.

### Visual Studio 2015
This version of VS uses C++11 (AFAIK). Unzip the file "Kanzi_VS2015.zip" in place.
The project generates a Windows 64 binary. Multithreading is supported with this version.

### mingw-w64
Go to the Kanzi directory and run 'mingw32-make.exe'. The Makefile contains all the necessary
targets. Tested successfully on Win64 with mingw-w64 using Gnu Make 4.2.1.
Multithreading is supported.

### Linux
Go to the Kanzi directory and run 'make'. The Makefile contains all the necessary
targets. g++ is required. Tested successfully on Ubuntu 16.04 with g++ 5.4.0
and clang++ 3.8.1. Multithreading is supported with g++ version 5.0.0 or newer.
