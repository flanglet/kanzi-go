Build Kanzi
===========

The C++ code can be built on Windows with Visual Studio and Linux with g++.
Porting to other system should be straightforward.

### Visual Studio 2008
This version of VS uses C++03 (AFAIK). Unzip the file "Kanzi_VS2008.zip" in place.
The project generates a Windows 32 binary. Multithreading is not supported with version.


### Visual Studio 2015
This version of VS uses C++11 (AFAIK). Unzip the file "Kanzi_VS2015.zip" in place.
The project generates a Windows 64 binary. Multithreading is supported with version.

### Linux
Go to the Kanzi directory and run 'make'. The Makefile contains all the necessary
targets. g++ is required. Tested successfully on Ubuntu 14.04 and g++ 4.8.4 and
Ununtu 16.04 with g++ 5.4.0.
