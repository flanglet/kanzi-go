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

public class BlockEvent 
{
   public enum Type
   {
      BEFORE_TRANSFORM,
      AFTER_TRANSFORM,
      BEFORE_ENTROPY,
      AFTER_ENTROPY
   }

   private final int id;
   private final int size;
   private final int hash;
   private final Type type;
   private final boolean hashing;
   
   
   public BlockEvent(Type type, int id, int size)
   {
      this(type, id, size, 0, false);
   }

   
   public BlockEvent(Type type, int id, int size, int hash)
   {
      this(type, id, size, hash, true);
   }
   
   
   protected BlockEvent(Type type, int id, int size, int hash, boolean hashing)
   {
      this.id = id;
      this.size = size;
      this.hash = hash;
      this.hashing = hashing;
      this.type =  type;
   }
   
   
   public int getId() 
   {
      return this.id;
   }

   
   public int getSize() 
   {
      return this.size;
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
      StringBuilder sb = new StringBuilder(200);
      sb.append("[").append(this.type);
      sb.append(",").append(this.id);
      sb.append(",").append(this.size);
      
      if (this.hashing == true)
         sb.append(",").append(Integer.toHexString(this.hash));

      sb.append("]");
      return sb.toString();
   }
}
