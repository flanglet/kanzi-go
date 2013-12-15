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

package kanzi.prediction;

import kanzi.Global;


public final class Stats
{
   // stats bins
   public final int[] residualBits; // bits written in bitstream for residual
   public final int[] blockDims; //4x4, 8x8, 16x16 or 32x32
   public final int[] error; // error for roundtrip per block
   public final int[] quantizers; // quantizer per block
   public final int[] predictionModes;
   public long quadtreeBits; // bits written in bitstream for quadtree
   public int quadtreeNodes; // nodes in the quadtree


   Stats()
   {
      this.residualBits = new int[128];
      this.blockDims = new int[4];
      this.error = new int[256];
      this.quantizers = new int[256];
      this.predictionModes = new int[8];
      this.quadtreeBits = 0;
   }


   public void updateResidual(int blockDim, int residueLength, int q, int predModeVal, int error)
   {
      if (residueLength >= this.residualBits.length)
         residueLength = this.residualBits.length - 1;

      this.residualBits[(int) residueLength]++;
      int blockDimIdx = 0;

      if (blockDim == 8)
         blockDimIdx = 1;
      else if (blockDim == 16)
         blockDimIdx = 2;
      else if (blockDim == 32)
         blockDimIdx = 3;

      this.blockDims[blockDimIdx]++;

      if (q >= this.quantizers.length)
         q = this.quantizers.length - 1;
      
      this.quantizers[q]++;
      this.predictionModes[predModeVal]++;

      int errorIdx = Global.sqrt((error << 4) / (blockDim*blockDim));
      errorIdx >>= 10;

      if (errorIdx >= this.error.length)
         errorIdx = this.error.length - 1;

      this.error[errorIdx]++;
   }


   public void updateQuadtree(int nodes, long bits)
   {
      this.quadtreeNodes += nodes;
      this.quadtreeBits += bits;
   }
}