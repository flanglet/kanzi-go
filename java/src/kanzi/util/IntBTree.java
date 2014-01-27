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
package kanzi.util;


// A tree based collection of sorted integers allowing log n time for add/remove 
// and fast access to minimum and maximum values (usually constant time, else log n time). 
// Not thread safe.
public final class IntBTree 
{
   private static final byte MAX_DIRTY = 1;
   private static final byte MIN_DIRTY = 2;

   private IntBTNode root;
   private int size;
   private byte flags;
   private int min;
   private int max;

   
   public IntBTree() 
   {
   }

   
   public int size() 
   {
      return this.size;
   }

   
   public void add(int val) 
   {
      IntBTNode node = new IntBTNode(val, 1);

      if (this.root == null) 
      {
         this.root = node;
         this.min = val;
         this.max = val;
      }
      else
      {
         addNode(this.root, node);

         if (val < this.min)
            this.min = val;

         if (val > this.max)
            this.max = val;
      }
   
      this.size++;
   }

   
   private static void addNode(IntBTNode parent, IntBTNode node) 
   {
      final int value = node.value;
      
      while (value != parent.value)
      {
         if (value < parent.value) 
         {
            if (parent.left == null)
            {
               parent.left = node;
               return;
            }
            
            parent = parent.left;
         } 
         else
         {
            if (parent.right == null)
            {
               parent.right = node;
               return;
            }
            
            parent = parent.right;
         }
      }

      parent.count++;
   }

   
   // Return the number of matches
   public int contains(int value)
   {
      IntBTNode res = findNode(this.root, value);      
      return (res == null) ? 0 : res.count;
   }
   
   
   private static IntBTNode findNode(IntBTNode current, int value) 
   {
      if (current == null)
         return null;
    
      while (value != current.value)
      {
         if (value < current.value) 
         {
            if (current.left == null)
               return null;

            current = current.left;
         } 
         else
         {
            if (current.right == null)
               return null;

            current = current.right;
         }
      }
      
      return current;
   }
      
   
   public boolean remove(int value) 
   {
      if (this.root == null)
         return false;

      IntBTNode res = this.removeNode(value);

      if (res == null)
         return false;

      this.size--;

      // Force recomputation of cached fields
      if (this.min == value)         
         this.flags |= MIN_DIRTY;

      if (this.max == value)         
         this.flags |= MAX_DIRTY;
      
      return true;
   }

   
   private IntBTNode removeNode(int value) 
   {
      IntBTNode current = this.root;
      IntBTNode prev = null;
      
      while (value != current.value)
      {
         if (value < current.value) 
         {
            if (current.left == null)
               return null;

            prev = current;
            current = current.left;
         } 
         else
         {
            if (current.right == null)
               return null;

            prev = current;
            current = current.right;
         }
      }

      // Found target
      current.count--;

      if (current.count != 0)
         return current;
      
      if (current == this.root)
      {
         // First, try easy substitutions of root
         if (current.right == null)
            this.root = current.left;
         else if (current.left == null)
            this.root = current.right;
         else if ((value & 1) == 0) // random choice or left or right 
         {
            this.root = current.right;
           
            // Re-insert left branch
            addNode(this.root, current.left);
         }
         else 
         {
            this.root = current.left;
           
            // Re-insert right branch
            addNode(this.root, current.right);
         }
      }
           
      if (prev != null)
      {
         // Remove current node from previous node
         if (prev.right == current) 
         {
            if (current.left != null) 
            {
               prev.right = current.left;

               if (current.right != null) 
               {
                  // Re-insert right branch
                  addNode(prev, current.right);
               }
            } 
            else
               prev.right = current.right;
         } 
         else 
         {
            prev.left = current.left;

            if (current.right != null) 
            {
               // Re-insert right branch
               addNode(prev, current.right);
            }
         }
      }

      return current;
   }

   
   public int[] scan(Callback cb, boolean reverse) 
   {
      if ((cb == null) || (this.root == null))
         return null;

      int[] res = new int[this.size];
      scanAndCall(this.root, res, 0, cb, reverse); // visitor pattern
      return res;
   }

   
   private static int scanAndCall(IntBTNode current, int[] array, int index, Callback cb, boolean reverse)
   {
      if (reverse == false) 
      {
         if (current.left != null)
            index = scanAndCall(current.left, array, index, cb, false);

         for (int i=current.count; i>0; i--)
            array[index++] = cb.call(current);

         if (current.right != null)
            index = scanAndCall(current.right, array, index, cb, false);
      } 
      else 
      {
         if (current.right != null)
            index = scanAndCall(current.right, array, index, cb, true);

         for (int i=current.count; i>0; i--)
            array[index++] = cb.call(current);

         if (current.left != null)
            index = scanAndCall(current.left, array, index, cb, true);
      }
      
      return index;
   }

   
   public void clear()
   {
      this.root = null;
      this.size = 0;
   }
   
   
   public int min()
   {
      if (this.root == null)
         throw new IllegalStateException("Tree is empty");

      if ((this.flags & MIN_DIRTY) != 0)
      {             
         // Dynamically scan tree to leftmost position
         IntBTNode node = this.root;

         while (node.left != null)
            node = node.left;

         this.min = node.value;
         this.flags &= ~MIN_DIRTY;
      }
      
      return this.min;
   }

   
   public int max() 
   {
      if (this.root == null)
         throw new IllegalStateException("Tree is empty");

      if ((this.flags & MAX_DIRTY) != 0)
      {
         // Dynamically scan tree to rightmost position
         IntBTNode node = this.root;

         while (node.right != null) 
            node = node.right;

         this.max = node.value;
         this.flags &= ~MAX_DIRTY;
      }
      
      return this.max;
   }

   
   public int[] toArray(int[] array)
   {
      if (this.root == null)
         return new int[0];

      if ((array == null) || (array.length < this.size))
         array = new int[this.size];
   
      final int[] res = array;
      
      Callback cb = new Callback()
      {
         @Override
         public int call(IntBTNode node) 
         {
            return node.value();
         }        
      };   
      
      scanAndCall(this.root, res, 0, cb, false);
      return res;
   }
   
   
   // Interface to implement visitor pattern. Must return the node value
   public interface Callback 
   {
      public int call(IntBTNode node);
   }

  
   // A node containing an integer (one or several times)
   public static class IntBTNode 
   {
      protected int value;
      protected int count;
      protected IntBTNode left;
      protected IntBTNode right;

      IntBTNode(int val, int count) 
      {
         this.value = val;
         this.count = count;
      }
      
      public int value()
      {
         return this.value;
      }
   }
   

}
