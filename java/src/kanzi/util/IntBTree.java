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
// and fast access to minimum and maximum values (almost always constant time). 
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
      this.flags = MIN_DIRTY | MAX_DIRTY;
   }

   
   public int size() 
   {
      return this.size;
   }

   
   public void add(int val) 
   {
      IntBTNode node = new IntBTNode(val, 1);

      if (this.root == null) 
         this.root = node;
      else
         this.scanAndAdd(this.root, node);

      if (val < this.min)
         this.min = val;
      
      if (val > this.max)
         this.max = val;
      
      this.size++;
   }

   
   private void scanAndAdd(IntBTNode parent, IntBTNode node) 
   {
      if (parent == null) 
         return;

      if (node.value < parent.value) 
      {
         if (parent.left == null)
            parent.left = node;
         else
            this.scanAndAdd(parent.left, node);
      } 
      else if (node.value > parent.value) 
      {
         if (parent.right == null)
            parent.right = node;
         else
            this.scanAndAdd(parent.right, node);
      }
      else 
      {
         parent.count++;
      }
   }

   
   public IntBTNode remove(int value) 
   {
      if (this.root == null)
         return null;

      IntBTNode res = this.scanAndRemove(this.root, null, value, true);

      if ((res == this.root) && (this.root.count == 0)) 
      {
         if (res.right != null)
            this.root = res.right;
         else if (res.left != null)
            this.root = res.left;
         else
            this.root = null;
      }

      if (res != null)
      {
         this.size = (this.root == null) ? 0 : this.size - 1;
      
         // Force recomputation of cached fields
         if (this.min == value)         
            this.flags |= MIN_DIRTY;

         if (this.max == value)         
            this.flags |= MAX_DIRTY;
      }
      
      return res;
   }

   
   private IntBTNode scanAndRemove(IntBTNode current, IntBTNode prev, int value, boolean right) 
   {
      if (current == null)
         return null;

      if (value < current.value) 
      {
         if (current.left == null)
            return null;
         else
            return this.scanAndRemove(current.left, current, value, false);
      } 
      else if (value > current.value) 
      {
         if (current.right == null)
            return null;
         else
            return this.scanAndRemove(current.right, current, value, true);
      }

      current.count--;

      if ((current.count == 0) && (prev != null)) 
      {
         if (right == true) 
         {
            if (current.left != null) 
            {
               prev.right = current.left;

               if (current.right != null) 
               {
                  // Re-insert right branch
                  this.scanAndAdd(this.root, current.right);
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
               this.scanAndAdd(this.root, current.right);
            }
         }
      }

      return current;
   }

   
   public void scan(Callback cb, boolean reverse) 
   {
      if (cb == null)
         return;

      scanAndCall(this.root, cb, reverse); // visitor pattern
   }

   
   private static void scanAndCall(IntBTNode current, Callback cb, boolean reverse) 
   {
      if (current == null)
         return;

      if (reverse == false) 
      {
         if (current.left != null)
            scanAndCall(current.left, cb, reverse);

         for (int i=current.count; i>0; i--)
            cb.call(current);

         if (current.right != null)
            scanAndCall(current.right, cb, reverse);
      } 
      else 
      {
         if (current.right != null)
            scanAndCall(current.right, cb, reverse);

         for (int i=current.count; i>0; i--)
            cb.call(current);

         if (current.left != null)
            scanAndCall(current.left, cb, reverse);
      }
   }

   
   public int min()
   {
      if (this.root == null)
         throw new IllegalStateException("Tree is empty");

      if ((this.flags & MIN_DIRTY) == 0)
         return this.min;
      
      // Dynamically scan tree to leftmost position
      IntBTNode node = this.root;
      int minimum = node.value;

      do
      {
         minimum = node.value;
         node = node.left;
      }
      while (node != null);
      
      this.min = minimum;
      this.flags &= ~MIN_DIRTY;
      return minimum;
   }

   
   public int max() 
   {
      if (this.root == null)
         throw new IllegalStateException("Tree is empty");

      if ((this.flags & MAX_DIRTY) == 0)
         return this.max;
 
      // Dynamically scan tree to rightmost position
      IntBTNode node = this.root;
      int maximum = node.value;

      do 
      {
         maximum = node.value;
         node = node.right;
      }
      while (node != null);
      
      this.max = maximum;
      this.flags &= ~MAX_DIRTY;
      return maximum;
   }

   
   // Interface to implement visitor pattern
   public interface Callback 
   {
      public void call(IntBTNode node);
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
