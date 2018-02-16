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


// APM maps a probability and a context into a new probability
// that the next bit will be 1. After each guess, it updates
// its state to improve future guesses.  

/*package*/ final class LinearAdaptiveProbMap
{
   private int index;        // last prob context
   private final int rate;   // update rate
   private final int[] data; // prob, context -> prob


   LinearAdaptiveProbMap(int n, int rate)
   {
      this.data = new int[n*65];
      this.rate = rate;

      for (int j=0; j<=64; j++)
         this.data[j] = (j<<6) << 4;

      for (int i=1; i<n; i++)
         System.arraycopy(this.data, 0, this.data, i*65, 65);
   }


   // Return improved prediction given current bit, prediction and context
   int get(int bit, int pr, int ctx)
   {
      // Update probability based on error and learning rate
      final int g = (bit<<16) + (bit<<this.rate) - (bit<<1);
      this.data[this.index] += ((g-this.data[this.index]) >> this.rate);      
      this.data[this.index+1] += ((g-this.data[this.index+1]) >> this.rate);

      // Find index: 65*ctx + quantized prediction in [0..64]
      this.index = (pr>>6) + (ctx<<6) + ctx;

      // Return interpolated probability
      final int w = pr & 127;
      return (this.data[this.index]*(128-w) + this.data[this.index+1]*w) >> 11;
   }   
}
