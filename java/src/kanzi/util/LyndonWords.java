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

import java.util.ArrayList;
import java.util.List;


// Utility class to decompose a String into Lyndon words using the Chen-Fox-Lyndon algorithm
public class LyndonWords 
{
   private final List<Integer> breakpoints;

   
   public LyndonWords()
   {
      this.breakpoints = new ArrayList<Integer>();
   }
   
   
   // Not thread safe
   private List<Integer> ChenFoxLyndonBreakpoints(String s)
   {
      int k = 0;
      final int length = s.length();
      byte[] buf = s.getBytes();
      this.breakpoints.clear();
     
      while (k < length)
      {
         int i = k;
         int j = k + 1;
        
         while ((j < length) && (buf[i] <= buf[j]))
         {
            i = (buf[i] == buf[j]) ? i+1 : k;
            j++;
         }
            
         while (k <= i)
         {
            k += (j-i);
            this.breakpoints.add(k);
         }
      }
      
      return this.breakpoints;
   }
   
   
   // Not thread safe
   public String[] split(String s)
   {
      ChenFoxLyndonBreakpoints(s);
      String[] res = new String[this.breakpoints.size()];
      int n = 0;
      int prev = 0;
     
      for (int bp : this.breakpoints)
      {
         res[n++] = s.substring(prev, bp);
         prev = bp;
      }  
     
      return res;
   }   

   
   public int[] getPositions(String s)
   {      
      ChenFoxLyndonBreakpoints(s);
      int[] res = new int[this.breakpoints.size()];     
      int n = 0;
      
      for (Integer bp : this.breakpoints)
         res[n++] = bp;
      
      return res;
   }   
   
}
