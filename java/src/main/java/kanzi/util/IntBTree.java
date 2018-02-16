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
package kanzi.util;


// A tree based collection of sorted integers allowing log n time for add/remove
// and fast access to minimum and maximum values (usually constant time, else log n time).
// Not thread safe.
public final class IntBTree
{
   private static final byte MAX_DIRTY = 1;
   private static final byte MIN_DIRTY = 2;
   private static final int LOG_NODE_BUFFER_SIZE = 4;
   private static final int NODE_BUFFER_SIZE = 1 << LOG_NODE_BUFFER_SIZE;
   private static final int MASK_NODE_BUFFER = NODE_BUFFER_SIZE - 1;

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
      if (this.root == null)
      {
         this.root = new IntBTNode(val);
         this.min = val;
         this.max = val;
      }
      else
      {
         addValue(this.root, val);

         if (val < this.min)
            this.min = val;

         if (val > this.max)
            this.max = val;
      }

      this.size++;
   }


   // Add existing node
   private static void addNode(IntBTNode parent, IntBTNode node)
   {
      final int value = node.base;

      while (true)
      {
         if (value < parent.base)
         {
            if (parent.left == null)
            {
               parent.left = node;
               return;
            }

            parent = parent.left;
         }
         else if (value >= parent.base + NODE_BUFFER_SIZE)
         {
            if (parent.right == null)
            {
               parent.right = node;
               return;
            }

            parent = parent.right;
         }
         else
            break;
      }

      parent.counts[value&MASK_NODE_BUFFER]++;
   }


   // Same as addNode but node is created lazily
   private static void addValue(IntBTNode parent, int value)
   {
      while (parent != null)
      {
         if (value < parent.base)
         {
            if (parent.left == null)
            {
               parent.left = new IntBTNode(value);
               break;
            }

            parent = parent.left;
         }
         else if (value >= parent.base + NODE_BUFFER_SIZE)
         {
            if (parent.right == null)
            {
               parent.right = new IntBTNode(value);
               break;
            }

            parent = parent.right;
         }
         else
         {
            parent.counts[value&MASK_NODE_BUFFER]++;
            break;
         }
      }
   }


   // Return the number of matches
   public int contains(int value)
   {
      IntBTNode res = findNode(this.root, value);
      return (res == null) ? 0 : res.counts[value&MASK_NODE_BUFFER];
   }


   private static IntBTNode findNode(IntBTNode current, int value)
   {
      while (current != null)
      {
         if (value < current.base)
            current = current.left;
         else if (value >= current.base + NODE_BUFFER_SIZE)
            current = current.right;
         else
            break;
      }

      return current;
   }


   public boolean remove(int value)
   {
      if (this.root == null)
         return false;

      if (this.removeNode(value) == null)
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

      while (true)
      {
         if (value < current.base)
         {
            if (current.left == null)
               return null;

            prev = current;
            current = current.left;
         }
         else if (value >= current.base + NODE_BUFFER_SIZE)
         {
            if (current.right == null)
               return null;

            prev = current;
            current = current.right;
         }
         else
            break;
      }

      // Found target
      current.counts[value&MASK_NODE_BUFFER]--;

      if (current.counts[value&MASK_NODE_BUFFER] != 0)
         return current;

      for (int i=0; i<NODE_BUFFER_SIZE; i++)
      {
         if (current.counts[i] != 0)
            return current;
      }
      
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

               // Re-insert right branch
               if (current.right != null)
                  addNode(prev, current.right);
            }
            else
               prev.right = current.right;
         }
         else
         {
            prev.left = current.left;

            // Re-insert right branch
            if (current.right != null)
               addNode(prev, current.right);
         }
      }

      return current;
   }


   public int rank(int value) 
   {
      if (this.root == null)
         return -1;
      
      if (this.min() == value)
         return 0;
      
      int rank = findRank(this.root, value, 0);
      return (rank == this.size) ? -1 : -rank;
   }
   
   
   private static int findRank(IntBTNode current, int value, int rank)
   { 
      if ((rank >= 0) && (current.left != null))
         rank = findRank(current.left, value, rank);

      for (int i=0; i<NODE_BUFFER_SIZE; i++)
      {
         if (value == current.base + i)
            return -rank; 

         if (rank >= 0)
            rank += current.counts[i];
      }
      
      if ((rank >= 0) && (current.right != null))
         rank = findRank(current.right, value, rank);
         
      return rank;
   }
   
   
   public int[] scan(Callback cb, boolean reverse)
   {
      if ((cb == null) || (this.root == null))
         return new int[0];

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

         index = cb.call(current, array, index, false);

         if (current.right != null)
            index = scanAndCall(current.right, array, index, cb, false);
      }
      else
      {
         if (current.right != null)
            index = scanAndCall(current.right, array, index, cb, true);

         index = cb.call(current, array, index, true);

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

         for (int i=0; i<NODE_BUFFER_SIZE; i++)
         {
            if (node.counts[i] > 0)
            {
               this.min = node.base + i;
               break;
            }
         }

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

         for (int i=NODE_BUFFER_SIZE-1; i>=0; i--)
         {
            if (node.counts[i] > 0)
            {
               this.max = node.base + i;
               break;
            }
         }

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
         public int call(IntBTNode node, int[] values, int idx, boolean reverse)
         {
            return node.values(values, idx, reverse);
         }
      };

      scanAndCall(this.root, res, 0, cb, false);
      return res;
   }


   @Override
   public String toString()
   {
      if (this.size() == 0)
         return "[]";
      
      int[] res = new int[this.size()];
      
      Callback cb = new Callback()
      {
         @Override
         public int call(IntBTNode node, int[] values, int idx, boolean reverse)
         {
            return node.values(values, idx, reverse);
         }
      };
      
      scanAndCall(this.root, res, 0, cb, false);
      StringBuilder sb = new StringBuilder(res.length*5);
      sb.append('[');
      
      if (res.length > 0)
      {
         sb.append(res[0]);
         
         for (int i=1; i<res.length; i++)
            sb.append(',').append(res[i]);
      }
      
      sb.append(']');
      return sb.toString();
   }
   
   
   // Interface to implement visitor pattern. Must return the node value
   public interface Callback
   {
      public int call(IntBTNode node, int[] values, int idx, boolean reverse);
   }


   // A node containing a range of integers
   public static class IntBTNode
   {
      protected final int base;  // range base
      protected final int[] counts;
      protected IntBTNode left;
      protected IntBTNode right;

      IntBTNode(int val)
      {
         this.base = val & -NODE_BUFFER_SIZE;
         this.counts = new int[NODE_BUFFER_SIZE];
         this.counts[val&MASK_NODE_BUFFER]++;
      }

      public int values(int[] values, int idx, boolean reverse)
      {
         if (reverse == true)
         {
            for (int i=NODE_BUFFER_SIZE-1; i>=0; i--)
            {
               for (int j=this.counts[i]; j>0; j--)
                  values[idx++] = this.base + i;
            }
         }
         else
         {
            for (int i=0; i<NODE_BUFFER_SIZE; i++)
            {
               for (int j=this.counts[i]; j>0; j--)
                  values[idx++] = this.base + i;
            }
         }
         
         return idx;
      }
   }


}
