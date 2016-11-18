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
package kanzi.entropy;

import kanzi.Global;

// APM maps a probability and a context into a new probability
// that bit y will next be 1.  After each guess it updates
// its state to improve future guesses.  Methods:
//
// APM a(N) creates with N contexts, uses 66*N bytes memory.
// a.get(y, pr, cx) returned adjusted probability in context cx (0 to
//   N-1).  rate determines the learning rate (smaller = faster, default 8).
//////////////////////////////////////////////////////////////////
/*package*/ class AdaptiveProbMap
{
   private int index;        // last p, context
   private final int rate;   // update rate 
   private final int[] data; // [NbCtx][33]:  p, context -> p


   AdaptiveProbMap(int n, int rate)
   {
      this.data = new int[n*33];
      this.rate = rate;

      for (int i=0, k=0; i<n; i++, k+=33)
      {
         for (int j=0; j<33; j++)
            this.data[k+j] = (i == 0) ? Global.squash((j-16)<<7) << 4 : this.data[j];
      }
   }


   int get(int bit, int pr, int ctx)
   {
      // Update probability based on error and learning rate
      final int g = (bit<<16) + (bit<<this.rate) - (bit<<1);
      this.data[this.index] += ((g-this.data[this.index]) >> this.rate);
      this.data[this.index+1] += ((g-this.data[this.index+1]) >> this.rate);
      pr = Global.STRETCH[pr];

      // Find new context
      this.index = ((pr+2048)>>7) + (ctx<<5) + ctx;

      // Return interpolated probability
      final int w = pr & 127;
      return (this.data[this.index]*(128-w) + this.data[this.index+1]*w) >> 11;
   }
}
