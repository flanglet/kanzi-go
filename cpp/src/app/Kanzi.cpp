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

int main(int argc, const char* argv[]) {

    if (argc > 1) {
       string str = argv[1];
       transform(str.begin(), str.end(), str.begin(), ::toupper);

       if ((str == "--COMPRESS") || (str == "-C")) {
		   // Remove argv[1]
		   for (int i=1; i<argc; i++)
			   argv[i] = argv[i+1];

         return kanzi::BlockCompressor::main(argc-1, argv);
       } else if ((str == "--DECOMPRESS") || (str == "-D"))  {
		   // Remove argv[1]
		   for (int i = 1; i<argc; i++)
			   argv[i] = argv[i + 1];

         return kanzi::BlockDecompressor::main(argc-1, argv);
       } else if ((str == "--HELP") || (str == "-H")) {
          cout << "kanzi --compress | --decompress | --help" << endl;
          return 0;
       }
    }

   cout << "Missing arguments: try '--help'" << endl;
   return 1;
}
