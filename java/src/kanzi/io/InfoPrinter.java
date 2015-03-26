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

package kanzi.io;

import java.io.PrintStream;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;

// An implementation of BlockListener to display block information (verbose option
// of the BlockCompressor/BlockDecompressor)
public class InfoPrinter implements BlockListener
{
   public enum Type { ENCODING, DECODING };
   
   private final PrintStream ps;
   private final AtomicInteger blockId;
   private final Map<Integer, BlockInfo> map;
   private final BlockEvent.Type[] thresholds;
   private final Type type;
   
   
   public InfoPrinter(Type type, PrintStream ps) 
   {
      if (ps == null)
         throw new NullPointerException("Invalid null print stream parameter");
      
      this.ps = ps;
      this.blockId = new AtomicInteger();
      this.type = type;
      this.map = new ConcurrentHashMap<Integer, BlockInfo>();
      this.thresholds = (type == Type.ENCODING) ? 
              new BlockEvent.Type[]
              { 
                 BlockEvent.Type.BEFORE_TRANSFORM,
                 BlockEvent.Type.AFTER_TRANSFORM,
                 BlockEvent.Type.AFTER_ENTROPY
              } :
              new BlockEvent.Type[]
              { 
                 BlockEvent.Type.AFTER_ENTROPY,
                 BlockEvent.Type.BEFORE_TRANSFORM,
                 BlockEvent.Type.AFTER_TRANSFORM
              };
   }
   
   
   @Override
   public void processEvent(BlockEvent evt) 
   {
      int currentBlockId = evt.getId();

      if (evt.getType() == this.thresholds[0])
      {
         // Register initial block size
         BlockInfo bi = new BlockInfo();
         bi.stage0Size = evt.getSize();
         this.map.put(currentBlockId, bi);
         bi.time = System.nanoTime();
      }
      else if (evt.getType() == this.thresholds[1])
      {
         // Register block size after stage 1
         BlockInfo bi = this.map.get(currentBlockId);
         
         if (bi != null)
            bi.stage1Size = evt.getSize();
      }
      else if (evt.getType() == this.thresholds[2])
      {        
         // Get block size after stage 2
         int stage2Size = evt.getSize();
         BlockInfo bi = this.map.remove(currentBlockId);
         
         if (bi == null)
            return;
             
         //long duration_ms = (System.nanoTime() - bi.time) / 1000000L; 
         
         // Display block info
         StringBuilder msg = new StringBuilder();
         msg.append(String.format("Block %d: %d => %d => %d", currentBlockId, 
                 bi.stage0Size, bi.stage1Size, stage2Size));

         // Add percentage for encoding
         if (this.type == Type.ENCODING)
            msg.append(String.format(" (%d%%)", (stage2Size*100L/(long) bi.stage0Size)));
         
         // Optionally add hash
         if (evt.getHash() != null) 
         {
            msg.append("  [");
            msg.append(Integer.toHexString(evt.getHash()));
            msg.append("]");
         }
         
         //msg.append(String.format(" [%d ms]", duration_ms));
         this.ps.println(msg.toString());
         this.blockId.getAndSet(currentBlockId);
      }
   }
   
   
   static class BlockInfo
   {
      long time;
      int stage0Size;
      int stage1Size;
   }
   
}
