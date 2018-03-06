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


#ifdef CONCURRENCY_ENABLED
#include <mutex>
#endif
#include <string>
#include <sstream>
#include <iostream>
#include <sys/stat.h>

#ifdef CONCURRENCY_ENABLED
#include <mutex>
#endif


using namespace std;

template <typename T>::string to_string(T value)
{
	ostringstream os;
	os << value;
	return os.str();
}

inline string __trim(string& str, bool left, bool right)
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

    return str.substr(begin, end - begin + 1);
}

inline string trim(string& str)  { return __trim(str, true, true); }
inline string ltrim(string& str) { return __trim(str, true, false); }
inline string rtrim(string& str) { return __trim(str, false, true); }

inline bool samePaths(string& f1, string& f2)
{
   if (f1.compare(f2) == 0)
      return true;

   struct stat buf1;
   int s1 = stat(f1.c_str(), &buf1);
   struct stat buf2;
   int s2 = stat(f2.c_str(), &buf2);

   if (s1 != s2)   
      return false;

   if (buf1.st_dev != buf2.st_dev)
      return false;

   if (buf1.st_ino != buf2.st_ino)
      return false;

   if (buf1.st_mode != buf2.st_mode)
      return false;

   if (buf1.st_nlink != buf2.st_nlink)
      return false;

   if (buf1.st_uid != buf2.st_uid)
      return false;

   if (buf1.st_gid != buf2.st_gid)
      return false;

   if (buf1.st_rdev != buf2.st_rdev)
      return false;

   if (buf1.st_size != buf2.st_size)
      return false;

   if (buf1.st_atime != buf2.st_atime)
      return false;

   if (buf1.st_mtime != buf2.st_mtime)
      return false;

   if (buf1.st_ctime != buf2.st_ctime)
      return false;

   return true;
}


inline string toString(int data[], int length) {
   stringstream ss;

   for (int i = 0; i < length; i++) {
       ss << i << " ";
   }

   return ss.str();
}

inline void fromString(string s, int data[], int length) {
   int n = 0;
   int idx = 0;

   for (uint i = 0; (i < s.length()) && (idx < length); i++) {
      if (s[i] != ' ')
         n = (10 * n) + s[i] - '0';
      else {
         data[idx++] = n;
         n = 0;
      }
   }
}

// Thread safe printer
class Printer 
{
   public:
      Printer(ostream* os) { _os = os; }
      ~Printer() {}

      void println(const char* msg, bool print) {
         if ((print == true) && (msg != nullptr)) {
#ifdef CONCURRENCY_ENABLED
            lock_guard<mutex> lock(_mtx);
#endif
            (*_os) << msg << endl;
         }
      }

   private:
#ifdef CONCURRENCY_ENABLED
      static mutex _mtx;
#endif
      ostream* _os;
};

#endif