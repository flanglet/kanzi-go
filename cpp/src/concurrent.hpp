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

#ifndef _concurrent_
#define _concurrent_

using namespace std;

template <class T>
class Task {
public:
    Task() {}
    virtual ~Task() {}
    virtual T call() = 0;
};

#if __cplusplus >= 201103L || _MSC_VER >= 1700
// C++ 11 (or partial)
#include <atomic>

#ifndef CONCURRENCY_ENABLED
#ifdef __GNUC__
// Require g++ 5.0 minimum, 4.8.4 generates exceptions on futures (?)
#if ((__GNUC__ << 16) + __GNUC_MINOR__ >= (5 << 16) + 0)
#define CONCURRENCY_ENABLED
#endif
#else
#define CONCURRENCY_ENABLED
#endif
#endif
#endif

#ifdef CONCURRENCY_ENABLED
#include <chrono>

using namespace chrono;

class Clock {
private:
    steady_clock::time_point _start;
    steady_clock::time_point _stop;

public:
    Clock()
    {
        start();
        _stop = _start;
    }

    void start()
    {
        _start = steady_clock::now();
    }

    void stop()
    {
        _stop = steady_clock::now();
    }

    double elapsed() const
    {
        // In millisec
        return duration_cast<duration<double> >(_stop - _start).count() * 1000.0;
    }
};
#else
#include <ctime>

class Clock {
private:
    clock_t _start;
    clock_t _stop;

public:
    Clock()
    {
        start();
    }

    void start()
    {
        _start = clock();
    }

    void stop()
    {
        _stop = clock();
    }

    double elapsed() const
    {
        // In millisec
        return double(_stop - _start) / CLOCKS_PER_SEC * 1000.0;
    }
};

#if (__cplusplus && __cplusplus < 201103L) || (_MSC_VER && _MSC_VER < 1700)
// ! Stubs for NON CONCURRENT USAGE !
// Used to compile and provide a non concurrent version AND
// when atomic.h is not available (VS C++)
const int memory_order_acquire = 0;
#include <iostream>

class atomic_int {
private:
    int _n;

public:
    atomic_int() { _n = 0; }
    atomic_int& operator=(int n)
    {
        _n = n;
        return *this;
    }
    int load() const { return _n; }
    void store(int n) { _n = n; }
    atomic_int& operator++(int)
    {
        _n++;
        return *this;
    }
};

class atomic_bool {
private:
    bool _b;

public:
    atomic_bool() { _b = false; }
    atomic_bool& operator=(bool b)
    {
        _b = b;
        return *this;
    }
    bool load() const { return _b; }
    void store(bool b) { _b = b; }
    bool exchange(bool expected, int)
    {
        bool b = _b;
        _b = expected;
        return b;
    }
};
#endif //   (__cplusplus && __cplusplus < 201103L) || (_MSC_VER && _MSC_VER < 1700)

#endif // CONCURRENCY_ENABLED

#endif
