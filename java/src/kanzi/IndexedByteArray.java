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

package kanzi;


public final class IndexedByteArray
{
    public byte[] array;
    public int index;
    
    
    public IndexedByteArray(byte[] array, int idx)
    {
        if (array == null)
           throw new NullPointerException("The array cannot be null");
        
        this.array = array;
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

            IndexedByteArray iba = (IndexedByteArray) o;
            return ((this.array == iba.array) && (this.index == iba.index));
        }
        catch (ClassCastException e)
        {
            return false;
        }
    }


    @Override
    public int hashCode()
    {
       // Non constant !
       return this.index + ((this.array == null) ? 0 : (17 * this.array.hashCode()));
    }

    
    @Override
    public String toString()
    {
        StringBuilder builder = new StringBuilder(100);
        builder.append("[");
        builder.append(String.valueOf(this.array));
        builder.append(","); 
        builder.append(this.index); 
        builder.append("]"); 
        return builder.toString();
    }
}
