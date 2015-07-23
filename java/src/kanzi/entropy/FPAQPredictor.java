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
   private static final int THRESHOLD = 96;
   
   private final short[] states; // 256 frequency contexts for each bit
   private int ctxIdx; // previous bits
   private int prediction;
   
   
   public FPAQPredictor()
   {
      this.ctxIdx = 2;
      this.states = new short[512];    
      this.prediction = 2048;
   }
   
   
   // Update the probability model
   @Override
   public void update(int bit)
   {
      final int idx = this.ctxIdx | (bit & 1);
      
      // Find the number of registered 0 & 1 given the previous bits (in this.ctxIdx)
      if (++this.states[idx] >= THRESHOLD) 
      {
         this.states[idx&-2] >>= 1;
         this.states[(idx&-2)+1] >>= 1;
      }
      
      // Update context by registering the current bit (or wrapping after 8 bits)
      if (idx < 256)
      {
         this.ctxIdx = idx << 1;
         this.prediction = ((this.states[this.ctxIdx+1]+1)<<12) / (this.states[this.ctxIdx]+this.states[this.ctxIdx+1]+2);          
      }
      else
      {
         this.ctxIdx = 2;
         this.prediction = ((this.states[3]+1)<<12) / (this.states[2]+this.states[3]+2);             
      }
   }

   
   // Return the split value representing the probability of 1 in the [0..4095] range. 
   @Override
   public int get()
   {
      return this.prediction;
   }
}   