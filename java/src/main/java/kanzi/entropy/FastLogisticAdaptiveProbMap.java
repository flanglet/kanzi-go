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
// that the next bit will be 1. After each guess, it updates
// its state to improve future guesses.

/*package*/ final class FastLogisticAdaptiveProbMap
{
   private int index;        // last prob, context
   private final int rate;   // update rate
   private final int[] data; // prob, context -> prob


   FastLogisticAdaptiveProbMap(int n, int rate)
   {
      this.data = new int[n*33];
      this.rate = rate;

      for (int j=0; j<=32; j++)
         this.data[j] = Global.squash((j-16)<<7) << 4;

      for (int i=1; i<n; i++)
         System.arraycopy(this.data, 0, this.data, i*33, 33);
   }


   // Return improved prediction given current bit, prediction and context
   int get(int bit, int pr, int ctx)
   {
      // Update probability based on error and learning rate
      final int g = (bit<<16) + (bit<<this.rate) - (bit<<1);
      this.data[this.index] += ((g-this.data[this.index]) >> this.rate);

      // Find index: 33*ctx + quantized prediction in [0..32]
      this.index = ((Global.STRETCH[pr]+2048)>>7) + (ctx<<5) + ctx;
      return (this.data[this.index]) >> 4;
   }
}
