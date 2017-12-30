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

package kanzi.util.sort;

import kanzi.ArrayComparator;


public final class DefaultArrayComparator implements ArrayComparator
{
    private final int[] array;
    
    
    public DefaultArrayComparator(int[] array)
    {
        if (array == null)
            throw new NullPointerException("Invalid null array parameter");
        
        this.array = array;
    }
        
    
    @Override
    public int compare(int lidx, int ridx)
    {
        int res = this.array[lidx] - this.array[ridx];
        
        // Make the sort stable
        if (res == 0)
           res = lidx - ridx;
        
        return res;
    }
}