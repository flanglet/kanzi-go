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

import kanzi.Predictor;


// Context model predictor based on BCM by Ilya Muravyov. 
// See https://github.com/encode84/bcm
public class CMPredictor implements Predictor
{
   private static final int FAST_RATE   = 2;
   private static final int MEDIUM_RATE = 4;
   private static final int SLOW_RATE   = 6;
   
   private int c1;
   private int c2;
   private int ctx;
   private int run;
   private int idx;
   private int runMask;
   private final int[][] counter1;
   private final int[][] counter2;
   

   public CMPredictor()
   {   
      this.ctx = 1;
      this.run = 1;
      this.idx = 8;
      this.counter1 = new int[256][257];
      this.counter2 = new int[512][17];
      
      for (int i=0; i<256; i++)
      {        
         for (int j=0; j<=256; j++)
            this.counter1[i][j] = 32768;
            
         for (int j=0; j<=16; j++)
         {
            this.counter2[i+i][j]   = j << 12;
            this.counter2[i+i+1][j] = j << 12;
         }
   
         this.counter2[i+i][16]   -= 16;
         this.counter2[i+i+1][16] -= 16;
      }			
   }
   
   
   // Update the probability model
   @Override
   public void update(int bit)
   { 
      final int[] counter1_ = this.counter1[this.ctx];
      this.ctx <<= 1;
      final int[] counter2_ = this.counter2[this.ctx|this.runMask];
      
      if (bit == 0)
      {
         counter1_[256]        -= (counter1_[256]        >> FAST_RATE);
         counter1_[this.c1]    -= (counter1_[this.c1]    >> MEDIUM_RATE);
         counter2_[this.idx+1] -= (counter2_[this.idx+1] >> SLOW_RATE);         
         counter2_[this.idx]   -= (counter2_[this.idx]   >> SLOW_RATE);
      } 
      else
      {
         counter1_[256]        += ((counter1_[256]^0xFFFF)        >> FAST_RATE);
         counter1_[this.c1]    += ((counter1_[this.c1]^0xFFFF)    >> MEDIUM_RATE);
         counter2_[this.idx+1] += ((counter2_[this.idx+1]^0xFFFF) >> SLOW_RATE);
         counter2_[this.idx]   += ((counter2_[this.idx]^0xFFFF)   >> SLOW_RATE);
         this.ctx++;
      } 
         
      if (this.ctx > 255)
      {
         this.c2 = this.c1;
         this.c1 = this.ctx & 0xFF;
         this.ctx = 1;

         if (this.c1 == this.c2)
         {
            this.run++;
            this.runMask = (2-this.run) >>> 31;
         }
         else
         {
            this.run = 0; 
            this.runMask = 0;
         }
      }
   }
   
   
   // Return the split value representing the probability of 1 in the [0..4095] range. 
   @Override
   public int get()
   {
      final int[] pc1 = this.counter1[this.ctx];
      final int p = (13*pc1[256]+14*pc1[this.c1]+5*pc1[this.c2]) >> 5;
      this.idx = p >>> 12;
      final int[] pc2 = this.counter2[(this.ctx<<1)|this.runMask];
      final int x1 = pc2[this.idx];
      final int x2 = pc2[this.idx+1];
      final int ssep = x1 + (((x2-x1)*(p&4095)) >> 12);
      return (p + ssep + ssep + ssep + 32) >>> 6; // rescale to [0..4095]
   }
}
