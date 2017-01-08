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

#ifndef _util_
#define _util_

#include <string>
#include <sstream>

using namespace std;

template <typename T>::string to_string(T value)
{
	ostringstream os;
	os << value;
	return os.str();
}

inline string& __trim(string& str, bool left, bool right)
{
    string::size_type begin=0;
    string::size_type end=str.size()-1;

    if (left) {
       while (begin<=end && (str[begin]<=0x20 || str[begin]==0x7F))
           begin++;
    }

    if (right) {
       while (end>begin && (str[end]<=0x20 || str[end]==0x7F))
         end--;
    }

    str = str.substr(begin, end - begin + 1);
    return str;
}

inline string& trim(string& str)  { return __trim(str, true, true); }
inline string& ltrim(string& str) { return __trim(str, true, false); }
inline string& rtrim(string& str) { return __trim(str, false, true); }

#endif