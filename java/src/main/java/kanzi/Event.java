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

package kanzi;

public class Event 
{
   public enum Type
   {
      COMPRESSION_START,
      DECOMPRESSION_START,
      BEFORE_TRANSFORM,
      AFTER_TRANSFORM,
      BEFORE_ENTROPY,
      AFTER_ENTROPY,
      COMPRESSION_END,
      DECOMPRESSION_END,
      AFTER_HEADER_DECODING
   }

   private final int id;
   private final long size;
   private final int hash;
   private final Type type;
   private final boolean hashing;
   private final long time;
   private final String msg;
      

   public Event(Type type, int id, long size)
   {
      this(type, id, size, 0, false);
   }
   
   
   public Event(Type type, int id, String msg)
   {
      this(type, id, msg, 0);
   }
   
   
   public Event(Type type, int id, String msg, long time)
   {
      this.id = id;
      this.size = 0L;
      this.hash = 0;
      this.hashing = false;
      this.type = type;
      this.time = (time > 0) ? time : System.nanoTime();
      this.msg = msg;
   }
   
   
   public Event(Type type, int id, long size, int hash, boolean hashing)
   {
      this(type, id, size, hash, hashing, 0);
   }
   
   
   public Event(Type type, int id, long size, int hash, boolean hashing, long time)
   {
      this.id = id;
      this.size = size;
      this.hash = hash;
      this.hashing = hashing;
      this.type = type;
      this.time = (time > 0) ? time : System.nanoTime();
      this.msg = null;
   }
   
   
   public int getId() 
   {
      return this.id;
   }

   
   public long getSize() 
   {
      return this.size;
   }

   
   public long getTime() 
   {
      return this.time;
   }

   
   public Integer getHash() 
   {
      return (this.hashing == false) ? null : this.hash;
   }  
   
   
   public Type getType()
   {
      return this.type;
   }
  
   
   @Override
   public String toString()
   {
      if (this.msg != null)
         return this.msg;
      
      StringBuilder sb = new StringBuilder(200);
      sb.append("{ \"type\":\"").append(this.getType()).append("\"");
      
      if (this.id >= 0)
         sb.append(", \"id\":").append(this.getId());
      
      sb.append(", \"size\":").append(this.getSize());
      sb.append(", \"time\":").append(this.getTime());
      
      if (this.hashing == true)
         sb.append(", \"hash\":").append(Integer.toHexString(this.getHash()));

      sb.append(" }");
      return sb.toString();
   }
}
