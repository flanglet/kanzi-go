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

package kanzi.app;

import kanzi.Event;
import java.io.PrintStream;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import kanzi.Listener;

// An implementation of Listener to display block information (verbose option
// of the BlockCompressor/BlockDecompressor)
public class InfoPrinter implements Listener
{
   public enum Type { ENCODING, DECODING }
   
   private final PrintStream ps;
   private final Map<Integer, BlockInfo> map;
   private final Event.Type[] thresholds;
   private final Type type;
   private final int level;
   
   
   public InfoPrinter(int infoLevel, Type type, PrintStream ps) 
   {
      if (ps == null)
         throw new NullPointerException("Invalid null print stream parameter");
      
      this.ps = ps;
      this.level = infoLevel;
      this.type = type;
      this.map = new ConcurrentHashMap<Integer, BlockInfo>();
      this.thresholds = (type == Type.ENCODING) ? 
              new Event.Type[]
              { 
                 Event.Type.COMPRESSION_START,
                 Event.Type.BEFORE_TRANSFORM,
                 Event.Type.AFTER_TRANSFORM,
                 Event.Type.BEFORE_ENTROPY,
                 Event.Type.AFTER_ENTROPY,
                 Event.Type.COMPRESSION_END
              } :
              new Event.Type[]
              { 
                 Event.Type.DECOMPRESSION_START,
                 Event.Type.BEFORE_ENTROPY,
                 Event.Type.AFTER_ENTROPY,
                 Event.Type.BEFORE_TRANSFORM,
                 Event.Type.AFTER_TRANSFORM,
                 Event.Type.DECOMPRESSION_END
              };
   }
   
   
   @Override
   public void processEvent(Event evt) 
   {
      int currentBlockId = evt.getId();

      if (evt.getType() == this.thresholds[1])
      {
         // Register initial block size
         BlockInfo bi = new BlockInfo();
         bi.time0 = evt.getTime();
         
         if (this.type == Type.ENCODING)
            bi.stage0Size = evt.getSize();
         
         this.map.put(currentBlockId, bi);
         
         if (this.level >= 5)
         {
            this.ps.println(evt);
         }
      }
      else if (evt.getType() == this.thresholds[2])
      { 
         BlockInfo bi = this.map.get(currentBlockId);
         
         if (bi == null)
            return;
         
         bi.time1 = evt.getTime();
         
         if (this.type == Type.DECODING)
            bi.stage0Size = evt.getSize();
                          
         if (this.level >= 5)
         {
            long duration_ms = (bi.time1 - bi.time0) / 1000000L; 
            this.ps.println(String.format("%s [%d ms]", evt, duration_ms));
         }
      }
      else if (evt.getType() == this.thresholds[3])
      {
         BlockInfo bi = this.map.get(currentBlockId);
         
         if (bi == null)
            return;
         
         bi.time2 = evt.getTime();
         bi.stage1Size = evt.getSize();
         
         if (this.level >= 5)
         {
            long duration_ms = (bi.time2 - bi.time1) / 1000000L; 
            this.ps.println(String.format("%s [%d ms]", evt, duration_ms));
         }
      }
      else if (evt.getType() == this.thresholds[4])
      {        
         long stage2Size = evt.getSize();
         BlockInfo bi = this.map.remove(currentBlockId);
         
         if ((bi == null) || (this.level < 3))
            return;
             
         bi.time3 = evt.getTime();
         long duration1_ms = (bi.time1 - bi.time0) / 1000000L; 
         long duration2_ms = (bi.time3 - bi.time2) / 1000000L; 
         StringBuilder msg = new StringBuilder();
                  
         if (this.level >= 5)
         {
            this.ps.println(String.format("%s [%d ms]", evt, duration2_ms));
         }
         
         // Display block info
         if (this.level >= 4)
         {
            msg.append(String.format("Block %d: %d => %d [%d ms] => %d [%d ms]", currentBlockId, 
                    bi.stage0Size, bi.stage1Size, duration1_ms, stage2Size, duration2_ms));

            // Add compression ratio for encoding
            if (this.type == Type.ENCODING)
            {
               if (bi.stage0Size != 0)
                  msg.append(String.format(" (%d%%)", (stage2Size*100L/(long) bi.stage0Size)));
            }

            // Optionally add hash
            if (evt.getHash() != null) 
               msg.append(String.format("  [%s]", Integer.toHexString(evt.getHash())));

            this.ps.println(msg.toString());         
         }
      }
      else if ((evt.getType() == Event.Type.AFTER_HEADER_DECODING) && (this.level >= 3))
      {
         this.ps.println(evt);
      }
      else if (this.level >= 5)
      {
         this.ps.println(evt);
      }
   }
   
   
   static class BlockInfo
   {
      long time0;
      long time1;
      long time2;
      long time3;
      long stage0Size;
      long stage1Size;
   }
}
