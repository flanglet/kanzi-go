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


// Context model predictor based on BCM by Ilya Muravyov. 
// See http://sourceforge.net/projects/bcm
public class CMPredictor implements Predictor
{
   private static final int LOW_RATE    = 2;
   private static final int MEDIUM_RATE = 4;
   private static final int FAST_RATE   = 6;
   
   private int c1;
   private int c2;
   private int ctx;
   private int run;
   private int bpos;
   private int idx;
   private final int[] counter0;
   private final int[][] counter1;
   private final int[][][] counter2;
   
   
   public CMPredictor()
   {   
      this.bpos = 7;
      this.ctx = 1;
      this.run = 1;
      this.idx = 8;
      this.counter0 = new int[256];
      this.counter1 = new int[256][256];
      this.counter2 = new int[2][256][17];
      
      for (int i=0; i<256; i++)
      {
         this.counter0[i] = 32768;
         
         for (int j=0; j<256; j++)
            this.counter1[i][j] = 32768;
            
         for (int j=0; j<16; j++)
         {
            this.counter2[0][i][j] = j << 12;
            this.counter2[1][i][j] = j << 12;
         }

         this.counter2[0][i][16] = 15 << 12;
         this.counter2[1][i][16] = 15 << 12;
      }			
   }
   
   
   // Update the probability model
   @Override
   public void update(int bit)
   { 
      final int ctx_ = this.ctx;
      final int runCtx = (2-this.run) >>> 31;
      final int[] counter0_ = this.counter0;
      final int[] counter1_ = this.counter1[this.c1];
      final int[] counter2_ = this.counter2[runCtx][ctx_];
           
      if (bit == 0)
      {
         counter0_[ctx_] -= (counter0_[ctx_] >> LOW_RATE);
         counter1_[ctx_] -= (counter1_[ctx_] >> MEDIUM_RATE);
         counter2_[this.idx] -= (counter2_[this.idx] >> FAST_RATE);
         counter2_[this.idx+1] -= (counter2_[this.idx+1] >> FAST_RATE);
         this.ctx <<= 1;
      }
      else
      {
         counter0_[ctx_] += ((counter0_[ctx_]^0xFFFF) >> LOW_RATE);
         counter1_[ctx_] += ((counter1_[ctx_]^0xFFFF) >> MEDIUM_RATE);
         counter2_[this.idx] += ((counter2_[this.idx]^0xFFFF) >> FAST_RATE);
         counter2_[this.idx+1] += ((counter2_[this.idx+1]^0xFFFF) >> FAST_RATE);
         this.ctx = (ctx_ << 1) | 1;
      } 
      
      this.bpos--;

      if (this.bpos < 0)
      {
        this.c2 = this.c1;
        this.c1 = this.ctx & 0xFF;
        this.bpos = 7;
        this.ctx = 1;

        if (this.c1 == this.c2)
           this.run++;
         else
           this.run = 0;     
      }      
   }

   
   // Return the split value representing the probability of 1 in the [0..4095] range. 
   @Override
   public int get()
   {
      final int p0 = this.counter0[this.ctx];
      final int p1 = this.counter1[this.c1][this.ctx];
      final int p2 = this.counter1[this.c2][this.ctx];
      final int p = ((p0<<2)+p1+p1+p1+p2+4) >> 3;
      this.idx = p >> 12;
      final int runCtx = (2-this.run) >>> 31;
      final int[] counter2_ = this.counter2[runCtx][this.ctx];            
      final int x1 = counter2_[this.idx];
      final int x2 = counter2_[this.idx+1];
      final int ssep = x1 + (((x2-x1)*(p&4095)) >> 12);
      return (p + ssep + ssep + ssep + 32) >> 6; // rescale to [0..4095]
   }
}   