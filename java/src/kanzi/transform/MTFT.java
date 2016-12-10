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

package kanzi.transform;


import kanzi.ByteTransform;
import kanzi.SliceByteArray;

// The Move-To-Front Transform is a simple reversible transform based on
// permutation of the data in the original message to reduce the entropy.
// See http://en.wikipedia.org/wiki/Move-to-front_transform
// Fast implementation using double linked lists to minimize the number of lookups

public final class MTFT implements ByteTransform
{
    private static final int RESET_THRESHOLD = 64;
    private static final int LIST_LENGTH = 17;

    private final Payload[] heads; // linked lists
    private final int[] lengths;   // length of linked list
    private final byte[] buckets;  // index of list
    private Payload anchor;


    public MTFT()
    {
        this.heads = new Payload[16];
        this.lengths = new int[16];
        this.buckets = new byte[256];
    }


    @Override
    public boolean inverse(SliceByteArray src, SliceByteArray dst)
    {
        if ((!SliceByteArray.isValid(src)) || (!SliceByteArray.isValid(dst)))
           return false;

        if (src.array == dst.array)
           return false;
                      
        final int count = src.length;

        if (dst.length < count)
           return false;
        
        if (dst.index + count > dst.array.length)
           return false;

        final byte[] indexes = this.buckets;

        for (int i=0; i<indexes.length; i++)
            indexes[i] = (byte) i;

        final byte[] input = src.array;
        final byte[] output = dst.array;
        final int srcIdx = src.index;
        final int dstIdx = dst.index;

        for (int i=0; i<count; i++)
        {
           int idx = input[srcIdx+i];
           
           if (idx == 0)
           {
              // Shortcut
              output[dstIdx+i] = indexes[0];
              continue;
           }
           
           idx &= 0xFF;
           final byte value = indexes[idx];
           output[dstIdx+i] = value;

           if (idx <= 16)
           {                
               for (int j=idx-1; j>=0; j--)
                 indexes[j+1] = indexes[j];                 
           }
           else
           {
              System.arraycopy(indexes, 0, indexes, 1, idx);
           }

           indexes[0] = value;
        }   
       
        src.index += count;
        dst.index += count;
        return true;
    }


    // Initialize the linked lists: 1 item in bucket 0 and LIST_LENGTH in each other
    // Used by forward() only
    private void initLists()
    {
        Payload[] buf = new Payload[257];
        buf[0] = new Payload((byte) 0);
        Payload previous = buf[0];
        this.heads[0] = previous;
        this.lengths[0] = 1;
        this.buckets[0] = 0;
        byte listIdx = 0;
                
        for (int i=1; i<256; i++)
        {
           buf[i] = new Payload((byte) i);

           if ((i-1) % LIST_LENGTH == 0)
           {
              listIdx++;
              this.heads[listIdx] = buf[i];
              this.lengths[listIdx] = LIST_LENGTH;
           }

           this.buckets[i] = listIdx;
           previous.next = buf[i];
           buf[i].previous = previous;
           previous = buf[i];
        }
 
        // Create a fake end payload so that every payload in every list has a successor
        buf[256] = new Payload((byte) 0);
        this.anchor = buf[256];
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
    public boolean forward(SliceByteArray src, SliceByteArray dst)
    {
        if ((!SliceByteArray.isValid(src)) || (!SliceByteArray.isValid(dst)))
           return false;

        if (src.array == dst.array)
           return false;
                    
        final int count = src.length;
           
        if (dst.length < count)
           return false;
       
        if (dst.index + count > dst.array.length)
           return false;
        
        if (this.anchor == null)
           this.initLists();
        else
           this.balanceLists(true);
        
       final byte[] input = src.array;
       final byte[] output = dst.array;
       final int srcIdx = src.index;
       final int dstIdx = dst.index;
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
             
             this.buckets[current & 0xFF] = 0;

             if ((this.lengths[0] >= RESET_THRESHOLD))
             {
                this.balanceLists(false);
             }
             else
             {
                this.lengths[listIdx]--;
                this.lengths[0]++;                
             }
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