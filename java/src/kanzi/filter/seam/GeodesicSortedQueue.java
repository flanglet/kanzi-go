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

package kanzi.filter.seam;

// Not thread safe
// Tree based sorted queue of geodesics
/*package*/ class GeodesicSortedQueue
{
    private final int maxSize;
    private int size;
    private Node head;
    private Node tail;
    private final Node[] nodes;
    private int freeNodeIdx;


    public GeodesicSortedQueue(int maxSize)
    {
        this.maxSize = maxSize;
        this.nodes = new Node[maxSize+1];

        // Pre-allocate
        for (int i=0; i<this.nodes.length; i++)
           this.nodes[i] = new Node(null, null, i);
    }


    // Grow queue until maxSize is reached
    // Return last value (geodesic with highest cost) in ordered collection
    // Null input value is not allowed
    public Geodesic add(Geodesic value)
    {
        if (this.size == 0)
        {
           Node node = this.nodes[this.freeNodeIdx++];
           node.value = value;
           this.head = node;
           this.tail = node;
           this.size++;
           return this.tail.value;
        }

        final int cost = value.cost; // aliasing

        if (cost >= this.tail.value.cost)
        {
           // Cost too high, do not add
           if (this.size == this.maxSize)
             return this.tail.value;

           // New node is tail
           Node node = this.nodes[this.freeNodeIdx++];
           node.value = value;
           node.parent = this.tail;
           this.tail.right = node;
           node.right = null;
           this.tail = node;
        }
        else if (cost < this.head.value.cost)
        {
           // New node is head
           Node node = this.nodes[this.freeNodeIdx++];
           node.value = value;
           this.head.parent = node;
           node.right = this.head;
           node.left = null;
           this.head = node;
        }
        else
        {
           // New node is not an extremity
           Node current = this.head;

           // Locate appropriate position in tree
           while (true)
           {
              if (cost > current.value.cost)
              {
                 if (current.right != null)
                    current = current.right;
                 else
                 {
                    Node node = this.nodes[this.freeNodeIdx++];
                    node.value = value;
                    node.parent = current;
                    current.right = node;
                    break;
                 }
              }
              else
              {
                 if (current.left != null)
                    current = current.left;
                 else
                 {
                    Node node = this.nodes[this.freeNodeIdx++];
                    node.value = value;
                    node.parent = current;
                    current.left = node;
                    break;
                 }
              }
           } 
        }

        if (this.size >= this.maxSize)
        {
           // Need to recompute tail
           Node last = this.tail;

           // Recycle evicted node
           this.freeNodeIdx = last.idx;

           if (last.left != null)
           {
               final Node left = last.left;
               final Node parent = last.parent;
               left.parent = parent;

               if (parent != null)
                  parent.right = left;
               
               last.parent = null;
               last.left = null;
               last = left;

               while (last.right != null)
                   last = last.right;

               // Set new tail to rightmost descendant of 'left'
               this.tail = last;
           }
           else
           {
              // Set new tail to parent of current tail
              this.tail = last.parent;
              this.tail.right = null;
              last.parent = null;
           }
        }
        else
        {
           this.size++;
        }

        return this.tail.value;
    }


    public boolean isFull()
    {
        return (this.size == this.maxSize);
    }


    public Geodesic getLast()
    {
        return (this.tail == null) ? null : this.tail.value;
    }


    public Geodesic getFirst()
    {
        return (this.head == null) ? null : this.head.value;
    }


    public int size()
    {
        return this.size;
    }


    public Geodesic[] toArray(Geodesic[] array)
    {
        if (this.size == 0)
           return new Geodesic[0];

        if (array.length < this.size)
           array = new Geodesic[this.size];

        scan(this.head, array, 0);
        return array;
    }


    private static int scan(Node n, Geodesic[] array, int idx)
    {
       if (n.left != null)
          idx = scan(n.left, array, idx);

       array[idx++] = n.value;

       if (n.right != null)
          idx = scan(n.right, array, idx);

       return idx;
    }



    private static class Node
    {
        Node left;
        Node right;
        Node parent;
        Geodesic value;
        final int idx;

        Node(Node parent, Geodesic value, int idx)
        {
            this.parent = parent;
            this.value = value;
            this.idx = idx;
        }
    }


    @Override
    public String toString()
    {
       StringBuilder builder = new StringBuilder((1+this.size)*8);
       Geodesic[] array = this.toArray(new Geodesic[this.size]);

       builder.append("Size=");
       builder.append(this.size);
       builder.append("\n");

       for (int i=0; i<array.length; i++)
       {
          if (i != 0)
             builder.append(",\n");

          builder.append(array[i]);
       }

       return builder.toString();
    }

}
