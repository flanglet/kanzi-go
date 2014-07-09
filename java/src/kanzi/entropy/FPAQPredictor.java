/*
Copyright 2011-2013 Frederic Langlet
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


// Based on fpaq1 by Matt Mahoney
// Simple (and fast) adaptive order 0 entropy coder predictor
public class FPAQPredictor implements Predictor
{
   private static final int THRESHOLD = 200;
   private static final int SHIFT = 1;
   
   private final short[] states; // 256 frequency contexts for each bit
   private int ctxIdx; // previous bits
   private int prediction;
   
   
   public FPAQPredictor()
   {
      this.ctxIdx = 1;
      this.states = new short[512];    
      this.prediction = 2048;
   }
   
   
   @Override
   public void update(int bit)
   {
      int idx = this.ctxIdx << 1;
      
      // Find the number of registered 0 & 1 given the previous bits (in this.ctxIdx)
      if (++this.states[idx+(bit&1)] >= THRESHOLD) 
      {
         this.states[idx] >>>= SHIFT;
         this.states[idx+1] >>>= SHIFT;
      }
      
      // Update context by registering the current bit (or wrapping after 8 bits)
      this.ctxIdx = (idx < 256) ? idx | (bit&1) : 1;
      idx = this.ctxIdx << 1;
      this.prediction = ((this.states[idx+1]+1)<<12) / (this.states[idx]+this.states[idx+1]+2);
   }

   
   // Return the split value representing the probability of 1 in the [0..4095] range. 
   @Override
   public int get()
   {
      return this.prediction;
   }
}   