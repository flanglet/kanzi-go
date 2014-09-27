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

package kanzi.transform;


import kanzi.ByteTransform;
import kanzi.IndexedByteArray;
import kanzi.Sizeable;

// The Move-To-Front Transform is a simple reversible transform based on
// permutation of the data in the original message to reduce the entropy.
// See http://en.wikipedia.org/wiki/Move-to-front_transform
// Fast implementation using double linked lists to minimize the number of lookups

public final class MTFT implements ByteTransform, Sizeable
{
    private static final int RESET_THRESHOLD = 64;
    private static final int LIST_LENGTH = 17;

    private int size;
    private final Payload[] heads; // linked lists
    private final int[] lengths;   // length of linked list
    private final byte[] buckets;  // index of list
    private Payload anchor;
    

    public MTFT()
    {
        this(0);
    }


    public MTFT(int size)
    {
        if (size < 0)
            throw new IllegalArgumentException("Invalid size parameter (must be at least 0)");

        this.size = size;
        this.heads = new Payload[16];
        this.lengths = new int[16];
        this.buckets = new byte[256];
    }


    @Override
    public boolean inverse(IndexedByteArray src, IndexedByteArray dst)
    {
        final byte[] indexes = this.buckets;

        for (int i=0; i<indexes.length; i++)
            indexes[i] = (byte) i;

        final byte[] input = src.array;
        final byte[] output = dst.array;
        final int srcIdx = src.index;
        final int dstIdx = dst.index;
        final int count = (this.size == 0) ? input.length - srcIdx : this.size;

        for (int i=0; i<count; i++)
        {
            int idx = input[srcIdx+i] & 0xFF;     
            final byte value = indexes[idx];
            System.arraycopy(indexes, 0, indexes, 1, idx);
            indexes[0] = value;
            output[dstIdx+i] = value;
        }

        src.index += count;
        dst.index += count;
        return true;
    }


    @Override
    public boolean setSize(int size)
    {
        if (size < 0) // 0 is valid
            return false;

        this.size = size;
        return true;
    }


    // Not thread safe
    @Override
    public int size()
    {
       return this.size;
    }


    // Initialize the linked lists: 1 item in bucket 0 and LIST_LENGTH in each other
    // Used by forward() only
    private void initLists()
    {
        Payload[] array = new Payload[256];
        array[0] = new Payload((byte) 0);
        Payload previous = array[0];
        this.heads[0] = previous;
        this.lengths[0] = 1;
        this.buckets[0] = 0;
        byte listIdx = 0;
                
        for (int i=1; i<256; i++)
        {
           array[i] = new Payload((byte) i);

           if ((i-1) % LIST_LENGTH == 0)
           {
              listIdx++;
              this.heads[listIdx] = array[i];
              this.lengths[listIdx] = LIST_LENGTH;
           }

           this.buckets[i] = listIdx;
           previous.next = array[i];
           array[i].previous = previous;
           previous = array[i];
        }
 
        // Create a fake end payload so that every payload in every list has a successor
        this.anchor = new Payload((byte) 0);
        previous.next = this.anchor;   
    }
    

    // Recreate one list with 1 item and 15 lists with LIST_LENGTH items
    // Update lengths and buckets accordingly. 
    // Used by forward() only
    private void balanceLists(boolean resetValues)
    {
       this.lengths[0] = 1;
       Payload p = this.heads[0].next;
       byte val = 0;

       if (resetValues == true)
       {
          this.heads[0].value = (byte) 0;
          this.buckets[0] = 0;
       }

       for (byte listIdx=1; listIdx<16; listIdx++)
       {
          this.heads[listIdx] = p;
          this.lengths[listIdx] = LIST_LENGTH;

          for (int n=0; n<LIST_LENGTH; n++)
          {
             if (resetValues == true)
                p.value = ++val;

             this.buckets[p.value & 0xFF] = listIdx;
             p = p.next;
          }
       }
    }


    @Override
    public boolean forward(IndexedByteArray src, IndexedByteArray dst)
    {
        if (this.anchor == null)
           this.initLists();
        else
           this.balanceLists(true);
        
       final byte[] input = src.array;
       final byte[] output = dst.array;
       final int srcIdx = src.index;
       final int dstIdx = dst.index;
       final int count = (this.size == 0) ? input.length - srcIdx :  this.size;
       byte previous = this.heads[0].value;

       for (int i=0; i<count; i++)
       {
          final byte current = input[srcIdx+i];

          if (current == previous)
          {
             output[dstIdx+i] = 0;
             continue;
          }

          // Find list index
          final int listIdx = this.buckets[current & 0xFF];
          Payload p = this.heads[listIdx];
          int idx = 0;

          for (int ii=0; ii<listIdx; ii++)
             idx += this.lengths[ii];
          
          // Find index in list (less than RESET_THRESHOLD iterations)
          while (p.value != current)
          {
             p = p.next;
             idx++;
          }

          output[dstIdx+i] = (byte) idx;

          // Unlink (the end anchor ensures p.next != null)
          p.previous.next = p.next;
          p.next.previous = p.previous;

          // Add to head of first list
          p.next = this.heads[0];
          p.next.previous = p;
          this.heads[0] = p;

          // Update list information
          if (listIdx != 0)
          {
             // Update head if needed
             if (p == this.heads[listIdx])
                this.heads[listIdx] = p.previous.next;
             
             this.lengths[listIdx]--;
             this.lengths[0]++;
             this.buckets[current & 0xFF] = 0;

             if ((this.lengths[0] > RESET_THRESHOLD) || (this.lengths[listIdx] == 0))
                this.balanceLists(false);
          }

          previous = current;
       }
       
       src.index += count;
       dst.index += count;
       return true;
    }


    private static class Payload
    {
        protected Payload previous;
        protected Payload next;
        protected byte value;


        Payload(byte value)
        {
            this.value = value;
        }
    }
}