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
	virtual T result() = 0;
};

#if __cplusplus >= 201103L || _MSC_VER >= 1700
// C++ 11 (or partial)
#define CONCURRENCY_ENABLED
#include <atomic>
#include <thread>
#include <deque>
#include <mutex>
#include <condition_variable>
#include "IllegalArgumentException.hpp"

template <class T>
class ThreadPool {
public:
	ThreadPool(int size, bool deallocateTasks = false) THROW;

	void add(Task<T>* task);

	int active_tasks();

	~ThreadPool();

private:
	friend class Task<T>;

	void runThread();

	bool _deallocateTasks;
	vector<thread> _threads;
	deque<Task<T>*> _tasks;
	mutex _mutex;
	condition_variable _condition;
	atomic_bool _stop;
};

template <class T>
ThreadPool<T>::ThreadPool(int jobs, bool deallocateTasks) THROW : _stop(false)
{
	if (jobs < 1)
		throw kanzi::IllegalArgumentException("At least 1 thread required to create a thread pool");

	for (int i = 0; i < jobs; i++) {
		_threads.push_back(thread(&ThreadPool<T>::runThread, this));
	}

	_deallocateTasks = deallocateTasks;
}

template <class T>
ThreadPool<T>::~ThreadPool()
{
	_stop.store(true);
	_condition.notify_all();

	for (uint i = 0; i < _threads.size(); i++)
		_threads[i].join();
}

template <class T>
void ThreadPool<T>::add(Task<T>* task)
{
	if (task == nullptr)
		return;

	{
		unique_lock<mutex> lock(_mutex);
		_tasks.push_back(task);
	} // release lock

	_condition.notify_one();
}

template <class T>
int ThreadPool<T>::active_tasks()
{
	unique_lock<mutex> lock(_mutex);
	return int(_tasks.size());
}

template <class T>
void ThreadPool<T>::runThread()
{
	while (true) {
		Task<T>* task;

		{
			unique_lock<mutex> lock(_mutex);

			// Busy loop, waiting for a task to run
			while ((_stop.load() == false) && (_tasks.empty())) {
				_condition.wait_for(lock, chrono::nanoseconds(100));
			}

			if (_stop.load() == true)
				break;

			task = _tasks.front();
			_tasks.pop_front();
		} // release lock

		  // Run task
		task->call();

		if (_deallocateTasks)
			delete task;
	}
}

#else
const int memory_order_acquire = 0;
#include <iostream>
// ! Stubs for NON CONCURRENT USAGE !
// Used to compile and provide a non concurrent version
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
	atomic_int& operator++(int n)
	{
		_n += n;
		return *this;
	}
};

class atomic_bool {
private:
	bool _b;

public:
	atomic_bool() { _b = false; }
	atomic_bool& operator=(bool b) { _b = b; return *this; }
	bool load() const { return _b; }
	void store(bool b) { _b = b; }
	bool exchange(bool expected, int memory) {
		bool b = _b;
		_b = expected;
		return b;
	}
};

// We just need to be able to instantiate a ThreadPool.
// Calling a method on it shall fail at compilation time.
template <class T>
class ThreadPool {
public:
	ThreadPool(int size) {}
	~ThreadPool() {}
};

#endif

#endif
