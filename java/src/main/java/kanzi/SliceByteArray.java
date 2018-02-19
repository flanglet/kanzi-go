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

import java.util.Objects;


// A lightweight slice implementation for byte[]
public final class SliceByteArray
{
    public byte[] array; // array.length is the slice capacity
    public int length;    
    public int index;
    
    
    public SliceByteArray()
    {
       this(new byte[0], 0, 0);
    }
    
        
    public SliceByteArray(byte[] array, int idx)
    {
        if (array == null)
           throw new NullPointerException("The array cannot be null");
        
        if (idx < 0)
           throw new NullPointerException("The index cannot be negative");
        
        this.array = array;
        this.length = array.length;
        this.index = idx;       
    }  
    
    
    public SliceByteArray(byte[] array, int length, int idx)
    {
        if (array == null)
           throw new NullPointerException("The array cannot be null");
        
        if (length < 0)
           throw new IllegalArgumentException("The length cannot be negative");
        
        if (idx < 0)
           throw new NullPointerException("The index cannot be negative");
        
        this.array = array;
        this.length = length;
        this.index = idx;
    }
    
    
    @Override
    public boolean equals(Object o)
    {
        try
        {
            if (o == null)
               return false;

            if (this == o)
               return true;

            SliceByteArray sa = (SliceByteArray) o;
            return ((this.array == sa.array)   && 
                    (this.length == sa.length) && 
                    (this.index == sa.index));
        }
        catch (ClassCastException e)
        {
            return false;
        }
    }


    @Override
    public int hashCode()
    {
       return Objects.hashCode(this.array);
    }

    
    @Override
    public String toString()
    {
        StringBuilder builder = new StringBuilder(100);
        builder.append("[");
        builder.append(String.valueOf(this.array));
        builder.append(","); 
        builder.append(this.length); 
        builder.append(","); 
        builder.append(this.index); 
        builder.append("]"); 
        return builder.toString();
    }
    
    
    public static boolean isValid(SliceByteArray sa)
    {
       if (sa == null)
          return false;
       
       if (sa.array == null)
          return false;
       
       if (sa.index < 0)
          return false;
       
       if (sa.length < 0)
          return false;
       
       return (sa.index + sa.length <= sa.array.length);
    }    
}
