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


public interface IntTransform
{
   // Read src.length ints from src.array[src.index], process them and
   // write them to dst.array[dst.index]. The index of each slice is updated
   // with the number of ints respectively read from and written to.  
   public boolean forward(SliceIntArray src, SliceIntArray dst);


   // Read src.length ints from src.array[src.index], process them and
   // write them to dst.array[dst.index]. The index of each slice is updated
   // with the number of ints respectively read from and written to.  
   public boolean inverse(SliceIntArray src, SliceIntArray dst);
}
