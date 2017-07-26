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

#ifndef _DivSufSort_
#define _DivSufSort_

#include "../types.hpp"

namespace kanzi
{

   // DivSufSort is a fast two-stage suffix sorting algorithm by Yuta Mori.
   // The original C code is here: https://code.google.com/p/libdivsufsort/
   // See also https://code.google.com/p/libdivsufsort/source/browse/wiki/SACA_Benchmarks.wiki
   // for comparison of different suffix array construction algorithms.
   // It is used to implement the forward stage of the BWT in linear time.
   class StackElement
   {
       friend class DivSufSort;
       friend class Stack;

   private:
       int _a, _b, _c, _d, _e;

       StackElement();

       ~StackElement() {}
   };

   // A stack of pre-allocated elements
   class Stack
   {
       friend class DivSufSort;

   private:
       StackElement** _arr;
       int _index;
       int _length;

       Stack(int size);

       ~Stack();

       StackElement* get(int idx) const { return _arr[idx]; }

       int size() const { return _index; }

       void push(int a, int b, int c, int d, int e);

       StackElement* pop();
   };

   class TRBudget
   {
       friend class DivSufSort;

   private:
       int _chance;
       int _remain;
       int _incVal;
       int _count;

       TRBudget(int chance, int incval);

       ~TRBudget() {}

       bool check(int size);
   };

   class DivSufSort
   {
   private:
       static const int SS_INSERTIONSORT_THRESHOLD = 8;
       static const int SS_BLOCKSIZE = 1024;
       static const int SS_MISORT_STACKSIZE = 16;
       static const int SS_SMERGE_STACKSIZE = 32;
       static const int TR_STACKSIZE = 64;
       static const int TR_INSERTIONSORT_THRESHOLD = 8;
       static const int SQQ_TABLE[];
       static const int LOG_TABLE[];

       int _length;
       int* _sa;
       short* _buffer;
       int _bucketA[256];
       int _bucketB[65536];
       Stack* _ssStack;
       Stack* _trStack;
       Stack* _mergeStack;

       void constructSuffixArray(int bucket_A[], int bucket_B[], int n, int m);

       int constructBWT(int bucket_A[], int bucket_B[], int n, int m);

       int sortTypeBstar(int bucket_A[], int bucket_B[], int n);

       void ssSort(int pa, int first, int last, int buf, int bufSize,
           int depth, int n, bool lastSuffix);

       int ssCompare(int pa, int pb, int p2, int depth);

       int ssCompare(int p1, int p2, int depth);

       void ssInplaceMerge(int pa, int first, int middle, int last, int depth);

       void ssRotate(int first, int middle, int last);

       void ssBlockSwap(int a, int b, int n);

       static int getIndex(int a) { return (a >= 0) ? a : ~a; }

       void ssSwapMerge(int pa, int first, int middle, int last, int buf,
           int bufSize, int depth);

       void ssMergeForward(int pa, int first, int middle, int last, int buf,
           int depth);

       void ssMergeBackward(int pa, int first, int middle, int last, int buf,
           int depth);

       void ssInsertionSort(int pa, int first, int last, int depth);

       static int ssIsqrt(int x);

       void ssMultiKeyIntroSort(const int pa, int first, int last, int depth);

       int ssPivot(int td, int pa, int first, int last);

       int ssMedian5(const int idx, int pa, int v1, int v2, int v3, int v4, int v5);

       int ssMedian3(int idx, int pa, int v1, int v2, int v3);

       int ssPartition(int pa, int first, int last, int depth);

       void ssHeapSort(int idx, int pa, int saIdx, int size);

       void ssFixDown(int idx, int pa, int saIdx, int i, int size);

       static int ssIlg(int n);

       void swapInSA(int a, int b);

       void trSort(int n, int depth);

       uint64 trPartition(int isad, int first, int middle, int last, int v);

       void trIntroSort(int isa, int isad, int first, int last, TRBudget& budget);

       static int trPivot(int arr[], int isad, int first, int last);

       static int trMedian5(int arr[], int isad, int v1, int v2, int v3, int v4, int v5);

       static int trMedian3(int arr[], int isad, int v1, int v2, int v3);

       void trHeapSort(int isad, int saIdx, int size);

       void trFixDown(int isad, int saIdx, int i, int size);

       void trInsertionSort(int isad, int first, int last);

       void trPartialCopy(int isa, int first, int a, int b, int last, int depth);

       void trCopy(int isa, int first, int a, int b, int last, int depth);

       void reset();

       static int trIlg(int n);

   public:
       DivSufSort();

       ~DivSufSort();

       void computeSuffixArray(byte input[], int sa[], int start, int length);

       int computeBWT(byte input[], int sa[], int start, int length);
   };

}
#endif